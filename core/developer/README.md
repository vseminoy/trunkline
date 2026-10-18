# core/developer/ — the developer pipeline

The core is `core/developer/PIPELINE.md`: the rules that hold at every step, the pipeline table, the
loading protocol. What lives here is everything loaded on demand.

Paths inside these files are given **from the root of the pipeline** — the directory holding `core/`,
`profiles/` and `agent-skills/` (`.trunkline/` in a project, the repository root here) — not from the
current directory. See the "STEP LOADING PROTOCOL" section of `PIPELINE.md`.

```
core/developer/
├── PIPELINE.md      ← core: cross-cutting rules, routes, the profile and step-loading protocol
├── QUICK.md         ← the short-change route: no steps at all, only the profile, four skills and the gates
├── steps/           ← one step = one file, read immediately before that step runs
├── ENFORCEMENT.md   ← installing hooks and machine gates (step 19)
├── scripts/         ← the machine halves of steps that have one: 7a, 9+10, 21, and the cost report
└── references/      ← artefact templates and the journal format
```

The gate scripts themselves are not here — they are stack-specific and live in the profile
(`profiles/<name>/hooks/`, the `hooks.pre_commit_gate` slot). `ENFORCEMENT.md` says how to install
one; it does not carry one.

There is no stack specificity here: the language, tools, commands, guidelines and layers come from
the profile (`profiles/<name>/profile.yaml`), and step overlays from `profiles/<name>/steps/`.

The delta mechanics (`.pipeline/knowledge/`) do not live here either, but in the shared skill
`core/skills/project-knowledge/SKILL.md` — both directions read it.

## The short-change route

`QUICK.md` is read instead of `PIPELINE.md`, and it uses none of the steps: its whole apparatus is
`core/skills/profile-conformant-change/SKILL.md` — where the profile's rules meet the code — and
three skills from `agent-skills/`. It exists because the pipeline's cost is
fixed while a change's size is not, and a route that is cheaper than the pipeline is the only honest
alternative to running the pipeline dishonestly — announcing seven steps and doing one. What keeps it
from becoming a second pipeline is its list of conditions: fail one, and the task goes back to
`/trunkline-dev` rather than continuing in a lighter mode.

## Why it is split up

A monolithic prompt sat in context in full at every step. By the later steps (refactoring, git) the
competition for attention grew precisely where a mistake costs the most. Now the context holds the
core plus one current step.

## How to add a step

1. Create `steps/NN-<name>.md`: skills / persona / reference, profile rules (by role), the order of
   work, and an "Output" section. Artefact templates go in `core/developer/references/`, not into
   the step text.
2. Add a row to the core's "PIPELINE" table and to the task-type routes in "BEFORE THE PIPELINE".
3. If the step finds things — decide which event it writes to the journal
   (`core/developer/references/pipeline-evals.md`).
4. Do not copy the contents of profile guidelines or `agent-skills/` here — link to them.
5. Do not put stack specifics (language, tools, commands) into the core — they go into the profile
   overlay (`profiles/<name>/steps/NN-*.md`) and are declared in `step_overlays`.

## Invariants that are easy to break

- A step must not depend on the text of another step: everything cross-cutting lives in the core.
- Subagents do not write files — only the main agent does (otherwise parallel writers overwrite
  each other).
- Rule identifiers run end to end **inside the run's working directory**: a rule gets its
  `R-<run>-<n>` at step 2 and keeps it through the coverage records of steps 5, 6 and 7 to the
  knowledge entry of step 20. Nothing committed carries the identifier — the spec, the schema, the
  tests and the commit state the rule as behaviour. An identifier is never reused, and a withdrawn
  one is never reassigned. If the chain breaks, step 7a is obliged to catch it.
