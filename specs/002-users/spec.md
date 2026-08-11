# Feature Specification: Users (As-Built)

**Feature Branch**: `003-users`

**Created**: 2026-07-22

**Status**: Draft

**Input**: Retrofit specification for the existing, already-implemented and deployed Users
feature. Per project constitution Principle I ("Reconcile Discrepancies Among Spec, RTM, and Code"), this document describes
verified current behavior of `internal/service/user.go`, `internal/api/grpc/user.go`,
`internal/repository/postgres/user.go`, `internal/domain/user.go`, the `users` table in
`podman/trusted-group/postgres/ubertool_schema_trusted.sql`, and
`api/proto/ubertool_trusted_backend/v1/user_service.proto` — cross-checked against
`docs/design/grpc_api_business_logic.md`'s "Users" section. It is **not** a proposal for
new behavior; every gap found is called out in "Known Discrepancies" below.

This is a deliberately small domain: the `UserService` proto exposes exactly two RPCs,
`GetUser` and `UpdateProfile`. Everything else that touches the `users` table (creation via
signup, password fields, org membership rows) is owned by the Authentication and
Organizations/Administration domains and is out of scope here — see Assumptions.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - View Own Profile (Priority: P1)

A logged-in user retrieves their own profile, including every organization they belong to
and their role/balance in each.

**Why this priority**: A read-only, side-effect-free lookup that most other client screens
depend on for basic identity/membership display.

**Independent Test**: Log in as a user who belongs to two orgs; call `GetUser`; verify the
returned profile includes both orgs with the correct role and balance for each.

**Acceptance Scenarios**:

1. **Given** an authenticated user, **When** `GetUser` is called, **Then** the response
   contains that user's `id`, `email`, `phone`, `name`, `avatar_url`, and one entry per
   organization they belong to (from `users_orgs` joined with `orgs`), each carrying that
   org's details plus the caller's `role` and `user_balance` (their `balance_cents` in that
   org).
2. **Given** an organization row exists in `users_orgs` but the corresponding `orgs` row
   cannot be fetched (e.g. transient error), **When** `GetUser` is called, **Then** that
   membership is silently skipped from the result — the call still succeeds with a
   partial org list rather than failing outright.
3. **Given** a user with no organization memberships, **When** `GetUser` is called,
   **Then** the response returns a profile with an empty `orgs` list, not an error.

---

### User Story 2 - Update Own Profile (Priority: P1)

A logged-in user updates their own display name, email, phone, and/or avatar URL in a
single call.

**Why this priority**: The only mutating capability in this domain; ranked alongside
`GetUser` because together they are this domain's entire surface.

**Independent Test**: Call `UpdateProfile` with new field values; call `GetUser` again and
confirm every field reflects the update.

**Acceptance Scenarios**:

1. **Given** an authenticated user and a request with `name`, `email`, `phone`, and
   `avatar_url`, **When** `UpdateProfile` is called, **Then** all four fields on that
   user's `users` row are overwritten with the request's values (not merged — every field
   is always written, even if empty) and the updated profile is returned.
2. **Given** a request whose `email` exactly matches another existing user's `email`
   (case-sensitive), **When** `UpdateProfile` is called, **Then** the database's `UNIQUE`
   constraint on `users.email` rejects the write and the call fails (see Known Discrepancy
   3 for what the caller actually sees).
3. **Given** a request with an empty `name`, an empty or malformed `email` (no `@`, wrong
   format), or an empty `phone`, **When** `UpdateProfile` is called, **Then** the write
   still succeeds — **as-built**, none of these fields are validated before being persisted
   (see Known Discrepancy 1). Only the database's `NOT NULL` and `UNIQUE` constraints on
   `email` can reject a write, and only for a `NULL` or exact-case-duplicate value.
4. **Given** a caller updates their own `email` to a new value, **When** `UpdateProfile`
   succeeds, **Then** the new email becomes immediately authoritative for future `Login`
   calls (Authentication domain) with **no confirmation step** — the caller is never asked
   to prove they control the new address before it takes effect (see Known Discrepancy 4).

---

### Edge Cases

- `UpdateProfile` always overwrites all four editable fields from the request — there is no
  partial-update semantics (e.g. an omitted/empty `phone` in the request clears the
  existing phone number rather than leaving it unchanged).
- Because `GetByEmail` (used at signup and login) matches case-insensitively
  (`LOWER(email) = LOWER($1)`) while the `users.email` `UNIQUE` constraint is
  case-sensitive, and `UpdateProfile` performs no uniqueness pre-check of its own, a user
  can set their email to an existing user's email in a different case (e.g.
  `USER@x.com` when `user@x.com` already exists) without the database rejecting it. This
  produces two accounts whose case-insensitive email lookup is now ambiguous (see Known
  Discrepancy 2).
- `GetUser` has no explicit handling for a user whose own `users` row has been deleted
  between JWT issuance and the call (would surface as whatever error `GetByID` returns,
  unmapped to a specific gRPC status).

## Known Discrepancies *(code vs. documentation, verified against source)*

1. **`UpdateProfile` performs no field validation, and the documentation is silent on
   whether it should.** `docs/design/grpc_api_business_logic.md`'s "Update Profile" section
   is a single line ("Update `users` table for the `user_id` in JWT") with no mention of
   validation rules. **As-built**, `userService.UpdateProfile` (`internal/service/user.go`)
   passes `name`/`email`/`phone`/`avatar_url` straight through to the database with zero
   format, length, or emptiness checks — the only enforcement is the database's `NOT NULL`
   (on `email`/`phone`/`name`) and `UNIQUE` (on `email`) constraints.
2. **Case-insensitive vs. case-sensitive email uniqueness is inconsistent between signup
   and profile update.** `authService.Signup` prevents a duplicate account by calling
   `GetByEmail` (case-insensitive) before creating a user. `userService.UpdateProfile` does
   not perform an equivalent pre-check — it relies solely on the database's `UNIQUE`
   constraint, which is case-sensitive. The two code paths do not agree on what "duplicate
   email" means, which is a genuine latent bug: `UpdateProfile` can create a
   case-variant duplicate that `Signup` would have blocked.
3. **A duplicate-email `UpdateProfile` failure is not translated to a structured error.**
   The only existing test for this path (`tests/e2e/user_test.go`, "UpdateProfile Email
   Uniqueness") asserts only `assert.Error(t, err)` — it does not assert a specific gRPC
   status code or a client-safe message. **As-built**, the raw error from the Postgres
   `UNIQUE` violation propagates back through the service and handler unmodified; the
   client cannot reliably distinguish "email already taken" from any other failure by
   status code alone.
4. **No email re-verification on change.** Neither `grpc_api_business_logic.md` nor the
   code describes or implements any confirmation step when a user changes their email via
   `UpdateProfile` — the new value is authoritative for `Login` and `ResetPassword`
   (Authentication domain) immediately. This spec records it as current behavior, not as an
   endorsed design; whether email re-verification should exist is a product decision left
   to a follow-up task.

## Current Test Coverage Baseline *(informational — grounds the next /speckit-tasks pass, not a requirement)*

Verified by inspection of `tests/unit/repos/user_test.go`, `tests/integration/user_test.go`,
and `tests/e2e/user_test.go`:

**Covered**: `GetByID`, `Create`, `ListMembersByOrg`, `SearchMembersByOrg` at the repository
level (unit, with a real-DB integration variant); `GetUser` returning a profile with org
memberships (e2e); `UpdateProfile` rejecting an exact-case duplicate email (e2e).

**Not covered anywhere**:

- `UpdateProfile` with an empty or malformed `email`, empty `name`, or empty `phone` (Known
  Discrepancy 1) — no test confirms whether these are accepted or should be rejected.
- The case-insensitive duplicate-email gap between `Signup` and `UpdateProfile` (Known
  Discrepancy 2) — no test creates a case-variant duplicate via `UpdateProfile`.
- The specific gRPC status code / error shape returned for a duplicate-email
  `UpdateProfile` failure (Known Discrepancy 3) — the existing test only checks that *an*
  error occurred.
- `UpdateProfile`'s full-overwrite (non-partial-update) semantics — no test verifies that
  omitting/blanking one field clears it rather than preserving the existing value.
- `GetUser`'s partial-result behavior when an org lookup fails (User Story 1 Scenario 2).
- `GetUser` for a user with zero organization memberships (User Story 1 Scenario 3).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: `GetUser` MUST return the caller's own profile (identity taken from the JWT,
  never from a request parameter) joined with every organization they belong to, including
  per-org role and balance.
- **FR-002**: `GetUser` MUST NOT fail outright solely because one membership's organization
  record could not be loaded — it MUST return the remaining memberships.
- **FR-003**: `UpdateProfile` MUST overwrite `name`, `email`, `phone`, and `avatar_url` on
  the caller's own `users` row (identity from JWT) with the request's values, with no
  partial-update behavior.
- **FR-004**: `UpdateProfile` MUST reject a write that would violate the `users.email`
  `UNIQUE` constraint. As-built, this enforcement is case-sensitive and happens only at the
  database layer with no application-level pre-check or structured error translation (Known
  Discrepancies 2 and 3) — any future task that changes this must update this requirement.
- **FR-005**: As-built, `UpdateProfile` MUST NOT be assumed to validate email format, or
  reject empty `name`/`phone`/`email` beyond the database's `NOT NULL` constraints (Known
  Discrepancy 1).

### Key Entities

- **User**: `users` table — `id`, `email` (unique, case-sensitive at the DB layer),
  `phone_number`, `password_hash` (never exposed via this domain's RPCs), `name`,
  `avatar_url`, `created_on`, `updated_on`.
- **UserOrg**: The caller's membership row(s) in `users_orgs`, surfaced through `GetUser` as
  per-org `role` and `user_balance` — owned by the Organizations/Administration domain but
  read here for profile display.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Both RPCs in this domain (`GetUser`, `UpdateProfile`) have automated test
  coverage for their documented success path and their documented failure paths (Known
  Discrepancies 1-3), not just the single exact-case-duplicate-email case that exists today.
- **SC-002**: The case-insensitive/case-sensitive email-uniqueness inconsistency (Known
  Discrepancy 2) is either fixed (an application-level case-insensitive pre-check added to
  `UpdateProfile`) or explicitly accepted as-is in an updated spec — it does not remain an
  undocumented latent bug.
- **SC-003**: A duplicate-email `UpdateProfile` failure returns a structured, documented
  gRPC status (e.g. `AlreadyExists`) that a client can branch on, rather than an
  unspecified error.
- **SC-004**: A developer reading only this spec can correctly predict what happens when
  `UpdateProfile` is called with an empty or malformed field, without needing to read the
  source.

## Assumptions

- User creation (`Signup`), password fields, and 2FA/session state are owned by the
  Authentication & Legal Consent domain (`specs/002-authentication-legal-consent/spec.md`)
  and are referenced here only where they interact with `users.email` (Known Discrepancy
  2); they are not re-specified in this document.
- Organization membership mutation (joining, role changes, blocking) is owned by the
  Organizations/Administration domain; this spec treats `users_orgs` as read-only context
  for `GetUser`.
- Whether email format validation or re-verification-on-change *should* be added (Known
  Discrepancies 1 and 4) is a product decision; this spec documents the current gap without
  prescribing the fix.
