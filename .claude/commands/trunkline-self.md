---
description: Run the trunkline pipeline on this repository itself (AI Senior Developer)
---

Read `core/developer/PIPELINE.md` and follow it as the AI Senior
Developer. That file is the core and is stack-agnostic.

The stack — language, build commands, guidelines, layers, step overlays — comes
from the profile:

- the profile name is in the `## Pipeline flags` block of this project's
  `AGENTS.md` (`profile: <name>`; no profile is declared in this project yet);
- the manifest is `profiles/<name>/profile.yaml` — read it in
  full right after the core;
- if no profile is declared, stop and ask which to apply. The candidates are the
  directories in `profiles/`. Do not guess from the code: the
  wrong profile silently disables half the cross-checks.

The same block declares the knowledge base (`knowledge`: a path, an
external path, or `none`) and the run journal (`metrics`). Announce
the run number, the profile and its version, the knowledge base and every flag in
the opening announcement, before the first step.

Step instructions live in `core/developer/steps/` and are read
one at a time, immediately before the step — not upfront. If the profile
declares an overlay for a step (`step_overlays`), read it right after the
step file.

Cross-cutting material: `core/skills/project-knowledge/SKILL.md`
(knowledge base), `core/developer/references/` (run journal),
`core/developer/ENFORCEMENT.md` (hooks and machine gates),
`core/ORCHESTRATION.md` (what your host can and cannot do).

Everything is relative to the project root. `.pipeline/` and `.claude/` are
this project's state; the pipeline is this repository itself, so its paths have no prefix.

The user's task: $ARGUMENTS
