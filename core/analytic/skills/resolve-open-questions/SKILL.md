---
name: resolve-open-questions
description: Returns to tasks that are already written and closes the open questions. Use when the decomposition is done but unresolved questions, unconfirmed assumptions or blockers (ready=false) remain. Works through the open-questions registry in the state until they are closed, updates the affected tasks and lifts the readiness gate.
---

# Scenario C: closing the open questions

## Overview

A decomposition rarely finishes cleanly: questions for the product or the team remain, along with
unconfirmed assumptions and tasks with `ready: false`. This scenario is the controlled return to
those tasks: work through the **open-questions registry** in the state, get an answer to each one
(from the user or from the code), take a decision, **update the affected tasks** and lift the
readiness gate wherever the blocker is gone. The goal is to bring the tasks to `ready: true`.

The mode is read-only with respect to the code; task files are changed only with the user's
consent.

## When to Use

- The tasks are already written (through A or B) and the state holds a non-empty open-questions
  registry.
- There are tasks with `ready: false`, or unconfirmed ASSUMPTIONS blocking creation in JIRA.
- The user has come back with answers to questions asked earlier.

**When NOT to use:** there are no tasks → `decompose-from-scratch`; the tasks exist but have
not been checked or estimated → `review-and-complete-tasks`.

## Process

At the start — load the state (`analysis-state`, scenario = C). If there is no state, collect the
open questions with a `review-and-complete-tasks` pass first, and only then start closing them.

### 1. Collect the open items `[STOP]`
From the state, gather everything unclosed: open questions (`Q*`), unconfirmed ASSUMPTIONS, tasks
with `ready: false`. For each one, state **which tasks it blocks**. Prioritise: questions blocking
the foundation tasks (contract, schema, migration) come first.

### 2. Close each question `[STOP]` (one at a time)
For every question:
- state it concretely and show what depends on the answer;
- get the answer: **ask the user** (Manage Confusion: 2–3 options) or **check the code** (Verify
  Don't Assume — `grep`/`Explore`, read, do not guess);
- record the **decision** and its consequences for the tasks.

If the answer creates work that no task covers, that is a **new gap**: raise a task (as in
`review-and-complete-tasks`, step 5) or a new open question — do not lose it.

### 3. Update the affected tasks `[STOP]`
Following the decisions taken, edit the body / `links` / front-matter of the affected tasks
(strictly per the format spec). If the scope of a decision changed the estimate or pushed it past 24
hours — send it back to be split. **Edits only with the user's consent.**

### 4. Lift the readiness gate `[STOP]`
For tasks whose blocking questions are all closed and whose blockers (`links`) are resolvable,
move `ready: false → true`. Tasks with questions still open stay `ready: false` — they are not
created in JIRA until they are ready.

### 5. Update the state and the log `[STOP]`
Mark the questions `closed` in the registry, with the recorded answer. Record what changed in the
"Decision log". Recompute "Next action": what is still open, or "all questions closed, the epic is
ready to be created". **Output:** the updated task files + the state + a summary of the questions
closed and the blockers remaining.

## Common Rationalizations

| Rationalization | Reality |
|---|---|
| "I will set ready=true and we will sort it out later" | `ready=true` creates the task in JIRA. Lift the gate only after the blocker is really closed. |
| "The question is small, I will answer it myself" | If the answer affects behaviour or the contract, it is the user's or the team's decision. Ask. |
| "The answer does not change the tasks" | Check explicitly: an answer often changes the body, the estimate or `links`. |
| "A new question came up — I will write it down later" | An unrecorded question is lost. Into the state registry immediately. |

## Red Flags

- `ready` moved to `true` while a blocking question is still open.
- A question marked "closed" in the state with no recorded answer or decision.
- A decision taken without checking the code, or without the user's consent to edit the files.
- New gaps that surfaced while answering were not turned into tasks or questions.
- The state does not reflect what was left open after the session.

## Verification

- [ ] Every open question in the state was considered; each has an answer or an explicit "still open" status.
- [ ] The affected tasks are updated per the format spec, with the user's consent.
- [ ] `ready` was lifted only where the blockers are genuinely closed; estimates recomputed where needed.
- [ ] The question registry and the decision log in the state are current; "Next action" is updated.
- [ ] New gaps are raised as tasks or open questions rather than lost.
