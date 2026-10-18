package atomicfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteCreates(t *testing.T) {
	dir := t.TempDir()
	abs := filepath.Join(dir, "nested", "a.md")
	if err := Write(abs, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(abs)
	if err != nil || string(b) != "x" {
		t.Fatalf("read back %q, %v", b, err)
	}
}

// TestWriteAppliesMode: the mode is the whole point for the gate script, and
// a temporary file is created 0600, so an explicit Chmod is what puts it right.
func TestWriteAppliesMode(t *testing.T) {
	abs := filepath.Join(t.TempDir(), "gate.sh")
	if err := Write(abs, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(abs)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o755 {
		t.Errorf("mode = %s, want 0755 whatever the umask", fi.Mode().Perm())
	}
}

// TestWriteReplacesRatherThanTruncates is the property that makes a half-written
// AGENTS.md impossible: the target is a different inode afterwards, so nothing
// ever observes it mid-write.
func TestWriteReplacesRatherThanTruncates(t *testing.T) {
	abs := filepath.Join(t.TempDir(), "a.md")
	if err := os.WriteFile(abs, []byte("before"), 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(abs)
	if err != nil {
		t.Fatal(err)
	}
	if err := Write(abs, []byte("after"), 0o644); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(abs)
	if err != nil {
		t.Fatal(err)
	}
	if os.SameFile(before, after) {
		t.Error("the file was written in place")
	}
}

func TestWriteRefusesSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	if err := os.WriteFile(target, []byte("precious"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.md")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	err := Write(link, []byte("ours"), 0o644)
	if err == nil {
		t.Fatal("wrote through a symlink")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Errorf("error = %v, want it to name the symlink", err)
	}
	if b, _ := os.ReadFile(target); string(b) != "precious" {
		t.Errorf("target = %q, want it untouched", b)
	}
}

func TestWriteLeavesNoTempFile(t *testing.T) {
	dir := t.TempDir()
	if err := Write(filepath.Join(dir, "a.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// And on a failure too: a symlink target refuses after the temp file exists
	// in some implementations, so the directory must still come out clean.
	link := filepath.Join(dir, "link.md")
	if err := os.Symlink(filepath.Join(dir, "a.md"), link); err == nil {
		_ = Write(link, []byte("y"), 0o644)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".trunkline-") {
			t.Errorf("temporary file %s left behind", e.Name())
		}
	}
}

func TestEnsureExecutable(t *testing.T) {
	t.Run("adds the bit when the caller asked for one", func(t *testing.T) {
		abs := filepath.Join(t.TempDir(), "gate.sh")
		if err := os.WriteFile(abs, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		fixed, err := EnsureExecutable(abs, 0o755)
		if err != nil || !fixed {
			t.Fatalf("EnsureExecutable = %v, %v; want it to report a repair", fixed, err)
		}
		fi, _ := os.Stat(abs)
		if fi.Mode()&0o111 == 0 {
			t.Errorf("mode = %s", fi.Mode())
		}
	})

	t.Run("leaves a non-executable target alone", func(t *testing.T) {
		abs := filepath.Join(t.TempDir(), "a.md")
		if err := os.WriteFile(abs, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		fixed, err := EnsureExecutable(abs, 0o644)
		if err != nil || fixed {
			t.Fatalf("EnsureExecutable = %v, %v; want no change", fixed, err)
		}
		fi, _ := os.Stat(abs)
		if fi.Mode().Perm() != 0o600 {
			t.Errorf("mode = %s, want 0600 left as the user set it", fi.Mode().Perm())
		}
	})

	t.Run("reports nothing to do when the bit is already there", func(t *testing.T) {
		abs := filepath.Join(t.TempDir(), "gate.sh")
		if err := os.WriteFile(abs, []byte("x"), 0o755); err != nil {
			t.Fatal(err)
		}
		if fixed, err := EnsureExecutable(abs, 0o755); err != nil || fixed {
			t.Errorf("EnsureExecutable = %v, %v; want no change", fixed, err)
		}
	})
}
