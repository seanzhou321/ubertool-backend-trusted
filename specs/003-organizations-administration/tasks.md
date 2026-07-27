# Tasks: Organizations & Administration (As-Built + Multi-Org FRs)

**Input**: Design documents from `specs/003-organizations-administration/`

**Focus**: Retrofit spec — tasks target gaps only:
- **New multi-org FRs**: FR-009, FR-010, FR-011, FR-012
- **Known Discrepancies**: KD-1 (admin auth - fixed, needs e2e), KD-2 (ApproveJoinRequest already-a-member), KD-3 (ListJoinRequests status filter)
- **Missing tests**: e2e coverage for admin auth, various rejection paths

**Tests**: Tests are OPTIONAL but recommended for gaps. Write failing tests first (TDD) where marked.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project already exists — no setup needed for this established domain

- [ ] T001 Verify `internal/service/org.go`, `internal/service/admin.go`, `internal/api/grpc/org.go`, `internal/api/grpc/admin.go` exist and compile

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Database schema and core models already exist — verify they support multi-org

- [ ] T002 [P] Verify `users_orgs` composite PK `(user_id, org_id)` supports FR-009 (multi-org membership) — check `podman/trusted-group/postgres/ubertool_schema_trusted.sql`
- [ ] T003 [P] Verify Redis key `user:{id}:current_org` with TTL=24h exists for FR-010 (org context)
- [ ] T004 [P] Verify `orgs.metro` column exists for FR-011 (cross-org metro filtering)
- [ ] T005 [P] Verify `tools.owner_org_id` FK to `orgs.id` exists for FR-011 (tools search)

---

## Phase 3: User Story 1 — Organization Discovery, Creation & Join (P1)

**Goal**: Verify existing US1 works + add FR-009 (multi-org membership) test coverage

**Independent Test**: Create org as user A (verify SUPER_ADMIN); user B joins via invite (verify MEMBER); both users can belong to multiple orgs simultaneously

### Tests for US1 (Recommended)

- [ ] T006 [P] [US1] **Contract test**: `CreateOrganization` → caller becomes SUPER_ADMIN with balance_cents=0 in `tests/integration/org_test.go`
- [ ] T007 [P] [US1] **Contract test**: `JoinOrganizationWithInvite` → caller becomes MEMBER with balance_cents=0, invitation marked used, admins notified in `tests/integration/org_test.go`
- [ ] T008 [P] [US1] **Integration test**: User belongs to 2+ orgs simultaneously — `ListMyOrganizations` returns both with correct role/balance each in `tests/integration/org_test.go`
- [ ] T009 [P] [US1] **E2E test**: `JoinOrganizationWithInvite` already-a-member rejection (FR-004 Scenario 6) in `tests/e2e/org_test.go`

### Implementation for US1 (Verify Existing)

- [ ] T010 [US1] Verify `CreateOrganization` inserts `organization` sets `role=SUPER_ADMIN`, `balance_cents=0` in `users_orgs` — no code change expected
- [ ] T011 [US1] Verify `JoinOrganizationWithInvite` notifies admins — no code change expected
- [ ] T012 [US1] **FR-009**: Add `ListMyOrganizations` test proving multi-org membership works (existing code supports it via composite PK)

---

## Phase 4: User Story 2 — Org Settings & Threshold Broadcast (P1)

**Goal**: Verify existing US2 works (already has unit tests for price-field gating)

**Independent Test**: ADMIN changes non-price fields (succeeds); ADMIN changes price fields (rejected); SUPER_ADMIN changes price fields (succeeds + broadcast to all active members)

### Tests for US2 (Recommended)

- [ ] T013 [P] [US2] **Integration test**: `UpdateOrganization` price-field change by SUPER_ADMIN → all active members receive notification+email+push in `tests/integration/org_test.go`
- [ ] T014 [P] [US2] **E2E test**: Non-member caller rejected (FR-002 Scenario 3) in `tests/e2e/org_test.go`

### Implementation for US2 (Verify Existing)

- [ ] T015 [US2] Verify `UpdateOrganization` price-field gating (SUPER_ADMIN only) — already tested in `TestOrganizationService_UpdateOrganization`
- [ ] T016 [US2] Verify broadcast on price change (background goroutine) — no code change expected

---

## Phase 5: User Story 3 — Admin Membership & Join-Request Management (P1)

**Goal**: KD-1 fix verified at e2e level; KD-2 (already-a-member guard); KD-3 (ListJoinRequests status filter)

**Independent Test**: MEMBER caller → all 8 AdminService RPCs rejected; ADMIN/SUPER_ADMIN caller → all 8 RPCs succeed

### Tests for US3 (Required — KD-1 needs e2e verification)

- [ ] T017 [P] [US3] **E2E test**: `AdminBlockUserAccount` rejects non-admin caller through full gRPC stack (live DB) — extends `TestAdminService_E2E` in `tests/e2e/admin_test.go` — **PRIORITY**
- [ ] T018 [P] [US3] **E2E test**: `ListJoinRequests` rejects non-admin caller in `tests/e2e/admin_test.go`
- [ ] T019 [P] [US3] **E2E test**: `SendInvitation` rejects non-admin caller in `tests/e2e/admin_test.go`
- [ ] T020 [P] [US3] **E2E test**: `ApproveJoinRequest` rejects non-admin caller in `tests/e2e/admin_test.go`
- [ ] T021 [P] [US3] **E2E test**: `RejectRequestToJoin` rejects non-admin caller in `tests/e2e/admin_test.go`
- [ ] T022 [P] [US3] **E2E test**: `GetMemberProfile` rejects non-admin caller in `tests/e2e/admin_test.go`
- [ ] T023 [P] [US3] **E2E test**: `SearchUsers` rejects non-admin caller in `tests/e2e/admin_test.go`
- [ ] T024 [P] [US3] **E2E test**: `ListMembers` rejects non-admin caller in `tests/e2e/admin_test.go`

### Implementation for US3 (Gap Fixes)

- [ ] T025 [US3] **KD-2**: Add already-a-member guard in `ApproveJoinRequest` → friendly error instead of PK violation — `internal/service/admin.go`
- [ ] T026 [US3] **KD-3**: Add `status = 'PENDING'` filter to `joinRequestRepository.ListByOrg` — `internal/repository/postgres/join_request.go:65-87`
- [ ] T027 [P] [US3] **Unit test**: `ApproveJoinRequest` already-a-member rejection in `tests/unit/admin_service_test.go`
- [ ] T028 [P] [US3] **Integration test**: `ListJoinRequests` returns only PENDING after KD-3 fix in `tests/integration/admin_join_request_test.go`

---

## Phase 6: User Story 4 — Multi-Org Context & Cross-Org Search (P1 — NEW FRs)

**Goal**: Implement FR-010 (org context), FR-011 (cross-org search), FR-012 (rental context switch)

**Independent Test**: User in Org A and Org B (same metro) — search returns tools from both; rental from Org B tool prompts context switch to Org B

### Tests for US4 (Required — New Functionality)

- [ ] T029 [P] [US4] **Contract test**: `SetCurrentOrganization` stores org in Redis with TTL=24h in `tests/integration/org_test.go`
- [ ] T030 [P] [US4] **Contract test**: `GetCurrentOrganization` returns stored org or first active org as fallback in `tests/integration/org_test.go`
- [ ] T031 [P] [US4] **Integration test**: `SearchOrganizations` / `SearchTools` cross-org — user in Org A + Org B (same metro) gets tools from both in `tests/integration/tool_test.go` (ref Organizations FR-011)
- [ ] T032 [P] [US4] **E2E test**: Rental context switch prompt — renter in Org A, tool owner in Org B → `CreateRentalRequest` returns `context_switch_required=true` + target org in `tests/e2e/rental_test.go` (ref Rentals FR-008)
- [ ] T033 [US4] **E2E test**: After `SetCurrentOrganization`, subsequent operations use new org context in `tests/e2e/org_test.go`

### Implementation for US4 (New Multi-Org FRs)

- [ ] T034 [US4] **FR-010**: Add `SetCurrentOrganization` / `GetCurrentOrganization` RPCs to `OrganizationService` — `internal/service/org.go`, `internal/api/grpc/org.go`, `organization_service.proto`
- [ ] T035 [US4] **FR-010**: Redis helper `SetCurrentOrg(userID, orgID)` / `GetCurrentOrg(userID)` — `internal/repository/redis/org_context.go`
- [ ] T036 [US4] **FR-010**: JWT interceptor populates `current_org_id` in context from Redis for downstream services — `internal/api/grpc/interceptors.go`
- [ ] T037 [US4] **FR-011**: Modify `SearchTools` to use caller's current org metro + include all user's orgs in that metro — `internal/service/tool.go` (in 006-tools-image-storage)
- [ ] T038 [US4] **FR-012**: `CreateRentalRequest` checks `tool.owner_org_id != current_org` → returns `context_switch_required` + target org — `internal/service/rental.go` (in 005-rentals)
- [ ] T039 [US4] **FR-012**: `CreateRentalRequest` accepts optional `current_organization_id` override for retry after switch — `rental_service.proto`, `internal/service/rental.go`
- [ ] T040 [P] [US4] **Proto regeneration**: `make proto` after T034, T039 changes

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Remaining gaps, documentation, regression locks

- [ ] T041 [P] Document `SearchOrganizations` public admin exposure (Known Discrepancy 1 edge case) — update spec or restrict to authenticated
- [ ] T042 [P] **Regression test**: `SearchOrganizations` unauthenticated access still works in `tests/e2e/org_test.go`
- [ ] T043 [P] Run full test suite: `make test-unit && make test-integration && make test-e2e` — all green
- [ ] T044 Update `sbr/rtm/003-organizations-administration.rtm.md` after KD-2, KD-3 fixes

---

## Dependencies & Execution Order

### Phase Dependencies
- **Phase 1-2**: No deps — can start immediately
- **Phase 3-5**: Depend on Phase 2 (schema verification)
- **Phase 6**: Depends on Phase 2 (Redis key, metro column) + Phase 5 (admin auth fixed)
- **Phase 7**: Depends on all prior phases

### Within-Phase Parallelism
- All `[P]` tasks in same phase can run in parallel (different files)
- T017-T024 (e2e admin auth) can run in parallel once Podman DB is up
- T029-T033 (US4 tests) can run in parallel with T034-T039 (US4 impl)

### User Story Dependencies
- US1 (P1): Independent — no deps on other stories
- US2 (P1): Independent — no deps on other stories
- US3 (P1): KD-1 fix must be e2e-verified before considering US3 done
- US4 (P1): Depends on Redis context (T035-T036) + Tools/Rentals impl (T037, T038)

### MVP Scope
- **MVP = US1 + US2 + US3 (with KD-1 e2e verified) + US4 FR-010**
- FR-011 (cross-org search) lives in Tools feature (006) — coordinate
- FR-012 (rental context switch) lives in Rentals feature (005) — coordinate

---

## Independent Test Criteria Per Story

| Story | Independent Test |
|-------|------------------|
| US1 | User A creates org → User B joins via invite → both in org; User A also in Org B → `ListMyOrganizations` shows both |
| US2 | ADMIN changes name (ok); ADMIN changes threshold (rejected); SUPER_ADMIN changes threshold (ok + broadcast) |
| US3 | Non-admin caller → all 8 RPCs rejected (unit + e2e); Admin caller → all 8 RPCs succeed |
| US4 | User in Org A + Org B (same metro); search returns tools from both; rental from Org B tool → context switch prompt |

---

## Notes

- **Retrofit discipline**: Do NOT re-implement US1-US3 happy paths — they exist. Tasks verify + close gaps only.
- **Cross-feature FRs**: FR-011 implemented in 006-tools-image-storage (T037). FR-012 implemented in 005-rentals (T038-T039).
- **Proto changes**: FR-010 adds 2 RPCs; FR-012 adds field to `CreateRentalRequest`. Run `make proto` after T034, T039.
- **Constitution**: Principle III (Push Notifications) — FR-010 context switch should notify user; Principle VI (Proto-First) — all new RPCs/fields in proto first.