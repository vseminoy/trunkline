package profile

import (
	"strings"
	"testing"
	"testing/fstest"
)

// manifest builds a minimal valid profile.yaml with extra lines appended, so
// each case names only what it is testing.
func manifest(extra string) string {
	return "name: p\nversion: 1.0.0\n" + extra
}

func load(t *testing.T, yaml string) (Manifest, error) {
	t.Helper()
	fsys := fstest.MapFS{"profiles/p/profile.yaml": {Data: []byte(yaml)}}
	return Load(fsys, "profiles/p")
}

func TestLoadRequiresNameAndVersion(t *testing.T) {
	if _, err := load(t, "version: 1.0.0\n"); err == nil {
		t.Error("a manifest with no name was accepted")
	}
	if _, err := load(t, "name: p\n"); err == nil {
		t.Error("a manifest with no version was accepted")
	}
}

// TestLoadRequiresNameToMatchDirectory: a profile is a movable pack. A manifest
// whose name disagrees with its directory has doctor checking one profile while
// the agent reads another.
func TestLoadRequiresNameToMatchDirectory(t *testing.T) {
	fsys := fstest.MapFS{"profiles/p/profile.yaml": {Data: []byte("name: other\nversion: 1.0.0\n")}}
	if _, err := Load(fsys, "profiles/p"); err == nil {
		t.Error("a manifest naming a different profile than its directory was accepted")
	}
}

// TestValidatePathsRejects covers every shape that cannot mean what it says.
// The `profiles/<name>/` case is the interesting one: it used to be the
// documented form, and it fails looking like a missing file rather than a
// wrong anchor — so the message has to say which it is.
func TestValidatePathsRejects(t *testing.T) {
	cases := map[string]string{
		"absolute guidelines.root":           "guidelines:\n  root: /etc\n",
		"guidelines.root climbs out":         "guidelines:\n  root: ../other\n",
		"role file climbs out":               "guidelines:\n  roles:\n    dev: ../../x.md\n",
		"role file with an interior ..":      "guidelines:\n  roles:\n    dev: a/../../x.md\n",
		"legacy repository anchor":           "guidelines:\n  root: profiles/p/guidelines\n",
		"step overlay climbs out":            "step_overlays:\n  \"08\": ../x.md\n",
		"absolute hook":                      "hooks:\n  pre_commit_gate: /usr/bin/evil\n",
		"tasks.format_spec is absolute":      "tasks:\n  format_spec: /etc/passwd\n  dir: t/\n",
		"tasks.dir points into the pipeline": "tasks:\n  format_spec: f.md\n  dir: core/steps\n",
		"tasks.dir absolute":                 "tasks:\n  format_spec: f.md\n  dir: /srv/analysis\n",
		"contract outside publishers/":       "tasks:\n  format_spec: f.md\n  publisher:\n    contract: ../x.md\n",
	}
	for name, extra := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := load(t, manifest(extra)); err == nil {
				t.Errorf("accepted:\n%s", extra)
			}
		})
	}
}

func TestValidatePathsAccepts(t *testing.T) {
	yaml := manifest(`guidelines:
  root: guidelines
  index: guidelines/INDEX.md
  roles:
    dev: dev.md
step_overlays:
  "08": overlays/08.md
hooks:
  pre_commit_gate: hooks/gate.sh
tasks:
  format_spec: docs/format.md
  dir: .pipeline/tasks
  analysis_dir: .pipeline/tasks/_analysis
  publisher:
    name: jira
    setup: docs/setup.md
    contract: publishers/jira/contract.md
`)
	m, err := load(t, yaml)
	if err != nil {
		t.Fatalf("a well-formed manifest was rejected: %v", err)
	}

	// Path anchors at the profile directory; RolePath goes through
	// guidelines.root, which is the distinction that used to be resolved three
	// different ways in three different callers.
	if got, want := m.Path("hooks/gate.sh"), "profiles/p/hooks/gate.sh"; got != want {
		t.Errorf("Path = %q, want %q", got, want)
	}
	if got, want := m.RolePath("dev"), "profiles/p/guidelines/dev.md"; got != want {
		t.Errorf("RolePath = %q, want %q", got, want)
	}
	if m.RolePath("nosuchrole") != "" {
		t.Errorf("RolePath for an unknown role = %q, want empty", m.RolePath("nosuchrole"))
	}
	if m.Path("") != "" {
		t.Error("Path(\"\") must stay empty, or an unset slot resolves to the profile directory")
	}
	// The contract is the one slot anchored at the pipeline root rather than at
	// the profile, because the publisher belongs to no profile.
	if c := m.Tasks.Publisher.Contract; !strings.HasPrefix(c, "publishers/") {
		t.Errorf("contract = %q", c)
	}
}

func TestList(t *testing.T) {
	fsys := fstest.MapFS{
		"profiles/a/profile.yaml": {Data: []byte(manifest(""))},
		"profiles/b/profile.yaml": {Data: []byte("name: b\nversion: 1.0.0\n")},
		// A directory with no manifest is not a profile.
		"profiles/notes/README.md": {Data: []byte("x")},
		// Nor is a stray file.
		"profiles/README.md": {Data: []byte("x")},
	}
	names, err := List(fsys)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 2 {
		t.Fatalf("List = %v, want two profiles", names)
	}
}

// TestTasksDirMayLeaveTheProject: the analysis often lives in its own
// repository — a sibling checkout or a submodule — so that the churn of the
// analyst's state stays out of the code history. The manifest must allow it;
// what git cannot vouch for, `trunkline doctor` checks instead.
func TestTasksDirMayLeaveTheProject(t *testing.T) {
	m, err := load(t, manifest("tasks:\n  format_spec: docs/format.md\n"+
		"  dir: ../analysis/tasks/\n  analysis_dir: ../analysis/tasks/_analysis/\n"))
	if err != nil {
		t.Fatalf("a sibling analysis checkout must be allowed: %v", err)
	}
	if m.Tasks.Dir != "../analysis/tasks/" {
		t.Errorf("tasks.dir = %q", m.Tasks.Dir)
	}
}
