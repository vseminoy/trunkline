# Step 0. Clarifier agent `[CONDITIONAL]`

**Skills:** `agent-skills/skills/interview-me/SKILL.md`, `agent-skills/skills/idea-refine/SKILL.md`
**Reference:** `core/analytic/references/ears-notation.md`

Runs when the request is vague: no concrete behaviour, no acceptance criteria, unclear boundaries, no
pointer to endpoints or layers.

Apply the process from the skills: one question at a time, until the stopping criterion below is met.
Additionally clarify:
- which layers are affected — against the profile's `layers` list;
- the stack constraints — against the profile's `stack`;
- what counts as "done" (acceptance criteria) for the request as a whole.

### Acceptance criteria — in EARS

The acceptance criteria the Clarifier produces are written **in EARS notation** (see "Requirements
notation — EARS", `core/analytic/references/ears-notation.md`). These are high-level criteria for the whole request — 3–7
requirements; the detailed work is done by the Analyst at step 2.

**EARS here is an interviewing instrument, not a formatting one.** The mechanic: the agent tries to
record its understanding of the request in a template. The place where the template will not fill in is
the next question for the user:

| What will not fill in | The question to ask |
|---|---|
| no subject in `<system> SHALL` | which service/layer is responsible for this behaviour? |
| no response, or an uncheckable one | what exactly does the system return or do? which response code? |
| you cannot choose between `WHEN` and `WHILE` | is this a one-off event or a state that holds over time? |
| there is no `IF … THEN` at all | what should happen if <the external system / validation / a timeout> fails? |
| more than two conditions | is this really one requirement? which conditions can be pulled out separately? |

### Stopping criterion (HARD)

The Clarifier finishes and reaches its `[STOP]` **only when ALL of these hold**:

1. Every acceptance criterion is written in one of the five EARS templates — with no "roughly",
   "generally", "and so on".
2. Every criterion has a **subject** (a concrete system/service/layer) and a **checkable response**.
3. There is **at least one** `IF … THEN` requirement (unwanted behaviour). If there genuinely are no
   error scenarios, that is written as an explicit line with a justification, not skipped silently.
4. The request boundaries are recorded: not only what is in scope but **what is NOT**.
5. No criterion contains a forbidden wording ("handle correctly", "work properly", "works as
   expected").

**Until every item holds, ask the next question rather than moving to step 1.** The "~95% certainty"
formulation is replaced by this list: it is checkable, and a feeling of certainty is not.

If the user declines to answer or there is no answer — **do not guess**: move what is unclosed into
`ASSUMPTIONS` marked explicitly "unconfirmed" and pass it to the Analyst's Open Questions.

**Output:** the request restated with explicit boundaries (including "what is NOT in scope") and
acceptance criteria in EARS that pass the stopping criterion.
