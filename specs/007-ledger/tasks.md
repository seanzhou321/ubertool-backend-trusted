# Tasks: Ledger (As-Built + Multi-Org Rollup FR-004)

**Input**: Design documents from `specs/007-ledger/`

**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/

**Tests**: OPTIONAL - Only include tests for the gap FR-004 and Known Discrepancies 1-4; existing functionality already has test coverage per SBR audit.

**Organization**: Tasks are grouped by user story (US1=P1, US2=P2, US3=P2) with the multi-org gap (FR-004) as a follow-up enhancement story.

## Format: `[ID] [P?] [Story?] Description with file path`

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Ensure project structure and tooling are in place (already satisfied in this monorepo)

- [ ] T001 Verify Go module and dependencies in `go.mod` / `go.sum`
- [ ] T002 [P] Verify `Makefile` has `proto`, `test`, `build` targets
- [ ] T003 [P] Verify `tests/` tier structure exists: `unit/`, `integration/`, `e2e/`, `smoke/`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core types and interfaces that all stories depend on (already exist in this retrofit)

- [ ] T004 Verify `internal/domain/ledger.go` defines `LedgerTransaction`, `LedgerSummary` types
- [ ] T005 Verify `internal/repository.LedgerRepository` interface exists with `GetBalance`, `ListTransactions`, `GetSummary`, `CreateTransaction`
- [ ] T006 Verify `internal/service.LedgerService` interface exists with `GetBalance`, `GetTransactions`, `GetLedgerSummary`
- [ ] T007 Verify gRPC handler `internal/api/grpc/ledger.go` implements all 3 RPCs
- [ ] T008 Verify proto `api/proto/ubertool_trusted_backend/v1/ledger_service.proto` defines all messages

**Checkpoint**: All foundational types/interfaces exist — user story work can begin

---

## Phase 3: User Story 1 - Check Current Balance (Priority: P1) 🎯 MVP

**Goal**: Return authenticated user's balance for a given organization via `GetBalance`

**Independent Test**: `go test ./tests/unit/... -run TestLedgerService_GetBalance` and `go test ./tests/e2e/... -run TestLedgerService_E2E/GetBalance`

### Implementation for User Story 1 (Existing — verify completeness)

- [ ] T009 [US1] Verify `internal/repository/postgres/ledger.go:GetBalance` reads `balance_cents` from `users_orgs`
- [ ] T010 [US1] Verify `internal/service/ledger.go:GetBalance` calls repository and returns balance
- [ ] T011 [US1] Verify `internal/api/grpc/ledger.go:GetBalance` handler maps request → service → response
- [ ] T012 [US1] **Known Discrepancy 1 Fix**: Update `internal/repository/postgres/ledger.go:GetBalance` to also SELECT `last_balance_updated_on` from `users_orgs`
- [ ] T013 [US1] **Known Discrepancy 1 Fix**: Update `internal/service/ledger.go:GetBalance` to return `(int32, string, error)` with the date
- [ ] T014 [US1] **Known Discrepancy 1 Fix**: Update `internal/api/grpc/ledger.go:GetBalance` to populate `LastUpdatedOn` in `GetBalanceResponse`
- [ ] T015 [US1] Regenerate proto: `make proto` (no proto change needed for this fix)

**Checkpoint**: US1 complete — `GetBalance` returns balance + populated `last_updated_on`

---

## Phase 4: User Story 2 - Browse Transaction History (Priority: P2)

**Goal**: Paginated transaction history via `GetTransactions`

**Independent Test**: `go test ./tests/unit/... -run TestLedgerService_GetTransactions` and `go test ./tests/e2e/... -run TestLedgerService_E2E/GetTransactions`

### Implementation for User Story 2 (Existing — verify completeness)

- [ ] T016 [US2] Verify `internal/repository/postgres/ledger.go:ListTransactions` queries `ledger_transactions` with pagination
- [ ] T017 [US2] Verify `internal/service/ledger.go:GetTransactions` calls repository and returns `([]LedgerTransaction, int32, error)`
- [ ] T018 [US2] Verify `internal/api/grpc/ledger.go:GetTransactions` handler maps pagination params correctly
- [ ] T019 [US2] **Gap**: Add unit test for pagination boundary (exact `total_count` across pages) in `tests/unit/ledger_service_test.go`
- [ ] T020 [US2] **Gap**: Add integration test for empty-result page in `tests/integration/ledger_test.go`

**Checkpoint**: US2 complete — `GetTransactions` paginated with accurate `total_count`

---

## Phase 5: User Story 3 - Ledger Summary Dashboard (Priority: P2)

**Goal**: Consolidated balance + rental activity counts via `GetLedgerSummary`

**Independent Test**: `go test ./tests/unit/... -run TestLedgerService_GetLedgerSummary` and `go test ./tests/e2e/... -run TestLedgerService_E2E/GetLedgerSummary`

### Implementation for User Story 3 (Existing + Known Discrepancy fixes)

- [ ] T021 [US3] Verify `internal/repository/postgres/ledger.go:GetSummary` computes balance + rental counts
- [ ] T022 [US3] Verify `internal/service/ledger.go:GetLedgerSummary` calls repository
- [ ] T023 [US3] Verify `internal/api/grpc/ledger.go:GetLedgerSummary` handler maps request/response
- [ ] T024 [US3] **Known Discrepancy 3 Fix**: Update `GetSummary` signature to accept `numberOfMonths int32`
- [ ] T025 [US3] **Known Discrepancy 3 Fix**: Add date-range filter to all 4 rental-count SQL queries in `GetSummary` using `numberOfMonths`
- [ ] T026 [US3] **Known Discrepancy 4 Fix**: Extend `GetLedgerSummaryResponse` proto to include `recent_transactions` field (repeated Transaction, limited to 5)
- [ ] T027 [US3] **Known Discrepancy 4 Fix**: Regenerate proto: `make proto`
- [ ] T028 [US3] **Known Discrepancy 4 Fix**: Update repository `GetSummary` to fetch 5 most recent transactions
- [ ] T029 [US3] **Known Discrepancy 4 Fix**: Update service and handler to populate `RecentTransactions` in response

**Checkpoint**: US3 complete — `GetLedgerSummary` applies `number_of_months` filter and returns recent transactions

---

## Phase 6: Multi-Org Rollup Enhancement (FR-004) — **The Gap**

**Goal**: Implement cross-organization ledger rollup when `organization_id = 0` (sentinel)

**Priority**: P2 (blocked by FR-004 being a Known Discrepancy Gap 2 in the as-built spec)

**Independent Test**: Call `GetLedgerSummary` with `organization_id: 0`; verify aggregated balance and status counts across all user's active orgs

### Design Decisions (from research.md)
- Sentinel: `org_id = 0` means "all active orgs" — no proto change needed
- Rollup logic in repository layer (SQL aggregation via CTEs)
- Membership anchor: `users_orgs` where `user_id = $1 AND status = 'ACTIVE'`
- Balance: `SUM(balance_cents)` across memberships
- Rental counts: `rentals` JOIN `users_orgs` on `org_id` where user is renter OR owner

### Tests for FR-004 (write FIRST — TDD for the gap)

- [ ] T030 [P] [FR-004] Unit test: `TestLedgerService_GetLedgerSummary_CrossOrgRollup` in `tests/unit/ledger_service_test.go` — mock repo returns aggregated data; verify service sums correctly
- [ ] T031 [P] [FR-004] Integration test: `TestLedgerRepository_GetSummary_CrossOrg` in `tests/integration/ledger_test.go` — seed 2 orgs with balances + rentals; call with `orgID=0`; verify aggregation
- [ ] T032 [P] [FR-004] E2E test: `TestLedgerService_E2E/GetLedgerSummary_CrossOrg` in `tests/e2e/ledger_test.go` — full gRPC call with `organization_id: 0`

### Implementation for FR-004

- [ ] T033 [FR-004] Update `internal/repository.LedgerRepository` interface: add `GetCrossOrgSummary(ctx, userID int32, numberOfMonths int32) (*domain.LedgerSummary, error)`
- [ ] T034 [FR-004] Implement `ledgerRepository.GetCrossOrgSummary` in `internal/repository/postgres/ledger.go` with CTE-based aggregation (see research.md query structure)
- [ ] T035 [FR-004] Update `internal/service/ledger.go:GetLedgerSummary` to branch: if `orgID == 0` call `GetCrossOrgSummary` else call `GetSummary`
- [ ] T036 [FR-004] Update `internal/api/grpc/ledger.go:GetLedgerSummary` to pass `orgID` through (no change needed — already passes)
- [ ] T037 [FR-004] **Known Discrepancy 3 applies here too**: Ensure `numberOfMonths` filter is applied per-org before aggregation in `GetCrossOrgSummary`
- [ ] T038 [FR-004] **Known Discrepancy 4 applies here too**: Include recent transactions (5 most recent across all orgs) in cross-org response

**Checkpoint**: FR-004 complete — `GetLedgerSummary(organization_id=0)` returns cross-org rollup

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Final validation and documentation updates

- [ ] T039 [P] Update `specs/007-ledger/spec.md`: Resolve Known Discrepancies 1-4 (mark as fixed or explicitly document remaining gaps)
- [ ] T040 [P] Run full test suite: `make test` (unit + integration + e2e + smoke)
- [ ] T041 [P] Verify `quickstart.md` validation steps pass against updated implementation
- [ ] T042 [P] Update `sbr/rtm/007-ledger.rtm.md` if new tests added (re-run `/speckit-sbr-audit`)

---

## Dependencies & Execution Order

### Phase Dependencies
- **Phase 1 (Setup)**: No dependencies — can start immediately
- **Phase 2 (Foundational)**: Depends on Phase 1 — BLOCKS all user stories (already satisfied in this retrofit)
- **Phase 3 (US1)**: Depends on Phase 2 — Can run in parallel with US2, US3
- **Phase 4 (US2)**: Depends on Phase 2 — Can run in parallel with US1, US3
- **Phase 5 (US3)**: Depends on Phase 2 — Can run in parallel with US1, US2
- **Phase 6 (FR-004)**: Depends on Phase 5 (US3) — Requires `GetSummary` working with `number_of_months` + recent transactions first
- **Phase 7 (Polish)**: Depends on all prior phases complete

### User Story Dependencies
- US1, US2, US3: **Independent** after Foundational phase — can be worked in parallel by different developers
- FR-004: **Depends on US3** — needs the enhanced `GetSummary` behavior (date filter, recent transactions) as baseline

### Within Each User Story
- Tests (if any) → Models → Repository → Service → Handler → Integration

---

## Parallel Opportunities

```bash
# Phase 1-2: All [P] tasks can run together
Task: T001, T002, T003, T004, T005, T006, T007, T008

# Phase 3-5: All user stories can run in parallel after Phase 2
# Developer A: T009-T015 (US1)
# Developer B: T016-T020 (US2)
# Developer C: T021-T029 (US3)

# Phase 6: FR-004 tests [P] can run together
Task: T030, T031, T032

# Phase 7: All [P] tasks can run together
Task: T039, T040, T041, T042
```

---

## MVP Scope

**Minimum Viable**: Phase 2 (Foundational) + Phase 3 (US1) + Phase 7 (Polish)
- US1 (Check Balance) is the highest-value, most-used RPC
- Known Discrepancy 1 (last_updated_on) is a quick fix in US1
- Deploy US1 fix independently, then add US2/US3/FR-004 incrementally

---

## Notes

- `[P]` = parallelizable (different files, no dependencies)
- `[US1]`, `[US2]`, `[US3]`, `[FR-004]` = story/gap label for traceability to spec.md
- This is a **retrofit** tasks.md: existing code verified against as-built spec; tasks focus on:
  1. Verifying existing implementation completeness
  2. Fixing 4 Known Discrepancies (doc-vs-code gaps)
  3. Implementing FR-004 (multi-org rollup) — the only net-new requirement
- No new database migrations needed (FR-004 uses existing `users_orgs` + `rentals` tables)
- Proto regeneration required only for Known Discrepancy 4 (adding `recent_transactions` field)