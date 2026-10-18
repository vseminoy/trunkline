package migrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vseminoy/trunkline/internal/install"

	"github.com/vseminoy/trunkline/internal/semver"
	"github.com/vseminoy/trunkline/internal/state"
)

func ids(ms []Migration) []string {
	out := make([]string, 0, len(ms))
	for _, m := range ms {
		out = append(out, m.ID)
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestPendingBounds pins both edges of the window. Introduced is the version
// that *requires* the migration, so a project already at that version does not
// need it, and a binary that predates it must not run it.
func TestPendingBounds(t *testing.T) {
	// The set a binary at 0.1.0 could run: later migrations must stay out of it,
	// which is the upper edge this test exists to pin.
	var upTo010 []string
	for _, m := range All {
		if !semver.Less("0.1.0", m.Introduced) {
			upTo010 = append(upTo010, m.ID)
		}
	}
	all := ids(All)

	cases := []struct {
		name     string
		from, to string
		applied  []string
		want     []string
	}{
		{
			name: "a project predating versioning gets everything the binary knows",
			from: "", to: "0.1.0",
			want: upTo010,
		},
		{
			name: "an unparseable installed version is treated as older than everything",
			from: "garbage", to: "0.1.0",
			want: upTo010,
		},
		{
			name: "a project already at the introducing version needs nothing",
			from: "0.1.0", to: "0.1.0",
			want: nil,
		},
		{
			name: "a project past 0.1.0 skips those but still gets 0.3.0",
			from: "0.2.0", to: "0.3.0",
			want: []string{"0.3.0-workdir"},
		},
		{
			name: "a project already at 0.3.0 needs nothing",
			from: "0.3.0", to: "0.3.0",
			want: nil,
		},
		{
			name: "a binary that predates the migration must not run it",
			from: "0.0.1", to: "0.0.9",
			want: nil,
		},
		{
			name: "already applied ids are never repeated",
			from: "", to: "0.3.0",
			applied: all,
			want:    nil,
		},
		{
			name: "a downgrade runs nothing",
			from: "0.2.0", to: "0.1.0",
			want: nil,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			st := state.New()
			for _, id := range c.applied {
				st.MarkApplied(id)
			}
			if got := ids(Pending(st, c.from, c.to)); !equal(got, c.want) {
				t.Errorf("Pending(%q -> %q) = %v, want %v", c.from, c.to, got, c.want)
			}
		})
	}
}

// TestMigrationsAreNoOpsOnACleanProject: every migration must be safe to run
// against a project that already looks migrated. Applied to a fresh directory,
// each one must change nothing and say nothing.
func TestMigrationsAreNoOpsOnACleanProject(t *testing.T) {
	for _, m := range All {
		dir := t.TempDir()
		msg, err := m.Apply(dir, false)
		if err != nil {
			t.Errorf("%s: %v", m.ID, err)
		}
		if msg != "" {
			t.Errorf("%s reported %q on a clean project", m.ID, msg)
		}
	}
}

func TestDropOldCommand(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, ".claude", "commands", "pipeline-dev.md")
	if err := os.MkdirAll(filepath.Dir(old), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(old, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := dropOldCommand(dir, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(old); err != nil {
		t.Fatal("dry run removed the file")
	}
	if _, err := dropOldCommand(dir, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Error("the superseded command file is still there")
	}
}

// TestMoveWorkdir: the migration has to move run state without touching the
// knowledge base, and has to refuse rather than merge when both layouts exist —
// a silent merge of two rule registries is unrecoverable by hand.
func TestMoveWorkdir(t *testing.T) {
	setup := func(t *testing.T) string {
		dir := t.TempDir()
		for _, d := range []string{".pipeline/rules", ".pipeline/review", ".pipeline/metrics",
			".pipeline/knowledge/slices", ".pipeline/knowledge/modules", "docs/adr"} {
			if err := os.MkdirAll(filepath.Join(dir, filepath.FromSlash(d)), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		write := func(rel, body string) {
			if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(rel)), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		write(".pipeline/rules/007.md", "registry")
		write(".pipeline/knowledge/slices/260813-x.md", "slice")
		write("docs/adr/ADR-001-x.md", "adr")
		return dir
	}

	t.Run("moves run state and ADRs, leaves the base alone", func(t *testing.T) {
		dir := setup(t)
		msg, err := moveWorkdir(dir, false)
		if err != nil {
			t.Fatal(err)
		}
		if msg == "" {
			t.Fatal("moveWorkdir said nothing about a project it changed")
		}
		// The base nests under the project directory, whose default is the name
		// of the directory the project lives in — t.TempDir() here.
		proj := install.ProjectSlug(filepath.Base(dir))
		for _, rel := range []string{
			".pipeline/workdir/rules/007.md",
			".pipeline/knowledge/" + proj + "/adr/ADR-001-x.md",
			".pipeline/knowledge/" + proj + "/slices/260813-x.md",
		} {
			if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); err != nil {
				t.Errorf("%s: %v", rel, err)
			}
		}
		if _, err := os.Stat(filepath.Join(dir, ".pipeline", "rules")); err == nil {
			t.Error(".pipeline/rules/ still there after the move")
		}
		if _, err := os.Stat(filepath.Join(dir, ".pipeline", "knowledge", "slices")); err == nil {
			t.Error("the base is still flat under the root after the move")
		}
	})

	// TestMoveWorkdir's fixture has no INDEX.md, so the index conversion gets its
	// own: moving the files without re-rooting the rows leaves an index pointing
	// one directory too high, and kb-sync reads the index to find an entry.
	t.Run("re-roots INDEX.md and adds the Project column", func(t *testing.T) {
		dir := setup(t)
		proj := install.ProjectSlug(filepath.Base(dir))
		if err := os.WriteFile(filepath.Join(dir, ".pipeline", "knowledge", "INDEX.md"), []byte(
			"# Code knowledge index\n\n"+
				"| Module | File | Last slice | Date |\n"+
				"|---|---|---|---|\n"+
				"| src/payments | modules/payments.md | 260813-x | 2026-08-13 |\n\n"+
				"## Slices\n"+
				"| Slice | Run | Task | Title | Type | Date | Modules |\n"+
				"|---|---|---|---|---|---|---|\n"+
				"| 260813-x | 260813142530 | PROJ-855 | t | feature | 2026-08-13 | src/payments |\n"),
			0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := moveWorkdir(dir, false); err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(filepath.Join(dir, ".pipeline", "knowledge", "INDEX.md"))
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{
			"| Project | Module | File | Last slice | Date |",
			"| " + proj + " | src/payments | " + proj + "/modules/payments.md |",
			"| Project | Slice | Run | Task | Title | Type | Date | Modules |",
			"| " + proj + " | 260813-x | 260813142530 |",
		} {
			if !strings.Contains(string(got), want) {
				t.Errorf("INDEX.md is missing %q:\n%s", want, got)
			}
		}

		// Rerunning must not stack a second Project column on top of the first.
		before := string(got)
		if _, err := addProjectColumn(dir, ".pipeline/knowledge", proj, false); err != nil {
			t.Fatal(err)
		}
		after, err := os.ReadFile(filepath.Join(dir, ".pipeline", "knowledge", "INDEX.md"))
		if err != nil {
			t.Fatal(err)
		}
		if string(after) != before {
			t.Errorf("a second pass changed the index:\n%s", after)
		}
	})

	// An absolute `knowledge:` — a company base several services share — must be
	// written to where it actually is. Joining it onto the project reproduced the
	// whole path inside the repository and left the real base empty.
	t.Run("an absolute knowledge path is used as it stands", func(t *testing.T) {
		dir := setup(t)
		base := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"),
			[]byte("## Pipeline flags\n- knowledge: "+base+"  # shared\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := moveWorkdir(dir, false); err != nil {
			t.Fatal(err)
		}
		proj := install.ProjectSlug(filepath.Base(dir))
		if _, err := os.Stat(filepath.Join(base, proj, "adr", "ADR-001-x.md")); err != nil {
			t.Errorf("the ADR did not reach the shared base: %v", err)
		}
		// Nothing may have been fabricated under the project from the absolute path.
		if _, err := os.Stat(filepath.Join(dir, strings.TrimPrefix(base, string(filepath.Separator)))); err == nil {
			t.Error("the absolute base path was reproduced inside the project")
		}
	})

	t.Run("a dry run changes nothing", func(t *testing.T) {
		dir := setup(t)
		if _, err := moveWorkdir(dir, true); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(dir, ".pipeline", "workdir")); err == nil {
			t.Error("a dry run created .pipeline/workdir/")
		}
	})

	t.Run("refuses when both layouts hold files", func(t *testing.T) {
		dir := setup(t)
		dst := filepath.Join(dir, ".pipeline", "workdir", "rules")
		if err := os.MkdirAll(dst, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dst, "008.md"), []byte("other"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := moveWorkdir(dir, false); err == nil {
			t.Error("moveWorkdir merged two rule registries instead of refusing")
		}
	})

	// A newer `init` creates the new directories before the migration runs, so a
	// half-set-up project has an EMPTY directory at every destination. Refusing
	// there sent the user to merge two directories of which one held nothing.
	t.Run("an empty destination is not a conflict", func(t *testing.T) {
		dir := setup(t)
		for _, d := range []string{"rules", "review", "metrics"} {
			if err := os.MkdirAll(filepath.Join(dir, ".pipeline", "workdir", d), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := moveWorkdir(dir, false); err != nil {
			t.Fatalf("an empty destination was treated as a conflict: %v", err)
		}
		if _, err := os.Stat(filepath.Join(dir, ".pipeline", "workdir", "rules", "007.md")); err != nil {
			t.Errorf("the registry did not arrive: %v", err)
		}
	})

	t.Run("retargets the metrics flag but leaves none alone", func(t *testing.T) {
		for flag, want := range map[string]string{
			"- metrics: .pipeline/metrics/pipeline.jsonl  # journal": ".pipeline/workdir/metrics/pipeline.jsonl",
			"- metrics: none  # journal":                             "none",
			"- metrics: ops/journal.jsonl  # journal":                "ops/journal.jsonl",
		} {
			dir := setup(t)
			if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"),
				[]byte("## Pipeline flags\n"+flag+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := moveWorkdir(dir, false); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(got), "- project: ") {
				t.Errorf("flag %q: no project flag was added", flag)
			}
			if !strings.Contains(string(got), "- metrics: "+want) {
				t.Errorf("flag %q became %q, want it to name %s", flag, got, want)
			}
		}
	})

	t.Run("honours knowledge: none", func(t *testing.T) {
		dir := setup(t)
		if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"),
			[]byte("## Pipeline flags\n- knowledge: none  # not kept\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := moveWorkdir(dir, false); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(dir, "docs", "adr", "ADR-001-x.md")); err != nil {
			t.Error("ADRs were moved into a base the project does not keep")
		}
	})
}
