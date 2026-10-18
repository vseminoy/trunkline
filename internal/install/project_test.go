package install

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vseminoy/trunkline/internal/state"
)

// TestRegisterHookShapes is a golden test over the forms .claude/settings.json
// actually takes in the wild. The two "refuses" cases are the point: a type
// assertion whose `ok` is discarded turns an unrecognised shape into nil, and
// the write that follows replaces the user's hooks with ours at exit code 0.
func TestRegisterHookShapes(t *testing.T) {
	cases := []struct {
		name    string
		before  string
		refuses bool
		// wantEntries is how many PreToolUse entries there must be afterwards.
		wantEntries int
		keeps       string // a substring that must survive
	}{
		{
			name:        "no file at all",
			before:      "",
			wantEntries: 1,
		},
		{
			name:        "empty object",
			before:      `{}`,
			wantEntries: 1,
		},
		{
			name:        "unrelated settings are preserved",
			before:      `{"model": "opus", "hooks": {"PostToolUse": []}}`,
			wantEntries: 1,
			keeps:       "opus",
		},
		{
			name: "an existing PreToolUse entry of the project's own",
			before: `{"hooks": {"PreToolUse": [` +
				`{"matcher": "Write", "hooks": [{"type": "command", "command": "echo hi"}]}]}}`,
			wantEntries: 2,
			keeps:       "echo hi",
		},
		{
			name: "the gate under a widened matcher is already registered",
			before: `{"hooks": {"PreToolUse": [` +
				`{"matcher": "Bash|Write", "hooks": [{"type": "command", ` +
				`"command": "bash .claude/hooks/pre-commit-gate.sh"}]}]}}`,
			wantEntries: 1, // no second copy appended
		},
		{
			name:    "hooks is not an object",
			before:  `{"hooks": "please"}`,
			refuses: true,
		},
		{
			name:    "PreToolUse is an object rather than an array",
			before:  `{"hooks": {"PreToolUse": {"matcher": "Bash"}}}`,
			refuses: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := newTestWriter(t)
			if c.before != "" {
				writeOnDisk(t, w.Dir, ClaudeSetting, c.before, 0o644)
			}

			err := w.registerHook()
			if c.refuses {
				if err == nil {
					t.Fatal("registerHook accepted a shape it cannot merge into")
				}
				if got := readOnDisk(t, w.Dir, ClaudeSetting); got != c.before {
					t.Errorf("the file was rewritten anyway:\n%s", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("registerHook: %v", err)
			}

			raw := readOnDisk(t, w.Dir, ClaudeSetting)
			var settings map[string]any
			if err := json.Unmarshal([]byte(raw), &settings); err != nil {
				t.Fatalf("result is not valid JSON: %v\n%s", err, raw)
			}
			pre, _, err := preToolUse(settings)
			if err != nil {
				t.Fatal(err)
			}
			if len(pre) != c.wantEntries {
				t.Errorf("PreToolUse has %d entries, want %d:\n%s", len(pre), c.wantEntries, raw)
			}
			if !gateInPreToolUse(pre) {
				t.Errorf("the gate is not registered:\n%s", raw)
			}
			if c.keeps != "" && !strings.Contains(raw, c.keeps) {
				t.Errorf("%q was dropped:\n%s", c.keeps, raw)
			}
		})
	}
}

// TestRegisterHookIsIdempotent: running init twice must not append a second
// copy, which would run the gate twice on every Bash call.
func TestRegisterHookIsIdempotent(t *testing.T) {
	w := newTestWriter(t)
	for i := range 2 {
		if err := w.registerHook(); err != nil {
			t.Fatalf("pass %d: %v", i, err)
		}
	}
	var settings map[string]any
	if err := json.Unmarshal([]byte(readOnDisk(t, w.Dir, ClaudeSetting)), &settings); err != nil {
		t.Fatal(err)
	}
	pre, _, err := preToolUse(settings)
	if err != nil {
		t.Fatal(err)
	}
	if len(pre) != 1 {
		t.Errorf("PreToolUse has %d entries after two installs, want 1", len(pre))
	}
}

// TestGateRegisteredParsesRatherThanGreps: the gate script's name appearing
// somewhere else in the file is not the gate being wired to a trigger.
func TestGateRegisteredParsesRatherThanGreps(t *testing.T) {
	cases := []struct {
		name     string
		settings string
		want     bool
	}{
		{
			name: "registered under PreToolUse",
			settings: `{"hooks": {"PreToolUse": [{"matcher": "Bash", "hooks": ` +
				`[{"type": "command", "command": "bash .claude/hooks/pre-commit-gate.sh"}]}]}}`,
			want: true,
		},
		{
			name: "named under PostToolUse only — fires after the fact, gates nothing",
			settings: `{"hooks": {"PostToolUse": [{"matcher": "Bash", "hooks": ` +
				`[{"type": "command", "command": "bash .claude/hooks/pre-commit-gate.sh"}]}]}}`,
			want: false,
		},
		{
			name:     "mentioned in an unrelated key",
			settings: `{"note": "we removed .claude/hooks/pre-commit-gate.sh on purpose"}`,
			want:     false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			writeOnDisk(t, dir, ClaudeSetting, c.settings, 0o644)
			kind, got := GateRegistered(dir)
			if got != c.want {
				t.Errorf("GateRegistered = %v (%s), want %v", got, kind, c.want)
			}
		})
	}
}

// TestGateBroken covers the failure the whole gate mechanism turns on: wired,
// reported installed, and inert.
func TestGateBroken(t *testing.T) {
	t.Run("executable script is fine", func(t *testing.T) {
		dir := t.TempDir()
		writeOnDisk(t, dir, ClaudeHook, "#!/bin/sh\n", 0o755)
		if why := GateBroken(dir, GateClaude); why != "" {
			t.Errorf("GateBroken = %q, want none", why)
		}
	})
	t.Run("non-executable script is broken", func(t *testing.T) {
		dir := t.TempDir()
		writeOnDisk(t, dir, ClaudeHook, "#!/bin/sh\n", 0o644)
		why := GateBroken(dir, GateClaude)
		if why == "" {
			t.Fatal("a non-executable gate script reported as working")
		}
		if !strings.Contains(why, "executable") {
			t.Errorf("reason = %q, want it to name the executable bit", why)
		}
	})
	t.Run("missing script is broken", func(t *testing.T) {
		if why := GateBroken(t.TempDir(), GateClaude); why == "" {
			t.Error("a missing gate script reported as working")
		}
	})
}

// TestEnsureFlagsNeverRewrites: the flags block is the user's configuration, and
// silently changing a flag silently changes which gates run.
func TestEnsureFlagsNeverRewrites(t *testing.T) {
	const existing = "# Code standards\n\n## Pipeline flags\n\n- profile: mine\n"
	w := newTestWriter(t)
	writeOnDisk(t, w.Dir, AgentsFile, existing, 0o644)

	status, err := w.EnsureFlags(Flags{Profile: "theirs"})
	if err != nil {
		t.Fatal(err)
	}
	if status != Unchanged {
		t.Errorf("status = %q, want %q", status, Unchanged)
	}
	if got := readOnDisk(t, w.Dir, AgentsFile); got != existing {
		t.Errorf("the block was rewritten:\n%s", got)
	}
}

// TestEnsureFlagsAppends: on a file without the block, and on one that does not
// end in a newline — the case that would otherwise glue the heading onto the
// last line of the user's text.
func TestEnsureFlagsAppends(t *testing.T) {
	cases := map[string]string{
		"no file":             "",
		"no trailing newline": "# Code standards\n\nUse tabs.",
		"trailing newline":    "# Code standards\n\nUse tabs.\n",
	}
	for name, before := range cases {
		t.Run(name, func(t *testing.T) {
			w := newTestWriter(t)
			if before != "" {
				writeOnDisk(t, w.Dir, AgentsFile, before, 0o644)
			}
			if _, err := w.EnsureFlags(Flags{Profile: "p"}); err != nil {
				t.Fatal(err)
			}
			got := readOnDisk(t, w.Dir, AgentsFile)
			if !strings.Contains(got, FlagsHeading) {
				t.Fatalf("no flags block:\n%s", got)
			}
			if before != "" && !strings.Contains(got, "Use tabs.") {
				t.Errorf("the user's text was lost:\n%s", got)
			}
			if strings.Contains(got, "tabs.## Pipeline") {
				t.Errorf("the heading was glued onto the last line:\n%s", got)
			}
		})
	}
}

// TestEnsureFlagsIsNotTracked: AGENTS.md belongs to the project, so update must
// never treat it as ours to refresh or prune.
func TestEnsureFlagsIsNotTracked(t *testing.T) {
	w := newTestWriter(t)
	if _, err := w.EnsureFlags(Flags{Profile: "p"}); err != nil {
		t.Fatal(err)
	}
	if w.State.Tracked(AgentsFile) {
		t.Errorf("%s was recorded in state.Files", AgentsFile)
	}
}

func TestFlagsBlockAlignment(t *testing.T) {
	block := FlagsBlock(Flags{Profile: "p"})
	var col = -1
	for line := range strings.SplitSeq(strings.TrimSpace(block), "\n") {
		if !strings.HasPrefix(line, "- ") && !strings.HasPrefix(line, "# - ") {
			continue
		}
		i := strings.Index(line, "  # ")
		if i < 0 {
			t.Errorf("no comment on %q", line)
			continue
		}
		if col == -1 {
			col = i
		} else if i != col {
			t.Errorf("comment column %d, want %d — %q", i, col, line)
		}
	}
	if col == -1 {
		t.Fatal("no flag rows in the block")
	}
}

// TestPipelineIgnored: the knowledge base being gitignored is a silent
// downgrade to "no knowledge base" for everyone but the person who wrote it.
func TestPipelineIgnored(t *testing.T) {
	type c struct {
		ignore string
		kb     string
		want   bool
	}
	cases := []c{
		{"", "", false},
		{"node_modules\n", "", false},
		{".pipeline\n", "", true},
		{"/.pipeline/\n", "", true},
		{".pipeline/knowledge/\n", "", true},
		{"# .pipeline is fine\n", "", false},
		// Run state is *supposed* to be ignored; it must not read as the
		// knowledge base being hidden.
		{".pipeline/workdir/\n", "", false},
		{"/.pipeline/workdir/\n", "", false},
		{".pipeline/workdir/review/\n", "", false},
		// A base of the project's own choosing, inside the repository.
		{"docs/kb/\n", "docs/kb", true},
		{"docs/\n", "docs/kb", true},
		{"docs/kb/\n", ".pipeline/knowledge", false},
		// A base outside the repository cannot be hidden by this .gitignore,
		// however sweeping the pattern: it is versioned, or not, elsewhere.
		{".pipeline\n", "/srv/company-knowledge", false},
		{".pipeline\n", "../platform/.pipeline/knowledge", false},
		{".pipeline\n", "none", false},
	}
	for _, tc := range cases {
		dir := t.TempDir()
		if tc.ignore != "" {
			writeOnDisk(t, dir, ".gitignore", tc.ignore, 0o644)
		}
		if got := PipelineIgnored(dir, tc.kb); got != tc.want {
			t.Errorf("PipelineIgnored(%q, %q) = %v, want %v", tc.ignore, tc.kb, got, tc.want)
		}
	}
}

// TestInternal: a flag path that leaves the project belongs to another
// checkout, and nothing this project does reaches it.
func TestInternal(t *testing.T) {
	for p, want := range map[string]bool{
		".pipeline/knowledge":             true,
		"docs/kb":                         true,
		"./docs/kb":                       true,
		"/srv/knowledge":                  false,
		"../platform/.pipeline/knowledge": false,
		"..":                              false,
		"none":                            false,
		"":                                false,
	} {
		if got := Internal(p); got != want {
			t.Errorf("Internal(%q) = %v, want %v", p, got, want)
		}
	}
}

func TestStateRejectsEscapingPaths(t *testing.T) {
	dir := t.TempDir()
	writeOnDisk(t, dir, state.File,
		"core: 0.1.0\nfiles:\n  ../../escaped.md: abc\n", 0o644)
	if _, err := state.Load(dir); err == nil {
		t.Fatal("state.Load accepted a path outside the project")
	}
}

func TestSafeJoin(t *testing.T) {
	dir := filepath.Join("/tmp", "project")
	ok := []string{"a.md", "a/b.md", "./a/b.md", ".trunkline/core/x.md"}
	for _, rel := range ok {
		if _, err := safeJoin(dir, rel); err != nil {
			t.Errorf("safeJoin(%q) = %v, want it accepted", rel, err)
		}
	}
	bad := []string{"", "..", "../x.md", "a/../../x.md", "/etc/passwd", "."}
	for _, rel := range bad {
		if got, err := safeJoin(dir, rel); err == nil {
			t.Errorf("safeJoin(%q) = %q, want it refused", rel, got)
		}
	}
}

// TestEnsurePipelineDirs: the tasks directory comes from the profile slot, not
// from a constant. A slot pointing outside the project names another repository
// — creating it would fabricate half of someone else's checkout, and would hide
// the one failure doctor exists to report.
func TestEnsurePipelineDirs(t *testing.T) {
	cases := map[string]struct{ created, outside bool }{
		"":                   {created: false},
		".pipeline/tasks/":   {created: true},
		"docs/epics/":        {created: true},
		"../analysis/tasks/": {created: false, outside: true},
	}
	for slot, want := range cases {
		w := newTestWriter(t)
		if err := w.EnsurePipelineDirs(slot, StateDir+"/knowledge/acme"); err != nil {
			t.Fatal(err)
		}
		for _, d := range []string{StateDir + "/knowledge/acme", WorkDir + "/metrics"} {
			if fi, err := os.Stat(filepath.Join(w.Dir, filepath.FromSlash(d))); err != nil || !fi.IsDir() {
				t.Errorf("tasks.dir=%q: %s missing", slot, d)
			}
		}
		if slot == "" {
			continue
		}
		_, err := os.Stat(filepath.Join(w.Dir, filepath.FromSlash(slot)))
		if want.created && err != nil {
			t.Errorf("tasks.dir=%q: not created, want created", slot)
		}
		if !want.created && err == nil {
			t.Errorf("tasks.dir=%q: created, want left to the other repository", slot)
		}
		note, rerr := os.ReadFile(filepath.Join(w.Dir, StateDir, "README.md"))
		if rerr != nil {
			t.Fatal(rerr)
		}
		if !strings.Contains(string(note), slot) {
			t.Errorf("tasks.dir=%q: the .pipeline note does not name it", slot)
		}
		if want.outside && !strings.Contains(string(note), "analysis checkout") {
			t.Errorf("tasks.dir=%q: the note does not say the tasks live elsewhere", slot)
		}
	}
}

// TestRunStateIgnored: run state committed is a slow tax rather than a break —
// the journal is append-only, so two branches that both ran the pipeline
// conflict on every merge. doctor warns, so the answer has to be right.
func TestRunStateIgnored(t *testing.T) {
	cases := map[string]int{
		"":                            1, // nothing ignored
		".pipeline\n":                 0, // the lot
		"/.pipeline/\n":               0,
		"/.pipeline/workdir/\n":       0, // the one line a project needs
		".pipeline/workdir\n":         0,
		".pipeline/workdir/review/\n": 1, // half of it is still carried into history
		"!.pipeline/workdir/\n":       1, // a negation ignores nothing
	}
	for content, want := range cases {
		dir := t.TempDir()
		if content != "" {
			writeOnDisk(t, dir, ".gitignore", content, 0o644)
		}
		if got := RunStateIgnored(dir); len(got) != want {
			t.Errorf("RunStateIgnored(%q) = %v (%d), want %d missing", content, got, len(got), want)
		}
	}
}

// TestEnsureSoundHooksIsIdempotent: the wiring runs on every update, not once
// behind a migration, so a second pass must not append a second beep — two
// hooks firing at every prompt is the failure the user notices, loudly.
func TestEnsureSoundHooksIsIdempotent(t *testing.T) {
	w := newTestWriter(t)
	writeOnDisk(t, w.Dir, ClaudeSetting,
		`{"hooks": {"Notification": [{"hooks": [{"type": "command", "command": "bash mine.sh"}]}]}}`, 0o644)
	for i := range 2 {
		if err := w.EnsureSoundHooks(); err != nil {
			t.Fatalf("pass %d: %v", i, err)
		}
	}
	raw := readOnDisk(t, w.Dir, ClaudeSetting)
	var settings map[string]any
	if err := json.Unmarshal([]byte(raw), &settings); err != nil {
		t.Fatal(err)
	}
	for _, ev := range soundEvents {
		entries, _, err := hookEvent(settings, ev[0])
		if err != nil {
			t.Fatal(err)
		}
		if !firesScript(entries, "notify-sound.sh") {
			t.Errorf("%s does not fire the sound hook:\n%s", ev[0], raw)
		}
	}
	notif, _, err := hookEvent(settings, "Notification")
	if err != nil {
		t.Fatal(err)
	}
	// The project's own Notification hook plus ours, and no more.
	if len(notif) != 2 {
		t.Errorf("Notification has %d entries after two passes, want 2:\n%s", len(notif), raw)
	}
	if !strings.Contains(raw, "mine.sh") {
		t.Errorf("the project's own hook was dropped:\n%s", raw)
	}
}

// TestEnsureSoundHooksDropsStaleEvents: a project wired by the earlier version
// beeped on Stop and SubagentStop too, i.e. on every step of the run. Update
// has to take those back out by itself — the user should not have to edit
// settings.json to stop the noise — while leaving hooks that are not ours.
func TestEnsureSoundHooksDropsStaleEvents(t *testing.T) {
	w := newTestWriter(t)
	writeOnDisk(t, w.Dir, ClaudeSetting, `{"hooks": {
		"Stop": [
			{"hooks": [{"type": "command", "command": "bash .trunkline/agent-skills/hooks/notify-sound.sh done"}]},
			{"hooks": [{"type": "command", "command": "bash mine.sh"}]}
		],
		"SubagentStop": [
			{"hooks": [{"type": "command", "command": "bash .trunkline/agent-skills/hooks/notify-sound.sh subagent"}]}
		]
	}}`, 0o644)
	if err := w.EnsureSoundHooks(); err != nil {
		t.Fatal(err)
	}
	raw := readOnDisk(t, w.Dir, ClaudeSetting)
	var settings map[string]any
	if err := json.Unmarshal([]byte(raw), &settings); err != nil {
		t.Fatal(err)
	}
	hooks, _ := settings["hooks"].(map[string]any)
	if _, still := hooks["SubagentStop"]; still {
		t.Errorf("SubagentStop survived with nothing left in it:\n%s", raw)
	}
	stop, _, err := hookEvent(settings, "Stop")
	if err != nil {
		t.Fatal(err)
	}
	if firesScript(stop, "notify-sound.sh") {
		t.Errorf("Stop still beeps:\n%s", raw)
	}
	if len(stop) != 1 || !strings.Contains(raw, "mine.sh") {
		t.Errorf("the project's own Stop hook was not kept intact:\n%s", raw)
	}
	notif, _, err := hookEvent(settings, "Notification")
	if err != nil {
		t.Fatal(err)
	}
	if !firesScript(notif, "notify-sound.sh") {
		t.Errorf("Notification did not get the beep:\n%s", raw)
	}
}

// TestDropScriptKeepsSharedEntry: an entry that runs our script alongside a
// hook of the project's own is left alone — dropping it would delete the
// project's hook as collateral, which is not ours to do.
func TestDropScriptKeepsSharedEntry(t *testing.T) {
	entries := []any{map[string]any{"hooks": []any{
		map[string]any{"command": "bash notify-sound.sh done"},
		map[string]any{"command": "bash mine.sh"},
	}}}
	if got := dropScript(entries, "notify-sound.sh"); len(got) != 1 {
		t.Errorf("dropScript removed a shared entry: %v", got)
	}
}

// TestEnsureSoundHooksRefusesUnknownShape: a `Stop` of another shape is the
// user's configuration, and rewriting it silently is worse than stopping.
func TestEnsureSoundHooksRefusesUnknownShape(t *testing.T) {
	w := newTestWriter(t)
	writeOnDisk(t, w.Dir, ClaudeSetting, `{"hooks": {"Stop": "beep"}}`, 0o644)
	if err := w.EnsureSoundHooks(); err == nil {
		t.Fatal("a `Stop` that is not an array was accepted")
	}
}
