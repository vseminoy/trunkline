---
name: analysis-state
description: Maintains the persistent analysis state in a file. Use when an analyst session starts, to recover which step the work stopped at, which tasks are unfinished and which open questions remain; and at the end of every step, to record progress. Shared across all three scenarios (from scratch / review / closing questions).
---

# The analysis state

## Overview

The analysis state is the analyst's only persistent carrier of state between sessions. It is a single
markdown file, `<tasks.analysis_dir>/<EPIC>.md`, and it answers three
questions whenever work resumes: **which step we stopped at**, **which tasks are still
unfinished** and **which open questions remain**. Without it, every new session starts the analysis
over and loses the assumptions and decisions already agreed.

The state also holds a **materials registry** (the DB schema, the previous code, the goal, specs and
so on) with stable IDs, and the **rationale behind the estimates** — so that references are not lost
and estimates can be explained and revised between sessions.

It holds one thing that is not the analyst's own state at all: the **`R<n>` → task trace**. That
table answers the analyst's own question — is every requirement anchored to exactly one task — and
step 4 rests on it. The numbers stop at the edge of the epic: task bodies carry requirement *text*,
and the developer assigns the identifiers that end up in code. What the state still gives the
developer, when they can reach it, is the picture no single task text holds — which requirements went
to the epic's *other* tasks. The summary report shows the same table in a reply and dies with the
session, so the state is a deliverable, not a scratch file.

The state is **an internal file, not a task**: it has no task front-matter
(`project/summary/ready/type`) and must never reach JIRA.

## When to Use

- **At the start of every session** — load the state and continue from the recorded step; when
  filling in "Context" and "Materials" for the first time, check `.pipeline/knowledge/` first (the
  `core/skills/project-knowledge/SKILL.md` skill).
- **At the end of every `[STOP]` step** and before the session ends — update the state.
- When the user asks "where did we stop / what is left / which questions are open" — the answer comes
  from the state.
- When switching between scenarios A/B/C — the state is shared and carries the context across.

**When NOT to use:** a one-off question about the code with no decomposition; the state is
unnecessary if there will be no task artefacts.

## Process

### Step 1: Load or create the state

1. **Derive the path — do not ask for it.** It is `<tasks.analysis_dir>/<EPIC>.md`: the directory
   comes from the profile manifest, the epic key from the command argument, and both are required.
   Nothing is left to agree, so nothing is asked; say which path you resolved.
2. Read that file.
   - **It exists** → read it in full. Recover: the scenario, the current step, the context, the
     task table, the open-questions registry, the ASSUMPTIONS and "Next action". Show the user a
     short summary and offer to continue from there.
   - **It does not** → this is a new epic. Create the file at that same path on the first save.

### Step 2: Keep the state current

Update the relevant sections after every step. The state is the source of truth; the `STATE` block in
each step's output must be a short projection of it.

### Step 3: Save it safely (not as a task)

The publisher's scanner treats a file with no valid front-matter as an invalid task, so the state
must go where the scanner never looks. `<tasks.analysis_dir>/<EPIC>.md` is exactly that: the scan
runs over **one epic folder at a time** and reads only the `.md` files directly inside it, skipping
every subdirectory, and the analysis directory is a sibling of the epic folders rather than a file
within one. It is out of reach by construction, not by an exclusion someone has to maintain.

**That path is the only one — writing and reading both.** There is no menu and no question: the
directory is a profile slot, the epic key is a required argument, and a state that could be in any of
several places is a state the developer's step 1 does not find. A carrier nobody can locate is the
same as no carrier, and what it carries is the `R<n>` → task trace.

If the profile declares no `tasks.analysis_dir`, that is a defect in the profile: say so, write the
state beside the epic's task folder rather than inside it, and name the path in the final reply.

## Structure of the state file

The full template — every section, with the example rows that show the expected form — is in
[assets/state-template.md](./assets/state-template.md). Read it in full when creating a new state
file or checking one against the format; it is not reproduced here; a summary is exactly where a
"Materials" row or an "Estimates" column would quietly stop matching what "Tasks" expects.

The sections it holds: **Meta** (epic, scenario, current step, session status) · **Context** (AS-IS /
TO-BE, briefly) · **Materials** (the reference registry, stable `M<n>` IDs — check
`.pipeline/knowledge/` first via `core/skills/project-knowledge/SKILL.md` before gathering by hand) ·
**Requirements (EARS) and coverage** (the `R<n>` → task trace the developer's pipeline reads at its
step 1 — copied verbatim from the step 2 registry, never reworded) · **Tasks** · **Estimates** (basis
by material ID) · **Open questions** · **Assumptions** · **Decision log** · **Next action**.

## Common Rationalizations

| Rationalization | Reality |
|---|---|
| "I remember everything, the state is unnecessary" | Session context is finite and gets reset. The state survives a compaction and a new session. |
| "I will update the state at the end" | By the end some decisions are forgotten. Update at every `[STOP]`. |
| "I will drop the state next to the tasks as it is" | Without front-matter the publisher's scanner will treat it as an invalid task. `<tasks.analysis_dir>/` is out of its reach by construction — write it there, not beside the tasks. |
| "The open questions are in the report anyway" | A report is shown and then lost. The question registry in the state survives the session and drives scenario C. |

## Red Flags

- Starting the analysis without checking whether a state already exists in the directory.
- AS-IS gathered from the code from scratch even though the project has `.pipeline/knowledge/<project>/modules/` with a ready description.
- The analyst editing files in `.pipeline/knowledge/` — only step 20 of the developer may write there.
- The state has not been updated for several steps and has drifted from reality.
- A reference to a material (the DB schema, previous code, a spec) written straight into a task or
  an estimate instead of the "Materials" registry — it gets duplicated and lost on updates.
- An estimate in the "Tasks" table that does not match the value in the "Estimates" section, or an
  estimate with no basis (a material ID) — it is unclear what it rests on.
- The state has no "Next action" section — on resuming it is unclear what to do.
- The requirements-and-coverage table is missing, or holds anchors without statements: the reader
  gets "R7 → C2" and no way to learn what R7 says. A number without its statement is worth nothing
  outside this epic — and the statement is the only thing that crosses to development.
- A requirement's statement in the table paraphrased instead of copied — a second, drifting version
  of the same requirement.
- The state saved as an ordinary task (with task front-matter) — it will end up in JIRA.
- The `STATE` block in the output has drifted from the state file.

## Verification

- [ ] At start the state was found and read (or a new one was deliberately created).
- [ ] The path was derived from `tasks.analysis_dir` + the epic key, named in the reply, and is not
      picked up by the scanner as a task.
- [ ] Filled in: Meta, Context, the Materials registry, the requirements-and-coverage table, the
      Tasks table, Estimates, the open-questions registry, Next action.
- [ ] Every `R<n>` from step 2 is in the requirements-and-coverage table, with its statement copied
      verbatim and an anchor of one of the three kinds; "existing task" and "dropped" are justified.
- [ ] Every estimate has a basis (a material ID); the value matches the "Estimate" column.
- [ ] References to artefacts are kept in the "Materials" registry by ID rather than duplicated through the text.
- [ ] The state was updated at the end of the step; the `STATE` block in the output matches the file.
