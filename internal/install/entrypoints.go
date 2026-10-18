package install

import (
	"fmt"
	"regexp"
)

// The pipeline is a body of text an agent reads. How that text gets in front of
// an agent differs per tool, so entry points are generated rather than shipped:
// one neutral file every agent can read (.trunkline/AGENTS.md) and one native
// invocation per host that has a native form (Claude Code slash command).

// AgentsEntry renders .trunkline/AGENTS.md — the host-neutral entry point.
// A tool with no slash commands, no subagents and no hooks still has this.
//
// hasTasks adds the analytic track. It is conditional because the track needs a
// tracker: a profile with no `tasks` section has no place to put tasks, and
// naming a route that dead-ends is worse than not naming it.
func AgentsEntry(profileName, language string, hasTasks bool) []byte {
	langNote := "Reply in the language the user writes to you in; write artefacts in " +
		"the language the project already uses."
	if language != "" && language != DefaultLanguage {
		langNote = fmt.Sprintf("The `language` flag is `%s`: the prompts you read are "+
			"localized and artefacts default to that language when the project has no "+
			"language of its own. Your replies follow the language the user writes to "+
			"you in.", language)
	}
	analytic := ""
	if hasTasks {
		analytic = "\n" + `## The other track: analysis

The pipeline above turns a decided task into code. The analytic track comes
before it: an epic becomes EARS requirements, then tasks ready for the tracker.
Its entry point is ` + "`.trunkline/core/analytic/PIPELINE.md`" + ` — read that
instead of the developer core, plus the same profile manifest. Its steps load the
same way as the developer's: one file at a time from
` + "`.trunkline/core/analytic/steps/aNN-*.md`" + `, immediately before the step
runs, never all upfront. Where the tasks
and the analysis state are written, and which tool publishes them, come from the
manifest's ` + "`tasks`" + ` section; the paths there are relative to this
project, and publishing is always started by the user, never by you.

The bridge between the tracks is the requirement itself, as text: the analyst
writes it into the task, and step 2 of the developer pipeline is what gives it an
identifier.
` + "\n"
	}
	return fmt.Appendf(nil, `# trunkline — agent entry point

You are running the trunkline development pipeline in this project. This file is
host-neutral: it names what to read and in which order. It assumes nothing about
subagents, slash commands or hooks — see `+"`.trunkline/core/ORCHESTRATION.md`"+`
for what your host can and cannot do.

## Read, in this order

1. `+"`.trunkline/core/developer/PIPELINE.md`"+` — the core: routes, gates,
   traceability, output format. Read once, keep for the whole run.
2. `+"`.trunkline/profiles/%[1]s/profile.yaml`"+` — the stack profile named by the
   `+"`profile`"+` flag below. Read once, in full, right after the core.
3. Step files, one at a time, immediately before executing that step:
   `+"`.trunkline/core/developer/steps/NN-*.md`"+`. Do not read them all upfront.
   If the profile declares a `+"`step_overlays`"+` entry for the step, read that
   straight after the step file.

## Where the settings live

`+"`## Pipeline flags`"+` in this project's `+"`AGENTS.md`"+`: which profile, which
language, where the knowledge base lives, whether the run journal is kept. An
unset flag means strict behaviour, not "skip the check". Announce every flag in
the opening announcement, before the first step.

## Language

%[2]s

## Path resolution

Paths starting with `+"`.trunkline/`"+` are the vendored pipeline. Paths starting
with `+"`.pipeline/`"+` and `+"`.claude/`"+` are this project's own state. Both are
relative to the project root, so no path rewriting is needed.

Inside the profile manifest, paths are relative to the profile's own directory
(`+"`guidelines/…`"+` means `+"`.trunkline/profiles/%[1]s/guidelines/…`"+`) —
except the `+"`tasks.dir`"+` and `+"`tasks.analysis_dir`"+` slots, which name
this project's data and are relative to the project root.
%[3]s
## The rule that matters most

Never start with code, and never call the task finished before step 20.
`, profileName, langNote, analytic)
}

// PipelineRoot is where the pipeline sits inside an installed project. It is
// the prefix of every path a generated entry point names.
//
// A checkout of trunkline itself is the other layout the core describes: there
// `core/` and `profiles/` are at the repository root, so the prefix is empty.
// Passing that empty root to the generators is what lets this repository run the
// same command text it ships instead of keeping a hand-written twin of it —
// which is how the twin came to say things the core contradicts.
const PipelineRoot = Root + "/"

// ClaudeCommandFile renders the Claude Code slash command for a pipeline rooted
// at root (PipelineRoot in a project, "" in a checkout of trunkline). It is the
// native form of the same entry point, not a second source of truth: it points
// at the core and the profile and adds nothing of its own.
//
// An empty profileName renders the command for a project that has not declared
// one; the text already says to stop and ask, so nothing needs inventing here.
func ClaudeCommandFile(profileName, root string) []byte {
	current := "no profile is declared in this project yet"
	if profileName != "" {
		current = "currently `" + profileName + "`"
	}
	where := "`" + root + "` is the pipeline"
	// The description follows the same split: in a checkout of trunkline the
	// command runs the pipeline on trunkline itself, and saying so is what
	// distinguishes it from the /trunkline-dev a project installs. Deriving it
	// here rather than editing the generated file by hand is the whole point —
	// a hand edit is how the previous twin drifted.
	what := "Run the trunkline development pipeline"
	if root == "" {
		where = "the pipeline is this repository itself, so its paths have no prefix"
		what = "Run the trunkline pipeline on this repository itself"
	}
	return fmt.Appendf(nil, `---
description: %[4]s (AI Senior Developer)
---

Read `+"`%[1]score/developer/PIPELINE.md`"+` and follow it as the AI Senior
Developer. That file is the core and is stack-agnostic.

The stack — language, build commands, guidelines, layers, step overlays — comes
from the profile:

- the profile name is in the `+"`## Pipeline flags`"+` block of this project's
  `+"`AGENTS.md`"+` (`+"`profile: <name>`"+`; %[2]s);
- the manifest is `+"`%[1]sprofiles/<name>/profile.yaml`"+` — read it in
  full right after the core;
- if no profile is declared, stop and ask which to apply. The candidates are the
  directories in `+"`%[1]sprofiles/`"+`. Do not guess from the code: the
  wrong profile silently disables half the cross-checks.

The same block declares the knowledge base (`+"`knowledge`"+`: a path, an
external path, or `+"`none`"+`) and the run journal (`+"`metrics`"+`). Announce
the run number, the profile and its version, the knowledge base and every flag in
the opening announcement, before the first step.

Step instructions live in `+"`%[1]score/developer/steps/`"+` and are read
one at a time, immediately before the step — not upfront. If the profile
declares an overlay for a step (`+"`step_overlays`"+`), read it right after the
step file.

Cross-cutting material: `+"`%[1]score/skills/project-knowledge/SKILL.md`"+`
(knowledge base), `+"`%[1]score/developer/references/`"+` (run journal),
`+"`%[1]score/developer/ENFORCEMENT.md`"+` (hooks and machine gates),
`+"`%[1]score/ORCHESTRATION.md`"+` (what your host can and cannot do).

Everything is relative to the project root. `+"`.pipeline/`"+` and `+"`.claude/`"+` are
this project's state; %[3]s.

The user's task: $ARGUMENTS
`, root, current, where, what)
}

// ClaudeQuickCommand renders the Claude Code slash command for the short-change
// route. That route does not enter the pipeline at all — its conditions, its
// skills and its gates are in core/developer/QUICK.md, and this file only points
// at it and at the profile.
//
// root works as it does for ClaudeCommandFile: PipelineRoot in a project, "" in
// a checkout of trunkline.
func ClaudeQuickCommand(profileName, root string) []byte {
	current := "no profile is declared in this project yet"
	if profileName != "" {
		current = "currently `" + profileName + "`"
	}
	what := "Make a short change without running the pipeline"
	// The route hands an oversized task back to the full pipeline, so it must
	// name the command that actually exists here: a checkout of trunkline runs
	// the pipeline on itself under /trunkline-self.
	full := "/trunkline-dev"
	if root == "" {
		what = "Make a short change to this repository without running the pipeline"
		full = "/trunkline-self"
	}
	return fmt.Appendf(nil, `---
description: %[3]s, under the profile's rules
---

Read `+"`%[1]score/developer/QUICK.md`"+` and follow it. It is a route of its
own, not a lighter pipeline: no steps, no step files, no subagents, no
approvals. What it uses is the skill
`+"`%[1]score/skills/profile-conformant-change/SKILL.md`"+` — how code is written
under this project's profile — plus the three skills that route names, and the
build and the lint gate, unchanged.

Read the route file first, then check the task against the conditions it lists
**before** touching anything. A task that fails one of them does not run here:
say which condition failed and hand it to `+"`%[4]s`"+`.

The stack comes from the profile:

- the profile name is in the `+"`## Pipeline flags`"+` block of this project's
  `+"`AGENTS.md`"+` (`+"`profile: <name>`"+`; %[2]s);
- the manifest is `+"`%[1]sprofiles/<name>/profile.yaml`"+` — read it in full
  before the change: `+"`commands.*`"+` are the gates, `+"`guidelines`"+` are the
  rules the change must obey, `+"`layers`"+` say where it sits;
- if no profile is declared, stop and ask. Do not guess from the code.

Everything is relative to the project root. `+"`.pipeline/`"+` and
`+"`.claude/`"+` are this project's state.

The user's task: $ARGUMENTS
`, root, current, what, full)
}

// ClaudeAnalyticCommand renders the slash command for the analytic track. Like
// the developer one it states nothing of its own — it points at the analytic
// core and the same profile manifest.
//
// It takes no paths: the previous hand-written version hardcoded an absolute
// path to one developer's checkout and the central task directory inside it,
// which is exactly what made the track unusable anywhere else.
func ClaudeAnalyticCommand(profileName string) []byte {
	return fmt.Appendf(nil, `---
description: Run the trunkline analysis pipeline (AI Analyst) for an epic
argument-hint: <EPIC_KEY>
---

Epic key (required): **$1**

If `+"`$1`"+` is empty, do not continue. Print
`+"`usage: /trunkline-analytic <EPIC_KEY>`"+` and stop.

Read `+"`.trunkline/core/analytic/PIPELINE.md`"+` in full and follow it literally
as the AI Analyst: the working mode, the scenario router, the step table and the
output format all live there. The steps themselves are separate files —
`+"`.trunkline/core/analytic/steps/aNN-*.md`"+`, read one at a time immediately
before running that step, not upfront. You do not write production code and you do
not change existing project files.

The profile is `+"`%s`"+` (from the `+"`## Pipeline flags`"+` block of this
project's `+"`AGENTS.md`"+`); its manifest is
`+"`.trunkline/profiles/%[1]s/profile.yaml`"+`, read in full at start. From it
come the layers, the guidelines and the `+"`tasks`"+` section below. If no profile
is declared, stop and ask — do not guess.

## Where the work goes

Everything comes from the manifest's `+"`tasks`"+` section; do not invent paths:

- tasks: `+"`<tasks.dir>/$1/`"+` — relative to this project's root, committed
  with the code they describe. Announce the resolved path before writing; do not
  ask which directory to use, it is the slot's answer. If the directory does not
  exist, say so and ask before creating anything.
- analysis state: `+"`<tasks.analysis_dir>/$1.md`"+` — one file, that path, both
  to read and to write. Found one? Continue from the recorded step instead of
  starting over.
- already-created tasks pulled from the tracker: `+"`<tasks.dir>/$1/in_jira/`"+` —
  the input for scenario B.
- the task format — **two documents, read in this order**:
  1. `+"`tasks.publisher.contract`"+` (relative to `+"`.trunkline/`"+`) — what the
     publisher's code requires. Breaking it means the task is refused.
  2. `+"`tasks.format_spec`"+` (relative to the profile directory) — the team's
     conventions for what goes inside those fields.

  Read both before writing any file and follow them literally.

## Boundaries

This command creates nothing in the tracker. You prepare the `+"`.md`"+` files and
the summary report, then propose the publisher's operations
(`+"`tasks.publisher.operations`"+`) to the user, in order: dry run first, publish
second. **The user starts them.** Publishing is irreversible and happens outside
the repository. Setting the publisher up is described in
`+"`tasks.publisher.setup`"+`.

The user's task: $ARGUMENTS
`, profileName)
}

// pointer is appended to the project's own AGENTS.md so an agent that only ever
// reads the conventional root file still finds the pipeline.
// AgentsPointer is exported so `doctor` can tell an AGENTS.md that only
// carries this block from one that also carries the project's standards.
const AgentsPointer = `
## trunkline pipeline

This project runs the trunkline development pipeline. Entry point:
` + "`.trunkline/AGENTS.md`" + `. Settings: the ` + "`## Pipeline flags`" + ` block
in this file. Do not start on code before reading them.
`

// EnsureAgentsPointer appends the pointer block to the project's AGENTS.md,
// creating the file if absent. Idempotent, and never rewrites what is there.
func (w *Writer) EnsureAgentsPointer() (Status, error) {
	// Not hashed into state: AGENTS.md is the project's file.
	return w.appendBlock(AgentsFile, "## trunkline pipeline", AgentsHeader, AgentsPointer)
}

// entryProfile matches the profile name inside a generated entry point. Both
// forms are covered: the manifest path the neutral entry and the analytic
// command point at, and the `currently `<name>“ note in the dev command.
var entryProfile = regexp.MustCompile(
	"profiles/([^/`<]+)/profile\\.yaml|currently `([^`]+)`")

// EntryProfile reports which profile a generated entry point was written for,
// or "" if the file names none. It exists so `doctor` can compare the entry
// points against the flags: the two are written at different times and nothing
// keeps them in step.
func EntryProfile(b []byte) string {
	for _, m := range entryProfile.FindAllSubmatch(b, -1) {
		for _, g := range m[1:] {
			if len(g) > 0 {
				return string(g)
			}
		}
	}
	return ""
}
