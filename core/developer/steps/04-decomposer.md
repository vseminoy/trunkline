# Step 4. Decomposer agent `[STOP]`

**Skills:** `agent-skills/skills/planning-and-task-breakdown/SKILL.md`

Apply the full process from the skill, including the sizing system (XS/S/M/L/XL).

Splits the work into changesets, cut vertically (each one = working functionality from API to
database):
```
Changeset 1: [endpoint/method] (spec + migration + handler + service + test)
Covers: R-007-1, R-007-2, R-007-3
```

Every changeset must list which rules it closes — every `R-<run>-<n>` in the registry, whatever its
`Origin`. A rule that lands in no changeset is either lost or must be explicitly marked as deferred.
This matters most for the rules whose `Origin` is an item of the task text: someone else wrote that
requirement down and the task promises it, so it cannot drop out silently.

**Write the assignment into `.pipeline/workdir/rules/<NNN>.md`** — the `Changeset` column step 2 left empty. One value per
row: the changeset number, or `— deferred` with the reason given in the report. This is not
bookkeeping. Step 7a runs once per changeset and reads that column to know which rules this one
answers for; without it every rule belonging to a later changeset arrives at the first one as a
`GAP`, and step 7a forbids step 8 on any gap. An unfilled column therefore does not weaken the
check — it deadlocks the route. A row left blank is reported by `trunkline trace-check` as a rule assigned
to nothing, which is the machine form of "lost" above.

Slicing rules:
- the unit is one vertical capability (one endpoint, one use case) — not a file count
- vertical only, never horizontal
- file count is a guardrail, not the target: past ~8 files, state in the plan why this is still one
  slice rather than splitting it — the human approves that call at this step's `[STOP]`, same as any
  other changeset decision here
- an XL task → split it further

**The explicit dependency question (mandatory):**
Once the changeset list is drawn up, apply the decision tree from the skill — its "Parallelization
Opportunities" analysis, which separates what may be reordered from what may not — and state the
result outright:
```
CHANGESET DEPENDENCIES:
Changeset N and Changeset M are independent (no shared files, no ordering dependency).
Changeset K must follow N: it needs the contract N defines.
Proposed order: N → K → M. Independent, so M can also be deferred out of this task.
Confirm the order, or say which changeset should go first.
```

**Changesets are executed one at a time, and the question is their order, not their concurrency.**
The skill's parallelization section opens with "when multiple agents or sessions are available";
here there is one main agent, and every changeset carries up to four human approvals (steps 5, 6, 7,
7a) at its own `[STOP]`s. Two changesets in flight would be two interleaved approval streams in one
conversation, and the loop that carries them (step 18, "The changeset loop" in the core) hands on to
exactly one successor. So do not offer to run changesets in parallel: it is not a mode this pipeline
has, and an offer of "less time" that nothing can deliver costs the user a decision for nothing.

What independence does buy is worth asking about, because all three are executable today:
- **order** — foundation changesets first (the contract, the schema, a migration others build on);
- **an unblocked next** — if the changeset in hand stalls on a decision or an external dependency,
  an independent one can be taken next instead of the run stalling with it;
- **deferral** — an independent changeset can be dropped from this task outright; the `Changeset`
  column already has `— deferred` for exactly that, and the reason goes in the report.

If the changesets depend on one another — name the dependency and offer only the order it forces.

**IMPORTANT: read-only with respect to the project.** Changing code, creating project files and
running state-changing commands are forbidden. The one exception is the `Changeset` column of
the registry above — this step's own output, the same way step 2 owns the rest of that file.

**Output:** changesets with acceptance criteria, rule coverage and verification steps; the
`Changeset` column of the registry is filled; the order is agreed.
