# Step 8. Generator agent `[STOP]`

**Skills:** `agent-skills/skills/incremental-implementation/SKILL.md`, `agent-skills/skills/source-driven-development/SKILL.md`, `agent-skills/skills/debugging-and-error-recovery/SKILL.md`

**Persona:** `core/developer/personas/senior-developer.md`

**Profile rules (mandatory, read from the files):** the roles `data_types` (types and data access
configuration), `auth` (authorization middleware and request validation), `api_spec` (generating
handlers from the contract).

Writes code ONLY after every approval **the current route requires** — see "MANDATORY APPROVALS" in
the core, which lists them and says which routes have which. Where step 7a ran, it must have no gaps
and no mismatches. Where a route requires no approvals at all (`refactor`), that is a fact to state
before writing, not a precondition to invent: check the route, name what applied, and proceed.
Apply the full processes from the skills.

**The incremental cycle (from incremental-implementation):**
```
Implement (~100 lines) → Test → Verify (profile: commands.build) → Checkpoint → Next
```

**A checkpoint is `git add`, not a commit.** The skill's own cycle ends in a commit; here it ends in
staging what the increment touched, and an increment that has to be undone is undone against that
staged state rather than reverted. Committing belongs to step 18: the commit gate
(`core/developer/ENFORCEMENT.md`) sits on the commit precisely so the intermediate, not-yet-building
states of this cycle do not trip it. Commit here and it fires on every increment.

**Source-driven (from source-driven-development):**
DETECT → FETCH → IMPLEMENT → CITE. For non-obvious patterns in the tools from the profile's `stack`,
check the documentation (the sdd-cache hook caches the domains listed in `hooks.doc_cache_targets`;
"sdd" there is *source*-driven, not spec-driven). If no documentation is found → mark it UNVERIFIED.
CITE is not optional here either: a non-obvious framework pattern carries `// Source: <full URL>` at
the place it is used.

**When the documentation contradicts the approved contract.** The skill's own answer is to surface
the conflict and let the user pick a pattern. That answer does not transfer to this step unchanged,
because the contract of step 5 is already approved and the approval is in the journal. Split the
conflict in two:
- **Mechanics of the API** (signature, hook, deprecated call, the recommended way to wire a
  middleware) — the documentation wins, silently. Nothing to ask: the contract does not say which
  function you call to honour it. Cite the source and move on.
- **Observable behaviour** (a status code, a field, an error branch, a name in the contract) — you
  do not decide it and you do not renegotiate it in passing. Stop, state the discrepancy with the
  citation, and send it back to step 5: the contract is changed and re-approved there, 7a re-runs on
  it, and only then does code get written. A contract quietly bent to match a doc page is exactly
  what 7a and the commit gate exist to catch.

**On a build error (from debugging-and-error-recovery):**
STOP → SAVE the output → FIND the root cause → FIX only that → VERIFY.

**Rules for writing the code:**
- Scope discipline: only the files of the current changeset
- Never mix refactoring with new functionality
- Private functions — a comment when the logic is non-trivial
- Comments describe the current state only — never what the code used to be, what was replaced or
  why it changed (see "Documentation describes the present" in the core)

**Output:** every increment was tested, the profile's `commands.build` passes, every non-obvious
framework pattern is either cited or marked UNVERIFIED, and no discrepancy with the contract was resolved here instead of at step 5.
