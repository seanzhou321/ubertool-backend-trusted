---
name: sbr-chat-constitution
description: Conversational, interview-driven skill for creating or updating this project's spec-kit constitution (.specify/memory/constitution.md). Use whenever the user invokes /skill:sbr-chat-constitution explicitly, wants to establish project governing principles from scratch, or wants to review/extend an existing constitution as the project enters new domains, tech, or compliance territory. Replaces raw /speckit.constitution for this purpose — draws out non-functional requirements, business/operational/security constraints, and regulatory mandates through a structured, one-question-at-a-time interview rather than requiring the user to already have a fully-formed dense paragraph of principles. Explicit invocation only — this skill does not trigger organically.
---

# sbr-chat-constitution

A conversational front-end to `/speckit.constitution`. Where `/speckit.constitution`
expects a single dense argument describing project principles, this skill
*earns* that argument through a structured interview — classifying the
project against a fixed taxonomy, checking for existing presets before
inventing anything, then interviewing only on genuine gaps.

This skill never writes `.specify/memory/constitution.md` directly. Its final
step is always a handoff: synthesize the interview into a structured
argument and invoke `/speckit.constitution` with it. `/speckit.constitution`
remains the single source of truth for semantic-versioning bumps, sync-impact
reporting, and propagation to dependent templates.

## Reference files

- `references/taxonomy.md` — the fixed-but-extensible classification enums
  (domain, app_type, tech_stack, trigger_mode, client_surface,
  financial_data_handling, source_authority, category). Read this before
  Phase A.
- `references/item-library.md` — the seeded, progressively-enriched library
  of candidate constitutional items, each tagged against the taxonomy. Read
  this before Phase B.

Both files grow over time. When this skill resolves an item with no match in
either file, and no external preset covers it, that's a candidate to add to
`item-library.md` (or, for a genuinely new classification, `taxonomy.md`) —
propose the addition to the user, don't add it silently.

## Overall flow

```
Detect mode (create vs. review/update)
        |
Phase A — Triage
        |
Preset check (per in-scope item)
        |
Phase B — Gap interview (create: full; review: gaps + flagged items only)
        |
Synthesize → handoff to /speckit.constitution
```

---

## Step 0 — Detect mode

Check whether `.specify/memory/constitution.md` exists and is populated
(not just the placeholder template with unfilled `[PLACEHOLDER]` tokens).

- **Does not exist, or is unfilled template** → **Create mode**.
- **Exists and is populated** → **Review/update/extend mode**.

In review mode, read the existing file fully before Phase A. If the
project's domain/app-type/tech-stack profile is already inferable from the
existing constitution's content (e.g. it already mentions gRPC, mobile
clients, etc.), skip re-asking those specific triage questions in Phase A —
confirm the inferred profile with the user in one line instead of
re-interviewing from scratch.

---

## Phase A — Triage

Goal: classify the project against `references/taxonomy.md`'s dimensions.
This determines which branches of `references/item-library.md` are even in
scope — skip categories entirely rather than asking about all of them.

Ask about each dimension in turn, **one at a time**, not as a single
multi-part questionnaire dump:

1. `domain` — what does the product do for its users?
2. `app_type` — what shape is the application?
3. `tech_stack` — what are the core technical building blocks? (multi-select)
4. `trigger_mode` — how is it invoked? (note: a project can have more than
   one — e.g. primarily `request-driven` with a `cron-scheduled`
   sub-workflow. Capture all that apply.)
5. `client_surface` — who/what calls it?
6. `financial_data_handling` — does it touch money or money-adjacent data
   in any way, even without processing payments?

**If the user's answer to any dimension doesn't cleanly fit an existing
enum value:**
- Draft a candidate new value with a one-line rationale (why the existing
  values don't fit).
- Confirm with the user before treating it as adopted.
- If confirmed, add it to `references/taxonomy.md` under that dimension's
  changelog, with rationale and date.
- Do not silently drop the distinction and do not silently force-fit an
  imperfect existing value.

**If an item or classification doesn't cleanly fit the fixed `category`
enum** during Phase B, the same rule applies — surface it, propose a
resolution (new category value, or note it as a cross-cutting item spanning
multiple categories), confirm with the user. Never silently drop a real
constraint just because it's awkward to categorize.

Record the resolved triage profile at the top of the working session (it
feeds both the preset check and the synthesis step).

---

## Phase B — Preset check + gap interview

For each dimension value resolved in Phase A, walk `references/item-library.md`
and collect every item whose `applies_when` matches (including `any`-tagged
items).

For each candidate item, **before interviewing the user on it**:

### B.1 — Check for an existing preset

1. Try `specify preset search <keywords derived from the item>` via the
   `specify` CLI, if available in the environment.
2. If the CLI is not available, or returns nothing relevant, fall back to a
   live web search against spec-kit's preset catalog / GitHub
   (`github.com/github/spec-kit/tree/main/presets`, `specify preset add`
   ecosystem, community catalogs).
3. If a matching preset exists: present it to the user as an option
   ("there's an existing preset that covers this — install it via
   `specify preset add <name>`, or would you rather define this item
   yourselves?") rather than interviewing from scratch. Record which path
   was taken.
4. If no matching preset exists: proceed to B.2.

### B.2 — Interview

One question at a time, Socratic style — dig into the *why* behind an
answer, not just capture a checkbox. Keep a running scratchpad of resolved
items as you go (item id → resolved value/decision → rationale), so nothing
has to be re-asked mid-session.

For `hard: true` items already seeded with a concrete `prompt_fragment`
(e.g. `guard-no-unprompted-architecture`, the `bugfix-*` gate items),
confirm applicability and any project-specific parameters (thresholds,
window lengths) rather than re-deriving the principle from nothing — the
seed content is a starting point, not a fill-in-the-blank template to
recite verbatim.

For `soft: true` items, actually probe whether the default fits this
project or should be overridden, and record the reasoning either way.

**Ask-don't-assume discipline**: if applicability of an item is genuinely
ambiguous after Phase A's profile, ask rather than silently including or
excluding it.

**In review/update mode**: don't re-run B.2 for items already present and
unflagged in the existing constitution. Diff the in-scope item set against
what's already there:
- In-scope item, missing from existing constitution → interview (gap).
- In-scope item, present in existing constitution → skip, unless the user
  flagged it for revisit.
- Item present in existing constitution but no longer in-scope per current
  Phase A profile → surface as a question ("this project's profile no
  longer suggests X — still needed, or should it be removed/updated?"),
  don't silently delete.

---

## Synthesis and handoff

Once Phase B is complete for all in-scope items:

1. Group resolved items by `category`, matching `/speckit.constitution`'s
   expected article/section structure.
2. For each item, write a concrete principle statement (not the raw
   `prompt_fragment` placeholder — the actual resolved value/threshold/
   decision from the interview).
3. Note `source_authority` inline where it affects who can grant a future
   exception (e.g. "security-derived; exceptions require security sign-off").
4. Present the full synthesized draft to the user for confirmation before
   handoff — this is the last checkpoint before it becomes binding.
5. On confirmation, invoke `/speckit.constitution` with the synthesized
   content as its argument. Let `/speckit.constitution` handle versioning,
   sync-impact reporting, and template propagation — this skill's
   responsibility ends at handoff.
6. After `/speckit.constitution` completes, note any items resolved in this
   session that had no existing preset match — these are candidates to add
   to `references/item-library.md` for future runs. Propose the addition;
   don't add silently.

---

## Non-goals

- Does not write `.specify/memory/constitution.md` directly.
- Does not duplicate `/speckit.constitution`'s versioning or sync-impact
  logic.
- Does not validate the resulting constitution's internal consistency —
  that's `/speckit.analyze`'s job downstream.
- Does not decompose or replace `sbr-bugfix` — the `bugfix` category items
  in `references/item-library.md` are constitutional gates referencing that
  skill's workflow, not a reimplementation of it.
- Does not make legal/regulatory determinations. If a `compliance-regulatory`
  item is genuinely unclear, say so explicitly and recommend the user
  confirm with appropriate counsel or a compliance function — do not guess.
