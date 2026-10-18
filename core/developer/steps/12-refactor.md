# Step 12. Refactoring agent `[STOP]`

**Skills:** `agent-skills/skills/performance-optimization/SKILL.md`
**Reference:** `agent-skills/references/performance-checklist.md`

Apply the MEASURE → IDENTIFY → FIX → VERIFY → GUARD cycle from the skill.

Checks (the backend section of performance-checklist.md):
- Minimal diff: no incidental changes
- N+1 queries, unbounded loops, missing pagination
- Redundant allocations, hot paths, data copying
- Race conditions, shared state, locking
- Behaviour at boundary volumes (large files, long lists, high concurrency)

Load context comes from the project's `rps` flag (see "Pipeline flags" in the core prompt; if the
flag is unset — ask the user, do not invent it). Optimise only where it is needed, and never at the
cost of readability.

**Output:** the diff is minimal, the performance checklist passes.
