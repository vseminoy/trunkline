package tools

// TaskListItem is one task returned by list_markdown_tasks.
type TaskListItem struct {
	File    string   `json:"file"`
	Epic    string   `json:"epic"`
	Project string   `json:"project"`
	Summary string   `json:"summary"`
	Type    string   `json:"type"`
	Ready   bool     `json:"ready"`
	Prefix  string   `json:"prefix"`
	JiraKey string   `json:"jira_key,omitempty"`
	Labels  []string `json:"labels,omitempty"`
	Links   []string `json:"links,omitempty"`
}

// TaskReport is the per-file outcome of create/dry-run operations.
type TaskReport struct {
	File    string `json:"file"`
	Status  string `json:"status"`
	JiraKey string `json:"jira_key,omitempty"`
	Message string `json:"message,omitempty"`
}

// DryRunItem describes what would happen to one task without writing to JIRA.
type DryRunItem struct {
	File        string   `json:"file"`
	Action      string   `json:"action"`
	Summary     string   `json:"summary,omitempty"`
	Project     string   `json:"project,omitempty"`
	Epic        string   `json:"epic,omitempty"`
	Type        string   `json:"type,omitempty"`
	Resolved    []string `json:"resolved_links,omitempty"`
	Attachments []string `json:"attachments,omitempty"`
	Message     string   `json:"message,omitempty"`
}
