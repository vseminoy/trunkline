package cli

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	trunkline "github.com/vseminoy/trunkline"
	"github.com/vseminoy/trunkline/internal/install"
	"github.com/vseminoy/trunkline/internal/migrate"
	"github.com/vseminoy/trunkline/internal/profile"
	"github.com/vseminoy/trunkline/internal/semver"
	"github.com/vseminoy/trunkline/internal/state"
)

func runInit(env Env, args []string) (err error) {
	fs_ := flag.NewFlagSet("init", flag.ContinueOnError)
	fs_.SetOutput(env.Err)
	var (
		profileName = fs_.String("profile", "", "stack profile to install (required unless exactly one ships)")
		language    = fs_.String("language", install.DefaultLanguage, "prompt and artefact language (en, or a directory under core/l10n/)")
		knowledge   = fs_.String("knowledge", "", "knowledge base root: path, external path, or none")
		project     = fs_.String("project", "", "this project's directory inside the knowledge base (default: the directory name)")
		metrics     = fs_.String("metrics", "", "run journal path, or none")
		gate        = fs_.String("gate", "claude", "commit gate trigger: claude (PreToolUse hook), git (.git/hooks/pre-commit), none")
		force       = fs_.Bool("force", false, "overwrite files that were modified locally")
		dry         = fs_.Bool("dry-run", false, "report what would change, write nothing")
	)
	fs_.Usage = func() {
		fmt.Fprint(env.Err, "Usage: trunkline [-C dir] init [--profile NAME] [flags]\n\n"+
			"Vendors the pipeline into .trunkline/, writes the flags block into\n"+
			"AGENTS.md, creates .pipeline/ and installs the commit gate.\n"+
			"Safe to rerun: an existing flags block is never rewritten.\n\n")
		fs_.PrintDefaults()
	}
	if err := fs_.Parse(args); err != nil {
		return err
	}

	// Everything that can be judged before a byte is written is judged here.
	// The gate used to be validated after the payload was on disk, so a typo in
	// --gate left a project with a hundred vendored files, no state.yaml, and an
	// `update` that refused to run.
	kind := install.GateKind(*gate)
	switch kind {
	case install.GateClaude, install.GateGit, install.GateNone:
	default:
		return fmt.Errorf("--gate must be claude, git or none (got %q)", *gate)
	}
	name, err := resolveProfile(*profileName)
	if err != nil {
		return err
	}
	lang, err := resolveLanguage(*language)
	if err != nil {
		return err
	}
	if kind == install.GateGit {
		if err := install.PrecheckGit(env.Dir); err != nil {
			return err
		}
	}

	st, err := state.Load(env.Dir)
	if err != nil {
		return err
	}
	if state.Exists(env.Dir) && !*force {
		fmt.Fprintf(env.Out, "%s already exists — reinstalling over it.\n"+
			"Use `trunkline update` to upgrade, or --force to overwrite local edits.\n\n",
			state.File)
	}

	// Migrations run before anything is written. A project on an older layout
	// must be moved onto the current one first. On a clean project every
	// migration is a no-op.
	for _, mig := range migrate.Pending(st, st.Core, trunkline.Version()) {
		msg, err := mig.Apply(env.Dir, *dry)
		if err != nil {
			return fmt.Errorf("migration %s: %w", mig.ID, err)
		}
		if msg != "" {
			fmt.Fprintf(env.Out, "migration %s: %s\n", mig.ID, msg)
		}
		st.MarkApplied(mig.ID)
	}

	w := install.NewWriter(env.Dir, st)
	w.DryRun, w.Force = *dry, *force

	// From here on the state is saved whatever happens, because from here on
	// files exist. A run that fails halfway and saves nothing leaves a project
	// full of files with no record of who wrote them — and the next run, unable
	// to tell them from the user's own, keeps every one of them.
	//
	// st.Core is deliberately *not* set here: on a failed run the old version
	// stays, doctor reports drift, and `update` re-vendors the tree. Claiming
	// the new version for a half-written install would make the repair invisible.
	st.Language = lang
	defer func() {
		if *dry {
			return
		}
		if serr := st.Save(env.Dir); serr != nil {
			err = errors.Join(err, fmt.Errorf("save %s: %w", state.File, serr))
		}
	}()

	m, err := installPayload(w, name, lang)
	if err != nil {
		return err
	}
	if err := writeEntryPoints(w, name, lang, m); err != nil {
		return err
	}

	// The default is the name of the directory `init` ran in. It is resolved to
	// an absolute path first: `trunkline -C .` in the project would otherwise
	// name the project "." — the one spelling that is both plausible to type and
	// useless as a directory inside a shared base.
	proj := install.ProjectSlug(*project)
	if *project == "" {
		abs, aerr := filepath.Abs(env.Dir)
		if aerr != nil {
			abs = env.Dir
		}
		proj = install.ProjectSlug(filepath.Base(abs))
	}
	flagStatus, err := w.EnsureFlags(install.Flags{
		Profile: name, Language: lang, Knowledge: *knowledge, Project: proj, Metrics: *metrics,
	})
	if err != nil {
		return err
	}
	// An existing flags block is deliberately never rewritten, which means
	// `init --profile B` on a project declaring A installs B and leaves the
	// agent following A. Silent divergence is the one outcome worse than either
	// choice, so it is reported.
	if flagStatus == install.Unchanged {
		declared, ferr := readFlags(env.Dir)
		if ferr != nil {
			fmt.Fprintf(env.Out, "warning: %s already has a `%s` block, but it cannot be "+
				"read (%v) — whether it agrees with what was just installed is unknown.\n\n",
				install.FlagsFile, install.FlagsHeading, ferr)
		} else {
			warnFlagDivergence(env, declared, name, lang)
		}
	}
	if _, err := w.EnsureAgentsPointer(); err != nil {
		return err
	}
	tasksDir := ""
	if m.Tasks != nil {
		tasksDir = m.Tasks.Dir
	}
	if err := w.EnsurePipelineDirs(tasksDir, knowledgeProjectDir(*knowledge, proj)); err != nil {
		return err
	}

	gated, err := installGate(w, kind, m)
	if err != nil {
		return err
	}
	if err := w.EnsureSoundHooks(); err != nil {
		return err
	}

	st.Core, st.CLI = trunkline.Version(), trunkline.Version()
	st.Profiles[name] = m.Version

	fmt.Fprintf(env.Out, "trunkline %s installed (profile %s %s)\n", trunkline.Version(), m.Name, m.Version)
	if drift, derr := l10nDrift(lang); derr == nil && len(drift) > 0 {
		fmt.Fprint(env.Out, l10nDriftMessage(lang, drift))
	}
	fmt.Fprint(env.Out, w.Summary())
	reportNextSteps(env, m, name, gated, kind)
	if *dry {
		fmt.Fprint(env.Out, "\n(dry run: nothing was written)\n")
	}
	return nil
}

// ensurePipelineNote rewrites .pipeline/README.md from the project's current
// flags. Unreadable flags are not fatal: the note is documentation, and failing
// an upgrade over it would be worse than leaving it one version behind.
func ensurePipelineNote(env Env, w *install.Writer, st *state.State) error {
	flags, err := readFlags(env.Dir)
	if err != nil {
		fmt.Fprintf(env.Out, "warning: %s could not be read (%v) — %s/README.md left as it is.\n",
			install.FlagsFile, err, install.StateDir)
		return nil
	}
	tasksDir := ""
	if name := flags["profile"]; name != "" {
		if m, perr := profile.Load(trunkline.Profiles, path.Join("profiles", name)); perr == nil &&
			m.Tasks != nil {
			tasksDir = m.Tasks.Dir
		}
	}
	proj := flags["project"]
	if proj == "" {
		abs, aerr := filepath.Abs(env.Dir)
		if aerr != nil {
			abs = env.Dir
		}
		proj = install.ProjectSlug(filepath.Base(abs))
	}
	return w.EnsurePipelineDirs(tasksDir, knowledgeProjectDir(flags["knowledge"], proj))
}

// knowledgeProjectDir joins the base root and the project directory into the
// path this repository actually writes to. `none` yields "", which is what
// switches the directory off rather than creating one called "none".
func knowledgeProjectDir(knowledge, project string) string {
	if knowledge == "none" {
		return ""
	}
	if knowledge == "" {
		knowledge = install.StateDir + "/knowledge"
	}
	return path.Join(knowledge, project)
}

// warnFlagDivergence reports flags that disagree with what was just installed.
// It only warns: rewriting the block would silently change which gates run,
// and choosing between the two configurations is the user's call.
func warnFlagDivergence(env Env, flags map[string]string, name, lang string) {
	if got := flags["profile"]; got != "" && got != name {
		fmt.Fprintf(env.Out, "warning: profile %s was installed, but %s still declares %s.\n"+
			"The agent follows the flags, so the run will use %s. Edit the `## Pipeline flags`\n"+
			"block to switch, or rerun with --profile %s.\n\n", name, install.FlagsFile, got, got, got)
	}
	declared := flags["language"]
	if declared == "" {
		declared = install.DefaultLanguage
	}
	if declared != lang {
		fmt.Fprintf(env.Out, "warning: the %s core was installed, but %s declares language %s.\n"+
			"The prompts on disk are %s; the flag does not change them.\n\n", lang, install.FlagsFile, declared, lang)
	}
}

func runUpdate(env Env, args []string) (err error) {
	fs_ := flag.NewFlagSet("update", flag.ContinueOnError)
	fs_.SetOutput(env.Err)
	var (
		force = fs_.Bool("force", false, "overwrite files that were modified locally")
		dry   = fs_.Bool("dry-run", false, "report what would change, write nothing")
	)
	fs_.Usage = func() {
		fmt.Fprint(env.Err, "Usage: trunkline [-C dir] update [flags]\n\n"+
			"Upgrades the vendored core and every installed profile to the versions\n"+
			"in this binary, then runs pending migrations. Files you edited are kept\n"+
			"and listed, never overwritten silently.\n\n")
		fs_.PrintDefaults()
	}
	if err := fs_.Parse(args); err != nil {
		return err
	}

	if !state.Exists(env.Dir) {
		return fmt.Errorf("no %s here — run `trunkline init` first", state.File)
	}
	st, err := state.Load(env.Dir)
	if err != nil {
		return err
	}

	from, to := st.Core, trunkline.Version()
	switch {
	case from == to:
		fmt.Fprintf(env.Out, "core already at %s; refreshing files and profiles.\n", to)
	case semver.Less(to, from):
		fmt.Fprintf(env.Out, "installed core %s is newer than this binary (%s) — downgrading.\n", from, to)
	default:
		fmt.Fprintf(env.Out, "core %s -> %s\n", from, to)
	}

	pending := migrate.Pending(st, from, to)
	w := install.NewWriter(env.Dir, st)
	w.DryRun, w.Force = *dry, *force

	// The language is the project's decision, made at init. update honours it
	// rather than re-asking: silently reverting a localized project to English
	// is the one thing this must never do.
	lang, err := activeLanguage(env.Dir, st)
	if err != nil {
		return err
	}
	st.Language = lang
	if lang != install.DefaultLanguage {
		fmt.Fprintf(env.Out, "language %s\n", lang)
		if drift, derr := l10nDrift(lang); derr == nil && len(drift) > 0 {
			fmt.Fprint(env.Out, l10nDriftMessage(lang, drift))
		}
	}

	// Same reason as in init: once files have been written, the record of them
	// must survive a later failure.
	defer func() {
		if *dry {
			return
		}
		if serr := st.Save(env.Dir); serr != nil {
			err = errors.Join(err, fmt.Errorf("save %s: %w", state.File, serr))
		}
	}()

	for name := range st.Profiles {
		m, err := installPayload(w, name, lang)
		if err != nil {
			return fmt.Errorf("profile %s: %w", name, err)
		}
		if old := st.Profiles[name]; old != m.Version {
			fmt.Fprintf(env.Out, "profile %s %s -> %s\n", name, old, m.Version)
		}
		st.Profiles[name] = m.Version
	}

	active, err := activeProfile(env.Dir, st)
	if err != nil {
		return err
	}
	if active != "" {
		// The entry points name the active profile and are regenerated from its
		// manifest, so the analytic command appears or disappears with the
		// profile's tracker rather than lingering from a previous install.
		am, err := profile.Load(trunkline.Profiles, path.Join("profiles", active))
		if err != nil {
			return fmt.Errorf("profile %s: %w", active, err)
		}
		if err := writeEntryPoints(w, active, lang, am); err != nil {
			return err
		}
	} else {
		fmt.Fprintf(env.Out, "no `profile:` in %s and several installed — "+
			"entry points left as they are.\n", install.FlagsFile)
	}

	// Not a migration: the wiring is idempotent, so running it on every update
	// is also what carries it into projects installed before it existed.
	if err := w.EnsureSoundHooks(); err != nil {
		return err
	}

	// Same reasoning for the .pipeline note. It is the written answer to "should
	// this be gitignored", and it was previously written by `init` alone — so a
	// project that upgraded across a layout change kept a note describing
	// directories the migration had just removed, which is worse than no note.
	// Written before the migrations run, then again after, because a migration
	// may be what changes the flags it is rendered from.
	if err := ensurePipelineNote(env, w, st); err != nil {
		return err
	}

	for _, mig := range pending {
		msg, err := mig.Apply(env.Dir, *dry)
		if err != nil {
			return fmt.Errorf("migration %s: %w", mig.ID, err)
		}
		if msg != "" {
			fmt.Fprintf(env.Out, "migration %s: %s\n", mig.ID, msg)
		}
		if !*dry {
			st.MarkApplied(mig.ID)
		}
	}
	if len(pending) > 0 {
		if err := ensurePipelineNote(env, w, st); err != nil {
			return err
		}
	}

	st.Core, st.CLI = to, to
	fmt.Fprint(env.Out, w.Summary())
	if *dry {
		fmt.Fprint(env.Out, "(dry run: nothing was written)\n")
	}
	return nil
}

// installPayload vendors the core, the named profile and the referenced
// agent-skills subset. Shared by init, update and `profile add` so the three
// cannot drift apart.
//
// The localization is applied over the English core rather than beside it: a
// translated file replaces its counterpart in place, an untranslated one keeps
// the English text. That way the pipeline resolves one set of paths whatever the
// language, and a half-finished localization degrades to English instead of
// breaking a route.
func installPayload(w *install.Writer, name, lang string) (profile.Manifest, error) {
	var m profile.Manifest

	src := path.Join("profiles", name)
	m, err := profile.Load(trunkline.Profiles, src)
	if err != nil {
		return m, err
	}

	if err := vendorCore(w, lang); err != nil {
		return m, err
	}
	if err := vendor(w, trunkline.Profiles, src, path.Join(install.ProfilesDir, name)); err != nil {
		return m, err
	}
	// The core links to profiles/README.md when it explains where stack-specific
	// rules belong. Vendoring it keeps that link alive in the project instead of
	// pointing at a file only this repository has.
	if b, err := fs.ReadFile(trunkline.Profiles, "profiles/README.md"); err == nil {
		if _, err := w.Write(path.Join(install.ProfilesDir, "README.md"), b, 0o644); err != nil {
			return m, err
		}
	}
	// Publisher contracts travel with the pipeline, not with the profile: the
	// analyst must obey them from inside a project, where the publisher's own
	// repository is not present, and two profiles declaring the same publisher
	// must read one contract rather than a copy each.
	if err := vendor(w, trunkline.Publishers, "publishers", install.PublishersDir); err != nil {
		return m, err
	}
	if err := vendor(w, trunkline.Skills, "agent-skills", install.SkillsDir); err != nil {
		return m, err
	}

	return m, nil
}

// writeEntryPoints regenerates the files an agent starts from. They name the
// active profile, so they are written once per install — for the profile the
// project actually runs, not for every profile that happens to be vendored.
func writeEntryPoints(w *install.Writer, active, lang string, m profile.Manifest) error {
	hasTasks := m.Tasks != nil
	if _, err := w.Write(install.AgentsEntryFile,
		install.AgentsEntry(active, lang, hasTasks), 0o644); err != nil {
		return err
	}
	if _, err := w.Write(install.ClaudeCommand, install.ClaudeCommandFile(active, install.PipelineRoot), 0o644); err != nil {
		return err
	}
	if _, err := w.Write(install.ClaudeQuick, install.ClaudeQuickCommand(active, install.PipelineRoot), 0o644); err != nil {
		return err
	}
	// The analytic entry point exists only where the profile declares a tracker.
	// Writing it regardless would offer a route that stops at the first step for
	// want of anywhere to put the tasks.
	if !hasTasks {
		return nil
	}
	_, err := w.Write(install.ClaudeAnalytic, install.ClaudeAnalyticCommand(active), 0o644)
	return err
}

// resolveLanguage checks the requested language against what actually ships.
// A typo must not silently fall back to English: the whole point of the flag is
// that the language is a stated decision, and a silent fallback would leave the
// project running English prompts while the flags claim otherwise.
// The value is matched against the list of localizations that ship, rather than
// used to build a path. Statting `core/l10n/<requested>` accepted anything that
// happened to resolve — `--language ..` named the l10n directory itself, passed,
// and wrote `- language: ..` into the flags block, where doctor reported it ok.
func resolveLanguage(requested string) (string, error) {
	if requested == "" || requested == install.DefaultLanguage {
		return install.DefaultLanguage, nil
	}
	available, _ := languages()
	if slices.Contains(available, requested) {
		return requested, nil
	}
	return "", fmt.Errorf("no localization %q; available: %s",
		requested, strings.Join(available, ", "))
}

// l10nDrift lists the core files whose English source has changed since the
// localization last recorded its hash — the same comparison
// scripts/lint-l10n.py makes, made again at install time.
//
// The lint runs in this repository; nothing carried the answer into a project.
// A binary built from a tree with a stale translation installs prompts that
// describe steps the English core no longer has, and neither `init` nor `doctor`
// said a word — in a product whose own rule is to announce a degradation out
// loud, that is the wrong silence.
func l10nDrift(lang string) ([]string, error) {
	if lang == "" || lang == install.DefaultLanguage {
		return nil, nil
	}
	b, err := fs.ReadFile(trunkline.Core, path.Join(install.L10nDir, lang, "l10n.lock"))
	if err != nil {
		return nil, err
	}
	var lock struct {
		Sources map[string]string `json:"sources"`
	}
	if err := json.Unmarshal(b, &lock); err != nil {
		return nil, fmt.Errorf("parse %s/%s/l10n.lock: %w", install.L10nDir, lang, err)
	}
	var out []string
	for rel, want := range lock.Sources {
		src, err := fs.ReadFile(trunkline.Core, path.Join("core", rel))
		if err != nil {
			out = append(out, rel) // translated, but the source is gone
			continue
		}
		if fmt.Sprintf("%x", sha256.Sum256(src)) != want {
			out = append(out, rel)
		}
	}
	sort.Strings(out)
	return out, nil
}

// l10nDriftMessage names the drifted files rather than counting them: which
// step is describing an older core decides whether the run can go ahead.
func l10nDriftMessage(lang string, drift []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "\nwarning: the %s localization is behind the English core in %d file(s):\n",
		lang, len(drift))
	for _, rel := range drift {
		fmt.Fprintf(&b, "  %s\n", rel)
	}
	fmt.Fprintf(&b, "Those prompts describe an older version of their step. The English core is\n"+
		"intact: `trunkline init --language %s` runs the original text instead.\n\n",
		install.DefaultLanguage)
	return b.String()
}

// languages lists what can be passed to --language: English plus every
// localization directory that ships.
func languages() ([]string, error) {
	out := []string{install.DefaultLanguage}
	entries, err := fs.ReadDir(trunkline.Core, install.L10nDir)
	if err != nil {
		return out, nil // no localizations shipped at all — English only
	}
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out, nil
}

// vendorCore writes the core in the requested language: the English tree first,
// then the localization over the top of it.
//
// The order matters and so does where Prune sits. Copy, then overlay, then
// prune against the union of both — otherwise a file that exists only in the
// localization looks like something we no longer ship and gets deleted on the
// next update.
//
// core/l10n/ itself is build material, not payload: a project gets one language,
// not every translation of the pipeline.
func vendorCore(w *install.Writer, lang string) error {
	l10nRoot := path.Join(install.L10nDir, lang)
	shipped := map[string]bool{}

	// One pass, one write per file: pick the localized bytes where they exist and
	// the English ones where they do not. Writing English first and overlaying it
	// would touch every localized file twice on every install, and report a
	// no-op update as dozens of changes.
	source := func(rel string) (string, error) {
		if lang == install.DefaultLanguage {
			return path.Join("core", rel), nil
		}
		loc := path.Join(l10nRoot, rel)
		if _, err := fs.Stat(trunkline.Core, loc); err == nil {
			return loc, nil
		}
		return path.Join("core", rel), nil
	}

	write := func(rel string) error {
		src, err := source(rel)
		if err != nil {
			return err
		}
		b, err := fs.ReadFile(trunkline.Core, src)
		if err != nil {
			return err
		}
		mode := os.FileMode(0o644)
		if strings.HasSuffix(rel, ".sh") {
			mode = 0o755
		}
		dst := path.Join(install.CoreDir, rel)
		shipped[dst] = true
		_, err = w.Write(dst, b, mode)
		return err
	}

	// The English tree defines what the payload consists of; a localization only
	// changes the contents of those files, never the set of them.
	err := fs.WalkDir(trunkline.Core, "core", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if strings.HasPrefix(p, install.L10nDir+"/") {
			return nil // build material, not payload
		}
		rel, err := filepath.Rel("core", p)
		if err != nil {
			return err
		}
		return write(filepath.ToSlash(rel))
	})
	if err != nil {
		return err
	}

	return w.Prune(install.CoreDir+"/", shipped)
}

// activeLanguage is the language this project is installed in.
//
// The state file answers first, and the flags block only for a project
// installed before the state recorded it. The order used to be the other way
// round, which made a localized project revert to English in two ordinary
// cases: a flags block that already existed at init (EnsureFlags never rewrites
// one, so `language: ru` never got in), and any read error at all, which the
// parser reported as an empty set of flags. English is the default for an
// unset language, so both looked like a deliberate `en`.
func activeLanguage(dir string, st *state.State) (string, error) {
	if st.Language != "" {
		return resolveLanguage(st.Language)
	}
	flags, err := readFlags(dir)
	if err != nil {
		return "", fmt.Errorf("cannot read the flags block of %s: %w — refusing to guess a "+
			"language, since guessing wrong replaces every prompt in the project", install.FlagsFile, err)
	}
	lang := flags["language"]
	if lang == "" {
		return install.DefaultLanguage, nil
	}
	return resolveLanguage(lang)
}

// activeProfile is the profile the project declares in its flags block. Falls back to
// the sole installed profile, and only to that: with several installed there is
// no safe guess.
func activeProfile(dir string, st *state.State) (string, error) {
	flags, err := readFlags(dir)
	if err != nil {
		return "", fmt.Errorf("cannot read the flags block of %s: %w", install.FlagsFile, err)
	}
	if name := flags["profile"]; name != "" {
		return name, nil
	}
	if len(st.Profiles) == 1 {
		for name := range st.Profiles {
			return name, nil
		}
	}
	return "", nil
}

// vendor copies a subtree and drops files that used to be part of it but are no
// longer shipped.
func vendor(w *install.Writer, fsys fs.FS, src, dst string) error {
	shipped, err := install.Shipped(fsys, src, dst)
	if err != nil {
		return err
	}
	if err := w.CopyTree(fsys, src, dst); err != nil {
		return err
	}
	return w.Prune(dst+"/", shipped)
}

func installGate(w *install.Writer, kind install.GateKind, m profile.Manifest) (bool, error) {
	if m.Hooks.PreCommitGate == "" || kind == install.GateNone {
		return false, nil
	}
	// The manifest resolves its own paths against the profile directory, which
	// in the payload is profiles/<name>/ — the same shape it has in a project.
	script, err := fs.ReadFile(trunkline.Profiles, m.Path(m.Hooks.PreCommitGate))
	if err != nil {
		return false, fmt.Errorf("profile declares hooks.pre_commit_gate %q but it is missing: %w",
			m.Hooks.PreCommitGate, err)
	}
	return w.EnsureGate(kind, m, script)
}

// resolveProfile picks the profile to install. Guessing is only allowed when
// there is exactly one candidate; otherwise the wrong profile would silently
// disable half the cross-checks.
func resolveProfile(requested string) (string, error) {
	names, err := profile.List(trunkline.Profiles)
	if err != nil {
		return "", err
	}
	sort.Strings(names)
	if requested != "" {
		for _, n := range names {
			if n == requested {
				return n, nil
			}
		}
		return "", fmt.Errorf("unknown profile %q; available: %s", requested, strings.Join(names, ", "))
	}
	if len(names) == 1 {
		return names[0], nil
	}
	return "", fmt.Errorf("--profile is required; available: %s", strings.Join(names, ", "))
}

func reportNextSteps(env Env, m profile.Manifest, name string, gated bool, kind install.GateKind) {
	fmt.Fprintf(env.Out, "\nNext:\n")
	fmt.Fprintf(env.Out, "  1. Check the `## Pipeline flags` block in %s — profile: %s.\n",
		install.FlagsFile, name)
	if len(m.RequiredProjectFiles) > 0 {
		var missing []string
		for _, r := range m.RequiredProjectFiles {
			missing = append(missing, r.Path)
		}
		fmt.Fprintf(env.Out, "  2. This profile needs these files in the project: %s.\n", strings.Join(missing, ", "))
	}
	switch {
	case gated && kind == install.GateGit:
		fmt.Fprint(env.Out, "  3. The commit gate is installed as .git/hooks/pre-commit.\n")
	case gated:
		fmt.Fprintf(env.Out, "  3. The commit gate is installed and registered in %s.\n", install.ClaudeSetting)
	case kind == install.GateNone:
		fmt.Fprint(env.Out, "  3. No commit gate (--gate=none): every gate now rests on the model obeying text.\n")
	default:
		fmt.Fprintf(env.Out, "  3. Profile %s ships no pre-commit gate — a known hole in enforcement.\n", name)
	}
	fmt.Fprint(env.Out, "  4. Run `trunkline doctor`, then start a task with /trunkline-dev,\n"+
		"     or /trunkline-quick for a change too small for the pipeline.\n")
}
