# AI Analyst working rules

What is here is what `core/analytic/PIPELINE.md` does **not** say: the hard invariants of
decomposition and estimation, and the one rule about task files that belongs to the scenarios rather
than to the core. Everything else that used to be restated here — the working mode, the four
principles, the `[STOP]` discipline, the output format — lives in the core, is read from there, and
is not copied. A copy is a second thing to keep in sync, and this file had already drifted from the
core on the mode and on the output format before the copies were removed.

On a divergence with the core, an explicit instruction from the user takes priority.

## Hard invariants of decomposition and estimation

- No task exceeds **24 working hours** for a mid-level developer. If it does, it goes back to the
  Decomposer and is split.
- Foundation tasks (the contract, the DB schema, the migration) come first; the order is a
  topological sort of the dependency graph, reflected in the file prefixes.
- An estimate covers the **whole cycle** of a task (spec, migrations, code, tests, review, CI) and
  is written only into the summary report, not into the task files (the format spec has no field
  for it).

## Task files are changed only with consent

Scenario A writes new task files. Scenarios B and C **change files someone else may have written**,
and that happens only with the user's explicit consent, per file. The read-only rule of the core
covers the project's code; this covers the tasks, which are the one thing this track does write.

## Where the rest is

| What | Where |
|---|---|
| The working mode, the four principles, the `[STOP]` discipline | `core/analytic/PIPELINE.md`, "WORKING MODE (STRICT)" |
| The output format and the `AGENT [SKILLS]:` header | `core/analytic/PIPELINE.md`, "OUTPUT FORMAT" |
| The state: what it holds, where it lives, when it is written | `core/analytic/skills/analysis-state/SKILL.md` |
| The estimation table, the decay rates, the multipliers | `core/analytic/steps/a05-estimator.md` |
| The task file format | the profile's `tasks.format_spec` |
