package markdown

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// MoveToDone moves a completed task file (and its attachment folder, if any)
// into the done/ subfolder of its epic folder, keeping the bundle intact
// (TZ §5). t.Path is updated to the new location.
func (t *Task) MoveToDone() error {
	epicDir := filepath.Dir(t.Path)
	doneDir := filepath.Join(epicDir, "done")
	if err := os.MkdirAll(doneDir, 0o755); err != nil {
		return fmt.Errorf("creating done dir: %w", err)
	}

	// Move the attachment folder first, if present, so a failure leaves the
	// .md in place (still discoverable on the next run).
	srcTaskDir := t.TaskDir()
	if info, err := os.Stat(srcTaskDir); err == nil && info.IsDir() {
		dstTaskDir := filepath.Join(doneDir, t.BaseName())
		if err := os.Rename(srcTaskDir, dstTaskDir); err != nil {
			return fmt.Errorf("moving task folder to done/: %w", err)
		}
	}

	dst := filepath.Join(doneDir, filepath.Base(t.Path))
	if err := os.Rename(t.Path, dst); err != nil {
		return fmt.Errorf("moving task file to done/: %w", err)
	}
	t.Path = dst
	return nil
}

// LookupDoneKey resolves a local link prefix (e.g. "C1") to a JIRA key by
// scanning the done/ folder of the given epic for a task with that prefix and
// reading its jira_key (TZ §7: done/ is read for link resolution even though it
// is excluded from the create scan).
//
// It returns ("", false, nil) when no matching done task exists.
func LookupDoneKey(tasksDir, epic, prefixRaw string) (string, bool, error) {
	doneDir := filepath.Join(tasksDir, epic, "done")
	entries, err := os.ReadDir(doneDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("reading done dir %q: %w", doneDir, err)
	}
	want := strings.ToUpper(prefixRaw)
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".md" {
			continue
		}
		p, err := ParsePrefix(e.Name())
		if err != nil {
			continue
		}
		if p.Raw != want {
			continue
		}
		task, err := ParseFile(filepath.Join(doneDir, e.Name()), epic)
		if err != nil {
			return "", false, err
		}
		if task.FM.JiraKey == "" {
			return "", false, nil
		}
		return task.FM.JiraKey, true, nil
	}
	return "", false, nil
}
