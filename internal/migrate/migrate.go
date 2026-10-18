// Package migrate carries a project across breaking changes in the pipeline
// layout. A migration runs at most once per project — `update` records applied
// ids in the state file — and must be safe to run against a project that
// already looks migrated.
package migrate

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/vseminoy/trunkline/internal/install"
	"github.com/vseminoy/trunkline/internal/semver"
	"github.com/vseminoy/trunkline/internal/state"
)

// Migration is one irreversible step between two core versions.
type Migration struct {
	// ID is stable and recorded in the state file.
	ID string
	// Introduced is the core version that requires this migration. It runs
	// when the installed version is older than this.
	Introduced string
	// What is a one-line description shown to the user.
	What string
	// Apply must be idempotent and must report what it changed.
	Apply func(dir string, dry bool) (string, error)
}

// All migrations, oldest first.
var All = []Migration{
	{
		ID:         "0.1.0-command-rename",
		Introduced: "0.1.0",
		What:       "replace the pipeline-dev slash command with trunkline-dev",
		Apply:      dropOldCommand,
	},
	{
		ID:         "0.3.0-workdir",
		Introduced: "0.3.0",
		What:       "move run state under .pipeline/workdir/, and ADRs and the module base under <knowledge>/<project>/",
		Apply:      moveWorkdir,
	},
}

// Pending returns the migrations a project at version `from` still needs.
// An empty or unparseable `from` is treated as "older than everything", so a
// project installed before versioning existed gets the full set.
func Pending(st *state.State, from, to string) []Migration {
	var out []Migration
	for _, m := range All {
		if st.Applied(m.ID) {
			continue
		}
		if semver.Less(from, m.Introduced) && !semver.Less(to, m.Introduced) {
			out = append(out, m)
		}
	}
	return out
}

func dropOldCommand(dir string, dry bool) (string, error) {
	old := filepath.Join(dir, ".claude", "commands", "pipeline-dev.md")
	if _, err := os.Stat(old); os.IsNotExist(err) {
		return "", nil
	} else if err != nil {
		return "", err
	}
	if dry {
		return "would remove .claude/commands/pipeline-dev.md", nil
	}
	if err := os.Remove(old); err != nil {
		return "", err
	}
	return "removed .claude/commands/pipeline-dev.md (superseded by trunkline-dev)", nil
}

// moveWorkdir gathers the run-state directories under .pipeline/workdir/ and
// moves the ADR directory into the knowledge base. Before this the four
// short-lived directories sat next to knowledge/ and needed four .gitignore
// lines; projects routinely got three of them right and carried the fourth into
// everyone's history. ADRs moved for the opposite reason: they are kept
// forever, so they belong with the other kept artefacts under the one path the
// `knowledge:` flag names.
func moveWorkdir(dir string, dry bool) (string, error) {
	var done []string

	work := filepath.Join(dir, ".pipeline", "workdir")
	for _, name := range []string{"rules", "consistency", "review", "metrics"} {
		src := filepath.Join(dir, ".pipeline", name)
		if _, err := os.Stat(src); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return "", err
		}
		dst := filepath.Join(work, name)
		if occupied, err := inTheWay(dst, dry); err != nil {
			return "", err
		} else if occupied {
			return "", fmt.Errorf("both .pipeline/%s/ and .pipeline/workdir/%s/ hold files; "+
				"merge them by hand, then rerun", name, name)
		}
		if dry {
			done = append(done, "would move .pipeline/"+name+"/ -> .pipeline/workdir/"+name+"/")
			continue
		}
		if err := os.MkdirAll(work, 0o755); err != nil {
			return "", err
		}
		if err := os.Rename(src, dst); err != nil {
			return "", err
		}
		done = append(done, "moved .pipeline/"+name+"/ -> .pipeline/workdir/"+name+"/")
	}

	root := knowledgeFlag(dir)
	if root != "" && root != "none" {
		project := projectFlag(dir)
		kb := path.Join(root, project)

		// The base used to be flat: adr/, modules/ and slices/ directly under
		// the root. A root shared by several services needs one directory per
		// project under it, or the second service's payments.md lands on the
		// first one's. INDEX.md stays at the root on purpose — it is the table
		// of contents over every project in the base.
		for _, name := range []string{"adr", "modules", "slices"} {
			src := filepath.Join(resolve(dir, root), name)
			if _, err := os.Stat(src); os.IsNotExist(err) {
				continue
			} else if err != nil {
				return "", err
			}
			dst := filepath.Join(resolve(dir, kb), name)
			if occupied, err := inTheWay(dst, dry); err != nil {
				return "", err
			} else if occupied {
				return "", fmt.Errorf("both %s/%s/ and %s/%s/ hold files; merge them by hand, "+
					"then rerun", root, name, kb, name)
			}
			if dry {
				done = append(done, "would move "+root+"/"+name+"/ -> "+kb+"/"+name+"/")
				continue
			}
			if err := os.MkdirAll(resolve(dir, kb), 0o755); err != nil {
				return "", err
			}
			if err := os.Rename(src, dst); err != nil {
				return "", err
			}
			done = append(done, "moved "+root+"/"+name+"/ -> "+kb+"/"+name+"/")
		}

		if msg, err := addProjectColumn(dir, root, project, dry); err != nil {
			return "", err
		} else if msg != "" {
			done = append(done, msg)
		}

		// The ADR directory is wherever the project put it; docs/adr/ is the
		// convention every project that had one used. A project that kept them
		// elsewhere is left alone — guessing at a path here would scatter files.
		src := filepath.Join(dir, "docs", "adr")
		dst := filepath.Join(resolve(dir, kb), "adr")
		if _, err := os.Stat(src); err == nil {
			if occupied, oerr := inTheWay(dst, dry); oerr != nil {
				return "", oerr
			} else if occupied {
				return "", fmt.Errorf("both docs/adr/ and %s/adr/ hold files; merge them by hand, "+
					"then rerun", kb)
			}
			if dry {
				done = append(done, "would move docs/adr/ -> "+kb+"/adr/")
			} else {
				if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
					return "", err
				}
				if err := os.Rename(src, dst); err != nil {
					return "", err
				}
				done = append(done, "moved docs/adr/ -> "+kb+"/adr/")
			}
		}
	}

	// Nothing moved means nothing to retarget: a project with no old layout is
	// either fresh — `init` writes the flag with the new default — or already
	// migrated. Reading AGENTS.md before that check made this migration fail on
	// a project it has no business touching, and `init` runs migrations before
	// it writes anything, so the failure landed on every fresh install whose
	// AGENTS.md could not be read.
	if len(done) == 0 {
		return "", nil
	}

	// The `metrics:` flag is written out with its default by `init`, so most
	// projects carry the old path spelled out in AGENTS.md. Moving the directory
	// under it and leaving the flag alone would point the journal at a directory
	// that no longer exists — and `metrics: none` is a legitimate value that must
	// survive untouched.
	if msg, err := retargetMetricsFlag(dir, dry); err != nil {
		return "", err
	} else if msg != "" {
		done = append(done, msg)
	}

	// `project` did not exist before this layout, so every migrated project is
	// missing it. Writing it here rather than leaving it to the user is what
	// keeps the move and the flag that describes it from disagreeing.
	if msg, err := addProjectFlag(dir, dry); err != nil {
		return "", err
	} else if msg != "" {
		done = append(done, msg)
	}

	return strings.Join(done, "; ") +
		" — replace the .pipeline/ lines in .gitignore with /.pipeline/workdir/", nil
}

// retargetMetricsFlag rewrites `- metrics: .pipeline/metrics/...` to the same
// path under workdir/. Any other value — `none`, or a path the project chose
// itself — is left as it stands.
func retargetMetricsFlag(dir string, dry bool) (string, error) {
	agents := filepath.Join(dir, "AGENTS.md")
	b, err := os.ReadFile(agents)
	if os.IsNotExist(err) {
		return "", nil
	} else if err != nil {
		return "", err
	}
	const old = ".pipeline/metrics/"
	lines := strings.Split(string(b), "\n")
	hit := false
	for i, line := range lines {
		if !strings.HasPrefix(strings.TrimSpace(line), "- metrics:") {
			continue
		}
		if !strings.Contains(line, old) {
			continue
		}
		lines[i] = strings.Replace(line, old, ".pipeline/workdir/metrics/", 1)
		hit = true
	}
	if !hit {
		return "", nil
	}
	if dry {
		return "would retarget the metrics flag in AGENTS.md", nil
	}
	if err := os.WriteFile(agents, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		return "", err
	}
	return "retargeted the metrics flag in AGENTS.md", nil
}

// addProjectColumn brings INDEX.md at the base's root onto the shared-base
// format: a leading `Project` column, and `File` paths relative to the root
// rather than to the project directory. Moving the files without this leaves an
// index whose every row points one directory too high — and kb-sync reads the
// index to find an entry, so the next `knowledge` run would report every module
// as undescribed.
//
// Only a table whose header has no `Project` column is touched, which is what
// makes the migration safe to rerun and safe on a base another project has
// already converted.
func addProjectColumn(dir, root, project string, dry bool) (string, error) {
	index := filepath.Join(resolve(dir, root), "INDEX.md")
	b, err := os.ReadFile(index)
	if os.IsNotExist(err) {
		return "", nil
	} else if err != nil {
		return "", err
	}

	lines := strings.Split(string(b), "\n")
	// fileCol is the index of the File column in the module table, so its cell
	// can be re-rooted. The slice table has no such column; -1 says so.
	converted, fileCol, inTable := 0, -1, false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "|") {
			inTable, fileCol = false, -1
			continue
		}
		cells := splitRow(trimmed)
		if !inTable {
			// A header row. Already converted, or not a table this touches.
			if len(cells) == 0 || strings.EqualFold(cells[0], "Project") {
				continue
			}
			if !strings.EqualFold(cells[0], "Module") && !strings.EqualFold(cells[0], "Slice") {
				continue
			}
			inTable = true
			for j, c := range cells {
				if strings.EqualFold(c, "File") {
					fileCol = j
				}
			}
			lines[i] = "| Project " + line[strings.Index(line, "|"):]
			continue
		}
		if isSeparatorRow(cells) {
			lines[i] = "|---" + line[strings.Index(line, "|"):]
			continue
		}
		if fileCol >= 0 && fileCol < len(cells) {
			v := cells[fileCol]
			if v != "" && !strings.HasPrefix(v, project+"/") {
				cells[fileCol] = project + "/" + v
			}
		}
		lines[i] = "| " + project + " | " + strings.Join(cells, " | ") + " |"
		converted++
	}
	if converted == 0 {
		return "", nil
	}
	if dry {
		return fmt.Sprintf("would add a Project column to %d row(s) of %s/INDEX.md",
			converted, root), nil
	}
	if err := os.WriteFile(index, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		return "", err
	}
	return fmt.Sprintf("added a Project column to %d row(s) of %s/INDEX.md", converted, root), nil
}

// inTheWay reports whether something at dst would make a move a merge. An EMPTY
// directory does not: a half-set-up project — one where a newer `init` created
// the new directories before the migration ran — had one of those at every
// destination, and refusing there sent the user to merge two directories of
// which one held nothing. The empty directory is removed so the rename can
// proceed; anything with contents in it is still a refusal, because merging two
// rule registries is not something to guess at.
func inTheWay(dst string, dry bool) (bool, error) {
	entries, err := os.ReadDir(dst)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		// Not a directory, or unreadable: either way this is not ours to move onto.
		return true, nil
	}
	if len(entries) > 0 {
		return true, nil
	}
	// A dry run reports and writes nothing, removals included; the rename it is
	// predicting would clear the empty directory out of the way itself.
	if dry {
		return false, nil
	}
	if err := os.Remove(dst); err != nil {
		return true, nil
	}
	return false, nil
}

// resolve turns a path from a flag into a filesystem path. The base may sit
// outside the project — a company base every service writes into — and joining
// an absolute path onto the project reproduces it INSIDE the project instead,
// which fabricates a tree nobody asked for and moves the real base nowhere.
func resolve(dir, p string) string {
	if filepath.IsAbs(p) {
		return filepath.FromSlash(p)
	}
	return filepath.Join(dir, filepath.FromSlash(p))
}

// splitRow returns the cells of a markdown table row, without the outer pipes.
func splitRow(line string) []string {
	line = strings.TrimPrefix(strings.TrimSuffix(line, "|"), "|")
	cells := strings.Split(line, "|")
	for i := range cells {
		cells[i] = strings.TrimSpace(cells[i])
	}
	return cells
}

// isSeparatorRow reports the `|---|---|` line under a header.
func isSeparatorRow(cells []string) bool {
	for _, c := range cells {
		if c == "" || strings.Trim(c, "-: ") != "" {
			return false
		}
	}
	return len(cells) > 0
}

// knowledgeFlag reads `- knowledge: <path>` out of the AGENTS.md flags block.
// A file that is missing or carries no such flag yields the default, because
// that is what the pipeline itself falls back to.
func knowledgeFlag(dir string) string {
	if v := readFlag(dir, "knowledge"); v != "" {
		return v
	}
	return ".pipeline/knowledge"
}

// projectFlag is the project's directory inside the base. A project migrating
// onto this layout has no such flag yet, and the fallback is the same one `init`
// uses: the name of the directory the project lives in.
func projectFlag(dir string) string {
	if v := readFlag(dir, "project"); v != "" {
		return v
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = dir
	}
	return install.ProjectSlug(filepath.Base(abs))
}

// readFlag returns one `- <name>: <value>` from the AGENTS.md flags block, or
// "" when the file or the flag is absent.
func readFlag(dir, name string) string {
	b, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		return ""
	}
	for line := range strings.SplitSeq(string(b), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "- "+name+":") {
			continue
		}
		v := strings.TrimSpace(strings.TrimPrefix(line, "- "+name+":"))
		if i := strings.Index(v, "#"); i >= 0 {
			v = strings.TrimSpace(v[:i])
		}
		return v
	}
	return ""
}

// addProjectFlag inserts `- project: <name>` after the knowledge flag. It is a
// no-op where the flag is already there, or where there is no flags block to
// put it in — `init` writes the whole block in that case.
func addProjectFlag(dir string, dry bool) (string, error) {
	if readFlag(dir, "project") != "" {
		return "", nil
	}
	agents := filepath.Join(dir, "AGENTS.md")
	b, err := os.ReadFile(agents)
	if os.IsNotExist(err) {
		return "", nil
	} else if err != nil {
		return "", err
	}
	project := projectFlag(dir)
	lines := strings.Split(string(b), "\n")

	// After `knowledge:` where there is one — the two flags describe one path
	// between them and are read together. Otherwise after the last flag of the
	// block, so it lands inside the block rather than under some later heading.
	at := -1
	inBlock := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			body := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
			if strings.HasPrefix(body, "-") {
				continue
			}
			if strings.EqualFold(body, "Pipeline flags") {
				inBlock, at = true, i
			} else if inBlock {
				break
			}
			continue
		}
		if !inBlock || !strings.HasPrefix(trimmed, "- ") {
			continue
		}
		at = i
		if strings.HasPrefix(trimmed, "- knowledge:") {
			break
		}
	}
	if at < 0 {
		return "", nil
	}
	if dry {
		return "would add `project: " + project + "` to AGENTS.md", nil
	}
	row := "- project: " + project +
		"  # this project's directory inside the knowledge base"
	lines = append(lines[:at+1], append([]string{row}, lines[at+1:]...)...)
	if err := os.WriteFile(agents, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		return "", err
	}
	return "added `project: " + project + "` to AGENTS.md", nil
}
