// Package markdown parses task markdown files (YAML front-matter + body) and
// manages their lifecycle on disk: reading, scanning epic folders, writing back
// the JIRA fields, and moving completed tasks into done/.
//
// The file format is the source of truth documented in docs/task-format.md.
package markdown

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// fence is the YAML front-matter delimiter.
const fence = "---"

// FrontMatter is the typed metadata block at the top of a task file. The struct
// field order is preserved when the file is written back, so auto-added JIRA
// fields appear at the end (see docs/task-format.md §5).
type FrontMatter struct {
	Project       string   `yaml:"project"`
	Summary       string   `yaml:"summary"`
	Ready         bool     `yaml:"ready"`
	Type          string   `yaml:"type"`
	Labels        []string `yaml:"labels,omitempty"`
	Links         []string `yaml:"links,omitempty"`
	Attachments   []string `yaml:"attachments,omitempty"`
	Estimate      string   `yaml:"estimate,omitempty"`
	Status        string   `yaml:"status,omitempty"`
	JiraKey       string   `yaml:"jira_key,omitempty"`
	JiraURL       string   `yaml:"jira_url,omitempty"`
	JiraCreatedAt string   `yaml:"jira_created_at,omitempty"`
}

// Task is a parsed task file together with its on-disk location and ordering.
type Task struct {
	Path   string      // absolute path to the .md file
	Epic   string      // epic key, taken from the parent folder name
	Prefix Prefix      // ordering prefix parsed from the file name
	FM     FrontMatter // parsed front-matter
	Body   string      // everything after the front-matter (the description)
}

// BaseName returns the file name without directory or extension. It is also the
// name of the task's attachment folder (docs/task-format.md §3.3).
func (t *Task) BaseName() string {
	return strings.TrimSuffix(filepath.Base(t.Path), filepath.Ext(t.Path))
}

// TaskDir returns the path to the task's attachment folder (sibling of the .md,
// named after the file without extension).
func (t *Task) TaskDir() string {
	return filepath.Join(filepath.Dir(t.Path), t.BaseName())
}

// ParseFile reads and parses a single task file. epic is the key derived from
// the parent folder name. It validates required front-matter fields and the
// file-name prefix.
func ParseFile(path, epic string) (*Task, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	fm, body, err := splitFrontMatter(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}

	var parsed FrontMatter
	if err := yaml.Unmarshal(fm, &parsed); err != nil {
		return nil, fmt.Errorf("%s: invalid front-matter YAML: %w", filepath.Base(path), err)
	}
	if err := validateRequired(fm); err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}

	prefix, err := ParsePrefix(filepath.Base(path))
	if err != nil {
		return nil, err
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	return &Task{
		Path:   abs,
		Epic:   epic,
		Prefix: prefix,
		FM:     parsed,
		Body:   body,
	}, nil
}

// splitFrontMatter separates the YAML front-matter block from the body. The file
// must begin with a "---" fence; the block ends at the next "---" line.
func splitFrontMatter(raw []byte) (frontMatter []byte, body string, err error) {
	text := string(bytes.ReplaceAll(raw, []byte("\r\n"), []byte("\n")))
	if !strings.HasPrefix(text, fence+"\n") && text != fence {
		return nil, "", fmt.Errorf("missing YAML front-matter (file must start with '---')")
	}
	rest := strings.TrimPrefix(text, fence+"\n")
	end := strings.Index(rest, "\n"+fence)
	if end < 0 {
		return nil, "", fmt.Errorf("unterminated front-matter (no closing '---')")
	}
	frontMatter = []byte(rest[:end])
	// Body starts after the closing fence line.
	afterFence := rest[end+len("\n"+fence):]
	body = strings.TrimPrefix(afterFence, "\n")
	return frontMatter, body, nil
}

// validateRequired ensures the four mandatory keys are present in the
// front-matter. Presence is checked on the raw map because a missing bool
// (ready) is indistinguishable from false on the typed struct.
func validateRequired(fm []byte) error {
	var m map[string]any
	if err := yaml.Unmarshal(fm, &m); err != nil {
		return fmt.Errorf("invalid front-matter YAML: %w", err)
	}
	var missing []string
	for _, key := range []string{"project", "summary", "ready", "type"} {
		if _, ok := m[key]; !ok {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required front-matter fields: %s", strings.Join(missing, ", "))
	}
	return nil
}

// Render serializes the task back to file bytes: front-matter (from t.FM) plus
// the unchanged body.
func (t *Task) Render() ([]byte, error) {
	fmBytes, err := yaml.Marshal(t.FM)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	buf.WriteString(fence + "\n")
	buf.Write(fmBytes)
	buf.WriteString(fence + "\n")
	if t.Body != "" {
		buf.WriteString(t.Body)
	}
	return buf.Bytes(), nil
}

// Save writes the task back to its current Path.
func (t *Task) Save() error {
	out, err := t.Render()
	if err != nil {
		return err
	}
	return os.WriteFile(t.Path, out, 0o644)
}
