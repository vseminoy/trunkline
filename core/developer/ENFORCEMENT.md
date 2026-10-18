# Machine enforcement of the gates — installation

This addresses the weakness that "enforcement is entirely social": sixteen stop gates resting purely
on the model's obedience. Whatever a machine can check should be checked by a machine — the prompt
text is left to carry only what a machine cannot check.

This is an installation guide. The hooks are configured in the `.claude/settings.json` of **the
project being developed**, once.

Three gates are machine-checked, and each one closes a different failure mode:

| Gate | Where it fires | What it stops |
|---|---|---|
| build + lint | `git commit` | a changeset that does not compile or does not lint |
| approvals (`core/developer/scripts/approval-gate.sh`) | `git commit` | production code committed before the approvals the route requires |
| the 7a trace check (`trunkline trace-check`) | step 7a, by hand | a rule that reached no artefact — the pipeline's only *silent* failure |

## Already in use (from agent-skills)

- `agent-skills/hooks/SDD-CACHE.md` — a WebFetch cache for the documentation of the tools listed in
  the profile's `hooks.doc_cache_targets` (step 8).
- `agent-skills/hooks/SIMPLIFY-IGNORE.md` — protection of performance-critical blocks (step 11).

## The lint and build gate before a commit

This turns the rule "the profile's `commands.lint` — if it does not pass, the task is not done" from
text into execution. A `PreToolUse` hook on `Bash` for commands containing `git commit`: exit code 2
blocks the call and returns the reason to the agent.

The script itself comes from the profile — the path is in the `hooks.pre_commit_gate` slot. Copy it
as is; there is no need to edit it, because it knows its own stack's commands.

`trunkline init --gate=claude|git` does this installation; the steps below are the manual equivalent,
for a setup done by hand or repaired after the fact.

Installation (from the root of the project being developed; `<pipeline root>` is the directory
holding `core/` and `profiles/` — `.trunkline/` in an installed project):

```bash
mkdir -p .claude/hooks
cp <pipeline root>/<the profile's hooks.pre_commit_gate> .claude/hooks/
chmod +x .claude/hooks/pre-commit-gate.sh
```

Registration in the project's `.claude/settings.json`:

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          { "type": "command", "command": "bash \"${CLAUDE_PROJECT_DIR}/.claude/hooks/pre-commit-gate.sh\"" }
        ]
      }
    ]
  }
}
```

The script's contract, identical for every profile: it stays out of the way in a project of a
different stack; it blocks the commit if `commands.build` or `commands.lint` fails; and it hands the
failing command's output back to the agent.

It must serve **both** triggers, because how a commit is recognised differs between them:

| `TRUNKLINE_GATE_MODE` | Trigger | How a commit is recognised |
|---|---|---|
| unset / `claude` | a `PreToolUse` hook | JSON on stdin; proceed only if `.tool_input.command` contains `git commit`, otherwise exit 0 |
| `git` | `.git/hooks/pre-commit` | there is no stdin to inspect — being called at all is the signal; run the checks |

Getting that split wrong is not a visible failure. A script that only knows the first mode reads an
empty stdin as a git hook, finds no command, exits 0 — and the gate passes every commit while
looking installed. Requirements and quirks of a particular implementation belong in the profile's
README.

If a profile has no `hooks.pre_commit_gate`, the gate is not installed — and that is announced to
the user as a known hole in enforcement rather than passed over in silence.

Verify after installing: a commit made in a deliberately non-building state must be rejected with
the build error text.

The gate sits on the commit rather than on every `Edit` on purpose: the incremental cycle of step 8
must not trip over intermediate non-building states.

## The approval gate: no production code before the route's approvals

`core/developer/scripts/approval-gate.sh` is the second machine gate. It turns prohibition 3 ("write
production code before every approval") from text into execution — the pipeline's most expensive
failure, because code written against an unapproved spec is not one wasted step but a changeset whose
every later step was spent on the wrong thing.

It needs no new subsystem. The journal already records which route is running and which steps it
plans (the `start` event), so the approvals a run owes are **derivable**, and each approval is one more
line in the same file (the `approval` event — see `core/developer/references/pipeline-evals.md`). The
gate is a second query to something that already exists: read the open run, take the changeset it has
reached, subtract the approvals recorded for that changeset **and the steps it recorded as
`skipped`**, and if any of steps 5, 6, 7, 7a are still owed while the staged diff contains production
code — exit 2 with the list.

**Per changeset, not per run.** All four gate steps live inside the changeset loop and step 18 commits
once per changeset, so a tally kept per run let changeset 1 pay for the whole task: its four lines
opened every later commit, which on a decomposed task is most of them. The changeset being committed
is read off the journal — the highest one its gate events have named — and needs no declaring,
because the commit is the last thing a changeset does: by step 18 that changeset's own approvals are
already in.

Subtracting `skipped` is not a loophole but the thing that makes conditional gates work at all. Step
6 is conditional on `feature` and steps 6, 7 and 7a are conditional on `migration`, and whether they
apply is settled mid-run, long after `start` was written. Without that subtraction a feature touching
no database reaches its commit owing an approval for a step that never ran, and the only way past is
to log an approval nobody gave — the exact failure the gate exists to prevent. So a `skipped` event
is as load-bearing as an `approval` one, and fabricating either is the same offence.

It ships stack-agnostic and lives in `core/` rather than in a profile, because what counts as an
approval is a property of the route, and the route is the same on every stack. The profile's
`hooks.pre_commit_gate` calls it first — it reads a journal and a diff, which is cheaper than a build,
and this is the failure worth reporting before the developer has waited for a linter.

What it does **not** do, stated plainly because a gate whose reach is overstated is worse than none:

| Limit | Why it is acceptable |
|---|---|
| Fires at the commit, not at the write | Same reason as the build gate: step 8's incremental cycle must not trip over intermediate states. Code can be written early; it cannot be committed. |
| Rests on the `start` line | A run that never wrote one is not treated as a pipeline run. Such a run is already broken and visibly so in the journal; blocking instead would block every hand-made commit in the project. |
| "Production code" comes from an artefact allowlist (`*.md`, `*.yaml`, `*.json`, `*.sql`, `.pipeline/`), not from knowing the stack | Deliberately permissive. A false block costs a commit; a false pass costs one changeset of a backstop. A project whose contract is `.proto` sets `TRUNKLINE_ARTEFACT_RE` rather than losing the gate. |
| `metrics: none` disables it | Announced on stderr, not passed over. With no journal there is nothing to read approvals from, and that is a fact about the project's configuration. |
| The tally rests on the journal naming its changesets | `approval` and `skipped` carry the changeset they belong to, and the gate counts per changeset — so changeset 3 owes its own four approvals, not changeset 1's. What it cannot do is notice a changeset that logged **nothing at all**: with no event of its own, the run has not moved past the previous one as far as the gate can see, and its commit is checked against that one's tally. A changeset that reached step 18 having recorded neither an approval nor a skip is a run that stopped writing the journal, which the report makes visible on its own. |

`TRUNKLINE_APPROVAL_GATE=off` disables it for one commit. That is a decision to record in the run
report: afterwards the journal shows a commit whose approvals were never logged, and nobody will
remember why.

**Logging an approval that was not given defeats the whole thing.** The journal is the only record
that these gates ran at all, so a fabricated `approval` line is worse than a skipped gate — it is a
skipped gate that looks passed. Same class of defect as a hand-filled 7a matrix.

## Where a host can do this better

On some hosts this gate is declarative rather than scripted. In SourceCraft Code Assistant (and in Roo
Code, its upstream) a mode declares its file permissions — `groups: [read, [edit, {fileRegex:
'\.(md|ya?ml)$'}]]` — and a write outside the pattern returns a **tool error**, before the edit, not
before the commit. That is strictly stronger than a diff check: prevention instead of a backstop.

It does not move into `core/`, because it is a host capability and the core must run everywhere. So
the core keeps the textual rule plus this script, and a host row in `core/ORCHESTRATION.md` may
declare that it has a mechanical implementation of its own — the same arrangement already used for
C3.

## What stays textual (and why)

- The `[STOP]` checkpoints — they require the user's judgement; no machine replaces that.
- Approval of the spec, the schema and the test description — a human checking meaning. The machine
  checks only that the approval was *recorded* before the code was committed, never that it was
  deserved.
- Step 7a's checks 3–6 — numbers and boundaries, the error contract, types across layers, the join
  with the analysis. Those reconcile meaning; a hook cannot express them. Its checks 1–2 are **no
  longer** textual: `trunkline trace-check` decides them by whole-word grep over the
  run's coverage records, which is possible because the pipeline already requires each producing step
  to record where its rule landed. That was the pipeline's only *silent* degradation, and it is the one place where making a
  gate mechanical mattered more than anywhere else.

For the textual gates there is exactly one machine instrument — the journal
`.pipeline/workdir/metrics/pipeline.jsonl` (its format is described in
`core/developer/references/pipeline-evals.md`): a skipped step is visible in the journal after
the fact. Systematically skipping the same gate is a signal either to back it with a hook or to
honestly remove it from the pipeline.

## Who initiates the installation

Step 19 (the CI agent) checks whether the gate exists and offers to install it if it does not.
Installation happens only with the user's explicit consent: the hook changes the behaviour of their
sessions.
