// Package cli implements the trunkline commands. Every command works on a
// project directory (-C, default: the working directory) and is safe to rerun.
package cli

import (
	"flag"
	"fmt"
	"io"
	"os"

	trunkline "github.com/vseminoy/trunkline"
)

// Env is what a command needs: where to write output and which project to act
// on. Kept explicit so commands stay testable.
type Env struct {
	Out io.Writer
	Err io.Writer
	Dir string
}

type command struct {
	name  string
	brief string
	run   func(Env, []string) error
}

var commands = []command{
	{"init", "install the pipeline into a project", runInit},
	{"update", "upgrade an installed pipeline and run migrations", runUpdate},
	{"doctor", "check that the setup is complete and consistent", runDoctor},
	{"profile", "list, show or add stack profiles", runProfile},
	{"version", "print the core version shipped in this binary", runVersion},
	{"trace-check", "step 7a's machine half: did every rule reach every artefact", runTraceCheck},
	{"review-check", "steps 9+10 over HTTP, for a host with no subagents", runReviewCheck},
	{"token-cost", "what a run cost, by token kind and in dollars", runTokenCost},
	{"kb-sync", "step 21's machine half: does a knowledge entry still match the code", runKBSync},
}

// exitCode is an error that also carries the process exit code the command
// wants. Main checks for it before falling back to the uniform exit 1 every
// other command uses. trace-check, kb-sync, review-check and token-cost each
// have a real three-way outcome their Python originals encoded in the exit
// code (0 clean / ran-and-found-something / could-not-run) and the pipeline
// steps that invoke them read that distinction; collapsing it to a plain
// error would lose it.
type exitCode int

func (e exitCode) Error() string { return fmt.Sprintf("exit %d", int(e)) }

// Main is the entry point. It returns a process exit code.
func Main(args []string) int {
	env := Env{Out: os.Stdout, Err: os.Stderr}

	fs := flag.NewFlagSet("trunkline", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.StringVar(&env.Dir, "C", ".", "project directory to act on")
	fs.Usage = func() { usage(os.Stderr) }
	if err := fs.Parse(args); err != nil {
		return 2
	}
	rest := fs.Args()
	if len(rest) == 0 {
		usage(os.Stdout)
		return 0
	}

	for _, c := range commands {
		if c.name != rest[0] {
			continue
		}
		if err := c.run(env, rest[1:]); err != nil {
			if ec, ok := err.(exitCode); ok {
				return int(ec)
			}
			fmt.Fprintf(env.Err, "trunkline %s: %v\n", c.name, err)
			return 1
		}
		return 0
	}
	fmt.Fprintf(env.Err, "trunkline: unknown command %q\n\n", rest[0])
	usage(os.Stderr)
	return 2
}

func usage(w io.Writer) {
	fmt.Fprintf(w, `trunkline %s — a vertical SDD pipeline for brownfield projects

Usage:
  trunkline [-C dir] <command> [flags]

Commands:
`, trunkline.Version())
	for _, c := range commands {
		fmt.Fprintf(w, "  %-13s %s\n", c.name, c.brief)
	}
	fmt.Fprint(w, `
Run "trunkline <command> -h" for the flags of one command.

A typical first setup:
  trunkline init --profile <name>   # vendor the pipeline, write the flags block
  trunkline doctor                  # confirm nothing is missing
`)
}

func runVersion(env Env, _ []string) error {
	fmt.Fprintf(env.Out, "trunkline %s\n", trunkline.Version())
	return nil
}
