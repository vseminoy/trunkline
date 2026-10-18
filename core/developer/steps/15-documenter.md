# Step 15. Documenter agent `[STOP]`

**Skills:** `agent-skills/skills/documentation-and-adrs/SKILL.md`

Apply the inline documentation rules and the changelog format from the skill.

Documentation states the current state only — no history of changes, no "was/became" (see
"Documentation describes the present" in the core). Editing an existing README means rewriting the
stale sentence, not appending a note that it changed.

Updates: the module README, architecture diagrams (Mermaid component / flow).
Sequence diagrams were produced at step 5 — do not duplicate them, link to them.
The same goes for the cross-check under `.pipeline/workdir/consistency/` and `tests_readme.md` — link, do
not duplicate.

**`[CONDITIONAL]` ADR — if an irreversible decision was made** (a technology choice, a new pattern,
a non-standard trade-off):
```markdown
# ADR-NNN: [Title]
## Status: Accepted | Proposed | Superseded by ADR-XXX
## Context: [problem, constraints]
## Decision: [what was decided]
## Consequences: Pros / Cons
```

**Where it goes: `<knowledge>/<project>/adr/ADR-NNN-<slug>.md`** — inside this project's directory in
the knowledge base, next to `modules/` and `slices/`, not in the project's `docs/`. An ADR is kept for the life of
the project and read by the same person who reads the slice entry that links to it; splitting the two
across two trees means one of them gets gitignored by accident. `NNN` continues the numbering already
in this project's `adr/` — numbering is per project, not per base, so two services sharing a root
each have their own ADR-001 — read the directory first; if the project has an established ADR location of its own with
records already in it, keep using it and say so, because moving someone's existing history is not
this step's call.

Never delete old ADRs — mark them Superseded. Reference any ADR written here in the slice entry at
step 20.

**An ADR is not the default.** Most decisions of a run belong in the slice entry's `Decisions` and
`Rejected alternatives`, which step 20 writes anyway. Write an ADR only where the decision is
irreversible in practice and its *cost* needs stating — the `Consequences` section is what the slice
entry has no room for. Two or three per module over a project's life is the expected density; a
run that produces one per changeset is using the wrong artefact.

**Output:** the README is current, the diagrams are current, and an ADR is written under
`<knowledge>/<project>/adr/` if one was needed.
