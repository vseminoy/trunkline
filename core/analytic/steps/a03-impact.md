# Step 3. Impact Analyst agent `[STOP]`

**Skills:** `agent-skills/skills/deprecation-and-migration/SKILL.md`

Assesses the impact of the changes on the profile's `layers`, and on migrations and authorization.

- Determine what changes: the contract (the spec) / the DB schema / the service layer / the handlers.
- Backward compatibility: do existing contracts or consumers break.
- If the task involves deleting or replacing code — the Zombie Code check
  (`grep -r "FunctionName" .`) and the choice of strategy (Parallel-run / Strangler Fig / Adapter /
  Feature Flag / Big Bang).

This exists so the decomposition respects **order**: the contract and the migrations usually come
before the code that depends on them.

**Work through the requirements registry.** The impact analysis proceeds **by the `R<n>` from step
2**, not by the epic as a whole: for each requirement, determine which layers it touches and what
changes in them. The requirements serve as anchor points — `WHEN a request is received …` points at
the handler and the contract, `WHILE a migration is running …` at the migration layer.

The result goes into a table:

| ID | Layers affected | What changes | Breaks compatibility |
|----|--------------------|--------------|----------------------|
| R1 | contract, handler, service | a new operation in the spec | no |
| R2, R4–R7 | service | logic only (validation branches) | no |
| R3 | migration, gateway | a new migration | yes — 503 on writes |

**Group what is identical.** Requirements with identical impact collapse into one row with the IDs
listed (`R2, R4–R7`). A 13-row table of which 11 rows are the same is noise: it gives the Decomposer
no additional information. Expand row by row only where the impact **differs**.

A requirement for which no layer can be named signals that it is either worded too abstractly or does
not concern this system: send it back to the Analyst (step 2) rather than carrying it forward.

**Output:** the affected layers and risks **broken down by `R<n>`**, and the ordering constraints
(what must come before what).
