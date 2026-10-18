# The short-change route

A route for a change small enough that the pipeline would cost more than the change: a typo in a
message, a renamed variable, a guard on a nil, a log line, a bumped constant, a comment that
contradicts the code.

**It does not use the pipeline.** No steps, no step files, no routes, no subagents, no rule
registry, no approvals, no journal, no knowledge base. `core/developer/PIPELINE.md` and
`core/developer/steps/` are not read on this route, and nothing here is a lighter version of a step.
Three things come from outside this file and nothing else does: the **profile**, the **persona** and
the **skills** below.

**Persona:** `core/developer/personas/senior-developer.md`

**Skills:** `core/skills/profile-conformant-change/SKILL.md`,
`agent-skills/skills/incremental-implementation/SKILL.md`,
`agent-skills/skills/source-driven-development/SKILL.md`,
`agent-skills/skills/debugging-and-error-recovery/SKILL.md`

Read them at the start of the change and follow them. They are what carries the work here, and the
first one carries the profile: which rules the change is written against, and what makes it
verifiably conformant. The other three are how the writing itself is done — incremental change
instead of decomposition, documented behaviour instead of assumed API behaviour, a named cause
instead of a guessed fix.

---

## What qualifies

Every one of these must hold. They are read against the change as it will be, not as the request
describes it.

- one intention, a handful of files, one commit;
- no change to a contract, a database schema, a migration or a public interface between layers;
- no new module, no new layer, no new dependency;
- behaviour is already decided — there is nothing to specify and nothing to analyse;
- nothing touching authentication, authorization, secrets, money or personal data;
- an existing test either covers it or is not the thing that makes it safe (a rename, a comment);
- for a fix: the cause is already named, not guessed.

**If any one of them fails, this route stops.** Say which condition failed and hand the task to the
pipeline (`/trunkline-dev`) — do not carry on in a lighter mode. The same applies mid-change: a fix
that turns out to need a schema change, a contract change or a second intention is not a short
change any more. Stop, say what it grew into, and let the user restart it on the pipeline. **The
work done so far is not committed on this route** — the pipeline starts from the task, not from a
half-made change.

## What is read

What to read and in which order is `profile-conformant-change`'s process, and this route adds
nothing to it: the profile flag, the manifest, the guidelines for the roles the change touches, and
the code being changed. All of it is mandatory here — the route is short, the project's rules are
not relaxed.

The one thing this route settles itself: step overlays (`step_overlays`) are not read, because they
refine steps and this route has none.

## The sequence

1. **Announce and confirm.** One short block, before any edit: what the change is, which files, why
   it qualifies (the conditions above), the profile and the guideline roles that apply, and the
   commands that will verify it. Wait for the user. This is the route's only stop.
2. **Change.** Only the files named in the announcement, in the increments
   `incremental-implementation` describes. A change that starts spreading beyond those files is the
   signal to stop, not to widen the announcement.
3. **Verify.** The profile's commands, as `profile-conformant-change` specifies them. If one fails,
   the change is not done: fix it or hand the task over, do not report it as passing with a caveat.
4. **Report.** That skill's conformance report, plus the diff, the command output, and anything
   noticed on the way that is not this change's business. Findings are reported, not fixed: a second
   intention would disqualify the route it was found on.
5. **Commit, if the user asks for it.** One commit, in the project's existing commit style. The
   build-and-lint gate of `core/developer/ENFORCEMENT.md` applies to it like any other commit; the
   approval gate lets it through because there is no pipeline run open, which is the honest state of
   this route and not a bypass.

## Output

Every reply begins with the line `Quick:` and a newline, then the announcement, the report or the
answer. The route has no sectioned output format: what it produces is a small diff a person can
read.

Reply in the language the user writes to you in; write the commit message in the language the
project already uses.
