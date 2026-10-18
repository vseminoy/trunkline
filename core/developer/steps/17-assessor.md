# Step 17. Assessor agent `[STOP]`

**Skills:** `agent-skills/skills/shipping-and-launch/SKILL.md`
**Reference:** `agent-skills/references/definition-of-done.md`

Runs **once per task, after the last changeset has been committed** — not before each commit (see
"The changeset loop" in the core). This is a checklist over the finished delivery; while changesets
are still outstanding, half of it has no answer yet.

Apply the full 6-category pre-launch checklist, the monitoring checklist and the post-launch
verification from the skill.

**Pre-launch:**
- the profile's `commands.build` — OK
- the profile's `commands.test_unit` — OK
- the profile's `commands.lint` — OK
- Conformance to the profile guidelines (all `guidelines.roles`) — no violations
- If steps 11, 12 or 16 changed code, steps 9+10 ran a **second** time over that diff and no Critical
  is left open (see "A review describes the code that existed when it ran" in the core). If they
  changed nothing, that is the answer — record it rather than leaving the item blank
- Step 7a passed: every `.pipeline/workdir/consistency/<NNN>-c*.md` of this run has no gaps or discrepancies, and the `trunkline trace-check` output
  pasted into it says `PASSED` (or records `unavailable` with the hand grep in its place)
- Secret scanning — governed by the project's `secrets_in_code` flag (skipped when `true`)
- Every approval from the "MANDATORY APPROVALS" section has been given
- Migrations apply without downtime
- Environment variables are documented

**`[CONDITIONAL]` Staged rollout — for critical paths (auth, payments, the main API):**
```
internal team → 5% → 25% → 50% → 100%
Rollback: error rate > 1% baseline OR p99 latency +20% → immediate rollback
```

If even one item fails → send it back for rework.
