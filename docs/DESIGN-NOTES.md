# Design notes: why the developer core is shaped the way it is

This file holds the **reasoning** behind rules that live in `core/developer/PIPELINE.md`. It is read
by whoever edits the pipeline, not by the agent running it: the core is loaded in full at the start
of every run and competes for attention with the project's own code, so a justification that answers
a maintainer's question does not belong in it. Each section below names the rule it justifies; the
rule itself stays in the core, in one or two lines.

This file is **not vendored into a project** — `assets.go` ships `core/`, `profiles/`, `publishers/`
and the referenced parts of `agent-skills/`, and nothing from `docs/`. That is deliberate: an
installed pipeline needs the rules, not their history. Which is also why the core must never name
this file by path — a path that resolves here resolves nowhere in an installed project, and
`scripts/lint-pipeline-paths.py` fails the build if one appears.

---

## Why step numbers do not ascend along a route

*The rule: never reorder a route to make its numbers ascend, and never renumber a step to match a
route.*

The numbers label roles, not positions. Two roles are needed in **opposite orders on different
routes**: `config/ci` runs CI (19) before the commit (18), `feature` runs the assessment (17) after
the last commit of the changeset loop. No numbering satisfies both, so any renumbering that makes one
route read smoothly breaks the other.

The labels already say this on their own: `7a` sits between 7 and 8, and `9+10` is one step carrying
two numbers. What the stable number buys is that the journal, the approval gate and the step files
can all refer to the same role — `steps_planned: [..., "7a", ...]` means the same thing on every
route, and the gate that reads it needs no route-specific translation.

## Why there is no unattended mode

*The rule: the pipeline has one way of running — a `[STOP]` at every step of the route.*

One existed and was removed. The arithmetic is why: of the sixteen `[STOP]` steps only four are
approvals (5, 6, 7, 7a). A no-confirmation mode honours those four and passes the other twelve — 1,
2, 3, 4, 8, 9+10, 11, 12, 15, 17, 18, 20 — with nobody but the author looking.

Two of those twelve are load-bearing in a way the rest are not.

- **Step 2** writes the rule registry, and step 7a checks that every identifier in it reached every
  artefact. Skip the human at step 2 and 7a stops being a cross-check: it becomes a proof that the
  agent does not contradict itself, and it goes green on a coherent set of wrong rules.
- **Step 1** ends with "→ Confirm or correct these before I continue". In such a mode that is the
  agent confirming its own assumptions, and Surface Assumptions is the first of the six mandatory
  behaviours.

It also bought less than it appeared to. Every step still ran and still cost its tokens; what was
saved was the user's clicking, not the pipeline's weight. When a route feels heavier than the task
deserves, the levers are the route, the conditional steps and the number of changesets — all of which
remove work. Removing supervision only hides what the work costs.

## Why the identifier stops at the repository boundary

*The rule: no committed file carries an `R-<run>-<n>`. Each producing step records where its rule
landed in a coverage record under `.pipeline/workdir/coverage/`, and 7a greps those.*

The registry is an artefact **of the task while it is being done**. It is written at step 2, read by
steps 4, 5, 6, 7, 7a and 9, and once step 20 has moved the rules that stay in force into the
knowledge base it has no reader left; the working directory it lives in is gitignored and gets
deleted. A `-- R-007-2` in a migration, an `R-007-2:` prefix in a `description` or a `Rule` column in
`tests_readme.md` therefore outlives the only thing that could explain it. Six months later the
number is unresolvable: a reader greps it, finds only the marker they started from, and learns
nothing. That is worse than no marker, because it looks like there is a trail.

What replaces it is not weaker but relocated. The committed artefacts state the rule **as
behaviour** — `minimum: 1`, `chk_amount_positive`, `TestCreate_ZeroAmount`, a `400` branch — which is
what a later reader can actually verify against the code. The identifier keeps living where its
registry lives, in the run's working directory, so `trunkline trace-check` still decides checks 1 and 2 by
whole-word grep, exactly as before; only the files it greps changed. And what must survive the run
survives in the knowledge base, whose "Business rules in force" table carries the identifier next to
an enforcement anchor — the one place a rule is meant to be looked up after the fact.

The cost is real and is paid deliberately: the grep no longer proves the identifier is *in* the
contract, only that step 5 said it reached it. So 7a's checks 3-6 are now obliged to open each place
a coverage row names — a row citing a field or a constraint that is not there is a `MISMATCH`, and
that is the defect class this relocation creates. Checks 3-6 were always read by hand; what changed
is that they now also confirm the row.

## Why the run number is inside the rule identifier

*The rule: the identifier is `R-<run>-<n>`, assigned at step 2 and nowhere else.*

Identifiers outlive their run in two places: the knowledge base entry of step 20, and the coverage
records an `audit` run is pointed at. Those identifiers are absent from the current run's registry,
so step 7a's orphan check would report every one of them — a project would pass 7a exactly once per
module and then fail on work it had already accepted.

Because the run is inside the identifier, an `R-006-1` met during run 007 is **decided** as belonging
to an earlier run rather than guessed at. Only an identifier of this run, or of a later one, can be
an orphan. The same property removes the need to look up "the next free number": the run number is
unique across the project and known before the first rule is written, so two tasks on one module can
never claim the same identifier without anyone coordinating.

## Why two steps commit, not one

*The rule: exactly two steps run `git commit` — step 18 (the changeset) and step 20 (the knowledge
base).*

It looks like an exception to "one changeset, one commit" and is a consequence of the order. Step 20
writes the slice entry, and that entry names the commits the run produced — so it can only be written
after they exist, and what it writes can only be committed after that.

The alternatives are worse. Amending a commit that has already passed the build, lint and approval
gates re-opens what those gates closed. Leaving the knowledge base uncommitted contradicts the one
thing that makes the delta mechanics work — that the base travels with the code — and `trunkline
doctor` reports it as a fault. A second, docs-only commit costs one line in `git log` and keeps both
properties.

The two never mix content: the changeset commit carries no `.pipeline/knowledge/`, and the knowledge
commit carries no code.

## Why the registry keeps the task's requirements beside the rules

*The rule: step 2 copies the task's "How to test" block into `.pipeline/workdir/rules/<NNN>.md` and every
rule names the item it came from.*

A registry is perfectly self-consistent without the requirement that never became a rule. Nothing
inside it can point at the omission, so a check that reads only the registry cannot find one. With
both halves in the same file the comparison is a count, which is why step 7a's check 6 is decided by
a script rather than by judgement — and why it applies to every task, not only to those that came
from analysis: a task always has text.

## Why steps 9+10 run a second time after steps 11, 12 or 16

*The rule: if those steps changed code, the review runs again over their diff, before the route's
verification gate.*

The two failures likeliest in exactly those steps are the two that tests do not catch: a race
introduced while removing a lock (step 12), and a secret or PII written into a new log line (step
16), long after the Security auditor finished. Both survive a green suite, and on `feature` and
`refactor` both land after the only review the changeset gets.

The second pass is scoped to those steps' diff alone: the rest was reviewed already, and re-reading
it costs context without buying a finding.

## Why an identifier is never minted outside step 2

*The rule: on a route with no step 2 the identifier-shaped instructions are "not applicable", and
raising a number on the spot is forbidden.*

An identifier raised outside step 2 appears in no registry. Nothing can then ask about it: step 7a
reads the registry and `trunkline trace-check` reads the registry. A row citing it looks like traceability
and answers to nobody — decorative, and worse than absent, because a reader who greps it finds a hit
and concludes the chain is intact.

The same reasoning is why the two failures are named separately in the core: passing the check
silently is indistinguishable from having checked it, and both hide the route property that made the
instruction inapplicable in the first place.
