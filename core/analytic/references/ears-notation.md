# Requirements notation — EARS

Functional requirements and acceptance criteria are written in **EARS** notation (Easy Approach to
Requirements Syntax). This is the single source of truth on the form of requirements for the
Clarifier (step 0) and the Analyst (step 2).

The keywords are written in **CAPITALS** and used only in their defined sense: `WHEN`, `WHILE`,
`IF … THEN`, `WHERE`, `SHALL`.

**Five templates:**

| Type | Template | When to use it |
|---|---|---|
| Ubiquitous | `<System> SHALL <response>.` | always in force, unconditional (invariants) |
| Event-driven | `WHEN <trigger>, <system> SHALL <response>.` | a reaction to an event or request — the main type for endpoints |
| State-driven | `WHILE <state>, <system> SHALL <response>.` | in force over an interval, while a mode holds |
| Unwanted behaviour | `IF <unwanted trigger>, THEN <system> SHALL <response>.` | errors, timeouts, failures, invalid input |
| Optional feature | `WHERE <feature is enabled>, <system> SHALL <response>.` | a feature flag, a configuration option, a plan option |

**Compound requirements** combine these in the strict order `WHERE → WHILE → WHEN/IF → SHALL`, with
**no more than two conditions**. Three or more conditions means it is not a requirement but an
algorithm; split it up.

Example:
```
WHILE the subscriber is in the "blocked" state, WHEN a service activation request arrives,
the service SHALL return 409 and the error code SUBSCRIBER_BLOCKED.
```

**Rules for wording:**
- **A subject is mandatory.** "Validation shall be implemented" is forbidden: it is unclear whose
  responsibility it is. Always name the system / service / layer.
- **The response is checkable.** Not "takes the status into account", but "returns 409 and the code
  `SUBSCRIBER_BLOCKED`". Wordings like "handle correctly", "work properly", "react adequately" are
  forbidden: formally that is EARS, substantively it is an empty requirement.
- **One requirement = one test.** If you cannot write one concrete test from a requirement, it is
  worded wrongly.
- **Tied to the contract.** Where a requirement concerns an endpoint, refer to the operation
  identifier from the contract. Do **not** duplicate the request/response structure (fields, types)
  in EARS — that lives in the spec. EARS describes **behaviour**: under which conditions exactly this
  code is returned, not the shape of the data.
- **Identifiers.** Every requirement gets a task-wide identifier `R1`, `R2`, … — they carry the trace
  through the Impact Analysis, the Decomposition and the "How to test" block of the tasks.
- **An identifier is a working note of this epic, not a contract downstream.** The developer (`core/developer`)
  reuses `R<n>` as is: the same number goes into the contract, the comment on a constraint and the
  test table. Two consequences follow:
  - **A number is never reassigned** — not when the wording is revised, not between sessions, not
    when a requirement moves to a different task. If a requirement is withdrawn —
    `R<n>: [WITHDRAWN]`; the number is not given to another.
  - **The `R<n>` → task trace serves this epic and stops at its edge.** It answers the analyst's
    own question — is every requirement anchored to exactly one task — and it lives in the
    **analysis state** (`<tasks.analysis_dir>/<EPIC>.md`, the "Requirements (EARS) and coverage"
    table), saved on every step by construction. What crosses to development is the requirement
    **text**, in the task's "How to test" block; the developer assigns their own identifiers at their
    step 2 and records the items they came from, so nothing is lost in the handover without a count
    showing it. Do not put `R<n>` into a task body: a number that means one thing inside the epic and
    nothing outside it is worse than no number, because it looks like a contract.

**Parameterised requirements.** If behaviour is identical in form but differs by a set of values (by
`purpose`, by channel, by document type) — that is **one requirement plus a parameter table**, not
one requirement per value. The value table goes into the "Outside EARS" section as data.

Right:
```
R9. WHEN a request with purpose=<P> is received, the service SHALL apply the message template, the
    code length, the TTL, the retry interval and the max attempts from the configuration of purpose
    <P>.
    (the values are in the "Purpose parameters" table in the "Outside EARS" section)
```
Wrong: seven separate requirements `R9…R15`, one per `purpose`. That bloats the registry, breaks the
estimate and provokes unnecessary splitting of tasks.

The criterion: if the requirements differ **only in the values in the same places** — collapse them
into one. If some value acquires **its own behavioural branch** (a different set of steps, a
different response code) — that is a separate requirement.

**What is NOT expressed in EARS** (written separately, in prose or tables — trying to force EARS onto
it is a mistake):
- non-functional requirements (RPS, latency, limits);
- data structures and field formats — that is the contract and the SQL schema;
- the business goal and motivation ("why") — that is the "Problem" block of the task;
- complex calculation algorithms — decision tables or pseudocode.

Realistic coverage is 60–70% of the functional part of the statement. The rest in prose; do not force
it.

**Ubiquitous requirements from the guidelines.** The rules in the profile guidelines (authorization,
migrations, data types) are in force for every task by default. Do **not** rewrite them into the
requirements of a particular task — only refer to them if the task explicitly breaks them or needs an
exception.
