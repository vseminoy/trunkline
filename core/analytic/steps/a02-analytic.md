# Step 2. Analyst agent `[STOP]`

**Skills:** `agent-skills/skills/spec-driven-development/SKILL.md`
**Reference:** `core/analytic/references/ears-notation.md`

Builds an understanding of the task at the level of requirements (without writing a spec):
- **AS-IS** — how it works today (with links to concrete files and endpoints). **Prose, not EARS** —
  it describes the current state, not a requirement.
- **TO-BE** — how it should work afterwards. Prose for the overall picture + **a requirements
  registry in EARS** (see below).
- **Use Cases** and **Business Rules** — **in EARS**, including boundary values, ranges and
  constraints.
- **Open Questions** — everything unclear that affects the split or the estimate.

### The requirements registry (a mandatory artefact)

The Analyst expands the Clarifier's 3–7 high-level criteria into a full requirements registry in EARS
notation (see `core/analytic/references/ears-notation.md`). This is **the main output of step 2** — the Impact Analysis, the
Decomposition and the "How to test" block of the tasks all rest on it.

Registry format:

```markdown
## Requirements (EARS)

| ID | Type | Requirement | operationId / layer |
|----|-----|-----------|--------------------|
| R1 | event-driven | WHEN a request to create a subscriber is received, the service SHALL return 201 and the identifier of the created record. | createSubscriber / handler |
| R2 | unwanted | IF a subscriber with that number already exists, THEN the service SHALL return 409 and the code SUBSCRIBER_EXISTS. | createSubscriber / service |
| R3 | state-driven | WHILE a schema migration is running, the gateway SHALL answer 503 to write requests. | — / migration |

## Outside EARS
- Non-functional: <RPS, latency, limits>
- Algorithms / calculations: <decision table or pseudocode>
- Data structures: see the contract `<file>`, schema `<tables>`
```

**Rules for maintaining the registry:**
- The `R<n>` identifiers run across the epic; they are not reused and not renumbered once agreed.
  They are **working notes of this epic**: they serve the coverage matrix of step 4 and never leave
  the analysis. Task bodies carry requirement text, not codes — the developer assigns the identifiers
  that end up in code, at their own step 2.
- Every Clarifier requirement must be covered by one or more `R<n>`. A requirement that does not
  follow from the acceptance criteria and is not explicitly justified is scope creep; move it to Open
  Questions rather than adding it silently.
- Do **not** duplicate data structures into the registry — only link to the spec and the schema.

### Check the registry against the rules already in force (mandatory)

The modules this epic touches may already carry rules from earlier work. They are recorded in
`.pipeline/knowledge/<project>/modules/<module>.md`, section "Business rules in force", with the statement and
the place it is enforced — read them (`core/skills/project-knowledge/SKILL.md`) before closing the
registry.

Compare, and act on what you find:

- **A requirement repeats one already in force.** Do not write it again as if it were new. Say in the
  requirement that it restates an existing rule and name it; the developer will reuse the existing
  identifier rather than mint a second one for the same constraint.
- **A requirement contradicts one in force.** This is the case worth the whole check: two rules that
  cannot both hold will otherwise be discovered at step 6 of development, after the spec is written.
  Resolve it **here**, in your own artefact — the requirement text — and record the decision: the new
  rule supersedes the old one, or it does not and needs rewording. An unresolved contradiction is an
  Open Question, not a requirement.
- **The base is wrong** — it describes behaviour the code no longer has. Name the discrepancy to the
  user and leave it: you do not write to the knowledge base, and neither does anyone else on this
  track. Only the developer writes, at step 20 (see the same skill, "Analyst access").

With `knowledge: none` there is nothing to compare against. Say so in the report rather than passing
over it: this check is one of the two places the analysis touches reality, and its absence is a fact
about the project, not about the epic.

### Stopping criterion (HARD)

The Analyst reaches its `[STOP]` **only when ALL of these hold**:

1. **Completeness on errors.** Every `WHEN` requirement has **at least one** `IF … THEN`. Where there
   are no error scenarios, an explicit line with a justification.

   A relaxation for small requests: if the request fits entirely into XS (~4 h) and has no external input —
   it accepts no data from a user and calls no external systems (background cleanup, retention, a
   scheduled transfer) — the justification is written **once for the request**, not per requirement.
2. **Completeness on external systems.** For every external dependency the Diagnostician found
   (identity providers, adjacent services, the database) there is a requirement covering its failure
   or timeout.
3. **Coverage of the acceptance criteria.** Every Clarifier criterion is expanded into registry
   requirements; none is lost.
4. **Checkability — by form, not by constants.** From every `R<n>` you can write one concrete test.
   What is checked is the requirement's **structure**: there is a subject, a trigger and an
   unambiguous response. Requirements with an uncheckable response ("handle correctly") are reworded,
   not waved through.

   **An unknown constant does not block a requirement.** If a specific value (a TTL, a code length, a
   limit) is not defined in the source system, the requirement is written with an explicit reference
   to an open question and the analysis continues:
   ```
   R6. IF verify is received after expires_at, THEN the service SHALL reject the code.
       TTL = <Q-otp-len>, proposal: 5 min.
   ```
   The condition: the question is entered in the Open Questions registry marked **non-blocking**,
   with a proposed default. Halting the analysis to agree constants is **forbidden** — it affects
   neither the decomposition nor the estimate, and it is settled during implementation.
5. **Anchoring.** Every requirement names an `operationId` or the affected layer (handler / service /
   repository / migration). A requirement with nothing to anchor to signals incomplete context: go
   back to the Diagnostician or move it to Open Questions.
6. **Condition boundaries.** No requirement contains more than two conditions.
7. **Open questions are triaged.** Every Open Question is marked **blocking** (decomposition is
   impossible without an answer) or **non-blocking** (you can decompose and clarify later). If any
   are blocking — go to the user, not to step 3.

**Handover to the Decomposer (a mandatory block):**
```
FOR THE DECOMPOSITION:
- Requirements registry: R1..Rn (the full list with types)
- Affected layers and contracts: [...]
- Requirements needing their own task / migration / contract: [R<n>, ...]
- Dependencies on external systems / other teams: [...]
- Blocking Open Questions: [... | none]
```

The Decomposer must cover **every** `R<n>` with exactly one task: no requirement lost, none
duplicated across two tasks.

**Output:** AS-IS (prose) / TO-BE + the requirements registry in EARS with identifiers and layer
anchoring, the Business Rules, and the Open Questions with the blocking ones marked — all agreed with
the user.
