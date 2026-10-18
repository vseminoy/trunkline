# Step 2. Analyst agent `[STOP]`

**Skills:** `agent-skills/skills/spec-driven-development/SKILL.md`

Apply the four-phase process from the skill: SPECIFY → PLAN → TASKS → IMPLEMENT.

Produces AS-IS / TO-BE, Use Cases, Business Rules. Unclear requirements → Open Questions.

AS-IS comes from the delta context of step 1 and is **extended**, not rebuilt.

## BUSINESS RULES with identifiers (mandatory)

Every checkable rule gets a stable identifier, carried through the whole task. It is the only
traceability mechanism for "requirement → spec → test → code", and step 7a runs on it.
**This step is the only place identifiers are assigned** — see "Rule traceability" in the core.

Three sources, in this order.

**First — the requirements in the task text.** Copy the "How to test" block out of the task
verbatim, one line per item, into the registry file. Then derive one rule per item. An EARS
statement may be compressed into a checkable assertion, but **its meaning and boundaries must not
change**; if a requirement contradicts the code or cannot be implemented, that is an Open Question
for the user, not a silent edit of someone else's requirement.

Copying the items is not bookkeeping. It is the only record of what the task asked for: the text was
pasted into a prompt and exists nowhere else. Step 7a's check 6 compares the two halves and reports a
requirement that became no rule — which a registry alone cannot show, being self-consistent without
it.

On `knowledge` the "task text" is what the user supplied at step 21.2 — a specification, an epic, a
handover note, or a description they typed. It is copied the same way and its items get the same
`task item N`, which is the whole mechanism by which "the document says X" becomes something
`trunkline trace-check` can count. Nothing about this step changes for that route: it needs no new origin,
because a rule read out of the code is what the third source below already calls a constraint implied
by the existing code.

**Second — the rules already in force in the modules being touched.** Step 1 read
`.pipeline/knowledge/<project>/modules/<module>.md`; its "Business rules in force" table lists what earlier
runs established. A rule already there is **reused under its existing identifier**, not registered
again under a new one. Two rows saying the same thing in different words is how a module's rule table
stops being able to answer "what is this rule". Where a new rule **supersedes** one in force, say so
explicitly — step 20 records the supersession, and the old identifier stops being current.

**Third — new rules.** Domain decisions that surfaced at steps 1–6 and constraints implied by the
existing code. Rules that follow from the profile guidelines are not registered: they are mandatory
by default, and restating them adds an identifier nobody enforces separately.

```
BUSINESS RULES for the specification:
- R-007-1 (task item 1): [statement, including boundary values, ranges, constraints]
- R-007-2 (task item 2): [statement]
- R-007-3 (development): [a rule that surfaced during diagnosis]
- R-004-2 (in force, reused): [an existing rule of this module the task also relies on]
→ The Specifier agent must express every rule in the YAML (validation, enum, a `description`
   stating the rule) and in the sequence diagram (an error branch for every rule violation), and
   record where each ID landed in its coverage record.
→ The Tester agent must cover every ID with at least one test and record which tests cover it in
   its coverage record.

The identifiers stay in this registry and in those records, all under `.pipeline/workdir/`. Nothing
committed carries them — see "Rule traceability" in the core.
```

Numbering rules:
- The form is `R-<run>-<n>`: this run's number, then a counter from 1. The run number was minted at
  classification, before the first step — see "The run number" in the core. It is not re-derived
  here, and it is not looked up in the journal, which `metrics: none` may have switched off.
- A number is never handed to another rule. If a rule is withdrawn, mark it
  `R-<run>-<n>: [WITHDRAWN]` and leave the number spent.
- A reused rule keeps the identifier of the run that first registered it — that is the point of
  reusing it.
- A requirement of the epic that this task does **not** close belongs to another task and does not
  enter the registry. Where the analysis state was read, list those in the report so the boundary of
  this task is visible.

## The registry goes to a file: `.pipeline/workdir/rules/<NNN>.md`

Once the user has approved the BUSINESS RULES, write them to
**`.pipeline/workdir/rules/<NNN>.md`**, where `<NNN>` is this run's number — the same one the journal
puts in `slice` and step 20 gives the knowledge entry. Two sections, and both are read by
`trunkline trace-check`:

```markdown
# Rules of run 260813142530
Task: PROJ-855

## Requirements from the task text

1. WHEN a create request arrives, the service SHALL return 201 and the new identifier.
2. IF the number is already taken, THEN the service SHALL return 409.

## Rules

| ID | Rule | Origin | Changeset |
|----|------|--------|-----------|
| R-007-1 | create returns 201 with the identifier | task item 1 | |
| R-007-2 | a duplicate number gives 409 | task item 2 | |
| R-007-3 | the plan name is at most 100 characters | development | |
| R-004-2 | the amount is greater than 0 | in force, run 004 | |
| R-007-4 | superseded by R-007-3 [WITHDRAWN] | development | |
```

The first section is the task's "How to test" block, copied line for line — the `Origin` column
points back into it by item number. `development` marks a rule the task did not ask for, `in force,
run NNN` one reused from the module's knowledge base. Anything else in `Origin` and check 6 reports
the rule as tracing to nothing.

**One registry per task, not per changeset.** This step runs once, before the changeset loop, so the
file holds every rule of the task. It lives under `.pipeline/` rather than beside the code because
it describes **the task, not the module**: it is written once, read through the changeset loop and
finished with. A changeset is vertical by definition and touches several directories, so there is no
"the module" to put it beside. What does stay beside the code is what describes the code and grows
with it — `tests_readme.md` and `sequence_diagrams.md`.

**Leave the `Changeset` column empty.** Step 4 fills it when it assigns rules to changesets, and step
7a reads it to know which rules a given changeset is answerable for. On a route with no step 4 the
column stays empty and nothing reads it.

This is not bookkeeping for its own sake. Step 7a's cross-check has a machine half
(`trunkline trace-check`), and this file is its input: without a registry on disk
the only list of identifiers is the one the model retypes from its own context at step 7a, and a
rule dropped there is a rule the cross-check will never ask about. A registry the human approved is
what makes "every identifier reached every artefact" a fact rather than a claim.

The console output of this step stays as it is — the registry file is the same content, persisted. It is
rewritten in place on a rollback to this step, and it is committed with the changeset.

**Output:** AS-IS and TO-BE with concrete acceptance criteria, BUSINESS RULES with identifiers
(`R-<run>-<n>`, with the task's requirements recorded beside them) written to
`.pipeline/workdir/rules/<NNN>.md`, Open Questions agreed.
