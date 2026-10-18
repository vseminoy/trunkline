package cli

import (
	"errors"
	"flag"
	"fmt"
	"path"
	"sort"
	"strings"

	trunkline "github.com/vseminoy/trunkline"
	"github.com/vseminoy/trunkline/internal/install"
	"github.com/vseminoy/trunkline/internal/profile"
	"github.com/vseminoy/trunkline/internal/state"
)

func runProfile(env Env, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: trunkline profile <list|show NAME|add NAME>")
	}
	switch args[0] {
	case "list":
		return profileList(env)
	case "show":
		if len(args) < 2 {
			return fmt.Errorf("usage: trunkline profile show NAME")
		}
		return profileShow(env, args[1])
	case "add":
		return profileAdd(env, args[1:])
	default:
		return fmt.Errorf("unknown subcommand %q; use list, show or add", args[0])
	}
}

func profileList(env Env) error {
	names, err := profile.List(trunkline.Profiles)
	if err != nil {
		return err
	}
	sort.Strings(names)

	st, _ := state.Load(env.Dir)
	fmt.Fprintf(env.Out, "%-24s %-9s %-10s %s\n", "PROFILE", "VERSION", "STATE", "STACK")
	for _, n := range names {
		m, err := profile.Load(trunkline.Profiles, path.Join("profiles", n))
		if err != nil {
			fmt.Fprintf(env.Out, "%-24s %s\n", n, err)
			continue
		}
		installed := "-"
		if v, ok := st.Profiles[n]; ok {
			installed = "installed"
			if v != m.Version {
				installed = "outdated"
			}
		}
		fmt.Fprintf(env.Out, "%-24s %-9s %-10s %s\n", m.Name, m.Version, installed, m.Language)
	}
	fmt.Fprint(env.Out, "\nA profile name starts with the organisation on purpose: guidelines are\n"+
		"somebody's standards. Same stack but your own rules means your own profile —\n"+
		"copying one and editing it \"a little\" surfaces as unexplainable step 7a findings.\n")
	return nil
}

func profileShow(env Env, name string) error {
	m, err := profile.Load(trunkline.Profiles, path.Join("profiles", name))
	if err != nil {
		return err
	}
	fmt.Fprintf(env.Out, "%s %s\n%s\n\n", m.Name, m.Version, strings.TrimSpace(m.Description))
	fmt.Fprintf(env.Out, "language:   %s\n", m.Language)

	var cmds []string
	for k, v := range m.Commands {
		cmds = append(cmds, fmt.Sprintf("  %-18s %s", k, v))
	}
	sort.Strings(cmds)
	fmt.Fprintf(env.Out, "commands:\n%s\n", strings.Join(cmds, "\n"))

	var roles []string
	for r := range m.Guidelines.Roles {
		roles = append(roles, r)
	}
	sort.Strings(roles)
	fmt.Fprintf(env.Out, "guidelines: %s\n", strings.Join(roles, ", "))

	var steps []string
	for s := range m.StepOverlays {
		steps = append(steps, s)
	}
	sort.Strings(steps)
	fmt.Fprintf(env.Out, "overlays:   steps %s\n", strings.Join(steps, ", "))

	gate := m.Hooks.PreCommitGate
	if gate == "" {
		gate = "none (known hole in enforcement)"
	}
	fmt.Fprintf(env.Out, "commit gate: %s\n", gate)

	var required []string
	for _, r := range m.RequiredProjectFiles {
		required = append(required, r.Path)
	}
	fmt.Fprintf(env.Out, "requires:   %s\n", strings.Join(required, ", "))
	return nil
}

func profileAdd(env Env, args []string) (err error) {
	fs_ := flag.NewFlagSet("profile add", flag.ContinueOnError)
	fs_.SetOutput(env.Err)
	var (
		force = fs_.Bool("force", false, "overwrite files that were modified locally")
		dry   = fs_.Bool("dry-run", false, "report what would change, write nothing")
	)
	if err := fs_.Parse(args); err != nil {
		return err
	}
	if fs_.NArg() != 1 {
		return fmt.Errorf("usage: trunkline profile add NAME [--force] [--dry-run]")
	}
	name, err := resolveProfile(fs_.Arg(0))
	if err != nil {
		return err
	}
	if !state.Exists(env.Dir) {
		return fmt.Errorf("no %s here — run `trunkline init` first", state.File)
	}

	st, err := state.Load(env.Dir)
	if err != nil {
		return err
	}
	w := install.NewWriter(env.Dir, st)
	w.DryRun, w.Force = *dry, *force

	// `profile add` never changes the language: it adds a stack, and the project
	// has already decided which language it runs in.
	lang, err := activeLanguage(env.Dir, st)
	if err != nil {
		return err
	}
	// Same reason as init and update: files exist from here on, and the record
	// of who wrote them has to survive a failure partway through.
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
	st.Profiles[name] = m.Version

	fmt.Fprintf(env.Out, "profile %s %s installed\n", m.Name, m.Version)
	fmt.Fprint(env.Out, w.Summary())
	fmt.Fprintf(env.Out, "\nIt is installed, not selected. Set `profile: %s` in the\n"+
		"`## Pipeline flags` block of %s to actually run it.\n", name, install.FlagsFile)
	if *dry {
		fmt.Fprint(env.Out, "\n(dry run: nothing was written)\n")
	}
	return nil
}
