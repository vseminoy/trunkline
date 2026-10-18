package cli

import (
	"bytes"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	trunkline "github.com/vseminoy/trunkline"
	"github.com/vseminoy/trunkline/internal/install"
	"github.com/vseminoy/trunkline/internal/migrate"
	"github.com/vseminoy/trunkline/internal/profile"
	"github.com/vseminoy/trunkline/internal/semver"
	"github.com/vseminoy/trunkline/internal/state"
)

// A check is one thing that can be wrong with a setup. Severity matters: a fail
// means the pipeline will stop or silently skip work, a warn means it will run
// with something disabled.
type checkResult struct {
	name   string
	level  string // "ok", "warn", "fail"
	detail string
}

func runDoctor(env Env, args []string) error {
	fs_ := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs_.SetOutput(env.Err)
	fs_.Usage = func() {
		fmt.Fprint(env.Err, "Usage: trunkline [-C dir] doctor\n\n"+
			"Checks the setup: flags, profile, required files, gates, knowledge\n"+
			"base, version drift. Exits non-zero if anything would break a run.\n")
	}
	if err := fs_.Parse(args); err != nil {
		return err
	}

	st, err := state.Load(env.Dir)
	if err != nil {
		return err
	}
	results := diagnose(env.Dir, st)

	failed := 0
	for _, r := range results {
		mark := map[string]string{"ok": "ok  ", "warn": "warn", "fail": "FAIL"}[r.level]
		fmt.Fprintf(env.Out, "[%s] %-28s %s\n", mark, r.name, r.detail)
		if r.level == "fail" {
			failed++
		}
	}
	if failed > 0 {
		return fmt.Errorf("%d check(s) failed", failed)
	}
	fmt.Fprint(env.Out, "\nSetup is complete.\n")
	return nil
}

func diagnose(dir string, st *state.State) []checkResult {
	var out []checkResult
	add := func(name, level, format string, a ...any) {
		out = append(out, checkResult{name, level, fmt.Sprintf(format, a...)})
	}

	if !state.Exists(dir) {
		add("installation", "fail", "no %s — run `trunkline init`", state.File)
		return out
	}
	add("installation", "ok", "core %s, profiles: %s", st.Core, joinProfiles(st.Profiles))

	switch {
	case st.Core == trunkline.Version():
		add("version", "ok", "up to date with this binary (%s)", trunkline.Version())
	case semver.Less(trunkline.Version(), st.Core):
		// `update` would apply this binary's older core. That is a legitimate
		// thing to want and runUpdate says "downgrading" when asked — but it is
		// not what a project in this state usually needs, so it is not advised.
		add("version", "warn", "installed core %s is newer than this binary (%s) — "+
			"update this binary; `trunkline update` here would downgrade the project",
			st.Core, trunkline.Version())
	default:
		add("version", "warn", "installed %s, binary ships %s — run `trunkline update`",
			st.Core, trunkline.Version())
	}

	// Every migration shipped so far targets a pre-1.0 layout, so a project
	// installed at 0.1.0 or later never sees one. They surface here only for a
	// project predating the state file, where applying them is a no-op that
	// records the ids and clears this line for good.
	if n := len(migrate.Pending(st, st.Core, trunkline.Version())); n > 0 {
		add("migrations", "warn", "%d pending (pre-1.0 layout) — `trunkline update` applies "+
			"and records them; a no-op unless this project predates %s", n, install.Root)
	}

	flags, err := readFlags(dir)
	if err != nil {
		// Not a soft warning: every flag now reads as unset, and an unset flag
		// has a default. The run would proceed on defaults nobody chose.
		add("flags", "fail", "cannot read %s: %v — every flag reads as unset and the "+
			"pipeline runs on defaults nobody chose", install.FlagsFile, err)
		flags = map[string]string{}
	}
	name := flags["profile"]
	switch {
	case err != nil:
	case name == "":
		add("flags", "fail", "no `profile:` in the `## Pipeline flags` block of %s — "+
			"the agent will stop and ask on every run", install.FlagsFile)
	default:
		add("flags", "ok", "profile: %s", name)
	}

	// The language is checked against what is actually installed, not against
	// what the binary could install: a project that declares `ru` but carries an
	// English core would run in English while the flags say otherwise.
	switch lang := flags["language"]; {
	case lang == "" || lang == install.DefaultLanguage:
		add("language", "ok", "%s (the core's own language)", install.DefaultLanguage)
	default:
		if _, err := fs.Stat(trunkline.Core, path.Join(install.L10nDir, lang)); err != nil {
			add("language", "fail", "%s is declared but this binary ships no such "+
				"localization — the core you are running is English", lang)
		} else if drift, err := l10nDrift(lang); err == nil && len(drift) > 0 {
			// The lock file travels in the binary, so this is answerable here and
			// nowhere else in a project. Without it a stale translation ships
			// prompts describing steps the English core no longer has, silently.
			add("language", "warn", "%s is applied, but %d file(s) are behind the "+
				"English core, starting with %s — those steps read as an older version",
				lang, len(drift), drift[0])
		} else {
			add("language", "ok", "%s (localization applied over the English core)", lang)
		}
	}

	var m profile.Manifest
	if name != "" {
		dirRel := path.Join(install.ProfilesDir, name)
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(dirRel), "profile.yaml")); err != nil {
			add("profile", "fail", "%s declared in %s is not installed — "+
				"run `trunkline profile add %s`", name, install.FlagsFile, name)
		} else {
			var err error
			m, err = profile.Load(os.DirFS(dir), dirRel)
			if err != nil {
				add("profile", "fail", "%s", err)
			} else {
				add("profile", "ok", "%s %s (%s)", m.Name, m.Version, m.Language)
				out = append(out, checkRequiredFiles(dir, m)...)
				out = append(out, checkGuidelines(dir, m)...)
				out = append(out, checkAnalytic(dir, m)...)
			}
		}
	}

	// Knowledge base: an unset flag is fine (there is a default), `none` is a
	// legitimate choice, but a path that does not exist is a silent downgrade
	// to "no knowledge base" that looks exactly like a working run.
	knowledge := flags["knowledge"]
	// The base may hold several projects, so what this repository writes to is
	// `<knowledge>/<project>`. An unset `project` is not a default worth
	// guessing at here: two services that both fell back to the directory name
	// would look fine and collide only once both had written, so doctor asks
	// for it explicitly.
	project := flags["project"]
	switch {
	case knowledge == "none":
		add("knowledge base", "warn", "none — step 1 rebuilds AS-IS from scratch every run")
	case project == "":
		add("knowledge base", "warn",
			"no `project` flag — the base holds one directory per project; add "+
				"`project: <name>` to %s, or steps 15 and 20 have to ask every run",
			install.FlagsFile)
	default:
		root := knowledge
		if root == "" {
			root = install.StateDir + "/knowledge"
		}
		kb := path.Join(root, project)
		if _, err := os.Stat(resolve(dir, kb)); err != nil {
			// Step 20 bootstraps the directory on the first run that writes
			// code, so its absence is normally fine. It is worth a warning in
			// exactly one case: this project HAS completed runs, yet has no
			// directory in a base that already holds other projects. Then a
			// misspelled `project` is the likely reason, and nothing else would
			// say so — step 20 would quietly start a second base beside the
			// real one.
			//
			// Both halves of that condition earn their keep. Warning on an
			// empty root fires on every fresh install of a shared base; warning
			// on a project that has never run fires on every correctly
			// configured new project. Either way the warning stops being read,
			// which costs more than the case it was meant to catch.
			siblings := projectDirs(resolve(dir, root), project)
			if len(siblings) > 0 && hasClosedRuns(dir, flags["metrics"]) {
				add("knowledge base", "warn",
					"%s does not exist although this project has completed runs, and the base "+
						"already holds %s — `project` is probably misspelled, and step 20 will "+
						"start a second directory beside the real one",
					kb, strings.Join(siblings, ", "))
			} else {
				add("knowledge base", "ok", "%s — step 20 creates it on the first run", kb)
			}
		} else {
			add("knowledge base", "ok", "%s", kb)
		}
	}

	kbRoot := knowledge
	if kbRoot == "" {
		kbRoot = install.StateDir + "/knowledge"
	}
	switch {
	case knowledge == "none":
		// Nothing to hide and nothing to commit; the warning above already said so.
	case !install.Internal(kbRoot):
		add("knowledge in git", "ok",
			"%s is outside this repository — commit it where it lives", kbRoot)
	case install.PipelineIgnored(dir, kbRoot):
		add("knowledge in git", "fail", "%s is in .gitignore — the knowledge base must be "+
			"committed, or every teammate pays full context price on every task", kbRoot)
	default:
		add("knowledge in git", "ok", "%s is committed", kbRoot)
	}

	// The inverse risk. Run state committed is not broken, only expensive: the
	// journal is append-only, so two branches that both ran the pipeline conflict
	// on every merge. Warn rather than fail — a team may commit it deliberately.
	switch missing := install.RunStateIgnored(dir); {
	case len(missing) == 0:
		add("run state in git", "ok", "%s/ is ignored", install.WorkDir)
	default:
		add("run state in git", "warn", "%s/ not in .gitignore — run state, superseded when the "+
			"run ends; the journal is append-only and conflicts on every merge",
			install.WorkDir)
	}

	kind, wired := install.GateRegistered(dir)
	broken := ""
	if wired {
		broken = install.GateBroken(dir, kind)
	}
	switch {
	case m.Hooks.PreCommitGate == "":
		add("commit gate", "warn", "the profile ships none — enforcement is text only")
	case !wired:
		add("commit gate", "warn", "script present but wired to no trigger — commits pass on a "+
			"broken build; rerun `trunkline init --gate=claude|git`")
	case broken != "":
		// Wired but inert. Every way this happens is silent by construction, so
		// it is a fail rather than a warning: the run looks gated and is not.
		add("commit gate", "fail", "%s; every commit passes on a broken build — "+
			"rerun `trunkline init --gate=%s`", broken, kind)
	case kind == install.GateGit:
		add("commit gate", "ok", "git hook, pre-commit")
	default:
		add("commit gate", "ok", "registered in %s", install.ClaudeSetting)
	}

	if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(install.ClaudeCommand))); err != nil {
		add("entry point", "warn", "%s missing — the neutral entry %s/AGENTS.md still works",
			install.ClaudeCommand, install.Root)
	} else {
		add("entry point", "ok", "/trunkline-dev and %s/AGENTS.md", install.Root)
	}
	if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(install.ClaudeQuick))); err != nil {
		add("short-change route", "warn",
			"%s missing — short changes have to go through the full pipeline; "+
				"rerun `trunkline init`", install.ClaudeQuick)
	} else {
		add("short-change route", "ok", "/trunkline-quick")
	}

	// The entry points name the profile they were generated for; the flags name
	// the profile the agent will actually use. `init --profile B` over a project
	// declaring A writes the first and leaves the second, and both profiles being
	// installed makes the check above pass — so the split is only visible here.
	if name != "" {
		if got, files := entryPointProfiles(dir, name); len(files) > 0 {
			add("entry point profile", "fail", "%s name profile %s while %s declares %s — "+
				"the agent follows the flags; rerun `trunkline init --profile %s` or fix the flag",
				strings.Join(files, ", "), got, install.FlagsFile, name, name)
		}
	}

	if modified := locallyModified(dir, st); len(modified) > 0 {
		add("local edits", "warn", "%d vendored file(s) edited in place, e.g. %s — "+
			"update will keep them and they will drift", len(modified), modified[0])
	}

	return out
}

func checkRequiredFiles(dir string, m profile.Manifest) []checkResult {
	var missing []string
	for _, r := range m.RequiredProjectFiles {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(r.Path))); err != nil {
			missing = append(missing, r.Path)
		}
	}
	if len(missing) > 0 {
		return []checkResult{{"required files", "fail",
			fmt.Sprintf("missing: %s — the pipeline stops at start", strings.Join(missing, ", "))}}
	}
	out := []checkResult{{"required files", "ok",
		fmt.Sprintf("%d present", len(m.RequiredProjectFiles))}}

	// AGENTS.md is required as the project's code standards, and `init` creates
	// it to hold the pipeline pointer. Presence alone would therefore always be
	// green while saying nothing, so the content is what gets checked.
	if pointerOnlyAgents(dir) {
		out = append(out, checkResult{"code standards", "warn",
			fmt.Sprintf("%s holds only the trunkline pointer — the profile requires it as the "+
				"project's code standards, and there are none in it yet", install.AgentsFile)})
	}
	return out
}

// pointerOnlyAgents reports whether AGENTS.md is what `init` left behind and
// nothing more. Both of the things `init` puts there — the pointer and the flags
// block — are dropped first: they are the pipeline's own configuration, not the
// project's code standards, so a file holding only them is still empty of what
// the profile requires.
func pointerOnlyAgents(dir string) bool {
	b, err := os.ReadFile(filepath.Join(dir, install.AgentsFile))
	if err != nil {
		return false
	}
	rest := bytes.ReplaceAll(b, []byte(install.AgentsPointer), nil)
	rest = bytes.TrimSpace(dropFlagsBlock(rest))
	return len(rest) <= len(install.AgentsHeader)
}

// dropFlagsBlock removes the `## Pipeline flags` block — its heading and
// everything up to the next heading. The block's own `# - flag: value` lines are
// commented-out flags rather than headings, so they must not end it.
func dropFlagsBlock(b []byte) []byte {
	var out [][]byte
	inBlock := false
	for _, line := range bytes.Split(b, []byte("\n")) {
		trimmed := strings.TrimSpace(string(line))
		if strings.HasPrefix(trimmed, "#") {
			body := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
			if strings.HasPrefix(body, "-") && inBlock {
				continue
			}
			inBlock = strings.EqualFold(body, "Pipeline flags")
		}
		if !inBlock {
			out = append(out, line)
		}
	}
	return bytes.Join(out, []byte("\n"))
}

func checkGuidelines(dir string, m profile.Manifest) []checkResult {
	if len(m.Guidelines.Roles) == 0 {
		return []checkResult{{"guidelines", "warn",
			"the profile declares no roles — step 7a cross-checks report 'not applicable' across the board"}}
	}
	// The manifest resolves its own paths; doctor must not rebuild them from a
	// directory name it assumes. A profile is free to keep its guidelines
	// wherever guidelines.root says, and checking somewhere else would report a
	// setup as broken that the agent reads perfectly well — or the reverse.
	var missing []string
	for role := range m.Guidelines.Roles {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(m.RolePath(role)))); err != nil {
			missing = append(missing, role)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		return []checkResult{{"guidelines", "fail",
			fmt.Sprintf("roles with no file: %s", strings.Join(missing, ", "))}}
	}
	return []checkResult{{"guidelines", "ok", fmt.Sprintf("%d roles resolved", len(m.Guidelines.Roles))}}
}

// pipelinePath resolves a path the pipeline text gives from its own root — the
// directory holding core/, profiles/, publishers/ — into a project-relative one.
func pipelinePath(rel string) string {
	if rel == "" {
		return ""
	}
	return path.Join(install.Root, rel)
}

// checkAnalytic reports on the analysis track. It exists because the track used
// to ship into projects while pointing at files that only the trunkline
// repository had: the route looked installed and stopped at its first step.
// tasksDirState returns why tasks.dir cannot serve, or "" when it can. Empty
// counts as absent for a path outside the project: an uninitialised submodule
// and a checkout nobody cloned look exactly alike, and both mean no analysis.
func tasksDirState(dir, tasks string) string {
	if tasks == "" {
		return ""
	}
	p := filepath.Join(dir, filepath.FromSlash(tasks))
	entries, err := os.ReadDir(p)
	switch {
	case err != nil && !strings.HasPrefix(tasks, ".."):
		return "" // inside the project: init created it, a first run fills it
	case err != nil:
		return fmt.Sprintf("tasks.dir %s does not exist — the analysis lives in another "+
			"checkout; clone it there, or `git submodule update --init`. Without it the "+
			"analyst writes into nothing and the developer's step 1 finds no analysis", tasks)
	case len(entries) == 0 && strings.HasPrefix(tasks, ".."):
		return fmt.Sprintf("tasks.dir %s is empty — an uninitialised submodule looks exactly "+
			"like this; `git submodule update --init`, or check the path", tasks)
	}
	return ""
}

func checkAnalytic(dir string, m profile.Manifest) []checkResult {
	if m.Tasks == nil {
		// Not "files only": without tasks.format_spec there is no format to write
		// a task in, and without tasks.dir nowhere agreed to put one. The
		// track is absent from this project, and the entry point is not written.
		return []checkResult{{"analytic track", "warn",
			"the profile declares no `tasks` section — this track is not available here; " +
				"the developer track is unaffected"}}
	}
	var missing []string
	for slot, rel := range map[string]string{
		"tasks.format_spec":     m.Path(m.Tasks.FormatSpec),
		"tasks.publisher.setup": m.Path(m.Tasks.Publisher.Setup),
		// Anchored at the pipeline root, so it is not resolved through the
		// profile: the contract belongs to the publisher.
		"tasks.publisher.contract": pipelinePath(m.Tasks.Publisher.Contract),
	} {
		if rel == "" {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); err != nil {
			missing = append(missing, slot)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		return []checkResult{{"analytic track", "fail",
			fmt.Sprintf("%s missing from the installed profile — the analyst stops "+
				"before writing any task", strings.Join(missing, ", "))}}
	}
	if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(install.ClaudeAnalytic))); err != nil {
		return []checkResult{{"analytic track", "warn",
			fmt.Sprintf("%s missing — run `trunkline update`; the neutral entry "+
				"%s/AGENTS.md still describes the track", install.ClaudeAnalytic, install.Root)}}
	}

	// tasks.dir is the one slot that can point outside the project — at a
	// sibling analysis checkout or a submodule. Nothing else notices when it is
	// absent: the analyst would write into a fresh empty directory and the
	// developer's step 1 would find no analysis state, so the boundary of the
	// task — which requirements went to the epic's other tasks — would quietly
	// become guesswork.
	if msg := tasksDirState(dir, m.Tasks.Dir); msg != "" {
		return []checkResult{{"analytic track", "fail", msg}}
	}
	return []checkResult{{"analytic track", "ok",
		fmt.Sprintf("/trunkline-analytic, tasks in %s", m.Tasks.Dir)}}
}

// entryPointProfiles returns the profile the entry points were generated for
// and the files naming it, when that profile is not the declared one. The
// entry points are written in one pass, so one name covers all of them. A
// missing or unreadable file is not reported here — the check above already
// covers presence.
func entryPointProfiles(dir, declared string) (string, []string) {
	var got string
	var files []string
	for _, rel := range []string{install.AgentsEntryFile, install.ClaudeCommand,
		install.ClaudeQuick, install.ClaudeAnalytic} {
		b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
		if err != nil {
			continue
		}
		if p := install.EntryProfile(b); p != "" && p != declared {
			got, files = p, append(files, rel)
		}
	}
	return got, files
}

func locallyModified(dir string, st *state.State) []string {
	var out []string
	for rel := range st.Files {
		if st.LocallyModified(dir, rel) {
			out = append(out, rel)
		}
	}
	sort.Strings(out)
	return out
}

// readFlags parses the `## Pipeline flags` block of AGENTS.md. It is a list of
// `- key: value` lines with optional trailing comments — deliberately the same
// thing a human reads, so there is no second source of truth.
//
// A file that cannot be read is an error, not an empty set of flags. Every
// caller has a default for a missing flag, so "unreadable" and "unset" used to
// be the same answer, and the defaults are not harmless: unset language means
// English, and `update` acted on it. AGENTS.md is read whole for the same
// reason — a bufio.Scanner stops at a 64 KiB line, and one minified JSON blob
// or base64 payload in the file was enough to truncate the parse without a word.
// hasClosedRuns reports whether the journal records a finished run of this
// project. It is the only durable evidence that step 20 has ever had a chance
// to write the knowledge directory: the workdir is swept by age, but the
// journal is exempt from the sweep. `metrics: none` leaves no evidence either
// way, and the honest answer there is "assume it has run" — a project that
// switched the journal off is not a fresh one.
func hasClosedRuns(dir, metrics string) bool {
	if metrics == "none" {
		return true
	}
	if metrics == "" {
		metrics = install.WorkDir + "/metrics/pipeline.jsonl"
	}
	b, err := os.ReadFile(resolve(dir, metrics))
	if err != nil {
		return false
	}
	return strings.Contains(string(b), `"event":"done"`) ||
		strings.Contains(string(b), `"event": "done"`)
}

// projectDirs lists the project directories a base root already holds, other
// than `except`. A directory counts as a project when it looks like one — it
// carries modules/, slices/ or adr/ — so a stray .git or .idea is not mistaken
// for a neighbouring service.
func projectDirs(root, except string) []string {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() || e.Name() == except || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		for _, marker := range []string{"modules", "slices", "adr"} {
			if fi, serr := os.Stat(filepath.Join(root, e.Name(), marker)); serr == nil && fi.IsDir() {
				out = append(out, e.Name())
				break
			}
		}
	}
	return out
}

func resolve(dir, p string) string {
	if filepath.IsAbs(p) {
		return filepath.FromSlash(p)
	}
	return filepath.Join(dir, filepath.FromSlash(p))
}

func readFlags(dir string) (map[string]string, error) {
	flags := map[string]string{}
	b, err := os.ReadFile(filepath.Join(dir, install.FlagsFile))
	if os.IsNotExist(err) {
		return flags, nil
	}
	if err != nil {
		return nil, err
	}

	inBlock := false
	for raw := range strings.SplitSeq(string(b), "\n") {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "#") {
			body := strings.TrimSpace(strings.TrimLeft(line, "#"))
			// `# - flag: value` is a commented-out flag, not a heading. Only a
			// real heading ends the block; the block's own heading starts it.
			if strings.HasPrefix(body, "-") {
				continue
			}
			if strings.EqualFold(body, "Pipeline flags") {
				inBlock = true
			} else if inBlock {
				break
			}
			continue
		}
		if !inBlock || !strings.HasPrefix(line, "- ") {
			continue
		}
		body := strings.TrimPrefix(line, "- ")
		key, value, ok := strings.Cut(body, ":")
		if !ok {
			continue
		}
		if i := strings.Index(value, "#"); i >= 0 {
			value = value[:i]
		}
		flags[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return flags, nil
}

func joinProfiles(p map[string]string) string {
	if len(p) == 0 {
		return "none"
	}
	var parts []string
	for name, v := range p {
		parts = append(parts, name+" "+v)
	}
	sort.Strings(parts)
	return strings.Join(parts, ", ")
}
