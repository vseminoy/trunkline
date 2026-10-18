package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go-jira-mcp/internal/config"
	"go-jira-mcp/internal/markdown"
)

func TestEpicJQL(t *testing.T) {
	if got := epicJQL("customfield_10101", "PROJ-855"); got != `cf[10101] = "PROJ-855" ORDER BY key ASC` {
		t.Errorf("with field id: got %q", got)
	}
	if got := epicJQL("", "PROJ-855"); got != `"Epic Link" = "PROJ-855" ORDER BY key ASC` {
		t.Errorf("without field id: got %q", got)
	}
}

func TestEpicTaskFileName(t *testing.T) {
	cases := map[string]string{
		"PROJ-901|Ускорить выгрузку отчёта": "PROJ901-ускорить-выгрузку-отчёта.md",
		"PROJ-902|Fix Excel export!":        "PROJ902-fix-excel-export.md",
		"PROJ-903|":                         "PROJ903.md",
	}
	for in, want := range cases {
		key, summary, _ := strings.Cut(in, "|")
		if got := epicTaskFileName(key, summary); got != want {
			t.Errorf("epicTaskFileName(%q,%q) = %q, want %q", key, summary, got, want)
		}
	}
}

func TestEpicTaskBodyVerbatim(t *testing.T) {
	// JIRA wiki text is kept exactly as-is (only \r\n normalized, trailing \n ensured).
	in := "h2. Что сломалось\r\nПри выгрузке падает 500.\r\n!error.png!"
	want := "h2. Что сломалось\nПри выгрузке падает 500.\n!error.png!\n"
	if got := epicTaskBody(in); got != want {
		t.Errorf("epicTaskBody = %q, want %q", got, want)
	}
	if got := epicTaskBody(""); got != "" {
		t.Errorf("empty description should stay empty, got %q", got)
	}
}

// TestRemoveStaleMirrorsDropsRenamedDuplicate proves a re-pull cannot leave a
// duplicate .md when an issue's summary (and thus its slug/file name) changed:
// the prior mirror and its attachment folder are removed, the unchanged one and
// other issues' mirrors are kept.
func TestRemoveStaleMirrorsDropsRenamedDuplicate(t *testing.T) {
	dir := t.TempDir()
	write := func(name string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mkdir := func(name string) {
		if err := os.MkdirAll(filepath.Join(dir, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// Old mirror of PROJ-901 (stale slug) with its attachment folder.
	write("PROJ901-staryj-zagolovok.md")
	mkdir("PROJ901-staryj-zagolovok")
	// A different issue that shares the leading digits — must NOT be touched.
	write("PROJ9011-other.md")
	// The bare-key form of a third issue, kept as-is on re-pull.
	write("PROJ902.md")

	if err := removeStaleMirrors(dir, "PROJ-901", "PROJ901-novyj-zagolovok.md"); err != nil {
		t.Fatal(err)
	}

	gone := []string{"PROJ901-staryj-zagolovok.md", "PROJ901-staryj-zagolovok"}
	for _, p := range gone {
		if _, err := os.Stat(filepath.Join(dir, p)); !os.IsNotExist(err) {
			t.Errorf("expected %q removed, err=%v", p, err)
		}
	}
	kept := []string{"PROJ9011-other.md", "PROJ902.md"}
	for _, p := range kept {
		if _, err := os.Stat(filepath.Join(dir, p)); err != nil {
			t.Errorf("expected %q kept: %v", p, err)
		}
	}

	// Keeping the same name (summary unchanged) must be a no-op overwrite case.
	write("PROJ903-stable.md")
	if err := removeStaleMirrors(dir, "PROJ-903", "PROJ903-stable.md"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "PROJ903-stable.md")); err != nil {
		t.Errorf("same-name mirror must be left in place: %v", err)
	}
}

// TestRenderedFileParsesBack proves a downloaded file round-trips through the
// markdown parser as a valid task (front-matter required fields + body).
func TestRenderedFileParsesBack(t *testing.T) {
	tk := &markdown.Task{
		FM: markdown.FrontMatter{
			Project: "PROJ", Summary: "Починить экспорт", Ready: true, Type: "Bug",
			Labels: []string{"reports"}, JiraKey: "PROJ-901",
			JiraURL: "https://j/browse/PROJ-901", JiraCreatedAt: "2026-06-17T16:40:00+03:00",
		},
		Body: epicTaskBody("описание"),
	}
	out, err := tk.Render()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := dir + "/" + epicTaskFileName("PROJ-901", "Починить экспорт")
	if err := os.WriteFile(path, out, 0o644); err != nil {
		t.Fatal(err)
	}
	parsed, err := markdown.ParseFile(path, "PROJ-855")
	if err != nil {
		t.Fatalf("downloaded file does not parse as a valid task: %v", err)
	}
	if !parsed.FM.Ready || parsed.FM.JiraKey != "PROJ-901" || parsed.FM.Project != "PROJ" {
		t.Errorf("round-trip lost fields: %+v", parsed.FM)
	}
}

// PullEpic used to take the epic from the working directory only, which is why
// it could not be an MCP tool at all: a server process has no meaningful cwd.
// These cover the naming half — everything that decides which folder is written
// to, before any JIRA call is made.
func TestPullEpicRejectsUnusableEpicNames(t *testing.T) {
	svc := NewService(&config.Config{TasksDir: t.TempDir()})
	for _, tc := range []struct{ name, epic, want string }{
		{"not a key", "notes", "not a valid JIRA epic key"},
		{"the done folder", "done", "not an epic folder"},
		{"a traversal reduces to its last element", "../../etc", "not a valid JIRA epic key"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.PullEpic(context.Background(), tc.epic)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("PullEpic(%q) = %v, want an error containing %q", tc.epic, err, tc.want)
			}
		})
	}
}

// A named epic is resolved under TASKS_DIR rather than against the process's own
// directory — that is what makes the operation callable from a server at all.
func TestPullEpicNamedEpicResolvesUnderTasksDir(t *testing.T) {
	svc := NewService(&config.Config{TasksDir: t.TempDir()})
	// No JIRA is reachable from a test, so the call fails at the search — after
	// the folder has been decided. What is asserted is that it got that far with
	// a path form that resolveEpic has to reduce to a bare name first.
	_, err := svc.PullEpic(context.Background(), "tasks/PROJ-855")
	if err == nil {
		t.Fatal("expected the JIRA search to fail without a server")
	}
	if strings.Contains(err.Error(), "not a valid JIRA epic key") {
		t.Fatalf("the epic name was not resolved out of the path: %v", err)
	}
}
