# Critical Gaps Implementation Plan

**Source**: `specs/gaps.md` + RTM files (`sbr/rtm/*.rtm.md`) + Convergence tasks
**Generated**: 2026-07-30
**Target**: Close all 27 critical/high-severity gaps with tests at all tiers (L1/L2/L3)

---

## Gap Inventory (27 Critical/High Gaps)

| # | Feature | Gap ID | Severity | Description |
|---|---------|--------|----------|-------------|
| 1 | 008-bill-split | FR-015 (C1) | **CRITICAL** | No membership auth check in `GetOrganizationBillSplitSummary` handler |
| 2 | 008-bill-split | FR-015 (C2) | **CRITICAL** | Service iterates ALL user's orgs, contradicts spec (needs single `org_id` + membership check) |
| 3 | 005-rentals | KD-1 (C3) | **CRITICAL** | Overlap check in `CreateRentalRequest` not implemented |
| 4 | 009-security | SEC-GLOB-003 | **CRITICAL** | No rate limiting on ~45 non-login endpoints |
| 5 | 005-rentals | FR-007 | **CRITICAL** | No e2e test at all for `ListMyRentals`/`ListMyLendings` |
| 6 | 008-bill-split | FR-015 | **CRITICAL** | No unit/integration test; sole e2e test doesn't exercise multi-org breakdown |
| 7 | 008-bill-split | FR-012 | **HIGH** | `ListResolvedDisputes` admin-only restriction — no rejection test at any tier |
| 8 | 005-rentals | FR-008 | **HIGH** | Shared-org rejection path untested at integration & e2e |
| 9 | 005-rentals | KD-2 (C4) | **HIGH** | `RejectRentalRequest` lacks `status == PENDING` check |
| 10 | 005-rentals | KD-3 (C5) | **HIGH** | `CancelRental` lacks cancelable-status guard |
| 11 | 005-rentals | KD-4 (C6) | **HIGH** | `GetRental` rejects org-admin (doc says allow) |
| 12 | 008-bill-split | KD-1 (C3) | **HIGH** | No pagination/filters on list RPCs (returns full result set) |
| 13 | 008-bill-split | FR-002 | **HIGH** | Asymmetric email-failure path untested (debtor fails / creditor succeeds) |
| 14 | 003-orgs | FR-001a | **HIGH** | No test for `allow_api_organization_creation=false` flag path |
| 15 | 003-orgs | KD-2 | **HIGH** | `ApproveJoinRequest` already-member error not friendly |
| 16 | 003-orgs | KD-3 | **HIGH** | `ListJoinRequests` needs PENDING filter test |
| 17 | 009-security | SEC-GLOB-002 | **MEDIUM** | 19 RPCs missing explicit `EndpointSecurityConfig` entry |
| 18 | 009-security | SEC-GLOB-004 | **MEDIUM** | No test for `user-id` metadata spoofing defense |
| 19 | 009-security | SEC-AUTH-001 | **MEDIUM** | No test for JWT algorithm-confusion |
| 20 | 009-security | SEC-ORG-003 | **MEDIUM** | Unthrottled unauthenticated wildcard `ILIKE` scan |
| 21 | 009-security | SEC-USER-004 | **MEDIUM** | No concurrent `UpdateProfile` email-uniqueness test |
| 22 | 009-security | SEC-ADMIN-001 | **MEDIUM** | Cross-org join-request-ID guessing — code defends, no test |
| 23 | 009-security | SEC-ADMIN-003 | **MEDIUM** | Self-block in `AdminBlockUserAccount` — no guard, no test |
| 24 | 009-security | SEC-ADMIN-006 | **MEDIUM** | Cross-org member-ID probing in `GetMemberProfile` — no tests |
| 25 | 009-security | SEC-ADMIN-007 | **MEDIUM** | No positive-path test at all for `SendInvitation` RPC |
| 26 | 006-tools | KD-2 (C3) | **HIGH** | `GetToolImages` has NO access check (any auth user can list images) |
| 27 | 006-tools | FR-004 | **HIGH** | `GetUploadUrl` no unit test; image mutation RPCs no e2e rejection tests |

> **Note**: 007-ledger FR-004 (cross-org rollup) is **already implemented and complete** per RTM — no action needed.

---

## Week 1: P0 — Critical Auth + Overlap Check + Security Regression Tests

### 1.1 008-bill-split FR-015: Membership Auth + Service Fix (CRITICAL)

**Files to Modify:**
- `internal/api/grpc/bill_split.go` — Add **membership** check in handler (not admin)
- `internal/service/bill_split.go` — Fix service to accept `org_id` param
- `api/proto/ubertool_trusted_backend/v1/bill_split_service.proto` — Ensure `organization_id` in request
- `tests/unit/bill_split_service_test.go` — Unit tests for auth + per-org logic
- `tests/integration/bill_split_test.go` — Integration tests with real DB
- `tests/e2e/bill_split_test.go` — E2E tests for multi-org breakdown

**Tasks:**
- [ ] T1.1.1 Add **membership** verification in `GetOrganizationBillSplitSummary` handler (`internal/api/grpc/bill_split.go`) — verify caller is member of target org (any role)
- [ ] T1.1.2 Fix `GetOrganizationBillSplitSummary` service to accept `org_id` parameter and query single org (`internal/service/bill_split.go`)
- [ ] T1.1.3 Verify proto has `organization_id` in `GetOrganizationBillSplitSummaryRequest`; run `make proto` if needed
- [ ] T1.1.4 [P] Unit test: Non-member caller rejected (`tests/unit/bill_split_service_test.go`)
- [ ] T1.1.5 [P] Unit test: Member in target org gets per-org breakdown with correct counts (`tests/unit/bill_split_service_test.go`)
- [ ] T1.1.6 [P] Integration test: Multi-org scenario — user in Org A+B → only requested org breakdown (`tests/integration/bill_split_test.go`)
- [ ] T1.1.7 [P] E2E test: Full gRPC call with `organization_id` parameter, verify per-org list content (`tests/e2e/bill_split_test.go`)

### 1.2 005-rentals KD-1: Overlap Check Implementation (CRITICAL)

**Files to Modify:**
- `internal/service/rental.go` — Add overlap check in `CreateRentalRequest`
- `internal/repository/postgres/rental.go` — Verify `HasOverlappingRental` exists and works
- `tests/unit/rental_test.go` — Unit test for overlap rejection
- `tests/integration/rental_ledger_test.go` — Integration test against real Postgres

**Tasks:**
- [ ] T1.2.1 Design decision: Which statuses block overlap? (PENDING/APPROVED/SCHEDULED/ACTIVE/OVERDUE) — document in code comments
- [ ] T1.2.2 Implement overlap check in `CreateRentalRequest` using `HasOverlappingRental` repo method (`internal/service/rental.go`)
- [ ] T1.2.3 [P] Unit test: Two overlapping requests for same tool → second rejected (`tests/unit/rental_test.go`) — **write test FIRST (TDD)**
- [ ] T1.2.4 [P] Integration test: Exercise `HasOverlappingRental` against real DB with overlapping rentals (`tests/integration/rental_ledger_test.go`)
- [ ] T1.2.5 Verify DB constraint `rentals_overlap_check` exists in schema as safety net

### 1.3 Security Regression Tests (CRITICAL)

**Files to Modify/Create:**
- `tests/unit/security_config_test.go` — New: reflection test for SEC-GLOB-002
- `tests/unit/auth_interceptor_test.go` — New: test for SEC-GLOB-004 (user-id spoofing)
- `tests/unit/token_test.go` — New: test for SEC-AUTH-001 (JWT algorithm confusion)
- `tests/unit/http_image_upload_handler_test.go` — Verify SEC-HTTP-003 (spoofable content-type)
- `tests/unit/user_service_test.go` — Add concurrent test for SEC-USER-004

**Tasks:**
- [ ] T1.3.1 [P] Unit test: Reflection-based test enumerating all proto RPCs and asserting each has explicit `EndpointSecurityConfig` entry or is on allowlist (SEC-GLOB-002)
- [ ] T1.3.2 [P] Unit test: Raw gRPC client injecting `user-id` metadata → assert it's ignored/overwritten by auth interceptor (SEC-GLOB-004)
- [ ] T1.3.3 [P] Unit test: Construct JWT with `alg: none` and RS256-key-substitution → assert `ValidateToken` rejects (SEC-AUTH-001)
- [ ] T1.3.4 [P] Unit test: Two concurrent `UpdateProfile` calls targeting same new email → assert DB unique constraint prevents race (SEC-USER-004)
- [ ] T1.3.5 [P] Unit test: Non-image bytes with spoofed `Content-Type: image/jpeg` → verify magic-byte sniffing or document gap (SEC-HTTP-003)

---

## Week 2: P1 — Ledger Cross-Org (Already Done) + Orgs KD Fixes + Tools E2E Enable

### 2.1 007-ledger FR-004: Verify Complete (No Action Needed)

**Verification Tasks:**
- [ ] T2.1.1 Verify `GetSummaryAllOrgs` exists in `internal/repository/postgres/ledger.go`
- [ ] T2.1.2 Verify service branching on `orgID == 0` in `internal/service/ledger.go`
- [ ] T2.1.3 Run existing tests: `make test-unit && make test-integration && make test-e2e` — confirm FR-004 tests pass
- [ ] T2.1.4 Update `sbr/rtm/007-ledger.rtm.md` if needed (already shows Complete)

### 2.2 003-organizations-administration: KD-2 + KD-3 Fixes (HIGH)

**Files to Modify:**
- `internal/service/admin.go` — Improve already-member error message (KD-2)
- `internal/repository/postgres/join_request.go` — Verify PENDING filter exists (KD-3)
- `tests/unit/admin_service_test.go` — Unit tests for both fixes
- `tests/integration/admin_join_request_test.go` — Integration test for PENDING filter

**Tasks:**
- [ ] T2.2.1 [P] Improve `ApproveJoinRequest` already-member error to "You are already a member of this organization" (`internal/service/admin.go:266`)
- [ ] T2.2.2 [P] Verify `ListByOrg` query has `AND jr.status = 'PENDING'` filter (`internal/repository/postgres/join_request.go:87`)
- [ ] T2.2.3 [P] Unit test: `ApproveJoinRequest` already-a-member rejection with friendly message (`tests/unit/admin_service_test.go`)
- [ ] T2.2.4 [P] Integration test: `ListJoinRequests` returns only PENDING after fix (`tests/integration/admin_join_request_test.go`)

### 2.3 006-tools-image-storage: Enable Disabled E2E Test (KD-3)

**Files to Modify:**
- `tests/e2e/search_tools_shared_org_test.go_` → `tests/e2e/search_tools_shared_org_test.go` (rename)

**Tasks:**
- [ ] T2.3.1 Rename `tests/e2e/search_tools_shared_org_test.go_` → `tests/e2e/search_tools_shared_org_test.go`
- [ ] T2.3.2 Run `make test-e2e` — confirm test passes (shared-org filtering at e2e level)
- [ ] T2.3.3 Update `sbr/rtm/006-tools-image-storage.rtm.md`

---

## Week 3: P1 — Remaining High-Severity Gaps

### 3.1 005-rentals: FR-007 + FR-008 E2E Tests (CRITICAL/HIGH)

**Files to Modify:**
- `tests/e2e/rental_test.go` — Add e2e tests for ListMyRentals/ListMyLendings + shared-org rejection

**Tasks:**
- [ ] T3.1.1 [P] E2E test: `ListMyRentals` and `ListMyLendings` with `organization_id=0` spanning multiple orgs (`tests/e2e/rental_test.go`)
- [ ] T3.1.2 [P] E2E test: `CreateRentalRequest` with non-shared org → `FAILED_PRECONDITION` + shared orgs list (`tests/e2e/rental_test.go`)
- [ ] T3.1.3 [P] Integration test: `CreateRentalRequest` shared-org rejection against real DB (`tests/integration/rental_ledger_test.go`)
- [ ] T3.1.4 [P] E2E test: `RejectRentalRequest` on APPROVED rental → rejection (FR-009 rejection branch)
- [ ] T3.1.5 [P] E2E test: `CancelRental` on ACTIVE rental → rejection (FR-010 rejection branch)

### 3.2 005-rentals: KD-2 + KD-3 + KD-4 Decisions (HIGH)

**Files to Modify:**
- `internal/service/rental.go` — Add status guards for Reject/Cancel + org-admin access for GetRental
- `tests/unit/rental_test.go` — Unit tests for all three decisions

**Tasks:**
- [ ] T3.2.1 **Decision KD-2**: Add `status == PENDING` check to `RejectRentalRequest` (match `ApproveRentalRequest`) — `internal/service/rental.go`
- [ ] T3.2.2 [P] Unit test: `RejectRentalRequest` on non-PENDING rental → rejected (`tests/unit/rental_test.go`)
- [ ] T3.2.3 **Decision KD-3**: Add cancelable status check to `CancelRental` (cancelable: PENDING/APPROVED/SCHEDULED) — `internal/service/rental.go`
- [ ] T3.2.4 [P] Unit test: `CancelRental` on ACTIVE/COMPLETED rental → rejected (`tests/unit/rental_test.go`)
- [ ] T3.2.5 **Decision KD-4**: Add org-admin access to `GetRental` (match doc) — `internal/service/rental.go`
- [ ] T3.2.6 [P] Unit test: `GetRental` by org admin (not renter/owner) → success (`tests/unit/rental_test.go`)
- [ ] T3.2.7 [P] E2E test: `GetRental` org-admin grant path (`tests/e2e/rental_test.go`)

### 3.3 006-tools-image-storage: FR-004 + FR-005 Tests (HIGH)

**Files to Modify:**
- `tests/unit/image_storage_service_test.go` — Unit test for GetUploadUrl + rejection tests
- `tests/e2e/image_storage_test.go` — E2E rejection tests for image mutations

**Tasks:**
- [ ] T3.3.1 [P] Unit test: `GetUploadUrl` basic functionality (`tests/unit/image_storage_service_test.go`)
- [ ] T3.3.2 [P] Unit test: `ConfirmImageUpload`/`DeleteImage`/`SetPrimaryImage` by non-owner → rejected (`tests/unit/image_storage_service_test.go`)
- [ ] T3.3.3 [P] Unit test: `GetDownloadUrl` owner-or-AVAILABLE boundary (`tests/unit/image_storage_service_test.go`)
- [ ] T3.3.4 [P] E2E test: Image mutation rejection paths (`tests/e2e/image_storage_test.go`)

### 3.4 008-bill-split: KD-1 + KD-2 Decisions (HIGH)

**Files to Modify:**
- `internal/service/bill_split.go` — Pagination/filters OR document intentional "return all"
- `internal/repository/postgres/bill_split.go` — Query modifications for pagination
- `internal/domain/bill.go` — Remove `ADMIN_COMMENT` OR add RPC
- `bill_split_service.proto` — Proto changes if needed
- Tests for both decisions

**Tasks:**
- [ ] T3.4.1 **Decision KD-1**: Implement pagination + `settlement_month`/`resolution_outcome` filters on `ListPayments`/`ListDisputedPayments`/`ListResolvedDisputes` — `internal/service/bill_split.go`, `internal/repository/postgres/bill_split.go`
- [ ] T3.4.2 [P] Unit + Integration tests for KD-1 decision (`tests/unit/bill_split_service_test.go`, `tests/integration/bill_split_test.go`)
- [ ] T3.4.3 **Decision KD-2**: Remove `ADMIN_COMMENT` from `BillActionType` constants + proto OR add `AddAdminComment` RPC — `internal/domain/bill.go`, `bill_split_service.proto`
- [ ] T3.4.4 [P] Test for KD-2 decision (`tests/unit/bill_split_service_test.go`)
- [ ] T3.4.5 Run `make proto` after proto changes

---

## Week 4: P2 — Security Adversarial Tests + RTM Verification

### 4.1 Security Adversarial Tests (MEDIUM)

**Files to Create/Modify:**
- `tests/unit/admin_service_test.go` — Tests for SEC-ADMIN-001, 003, 006, 007
- `tests/unit/rental_test.go` — Tests for SEC-RENTAL-003, 004, 007, 008, 011
- `tests/unit/image_storage_service_test.go` — Tests for SEC-IMG-001, 002, 003, 005
- `tests/unit/notification_service_test.go` — Tests for SEC-NOTIF-001, 003
- `tests/unit/ledger_service_test.go` — Test for SEC-LEDGER-002
- `tests/unit/http_image_upload_handler_test.go` — Test for SEC-HTTP-003

**Tasks:**
- [ ] T4.1.1 [P] Unit test: Cross-org join-request-ID guessing → rejected (SEC-ADMIN-001)
- [ ] T4.1.2 [P] Unit test: Self-block in `AdminBlockUserAccount` → rejected (SEC-ADMIN-003)
- [ ] T4.1.3 [P] Unit test: Cross-org member-ID probing in `GetMemberProfile` → rejected (SEC-ADMIN-006)
- [ ] T4.1.4 [P] Unit test: Positive path for `SendInvitation` RPC (SEC-ADMIN-007)
- [ ] T4.1.5 [P] Unit test: `RejectRentalRequest` status guard + adversarial (SEC-RENTAL-003)
- [ ] T4.1.6 [P] Unit test: `CancelRental` status guard + adversarial (SEC-RENTAL-004)
- [ ] T4.1.7 [P] Unit test: `ActivateRental` ownership check + adversarial (SEC-RENTAL-007)
- [ ] T4.1.8 [P] Unit test: `ChangeRentalDates` ownership check + adversarial (SEC-RENTAL-008)
- [ ] T4.1.9 [P] Unit test: `ListToolRentals` ownership check + adversarial (SEC-RENTAL-011)
- [ ] T4.1.10 [P] Unit test: Path traversal via `filename` in `GetUploadUrl` (SEC-IMG-001)
- [ ] T4.1.11 [P] Unit test: Content-type/file-size validation in `GetUploadUrl` (SEC-IMG-002)
- [ ] T4.1.12 [P] Unit test: Image content validation in `ConfirmImageUpload` (SEC-IMG-003)
- [ ] T4.1.13 [P] Unit test: Cross-tool `image.ToolID != toolID` in `ConfirmImageUpload` (SEC-IMG-005)
- [ ] T4.1.14 [P] Unit test: Negative/zero `page` or unbounded `page_size` in `GetTransactions` (SEC-LEDGER-002)
- [ ] T4.1.15 [P] Unit test: Upper bound on `GetNotifications` `limit` parameter (SEC-NOTIF-001)
- [ ] T4.1.16 [P] Unit test: Cross-user FCM token hijack path (SEC-NOTIF-003)

### 4.2 003-orgs Remaining Gaps (HIGH)

**Files to Modify:**
- `tests/unit/handlers/org_handler_test.go` — Test for FR-001a
- `tests/unit/org_service_test.go` — Test for FR-004 admin notification fan-out
- `tests/unit/org_service_proto_test.go` — Regression test for FR-010 (no current-org RPCs)

**Tasks:**
- [ ] T4.2.1 [P] Unit test: `CreateOrganization` rejected when `allow_api_organization_creation=false` (`tests/unit/handlers/org_handler_test.go`)
- [ ] T4.2.2 [P] E2E test: `CreateOrganization` rejected when API creation disabled (needs separate config) (`tests/e2e/org_test.go`)
- [ ] T4.2.3 [P] Unit test: `JoinOrganizationWithInvite` notifies all org admins (`tests/unit/org_service_test.go`)
- [ ] T4.2.4 [P] Unit test: Reflection check — no `SetCurrentOrganization`/`GetCurrentOrganization` RPCs exist (FR-010 regression guard)

### 4.3 006-tools Remaining Gaps (MEDIUM)

**Files to Modify:**
- `internal/service/image_storage.go` — KD-2 decision: access check for `GetToolImages`
- `internal/service/tool.go` — KD-4 decision: `ListToolCategories` query DB or document
- `tests/unit/image_storage_service_test.go` — Test for KD-2
- `tests/unit/tool_service_test.go` — Test for KD-4
- `internal/service/tool.go` — KD-5: Replace `fmt.Printf` with `internal/logger`
- `internal/service/image_storage.go` — KD-5: Replace `fmt.Printf` with `internal/logger`

**Tasks:**
- [ ] T4.3.1 **Decision KD-2**: Add access check to `GetToolImages` (owner or AVAILABLE) OR document as public — `internal/service/image_storage.go`
- [ ] T4.3.2 [P] Unit test for KD-2 decision (`tests/unit/image_storage_service_test.go`)
- [ ] T4.3.3 **Decision KD-4**: Implement `ListToolCategories` as `DISTINCT categories` query OR document as static UX list — `internal/service/tool.go`
- [ ] T4.3.4 [P] Unit test for KD-4 decision (`tests/unit/tool_service_test.go`)
- [ ] T4.3.5 Replace `fmt.Printf` in `tool.go` with `logger.Debug()`/`logger.Error()` (KD-5)
- [ ] T4.3.6 Replace `fmt.Printf` in `image_storage.go` with `logger.Warn()` (KD-5)

### 4.4 008-bill-split Remaining Gaps (HIGH)

**Files to Modify:**
- `tests/integration/notification_jobs_test.go` — FR-002 asymmetric email failure tests
- `tests/unit/bill_split_service_test.go` — FR-012 `ListResolvedDisputes` admin rejection test

**Tasks:**
- [ ] T4.4.1 [P] Integration test: `SendBillSplittingNotices` debtor email failure → `notice_sent_at` stays NULL (`tests/integration/notification_jobs_test.go`)
- [ ] T4.4.2 [P] Integration test: `SendBillSplittingNotices` creditor fails/debtor succeeds → `notice_sent_at` set (`tests/integration/notification_jobs_test.go`)
- [ ] T4.4.3 [P] Unit test: `ListResolvedDisputes` non-admin caller → rejected (`tests/unit/bill_split_service_test.go`)

---

## Cross-Cutting: Rate Limiting (SEC-GLOB-003) — P2/P3

**Scope**: Add rate limiting to ~45 non-login endpoints

**Approach**:
1. Extend `internal/api/grpc/interceptor/rate_limit_interceptor.go` beyond just Login/Verify2FA
2. Use per-user token bucket for state-changing RPCs
3. Configure limits in `internal/config/security_config.go`

**Tasks:**
- [ ] T5.1 Design rate limit tiers: auth-heavy (Login), write-heavy (Create*), read-heavy (Search*), admin
- [ ] T5.2 Implement token bucket in rate limit interceptor (or use existing `internal/security/rate_limiter.go`)
- [ ] T5.3 Add explicit `EndpointSecurityConfig` entries for all 19 missing RPCs (SEC-GLOB-002) with rate limit config
- [ ] T5.4 [P] Integration test: Burst against `CreateOrganization` → throttled
- [ ] T5.5 [P] Integration test: Burst against `SendInvitation` → throttled
- [ ] T5.6 [P] Integration test: Burst against `SearchOrganizations` → throttled
- [ ] T5.7 [P] Integration test: Burst against `GetUploadUrl` → throttled
- [ ] T5.8 [P] Integration test: Burst against `CreateRentalRequest` → throttled

---

## Test Execution Checklist

### Pre-Implementation Baseline
- [ ] Run `make test-unit && make test-integration && make test-e2e` — capture baseline pass/fail
- [ ] Verify all existing tests pass before changes

### Per-Week Validation
- [ ] Week 1: Run full test suite after each major change
- [ ] Week 2: Run full test suite after each major change
- [ ] Week 3: Run full test suite after each major change
- [ ] Week 4: Run full test suite after each major change

### Final Validation
- [ ] Run `make test-unit && make test-integration && make test-e2e` — all green
- [ ] Run `make proto` — no proto compilation errors
- [ ] Run `go build ./...` — no build errors
- [ ] Run `go vet ./...` — no vet warnings
- [ ] Run `golangci-lint run` — no lint errors (if configured)

---

## RTM Update Checklist

After all fixes, update RTMs to reflect closed gaps:

- [ ] `sbr/rtm/008-bill-split.rtm.md` — FR-015, FR-012, FR-002, KD-1, KD-2
- [ ] `sbr/rtm/005-rentals.rtm.md` — FR-007, FR-008, FR-009, FR-010, KD-1, KD-2, KD-3, KD-4
- [ ] `sbr/rtm/003-organizations-administration.rtm.md` — FR-001a, FR-004, FR-010, KD-2, KD-3
- [ ] `sbr/rtm/006-tools-image-storage.rtm.md` — FR-004, FR-005, KD-2, KD-3, KD-4, KD-5
- [ ] `sbr/rtm/009-security.rtm.md` — SEC-GLOB-002, SEC-GLOB-003, SEC-GLOB-004, SEC-AUTH-001, SEC-ORG-003, SEC-USER-004, SEC-ADMIN-001, SEC-ADMIN-003, SEC-ADMIN-006, SEC-ADMIN-007, SEC-RENTAL-003, SEC-RENTAL-004, SEC-RENTAL-007, SEC-RENTAL-008, SEC-RENTAL-011, SEC-IMG-001, SEC-IMG-002, SEC-IMG-003, SEC-IMG-005, SEC-HTTP-003, SEC-LEDGER-002, SEC-NOTIF-001, SEC-NOTIF-003

---

## Dependencies & Coordination

| Task | Depends On | Coordination |
|------|------------|--------------|
| 005-rentals FR-008 e2e | 003-orgs FR-009 (multi-org membership) | Uses `users_orgs` composite PK |
| 006-tools FR-008/009 | 003-orgs FR-009 + FR-011 (metro) | Uses `orgs.metro` + `getSharedOrganizations` |
| 008-bill-split FR-014/015 | 003-orgs FR-009 + Admin auth | Uses `users_orgs` for caller's orgs |
| 005-rentals KD-4 (GetRental org-admin) | 003-orgs Admin auth (FR-005) | Uses same admin role check |
| Security rate limiting | All features | Cross-cutting infra change |

---

## Notes for Implementers

1. **Retrofit Discipline**: Do NOT re-implement happy paths — they exist. Tasks verify + close gaps only.
2. **Proto-First** (Constitution VI): Any new fields require `make proto-gen` before implementation.
3. **Push Notifications** (Constitution III): Background jobs must launch notifications async.
4. **No Redis/Server-Side Context** (Multi-Org Design): `organization_id` is always explicit in requests — no "current org" fallback.
5. **Test Mappings Need Cleanup**: The 001/002/004 RTMs have mismatched citations; a `sbr-audit` re-run after fixes would correct this.
6. **Write Tests First (TDD)**: For all gap fixes, write the failing test first, then implement the fix.
7. **Parallel Execution**: Tasks marked `[P]` can run in parallel (different files).

---

## Success Criteria

All 27 critical/high gaps closed with:
- [ ] Unit tests (L1) for each gap
- [ ] Integration tests (L2) where applicable (DB-dependent logic)
- [ ] E2E tests (L3) for user-facing RPCs
- [ ] All existing tests still pass
- [ ] RTMs updated to show "Complete" for all previously-gapped FRs
- [ ] `make test-unit && make test-integration && make test-e2e` — all green