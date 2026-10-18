package state

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidRel(t *testing.T) {
	ok := []string{"a.md", "a/b.md", ".trunkline/core/x.md", "./a.md", "a/./b.md"}
	for _, rel := range ok {
		if !ValidRel(rel) {
			t.Errorf("ValidRel(%q) = false, want true", rel)
		}
	}
	bad := []string{
		"", ".", "..", "../x", "a/../../x", "/etc/passwd",
		"a\x00b",  // a NUL truncates the path at the syscall boundary
		`a\..\..`, // backslashes are not a separator here and must not be guessed at
	}
	for _, rel := range bad {
		if ValidRel(rel) {
			t.Errorf("ValidRel(%q) = true, want false", rel)
		}
	}
}

// TestLoadRejectsEscapingPaths: the state file is committed, so it arrives with
// the checkout. A path in it that leaves the project is an instruction to
// delete something outside it, not a typo to tolerate.
func TestLoadRejectsEscapingPaths(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "core: 0.1.0\nfiles:\n  ../../escaped.md: abc\n")
	_, err := Load(dir)
	if err == nil {
		t.Fatal("Load accepted a path outside the project")
	}
	if !strings.Contains(err.Error(), "escaped.md") {
		t.Errorf("the error does not name the offending entry: %v", err)
	}
}

func TestLoadMissingIsNotAnError(t *testing.T) {
	s, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load on a project with no state: %v", err)
	}
	if s == nil || s.Files == nil || s.Profiles == nil {
		t.Error("Load returned a state with nil maps; every caller would panic")
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s := New()
	s.Core, s.CLI, s.Language = "0.1.0", "0.1.0", "ru"
	s.Profiles["p"] = "1.0.0"
	s.Files["a.md"] = Hash([]byte("x"))
	s.MarkApplied("m1")
	if err := s.Save(dir); err != nil {
		t.Fatal(err)
	}

	got, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Core != "0.1.0" || got.Language != "ru" || got.Profiles["p"] != "1.0.0" {
		t.Errorf("round trip lost fields: %+v", got)
	}
	if !got.Applied("m1") {
		t.Error("applied migrations did not survive the round trip")
	}
	if got.UpdatedAt == "" {
		t.Error("updated_at was not stamped")
	}
	// The header is what tells a reader to commit the file.
	b, err := os.ReadFile(filepath.Join(dir, File))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(b), "#") {
		t.Error("the explanatory header is gone")
	}
}

func TestMarkAppliedIsIdempotent(t *testing.T) {
	s := New()
	s.MarkApplied("m1")
	s.MarkApplied("m1")
	if len(s.Migrations) != 1 {
		t.Errorf("migrations = %v, want one entry", s.Migrations)
	}
}

// TestLocallyModified pins the three answers, which point in three directions
// on purpose.
func TestLocallyModified(t *testing.T) {
	dir := t.TempDir()
	s := New()

	t.Run("untracked is not modified", func(t *testing.T) {
		if s.LocallyModified(dir, "unknown.md") {
			t.Error("an untracked path answered this question at all; " +
				"ownership is a different question from change")
		}
	})

	t.Run("matching content is not modified", func(t *testing.T) {
		write(t, dir, "")
		if err := os.WriteFile(filepath.Join(dir, "a.md"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		s.Files["a.md"] = Hash([]byte("x"))
		if s.LocallyModified(dir, "a.md") {
			t.Error("an unchanged file reported as modified")
		}
	})

	t.Run("edited content is modified", func(t *testing.T) {
		if err := os.WriteFile(filepath.Join(dir, "a.md"), []byte("edited"), 0o644); err != nil {
			t.Fatal(err)
		}
		if !s.LocallyModified(dir, "a.md") {
			t.Error("an edited file reported as unchanged")
		}
	})

	t.Run("a deleted file is not modified — writing it back is the repair", func(t *testing.T) {
		s.Files["gone.md"] = Hash([]byte("x"))
		if s.LocallyModified(dir, "gone.md") {
			t.Error("a deleted file reported as modified; it would never be restored")
		}
	})

	t.Run("an unreadable file counts as modified", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root reads everything")
		}
		p := filepath.Join(dir, "secret.md")
		if err := os.WriteFile(p, []byte("x"), 0o000); err != nil {
			t.Fatal(err)
		}
		s.Files["secret.md"] = Hash([]byte("x"))
		if !s.LocallyModified(dir, "secret.md") {
			t.Error("a file that cannot be read reported as unchanged; " +
				"it would be overwritten without ever being compared")
		}
	})
}

func write(t *testing.T, dir, content string) {
	t.Helper()
	if content == "" {
		return
	}
	p := filepath.Join(dir, File)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
