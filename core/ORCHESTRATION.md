# Orchestration — what the pipeline requires, what it merely uses

The pipeline is text an agent reads. Everything else — subagents, slash
commands, hooks, parallel tool calls — is a **capability of the host**, and the
pipeline must survive its absence. This file draws that line: what a host must
be able to do, what it may additionally offer, and exactly what degrades when it
cannot.

A host that only reads files and edits code runs the full route. It runs it
slower and with weaker enforcement, and both are stated out loud rather than
quietly skipped.

---

## The baseline — required of every host

A host qualifies if it can do these five things. Nothing in the core assumes
more.

| # | Requirement | Used for |
|---|---|---|
| B1 | Read files the user points it at | core, profile manifest, step files, guidelines |
| B2 | Write and edit files in the project | code, specs, migrations, `.pipeline/` |
| B3 | Run shell commands and read their output | `commands.build`, `commands.lint`, `commands.test` |
| B4 | Stop mid-task and wait for a human reply | every `[STOP]` gate |
| B5 | Keep one conversation's context across steps | the route is sequential and cumulative |

B4 is the one that is not negotiable. A host that cannot pause and hand control
back — a batch runner, a one-shot completion — cannot run this pipeline, because
every approval gate collapses into the agent approving itself. Do not "adapt" it
by removing the gates; use a different host.

---

## Capabilities — optional, and what happens without them

Each capability names where the pipeline uses it and what the run looks like
when the host lacks it. **Absence is never silent**: the agent announces the
degradation in the opening announcement, alongside the profile and the flags.

### C1. Subagents

*Used at:* step 9+10 (Critic and Security Auditor in parallel), step 1 (isolated
reading of large code volumes), step 1 (competing hypotheses, by agreement with
the user).

*Without it:* the main agent plays both roles sequentially in one context. The
findings are the same; the review is weaker, because a fresh context is what
makes the Critic disagree with the author. Say so, and consider a second pass on
a clean session for high-risk changesets.

*Note:* real parallelism, where it exists, requires several spawn calls in a
**single** assistant turn. Spawns in consecutive turns are sequential and buy
nothing but hops.

### C2. Parallel tool calls

*Used at:* reading several artifacts at once (steps 3, 7a, 17).

*Without it:* reads are sequential. Slower, no behavioural difference.

### C3. Hooks / interceptors

*Used at:* the commit gate (`core/developer/ENFORCEMENT.md`), documentation caching for step 8,
protection of performance-critical blocks from step 11.

*Without it:* install the commit gate as a plain git hook instead — same script,
different trigger (`trunkline init --gate=git`). Doc caching and simplify-ignore
have no replacement; the run costs more tokens and step 11 must be told by hand
which blocks not to touch.

The commit gate is worth this trouble. Of the sixteen `[STOP]` gates in this
pipeline, none is enforced by a machine — they rest on the model. What a machine
does enforce fires here, at the commit: the build and lint gate, and the approval
gate riding on it (`core/developer/ENFORCEMENT.md`). Lose this capability without
installing the git-hook equivalent and the pipeline's enforcement becomes social
in its entirety.

### C4. Native slash commands

*Used at:* starting a run.

*Without it:* paste the contents of `.trunkline/AGENTS.md`, or say "follow
`.trunkline/AGENTS.md`". That file exists exactly so the pipeline has an entry
point that needs no host feature at all.

### C5. Fetching documentation

*Used at:* step 8, to check API behaviour against current docs rather than
recalling it.

*Without it:* prohibition 4 ("never assume API behaviour without checking the
docs") becomes the user's job — the agent must ask for the relevant page instead
of guessing, and must say that it is asking because it cannot fetch.

### C6. Persistent per-project agent instructions

*Used at:* the `## Pipeline flags` block, which the agent must find on its own.
The block lives in the project's `AGENTS.md` — the cross-host convention — so
this capability is about the host loading that file automatically, not about
which file it is.

*Without it:* pass the flags in the opening message of the run. Do not let the
agent infer them: an unset flag means strict behaviour, and an inferred one
means an invisible disabled check.

---

## Known hosts

| Host | C1 | C2 | C3 | C4 | C5 | C6 | Gate to install | Tried on a real run |
|---|:--:|:--:|:--:|:--:|:--:|:--:|---|---|
| Claude Code | yes | yes | yes | yes | yes | yes | `--gate=claude` | yes |
| Cursor | no | yes | no | yes | yes | yes | `--gate=git` | **not yet** |
| Codex CLI | no | yes | no | no | yes | yes | `--gate=git` | **not yet** |
| Generic / unknown | no | no | no | no | no | no | `--gate=git` | **not yet** |

C6 is "the host loads `AGENTS.md` by itself". Where it is `no`, the file is still
the one place the flags live — the user pastes its flags block into the opening
message instead of the agent finding it.

The capability columns are what the host offers; the last column is what has
been run through the pipeline. They are separate on purpose — the rows marked
"not yet" are derived from the hosts' documented features, not from a route run
end to end on them (see "Verification status").

Any row can run the full route. Only the first gets the pipeline as designed;
the rest trade parallel review and machine enforcement for portability, and the
trade is stated in the opening announcement, not discovered at step 18.

The capability set is declared, not detected. Put it in the flags block when it
is not the default:

```markdown
- host: cursor    # claude-code (default) | cursor | codex | generic
```

---

## Verification status

Portability claims are worth exactly what has been tested, so this section says what has been.

| Check | Status |
|---|---|
| The payload is self-contained: every path the vendored core names resolves inside the project | **verified by a gate** in trunkline's own repository, on every commit |
| `.trunkline/AGENTS.md` names the core, the profile and the step files with no host feature | **verified** |
| `--gate=git` installs a hook that blocks a commit on a failing build and allows a passing one | **verified** |
| The gate script serves both triggers (see `core/developer/ENFORCEMENT.md`) | **verified** — the first git-hook install silently gated nothing until it did |
| One real `feature` route run end to end in Cursor or Codex CLI | **not done** |

The first row rests on one structural property, worth stating because everything else about path
resolution follows from it: **the pipeline root is self-similar.** `core/`, `profiles/` and
`agent-skills/` are siblings both in trunkline's repository and under `.trunkline/` in a project, so
a path that resolves against one root resolves against the other. That is why the core writes
`agent-skills/skills/...` once and needs no rewriting at install time — and why the gate checking
those paths in the repository is a real check of the installed layout rather than a proxy for it.
The claim held only after the wording was fixed: the core used to anchor its paths to a root that
only trunkline's own checkout has, which no installed project could resolve against.

The last row is the one that matters most and is the one still open. A structural check cannot see
whether a step still carries enough meaning once C1 and C3 are gone; only a run can. Until it
happens, treat the Cursor and Codex rows in the table above as designed-for, not proven.

## How to add a host

1. Fill a row in the table above: which capabilities exist, which do not.
2. For every "no", check that the degradation named in C1–C6 is acceptable. If
   a `[STOP]` gate would disappear rather than degrade, the host does not
   qualify — see B4.
3. Add the entry point in the host's native form, generated from the same
   source as the others (`internal/install/entrypoints.go`). It must point at
   the core and the profile and state nothing of its own — a second copy of the
   rules is a second thing to keep in sync, and it will not stay in sync.
4. Run one real task end to end on the `feature` route. A structural check
   cannot see whether a step still carries enough meaning after degradation;
   only a run can.
