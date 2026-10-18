# core/ — the pipeline core

The stack-agnostic part: routes, roles, gates, traceability, knowledge accumulation and telemetry.
Nothing about languages, frameworks or build commands belongs here — that is `profiles/`.

```
core/
├── developer/   ← the development pipeline: PIPELINE.md (core) + steps/ + references/
├── analytic/    ← the analysis pipeline: PIPELINE.md + rules/ + skills/
└── skills/      ← project-knowledge (.pipeline/knowledge/, both tracks) and
                 profile-conformant-change (writing code under the profile)
```

## What counts as core

- routes by task type and the modes a run can take;
- orchestration discipline and the `[STOP]` gates;
- `R-<run>-<n>` traceability from requirement to the knowledge base;
- artefact cross-checking (step 7a);
- the code knowledge base and the run journal (`.pipeline/`);
- the output format and the mandatory approvals.

## What is not core

Languages, code generators, `build`/`lint`/`test` commands, guideline names, system layers, gate
scripts. The core takes all of that from the profile manifest through slots — see the "STACK
PROFILE" section of `core/developer/PIPELINE.md` and `profiles/README.md`.

Artefact templates are split rather than assigned: **the shape of an artefact is core, the names and
values in it are the profile's.** `core/developer/references/tests-readme-example.md` shows what
sections a tests README has and in what order — that is the same in every stack; the layers it names
come from the profile's `layers` and the step overlay. A template that cannot be read without
knowing the language belongs in the profile.

A check when editing: if a sentence starts with "in \<language\> we usually" — it belongs in a
profile. The machine version of the same check: `python3 scripts/lint-core-profile-boundary.py`.
