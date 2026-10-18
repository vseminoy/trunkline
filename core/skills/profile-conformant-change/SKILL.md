---
name: profile-conformant-change
description: Writes code under the rules of the stack profile a project declares — the manifest's layers and commands, and the guidelines for the roles the change touches. Use when changing code in a project that declares a `profile` flag, outside the pipeline's steps: the short-change route, or any edit where the profile's rules apply but no route was announced. Not for deciding what to change — it governs how the change is written and verified.
---

# Writing code under the stack profile

## Overview

**Persona:** `core/developer/personas/senior-developer.md` — an AI Senior Developer of the language
the manifest's `language` slot names. That file is the role; this one is the rules the role is held
to on a change made outside the pipeline's steps.

A project that installed this pipeline keeps its stack rules in one place: the profile. The manifest
(`profiles/<name>/profile.yaml`) names the layers, the build and test commands and the guideline
files; the guidelines carry the rules the team actually enforces on contracts, migrations, types,
authorization and schema style. This skill is how those rules reach a change that is written without
a pipeline route: which of them to read, when to apply them, and what makes the change verifiably
conformant rather than plausibly so.

It governs **how** a change is written. What to change, and whether the change is small enough to be
made this way, is decided before this skill starts.

## When to Use

- A change to code in a project whose `AGENTS.md` declares a `profile` flag, made outside the
  numbered steps — the short-change route (`core/developer/QUICK.md`) is its main caller.
- Any edit where a rule of the profile plausibly applies: a handler, a query, a type, a migration
  touched in passing, an error path.

**When NOT to use:**
- Inside the pipeline's steps. Each of them names the guideline roles it needs itself, and a second
  reading of the same rules from here adds nothing.
- A project with no profile declared. Stop and ask which profile applies — this skill has no rules of
  its own to fall back on, and inventing them is worse than asking.
- A profile with no `guidelines`. That is allowed: say the conformance block is not applicable and
  keep the commands half of the process.

## Core Process

### 1. Resolve the rules that apply — before the first edit

1. Read the `## Pipeline flags` block of the project's `AGENTS.md`. `profile: <name>` is the answer
   to which stack this is. No profile declared → stop and ask; do not guess it from the code, since
   the wrong profile brings the wrong rules and nothing later will contradict them.
2. Read `profiles/<name>/profile.yaml` in full. It is small, and the four things needed here are all
   in it: `language`, `layers`, `commands.*`, `guidelines`.
3. Name the layers the change touches, from `layers`. This is the step that decides everything after
   it, so say the answer out loud rather than holding it.
4. Read the guideline files for the roles those layers need (`guidelines.roles.*`, with
   `guidelines.index` as the table of contents). **Read the roles the change touches, not all of
   them** — and read them in the files themselves; they are extended independently of the pipeline,
   so a remembered version of a rule is a stale one.

### 2. Write the change against those rules, not against habit

- The rules are applied **while writing**, not audited afterwards. A rule discovered after the fact
  turns a small change into a rewrite, which is exactly what this route was meant to avoid.
- Precedence on a conflict: **the project's `AGENTS.md` → the profile guidelines → this skill → your
  own preferences.** A guideline that contradicts your habit wins, and it wins without discussion.
- A guideline that contradicts the code around it is a finding, not a licence. Follow the guideline
  for the lines you write, and report the discrepancy; do not restyle the neighbours to match either
  side.
- A rule you cannot satisfy without growing the change beyond its scope is where you stop and say so.

### 3. Verify with the profile's own commands

Run, from the profile: `commands.build`, `commands.lint`, and the narrowest test command it declares
that covers the touched code (`commands.test_unit` where there is one, otherwise `commands.test`).

- A slot that is absent or `null` is reported as absent — "not applicable, the profile declares no
  `commands.lint`" — never silently skipped.
- A failing command means the change is not done. Fix it, or hand the task back. "Fails for an
  unrelated reason" is a claim that needs the output to back it.

### 4. Report conformance in a form that can be checked

For each guideline role read, one line: the rule that applied, and where it applied — `file:line` —
or "no rule of this role applied to this change", with why. A report that says "follows the
guidelines" without naming one is not evidence; it is the same sentence a change that read nothing
would produce.

Then the commands and their results, and anything noticed that is outside this change's scope,
reported and left alone.

## Common Rationalizations

| Rationalization | Reality |
|---|---|
| "The change is two lines, the guidelines are for real features" | The guidelines are about how code is written, and two lines are written code. Reading one role costs less than the review that catches the violation. |
| "I know this profile, I read it last time" | Guidelines change independently of the pipeline, and you are remembering a file, not reading it. Reading it again costs seconds. |
| "The surrounding code does it this way" | The surrounding code predates the rule as often as it embodies it. The guideline decides for the lines you write; the neighbours are a finding, not a precedent. |
| "No profile is declared, but the stack is obvious from the code" | The stack is obvious; the team's rules are not, and those are what the profile carries. Guessing installs the wrong ones silently. |
| "Lint fails on something unrelated to my change" | Then show the output and say so explicitly. Unverified, that sentence is indistinguishable from a change that broke the build. |
| "Following the rule would mean touching three more files" | That is the signal the change is bigger than it looked. Say so and hand it back, instead of shipping the half that fits. |

## Red Flags

The checklist below catches what was skipped. These two are not skips but substitutions, and no
checkbox notices them: a guideline rule quoted from memory rather than read from the file, and a
command from `commands.*` replaced with a quicker one you preferred.

## Verification

- [ ] The `profile` flag was read from the project's `AGENTS.md`, and the manifest was read in full.
- [ ] The manifest's `language` is known, and the code is written idiomatically for it.
- [ ] No file was edited before the manifest was read.
- [ ] The touched layers are named, and the guideline roles follow from them.
- [ ] Each of those roles was read from its file in this session.
- [ ] `commands.build` and `commands.lint` ran and passed; absent slots are reported as absent.
- [ ] The test command covering the touched code ran and passed.
- [ ] The report names, per role, the rule applied and the `file:line` it applied to.
- [ ] Everything noticed outside the change's scope is reported and untouched.
