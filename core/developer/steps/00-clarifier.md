# Step 0. Clarifier agent `[CONDITIONAL]`

**Skills:** `agent-skills/skills/interview-me/SKILL.md`, `agent-skills/skills/idea-refine/SKILL.md`

Runs when the task is vague: no concrete behaviour, no acceptance criteria, no pointer to files or
endpoints.

Apply the full process from the skills. Additionally clarify:
- affected layers — against the profile's `layers` list;
- stack constraints — against the profile's `stack` (what is generated, what is written by hand).

**Delta context:** if `.pipeline/knowledge/` already holds entries for the modules mentioned, read
them BEFORE the interview and do not ask what is already recorded there. Ask only about the delta.

**Output:** the task restated with acceptance criteria and an agreed approach.
