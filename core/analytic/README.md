# core/analytic — the AI Analyst prompt, skills and rules

These files are used by the [`PIPELINE.md`](PIPELINE.md) prompt. They describe **how** the analyst
works in three scenarios and **what** it persists between sessions, so it always knows which step it
stopped at, which tasks are unfinished and which open questions remain.

The task format has a single source of truth: the file named by the profile's `tasks.format_spec`
slot. These skills do **not** duplicate it; they link to it.

## Structure

```
core/analytic/
├── PIPELINE.md                                ← the core: rules in force at every step + the step table
├── README.md                                  ← this file (index + scenario router)
├── steps/                                     ← one file per step, loaded one at a time
│   ├── 00-clarifier.md                        ← [CONDITIONAL] the request is vague
│   ├── 01-diagnost.md                         ← context and ASSUMPTIONS
│   ├── 02-analytic.md                         ← AS-IS/TO-BE + the EARS requirements registry
│   ├── 03-impact.md                           ← impact per `R<n>`, ordering constraints
│   ├── 04-decomposer.md                       ← tasks + the coverage matrix + the doubt cycle
│   └── 05-estimator.md                        ← the task files, the estimates, the summary report
├── references/
│   └── ears-notation.md                       ← the five EARS templates (steps 0 and 2 read it)
├── rules/
│   └── analytic-rules.md                      ← the invariants of decomposition and estimation
└── skills/
    ├── analysis-state/SKILL.md                ← the analysis state (persistence)
    ├── decompose-from-scratch/SKILL.md        ← Scenario A: an epic from scratch
    ├── review-and-complete-tasks/SKILL.md  ← Scenario B: review + fill estimates + complete
    └── resolve-open-questions/SKILL.md         ← Scenario C: close the open questions
```

`PIPELINE.md` is read once at the start and held to the end; a step file is read immediately before
its step and not returned to once its `[STOP]` is passed (see "STEP LOADING PROTOCOL" there). The
whole pipeline in context at once costs ~15 KB in English and ~23 KB in Russian before a single line
of project code is read, and buys nothing: the checks that remain are about the artefacts, not about
the text of a step already approved.

## Scenario router (what to run first)

At start the analyst **first** loads the state (`analysis-state`), then picks a scenario. If an epic
state already exists at `<tasks.analysis_dir>/<EPIC>.md`, it reads it and offers to continue from the
recorded step. If not, it determines the scenario from the input:

| Sign at the input | Scenario | Skill |
|---|---|---|
| The epic directory is empty / there are no tasks; all you have is the epic description | **A** | `decompose-from-scratch` |
| A description of **one** task and the epic it belongs to — write that task up, not the whole epic | **A**, scoped to one task | `decompose-from-scratch`, "One task instead of an epic" |
| The directory already holds tasks that need reviewing / estimating / completing | **B** | `review-and-complete-tasks` |
| The tasks exist and you need to come back and close open questions / clear blockers | **C** | `resolve-open-questions` |

The scenarios are not mutually exclusive: B often flows into C (questions remain after the review),
and A ends in a state with open questions that are returned to through C. The state is shared across
all three.

## Reading order for the analyst

1. [`rules/analytic-rules.md`](rules/analytic-rules.md) — the invariants of decomposition and
   estimation (always; the mode and the output format are in `PIPELINE.md`).
2. [`skills/analysis-state/SKILL.md`](skills/analysis-state/SKILL.md) — load or create the state
   (always).
3. The skill for the chosen scenario (A / B / C).
4. The format spec (`tasks.format_spec`) — before saving any task `.md` files.

## Relationship with the analyst prompt

- The agent pipeline (Clarifier→…→Estimator) inside the prompt is **the body of scenario A**
  (`decompose-from-scratch`). Scenarios B and C reuse individual agents of the pipeline (Impact
  Analyst, Decomposer, Estimator), but they do not enter them "from scratch".
- The `STATE` block in the prompt's "OUTPUT FORMAT" is a short projection of the state file
  (`<tasks.analysis_dir>/<EPIC>.md`); the file itself is the persistent carrier of that state.

---

## The link with the publisher (the task-creation tool)

The publisher named by the profile's `tasks.publisher` slot is a tool that reads markdown tasks and
creates them in JIRA. The analyst and the publisher work through a **shared task directory**
(`tasks.dir`) and a shared file format (`tasks.format_spec`): the analyst writes `.md`, the publisher
publishes them. Neither knows the other's internals. The profile declares no publisher → the analyst
stops at the files and the summary report. A profile with no `tasks` section **at all** is the other
case entirely — this track is then not available in that project; see `core/analytic/PIPELINE.md`.

### The lifecycle of a task

```
JIRA ──operations.pull──► <tasks.dir>/<EPIC>/in_jira/  ← what already exists (input for scenario B)
                          │
                    the analyst reads and analyses
                          ▼
                    <tasks.dir>/<EPIC>/*.md    ← tasks per `tasks.format_spec`
                    <tasks.analysis_dir>/<EPIC>.md  ← the analysis state (internal, not a task)
                          │
                    operations.dry_run ──► a check with no writes
                          ▼
                    operations.publish ──► JIRA, the files move to <tasks.dir>/<EPIC>/done/
```

### Who is responsible for what

| Role | What it does | What it does not do |
|---|---|---|
| The analyst (the prompt) | breaks the epic down, writes the task `.md` files and the state | does not create tasks in JIRA itself |
| The publisher (`tasks.publisher`) | validates, publishes, pulls attachments, moves files to `done/` | does not decide what the tasks say |
| The user | confirms the directory, the epic, the front-matter fields and **the act of publishing** | — |

Publishing to JIRA is **an irreversible action outside the repository**. The analyst never starts it
on its own initiative: it prepares the files, shows the summary report and proposes the next step
(`operations.dry_run`, then `operations.publish`). The user starts it.

### Where things live

- The task root is the profile's `tasks.dir`, with a subdirectory per epic key.
- The analysis state is `<tasks.analysis_dir>/<EPIC>.md`: it is **not** a task, it has no task
  front-matter, and the publisher's scanner does not pick it up (see `skills/analysis-state/SKILL.md`,
  "Saving safely").
- The task `.md` format, the front-matter fields, `ready`, `links` and attachments are covered
  only by the profile's `tasks.format_spec` and are not duplicated in the prompt.
- Setting the publisher up (building it, credentials, registering it in the host, the tool list) is in
  the document named by `tasks.publisher.setup`.

### Tracing requirements outward

What crosses to development is **requirement text, not codes**. The `R<n>` numbers are working
notes of this epic: they let step 4 prove that every requirement is anchored to exactly one task, and
they stop there. A number that means something inside the epic and nothing outside it would look like
a contract without being one.

So a task body carries its requirements in the "How to test" block, in EARS, one item per
requirement. The developer's step 2 copies that block, derives one rule per item and assigns the
identifiers that end up in the code — and because both halves land in one file, their step 7a checks
mechanically that no requirement was dropped in the handover.

**The analysis state** (`<tasks.analysis_dir>/<EPIC>.md`, the "Requirements (EARS) and coverage"
table) keeps the epic's whole picture — every requirement and the task it was anchored to. It is the
analyst's own instrument, and it answers what a single task's text cannot: what this task does *not*
cover, and where the rest went.

The summary report shows the same table to the user in a reply. It is a **view**, not the carrier:
a reply is lost with the session, and a bridge that lives only there is not a bridge. On a
disagreement between the two, the state wins.
