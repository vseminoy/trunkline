# core/l10n — localizations of the core

The core is written in English. A project can run it in another language by
declaring one in its flags block:

```markdown
## Pipeline flags
- language: ru
```

`trunkline init --language ru` writes that flag and installs the localized core.

## How it resolves

A localization is an **overlay**, not a fork. `core/l10n/<lang>/` mirrors the layout of `core/`, and
`trunkline` applies it over the English tree at install time:

```
core/analytic/PIPELINE.md                        ← English source
core/l10n/ru/analytic/PIPELINE.md                ← replaces it when language: ru
core/l10n/ru/developer/steps/07a-consistency.md  ← absent → the English file stays
```

Consequences that matter:

- **A partial localization ships.** An untranslated file falls back to English rather than breaking
  a route. A localization is expected to stay partial — see "What earns a translation".
- **Paths never change.** The pipeline resolves one set of paths whatever the language, so nothing
  in the prompts, the profile manifest or the CLI has to know which language is active.
- **A project gets one language.** `core/l10n/` is build material; only the selected localization is
  vendored into `.trunkline/core/`.
- **`update` keeps the project's language.** It reads the flag rather than re-asking. Silently
  reverting a localized project to English is the one thing it must never do.

## What earns a translation

Translating a prompt does not change the language the agent writes in. Both cores say so in their
own text, under "OUTPUT FORMAT":

> These instructions are in English; your output is not. Reply in the language the user writes to
> you in, and write the artefacts […] in the language the project already uses.

A Russian project therefore gets Russian specs, a Russian knowledge base and Russian commit messages
out of the **English** core. That is one paragraph doing the work that translating sixty files was
assumed to do, and it is why the localization is small on purpose.

A file earns a translation when one of two things is true:

- **(a) it is copied verbatim into an artefact** — a template whose headings become the structure of
  a file that a later step reads back;
- **(b) it reasons about the surface form of the target language** — notation that has to be fixed
  in that language, or checks on how a sentence is phrased.

Everything else is procedure — "read the guideline for role X, check Y, roll back to step Z on a
Critical". Procedure does not become more followable in translation; it becomes a second copy to
keep in step, and complex negative constraints (`>` vs `>=` is a mismatch, spawning is forbidden
here) are followed at least as reliably in English.

What that leaves, for `ru`:

| Translated | Which rule | Why |
|---|---|---|
| `analytic/references/ears-notation.md` | b | EARS is notation: `КОГДА` / `ПОКА` / `ЕСЛИ … ТО` / `ГДЕ` / `ДОЛЖНА` have to be fixed, or every run invents its own wording and the registry stops being greppable. It also carries the table that diagnoses badly phrased requirements — a check on Russian surface form. |
| `analytic/PIPELINE.md`, `analytic/steps/*` | b | The requirements registry, the coverage matrix and the "How to test" block are written in Russian, and their wording is what the team reads in the tracker. |
| `analytic/skills/*` | a, b | The task files these produce go to a tracker read by the team. |
| `developer/references/tests-readme-example.md` | a | The file says outright that everything after the separator is copied as a template. |

The whole developer route — every file under `core/developer/steps/`, plus
`core/developer/PIPELINE.md` and `core/developer/ENFORCEMENT.md` — is procedure, falls back to
English, and produces Russian artefacts anyway.

**`skills/project-knowledge/SKILL.md` was translated under rule (a) and no longer is** — the same
rule now argues the other way, and the reversal is worth keeping written down, because the next
candidate for translation will look exactly like it did. Rule (a) says a template earns a
translation because its headings become the structure of an artefact that is read back. Once
`trunkline kb-sync` began reading that structure **by machine** — finding the rule
table by its `## Business rules in force` heading, the drift window by the `Updated:` date — the
headings stopped being an artefact's prose and became protocol, and protocol keeps one spelling.
`## Действующие бизнес-правила` did not produce an error: the script found no table, reported
`0 rule(s)`, printed `ANCHORS CLEAN` and exited 0 over a base it had never read. With the headings
fixed to English, everything left in that translation was prose about how to write — procedure,
which earns nothing.

The generalisation: **rule (a) holds only while the artefact is read back by a model.** The moment
anything parses it, the headings move to the "What is not localized" list below, and what remains of
the file has to earn a translation on its own.

## The part that will bite you

Two copies of the same text always drift. That is not a risk to manage with care — it is a
certainty to catch with a gate.

`core/l10n/<lang>/l10n.lock` records the sha256 of each English source **at the moment its
translation was declared current**. When the English file changes and the translation does not, the
gate fails:

```bash
python3 scripts/lint-l10n.py
#   ERROR   ru/developer/steps/18-git.md: английский исходник изменился после перевода

# after actually redoing the translation:
python3 scripts/lint-l10n.py --update ru
```

Run `--update` **only together with the translation itself**. Running it to make the gate green is
how you get a lock file that certifies a lie, which is worse than no lock file at all: the drift
becomes invisible instead of merely present.

The gate also catches an orphan — a translated file whose English source was renamed or deleted —
and reports the untranslated count as a single line rather than a file per line: that set is the
scope decision above, not a backlog, and a gate that prints thirty reminders of a decision is a gate
people learn to scroll past.

The hash measures drift of the source, not integrity of the translation, so it is paired with a
check on the markers themselves: `[STOP]`, `[CONDITIONAL]`, `ADDED` / `MODIFIED` / `REMOVED` are
counted on both sides and must agree, and a rule identifier (`R-007-2`) must survive into the
translation.

**Markers keep their English spelling in every language.** They are read by the parser and by step
7a, not by a person, and a second spelling means a second parser. `ru` did localize them at first
(`[СТОП]`, `[УСЛОВНЫЙ]`), and it cost exactly what this gate exists to catch: `[УСЛОВНЫЙ]`
declined into `[УСЛОВНЫЕ]` reads perfectly well and parses as nothing at all, in two files, found
on the check's first run. Translate the prose around a marker, never the marker.

## Adding a language

1. `mkdir core/l10n/<lang>` and translate files into it, mirroring `core/`.
2. Translate only what meets rule (a) or (b) above — for `ru` that is the analytic pipeline (its
   steps, its skills and the EARS notation) plus the one file copied verbatim into an artefact, and
   the list is unlikely to differ much for another language. Before translating a template under
   rule (a), check that nothing parses it: if something does, its headings are protocol and the
   translation is a false green waiting to happen. `lint-l10n.py` prints the current count
   on every run; it is not repeated here, because a count in prose is exactly the fact this page
   argues should be left to a gate — it was right when written and went stale the day one core file
   was split into six, without a word of the translation changing. Start with the analytic pipeline:
   EARS notation in the target language is the one thing the English core cannot supply. Do **not**
   work through the step files; they are procedure, and translating them buys a drift surface and
   nothing else.
3. Keep the identifiers, the flag names, the slot names, the file names and the markers (`R-007-2`,
   `[STOP]`, `[CONDITIONAL]`, `ADDED` / `MODIFIED` / `REMOVED`) **machine-identical to what
   the rest of the toolchain expects** — same spelling, English, whatever the language of the prose
   around them. Translate the prose, not the protocol. Step 7a and the journal parse these.
4. `python3 scripts/lint-l10n.py --update <lang>` to record the baseline.
5. `python3 scripts/lint-core-profile-boundary.py` — the boundary rules apply to a localization
   exactly as they do to the English core. No stack vocabulary, whatever the language.
6. Run one real task on the `feature` route in that language. A structural check cannot see whether
   a step still carries enough instruction after translation; only a run can.

## What is not localized

- The developer route in full — see "What earns a translation". It is procedure; it falls back to
  English and still writes its artefacts in the project's language.
- The protocol markers, in any language: `[STOP]`, `[CONDITIONAL]`, `ADDED` / `MODIFIED` /
  `REMOVED`, the rule identifiers (`R-007-2`), the commit trailer `DIDN'T TOUCH:`, the 7a statuses
  (`OK` / `GAP` / `MISMATCH` / `N/A`) and the output header `AGENT [SKILL1, SKILL2]:`.
- The headings and header fields of the knowledge-base templates — `## Business rules in force`,
  `## Invariants and pitfalls`, `Updated:`, `Slice:` — for the same reason: `trunkline kb-sync` parses them.
  `lint-l10n.py` counts the two headings like any other marker. The header fields are not in that
  list because the analysis state has a legitimate `Updated:` of its own, and a check applied to
  every file at once cannot tell the two apart — that one is held by
  `core/skills/project-knowledge/SKILL.md` saying so.
- `profiles/` — the profiles are already whatever language their owners wrote them in.
- The CLI's own output, `--help` and `doctor`'s report.
- Skill front-matter — `name:` and `description:` are copied from the source unchanged. Routing is
  done by the host model reading the description, and it matches across languages; keeping one form
  is what lets the skill linter and the routing evals check the thing that actually ships. `ru` had
  translated all five of them before this was enforced, so `lint-l10n.py` now compares front-matter
  byte for byte rather than trusting the rule to be remembered.
