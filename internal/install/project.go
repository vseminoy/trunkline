// Package install turns the embedded payload into a working setup inside a
// target project: the vendored pipeline under .trunkline/, the flags block in
// AGENTS.md, the entry points agents read, and the machine gate on commits.
//
// Everything it writes is recorded in .trunkline/state.yaml, so a later update
// can distinguish its own output from the user's edits.
package install

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/vseminoy/trunkline/internal/atomicfile"
	"github.com/vseminoy/trunkline/internal/profile"
)

// Layout is where things land in the project. The pipeline text resolves its
// own paths against Root, so a project needs no path fixups after install.
const (
	Root          = ".trunkline"
	CoreDir       = Root + "/core"
	ProfilesDir   = Root + "/profiles"
	PublishersDir = Root + "/publishers"
	SkillsDir     = Root + "/agent-skills"
	StateDir      = ".pipeline"
	// WorkDir holds the state of a single run — the rule registry, step 7a's
	// cross-check, the findings of steps 9+10 and the journal. It is swept by
	// age (see core/developer/scripts/workdir-clean.sh) and never committed.
	WorkDir       = StateDir + "/workdir"
	ClaudeCommand = ".claude/commands/trunkline-dev.md"
	// ClaudeQuick is the short-change route: the same core and the same profile
	// rules, entered without the pipeline.
	ClaudeQuick = ".claude/commands/trunkline-quick.md"
	// ClaudeAnalytic is written only for a profile that declares a tracker.
	ClaudeAnalytic = ".claude/commands/trunkline-analytic.md"
	ClaudeHook     = ".claude/hooks/pre-commit-gate.sh"
	ClaudeSetting  = ".claude/settings.json"
	AgentsFile     = "AGENTS.md"
	// FlagsFile holds the `## Pipeline flags` block. It is AGENTS.md because the
	// flags configure the pipeline, not one host: AGENTS.md is the cross-host
	// convention every agent reads, so the settings stay readable whatever the
	// user runs the pipeline on.
	FlagsFile = AgentsFile
	// AgentsEntryFile is the host-neutral entry point inside the payload.
	AgentsEntryFile = Root + "/AGENTS.md"
)

// AgentsHeader starts an AGENTS.md that `init` had to create itself.
const AgentsHeader = "# Code standards"

// Flags are the project settings the pipeline reads from AGENTS.md.
type Flags struct {
	Profile   string
	Language  string
	Knowledge string
	// Project is the directory inside the knowledge base that holds THIS
	// project's artefacts. The base itself may span a company: several services
	// share one `knowledge:` path, and `INDEX.md` at its root is the table of
	// contents over all of them. Without this the second service to write into
	// a shared base would land its `payments.md` on top of the first one's.
	Project string
	Metrics string
}

// DefaultLanguage is the language the core itself is written in. It is also the
// fallback for every file a localization has not translated, which is what lets
// a partial localization ship without breaking a route.
const DefaultLanguage = "en"

// L10nDir holds the localizations, one directory per language, mirroring the
// layout of core/. A file present here replaces its English counterpart; a file
// absent here falls back to English.
const L10nDir = "core/l10n"

// FlagsBlock renders the `## Pipeline flags` block. Only profile is required;
// the rest are written with their defaults so an unset flag is a visible
// decision rather than a forgotten one.
func FlagsBlock(f Flags) string {
	knowledge := f.Knowledge
	if knowledge == "" {
		knowledge = StateDir + "/knowledge"
	}
	metrics := f.Metrics
	if metrics == "" {
		metrics = WorkDir + "/metrics/pipeline.jsonl"
	}
	language := f.Language
	if language == "" {
		language = DefaultLanguage
	}
	rows := [][2]string{
		{"profile: " + f.Profile, "stack profile, from .trunkline/profiles/ — required"},
		{"language: " + language, "prompt and artefact language; en is the source"},
		{"knowledge: " + knowledge, "root of the knowledge base; may span several projects, or none"},
		{"project: " + f.Project, "this project's directory inside it — adr/, modules/, slices/"},
		{"metrics: " + metrics, "run journal; none to disable"},
		{"secrets_in_code: false", "secrets in sources are a deliberate project norm"},
		{"long_lived_branches: false", "branches routinely live longer than 3 days"},
		{"staged_rollout: false", "roll critical paths out in stages"},
		{"sound: true", "beep when the run stops and waits for the user"},
	}
	width := 0
	for _, r := range rows {
		if n := len(r[0]); n > width {
			width = n
		}
	}

	var b strings.Builder
	b.WriteString("## Pipeline flags\n\n")
	for _, r := range rows {
		fmt.Fprintf(&b, "- %-*s  # %s\n", width, r[0], r[1])
	}
	// Commented out on purpose: an unset rps makes the agent ask rather than
	// invent a load target.
	// width-2 keeps the `#` column aligned with the uncommented rows above: the
	// "# " prefix already costs two characters.
	fmt.Fprintf(&b, "# - %-*s  # %s\n", width-2, "rps: 2000", "load target for steps 12 and 16")
	// Same reasoning, different default: unset means claude-code, the host that
	// degrades nothing. It ships commented rather than omitted because a flag
	// nobody can see is a flag nobody sets — and on another host the run must
	// announce which gates it loses (see core/ORCHESTRATION.md).
	fmt.Fprintf(&b, "# - %-*s  # %s\n", width-2, "host: cursor",
		"claude-code (default) | cursor | codex | generic")
	return b.String()
}

// ProjectSlug turns a directory name into the name of a directory inside the
// knowledge base. The default is the name of the directory `init` ran in, which
// is the project's name in every case anyone has to type it — and a base shared
// by several services is browsed by a human, so the names have to stay
// recognisable rather than becoming hashes.
//
// Only what would make the path ambiguous is replaced: separators, and the
// leading dots that would hide the directory. An empty or unusable name yields
// "project" rather than an error, because a name is never worth failing an
// install over — the flag is written into AGENTS.md where it can be edited.
func ProjectSlug(name string) string {
	name = strings.TrimSpace(name)
	name = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		case r == '-', r == '_', r == '.':
			return r
		default:
			return '-'
		}
	}, name)
	name = strings.Trim(name, "-.")
	if name == "" {
		return "project"
	}
	return name
}

// FlagsHeading opens the flags block and is what marks a file as already
// carrying one.
const FlagsHeading = "## Pipeline flags"

// EnsureFlags adds the flags block to AGENTS.md if it is not already there. An
// existing block is never rewritten: it is the user's configuration, and
// silently changing a flag would silently change which gates run.
func (w *Writer) EnsureFlags(f Flags) (Status, error) {
	// Deliberately not hashed into state: AGENTS.md belongs to the project, and
	// update must never touch it again.
	return w.appendBlock(FlagsFile, FlagsHeading, AgentsHeader, "\n"+FlagsBlock(f))
}

// appendBlock appends block to a project-owned file, creating it with header if
// absent and doing nothing at all if marker is already present. The file is the
// user's, so this only ever adds — it never rewrites what is there.
func (w *Writer) appendBlock(rel, marker, header, block string) (Status, error) {
	abs, err := safeJoin(w.Dir, rel)
	if err != nil {
		return "", err
	}
	existing, err := os.ReadFile(abs)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	if bytes.Contains(existing, []byte(marker)) {
		w.record(rel, Unchanged)
		return Unchanged, nil
	}

	var buf bytes.Buffer
	if len(existing) > 0 {
		buf.Write(existing)
		if !bytes.HasSuffix(existing, []byte("\n")) {
			buf.WriteByte('\n')
		}
	} else {
		buf.WriteString(header + "\n")
	}
	buf.WriteString(block)

	status := Created
	if len(existing) > 0 {
		status = Updated
	}
	if !w.DryRun {
		if err := atomicfile.Write(abs, buf.Bytes(), 0o644); err != nil {
			return status, err
		}
	}
	w.record(rel, status)
	return status, nil
}

// EnsurePipelineDirs creates .pipeline/ with a note saying which half of it is
// committed. Step 20 fills knowledge/; the directory exists early so the answer
// to "should this be gitignored" is written down before the first run.
//
// tasksDir is the profile's tasks.dir slot, empty when the profile declares no
// analytic track. It is read rather than assumed: the slot may point outside the
// project — `../analysis/tasks/` when the analysis lives in its own repository —
// and creating .pipeline/tasks regardless would leave an empty directory nobody
// writes to next to a real one nobody created.
//
// kbDir is this project's directory inside the knowledge base — `<knowledge>/
// <project>` — empty when the base is switched off. Like tasksDir it may point
// outside the project, and for the same reason it is then left to whoever owns
// that checkout: a company base shared by six services is not this install's to
// create.
func (w *Writer) EnsurePipelineDirs(tasksDir, kbDir string) error {
	tasksNote := ""
	var extraDirs []string
	if tasksDir != "" {
		where := "here in this project"
		if strings.HasPrefix(tasksDir, "..") {
			where = "outside this project, in the analysis checkout"
		}
		tasksNote = "\n- " + tasksDir + " — the analytic track's tasks, one directory per epic,\n" +
			"  plus _analysis/ for the analysis state. They live " + where + ".\n"
		// A path leaving the project belongs to another repository: creating it
		// would fabricate half of someone else's checkout. doctor reports it
		// missing instead, which is the honest failure.
		if !strings.HasPrefix(tasksDir, "..") {
			extraDirs = []string{strings.TrimSuffix(tasksDir, "/")}
		}
	}
	// Three different facts, and the note has to state the right one: a base
	// switched off, a base inside this repository (commit it here), or a base in
	// another checkout (this repository commits nothing of it, and saying
	// "COMMIT these" would be an instruction nobody can carry out).
	kbHeading := "The knowledge base"
	kbNote := "- This project keeps no knowledge base (`knowledge: none`). Step 1 gathers\n" +
		"  AS-IS from scratch every run and says so.\n"
	switch {
	case kbDir == "":
	case Internal(kbDir):
		kbHeading = "COMMIT the knowledge base — it describes the code at a specific commit, and a\n" +
			"teammate without it pays full context price on every task"
		kbNote = "- " + kbDir + "/ — this project's artefacts: adr/, modules/, slices/.\n"
	default:
		kbHeading = "The knowledge base lives OUTSIDE this repository, so there is nothing here to\n" +
			"commit for it — version it where it lives"
		kbNote = "- " + kbDir + "/ — this project's artefacts: adr/, modules/, slices/.\n"
	}
	note := `# .pipeline/

State the pipeline keeps about *this* project, split by how long it lives.

` + kbHeading + `:

` + kbNote + `
  <knowledge>/
  ├── INDEX.md          ← the table of contents, over every project in the base
  └── <project>/
      ├── adr/          ← irreversible decisions and their cost (step 15)
      ├── modules/      ← the AS-IS of every module: what the code does today
      └── slices/       ← the changeset history, one immutable entry per run

Written by steps 15 and 20, read first by step 1. Two flags name it: ` + "`knowledge:`" + `
is the root of the base, ` + "`project:`" + ` the directory inside it that is this
repository. Several services sharing one base point ` + "`knowledge:`" + ` at the same
path and differ only in ` + "`project:`" + `.
` + tasksNote + `
DO NOT COMMIT workdir/ — put ` + "`/.pipeline/workdir/`" + ` in .gitignore:

- workdir/rules/       — the rule registry step 2 writes.
- workdir/consistency/ — step 7a's cross-check.
- workdir/review/      — the findings of steps 9+10.
- workdir/metrics/     — the run journal, one line per event.

They are read during the run and superseded when it ends: what survives goes to
knowledge/ and to the commit's ` + "`Rules:`" + ` trailer. Sweep them by age — a
month of runs is hundreds of kilobytes nobody reads:

    .trunkline/core/developer/scripts/workdir-clean.sh   # deletes files older than 7 days

The journal is exempt from the sweep: it is one append-only file measuring the
process across many runs, not a per-run artefact.
`
	if _, err := w.Write(StateDir+"/README.md", []byte(note), 0o644); err != nil {
		return err
	}
	if w.DryRun {
		return nil
	}
	dirs := []string{WorkDir + "/metrics"}
	// A base outside this repository belongs to another checkout — the company
	// base every service writes into. Creating it here would fabricate half of
	// it; doctor reports it missing instead, which is the honest failure.
	//
	// Internal() rather than a "../" check: an ABSOLUTE path joined onto the
	// project reproduced the whole path inside it — `--knowledge /srv/kb` left a
	// `srv/kb/` tree in the repository and nothing at /srv/kb.
	if Internal(kbDir) {
		dirs = append(dirs, kbDir)
	}
	for _, d := range append(dirs, extraDirs...) {
		if err := os.MkdirAll(filepath.Join(w.Dir, d), 0o755); err != nil {
			return err
		}
	}
	return nil
}

// GateKind is how the commit gate is triggered. The script is the same in
// every case — only the host that fires it differs, which is the whole point of
// treating orchestration as a capability rather than a dependency.
type GateKind string

const (
	// GateClaude intercepts the agent's `git commit` call via a PreToolUse
	// hook. Fires before the commit is attempted and hands the failure back
	// to the agent, which can then fix it.
	GateClaude GateKind = "claude"
	// GateGit is a real .git/hooks/pre-commit. Works with any host, including
	// ones with no hook mechanism, and also catches commits made by hand.
	GateGit GateKind = "git"
	// GateNone leaves enforcement entirely to the model obeying text.
	GateNone GateKind = "none"
)

// EnsureGate installs the profile's pre-commit gate in the requested form. A
// profile without a gate is not an error — it is a known hole in enforcement,
// reported to the caller.
func (w *Writer) EnsureGate(kind GateKind, m profile.Manifest, script []byte) (bool, error) {
	if kind == GateNone || m.Hooks.PreCommitGate == "" || len(script) == 0 {
		return false, nil
	}
	if _, err := w.Write(ClaudeHook, script, 0o755); err != nil {
		return false, err
	}
	switch kind {
	case GateGit:
		return true, w.installGitHook()
	default:
		return true, w.registerHook()
	}
}

// gitPreCommit delegates to the profile script rather than duplicating it, so
// the two gate forms cannot enforce different things.
const gitPreCommit = `#!/usr/bin/env bash
# Installed by trunkline. Delegates to the profile's commit gate.
# Remove this file to disable, or reinstall with: trunkline init --gate=git
set -euo pipefail
gate="$(git rev-parse --show-toplevel)/.claude/hooks/pre-commit-gate.sh"
[ -x "$gate" ] || exit 0
# The same script serves both triggers. In this one there is no JSON on stdin to
# recognise a commit from — being called at all is the signal — so the mode is
# declared rather than sniffed. Without it the script finds no command and exits
# 0, and the gate silently stops gating.
export TRUNKLINE_GATE_MODE=git
exec bash "$gate"
`

// GitHooksDir resolves where this repository's hooks actually live.
//
// `git rev-parse --git-path hooks` is asked rather than assuming .git/hooks,
// because that assumption is wrong in two ordinary setups: a linked worktree,
// where .git is a file, and a repository with core.hooksPath set. Both would
// take a hook written to .git/hooks and never run it, while `doctor` went on
// reporting the gate installed.
func GitHooksDir(dir string) (string, error) {
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		return "", fmt.Errorf("--gate=git needs a git repository here (no .git found)")
	}
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--git-path", "hooks").Output()
	if err != nil {
		// No git binary is not the same as no repository: fall back to the
		// layout that is right for a plain, non-worktree checkout.
		return filepath.Join(dir, ".git", "hooks"), nil
	}
	p := strings.TrimSpace(string(out))
	if p == "" {
		return filepath.Join(dir, ".git", "hooks"), nil
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(dir, p)
	}
	return p, nil
}

// gitHooksPathConfigured reports a core.hooksPath set in this repository. It is
// a refusal condition rather than something to follow: that setting means
// another tool (husky, lefthook) owns the hooks here, and both writing our hook
// into its directory and writing it into .git/hooks — where it would never
// run — are worse than saying so.
func gitHooksPathConfigured(dir string) string {
	out, err := exec.Command("git", "-C", dir, "config", "--get", "core.hooksPath").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// PrecheckGit reports what would stop the git gate from being installed, before
// anything has been written.
//
// The ordering is the point. The gate goes in last, so without this check a
// repository that cannot take it is discovered after a hundred-odd files have
// already been vendored — and `init` then fails with the project half installed.
func PrecheckGit(dir string) error {
	if _, err := GitHooksDir(dir); err != nil {
		return err
	}
	if p := gitHooksPathConfigured(dir); p != "" {
		return fmt.Errorf("this repository routes hooks through core.hooksPath=%s, which "+
			"another tool (husky, lefthook) normally owns — a hook written to .git/hooks "+
			"would never run. Add `bash .claude/hooks/pre-commit-gate.sh` to %s/pre-commit "+
			"yourself, or install with --gate=claude", p, p)
	}
	hooks, err := GitHooksDir(dir)
	if err != nil {
		return err
	}
	target := filepath.Join(hooks, "pre-commit")
	existing, err := os.ReadFile(target)
	switch {
	case err == nil && !bytes.Contains(existing, []byte("Installed by trunkline")):
		return fmt.Errorf("%s already exists and is not ours; add "+
			"`bash .claude/hooks/pre-commit-gate.sh` to it by hand", target)
	case err != nil && !os.IsNotExist(err):
		// Unreadable is not absent. The previous version let this case through
		// to the write, so a hook we could not read was clobbered — the exact
		// thing the check above exists to prevent.
		return fmt.Errorf("cannot read %s (%v); refusing to replace a hook I cannot look at", target, err)
	}
	return nil
}

// installGitHook writes the repository's pre-commit hook. It refuses to clobber
// a hook the project already has: a build gate is not worth losing someone's
// existing one.
func (w *Writer) installGitHook() error {
	if err := PrecheckGit(w.Dir); err != nil {
		return err
	}
	hooks, err := GitHooksDir(w.Dir)
	if err != nil {
		return err
	}
	target := filepath.Join(hooks, "pre-commit")
	rel := filepath.ToSlash(target)
	if r, err := filepath.Rel(w.Dir, target); err == nil {
		rel = filepath.ToSlash(r)
	}
	if w.DryRun {
		w.record(rel, Created)
		return nil
	}
	if err := atomicfile.Write(target, []byte(gitPreCommit), 0o755); err != nil {
		return err
	}
	// Not recorded in state.Files: the hooks directory is not part of the
	// vendored tree and is never pruned or updated.
	w.record(rel, Created)
	return nil
}

const gateCommand = `bash "${CLAUDE_PROJECT_DIR}/.claude/hooks/pre-commit-gate.sh"`

// gateInPreToolUse reports whether some PreToolUse entry already fires the gate.
//
// The match is on the command, not on the matcher. A project that widened its
// matcher to "Bash|Write" still runs the gate, and looking only under the exact
// matcher "Bash" would append a second copy on every install — after which the
// gate runs twice on every Bash call, and its output is duplicated in every
// failure the agent has to read.
func gateInPreToolUse(pre []any) bool {
	return firesScript(pre, "pre-commit-gate.sh")
}

// preToolUse pulls hooks.PreToolUse out of a settings object.
//
// A key that is present but of another shape is an error, never an empty
// result. The discarded `ok` of a type assertion turns a hooks object the tool
// does not recognise into nil, and the write that follows then replaces the
// user's configuration with ours — silently, exit code 0. Refusing is the only
// honest answer: we cannot merge into a shape we do not understand.
func preToolUse(settings map[string]any) ([]any, map[string]any, error) {
	return hookEvent(settings, "PreToolUse")
}

// hookEvent pulls one hooks.<event> array out of a settings object, with the
// same refusal on an unrecognised shape as preToolUse describes.
func hookEvent(settings map[string]any, event string) ([]any, map[string]any, error) {
	hooks := map[string]any{}
	if raw, ok := settings["hooks"]; ok && raw != nil {
		h, isObj := raw.(map[string]any)
		if !isObj {
			return nil, nil, fmt.Errorf("%s has a `hooks` key that is not an object (%T); "+
				"refusing to rewrite it — add the hook by hand", ClaudeSetting, raw)
		}
		hooks = h
	}
	var entries []any
	if raw, ok := hooks[event]; ok && raw != nil {
		e, isArr := raw.([]any)
		if !isArr {
			return nil, nil, fmt.Errorf("%s has a `hooks.%s` that is not an array (%T); "+
				"refusing to rewrite it — add the hook by hand", ClaudeSetting, event, raw)
		}
		entries = e
	}
	return entries, hooks, nil
}

// firesScript reports whether some entry of a hooks array already runs a script
// with this name. Matching on the name rather than the whole command is what
// keeps a project that edited the path or added an argument from collecting a
// second copy on every install.
func firesScript(entries []any, name string) bool {
	for _, entry := range entries {
		for _, h := range hooksOf(entry) {
			hm, _ := h.(map[string]any)
			if cmd, _ := hm["command"].(string); strings.Contains(cmd, name) {
				return true
			}
		}
	}
	return false
}

// dropScript returns the entries that do not run a script with this name. An
// entry mixing our script with the project's own hooks is left alone: removing
// it would take the project's hook with it, and that is not ours to delete.
func dropScript(entries []any, name string) []any {
	kept := make([]any, 0, len(entries))
	for _, entry := range entries {
		if firesScript([]any{entry}, name) && len(hooksOf(entry)) == 1 {
			continue
		}
		kept = append(kept, entry)
	}
	return kept
}

// hooksOf pulls the inner hooks array out of one entry of a hooks event.
func hooksOf(entry any) []any {
	e, _ := entry.(map[string]any)
	if e == nil {
		return nil
	}
	inner, _ := e["hooks"].([]any)
	return inner
}

// SoundHook is the script the sound hooks run, vendored with the payload.
const SoundHook = SkillsDir + "/hooks/notify-sound.sh"

// soundEvents maps each host event that hands the turn back to the user to the
// argument notify-sound.sh takes for it. Only Notification qualifies: it is the
// agent asking a human for something. Stop and SubagentStop mean the agent
// finished a step, which on a pipeline run happens dozens of times — beeping
// there turns the signal into noise, so they are removed rather than wired.
var soundEvents = [][2]string{
	{"Notification", "waiting"},
}

// staleSoundEvents are events an earlier version wired the beep into. They are
// stripped on every install so a project that already has them goes quiet from
// `trunkline update`, without the user editing settings.json by hand.
var staleSoundEvents = []string{"Stop", "SubagentStop"}

// EnsureSoundHooks wires the beep that calls the user back to the terminal into
// .claude/settings.json, leaving hooks the project already has untouched.
//
// It runs on update as well as init, and is idempotent, so a project installed
// before the sound existed picks it up from `trunkline update` rather than
// needing a migration. Whether the beep actually sounds is not decided here but
// by the `sound` flag in AGENTS.md, which the script reads on every call — so
// turning it off is an edit to the flags block, not a reinstall.
func (w *Writer) EnsureSoundHooks() error {
	abs, err := safeJoin(w.Dir, ClaudeSetting)
	if err != nil {
		return err
	}
	settings := map[string]any{}
	if b, err := os.ReadFile(abs); err == nil {
		if err := json.Unmarshal(b, &settings); err != nil {
			return fmt.Errorf("parse %s: %w", ClaudeSetting, err)
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	changed := false
	for _, ev := range soundEvents {
		event, kind := ev[0], ev[1]
		entries, hooks, err := hookEvent(settings, event)
		if err != nil {
			return err
		}
		if firesScript(entries, "notify-sound.sh") {
			continue
		}
		entries = append(entries, map[string]any{
			"hooks": []any{map[string]any{
				"type":    "command",
				"command": fmt.Sprintf(`bash "${CLAUDE_PROJECT_DIR}/%s" %s`, SoundHook, kind),
				"timeout": 5,
			}},
		})
		hooks[event] = entries
		settings["hooks"] = hooks
		changed = true
	}
	for _, event := range staleSoundEvents {
		entries, hooks, err := hookEvent(settings, event)
		if err != nil {
			return err
		}
		kept := dropScript(entries, "notify-sound.sh")
		if len(kept) == len(entries) {
			continue
		}
		// An event left with no hooks at all loses its key: an empty array is
		// not configuration, it is a leftover of one.
		if len(kept) == 0 {
			delete(hooks, event)
		} else {
			hooks[event] = kept
		}
		settings["hooks"] = hooks
		changed = true
	}
	if !changed {
		w.record(ClaudeSetting, Unchanged)
		return nil
	}

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')
	status := Updated
	if _, err := os.Stat(abs); os.IsNotExist(err) {
		status = Created
	}
	if !w.DryRun {
		if err := atomicfile.Write(abs, out, 0o644); err != nil {
			return err
		}
	}
	w.record(ClaudeSetting, status)
	return nil
}

// registerHook merges the gate into .claude/settings.json without disturbing
// hooks the project already has.
func (w *Writer) registerHook() error {
	abs, err := safeJoin(w.Dir, ClaudeSetting)
	if err != nil {
		return err
	}
	settings := map[string]any{}
	if b, err := os.ReadFile(abs); err == nil {
		if err := json.Unmarshal(b, &settings); err != nil {
			return fmt.Errorf("parse %s: %w", ClaudeSetting, err)
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	pre, hooks, err := preToolUse(settings)
	if err != nil {
		return err
	}
	if gateInPreToolUse(pre) {
		w.record(ClaudeSetting, Unchanged)
		return nil
	}

	pre = append(pre, map[string]any{
		"matcher": "Bash",
		"hooks": []any{map[string]any{
			"type":    "command",
			"command": gateCommand,
		}},
	})
	hooks["PreToolUse"] = pre
	settings["hooks"] = hooks

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')
	status := Updated
	if _, err := os.Stat(abs); os.IsNotExist(err) {
		status = Created
	}
	if !w.DryRun {
		if err := atomicfile.Write(abs, out, 0o644); err != nil {
			return err
		}
	}
	w.record(ClaudeSetting, status)
	return nil
}

// GateRegistered reports whether the commit gate is actually wired to a
// trigger, in either supported form, and which one.
//
// The settings file is parsed rather than searched for a substring: the name of
// the gate script appearing anywhere in the JSON — in a PostToolUse entry, in a
// comment-like key, in a path — used to be enough to report the gate installed.
func GateRegistered(dir string) (GateKind, bool) {
	if b, err := os.ReadFile(filepath.Join(dir, ClaudeSetting)); err == nil {
		var settings map[string]any
		if json.Unmarshal(b, &settings) == nil {
			if pre, _, err := preToolUse(settings); err == nil && gateInPreToolUse(pre) {
				return GateClaude, true
			}
		}
	}
	if hooks, err := GitHooksDir(dir); err == nil {
		if b, err := os.ReadFile(filepath.Join(hooks, "pre-commit")); err == nil &&
			bytes.Contains(b, []byte("pre-commit-gate.sh")) {
			return GateGit, true
		}
	}
	return GateNone, false
}

// GateBroken names the reason a gate that looks wired would not actually stop a
// commit, or "" if it would.
//
// The reason it is a separate check is that every failure here is silent by
// construction: git skips a hook without the executable bit, the hook's own
// `[ -x "$gate" ] || exit 0` exits zero for the same reason, and neither says
// anything. Both paths end with a green `init` and a green `doctor` over a
// pipeline whose one machine gate has stopped gating.
func GateBroken(dir string, kind GateKind) string {
	script := filepath.Join(dir, filepath.FromSlash(ClaudeHook))
	switch fi, err := os.Stat(script); {
	case err != nil:
		return fmt.Sprintf("%s is missing", ClaudeHook)
	case fi.Mode()&0o111 == 0:
		return fmt.Sprintf("%s is not executable (%s), so the hook exits 0 without running it",
			ClaudeHook, fi.Mode().Perm())
	}
	if kind != GateGit {
		return ""
	}
	hooks, err := GitHooksDir(dir)
	if err != nil {
		return err.Error()
	}
	target := filepath.Join(hooks, "pre-commit")
	fi, err := os.Stat(target)
	switch {
	case err != nil:
		return fmt.Sprintf("%s is missing", target)
	case fi.Mode()&0o111 == 0:
		return fmt.Sprintf("%s is not executable (%s), so git skips it without a word",
			target, fi.Mode().Perm())
	}
	return ""
}

// PipelineIgnored reports whether .gitignore would swallow the knowledge base.
// kb is the base's root as the `knowledge` flag spells it; "" means the default.
// A base that does not live inside this repository cannot be hidden by this
// repository's .gitignore — a company base at /srv/knowledge is versioned, or
// not, somewhere else entirely — so an external path is never a finding here.
// Reporting one made every project with a shared base fail a check it had no
// way to satisfy.
func PipelineIgnored(dir, kb string) bool {
	if kb == "" {
		kb = StateDir + "/knowledge"
	}
	if kb == "none" || !Internal(kb) {
		return false
	}
	kb = strings.TrimSuffix(path.Clean(filepath.ToSlash(kb)), "/")
	for _, p := range ignorePatterns(dir) {
		p = strings.TrimSuffix(p, "/")
		// A pattern covering .pipeline/ as a whole, or the base itself, or any
		// directory above it. `.pipeline/workdir/` is a different pattern and a
		// correct one — see RunStateIgnored.
		if p == kb || strings.HasPrefix(kb, p+"/") {
			return true
		}
	}
	return false
}

// Internal reports whether a path from a flag points inside this project. An
// absolute path or one climbing out with ".." belongs to another checkout, and
// nothing this project does — gitignoring, creating, committing — reaches it.
func Internal(p string) bool {
	if p == "" || p == "none" {
		return false
	}
	if filepath.IsAbs(p) || strings.HasPrefix(p, "/") {
		return false
	}
	return !strings.HasPrefix(path.Clean(filepath.ToSlash(p)), "..")
}

// RunStateDirs are the parts of .pipeline/ that hold the state of a single run
// and are superseded when it ends: the rule registry, step 7a's cross-check,
// the findings of steps 9+10 and the journal. They live under one directory so
// .gitignore needs one line rather than four — a project that got three of the
// four right still carried the fourth into everyone's history. Committing them
// is not a catastrophe but a slow tax: the journal is append-only, so every
// branch that ran the pipeline conflicts with every other one on merge, forever.
var RunStateDirs = []string{"workdir"}

// RunStateIgnored reports which of RunStateDirs are NOT covered by .gitignore.
// The inverse of PipelineIgnored: there the risk is losing the knowledge base,
// here it is carrying run state into everyone's history.
//
// The pattern has to cover the directory whole. `.pipeline/workdir/review/`
// ignores one quarter of the run state and reads, to a prefix match, exactly
// like ignoring all of it — which is the mistake the move to a single directory
// was meant to end, so it must not be reported as covered.
func RunStateIgnored(dir string) (missing []string) {
	patterns := ignorePatterns(dir)
	for _, d := range RunStateDirs {
		want := StateDir + "/" + d
		covered := false
		for _, p := range patterns {
			p = strings.TrimSuffix(p, "/")
			if p == StateDir || p == want {
				covered = true
				break
			}
		}
		if !covered {
			missing = append(missing, d)
		}
	}
	return missing
}

// ignorePatterns returns the meaningful lines of .gitignore, leading "/" trimmed.
func ignorePatterns(dir string) []string {
	b, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		return nil
	}
	var out []string
	for line := range strings.SplitSeq(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
			continue
		}
		out = append(out, strings.TrimPrefix(line, "/"))
	}
	return out
}
