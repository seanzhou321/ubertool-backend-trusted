# Tasks: Bill Split (As-Built + Multi-Org FR-014, FR-015)

**Input**: Design documents from `specs/008-bill-split/`

**Focus**: Retrofit spec — tasks target gaps only:
- **New multi-org FRs**: FR-014 (Global summary), FR-015 (Per-org summary)
- **Known Discrepancies**: KD-1 (ListPayments/ListDisputed/ListResolved no filter/pagination), KD-2 (ADMIN_COMMENT action unused), KD-3 (ResolveDisputedBills job untested), KD-4 (ListPayments org_id was mandatory, now optional — fixed 2026-07-25)
- **Missing tests**: SC-002 (jobs tests), SC-003 (graceful after dispute), GRACEFUL resolution test

**Tests**: Tests are OPTIONAL for happy paths with coverage. REQUIRED for gaps (new FRs, KDs, missing test coverage).

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project already exists — verify structure

- [ ] T001 Verify `internal/service/bill_split.go`, `internal/api/grpc/bill_split.go`, `internal/jobs/billing_jobs.go`, `internal/domain/bill.go` exist per plan.md
- [ ] T002 Verify `bills` / `bill_actions` tables in `podman/trusted-group/postgres/ubertool_schema_trusted.sql` match data-model.md
- [ ] T003 [P] Verify `bill_split_service.proto` matches contracts/README.md (run `make proto` to confirm)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core domain models and jobs infrastructure already exist — verify they support multi-org

- [ ] T004 [P] Verify `bills.org_id` FK to `orgs.id` exists for FR-015 per-org queries
- [ ] T005 [P] Verify `users_orgs` composite PK supports caller's org membership for FR-014 global query
- [ ] T006 [P] Verify `BillStatus` constants in `internal/domain/bill.go` cover all 5 statuses (PENDING, PAID, DISPUTED, ADMIN_RESOLVED, SYSTEM_DEFAULT_ACTION)
- [ ] T007 [P] Run `make test-unit` — confirm existing bill-split unit tests pass (baseline)

**Checkpoint**: Foundation ready — user story implementation can begin

---

## Phase 3: User Story 1 — Monthly Bill Generation & Notice Pipeline (P1)

**Goal**: Verify existing pipeline works; KD-3 (jobs untested) needs tests

**Independent Test**: Seed balances → run settlement → verify bills created + notice_sent_at stamped

### Tests for US1 (Required for KD-3)

- [ ] T008 [P] [US1] **Integration test**: `TakeBalanceSnapshots` records all members' balances for month in `tests/integration/billing_jobs_test.go`
- [ ] T009 [P] [US1] **Integration test**: `PerformBillSplittingForOrg` netting algorithm creates correct debtor→creditor bills in `tests/integration/billing_jobs_test.go`
- [ ] T010 [P] [US1] **Integration test**: `SendBillSplittingNotices` stamps `notice_sent_at` only on debtor email success in `tests/integration/billing_jobs_test.go`
- [ ] T011 [P] [US1] **Integration test**: `CheckOverdueBills` — 10-day PENDING→DISPUTED transition with reason in `tests/integration/billing_jobs_test.go` — **CLOSES KD-3**
- [ ] T012 [P] [US1] **Integration test**: `ResolveDisputedBills` — month-end DISPUTED→SYSTEM_DEFAULT_ACTION (BOTH_FAULT, no balance penalty) in `tests/integration/billing_jobs_test.go` — **CLOSES KD-3**

### Implementation for US1 (Verify Existing)

- [ ] T013 [US1] Verify settlement pipeline: snapshot → netting → bills → notices — no code change expected
- [ ] T014 [US1] Verify `ON CONFLICT DO NOTHING` on `(org, debtor, creditor, month)` — no code change expected
- [ ] T015 [US1] Verify notice retry logic (debtor email failure leaves `notice_sent_at` NULL) — no code change expected

**Checkpoint**: US1 complete when T008-T012 pass (jobs have integration test coverage)

---

## Phase 4: User Story 2 — Payment Acknowledgment Cycle (P1)

**Goal**: Verify acknowledgment flow; SC-003 (graceful after dispute) needs dedicated test

**Independent Test**: Create PENDING bill → debtor ack → creditor ack → PAID + balance transfer

### Tests for US2 (Required for SC-003 Gap)

- [ ] T016 [P] [US2] **Unit test**: `AcknowledgePayment` by debtor then creditor → PAID + balance transfer in `tests/unit/bill_split_service_test.go`
- [ ] T017 [P] [US2] **Unit test**: `AcknowledgePayment` in DISPUTED status → PAID + GRACEFUL (SC-003 gap) in `tests/unit/bill_split_service_test.go`
- [ ] T018 [P] [US2] **Unit test**: Creditor ack before debtor → rejected in `tests/unit/bill_split_service_test.go`
- [ ] T019 [P] [US2] **Unit test**: Double ack by same party → rejected in `tests/unit/bill_split_service_test.go`
- [ ] T020 [P] [US2] **Integration test**: Full ack cycle with `charge_billsplit=true` → ledger + balance in `tests/integration/bill_split_test.go`
- [ ] T021 [P] [US2] **E2E test**: Ack cycle end-to-end in `tests/e2e/bill_split_test.go`

### Implementation for US2 (Verify Existing)

- [ ] T022 [US2] Verify debtor/creditor ack rules (status, order, no-double) — already unit tested
- [ ] T023 [US2] Verify GRACEFUL outcome sets `resolution_outcome=GRACEFUL`, `resolved_at=NOW()` — already implemented

**Checkpoint**: US2 complete when T017 passes (GRACEFUL-after-dispute test added)

---

## Phase 5: User Story 3 — Automated Dispute Lifecycle (P2)

**Goal**: Verify background jobs; KD-3 already addressed in US1 tests

**Independent Test**: Bill with notice_sent_at 11 days ago → CheckOverdueBills → DISPUTED; DISPUTED at month-end → ResolveDisputedBills → SYSTEM_DEFAULT_ACTION + blocks

### Tests for US3 (Already Covered in US1)

- [ ] T024 [P] [US3] **Integration test**: `SendBillReminders` at 72h → reminder email (no status change) in `tests/integration/billing_jobs_test.go`
- [ ] T025 [P] [US3] **Integration test**: `CheckOverdueBills` at 10 days → DISPUTED with reason + DISPUTE_OPENED action in `tests/integration/billing_jobs_test.go` (T011)
- [ ] T026 [P] [US3] **Integration test**: `ResolveDisputedBills` at month-end → SYSTEM_DEFAULT_ACTION + BOTH_FAULT blocks in `tests/integration/billing_jobs_test.go` (T012)

### Implementation for US3 (Verify Existing)

- [ ] T027 [US3] Verify reminder email at 72h — no code change expected
- [ ] T028 [US3] Verify DISPUTED transition at 10 days with correct reason — no code change expected
- [ ] T029 [US3] Verify month-end force-resolve with blocking (no balance penalty) — no code change expected

**Checkpoint**: US3 complete when T024-T026 pass

---

## Phase 6: User Story 4 — Admin Dispute Resolution (P2)

**Goal**: Verify admin resolution with 4 outcomes; all 4 now unit tested (SC-001 met 2026-07-22)

**Independent Test**: DISPUTED bill → admin calls ResolveDispute with each outcome → verify balances/blocks/notifications

### Tests for US4 (Verify Coverage)

- [ ] T030 [P] [US4] **Unit test**: `ResolveDispute` DEBTOR_FAULT → debtor balance -amount, debtor renting_blocked in `tests/unit/bill_split_service_test.go`
- [ ] T031 [P] [US4] **Unit test**: `ResolveDispute` CREDITOR_FAULT → creditor balance -amount, creditor lending_blocked in `tests/unit/bill_split_service_test.go`
- [ ] T032 [P] [US4] **Unit test**: `ResolveDispute` BOTH_FAULT → both penalized, both blocked in `tests/unit/bill_split_service_test.go`
- [ ] T033 [P] [US4] **Unit test**: `ResolveDispute` GRACEFUL → normal payment, no blocks, no penalties in `tests/unit/bill_split_service_test.go`
- [ ] T034 [P] [US4] **Unit test**: Admin involved as debtor/creditor → rejected in `tests/unit/bill_split_service_test.go`
- [ ] T035 [P] [US4] **Unit test**: Bill not DISPUTED → rejected in `tests/unit/bill_split_service_test.go`
- [ ] T036 [P] [US4] **Integration test**: Admin resolution full flow with notifications in `tests/integration/bill_split_test.go`
- [ ] T037 [P] [US4] **E2E test**: Admin resolves dispute end-to-end in `tests/e2e/bill_split_test.go`

### Implementation for US4 (Verify Existing)

- [ ] T038 [US4] Verify 4 resolution outcomes implemented — no code change expected
- [ ] T039 [US4] Verify admin-only + not-party-to-bill auth — no code change expected
- [ ] T040 [US4] Verify notifications (in-app, push, email) on resolution — no code change expected

**Checkpoint**: US4 complete when T030-T037 pass (all 4 outcomes + auth + e2e)

---

## Phase 7: User Story 5 — Bill Split Dashboards & History (P3)

**Goal**: Verify list/detail queries; KD-4 (org_id optional) fixed 2026-07-25; KD-1 (no pagination/filter)

**Independent Test**: ListPayments with status filter + org_id=0 returns across all caller's orgs

### Tests for US5 (Verify Existing + KD-1 Decision)

- [ ] T041 [P] [US5] **Unit test**: `ListPayments` OR-combines multiple statuses in `tests/unit/bill_split_service_test.go`
- [ ] T042 [P] [US5] **Integration test**: `ListPayments` org_id=0 returns across all orgs in `tests/integration/bill_split_test.go` (KD-4 regression)
- [ ] T043 [P] [US5] **Unit test**: `GetPaymentDetail` access: debtor, creditor, org admin in `tests/unit/bill_split_service_test.go`
- [ ] T044 [P] [US5] **Unit test**: `ListDisputedPayments` / `ListResolvedDisputes` admin-only, exclude admin's own bills in `tests/unit/bill_split_service_test.go`
- [ ] T045 [P] [US5] **E2E test**: Dashboard queries end-to-end in `tests/e2e/bill_split_test.go`

### Implementation for US5 (KD-1 Decision)

- [ ] T046 [US5] **KD-1 Decision**: Implement pagination + `settlement_month`/`resolution_outcome` filters on `ListPayments`, `ListDisputedPayments`, `ListResolvedDisputes` OR document as intentional "return all" — `internal/service/bill_split.go`, `internal/repository/postgres/bill_split.go`
- [ ] T047 [P] [US5] **Test for KD-1**: After T046, tests in `tests/unit/bill_split_service_test.go` + `tests/integration/bill_split_test.go`

**Checkpoint**: US5 complete when KD-1 decision implemented + tested

---

## Phase 8: User Story 6 — Multi-Org Bill Split Summaries (P1 — NEW FR-014, FR-015)

**Goal**: Implement FR-014 (Global summary) + FR-015 (Per-org summary) — new RPCs

**Independent Test**: User in Org A + Org B → GetGlobalBillSplitSummary returns summed counts; GetOrganizationBillSplitSummary (as admin) returns per-org breakdown

### Tests for US6 (Required — New Functionality)

- [ ] T048 [P] [US6] **Contract test**: `GetGlobalBillSplitRequest` (empty) → 4 counts + total_amount_cents in `tests/integration/bill_split_test.go`
- [ ] T049 [P] [US6] **Contract test**: `GetOrganizationBillSplitSummaryRequest {org_id}` → 4 counts + total_amount_cents in `tests/integration/bill_split_test.go`
- [ ] T050 [P] [US6] **Integration test**: Global summary aggregates across ALL user's active orgs (debtor OR creditor) in `tests/integration/bill_split_test.go`
- [ ] T051 [P] [US6] **Integration test**: Org summary returns per-org breakdown (list of orgs with 4 counts each) in `tests/integration/bill_split_test.go`
- [ ] T052 [P] [US6] **Integration test**: Org summary requires ADMIN/SUPER_ADMIN in target org in `tests/integration/bill_split_test.go`
- [ ] T053 [P] [US6] **E2E test**: Global + per-org summary end-to-end in `tests/e2e/bill_split_test.go`

### Implementation for US6 (New FR-014, FR-015)

- [ ] T054 [US6] Add `GetGlobalBillSplitSummary` + `GetOrganizationBillSplitSummary` RPCs to proto — `api/proto/ubertool_trusted_backend/v1/bill_split_service.proto`
- [ ] T055 [US6] Implement `GetGlobalBillSplitSummary` in service: query bills WHERE (debtor=user OR creditor=user) AND org IN (user's active orgs) — `internal/service/bill_split.go`
- [ ] T056 [US6] Implement `GetOrganizationBillSplitSummary` in service: verify admin in org, then query bills in that org — `internal/service/bill_split.go`
- [ ] T057 [US6] Add repo methods: `GetGlobalSummary(userID)` + `GetOrgSummary(orgID)` — `internal/repository/postgres/bill_split.go`
- [ ] T058 [US6] gRPC handlers extract userID, call service — `internal/api/grpc/bill_split.go`
- [ ] T059 [P] [US6] **Proto regeneration**: `make proto` after T054

**Checkpoint**: US6 complete when T048-T053 pass (integration + e2e)

---

## Phase 9: Known Discrepancy 2 — ADMIN_COMMENT Action (Unused)

**Goal**: `ADMIN_COMMENT` bill_action type exists in constants/proto but never created

### Decision + Implementation

- [ ] T060 [KD2] **Decision**: Remove `ADMIN_COMMENT` from `BillActionType` constants + proto OR add RPC to create it (admin note on bill) — `internal/domain/bill.go`, `bill_split_service.proto`
- [ ] T061 [P] [KD2] **Test**: After T060, verify no dead code / test in `tests/unit/bill_split_service_test.go`

**Checkpoint**: KD-2 complete when decision implemented

---

## Phase 10: Polish & Cross-Cutting Concerns

**Purpose**: Remaining gaps, documentation, regression locks

- [ ] T062 [P] Run full test suite: `make test-unit && make test-integration && make test-e2e` — all green
- [ ] T063 Update `sbr/rtm/008-bill-split.rtm.md` after FR-014, FR-015, KD-1, KD-2 fixes
- [ ] T064 [P] Quickstart validation: run `specs/008-bill-split/quickstart.md` scenarios manually

---

## Dependencies & Execution Order

### Phase Dependencies
- **Phase 1-2**: No deps — start immediately
- **Phase 3-7**: Depend on Phase 2
- **Phase 8 (US6)**: Depends on Phase 2 + Users/Orgs multi-org membership (003 FR-009)
- **Phase 9**: Independent — can run in parallel
- **Phase 10**: Depends on all prior phases

### Within-Phase Parallelism
- All `[P]` tasks in same phase = parallel (different files)
- T008-T012 (US1 jobs tests) parallel
- T016-T021 (US2 ack tests) parallel
- T030-T037 (US4 resolution tests) parallel
- T041-T045 (US5 tests) parallel
- T048-T053 (US6 new FR tests) parallel

### Cross-Feature Dependencies
| This Feature | Depends On | Coordination |
|--------------|------------|--------------|
| FR-014 (Global) | Users/Orgs FR-009 (multi-org membership) | Uses `users_orgs` for caller's active orgs |
| FR-015 (Per-org) | Users/Orgs FR-009 + Admin auth | Requires admin check in target org |

### MVP Scope
- **MVP = US1 (jobs tested) + US2 (GRACEFUL-after-dispute test) + US4 (4 outcomes tested) + US6 (FR-014, FR-015)**
- US3 covered by US1 job tests
- US5 KD-1 decision = follow-up (doc says "return all" currently)
- KD-2 (ADMIN_COMMENT) = follow-up cleanup

---

## Independent Test Criteria Per Story

| Story | Independent Test |
|-------|------------------|
| US1 | Seed balances → run jobs → bills created with notice_sent_at (debtor success only) |
| US2 | PENDING bill → debtor ack → creditor ack → PAID + balance transfer; DISPUTED bill → same → GRACEFUL |
| US3 | 11-day-old notice → CheckOverdueBills → DISPUTED; month-end DISPUTED → SYSTEM_DEFAULT_ACTION + blocks |
| US4 | DISPUTED bill → admin DEBTOR_FAULT/CREDITOR_FAULT/BOTH_FAULT/GRACEFUL → correct balances/blocks |
| US5 | ListPayments org_id=0 → across all orgs; admin ListDisputed excludes own bills |
| US6 | User in Org A+B → Global = sum; Admin in Org A → Org A breakdown |

---

## Phase 11: Convergence

### Convergence Findings

| ID | Gap Type | Severity | Source | Evidence | Remaining Work |
|----|----------|----------|--------|----------|----------------|
| C1 | missing | CRITICAL | FR-015 | `GetOrganizationBillSplitSummary` handler has NO admin auth check — any member can call per-org summary | Add admin role verification in handler |
| C2 | missing | HIGH | FR-015 | `GetOrganizationBillSplitSummary` service iterates ALL user's orgs (not specified org) — contradicts spec which requires single org_id param | Fix service to accept org_id parameter + admin check |
| C3 | missing | HIGH | KD-1 | `ListPayments`/`ListDisputedPayments`/`ListResolvedDisputes` return full result set; no pagination, no `settlement_month` filter, no `resolution_outcome` filter | Implement pagination + filters or document as intentional |
| C4 | missing | MEDIUM | KD-2 | `BillActionTypeAdminComment` exists in domain but NEVER created by any code path | Remove from domain/proto OR implement admin note RPC |
| C5 | missing | MEDIUM | FR-014 | `GetGlobalBillSplitSummary` returns 4 counts + total_amount_cents; spec FR-014 requires same 4 counts but `total_amount_cents` not in spec | Verify spec alignment or update spec |
| C6 | partial | LOW | Constitution III | Jobs (`CheckOverdueBills`, `ResolveDisputedBills`) send notifications async but no push notification on dispute open/force-resolve | Add push notifications |
| C7 | missing | LOW | SC-002 | `ResolveDisputedBills` (month-end job) has integration test (T012) but no e2e test | Add e2e test for month-end force-resolve |

### Summary
- Requirements checked: 15 FRs, 6 US acceptance scenarios, 4 KDs, 3 SCs
- Constitution principles checked: 6 (I-VI)
- Findings: 7 (1 CRITICAL auth gap, 2 HIGH spec gaps, 2 MEDIUM dead code, 2 LOW)
- **Status**: NOT converged — FR-015 missing admin auth; FR-015 service logic contradicts spec; KD-1/KD-2 need decisions

### Appended Convergence Tasks

- [ ] T065 [C1] **FR-015 CRITICAL**: Add admin role check in `GetOrganizationBillSplitSummary` handler — `internal/api/grpc/bill_split.go` (missing)
- [ ] T066 [C2] **FR-015 HIGH**: Fix `GetOrganizationBillSplitSummary` service to accept `org_id` parameter + admin verification — `internal/service/bill_split.go` (contradicts)
- [ ] T067 [C2] **FR-015 HIGH**: Update proto `GetOrganizationBillSplitSummaryRequest` to include `organization_id` — `bill_split_service.proto` (missing)
- [ ] T068 [C1,C2] **FR-015**: Proto regeneration — `make proto` (missing)
- [ ] T069 [C3] **KD-1**: Decision + implement: pagination + `settlement_month` + `resolution_outcome` filters on `ListPayments`/`ListDisputedPayments`/`ListResolvedDisputes` OR document — `internal/service/bill_split.go`, `internal/repository/postgres/bill_split.go` (missing)
- [ ] T070 [C3] **KD-1**: Tests for KD-1 decision — `tests/unit/bill_split_service_test.go`, `tests/integration/bill_split_test.go` (missing)
- [ ] T071 [C4] **KD-2**: Decision + implement: remove `ADMIN_COMMENT` from domain/proto OR add `AddAdminComment` RPC — `internal/domain/bill.go`, `bill_split_service.proto` (unrequested)
- [ ] T072 [C4] **KD-2**: Test for KD-2 decision — `tests/unit/bill_split_service_test.go` (missing)
- [ ] T073 [C5] **FR-014**: Verify `GetGlobalBillSplitSummary` response matches spec (4 counts + total_amount_cents vs 4 counts only) — update spec or response (partial)
- [ ] T074 [C6] **Constitution III**: Push notification on dispute open (CheckOverdueBills) and force-resolve (ResolveDisputedBills) — `internal/jobs/billing_jobs.go` (missing)
- [ ] T075 [C7] **SC-002**: Add e2e test for `ResolveDisputedBills` month-end force-resolve — `tests/e2e/bill_split_test.go` (missing)
- [ ] T076 Run `make test-unit && make test-integration && make test-e2e` — all green (verify)
- [ ] T077 Update `sbr/rtm/008-bill-split.rtm.md` after C1, C2, C3, C4 fixes

---

## Notes

- **Retrofit discipline**: Do NOT re-implement US1-US5 happy paths — they exist. Tasks verify + close gaps only.
- **FR-014/FR-015**: Two new RPCs added to `bill_split_service.proto`. No schema changes (reads existing `bills` table).
- **Global summary**: Aggregates across all orgs where user is ACTIVE member (debtor OR creditor on bill).
- **Per-org summary**: Admin-only in target org. Returns list of {org_id, org_name, 4 counts} for drill-down.
- **KD-1 (no pagination)**: Currently returns full result set. Decision: implement filters/pagination OR document.
- **KD-2 (ADMIN_COMMENT)**: Dead code. Decision: remove OR implement admin note feature.
- **KD-3 (jobs untested)**: Closed by T008-T012 (integration tests for all 4 jobs).
- **KD-4 (org_id optional)**: Fixed 2026-07-25. Regression test at T042.
- **Proto changes**: Run `make proto` after T054 (new RPCs).
- **Constitution**: Principle III (Push) — jobs send notifications async; Principle VI (Proto-First) — new RPCs in proto first.