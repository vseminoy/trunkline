# Step 7. Tester agent `[STOP — the description BEFORE the code]`

**Skills:** `agent-skills/skills/test-driven-development/SKILL.md`
**Persona:** `agent-skills/agents/test-engineer.md`
**References:** `agent-skills/references/testing-patterns.md`,
`core/developer/references/tests-readme-example.md` (the description-format template)

Apply the full TDD process from the skill (RED → GREEN → REFACTOR) and the Test Pyramid
(80% unit / 15% integration / 5% e2e).

**Profile rules (mandatory):** the role `api_spec` — the error contract and which codes carry a body.

**Step 1 — the test description (BLOCKING):**
Produce a `tests_readme.md` document next to the module. **The format follows
`core/developer/references/tests-readme-example.md` exactly** (read it before drafting). The
elements below are mandatory — the step checks that each is present:

- At the top — a **table of contents** linking to the sections.
- **Tests are split by layer; each layer gets its own Markdown heading (`##`) and its OWN table.**
  The layers and their test files come from the profile's `layers`, bottom up. Headings take the
  form `## <file path> — <Method>`.
- **The layer is the ONLY axis of the document.** There is no other one — not the changeset, not the
  run, not the date. A heading, a section, a table, a column or a note named after a changeset
  ("changeset 2", "added in the second changeset", "the description is extended per changeset") is a
  defect: remove it and fold its rows into the layer table they belong to. A reader must be able to
  open the file and see, per layer, every test that exists for that module — never a chronology of
  how it was written.
- **Under each heading — a `>` block quote naming WHAT is mocked** (replaced by stubs): for example,
  `> Replaced by stubs: the payment gateway (fakeGateway), the repository (fakeRepository).`
  If a layer has no mocks (pure unit functions) — say so explicitly.
- A three-column table: `Test` (the exact test name) | `Scenario` | `Expected result`.
  (Where needed, use `###` sub-headings inside a section for helper functions.)
- **No rule identifiers anywhere in the file.** `tests_readme.md` is committed and outlives the run;
  the registry is deleted with it (see "Rule traceability" in the core), so a `Rule` column or a
  "Business rule coverage" section would leave every reader after this run holding a number with
  nothing behind it. What the scenario is has to be readable from the scenario.

**Rule coverage goes into the run's working directory,** not into the document: write
`.pipeline/workdir/coverage/<NNN>-c<K>-tests.md`, an `ID | tests` table listing every identifier of
the changeset and the test names that cover it.

```
| ID | Tests |
|----|-------|
| R-007-2 | CreatePayment_ZeroAmount, CreatePayment_NegativeAmount |
| R-007-4 | CreatePayment_DuplicateExternalID, HandlerCreatePayment_Duplicate |
```

A rule with no test at all blocks the step. Step 7a cross-checks this record.
**Only where step 2 is in the route** — see "Rule traceability" in the core. On `bug` it is not, so
there is no registry to cover: write the file with one line saying step 2 is not on this route. The
Prove-It test carries the changeset there, not an identifier.

**The test description is written TO A FILE, not to the console:**
- The description goes into `tests_readme.md` next to the module.
- Do NOT print the description to the console for approval. The reply states the file path and
  briefly lists what is in it.
- Ask the user to approve the test description (the user reads it in the file).
- If the user asks for changes — edit THE SAME `tests_readme.md` and ask for approval again. Repeat
  until the user confirms with an explicit "yes".

**Record the concrete error in `Expected result` — that is what makes its path traceable across the
layers:**
- the **data access** layer: what the query returns (no rows, a connection error, a constraint
  violation);
- the **service** layer: what it turns into (a domain error with specific text / an internal error);
- the **HTTP** layer: the resulting code and body — per the error contract from the profile's
  `api_spec` role.

Write it into `tests_readme.md` → state the path and wait for approval (the user reads the file) →
make edits in that same file → only after an explicit "yes" write code.

A worked example covering every layer is in `core/developer/references/tests-readme-example.md`
(which also has a "What to watch for when copying" section). It is not duplicated here. If the
profile overlay for this step carries its own instance of the format, read it after the template:
it fixes the naming conventions and the concrete error values of the stack, not the format.

**A second changeset edits the same document in place.** Where a route has several changesets, this
step runs once per changeset, but it does not produce a second document and does not append a new
section. Read the existing `tests_readme.md`, add each new test as a row in the table of ITS layer
(creating the `##` heading only when that layer is genuinely absent), extend the mock block if the
stubs changed, and write the changeset's own coverage record. The rows earlier
changesets wrote stay where they are. What changed in this changeset is stated in the reply to the
user and in the journal — not inside the document.

**Only the layers the changeset actually touches are described.** The layer blocks below are
requirements for the layers that are in the changeset, not a demand that every changeset reach every
layer.
A layer the changeset does not touch is named "not applicable" in one line rather than filled with
invented tests — on the `migration` route, where this step runs after step 6, the description is
usually confined to the data-access layer and the constraints the migration introduced.

**HTTP layer:**
- Every response code in the contract is covered.
- Which codes carry a body, and the error format — per the `api_spec` role; the test checks
  conformance to the contract, not a guess.
- The layer's dependencies are replaced by stubs; no real database is started.

**Service layer:**
- Every public method: success + every business error.
- A storage failure → an internal error, not a user-facing one.

Language details (file names, how dependencies are substituted, helpers) belong in the profile
overlay.

**`[CONDITIONAL]` Prove-It Pattern (for a bug fix):**
1. Write a test that reproduces the bug — it must fail against the current code
2. Run it and show the failure to the user → "yes"
3. Only then move on to the Generator agent


**Once the user's "yes" is in, write the approval to the journal** — one `approval` line naming this
step **and the changeset it belongs to** (`core/developer/references/pipeline-evals.md`). It is not
telemetry: the commit gate reads it back and blocks production code while an approval the route
requires is unrecorded (`core/developer/ENFORCEMENT.md`). The changeset is not optional there — this
step runs once per changeset, and without it the gate credits changeset 1's approval to every commit
of the task. Write it when the approval is given, and only if it was given.

**Output:** the test description is agreed and recorded in the journal, the coverage record shows
every rule covered, and the tests are written exactly to the description.
