// Package jira is a minimal client for the JIRA Server / Data Center REST API
// v2, using only net/http and Bearer (Personal Access Token) authentication.
package jira

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client talks to a JIRA instance over REST API v2.
type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

// New creates a client for the given base URL (no trailing slash) and PAT.
func New(baseURL, token string) *Client {
	return &Client{
		baseURL: baseURL,
		token:   token,
		http:    &http.Client{Timeout: 60 * time.Second},
	}
}

// apiError is returned for non-2xx responses, carrying status and body for
// diagnostics.
type apiError struct {
	Method string
	Path   string
	Status int
	Body   string
}

func (e *apiError) Error() string {
	return fmt.Sprintf("jira %s %s: status %d: %s", e.Method, e.Path, e.Status, e.Body)
}

// do performs an authenticated JSON request and decodes a 2xx response body into
// out (out may be nil to discard the body). extraHeaders are applied last.
func (c *Client) do(ctx context.Context, method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshaling request: %w", err)
		}
		reader = bytes.NewReader(buf)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("jira %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &apiError{Method: method, Path: path, Status: resp.StatusCode, Body: string(raw)}
	}
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("decoding %s %s response: %w", method, path, err)
		}
	}
	return nil
}

// Myself is the subset of GET /myself used for the connection check.
type Myself struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Email       string `json:"emailAddress"`
}

// CheckConnection verifies the URL and token via GET /rest/api/2/myself.
func (c *Client) CheckConnection(ctx context.Context) (*Myself, error) {
	var me Myself
	if err := c.do(ctx, http.MethodGet, "/rest/api/2/myself", nil, &me); err != nil {
		return nil, err
	}
	return &me, nil
}

// field is one entry of GET /rest/api/2/field.
type field struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// FindEpicField returns the custom field id whose name is "Epic Link"
// (GET /rest/api/2/field).
func (c *Client) FindEpicField(ctx context.Context) (id string, err error) {
	var fields []field
	if err := c.do(ctx, http.MethodGet, "/rest/api/2/field", nil, &fields); err != nil {
		return "", err
	}
	for _, f := range fields {
		if f.Name == "Epic Link" {
			return f.ID, nil
		}
	}
	return "", fmt.Errorf(`no field named "Epic Link" found on this instance`)
}

// IssueFields holds the data needed to create or update an issue. EpicFieldID is
// the custom field id for "Epic Link"; when both it and EpicKey are set, the epic
// link is included.
type IssueFields struct {
	ProjectKey  string
	Summary     string
	Description string
	IssueType   string
	Labels      []string
	EpicFieldID string
	EpicKey     string
}

// fieldsMap builds the "fields" object. priority is intentionally omitted so the
// project default (Medium) applies (TZ §7).
func (f IssueFields) fieldsMap(includeProjectAndType bool) map[string]any {
	fields := map[string]any{
		"summary":     f.Summary,
		"description": f.Description,
	}
	if includeProjectAndType {
		fields["project"] = map[string]any{"key": f.ProjectKey}
		fields["issuetype"] = map[string]any{"name": f.IssueType}
	}
	if f.Labels != nil {
		fields["labels"] = f.Labels
	}
	if f.EpicFieldID != "" && f.EpicKey != "" {
		fields[f.EpicFieldID] = f.EpicKey
	}
	return fields
}

// CreateResult is the response of POST /issue.
type CreateResult struct {
	ID  string `json:"id"`
	Key string `json:"key"`
}

// CreateIssue creates an issue and returns its new key.
func (c *Client) CreateIssue(ctx context.Context, f IssueFields) (*CreateResult, error) {
	body := map[string]any{"fields": f.fieldsMap(true)}
	var res CreateResult
	if err := c.do(ctx, http.MethodPost, "/rest/api/2/issue", body, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

// UpdateIssue updates an existing issue's editable fields (summary, description,
// labels, epic link). Project and issue type are not changed on update.
func (c *Client) UpdateIssue(ctx context.Context, key string, f IssueFields) error {
	body := map[string]any{"fields": f.fieldsMap(false)}
	return c.do(ctx, http.MethodPut, "/rest/api/2/issue/"+key, body, nil)
}

// AttachmentMeta is the metadata of an issue attachment needed to download it:
// the file name and the absolute content URL returned by the search API.
type AttachmentMeta struct {
	Filename string `json:"filename"`
	Content  string `json:"content"`
}

// EpicIssue is the subset of a JIRA issue needed to mirror it into a local
// markdown task file (see SearchEpicIssues).
type EpicIssue struct {
	Key              string
	Summary          string
	Description      string
	Type             string
	ProjectKey       string
	Labels           []string
	Created          string
	OriginalEstimate string // human-readable original estimate, e.g. "2d 4h"
	Status           string // workflow status name, e.g. "Open", "In Progress", "Closed"
	Attachments      []AttachmentMeta
}

// searchResponse is the subset of POST /search used by SearchEpicIssues.
type searchResponse struct {
	StartAt int `json:"startAt"`
	Total   int `json:"total"`
	Issues  []struct {
		Key    string `json:"key"`
		Fields struct {
			Summary     string           `json:"summary"`
			Description string           `json:"description"`
			Labels      []string         `json:"labels"`
			Created     string           `json:"created"`
			Attachment  []AttachmentMeta `json:"attachment"`
			IssueType   struct {
				Name string `json:"name"`
			} `json:"issuetype"`
			Project struct {
				Key string `json:"key"`
			} `json:"project"`
			TimeTracking struct {
				OriginalEstimate string `json:"originalEstimate"`
			} `json:"timetracking"`
			Status struct {
				Name string `json:"name"`
			} `json:"status"`
		} `json:"fields"`
	} `json:"issues"`
}

// SearchEpicIssues runs a JQL query and returns every matching issue, following
// pagination (POST /rest/api/2/search).
func (c *Client) SearchEpicIssues(ctx context.Context, jql string) ([]EpicIssue, error) {
	const page = 100
	var out []EpicIssue
	for startAt := 0; ; {
		body := map[string]any{
			"jql":        jql,
			"startAt":    startAt,
			"maxResults": page,
			"fields":     []string{"summary", "description", "issuetype", "project", "labels", "created", "attachment", "timetracking", "status"},
		}
		var resp searchResponse
		if err := c.do(ctx, http.MethodPost, "/rest/api/2/search", body, &resp); err != nil {
			return nil, err
		}
		for _, is := range resp.Issues {
			out = append(out, EpicIssue{
				Key:              is.Key,
				Summary:          is.Fields.Summary,
				Description:      is.Fields.Description,
				Type:             is.Fields.IssueType.Name,
				ProjectKey:       is.Fields.Project.Key,
				Labels:           is.Fields.Labels,
				Created:          is.Fields.Created,
				OriginalEstimate: is.Fields.TimeTracking.OriginalEstimate,
				// Verbatim workflow status name (Open, In Progress, In Review,
				// Closed, …) — reflects the issue's exact position in the workflow.
				Status:      is.Fields.Status.Name,
				Attachments: is.Fields.Attachment,
			})
		}
		startAt += len(resp.Issues)
		if len(resp.Issues) == 0 || startAt >= resp.Total {
			break
		}
	}
	return out, nil
}

// CreateBlockingLink creates a link where currentKey "is blocked by" blockerKey.
// For the Blocks link type, outwardIssue blocks inwardIssue, so the current task
// is the inwardIssue and the blocker is the outwardIssue (TZ §7).
func (c *Client) CreateBlockingLink(ctx context.Context, linkType, currentKey, blockerKey string) error {
	body := map[string]any{
		"type":         map[string]any{"name": linkType},
		"inwardIssue":  map[string]any{"key": currentKey},
		"outwardIssue": map[string]any{"key": blockerKey},
	}
	return c.do(ctx, http.MethodPost, "/rest/api/2/issueLink", body, nil)
}
