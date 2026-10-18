# Step 3. Impact analyst agent `[STOP]`

**Skills:** `agent-skills/skills/deprecation-and-migration/SKILL.md`

**Profile rules (load ALL `guidelines.roles` — this is where they start being applied).**
When assessing the impact on the contract, data access, migrations and authorization, check the
proposed changes against those rules and call out violations explicitly in the impact report.

Determines the impact of the changes on every layer in the profile's `layers`.

**Delta context:** the "Consumers / links" section of `.pipeline/knowledge/<project>/modules/<module>.md` is a
ready-made list of what depends on the code being changed. Use it as the starting point for the
blast radius, then confirm it against the code.

**If the task involves deleting or replacing code — apply the full process from the skill:**
- Decision Framework (5 questions)
- Zombie Code check: `grep -r "FunctionName" .`
- Strategy choice: Parallel-run / Strangler Fig / Adapter Pattern / Feature Flag / Big Bang

**Output:** affected layers, risks, backward compatibility and the migration strategy are agreed.
