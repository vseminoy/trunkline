package markdown

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// RegistryFile is the name of the machine-readable registry kept at the root of
// the tasks directory (TZ §5).
const RegistryFile = "index.json"

// RegistryEntry is one record in index.json mapping a file to its JIRA issue.
type RegistryEntry struct {
	File        string   `json:"file"`        // path relative to the tasks dir
	Epic        string   `json:"epic"`        // epic key
	JiraKey     string   `json:"jira_key"`    // created/updated issue key
	JiraURL     string   `json:"jira_url"`    // direct link to the issue
	Summary     string   `json:"summary"`     // summary stored in the file (no WIP prefix)
	Attachments []string `json:"attachments"` // attachment names uploaded
	CreatedAt   string   `json:"created_at"`  // ISO-8601 creation time
}

// LoadRegistry reads index.json from tasksDir. A missing file yields an empty
// registry.
func LoadRegistry(tasksDir string) ([]RegistryEntry, error) {
	path := filepath.Join(tasksDir, RegistryFile)
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading registry %q: %w", path, err)
	}
	var entries []RegistryEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, fmt.Errorf("parsing registry %q: %w", path, err)
	}
	return entries, nil
}

// UpsertRegistry adds or replaces (matched by JiraKey) an entry in index.json
// and writes it back atomically-ish (write then rename).
func UpsertRegistry(tasksDir string, entry RegistryEntry) error {
	entries, err := LoadRegistry(tasksDir)
	if err != nil {
		return err
	}
	replaced := false
	for i := range entries {
		if entries[i].JiraKey == entry.JiraKey {
			entries[i] = entry
			replaced = true
			break
		}
	}
	if !replaced {
		entries = append(entries, entry)
	}

	out, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(tasksDir, RegistryFile)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, out, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
