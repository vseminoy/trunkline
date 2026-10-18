# Steps 9+10. Critic agent + Security agent `[IN PARALLEL → a single STOP]`

Pattern 3 (parallel fan-out) from `agent-skills/references/orchestration-patterns.md`.
A Critical from either one → roll back to step 8.

**Launching (mandatory):** the main agent makes **two Agent tool calls — the Critic and the Security
auditor — in a single assistant turn**. Not two turns in a row: consecutive calls serialize, and the
step stops being parallel. Where the declared host has no subagents at all, see "Hosts without
subagents (C1 absent)" below — that case has its own route through this step, not a silent one.

**Pattern 3 validation (the catalogue checklist under "3. Parallel fan-out with merge" in
`agent-skills/references/orchestration-patterns.md`) — passed for this
pair:**
- [x] *Can both run at once with no ordering problems?* Yes: both read the same finished diff from
      step 8, neither changes anything, and there is no shared mutable state (writing the review file is
      the main agent's job).
- [x] *Do the personas produce different KINDS of finding, rather than the same one from different
      angles?* Yes: the Critic covers correctness, readability, architecture and conformance to the
      profile guidelines; the Security auditor covers exploitable vulnerabilities, OWASP and trust
      boundaries. Their severity scales differ too (Critical/Important/Nit vs
      Critical/High/Medium-Low).
- [x] *Will the merge fit in the main agent's remaining context?* Yes: the merge is a concatenation
      of two reports into one file, with no reinterpretation of the content.
- [x] *Is the time saving noticeable?* Yes: both passes read the whole changeset diff, and this is the
      longest step of the pipeline.

If any item turns into a "no" on a particular task (e.g. the changeset is so small that both reports are
trivial) — fall back to a sequential launch: the Critic first, then the Security auditor; the single
`[STOP]` remains.

**On `audit` there is no step 8, so the diff is replaced by the scope.** That route reviews code that
already exists: what the subagents get is the scope named at step 1 — a commit range, a branch, a
module path — plus the rules step 7a found in an earlier run's registry, if there were any. Two
consequences. A Critical does not roll back to step 8, because there is nothing to roll back to: it
is the review's most severe finding and it goes to the report. And `<K>` in the file name below is
`1` — the route has one changeset and it is empty.
Everything else on this step is unchanged, including that the main agent is the only writer.

**On `audit` this is also the last step: step 20 does not run**, so this step closes the run itself.
After the merge (point 5 below), write the `done` line to `.pipeline/workdir/metrics/pipeline.jsonl` with
`knowledge: none` — see `core/developer/references/pipeline-evals.md` — and say so in the reply. A
run left without `done` stays open, and the approval gate reads an open run as an unfinished
delivery. `confirmations`, `rollbacks` and `duration_min` are counted off this run's own lines here
exactly as step 20 would count them; the reference says how.

**Context for the subagents:** pass into each subagent's task, as text — the changeset diff (on
`audit`, the scope above), the BUSINESS
RULES block (the identifiers and their statements, or the path to the registry), the path to this changeset's
cross-check under `.pipeline/workdir/consistency/`, and **the artefact language** (the project's
`language` flag, or the language its own code and docs already use — see "Language" in OUTPUT
FORMAT). Subagents do not inherit the session context; without the first three the Critic cannot
check the code against the business rules, and without the last one it writes its report in
English regardless of the flag — which then lands in the review file almost verbatim, since the
merge step below is a concatenation, not a rewrite.

**The findings are written TO THE FILE `.pipeline/workdir/review/<NNN>-c<K>.md` — where `<NNN>` is the run and `<K>` the
changeset — not to the console (BLOCKING order):**

One file per changeset. The review is a statement about one commit's worth of work, and a single
shared file would let the last changeset erase what was found in the first.

**Who writes the file:** the review file is composed and written ONLY by the main agent (the
orchestrator) at the merge step. The Critic agent and the Security agent are subagents: they **do not
write to files and do not touch it**, they return their report as text in the reply to the
main agent. This is a Pattern 3 requirement: subagents give independent reports, and the merge
happens in the main agent's context. Two parallel writers to one overwritten file would clobber each
other — so there is exactly one writer.

Order of the merge step:
1. The main agent receives both reports (Critic, Security auditor) as text.
2. In **a single write operation**, the main agent creates `.pipeline/workdir/review/<NNN>-c<K>.md`,
   overwriting it entirely with the results of this run: first the Critic agent's section, then the
   Security agent's (each under its own `##` heading). The section formats follow the templates in
   the corresponding personas (see below).
3. Do NOT print the report contents to the console. The reply states the path to that file and
   briefly lists how many Critical / Important / Nit (Critic) and Critical / High / Medium-Low
   (Security auditor) were found, and whether there is a Critical (→ roll back to step 8).
4. The user reads the details there. If they ask for changes or a recheck, the main agent
   reruns the relevant subagent and again overwrites THAT SAME file in full (partial edits from
   subagents are forbidden).
5. Record the number of findings by severity in the pipeline journal
   (`.pipeline/workdir/metrics/pipeline.jsonl`), **naming the changeset**, and the rollback to step 8 if
   there was one — see `core/developer/references/pipeline-evals.md`. This is the main source of data
   on whether the step pays for itself, and without `changeset` it says which run the findings belong
   to but not which piece of work — this step runs once per changeset.

## Hosts without subagents (C1 absent)

The value of this step comes from *ignorance*: the Critic disagrees with the author because the
Critic was not there while the code was argued into existence. Without C1 the main agent plays both
roles in the session that just wrote the code, and the step quietly becomes the author reviewing the
author — a degradation nothing in the transcript shows.

`trunkline review-check` restores the fresh context over HTTP where the host cannot
spawn: it concatenates the same personas and skills named below, calls an OpenAI-compatible endpoint
twice — each call from a clean context — and prints the two reports. It is not a subagent and not a
host: it applies nothing, writes nothing, runs no commands.

**The switch is not in the script.** The capability set is declared, not detected
(`core/ORCHESTRATION.md`), so this step decides, by looking at the declared `host` in the flags
block:

- **host has C1** (`claude-code`, Cline, Kilo Code, SourceCraft) — spawn as above. Do not call the
  script; it would be dead weight.
- **host has no C1** (Cursor, Codex CLI, Kiro, `generic`) — call the command instead of spawning,
  and say in the announcement that steps 9+10 ran through it and against which model.
- **on request** — a high-risk changeset may go through it even where C1 exists, which is the "second
  pass on a clean session" C1 already recommends, made reproducible. A weak model in a fresh context
  is still a weak reviewer; the command can be pointed at a stronger model than the one running the
  route.

```bash
git diff <base>..HEAD | trunkline review-check \
    --base-url "$TRUNKLINE_REVIEW_BASE_URL" --model <model> \
    --rules .pipeline/workdir/rules/<NNN>.md \
    --consistency .pipeline/workdir/consistency/<NNN>-c<K>.md \
    --guideline <role>=<path> \  # every guidelines.roles of the profile; repeatable
    --language <language>        # the project's `language` flag — see "Language" in OUTPUT FORMAT
```

The key comes from `TRUNKLINE_REVIEW_API_KEY` and has no flag: the invocation is quoted in the review
file. Guidelines are passed in because the command does not read `profile.yaml` — it is stack-agnostic
the same way the core is, and this step knows the profile, the command does not. `--language` exists
for the same reason: the personas concatenated into the call are shipped in English, and without it
the report comes back in English regardless of the flag.

**Nothing else about the step changes.** The two reports come back as text to the main agent, which
merges and writes `.pipeline/workdir/review/<NNN>-c<K>.md` exactly as above — one writer, unchanged. The
command issues no verdict and counts no findings: exit 0 means the reviews happened, not that they
passed, and there is no exit code 1 at all. Counting Critical / Important / Nit, deciding the
rollback to step 8 and writing the journal remain this step's, as they are with subagents. Mark the
sections in the file as obtained by the command, naming the model.

**Exit 2 — a report is missing, in whole or in part.** Whatever arrived is still printed and is
still worth reading, but half a review is not this step. Record `review_verify: unavailable` in the
journal, fall back to playing both roles sequentially in the session, and announce the degraded
pass. Never report the step as completed.

## The second pass (steps 11, 12 or 16 changed the code)

When this step runs a second time in one changeset — because simplification, performance work or
telemetry landed after the first pass (see "A review describes the code that existed when it ran" in
the core) — three things change, and nothing else does:

1. **The diff is narrower.** Pass only the diff of steps 11, 12 and 16, and name in each subagent's
   task which steps produced it and what they were trying to achieve. Re-reading the whole changeset
   buys nothing but tokens; the rest of it was reviewed and accepted on the first pass.
2. **Both personas still run.** Simplification and performance work are the Critic's ground, a new
   log line is the Security auditor's. Dropping the auditor because "it is only a refactor" is how a
   logged token reaches production — step 16 writes logging code, and no test in this pipeline looks
   at what a log line contains.
3. **A Critical rolls back to the step that introduced it** — 11, 12 or 16 — not to step 8. The code
   step 8 produced was reviewed and accepted already; sending the changeset back there would redo
   accepted work and lose the simplification along with the defect. The rollback is done by editing:
   nothing inside a changeset is committed, so there is no revision to reset to and a git command
   here would take step 8's work with it (see "The changeset loop" in the core). The finding names
   the code it is about, and that code is minutes old.

The second pass **appends** its sections to the same changeset file under a `## Second pass (steps
11, 12, 16)` heading rather than overwriting it: the first pass's Important and Nit findings are what
step 20 turns into "Known tech debt", and a partial re-review must not erase them. The one-writer
rule is untouched — the main agent is still the only one writing. This pass also records its own
`review` / `security` events in the journal. That is what lets the report tell a changeset reviewed once from one reviewed twice, and
answer whether the second pass ever finds anything — if it does not, across many changesets, it is a
candidate for removal like any other step.

## 9. Critic agent

**Skills:** `agent-skills/skills/code-review-and-quality/SKILL.md`, `agent-skills/skills/doubt-driven-development/SKILL.md`
**Persona:** `agent-skills/agents/code-reviewer.md`

Apply the five-axis analysis and the CLAIM→EXTRACT→DOUBT→RECONCILE→STOP cycle from the skills.
Apply the PR sizes from code-review-and-quality (100/300/1000 lines).
Read the tests first.

**Profile rules (check conformance):** every `guidelines.roles` — record any violation as
Critical/Important.

**Traceability check:** for every identifier of the changeset, confirm that the code really enforces
the rule — read the validation, the branch or the error path it lives in. Pay particular attention
to the rules whose `Origin` names a task item: those were written by someone else, in EARS, and a
meaning shifted in transit is a classic defect. A rule the code nowhere enforces is Critical: step 7a
checks the artefacts, this step checks the implementation.

**Only where step 2 is in the route** — see "Rule traceability" in the core. On `bug` and `refactor`
it is not, so there are no identifiers: mark this check "not applicable, step 2 is not on the
`<type>` route" in one line. Never let it pass silently — an unconditional check that reports nothing
reads as a check that found nothing. What replaces it is route-specific and still mandatory: on
`refactor`, that behaviour is unchanged; on `bug`, that the Prove-It test of step 7 fails against the
old code and passes against the new.

**Code conventions** (the source is the conventions file from the profile's
`required_project_files`; if only `AGENTS.md` is declared, take them from there):
- naming: no type in the name, nouns, business terms
- methods: `get` → `find` if it can return empty; booleans: `is/has/can`
- SQL schema: conformance to the profile's `db_schema_style` role (naming, comments, access)
- no swallowed errors, no mutable state in the service

The report format follows the template in the persona (Summary / Critical / Important / Nit / What's
Done Well / Verification Story). The report is returned to the main agent as text; the main agent is
what writes it to the file (the Critic section of the review). The subagent itself writes no files
and prints no report to the console.

## 10. Security agent

**Skills:** `agent-skills/skills/security-and-hardening/SKILL.md`
**Persona:** `agent-skills/agents/security-auditor.md`
**Reference:** `agent-skills/references/security-checklist.md`

Apply the three-level boundary system and the OWASP Top 10 from the skill. The focus is on
practically exploitable vulnerabilities.

The pre-commit secret check runs unless the project's `secrets_in_code` flag is set (see "Pipeline
flags" in the core prompt). With `secrets_in_code: true` — skip it; that is a deliberate decision by
the project team.

Severity: Critical → blocks (roll back to 8) / High → fix before continuing / Medium-Low → tech debt.

The report format follows the template in the persona. The report is returned to the main agent as
text; the main agent is what writes it to the file (the Security section of the review). The
subagent itself writes no files and prints no report to the console.
