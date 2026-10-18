# `slices/YYMMDD-<slug>.md` — template

An immutable entry. Never edit it after the fact — a new slice writes a new entry.

```markdown
# Slice: <title>

Run: 260813142530 | Task: PROJ-855
Date: 2026-08-13 | Type: feature | Commits: a1b2c3d, e4f5g6h
Modules: src/payments

## Business rules
| ID | Statement | Where it is enforced | Source |
|---|---|---|---|
| R-007-1 | the payment amount is > 0 | service.<ext>:42, a check in the schema | analysis, epic PROJ-855 |
| R-007-3 | the currency is from the supported list | service.<ext>:51 | development |

## ADDED
- `POST /payments/create` (`createPayment`) — handler + service + the `CreatePayment` DB query
- the `payments` table, migration `20260805_create_payments.sql`

## MODIFIED
- `plan.Service.Get` — signature: `ctx context.Context` added as the first argument

## REMOVED
- `legacy.ChargeV1` and its tests (strategy: Big Bang, no consumers left)

## Decisions
- <what was decided and why>

## Rejected alternatives
- <what was considered and why it was turned down> ← answers Chesterton's Fence for future step 11s

## ADR
ADR-004 (if there was one) — `adr/ADR-004-<slug>.md`
```
