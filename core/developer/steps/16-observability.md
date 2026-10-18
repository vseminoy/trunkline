# Step 16. Observability agent `[CONDITIONAL]`

**Skills:** `agent-skills/skills/observability-and-instrumentation/SKILL.md`
**Reference:** `agent-skills/references/observability-checklist.md`

Runs for **feature** tasks that touch hot paths (the project's `rps` flag): new endpoints, calls to
the database or external services, background processing. For bug/refactor/migration/config it is
usually "not applicable" (unless the fix exposed a blind spot in the telemetry).

The goal is telemetry written **together with the code**, not after an incident. If a feature ships
without telemetry, the first user-reported bug turns into an excavation instead of a query.

**Step 1 — define what "working" means (BLOCKING):**
State the questions the telemetry must answer BEFORE instrumenting anything. If you cannot name the
questions, you are not ready to instrument (you will log everything and learn nothing).
```
OBSERVABILITY QUESTIONS:
- [question, e.g. "how often does a payment fail because of the gateway vs because of the data?"]
- [question, e.g. "p99 latency of query X?"]
```

**Step 2 — pick a signal per question (from the skill):**
- **Metric** — "how often / how fast in aggregate?" → RED (Rate, Errors, Duration) per handler and
  per external call.
- **Trace** — "where did the time go across handler → service → DB / external call?" (sampled).
- **Structured log** — "what exactly happened in this case?" (a stable event name plus fields).
- The rule: a metric tells you **what** broke, a trace **where**, a log **why**.

**General instrumentation rules:**
- Logs are structured (the project logger): a stable event name (`payment_failed`) plus
  machine-readable fields, not interpolated prose. Do not log PII or secrets in runtime logs (the
  `secrets_in_code` flag covers sources only; it does not extend to logs).
- Thread the request context through every layer in the profile's `layers` — for trace spans and
  cancellation.
- Metrics at the boundaries: inbound HTTP, outbound calls to the database and to external services.
- Do not instrument everything — only what answers the questions from Step 1 (metric cardinality is
  finite).

**Step 3 — alert on symptoms, not on causes:**
Alert on what the user feels (error rate, p99 latency), not on every internal metric. Tie the
threshold to a baseline (cf. the staged rollout in step 17: error rate > 1% baseline OR p99 +20%).

Fine-grained optimisation of measured slowness does NOT belong here — it belongs to step 12
(`performance-optimization`). This step provides visibility; it does not fix performance.

**Output:** the observability questions are named, each has a signal (metric/trace/log), the
telemetry is added to the current changeset's code, and symptom-based alerts are proposed. Tests pass
after the changes (the profile's `commands.build` and `commands.test_unit`).
