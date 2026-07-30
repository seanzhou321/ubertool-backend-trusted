# Analysis: `number_of_months` in FR-004 (Ledger Summary)

## Overview
`number_of_months` is a **lookback window parameter** (`int32`) in `GetLedgerSummaryRequest` that **should** limit rental-activity counts to the last N months — but is **currently a no-op** (Known Discrepancy 3).

---

## Current State

| Artifact | Status |
|----------|--------|
| **Proto** (`ledger_service.proto`) | ✅ Defined: `int32 number_of_months = 2` |
| **Design Doc** (`grpc_api_business_logic.md`) | ✅ Documented: "Retrieve rentals for the last `number_of_months`" |
| **Spec** (`specs/007-ledger/spec.md`) | ❌ **Known Discrepancy #3**: "accepted by proto but read by nothing" |
| **RTM** (`sbr/rtm/007-ledger.rtm.md`) | ❌ **Gap FR-004(c)**: Tracked as deliberate scope boundary |
| **Implementation** | ❌ **No-op** — field ignored at handler, service, and repo layers |

---

## Call Chain Analysis (No-Op Verified)

```
LedgerHandler.GetLedgerSummary(req)
  → ledgerService.GetLedgerSummary(ctx, orgID, userID)      // number_of_months NOT passed
    → ledgerRepository.GetSummary(ctx, userID, orgID)       // number_of_months NOT passed
      → SQL: SELECT ... FROM rentals WHERE org_id = $1      // NO date filter
```

**Result**: All rental counts (`ActiveRentalsCount`, `ActiveLendingsCount`, `PendingRequestsCount`, `StatusCount` map) reflect **entire history**, not the requested window.

---

## FR-004 Multi-Org Rollup Scope

| Sub-req | Description | Status |
|---------|-------------|--------|
| **(a)** Balance rollup across all orgs (`SUM(balance_cents)`) | ✅ Done (`GetSummaryAllOrgs`) |
| **(b)** Rental-count rollup across all orgs | ✅ Done (same method) |
| **(c)** `number_of_months` filtering **per-org before aggregation** | ❌ **Gap** — blocked on single-org filtering first |

> **Architectural Decision** (per spec): Gap (c) deliberately deferred because `number_of_months` isn't implemented for the single-org path either. Fixing it for rollup *first* would implement multi-org filtering before the single-org case it mirrors. Tracked as separate Phase 6 task.

---

## Success Criteria (SC-003)
> "`number_of_months` either gains real filtering behavior, or is removed from the proto/doc as dead input — Known Discrepancy 3 does not remain a parameter that silently does nothing."

---

## Implementation Plan (Phase 6 in `specs/007-ledger/tasks.md`)

### 1. Handler (`internal/api/grpc/ledger.go`)
```go
req.NumberOfMonths → pass to service
```

### 2. Service (`internal/service/ledger.go`)
```go
func (s *LedgerService) GetLedgerSummary(ctx, orgID, userID, numberOfMonths) {
    cutoff := time.Now().AddDate(0, -int(numberOfMonths), 0)
    // pass cutoff to repo
}
```

### 3. Repository (`internal/repository/postgres/ledger.go`)
- `GetSummary`: Add `WHERE created_on >= $cutoff` to rental-count queries
- `GetSummaryAllOrgs`: Apply **per-org** cutoff before aggregation (same cutoff for all orgs, or org-specific if business logic requires)

### 4. Tests (SBR: L1→L2→L3, written failing-first)
| Tier | Test | Assertion |
|------|------|-----------|
| L1 Unit | `TestLedgerService_GetLedgerSummary_FiltersByMonths` | Mock repo receives correct cutoff |
| L2 Int | `TestLedgerRepository_GetSummary_FiltersByMonths` | Seed old/new rentals; only new counted |
| L3 E2E | `TestLedgerService_E2E_GetLedgerSummary_FiltersByMonths` | gRPC call with `number_of_months=3` returns filtered counts |

> **Current Test Gap**: Zero tests seed rentals old enough to distinguish filtered vs. unfiltered behavior.

---

## Files to Modify

| File | Change |
|------|--------|
| `internal/api/grpc/ledger.go` | Read & forward `NumberOfMonths` |
| `internal/service/ledger.go` | Compute cutoff, thread through |
| `internal/repository/postgres/ledger.go` | Add date-window `WHERE` clauses |
| `tests/unit/ledger_service_test.go` | L1 mock verification |
| `tests/integration/ledger_test.go` | L2 seeded data assertion |
| `tests/e2e/ledger_test.go` | L3 end-to-end assertion |

---

## Dependencies
- **Blocked on**: Phase 5 (US3) — `GetSummary` working with `number_of_months` + recent transactions for **single-org** path
- **Then**: Phase 6 extends same filtering to `GetSummaryAllOrgs` (multi-org rollup)

---

## References
- `specs/007-ledger/spec.md` — Known Discrepancy 3, FR-004(c), SC-003
- `specs/007-ledger/tasks.md` — Phase 6 task
- `sbr/rtm/007-ledger.rtm.md` — Gap FR-004(c), Known Discrepancy 3
- `docs/design/grpc_api_business_logic.md` — "Get Ledger Summary" step 2
- `api/proto/ubertool_trusted_backend/v1/ledger_service.proto` — Proto definition