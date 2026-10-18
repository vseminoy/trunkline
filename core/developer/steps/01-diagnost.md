# Step 1. Diagnostician agent `[STOP]`

**Skills:** `agent-skills/skills/context-engineering/SKILL.md`, `agent-skills/skills/using-agent-skills/SKILL.md`,
`core/skills/project-knowledge/SKILL.md` (the knowledge base format — read it before 1.1)

Gathers the project context and states its assumptions.

## 1.1 Delta context AS THE VERY FIRST ACTION (mandatory)

Before reading any source — read the accumulated knowledge about the code:

1. `.pipeline/knowledge/INDEX.md` — which modules are already described, by which slice, and when.
   It sits at the base's **root** and covers every project in it; read this project's rows (the
   `Project` column matches the `project` flag) and ignore the rest.
2. `.pipeline/knowledge/<project>/modules/<module>.md` — AS-IS for every affected module.
3. `.pipeline/knowledge/<project>/slices/` — the last 3 entries for those same modules (ADDED / MODIFIED /
   REMOVED).

Rules of use:
- Anything already recorded is **not rebuilt from scratch** — it is taken as the starting AS-IS.
- An entry is not gospel: before relying on a specific fact (a signature, a file name, the presence
  of a column), confirm it in the code with one targeted read. A discrepancy → mark it `STALE:` in
  the report and fix the entry at step 20.
- Two different situations, and they end differently at step 20 — do not merge them:
  - **`knowledge: none`** — the project keeps no base by decision. Say so, gather AS-IS in full, and
    expect **no** slice entry and **no** bootstrap at step 20. Setting the store up anyway would
    override a flag silently.
  - **the flag names a path, but the directory is not there** — the base has simply not been started
    yet. Say so, gather AS-IS in full; step 20 creates the structure (`bootstrap`).

The report must contain this block:
```
DELTA CONTEXT:
- Reused from .pipeline/knowledge: [module → what was taken]
- Rebuilt: [what, and why it was not in the base]
- STALE (the entry has drifted from the code): [exactly what]
```

## 1.2 Gathering the missing context

Always request (whatever the delta context does not cover): the code, the contract, the DB schema and
queries, the project structure, `AGENTS.md` and the rest of the profile's
`required_project_files`.

**For large projects** — use an isolated `Explore` sub-agent (Pattern 5 from
`agent-skills/references/orchestration-patterns.md`): it reads the files and returns only a digest.

**For hard bugs with 3+ competing hypotheses** — propose Agent Teams:
```
Spawn teammates: code-reviewer, security-auditor, test-engineer
Mode: adversarial debate
Requires: CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1 in ~/.claude/settings.json
```

**A limitation of teammate mode** (must be accounted for — `agent-skills/references/orchestration-patterns.md`,
"Subagents vs. Agent Teams")**:** for a
persona launched as a teammate, the `skills` and `mcpServers` frontmatter fields are **ignored** —
the skills come from the project and user settings, as in an ordinary session. So the skills that
steps 7/9/10 rest on (`test-driven-development`, `code-review-and-quality`,
`doubt-driven-development`, `security-and-hardening`) will not be picked up by themselves in this
mode. Before running the command either:
- enable those skills at the session or project level (`.claude/settings.json`), or
- list the required skills directly in the task text given to the teammates.

As subagents (step 9+10) the same personas get their `skills` normally — the limitation applies only
to teammate mode.

## 1.3 The task's requirements (mandatory)

**On `audit` this subsection is replaced by naming the scope.** That route has no task text to read
requirements out of: ask once what to review — a commit range, a branch, a module path — and say what
you settled on ("the project" is not a scope). If the code in scope came from an earlier run of this
pipeline, its registry (`.pipeline/workdir/rules/<NNN>.md`) is what step 7a will cross-check against, so name
whether one exists here rather than discovering it there. Everything else on this step is unchanged:
the delta context of 1.1 is what makes the review cheap, and the assumptions of 1.4 are what it rests
on.

**On `knowledge` the scope is a module and the requirements come from the user, not from a tracker.**
Step 21.1 names the module; here, gather what that route's step 21.2 asks the user for — the
documents or the plain-text description of the module — and treat it as the task text: verbatim, one
item per requirement, because step 2 copies exactly this into the registry and that is what makes a
document's claim checkable against the code. "There is nothing" is a complete answer and is recorded
as one: the run then compares the entry against the code alone. The delta context of 1.1 is not
optional on this route — it is the artefact under test.

Whatever else the task carries, it carries what it must do. Gather that here, verbatim — step 2 turns
it into rules, and step 7a counts the items to prove nothing was dropped between the two.

**The task text carries requirements, not identifiers.** Its "How to test" block holds them in EARS,
one item per requirement. Where the task came from the analytic track (`core/analytic/`) the analyst numbered
them too, but those numbers are working notes of their epic and stop at its edge; the identifiers
that end up in the code are assigned at step 2 of this pipeline (see "Rule traceability" in the
core). So do not go looking for codes — gather statements.

A second source exists and is worth reaching for when it is available:

- **The analysis state** — the "Requirements (EARS) and coverage" table, which holds the epic's whole
  picture: every requirement and the task it was anchored to. The analyst writes this file on every
  step, so it survives the session that produced it. Canonical location: the profile's
  `tasks.analysis_dir`, one file per epic (see `core/analytic/skills/analysis-state/SKILL.md`).
- The decomposition summary report holds the same table, but it is a view the analyst showed in a
  reply; it exists as a file only if someone asked for one. Use it if you are handed it — do not go
  looking for it.

Order — **read, then look, and only then ask**:
1. **Take the requirements out of the task text**, verbatim, one line per "How to test" item. The
   EARS line *is* the statement. This costs nothing and needs no file.
2. **Then derive the analysis state and read it if it is there**: `<tasks.analysis_dir>/<EPIC>.md`,
   the directory from the profile manifest and the epic from the task at hand. It adds what one
   task's text cannot — which requirements went to the epic's *other* tasks, so the boundary of this
   task is visible rather than assumed.
3. **Not there, and the boundary matters** → one question to the user: does this task have an
   analysis, and where. One question, not an investigation: no answer — move on.
4. **The task text has no requirements either** — a one-line bug report, a hand-written task — say so
   explicitly: the rules will be derived at step 2 from the code and the diagnosis, and there is
   nothing to check them against but the user. That is a normal mode, not a blocker.

Reading before asking is not a convenience. The question lands at the one seam where someone else's
work is handed over, and a user who does not know the answer — a different person, or the same one
months later — makes "no analysis" the default reply. The requirements then survive only as whatever
the developer happened to notice.

The block for the report:
```
REQUIREMENTS:
- Source: the task text (N items) | + the analysis state at <path> | none
- Read: N requirements, verbatim — they become rules at step 2, identifiers assigned there
- Not covered by this task: the epic's other tasks carry the rest — only where the state was read
```

## 1.4 ASSUMPTIONS (mandatory block)

```
ASSUMPTIONS:
1. [assumption]
→ Confirm or correct these before I continue.
```

**Output:** the delta context is applied, the missing context is gathered, the source of the
requirements is established, and the ASSUMPTIONS are agreed.
