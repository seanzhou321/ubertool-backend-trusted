# Tasks: Tools & Image Storage (As-Built + Multi-Org FR-008, FR-009)

**Input**: Design documents from `specs/006-tools-image-storage/`

**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/, quickstart.md

**Tests**: Include test tasks for all gaps (Known Discrepancies + new FR-008, FR-009). Tests are OPTIONAL for happy paths that already have coverage.

**Organization**: Tasks grouped by user story to enable independent implementation and testing of each story.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Verify project structure and dependencies for Tools + Image Storage domain

- [ ] T001 Verify `internal/service/tool.go`, `internal/service/image_storage.go`, `internal/api/grpc/tool.go`, `internal/api/grpc/image_storage.go` exist per plan.md
- [ ] T002 Verify `tools` + `tool_images` tables in `podman/trusted-group/postgres/ubertool_schema_trusted.sql` match data-model.md
- [ ] T003 [P] Verify `tool_service.proto` + `image_storage_service.proto` match contracts/README.md (run `make proto`)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented

- [ ] T004 [P] Verify `ToolStatus` constants in `internal/domain/tool.go` (AVAILABLE, RENTED, MAINTENANCE, RETIRED)
- [ ] T005 [P] Verify `tool.owner_org_id` FK to `orgs.id` exists for FR-008, FR-009
- [ ] T006 [P] Verify Redis `user:{id}:current_org` key exists (Organizations FR-010) for FR-008 metro resolution
- [ ] T007 [P] Run `make test-unit` — confirm existing unit tests pass (baseline)

**Checkpoint**: Foundation ready — user story implementation can now begin in parallel

---

## Phase 3: User Story 1 — Tool Listing Ownership & Lifecycle (Priority: P1)

**Goal**: KD-1 fix (ownership checks on Update/Delete) verified at e2e level; verify AddTool uses JWT owner_id

**Independent Test**: User A creates tool → User B cannot update/delete → User A can

### Tests for User Story 1 (Required for KD-1 e2e Verification)

- [ ] T008 [P] [US1] **E2E test**: `UpdateTool` by non-owner → rejected through full gRPC stack in `tests/e2e/tool_test.go`
- [ ] T009 [P] [US1] **E2E test**: `DeleteTool` by non-owner → rejected through full gRPC stack in `tests/e2e/tool_test.go`
- [ ] T010 [P] [US1] **E2E test**: `UpdateTool`/`DeleteTool` by actual owner → succeeds in `tests/e2e/tool_test.go`
- [ ] T011 [P] [US1] **Integration test**: `AddTool` sets `owner_id` from JWT, `owner_org_id` from Redis current org in `tests/integration/tool_test.go`
- [ ] T012 [P] [US1] **Unit test**: `ListMyTools` returns only caller's tools in `tests/unit/tool_service_test.go`

### Implementation for User Story 1 (Verify Existing)

- [ ] T013 [US1] Verify `UpdateTool`/`DeleteTool` ownership check (KD-1 fix) — `internal/service/tool.go` (already done)
- [ ] T014 [US1] Verify `AddTool` owner_id from JWT, owner_org_id from Redis — `internal/service/tool.go`
- [ ] T015 [US1] Verify `ListMyTools` filters by owner_id — no code change expected

**Checkpoint**: US1 complete when T008-T011 pass (KD-1 e2e verified)

---

## Phase 4: User Story 2 — Tool Discovery (Priority: P1)

**Goal**: Verify existing search + implement FR-008 (cross-org search) + FR-009 (shared-org owner filtering)

**Independent Test**: User in Org A + Org B (same metro) → `SearchTools` returns tools from both; owner orgs filtered to shared orgs only

### Tests for User Story 2 (Required for New FRs + KD-2)

- [ ] T016 [P] [US2] **Contract test**: `SearchTools` with `include_all_my_orgs=true` returns tools from all user's orgs in metro in `tests/integration/tool_test.go`
- [ ] T017 [P] [US2] **Contract test**: `SearchTools` with `include_all_my_orgs=false` (default) returns only current org tools in `tests/integration/tool_test.go`
- [ ] T018 [P] [US2] **Integration test**: `GetTool` owner_organizations only includes orgs shared between caller + tool owner in `tests/integration/tool_test.go`
- [ ] T019 [P] [US2] **E2E test**: `SearchTools` cross-org flow: user in 2 orgs same metro → include_all_my_orgs=true gets both in `tests/e2e/tool_test.go`
- [ ] T020 [P] [US2] **E2E test**: `GetTool` shared-org filtering — owner in Org A + Org B, caller in Org A only → only Org A returned in `tests/e2e/tool_test.go`
- [ ] T021 [P] [US2] **E2E test**: `GetToolImages` by non-owner, tool AVAILABLE → allowed (or document KD-2) in `tests/e2e/tool_test.go`
- [ ] T022 [P] [US2] **Unit test**: `ListToolCategories` matches actual tools.categories data (KD-4) in `tests/unit/tool_service_test.go`

### Implementation for User Story 2 (New FRs + KD Decisions)

- [ ] T023 [US2] **FR-008**: Add `include_all_my_orgs` field to `SearchToolsRequest` proto — `tool_service.proto`
- [ ] T024 [US2] **FR-008**: In `toolService.SearchTools`: when `include_all_my_orgs=true`, query tools from ALL user's active orgs in current org's metro — `internal/service/tool.go`
- [ ] T025 [US2] **FR-008**: Metro resolution: use Redis `current_org` → `orgs.metro` (Organizations FR-010) — `internal/service/tool.go`
- [ ] T026 [US2] **FR-009**: In `toolService.GetTool`: populate `owner_organizations` = intersection of caller's orgs ∩ tool owner's orgs — `internal/service/tool.go`
- [ ] T027 [US2] **FR-009**: Add `owner_organizations` (repeated OrganizationSummary) to `Tool` proto message — `tool_service.proto`
- [ ] T028 [P] [US2] **Proto regeneration**: `make proto` after T023, T027
- [ ] T029 [US2] **KD-2 Decision**: Implement access check in `GetToolImages` (owner or AVAILABLE) OR update doc — `internal/service/image_storage.go`
- [ ] T030 [P] [US2] **Test for KD-2**: After T029, test passes in `tests/unit/image_storage_service_test.go`
- [ ] T031 [US2] **KD-4 Decision**: Implement `ListToolCategories` as `DISTINCT categories` query OR document as static list — `internal/service/tool.go`
- [ ] T032 [P] [US2] **Test for KD-4**: After T031, test passes in `tests/unit/tool_service_test.go`

**Checkpoint**: US2 complete when FR-008, FR-009 tests (T016-T020) pass + KD-2, KD-4 decisions implemented

---

## Phase 5: User Story 3 — Image Upload Pipeline (Priority: P2)

**Goal**: Verify presigned upload → confirm → thumbnail pipeline works; FR-004 ownership checks regression-locked

**Independent Test**: Owner gets upload URL → uploads to S3 → confirms → thumbnail generated async

### Tests for User Story 3 (Verify Existing + Regression Locks)

- [ ] T033 [P] [US3] **E2E test**: Full upload pipeline: `GetUploadUrl` → S3 put → `ConfirmImageUpload` → thumbnail in `tests/e2e/image_storage_test.go`
- [ ] T034 [P] [US3] **Unit test**: `GetUploadUrl`/`ConfirmImageUpload`/`DeleteImage`/`SetPrimaryImage` by non-owner → rejected in `tests/unit/image_storage_service_test.go` (FR-004 regression)
- [ ] T035 [P] [US3] **Unit test**: `GetDownloadUrl` by owner → allowed; by non-owner + tool AVAILABLE → allowed; by non-owner + tool RENTED → rejected in `tests/unit/image_storage_service_test.go` (FR-005 regression)
- [ ] T036 [P] [US3] **Integration test**: Thumbnail generation async (doesn't block RPC) in `tests/integration/image_storage_test.go`

### Implementation for User Story 3 (Verify Existing)

- [ ] T037 [US3] Verify `GetUploadUrl` creates PENDING row + presigned URLs — no code change expected
- [ ] T038 [US3] Verify `ConfirmImageUpload` validates file exists, marks CONFIRMED, launches thumbnail goroutine — no code change expected
- [ ] T039 [US3] Verify thumbnail goroutine uses `context.WithoutCancel` (doesn't fail RPC) — no code change expected

**Checkpoint**: US3 complete when T033-T036 pass (regression locks)

---

## Phase 6: User Story 4 — Image Management (Priority: P2)

**Goal**: Verify DeleteImage/SetPrimaryImage/GetToolImages; KD-2 (GetToolImages access) decision

**Independent Test**: Owner uploads 2 images → promotes second to primary → deletes first → oldest remaining promoted

### Tests for User Story 4 (Required for Gaps)

- [ ] T040 [P] [US4] **Unit test**: `DeleteImage` by non-owner → rejected in `tests/unit/image_storage_service_test.go`
- [ ] T041 [P] [US4] **Unit test**: `SetPrimaryImage` swaps primary in transaction (unique constraint never violated) in `tests/unit/image_storage_service_test.go`
- [ ] T042 [P] [US4] **Unit test**: `SetPrimaryImage` rejects if target image not CONFIRMED or belongs to different tool in `tests/unit/image_storage_service_test.go`
- [ ] T043 [P] [US4] **Integration test**: `GetToolImages` returns CONFIRMED images primary-first in `tests/integration/image_storage_test.go`
- [ ] T044 [P] [US4] **E2E test**: Image management flow in `tests/e2e/image_storage_test.go`

### Implementation for User Story 4 (Verify Existing)

- [ ] T045 [US4] Verify `DeleteImage` soft-deletes + promotes oldest if primary — no code change expected
- [ ] T046 [US4] Verify `SetPrimaryImage` uses transaction for atomic swap — no code change expected
- [ ] T047 [US4] **KD-2**: See T029-T030 (US2)

**Checkpoint**: US4 complete when T040-T044 pass

---

## Phase 7: Known Discrepancy 3 — Disabled E2E Test

**Goal**: Enable `tests/e2e/search_tools_shared_org_test.go_` (remove trailing underscore)

### Implementation

- [ ] T048 [KD3] Rename `tests/e2e/search_tools_shared_org_test.go_` → `tests/e2e/search_tools_shared_org_test.go`
- [ ] T049 [P] [KD3] Run `make test-e2e` — confirm test passes (shared-org filtering at e2e level)

**Checkpoint**: KD-3 complete when test runs green

---

## Phase 8: Known Discrepancy 5 — Debug Logging

**Goal**: Replace `fmt.Printf` with structured `internal/logger` in `toolService.SearchTools` and `imageStorageService.DeleteImage`

### Implementation

- [ ] T050 [KD5] Replace `fmt.Printf("DEBUG ...")` in `SearchTools` with `logger.Debug()` — `internal/service/tool.go`
- [ ] T051 [KD5] Replace `fmt.Printf("Warning: ...")` in `DeleteImage` with `logger.Warn()` — `internal/service/image_storage.go`
- [ ] T052 [P] [KD5] Verify no `fmt.Printf` remains in `tool.go` or `image_storage.go` — `grep -r "fmt.Printf" internal/service/tool.go internal/service/image_storage.go`

**Checkpoint**: KD-5 complete when T052 passes (no fmt.Printf in these files)

---

## Phase 9: Polish & Cross-Cutting Concerns

**Purpose**: Remaining gaps, documentation, regression locks

- [ ] T053 [P] Run full test suite: `make test-unit && make test-integration && make test-e2e` — all green
- [ ] T054 Update `sbr/rtm/006-tools-image-storage.rtm.md` after FR-008, FR-009, KD-2, KD-4 fixes
- [ ] T055 [P] Quickstart validation: run `specs/006-tools-image-storage/quickstart.md` scenarios manually

---

## Dependencies & Execution Order

### Phase Dependencies
- **Phase 1-2**: No deps — start immediately
- **Phase 3-4**: Depend on Phase 2
- **Phase 5-6**: Depend on Phase 2
- **Phase 7-8**: Independent — can run in parallel
- **Phase 9**: Depends on all prior phases

### Within-Phase Parallelism
- All `[P]` tasks in same phase = parallel (different files)
- T008-T012 (US1 e2e/integration) parallel
- T016-T022 (US2 new FR + KD tests) parallel
- T033-T036 (US3 regression tests) parallel
- T040-T044 (US4 tests) parallel

### Cross-Feature Dependencies
| This Feature | Depends On | Coordination |
|--------------|------------|--------------|
| FR-008 (cross-org search) | Organizations FR-010 (Redis current_org) | Must deploy 003 FR-010 first |
| FR-008 metro resolution | Organizations FR-011 (metro per org) | Uses `orgs.metro` |
| FR-009 (shared orgs) | Users FR-001/FR-002 (multi-org membership) | Uses `users_orgs` composite PK |

### MVP Scope
- **MVP = US1 (KD-1 e2e) + US2 (FR-008, FR-009) + US3 (FR-004/005 regression) + KD-3 (disabled test)**
- KD-2 (GetToolImages access) + KD-4 (ListCategories) + KD-5 (logging) = follow-up polish

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
- **FR-008 (cross-org search)**: Adds `include_all_my_orgs` to proto. Requires Organizations FR-010 (Redis context) deployed first.
- **FR-009 (shared-org owner info)**: Adds `owner_organizations` to `Tool` proto. Computed as intersection of caller's orgs ∩ tool owner's orgs.
- **KD-2 (GetToolImages)**: Lower severity (read-only metadata). Decision: implement access check OR document as public.
- **KD-4 (ListCategories)**: Low severity. Decision: query DB OR document as static UX list.
- **KD-5 (logging)**: Code quality, not functional. Replace `fmt.Printf` with `internal/logger`.
- **Proto changes**: FR-008 adds field to `SearchToolsRequest`. FR-009 adds `owner_organizations` to `Tool`. Run `make proto` after T023, T027.
- **Constitution**: Principle III (Push) — ConfirmImageUpload launches thumbnail async; Principle VI (Proto-First) — all new fields in proto first.