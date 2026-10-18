package cli

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func runTokenCostCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out, errOut bytes.Buffer
	env := Env{Out: &out, Err: &errOut, Dir: "."}
	err := runTokenCost(env, args)
	return out.String() + errOut.String(), err
}

// tcCallLine writes one synthetic Claude Code transcript record — the shape
// read_calls (tcReadCalls) deduplicates by message.id.
func tcCallLine(msgID, model string, inputTok, outputTok int) string {
	return fmt.Sprintf(`{"type":"assistant","timestamp":"2026-08-01T10:00:00Z","cwd":"/x",`+
		`"message":{"id":%q,"model":%q,"usage":{"input_tokens":%d,"output_tokens":%d,`+
		`"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}}`, msgID, model, inputTok, outputTok)
}

func TestTokenCostDedupByMessageID(t *testing.T) {
	dir := t.TempDir()
	tr := filepath.Join(dir, "tr")
	mustMkdirAll(t, tr)
	// The same message.id three times, as the host writes one API response
	// as one record per content block — summing all three would triple it.
	line := tcCallLine("msg1", "claude-opus-5", 0, 1_000_000)
	content := strings.Repeat(line+"\n", 3)
	mustWrite(t, filepath.Join(tr, "dedup.jsonl"), content)

	out, err := runTokenCostCmd(t, "--transcripts", tr, "--session", "dedup")
	if err != nil {
		t.Fatalf("unexpected error: %v\n%s", err, out)
	}
	if !strings.Contains(out, "1 API call") {
		t.Errorf("want exactly 1 API call after dedup, got:\n%s", out)
	}
	if !strings.Contains(out, "$25.00") {
		t.Errorf("want $25.00 (1M output @ $25/MTok, not 3M), got:\n%s", out)
	}
}

func TestTokenCostLongestPrefixMatch(t *testing.T) {
	dir := t.TempDir()
	tr := filepath.Join(dir, "tr")
	mustMkdirAll(t, tr)
	mustWrite(t, filepath.Join(tr, "haiku.jsonl"),
		tcCallLine("h1", "claude-haiku-4-5-20251001", 0, 1_000_000)+"\n")

	out, err := runTokenCostCmd(t, "--transcripts", tr, "--session", "haiku")
	if err != nil {
		t.Fatalf("unexpected error: %v\n%s", err, out)
	}
	// claude-haiku-4-5-20251001 should resolve through the claude-haiku-4-5
	// family prefix, not come back unpriced.
	if !strings.Contains(out, "$5.00") {
		t.Errorf("want dated id to match its family prefix at $5.00, got:\n%s", out)
	}
	if strings.Contains(out, "no price for") {
		t.Errorf("dated id should not be reported unpriced, got:\n%s", out)
	}
}

func TestTokenCostUnknownModelNotZero(t *testing.T) {
	dir := t.TempDir()
	tr := filepath.Join(dir, "tr")
	mustMkdirAll(t, tr)
	mustWrite(t, filepath.Join(tr, "u1.jsonl"),
		tcCallLine("u1", "model-from-the-future", 0, 1_000_000)+"\n")

	out, err := runTokenCostCmd(t, "--transcripts", tr, "--session", "u1")
	if err != nil {
		t.Fatalf("unexpected error: %v\n%s", err, out)
	}
	if !strings.Contains(out, "no price for model-from-the-future") {
		t.Errorf("want unpriced note, got:\n%s", out)
	}
	if !strings.Contains(out, "TOTAL") || !strings.Contains(out, "—") {
		t.Errorf("want a dash total, not a zero, got:\n%s", out)
	}
}

func TestTokenCostNoTranscripts(t *testing.T) {
	dir := t.TempDir()
	_, err := runTokenCostCmd(t, "--transcripts", filepath.Join(dir, "nope"), "--all")
	if ec, ok := err.(exitCode); !ok || ec != 2 {
		t.Fatalf("want exit 2, got %v", err)
	}
}

func TestTokenCostSelectorRequired(t *testing.T) {
	_, err := runTokenCostCmd(t)
	if ec, ok := err.(exitCode); !ok || ec != 2 {
		t.Fatalf("want exit 2 when no selector given, got %v", err)
	}
}

func TestTokenCostBadPricesFile(t *testing.T) {
	dir := t.TempDir()
	_, err := runTokenCostCmd(t, "--all", "--prices", filepath.Join(dir, "nosuch.json"))
	if ec, ok := err.(exitCode); !ok || ec != 2 {
		t.Fatalf("want exit 2 for a missing prices file, got %v", err)
	}

	bad := filepath.Join(dir, "empty.json")
	mustWrite(t, bad, `{"as_of":"2026-01-01","models":{}}`)
	_, err = runTokenCostCmd(t, "--all", "--prices", bad)
	if ec, ok := err.(exitCode); !ok || ec != 2 {
		t.Fatalf("want exit 2 for an empty models table, got %v", err)
	}
}
