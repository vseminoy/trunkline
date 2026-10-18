---
name: decompose-from-scratch
description: Breaks a new epic down into Jira tasks for the first time, or writes up one task of an epic when only that one is asked for. Use when no tasks exist yet and the epic directory is empty — all you have is a description of the work to be done (a feature, a bug, a raw idea) that still has to be clarified, or one such description and its epic key. Runs the full analyst pipeline: clarify, split into tasks, estimate each, order them by dependency, and write the task files ready for publishing to Jira.
---

# Scenario A: decomposition from scratch

## Overview

Take a task, understand it, split it into tasks (Problem → Solution → How to test), estimate each
one for a mid-level developer, and order them by dependency. It applies when **nothing has been
decomposed yet**.

The mode is read-only with respect to the code (see `rules/analytic-rules.md`). The only artefacts
are the task files per the format spec, the analysis state and the summary report.

## When to Use

- The epic's task directory is empty, or there are no tasks for this task yet.
- What you have is a description of a feature / bug / idea that must become a set of tasks.

**When NOT to use:** the tasks already exist and need reviewing or estimating →
`review-and-complete-tasks`; open questions on finished tasks need closing →
`resolve-open-questions`.

## Process

**Scenario A has no process of its own: its body is the pipeline.** The steps are one
file each under `core/analytic/steps/` — Clarifier (0) → Diagnostician (1) → Analyst (2) → Impact
Analyst (3) → Decomposer (4) → Estimator (5); the table that maps a step to its file is in
`core/analytic/PIPELINE.md`, section "The steps". Read a step there, immediately before running it,
and follow it literally.

This file deliberately does **not** restate them. Most of what makes those steps work is in their
hard stopping criteria — the five checkable conditions that replaced "~95% certainty" at step 0, the
requirements registry in EARS that is the main output of step 2, the "more than 3 layers or 2 tables"
signal for splitting at step 4, the doubt cycle's contract — and a summary is exactly where that
detail is lost. A second copy is a second thing to keep in sync, and the one that used to live here
had already drifted on four of those points.

What belongs to the scenario rather than to the steps:

- **Entering.** Load or create the state first (`analysis-state`, scenario = A). After every step —
  a `[STOP]` and a state update.
- **Everything is built, not reconciled.** Unlike B and C there is no existing task to work
  against and no registry to reconstruct: step 2 builds the `R<n>` registry from scratch, and it is
  the source every later step reads.
- **Leaving.** The `R<n>` → task trace goes into the state's "Requirements (EARS) and coverage"
  table — that is what the developer's pipeline reads at its step 1. The summary report is shown in
  the reply as a view of the same table. Both are described at step 5.

Estimation — the size table, the decay rates per layer, the multiplier for a thin proxy, the flat
rate for migrations — is in the Estimator (step 5) and is not summarised here either: a copy of that
table without the decay rules produces estimates roughly twice too large.

## One task instead of an epic

The input is a description of **one** task and the key of the epic it belongs to, and what is asked
for is that task written up — not a decomposition. It is the same scenario with the area narrowed to
one task, not a scenario of its own: what makes a task usable by a developer is the requirements
registry of step 2 and the write-up rules of step 5, and those are the two things a separate,
summarised route would lose.

**The route is `1 → 2 → 3 → 4 → 5`.** Step 0 runs when the description is vague, as everywhere.

**Step 4 does not split — it checks.** There is nothing to divide, so what remains of it is the part
that is not about dividing: the 24 h ceiling and the doubt cycle. If the requirements of step 2 add
up to more than 24 hours, that **is** the answer to the request: what arrived as one task is
several, and the split is the deliverable rather than a failure to produce the single file asked
for. Say it in those terms and let the user decide. Of its stopping criteria, 1, 2, 4 and 5 apply
unchanged (nothing lost, nothing anchored twice, no requirement cut, no `IF … THEN` torn from its
`WHEN`); 3 is trivially true with one task; 7 reduces to naming what this task must follow.

**The numbers continue the epic's, they do not restart.** The state is the same file,
`<tasks.analysis_dir>/<EPIC>.md`. If it exists, read the "Requirements (EARS) and coverage" table and
number from after the highest `R<n>` in it: the numbers run across the **epic** and are never
reused, and a second task numbered from `R1` would collide with the first the moment both are read
together. If it does not exist, create it — this task's requirements are the epic's first, and the
next run continues after them. Either way the coverage table gains rows anchored to this task, and
the requirements of the epic's other tasks, where the file already knows them, are what tells you
what this task must **not** cover.

**The estimate is produced as usual** — step 5 in full, with the decay rates, the character
multiplier and the flat rate for migrations. The summary report degenerates to one row, and that is
fine; what does not degenerate is the "Risks and open questions" block and the rule about a
divergence of more than 1.5× between the two ways of estimating, which on a single task is the only
signal that its boundaries are wrong.

**Publishing.** One file means the publisher's single-task operation
(`tasks.publisher.operations.publish_one`) rather than the whole-epic one. It creates or updates by
the same rule the format spec gives: a file whose front-matter already carries the task's tracker key
**updates** that issue, one without it creates a new one. So a task that already exists in the
tracker keeps its key and its history — check that the key is in the file before proposing the
operation, because publishing without it is what creates the duplicate. As everywhere on this track,
the user runs it.

## Common Rationalizations

| Rationalization | Reality |
|---|---|
| "The task is clear, I will skip the Clarifier" | If it has no acceptance criteria, it is not clear. Clarifying is cheap now. |
| "I will split horizontally: all the DB first, then the handlers" | Horizontal slices produce no working increments. Cut vertically. |
| "This task is ~30 h, I will leave it" | The 24 h ceiling is hard. Send it back to the Decomposer and split it. |
| "I will just save the files, the directory is obvious" | It *is* obvious — it is `<tasks.dir>/<EPIC>/`, from the manifest. Announce the resolved path, agree the front-matter, then save. Asking which directory is how an answer that disagrees with the slot gets in. |
| "One task, so I will skip the registry and just write the file" | The "How to test" block *is* the registry, one item per requirement. Skip step 2 and the developer's step 7a has nothing to count against — the one mechanical check spanning the two tracks. |
| "One task, so numbering starts at R1" | The numbers run across the epic. Read the state and continue from the highest one. |
| "I know the steps, I will work from this file" | This file is a router. The steps and their stopping criteria are in `core/analytic/steps/`; working from a summary is how the registry and the doubt cycle get skipped. |

## Red Flags

- Decomposition started with no context gathered and no ASSUMPTIONS.
- A task with no clear Problem / Solution / How to test.
- The tasks are not ordered by dependency; the foundations are not first.
- Files saved without reading the format spec and without agreeing it with the user.
- No state was created — the next session will start over.
- A step passed without its stopping criterion being checked, because the criterion was never read.

## Verification

- [ ] The state is created (scenario A), with the context and the open-questions registry filled in.
- [ ] The `R<n>` registry of step 2 exists and the trace reached the state (step 5, stopping
      criterion 3).
- [ ] Every task is ≤ 24 h and written per the format spec.
- [ ] The tasks are ordered by prefix following the graph's topology, foundations first.
- [ ] The task path was resolved from `tasks.dir` + the epic key and announced, and the
      front-matter was agreed before saving (the paths are derived, not agreed — see
      `analysis-state`).
- [ ] The summary report with the estimates was shown to the user.
