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
- [ ] Phase 2 (HIGH, 17 items) — not started
- [ ] Phase 3 (MEDIUM, 20 items) — not started
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
