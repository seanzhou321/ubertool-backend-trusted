# Bug-fix RTM — Corner-Case Defects (Appendix B)

- **Source**: no single `spec.md` — this RTM tracks defects fixed via `[BUGFIX_SKILL_PATH]`
  (this project: `.claude/skills/sbr-bugfix/SKILL.md`) whose root cause is a pure
  architecture/implementation gap against an already-correct requirement, or a
  missing-requirement gap too narrow/corner-case to generalize into a new formal `FR-XXX`.
  Generalizable missing-requirement fixes go to the owning feature's `spec.md` + its feature RTM
  instead — see `sbr/README.md` → "Bug-fix traceability: spec update vs. bug-fix RTM" for the
  routing rule that decides which home a given fix belongs in.
- **Schema**: one row per fixed defect, added only once the fix is verified ([LIST THIS PROJECT'S
  TEST TIERS] suites green, per the adapter's run commands). This file is a closed-defect log,
  not a coverage audit — there are no `Gap`/`Unclassified` rows here; a defect isn't listed until
  it's fixed.
- **Tier mapping in effect**: `sbr/README.md` → "This repo's adapter" ([SUMMARIZE THIS PROJECT'S
  L1/L2/L3 → DIRECTORY MAPPING]).
- **Bug-ID convention**: `BUGFIX-NNN`, assigned sequentially in the order fixes land here.

<!--
  ============================================================================
  IMPORTANT: This is the empty-state template for a project's bug-fix RTM — the traceability
  file `sbr-bugfix` appends to for defects that don't warrant a spec.md/FR change.

  When first adopting the sbr-bugfix skill in a project:
  1. Copy this file to sbr/rtm/bugfix-<descriptive-slug>.rtm.md — e.g. bugfix-general.rtm.md
     for a single project-wide log, or a domain-scoped name (e.g. bugfix-<area>.rtm.md) if the
     project later chooses to split it. The file MUST match the glob sbr/rtm/bugfix-*.rtm.md —
     this is how sbr-bugfix discovers it, deliberately independent of the numeric
     001/009-style prefixes used by feature and security RTMs, so it never collides with or
     needs renumbering when a new feature spec is added.
  2. Fill in the bracketed placeholders above from sbr/README.md's adapter section (tier
     mapping, test directories, run commands).
  3. Delete this comment block.

  The sbr-bugfix skill appends rows to the copied file going forward — it never
  regenerates the whole file from this template on a later run, and this template itself is
  never filled in with real defect rows. If sbr/rtm/bugfix-*.rtm.md ever matches more than one
  file, the skill asks which one to use rather than guessing — see SKILL.md Step 8.
  ============================================================================
-->

## Defect log

| Bug-ID | Reported As | Root-Cause Category | Reproduction Test | Root-Cause-Isolating Test | Fix | Notes |
|---|---|---|---|---|---|---|
| _(none yet)_ | | | | | | |

## Summary

- Total defects tracked: 0
- Fixed: 0

*Rows are appended by the bug-fix skill, never rewritten wholesale — each run adds exactly the
row(s) for the defect(s) it closed in this file. A fix that instead updated a feature's `spec.md`
and per-feature RTM is recorded there, not here — cross-reference by FR-ID if needed.*
