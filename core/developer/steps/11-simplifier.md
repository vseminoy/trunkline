# Step 11. Simplifier agent `[STOP]`

**Skills:** `agent-skills/skills/code-simplification/SKILL.md`

Apply the five principles and the four-step simplification process from the skill (Rule of 500).

**Before every change — Chesterton's Fence:**
Understand why the code was written the way it is (`git blame`, tests, comments). If the reason is
unclear → do not change it, ask the user. Entries in `.pipeline/knowledge/<project>/slices/` are the first
place to look for the "why": they record the decisions of past slices.

Guard performance-critical blocks with `/* simplify-ignore-start: reason */`...`/* simplify-ignore-end */`
(the simplify-ignore hook).

Run the tests after every change. If they fail → undo **that change**, against the state step 8
staged. Not a git command: nothing is committed inside a changeset, so `git checkout -- .` would
throw away step 8's work along with yours (see "The changeset loop" in the core).

**Output:** idiomatic code for the stack, complex constructs simplified, behaviour unchanged.
