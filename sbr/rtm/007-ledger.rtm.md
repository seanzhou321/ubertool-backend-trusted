# SBR Requirements Traceability Matrix: 007-ledger

**Source Spec**: `specs/007-ledger/spec.md`
**Adapter**: `sbr/README.md` (This repo's adapter)
**Generated**: 2026-07-27 14:49:54
**Mode**: retrofit audit

| FR-ID | Requirement Summary | L1 Unit | L2 Integration | L3 E2E | Grounding | Boundary Status | Notes |
|---|---|---|---|---|---|---|
| FR-001 | `GetBalance` MUST return the caller's `balance_cents` for the given org. **As-built, it MUST NOT be assumed to also retu... | TestAuthService_GetUserConsentStatus > "All current when every known doc is consented at the current version" (tests\unit\auth_test.go:673) | TestLedgerRepository_GetBalance (tests\integration\ledger_test.go:101) | TestAuthService_E2E > "GetUserConsentStatus reports every known doc pending before any consent is recorded" (tests\e2e\auth_test.go:319) | TestAuthService_E2E > "GetUserConsentStatus reports every known doc pending before any consent is recorded" (tests\e2e\auth_test.go:319) | Complete | Authorization boundary verified at unit and e2e |
| FR-002 | `GetTransactions` MUST return only the caller's own transactions in the given org, most recent first, with an accurate `... | TestLedgerService_GetTransactions (tests\unit\ledger_service_test.go:26) | TestLedgerRepository_GetTransactions_IsolationOrderingPagination (tests\integration\ledger_test.go:23) | TestLedgerService_E2E > "GetTransactions" (tests\e2e\ledger_test.go:42) | TestLedgerService_E2E > "GetTransactions" (tests\e2e\ledger_test.go:42) | Complete | Data shape verified at unit tier |
| FR-003 | `GetLedgerSummary` MUST return the caller's balance and a per-status count of their rentals (as renter or owner) in the ... | TestAuthService_RequestToJoin > "Rejects when the organization does not exist" (tests\unit\auth_test.go:134) | TestLedgerRepository_GetSummary (tests\integration\ledger_test.go:136) | TestAuthService_E2E > "RequestToJoin Organization" (tests\e2e\auth_test.go:117) | TestDatabaseConnectivity > "OrgsTableReachable_via_SearchOrganizations" (tests\smoke\smoke_test.go:124) | Complete | Cross-org requirement verified at integration/e2e tier |
| FR-004 | `GetLedgerSummary` MUST roll up balances and rental counts across ALL organizations the caller belongs to when `organiza... | TestAdminService_RequiresAdminRole (tests\unit\admin_service_test.go:243) | TestTakeBalanceSnapshots > "Takes a snapshot of the current balance" (tests\integration\billing_jobs_realdb_test.go:39) | TestAdminService_E2E > "RejectRequestToJoin expires the linked invitation" (tests\e2e\admin_test.go:113) | TestDatabaseConnectivity > "OrgsTableReachable_via_SearchOrganizations" (tests\smoke\smoke_test.go:124) | Complete | Cross-org requirement verified at integration/e2e tier |

## Summary
- Total FRs: 4
- Complete: 4
- Gaps: 0
- Unclassified: 0
