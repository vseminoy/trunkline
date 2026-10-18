# Step 21. Reconciler agent `[STOP — blocking]`

**Skills:** `core/skills/project-knowledge/SKILL.md` (the storage format — read it before 21.1),
`agent-skills/skills/doubt-driven-development/SKILL.md`
**Tool:** `trunkline kb-sync` — the machine half of checks 1-4

The reading-and-comparing half of the `knowledge` route. Step 2 has just derived what the module's
rules **ought** to be, from the documents the user supplied and from the code itself; this step puts
that beside what `.pipeline/knowledge/<project>/modules/<module>.md` **says** they are, and hands step 20 a
list of differences the user has approved.

Executed by the **main agent**, like step 7a and for the same reason: it needs the registry, the
entry and the code at once, and it holds the authority to block the write. Spawning is forbidden.

## When it runs

Only on the `knowledge` route, and always on it — it is that route's whole point. It is not
available as a conditional extra on the other six: a route that changes code records its delta at
step 20 from what it just did, and reconciling an entry against code the same run rewrote answers a
question nobody asked.

`knowledge: none` kills the route before this step: there is no base to reconcile against, and that
is said at classification rather than discovered here.

## 21.1 The scope is named, not derived

**One module per run.** Ask, once, at the start, which module — a directory path. "The project" is
not a scope, and neither is "everything that looks stale": the entry is per module, the ≤150-line
budget is per module, and a run that reconciles four modules produces one approval covering four
independent judgements.

Announce the path you settled on, and the entry file it resolves to, before reading anything. Where
`INDEX.md` has no row for the module, say so: this run will be the module's **first** description
rather than a reconciliation, which changes what the report can claim — there is nothing to
contradict, only something to write.

## 21.2 The materials are asked for, not assumed

Before comparing, ask the user for what is not in the repository:

- documents describing the module — a specification, a tracker epic, a page of a wiki, an old
  handover note;
- a plain-text description, if there is no document — what the module is for, what must always hold,
  what is deliberately not done here;
- whether anything is known to be out of date already.

**Ask once and accept "there is nothing".** A module with no documentation is the ordinary case and
not a blocker: the run then compares the entry against the code alone, and says in the report that
it did. What is forbidden is comparing against a document nobody offered and calling its absence a
clean result.

Whatever the user supplies goes into step 2 as the task text — that is how a requirement of a
document acquires `Origin: task item N` and becomes machine-checkable in the registry. A document
arriving **after** step 2 has run is a rollback to step 2, not an amendment made here.

## 21.3 The mechanical half

Run the script over the module and the registry step 2 wrote:

```
trunkline kb-sync \
    --module src/payments \
    --project <the `project` flag> \
    --rules .pipeline/workdir/rules/260813142530.md
```

It decides what a grep can decide — an enforcement anchor that no longer resolves, an identifier a
registry declares and the entry records nowhere, a file the entry describes that has been deleted, a
source file nothing describes, an entry past 150 lines, an
`INDEX.md` row that disagrees with the directory, and commits touching the module since the entry
was last updated. It decides nothing about meaning.

Its output is evidence for the table below, not the table. **A green script is not a green step 21:**
every anchor resolving says the entry points at code that exists, not that it says true things about
it. That is the half this step exists for, and it is the half that follows.

## 21.4 The semantic half

Three questions, in this order, each answered against the code — not against the entry, and not
against the document. The entry and the document are the two claims under test; the code is the only
witness.

1. **Contradiction.** A statement in the entry or the document that the code refutes. The strongest
   finding this route produces and the reason it exists: an entry that lies is worse than no entry,
   because step 1 of every later run takes it as the starting AS-IS.
2. **Missing rule.** Behaviour the code enforces that no rule in the entry states. These arrive from
   step 2 with `Origin: development` — they are constraints implied by the existing code, which is
   exactly the category step 2 already defines.
3. **Stale fact.** Something the entry says that was true and no longer is — a renamed symbol, a
   moved file, a dropped column, a consumer that no longer consumes.

A fourth category is reported and **not acted on**: a requirement in the supplied document that the
code does not implement. That is a defect of the code or an obsolete document, and either way it is
a `feature` or a `bug` the user decides to open — never something this route quietly writes into the
entry as though it were in force. Say which of the two you think it is, and stop there.

## 21.5 The report

```
| ID / subject | Entry says | Code says | Kind | Proposed |
|---|---|---|---|---|
| R-004-2 | the amount is > 0 | service.go:51 checks >= 0 | contradiction | correct the entry |
| — | — | handler.go:88 rejects currency not in the list | missing rule | add R-263-4 |
| Entry points | POST /payments/charge | removed in 3a91f2c | stale fact | delete the row |
| doc item 3 | — | not implemented anywhere | doc vs code | open a `bug`, do not record |
```

`Proposed` is what step 20 will write if the user approves — one line each, concrete enough to be
executed without rereading this table's evidence.

**Every finding names its witness in the code** (`file:symbol`, and a commit where the answer is "it
was deleted"). A finding whose evidence is "the entry looks out of date" is not a finding; it is the
feeling that made the user start this run, and it is what the step is supposed to replace.

**An empty table is a real outcome and is reported as one.** "The entry agrees with the code on
every point checked" is worth the run — it converts a suspicion into a dated fact, and step 20
records it by refreshing `Updated:` and nothing else.

## `[STOP]`

**Nothing is written until the user approves this table.** The differences are hypotheses derived
from code by a model, and `slices/` is append-only: an entry written on a wrong reading cannot be
retracted, only contradicted later by another run. This is the one approval of the route, and it is
recorded in the journal as an `approval` event for step 21 the moment it is given (the format is in
`core/developer/references/pipeline-evals.md`).

It is **not** one of the four MANDATORY APPROVALS. Those authorise writing production code, and this
route writes none — which is also why `core/developer/scripts/approval-gate.sh` neither counts it nor
waits for it. Recording it is for the count in `done` and for the record that a human read the table,
not for a gate.

The user rejecting a row is a normal outcome: it drops out of `Proposed` and step 20 does not write
it. The user rejecting the whole table sends the run back to 21.4 with what they said, not to step 2.

**Output:** the approved difference table, the script's run recorded (`pass` / `fail` /
`unavailable`), and the list of rules step 20 is to add, correct or retire.
