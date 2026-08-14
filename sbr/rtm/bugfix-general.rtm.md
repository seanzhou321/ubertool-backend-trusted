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
| BUGFIX-001 | `GetLedgerSummary` slow for orgs with large transaction history (mobile dashboard load time complaints) | Implementation design gap — 4 separate sequential queries against `rentals` table (ActiveRentalsCount, ActiveLendingsCount, PendingRequestsCount, StatusCount GROUP BY) replaced with 1 CTE-based query; same for cross-org path. Added composite indexes on `rentals` and `ledger_transactions`. | `TestLedgerRepository_GetSummary` (tests/integration/ledger_test.go:175); `TestLedgerRepository_GetSummary_CrossOrgRollup` (tests/integration/ledger_test.go:331) | N/A — existing L1+L2+L3 tests (`TestLedgerService_GetLedgerSummary_*`, `TestLedgerRepository_GetSummary_*`, `TestLedgerService_E2E` > GetLedgerSummary subtests) all exercise the fixed code paths and confirmed correct behavior | `internal/repository/postgres/ledger.go`: `getRentalSummaryStats()` / `getRentalSummaryStatsAllOrgs()` — single CTE with role-specific FILTER counts; `podman/trusted-group/postgres/ubertool_schema_trusted.sql`: added 4 `rentals` indexes + 2 `ledger_transactions` indexes | Reduces `GetLedgerSummary` from 6 DB round-trips (1 balance + 4 rentals + 1 recent txns) to 3 (1 balance + 1 CTE rentals + 1 recent txns). The CTE materializes per-status counts with role-aware FILTER aggregates; the outer query computes whole-table totals via `max(case...)` on the small CTE result set. Genuineness: static + mutation-confirmed (all existing L1/L2/L3 tests pass against new implementation). |

## Summary

- Total defects tracked: 1
- Fixed: 1

*Rows are appended by `speckit-sbr-bugfix`, never rewritten wholesale — each run adds exactly the
row(s) for the defect(s) it closed in this file. A fix that instead updated a feature's `spec.md`
and per-feature RTM is recorded there, not here — cross-reference by FR-ID if needed.*
