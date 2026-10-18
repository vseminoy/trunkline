package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"go-jira-mcp/internal/jira"
	"go-jira-mcp/internal/markdown"
)

// inJiraDir is the subfolder, inside the epic folder, where pull_epic writes the
// downloaded task files.
const inJiraDir = "in_jira"

// PullEpicResult summarizes a pull_epic download.
type PullEpicResult struct {
	Epic  string   `json:"epic"`
	Dir   string   `json:"dir"`
	Count int      `json:"count"`
	Files []string `json:"files"`
}

// PullEpic downloads every issue of one epic and writes one markdown task file
// per issue into that epic folder's ./in_jira subfolder. Files are formatted per
// docs/task-format.md and marked ready:true; the known
// jira_key/jira_url/jira_created_at are filled in (§5), so the mirrors are
// recognised as already-synced and never duplicated.
//
// The epic is named explicitly, like every other tool here, and resolved under
// TASKS_DIR. An empty name falls back to the current working directory, which is
// the CLI's original contract ("run this inside an epic folder") — a server
// process has no meaningful cwd, so over MCP the name is effectively required.
// It was that cwd-only contract that kept this operation off the MCP surface
// while the pipeline profile went on declaring it alongside the others.
func (s *Service) PullEpic(ctx context.Context, epic string) (PullEpicResult, error) {
	var dir string
	if strings.TrimSpace(epic) == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return PullEpicResult{}, err
		}
		epic, dir = filepath.Base(cwd), cwd
	} else {
		name, err := s.resolveEpic(epic)
		if err != nil {
			return PullEpicResult{}, err
		}
		abs, err := filepath.Abs(filepath.Join(s.cfg.TasksDir, name))
		if err != nil {
			return PullEpicResult{}, err
		}
		epic, dir = name, abs
	}
	if !jiraKeyRe.MatchString(epic) {
		return PullEpicResult{}, fmt.Errorf(
			"%q is not a valid JIRA epic key (expected e.g. PROJ-855)", epic)
	}

	issues, err := s.client.SearchEpicIssues(ctx, epicJQL(s.cfg.EpicField, epic))
	if err != nil {
		return PullEpicResult{}, err
	}

	dir = filepath.Join(dir, inJiraDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return PullEpicResult{}, fmt.Errorf("creating %s: %w", inJiraDir, err)
	}

	res := PullEpicResult{Epic: epic, Dir: inJiraDir}
	for _, is := range issues {
		name := epicTaskFileName(is.Key, is.Summary)
		base := strings.TrimSuffix(name, ".md")

		// A re-pull overwrites the mirror in place, but if the issue's summary
		// changed since the last pull its slug — and thus the file name — changes
		// too. Drop any prior mirror of the same issue under a different name so a
		// changed summary cannot leave a duplicate .md (and stale attachment folder).
		if err := removeStaleMirrors(dir, is.Key, name); err != nil {
			return res, fmt.Errorf("%s: %w", is.Key, err)
		}

		atts, err := s.downloadAttachments(ctx, dir, base, is.Attachments)
		if err != nil {
			return res, fmt.Errorf("%s: %w", is.Key, err)
		}

		t := &markdown.Task{
			FM: markdown.FrontMatter{
				Project:       is.ProjectKey,
				Summary:       is.Summary,
				Ready:         true,
				Type:          is.Type,
				Labels:        is.Labels,
				Attachments:   atts,
				Estimate:      is.OriginalEstimate,
				Status:        is.Status,
				JiraKey:       is.Key,
				JiraURL:       s.cfg.JiraURL + "/browse/" + is.Key,
				JiraCreatedAt: is.Created,
			},
			Body: epicTaskBody(is.Description),
		}
		out, err := t.Render()
		if err != nil {
			return res, fmt.Errorf("rendering %s: %w", is.Key, err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), out, 0o644); err != nil {
			return res, fmt.Errorf("writing %s: %w", name, err)
		}
		res.Files = append(res.Files, filepath.ToSlash(filepath.Join(inJiraDir, name)))
	}
	res.Count = len(res.Files)
	return res, nil
}

// downloadAttachments saves an issue's attachments into the task's attachment
// folder (in_jira/<base>/, mirroring the layout new tasks use — task-format.md
// §3.3) and returns their file names for the `attachments` front-matter field. It
// returns nil and creates no folder when the issue has no attachments.
func (s *Service) downloadAttachments(ctx context.Context, dir, base string, metas []jira.AttachmentMeta) ([]string, error) {
	if len(metas) == 0 {
		return nil, nil
	}
	taskDir := filepath.Join(dir, base)
	if err := os.MkdirAll(taskDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating task folder %q: %w", base, err)
	}
	names := make([]string, 0, len(metas))
	for _, a := range metas {
		// JIRA-supplied names are uploader-controlled; reduce to a base name so a
		// crafted name (e.g. "../x") cannot escape the task folder.
		fname := filepath.Base(a.Filename)
		dest := filepath.Join(taskDir, fname)
		if err := s.client.DownloadFile(ctx, a.Content, dest); err != nil {
			return nil, fmt.Errorf("downloading attachment %q: %w", a.Filename, err)
		}
		names = append(names, fname)
	}
	return names, nil
}

// epicJQL builds the JQL selecting an epic's issues. When the Epic Link field id
// is known (customfield_NNNNN) it uses cf[NNNNN], which is independent of the
// field's localized display name; otherwise it falls back to the "Epic Link" name.
func epicJQL(epicField, epic string) string {
	if num, ok := strings.CutPrefix(epicField, "customfield_"); ok && num != "" {
		return fmt.Sprintf(`cf[%s] = %q ORDER BY key ASC`, num, epic)
	}
	return fmt.Sprintf(`"Epic Link" = %q ORDER BY key ASC`, epic)
}

// epicTaskBody returns the JIRA issue description verbatim — only line endings
// are normalized to \n and a trailing newline is ensured. The text is kept
// exactly as stored in JIRA so that editing the file and re-uploading the task
// leaves the description in JIRA unchanged.
func epicTaskBody(description string) string {
	body := strings.ReplaceAll(description, "\r\n", "\n")
	if body == "" {
		return ""
	}
	if !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	return body
}

// epicTaskFileName builds a stable file name from the issue key and a slug of
// its summary, e.g. "PROJ901-uskorit-vygruzku.md". The dash is dropped from the key
// so the name carries a valid letter+number ordering prefix (docs/task-format.md
// §2.1) and parses as a task if moved into the epic folder for re-sync; the key
// stays traceable and the name is stable per issue so re-runs overwrite in place.
// The slug is best-effort and may be empty.
func epicTaskFileName(key, summary string) string {
	prefix := strings.ReplaceAll(key, "-", "")
	slug := slugify(summary)
	if slug == "" {
		return prefix + ".md"
	}
	return prefix + "-" + slug + ".md"
}

// removeStaleMirrors deletes any previously pulled mirror of the issue (its .md
// file and matching attachment folder) whose name differs from keepName. All
// mirrors of one issue share the key prefix produced by epicTaskFileName, so they
// are either "<prefix>.md" or "<prefix>-<slug>.md"; the trailing "." or "-"
// boundary keeps a longer key (e.g. PROJ9011) from matching a shorter one (PROJ901).
func removeStaleMirrors(dir, key, keepName string) error {
	prefix := strings.ReplaceAll(key, "-", "")
	candidates := []string{filepath.Join(dir, prefix+".md")}
	matches, err := filepath.Glob(filepath.Join(dir, prefix+"-*.md"))
	if err != nil {
		return err
	}
	candidates = append(candidates, matches...)

	for _, md := range candidates {
		if filepath.Base(md) == keepName {
			continue
		}
		if _, err := os.Stat(md); err != nil {
			continue // exact-name candidate that does not exist
		}
		if err := os.Remove(md); err != nil {
			return fmt.Errorf("removing stale mirror %q: %w", filepath.Base(md), err)
		}
		// Drop the matching attachment folder (named after the file without .md).
		if attDir, ok := strings.CutSuffix(md, ".md"); ok {
			if err := os.RemoveAll(attDir); err != nil {
				return fmt.Errorf("removing stale attachment folder %q: %w", filepath.Base(attDir), err)
			}
		}
	}
	return nil
}

// slugify lower-cases s and keeps only letters and digits, collapsing any other
// run into a single dash. Unicode letters (incl. Cyrillic) are preserved. The
// result is trimmed of dashes and capped to keep file names sane.
func slugify(s string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash && b.Len() > 0 {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	slug := strings.Trim(b.String(), "-")
	if r := []rune(slug); len(r) > 60 {
		slug = strings.Trim(string(r[:60]), "-")
	}
	return slug
}
