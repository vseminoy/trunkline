package install_test

import (
	"flag"
	"os"
	"testing"

	"github.com/vseminoy/trunkline/internal/install"
)

// dogfoodPath is this repository's own Claude Code slash command. A checkout of
// trunkline is a pipeline root like any other — `core/` and `profiles/` sit at
// its top level — so the command that runs the pipeline here is the shipped
// generator's output with an empty root, not a text of its own.
//
// The name differs from the ClaudeCommand a project installs on purpose: both
// would otherwise be /trunkline-dev, and in a checkout of trunkline the two are
// not the same command — this one runs the pipeline on trunkline itself.
const dogfoodPath = "../../.claude/commands/trunkline-self.md"

// dogfoodQuickPath is the same for the short-change route. Here the name needs
// no suffix: a checkout of trunkline installs nothing into itself, so the only
// /trunkline-quick in it is this one.
const dogfoodQuickPath = "../../.claude/commands/trunkline-quick.md"

var update = flag.Bool("update", false, "rewrite the repository's own entry point from the generator")

// The file this checks used to be hand-written, and it had drifted: it told the
// agent to announce the flags "at step 0" when the core insists the opening
// announcement belongs to classification and step 0 is conditional. A second
// copy of an entry point drifts silently, because nothing reads the two
// together — so the copy is generated and this test is what reads them together.
func TestRepositoryEntryPointMatchesTheGenerator(t *testing.T) {
	want := install.ClaudeCommandFile("", "")
	if *update {
		if err := os.WriteFile(dogfoodPath, want, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	got, err := os.ReadFile(dogfoodPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Errorf("%s has drifted from install.ClaudeCommandFile. It is generated, not "+
			"maintained by hand: run\n\tgo test ./internal/install -run %s -update",
			dogfoodPath, t.Name())
	}
}

// The short-change command is generated from the same generator for the same
// reason: a hand-kept copy of an entry point drifts from the route it names.
func TestRepositoryQuickEntryPointMatchesTheGenerator(t *testing.T) {
	want := install.ClaudeQuickCommand("", "")
	if *update {
		if err := os.WriteFile(dogfoodQuickPath, want, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	got, err := os.ReadFile(dogfoodQuickPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Errorf("%s has drifted from install.ClaudeQuickCommand. It is generated, not "+
			"maintained by hand: run\n\tgo test ./internal/install -run %s -update",
			dogfoodQuickPath, t.Name())
	}
}
