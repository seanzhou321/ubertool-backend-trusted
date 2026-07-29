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

- [ ] T004 [P] Verify `ToolStatus` constants in `internal/domain/tool.go` (`AVAILABLE`, `UNAVAILABLE`, `RENTED` — not `MAINTENANCE`/`RETIRED`, see `docs/design/multi-org.md` correction note in data-model.md)
- [x] T005 [P] `tool.owner_org_id` does NOT exist and must not be added — see `docs/design/multi-org.md`. FR-008/FR-009 are implemented without it (metro + per-tool shared-org filter).
- [x] T006 [P] Organizations FR-010 (Redis `user:{id}:current_org`) was REMOVED — dropped, not a dependency. `SearchTools` takes `metro` (or an `organization_id` to resolve one from) as an explicit request parameter instead.
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
- [ ] T011 [P] [US1] **Integration test**: `AddTool` sets `owner_id` from JWT only — no org is recorded on the tool — in `tests/integration/tool_test.go`
- [ ] T012 [P] [US1] **Unit test**: `ListMyTools` returns only caller's tools in `tests/unit/tool_service_test.go`

### Implementation for User Story 1 (Verify Existing)

- [ ] T013 [US1] Verify `UpdateTool`/`DeleteTool` ownership check (KD-1 fix) — `internal/service/tool.go` (already done)
- [ ] T014 [US1] Verify `AddTool` sets `owner_id` from JWT only (no org field exists on `Tool` to set) — `internal/service/tool.go`
- [ ] T015 [US1] Verify `ListMyTools` filters by owner_id — no code change expected

**Checkpoint**: US1 complete when T008-T011 pass (KD-1 e2e verified)

---

## Phase 4: User Story 2 — Tool Discovery (Priority: P1)

**Goal**: Verify existing search + implement FR-008 (cross-org search) + FR-009 (shared-org owner filtering)

**Independent Test**: User in Org A + Org B (same metro) → `SearchTools` returns tools owned by
users in either org (excluding the caller's own tools); owner orgs filtered to shared orgs only

### Tests for User Story 2 (Required for New FRs + KD-2)

- [x] ~~T016~~ / ~~T017~~ Dropped — there is no `include_all_my_orgs` flag. `SearchTools` is always metro-scoped (no single-org "default" to expand from) plus the FR-009 shared-org filter; see T024.
- [ ] T018 [P] [US2] **Integration test**: `GetTool` response's `tool.owner.orgs` (the pre-existing `User.orgs` field — no `owner_organizations` field exists) only includes orgs shared between caller + tool owner in `tests/integration/tool_test.go`
- [ ] T019 [P] [US2] Dropped along with T016/T017 (no `include_all_my_orgs`) — replaced by: **E2E test**: `SearchTools` in a shared metro returns tools from every org member the caller shares an org with, and excludes tools whose owner shares none, in `tests/e2e/tool_test.go`
- [ ] T020 [P] [US2] **E2E test**: `GetTool` shared-org filtering — owner in Org A + Org B, caller in Org A only → only Org A returned in `tests/e2e/tool_test.go`
- [ ] T021 [P] [US2] **E2E test**: `GetToolImages` by non-owner, tool AVAILABLE → allowed (or document KD-2) in `tests/e2e/tool_test.go`
- [ ] T022 [P] [US2] **Unit test**: `ListToolCategories` matches actual tools.categories data (KD-4) in `tests/unit/tool_service_test.go`

### Implementation for User Story 2 (New FRs + KD Decisions)

- [x] ~~T023~~ Dropped — no `include_all_my_orgs` field; `SearchTools` already takes `metro` (and `orgID` to resolve one) as explicit request parameters.
- [x] T024 [US2] **FR-008**: `toolService.SearchTools` filters by `tools.metro` (tools are metro-scoped, never org-scoped) plus status/owner exclusion, then post-filters each result by shared active orgs via `getSharedOrganizations` — `internal/service/tool.go`
- [x] ~~T025~~ Dropped along with Organizations FR-010 — no Redis `current_org` to resolve metro from. The caller passes `metro` directly, or `orgID` (an org they're an active member of) whose `metro` the service looks up.
- [x] T026 [US2] **FR-009**: `toolService.GetTool`/`populateToolOwner` populate `owner.Orgs` (the domain field behind the pre-existing `User.orgs` proto field) = intersection of caller's orgs ∩ tool owner's orgs, via `getSharedOrganizations` — `internal/service/tool.go`
- [x] ~~T027~~ Dropped — no `owner_organizations`/`OrganizationSummary` field was added; `Tool.owner` already carries `User.orgs`, and the existing mapper (`internal/api/grpc/mapper.go`) already serializes it.
- [x] ~~T028~~ Dropped along with T027 — no proto change was made.
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
| FR-008 (cross-org search) | None (Organizations FR-010 was removed) | Metro + shared-org filter, no server-side "current org" needed |
| FR-008 metro resolution | Organizations FR-011 (metro per org) | Uses `orgs.metro` |
| FR-009 (shared orgs) | Users FR-001/FR-002 (multi-org membership) | Uses `users_orgs` composite PK |

### MVP Scope
- **MVP = US1 (KD-1 e2e) + US2 (FR-008, FR-009) + US3 (FR-004/005 regression) + KD-3 (disabled test)**
- KD-2 (GetToolImages access) + KD-4 (ListCategories) + KD-5 (logging) = follow-up polish

---

## Independent Test Criteria Per Story

| Story | Independent Test |
|-------|------------------|
| US1 | Non-owner Update/Delete rejected (e2e); AddTool uses JWT owner_id only — no org field |
| US2 | `SearchTools` (metro-scoped + shared-org filter) returns tools from every org the caller shares with an owner in that metro; `GetTool`/`SearchTools` `Tool.owner.orgs` = shared orgs only |
| US3 | Upload → confirm → thumbnail async; ownership checks on all image mutations; GetDownloadUrl owner-or-AVAILABLE |
| US4 | Delete promotes oldest; SetPrimaryImage atomic swap; GetToolImages CONFIRMED primary-first |

---

## Phase 10: Convergence

### Convergence Findings

| ID | Gap Type | Severity | Source | Evidence | Remaining Work |
|----|----------|----------|--------|----------|----------------|
| C1 | resolved | — | FR-008 | `SearchTools` is metro-scoped (no org-scoping to begin with) plus a per-tool shared-org post-filter (`getSharedOrganizations`) — no `include_all_my_orgs` param was needed | `internal/service/tool.go` |
| C2 | resolved | — | FR-009 | No `owner_organizations` field exists or is needed — `populateToolOwner` sets `Owner.Orgs`, and the existing mapper (`internal/api/grpc/mapper.go`, `MapDomainUserToProto` used for `Tool.owner`) already serializes it to the wire as `tool.owner.orgs` | `internal/service/tool.go`, `internal/api/grpc/mapper.go` |
| C3 | missing | HIGH | KD-2 | `GetToolImages` has NO access check (any auth user can list images) | Add owner-or-AVAILABLE check or document as public |
| C4 | missing | MEDIUM | KD-4 | `ListToolCategories` returns hardcoded list, not `DISTINCT categories` from DB | Query DB or document as static UX list |
| C5 | missing | LOW | KD-5 | `fmt.Printf` debug logging in `tool.go` (25 lines) and `image_storage.go` (2 lines) | Replace with `internal/logger` |
| C6 | missing | MEDIUM | KD-3 | `tests/e2e/search_tools_shared_org_test.go_` disabled (trailing underscore) | Rename to enable e2e test |
| C7 | N/A | — | FR-008 | `SearchTools` takes `metro` directly (or `orgID` to resolve one) as an explicit request parameter — there is no server-side "current org" to fall back to, by design | `docs/design/multi-org.md` |
| C8 | N/A | — | FR-009 | There is no `owner_org_id` field on `Tool` at all — `AddTool` sets only `owner_id` from the JWT | `internal/domain/tool.go` |
| C9 | missing | LOW | Constitution III | `ConfirmImageUpload` launches thumbnail async but no push notification on completion | Add push notification |

### Summary
- Requirements checked: 9 FRs, 4 US acceptance scenarios, 5 KDs
- Constitution principles checked: 6 (I-VI)
- Findings: 9 (2 CRITICAL new FR gaps, 1 HIGH KD, 3 MEDIUM, 3 LOW)
- **Status**: NOT converged — FR-008/FR-009 blocked on 003 FR-010; proto changes needed; 1 HIGH KD needs decision

### Appended Convergence Tasks

- [x] ~~T056~~ Dropped — no `include_all_my_orgs` field was added to `SearchToolsRequest`.
- [x] T057 [C1] **FR-008**: `toolService.SearchTools` filters by `tools.metro` then post-filters per-tool by shared active orgs (`getSharedOrganizations`) — `internal/service/tool.go`
- [x] ~~T058~~ Dropped — no Redis `current_org`/003 FR-010 dependency. The caller supplies `metro` directly, or an `orgID` whose `metro` the service resolves.
- [x] ~~T059~~ Dropped — no `owner_organizations`/`OrganizationSummary` field was added; `Tool.owner` already carries `User.orgs`.
- [x] T060 [C2] **FR-009**: `toolService.GetTool`/`populateToolOwner` populate `owner.Orgs` = intersection of caller's orgs ∩ tool owner's orgs — `internal/service/tool.go` (done, exposed via existing mapper — see C2)
- [x] ~~T061~~ Dropped — no proto change was made for FR-008/FR-009.
- [ ] T062 [C3] **KD-2**: Decision + implement: add access check to `GetToolImages` (owner or AVAILABLE) OR document as public — `internal/service/image_storage.go` (missing)
- [ ] T063 [C3] **KD-2**: Test for KD-2 decision — `tests/unit/image_storage_service_test.go` (missing)
- [ ] T064 [C4] **KD-4**: Decision + implement: query `DISTINCT categories` from `tools` table OR document as static UX list — `internal/service/tool.go` (missing)
- [ ] T065 [C4] **KD-4**: Test for KD-4 decision — `tests/unit/tool_service_test.go` (missing)
- [ ] T066 [C5] **KD-5**: Replace all `fmt.Printf` in `tool.go` with `logger.Debug()`/`logger.Error()` — `internal/service/tool.go` (missing)
- [ ] T067 [C5] **KD-5**: Replace `fmt.Printf` in `image_storage.go` with `logger.Warn()` — `internal/service/image_storage.go` (missing)
- [ ] T068 [C6] **KD-3**: Rename `tests/e2e/search_tools_shared_org_test.go_` → `tests/e2e/search_tools_shared_org_test.go` (missing)
- [x] ~~T069~~ N/A — there is no `owner_org_id` field to set; `AddTool` sets only `owner_id` from the JWT.
- [ ] T070 [C9] **Constitution III**: Push notification on thumbnail generation completion — `internal/service/image_storage.go` (missing)
- [ ] T071 Run `make test-unit && make test-integration && make test-e2e` — all green (verify)
- [ ] T072 Update `sbr/rtm/006-tools-image-storage.rtm.md` after C1, C2, C3, C4, C5 fixes

---

## Notes

- **Retrofit discipline**: Do NOT re-implement US1-US4 happy paths — they exist. Tasks verify + close gaps only.
- **FR-008 (cross-org search)**: No proto/Redis change needed — resolved via metro-scoped search plus the existing per-tool shared-org filter. Organizations FR-010 (Redis "current org") was removed and is not a dependency.
- **FR-009 (shared-org owner info)**: No proto change — reuses the existing `User.orgs` field via `Tool.owner`. Computed as intersection of caller's orgs ∩ tool owner's orgs. This is also why `CreateRentalRequestResponse.shared_organization_ids`/`shared_organization_names` (005-rentals) were removed: org discovery belongs here, at search/GetTool time, not in the rental-creation response.
- **KD-2 (GetToolImages)**: Lower severity (read-only metadata). Decision: implement access check OR document as public.
- **KD-4 (ListCategories)**: Low severity. Decision: query DB OR document as static UX list.
- **KD-5 (logging)**: Code quality, not functional. Replace `fmt.Printf` with `internal/logger`.
- **Proto changes**: None for FR-008 or FR-009 — both reuse existing fields (`metro`/`organization_id` on `SearchToolsRequest`; `User.orgs` via `Tool.owner`).
- **Constitution**: Principle III (Push) — ConfirmImageUpload launches thumbnail async; Principle VI (Proto-First) — all new fields in proto first.