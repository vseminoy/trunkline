# Pipeline evals: measuring the process itself

This addresses the weakness that "there is observability for the product, but no quality metrics for
the pipeline". Without data you cannot cut steps deliberately: you do not know which step actually
catches defects and which one is ceremony.

The principle is the same as at step 16: questions first, instrumentation second.

## The questions the journal must answer

1. Which steps find defects, and which have found nothing at all across N slices?
2. Where do rollbacks happen most often (step 7a → 5/6/7, steps 9/10 → 8), and why?
3. What does a changeset cost: how many steps, how many user confirmations, how many iterations.
4. Do the delta mechanics pay off: the share of context reused from `.pipeline/knowledge/`, and the
   `STALE` findings.
5. How many traceability gaps step 7a catches — and whether that number falls over time. This one
   is answerable **only if the failing runs are in the journal too**. Step 7a reruns until it is
   clean, so recording just the final run makes a step that catches defects every week indexed as a
   step that has never caught one — the failure mode question 1 exists to prevent, arriving through
   question 5.
6. How often the machine half of 7a (`trace_check`) ran at all. A run of slices in which it is
   `unavailable` says the gate is decorative on that setup, and that is a fact about the setup, not
   about the code.

## Journal format

`.pipeline/workdir/metrics/pipeline.jsonl` in the project being developed, one line = one event, JSON.
It is only ever **appended to** (`>>`), never rewritten.

**It is not committed.** It measures the process, not the code, and an append-only file that every
branch writes to conflicts on every merge — always, because every writer appends at the same place.
It belongs in the project's `.gitignore` next to the other run directories.

A team may decide it wants the measurement across everyone: "which gate does this team skip
systematically" is a question one developer's history cannot answer. If it does, **split the journal
per run first** — `.pipeline/workdir/metrics/<NNN>.jsonl` instead of one appended file. Separate files merge
without conflicts, the report reads the directory, and the approval gate's state machine already
handles several runs in one stream, so it needs only to be given the files in name order. Committing
the single appended file is the one arrangement that does not work.

Mandatory fields: `ts` (ISO-8601), `slice`, `step`, `event`.

`slice` is the run number — the UTC start time, `YYMMDDHHMMSS`, minted at classification before
this file is first written (see "The run number" in `core/developer/PIPELINE.md`). It is not
allocated from this file: a journal that `metrics: none` switched off would leave step 2 with no
number at all, and a counter read from the last line collides the moment two developers run at
once. The tracker key of the task, where there is one, rides along as `task` on the `start` event —
beside the number, never inside it.

The examples below use a short `007` so the lines stay readable; a real value is the timestamp.

**Plus `changeset` on every event written inside the changeset loop** — the number step 4 gave it.
Steps 5, 6, 7, 7a, 8, 9+10, 11, 12, 15, 16 and 18 all run once per changeset, so an event from them
without it says which run it belongs to and not which piece of work. It is load-bearing on exactly
two of them: `approval` and `skipped` are what the commit gate counts, and it counts them per
changeset — without the field changeset 1's four approvals open every later commit of the task.
Routes with no step 4 have one changeset; the field may be omitted there and reads as `1`.

**Plus `session_id` on `start` and `done`, on a host that has one.** It is the only optional field
here, and it is optional because it is the only one whose value comes from the host rather than from
the pipeline. It costs one environment variable to write — under Claude Code, `CLAUDE_CODE_SESSION_ID`
— and it is what turns "what did this run cost in money" from a wall-clock guess into an answer: see
"What the run cost in money" below. Nothing reads it as a gate; a missing one degrades the cost
report and blocks nothing. Write it on both events, not one: a run interrupted and resumed the next
day has two, and the pair is what says so.

```jsonl
{"ts":"2026-08-05T10:00:00+03:00","slice":"007","step":0,"event":"start","session_id":"951d33f0-c9f3-490f-a11c-7128e80c33ad","task":"PROJ-855","task_type":"feature","steps_planned":[1,2,3,4,5,6,7,"7a",8,9,10,11,12,15,17,18,20]}
{"ts":"2026-08-05T10:05:00+03:00","slice":"007","step":1,"event":"delta_context","reused_modules":2,"rebuilt_modules":1,"stale":1}
{"ts":"2026-08-05T10:25:00+03:00","slice":"007","changeset":1,"step":"5","event":"approval","artefact":"plan.yaml + sequence_diagrams.md"}
{"ts":"2026-08-05T10:40:00+03:00","slice":"007","changeset":1,"step":"7a","event":"consistency","gaps":2,"mismatches":1,"lost":1,"origin":0,"trace_check":"fail","rollback_to":5}
{"ts":"2026-08-05T11:20:00+03:00","slice":"007","changeset":1,"step":"7a","event":"consistency","gaps":0,"mismatches":0,"lost":0,"origin":0,"trace_check":"pass"}
{"ts":"2026-08-05T12:10:00+03:00","slice":"007","step":9,"event":"review","critical":1,"important":3,"nit":5,"rollback_to":8}
{"ts":"2026-08-05T12:10:00+03:00","slice":"007","step":10,"event":"security","critical":0,"high":1,"medium_low":2}
{"ts":"2026-08-05T13:00:00+03:00","slice":"007","changeset":1,"step":16,"event":"skipped","reason":"not a hot path"}
{"ts":"2026-08-05T13:30:00+03:00","slice":"007","step":20,"event":"done","session_id":"951d33f0-c9f3-490f-a11c-7128e80c33ad","confirmations":14,"rollbacks":2,"duration_min":210}
```

## When a line is written (mandatory points)

The **main agent** always writes; subagents never write to the journal.

| Moment | event | Mandatory fields |
|---|---|---|
| After the route is agreed, before step 1 | `start` | `task_type`, `steps_planned`, `task` (the tracker key, if there is one), `session_id` (if the host has one) |
| Step 1, after the delta context | `delta_context` | `reused_modules`, `rebuilt_modules`, `stale` |
| Each of the MANDATORY APPROVALS the route requires, **the moment the user gives it** (steps 5, 6, 7, 7a), and step 21's on the `knowledge` route | `approval` | `step`, `changeset`, `artefact` |
| Step 7a, every run — **the failing ones included, written before the rollback** | `consistency` | `changeset`, `gaps`, `mismatches`, `lost`, `origin`, `trace_check` (`pass` / `fail` / `unavailable`), `rollback_to` (if there was one) |
| Steps 9/10, every run | `review` / `security` | `changeset`, findings by severity, `rollback_to` (if there was one) |
| Any step skipped as not applicable | `skipped` | `step`, `reason`, `changeset` (inside the loop) |
| Any rollback to an earlier step | `rollback` | `rollback_to`, `reason`, `changeset` (when the rollback happens inside the loop — which is every rollback from 7a, 9 or 10) |
| Step 20 — on `audit`, step 9+10 instead | `done` | `confirmations`, `rollbacks`, `duration_min`, `session_id` (if the host has one) — **each derived by reading this run's own lines back, see below** |

Writing is a single append command, with no full read of the file. It does not block the step: if
the write fails — report it and carry on; the journal must not become another gate.

**`done` is the one exception, and it has to be.** Its three fields are counts over the whole run,
so none of them can be produced by an agent that has not read the run back — and a mandatory field
with no way to obtain it is a field that gets written as `null`. Read this run's own lines once,
filter to this `slice`, and derive:

| Field | Derived from |
|---|---|
| `confirmations` | the number of `approval` events of this `slice` |
| `rollbacks` | the number of `rollback` events of this `slice` |
| `duration_min` | `ts` of `done` minus `ts` of `start`, in whole minutes |

One read, once per run, at the moment the run closes — that is not the per-event cost the rule
above exists to prevent. `null` in any of the three is not "the field was not applicable": it is a
lost data point, and it is invisible, because a `done` line with empty fields looks exactly like a
recorded one. It is what makes question 3 unanswerable for the run.

On `audit` there is no step 20 (see the route matrix in `core/developer/PIPELINE.md`), so step 9+10
writes the `done` line itself, with `knowledge: none`. A run left without `done` counts as
unfinished, and the approval gate treats it as such.

On `knowledge` step 20 is present and writes `done` as usual — that route changes no code but does
change the base, which is what step 20 is for. Its `approval` line is step 21's, and it is the only
one the run has: `approval-gate.sh` reads approvals for steps 5, 6, 7 and 7a only, so recording step
21 neither arms a gate nor satisfies one. It is recorded for the `confirmations` count and for the
record that a human read the difference table before an append-only entry was written.

**`steps_planned` names what the route intended, conditional steps included.** Put every step of the
route in it — `6` on `feature`, `6`, `7` and `7a` on `migration` — even though whether they apply is
not known when the line is written. A step that then turns out not to apply is retired by its
`skipped` event, which is why that event carries the step number and is not optional for a step the
route named. The pair is what lets the journal answer both "what did this route mean to do" and
"what did it actually do"; `steps_planned` alone answers neither.

**A skip is a fact about one changeset, not about the task.** Changeset 1 touching no database says
nothing about changeset 2, so a conditional step the route named is answered per changeset — either
an `approval` or a `skipped`, each carrying its `changeset`. Write the `skipped` line for the
changeset in hand rather than assuming an earlier one settled the question; the gate blocks the
commit otherwise, and its message names the step and the changeset it is waiting on.

`slice` identifies the run and does not change inside it — including across the changeset loop. One
task is one run: one `start`, one `done`, one knowledge entry listing every commit the run produced
(`Commits: a1b2c3d, e4f5g6h`). The events of the looped steps therefore appear N times under the same
`slice`, which is normal and not a duplicate. Events carrying a **different** `slice` belong to a
different run and are read as such.

**One exception, and it is deliberate: the `approval` event.** It is read back by the approval gate
(`core/developer/ENFORCEMENT.md`), so a missing line does not merely lose a data point — it blocks the
commit until the approval is recorded. That is the point: the line is what makes "the user approved
step 5" a fact a machine can check rather than a claim in a chat window. It is written when the
approval is **given**, not in a batch at the end, and never for an approval that was not given: the
journal is the only record that these gates ran at all, so a fabricated line is a skipped gate that
looks passed.

## The report

On the user's request ("pipeline report") or automatically every 10 slices, the main agent reads
`pipeline.jsonl` and produces:

```
OVER THE LAST N SLICES:
- Steps that found defects: 7a (5 gaps), 9 (3 Critical), 10 (1 High)
- Steps with no findings at all: 11, 15 → candidates for removal from the route
- Rollback points: 7a → 5 (×3) — rules regularly fail to reach the spec
- Delta mechanics: 68% of modules reused, STALE 4 times
- Average changeset cost: 15 confirmations, 2 rollbacks
```

The report is a proposal to change the pipeline, not an automatic change. Cutting steps is the
user's decision.

## What the run cost in money

Everything above measures the process: steps, confirmations, rollbacks, minutes. The other cost —
the one that decides whether a 21-step route is worth running on a two-line fix — is tokens, and it
is deliberately **not** a journal field. The agent does not see its own usage; only the host does,
and every host records it differently. A field the agent has to fill from something it cannot read
is a field that gets filled with a guess.

So it is a separate tool over the host's own record:

```bash
trunkline token-cost --slice 007      # one run
trunkline token-cost --session current # this session so far
trunkline token-cost --all             # everything this project cost
```

It reads Claude Code transcripts. On a host that keeps none there is no report, and that is the same
kind of statement as `trace_check: unavailable` — a fact about the setup, not a broken step. Nothing
in the pipeline depends on it: it is run when someone asks what a run cost.

Three things make the number worth trusting, and each of them is a way the obvious implementation
gets it wrong:

- **Records are not calls.** One API response is written to the transcript as several records — one
  per content block — and every one of them repeats the same `usage`. Summing records overstates a
  normal agentic run by 2-9x, and it does so plausibly.
- **A token is not a token.** Uncached input, cache write at a 5-minute TTL, cache write at an hour,
  cache read and output bill at five different rates spanning 50x. On a run of this pipeline cache
  reads outnumber output ~280x while costing a fiftieth per token, so any single "tokens" figure is
  dominated by the cheapest thing in it. The report leads with dollars for that reason.
- **A window is not a run.** Without `session_id` in the journal the tool can only take the
  `start`→`done` wall clock and count everything the project did inside it. With it, the report is
  bound to the run's own sessions and says `exact`; without it, it says `wall-clock` rather than
  pretending. That is the whole reason the field is in the schema.

Two things it does not measure, both worth knowing before quoting the number: work before the
`start` line — step 0's clarification, and the route negotiation itself — falls outside the window,
and the reply that writes `done` is not in the transcript when the report is taken.

Rates live in `core/developer/scripts/prices.json` beside the script — data, so that updating one is
editing a number rather than patching code — and the file carries the date it was taken on. It does
not update itself, and the way it fails is asymmetric: a model the table has never heard of is
reported unpriced and is obvious, while a model whose rate moved is priced confidently at the old
one. That is what the staleness warning after three months is for.
