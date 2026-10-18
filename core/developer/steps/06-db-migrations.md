# Step 6. DB migrations agent `[STOP — on schema, queries, migration]`

If there are no database changes → "not applicable", move on.

**Profile rules (mandatory):** the roles `migrations` (migration format), `data_types` (column
types), `db_schema_style` (schema style, naming, grants, auto-increment, dates, indexes,
documentation in comments).

Produces artefacts BEFORE any code is written. Order: schema → database queries → migration.

**Read the rules from the guideline files, not from memory.** The migration and data access tools
come from the profile's `stack.migrations` and `stack.db_access`.

**Traceability:** a database constraint that expresses a business rule (`check`, `not null`,
`unique`, a foreign key, a field length) is named and commented for what it enforces — the SQL
comment states the rule, not its identifier. The schema is committed and outlives the run, and the
registry is deleted with it (see "Rule traceability" in the core): `-- R-007-2` in a migration points
a later reader at a file that is gone.

The identifier goes into the coverage record,
`.pipeline/workdir/coverage/<NNN>-c<K>-schema.md`, which step 7a reads — one row per rule the
schema enforces, naming the constraint:

```
| ID | Constraint |
|----|------------|
| R-007-2 | payments.chk_amount_positive |
| R-007-4 | payments.uq_external_id |
```

Not every rule reaches the schema, and that is the design rather than a shortfall: a currency enum or
a header format has nothing to constrain here. So step 7a treats the schema as a **partial** column
(`--partial`) — a rule that never needed a database constraint is not missing from it, while a row
naming an identifier the registry does not have is an orphan.

Each artefact is agreed separately with an explicit "yes".

**Once the user's "yes" is in, write the approval to the journal** — one `approval` line naming this
step **and the changeset it belongs to** (`core/developer/references/pipeline-evals.md`). It is not
telemetry: the commit gate reads it back and blocks production code while an approval the route
requires is unrecorded (`core/developer/ENFORCEMENT.md`). The changeset is not optional there — this
step runs once per changeset, and without it the gate credits changeset 1's approval to every commit
of the task. Write it when the approval is given, and only if it was given.

**Output:** SQL schema, database queries, migration and the schema coverage record — all agreed, and
the approval is recorded in the journal.
