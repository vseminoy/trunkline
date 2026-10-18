package cli

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// trace-check — the machine half of step 7a: did every rule reach every
// artefact the route produced, as recorded by the step that produced it.
//
// Why this exists. Step 7a is the most valuable gate in the pipeline and,
// without this check, the only one whose failure was silent. A weak model
// does not fail 7a — it fills the matrix with plausible OK statuses that
// nobody grepped, and the gate created to catch "the rule never reached the
// YAML, the test was written against the YAML, the test is green, the
// requirement is not implemented" passes exactly that defect through.
//
// What it decides: check 1 (completeness — every identifier from the step 2
// registry is present in every complete column), the mechanical half of
// check 2 (no orphans — an identifier of this run that appears in an
// artefact but is absent from the registry), and check 6 (nothing lost in
// the handover — every requirement step 2 copied out of the task text has a
// rule, and every rule traces back to one or is marked as raised during
// development). It does not decide checks 3-5 (numbers and boundaries
// matching literally, the error contract, types across layers) — those are
// about meaning, and a grep cannot see them. A green run of this command is
// not a green 7a.
//
// Exit codes: 0 — every identifier reaches every complete column, is
// assigned to a changeset, and there are no orphans; 1 — gaps, orphans or
// unassigned rules (step 8 is blocked); 2 — the command could not run (bad
// arguments, missing paths). Exit 2 is not a pass.

// idRe matches `R-007-3`, whole word: `R-<run>-<n>`, one family, one
// assigner (step 2 of the development pipeline). Matched whole-word, or
// `R-7-1` would be found inside `R-7-10`. The run number also keeps epic
// keys out: `PROJ-855` is a JIRA key, not a rule.
var tcIDRe = regexp.MustCompile(`\bR-\d+-\d+\b`)

// tcWithdrawnRe: a withdrawn rule keeps its number and is expected nowhere.
// Carrying it into the matrix would produce a permanent GAP that teaches
// everyone to ignore GAPs.
var tcWithdrawnRe = regexp.MustCompile(`(?i)\[WITHDRAWN\]`)

// tcLineRe: an identifier counts as declared when it opens a line — a table
// row (`| R-007-3 | ...`) or a bullet (`- R-007-3: ...`). Mentioned
// mid-sentence, it is a reference, not a declaration.
var tcLineRe = regexp.MustCompile(`^(?:\|\s*|[-*+]\s*|\d+\.\s*)?(R-\d+-\d+)\b`)

var tcSourceLineRe = regexp.MustCompile(`^\d+\.\s+\S`)
var tcTaskItemRe = regexp.MustCompile(`(?i)task item\s+(\d+)`)
var tcDevelopmentRe = regexp.MustCompile(`(?i)development`)

// tcSkipNames: step 7a's own output quotes every identifier by construction,
// so counting it as a column would make the check pass against itself.
var tcSkipNames = map[string]bool{"trace-check.out": true}

// tcSkipDirs: vendored copies of the pipeline, or the run's other working
// files — walking into them would answer the grep for the wrong reason. A
// column that names a file inside one is still read.
var tcSkipDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true,
	".pipeline": true, ".trunkline": true, "__pycache__": true,
}

var tcTextSuffixes = map[string]bool{
	".md": true, ".yaml": true, ".yml": true, ".json": true, ".sql": true,
	".go": true, ".py": true, ".ts": true, ".tsx": true, ".js": true,
	".java": true, ".kt": true, ".rs": true, ".rb": true, ".php": true,
	".cs": true, ".txt": true, ".proto": true, ".graphql": true, ".tf": true,
	".sh": true,
}

func tcDie(env Env, format string, a ...any) error {
	fmt.Fprintf(env.Err, "trace-check: "+format+"\n", a...)
	return exitCode(2)
}

func tcCellsOf(row string) []string {
	if !strings.HasPrefix(row, "|") {
		return nil
	}
	trimmed := strings.Trim(row, "|")
	parts := strings.Split(trimmed, "|")
	out := make([]string, len(parts))
	for i, p := range parts {
		out[i] = strings.TrimSpace(p)
	}
	return out
}

// tcRunOf: the run number an identifier belongs to: `R-007-3` -> 7.
func tcRunOf(rid string) int {
	parts := strings.Split(rid, "-")
	n, _ := strconv.Atoi(parts[1])
	return n
}

// tcThisRun decides which run a registry belongs to, or nil if it cannot be
// told. The canonical location is `.pipeline/workdir/rules/<NNN>.md`, so the
// file name answers it even for an empty registry; the identifiers
// themselves answer it otherwise.
func tcThisRun(path string, known []string) *int {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if n, err := strconv.Atoi(base); err == nil {
		return &n
	}
	if len(known) > 0 {
		n := tcRunOf(known[0])
		return &n
	}
	return nil
}

type tcRegistryResult struct {
	ids, known, withdrawn, unassigned, sources []string
	origins                                    map[string]string
}

// tcRegistry reads the step 2 rule registry: identifiers in declaration
// order, narrowed to one changeset of the step 4 plan when changeset != "".
func tcRegistry(env Env, path, changeset string) (tcRegistryResult, error) {
	var res tcRegistryResult
	res.origins = map[string]string{}

	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return res, tcDie(env, "--rules %s: not a file. Step 2 writes the rule registry to "+
			".pipeline/workdir/rules/<run>.md, once per task (see "+
			"core/developer/steps/02-analytic.md); without it there is nothing "+
			"to check the artefacts against", path)
	}
	f, err := os.Open(path)
	if err != nil {
		return res, tcDie(env, "--rules %s: %v", path, err)
	}
	defer f.Close()

	ids := []string{}
	known := []string{}
	knownSet := map[string]bool{}
	withdrawn := []string{}
	unassigned := []string{}
	sources := []string{}
	origins := map[string]string{}
	inSources := false
	colIdx, originIdx := -1, -1
	idsSet := map[string]bool{}

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	for sc.Scan() {
		line := sc.Text()
		stripped := strings.TrimSpace(line)

		if strings.HasPrefix(strings.ToLower(stripped), "## ") {
			inSources = strings.Contains(strings.ToLower(stripped), "requirements from the task")
		}
		if inSources && tcSourceLineRe.MatchString(stripped) {
			sources = append(sources, stripped)
			continue
		}

		header := tcCellsOf(stripped)
		if colIdx == -1 {
			for i, c := range header {
				if strings.ToLower(c) == "changeset" {
					colIdx = i
					break
				}
			}
		}
		if originIdx == -1 {
			for i, c := range header {
				if strings.ToLower(c) == "origin" {
					originIdx = i
					break
				}
			}
		}

		m := tcLineRe.FindStringSubmatch(stripped)
		if m == nil {
			continue
		}
		rid := m[1]
		if tcWithdrawnRe.MatchString(line) {
			withdrawn = append(withdrawn, rid)
			continue
		}
		if !knownSet[rid] {
			knownSet[rid] = true
			known = append(known, rid)
		}
		if originIdx != -1 {
			cells := tcCellsOf(stripped)
			if originIdx < len(cells) {
				origins[rid] = cells[originIdx]
			}
		}
		if changeset != "" {
			if colIdx == -1 {
				return res, tcDie(env, "--changeset %s: %s has no `Changeset` column. "+
					"Step 4 fills it when it assigns rules to changesets (see "+
					"core/developer/steps/04-decomposer.md); without it there is "+
					"no way to tell this changeset's rules from a later one's", changeset, path)
			}
			cells := tcCellsOf(stripped)
			value := ""
			if colIdx < len(cells) {
				value = cells[colIdx]
			}
			if value == "" || value == "—" || value == "-" {
				unassigned = append(unassigned, rid)
				continue
			}
			if value != changeset {
				continue
			}
		}
		if !idsSet[rid] {
			idsSet[rid] = true
			ids = append(ids, rid)
		}
	}
	if err := sc.Err(); err != nil {
		return res, tcDie(env, "cannot read %s: %v", path, err)
	}
	res.ids, res.known, res.withdrawn, res.unassigned = ids, known, withdrawn, unassigned
	res.sources, res.origins = sources, origins
	return res, nil
}

type tcFileEntry struct {
	path  string
	named bool
}

// tcFilesOf expands one column's comma-separated list of files and
// directories into (path, named) pairs. named marks a path the column
// spelled out, as opposed to one a directory walk happened to reach.
func tcFilesOf(env Env, spec string) ([]tcFileEntry, error) {
	var out []tcFileEntry
	for _, raw := range strings.Split(spec, ",") {
		p := strings.TrimSpace(raw)
		if p == "" {
			continue
		}
		info, err := os.Stat(p)
		switch {
		case err == nil && !info.IsDir():
			out = append(out, tcFileEntry{p, true})
		case err == nil && info.IsDir():
			var dirs []string
			dirFiles := map[string][]string{}
			err := filepath.WalkDir(p, func(path string, d os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() {
					if path != p && tcSkipDirs[d.Name()] {
						return filepath.SkipDir
					}
					dirs = append(dirs, path)
					return nil
				}
				dirFiles[filepath.Dir(path)] = append(dirFiles[filepath.Dir(path)], d.Name())
				return nil
			})
			if err != nil {
				return nil, tcDie(env, "cannot walk %s: %v", p, err)
			}
			sort.Strings(dirs)
			for _, base := range dirs {
				names := dirFiles[base]
				sort.Strings(names)
				for _, n := range names {
					if tcSkipNames[n] || !tcTextSuffixes[filepath.Ext(n)] {
						continue
					}
					out = append(out, tcFileEntry{filepath.Join(base, n), false})
				}
			}
		default:
			return nil, tcDie(env, "path does not exist: %s", p)
		}
	}
	var kept []tcFileEntry
	for _, e := range out {
		if !tcSkipNames[filepath.Base(e.path)] {
			kept = append(kept, e)
		}
	}
	if len(kept) == 0 {
		return nil, tcDie(env, "column matched no readable artefact: %s", spec)
	}
	return kept, nil
}

type tcColumn struct {
	name  string
	files []string
}

type tcOwner struct {
	name  string
	named bool
}

// tcAssignColumns gives every file to exactly one column: an artefact
// answers for itself. A file a column names beats one it merely reached by
// walking a directory; two columns covering a file the same way is a
// question with no answer.
func tcAssignColumns(env Env, columns []struct {
	name    string
	entries []tcFileEntry
}) ([]tcColumn, error) {
	owner := map[string]tcOwner{}
	for _, col := range columns {
		for _, e := range col.entries {
			key := realpath(e.path)
			prev, ok := owner[key]
			if !ok {
				owner[key] = tcOwner{col.name, e.named}
				continue
			}
			if prev.name == col.name || (prev.named && !e.named) {
				continue
			}
			if e.named && !prev.named {
				owner[key] = tcOwner{col.name, e.named}
				continue
			}
			return nil, tcDie(env, "%s is covered by columns `%s` and `%s` in the same "+
				"way, so neither can be said to own it. Name the file in the column it "+
				"belongs to and keep it out of the other's paths", e.path, prev.name, col.name)
		}
	}
	var out []tcColumn
	for _, col := range columns {
		var kept []string
		for _, e := range col.entries {
			if owner[realpath(e.path)].name == col.name {
				kept = append(kept, e.path)
			}
		}
		if len(kept) == 0 {
			return nil, tcDie(env, "column `%s` has no files of its own: everything its paths cover "+
				"belongs to another column. It would report every rule as a gap", col.name)
		}
		out = append(out, tcColumn{col.name, kept})
	}
	return out, nil
}

// tcScan returns, for one column, every identifier found and the files it
// appears in (deduplicated per file).
func tcScan(env Env, paths []string) (map[string][]string, error) {
	hits := map[string][]string{}
	for _, p := range paths {
		text, err := readTextLenient(p)
		if err != nil {
			return nil, tcDie(env, "cannot read %s: %v", p, err)
		}
		seen := map[string]bool{}
		for _, rid := range tcIDRe.FindAllString(text, -1) {
			if !seen[rid] {
				seen[rid] = true
				hits[rid] = append(hits[rid], p)
			}
		}
	}
	return hits, nil
}

func runTraceCheck(env Env, args []string) error {
	fs := flag.NewFlagSet("trace-check", flag.ContinueOnError)
	fs.SetOutput(env.Err)
	var rulesPath, changeset, format string
	var columnArgs, partialArgs, naArgs repeatedFlag
	fs.StringVar(&rulesPath, "rules", "", "the step 2 rule registry (rules.md next to the module)")
	fs.Var(&columnArgs, "column", "a column every rule must reach: the contract and tests coverage records")
	fs.Var(&partialArgs, "partial", "a column only some rules reach — the schema, the diagram. "+
		"Scanned for orphans and kept in the matrix for checks 3-6; absence is not a gap")
	fs.Var(&naArgs, "na", "cells the rule does not need; justify each in consistency.md")
	fs.StringVar(&changeset, "changeset", "", "narrow the registry to one changeset of the step 4 plan "+
		"(reads the `Changeset` column); omit where the route has no step 4")
	fs.StringVar(&format, "format", "md", "md or text")
	fs.Usage = func() {
		fmt.Fprint(env.Err, "Usage: trunkline trace-check --rules FILE --column NAME=PATHS "+
			"--column NAME=PATHS [flags]\n\n"+
			"The machine half of step 7a: did every rule reach every artefact the\n"+
			"route produced. See core/developer/steps/07a-consistency.md.\n")
	}
	if err := fs.Parse(args); err != nil {
		return exitCode(2)
	}
	if rulesPath == "" {
		return tcDie(env, "--rules is required")
	}
	if format != "md" && format != "text" {
		return tcDie(env, "--format must be md or text")
	}

	type rawColumn struct {
		name    string
		entries []tcFileEntry
	}
	var raw []rawColumn
	partial := map[string]bool{}
	for _, spec := range columnArgs {
		name, paths, ok := strings.Cut(spec, "=")
		if !ok {
			return tcDie(env, "--column expects NAME=PATHS, got %q", spec)
		}
		entries, err := tcFilesOf(env, paths)
		if err != nil {
			return err
		}
		raw = append(raw, rawColumn{strings.TrimSpace(name), entries})
	}
	for _, spec := range partialArgs {
		name, paths, ok := strings.Cut(spec, "=")
		if !ok {
			return tcDie(env, "--partial expects NAME=PATHS, got %q", spec)
		}
		entries, err := tcFilesOf(env, paths)
		if err != nil {
			return err
		}
		name = strings.TrimSpace(name)
		raw = append(raw, rawColumn{name, entries})
		partial[name] = true
	}
	liveCount := 0
	for _, c := range raw {
		if !partial[c.name] {
			liveCount++
		}
	}
	if liveCount == 0 {
		return tcDie(env, "at least one --column is required: with only partial columns nothing is "+
			"checked for completeness, and the run would go green without asking the "+
			"question this command exists to ask")
	}
	if len(raw) < 2 {
		return tcDie(env, "at least two columns are required — with one live column there is "+
			"nothing to cross-check (see core/developer/steps/07a-consistency.md)")
	}
	rawForAssign := make([]struct {
		name    string
		entries []tcFileEntry
	}, len(raw))
	for i, c := range raw {
		rawForAssign[i] = struct {
			name    string
			entries []tcFileEntry
		}{c.name, c.entries}
	}
	columns, err := tcAssignColumns(env, rawForAssign)
	if err != nil {
		return err
	}
	known := map[string]bool{}
	for _, c := range columns {
		known[c.name] = true
	}

	type exemptKey struct{ rid, name string }
	exempt := map[exemptKey]bool{}
	for _, spec := range naArgs {
		rid, names, ok := strings.Cut(spec, "=")
		if !ok {
			return tcDie(env, "--na expects ID=COLUMNS, got %q", spec)
		}
		for _, name := range strings.Split(names, ",") {
			name = strings.TrimSpace(name)
			if !known[name] {
				var sortedNames []string
				for n := range known {
					sortedNames = append(sortedNames, n)
				}
				sort.Strings(sortedNames)
				return tcDie(env, "--na %s: no column named %q; columns are %s",
					spec, name, strings.Join(sortedNames, ", "))
			}
			exempt[exemptKey{strings.TrimSpace(rid), name}] = true
		}
	}

	reg, err := tcRegistry(env, rulesPath, changeset)
	if err != nil {
		return err
	}
	idsSet := map[string]bool{}
	for _, id := range reg.ids {
		idsSet[id] = true
	}
	var sortedExempt []exemptKey
	for k := range exempt {
		sortedExempt = append(sortedExempt, k)
	}
	sort.Slice(sortedExempt, func(i, j int) bool {
		if sortedExempt[i].rid != sortedExempt[j].rid {
			return sortedExempt[i].rid < sortedExempt[j].rid
		}
		return sortedExempt[i].name < sortedExempt[j].name
	})
	for _, k := range sortedExempt {
		if !idsSet[k.rid] {
			return tcDie(env, "--na %s=%s: %s is not in the registry %s", k.rid, k.name, k.rid, rulesPath)
		}
	}

	scanned := map[string]map[string][]string{}
	for _, c := range columns {
		hits, err := tcScan(env, c.files)
		if err != nil {
			return err
		}
		scanned[c.name] = hits
	}

	knownAndWithdrawn := append(append([]string{}, reg.known...), reg.withdrawn...)
	run := tcThisRun(rulesPath, knownAndWithdrawn)

	type gapEntry struct{ rid, name string }
	type orphanEntry struct {
		rid, name string
		where     []string
	}
	var gaps []gapEntry
	var orphans []orphanEntry
	carried := map[string]bool{}
	knownSet := map[string]bool{}
	for _, k := range reg.known {
		knownSet[k] = true
	}
	withdrawnSet := map[string]bool{}
	for _, w := range reg.withdrawn {
		withdrawnSet[w] = true
	}

	for _, c := range columns {
		hits := scanned[c.name]
		for _, rid := range reg.ids {
			if _, ok := hits[rid]; !ok && !exempt[exemptKey{rid, c.name}] && !partial[c.name] {
				gaps = append(gaps, gapEntry{rid, c.name})
			}
		}
		var hitIDs []string
		for rid := range hits {
			hitIDs = append(hitIDs, rid)
		}
		sort.Strings(hitIDs)
		for _, rid := range hitIDs {
			if knownSet[rid] || withdrawnSet[rid] {
				continue
			}
			if run != nil && tcRunOf(rid) < *run {
				carried[rid] = true
				continue
			}
			orphans = append(orphans, orphanEntry{rid, c.name, hits[rid]})
		}
	}

	head := append([]string{"ID"}, func() []string {
		var names []string
		for _, c := range columns {
			names = append(names, c.name)
		}
		return names
	}()...)
	var rows [][]string
	for _, rid := range reg.ids {
		row := []string{rid}
		for _, c := range columns {
			switch {
			case scanned[c.name][rid] != nil:
				row = append(row, "found")
			case exempt[exemptKey{rid, c.name}]:
				row = append(row, "N/A (declared)")
			case partial[c.name]:
				row = append(row, "—")
			default:
				row = append(row, "**MISSING**")
			}
		}
		rows = append(rows, row)
	}

	// Check 6: nothing lost between the task text and the registry.
	var lost []int
	var dangling []string
	if len(reg.sources) > 0 {
		claimed := map[int]bool{}
		for _, rid := range reg.known {
			origin := reg.origins[rid]
			if m := tcTaskItemRe.FindStringSubmatch(origin); m != nil {
				n, _ := strconv.Atoi(m[1])
				claimed[n] = true
			} else if !tcDevelopmentRe.MatchString(origin) {
				dangling = append(dangling, rid)
			}
		}
		for n := 1; n <= len(reg.sources); n++ {
			if !claimed[n] {
				lost = append(lost, n)
			}
		}
		var sortedClaimed []int
		for n := range claimed {
			sortedClaimed = append(sortedClaimed, n)
		}
		sort.Ints(sortedClaimed)
		for _, n := range sortedClaimed {
			if n > len(reg.sources) {
				dangling = append(dangling, fmt.Sprintf("origin `task item %d` (only %d recorded)", n, len(reg.sources)))
			}
		}
	}

	out := env.Out
	if len(reg.ids) == 0 {
		if len(reg.known) == 0 {
			fmt.Fprint(out, "NOTE     the registry declares no rules. Legitimate where step 2 raised none — a\n"+
				"         feature whose behaviour follows entirely from the profile guidelines has\n"+
				"         no domain rule of its own. An identifier of this run found in a coverage\n"+
				"         record below is then an orphan, which is how a wrong --rules path shows up.\n")
		} else {
			fmt.Fprintf(out, "NOTE     no rule is assigned to changeset %s. The registry has "+
				"%d, all\n         belonging to other changesets of this task.\n", changeset, len(reg.known))
		}
	}

	if format == "md" {
		fmt.Fprintln(out, "| "+strings.Join(head, " | ")+" |")
		seps := make([]string, len(head))
		for i := range seps {
			seps[i] = "---"
		}
		fmt.Fprintln(out, "|"+strings.Join(seps, "|")+"|")
		for _, row := range rows {
			fmt.Fprintln(out, "| "+strings.Join(row, " | ")+" |")
		}
	} else {
		width := len(head[0])
		for _, h := range head {
			if len(h) > width {
				width = len(h)
			}
		}
		for _, r := range rows {
			if len(r[0]) > width {
				width = len(r[0])
			}
		}
		for _, row := range rows {
			var parts []string
			for i := 1; i < len(row); i++ {
				parts = append(parts, fmt.Sprintf("%s=%s", columns[i-1].name, row[i]))
			}
			fmt.Fprintf(out, "%-*s  %s\n", width, row[0], strings.Join(parts, "  "))
		}
	}

	fmt.Fprintln(out)
	for _, g := range gaps {
		fmt.Fprintf(out, "GAP      %s does not appear in column `%s`\n", g.rid, g.name)
	}
	for _, o := range orphans {
		shown := o.where
		if len(shown) > 3 {
			shown = shown[:3]
		}
		fmt.Fprintf(out, "ORPHAN   %s appears in `%s` (%s) but is not in the step 2 registry\n",
			o.rid, o.name, strings.Join(shown, ", "))
	}
	for _, k := range sortedExempt {
		fmt.Fprintf(out, "N/A      %s declared not applicable to column `%s` — justify in consistency.md\n", k.rid, k.name)
	}
	for _, n := range lost {
		src := ""
		if n-1 < len(reg.sources) {
			src = reg.sources[n-1]
			if len(src) > 96 {
				src = src[:96]
			}
		}
		fmt.Fprintf(out, "LOST     task requirement %d became no rule — step 2 read it and did not "+
			"register it:\n         %s\n", n, src)
	}
	for _, d := range dangling {
		fmt.Fprintf(out, "ORIGIN   %s traces to no recorded requirement and is not marked `development`\n", d)
	}
	for _, rid := range reg.unassigned {
		fmt.Fprintf(out, "NOCHANGESET  %s is in the registry but assigned to no changeset "+
			"— step 4 either lost it or must mark it deferred\n", rid)
	}
	if len(reg.withdrawn) > 0 {
		wset := map[string]bool{}
		for _, w := range reg.withdrawn {
			wset[w] = true
		}
		var ws []string
		for w := range wset {
			ws = append(ws, w)
		}
		sort.Strings(ws)
		fmt.Fprintf(out, "NOTE     withdrawn, expected nowhere: %s\n", strings.Join(ws, ", "))
	}
	if len(carried) > 0 {
		var shown []string
		for c := range carried {
			shown = append(shown, c)
		}
		sort.Strings(shown)
		tail := ""
		display := shown
		if len(shown) > 8 {
			display = shown[:8]
			tail = ", …"
		}
		fmt.Fprintf(out, "NOTE     %d identifier(s) from earlier runs in these records, not "+
			"orphans:\n         %s%s\n", len(shown), strings.Join(display, ", "), tail)
	}

	var scopeParts []string
	for _, c := range columns {
		if partial[c.name] {
			scopeParts = append(scopeParts, c.name+"*")
		} else {
			scopeParts = append(scopeParts, c.name)
		}
	}
	where := ""
	if changeset != "" {
		where = fmt.Sprintf(", changeset %s", changeset)
	}
	failed := len(gaps) > 0 || len(orphans) > 0 || len(reg.unassigned) > 0 || len(lost) > 0 || len(dangling) > 0
	star := ""
	if len(partial) > 0 {
		star = "; * = partial, absence is not a gap"
	}
	verdict := "PASSED"
	if failed {
		verdict = "FAILED"
	}
	fmt.Fprintf(out, "\nTRACE-CHECK: %d identifier(s) x %d column(s) (%s%s) — %d gap(s), %d orphan(s), "+
		"%d unassigned, %d lost, %d declared N/A%s — %s\n",
		len(reg.ids), len(columns), strings.Join(scopeParts, ", "), where,
		len(gaps), len(orphans), len(reg.unassigned), len(lost), len(exempt), star, verdict)
	fmt.Fprint(out, "Mechanical only: identifiers are wired up and nothing was dropped between "+
		"the task text and the registry. Checks 3-5 of step 7a (numbers and "+
		"boundaries, the error contract, types across layers) are about meaning and "+
		"are still yours.\n")

	if failed {
		return exitCode(1)
	}
	return nil
}
