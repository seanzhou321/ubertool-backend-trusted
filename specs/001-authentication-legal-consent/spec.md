# Feature Specification: Authentication & Legal Consent (As-Built)

**Feature Branch**: `002-authentication-legal-consent`

**Created**: 2026-07-22

**Status**: Draft

**Input**: Retrofit specification for the existing, already-implemented and deployed
Authentication & Legal Consent feature. Per project constitution Principle I ("Code Is
Truth"), this document describes verified current behavior of
`internal/service/auth.go`, `internal/api/grpc/auth.go`,
`internal/api/grpc/interceptor/auth_interceptor.go`,
`internal/api/grpc/interceptor/rate_limit_interceptor.go`, `internal/security/token.go`,
`internal/security/rate_limiter.go`, `internal/domain/user.go`,
`internal/domain/legal_consent.go`, `internal/domain/invitation.go`, the
`users`/`invitations`/`pending_2fa_codes`/`pending_credentials`/`user_legal_consents`
tables in `podman/trusted-group/postgres/ubertool_schema_trusted.sql`, and
`api/proto/ubertool_trusted_backend/v1/auth_service.proto` — cross-checked against
`docs/design/grpc_api_business_logic.md`'s "Authentication" section,
`docs/design/2FA_AUTHENTICATION_SPEC.md`, and `docs/design/legal_signoff/`. It is **not** a
proposal for new behavior; every place source and docs disagreed is called out in "Known
Discrepancies" below rather than silently resolved.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Login & Two-Factor Verification (Priority: P1)

A user authenticates with email/password, receives a short-lived 2FA challenge token, and
completes login by submitting the 2FA code sent to their email.

**Why this priority**: This is the mandatory front door to every authenticated capability
in the system — nothing else in this spec or any other domain spec is reachable without it.

**Independent Test**: Log in with a valid email/password, verify a `2fa_pending` JWT and a
2FA code are produced; submit the code to `Verify2FA` and verify access + refresh tokens are
returned and usable on a protected RPC.

**Acceptance Scenarios**:

1. **Given** a user's canonical password matches `users.password_hash`, **When** `Login` is
   called, **Then** the system issues a `2fa_pending` JWT (10-minute expiry, carries
   `user_id` and a `temp_pwd=false` flag) and generates a 2FA verification code.
2. **Given** the canonical password does not match, **When** `Login` is called, **Then**
   the system falls back to `pending_credentials` for that user: if a row exists with
   `used_at IS NULL`, `expires_at` in the future, and the temp password hash matches, login
   proceeds with the same `2fa_pending` JWT but `temp_pwd=true`.
3. **Given** neither the canonical nor a valid temporary password matches, **When** `Login`
   is called, **Then** it is rejected with a generic "invalid email or password" error
   (no distinction given to the caller between "no such user" and "wrong password").
4. **Given** the server's `two_fa.enabled` config is `true` (the production setting),
   **When** `Login` succeeds, **Then** a random 5-digit code is generated, stored
   in-process keyed by `user_id`, and emailed to the user — the client never sees the code.
5. **Given** the server's `two_fa.enabled` config is `false` (an automated-test-only
   setting), **When** `Login` succeeds, **Then** the fixed passcode from config is used
   instead of a random code, **no email is sent**, and the code is logged at `Warn` level
   server-side. This mode exists solely to make UI automation deterministic and must never
   be enabled in a production deployment (see Known Discrepancy 1).
6. **Given** a valid `2fa_pending` token in the `authorization` header and the correct code
   in the request body, **When** `Verify2FA` is called, **Then** the code is checked
   against the in-process store, deleted immediately after a successful check (preventing
   reuse), and the system issues an access token (1-hour expiry) and a refresh token
   (7-day expiry), plus `reset_password = temp_pwd` from the original `Login` call.
7. **Given** an incorrect code, a missing `2fa_pending` token, or no pending code stored for
   the user (e.g. it was already consumed), **When** `Verify2FA` is called, **Then** it is
   rejected and no tokens are issued.
8. **Given** repeated calls from the same client IP, **When** `Login` or `Verify2FA` is
   called more than 3 times within a burst window, **Then** the IP-based token-bucket rate
   limiter (3-token burst, 1 token refilled per 3 minutes, per-process, evicted after 10
   minutes idle) throttles further attempts from that IP (see Known Discrepancy 4).

---

### User Story 2 - Session Lifecycle: Refresh & Logout (Priority: P1)

A logged-in user can exchange a valid refresh token for a new access/refresh pair, and can
log out, which stops push notifications from being routed to that device.

**Why this priority**: Session lifecycle correctness (and its limits) directly affects the
security posture of every other authenticated flow.

**Independent Test**: Call `RefreshToken` with a valid refresh token and confirm new tokens
are issued; call `Logout` and confirm the device's FCM token becomes `OBSOLETE`.

**Acceptance Scenarios**:

1. **Given** a valid, non-expired refresh token supplied via the `refresh-token` metadata
   header, **When** `RefreshToken` is called, **Then** the system validates it is of type
   `refresh`, and issues a new access token and a new refresh token carrying the same
   `user_id`/`email`/`roles`.
2. **Given** an invalid, expired, or wrong-type token, **When** `RefreshToken` is called,
   **Then** the call is rejected. **As-built**, no security warning is logged on this
   failure path (see Known Discrepancy 5).
3. **Given** an authenticated user with an `android_device_id`, **When** `Logout` is
   called, **Then** any `ACTIVE` `fcm_tokens` row for that user+device is set to
   `OBSOLETE`, stopping future push notifications to that device.
4. **As-built**, `Logout` does **not** invalidate, blacklist, or otherwise revoke the
   caller's access or refresh JWTs (see Known Discrepancy 2) — a token issued before logout
   remains independently valid, subject only to its own expiry (up to 1 hour for access, up
   to 7 days for refresh), regardless of a subsequent logout call.

---

### User Story 3 - Invitation-Gated Signup & Join Requests (Priority: P2)

A prospective member either follows an emailed invitation code to create an account, or
searches for an organization and submits a request to join that an admin must act on.

**Why this priority**: Onboarding is required before a new user can reach any of the above,
but happens far less frequently per-user than login, and is independently testable without
touching token/session mechanics.

**Independent Test**: Call `ValidateInvite` with a valid code+email, then `UserSignup` with
the same pair; confirm the user is created, the invitation is marked used, and they are
added to the invitation's organization. Separately call `RequestToJoinOrganization` and
confirm a `join_requests` row and an admin notification/email are produced.

**Acceptance Scenarios**:

1. **Given** an `(invitation_code, email)` pair that exists, is unused (`used_on IS NULL`),
   and unexpired, **When** `ValidateInvite` is called, **Then** it returns `valid = true`.
   If a user with that email exists **and** the caller's own JWT (if any, checked via
   context) resolves to that same user ID, the response also includes that user's profile —
   otherwise no user object is returned, even if the email belongs to an existing account.
2. **Given** an invitation that is expired or already used, **When** `ValidateInvite` is
   called, **Then** it returns `valid = false` with an explanatory message.
3. **Given** a valid, unused, unexpired invitation and an email with no existing user,
   **When** `UserSignup` is called, **Then** a new `users` row is created (bcrypt-hashed
   password), the invitation is stamped `used_on`/`used_by_user_id`, any linked
   `join_requests` row is marked `JOINED`, and the user is added to the invitation's
   organization as `MEMBER` with `balance_cents = 0`. No tokens are returned — the caller
   must log in separately afterward.
4. **Given** an email that already has a `users` row, **When** `UserSignup` is called,
   **Then** it is rejected ("Email already registered. Please log in instead."), regardless
   of whether the invitation itself is otherwise valid.
5. **Given** an organization ID and applicant details, **When**
   `RequestToJoinOrganization` is called, **Then** a `join_requests` row (`status =
   PENDING`) is created, linked to an existing user by email if one is found, and the named
   `admin_email` (verified to hold `ADMIN`/`SUPER_ADMIN` in that org) receives an email and
   an in-app notification about the request.
6. **Given** the named `admin_email` does not belong to an admin/super-admin of the target
   org, **When** `RequestToJoinOrganization` is called, **Then** it fails **after** the
   `join_requests` row has already been created — the request is persisted even though the
   admin notification step subsequently errors.

---

### User Story 4 - Password Recovery: Change & Reset (Priority: P2)

An authenticated user can change their own password; an unauthenticated user who forgot
their password can request a time-boxed temporary password by email and use it to log in.

**Why this priority**: A necessary safety-net flow, used far less often than login itself.

**Independent Test**: Call `ResetPassword` for a known email, confirm a `pending_credentials`
row is created and an email is sent; log in with the temporary password and confirm
`temp_pwd = true` propagates through to `Verify2FA`'s `reset_password` field; call
`ChangePassword` and confirm the temporary credential is stamped used.

**Acceptance Scenarios**:

1. **Given** an authenticated user, **When** `ChangePassword` is called with a correct
   `old_password` (canonical or a still-valid temporary password) and a `new_password`,
   **Then** `users.password_hash` is updated, and any unused `pending_credentials` row for
   that user is stamped `used_at = NOW()` (invalidating the temporary password).
2. **Given** an incorrect `old_password` (matches neither canonical nor a valid temporary
   credential), **When** `ChangePassword` is called, **Then** it is rejected.
3. **Given** any email (whether or not it belongs to a user), **When** `ResetPassword` is
   called, **Then** the system always returns the same generic success message. If a user
   with that email exists, a 16-hex-character random temporary password is generated,
   bcrypt-hashed, upserted into `pending_credentials` with a 30-minute expiry, and emailed
   in plain text to the user.
4. **Given** a temporary password whose `pending_credentials` row has `used_at` set or
   `expires_at` in the past, **When** it is used in `Login` or `ChangePassword`, **Then**
   it is treated as invalid, identically to a wrong password.

---

### User Story 5 - Legal Consent Tracking (Priority: P3)

The system records which versioned legal documents a user has accepted, and reports back
which of the canonical document set they still need to (re-)accept.

**Why this priority**: Compliance bookkeeping that gates a UI prompt; it does not block or
enable any other functional flow in this spec.

**Independent Test**: Call `RecordLegalConsent` with a set of doc names and a version, then
call `GetUserConsentStatus` with the same version and confirm `all_current = true` and
`pending_docs` is empty; call it again with a newer version and confirm the same docs
reappear as pending.

**Acceptance Scenarios**:

1. **Given** an authenticated user and a non-empty `doc_names` + `version`, **When**
   `RecordLegalConsent` is called, **Then** one `user_legal_consents` row per doc name is
   inserted (`ON CONFLICT DO NOTHING`, making repeat calls idempotent).
2. **Given** empty `doc_names` or an empty `version`, **When** `RecordLegalConsent` is
   called, **Then** it is rejected with `INVALID_ARGUMENT`.
3. **Given** a `current_version` string, **When** `GetUserConsentStatus` is called,
   **Then** the system compares the user's consent rows against the fixed canonical list
   `domain.KnownLegalDocs` (8 documents) and returns every doc name that has no row at
   exactly `version = current_version` as `pending_docs`, with `all_current` true only when
   that list is empty.
4. **Given** an empty `current_version`, **When** `GetUserConsentStatus` is called,
   **Then** it is rejected with `INVALID_ARGUMENT`.

---

### Edge Cases

- The `two_fa.enabled = false` test-bypass mode (US1 Scenario 5) has no code-level guard
  preventing it from being left enabled in a production config file — it is purely a
  configuration convention, not an enforced constraint.
- Both the 2FA-code store (`sync.Map` in `authService`) and the IP rate limiter
  (`sync.Map`-backed, in `internal/security/rate_limiter.go`) are per-process in-memory
  state: a server restart silently clears all pending 2FA codes (forcing affected users to
  log in again) and resets all rate-limit counters. In a hypothetical multi-instance
  deployment they would not be shared across instances either — not a current issue given
  today's single-instance Podman/EC2 deployments (Constitution Principle V), but a latent
  constraint on horizontal scaling.
- `ValidateInvite` only returns a `User` object when the *caller's own* authenticated
  identity matches the invited email — it will never leak another user's profile just
  because that email happens to already have an account.
- `RequestToJoinOrganization` can partially succeed: the `join_requests` row is always
  created before the admin-email/role check runs, so a misconfigured `admin_email` produces
  an orphaned-but-real pending request with no notification ever sent (US3 Scenario 6).
- A `ResetPassword` call's temporary password and a still-valid canonical password can
  coexist — `Login`/`ChangePassword` accept either until the temporary one is explicitly
  stamped used or expires.

## Known Discrepancies *(code vs. documentation, verified against source)*

1. **`Logout` does not invalidate JWTs, contrary to documentation.**
   `docs/design/grpc_api_business_logic.md`'s "Logout" section states step 2 is
   "Invalidate/blacklist the current JWT tokens (access and refresh tokens)." **As-built**,
   `authService.Logout` (`internal/service/auth.go`) only marks the caller's FCM push
   tokens `OBSOLETE` for the given device — there is no blacklist table, revocation list, or
   any other mechanism anywhere in the codebase (verified: no `blacklist`/`revoke`
   reference exists outside of an unrelated FCM-token test). A token issued before a
   "logout" call remains fully valid until its own expiry. This is the most
   security-relevant gap found in this domain.
2. **`pending_2fa_codes` table is schema-only, never used by application code.** The schema
   models a persistent 2FA-code table (`user_id` PK, `code`, `expires_at`), but
   `authService` stores and validates 2FA codes exclusively in an in-process `sync.Map`
   (see Edge Cases). The table is referenced only by the test-data-setup script, never by
   any repository or service code — it does not currently do anything.
3. **`docs/design/2FA_AUTHENTICATION_SPEC.md` documents only the test-bypass mode as if it
   were the standard flow.** It states the backend "sends hardcoded 2FA code '123456' via
   email" unconditionally. **As-built**, that describes only the `two_fa.enabled = false`
   path (US1 Scenario 5), and even then no email is actually sent in that mode — the code
   is only logged server-side. The production path (`two_fa.enabled = true`) generates a
   random 5-digit code and does email it (US1 Scenario 4). This document should be updated
   to describe both modes and mark the fixed-code path as test-only.
4. **IP-based rate limiting on `Login`/`Verify2FA` is implemented but entirely
   undocumented.** `internal/security/rate_limiter.go` + `rate_limit_interceptor.go` (wired
   into `cmd/server/main.go`) enforce a 3-burst/3-minute-refill token bucket per client IP
   on these two endpoints. `docs/design/grpc_api_business_logic.md`'s Login/Verify2FA
   sections say nothing about it.
5. **`RefreshToken`'s documented failure-logging step is not implemented.**
   `docs/design/grpc_api_business_logic.md` step 3 for "Refresh Token" says "if fail, output
   a warning security log." **As-built**, `authService.RefreshToken` has no `logger` calls
   on any path — unlike every other method in this file, which logs `EnterMethod`/
   `ExitMethodWithError` consistently.
6. **A JWT-verification test file is excluded from the build.**
   `tests/e2e/jwt_verification_test.go_` (trailing underscore — not a `.go` file, so the Go
   toolchain never compiles or runs it) contains `TestJWTTokenVerification`. Whatever it
   was meant to verify about token validation has not run as part of the automated suite
   for as long as that filename has had the extra underscore.

## Current Test Coverage Baseline *(informational — grounds the next /speckit-tasks pass, not a requirement)*

Verified by inspection of `tests/unit/auth_test.go`, `tests/integration/2fa_test.go`,
`tests/e2e/auth_test.go`, and the disabled `tests/e2e/jwt_verification_test.go_`:

**Covered**: `ValidateInvite` and `RequestToJoin` (unit, service-level); the full happy-path
2FA sequence — login issues a 2FA token, wrong code rejected, missing token rejected,
correct code succeeds and returns a usable access token (integration, `Test2FAFlow`);
signup with a valid invitation, `RequestToJoinOrganization`, and a login call (e2e,
`TestAuthService_E2E`).

**Not covered anywhere** (no unit, integration, or e2e test in the active build exercises
these):

- `ChangePassword` (any path — correct password, wrong password, or via temporary
  credential).
- `ResetPassword` (temporary credential creation, email content, expiry behavior).
- The temporary-password login fallback in `Login` (US1 Scenario 2) and its interaction
  with `reset_password`/`temp_pwd` propagation through `Verify2FA`.
- `RecordLegalConsent` and `GetUserConsentStatus` (including idempotency and the
  version-mismatch "pending docs" computation).
- `Logout`, including the FCM-obsolete side effect it does have and the token-invalidation
  behavior it does *not* have (Known Discrepancy 1).
- `RefreshToken` as a dedicated flow (only implied by a field check inside the 2FA
  integration test, not a direct test of the RPC).
- IP-based rate limiting (`AllowLogin`/`AllowVerify2FA`) — no test drives more than 3 rapid
  attempts from one IP.
- Direct JWT validation edge cases (expired token, wrong token type for the endpoint,
  tampered signature) — these exist only in the disabled `jwt_verification_test.go_`.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: `Login` MUST authenticate against the canonical `users.password_hash`, falling
  back to an unexpired, unused `pending_credentials` row for the same user, and MUST reject
  the call with a generic invalid-credentials error if neither matches.
- **FR-002**: On successful password verification, `Login` MUST issue a `2fa_pending` JWT
  (10-minute expiry) and generate a 2FA code, using the production random-code-plus-email
  path when `two_fa.enabled = true` and the fixed-passcode-no-email path only when
  `two_fa.enabled = false`.
- **FR-003**: `Verify2FA` MUST require a valid `2fa_pending` token, MUST reject an
  incorrect or already-consumed code, and on success MUST issue a new access token
  (1-hour expiry) and refresh token (7-day expiry) and MUST delete the consumed code.
- **FR-004**: `RefreshToken` MUST require a valid, non-expired token of type `refresh` and
  MUST issue a new access/refresh pair carrying the same identity claims.
- **FR-005**: `Logout` MUST mark the caller's FCM token(s) for the given device `OBSOLETE`.
  As-built, it MUST NOT be relied upon to invalidate the caller's JWTs (Known Discrepancy 1)
  — any spec, plan, or task that assumes logout revokes tokens is describing intended, not
  current, behavior.
- **FR-006**: `UserSignup` MUST require a valid, unused, unexpired invitation matching the
  given email, MUST reject signup if a user with that email already exists, and on success
  MUST create the user, mark the invitation used, link any originating join request as
  `JOINED`, and add the user to the invitation's organization as `MEMBER`.
- **FR-007**: `RequestToJoinOrganization` MUST create a `join_requests` row before
  attempting to notify the named admin, MUST verify the organization exists, and MUST
  verify the named `admin_email` holds `ADMIN`/`SUPER_ADMIN` in that org before emailing
  and notifying them.
- **FR-008**: `ChangePassword` MUST verify the old password against the canonical hash or a
  valid temporary credential, and on success MUST update the canonical hash and stamp any
  outstanding temporary credential as used.
- **FR-009**: `ResetPassword` MUST return an identical generic success response regardless
  of whether the given email has an account, and MUST only create/email a temporary
  credential when it does.
- **FR-010**: `RecordLegalConsent` MUST reject empty `doc_names` or `version` with
  `INVALID_ARGUMENT`, and MUST be idempotent under repeated identical calls.
- **FR-011**: `GetUserConsentStatus` MUST reject an empty `current_version` with
  `INVALID_ARGUMENT`, and MUST compute `pending_docs` against the fixed
  `domain.KnownLegalDocs` list, not any caller-supplied document list.
- **FR-012**: `Login` and `Verify2FA` MUST be rate-limited per client IP (3-attempt burst,
  1 refill per 3 minutes) as implemented in `internal/security/rate_limiter.go`.
- **FR-013** *(added 2026-07-25 — previously described only in Acceptance Scenarios, with no
  FR-ID or RTM row)*: `ValidateInvite` MUST return `valid = false` with an explanatory message
  when the `(invitation_code, email)` pair does not exist, is already used, or is expired.
  When valid, the response MUST include a `User` object only when the caller's own
  authenticated session (JWT, resolved via `GetUserIDFromContext`) identifies the same user
  ID as the account matching `email` — never merely because a `users` row with that email
  exists. `AuthHandler.ValidateInvite` (`internal/api/grpc/auth.go`), not the service layer,
  performs this identity comparison; `authService.ValidateInvite` itself returns the matched
  user record whenever one exists by email, deferring the login-match gate to the handler.

### Key Entities

- **User**: Canonical account record (`users` table) — email, phone, bcrypt password hash,
  name, avatar. Owns zero or more `UserOrg` memberships.
- **PendingCredential**: A single active temporary password per user (`pending_credentials`,
  PK on `user_id`) — hashed, time-boxed (30 min from `ResetPassword`), single-use via
  `used_at`.
- **Invitation**: A one-time code (`invitations`) binding an email to an organization,
  optionally linked to an originating `JoinRequest`, with an expiry and used/unused state.
- **LegalConsent**: One row per `(user, doc_name, version)` the user has accepted
  (`user_legal_consents`); the canonical document set is `domain.KnownLegalDocs` (8 fixed
  names), not stored per-org or per-version anywhere else.
- **UserClaims / JWT**: Three token types — `access` (1h), `refresh` (7d), `2fa_pending`
  (10m) — distinguished by a `type` claim that the auth interceptor enforces per-endpoint
  via `internal/config.EndpointSecurityConfig`.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Every RPC in this domain (`ValidateInvite`, `RequestToJoinOrganization`,
  `UserSignup`, `Login`, `Verify2FA`, `RefreshToken`, `Logout`, `ChangePassword`,
  `ResetPassword`, `RecordLegalConsent`, `GetUserConsentStatus`) has at least one automated
  test exercising both a success path and its primary rejection path.
- **SC-002**: The token-invalidation gap (Known Discrepancy 1) is either fixed, or
  `docs/design/grpc_api_business_logic.md` is corrected to describe current behavior —
  it does not remain silently contradictory between doc and code.
- **SC-003**: The IP rate limiter has an automated test proving that a 4th rapid attempt
  from the same simulated IP is rejected while the first 3 succeed (subject to valid
  credentials/codes).
- **SC-004**: `tests/e2e/jwt_verification_test.go_` is either restored to the active test
  suite (renamed to `.go` and made to pass) or explicitly superseded by equivalent coverage
  elsewhere — it does not remain permanently excluded without a documented reason.
- **SC-005**: A developer reading only this spec can correctly predict, for any of the
  eleven RPCs, what happens on both its happy path and its most likely misuse (wrong
  credentials, expired token, wrong token type, empty required field).

## Assumptions

- `two_fa.enabled` is assumed `true` in both deployment targets (Podman local, EC2) at all
  times except during automated UI test runs against a dedicated test instance; this spec
  does not attempt to verify that assumption operationally, only to document the two code
  paths that exist.
- The single-instance nature of both current deployments (Constitution Principle V) is
  assumed to make the in-memory 2FA-code store and rate limiter safe in practice today; this
  spec does not treat that as a defect, only as a documented constraint that would need
  revisiting before any horizontal-scaling change.
- Fixing Known Discrepancy 1 (logout token invalidation) is a product/security decision with
  real design tradeoffs (e.g. requiring a server-side revocation store changes the
  stateless-JWT model used everywhere else); this spec documents the gap but does not
  prescribe the fix.
- General user-profile management (`GetUser`, `UpdateProfile`) is covered separately in the
  Users-domain spec, not here, even though it shares the `users` table.
