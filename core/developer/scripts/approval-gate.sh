#!/usr/bin/env bash
# approval-gate.sh — the second machine gate: no production code lands before the
# approvals the route requires have been given.
#
# Why. Until this script the pipeline had exactly one machine gate — build and lint
# before a commit — and prohibition 3 ("write production code before every
# approval") rested entirely on the model obeying text written forty thousand tokens
# earlier. That is the pipeline's most expensive failure: code written against an
# unapproved spec is not a wasted step, it is a changeset whose every later step (tests,
# review, docs) was spent on the wrong thing.
#
# What it actually enforces. The journal already records which route is running and
# which steps it plans (`start`), so the approvals a run owes are derivable rather
# than declared twice. This script reads them back and compares them against the
# staged diff. It is a **second query to a subsystem that exists**, not a new one.
#
# Honest limits, because a gate whose reach is overstated is worse than none:
#
#   * It fires at the commit, not at the write. Code can still be *written* before an
#     approval; it cannot be *committed*. That is deliberate and matches the build
#     gate: the incremental cycle of step 8 must not trip over intermediate states.
#   * It rests on the `start` line. A run that never wrote one is not recognised as a
#     pipeline run and is let through — but such a run is already broken, and visibly
#     so, in the journal. Blocking instead would block every ordinary hand-made
#     commit in the project, which is not this script's business.
#   * "Production code" is decided by an artefact allowlist (below), not by knowing
#     the stack. It is deliberately permissive: a false block costs a developer their
#     commit, a false pass costs one changeset of a gate that is a backstop anyway.
#
# Stack-agnostic on purpose — it lives in core/, not in a profile: what counts as an
# approval is a property of the route, and the route is the same on every stack.
#
# Exit codes: 0 — nothing to block (or the gate could not run, said out loud);
# 2 — blocked, with the reason on stderr.
#
# Run from the root of the project being developed. Dependencies: git, jq.
#
# Set TRUNKLINE_APPROVAL_GATE=off to disable it for one commit. That is a decision
# to record in the run report, not a shortcut: the journal will show a commit whose
# approvals were never logged, and nobody will remember why.

set -uo pipefail

[ "${TRUNKLINE_APPROVAL_GATE:-on}" = "off" ] && exit 0

say() { printf 'approval-gate: %s\n' "$1" >&2; }

# --- Where the journal is ------------------------------------------------------
#
# The `metrics` flag owns this path. Reading it rather than hardcoding the default
# is the difference between a gate that follows the project's configuration and one
# that silently checks a file nobody writes.

JOURNAL="${TRUNKLINE_METRICS:-}"
if [ -z "$JOURNAL" ] && [ -f AGENTS.md ]; then
  JOURNAL=$(sed -n 's/^[[:space:]]*-[[:space:]]*metrics:[[:space:]]*\([^[:space:]#]*\).*/\1/p' \
    AGENTS.md | head -1)
fi
JOURNAL="${JOURNAL:-.pipeline/workdir/metrics/pipeline.jsonl}"

if [ "$JOURNAL" = "none" ]; then
  say "the metrics flag is none, so there is no journal to read approvals from;
  this gate is not enforcing anything on this project (see core/developer/ENFORCEMENT.md)"
  exit 0
fi
[ -f "$JOURNAL" ] || exit 0

command -v jq >/dev/null 2>&1 || {
  say "jq not found — the approval gate cannot read the journal and is NOT enforcing
  anything on this commit. Install jq, or accept that prohibition 3 is textual here."
  exit 0
}
command -v git >/dev/null 2>&1 || exit 0

# --- Which approvals this run still owes ---------------------------------------
#
# One pass over the journal, in order. `start` opens a run and resets the tally,
# `done` closes it, `approval` records one. Only the run that is still open counts:
# a commit made after step 20 owes nothing.
#
# Steps 5, 6, 7 and 7a are the four MANDATORY APPROVALS of the core. Which of them
# apply is decided by `steps_planned` and the `skipped` events, and by nothing else
# — that is the same rule the core states, so the gate cannot invent a fifth
# approval or forgive a fourth.
#
# `skipped` is what makes conditional gates work, and without it this gate is a
# deadlock rather than a check. Routes carry conditional approvals — step 6 on
# `feature`, steps 6/7/7a on `migration` — and whether they apply is settled during
# the run, long after `start` was written: a feature that turns out to touch no
# database never reaches step 6. `steps_planned` names what the route intended, so
# such a step is in it; the `skipped` event is what retires it from what the run
# owes. Drop this branch and the commonest run in the pipeline arrives at the commit
# owing an approval for a step that never happened, with no honest way past — the
# only exit left is to log an approval nobody gave, which is precisely the failure
# this gate exists to prevent.
#
# **Approvals are counted per changeset, and this is what the gate got wrong before.**
# Every gate step — 5, 6, 7, 7a — lives inside the changeset loop and runs once per
# changeset; step 18 commits once per changeset. Tallying approvals per *run* meant
# changeset 1 paid for everyone: once its four lines were in, every later commit of
# the task passed the gate owing nothing, which is the majority of the commits on any
# task big enough to be decomposed. So `approval` and `skipped` carry the changeset
# they belong to, and the tally is kept per changeset.
#
# Which changeset is being committed is read off the journal rather than declared:
# it is the highest changeset any gate event of the open run has named. That works
# because the commit is the *last* thing a changeset does — by the time step 18 runs,
# that changeset's own approvals are already logged, and they are what moves the
# counter. A missing `changeset` field reads as 1, so the routes with no step 4
# (`bug`, `migration`, `config/ci`) have exactly one changeset and behave as before,
# as do journals written before this field existed.

STATE=$(jq -nr '
  def gates: ["5","6","7","7a"];
  def fresh: {open:false, slice:null, steps:[], ok:{}, skip:{}, cs:1};
  def cs_of($e): ($e.changeset // 1) | tostring;
  reduce inputs as $e (fresh;
      if $e.event == "start" then
        {open:true, slice:($e.slice|tostring),
         steps:(($e.steps_planned // []) | map(tostring)),
         ok:{}, skip:{}, cs:1}
      elif (.open|not) or (($e.slice|tostring) != .slice) then .
      elif $e.event == "done" then fresh
      elif $e.event == "approval" then
        cs_of($e) as $c
        | .ok[$c] = ((.ok[$c] // []) + [$e.step|tostring])
        | .cs = ([.cs, ($e.changeset // 1)] | max)
      elif $e.event == "skipped" then
        cs_of($e) as $c
        | .skip[$c] = ((.skip[$c] // []) + [$e.step|tostring])
        | .cs = ([.cs, ($e.changeset // 1)] | max)
      else . end)
  | if (.open|not) then ""
    else (.cs|tostring) as $c
      | (([.steps[] | select(. as $s | gates | index($s))]
          - (.skip[$c] // []) - (.ok[$c] // [])) | join(" ")) as $missing
      | if $missing == "" then "" else "\($c)\t\($missing)" end
    end
' "$JOURNAL" 2>/dev/null) || exit 0

[ -z "$STATE" ] && exit 0
CHANGESET=${STATE%%	*}
MISSING=${STATE#*	}

# --- Is there production code in the diff --------------------------------------
#
# Everything the pipeline itself produces before the code — specs, schemas, test
# descriptions, diagrams, the registry, the cross-check — is markdown, YAML, JSON or
# SQL, and the state it keeps lives under .pipeline/. Anything else staged is the
# thing the approvals are supposed to precede.
#
# A project whose contract is .proto, or whose migrations are code, overrides the
# pattern rather than losing the gate:
#   TRUNKLINE_ARTEFACT_RE='(^|/)\.(pipeline|trunkline|claude)/|\.(md|ya?ml|json|sql|proto)$'

ARTEFACT_RE="${TRUNKLINE_ARTEFACT_RE:-(^|/)\.(pipeline|trunkline|claude)/|\.(md|ya?ml|json|sql)$}"

CODE=$(git diff --cached --name-only --diff-filter=ACMR 2>/dev/null \
  | grep -Ev "$ARTEFACT_RE" || true)

[ -z "$CODE" ] && exit 0

{
  printf 'Commit blocked: production code before an approval the route requires.\n\n'
  printf 'Changeset %s of this run has no approval recorded in %s for step(s): %s\n' \
    "$CHANGESET" "$JOURNAL" "$MISSING"
  printf '  5 = YAML specs + sequence diagrams   6 = SQL schema, queries, migrations\n'
  printf '  7 = the test description             7a = the artefact cross-check\n'
  printf 'Approvals belong to a changeset, not to the task: every one of these steps runs\n'
  printf 'once per changeset, so changeset %s owes its own, whatever earlier ones recorded.\n\n' \
    "$CHANGESET"
  printf 'Production code staged:\n'
  printf '%s\n' "$CODE" | sed 's/^/  /'
  printf '\nThree honest ways out, and one that is not:\n'
  printf '  * the approval happened — write its line to the journal, then commit:\n'
  printf '      {"ts":"<iso8601>","slice":"<NNN>","changeset":%s,"step":"<N>","event":"approval"}\n' \
    "$CHANGESET"
  printf '  * the step does not apply to this changeset — record that, with the reason:\n'
  printf '      {"ts":"<iso8601>","slice":"<NNN>","changeset":%s,"step":"<N>","event":"skipped","reason":"..."}\n' \
    "$CHANGESET"
  printf '  * it did not happen — go back to that step and get it. This is prohibition 3.\n'
  printf '  * do NOT log an approval that was never given: the journal is the only\n'
  printf '    record that these gates ran at all.\n'
} >&2
exit 2
