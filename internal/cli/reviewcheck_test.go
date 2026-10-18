package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func runRC(t *testing.T, env *Env, args ...string) (string, error) {
	t.Helper()
	var out, errOut bytes.Buffer
	if env == nil {
		env = &Env{Dir: "."}
	}
	env.Out, env.Err = &out, &errOut
	err := runReviewCheck(*env, args)
	return out.String() + errOut.String(), err
}

// skillsRoot points at this repo's own agent-skills checkout, independent of
// the test binary's working directory (internal/cli, two levels under root).
func skillsRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", "agent-skills"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "agents", "code-reviewer.md")); err != nil {
		t.Skipf("agent-skills submodule not checked out: %v", err)
	}
	return root
}

func TestReviewCheckDryRun(t *testing.T) {
	root := skillsRoot(t)
	dir := t.TempDir()
	diff := filepath.Join(dir, "c.diff")
	mustWrite(t, diff, "diff --git a/x.go b/x.go\n+func New() {}\n")

	out, err := runRC(t, nil, "--dry-run", "--diff", diff,
		"--base-url", "http://127.0.0.1:1/v1", "--model", "m", "--skills-root", root)
	if err != nil {
		t.Fatalf("dry-run should not error without a key or network: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Critic agent (step 9)") || !strings.Contains(out, "chars") {
		t.Errorf("want persona sizes printed, got:\n%s", out)
	}
	if !strings.Contains(out, "nothing reviewed") {
		t.Errorf("dry run must not claim to have reviewed anything, got:\n%s", out)
	}
}

func TestReviewCheckEmptyDiffRejected(t *testing.T) {
	_, err := runRC(t, nil, "--diff", "/dev/null", "--base-url", "http://127.0.0.1:1/v1", "--model", "m")
	if ec, ok := err.(exitCode); !ok || ec != 2 {
		t.Fatalf("want exit 2 for an empty diff, got %v", err)
	}
}

func TestReviewCheckNoKeyInEnv(t *testing.T) {
	t.Setenv("TRUNKLINE_REVIEW_API_KEY", "")
	root := skillsRoot(t)
	dir := t.TempDir()
	diff := filepath.Join(dir, "c.diff")
	mustWrite(t, diff, "diff --git a/x.go b/x.go\n+func New() {}\n")
	_, err := runRC(t, nil, "--diff", diff, "--base-url", "http://127.0.0.1:1/v1",
		"--model", "m", "--skills-root", root)
	if ec, ok := err.(exitCode); !ok || ec != 2 {
		t.Fatalf("want exit 2 without a key, got %v", err)
	}
}

// mockReviewServer mimics an OpenAI-compatible endpoint that, on its first
// call carrying max_tokens, rejects it and asks for max_completion_tokens —
// the gateway dialect review-check.py originally had to paper over.
func mockReviewServer(t *testing.T) (*httptest.Server, *int32) {
	t.Helper()
	var calls int32
	var renamed int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if _, hasOld := body["max_tokens"]; hasOld && atomic.CompareAndSwapInt32(&renamed, 0, 1) {
			w.WriteHeader(400)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "use max_completion_tokens instead"})
			return
		}
		w.WriteHeader(200)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{
				"message":       map[string]string{"content": strings.Repeat("Finding. ", 60)},
				"finish_reason": "stop",
			}},
		})
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func TestReviewCheckRenamesTokenFieldWithoutLosingTheCall(t *testing.T) {
	rcTokenField = "max_tokens" // reset package state between tests
	root := skillsRoot(t)
	srv, calls := mockReviewServer(t)
	dir := t.TempDir()
	diff := filepath.Join(dir, "c.diff")
	mustWrite(t, diff, "diff --git a/x.go b/x.go\n+func New() {}\n")

	env := Env{}
	t.Setenv("TRUNKLINE_REVIEW_API_KEY", "secret")
	out, err := runRC(t, &env, "--diff", diff, "--base-url", srv.URL, "--model", "mock-1",
		"--skills-root", root, "--retries", "0", "--changeset", "1")
	if err != nil {
		t.Fatalf("the renamed field must not cost the retry budget: %v\n%s", err, out)
	}
	if !strings.Contains(out, "COMPLETE") {
		t.Errorf("want both personas complete, got:\n%s", out)
	}
	// One rename call plus one resend per persona, both personas: 2 personas
	// x (1 failed + 1 resent) = 4, not 2 — the rename must not drop a call.
	if got := atomic.LoadInt32(calls); got < 3 {
		t.Errorf("want at least 3 calls (one 400 plus resends), got %d", got)
	}
}
