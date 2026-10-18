# Step 7a. Cross-check agent `[STOP — blocking]`

**Skills:** `agent-skills/skills/doubt-driven-development/SKILL.md`
**Tool:** `trunkline trace-check` — the machine half of checks 1 and 2

The analogue of `/speckit.analyze`. It closes the gap that lets the artefacts of steps 2, 5, 6 and 7
drift apart unnoticed: a rule never reaches the contract, the test is written against the contract
and is green, and the requirement is not implemented.

Executed by the **main agent** (not a subagent): it needs access to every artefact at once and the
authority to block the move to step 8. Spawning is forbidden.

## When it runs

The condition is mechanical, not a judgement: **7a runs when step 5 or step 6 was executed** — on
`feature` that is always, on `migration` whenever step 6 applied. A `migration` that only removes
code produces no artefact to cross-check: 7a is "not applicable", and the reason is recorded rather
than left implicit. On `bug` and `refactor` the step is not in the route at all; run on request, it
obeys the same condition.

**On `audit` the step runs against an earlier run's registry, and it is a report rather than a
gate.** That route has no step 2, so nothing here was written by this run: if the module in scope
carries `.pipeline/workdir/rules/<NNN>.md` from an earlier one — with that run's coverage records
beside it — cross-check the committed code against them: drift between code and its approved
artefacts is what the route exists to find. If the working directory is gone, the step is "not
applicable: this run has nothing to cross-check against". Rebuilding a registry from the code is
forbidden here for the same reason it is everywhere else on this step. Two things change with the
status: the columns are that earlier run's coverage records, not the route's fresh ones, and the
cells are verified against the committed artefacts by reading them, and **no `approval` event is
written** — the approval authorises writing
production code, and none will be written (see "`audit` is the one route that writes no code" in the
core). Findings go to the report — step 20 is not on that route either — and not into a block on a
step 8 that is not there.

One case is not a licence to skip: **BUSINESS RULES exist but the route produced nothing to check
them against.** That is a defect of the route, not of the work. Report it as such and say which step
is missing — do not pass an empty matrix off as a clean cross-check.

## What is cross-checked (the matrix)

The source of truth is `.pipeline/workdir/rules/<NNN>.md`, the registry step 2 wrote — the file, not your
recollection of the console output.

**The matrix covers this changeset's rules, not the task's.** The registry is written once, before
the loop; this step runs inside it. Take the rows whose `Changeset` column step 4 set to the
changeset being cross-checked. Rules belonging to a later changeset are not gaps — they have not been
written yet, and counting them as gaps would block the first changeset of every multi-part task
permanently. On a route with no step 4 there is one changeset and the whole registry applies.

Fill one row per identifier in scope — every `R-<run>-<n>` this changeset closes, whatever its
`Origin`:

```
| ID | Contract (step 5) | Schema/migration (step 6) | Tests (step 7) | Diagram | Status |
|----|-------------------|---------------------------|----------------|---------|--------|
| R-007-1 | createPlan.amount minimum:1 | chk_amount_positive | TestCreatePlan_ZeroAmount | 400 branch | OK |
| R-007-2 | — | — | — | — | GAP |
| R-007-3 | createPlan.currency enum | — | TestCreatePlan_BadCurrency | 400 branch | OK |
```

Statuses: `OK` / `GAP` (the rule appears nowhere) / `MISMATCH` (it appears, but differently) /
`N/A` (the rule does not need that artefact — justify in one line).

**The columns follow the route, not this template.** A column whose step is not in the current route
is filled with `N/A — step 5 not in this route` in every row, never left blank: a blank cell reads as
"the rule failed to reach the artefact", which is a `GAP`, and that is a different statement about
the work. On `migration` the live columns are the schema (step 6) and the test description (step 7);
contract and diagram are `N/A` for the whole matrix — and if the change did reach the contract, the
task was a `feature`, not a `migration`.

**Two live columns is the minimum.** With one there is nothing to cross-check: say so plainly
instead of producing a matrix that only looks like one.

## Fill the matrix with the script, not from memory `[MANDATORY]`

**Run this before writing a single cell:**

```bash
trunkline trace-check \
    --rules .pipeline/workdir/rules/<NNN>.md \
    --changeset <N> \       # this changeset's number; omit where the route has no step 4
    --column  contract=.pipeline/workdir/coverage/<NNN>-c<K>-contract.md \
    --column  tests=.pipeline/workdir/coverage/<NNN>-c<K>-tests.md \
    --partial schema=.pipeline/workdir/coverage/<NNN>-c<K>-schema.md \
    --partial diagram=.pipeline/workdir/coverage/<NNN>-c<K>-diagram.md \
    --na <ID>=<column>       # one per cell of a complete column the rule does not need
```

Pass **only the columns whose step is in this route** — a column that is not passed is "not in
route", which is a different statement from `GAP`. Pass the **coverage records**, never the
committed artefacts: those carry the rule as behaviour and not as an identifier (see "Rule
traceability" in the core), so a grep over them finds nothing and every rule reads as a gap. This
step's own output cannot be passed either — it quotes every identifier by construction, so the check
would pass against itself; keep it in `.pipeline/workdir/consistency/`, away from the coverage
directory, and the trap is closed by the layout rather than by remembering.

The script decides **check 1** (BR completeness) and the mechanical half of **check 2** (no orphans)
by whole-word grep over the coverage records, because each producing step is required to write its
row into one (see "Rule traceability" in the core). Its output — the whole block, table and summary line —
is **copied into the cross-check file** as the fill layer of the matrix. Your job on those two checks is
to explain what it found, not to restate it.

**Why the step is not allowed to fill them by hand.** Every other degradation in this pipeline is
loud: a skipped step shows up in the journal, code before an approval shows up in the diff, a broken
build shows up at the commit gate. A hand-filled 7a matrix is the one failure that is **silent** — it
does not come out as "7a failed", it comes out as a plausible matrix of `OK` statuses that nobody
grepped, and the defect it was built to catch (the rule never reached the YAML, the test was written
against the YAML, the test is green, the requirement is not implemented) passes straight through. The
script is what turns a promise into a fact.

**A green script is not a green 7a.** It says every rule was recorded as reaching its artefact.
Whether it actually did is checks 3-6 below — numbers, boundaries, response codes, types — and those
are read in the committed artefacts themselves, at the place the coverage row names. A grep sees
none of it. What the script buys is that you compare values in cells you know exist, instead of
asserting that the cells exist. **Each cell is opened at least once:** a coverage row naming a field
or a constraint that is not there is a `MISMATCH`, and it is the one defect the relocation of the
identifiers makes possible.

**If `trunkline` is not on PATH**, say so in one line in the cross-check file and in the console, record
`trace_check: unavailable` in the journal event, and fill the matrix by hand with `grep -rnw`
per identifier — pasting the grep commands and their output. A missing tool is a **declared**
degradation of this step, never a silent one, and never a reason to report the check as passed.

## Six mandatory checks

A check whose **both** sides are missing from the route is `N/A` **with the absent step named** — not
omitted, and never counted as passed. A check with one side present runs against what exists: on
`migration`, check 5 compares the column type with the type in the test description, and there is
simply no contract to add as a third source.

1. **Completeness** — *decided by the script.* Every identifier from step 2 is
   present in every **complete** column: the contract record (step 5 blocks on an identifier with no
   row in it) and the tests record (step 7 blocks on a rule with no test). Absence there is a
   `GAP`.
   The schema and the diagram are **partial** columns and are passed with `--partial`: only some
   rules reach them by design — step 6 marks "a database constraint *that expresses* a business
   rule", and a diagram branch exists per *violable* rule. Absence in a partial column is `—`, not a
   gap. Passing them as `--column` instead turns every rule without a database constraint into a
   gap, and the only escape is an `--na` per cell — at which point exemptions are the norm and the
   check has stopped being one.
   On `migration` there is no step 5, so the tests record is the complete column and the schema
   stays partial. That is not a weaker check: completeness asks "did the rule reach the artefacts at
   all", and step 7 is what requires every rule to be there. What the schema is compared *against*
   is checks 3 and 5, which need it live and do not need it complete.
   A cell in a complete column that the rule genuinely does not need is declared with `--na` and
   justified here in one line; an undeclared absence is a `GAP`, not an implied `N/A`.
2. **No orphans** — *the script decides the mechanical half.* It reports identifiers **of this run**
   that appear in a coverage record and not in the registry. An identifier of an earlier run is not
   an orphan and the script says so in a `NOTE`.
   What is left to you is the other half: a live artefact holding a **constraint that no rule
   claims** and that follows from neither the BRs nor the profile guidelines. The coverage records
   name every place a rule landed, so anything constraining behaviour that no row names is either a
   forgotten rule or improvisation — say which.
3. **Numbers and boundaries match literally.** Open the places the coverage rows name. The value in
   the BR ("no more than 100"), the value in the contract (`maximum`), the value in the schema
   (field length, `check`) and the value in the test are the same number. An off-by-one difference (`>` vs `>=`) is a `MISMATCH`, not a Nit.
4. **The error contract lines up.** Every error branch in the sequence diagram has a response code
   in the contract and a test for that code, with the body correctly present or absent (per the
   profile's `api_spec` role).
5. **Types line up across layers.** The field type in the contract ↔ the column type in the schema ↔
   the type in the test description, taking the profile's `data_types` role into account.
6. **Nothing lost between the task text and the registry** — *decided by the script.* Step 2 copied
   the task's "How to test" block into the registry beside the rules it derived, and every rule names
   the item it came from or `development`. The script reports a requirement that became no rule
   (`LOST`) and a rule whose origin points at an item that does not exist (`ORIGIN`).
   This check needs no analysis to have happened: a task always has text, so unlike everything else
   on this list it applies on every route that runs step 2. What is left to you is the half a count
   cannot see — that item 2 became *this* rule and not a different one.

## Order of work (blocking)

1. Run `trunkline trace-check` over the registry and the route's coverage records. Its exit code is not
   the verdict of the step, but a non-zero one means checks 1–2 already failed and there is nothing to
   reconcile until they pass.
2. Fill in the matrix — the command's output is the fill layer — and work the six checks.
3. Write the result **to a file**, `.pipeline/workdir/consistency/<NNN>-c<K>.md` where `<K>` is the
   changeset, with the command's full output pasted in, including the command line it was run with.
   One file per changeset, not one per module: each is the record that authorised its own commit,
   and a shared file would let the last cross-check erase the evidence for every earlier one. To the console — only the path and a
   summary: how many `OK` / `GAP` / `MISMATCH` / `N/A`, and whether the trace check passed.
4. Any `GAP`, `MISMATCH`, `LOST` or `ORIGIN` — **moving to step 8 is forbidden**. In this order:
   **first write the journal event for the run that failed** (item 6, with `rollback_to` naming the
   step you are returning to), then go back to the step that owns the artefact (2 / 5 / 6 / 7), fix
   it, rerun step 7a in full — script included — and overwrite the same file.
   **The order is the whole point.** The rerun writes its own, clean event, and if the failing run
   was never recorded the journal ends up saying this step has never found anything — while the
   `rollback` event next to it says it did. That reads as ceremony and gets the step proposed for
   removal, which is exactly backwards: it is the find that caused the rollback. The file in
   `.pipeline/workdir/consistency/` is overwritten by the rerun and cannot carry this either; the journal is
   append-only and is the only place the failing run survives.
5. An empty matrix (0 rows) when BUSINESS RULES exist is itself a defect: it means the rules reached
   no artefact at all. So is a missing registry: that is a defect of step 2, and the fix is to go
   back and write it, not to retype the registry here from context.
   An **empty registry** is a different thing and not a defect. A feature whose behaviour follows
   entirely from the profile guidelines raises no domain rule of its own, and step 2 is right to have
   written none — the script says so and passes. Say it in one line rather than leaving a blank
   matrix to be read as a failure. The case does not hide a wrong `--rules` path either: the
   registry's own identifiers become gaps in every column, and any identifier of this run found in
   a coverage record but declared nowhere is an orphan.
6. Record an event in the pipeline journal (`.pipeline/workdir/metrics/pipeline.jsonl`) with the number of
   gaps and mismatches, the counts of `LOST` and `ORIGIN` from check 6, and the `trace_check` result
   — see `core/developer/references/pipeline-evals.md`.
   **On every run of this step, not only the one that passed.** A changeset that needed two attempts
   leaves two lines, the first carrying its `rollback_to`. Counting `LOST` and `ORIGIN` separately
   matters for the same reason: check 6 is decided by the script and catches a requirement that
   reached no rule at all — the one defect class the other five checks structurally cannot see — and
   folding it into `gaps` would leave nothing in the journal to show that check 6 is what earns this
   step its place.
7. On the user's "yes", write the `approval` line for step 7a as well, **naming the changeset** — it
   is what the commit gate counts against, and this step runs once per changeset. A clean
   cross-check is a claim about meaning, and it is the user who accepts it; the commit gate will not
   let production code through until that acceptance is on record.

**Output:** `.pipeline/workdir/consistency/<NNN>-c<K>.md` is written with the trace-check output in it, 0 gaps and 0 mismatches,
and the user has approved moving on to writing code.
