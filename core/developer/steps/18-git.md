# Step 18. Git agent `[STOP]`

**Skills:** `agent-skills/skills/git-workflow-and-versioning/SKILL.md`

Apply the full process from the skill, including worktrees for parallel work and git bisect for
debugging.

**One exception, and it is deliberate: the skill's Save Point Pattern is not in force here.** The
skill commits after every change that passes its tests; this pipeline commits **once per changeset**
and nowhere else — an increment is a `git add` (step 8), and no step between 8 and this one commits
anything. So no `chore: save point` commits, no scratch anchors to fold away later, and the atomic
rule below holds without an exception to remember. What replaces the anchor is a human at every
`[STOP]`: each of steps 11, 12 and 16 is accepted before the next one starts, so a bad change is
caught one step after it was made rather than five.

**Runs only after the changeset has been verified — by whatever the route provides:** no Critical
left open from a review step the route ran (9+10), and the profile's `commands.build` and
`commands.lint` passing. Those are the same two commands the commit gate runs, so on every route that
half is enforced by a machine rather than remembered (`core/developer/ENFORCEMENT.md`).

The Assessor (step 17) is **not** that gate and does not precede this step. It is a pre-launch
checklist over the whole delivery and runs once, after the last changeset is committed (see "The
changeset loop" in the core). A commit is verified per changeset; a delivery is assessed once.

On `refactor` the review must also **postdate steps 11 and 12** — they are the entire point of that
route, so a review taken before them describes code that no longer exists. If they changed anything,
9+10 runs again over their diff first (see "A review describes the code that existed when it ran" in
the core).

Say what verified this commit. "Step 17 is not what gates a commit" and "nothing verified this work"
are different statements, and a run that cannot tell them apart will settle into the second.

**Branches:** the project takes trunk-based development as its reference point, but branches may
live longer than 3 days if the project's `long_lived_branches` flag is set — that is fine. On a
long-lived branch the agent must run `git fetch && git rebase origin/main` before committing (or
offer to) so merge conflicts stay small. Do not create blockers and do not warn about a policy
violation because of a branch's age.

**Pre-commit hygiene:**
- `git diff --staged` — only what is needed, no stray files
- Secret scanning — governed by the project's `secrets_in_code` flag (skipped when `true`)
- No `fmt.Println` / debug lines
- No `*.tmp`, `*.bak`

**Atomic commits:** each changeset = its own commit. Refactoring separate from functionality.

**What is committed alongside the code:** the artefacts that describe the code and live with it —
`tests_readme.md`, `sequence_diagrams.md`, YAML specs, migrations.

**Nothing under `.pipeline/workdir/` is committed:** the rule registry, the cross-check and the
review reports are working files, read during the run and superseded by the time it ends. What
survives them is what a reader actually needs: the rule with its statement and enforcement point goes
to the knowledge base at step 20, and the committed artefacts state the rule as behaviour. **No
committed file carries an `R-<run>-<n>`** — not a comment, not a `description`, not a commit
message: it would point at a working file that is about to be deleted.

`.pipeline/` splits in two along exactly one line. **Kept:** the directory the `knowledge:` flag names
— `adr/`, `modules/`, `slices/`, `INDEX.md` — plus `tasks/`; they describe the project and are
committed. **Discarded:** everything under `.pipeline/workdir/`, the state of one run, which belongs
in the project's `.gitignore` as the single line `/.pipeline/workdir/`. The knowledge base is
committed at step 20 as a separate `docs(knowledge): ...` commit; an ADR written at step 15 rides
along with it.

**Commit format:**
```
<type>(<scope>): <short description>

<body: WHAT and WHY, if not obvious>

DIDN'T TOUCH: <what was deliberately left alone>
```
`DIDN'T TOUCH` is mandatory whenever the scope could raise questions.

Show `git log --oneline -5`. Do NOT push without an explicit request.

**Handing the loop on (on routes that have step 4).** This step is the seam between changesets. Once
the commit is in, name what is left from the step 4 plan — how many changesets remain, which ones and
what each closes — and stop. On "carry on" the run begins the next changeset at the **first step of
the loop**, not at the next numbered step: that is the single exception to what "carry on" means in
this pipeline, so say which of the two you mean rather than letting it be inferred. When the plan is
exhausted, say so and go on to the route's remaining steps — 17, then 19? and 20.

A plan with items still on it and a run that walks past this point is how changesets get dropped in
silence: the commit looks like completion, and nothing downstream checks that it was the last one.
