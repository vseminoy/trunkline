# Step 4. Decomposer agent `[STOP]`

**Skills:** `agent-skills/skills/planning-and-task-breakdown/SKILL.md`, `agent-skills/skills/doubt-driven-development/SKILL.md`

Splits the epic into tasks. Apply the full process from the skill.

**The unit of splitting is a requirement.** Decomposition follows the `R<n>` registry from step 2: a
task is a coherent group of requirements, not "a chunk of work by eye". That makes the splitting
checkable rather than intuitive.

**Splitting principles:**
- **Vertical slices**, not horizontal: every task is a finished piece of functionality (where
  possible from the contract/DB up to the handler and the tests), not "all the DB separately, all the
  handlers separately". The exception is the foundation tasks the others depend on (contract,
  schema, migration): those are pulled out separately and come first.
- **Every task ≤ 24 working hours** for a mid-level developer (see the estimation table in
  `core/analytic/steps/a05-estimator.md`). If it is more — split further.
- **A requirement is never split across tasks.** One `R<n>` is implemented entirely within one
  task. If a requirement will not fit into 24 hours, that is not a decomposition problem but a
  sign the requirement is too large: send it back to the Analyst (step 2) to be split, rather than
  cutting it across tasks. A cut-up requirement cannot be checked by one test, and that breaks the
  whole trace.
- **An `IF … THEN` travels with its `WHEN`.** A requirement for an error scenario lands in the same
  task as the main scenario it belongs to. Deferring error handling into a separate "later"
  task is forbidden — that is exactly how error branches fail to reach production.
- Signs a task needs splitting further: the title contains "and"; it touches two independent
  subsystems; it cannot be described in one paragraph.
- A task should leave the system in a working state (where applicable).

**What to measure size by: points of change, not the number of requirements.** Count the **layers and
tables** touched, not the number of `R<n>`. Twelve requirements against one service with one table
and two endpoints is **one** healthy task (a shared skeleton, shared tests); two requirements
against two different subsystems is **two** tasks.

The signal to split: more than **3 layers** or more than **2 tables** in one task. The number of
requirements is not in itself a signal — there is no reason to split eight error branches of one
handler.

**The coverage matrix (a mandatory artefact).** Every requirement gets one of **three** kinds of
anchor:

| Kind of anchor | Value | When |
|---|---|---|
| A new task | a prefix (`C2`) | the requirement is implemented in this pass |
| An existing task | a JIRA key (`DEMO-793`) | the requirement is already covered by an existing task — do not create another |
| Dropped | `— <reason>` | the requirement is already implemented in the code, or was taken out of scope by the user's decision |

```
| ID | Requirement (short)            | Anchor                          |
|----|--------------------------------|---------------------------------|
| R1 | create subscriber → 201        | C2                              |
| R2 | duplicate number → 409         | C2                              |
| R3 | 503 during the migration       | C1                              |
| R4 | send SMS via the gateway       | DEMO-546 (Closed, implemented)   |
| R5 | Telegram channel               | — out of scope, the user's decision |
```

Anchors of the kinds "existing task" and "dropped" **must be justified in one line** — with a
reference to the code (what exactly is already implemented) or to the user's directive. Dropping a
requirement without justification is forbidden: that is a silent loss of scope.

### Checking the decomposition with a fresh context (MANDATORY)

An error in the decomposition costs more than an error in the code: it multiplies across every
task and surfaces only at the developer. So the shape of the epic **is not taken on faith by your
own context** — it goes through the `CLAIM → EXTRACT → DOUBT → RECONCILE → STOP` cycle from
`doubt-driven-development`.

Applying the cycle to a decomposition:

1. **CLAIM.** State the claim being checked, e.g.: "the coverage matrix is complete and consistent:
   every `R<n>` is anchored exactly once and there are no extra tasks".
2. **EXTRACT.** Assemble the minimal checkable artefact: **the requirements registry (step 2) + the
   coverage matrix + the task list with boundaries**. Do **not** include your reasoning about why
   the shape is what it is.
3. **DOUBT.** Run a reviewer in a **fresh context** (a subagent) with an adversarial prompt. Pass
   **only the ARTEFACT + CONTRACT, without the CLAIM** — otherwise the reviewer adapts to the
   conclusion. The CONTRACT for this step is items 1–7 of the stopping criterion below.
   Ask: which requirement is lost, which is cut across tasks, which task covers no `R<n>` at
   all, where an `IF … THEN` is torn from its `WHEN`, which justification for dropping does not hold
   up.
4. **RECONCILE.** Classify each finding against the text of the artefact: confirmed (fix the
   decomposition) / rejected (with a justification) / out of scope. Silently ignoring findings is
   forbidden.
5. **STOP.** No more than **3 cycles**; exit earlier once only trivial findings remain.

If the cycle exposed a lost or cut-up requirement, that is a defect of **step 2**, not step 4: go back
to the Analyst rather than "topping up" the registry in the Decomposer (see the rule after the
stopping criterion).

The result of the cycle (what was checked, what was found, what was fixed) is recorded in the summary
report in one paragraph.

### Stopping criterion (HARD)

**Applicability.** Items 1–5 apply **only if a requirements registry exists** — that is, in scenario
A, where the Analyst (step 2) ran. In scenarios B and C there is no registry (see "Tracing in
scenarios B and C" below) — there, only items 6–7 are checked.

The Decomposer reaches its `[STOP]` **only when ALL applicable items hold**:

1. **Completeness.** Every `R<n>` from the step 2 registry appears in the coverage matrix — with an
   anchor of any of the three kinds. There are no unanchored requirements.
2. **No duplicates.** Every `R<n>` is anchored to exactly **one** task (for anchors of the "new
   task" kind).
3. **Nothing extra.** Every task covers at least one `R<n>`. A task anchored to no requirement
   is either scope creep (remove it) or a missed requirement (send it back to the Analyst at step 2).
   The exception is purely infrastructural foundations (creating a migration, generating from the
   spec): mark those explicitly as `infra` and justify them in one line.
4. **Requirement integrity.** No `R<n>` is cut across tasks.
5. **Error scenarios in place.** Every `IF … THEN` sits in the same task as the `WHEN` it relates
   to.
6. **Justifications recorded.** Every anchor of the "existing task" and "dropped" kinds carries a
   line of justification.
7. **The order agrees with the impact analysis.** The ordering constraints from step 3 are reflected
   in the dependency graph.
8. **The doubt cycle has been run** (this applies in every scenario, including B and C). The
   fresh-context review was performed, every finding was classified, and the stop condition was
   reached. Skipping the cycle is permissible only on the user's explicit instruction and **is
   recorded in the report** — a silent skip is forbidden.

If items 1, 3 or 4 are violated — **do not "top it up" yourself; go back to the relevant step** (2 or
3) and tell the user exactly what diverged.

### Tracing in scenarios B and C

Scenarios B (review + estimates) and C (closing questions) enter the pipeline **without step 2**, so
they have no `R<n>` registry. The work goes like this:

- **By default tracing does not apply.** The Decomposer works from the existing tasks; criteria
  1–5 are skipped with an explicit note "no requirements registry (scenario B/C)".
- **The registry can be reconstructed** — if the user asks for coverage completeness to be checked.
  The source: the "How to test" blocks of the tasks already written, plus requirements from
  related JIRA tasks. A reconstructed registry is marked as such, and items 1–5 are checked against
  it in the normal way.
- **The doubt cycle applies here too**, but the artefact for EXTRACT is not the coverage matrix but
  **the set of existing tasks**, and the CONTRACT is the format spec plus coverage completeness
  against the epic description. The question to the reviewer changes from "what is lost from the
  registry" to "which scenario from the epic description does no task cover".
- **Gap analysis.** When some requirements are covered by tasks already created in JIRA, that is
  normal rather than a gap: those requirements get an anchor of the "existing task" kind. Do not
  create new tasks for them.

**Build the dependency graph** (what depends on what) and order it bottom up — the foundation first:
```
DB schema / migration → database queries → service → handler → contract consumers
contract              → code generation  → handler
```
(the concrete layers and tools come from the profile's `layers` and `stack`)

**IMPORTANT: read-only mode.** Changing code, creating project files and running state-changing
commands are forbidden.

**Output:** the task list with draft boundaries, the `R<n>` → task coverage matrix, and a
preliminary dependency graph.
