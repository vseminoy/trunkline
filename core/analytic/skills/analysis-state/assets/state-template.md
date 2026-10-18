# Analysis state file — template

```markdown
# Analysis state: <epic key / task title>

> The analyst's internal file. NOT a task for the MCP (no task front-matter).
> Updated at the end of every step. The source of truth on the state of the analysis.

## Meta
- Epic / task directory: PROJ-855
- Scenario: A (from scratch) | B (review + estimates) | C (closing questions)
- Current step: 4. Decomposer
- Session status: in progress | waiting on the user | finished
- Updated: 2026-06-18

## Context (what is already known)
- AS-IS: <how it works today — briefly; details and links in "Materials">
- TO-BE: <how it should work — briefly; details and links in "Materials">
- Affected layers / contracts: <the profile's `layers` + the contract>

## Materials (reference registry)
A single registry of every external artefact. Each material has a stable ID (`M1`, `M2`…) that the
tasks and estimates refer to — so the text is not duplicated and the references are not lost
between sessions. Describe a material once, here.

| ID | Type | What it is | Link / path | Freshness |
|----|-----|---------|---------------|--------------|
| M1 | db-schema  | DB schema (order tables)     | profile guideline: db_schema_style | 2026-06-18 |
| M2 | as-is-code | The current upload handler   | internal/handler/upload.<ext>:42 | current |
| M3 | spec       | The epic's spec directory    | specs/PROJ-855/                 | current |
| M4 | to-be-goal | Goal: add contract v2        | specs/PROJ-855/goal.md          | current |
| M5 | openapi    | The contract                 | api/openapi.yaml#/Upload      | stale   |
| M6 | knowledge  | AS-IS of the payments module | .pipeline/knowledge/<project>/modules/payments.md | 2026-08-05 |

Types (extensible): `db-schema` / `as-is-code` / `to-be-goal` / `spec` / `openapi` / `migration` /
`ticket` / `doc` / `knowledge` / `other`. Freshness: the date it was checked, or `current` / `stale`.

**Look in the code knowledge base first.** If the project has `.pipeline/knowledge/`, then AS-IS, the
entry points, the consumers and the rules in force are already described there — take them from there
and link to them (type `knowledge`) rather than regathering from the code. The base's format and the
access boundaries are in `core/skills/project-knowledge/SKILL.md`. The analyst **only reads** this
base; report any discrepancy between an entry and the code to the user, and the developer will make
the correction.

## Requirements (EARS) and coverage

The step 2 registry and the step 4 anchor in **one** table, because they are always read together:
whoever arrives here has a task key and needs both "which `R<n>` are mine" and "what exactly do
they say".

**This is the trace the developer's pipeline looks for at its step 1.** The `R<n>` codes are
deliberately kept out of the JIRA task bodies, and the summary report is shown in a reply and then
lost with the session — so this table is the only copy that survives. Fill it in as soon as step 2
produces the registry, and keep the anchors current from step 4 onward; a session that ends half-way
still leaves a usable trace.

| ID | Type | Requirement (EARS, in full) | Anchor | Justification |
|----|------|------------------------------|--------|---------------|
| R1 | event-driven | WHEN a request to create a subscriber is received, the service SHALL return 201 and the identifier of the created record. | C2 | — |
| R2 | unwanted | IF a subscriber with that number already exists, THEN the service SHALL return 409 and the code SUBSCRIBER_EXISTS. | C2 | — |
| R4 | event-driven | WHEN a subscriber is created, the service SHALL send the SMS via the gateway. | DEMO-546 | Closed, implemented in common_sms_notification |
| R5 | event-driven | WHERE the Telegram channel is enabled, the service SHALL duplicate the notification into it. | — dropped | out of scope, the user's decision |

- The anchors are the three kinds from step 4: a new task's prefix / an existing tracker key /
  dropped. The last two carry a one-line justification, here as well as in the report.
- **Statements are copied from the step 2 registry verbatim.** A shortened wording here would be a
  second version of the requirement, and it would drift from the first.
- A withdrawn requirement stays as `R<n>: [WITHDRAWN]` — the number is never given to another.
- In scenarios B and C there is usually no registry; the table is then either absent with a one-line
  note, or filled from a reconstructed registry and marked as reconstructed.

## Tasks
| Prefix | File | summary | Estimate | ready | Blocked by (links) | Analysis status |
|---------|------|---------|--------|-------|---------------------|----------------|
| C1 | C1-add-schema.md | ... | 8 h | true  | none | ready |
| C2 | C2-add-handler.md | ... | —   | false | C1  | needs an estimate |

The "Estimate" column here is a short projection; the rationale and history are in the "Estimates"
section. Analysis statuses: `draft` / `needs an estimate` / `incomplete` / `off-format` /
`needs rework` / `ready`.

## Estimates
The details behind the task estimates: the rationale (via material IDs), the confidence and the
revision history. The value must match the "Estimate" column in the "Tasks" table.

| Task | Estimate | Split (analysis/dev/test) | Confidence | Basis (ref) | History |
|-----------|--------|----------------------------|-------------|-----------------|---------|
| C1 | 8 h | 1/5/2 | high | M1, M2 | 6h→8h (a migration turned up) |
| C2 | —   | —     | —       | M3     | needs an estimate |

Confidence: `high` / `medium` / `low`. The basis is an ID from the "Materials" registry.

## Open questions
| # | Question | Related to | For whom | Status | Answer / decision |
|---|--------|----------|------|--------|-----------------|
| Q1 | What is the file size limit? | C2 | the user | open | — |
| Q2 | Are we changing contract v1? | the whole epic | the backend team | closed | keep v1, add v2 |

## Assumptions (ASSUMPTIONS)
| # | Assumption | Confirmed? |
|---|---------------|---------------|
| A1 | The DB is the project's existing one (per the current schema), no new storage | yes |

## Decision log
- 2026-06-18: split C2 into C2/C3 — it exceeded 24 h.

## Next action
<what to do when resuming: which step to run, what to ask the user>
```
