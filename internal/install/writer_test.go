package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vseminoy/trunkline/internal/state"
)

// newTestWriter gives a Writer over a fresh temporary project.
func newTestWriter(t *testing.T) *Writer {
	t.Helper()
	return NewWriter(t.TempDir(), state.New())
}

func writeOnDisk(t *testing.T, dir, rel, content string, perm os.FileMode) {
	t.Helper()
	abs := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(content), perm); err != nil {
		t.Fatal(err)
	}
}

func readOnDisk(t *testing.T, dir, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// TestWriteOwnership covers the matrix that decides whether a file is replaced.
// The third case is the one that used to be wrong: a file present on disk with
// no record of us writing it was treated as ours and overwritten.
func TestWriteOwnership(t *testing.T) {
	const rel = "a/b.md"

	cases := []struct {
		name    string
		setup   func(t *testing.T, w *Writer)
		force   bool
		want    Status
		content string // what must be on disk afterwards
	}{
		{
			name:    "absent",
			setup:   func(*testing.T, *Writer) {},
			want:    Created,
			content: "new",
		},
		{
			name: "identical and recorded",
			setup: func(t *testing.T, w *Writer) {
				writeOnDisk(t, w.Dir, rel, "new", 0o644)
				w.State.Files[rel] = state.Hash([]byte("new"))
			},
			want:    Unchanged,
			content: "new",
		},
		{
			name: "recorded and edited",
			setup: func(t *testing.T, w *Writer) {
				writeOnDisk(t, w.Dir, rel, "mine", 0o644)
				w.State.Files[rel] = state.Hash([]byte("old"))
			},
			want:    Kept,
			content: "mine",
		},
		{
			name: "present but never recorded",
			setup: func(t *testing.T, w *Writer) {
				writeOnDisk(t, w.Dir, rel, "theirs", 0o644)
			},
			want:    Foreign,
			content: "theirs",
		},
		{
			name: "present but never recorded, with --force",
			setup: func(t *testing.T, w *Writer) {
				writeOnDisk(t, w.Dir, rel, "theirs", 0o644)
			},
			force:   true,
			want:    Updated,
			content: "new",
		},
		{
			name: "recorded and edited, with --force",
			setup: func(t *testing.T, w *Writer) {
				writeOnDisk(t, w.Dir, rel, "mine", 0o644)
				w.State.Files[rel] = state.Hash([]byte("old"))
			},
			force:   true,
			want:    Updated,
			content: "new",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := newTestWriter(t)
			w.Force = c.force
			c.setup(t, w)

			got, err := w.Write(rel, []byte("new"), 0o644)
			if err != nil {
				t.Fatalf("Write: %v", err)
			}
			if got != c.want {
				t.Errorf("status = %q, want %q", got, c.want)
			}
			if on := readOnDisk(t, w.Dir, rel); on != c.content {
				t.Errorf("on disk = %q, want %q", on, c.content)
			}
		})
	}
}

// TestWriteRestoresExecutableBit is the commit gate's failure mode: the content
// matches, so nothing is written, and without an explicit repair the missing
// +x survives every reinstall while init and doctor both report the gate
// installed.
func TestWriteRestoresExecutableBit(t *testing.T) {
	w := newTestWriter(t)
	const rel = ".claude/hooks/pre-commit-gate.sh"
	writeOnDisk(t, w.Dir, rel, "#!/bin/sh\n", 0o644)
	w.State.Files[rel] = state.Hash([]byte("#!/bin/sh\n"))

	got, err := w.Write(rel, []byte("#!/bin/sh\n"), 0o755)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if got != Updated {
		t.Errorf("status = %q, want %q — a mode repair is a change and must show in the summary", got, Updated)
	}
	fi, err := os.Stat(filepath.Join(w.Dir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode()&0o111 == 0 {
		t.Errorf("mode = %s, want the executable bit restored", fi.Mode())
	}
}

// TestWriteDoesNotTightenMode: only the executable bit is restored. A user who
// deliberately tightened permissions must not have it undone on every run.
func TestWriteDoesNotTightenMode(t *testing.T) {
	w := newTestWriter(t)
	const rel = "core/x.md"
	writeOnDisk(t, w.Dir, rel, "same", 0o600)
	w.State.Files[rel] = state.Hash([]byte("same"))

	if got, err := w.Write(rel, []byte("same"), 0o644); err != nil || got != Unchanged {
		t.Fatalf("Write = %q, %v; want unchanged", got, err)
	}
	fi, err := os.Stat(filepath.Join(w.Dir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("mode = %s, want 0600 left alone", fi.Mode().Perm())
	}
}

// TestWriteRefusesSymlink: a checkout where a target is a link must not have
// the link's target rewritten. Two layers stop it, and both are checked —
// without --force the file is simply not ours, and with --force, which is the
// user overriding exactly that, the write itself refuses.
func TestWriteRefusesSymlink(t *testing.T) {
	for _, force := range []bool{false, true} {
		w := newTestWriter(t)
		w.Force = force
		outside := filepath.Join(t.TempDir(), "outside.txt")
		if err := os.WriteFile(outside, []byte("precious"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, filepath.Join(w.Dir, "AGENTS.md")); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}

		status, err := w.Write("AGENTS.md", []byte("ours"), 0o644)
		switch {
		case force && err == nil:
			t.Error("--force wrote through a symlink; the write must refuse")
		case !force && (err != nil || status != Foreign):
			t.Errorf("without --force: status %q, err %v; want %q", status, err, Foreign)
		}
		if b, _ := os.ReadFile(outside); string(b) != "precious" {
			t.Errorf("force=%v: link target = %q, want it untouched", force, b)
		}
	}
}

// TestWriteRefusesEscapingPath and TestPruneRefusesEscapingPath cover the same
// hole from both ends: state.yaml is committed, so its paths arrive with the
// checkout and are not trusted input.
func TestWriteRefusesEscapingPath(t *testing.T) {
	w := newTestWriter(t)
	for _, rel := range []string{"../escaped.md", "a/../../escaped.md", "/etc/passwd"} {
		if _, err := w.Write(rel, []byte("x"), 0o644); err == nil {
			t.Errorf("Write(%q) succeeded; it must refuse", rel)
		}
	}
}

func TestPruneRefusesEscapingPath(t *testing.T) {
	w := newTestWriter(t)
	// The shape a tampered state file takes: an entry that carries the pruned
	// prefix — so Prune picks it up — and climbs back out of the project.
	const rel = CoreDir + "/../../../victim.md"
	victim := filepath.Join(w.Dir, "..", "victim.md")
	if err := os.WriteFile(victim, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(victim) })
	w.State.Files[rel] = "deadbeef"

	err := w.Prune(CoreDir+"/", map[string]bool{})
	if err == nil {
		t.Fatal("Prune followed a path out of the project; it must refuse")
	}
	if !strings.Contains(err.Error(), "refusing") {
		t.Errorf("error = %v, want it to say what it refused", err)
	}
	if _, statErr := os.Stat(victim); statErr != nil {
		t.Errorf("the file outside the project was removed: %v", statErr)
	}
}

// TestPruneKeepsEdited: a stale file the user has edited is kept, not deleted.
func TestPruneKeepsEdited(t *testing.T) {
	w := newTestWriter(t)
	const rel = CoreDir + "/steps/gone.md"
	writeOnDisk(t, w.Dir, rel, "edited", 0o644)
	w.State.Files[rel] = state.Hash([]byte("shipped"))

	if err := w.Prune(CoreDir+"/", map[string]bool{}); err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if readOnDisk(t, w.Dir, rel) != "edited" {
		t.Error("an edited file was pruned")
	}
}

// TestWriteIsAtomic: no partially written file is ever visible. Checked through
// the property that makes it true — the target is replaced by a rename, so its
// inode changes and the old content is never truncated in place.
func TestWriteIsAtomic(t *testing.T) {
	w := newTestWriter(t)
	const rel = "AGENTS.md"
	writeOnDisk(t, w.Dir, rel, "before", 0o644)
	w.State.Files[rel] = state.Hash([]byte("before"))
	abs := filepath.Join(w.Dir, rel)

	before, err := os.Stat(abs)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(rel, []byte("after"), 0o644); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(abs)
	if err != nil {
		t.Fatal(err)
	}
	if os.SameFile(before, after) {
		t.Error("the file was written in place; a crash mid-write would truncate it")
	}
	if readOnDisk(t, w.Dir, rel) != "after" {
		t.Error("content not replaced")
	}
	// The temporary file must not survive.
	entries, err := os.ReadDir(w.Dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".trunkline-") {
			t.Errorf("temporary file %s left behind", e.Name())
		}
	}
}

// TestWriteRefusesUnreadable: unreadable is not absent. Treating it as absent
// is how a file we could not look at was overwritten.
func TestWriteRefusesUnreadable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads everything")
	}
	w := newTestWriter(t)
	const rel = "secret.md"
	writeOnDisk(t, w.Dir, rel, "secret", 0o000)

	if _, err := w.Write(rel, []byte("ours"), 0o644); err == nil {
		t.Fatal("Write over an unreadable file succeeded; it must refuse")
	}
}

// TestDryRunWritesNothing guards the flag every other test would silently
// bypass: --dry-run must not touch the filesystem.
func TestDryRunWritesNothing(t *testing.T) {
	w := newTestWriter(t)
	w.DryRun = true
	if _, err := w.Write("a/b.md", []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(w.Dir, "a", "b.md")); !os.IsNotExist(err) {
		t.Error("dry run created a file")
	}
}
