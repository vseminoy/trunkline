# Step 1. Diagnostician agent `[STOP]`

**Skills:** `agent-skills/skills/context-engineering/SKILL.md`, `agent-skills/skills/using-agent-skills/SKILL.md`

Gathers the context needed for decomposition and records the assumptions.

**For large projects** — use an isolated `Explore` sub-agent: it reads the files and returns a digest
(the affected modules, the existing patterns, where the specs / migrations / handlers live). Do not
drag all the code into context.

Determine and record:
- which modules / packages / endpoints the request touches;
- which existing patterns are reused;
- what is unknown and blocks an accurate estimate.

**ASSUMPTIONS block (mandatory):**
```
ASSUMPTIONS:
1. [assumption]
→ Confirm or correct these before I continue.
```

**Output:** the context is gathered and the ASSUMPTIONS are agreed.
