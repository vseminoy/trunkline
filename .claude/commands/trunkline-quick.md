---
description: Make a short change to this repository without running the pipeline, under the profile's rules
---

Read `core/developer/QUICK.md` and follow it. It is a route of its
own, not a lighter pipeline: no steps, no step files, no subagents, no
approvals. What it uses is the skill
`core/skills/profile-conformant-change/SKILL.md` — how code is written
under this project's profile — plus the three skills that route names, and the
build and the lint gate, unchanged.

Read the route file first, then check the task against the conditions it lists
**before** touching anything. A task that fails one of them does not run here:
say which condition failed and hand it to `/trunkline-self`.

The stack comes from the profile:

- the profile name is in the `## Pipeline flags` block of this project's
  `AGENTS.md` (`profile: <name>`; no profile is declared in this project yet);
- the manifest is `profiles/<name>/profile.yaml` — read it in full
  before the change: `commands.*` are the gates, `guidelines` are the
  rules the change must obey, `layers` say where it sits;
- if no profile is declared, stop and ask. Do not guess from the code.

Everything is relative to the project root. `.pipeline/` and
`.claude/` are this project's state.

The user's task: $ARGUMENTS
