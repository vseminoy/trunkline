# `modules/<module>.md` — template and field rules

Read this in full immediately before creating or updating a module file — not from memory, and
not from a copy kept anywhere else.

Target size ≤ 150 lines. Always the current state; anything stale is deleted.

```markdown
# Module: <path, e.g. src/payments>

Updated: 2026-08-13 | Slice: 260813-payments-create

## Purpose
<2–3 lines: what the module is responsible for, in business terms>

## Entry points
| Endpoint / method | operationId | File | Note |
|---|---|---|---|

## Layers and key types
- handler: `api/handler.<ext>` — <types, what it does>
- service: `service.<ext>` — `Service` (stateless), methods: ...
- repository: <the DB access mechanism per the profile>, queries in `<path>`

## Data
| Table | Key columns | Constraints expressing rules |
|---|---|---|

## Consumers and links
Who depends on this module and through what (import, HTTP, queue). The source for assessing the blast
radius at step 3.

## Invariants and pitfalls
The non-obvious things that are easy to break: call order, nil assumptions, timing, idempotency.
One line each, in the form **`<what must not change>` — `<what breaks if it does>`**. The second half
is what the code cannot say and the first half cannot defend without: "the pages are read by
ascending id (keyset)" invites a later run to simplify it away; "— without it `process` loops for
`maxNotificationToRead / chunk` iterations" does not. A pitfall with no consequence attached is a
note; with one, it is a guard.

## Business rules in force
| ID | Statement | Where it is enforced |
|---|---|---|

`ID` is `R-<run>-<n>`, assigned by step 2 of the development pipeline — the run number, then a
counter. **There is no `Slice` column**: the run number inside the identifier already names the slice
that registered the rule, and `INDEX.md` maps a run to its entry. A column whose every cell repeats
the same derivable value is the cheapest kind of bloat to acquire and the hardest to notice — the
table looks fuller, not longer.

The numbers are not reassigned when the file is updated, and a rule already listed here is
**reused under its existing identifier** rather than registered again by a later run.

**A statement names a subject, a condition and an observable response.** Not the full EARS sentence
the requirement was written in — that lives in `.pipeline/workdir/rules/<NNN>.md`, section "Requirements from
the task text", and the run number inside the identifier is the way back to it. Here it is one line, and copying the
EARS text into it would blow the 150-line budget on a module with thirty rules. But the three parts
survive the compression, because they are what makes the rule checkable: "the service rejects an
amount of 0 or less with 400 `AMOUNT_INVALID`" is a statement; "amount validation" is a topic, and a
topic cannot be contradicted by code, which is what step 21 needs it to be able to do.

**Rules that differ only in a value collapse into one row plus a table.** Seven rows for seven
`purpose` values is the same requirement seven times: keep one row, parameterise it, and put the
values in a table under "Invariants and pitfalls". A value that acquires its own behavioural branch —
a different response code, a different sequence — has stopped being a value and is its own rule. The
criterion is the one the analyst's EARS reference uses for parameterised requirements, and it is
what keeps a rule table compact without making it vague.

**A path in backticks is a path in this repository.** That is what lets
`trunkline kb-sync` report a path that no longer resolves as a stale entry rather
than as noise. A file belonging to a **dependency** is named in plain text instead, with its module
and version — `boilerplate-go.git@v1.0.0 src/.../http_client.go`, not `` `http_client.go` `` — because
nobody can open it in this tree, and dressing it as a local path is what makes a real deletion
indistinguishable from a reference that was never local to begin with. The same holds for a file in
another repository of a monorepo pair, or a path inside a container image.

**`Where it is enforced` is an anchor, `file:symbol`.** A function, a method, a constraint name, a
migration — something that can be looked up. **Never `file:line`**: line numbers do not survive a
reformat, so a line-anchored entry reports drift on its first unrelated commit and stops being read
within a week. Prose ("a check in the schema") is allowed where there is genuinely no symbol, and it
is the one form `trunkline kb-sync` cannot resolve — so it is a cost, paid
deliberately, not a default.

A rule that a later run **supersedes** leaves this table — it is no longer in force — and the
pairing goes into that run's entry as `superseded: <old ID> by <new ID>`. Without it a reader who
meets the old identifier in an earlier entry cannot tell whether the rule still holds.

## Known tech debt
Medium-Low findings from steps 9/10 that were not taken on, with the slice noted.
```
