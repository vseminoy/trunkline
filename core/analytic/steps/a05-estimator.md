# Step 5. Estimator agent `[the final document]` `[STOP]`

**Reference:** `agent-skills/references/definition-of-done.md`

Writes each task up as a separate `.md` file, adds the estimate and the final order.

### Acceptance criteria ≠ Definition of Done

These are two different instruments, and confusing them means either bloating every task with
generalities or losing a constant quality bar.

| | Acceptance criteria | Definition of Done |
|---|---|---|
| What it is | **Unique to the task**: what exactly must start working | **The same for all** tasks: the project's quality bar |
| Who owns it | The analyst — writes it in the "How to test" block | The project — the bar it adopts; the shipped default is `agent-skills/references/definition-of-done.md` |
| Where it lives | In the body of every task | **Once**, outside the tasks |

**Rule: the DoD is not copied into tasks.** "Tests written", "the linter passes", "the code was
reviewed" — that is the project standard, checked by the developer at their step 17, and it should
not take up room in every task description. Only criteria specific to the task go into it.

**What the analyst must check against the DoD** (`agent-skills/references/definition-of-done.md`) — not "is it done", but **does the
task contain everything the developer needs in order to do it**:

- **Correctness.** The "How to test" block makes clear which behaviour to check, including the
  **error branches** (`IF … THEN`), not just the happy path. If a task has no checkable criterion
  at all, it is not ready.
- **Integration.** If the task touches migrations, configs or feature flags — that is named
  explicitly rather than implied.
- **Backward compatibility.** If a public contract changes — compatibility is either preserved or the
  break is named and agreed (this overlaps with the step 3 impact analysis).
- **Ship-readiness.** If the task touches untrusted input, authorization or personal data — that
  is noted in the description so the developer brings in the security check rather than discovering
  it at review.

A gap on any item is not a reason to add a generic sentence to the task but a reason to **send the
requirement back to the Analyst** (step 2) or to ask the user.

**File formatting follows the format spec strictly.** Before writing anything the
agent MUST read that document and follow it literally: the front-matter structure, the required
fields, the three-block body, file naming and placement in the epic directories all come **from
there**. The format rules are not duplicated in this prompt; on a divergence, the format spec wins.

### The "How to test" block — from the requirements

The **"How to test"** block of each task is assembled from the `R<n>` anchored to it by the
coverage matrix (step 4): **one item per requirement, using the text of the EARS requirement itself**.

Three rules that follow from the format spec:

- **The `R<n>` identifiers do not go into the task body.** They are working notes of this epic: they
  serve the coverage matrix and stop at its edge. What the developer needs is the requirement itself,
  and an EARS statement is self-contained — it reads without any registry. Their step 2 copies this
  block verbatim, derives one rule per item and assigns the identifiers that end up in the code, so
  the count of items is what proves nothing was lost in the handover. That is why **one item means
  one requirement**: two requirements in a line, or a line of prose that is not a requirement at all,
  break the only mechanical check that spans the two tracks.
- **Tables only in JIRA wiki syntax** (the publisher contract, the section on JIRA wiki markup —
  the format spec refers to it rather than restating it): `|| Header ||`; markdown tables are
  forbidden. If a
  task has 1–3 requirements, use a numbered list — a table is overkill.
- **Non-functional requirements and reproduction steps** are added to the block separately — they are
  not expressible in EARS, but they must not fall out of "How to test".

An example block for a task covering R1 and R2:

```text
1. WHEN a request to create a subscriber is received, the service SHALL return 201 and the identifier of the created record.
2. IF a subscriber with that number already exists, THEN the service SHALL return 409 and the error code SUBSCRIBER_EXISTS.
```

### Where the files go — derived, then announced

**The directory and the epic are not asked about.** The directory is the profile's `tasks.dir`, the
subdirectory is the epic key, and the key was given when the work started — it is what the state
file is named after and what the whole route has been carrying since step 1. Asking again is not
caution: an answer that differs from the slot puts the files where neither the developer's step 1
(`core/developer/steps/01-diagnost.md`) nor the publisher's scanner will look, and both of them
resolve the path from the manifest rather than from this conversation.

So **say the resolved path and carry on**:

```
Saving into <tasks.dir>/<EPIC>/ — 5 task files. The analysis state is <tasks.analysis_dir>/<EPIC>.md.
```

Ask only where there is genuinely nothing to derive from, and ask about that one thing rather than
about the layout as a whole:

- **The profile declares no `tasks.dir`** — that is a defect in the profile. Say so, name what is
  missing, and ask where to put the files; do not invent a directory (see "SCENARIOS AND THE
  PROFILE" in `core/analytic/PIPELINE.md`).
- **The directory does not exist** — ask whether to create it. That is a question about an action,
  not about a path.
- **No epic key** — the route should not have got this far without one; go back rather than
  guessing a directory name.

### What to ask the user (MANDATORY, before saving)

One question remains, and it is about content rather than placement. Do not write files until it is
answered; until then the result may be shown in the reply.

**The front-matter fields** — how to fill in the task's fields (the field set and rules are in the
format spec). Values the agent can derive from the analysis (e.g. the blockers for `links` from the
dependency graph) it fills in and shows for confirmation rather than asking about blind.

### The estimate

The format spec has no field for an estimate, so it is **not written** into the task files. The
estimate for each task (hours / size / dependencies) goes **into the summary report**, which the
agent shows the user (see "Final artefacts" below), not into the task `.md` files. The estimation
formula follows the table below.

### Estimation table (for a mid-level developer)

The estimate is high-level, in working hours, and covers **the whole development cycle of a task**,
not just writing the code. Reference: a working day ≈ 8 hours.

Each task's estimate includes the time for every stage:
- writing or amending the contract, and agreeing it;
- DB migrations and the queries against it;
- writing the production code;
- **writing the tests** (unit/integration);
- code review and the rework it produces;
- getting through CI, debugging failures, fixing them.

| Size | Hours | Volume | Example |
|--------|------|-------|--------|
| **XS** | ~4 h  | 1 file, a trivial change | add validation to a field |
| **S**  | ~8 h  | 1–2 files, one endpoint/method | a new simple handler + test |
| **M**  | 16–24 h | 3–5 files, one slice | an endpoint with a migration, a service and tests |
| **L**  | > 24 h | **too big — split it** | — |

- **A hard ceiling: 24 working hours per task.** Any task estimated at more than 24 hours goes
  back to the Decomposer (step 4) to be split.
- Account for: unfamiliar code (+time), code generation from the profile's `stack` (saves time on
  structures), migrations and contract negotiation (separate work), hot paths at a high `rps` (extra
  attention to correctness and performance).
- The estimate is for a mid-level developer: assuming no expert knowledge of this module, but no
  learning from zero either.

### Estimating through the requirements

The number and type of `R<n>` anchored to a task are **an input signal for the estimate**, not the
estimate itself. The order of work: first gauge the volume from the requirements, then adjust against
the size table above.

**Count with decay.** Requirements inside one task share a skeleton: the handler, the table, the
mocks, the test file. Only the **first** requirement in a layer pays full price — later ones in the
same layer cost a fraction. Multiplying "number of requirements × price" directly overstates the
estimate by roughly a factor of two and leads to splitting healthy tasks.

Reference rates (the whole cycle: code + test + review + CI):

| Requirement type | First in a layer | Each subsequent one in the same layer |
|---|---|---|
| ubiquitous | ~1 h | ~0.25 h (usually already covered by the guidelines) |
| event-driven | 4–6 h | 1.5–2 h |
| state-driven | 5–8 h | 2–3 h |
| unwanted | 2–3 h | 0.5–1 h |
| optional feature | +3–4 h | +1 h |

Example: a service with 5 event-driven and 8 error requirements in one layer →
`(5 + 4×1.75) + (2.5 + 7×0.75)` ≈ **20 h**, not the 38 h of direct multiplication.

**A multiplier for the task's character.** The rates above are for code with business logic. If
the task is a **thin proxy / passthrough** (forwarding a call with no validation, no calculation
and no state handling; all the logic is on the other side), every rate is multiplied by **0.7**. The
sign: the "Solution" block contains nothing but forwarding calls and a permissions check.

**Migrations are counted at a flat rate, not by requirements.** The migration layer is estimated at
**3–5 h** (schema + indexes + reversibility + a run and a rollback) **on top of** the requirements,
regardless of how many requirements describe it. Costing a migration as a "ubiquitous" requirement at
1 h is a systematic underestimate: the decay rule does not apply to migrations, and two migrations
cost as much as two.

**Retiring the old path is estimated separately.** If the task not only adds behaviour but also
retires the previous implementation (deleting an endpoint, a branch, a table; switching consumers;
Strangler Fig) — that is **+2–4 h** as its own line. Requirements do not express that work: they
describe the target behaviour, not the dismantling of the previous one, so counting from the registry
loses it. The reference for it is the strategy the Impact Analyst chose at step 3.

- **Never zero out error requirements.** Every `IF … THEN` is a separate code branch and a separate
  test. Cheap, but **never zero**.
- **Decay is computed per layer, not per task.** Requirements landing in different layers (handler
  / service / migration) share no skeleton — each pays full price as the first in its own layer.
- **A divergence is a signal.** If the estimate from the requirements and the estimate from the size
  table differ **by more than a factor of 1.5**, the agent does **not** silently take the larger one
  but writes about it in the "Risks" block of the summary report. Usually it means one of two things:
  the requirements registry is incomplete, or the task is badly bounded.

**The calculation order (in full):**
```
estimate = (the sum over requirements with decay, per layer) × the character multiplier
         + the flat migration rate (3–5 h per migration)
         + retiring the old path (2–4 h, if applicable)
```

**The rates above are defaults, and a project may have measured its own.** If the profile declares
`tasks.estimation_calibration`, read that file and prefer its figures: how they were calibrated, on
what sample, and where they are known not to fit belong to the team that measured them, not to a
stack-agnostic prompt. Without the slot the defaults stand as they are.

As closed tasks accumulate, that file is where the actual figures go — compare plan against fact and
adjust the base rates first, then the decay coefficient.

### The final order

Order the tasks so that **the ones others depend on come first** (a topological sort of the
dependency graph). Where dependencies are equal, the riskier and more foundational ones (contract,
schema, migration) come earlier.

### Final artefacts

1. **The task files** — one `.md` file per task, written strictly per the format spec (the
   directory, the naming, the front-matter and the body all come from there).
2. **The summary report** — shown to the user in the reply; saved to a file only on request (it is
   not a task for the publisher). It is a **view** of what the analysis state already holds, not a
   second carrier: the `R<n>` → task trace below is the same table as in the state, and the state
   is what the developer reads. If the two ever disagree, the state wins and the report is redrawn.
   The reply that carries this report names the state's path:

```markdown
# Decomposition: <task title>

## Summary
<1 paragraph: what we are doing and why>

## Tasks (in execution order)
| Prefix | File | summary | Requirements | Estimate | Blocked by (links) |
|---------|------|---------|-----------|--------|---------------------|
| C1 | C1-...md | ... | R3 | 8 h | none |
| C2 | C2-...md | ... | R1, R2 | 12 h | C1 |
...

## Requirements traceability
| ID | Requirement (short) | Type | Anchor | Justification |
|----|--------------------|-----|----------|-------------|
| R1 | create subscriber → 201 | event-driven | C2 | — |
| R2 | duplicate number → 409 | unwanted | C2 | — |
| R3 | 503 during the migration | state-driven | C1 | — |
| R4 | send SMS via the gateway | event-driven | DEMO-546 | Closed, implemented in common_sms_notification |
| R5 | Telegram channel | event-driven | — dropped | out of scope, the user's decision |

**Coverage:** <N> requirements → <M> new tasks, <K> covered by existing tasks, <L> dropped.
Uncovered requirements: <none | list>.

_(this section is omitted in scenarios B and C if the requirements registry was not reconstructed)_

## Dependency graph
<a text diagram: what depends on what>

## Estimate summary
**Total:** <sum of hours>

## Risks and open questions
<what could increase the estimate, what needs clarifying before the start>
<divergences between the requirement-based estimate and the size table — if any>
```

### Stopping criterion (HARD)

The Estimator saves the files **only when ALL of these hold**:

1. **The "How to test" block is non-empty in every task** and contains the EARS requirements
   anchored to it by the coverage matrix.
2. **The trace is complete** (only if a requirements registry exists — scenario A, or a reconstructed
   registry in B/C). Every `R<n>` has an anchor of one of the three kinds; the "existing task" and
   "dropped" anchors are justified; the "Uncovered requirements" line is filled in explicitly (`none`
   or a list).
3. **The trace is written down where it survives the session** — the "Requirements (EARS) and
   coverage" table of the analysis state is filled in, with statements copied verbatim from the
   registry, and **the state's path is named in the final reply**. A trace that exists only in the
   report is not saved: the report is a view, the state is the carrier, and this is the single item
   on which the whole bridge to the developer rests.
4. **There are no `R<n>` codes in the task bodies** — they live in the state and in the report (see
   above).
5. **Tables in the task bodies use JIRA wiki syntax**; there are no markdown tables (the publisher
   contract, the section on JIRA wiki markup).
6. **No task exceeds 24 hours**; any that did were sent back to the Decomposer.
7. **The path was resolved from the profile and announced, and the front-matter fields are agreed**
   with the user (see above). The directory and the epic are derived, not negotiated.
8. **Readiness against the DoD (`agent-skills/references/definition-of-done.md`) is checked** (see "Acceptance criteria ≠ Definition of
   Done"): every task has checkable criteria including the error branches; migrations, configs and
   flags, backward-compatibility breaks, and work with untrusted input, authorization or personal
   data are named explicitly where they apply. The generic DoD items are not copied into the task
   bodies.

**Output:** a set of `.md` task files per the format spec (each in the Problem / Solution / How to
test format, with the "How to test" block assembled from the EARS requirements, ≤ 24 working hours,
ordered by prefix following the dependencies) plus the summary report with the estimates and the
requirements trace.
