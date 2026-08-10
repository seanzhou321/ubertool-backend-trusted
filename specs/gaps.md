# Implementation & Testing Gaps Analysis

**Generated**: 2026-07-29
**Source**: SBR RTM files (`sbr/rtm/*.rtm.md`) + Task files (`specs/*/tasks.md`)

---

## Executive Summary

| Category | Count |
|----------|-------|
| **Critical / High-Severity Gaps** | 27 |
| **Medium-Severity Gaps** | 15 |
| **Security Audit Gaps** | 23 |
| **Total Tracked Gaps** | ~65 |

**Most Critical**:
1. **008-bill-split FR-015** — Missing membership auth on `GetOrganizationBillSplitSummary` handler + service logic contradicts spec
2. **005-rentals KD-1** — Overlap check in `CreateRentalRequest` not implemented (DB constraint exists, service check missing)
3. **007-ledger FR-004** — Cross-org rollup (`organization_id=0`) not implemented at all
4. **Security SEC-GLOB-003** — No rate limiting on ~45 non-login endpoints

---

## 🔴 Critical / High-Severity Gaps by Feature

### 005-rentals (RTM: 4 gaps + Convergence: 4 missing)

| ID | Gap Type | Severity | Description |
|----|----------|----------|-------------|
| FR-001 | L2 Gap | High | `HasOverlappingRental` repo method untested against real Postgres |
| FR-006 | L3 Gap | High | Org-admin grant in `GetRental` untested at e2e |
| FR-007 | L3 Gap | **Critical** | **No e2e test at all** for `ListMyRentals`/`ListMyLendings` |
| FR-008 | L2/L3 Gap | High | Shared-org rejection path untested at integration & e2e |
| FR-009/010 | L3 Gap | Medium | Rejection branches (non-PENDING) untested at e2e |
| **KD-1** (Conv C3) | **MISSING IMPL** | **Critical** | Overlap check in `CreateRentalRequest` not implemented |
| **KD-2** (Conv C4) | **MISSING IMPL** | High | `RejectRentalRequest` lacks `status == PENDING` check |
| **KD-3** (Conv C5) | **MISSING IMPL** | High | `CancelRental` lacks cancelable-status guard |
| **KD-4** (Conv C6) | **MISSING IMPL** | High | `GetRental` rejects org-admin (doc says allow) |

### 008-bill-split (RTM: 3 gaps + Convergence: 4 gaps)

| ID | Gap Type | Severity | Description |
|----|----------|----------|-------------|
| FR-002 | L2 Gap | High | Asymmetric email-failure path untested (debtor fails / creditor succeeds) |
| FR-012 | L1 Gap | High | `ListResolvedDisputes` admin-only restriction — **no rejection test at any tier** |
| FR-015 | L1/L2/L3 Gap | **Critical** | No unit/integration test; sole e2e test doesn't exercise multi-org breakdown |
| **FR-015** (Conv C1) | **MISSING AUTH** | **CRITICAL** | **No membership auth check in handler** — any authenticated user can call per-org summary for any org |
| **FR-015** (Conv C2) | **LOGIC BUG** | **HIGH** | Service iterates ALL user's orgs, **contradicts spec FR-015** (requires single `org_id` param for per-org drill-down) — needs `org_id` param + membership check |
| **KD-1** (Conv C3) | **MISSING IMPL** | **HIGH** | No pagination/filters on list RPCs (returns full result set) |
| **KD-2** (Conv C4) | DEAD CODE | Medium | `ADMIN_COMMENT` bill action never created |

### 009-security (Adversarial Audit — 23 gaps)

| AV-ID | Status | Tier Gap | Description |
|-------|--------|----------|-------------|
| SEC-GLOB-002 | Gap | L1 | 19 RPCs missing explicit `EndpointSecurityConfig` entry |
| SEC-GLOB-003 | Gap | L2/L3 | **No rate limiting** on ~45 non-login endpoints |
| SEC-GLOB-004 | Gap | L3 | No test for `user-id` metadata spoofing defense |
| SEC-AUTH-001 | Gap | L1 | **No test for JWT algorithm-confusion** (defense exists, untested) |
| SEC-ORG-001 | Accepted Risk | — | Unauthenticated `SearchOrganizations` returns all admin PII |
| SEC-ORG-003 | Gap | L3 | Unthrottled unauthenticated wildcard `ILIKE` scan |
| SEC-USER-004 | Gap | Concurrency | No concurrent `UpdateProfile` email-uniqueness test |
| SEC-ADMIN-001 | Gap | L1/L3 | Cross-org join-request-ID guessing — code defends, no test |
| SEC-ADMIN-003 | Gap | L1 | Self-block in `AdminBlockUserAccount` — no guard, no test |
| SEC-ADMIN-006 | Gap | L1/L3 | Cross-org member-ID probing in `GetMemberProfile` — no tests |
| SEC-ADMIN-007 | Gap | L1/L3 | **No positive-path test at all** for `SendInvitation` RPC |
| SEC-RENTAL-003 | Gap | L1 | `RejectRentalRequest` lacks status guard + no adversarial test |
| SEC-RENTAL-004 | Gap | L1 | `CancelRental` lacks status guard + no adversarial test |
| SEC-RENTAL-007 | Gap | L1 | `ActivateRental` ownership check present, no adversarial test |
| SEC-RENTAL-008 | Gap | L1 | `ChangeRentalDates` ownership check present, no adversarial test |
| SEC-RENTAL-011 | Gap | L1/L3 | `ListToolRentals` ownership check present, no adversarial test |
| SEC-TOOL-005 | Complete | — | Good coverage on `SearchTools` |
| SEC-IMG-001 | Gap | L1/L3 | Path traversal via `filename` in `GetUploadUrl` |
| SEC-IMG-002 | Gap | L1/L3 | Content-type/file-size not validated server-side |
| SEC-IMG-003 | Gap | L1/L3 | No image content validation in `ConfirmImageUpload` |
| SEC-IMG-005 | Gap | L1 | `ConfirmImageUpload` cross-tool `image.ToolID != toolID` check missing |
| SEC-HTTP-003 | Gap | L2 | Spoofable content-type on HTTP upload (no magic-byte sniffing) |
| SEC-LEDGER-002 | Gap | L1 | No test for negative/zero `page` or unbounded `page_size` |
| SEC-NOTIF-001 | Gap | L1 | No upper bound on `GetNotifications` `limit` parameter |
| SEC-NOTIF-003 | Gap | L2 | Cross-user FCM token hijack path untested |

### 003-organizations-administration (RTM: 5 gaps)

| ID | Gap Type | Severity | Description |
|----|----------|----------|-------------|
| FR-001a | L1/L2/L3 | High | No test for `allow_api_organization_creation=false` flag path |
| FR-004 | L1/L2/L3 | Medium | Admin-notification fan-out on `JoinOrganizationWithInvite` untested |
| FR-010 | L1 Gap | Medium | No regression test guarding "no server-side current-org" boundary |
| FR-011 | L1/L3 | Medium | Exact org-driven-metro scenario untested (general cross-org covered) |
| FR-012 | L2/L3 | High | Shared-org rejection untested at integration/e2e (see 005-rentals FR-008) |

### 006-tools-image-storage (RTM: 3 residual gaps)

| ID | Gap Type | Severity | Description |
|----|----------|----------|-------------|
| FR-003/008 | L1/L3 | Medium | Precise `organization_id=X`-drives-metro + cross-org-via-different-shared-org scenario untested |
| FR-004 | L1 + L3 | High | `GetUploadUrl` no unit test; `ConfirmImageUpload`/`DeleteImage`/`SetPrimaryImage` no e2e rejection tests |
| FR-005 | L3 | Medium | Owner-vs-AVAILABLE boundary untested at e2e |

### 007-ledger (RTM: 0 gaps, but tasks.md tracks 1 major gap)

| ID | Gap Type | Severity | Description |
|----|----------|----------|-------------|
| **FR-004** | L1/L2/L3 | **Critical** | **Multi-org rollup (`organization_id=0`) not implemented** — requires new repo method `GetCrossOrgSummary`, service branching, and tests at all 3 tiers |

### 001-auth / 002-users / 004-notifications

> **Note**: These RTMs show "Complete: 0 Gaps" but **test citations are highly mismatched** (e.g., admin tests cited for auth FRs, auth tests cited for notification FRs). Manual verification recommended.

---

## 📋 Task Status Summary (from tasks.md)

| Spec | Total Tasks | Completed | Remaining | Key Blockers |
|------|-------------|-----------|-----------|--------------|
| **003-orgs** | ~54 | ~20 | ~34 | KD-2/KD-3 fixes, FR-011/FR-012 in other features |
| **005-rentals** | ~84 | ~20 | ~64 | KD-1 overlap check, KD-2/3/4 decisions, FR-008 cross-feature |
| **006-tools** | ~72 | ~25 | ~47 | KD-2 (GetToolImages access), KD-4 (ListCategories), KD-5 (logging), KD-3 (disabled e2e) |
| **007-ledger** | ~42 | ~5 | ~37 | **FR-004 cross-org rollup not started** (depends on US3 fixes first) |
| **008-bill-split** | ~77 | ~10 | ~67 | **FR-015 CRITICAL auth gap**, FR-015 service logic, KD-1/2 decisions |

---

## 🎯 Top Priority Actions (Ranked)

### P0 — Do Immediately
1. **Fix 008-bill-split FR-015** — Add membership auth check in handler + fix service to accept `org_id` param
2. **Implement 005-rentals KD-1** — Add overlap check in `CreateRentalRequest` service layer
3. **Add security regression tests** for SEC-GLOB-002, SEC-GLOB-003, SEC-GLOB-004, SEC-AUTH-001

### P1 — This Sprint
4. **Implement 007-ledger FR-004** — Cross-org rollup (new repo method + service branching + 3-tier tests)
5. **Fix 003-orgs KD-2/KD-3** — ApproveJoinRequest already-member error + ListJoinRequests PENDING filter
6. **Enable disabled e2e test** in 006-tools (KD-3: rename `search_tools_shared_org_test.go_`)

### P2 — Next Sprint
7. **Add e2e tests for 005-rentals FR-007** — `ListMyRentals`/`ListMyLendings` currently have zero e2e coverage
8. **Add integration/e2e tests for 005-rentals FR-008** — Shared-org rejection path
9. **Fix 006-tools FR-004/005** — Unit test for `GetUploadUrl`, e2e rejection tests for image RPCs
10. **Address 008-bill-split KD-1/KD-2** — Pagination/filters decision + ADMIN_COMMENT cleanup

### P3 — Ongoing
11. **Verify test mappings** for 001-auth, 002-users, 004-notifications (RTM citations appear mismatched)
12. **Add adversarial tests** for remaining SEC-RENTAL, SEC-ADMIN, SEC-IMG, SEC-NOTIF gaps
13. **Add concurrency test** for SEC-USER-004 (UpdateProfile email uniqueness)

---

## ✅ What's Actually Complete (RTM "Complete" with caveats)

| Feature | FRs Complete | Caveat |
|---------|--------------|--------|
| **001-auth** | 13/13 | Test-to-FR citations often cite wrong tests (admin→auth, etc.) |
| **002-users** | 5/5 | Same citation mismatch issue |
| **004-notifications** | 8/8 | Same citation mismatch issue |
| **007-ledger** | 4/4 | FR-004 cross-org rollup tracked separately as "The Gap" in tasks.md |

> **Retrofit Audit Mode**: These "Complete" ratings mean *some* test evidence exists at each tier for each FR, not that the tests are well-targeted or correctly mapped. The SBR audit mode checks for *existence* of evidence, not *quality* of mapping.

---

## 📊 Gap Density by Test Tier

| Feature | L1 Unit Gaps | L2 Integration Gaps | L3 E2E Gaps | Grounding Gaps |
|---------|--------------|---------------------|-------------|----------------|
| 001-auth | 0 | 0 | 0 | 0 |
| 002-users | 0 | 0 | 0 | 0 |
| 003-orgs | 2 (FR-001a, FR-010) | 2 (FR-001a, FR-004) | 4 (FR-001a, FR-004, FR-011, FR-012) | 0 |
| 004-notifications | 0 | 0 | 0 | 0 |
| 005-rentals | 1 (FR-001) | 2 (FR-001, FR-008) | 5 (FR-006, FR-007, FR-008, FR-009, FR-010) | 0 |
| 006-tools | 2 (FR-004, FR-003/008) | 0 | 3 (FR-004, FR-005, FR-003/008) | 0 |
| 007-ledger | 1 (FR-004) | 1 (FR-004) | 1 (FR-004) | 0 |
| 008-bill-split | 2 (FR-012, FR-015) | 1 (FR-002) | 1 (FR-015) | 0 |
| **Security** | 7 | 3 | 10 | 0 |

---

## 🔗 Cross-Feature Dependencies

| Gap | Depends On | Blocking |
|-----|------------|----------|
| 005-rentals FR-008 (shared-org) | 003-orgs FR-009 (multi-org membership) | 003-orgs KD-2/KD-3 |
| 006-tools FR-008/009 (cross-org search) | 003-orgs FR-009 + FR-011 (metro) | 003-orgs KD-2/KD-3 |
| 007-ledger FR-004 (rollup) | 003-orgs FR-009 (users_orgs) | None (schema exists) |
| 008-bill-split FR-014/015 | 003-orgs FR-009 + Admin auth | 008-bill-split FR-015 auth fix |
| 005-rentals FR-012 / 003-orgs FR-012 | Same requirement, different features | Must stay in sync |

---

## 📝 Notes for Implementers

1. **Retrofit Discipline**: Do NOT re-implement happy paths — they exist. Tasks verify + close gaps only.
2. **Proto-First** (Constitution VI): Any new fields require `make proto-gen` before implementation.
3. **Push Notifications** (Constitution III): Background jobs must launch notifications async.
4. **No Redis/Server-Side Context** (Multi-Org Design): `organization_id` is always explicit in requests — no "current org" fallback.
5. **Test Mappings Need Cleanup**: The 001/002/004 RTMs have mismatched citations; a `speckit-sbr-audit` re-run after fixes would correct this.

---

## 📅 Suggested Execution Order

```
Week 1: 008-bill-split FR-015 (CRITICAL) + 005-rentals KD-1 + Security regression tests
Week 2: 007-ledger FR-004 + 003-orgs KD-2/KD-3 + Enable 006-tools KD-3 e2e test
Week 3: 005-rentals FR-007/008 e2e + 006-tools FR-004/005 + 008-bill-split KD-1/2
Week 4: Security adversarial tests + Verify RTM citations + Convergence re-check
```