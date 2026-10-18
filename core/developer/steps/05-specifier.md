# Step 5. Specifier agent `[STOP — on every YAML and every diagram]`

**Skills:** `agent-skills/skills/spec-driven-development/SKILL.md`, `agent-skills/skills/api-and-interface-design/SKILL.md`

**Profile rules (mandatory):** the roles `api_spec` (in full), `data_types` (types in the contract),
`auth` (authorization in the contract). If the `stack.contract` slot is empty, the step reduces to
sequence diagrams and no spec is produced.

Apply the principles from api-and-interface-design: hard to misuse, Hyrum's Law, backward
compatibility, consistency with the existing endpoints.

**The contract** (permitted methods, error codes and bodies, operation identifiers, authorization,
types) follows the profile guidelines strictly. It is not restated here — read the rules and
checklists in the files themselves.

**The agent's steps (specific to this stage):**
1. Find the contract file directory. Check for, or create, the shared file of reusable error
   schemas.
2. Every contract file references the shared one instead of duplicating error descriptions.
3. Write endpoint paths out in full.
4. Sequence diagrams (Mermaid): client → the profile's `layers` top to bottom, every error branch.

## Rule traceability in the spec (mandatory)

Every rule from the BUSINESS RULES block of step 2 must be visible in the spec **as behaviour**:
- the constraint is expressed machine-readably wherever possible (`enum`, `minimum`, `maxLength`,
  `pattern`), not in prose alone;
- where prose is needed, the `description` states the rule itself — the limit, the range, the
  permitted values;
- a rule violation = an error branch in the sequence diagram.

**The identifier does not go into the spec or the diagram.** Both are committed and outlive the run;
the registry is deleted with it (see "Rule traceability" in the core). A `description` reading
`R-007-2: the amount is greater than zero` points a future reader at a file that no longer exists —
write `the amount is greater than zero`.

Where the identifier goes is the coverage record, written to
`.pipeline/workdir/coverage/<NNN>-c<K>-contract.md` and
`.pipeline/workdir/coverage/<NNN>-c<K>-diagram.md`, which step 7a reads:

```
| ID | Where in the spec |
|----|-------------------|
| R-007-1 | createPlan.request.amount (minimum: 1) |
| R-007-3 | createPlan.request.currency (enum) |
```

```
| ID | Error branch in the diagram |
|----|-----------------------------|
| R-007-1 | 400 invalid amount |
| R-007-3 | 400 invalid currency |
```

The cell names the exact place, so the row gets a reader from the rule to the artefact in one hop
without the artefact carrying the number. An identifier with no row in the contract record blocks
the step — do not move on. The diagram record is partial: only a **violable** rule has a branch.

**Diagrams are written TO A FILE, not to the console (BLOCKING order):**
- Sequence diagrams go into `sequence_diagrams.md` next to the module — that name and no other;
  contract files go into the specifications directory. The name is fixed because later steps go to
  it by name: 7a's checks 3-6 read the branches, step 15 links to it instead of redrawing it, and
  the diagram coverage record cites branches in it. A module that calls its diagrams something else
  drops out of all three without anything failing. Where a module already carries another name,
  rename the file rather than teaching every later step about the exception.
- Do NOT print diagram contents to the console for approval. The reply states the file paths and
  briefly lists what is in them.
- Ask the user to approve the diagrams and specifications (the user reads them in the file).
- If the user asks for changes — edit THE SAME FILE and ask for approval again. Repeat until the
  user confirms with an explicit "yes".


**Once the user's "yes" is in, write the approval to the journal** — one `approval` line naming this
step **and the changeset it belongs to** (`core/developer/references/pipeline-evals.md`). It is not
telemetry: the commit gate reads it back and blocks production code while an approval the route
requires is unrecorded (`core/developer/ENFORCEMENT.md`). The changeset is not optional there — this
step runs once per changeset, and without it the gate credits changeset 1's approval to every commit
of the task. Write it when the approval is given, and only if it was given.

**Output:** the shared error-schema file, all contract files and all sequence diagrams are written
to files, the coverage records are complete, everything is explicitly approved by the user, and the
approval is recorded in the journal.
