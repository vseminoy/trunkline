package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runTC(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out, errOut bytes.Buffer
	env := Env{Out: &out, Err: &errOut, Dir: "."}
	err := runTraceCheck(env, args)
	return out.String() + errOut.String(), err
}

func writeTCFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestTraceCheckGapAndOrphan(t *testing.T) {
	dir := t.TempDir()
	rules := filepath.Join(dir, "rules.md")
	writeTCFile(t, rules, "| ID | Rule | Origin | Changeset |\n|--|--|--|--|\n"+
		"| R-007-1 | amount > 0 | analysis | 1 |\n"+
		"| R-007-2 | currency valid | analysis | 1 |\n")
	contract := filepath.Join(dir, "contract.md")
	writeTCFile(t, contract, "| ID |\n| R-007-1 |\n| R-007-9 |\n") // R-007-2 missing -> GAP, R-007-9 -> ORPHAN
	tests := filepath.Join(dir, "tests.md")
	writeTCFile(t, tests, "| ID |\n| R-007-1 |\n| R-007-2 |\n")

	out, err := runTC(t, "--rules", rules,
		"--column", "contract="+contract, "--column", "tests="+tests, "--changeset", "1")
	if ec, ok := err.(exitCode); !ok || ec != 1 {
		t.Fatalf("want exit 1, got %v", err)
	}
	if !strings.Contains(out, "GAP      R-007-2 does not appear in column `contract`") {
		t.Errorf("missing GAP line, got:\n%s", out)
	}
	if !strings.Contains(out, "ORPHAN   R-007-9 appears in `contract`") {
		t.Errorf("missing ORPHAN line, got:\n%s", out)
	}
	if !strings.Contains(out, "FAILED") {
		t.Errorf("expected FAILED verdict, got:\n%s", out)
	}
}

func TestTraceCheckNAAndPartial(t *testing.T) {
	dir := t.TempDir()
	rules := filepath.Join(dir, "rules.md")
	writeTCFile(t, rules, "| ID | Rule | Origin | Changeset |\n|--|--|--|--|\n"+
		"| R-007-1 | amount > 0 | analysis | 1 |\n"+
		"| R-007-2 | currency valid | analysis | 1 |\n")
	contract := filepath.Join(dir, "contract.md")
	writeTCFile(t, contract, "| ID |\n| R-007-1 |\n| R-007-2 |\n")
	tests := filepath.Join(dir, "tests.md")
	writeTCFile(t, tests, "| ID |\n| R-007-1 |\n| R-007-2 |\n")
	schema := filepath.Join(dir, "schema.sql")
	writeTCFile(t, schema, "-- R-007-1 only\n")

	// Without --na, R-007-9 absent from registry entirely -- use schema as
	// --partial so R-007-2 missing a schema constraint is not a gap.
	out, err := runTC(t, "--rules", rules,
		"--column", "contract="+contract, "--column", "tests="+tests,
		"--partial", "schema="+schema, "--changeset", "1")
	if err != nil {
		t.Fatalf("want exit 0 with partial column, got %v\n%s", err, out)
	}
	if !strings.Contains(out, "PASSED") {
		t.Errorf("want PASSED, got:\n%s", out)
	}

	// Same schema as a full --column makes R-007-2 a gap.
	out2, err2 := runTC(t, "--rules", rules,
		"--column", "contract="+contract, "--column", "tests="+tests,
		"--column", "schema="+schema, "--changeset", "1")
	if ec, ok := err2.(exitCode); !ok || ec != 1 {
		t.Fatalf("want exit 1 with schema as full column, got %v\n%s", err2, out2)
	}

	// --na exempts the schema cell for R-007-2 and restores a pass.
	out3, err3 := runTC(t, "--rules", rules,
		"--column", "contract="+contract, "--column", "tests="+tests,
		"--column", "schema="+schema, "--changeset", "1", "--na", "R-007-2=schema")
	if err3 != nil {
		t.Fatalf("want exit 0 with --na, got %v\n%s", err3, out3)
	}
	if !strings.Contains(out3, "N/A      R-007-2 declared not applicable to column `schema`") {
		t.Errorf("missing N/A line, got:\n%s", out3)
	}
}

func TestTraceCheckChangesetNarrowing(t *testing.T) {
	dir := t.TempDir()
	rules := filepath.Join(dir, "rules.md")
	writeTCFile(t, rules, "| ID | Rule | Origin | Changeset |\n|--|--|--|--|\n"+
		"| R-007-1 | amount > 0 | analysis | 1 |\n"+
		"| R-007-2 | name length | analysis | 2 |\n")
	contract := filepath.Join(dir, "contract.md")
	writeTCFile(t, contract, "| ID |\n| R-007-1 |\n")
	tests := filepath.Join(dir, "tests.md")
	writeTCFile(t, tests, "| ID |\n| R-007-1 |\n")

	// Changeset 1 only needs R-007-1, which both columns cover -- PASSED.
	_, err := runTC(t, "--rules", rules,
		"--column", "contract="+contract, "--column", "tests="+tests, "--changeset", "1")
	if err != nil {
		t.Fatalf("changeset 1 should pass, got %v", err)
	}

	// Without --changeset, R-007-2 (changeset 2's rule) is a gap on these
	// columns that only carry changeset 1's artefacts.
	out, err := runTC(t, "--rules", rules,
		"--column", "contract="+contract, "--column", "tests="+tests)
	if ec, ok := err.(exitCode); !ok || ec != 1 {
		t.Fatalf("want exit 1 without changeset filter, got %v\n%s", err, out)
	}
}

func TestTraceCheckRequiresRules(t *testing.T) {
	_, err := runTC(t, "--column", "a=x", "--column", "b=y")
	if ec, ok := err.(exitCode); !ok || ec != 2 {
		t.Fatalf("want exit 2 for missing --rules, got %v", err)
	}
}
