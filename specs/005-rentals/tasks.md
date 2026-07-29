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

## Phase 7: User Story 5 — Multi-Org Rental Shared-Org Validation (Priority: P1 — FR-008)

**Goal**: Implement FR-008: rental `org_id` must be an organization that **both the renter (caller) and the tool owner are active members of**. No `owner_org_id` on tools — tools belong to users.

**Independent Test**: User in Org A + Org B; tool owner in Org B + Org C; `CreateRentalRequest` with `org_id = Org A` → rejected with `FAILED_PRECONDITION` listing shared orgs [Org B]; with `org_id = Org B` → succeeds.

### Tests for User Story 5 (Required — New Functionality)

- [ ] T043 [P] [US5] **Contract test**: `CreateRentalRequest` validates shared org in `tests/integration/rental_test.go`
- [ ] T044 [P] [US5] **Integration test**: Rental with non-shared org_id → `FAILED_PRECONDITION` + shared orgs list in `tests/integration/rental_test.go`
- [ ] T045 [P] [US5] **Integration test**: Rental with shared org_id (Org B) → succeeds, rental.org_id = Org B in `tests/integration/rental_test.go`
- [ ] T046 [P] [US5] **E2E test**: Full cross-org flow: wrong `organization_id` → `FAILED_PRECONDITION` with shared orgs → retry `CreateRentalRequest` directly with a returned shared org ID (no `SetCurrentOrganization` — it doesn't exist) → approve → finalize → complete in `tests/e2e/rental_test.go`
- [ ] T047 [P] [US5] **Unit test**: DB constraint/trigger rejects non-shared org at insert in `tests/unit/rental_test.go`

### Implementation for User Story 5 (FR-008)

- [x] T048 [US5] **Database**: `rentals_shared_org_check` CHECK constraint on `rentals` verifies shared active membership — `podman/trusted-group/postgres/ubertool_schema_trusted.sql`
- [x] ~~T049~~ Dropped — `CreateRentalRequestRequest.organization_id` stays a plain, required, caller-supplied field. There is no server-side "current org" default to fall back to (see `docs/design/multi-org.md`); the client always states which shared org it means.
- [x] ~~T050~~ Reverted 2026-07-29 — `shared_organization_ids`/`shared_organization_names` on `CreateRentalRequestResponse` were removed. A rental-request response is the wrong layer for a list of alternative orgs; that discovery happens earlier via `Tool.owner.orgs` on `SearchTools`/`GetTool` (006-tools-image-storage FR-009). See `sbr/rtm/005-rentals.rtm.md` FR-008 correction note.
- [x] ~~T051~~ Dropped — no auth-interceptor "effective org" injection, no `current-org-id` metadata, no Redis. `internal/api/grpc/rental.go`'s `CreateRentalRequest` handler passes `req.OrganizationId` straight through, exactly like every other rental list/lending endpoint.
- [x] T052 [US5] **Service**: `CreateRentalRequest` validates `orgID` (the caller-supplied `organization_id`) directly — the renter must be an active member (SEC-RENTAL-001) and the tool owner must be an active member of the same org (`isSharedOrganization`); on mismatch returns `FAILED_PRECONDITION` with shared orgs (`getSharedOrganizations`) — `internal/service/rental.go`
- [x] T053 [US5] **Service**: `rental.OrgID = orgID` (the validated caller-supplied `organization_id`) — `internal/service/rental.go`
- [x] T054 [P] [US5] **Proto regeneration**: `make proto-gen` (no proto change was ultimately needed for this feature, since `organization_id` and the shared-org response fields already existed)
- [x] ~~T055~~ Dropped along with T051 — the handler never had an "effective org" to pass; see T052.

**Checkpoint**: US5 complete when T044-T047 pass (integration + e2e + unit for DB constraint)

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
- **Phase 7 (US5)**: Depends on Phase 2 only. Organizations FR-010 (server-side "current org" context) was REMOVED — US5 validates the caller-supplied `organization_id` directly and has no dependency on it.
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
| FR-008 (US5) | None (Organizations FR-010 was removed — no server-side "current org") | Validates the caller-supplied `organization_id` directly |
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
| US1 | Non-owner Update/Delete rejected (e2e); AddTool uses JWT owner_id only — tools have no org_id/owner_org_id (see `docs/design/multi-org.md`) |
| US2 | `SearchTools` (metro-scoped + shared-org filter) returns tools from every org the caller shares with an owner in that metro; `GetTool`/`SearchTools` `Tool.owner.orgs` = shared orgs only |
| US3 | Upload → confirm → thumbnail async; ownership checks on all image mutations; GetDownloadUrl owner-or-AVAILABLE |
| US4 | Delete promotes oldest; SetPrimaryImage atomic swap; GetToolImages CONFIRMED primary-first |

---

## Phase 10: Convergence

### Convergence Findings

| ID | Gap Type | Severity | Source | Evidence | Remaining Work |
|----|----------|----------|--------|----------|----------------|
| C1 | resolved 2026-07-28 | — | FR-008 | `CreateRentalRequest` validates the caller-supplied `organization_id` via `isSharedOrganization`; rejects with `FAILED_PRECONDITION` + shared orgs — no `current_organization_id`/context-switch field was needed | `internal/service/rental.go` |
| C2 | N/A | — | FR-008 | There is no `tool.owner_org_id` (tools have no org column at all — see `docs/design/multi-org.md`); `rental.org_id` is the validated caller-supplied `organization_id` | `internal/service/rental.go` |
| C3 | missing | HIGH | KD-1 | `CreateRentalRequest`: no overlap check (comment only: "Ideally check if tool is already rented in this period") | Design + implement overlap query |
| C4 | missing | HIGH | KD-2 | `RejectRentalRequest` lacks `status == PENDING` check | Add status guard or document intentional |
| C5 | missing | HIGH | KD-3 | `CancelRental` lacks status check (can cancel ACTIVE/COMPLETED) | Add cancelable status guard or document intentional |
| C6 | missing | HIGH | KD-4 | `GetRental` rejects org admin (only renter/owner) | Add org-admin access OR update doc |
| C7 | missing | MEDIUM | US3 | `ApproveReturnDateChange`, `AcknowledgeReturnDateRejection` no unit tests | Add unit tests |
| C8 | missing | MEDIUM | US4 | `ListToolRentals` ownership rejection + filters untested | Add integration tests |
| C9 | missing | MEDIUM | SC-004 | Not all 10 `RentalStatus` values reached by tests | Audit + add missing status coverage |
| C10 | partial | LOW | FR-007 | `ListMyRentals`/`ListMyLendings` org_id=0 fixed 2026-07-25 but regression test exists | Verify regression test passes |

### Summary
- Requirements checked: 8 FRs, 4 US acceptance scenarios, 4 KDs, 4 SCs
- Constitution principles checked: 6 (I-VI)
- Findings: 10 (2 CRITICAL new FR gaps, 4 HIGH KD gaps, 4 MEDIUM test gaps)
- **Status**: NOT converged — FR-008 (C1/C2) resolved 2026-07-28 via the shared-org validation approach (no dependency on Organizations FR-010, which was removed); 4 KDs still need decisions

### Appended Convergence Tasks

- [x] ~~T065~~ Dropped — no `current_organization_id`/`context_switch_required`/`required_organization_id` fields were needed. `CreateRentalRequestRequest.organization_id` (existing) is the caller's explicit choice. `CreateRentalRequestResponse` carries no shared-org list either (see T050) — the rejection is a plain `FAILED_PRECONDITION` message.
- [x] T066 [C1] **FR-008**: `rentalService.CreateRentalRequest` validates the renter is an active member of `organization_id` (SEC-RENTAL-001) and the tool owner is too (`isSharedOrganization`); rejects with `FAILED_PRECONDITION` + shared orgs (`getSharedOrganizations`) on mismatch — `internal/service/rental.go`
- [x] T067 [C2] **FR-008**: `rental.OrgID` = the validated caller-supplied `organization_id` (there is no `tool.owner_org_id` to fall back to) — `internal/service/rental.go`
- [x] ~~T068~~ Dropped along with T065 — the gRPC handler passes `req.OrganizationId` straight through; there is no "effective org" extracted from context. `internal/api/grpc/rental.go`
- [x] ~~T069~~ Dropped — no proto change was needed for FR-008
- [ ] T070 [C3] **KD-1**: Design + implement overlap check in `CreateRentalRequest` (which statuses block: PENDING/APPROVED/SCHEDULED/ACTIVE/OVERDUE?) — `internal/service/rental.go` (missing)
- [ ] T071 [C3] **KD-1**: Unit test: two overlapping requests for same tool → second rejected — `tests/unit/rental_test.go` (missing)
- [ ] T072 [C4] **KD-2**: Decision + implement: add `status == PENDING` check to `RejectRentalRequest` OR update spec — `internal/service/rental.go` (missing)
- [ ] T073 [C4] **KD-2**: Test for KD-2 decision — `tests/unit/rental_test.go` (missing)
- [ ] T074 [C5] **KD-3**: Decision + implement: add cancelable status check to `CancelRental` OR update spec — `internal/service/rental.go` (missing)
- [ ] T075 [C5] **KD-3**: Test for KD-3 decision — `tests/unit/rental_test.go` (missing)
- [ ] T076 [C6] **KD-4**: Decision + implement: add org-admin access to `GetRental` (match doc) OR update doc — `internal/service/rental.go` (missing)
- [ ] T077 [C6] **KD-4**: Test for KD-4 decision — `tests/unit/rental_test.go` (missing)
- [ ] T078 [C7] **US3**: Unit test `ApproveReturnDateChange` — `tests/unit/rental_test.go` (missing)
- [ ] T079 [C7] **US3**: Unit test `AcknowledgeReturnDateRejection` — `tests/unit/rental_test.go` (missing)
- [ ] T080 [C8] **US4**: Integration test `ListToolRentals` ownership + filters — `tests/integration/rental_list_test.go` (missing)
- [ ] T081 [C9] **SC-004**: Audit all 10 `RentalStatus` values have test coverage; add missing — `tests/unit/rental_test.go` + `tests/e2e/rental_test.go` (missing)
- [ ] T082 [C10] **FR-007**: Verify `ListMyRentals`/`ListMyLendings` org_id=0 regression test passes — `tests/integration/rental_list_test.go` (verify)
- [ ] T083 Run `make test-unit && make test-integration && make test-e2e` — all green (verify)
- [ ] T084 Update `sbr/rtm/005-rentals.rtm.md` after C3, C4, C5, C6, C7, C8, C9 fixes

---

## Notes

- **Retrofit discipline**: Do NOT re-implement US1-US4 happy paths — they exist. Tasks verify + close gaps only.
- **FR-008 (cross-org rental)**: Resolved without any proto change — validates the existing `organization_id` field directly. Organizations FR-010 (server-side "current org"/Redis context) was REMOVED and is not a dependency.
- **KD-1 (overlap check)**: Highest severity gap in this domain. Requires design decision on which statuses block.
- **KD-4 (GetRental org-admin)**: Doc says yes, code says no. Decision: implement OR update doc.
- **KD-2, KD-3 (Reject/Cancel status)**: Sibling methods have checks (Approve requires PENDING). Decision: align OR document.
- **Proto changes**: FR-008 adds field to `CreateRentalRequest` + fields to `CreateRentalResponse`. Run `make proto` after T048, T052.
- **Constitution**: Principle III (Push) — CompleteRental launches notifications async; Principle VI (Proto-First) — all new fields in proto first.