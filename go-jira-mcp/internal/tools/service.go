// Package tools implements the MCP tools that scan markdown task files and
// create or update the corresponding JIRA issues.
package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go-jira-mcp/internal/config"
	"go-jira-mcp/internal/jira"
	"go-jira-mcp/internal/markdown"
)

// wipPrefix is prepended to the JIRA summary of draft (ready:false) issues.
const wipPrefix = "WIP: "

// msk is the Moscow timezone used for jira_created_at timestamps (TZ examples
// use +03:00).
var msk = time.FixedZone("MSK", 3*60*60)

// linkEpicToJira controls whether tasks are linked to their epic via the Epic
// Link custom field. Enabled now that the required edit permissions are granted.
const linkEpicToJira = true

// Service holds the dependencies shared by all tools.
type Service struct {
	cfg    *config.Config
	client *jira.Client
}

// NewService builds a Service from configuration.
func NewService(cfg *config.Config) *Service {
	return &Service{cfg: cfg, client: jira.New(cfg.JiraURL, cfg.Token)}
}

// CheckConnection verifies URL and token via GET /myself.
func (s *Service) CheckConnection(ctx context.Context) (map[string]any, error) {
	me, err := s.client.CheckConnection(ctx)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"ok":          true,
		"name":        me.Name,
		"displayName": me.DisplayName,
		"email":       me.Email,
	}, nil
}

// FindEpicField resolves the Epic Link custom field id via GET /field.
func (s *Service) FindEpicField(ctx context.Context) (map[string]any, error) {
	id, err := s.client.FindEpicField(ctx)
	if err != nil {
		return nil, err
	}
	s.cfg.EpicField = id
	return map[string]any{"epic_field": id}, nil
}

// ListMarkdownTasks scans a single epic folder under TASKS_DIR and returns
// parsed task metadata. The epic must be named explicitly; the whole tasks tree
// is never scanned at once.
func (s *Service) ListMarkdownTasks(_ context.Context, epic string) ([]TaskListItem, []TaskReport, error) {
	epic, err := s.resolveEpic(epic)
	if err != nil {
		return nil, nil, err
	}
	scan, err := markdown.ScanEpic(s.cfg.TasksDir, epic)
	if err != nil {
		return nil, nil, err
	}
	items := make([]TaskListItem, 0, len(scan.Tasks))
	for _, t := range scan.Tasks {
		items = append(items, taskListItem(s.cfg.TasksDir, t))
	}
	var errs []TaskReport
	for _, e := range scan.Errors {
		errs = append(errs, TaskReport{File: relPath(s.cfg.TasksDir, e.Path), Status: "error", Message: e.Err.Error()})
	}
	return items, errs, nil
}

// DryRun validates one epic's tasks and reports what would be created/updated
// without JIRA writes. The epic must be named explicitly.
func (s *Service) DryRun(_ context.Context, epic string, publishUnready bool) ([]DryRunItem, error) {
	epic, err := s.resolveEpic(epic)
	if err != nil {
		return nil, err
	}
	scan, err := markdown.ScanEpic(s.cfg.TasksDir, epic)
	if err != nil {
		return nil, err
	}
	prefixMap := map[string]string{}
	var out []DryRunItem
	for _, e := range scan.Errors {
		out = append(out, DryRunItem{File: relPath(s.cfg.TasksDir, e.Path), Action: "error", Message: e.Err.Error()})
	}
	for _, task := range scan.Tasks {
		item, key, err := s.planTask(task, prefixMap, publishUnready)
		if err != nil {
			item.Action = "error"
			item.Message = err.Error()
		} else if key != "" {
			prefixMap[localRefKey(task.Epic, task.Prefix.Raw)] = key
		}
		out = append(out, item)
	}
	return out, nil
}

// CreateAllTasks processes every task file in one epic folder in prefix order.
// The epic must be named explicitly; the whole tasks tree is never processed at
// once.
func (s *Service) CreateAllTasks(ctx context.Context, epic string, publishUnready bool) ([]TaskReport, error) {
	epic, err := s.resolveEpic(epic)
	if err != nil {
		return nil, err
	}
	if linkEpicToJira {
		if err := s.ensureEpicField(ctx); err != nil {
			return nil, err
		}
	}
	scan, err := markdown.ScanEpic(s.cfg.TasksDir, epic)
	if err != nil {
		return nil, err
	}
	prefixMap := map[string]string{}
	var reports []TaskReport
	for _, e := range scan.Errors {
		reports = append(reports, TaskReport{File: relPath(s.cfg.TasksDir, e.Path), Status: "error", Message: e.Err.Error()})
	}
	for _, task := range scan.Tasks {
		rep := s.processTask(ctx, task, prefixMap, publishUnready, false)
		reports = append(reports, rep)
		if rep.JiraKey != "" && (rep.Status == "created" || rep.Status == "updated") {
			prefixMap[localRefKey(task.Epic, task.Prefix.Raw)] = rep.JiraKey
		}
	}
	return reports, nil
}

// CreateJiraTask creates or updates a single task file.
func (s *Service) CreateJiraTask(ctx context.Context, file string, publishUnready bool) (TaskReport, error) {
	if linkEpicToJira {
		if err := s.ensureEpicField(ctx); err != nil {
			return TaskReport{}, err
		}
	}
	path, epic, err := s.resolveTaskFile(file)
	if err != nil {
		return TaskReport{File: file, Status: "error", Message: err.Error()}, nil
	}
	if strings.Contains(filepath.ToSlash(path), "/done/") {
		return TaskReport{File: relPath(s.cfg.TasksDir, path), Status: "skipped (done)", Message: "file is in done/"}, nil
	}
	task, err := markdown.ParseFile(path, epic)
	if err != nil {
		return TaskReport{File: relPath(s.cfg.TasksDir, path), Status: "error", Message: err.Error()}, nil
	}
	return s.processTask(ctx, task, map[string]string{}, publishUnready, false), nil
}

func taskListItem(tasksDir string, t *markdown.Task) TaskListItem {
	return TaskListItem{
		File:    relPath(tasksDir, t.Path),
		Epic:    t.Epic,
		Project: t.FM.Project,
		Summary: t.FM.Summary,
		Type:    t.FM.Type,
		Ready:   t.FM.Ready,
		Prefix:  t.Prefix.Raw,
		JiraKey: t.FM.JiraKey,
		Labels:  t.FM.Labels,
		Links:   t.FM.Links,
	}
}

func (s *Service) ensureEpicField(ctx context.Context) error {
	if s.cfg.EpicField != "" {
		return nil
	}
	id, err := s.client.FindEpicField(ctx)
	if err != nil {
		return fmt.Errorf("JIRA_EPIC_FIELD is not set and auto-discovery failed: %w", err)
	}
	s.cfg.EpicField = id
	return nil
}

// resolveEpic normalizes a caller-supplied epic identifier to the epic folder
// name under TASKS_DIR. It accepts a bare key ("PROJ-855") or a path
// ("tasks/PROJ-855"), taking the final path element either way so the scan is
// always scoped to one chosen epic. An empty value is rejected: callers must
// name the epic they are working with.
func (s *Service) resolveEpic(epic string) (string, error) {
	epic = strings.TrimSpace(epic)
	epic = strings.Trim(filepath.ToSlash(epic), "/")
	if epic != "" {
		epic = filepath.Base(epic)
	}
	if epic == "" || epic == "." {
		return "", fmt.Errorf("an epic folder must be specified (e.g. PROJ-855); refusing to scan the whole tasks tree")
	}
	if epic == "done" {
		return "", fmt.Errorf("done/ is not an epic folder")
	}
	return epic, nil
}

func (s *Service) resolveTaskFile(file string) (path, epic string, err error) {
	candidates := []string{file}
	if !filepath.IsAbs(file) {
		candidates = append(candidates, filepath.Join(s.cfg.TasksDir, file))
	}
	for _, c := range candidates {
		abs, err := filepath.Abs(c)
		if err != nil {
			continue
		}
		if _, err := os.Stat(abs); err != nil {
			continue
		}
		epicDir := filepath.Dir(abs)
		epic = filepath.Base(epicDir)
		if epic == "done" {
			return "", "", fmt.Errorf("task file must not be inside done/")
		}
		return abs, epic, nil
	}
	return "", "", fmt.Errorf("task file not found: %q", file)
}

func (s *Service) planTask(task *markdown.Task, prefixMap map[string]string, publishUnready bool) (DryRunItem, string, error) {
	item := DryRunItem{
		File:    relPath(s.cfg.TasksDir, task.Path),
		Project: task.FM.Project,
		Epic:    task.Epic,
		Type:    task.FM.Type,
		Summary: effectiveSummary(task.FM.Summary, task.FM.Ready),
	}
	if skip, reason := shouldSkip(task, publishUnready); skip {
		item.Action = reason
		return item, "", nil
	}
	prep, atts, resolved, err := s.prepareTask(task, prefixMap)
	if err != nil {
		return item, "", err
	}
	item.Resolved = resolved
	item.Attachments = attachmentNames(atts)
	if task.FM.JiraKey != "" {
		item.Action = "update"
		return item, task.FM.JiraKey, nil
	}
	item.Action = "create"
	_ = prep
	return item, fmt.Sprintf("NEW-%s", task.Prefix.Raw), nil
}

func (s *Service) processTask(ctx context.Context, task *markdown.Task, prefixMap map[string]string, publishUnready, dryRun bool) TaskReport {
	file := relPath(s.cfg.TasksDir, task.Path)
	if skip, reason := shouldSkip(task, publishUnready); skip {
		return TaskReport{File: file, Status: reason}
	}
	prep, atts, _, err := s.prepareTask(task, prefixMap)
	if err != nil {
		return TaskReport{File: file, Status: "error", Message: err.Error()}
	}
	if dryRun {
		action := "created"
		if task.FM.JiraKey != "" {
			action = "updated"
		}
		return TaskReport{File: file, Status: action, JiraKey: task.FM.JiraKey}
	}

	fields := jira.IssueFields{
		ProjectKey:  task.FM.Project,
		Summary:     effectiveSummary(task.FM.Summary, task.FM.Ready),
		Description: prep.Text,
		IssueType:   task.FM.Type,
		Labels:      task.FM.Labels,
	}
	if linkEpicToJira {
		fields.EpicFieldID = s.cfg.EpicField
		fields.EpicKey = task.Epic
	}

	var key string
	created := false
	if task.FM.JiraKey != "" {
		key = task.FM.JiraKey
		if err := s.client.UpdateIssue(ctx, key, fields); err != nil {
			return TaskReport{File: file, Status: "error", Message: err.Error()}
		}
		// Clear attachments that are about to be re-uploaded (JIRA keeps
		// same-named duplicates) and orphans from prior syncs before re-adding.
		if err := s.removeStaleAttachments(ctx, key, atts); err != nil {
			return TaskReport{File: file, Status: "error", Message: fmt.Sprintf("issue %s updated but clearing old attachments failed: %v", key, err)}
		}
	} else {
		res, err := s.client.CreateIssue(ctx, fields)
		if err != nil {
			return TaskReport{File: file, Status: "error", Message: err.Error()}
		}
		key = res.Key
		created = true
	}

	for _, att := range atts {
		if err := s.client.AddAttachment(ctx, key, att.Path); err != nil {
			return TaskReport{File: file, Status: "error", Message: fmt.Sprintf("issue %s created but attachment %q failed: %v", key, att.Base, err)}
		}
	}

	if created {
		for _, blocker := range prep.Blockers {
			if err := s.client.CreateBlockingLink(ctx, s.cfg.LinkType, key, blocker); err != nil {
				return TaskReport{File: file, Status: "error", Message: fmt.Sprintf("issue %s created but link to %s failed: %v", key, blocker, err)}
			}
		}
	}

	now := time.Now().In(msk).Format(time.RFC3339)
	if task.FM.JiraCreatedAt == "" {
		task.FM.JiraCreatedAt = now
	}
	task.FM.JiraKey = key
	task.FM.JiraURL = s.cfg.JiraURL + "/browse/" + key
	if err := task.Save(); err != nil {
		return TaskReport{File: file, Status: "error", Message: fmt.Sprintf("issue %s saved in JIRA but file write failed: %v", key, err)}
	}

	entry := markdown.RegistryEntry{
		File:        relPath(s.cfg.TasksDir, task.Path),
		Epic:        task.Epic,
		JiraKey:     key,
		JiraURL:     task.FM.JiraURL,
		Summary:     task.FM.Summary,
		Attachments: attachmentNames(atts),
		CreatedAt:   task.FM.JiraCreatedAt,
	}
	if err := markdown.UpsertRegistry(s.cfg.TasksDir, entry); err != nil {
		return TaskReport{File: file, Status: "error", Message: fmt.Sprintf("issue %s saved but registry update failed: %v", key, err)}
	}

	status := "updated"
	if created {
		status = "created"
	}
	if task.FM.Ready {
		if err := task.MoveToDone(); err != nil {
			return TaskReport{File: file, Status: "error", Message: fmt.Sprintf("issue %s saved but move to done/ failed: %v", key, err)}
		}
		entry.File = relPath(s.cfg.TasksDir, task.Path)
		if err := markdown.UpsertRegistry(s.cfg.TasksDir, entry); err != nil {
			return TaskReport{File: file, Status: "error", Message: fmt.Sprintf("moved to done/ but registry update failed: %v", err)}
		}
	}
	return TaskReport{File: file, Status: status, JiraKey: key}
}

type preparedTask struct {
	Text     string
	Blockers []string
	Images   []string
	Attach   []resolvedAttachment
}

func (s *Service) prepareTask(task *markdown.Task, prefixMap map[string]string) (preparedTask, []resolvedAttachment, []string, error) {
	desc, err := markdown.PrepareDescription(task.Body)
	if err != nil {
		return preparedTask{}, nil, nil, err
	}
	atts, err := s.collectAttachments(task, desc.Images)
	if err != nil {
		return preparedTask{}, nil, nil, err
	}
	var blockers []string
	for _, link := range task.FM.Links {
		key, err := s.resolveBlocker(task, link, prefixMap)
		if err != nil {
			return preparedTask{}, nil, nil, err
		}
		blockers = append(blockers, key)
	}
	return preparedTask{Text: desc.Text, Blockers: blockers, Images: desc.Images, Attach: atts}, atts, blockers, nil
}

func shouldSkip(task *markdown.Task, publishUnready bool) (bool, string) {
	if task.FM.JiraKey != "" {
		return false, ""
	}
	if !task.FM.Ready && !publishUnready {
		return true, "skipped (not ready)"
	}
	return false, ""
}

func (s *Service) resolveBlocker(task *markdown.Task, link string, prefixMap map[string]string) (string, error) {
	if jiraKeyRe.MatchString(link) {
		return link, nil
	}
	blockerPrefix, err := markdown.ParsePrefix(link + ".md")
	if err != nil {
		return "", fmt.Errorf("invalid local link %q: %w", link, err)
	}
	if !blockerPrefix.Less(task.Prefix) {
		return "", fmt.Errorf("local link %q must refer to a task with a smaller prefix than %s", link, task.Prefix.Raw)
	}
	if key, ok := prefixMap[localRefKey(task.Epic, link)]; ok {
		return key, nil
	}
	if key, ok, err := markdown.LookupDoneKey(s.cfg.TasksDir, task.Epic, link); err != nil {
		return "", err
	} else if ok {
		return key, nil
	}
	if key, ok := lookupRegistryByPrefix(s.cfg.TasksDir, task.Epic, link); ok {
		return key, nil
	}
	return "", fmt.Errorf("cannot resolve local link %q: blocker not found in done/ or current run", link)
}

func lookupRegistryByPrefix(tasksDir, epic, prefixRaw string) (string, bool) {
	entries, err := markdown.LoadRegistry(tasksDir)
	if err != nil {
		return "", false
	}
	want := strings.ToUpper(prefixRaw)
	for _, e := range entries {
		if e.Epic != epic || e.JiraKey == "" {
			continue
		}
		base := filepath.Base(e.File)
		p, err := markdown.ParsePrefix(base)
		if err != nil {
			continue
		}
		if p.Raw == want {
			return e.JiraKey, true
		}
	}
	return "", false
}

func relPath(tasksDir, abs string) string {
	if rel, err := filepath.Rel(tasksDir, abs); err == nil {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(abs)
}

// removeStaleAttachments deletes issue attachments that the update must not
// keep: those about to be re-uploaded (JIRA keeps same-named duplicates) and
// orphans uploaded on a previous sync that are no longer part of the task.
// Matching is by file name and covers both declared attachments and inline body
// images, since collectAttachments and the registry track both alike.
func (s *Service) removeStaleAttachments(ctx context.Context, key string, atts []resolvedAttachment) error {
	stale := make(map[string]bool, len(atts))
	for _, a := range atts {
		stale[a.Base] = true // re-uploaded below -> drop the old copy first
	}
	for _, name := range s.previousAttachmentNames(key) {
		stale[name] = true // previously uploaded; orphans get removed, re-added ones get refreshed
	}
	if len(stale) == 0 {
		return nil
	}
	existing, err := s.client.ListAttachments(ctx, key)
	if err != nil {
		return err
	}
	for _, e := range existing {
		if stale[e.Filename] {
			if err := s.client.DeleteAttachment(ctx, e.ID); err != nil {
				return fmt.Errorf("deleting old attachment %q (id %s): %w", e.Filename, e.ID, err)
			}
		}
	}
	return nil
}

// previousAttachmentNames returns the attachment names recorded for the issue on
// its last successful sync, used to detect orphans. The registry is updated only
// after attachments are processed, so during an update it still holds the prior
// run's names.
func (s *Service) previousAttachmentNames(key string) []string {
	entries, err := markdown.LoadRegistry(s.cfg.TasksDir)
	if err != nil {
		return nil
	}
	for _, e := range entries {
		if e.JiraKey == key {
			return e.Attachments
		}
	}
	return nil
}

func attachmentNames(atts []resolvedAttachment) []string {
	names := make([]string, len(atts))
	for i, a := range atts {
		names[i] = a.Base
	}
	return names
}
