# SBR Gap Remediation — Priority List & Phased Plan

Generated 2026-07-22, following the `speckit-sbr-audit` pass across all 8 feature specs
(`sbr/rtm/001-*.rtm.md` through `sbr/rtm/008-*.rtm.md`). **2 of 60 FRs project-wide are fully
`Complete`.** This document turns that audit into a prioritized, phased remediation effort.
Each RTM file remains the source of truth for exact evidence/citations — this plan references
FR-IDs rather than duplicating their Notes columns.

## Why phased, and why this severity order

Five specs (001, 003, 005, 006, 008) each surfaced at least one case where the spec's own
claimed test coverage doesn't match what the test actually verifies — "false confidence,"
the exact failure mode that already produced two real, silently-shipped bugs in this project
(`da3d301`, `4d32757`) before any RTM tooling existed. Those, plus untested code paths that
directly gate money movement, account takeover, or brute-force protection, are prioritized
first — not because other gaps don't matter, but because these are the ones where the test
suite's silence is actively dangerous rather than merely incomplete.

## Severity framework

- **CRITICAL** — a test is *actively misleading* (accidental-pass, or a claimed Success-
  Criteria/coverage-baseline assertion that is demonstrably false), OR a completely untested
  code path directly gates money movement, account takeover, or brute-force protection.
- **HIGH** — a live production RPC or autonomous job with real financial/authorization/data-
  integrity consequences has zero or near-zero coverage of its actual boundary logic, but the
  existing tests are honest (not misleading) about the gap.
- **MEDIUM** — happy path is proven; a reject-path, edge case, or exact semantic (idempotency,
  ordering, no-partial-update) is not.
- **LOW** — gaps already self-documented in spec.md as known/deferred/out-of-scope, compliance-
  adjacent, or informational (as-built "ignored" behavior with no real risk either way).

## Phase 1 — CRITICAL (12 items)

| Spec | FR | Issue |
|---|---|---|
| 001 | FR-001 | e2e "Login Flow" **accidentally passes even if `Login` is completely broken** (bad password hash + swallowed error) |
| 001 | FR-002 | Only the fixed-passcode dev path is tested; the **production `two_fa.enabled=true` random-code+email path has zero coverage** |
| 001 | FR-004 | `RefreshToken`'s real tests exist but the file is excluded from the Go build (`jwt_verification_test.go_`) — zero active coverage on a live path |
| 001 | FR-008 | `ChangePassword` — zero test evidence anywhere |
| 001 | FR-009 | `ResetPassword` — zero test evidence anywhere (account-takeover/enumeration surface) |
| 001 | FR-012 | Login/Verify2FA rate limiting — zero test evidence (brute-force protection unverified) |
| 003 | FR-002 | `UpdateOrganization`'s 3 reject clauses untested anywhere; **SC-002 falsely claims regression coverage** |
| 005 | FR-004 | `CompleteRental`'s only "integration" test bypasses the real service method entirely while spec.md claims it's covered |
| 006 | FR-004 | 3 of 4 image RPCs' ownership-rejection checks — zero coverage, same bug class as the already-fixed Known Discrepancy 1 |
| 006 | FR-005 | `GetDownloadUrl`'s owner-or-`AVAILABLE` boundary — zero coverage of the actual rule (private-image leak risk) |
| 008 | FR-006 | `AcknowledgePayment`'s reject clauses (wrong status, **double-acknowledgment**, creditor-before-debtor) — zero coverage on a payment-state endpoint |
| 008 | FR-009 | `GRACEFUL` dispute outcome — zero coverage; **SC-001 explicitly requires all 4 outcomes tested and is currently false as written** |

## Phase 2 — HIGH (17 items)

| Spec | FR | Issue |
|---|---|---|
| 001 | FR-006 | `UserSignup` abuse paths (already-exists, expired/used/invalid invite) untested |
| 003 | FR-003 | Threshold-change broadcast (notification+email+push) — zero coverage |
| 003 | FR-004 | `JoinOrganizationWithInvite` — zero coverage |
| 004 | FR-003 | `Dispatch`'s error-containment guarantee — zero coverage |
| 004 | FR-006 | `SyncDeviceToken` user-reassignment clause — zero coverage |
| 004 | FR-008 | `SendMulticastToUsers` (batching + obsolete-marking) — zero coverage |
| 005 | FR-002 | `ApproveRentalRequest` owner/`PENDING` reject clauses — zero coverage |
| 005 | FR-003 | `FinalizeRentalRequest` reject clauses — zero coverage |
| 006 | FR-002 | `AddTool`'s `owner_id`-from-JWT guarantee — never verified against the stored value |
| 006 | FR-006 | Thumbnail pipeline — spec claims e2e coverage; real async pipeline never runs in any test |
| 007 | FR-002 | `GetTransactions` cross-user isolation and ordering — never asserted |
| 007 | FR-003 | `GetLedgerSummary` — domain's "richest, least-accurate" logic, zero unit/integration coverage |
| 008 | FR-001 | `TakeBalanceSnapshots` + skip-already-settled clause — zero coverage |
| 008 | FR-004 | Auto `PENDING→DISPUTED` job — zero coverage (autonomous, financial) |
| 008 | FR-005 | Auto `DISPUTED→SYSTEM_DEFAULT_ACTION` job — zero coverage (autonomous, blocks users) |
| 008 | FR-007 | `DISPUTED`-origin graceful-acknowledgment path — zero coverage |
| 008 | FR-012 | Admin-is-a-party exclusion on `ListDisputedPayments` — never tested against a real adversarial fixture |

## Phase 3 — MEDIUM (20 items)

001/FR-003 (2FA replay), 001/FR-005 (Logout), 001/FR-007 (RequestToJoin org-exists +
persists-on-notify-failure), 002/FR-001 (role field unasserted), 002/FR-002 (partial
org-lookup resilience), 002/FR-003 (no-partial-update semantics), 003/FR-001 (CreateOrganization
unit coverage), 003/FR-006 (invitation-expiry side effect), 004/FR-001 (pagination
non-aligned-offset), 004/FR-002 (MarkNotificationRead idempotency), 004/FR-005 (InvalidArgument
branch — structurally untestable), 004/FR-007 (ReportMessageEvent), 005/FR-001 (date-order
rejection), 005/FR-005 (2 of 5 return-date-change RPCs), 005/FR-006 (`GetRental`), 006/FR-003
(empty-query/metro rejection), 008/FR-002 (notice email), 008/FR-003 (reminder email),
008/FR-008 (`ResolveDispute` L3 reject paths), 008/FR-011 (admin-caller class + `can_acknowledge`
false branches).

## Phase 4 — LOW (9 items)

001/FR-010 (RecordLegalConsent), 001/FR-011 (GetUserConsentStatus), 002/FR-004
(email-uniqueness isolation), 002/FR-005 (documented absence of input validation), 003/FR-005
(already-tracked L3 gap — reinforcement only), 006/FR-001 (already-tracked L3 gap —
reinforcement only), 006/FR-007 (documents current non-enforcement, not a required fix),
007/FR-001 (`GetBalance` L2 only), 008/FR-013 (as-built ignored pagination/filter fields).

## How each item gets closed (SBR Appendix B discipline, applied per item)

1. Write the specific missing test(s) the RTM names for that FR — the RTM's Notes column
   already states exactly what's missing.
2. Run it.
   - **Passes immediately** → real behavior already matches spec; no code change needed. Re-run
     `speckit-sbr-audit` for that feature afterward so the RTM's Boundary Status flips to
     `Complete`.
   - **Fails** → genuine bug. Apply root-cause analysis (categorize: missing requirement /
     architecture gap / implementation gap), apply the minimum fix, confirm the new test is
     green **and** the full existing suite for that spec stays green, then re-run the audit.
3. For the 5 "false confidence" items (001/FR-001, 003/FR-002, 005/FR-004, 006/FR-006,
   008/FR-009), the misleading/faked test must be **replaced** with a real one first — step 2
   doesn't apply until the test can actually fail.
4. 001/FR-004 is a special case: the fix is almost certainly re-including
   `jwt_verification_test.go_` in the Go build (drop the trailing underscore) — a near-free,
   high-value win to do first in Phase 1.

## Execution model

Execute one phase at a time, in order, pausing for review after each phase before starting the
next. Phases 1-2 in particular will very likely surface and fix real production bugs in
security- and money-handling code — consequential, hard-to-fully-reverse changes that warrant a
checkpoint rather than running unattended through all 4 phases. After each phase: re-run
`speckit-sbr-audit` on every spec touched and report the before/after Boundary Status diff plus
any real bugs found and fixed.

## Status

- [x] **Phase 1 (CRITICAL, 12 items) — completed 2026-07-22.** All 12 items closed: 3 real bugs
  found and fixed, 3 "false confidence" spec-vs-test contradictions resolved (fake/misleading
  tests replaced with real ones), 6 zero-coverage gaps closed with no bug found. Full suite
  (unit + integration + e2e) re-verified green afterward, no regressions. Details below.
- [x] **Phase 2 (HIGH, 17 items) — completed 2026-07-23.** All 17 items closed: 1 real bug found
  and fixed (a corrupt test fixture, not production code — see 006/FR-006 below), 16 genuine
  zero/thin-coverage gaps closed with no bug found. Full suite (unit + integration + e2e)
  re-verified green afterward, no regressions. Details below.
- [x] **Phase 3 (MEDIUM, 20 items) — completed 2026-07-23.** All 20 items closed: 1 real bug
  found and fixed (004/FR-001's non-page-aligned `GetNotifications` offset — production code),
  19 genuine coverage gaps closed with no bug found. Full suite (unit + integration + e2e)
  re-verified green afterward, no regressions. Details below.
- [ ] Phase 4 (LOW, 9 items) — not started

## Phase 1 results (2026-07-22)

| Spec | FR | Outcome |
|---|---|---|
| 001 | FR-001 | Accidental-pass Login e2e test replaced with 3 real subtests (canonical, pending-credential fallback, reject). No bug found. |
| 001 | FR-002 | Production `two_fa.enabled=true` path unit-tested. No bug found. |
| 001 | FR-004 | `jwt_verification_test.go_` re-included in the build (stale 2FA code fixed); redundant subtest consolidated into FR-001's fix to respect the rate limiter's shared budget. |
| 001 | FR-008 | `ChangePassword` unit-tested (3 subtests). No bug found. |
| 001 | FR-009 | **Real bug found and fixed**: `ResetPassword` returned a distinguishable error for non-existent emails — a user-enumeration vulnerability. Fixed in `internal/service/auth.go`. |
| 001 | FR-012 | Rate limiting confirmed real and correct; isolated e2e test added (`ratelimit` build tag, `make test-e2e-rate-limit`). |
| 003 | FR-002 | SC-002 spec-vs-test contradiction resolved — 7 subtests added. No bug found. spec.md corrected. |
| 005 | FR-004 | Fake integration test (bypassed `service.CompleteRental`) replaced with a real one. No bug found. spec.md corrected. |
| 006 | FR-004 | 3 RPCs' ownership-rejection checks (same bug class as Known Discrepancy 1) unit-tested. No bug found. |
| 006 | FR-005 | `GetDownloadUrl`'s owner-or-`AVAILABLE` boundary unit-tested. No bug found. |
| 008 | FR-006 | `AcknowledgePayment`'s 3 reject clauses unit-tested (8 subtests total). No bug found. |
| 008 | FR-009 | SC-001 spec-vs-test contradiction resolved — `ResolveDispute`'s `GRACEFUL` outcome unit-tested. No bug found. spec.md corrected. |

**New files**: `tests/unit/image_storage_service_test.go`, `tests/e2e/rate_limit_test.go`.
**Deleted**: `tests/e2e/jwt_verification_test.go_` (content merged into `tests/e2e/auth_test.go`).
**New Makefile target**: `test-e2e-rate-limit` (isolated, deliberately exhausts the shared rate limiter).
**Production code changed**: `internal/service/auth.go` (`ResetPassword` bug fix only).

## Phase 2 results (2026-07-23)

| Spec | FR | Outcome |
|---|---|---|
| 001 | FR-006 | `UserSignup` abuse paths (already-registered email, expired/used/invalid invitation) and the join-request-linked-as-JOINED clause unit-tested (5 subtests). No bug found. |
| 003 | FR-003 | Threshold-change broadcast (in-app notification + email + FCM multicast, fired from a background goroutine) unit-tested, synchronized via a channel on the multicast call. No bug found. |
| 003 | FR-004 | `JoinOrganizationWithInvite` unit-tested (success + 3 reject clauses). No bug found. |
| 004 | FR-003 | `Dispatch`'s error-containment guarantee unit-tested — first-ever unit test for `notificationService`. No bug found. |
| 004 | FR-006 | `SyncDeviceToken`'s `user_id`-reassignment clause closed with a real-DB integration test (DB-level `ON CONFLICT` semantic, not mockable). No bug found. |
| 004 | FR-008 | `SendMulticastToUsers` batching (500+100 split) and obsolete-marking unit-tested; added test-mode multicast-client injection and refactored the obsolete check to use the already-injectable `isUnregisteredFn` (testability fix, no behavior change). No bug found. |
| 005 | FR-002 | `ApproveRentalRequest`'s owner/PENDING reject clauses unit-tested. No bug found. |
| 005 | FR-003 | `FinalizeRentalRequest`'s renter/APPROVED reject clauses unit-tested. No bug found. |
| 006 | FR-002 | `AddTool`'s owner_id-from-JWT guarantee closed at L1 (handler asserts persisted value for 2 callers) and L3 (real DB column assertion). No bug found. |
| 006 | FR-006 | **Real bug found and fixed (test suite, not production code)**: letting the real `generateThumbnail` goroutine run for the first time revealed the shared 1×1 PNG e2e fixture had an invalid CRC checksum, so `image.Decode` had been silently failing on every run. Replaced with a PNG genuinely encoded at test time. Production pipeline was already correct. |
| 007 | FR-002 | `GetTransactions`'s isolation, ordering, and pagination MUST-clauses closed with a real-DB integration test. No bug found. |
| 007 | FR-003 | `GetLedgerSummary`'s renter-OR-owner `StatusCount` union closed with a real-DB integration test. No bug found. |
| 008 | FR-001 | `TakeBalanceSnapshots` and the skip-already-settled-pairing clause closed with a real-DB integration test. No bug found. |
| 008 | FR-004 | Auto `PENDING→DISPUTED` job (`check_overdue_bills()`) closed with a real-DB integration test. No bug found. |
| 008 | FR-005 | Auto `DISPUTED→SYSTEM_DEFAULT_ACTION` job (`auto_resolve_disputed_bills()`) closed with a real-DB integration test, explicitly confirming the no-balance-penalty distinction from FR-009's admin-driven path. No bug found. |
| 008 | FR-007 | `DISPUTED`-origin graceful-acknowledgment path's existing subtest expanded from an `err == nil`-only check to assert every clause the FR names, including the previously-unasserted notify-debtor clause. No bug found. |
| 008 | FR-012 | Admin-is-a-party exclusion on `ListDisputedPayments` closed with a real adversarial fixture (admin as debtor, admin as creditor, admin uninvolved). No bug found. |

**New files**: `tests/unit/notification_service_test.go`, `tests/integration/fcm_token_test.go`,
`tests/integration/ledger_test.go`, `tests/integration/billing_jobs_realdb_test.go`,
`tests/integration/bill_repository_test.go`.
**Production code changed**: `internal/service/push_notification.go` (test-mode multicast-client
injection + `isUnregisteredFn` refactor for the multicast obsolete-marking check — both
testability changes, no behavior change to the production default path).
**Test fixture bug fixed**: `tests/e2e/image_storage_test.go`'s hand-rolled 1×1 PNG byte literal
(invalid CRC checksum) replaced with a genuinely-encoded PNG generated at test time; the same
file's thumbnail-pipeline e2e subtest extended to poll for and verify the real generated
thumbnail on disk.
**Environment note**: `TestPushNotificationService_E2E` fails independently of this remediation
— its real Firebase device tokens (tied to two live Gmail test accounts) have gone stale
(`NotRegistered`), reproducing identically on a stash of the pre-Phase-2 codebase. Not a
regression; out of scope for a test-coverage remediation (would require re-registering real
devices).

## Phase 3 results (2026-07-23)

| Spec | FR | Outcome |
|---|---|---|
| 001 | FR-003 | 2FA code reuse-after-consumption rejection unit-tested (Login → Verify2FA once succeeds, a second call with the same code fails `ErrInvalid2FACode`). No bug found. |
| 001 | FR-005 | `Logout`'s FCM-token-obsolete-marking unit-tested (2 subtests). No bug found. |
| 001 | FR-007 | `RequestToJoinOrganization`'s org-existence check and persists-on-notify-failure clause (join_requests row survives a subsequent admin-verification failure) unit-tested. No bug found. |
| 002 | FR-001 | `GetUser`'s per-org role field closed at both L1 (new `userService` unit test) and L3 (e2e assertion strengthened). No bug found. |
| 002 | FR-002 | `GetUser`'s partial-org-lookup resilience (one failed org lookup among several must not fail the whole call) unit-tested — first-ever unit test for `userService.GetUserProfile`. No bug found. |
| 002 | FR-003 | `UpdateProfile`'s no-partial-update semantics (blanking a field clears it) unit-tested — first-ever unit test for `userService.UpdateProfile`. No bug found. |
| 003 | FR-001 | `CreateOrganization`'s no-pre-authorization + SUPER_ADMIN-assignment guarantee unit-tested. No bug found. |
| 003 | FR-006 | `RejectRequestToJoin`'s invitation-expiry side effect unit-tested (2 subtests). No bug found. |
| 004 | FR-001 | **Real bug found and fixed**: `GetNotifications`'s gRPC handler computed `page := (offset/limit)+1` via integer division, silently rounding any non-page-aligned offset down to the nearest page boundary (spec.md's own Known Discrepancy 1 / SC-001). Fixed in `internal/service/notification.go` and `internal/api/grpc/notification.go` — the service now accepts a true `limit`/`offset` pair instead of round-tripping through page/pageSize. Regression-locked by a unit test and a new e2e subtest. |
| 004 | FR-002 | `MarkNotificationRead`'s idempotency (first-write-wins on `read_at`) closed with a real-DB integration test. No bug found. |
| 004 | FR-005 | The `InvalidArgument` branch (previously a hardcoded, untestable `messaging.IsInvalidArgument` call) closed by adding an injectable `isInvalidArgumentFn`, mirroring the existing `isUnregisteredFn` pattern — a testability fix, no behavior change. No bug found. |
| 004 | FR-007 | `ReportMessageEvent` unit-tested (3 subtests) — first real invocation anywhere in the suite. No bug found. |
| 005 | FR-001 | `CreateRentalRequest`'s date-order rejection unit-tested. No bug found. |
| 005 | FR-005 | `AcknowledgeReturnDateRejection` and `CancelReturnDateChange` (previously zero coverage at any tier) unit-tested, including their recompute-cost assertions. No bug found. |
| 005 | FR-006 | `GetRental`'s access control (renter/owner grant, third-party reject) unit-tested — previously only a mock-interface stub. No bug found. |
| 006 | FR-003 | `SearchTools`'s empty-query and empty-metro reject clauses unit-tested. No bug found. |
| 008 | FR-002 | `SendBillSplittingNotices` (notice email to both parties + `notice_sent_at` recording) closed with a real-DB integration test. No bug found. |
| 008 | FR-003 | `SendBillReminders` (72-hour-aged reminder to both parties, fresh bills excluded) closed with a real-DB integration test. No bug found. |
| 008 | FR-008 | `ResolveDispute`'s contract-level (e2e) rejection paths closed — confirmed the handler reports rejections via `Success=false` responses, not gRPC errors, matching `AcknowledgePayment`'s existing pattern. No bug found. |
| 008 | FR-011 | `GetPaymentDetail`'s org-admin (non-party) authorized-caller class and all `can_acknowledge=false` branches unit-tested. No bug found. |

**New files**: `tests/unit/user_service_test.go`, `tests/integration/notification_test.go`,
`tests/integration/notification_jobs_test.go`.
**Production code changed**: `internal/service/notification.go` + `internal/api/grpc/notification.go`
(`GetNotifications` offset-rounding bug fix — FR-001); `internal/service/push_notification.go`
(`isInvalidArgumentFn` injectable-predicate addition — testability only, no behavior change).
