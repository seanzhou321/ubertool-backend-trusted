# Bug-fix RTM — Corner-Case Defects (Appendix B)

- **Source**: no single `spec.md` — this RTM tracks defects fixed via
  `.claude/skills/speckit-sbr-bugfix/SKILL.md` whose root cause is a pure architecture/
  implementation gap against an already-correct requirement, or a missing-requirement gap too
  narrow/corner-case to generalize into a new formal `FR-XXX`. Generalizable missing-requirement
  fixes go to the owning feature's `spec.md` + `sbr/rtm/<feature-slug>.rtm.md` instead — see
  `sbr/README.md` → "Bug-fix traceability: spec update vs. bug-fix RTM" for the routing rule that
  decides which home a given fix belongs in.
- **Schema**: one row per fixed defect, added only once the fix is verified (unit + integration +
  e2e suites green, per the adapter's run commands). This file is a closed-defect log, not a
  coverage audit — there are no `Gap`/`Unclassified` rows here; a defect isn't listed until it's
  fixed.
- **Tier mapping in effect**: `sbr/README.md` → "This repo's adapter" (L1=`tests/unit`,
  L2=`tests/integration`, L3=`tests/e2e`).
- **Bug-ID convention**: `BUGFIX-NNN`, assigned sequentially in the order fixes land here.

## Defect log

| Bug-ID | Reported As | Root-Cause Category | Reproduction Test | Root-Cause-Isolating Test | Fix | Notes |
|---|---|---|---|---|---|---|
| _(none yet)_ | | | | | | |

## Summary

- Total defects tracked: 0
- Fixed: 0

*Rows are appended by `speckit-sbr-bugfix`, never rewritten wholesale — each run adds exactly the
row(s) for the defect(s) it closed in this file. A fix that instead updated a feature's `spec.md`
and per-feature RTM is recorded there, not here — cross-reference by FR-ID if needed.*
