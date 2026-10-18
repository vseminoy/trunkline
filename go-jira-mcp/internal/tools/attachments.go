package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"go-jira-mcp/internal/markdown"
)

// jiraKeyRe matches a fully-qualified JIRA key (LETTERS-NUMBER); anything else
// in a links entry is treated as a local file prefix.
var jiraKeyRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]*-\d+$`)

// resolvedAttachment is a validated attachment ready for upload.
type resolvedAttachment struct {
	Base string // file name as JIRA will see it
	Path string // absolute path on disk
}

// collectAttachments resolves the task's declared attachments plus the local
// images referenced by the body, validating that each exists and is within the
// size limit (TZ §7: strict check, otherwise the task is not created). Files are
// de-duplicated by base name.
func (s *Service) collectAttachments(task *markdown.Task, images []string) ([]resolvedAttachment, error) {
	taskDir := task.TaskDir()
	seen := map[string]bool{}
	var out []resolvedAttachment

	add := func(rel string) error {
		base := filepath.Base(rel)
		if seen[base] {
			return nil
		}
		full := filepath.Join(taskDir, rel)
		info, err := os.Stat(full)
		if err != nil {
			return fmt.Errorf("attachment %q not found in task folder %q", rel, taskDir)
		}
		if info.IsDir() {
			return fmt.Errorf("attachment %q is a directory", rel)
		}
		if info.Size() > s.cfg.MaxAttachmentBytes {
			return fmt.Errorf("attachment %q is %d bytes, exceeds limit of %d", rel, info.Size(), s.cfg.MaxAttachmentBytes)
		}
		seen[base] = true
		out = append(out, resolvedAttachment{Base: base, Path: full})
		return nil
	}

	for _, img := range images {
		if err := add(img); err != nil {
			return nil, err
		}
	}
	for _, att := range task.FM.Attachments {
		if err := add(att); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// localRefKey is the composite key for the per-run prefix→issue map, scoped by
// epic because local links only reference the same epic folder.
func localRefKey(epic, prefixRaw string) string {
	return epic + "/" + strings.ToUpper(prefixRaw)
}

// effectiveSummary applies the WIP prefix rule (TZ §5).
func effectiveSummary(summary string, ready bool) string {
	clean := strings.TrimPrefix(summary, wipPrefix)
	if ready {
		return clean
	}
	return wipPrefix + clean
}
