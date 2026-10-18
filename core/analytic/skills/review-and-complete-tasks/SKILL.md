---
name: review-and-complete-tasks
description: Reviews tasks that already exist, estimates the ones without an estimate, and adds the missing ones. Use when the epic directory already holds a set of tasks that need looking over, checking against the format and for coverage completeness, estimating where estimates are absent, and completing with the tasks that were missed. The decomposition already exists; this puts it in order.
---

# Scenario B: reviewing, estimating and completing tasks

## Overview

When a decomposition is already partly done (yours or someone else's), the analyst's job is not to
rewrite it but to **put it in order**: inventory the existing tasks, check each against
the format spec and for completeness (Problem / Solution / How to test), **add estimates where there
are none**, find **coverage gaps**, **write the missing tasks**, and then rebuild the dependency
order. The mode is read-only with respect to the code; task files are changed or added only with
the user's consent.

## When to Use

- The epic directory already holds tasks that need reviewing / estimating / completing.
- Some tasks have no estimate, some do not follow the format, and coverage looks like it has gaps.
- Someone else's decomposition has arrived for review.

**When NOT to use:** there are no tasks at all → `decompose-from-scratch`; the tasks are fine
but open questions or blockers remain → `resolve-open-questions`.

## Where the tasks come from

Two inputs, and the scenario runs on either:

- **`<tasks.dir>/<EPIC>/`** — tasks that live in the repository, written by an earlier run of
  scenario A or by a person.
- **`<tasks.dir>/<EPIC>/in_jira/`** — mirrors of issues that already exist in the tracker,
  downloaded by the publisher's `operations.pull`. This is what makes "someone else's
  decomposition has arrived for review" a route rather than a retyping exercise: the tracker is
  the source, and the mirrors carry the tracker key in their front-matter, so a later publish
  **updates** those issues instead of creating duplicates.

Both paths come from the manifest — resolve `tasks.dir` and append the epic key; never assume a
literal `tasks/`. If `in_jira/` is absent and the epic clearly exists in the tracker, say so and
propose `operations.pull` to the user, in the same way publishing is proposed: the operation is the
user's to run. If the profile declares no `operations.pull`, that input is simply not available in
this project — say it in one line rather than inventing a way to fetch the issues.

## Process

At the start — load or create the state (`analysis-state`, scenario = B).

### 1. Inventory `[STOP]`
Read every `.md` task in the epic directory (excluding `done/`), and the mirrors in `in_jira/`
where there are any. For each one, build a row of the
state table: prefix, file, summary, estimate, `ready`, `links`. Do not guess the contents — read the
files.

### 2. Format check `[STOP]`
Read the format spec (the profile's `tasks.format_spec`). For each task check:
- valid YAML front-matter, with the required `project`/`summary`/`ready`/`type` fields;
- a body of three blocks: **Problem → Solution → How to test**;
- `summary` conveys the business effect; `links` point at blockers that exist and are local;
- placement in the epic directory and a correct prefix (`A`<`B`<`C`, numbers compared as numbers).

Classify each one's analysis status: `ready` / `incomplete` / `off-format` / `needs an estimate` /
`needs rework` / `duplicate`.

### 3. Coverage analysis (finding what is missing) `[STOP]`
Check against the requirements to find the **missing** tasks. A light pass of the Impact Analyst
from `decompose-from-scratch` (the `deprecation-and-migration` skill): which layers / contracts /
migrations TO-BE says must be touched, and which of them **no existing task covers**. Typical
gaps: a migration with no database queries, a handler with no contract update, a feature with no
tests, a missing foundation task (schema/contract), no backward compatibility.

Your own view of someone else's decomposition quickly becomes complicit: run the **doubt cycle** (the
`doubt-driven-development` skill). The ARTEFACT is the set of existing tasks; the CONTRACT is
the format spec plus coverage completeness against the epic description; the reviewer gets it
**without** your conclusions. The question: which scenario from the description does no task
cover.

**Output:** the list of gaps → new tasks (drafts).

### 4. Filling in the estimates `[STOP]`
Give every task **with no estimate** one from the table below (the same criterion as in
`decompose-from-scratch`). Any task estimated at **> 24 h** goes back to be split (as in the
Decomposer). Estimates go in the summary report, not in the task files.

### 5. Write the missing ones and rebuild the order `[STOP]`
Write the new tasks strictly per the format spec, assigning **the next free prefixes** so that
blockers come before what depends on them. Recompute the dependency graph and the `links` of the
affected tasks. **Before saving or editing** ask for: confirmation to edit the existing files, the
front-matter fields of the new ones, and the epic/directory (if not already in the state).
**Output:** the updated file set + the summary report + the state (with the open questions found
during the review).

## Estimation

The size table and everything that goes with it — the decay rates per requirement type and layer, the
multiplier for a thin proxy, the flat rate for migrations, the separate line for retiring an old path
— are in the Estimator, `core/analytic/steps/a05-estimator.md`. Use them from there.

They are not summarised here on purpose: a copy of the size table without the decay rules overstates
a multi-requirement task by roughly a factor of two, which is exactly the mistake step 5 warns
about, and it splits tasks that were healthy.

## Review checklist for a single task

- [ ] The front-matter is valid and the required fields are present.
- [ ] The body: Problem / Solution / How to test — all three, none empty.
- [ ] `summary` is about the business effect, not the implementation.
- [ ] `links` are correct (the blocker exists and has a smaller prefix, or it is a JIRA key).
- [ ] An estimate is given and it is ≤ 24 h.
- [ ] The task is a complete vertical slice, not "a piece of a layer".
- [ ] It does not duplicate another task.

## Common Rationalizations

| Rationalization | Reality |
|---|---|
| "I will rewrite it all — that is faster" | That is scenario A. Here the point is to preserve what is done and build on it, not to throw it away. |
| "I will estimate by eye without reading the file" | An estimate made without reading the body and the affected layers is fiction. Read. |
| "The coverage looks complete" | Gaps are invisible without checking TO-BE layer by layer. Run the coverage analysis explicitly. |
| "I will fix someone else's files quietly" | Edits to existing tasks happen only with the user's consent. |

## Red Flags

- An inventory built from file names without reading their contents.
- New tasks with prefixes that break the blocker ordering.
- Estimates filled in without checking the 24 h ceiling.
- Gaps that were found were turned into neither tasks nor open questions.
- The state was not updated with the tasks' analysis statuses.

## Verification

- [ ] Every existing task has been read and classified in the state table.
- [ ] Each was checked against the format spec; the deviations are recorded.
- [ ] Every task without an estimate now has one; none exceeds 24 h.
- [ ] Coverage gaps are closed by new tasks or entered as open questions.
- [ ] The order (prefixes) and `links` are rebuilt from the dependency graph.
- [ ] Edits and additions were agreed before saving; the state and the report are updated.
