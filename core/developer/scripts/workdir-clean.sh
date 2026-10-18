#!/usr/bin/env bash
# workdir-clean.sh — sweep .pipeline/workdir/ of artefacts nobody reads any more.
#
# Why. Three of the four directories under workdir/ hold the state of one run and
# are superseded the moment it ends: what survives goes into the knowledge base.
# Nothing in the pipeline deletes them, so
# they only grow — a month of ordinary work leaves review reports of 40-80 KB each
# next to a knowledge base a quarter their size, and the directory stops being
# something a human will open.
#
# What it does NOT delete:
#
#   * anything under the knowledge base — that is the half of .pipeline/ that is
#     kept forever and committed. This script never looks outside workdir/.
#   * the journal (workdir/metrics/pipeline.jsonl). It is one append-only file
#     measuring the process across many runs, not a per-run artefact: deleting it
#     by age would silently truncate every cost and eval report. Split it per run
#     (see core/developer/references/pipeline-evals.md) if it grows unwieldy.
#   * the artefacts of a run that has not closed. A run whose `done` line is not in
#     the journal may still be resumed in another session, so its files are kept
#     regardless of age — see --force to override.
#
# Usage:
#   workdir-clean.sh [--workdir DIR] [--days N] [--force] [--dry-run]
#
#   --workdir  default .pipeline/workdir
#   --days     delete files last modified more than N days ago; default 7
#   --force    sweep by age alone, ignoring whether a run has closed
#   --dry-run  list what would go, delete nothing
#
# Exit status is 0 unless the workdir is unreadable: this is housekeeping, and a
# sweep that found nothing is a normal outcome, not a failure.

set -euo pipefail

WORKDIR=".pipeline/workdir"
DAYS=7
DRY=0
FORCE=0

while [ $# -gt 0 ]; do
  case "$1" in
    --workdir) WORKDIR="$2"; shift 2 ;;
    --days)    DAYS="$2"; shift 2 ;;
    --dry-run) DRY=1; shift ;;
    --force)   FORCE=1; shift ;;
    -h|--help) sed -n '2,32p' "$0" | sed 's|^# \{0,1\}||'; exit 0 ;;
    *) echo "workdir-clean: unknown argument: $1" >&2; exit 2 ;;
  esac
done

case "$DAYS" in
  ''|*[!0-9]*) echo "workdir-clean: --days needs a whole number, got: $DAYS" >&2; exit 2 ;;
esac

if [ ! -d "$WORKDIR" ]; then
  echo "workdir-clean: $WORKDIR does not exist — nothing to sweep"
  exit 0
fi

JOURNAL="$WORKDIR/metrics/pipeline.jsonl"

# Runs still open, by run number — the journal's `slice` field. A file whose name
# carries one of these is kept whatever its age: the run may be resumed in another
# session, and step 7a's cross-check and the rule registry are read on resume.
# `done` closes a run; everything seen without a `done` is open.
open_runs=""
if [ "$FORCE" -eq 0 ] && [ -f "$JOURNAL" ]; then
  open_runs=$(
    awk '
      match($0, /"slice"[[:space:]]*:[[:space:]]*"?[0-9]+"?/) {
        s = substr($0, RSTART, RLENGTH); gsub(/[^0-9]/, "", s)
        if ($0 ~ /"event"[[:space:]]*:[[:space:]]*"done"/) closed[s] = 1
        else seen[s] = 1
      }
      END { for (r in seen) if (!(r in closed)) print r }
    ' "$JOURNAL"
  )
fi

kept_open=0
swept=0
bytes=0

# -mtime +N is "more than N*24h ago", which is the reading --days advertises.
while IFS= read -r f; do
  [ -n "$f" ] || continue
  case "$f" in
    "$JOURNAL") continue ;;
  esac
  base=$(basename "$f")
  skip=0
  for run in $open_runs; do
    case "$base" in
      *"$run"*) skip=1; break ;;
    esac
  done
  if [ "$skip" -eq 1 ]; then
    kept_open=$((kept_open + 1))
    continue
  fi
  size=$(wc -c < "$f" 2>/dev/null || echo 0)
  bytes=$((bytes + size))
  swept=$((swept + 1))
  if [ "$DRY" -eq 1 ]; then
    echo "would delete  $f"
  else
    rm -f "$f"
  fi
done <<< "$(find "$WORKDIR" -type f -mtime "+$DAYS" 2>/dev/null)"

# Directories the sweep emptied are noise of their own.
[ "$DRY" -eq 1 ] || find "$WORKDIR" -mindepth 1 -type d -empty -delete 2>/dev/null || true

verb="deleted"
[ "$DRY" -eq 1 ] && verb="would delete"
echo "workdir-clean: $verb $swept file(s), $((bytes / 1024)) KB, older than $DAYS day(s) in $WORKDIR"
if [ "$kept_open" -gt 0 ]; then
  echo "workdir-clean: kept $kept_open file(s) belonging to runs with no 'done' line (--force to sweep anyway)"
fi
