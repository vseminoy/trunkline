# Step 20. Archivist agent `[STOP — the final step]`

The writing half of the delta mechanics. Step 1 reads `.pipeline/knowledge/`; this step adds to it.
Without it every next task rebuilds AS-IS from scratch — the exact bottleneck the mechanics exist to
remove.

**Skill:** `core/skills/project-knowledge/SKILL.md` — the storage format and the writing rules. What
follows is the order of operations.

Runs after the Git agent, ALWAYS on every route that writes code (including `bug` and `config`). It
may be skipped only if the user explicitly declines — and that is recorded in the journal as
`skipped`.

**On `knowledge` it writes a reconciliation, not a delta.** That route changed no code, so the
`ADDED` / `MODIFIED` / `REMOVED` blocks of the slice entry are `—` and the entry records what step 21
found instead: what the entry claimed, what the code said, and which rows were corrected, added or
retired. Three things differ from an ordinary run and nothing else does.

- **Only what step 21 got approved is written.** The difference table is the work order; a finding
  the user struck out is not written, and a finding step 21 classified as "doc vs code" is not
  written either — it is a `feature` or a `bug` for the user to open, and putting it in the rule
  table would record a requirement nobody implemented as a rule in force.
- **A corrected rule keeps its identifier where the rule is the same and the entry merely
  misdescribed it.** Correcting the wording of `R-004-2` is not superseding it: there is no new rule,
  so there is no `superseded:` line and no new number. A rule the code turns out to enforce
  *differently* — a different boundary, a different code — **is** a new rule, gets this run's
  identifier, and the supersession is recorded as usual.
- **The commit message says so:** `docs(knowledge): reconcile <module> — <n> corrected, <m> added`.
  It carries `.pipeline/knowledge/` and, where step 2 wrote one, `.pipeline/workdir/rules/<NNN>.md`. There
  is nothing else in the run to commit, and step 18 did not run.

Where the module had no entry at all, this is its first description rather than a reconciliation, and
the entry is written from scratch — say which of the two happened, because "the base agreed with the
code" and "the base said nothing about this module" are different facts about the project.

**It does not run on `audit`.** This step records the delta of a change, and that route changes
nothing: no slice entry, no module update, no `docs(knowledge): ...` commit. The run is closed by
step 9+10 instead, which writes the `done` line itself with `knowledge: none`. Findings nobody took
on stay in the review report under `.pipeline/workdir/review/`; putting them into a module file's "Known
tech debt" is the job of whatever `refactor` the user asks for next, not of the review.

## Order

0. **Flag.** `knowledge: none` — no slice entry; announce that to the user and go straight to the
   journal.
1. **Bootstrap.** If the knowledge directory does not exist — create the structure from
   `core/skills/project-knowledge/SKILL.md` (`adr/`, `modules/`, `slices/`, `INDEX.md`) and tell the
   user that the store has been set up. Check `.gitignore` at the same time: a pattern that swallows
   `.pipeline/` as a whole hides the base from everyone but the person who wrote it, and the correct
   line is the single `/.pipeline/workdir/`. Say so rather than editing `.gitignore` silently.
2. **Slice entry.** Create `.pipeline/knowledge/<project>/slices/<YYMMDD>-<slug>.md` from the template — the
   **date** half of the run number, not the whole of it: the entry is read by a human browsing the
   directory, and a full timestamp in every file name buys nothing there that the run number inside
   the entry does not already say. The number itself goes in the entry's `Run:` field, which is what
   joins it to the registry, the journal and the identifiers in the code. If the name is already
   taken — the same slug on the same day — append the run's time (`<YYMMDD>-<HHMMSS>-<slug>.md`)
   rather than overwriting: a slice entry is immutable history.

   From the template: date, task
   type, commits, the rules with their statements and their `Origin` as step 2 recorded it (an item
   of the task text — name the epic; a rule already in force in the module; new in this run), the
   `ADDED` / `MODIFIED` / `REMOVED` blocks, the decisions taken and the alternatives rejected, and a
   link to the ADR if there was one.

   Recording a rule's origin is mandatory: a year from now it is the only way back from the code to
   the original requirement and its context.
3. **Module updates.** For every module touched, update `.pipeline/knowledge/<project>/modules/<module>.md`:
   bring AS-IS to the state AFTER the changes, add the new links and consumers, and fix everything
   step 1 marked `STALE`.

   **The "Business rules in force" table merges rather than grows.** Each rule of this run is one of
   three things, and they end differently: *new* — add a row; *already there under another
   identifier* — do not add it, and say in the report that the existing one was reused, because a
   second row for the same rule is how the table stops being able to answer "what is this rule";
   *supersedes one in force* — replace the row, and record the supersession in the changeset entry
   (step 2 above) as its own line: `superseded: R-004-2 by R-007-3`. The entry is immutable history
   and the module file is current state, so that is the only place the pairing survives — and it is
   what a reader who meets the old identifier in an earlier entry needs to find.
4. **Index.** Append a row to `.pipeline/knowledge/INDEX.md` — the file at the base's **root**, not
   inside the project directory. Fill the `Project` column with the `project` flag and make the
   `File` path relative to the root (`<project>/modules/<module>.md`). Touch only rows whose
   `Project` is this run's: a base shared by several services carries other projects' rows, and a
   run that rewrites one is rewriting a fact it never checked. Where the index does not exist yet,
   create it from the template — a shared base gets its first row from whichever project runs first.
5. **Journal.** Close the run entry in `.pipeline/workdir/metrics/pipeline.jsonl` — see
   `core/developer/references/pipeline-evals.md`. Carry the host's session identifier into the `done`
   line as well as the `start` one (`$CLAUDE_CODE_SESSION_ID` under Claude Code): a run resumed in a
   second session has two, and only the pair says so. That is what binds a `--slice` cost report
   (`trunkline token-cost`) to this run rather than to a wall clock.
   **`done` also carries `confirmations`, `rollbacks` and `duration_min`, and all three are counted,
   not recalled.** Read the journal back once, keep this run's `slice`, and count its `approval` and
   `rollback` events; the duration is this line's `ts` minus the `start` line's. This is the only
   place in the pipeline that reads the journal before writing to it, and the reason is in the
   reference: these are totals over the run, so an agent writing from memory writes `null`, and a
   `done` line with empty fields is indistinguishable from a complete one.
6. **Commit — only if the base is inside this repository.** A separate commit:
   `docs(knowledge): slice entry <YYMMDD>-<slug> — <title>`. This step and step 18
   are the only two in the pipeline that commit at all (see "The commit boundary is also the recovery
   boundary" in the core), and this commit carries **only** the knowledge directory — `adr/`,
   `modules/`, `slices/`, `INDEX.md` — including an ADR step 15 wrote, which lives there and has not
   been committed by anything else. No code, no specs, no tests. Those were committed by step 18 with the changeset that produced them; anything
   of theirs still unstaged here is a defect of step 18, and the fix is to say so, not to sweep it
   into this commit.

   **A base outside this repository is not committed from here.** When `knowledge` names an external
   path — a company base several services share, `/srv/knowledge` or `../platform/...` — this
   repository has nothing of it to stage, and `git add` from here would either fail or reach into
   another checkout. Say where the base is, say what was written into it, and leave the committing to
   whoever owns it. The run is complete either way: the base is written, and that is what step 1 of
   the next task reads.

7. **Sweep the workdir.** Run
   `.trunkline/core/developer/scripts/workdir-clean.sh` — it deletes files under
   `.pipeline/workdir/` last modified more than a week ago, skipping the journal and skipping any
   run that has no `done` line yet. It runs here because this is the step that has just written the
   `done` line: at any earlier point this run's own files are still live. Report the line it prints;
   a sweep that found nothing is the normal outcome, and a failure here does not fail the run —
   housekeeping is not a gate.

## Writing rules

- **Write for an agent, not for a human reader, and name things only where you state a fact about
  them.** "Refactored the service" is useless; "`plan.Service.Create` now takes `context.Context` as
  its first argument" is useful. But a list of every type a file declares is an inventory, not a
  fact: a grep rebuilds it in seconds, and it goes stale on the next rename while saying nothing that
  would be missed. Name the symbol an invariant constrains, a rule is enforced at, or a consumer
  calls — and stop there.
- **A delta, not a full dump (target size ≤ 150 lines), and what to cut is decided by one question:
  does a grep answer it?** Signatures, type lists, column names, route tables — re-derivable in
  seconds, stale on every rename; those go first. What only the author knew goes last, if ever: why
  the pagination is keyset and what breaks without it, which check lives in this module and is
  deliberately not duplicated by the caller, which value was chosen and which was rejected. A module
  file that got under the budget by losing its pitfalls did not get compact, it got useless — that is
  the more expensive of the two failures, because a bloated file merely costs context and a hollowed
  one silently stops warning anybody.
- **Delete what is stale, do not accumulate it, and split by the question answered rather than by
  age.** The module file says **what holds now**; the slice entry says **why it was decided that way
  and what was rejected**. A module file is overwritten, so a stale fact in it is replaced; a
  rationale left in it is history that nothing will ever mark stale, so it is never replaced and the
  file grows by one paragraph per run. Rationale goes to the slice — that is what makes the module
  file bounded.
- **Do not duplicate what another artefact already holds** — "whose fact is this" is answered by the
  ownership table in `core/skills/project-knowledge/SKILL.md` ("Which artefact owns a fact"), and a
  fact belonging to a row above the module file reaches it as an identifier or a link, never as text.
  That covers the profile guidelines (those are rules; this is knowledge about this code) and the
  artefacts beside the code: the contract, `tests_readme.md` and `sequence_diagrams.md`. Retelling
  creates a second copy under a second numbering that drifts from the first and is complete in
  neither. Link, and record only what the other artefact does not say. A module whose diagrams are
  under some other file name is a defect to report, not an exception to work around — step 5 fixes
  the name, and a file outside it is invisible to 7a's checks of the error contract as well as to
  this rule.
- **Rejected alternatives matter more than the decisions taken.** They are what saves time on the
  next task and answers step 11's Chesterton's Fence.

**Output:** the slice entry is created, the module files are current, INDEX is updated, the journal
is closed, everything is committed.
