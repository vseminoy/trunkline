package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runKB(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out, errOut bytes.Buffer
	env := Env{Out: &out, Err: &errOut, Dir: "."}
	err := runKBSync(env, args)
	return out.String() + errOut.String(), err
}

func kbFixture(t *testing.T) (repo string, entry string) {
	t.Helper()
	repo = t.TempDir()
	mustMkdirAll(t, filepath.Join(repo, "src/payments"))
	mustMkdirAll(t, filepath.Join(repo, ".pipeline/knowledge/proj/modules"))
	mustWrite(t, filepath.Join(repo, "src/payments/service.go"),
		"package payments\n\n// R-004-2: amount positive\nfunc CreatePayment(amount int) error { return nil }\n")
	mustWrite(t, filepath.Join(repo, "src/payments/handler.go"), "package payments\n\nfunc Charge() {}\n")
	mustWrite(t, filepath.Join(repo, ".pipeline/workdir/rules/007.md"),
		"| ID | Rule | Origin | Changeset |\n|--|--|--|--|\n| R-004-2 | amount > 0 | development | |\n")
	mustWrite(t, filepath.Join(repo, ".pipeline/knowledge/INDEX.md"),
		"| Project | Module | File |\n|---|---|---|\n| proj | src/payments | proj/modules/payments.md |\n")
	entry = filepath.Join(repo, ".pipeline/knowledge/proj/modules/payments.md")
	return repo, entry
}

func writeEntry(t *testing.T, entry, anchor string) {
	t.Helper()
	mustWrite(t, entry, "# Module: src/payments\n\nUpdated: 2099-01-01\n\n"+
		"## Layers and key types\n"+
		"- service: `src/payments/service.go` — `CreatePayment`\n"+
		"- handler: `src/payments/handler.go` — `Charge`\n\n"+
		"## Business rules in force\n"+
		"| ID | Statement | Where it is enforced | Slice |\n|---|---|---|---|\n"+
		"| R-004-2 | amount > 0 | "+anchor+" | - |\n")
}

func mustMkdirAll(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, p, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestKBSyncClean(t *testing.T) {
	repo, entry := kbFixture(t)
	writeEntry(t, entry, "`src/payments/service.go:CreatePayment`")
	out, err := runKB(t, "--repo", repo, "--module", "src/payments", "--project", "proj", "--ext", ".go")
	if err != nil {
		t.Fatalf("want exit 0, got %v\n%s", err, out)
	}
	if !strings.Contains(out, "ANCHORS CLEAN") {
		t.Errorf("want ANCHORS CLEAN, got:\n%s", out)
	}
}

func TestKBSyncDeadAnchor(t *testing.T) {
	repo, entry := kbFixture(t)
	writeEntry(t, entry, "`src/payments/gone.go:CreatePayment`")
	out, err := runKB(t, "--repo", repo, "--module", "src/payments", "--project", "proj", "--ext", ".go")
	if ec, ok := err.(exitCode); !ok || ec != 1 {
		t.Fatalf("want exit 1, got %v", err)
	}
	if !strings.Contains(out, "DEAD") {
		t.Errorf("want DEAD finding, got:\n%s", out)
	}
}

func TestKBSyncStaleAnchor(t *testing.T) {
	repo, entry := kbFixture(t)
	writeEntry(t, entry, "`src/payments/service.go:NoSuchFunc`")
	out, err := runKB(t, "--repo", repo, "--module", "src/payments", "--project", "proj", "--ext", ".go")
	if ec, ok := err.(exitCode); !ok || ec != 1 {
		t.Fatalf("want exit 1, got %v", err)
	}
	if !strings.Contains(out, "STALE") {
		t.Errorf("want STALE finding, got:\n%s", out)
	}
}

func TestKBSyncMissingEntryIsFirstDescription(t *testing.T) {
	repo := t.TempDir()
	mustMkdirAll(t, filepath.Join(repo, "src/payments"))
	mustMkdirAll(t, filepath.Join(repo, ".pipeline/knowledge/proj"))
	out, err := runKB(t, "--repo", repo, "--module", "src/payments")
	if err != nil {
		t.Fatalf("missing entry should be exit 0 (first description), got %v\n%s", err, out)
	}
	if !strings.Contains(out, "FIRST description") {
		t.Errorf("want FIRST description note, got:\n%s", out)
	}
}

func TestKBSyncModuleNotADirectory(t *testing.T) {
	repo := t.TempDir()
	mustMkdirAll(t, filepath.Join(repo, ".pipeline/knowledge/proj"))
	_, err := runKB(t, "--repo", repo, "--module", "src/nope")
	if ec, ok := err.(exitCode); !ok || ec != 2 {
		t.Fatalf("want exit 2, got %v", err)
	}
}
