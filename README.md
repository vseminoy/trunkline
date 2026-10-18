# trunkline

[![License: Polyform Noncommercial 1.0.0](https://img.shields.io/badge/license-Polyform%20Noncommercial%201.0.0-blue.svg)](LICENSE)

**A vertical, brownfield spec-driven development pipeline for coding agents — and the CLI that
installs it into your project.**

Most SDD frameworks give you a process and leave your domain rules in the developer's head. They do
not know that timestamps in your schema are `timestamp(0)` in a specific timezone, that error schemas
live behind a `$ref` in a shared file, that only `2xx` and `400` carry a body, or that `Up` and
`Down` belong in one migration file.

trunkline inverts that. The domain rules are part of the process. They live in a **stack profile**,
and the stack-agnostic **core** references them by role instead of restating them. Extend a
guideline and the pipeline does not change. Change stacks and the profile changes, not the core.

It is built for **existing** codebases: 21 steps, 6 task routes, a blocking artefact cross-check, and
a knowledge base that makes the second task on a module cheaper than the first.

In Russian: [`README_ru.md`](README_ru.md).

---

## Quick start

```bash
# Not published yet — there is no public remote to install from.
# Build from this repository's own checkout instead (installs into its own
# folder and offers to add it to PATH; `make uninstall` reverses it):
make install

cd your-project
trunkline init --profile oss-py-fastapi   # or: example-stack, as a template for your own
trunkline doctor
```

Then, in your agent:

```
/trunkline-dev the autopay schedule is not saved, it fails on an empty body
```

A change too small for the pipeline — a typo, a guard, a log line — has its own route, with the same
profile rules and the same commit gate and none of the steps:

```
/trunkline-quick the error message in the payment handler says "оплата" twice
```

`init` vendors the pipeline into `.trunkline/`, adds a `## Pipeline flags` block to your `AGENTS.md`,
creates `.pipeline/` — `knowledge/` kept, `workdir/` discarded — and installs the commit gate.
`doctor` tells you what would break before you find out mid-run. Both are safe to rerun — an existing
flags block is never rewritten.

No slash commands in your agent? Say "follow `.trunkline/AGENTS.md`" instead. That file exists so the
pipeline has an entry point requiring no host feature at all.

Not an English-speaking team? `--language ru` installs the Russian localization. It is deliberately
small, because the core already tells the agent to write its artefacts in the project's language, so
a Russian project gets Russian specs and a Russian knowledge base out of the English prompts. What gets translated is what the English core cannot supply: EARS notation in the
target language, and the templates that are copied verbatim into artefacts. Localizations are
overlays, so everything else falls back to English instead of breaking a route, and a gate catches
translations that have drifted from their source — see
[`core/l10n/README.md`](core/l10n/README.md).

Full setup, every flag and its default: [`SETUP.md`](SETUP.md).

---

## What you actually get

**1. The agent does not start with code.** The spec, the schema and the test description come first,
each with an explicit "yes" from a human. On the `feature` route that is four approvals per
changeset; shorter routes have fewer, and the run says at the start how many it has. There is no
way to skip them, and none to skip the other stops either — the pipeline has a single way of
running, and "There is no unattended mode" in the core says why.

**2. Nothing is lost between layers.** A requirement crosses from the analyst as text, becomes a rule
with an identifier once, and every step that turns that rule into an artefact records where it
landed:

```
analyst, step 2   R7 (EARS): IF the payment amount ≤ 0, THEN the service SHALL return 400
analyst, step 4   coverage matrix: R7 → task C2      ← working notes of the epic, they stop here
analyst, step 5   the task's "How to test": "IF the payment amount ≤ 0, THEN … 400"
─────────────────────── handover: the requirement, as text ───────────────────────
developer, 1      reads it out of the task, verbatim
developer, 2      registry: | R-007-2 | amount above zero | task item 3 |   ← the identifier is born
developer, 5      YAML: amount { minimum: 1 }        → coverage: | R-007-2 | amount (minimum: 1) |
developer, 6      schema: check (amount > 0)         → coverage: | R-007-2 | chk_amount_positive |
developer, 7      test TestCreate_ZeroAmount         → coverage: | R-007-2 | TestCreate_ZeroAmount |
developer, 20     knowledge base: | R-007-2 | amount above zero | service.go:42 |
```

One family, one assigner: `R-<run>-<n>` is handed out at the developer's step 2 and nowhere else, so
two tasks on one module can never claim the same number. The analyst numbers requirements too, but
those numbers serve their own coverage matrix and never leave the epic — what crosses is the
statement. Because step 2 records the task's items beside the rules it derived, step 7a checks by
machine that none was dropped in the handover.

`grep -rnw "R-007-2"` over the run's working directory answers what covers a requirement across the
spec, the schema and the tests, and each row names the exact place to open. (Whole words on purpose:
a bare `R-007-1` would also match `R-007-10`.) **The identifier stops at the repository boundary:**
the committed artefacts state the rule as behaviour — a `minimum`, a named constraint, a named test
— because the registry is a working file of the run and is deleted with it, and a number pointing at
a deleted file is worse than no number. What outlives the run is the knowledge base entry, which is
where a rule is looked up afterwards. Whether the code really enforces the rule is the one link no
grep settles: it is a judgement, and step 9 makes it by reading the code.

**3. Step 7a blocks on drift.** Between the spec, the schema, the test description and the code sit
four independent artefacts. The real failure is quiet: a rule never reaches the YAML, the test is
written against the YAML, it is **green**, and the requirement is not implemented. Step 7a builds an
`ID × spec × schema × tests × diagram` matrix and runs six checks — completeness, orphans, literal
agreement of numbers and boundaries (`>` vs `>=` is a mismatch, not a nitpick), the error contract,
types across layers, and — by machine — that no requirement of the task text became no rule. Any gap
and writing code is forbidden.

**4. Knowledge accumulates.** `.pipeline/knowledge/` — the root the `knowledge:` flag names — holds
everything the project keeps for good: the ADRs, the current AS-IS per module, and the slice history
of what was added, modified and removed, the decisions taken and, more usefully, the alternatives
rejected. The root may span a company: several services point at it and differ only in the
`project:` flag, which names their directory inside it, while one `INDEX.md` at the root spans all
of them. Step 1 reads it before touching any source; steps 15 and 20 write it. It is
committed with your code. Everything else the pipeline writes lives under `.pipeline/workdir/`, is
superseded when the run ends, and is swept by age — one `.gitignore` line, no decisions.

**5. The development process is measurable** (the analytic track keeps no journal — its persistence
is the analysis state). `.pipeline/workdir/metrics/pipeline.jsonl` records findings by severity,
rollbacks, skips and the cost of a changeset in confirmations. After a month you can see which steps
catch defects and which are ceremony:

```
Steps that found defects: 7a (5 gaps), 9 (3 Critical), 10 (1 High)
Steps with no findings at all: 11, 15 → candidates for removal from the route
Rollback points: 7a → 5 (×3) — rules regularly fail to reach the spec
```

Cut the pipeline on data, not on feel. The report is a proposal; the decision is yours.

**6. Machine gates, not promises.** Sixteen `[STOP]` gates rest on the model's obedience — that is
an honest weakness. Whatever a machine can check is checked by a machine: the commit gate blocks
`git commit` when the profile's build or lint command fails, as a Claude Code hook or as a plain git
hook (`--gate=git`), your choice.

---

## Four units of work

A request becomes a commit through four decompositions. Each has its own owner, its own size limit
and its own name, and the names are not interchangeable — three of the four are read by something
outside this pipeline.

| Unit | Produced by | Size | Ends up as |
|---|---|---|---|
| **epic** | the tracker — this is the input | — | what the analysis starts from |
| **task** | the analyst, step 4 | ≤ 24 working hours | a Jira issue (`Task` / `Story` / `Bug`) on the epic's Epic Link, created by the publisher |
| **changeset** | the developer, step 4 | one vertical capability, API to database; past ~5 files, the plan says why | one commit |
| **increment** | the developer, step 8 | ~100 lines | one `git add` — never a commit |

Read it downward: the analytic track splits an epic into tasks; **one** task arrives at the
development track — that is the "task" its step 0 classifies — and is split into changesets; a
changeset is written increment by increment and lands as a single commit at step 18. Both tracks cut
**vertically** — a task and a changeset are each working functionality through every layer, never
"all the DB first, then the handlers". What differs is the altitude and the reader: a task is read by
a human in a tracker, a changeset by `git log`.

`task` is the one name here the project does not own: it is Jira's issue type, it is what the
publisher writes into `issuetype`, and the other three are named around it. Two things are called a
"type" on that level and they are not the same — front-matter `type: Bug` is the **Jira issue type**,
while the journal's `task_type: bug` is the **route** the developer core picked at classification.
A Jira `Task` routinely runs the `bug` route.

`subtask` is **not** a name in this pipeline. In Jira it is the level *below* a task, attached by
`parent` rather than by Epic Link, and this pipeline never creates one.

**`slice` is not on this list.** It means one thing only: an entry in the knowledge base
(`.pipeline/knowledge/<project>/slices/NNN-*.md`) and the `slice` field of the run journal — the record of one
completed run. A unit of history, not a unit of decomposition.

---

## Routes

Step 0 classifies the task and runs only the steps it needs — a bug does not get the full ceremony.

| Type | Steps |
|---|---|
| **bug** | 0? → 1 → 7(prove-it) → 8 → 9+10 → 18 → 20 |
| **feature** | 0? → 1 → 2 → 3 → 4 → (5 → 6? → 7 → **7a** → 8 → 9+10 → 11 → 12 → 15 → 16? → 18)×N → 17 → 19? → 20 |
| **refactor** | 0? → 1 → 3 → 4 → (8 → 9+10 → 11 → 12 → 18)×N → 20 |
| **migration** | 0? → 1 → 2 → 3 → 6? → 7? → **7a?** → 8 → 9+10 → 18 → 20 |
| **config/ci** | 0? → 1 → 19 → 18 → 20 |
| **audit** | 1 → 7a? → 9+10 — reviews existing code, writes none (no step 20) |
| **knowledge** | 0? → 1 → 2 → **21** → 20 — reconciles `.pipeline/knowledge/` against the code, writes no code |

`(…)×N` is the changeset loop: step 4 splits the work into changesets, each becomes one commit, and
the steps that produce a commit repeat once per changeset. It exists exactly where step 4 is in the
route. Step 18 is the seam — it names what is left and stops; step 17 assesses the finished delivery
once, after the last commit.

The numbers label roles, not positions: the route decides the order, and on some routes it does not
ascend — `config/ci` runs CI before the commit, `feature` runs the assessment after it. No numbering
could satisfy both.

These are **roles of one main agent, not 19 subagents.** Spawning is allowed in exactly three places
(the parallel Critic and Security auditor, `Explore` for large reads, Agent Teams for 3+ competing
hypotheses). Everywhere else it is forbidden: an extra hop retells the context and breaks the `[STOP]`
checkpoints that a human's participation rests on.

---

## How it compares

| | trunkline | Kiro | Spec Kit | BMAD | OpenSpec |
|---|---|---|---|---|---|
| Brownfield | **yes, the target case** | no | weak | no | yes |
| Domain specificity | **maximal (the profile)** | no | no | no | no |
| Changing stacks | **swap the profile, the core is untouched** | n/a | n/a | n/a | n/a |
| Task classification | **yes, 6 types** | no | no | Quick mode | workflow profiles |
| Cycle coverage | **spec → prod → CI** | spec → code | spec → code | spec → code | spec → code |
| Artefact cross-check | **yes (step 7a)** | partial | yes (`analyze`) | yes | structure only |
| Context accumulation | **yes (`.pipeline/knowledge/`)** | no | no | no | yes |
| Traceability | **yes, end to end (requirement → code)** | strong (EARS) | medium | strong | medium |
| Process measurability | **yes (a run journal)** | no | no | no | no |
| Enforcement | text + machine gates | **UI-structural** | text | text | text |
| Portability across agents | **medium** | medium | **high** | high | high |

OpenSpec's "structure only" is not a gap in validation but a different target: `/opsx:verify` and
spec validation check that the change artefacts are well-formed, not that a rule agreed across the
spec, the schema, the tests and the code. Its "workflow profiles" row is the `default`/`extended`
profile switch plus the fast route for small changes.

Honest about the weaknesses. Enforcement is still largely social: one "carry on" in the wrong place
and a contract is broken. Portability is real but uneven — subagents and hooks are Claude Code
features, and without them the review is weaker and the gate must be installed as a git hook.
[`core/ORCHESTRATION.md`](core/ORCHESTRATION.md) states exactly what a host must be able to do, what
is optional, and what degrades without each capability. Nothing degrades silently: the agent
announces it at step 0.

---

## The CLI

| Command | What it does |
|---|---|
| `trunkline init` | Vendors the pipeline, writes the flags block, creates `.pipeline/`, installs the gate |
| `trunkline update` | Upgrades the core and the profiles; runs format migrations; keeps and lists files you edited |
| `trunkline doctor` | Checks the whole setup — flags, profile, required files, guideline roles, gate, knowledge base, version drift, local edits |
| `trunkline profile list/show/add` | Inspects and installs stack profiles |

Everything written is recorded with a content hash in `.trunkline/state.yaml`, so an update can tell
its own output from your edits and never overwrites yours silently. A file that is *not* in that
record is left alone too, and reported as `kept (not ours)`: no record means the file was not put
there by trunkline, and `--force` is how you say otherwise. Commit `state.yaml` — `update` diffs
against it.

---

## Profile guidelines

The domain rules live in the profile, and the core reaches them by **role**, never by file name. A
role is the slot in `profile.yaml`; the file is what the slot points at. Rename the file, or put a
different document in the same slot, and nothing in the core changes.

```yaml
guidelines:
  roles:
    api_spec: 02-openapi-spec.md      # role → file
    data_types: 03-data-types.md
```

Every step that needs them opens with a **Profile rules** block naming the roles mandatory on it —
not the whole set, so a step is given what it has to apply and not a library to skim. The rules are
binding from step 3 onwards:

| Step | Roles it reads | What it does with them |
|---|---|---|
| **3** Impact | **all** — this is where they start applying | checks the proposed change against them, names violations in the impact report |
| **5** Specifier | `api_spec` (in full), `data_types`, `auth` | the spec is not handed over if it breaks them |
| **6** DB migrations | `migrations`, `data_types`, `db_schema_style` | schema → queries → migration, all three by the rules |
| **7** Tester | `api_spec` | the error contract: which codes carry a body |
| **7a** Cross-check | **all** — as the ubiquitous constraints | a live constraint that follows from neither the task nor a guideline is an orphan |
| **8** Generator | `data_types`, `auth`, `api_spec` | writes the code against them |
| **9+10** Critic & Security | **all** | any violation is Critical/Important → rollback to step 8 |
| **17** Assessor | **all** | pre-launch checklist item: conformance, no violations |

Four of those rows say **all**, and that is what makes the set extensible: the core iterates
`guidelines.roles`, so a role a profile adds of its own is picked up at steps 3, 7a, 9+10 and 17
without a single edit in `core/`. A profile with no guidelines at all is valid — the conformance
block then reports "not applicable".

A rule expensive to miss should not rest on this alone. Everything above is an instruction to a
model, and instructions degrade over a long session; whatever a machine can decide belongs in the
commit gate too — see "Machine gates, not promises".

---

## Layout

```
core/         the pipeline: routes, gates, traceability, knowledge, telemetry — stack-agnostic
├── developer/    the development pipeline (21 steps) — core + steps/ + scripts/ (the machine gates)
├── analytic/     the analysis pipeline (epic → EARS requirements → Jira tasks) — core + steps/
├── skills/       shared: the code knowledge base
├── l10n/         localizations, applied as overlays over the English source
└── ORCHESTRATION.md   what a host must do, and what degrades without it
profiles/     stack profiles: language, commands, guidelines, step overlays
publishers/   contracts of the task publishers a profile may declare — owned by the tool
internal/     the CLI
evals/        routing test cases, one file per skill
scripts/      the gates run before a commit
agent-skills/ submodule → addyosmani/agent-skills (read-only)
```

The core knows: routes, roles, `[STOP]` gates, `R-<run>-<n>` traceability, the 7a cross-check, the knowledge
base, the run journal, the output format. The core does **not** know: languages, code generators,
`build`/`lint`/`test` commands, guideline names, system layers, gate scripts. All of that comes from
the profile manifest through slots. Artefact templates straddle the line: the shape is core, the
names and values in it are the profile's. A linter enforces the boundary:

`core/developer/scripts/` still holds `approval-gate.sh`, `workdir-clean.sh` and `prices.json`, and
is different in kind from `scripts/` at the repo root: it ships **into** a project and runs there.
`approval-gate.sh` refuses a commit carrying production code while an approval that changeset owes
is unrecorded — see `core/developer/ENFORCEMENT.md`. The four machine gates that used to be scripts
next to it are now subcommands of the same `trunkline` binary a project already has on PATH:
`trunkline trace-check` decides checks 1–2 of step 7a, and at step 17 the one link no earlier step
can check — that every rule reached the code; `trunkline review-check` runs steps 9+10 as two API
calls where the host has no subagents, so a review from a fresh context is not a Claude-only
privilege; `trunkline token-cost` answers what a run cost in tokens and dollars, which the journal
cannot — the agent does not see its own usage, only the host does; `trunkline kb-sync` is the
machine half of the step 21 reconciliation. None of the four reads a profile: paths and roles come
in as arguments or environment, and `prices.json` — the only one of the four with state — ships as
data the binary embeds, not code, so updating a rate is not a patch.

```bash
python3 scripts/lint-core-profile-boundary.py   # core ↔ profile boundary
python3 scripts/lint-pipeline-paths.py          # core paths resolve from the pipeline root
python3 scripts/lint-l10n.py                    # localizations vs their English sources
node scripts/lint-own-skills.js                 # structure of our own skills
node scripts/run-own-evals.js                   # skill routing by description
bash scripts/test-approval-gate.sh              # the approval gate against a journal
bash scripts/test-trace-check.sh                # the 7a trace check against a registry
bash scripts/test-review-check.sh               # the 9+10 verifier against a mock endpoint
bash scripts/test-token-cost.sh                 # the cost report against synthetic transcripts
bash scripts/test-kb-sync.sh                    # the step 21 reconciliation against a knowledge base
```

---

## Documentation

| Document | What is in it |
|---|---|
| [`SETUP.md`](SETUP.md) | Installing into a project: flags, knowledge base, gates, troubleshooting (RU) |
| [`AGENTS.md`](AGENTS.md) | Entry point for agents working on trunkline itself |
| [`core/README.md`](core/README.md) | The core ↔ profile boundary |
| [`profiles/README.md`](profiles/README.md) | Writing a profile for your own stack |
| [`core/ORCHESTRATION.md`](core/ORCHESTRATION.md) | Host capabilities and their degradations |
| [`docs/DESIGN-NOTES.md`](docs/DESIGN-NOTES.md) | Why the core's rules are what they are — not vendored into a project |
| [`README_ru.md`](README_ru.md) | This file in Russian |
| [`trunkline-overview.ru.html`](trunkline-overview.ru.html) | Slide-deck overview of the pipeline (Russian) |

---

## License

[Polyform Noncommercial License 1.0.0](LICENSE) — free for personal, research, educational and
other noncommercial use. Commercial use requires a separate license from the author.

`agent-skills/` is a vendored submodule (upstream [addyosmani/agent-skills](https://github.com/addyosmani/agent-skills)),
MIT-licensed by its own author; that license travels with it and is unaffected by the license above.

---

## Status

Pre-release. The core is stable and in daily use; the CLI is version 0.1.0 and the payload format is
still moving, which is what the migrations are for.

Contributions: read [`AGENTS.md`](AGENTS.md) first — the core ↔ profile boundary is the rule that
changes get wrong most often.
