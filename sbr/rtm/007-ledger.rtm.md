# Requirements Traceability Matrix — Ledger

- **Source spec**: `specs/007-ledger/spec.md`
- **Tier mapping in effect**: `sbr/README.md` → "This repo's adapter" (L1=`tests/unit`,
  L2=`tests/integration`, L3=`tests/e2e`, Grounding=`tests/smoke`+`tests/e2e` journeys)
- **Generated**: 2026-07-22 by `speckit-sbr-audit`; updated 2026-07-23 (Phase 4 re-audit)
- **Mode**: retrofit audit (as-built) — see project constitution Principle I

| FR-ID | Requirement Summary | L1 Unit | L2 Integration | L3 E2E | Grounding (Smoke) | Boundary Status | Notes |
|---|---|---|---|---|---|---|---|
| FR-001 | `GetBalance` MUST return the caller's `balance_cents` for the given org (and, as-built, MUST NOT be assumed to populate `last_updated_on`). | `TestLedgerService_GetBalance > "Success"` (`tests/unit/ledger_service_test.go:17`); `TestLedgerRepository_GetBalance > "Success"` (`tests/unit/repos/ledger_test.go:52`) | `TestLedgerRepository_GetBalance` (`tests/integration/ledger_test.go`) — real DB, 2 subtests: returns the caller's `balance_cents` for the given org, errors for a user with no membership in the org | `TestLedgerService_E2E > "GetBalance"` (`tests/e2e/ledger_test.go:23`); re-exercised across a before/after-settlement comparison in `TestLedgerService_E2E > "Ledger Updates After Rental Completion"` (`tests/e2e/ledger_test.go:163-207`) | — | **Complete** | **Fixed 2026-07-23 (SBR remediation Phase 4).** Closed the L2 gap: `ledgerRepo.GetBalance` is now exercised directly against a real Postgres instance (the only prior real-DB ledger integration test, `TestRentalAndLedger_Integration`, never called `GetBalance`). No bug found. `last_updated_on` (Known Discrepancy 1) remains intentionally undocumented-by-test, as spec.md itself frames it as an as-built absence rather than a guarantee to regression-lock. |
| FR-002 | `GetTransactions` MUST return only the caller's own transactions in the given org, most recent first, with an accurate `total_count` independent of the current page. | `TestLedgerService_GetTransactions > "Success"` (`tests/unit/ledger_service_test.go:31`) | `TestLedgerRepository_GetTransactions_IsolationOrderingPagination` (`tests/integration/ledger_test.go`) — real DB, 3 subtests: isolation (excludes a second user's rows in the same org), ordering (3 distinct-dated transactions returned newest-first), pagination (`total_count` stays accurate across a 2-item page 1 and a 1-item page 2, with no overlap) | `TestLedgerService_E2E > "GetTransactions"` (`tests/e2e/ledger_test.go:42`) | — | **Complete** | **Fixed 2026-07-23 (SBR remediation Phase 2).** All 3 MUST-clauses now locked in at L2 against a real Postgres instance (the tier that actually exercises the repository's `ORDER BY`/`WHERE`/`LIMIT`/`OFFSET` SQL). No bug found. |
| FR-003 | `GetLedgerSummary` MUST return the caller's balance and a per-status count of their rentals (as renter or owner) in the given org; as-built, MUST NOT be assumed to roll up across orgs, apply `number_of_months` filtering, or include recent transactions (Known Discrepancies 2-4). | — | `TestLedgerRepository_GetSummary` (`tests/integration/ledger_test.go`) — real DB: balance, active-rentals-as-renter count, active-lendings-as-owner count, pending-requests count, and StatusCount correctly combining both the renter and owner roles (2 ACTIVE total from one of each role + 1 PENDING) | `TestLedgerService_E2E > "GetLedgerSummary"` (`tests/e2e/ledger_test.go:72`) | — | **Complete** | **Fixed 2026-07-23 (SBR remediation Phase 2).** Closed the L1/L2 gap with a real-DB integration test proving the renter-OR-owner union in `StatusCount` (the domain's most business-logic-heavy computation). The three documented as-built discrepancies (Known Discrepancies 2-4: no cross-org rollup, `number_of_months` ignored, no transactions field) remain intentionally undocumented-by-test, as spec.md itself frames them as out-of-scope/target-state rather than current guarantees. No bug found in the in-scope behavior. |

## Summary

- **3 FR-IDs audited, all 3 fully `Complete`** (FR-002, FR-003 closed 2026-07-23 as SBR
  remediation Phase 2; FR-001 closed 2026-07-23 as Phase 4).
- **Remaining gaps**: none. (`last_updated_on` remains undocumented-by-test, per Known
  Discrepancy 1 — spec.md itself frames this as an as-built absence, not a guarantee, so no
  test gap is tracked against it.)
- **Phase 2 outcome**: FR-002's isolation, ordering, and pagination MUST-clauses closed with a
  real-DB integration test (a second user's rows excluded, 3 distinct-dated rows returned
  newest-first, `total_count` verified accurate across two pages). FR-003's renter-OR-owner
  `StatusCount` union closed the same way. No bugs found in either — production behavior matched
  the spec exactly.
- **Phase 4 outcome**: FR-001's L2 gap closed — `ledgerRepo.GetBalance` now has a dedicated
  real-DB integration test. No bug found.
- **Unclassified**: none — every row above was resolved to either cited evidence or an explicit,
  named gap.
