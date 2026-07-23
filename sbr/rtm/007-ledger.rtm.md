# Requirements Traceability Matrix — Ledger

- **Source spec**: `specs/007-ledger/spec.md`
- **Tier mapping in effect**: `sbr/README.md` → "This repo's adapter" (L1=`tests/unit`,
  L2=`tests/integration`, L3=`tests/e2e`, Grounding=`tests/smoke`+`tests/e2e` journeys)
- **Generated**: 2026-07-22 by `speckit-sbr-audit`
- **Mode**: retrofit audit (as-built) — see project constitution Principle I

| FR-ID | Requirement Summary | L1 Unit | L2 Integration | L3 E2E | Grounding (Smoke) | Boundary Status | Notes |
|---|---|---|---|---|---|---|---|
| FR-001 | `GetBalance` MUST return the caller's `balance_cents` for the given org (and, as-built, MUST NOT be assumed to populate `last_updated_on`). | `TestLedgerService_GetBalance > "Success"` (`tests/unit/ledger_service_test.go:17`); `TestLedgerRepository_GetBalance > "Success"` (`tests/unit/repos/ledger_test.go:52`) | — | `TestLedgerService_E2E > "GetBalance"` (`tests/e2e/ledger_test.go:23`); re-exercised across a before/after-settlement comparison in `TestLedgerService_E2E > "Ledger Updates After Rental Completion"` (`tests/e2e/ledger_test.go:163-207`) | — | **Gap — L2** | The core `balance_cents` return value is well covered at L1 (both service and repository layers, via mocked repo and sqlmock respectively) and at L3 with a real before/after-settlement value change, giving genuine confidence in the happy path. No test exercises `ledgerRepo.GetBalance`/`LedgerHandler.GetBalance` against a real Postgres instance directly (L2) — the only real-DB ledger integration test, `TestRentalAndLedger_Integration`, never calls `GetBalance`. Consistent with spec.md's own "Not covered anywhere" list, no test anywhere asserts on `last_updated_on` (Known Discrepancy 1) — the as-built always-empty value is undocumented-by-test as well as undocumented-by-doc, so a future fix to Known Discrepancy 1 has no regression test to break as a warning signal. |
| FR-002 | `GetTransactions` MUST return only the caller's own transactions in the given org, most recent first, with an accurate `total_count` independent of the current page. | `TestLedgerService_GetTransactions > "Success"` (`tests/unit/ledger_service_test.go:31`) — mocked repo, single call, no ordering/isolation assertion | `TestRentalAndLedger_Integration > "Full Lifecycle"` (`tests/integration/rental_ledger_test.go:170`) — real-DB `ledgerRepo.ListTransactions` call; asserts `total >= 1` and only the first returned row's `Amount` | `TestLedgerService_E2E > "GetTransactions"` (`tests/e2e/ledger_test.go:42`) — asserts `len(resp.Transactions) >= 2` and `resp.TotalCount >= 2` after seeding 2 rows for one user | — | **Gap — ordering, isolation, and pagination unverified (all tiers, partial)** | Every tier has *some* test that calls the code path and gets a non-empty result, so this is not a zero-coverage FR — but none of the three MUST-clauses in this requirement is actually locked in anywhere: (1) no test seeds a second user's transactions and confirms `GetTransactions` excludes them (isolation); (2) no test seeds transactions with distinct timestamps and asserts response order (`ORDER BY created_on DESC` in `internal/repository/postgres/ledger.go:37` is unexercised by any assertion); (3) no test requests page 2 or checks `total_count` stays accurate across multiple pages/an empty page. This exactly matches spec.md's own "Not covered anywhere" callout for pagination boundaries — confirmed accurate, and the ordering/isolation gaps go further than what spec.md's baseline claims. |
| FR-003 | `GetLedgerSummary` MUST return the caller's balance and a per-status count of their rentals (as renter or owner) in the given org; as-built, MUST NOT be assumed to roll up across orgs, apply `number_of_months` filtering, or include recent transactions (Known Discrepancies 2-4). | — | — | `TestLedgerService_E2E > "GetLedgerSummary"` (`tests/e2e/ledger_test.go:72`) — single concrete `organization_id`, asserts `Balance` and three `StatusCount` entries (`COMPLETED`/`SCHEDULED`/`PENDING` each `>= 1`) | — | **Gap — L1, L2** | This is the RPC spec.md itself flags as having "the richest (and least accurately implemented) business logic in this domain," yet it has the thinnest test coverage of the three FRs in this feature: zero unit tests for `ledgerService.GetLedgerSummary` or `ledgerRepository.GetSummary`, zero integration tests against real Postgres, and the single e2e subtest only exercises the happy path with a valid `organization_id` — it does not seed old rentals to distinguish "filtered" from "unfiltered" (Known Discrepancy 3), does not call with `organization_id` omitted to observe the documented "no rows" failure (Known Discrepancy 2), and does not assert on the absence of a transactions field (Known Discrepancy 4). None of spec.md's three documented as-built discrepancies for this RPC has a regression test locking in current (arguably-broken) behavior, so any future fix — or future accidental further regression — would go unnoticed by the test suite either way. |

## Summary

- **3 FR-IDs audited, 0 fully `Complete`.**
- **Gap count by tier**: L1 — 2 (FR-002 partial, FR-003 missing); L2 — 3 (FR-001 missing, FR-002
  partial/real-DB-but-unasserted, FR-003 missing); L3 — 2 (FR-002 partial: no ordering/isolation/
  pagination assertions, FR-003 partial: happy-path only, no discrepancy-locking assertions);
  Grounding — 3 (no FR in this feature has smoke-tier evidence, expected — smoke targets
  deployment liveness, not per-requirement behavior).
- **FR-IDs with gaps**: FR-001, FR-002, FR-003 (all three FRs in this feature have at least one
  gap).
- **Highest-priority finding**: `GetLedgerSummary` (FR-003) — the RPC spec.md itself identifies as
  having the domain's richest and least-accurate business logic, with three documented as-built
  discrepancies (no cross-org rollup, `number_of_months` silently ignored, no recent-transactions
  field) — has **zero** unit or integration coverage and only a single e2e happy-path subtest.
  None of the three discrepancies has a regression test locking in current behavior, meaning a
  well-intentioned fix to any of them (per SC-002/SC-003) could silently change other behavior
  with nothing in the suite to catch it. Unlike the 003 audit, spec.md's own "Current Test
  Coverage Baseline" section does **not** contradict what this audit found — it is honest about
  most gaps, though it slightly *undersells* its own e2e coverage (it credits e2e coverage only to
  `GetBalance` and `GetLedgerSummary`, omitting the `GetTransactions` e2e subtest that in fact
  exists at `tests/e2e/ledger_test.go:42`) — an understatement, the opposite direction from 003's
  overstated SC-002 claim, and lower-severity since it errs toward caution rather than false
  confidence.
- **Unclassified**: none — every row above was resolved to either cited evidence or an explicit,
  named gap.
