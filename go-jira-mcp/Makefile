.PHONY: build cli run test vet tidy clean \
	check_connection find_epic_field list_markdown_tasks dry_run \
	create_jira_task create_all_tasks pull_epic

# PUBLISH=1 adds --publish-unready to the task commands that support it.
# list_markdown_tasks, dry_run and create_all_tasks each scan ONE epic folder:
# name it positionally (make dry_run PROJ-855) or as EPIC=PROJ-855. They never scan
# the whole tasks tree.
# create_jira_task takes the task file either positionally
# (make create_jira_task path/to/task.md) or as FILE=path/to/task.md.
# pull_epic takes the epic folder positionally (make pull_epic tasks/PROJ-855)
# or as DIR=tasks/PROJ-855.
PUBLISH ?=
FILE ?=
DIR ?=
EPIC ?=
PUBLISH_FLAG := $(if $(PUBLISH),--publish-unready,)

build:
	go build -o bin/server ./cmd/server

cli:
	go build -o bin/cli ./cmd/cli

run: build
	./bin/server

test:
	go test ./...

vet:
	go vet ./...

tidy:
	go mod tidy

clean:
	rm -rf bin

# --- JIRA task commands (mirror the MCP tools) ---

check_connection: cli      ## GET /myself — verify URL and token
	./bin/cli check_connection

find_epic_field: cli       ## GET /field — find the "Epic Link" field id
	./bin/cli find_epic_field

list_markdown_tasks: cli   ## list one epic's tasks (read-only): make list_markdown_tasks PROJ-855 [or EPIC=PROJ-855]
	@epic="$(or $(EPIC),$(filter-out $@,$(MAKECMDGOALS)))"; \
	test -n "$$epic" || { echo "usage: make list_markdown_tasks <EPIC-123>  (or EPIC=<epic>)"; exit 2; }; \
	./bin/cli list_markdown_tasks "$$epic"

dry_run: cli               ## show what would be created for one epic, no writes: make dry_run PROJ-855 [PUBLISH=1] [or EPIC=PROJ-855]
	@epic="$(or $(EPIC),$(filter-out $@,$(MAKECMDGOALS)))"; \
	test -n "$$epic" || { echo "usage: make dry_run <EPIC-123> [PUBLISH=1]  (or EPIC=<epic>)"; exit 2; }; \
	./bin/cli dry_run $(PUBLISH_FLAG) "$$epic"

create_jira_task: cli      ## create/update one task: make create_jira_task path/to/task.md [PUBLISH=1]
	@file="$(or $(FILE),$(filter-out $@,$(MAKECMDGOALS)))"; \
	test -n "$$file" || { echo "usage: make create_jira_task <path/to/task.md> [PUBLISH=1]  (or FILE=<path>)"; exit 2; }; \
	./bin/cli create_jira_task $(PUBLISH_FLAG) "$$file"

create_all_tasks: cli      ## create/update one epic's tasks in batch: make create_all_tasks PROJ-855 [PUBLISH=1] [or EPIC=PROJ-855]
	@epic="$(or $(EPIC),$(filter-out $@,$(MAKECMDGOALS)))"; \
	test -n "$$epic" || { echo "usage: make create_all_tasks <EPIC-123> [PUBLISH=1]  (or EPIC=<epic>)"; exit 2; }; \
	./bin/cli create_all_tasks $(PUBLISH_FLAG) "$$epic"

pull_epic: cli             ## download an epic's issues into <dir>/in_jira: make pull_epic tasks/PROJ-855 [or DIR=path]
	@dir="$(or $(DIR),$(filter-out $@,$(MAKECMDGOALS)))"; \
	test -n "$$dir" || { echo "usage: make pull_epic <tasks/EPIC-123>  (or DIR=<path>)"; exit 2; }; \
	cd "$$dir" && "$(CURDIR)/bin/cli" pull_epic

# Treat extra command-line words (e.g. the task path passed positionally to
# create_jira_task) as no-op goals instead of unknown targets. Explicit targets
# above always take precedence over this catch-all.
%:
	@:
