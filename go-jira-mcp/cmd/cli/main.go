// CLI mirror of the MCP tools: each subcommand calls the same tools.Service
// method that internal/tools/mcp.go exposes over MCP, so terminal and MCP
// behaviour stay identical. Output is the same JSON payload the MCP tools
// return.
//
//	cli check_connection
//	cli find_epic_field
//	cli list_markdown_tasks
//	cli dry_run            [--publish-unready]
//	cli create_jira_task   [--publish-unready] <task.md>
//	cli create_all_tasks   [--publish-unready]
//	cli pull_epic          [<epic>]   (no arg: the current folder is the epic)
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"go-jira-mcp/internal/config"
	"go-jira-mcp/internal/tools"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	cmd, args := os.Args[1], os.Args[2:]

	cfg, err := config.Load()
	if err != nil {
		fail(err)
	}
	svc := tools.NewService(cfg)
	ctx := context.Background()

	switch cmd {
	case "check_connection":
		emit(svc.CheckConnection(ctx))

	case "find_epic_field":
		emit(svc.FindEpicField(ctx))

	case "list_markdown_tasks":
		epic, _ := epicArg(cmd, args, false)
		tasks, errs, err := svc.ListMarkdownTasks(ctx, epic)
		emit(map[string]any{"tasks": tasks, "errors": errs}, err)

	case "dry_run":
		epic, pub := epicArg(cmd, args, true)
		items, err := svc.DryRun(ctx, epic, pub)
		emit(map[string]any{"items": items}, err)

	case "create_jira_task":
		fs := flag.NewFlagSet(cmd, flag.ExitOnError)
		pub := fs.Bool("publish-unready", false, "create ready:false drafts")
		_ = fs.Parse(args)
		if fs.NArg() != 1 {
			fmt.Fprintln(os.Stderr, "usage: cli create_jira_task [--publish-unready] <task.md>")
			os.Exit(2)
		}
		rep, err := svc.CreateJiraTask(ctx, fs.Arg(0), *pub)
		emit(rep, err)

	case "create_all_tasks":
		epic, pub := epicArg(cmd, args, true)
		reports, err := svc.CreateAllTasks(ctx, epic, pub)
		emit(map[string]any{"reports": reports}, err)

	case "pull_epic":
		// The epic is optional here and only here: run inside an epic folder and
		// the folder names it, which is this command's original contract.
		res, err := svc.PullEpic(ctx, firstArg(args))
		emit(res, err)

	default:
		usage()
		os.Exit(2)
	}
}

// epicArg parses the required positional epic folder (e.g. PROJ-855 or
// tasks/PROJ-855) shared by the scan subcommands, plus the optional
// --publish-unready flag when withPublish is set. Each scan is scoped to one
// epic, so the epic must always be named; a missing one is a usage error.
func epicArg(cmd string, args []string, withPublish bool) (epic string, publish bool) {
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	var pub *bool
	if withPublish {
		pub = fs.Bool("publish-unready", false, "publish ready:false tasks")
	}
	_ = fs.Parse(args)
	if fs.NArg() != 1 {
		suffix := ""
		if withPublish {
			suffix = " [--publish-unready]"
		}
		fmt.Fprintf(os.Stderr, "usage: cli %s%s <epic>   (e.g. %s PROJ-855)\n", cmd, suffix, cmd)
		os.Exit(2)
	}
	if pub != nil {
		publish = *pub
	}
	return fs.Arg(0), publish
}

// firstArg returns the optional positional argument, or "" when there is none.
// Only pull_epic uses it: every other epic-scoped command requires the epic
// (epicArg), while there the empty value means "the current folder is the epic".
func firstArg(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[0]
}

// emit prints the result as indented JSON, or the error to stderr, exiting
// non-zero on failure.
func emit(out any, err error) {
	if err != nil {
		fail(err)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func usage() {
	fmt.Fprintln(os.Stderr, `usage: cli <command> [flags] [args]

commands:
  check_connection                          verify JIRA URL and token (GET /myself)
  find_epic_field                           discover the Epic Link field id (GET /field)
  list_markdown_tasks <epic>                scan one epic folder and list tasks (read-only)
  dry_run            [--publish-unready] <epic>   show what would be created/updated, no writes
  create_jira_task   [--publish-unready] <task.md>   create/update one task from a file
  create_all_tasks   [--publish-unready] <epic>   create/update all tasks in one epic folder
  pull_epic          [<epic>]               download all issues of one epic into <epic>/in_jira
                                            (no argument: the current folder is the epic)`)
}
