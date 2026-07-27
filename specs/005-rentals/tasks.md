# Tasks: Rentals (As-Built + Multi-Org FR-008)

**Input**: Design documents from `specs/005-rentals/`

**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/, quickstart.md

**Tests**: Include test tasks for all gaps (Known Discrepancies + new FR-008). Tests are OPTIONAL for happy paths that already have coverage.

**Organization**: Tasks grouped by user story to enable independent implementation and testing of each story.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Verify project structure and dependencies for Rentals domain

- [ ] T001 Verify `internal/service/rental.go`, `internal/api/grpc/rental.go`, `internal/domain/rental.go` exist per plan.md
- [ ] T002 Verify `rentals` table schema in `podman/trusted-group/postgres/ubertool_schema_trusted.sql` matches data-model.md
- [ ] T003 [P] Verify `rental_service.proto` matches contracts/README.md (run `make proto` to confirm)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented

⚠️ **CRITICAL**: No user story work can begin until this phase is complete

- [ ] T004 Verify `RentalStatus` constants in `internal/domain/rental.go` cover all 10 statuses (PENDING, APPROVED, SCHEDULED, ACTIVE, OVERDUE, RETURN_DATE_CHANGED, RETURN_DATE_CHANGE_REJECTED, COMPLETED, CANCELLED, REJECTED)
- [ ] T005 [P] Verify `utils.CalculateRentalCost` (pricing algorithm) has passing tests in `tests/unit/pricing_test.go`
- [ ] T006 [P] Verify database trigger on `ledger_transactions` → `users_orgs.balance_cents` exists and works
- [ ] T007 [P] Run `make test-unit` — confirm all existing rental unit tests pass (baseline)

**Checkpoint**: Foundation ready — user story implementation can now begin in parallel

---

## Phase 3: User Story 1 — Rental Request Lifecycle (Priority: P1)

**Goal**: Verify existing lifecycle works; close KD-1 (overlap check) if prioritized

**Independent Test**: Create request → approve → finalize → tool becomes RENTED, rental becomes SCHEDULED

### Tests for User Story 1 (Required for Gaps)

- [ ] T008 [P] [US1] **Unit test**: `CreateRentalRequest` with `end_date <= start_date` rejected — `tests/unit/rental_test.go`
- [ ] T009 [P] [US1] **Integration test**: `CreateRentalRequest` → `ApproveRentalRequest` → `FinalizeRentalRequest` happy path in `tests/integration/rental_test.go`
- [ ] T010 [P] [US1] **E2E test**: Full lifecycle (request → approve → finalize → activate → complete with `charge_billsplit=true`) in `tests/e2e/rental_test.go`
- [ ] T011 [P] [US1] **E2E test**: `CreateRentalRequest` with `charge_billsplit=false` → completion skips ledger/balance in `tests/e2e/rental_test.go`

### Implementation for User Story 1 (Verify Existing + Gap Decision)

- [ ] T012 [US1] **KD-1 (HIGH)**: Decision + implementation: add overlap check in `CreateRentalRequest` — `internal/service/rental.go` — requires design decision on which statuses block (PENDING/APPROVED/SCHEDULED/ACTIVE/OVERDUE?)
- [ ] T013 [P] [US1] **Test for KD-1**: Two overlapping requests for same tool → second rejected in `tests/unit/rental_test.go` (write test FIRST, fail, then implement T012)
- [ ] T014 [US1] Verify `CreateRentalRequest` snapshots tool prices — no code change expected
- [ ] T015 [US1] Verify `ApproveRentalRequest` requires owner + PENDING status — no code change expected
- [ ] T016 [US1] Verify `FinalizeRentalRequest` requires renter + APPROVED status, sets tool RENTED — no code change expected
- [ ] T017 [US1] Verify `CancelRentalRequest` works from any status (KD-3 — see Phase 8)

**Checkpoint**: US1 complete when KD-1 decision documented (implemented or explicitly allowed) + test covers it

---

## Phase 4: User Story 2 — Active Rental Lifecycle: Pickup & Return (Priority: P1)

**Goal**: Verify pickup/return/completion with bill-split; KD-5 fix (owner-only CompleteRental) regression-locked

**Independent Test**: Activate SCHEDULED rental → Complete with `charge_billsplit=true` → both balances + ledger updated

### Tests for User Story 2 (Required for Regression Locks)

- [ ] T018 [P] [US2] **Integration test**: `CompleteRental` with `charge_billsplit=true` → ledger + balance updates in `tests/integration/rental_ledger_test.go`
- [ ] T019 [P] [US2] **Unit test**: `CompleteRental` rejects renter caller (KD-5 fix regression) in `tests/unit/rental_test.go`
- [ ] T020 [P] [US2] **Integration test**: `CompleteRental` rejects renter caller (KD-5 fix) in `tests/integration/rental_ledger_test.go`
- [ ] T021 [P] [US2] **Unit test**: `CompleteRental` recalculates cost from price snapshot (not current tool prices) in `tests/unit/rental_test.go`
- [ ] T022 [P] [US2] **E2E test**: `ActivateRental` by either party → other party notified in `tests/e2e/rental_test.go`
- [ ] T023 [P] [US2] **E2E test**: `CompleteRental` with `charge_billsplit=false` → no ledger, direct-settlement reminder in notifications in `tests/e2e/rental_test.go`

### Implementation for User Story 2 (Verify Existing)

- [ ] T024 [US2] Verify `ActivateRental` requires SCHEDULED status — no code change expected
- [ ] T025 [US2] Verify `CompleteRental` requires owner + ACTIVE/SCHEDULED/OVERDUE — KD-5 fix already in place
- [ ] T026 [US2] Verify background goroutine for notifications on completion — no code change expected

**Checkpoint**: US2 complete when KD-5 regression tests (T019, T020) pass

---

## Phase 5: User Story 3 — Return Date Change Negotiation (Priority: P2)

**Goal**: Verify date change sub-flow works; add missing unit tests

**Independent Test**: Renter requests extension → owner rejects with counter → renter acknowledges → rolls back

### Tests for User Story 3 (Required for Coverage Gaps)

- [ ] T027 [P] [US3] **Unit test**: `ApproveReturnDateChange` happy path in `tests/unit/rental_test.go`
- [ ] T028 [P] [US3] **Unit test**: `AcknowledgeReturnDateRejection` rolls back to `last_agreed_end_date` in `tests/unit/rental_test.go`
- [ ] T029 [P] [US3] **Unit test**: `CancelReturnDateChange` same rollback behavior in `tests/unit/rental_test.go`
- [ ] T030 [P] [US3] **Unit test**: `RejectReturnDateChange` requires different counter-date (not earlier than start) in `tests/unit/rental_test.go`
- [ ] T031 [P] [US3] **Integration test**: Full date-change negotiation flow in `tests/integration/rental_test.go`

### Implementation for User Story 3 (Verify Existing)

- [ ] T032 [US3] Verify `ChangeRentalDates` in ACTIVE/OVERDUE only allows end_date change — no code change expected
- [ ] T033 [US3] Verify cost recomputation on every end_date change (FR-005) — no code change expected
- [ ] T034 [US3] Verify status transitions: RETURN_DATE_CHANGED → APPROVE → ACTIVE/OVERDUE — no code change expected

**Checkpoint**: US3 complete when T027-T031 pass

---

## Phase 6: User Story 4 — Rental Visibility (Priority: P3)

**Goal**: Verify list/detail queries; FR-007 (org_id optional filter) already fixed; KD-4 (GetRental org-admin access)

**Independent Test**: `ListMyRentals` with status filter + org_id=0 returns across all caller's orgs

### Tests for User Story 4 (Required for Gaps)

- [ ] T035 [P] [US4] **Unit test**: `GetRental` by org admin (not renter/owner) — currently fails (KD-4) in `tests/unit/rental_test.go`
- [ ] T036 [P] [US4] **Integration test**: `ListMyRentals` OR-combines multiple statuses in `tests/integration/rental_list_test.go`
- [ ] T037 [P] [US4] **Integration test**: `ListMyLendings` org_id=0 returns across all caller's orgs in `tests/integration/rental_list_test.go`
- [ ] T038 [P] [US4] **Unit test**: `ListToolRentals` ownership rejection + status/org filtering in `tests/unit/rental_test.go`
- [ ] T039 [P] [US4] **E2E test**: Rental visibility end-to-end in `tests/e2e/rental_test.go`

### Implementation for User Story 4 (Gap Decisions)

- [ ] T040 [US4] **KD-4 Decision**: Implement org-admin access in `GetRental` (match doc) OR update doc to match code (renter/owner only) — `internal/service/rental.go`
- [ ] T041 [P] [US4] **Test for KD-4**: After T040 decision, add passing test in `tests/unit/rental_test.go`
- [ ] T042 [US4] Verify FR-007 fix (org_id optional) — already done 2026-07-25, regression test exists in `tests/integration/rental_list_test.go`

**Checkpoint**: US4 complete when KD-4 decision implemented + tested

---

## Phase 7: User Story 5 — Multi-Org Rental Context Switch (Priority: P1 — NEW FR-008)

**Goal**: Implement FR-008: context switch when renting cross-org tool

**Independent Test**: User in Org A + Org B; tool in Org B; CreateRentalRequest from Org A context → rejected with `context_switch_required` + target org; after `SetCurrentOrganization`, retry succeeds

### Tests for User Story 5 (Required — New Functionality)

- [ ] T043 [P] [US5] **Contract test**: `CreateRentalRequest` with `current_organization_id` field in `tests/integration/rental_test.go`
- [ ] T044 [P] [US5] **Integration test**: Cross-org rental from wrong context → `context_switch_required` error + target org in `tests/integration/rental_test.go`
- [ ] T045 [P] [US5] **Integration test**: After `SetCurrentOrganization` (via Organizations FR-010), retry succeeds → rental.org_id = tool.org_id in `tests/integration/rental_test.go`
- [ ] T046 [P] [US5] **E2E test**: Full cross-org rental flow: wrong context → switch → retry → approve → finalize → complete in `tests/e2e/rental_test.go`
- [ ] T047 [P] [US5] **Unit test**: Rental.org_id always set to tool.owner_org_id regardless of caller's context in `tests/unit/rental_test.go`

### Implementation for User Story 5 (New FR-008)

- [ ] T048 [US5] Add `current_organization_id` to `CreateRentalRequest` proto — `api/proto/ubertool_trusted_backend/v1/rental_service.proto`
- [ ] T049 [US5] In `rentalService.CreateRentalRequest`: validate caller is member of `tool.owner_org_id` — `internal/service/rental.go`
- [ ] T050 [US5] If `current_organization_id != tool.owner_org_id`: reject with `context_switch_required=true`, `required_organization_id=tool.owner_org_id` — `internal/service/rental.go`
- [ ] T051 [US5] Set `rental.org_id = tool.owner_org_id` always (not caller's context) — `internal/service/rental.go`
- [ ] T052 [US5] Add `context_switch_required` + `required_organization_id` to `CreateRentalResponse` proto — `rental_service.proto`
- [ ] T053 [P] [US5] **Proto regeneration**: `make proto` after T048, T052
- [ ] T054 [US5] gRPC handler extracts `current_organization_id` from context (set by Organizations FR-010 Redis interceptor) — `internal/api/grpc/rental.go`

**Checkpoint**: US5 complete when T044-T047 pass (integration + e2e)

---

## Phase 8: Known Discrepancies 2 & 3 — Status Gating on Reject/Cancel

**Goal**: Close KD-2 (RejectRentalRequest) and KD-3 (CancelRentalRequest) status checks

**Independent Test**: Call Reject on APPROVED rental → rejected? Call Cancel on ACTIVE rental → rejected?

### Tests

- [ ] T055 [P] [KD2] **Unit test**: `RejectRentalRequest` on APPROVED rental → rejected (or document as intentional) in `tests/unit/rental_test.go`
- [ ] T056 [P] [KD3] **Unit test**: `CancelRentalRequest` on ACTIVE rental → rejected (or document as intentional) in `tests/unit/rental_test.go`

### Implementation (Decision + Code)

- [ ] T057 [KD2] **Decision**: Add `status == PENDING` check to `RejectRentalRequest` (match `ApproveRentalRequest`) OR update spec to state intentional — `internal/service/rental.go`
- [ ] T058 [KD3] **Decision**: Add status check to `CancelRentalRequest` (cancelable: PENDING/APPROVED/SCHEDULED?) OR update spec — `internal/service/rental.go`
- [ ] T059 [P] [KD2] **Test**: After T057, test passes in `tests/unit/rental_test.go`
- [ ] T060 [P] [KD3] **Test**: After T058, test passes in `tests/unit/rental_test.go`

**Checkpoint**: KD-2, KD-3 decisions documented + tests pass

---

## Phase 9: Polish & Cross-Cutting Concerns

**Purpose**: Remaining gaps, documentation, regression locks

- [ ] T061 [P] **SC-004**: Ensure all 10 `RentalStatus` values have at least one test reaching them — audit `tests/unit/rental_test.go` + `tests/e2e/rental_test.go`
- [ ] T062 [P] Run full test suite: `make test-unit && make test-integration && make test-e2e` — all green
- [ ] T063 Update `sbr/rtm/005-rentals.rtm.md` after KD-1, KD-2, KD-3, KD-4, FR-008 fixes
- [ ] T064 [P] Quickstart validation: run `specs/005-rentals/quickstart.md` scenarios manually

---

## Dependencies & Execution Order

### Phase Dependencies
- **Phase 1-2**: No deps — start immediately
- **Phase 3-6**: Depend on Phase 2
- **Phase 7 (US5)**: Depends on Phase 2 + **Organizations FR-010** (Redis context) must be deployed first
- **Phase 8**: Independent — can run in parallel with Phase 7
- **Phase 9**: Depends on all prior phases

### Within-Phase Parallelism
- All `[P]` tasks in same phase = parallel (different files)
- T008-T011 (US1 tests) parallel
- T018-T023 (US2 tests) parallel
- T027-T031 (US3 tests) parallel
- T035-T039 (US4 tests) parallel
- T043-T047 (US5 tests) parallel
- T055-T056 (KD tests) parallel

### Cross-Feature Dependencies
| This Feature | Depends On | Coordination |
|--------------|------------|--------------|
| FR-008 (US5) | Organizations FR-010 (Redis current_org) | Must deploy 003 FR-010 first |
| FR-008 metro resolution | Organizations FR-011 (metro per org) | Uses `orgs.metro` |
| FR-008 org membership | Users FR-001/FR-002 (multi-org) | Uses `users_orgs` composite PK |

### MVP Scope
- **MVP = US1 (KD-1 e2e) + US2 (KD-5 regression) + US4 (KD-4 decision) + US5 (FR-008)**
- US3 (date change) = follow-up (unit tests only)
- KD-2, KD-3 = follow-up decisions

---

## Independent Test Criteria Per Story

| Story | Independent Test |
|-------|------------------|
| US1 | Non-owner Update/Delete rejected (e2e); AddTool uses JWT owner_id + Redis org_id |
| US2 | `include_all_my_orgs=true` returns tools from all user's orgs in metro; `GetTool` owner_organizations = shared orgs only |
| US3 | Upload → confirm → thumbnail async; ownership checks on all image mutations; GetDownloadUrl owner-or-AVAILABLE |
| US4 | Delete promotes oldest; SetPrimaryImage atomic swap; GetToolImages CONFIRMED primary-first |

---

## Notes

- **Retrofit discipline**: Do NOT re-implement US1-US4 happy paths — they exist. Tasks verify + close gaps only.
- **FR-008 (cross-org rental)**: Adds `current_organization_id` to proto. Requires Organizations FR-010 (Redis context) deployed first.
- **KD-1 (overlap check)**: Highest severity gap in this domain. Requires design decision on which statuses block.
- **KD-4 (GetRental org-admin)**: Doc says yes, code says no. Decision: implement OR update doc.
- **KD-2, KD-3 (Reject/Cancel status)**: Sibling methods have checks (Approve requires PENDING). Decision: align OR document.
- **Proto changes**: FR-008 adds field to `CreateRentalRequest` + fields to `CreateRentalResponse`. Run `make proto` after T048, T052.
- **Constitution**: Principle III (Push) — CompleteRental launches notifications async; Principle VI (Proto-First) — all new fields in proto first.