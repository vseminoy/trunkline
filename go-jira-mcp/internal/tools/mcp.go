package tools

import (
	"context"
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Register wires all JIRA task tools onto the MCP server.
func Register(server *mcp.Server, svc *Service) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "check_connection",
		Description: "Verify JIRA URL and Personal Access Token via GET /rest/api/2/myself",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, map[string]any, error) {
		out, err := svc.CheckConnection(ctx)
		return toolResult(out, err)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "find_epic_field",
		Description: "Discover the Epic Link custom field id via GET /rest/api/2/field",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, map[string]any, error) {
		out, err := svc.FindEpicField(ctx)
		return toolResult(out, err)
	})

	type listIn struct {
		Epic string `json:"epic" jsonschema:"epic folder to scan, e.g. PROJ-855 (required: scans only this epic, not the whole tasks tree)"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_markdown_tasks",
		Description: "List parsed markdown tasks for one epic folder under TASKS_DIR (read-only). Always ask which epic to work with; this never scans the whole tasks tree.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in listIn) (*mcp.CallToolResult, map[string]any, error) {
		tasks, errs, err := svc.ListMarkdownTasks(ctx, in.Epic)
		if err != nil {
			return toolResult(nil, err)
		}
		return toolResult(map[string]any{"tasks": tasks, "errors": errs}, nil)
	})

	type dryRunIn struct {
		Epic           string `json:"epic" jsonschema:"epic folder to scan, e.g. PROJ-855 (required: scans only this epic, not the whole tasks tree)"`
		PublishUnready bool   `json:"publish_unready" jsonschema:"publish ready:false tasks"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "dry_run",
		Description: "Parse one epic's tasks and show what would be created/updated without writing to JIRA. Always ask which epic to work with; this never scans the whole tasks tree.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in dryRunIn) (*mcp.CallToolResult, map[string]any, error) {
		items, err := svc.DryRun(ctx, in.Epic, in.PublishUnready)
		return toolResult(map[string]any{"items": items}, err)
	})

	type createOneIn struct {
		File           string `json:"file" jsonschema:"path to the .md task file"`
		PublishUnready bool   `json:"publish_unready" jsonschema:"create ready:false drafts"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "create_jira_task",
		Description: "Create or update a single JIRA issue from a markdown task file",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in createOneIn) (*mcp.CallToolResult, map[string]any, error) {
		rep, err := svc.CreateJiraTask(ctx, in.File, in.PublishUnready)
		if err != nil {
			return toolResult(nil, err)
		}
		return toolResult(rep, nil)
	})

	type createAllIn struct {
		Epic           string `json:"epic" jsonschema:"epic folder to process, e.g. PROJ-855 (required: processes only this epic, not the whole tasks tree)"`
		PublishUnready bool   `json:"publish_unready" jsonschema:"publish ready:false tasks"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "create_all_tasks",
		Description: "Create or update all markdown tasks in one epic folder in prefix order. Always ask which epic to work with; this never processes the whole tasks tree.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in createAllIn) (*mcp.CallToolResult, map[string]any, error) {
		reports, err := svc.CreateAllTasks(ctx, in.Epic, in.PublishUnready)
		return toolResult(map[string]any{"reports": reports}, err)
	})

	type pullIn struct {
		Epic string `json:"epic" jsonschema:"epic folder to download into, e.g. PROJ-855 (required over MCP: a server has no meaningful working directory)"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "pull_epic",
		Description: "Download every issue of one epic from JIRA into <epic>/in_jira as markdown task files (read-only against JIRA; writes files). This is the input of the analyst's scenario B — reviewing and completing tasks that already exist.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in pullIn) (*mcp.CallToolResult, map[string]any, error) {
		res, err := svc.PullEpic(ctx, in.Epic)
		return toolResult(res, err)
	})
}

func toolResult(out any, err error) (*mcp.CallToolResult, map[string]any, error) {
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
			IsError: true,
		}, nil, nil
	}
	if m, ok := out.(map[string]any); ok {
		return nil, m, nil
	}
	raw, _ := json.Marshal(out)
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	if m == nil {
		m = map[string]any{"result": out}
	}
	return nil, m, nil
}
