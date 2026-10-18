// Package config loads server configuration from the environment.
//
// Values come from environment variables; for local development a ".env" file
// in the working directory (or any ancestor) is loaded first, with real
// environment variables taking precedence over its contents.
package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// DefaultMaxAttachmentBytes is the attachment size limit used when
// JIRA_MAX_ATTACHMENT_BYTES is not set (10 MB).
const DefaultMaxAttachmentBytes = 10 * 1024 * 1024

// DefaultLinkType is the JIRA issue-link type used for blocking links when
// JIRA_LINK_TYPE is not set.
const DefaultLinkType = "Blocks"

// Config holds the resolved server configuration.
type Config struct {
	JiraURL             string // base URL of the JIRA instance, no trailing slash
	Token               string // Personal Access Token for Bearer auth
	EpicField           string // custom field id for "Epic Link" (e.g. customfield_10008)
	TasksDir            string // root directory containing epic folders
	LinkType            string // issue-link type name for blocking links
	MaxAttachmentBytes  int64  // per-file attachment size limit
}

// Load reads configuration from the environment, first loading a ".env" file if
// one is found. It returns an error only for missing required values; optional
// values fall back to their defaults.
func Load() (*Config, error) {
	loadDotEnv()

	cfg := &Config{
		JiraURL:            strings.TrimRight(os.Getenv("JIRA_URL"), "/"),
		Token:              os.Getenv("JIRA_PERSONAL_ACCESS_TOKEN"),
		EpicField:          os.Getenv("JIRA_EPIC_FIELD"),
		TasksDir:           os.Getenv("TASKS_DIR"),
		LinkType:           os.Getenv("JIRA_LINK_TYPE"),
		MaxAttachmentBytes: DefaultMaxAttachmentBytes,
	}

	if cfg.TasksDir == "" {
		cfg.TasksDir = "./tasks"
	}
	if cfg.LinkType == "" {
		cfg.LinkType = DefaultLinkType
	}
	if raw := os.Getenv("JIRA_MAX_ATTACHMENT_BYTES"); raw != "" {
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("JIRA_MAX_ATTACHMENT_BYTES must be a positive integer, got %q", raw)
		}
		cfg.MaxAttachmentBytes = n
	}

	var missing []string
	if cfg.JiraURL == "" {
		missing = append(missing, "JIRA_URL")
	}
	if cfg.Token == "" {
		missing = append(missing, "JIRA_PERSONAL_ACCESS_TOKEN")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}

	return cfg, nil
}

// loadDotEnv loads KEY=VALUE pairs from the nearest ".env" file (searching the
// working directory and its ancestors). Existing environment variables are not
// overwritten. Missing files are ignored — .env is optional.
func loadDotEnv() {
	path, ok := findDotEnv()
	if !ok {
		return
	}
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.Trim(strings.TrimSpace(val), `"'`)
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, val)
		}
	}
}

// findDotEnv walks up from the working directory looking for a ".env" file.
func findDotEnv() (string, bool) {
	dir, err := os.Getwd()
	if err != nil {
		return "", false
	}
	for {
		candidate := filepath.Join(dir, ".env")
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}
