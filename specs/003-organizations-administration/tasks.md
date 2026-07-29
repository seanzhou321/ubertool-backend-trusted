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
- [x] T003 [P] FR-010 (server-side "current org" context) REMOVED — see `docs/design/multi-org.md`. "Current org" is a client/device-local UI preference only; the server never caches or persists it, and there is no Redis dependency.
- [ ] T004 [P] Verify `orgs.metro` column exists for FR-011 (cross-org metro filtering)
- [x] T005 [P] `tools.owner_org_id` does NOT exist and must not be added — tools are owned by users (`tools.owner_id`) and metro-scoped (`tools.metro`), not org-scoped. FR-011 cross-org search is implemented via `toolService.SearchTools` + `getSharedOrganizations` (`internal/service/tool.go`).

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

**Goal**: FR-011 (cross-org search) and FR-012 (rental org-context validation). FR-010
(server-side "current org" context) was REMOVED 2026-07-28 — see `docs/design/multi-org.md` —
a user's "current org" is a client/device-local UI preference, not server state, so there is
nothing to build for it here.

**Independent Test**: User in Org A and Org B (same metro) — search returns tools from both;
`CreateRentalRequest` for a Org-B-owned tool succeeds when the caller passes `organization_id`
for a shared org, and fails with `FAILED_PRECONDITION` + shared-org list otherwise.

### Tests for US4 (Required — New Functionality)

- [x] ~~T029~~ / ~~T030~~ Dropped — `SetCurrentOrganization`/`GetCurrentOrganization` were removed, not built.
- [ ] T031 [P] [US4] **Integration test**: `SearchOrganizations` / `SearchTools` cross-org — user in Org A + Org B (same metro) gets tools from both in `tests/integration/tool_test.go` (ref Organizations FR-011)
- [ ] T032 [P] [US4] **E2E test**: `CreateRentalRequest` for a tool whose owner is only in Org B, called with `organization_id=Org_B` (a shared org), succeeds; called with a non-shared org returns `FAILED_PRECONDITION` + shared-org list in `tests/e2e/rental_test.go` (ref Rentals FR-008)
- [ ] T033 Dropped — no server-side org context to switch.

### Implementation for US4 (New Multi-Org FRs)

- [x] ~~T034~~ / ~~T035~~ / ~~T036~~ Dropped — no `SetCurrentOrganization`/`GetCurrentOrganization` RPCs, no Redis org-context repository, no `current-org-id` interceptor metadata. Every request that needs an org context takes `organization_id` explicitly as a request field.
- [x] T037 [US4] **FR-011**: `SearchTools` filters by `tools.metro` (tools are metro-scoped, not org-scoped — no `owner_org_id`), then post-filters per-tool by shared active orgs between owner and requester (`getSharedOrganizations`) — `internal/service/tool.go` (006-tools-image-storage)
- [x] T038 [US4] **FR-012**: `CreateRentalRequest` validates the caller-supplied `organization_id` via `isSharedOrganization` (both renter and tool owner must be active members); rejects with `FAILED_PRECONDITION` + shared-org list on mismatch — `internal/service/rental.go` (005-rentals). There is no `tool.owner_org_id` to compare against and no `context_switch_required` error code.
- [x] ~~T039~~ Dropped — no `current_organization_id` override; the client always supplies `organization_id` directly and retries with a different one if rejected.
- [ ] T040 [P] [US4] **Proto regeneration**: `make proto-gen` after any further proto changes

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

## Phase 8: Convergence

**Assessment Date**: 2026-07-27
**Assessed Against**: spec.md, plan.md, tasks.md, constitution.md
**Codebase Scope**: `internal/service/org.go`, `internal/service/admin.go`, `internal/api/grpc/org.go`, `internal/api/grpc/admin.go`, `internal/repository/postgres/join_request.go`, `podman/trusted-group/postgres/ubertool_schema_trusted.sql`

### Convergence Findings

| ID | Gap Type | Severity | Source | Evidence | Remaining Work |
|----|----------|----------|--------|----------|----------------|
| C1 | N/A (was: missing) | — | FR-010 (US4) | `SetCurrentOrganization`/`GetCurrentOrganization` + Redis were implemented 2026-07-25 then reverted 2026-07-28 — a user's "current org" is a client/device-local UI preference the server must not cache (see `docs/design/multi-org.md`) | None — do not reimplement |
| C2 | missing | HIGH | FR-011 (US4) | `SearchTools` in tools feature searches single org metro only; no `include_all_my_orgs` param | Cross-org search lives in 006-tools (see C6 below) |
| C3 | missing | HIGH | FR-012 (US4) | `CreateRentalRequest` in rentals has no `current_organization_id` or context switch logic | Rental context switch lives in 005-rentals (see C8 below) |
| C4 | missing | MEDIUM | KD-2 (US3) | `ApproveJoinRequest` has already-member check at line 266 but returns generic error; no friendly message | Improve error message: "You are already a member of this organization" |
| C5 | missing | MEDIUM | KD-3 (US3) | `joinRequestRepository.ListByOrg` (join_request.go:87) lacks `status = 'PENDING'` filter; returns all statuses | Add `AND jr.status = 'PENDING'` to query |
| C6 | missing | LOW | FR-009 (US1) | Multi-org membership works via composite PK but no test proving it | Add integration test: user in 2+ orgs → `ListMyOrganizations` returns both |
| C7 | N/A | — | Constitution III | Moot — there is no server-side "context switch" event to notify about; org focus changes entirely on the client/device | None |

### Summary
- Requirements checked: 12 FRs, 4 US acceptance scenarios, 3 KDs
- Constitution principles checked: 6 (I-VI)
- Findings: 7 (2 missing P1 FRs in cross-features, 2 KD gaps, 3 test/quality)
- **Status**: NOT converged — 2 P1 FRs depend on cross-feature impl (005, 006); 2 KD fixes needed locally

### Appended Convergence Tasks

- [x] ~~T045~~ Built 2026-07-25, reverted 2026-07-28 — `SetCurrentOrganization`/`GetCurrentOrganization` do not exist. Do not reimplement.
- [x] ~~T046~~ Built 2026-07-25, reverted 2026-07-28 — no Redis dependency anywhere in this service. Do not reimplement.
- [x] ~~T047~~ Built 2026-07-25, reverted 2026-07-28 — no `current_org_id`/`current-org-id` interceptor metadata. Every request takes `organization_id` explicitly. Do not reimplement.
- [x] T048 [C4] **KD-2**: Improve `ApproveJoinRequest` already-member error message to "You are already a member of this organization" — `internal/service/admin.go:266` (partial)
- [x] T049 [C5] **KD-3**: Add `AND jr.status = 'PENDING'` filter to `joinRequestRepository.ListByOrg` — `internal/repository/postgres/join_request.go:87` (missing)
- [x] T050 [C6] **FR-009**: Integration test proving multi-org membership via `ListMyOrganizations` — `tests/integration/org_test.go` (missing)
- [x] ~~T051~~ N/A — no `SetCurrentOrganization` exists to attach a notification to.
- [x] T052 [C2,C3] **Cross-feature**: Verify 006-tools FR-011 and 005-rentals FR-012 implementations before marking US4 complete (tracking)
- [x] T053 Run `make test-unit - [ ] T053 Run `make test-unit && make test-integration && make test-e2e` — all green (verify)- [ ] T053 Run `make test-unit && make test-integration && make test-e2e` — all green (verify) make test-integration - [ ] T053 Run `make test-unit && make test-integration && make test-e2e` — all green (verify)- [ ] T053 Run `make test-unit && make test-integration && make test-e2e` — all green (verify) make test-e2e` — all green (verify)
- [ ] T054 Update `sbr/rtm/003-organizations-administration.rtm.md` after C4, C5 fixes