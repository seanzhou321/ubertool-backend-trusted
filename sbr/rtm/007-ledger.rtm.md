# Requirements Traceability Matrix — Ledger

- **Source spec**: `specs/007-ledger/spec.md`
- **Tier mapping in effect**: `sbr/README.md` → "This repo's adapter" (L1=`tests/unit`,
  L2=`tests/integration`, L3=`tests/e2e`, Grounding=`tests/smoke`+`tests/e2e` journeys)
- **Generated**: 2026-07-22 by `speckit-sbr-audit`
- **Mode**: retrofit audit (as-built) — see project constitution Principle I

| FR-ID | Requirement Summary | L1 Unit | L2 Integration | L3 E2E | Grounding (Smoke) | Boundary Status | Notes |
|---|---|---|---|---|---|---|---|
| FR-001 | `GetBalance` MUST return the caller's `balance_cents` for the given org (and, as-built, MUST NOT be assumed to populate `last_updated_on`). | `TestLedgerService_GetBalance > "Success"` (`tests/unit/ledger_service_test.go:17`); `TestLedgerRepository_GetBalance > "Success"` (`tests/unit/repos/ledger_test.go:52`) | — | `TestLedgerService_E2E > "GetBalance"` (`tests/e2e/ledger_test.go:23`); re-exercised across a before/after-settlement comparison in `TestLedgerService_E2E > "Ledger Updates After Rental Completion"` (`tests/e2e/ledger_test.go:163-207`) | — | **Gap — L2** | The core `balance_cents` return value is well covered at L1 (both service and repository layers, via mocked repo and sqlmock respectively) and at L3 with a real before/after-settlement value change, giving genuine confidence in the happy path. No test exercises `ledgerRepo.GetBalance`/`LedgerHandler.GetBalance` against a real Postgres instance directly (L2) — the only real-DB ledger integration test, `TestRentalAndLedger_Integration`, never calls `GetBalance`. Consistent with spec.md's own "Not covered anywhere" list, no test anywhere asserts on `last_updated_on` (Known Discrepancy 1) — the as-built always-empty value is undocumented-by-test as well as undocumented-by-doc, so a future fix to Known Discrepancy 1 has no regression test to break as a warning signal. |
| FR-002 | `GetTransactions` MUST return only the caller's own transactions in the given org, most recent first, with an accurate `total_count` independent of the current page. | `TestLedgerService_GetTransactions > "Success"` (`tests/unit/ledger_service_test.go:31`) | `TestLedgerRepository_GetTransactions_IsolationOrderingPagination` (`tests/integration/ledger_test.go`) — real DB, 3 subtests: isolation (excludes a second user's rows in the same org), ordering (3 distinct-dated transactions returned newest-first), pagination (`total_count` stays accurate across a 2-item page 1 and a 1-item page 2, with no overlap) | `TestLedgerService_E2E > "GetTransactions"` (`tests/e2e/ledger_test.go:42`) | — | **Complete** | **Fixed 2026-07-23 (SBR remediation Phase 2).** All 3 MUST-clauses now locked in at L2 against a real Postgres instance (the tier that actually exercises the repository's `ORDER BY`/`WHERE`/`LIMIT`/`OFFSET` SQL). No bug found. |
| FR-003 | `GetLedgerSummary` MUST return the caller's balance and a per-status count of their rentals (as renter or owner) in the given org; as-built, MUST NOT be assumed to roll up across orgs, apply `number_of_months` filtering, or include recent transactions (Known Discrepancies 2-4). | — | `TestLedgerRepository_GetSummary` (`tests/integration/ledger_test.go`) — real DB: balance, active-rentals-as-renter count, active-lendings-as-owner count, pending-requests count, and StatusCount correctly combining both the renter and owner roles (2 ACTIVE total from one of each role + 1 PENDING) | `TestLedgerService_E2E > "GetLedgerSummary"` (`tests/e2e/ledger_test.go:72`) | — | **Complete** | **Fixed 2026-07-23 (SBR remediation Phase 2).** Closed the L1/L2 gap with a real-DB integration test proving the renter-OR-owner union in `StatusCount` (the domain's most business-logic-heavy computation). The three documented as-built discrepancies (Known Discrepancies 2-4: no cross-org rollup, `number_of_months` ignored, no transactions field) remain intentionally undocumented-by-test, as spec.md itself frames them as out-of-scope/target-state rather than current guarantees. No bug found in the in-scope behavior. |

## Summary

- **3 FR-IDs audited, 2 fully `Complete`** (FR-002, FR-003 closed 2026-07-23 as SBR remediation
  Phase 2).
- **Remaining gaps**: FR-001 (L2 — no real-DB test calls `GetBalance` directly; `last_updated_on`
  remains undocumented-by-test, per Known Discrepancy 1) — tracked in `sbr/remediation-plan.md`
  Phase 3.
- **Phase 2 outcome**: FR-002's isolation, ordering, and pagination MUST-clauses closed with a
  real-DB integration test (a second user's rows excluded, 3 distinct-dated rows returned
  newest-first, `total_count` verified accurate across two pages). FR-003's renter-OR-owner
  `StatusCount` union closed the same way. No bugs found in either — production behavior matched
  the spec exactly.
- **Unclassified**: none — every row above was resolved to either cited evidence or an explicit,
  named gap.
