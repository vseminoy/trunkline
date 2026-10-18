package jira

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
)

// Attachment is the subset of an issue attachment used for de-duplication on
// update: the id needed to delete it and the filename JIRA shows.
type Attachment struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
}

// ListAttachments returns the attachments currently on an issue via
// GET /rest/api/2/issue/{key}?fields=attachment.
func (c *Client) ListAttachments(ctx context.Context, key string) ([]Attachment, error) {
	var res struct {
		Fields struct {
			Attachment []Attachment `json:"attachment"`
		} `json:"fields"`
	}
	if err := c.do(ctx, http.MethodGet, "/rest/api/2/issue/"+key+"?fields=attachment", nil, &res); err != nil {
		return nil, err
	}
	return res.Fields.Attachment, nil
}

// DeleteAttachment removes a single attachment by its id via
// DELETE /rest/api/2/attachment/{id}.
func (c *Client) DeleteAttachment(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/rest/api/2/attachment/"+id, nil, nil)
}

// DownloadFile fetches an attachment from its absolute content URL (as returned
// by the search API in AttachmentMeta.Content) using Bearer auth and writes the
// bytes to destPath, which must already have its parent directory created.
func (c *Client) DownloadFile(ctx context.Context, url, destPath string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("downloading %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return &apiError{Method: http.MethodGet, Path: url, Status: resp.StatusCode, Body: string(raw)}
	}

	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("creating %q: %w", destPath, err)
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("writing %q: %w", destPath, err)
	}
	return nil
}

// AddAttachment uploads a single file to an issue via
// POST /rest/api/2/issue/{key}/attachments. The X-Atlassian-Token: no-check
// header is required to bypass XSRF protection (TZ §7).
func (c *Client) AddAttachment(ctx context.Context, key, filePath string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("opening attachment %q: %w", filePath, err)
	}
	defer f.Close()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return err
	}
	if _, err := io.Copy(part, f); err != nil {
		return fmt.Errorf("reading attachment %q: %w", filePath, err)
	}
	if err := w.Close(); err != nil {
		return err
	}

	url := c.baseURL + "/rest/api/2/issue/" + key + "/attachments"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("X-Atlassian-Token", "no-check")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("uploading attachment to %s: %w", key, err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &apiError{Method: http.MethodPost, Path: "/issue/" + key + "/attachments", Status: resp.StatusCode, Body: string(raw)}
	}
	return nil
}
