package markdown

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// ScanResult is the outcome of scanning a tasks root: successfully parsed tasks
// (ordered by epic then prefix) and per-file parse errors.
type ScanResult struct {
	Tasks  []*Task
	Errors []ScanError
}

// ScanError records a file that could not be parsed into a valid task.
type ScanError struct {
	Path string
	Err  error
}

// ScanEpic scans a single epic folder (tasksDir/epic) for task files to create
// or update, rather than every epic under tasksDir. The folder name is the epic
// key. Any "done/" subtree is skipped (TZ §5: processed tasks are not re-picked
// up).
//
// Tasks are returned sorted by file-name prefix, which is the creation order
// required so blockers are created before their dependents. Scanning is scoped
// to one epic so an operation always works on the epic the caller chose, never
// the whole tasks tree.
func ScanEpic(tasksDir, epic string) (*ScanResult, error) {
	epicDir := filepath.Join(tasksDir, epic)
	info, err := os.Stat(epicDir)
	if err != nil {
		return nil, fmt.Errorf("reading epic dir %q: %w", epicDir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("epic %q is not a directory under %q", epic, tasksDir)
	}

	res := &ScanResult{}
	mdFiles, err := mdFilesIn(epicDir)
	if err != nil {
		return nil, err
	}
	for _, path := range mdFiles {
		task, err := ParseFile(path, epic)
		if err != nil {
			res.Errors = append(res.Errors, ScanError{Path: path, Err: err})
			continue
		}
		res.Tasks = append(res.Tasks, task)
	}

	sort.SliceStable(res.Tasks, func(i, j int) bool {
		return res.Tasks[i].Prefix.Less(res.Tasks[j].Prefix)
	})
	return res, nil
}

// mdFilesIn returns the .md files directly inside an epic folder, excluding the
// done/ subtree. The scan is shallow on purpose: task files live at the top of
// the epic folder; subdirectories are task attachment folders, not tasks.
func mdFilesIn(epicDir string) ([]string, error) {
	entries, err := os.ReadDir(epicDir)
	if err != nil {
		return nil, fmt.Errorf("reading epic dir %q: %w", epicDir, err)
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue // done/ and task attachment folders are not tasks
		}
		if filepath.Ext(e.Name()) == ".md" {
			out = append(out, filepath.Join(epicDir, e.Name()))
		}
	}
	return out, nil
}
