---
name: project-knowledge
description: Maintains the accumulated knowledge base about the project's codebase (.pipeline/knowledge/) — the AS-IS of each module and the history of changes by slice. Use when a task starts, so AS-IS is not rebuilt from scratch, and when it ends, to record the delta. Both tracks (development and analysis) read it; only the developer writes, at the final step of the pipeline.
---

# Accumulated knowledge about the code (delta mechanics)

## Overview

This addresses the weakness that "every task rebuilds AS-IS from scratch". Knowledge about the code
accumulates between tasks the same way rules accumulate in the profile guidelines: the second task on
the same module costs an order of magnitude less than the first.

The store lives **in the repository of the project being developed** and is committed there.
Not under the pipeline root (`.trunkline/`), which holds the pipeline and is replaced on every
update.

**Which store exactly is declared by two flags** in the `## Pipeline flags` block of the project's
`AGENTS.md`, exactly as the profile is declared.

- **`knowledge`** — the base's root. Unset: `.pipeline/knowledge/` in the current project. A path:
  the base lives there, which is how a monorepo's shared base or a company base spanning several
  services is attached. `none`: the project keeps no base — nothing to read, nowhere to write, and
  that is said out loud.
- **`project`** — the directory inside the root that is this repository, defaulting to the name of
  the directory `init` ran in. A root shared by several services needs it: without it the second
  service to run writes its `modules/payments.md` over the first one's, and neither run says a word.

Below, `.pipeline/knowledge/<project>/` is the default spelling of "root, then project directory",
not a hard path.

## When to Use

**Reading — at the start of the work, always:**
- the developer, step 1 (Diagnostician agent) — as the very first action, before reading any source;
- the developer, step 0 — so as not to ask what is already recorded;
- the developer, step 3 — the "Consumers and links" section as the starting point for the blast
  radius;
- the developer, step 11 — "Rejected alternatives" answer Chesterton's Fence;
- **the analyst** — when filling in the "Materials" registry and the AS-IS block, instead of
  gathering by hand (see "Analyst access" below).

**Writing — once, at the end:** the developer, step 20 (Archivist agent), after the Git agent.

**When NOT to use:** a one-off question about the code with no changes; a project that has no
intention of setting up `.pipeline/`.

## Who writes

**There is exactly one writer — step 20 of the developer pipeline.** The analyst, the subagents and
every other role read but do not write: two writers into the overwritten files under `modules/` would
clobber each other, and the `slices/` history would lose its order. An analyst who spots a
discrepancy between an entry and the code reports it to the user — the developer makes the correction
at their step 20.

That holds on every route, the `knowledge` route included: it reaches the same step 20 through step
21, which reconciles the entry against the code and has the user approve the differences first. A
second writer is not created by giving the base a route of its own — the route exists precisely so
that reading the base back is somebody's job rather than nobody's.

## Which artefact owns a fact

A development run leaves six places a fact could go, and a module file that answers questions
another artefact already answers is how the base stops being worth reading: the copy is written
once, the original is updated by every later run, and nothing marks the copy stale. So the question
is never "is this true and useful" — it is **whose fact is this**:

| Fact | Where it lives | Written by |
|---|---|---|
| The requirement as the task worded it | `.pipeline/workdir/rules/<NNN>.md` | step 2 |
| Fields, types, error codes, constraints | the contract (YAML) | step 5 |
| Call order, branching, an error branch per rule violation | `sequence_diagrams.md` | step 5 |
| The tests of the module, layer by layer | `tests_readme.md` | step 7 |
| Which artefact each rule reached | `.pipeline/workdir/coverage/` | steps 5, 6, 7 |
| What holds now, and the anchor where it is enforced | `modules/<module>.md` | step 20 |
| Why it was decided that way and what was rejected | `slices/<...>.md` | step 20 |

**A fact from a row above reaches the module file as an identifier or a link, never as text.** The
identifier is what makes the duplication safe: `R-007-3` in the module's rule table and `R-007-3` in
the run's coverage records are one fact in two places, and `trunkline trace-check` verifies they stayed
one. A paragraph retelling a diagram branch is a second copy, and nothing verifies anything about
it. The rows that live with the code carry no identifier — they are reached through the enforcement
point, which is the column next to the rule.

**"Invariants and pitfalls" is what fell through every row.** Not a requirement, not the contract,
not call order, not a test, not a decision — something true about this code that no artefact of the
pipeline has a place for. "A boilerplate HTTP client logs the `Authorization` header on any response
>= 400" is exactly that shape, and it is why the section exists.

Two consequences worth stating outright, because both were learned the hard way:

- **The diagram file has one name, `sequence_diagrams.md`.** Step 5 fixes it, and steps 7a and 17
  pass it to `trunkline trace-check` as the diagram column: a module that calls its diagrams something else
  is outside the traceability check, and its rules are outside this table's reach too. Finding such a
  file is a defect to report — the fix is to rename it, not to record the exception here.
- **A module whose rules predate the registry has an incomplete rule table, and that is contagious.**
  Step 2 looks there for rules already in force; rules that live only in a diagram under a local
  numbering (`BR4`) are invisible to it, so the next run registers one of them again under a fresh
  identifier and the same rule ends up with two numbers. Raising such rules into the registry is what
  the `knowledge` route is for, and doing it **before** a feature run touches the module is cheaper
  than untangling the pair afterwards.


## Structure

```
.pipeline/
├── knowledge/                        ← KEPT and committed. The `knowledge:` flag names this
│   │                                   root; it may hold several projects.
│   ├── INDEX.md                      ← table of contents over EVERY project in the base:
│   │                                   project → module → file → last slice → date
│   ├── <project>/                    ← the `project:` flag — this repository
│   │   ├── adr/
│   │   │   └── ADR-NNN-<slug>.md     ← irreversible decisions and their cost (step 15, append only)
│   │   ├── modules/
│   │   │   └── <module>.md           ← the module's current AS-IS (overwritten)
│   │   └── slices/
│   │       └── YYMMDD-<slug>.md      ← change history (append only)
│   └── <other-service>/              ← another repository sharing this root
└── workdir/                          ← DISCARDED. One .gitignore line: /.pipeline/workdir/
    ├── rules/<NNN>.md                ← the rule registry of one run (step 2)
    ├── consistency/<NNN>-c<K>.md     ← step 7a's cross-check
    ├── review/<NNN>-c<K>.md          ← the findings of steps 9+10
    └── metrics/pipeline.jsonl        ← the run journal, see core/developer/references/pipeline-evals.md
```

**`workdir/` is never shared.** It is the state of one run in one repository, so it stays under this
project's `.pipeline/` even when `knowledge:` points at a base on the other side of the filesystem.
Only the kept half travels.

**`INDEX.md` is the one file at the root, and it is written by whichever project ran last.** That is
deliberate: "does any of our services already describe this module" is the question a shared base
exists to answer, and it cannot be answered from inside one project's directory. Its rows therefore
carry a `Project` column, and a project appends and updates only its own rows — a run that rewrites
another project's row is rewriting a fact it did not check.

**The split is by lifetime, not by subject.** Everything on the left of it answers "what does this
code do and why is it like that" to a reader arriving a year later; everything on the right is read
during one run and superseded when it ends — what survives goes into `knowledge/`. That is why
`workdir/` is swept by age
(`core/developer/scripts/workdir-clean.sh`, files older than a week) and `knowledge/` never is.

Keeping ADRs here rather than in the project's `docs/` follows from the same split: an ADR is kept
forever and is read together with the slice entry that links to it. Two trees meant one of them got
gitignored by accident — which is the failure the single `knowledge:` path exists to prevent.

A slice is one run of the development pipeline, and the run is identified by its **run number** —
the UTC start time, `YYMMDDHHMMSS`, minted at classification (see "The run number" in
`core/developer/PIPELINE.md`). It is the clock rather than a counter so that several developers
working at once cannot mint the same one, and so that it exists even where `metrics: none` and
`knowledge: none` leave no file to count from.

**The file name carries only the date half**, `YYMMDD-<slug>.md`: the directory is browsed by
people, and the full number lives inside the entry in its `Run:` field, which is what joins it to
`.pipeline/workdir/rules/<NNN>.md` and to the journal's `slice`. The code carries no `R-<run>-<n>`:
the identifier lives in the run's working directory and in this base, and the entry's enforcement
point is what leads from the rule to the code.
Same slug on the same day — append the time, `YYMMDD-HHMMSS-<slug>.md`; entries are never
overwritten.

## Templates

Full templates for the three files this base holds — including the field-by-field rules for filling
each one (the `R-<run>-<n>` numbering, the `file:symbol` anchor format, what collapses into one row,
what counts as a local path) — are one hop away. Read the one you are about to touch, in full, right
before writing it; they are not summarised here because a summary is exactly where those rules go
stale first:

- [references/module-template.md](./references/module-template.md) — `modules/<module>.md`, target
  size ≤ 150 lines, always the current state.
- [references/slice-template.md](./references/slice-template.md) — `slices/<slug>.md`, an immutable
  entry, never edited after the fact.
- [references/index-template.md](./references/index-template.md) — `INDEX.md`, the base-wide table
  of contents.

**The headings of the module template are protocol, not prose: they keep their English spelling in
every language.** `## Business rules in force`, `## Invariants and pitfalls`, `Updated:`, `Slice:` —
these are parsed by `trunkline kb-sync`, which finds the rule table by its heading and
the drift window by the `Updated:` date. A localized heading is not a smaller failure than a missing
file: the script finds no table, reports `0 rule(s)`, and prints `ANCHORS CLEAN` over a base it never
read — a false green, which is worse than an error, because step 21 then goes looking for meaning in
an entry whose mechanics nobody checked. Same rule as the protocol markers `[STOP]` and `ADDED` /
`MODIFIED` / `REMOVED`, and for the same reason — a second spelling means a second parser. Translate
the prose under a heading, never the heading itself.

## Analyst access (read-only)

The analyst (the analytic track, `core/analytic/`) gathers AS-IS and a materials registry for the same codebase. If
`.pipeline/knowledge/` exists in the project, it is a ready-made source and nothing needs regathering:

| What the analyst needs | Where it comes from |
|---|---|
| the module's AS-IS for the "Context" block | `modules/<module>.md` — "Purpose", "Layers and key types" |
| materials of type `as-is-code`, `db-schema` | "Entry points" and "Data" in that same file |
| who is affected by a change | "Consumers and links" |
| the rules in force and their identifiers | "Business rules in force" |
| why it was done that way last time | `slices/*.md`, "Rejected alternatives" |

Rules for the analyst:
- **Link, do not copy.** In the "Materials" registry of the analysis state, give the path to
  `modules/<module>.md` rather than transferring its contents.
- **Do not write.** Not to `modules/`, not to `slices/`, not to `INDEX.md`.
- **Check freshness.** An entry is a hint: confirm in the code any fact an estimate or a requirement
  is built on. Report a discrepancy to the user.
- If there is no base — work as before; that is not a blocker.

The analysis state (`<tasks.analysis_dir>/<EPIC>.md`) **neither replaces nor duplicates** this base: the state
holds the state of the work (which step we are on, what is unfinished), the base holds the state of
the system. Different lifetimes too: the analysis state dies with the epic, the base lives with the
project.

## Hygiene

- An entry is **a hint, not the truth**: step 1 must verify, pointwise, the facts it actually relies
  on and mark discrepancies `STALE`.
- A discrepancy is fixed at step 20 of the same run — not deferred.
- If a module file passes ~150 lines, or no longer matches the code in a substantial way — rewrite it
  in full rather than patching it.
- Roughly every 10 slices it is worth proposing a review to the user: delete entries about deleted
  code, merge duplicated modules. The instrument for that review is the `knowledge` route — one
  module per run, `trunkline kb-sync` for the anchors and the identifiers, step 21.4 for the meaning — not
  a free-hand tidy-up between tasks.
- **A module nobody has touched in months is the one most likely to be wrong**, and the least likely
  to be caught: staleness is found by step 1 only where a task happens to lean on the stale fact.
  `trunkline kb-sync` reads the commit log since the entry's `Updated:` date, which is the cheapest
  available prior for which entry to reconcile next.

## Common Rationalizations

| Excuse | Why it does not hold |
|---|---|
| "The task is small, I will skip the slice entry" | Cheap slices are the main source of the delta. Skipping makes the base selective, and a selective base stops being trusted — so the next run gathers AS-IS from scratch again. |
| "The base is stale, it is easier to rebuild AS-IS" | Going stale is the normal mode; the verification under "Hygiene" is designed for it. Rebuilding and not recording the result guarantees the next time will be from scratch too. |
| "I will copy the AS-IS from the base into the analysis state, it is handier" | A copy goes stale silently and drifts from the original. A link to `modules/<module>.md` goes stale with the base and is fixed in one place. |
| "I will write the modules and the slice at the end — a week's worth at once" | The delta is reconstructed inaccurately from memory: the rejected alternatives and the reasons behind decisions disappear — exactly what the base exists for. |
| "The analyst can fix the module file too, they can see the discrepancy" | Two writing tracks means two diverging pictures. The analyst names the discrepancy to the user; only the developer writes, at step 20. |

## Red Flags

- A run finished and `slices/` gained nothing — the delta is lost.
- A module file describes code that no longer exists: not marked `STALE`, not rewritten.
- A requirement in a task is justified by an entry in the base but never checked against the code.
- The analysis state contains paragraphs copied from `modules/` instead of a path to the file.
- A module file has grown past ~150 lines and is being patched with spot edits.
- `INDEX.md` does not agree with the contents of `modules/` and `slices/`.
- The analyst left an edit in `.pipeline/knowledge/`.

## Verification

Before closing a run:

- [ ] The slice `slices/<YYMMDD>-<slug>.md` is created, its `Run:` field carries the run number, and the `ADDED` / `MODIFIED` / `REMOVED` blocks are filled in (an empty block is an explicit `—`).
- [ ] The files of the affected modules are updated; business rules carry their identifiers, and a rule reused from an earlier run kept its own.
- [ ] Discrepancies found at step 1 are resolved in this same run, not marked `STALE` "for later".
- [ ] The decisions taken and the **rejected alternatives** are recorded — not just the outcome.
- [ ] `INDEX.md` has gained the slice row and agrees with the contents of `modules/` and `slices/`.
- [ ] No module file has passed ~150 lines without being rewritten.
- [ ] Only the developer wrote: there are no traces of analyst edits.
- [ ] Every `Where it is enforced` cell is a `file:symbol` anchor, or prose with a reason — no line
      numbers.

On a `knowledge` run, the last check is the whole point rather than hygiene: `trunkline kb-sync` decides it
mechanically, and an anchor it cannot resolve is a finding for step 21's table.
