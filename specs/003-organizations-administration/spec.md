# Feature Specification: Organizations & Administration (As-Built)

**Feature Branch**: `004-organizations-administration`

**Created**: 2026-07-22

**Status**: Draft — **Known Discrepancy 1 (missing `AdminService` authorization) was found
CRITICAL and fixed on 2026-07-22, same session it was discovered; see that section for what
changed and what still needs live-environment verification.**

**Input**: Retrofit specification for the existing, already-implemented and deployed
Organizations & Administration feature. Per project constitution Principle I ("Code Is
Truth"), this document describes verified current behavior of
`internal/service/org.go`, `internal/service/admin.go`, `internal/api/grpc/org.go`,
`internal/api/grpc/admin.go`, `internal/config/security_config.go`, the
`orgs`/`users_orgs`/`join_requests`/`invitations` tables in
`podman/trusted-group/postgres/ubertool_schema_trusted.sql`, and
`api/proto/ubertool_trusted_backend/v1/organization_service.proto` +
`admin_service.proto` — cross-checked against `docs/design/grpc_api_business_logic.md`'s
"Organizations" and "Administration" sections. It is **not** a proposal for new behavior;
every gap found is called out in "Known Discrepancies" below.

`RequestToJoinOrganization` (the applicant-facing side of the join-request flow) lives in
`internal/service/auth.go` and is already specified in
`specs/002-authentication-legal-consent/spec.md` (User Story 3) — it is referenced here
only as a dependency, not re-specified.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Organization Discovery, Creation & Self-Service Join (Priority: P1)

Any authenticated user can browse/search organizations, create a new one (becoming its
first `SUPER_ADMIN`), and join an existing one they've been invited to.

**Why this priority**: The entry point into every other org-scoped capability in the
system — without an org membership, nothing else in this domain or Bill Split/Rentals/Tools
is reachable for a given user.

**Independent Test**: Create an org as user A (verify A becomes `SUPER_ADMIN` with
`balance_cents = 0`); have user B call `JoinOrganizationWithInvite` with a valid invitation
for that org and confirm B becomes a `MEMBER`.

**Acceptance Scenarios**:

1. **Given** any authenticated user, **When** `CreateOrganization` is called, **Then** a
   new `orgs` row is created and the caller is added to `users_orgs` as `SUPER_ADMIN` with
   `balance_cents = 0` — no pre-authorization is required or expected to create an org.
2. **Given** an org ID, **When** `GetOrganization` is called, **Then** the response
   includes the org's details, its non-blocked member count, and — only if the caller is a
   member — their own role in that org.
3. **Given** a `name` and/or `metro` filter, **When** `SearchOrganizations` is called,
   **Then** matching orgs are returned with each org's `ADMIN`/`SUPER_ADMIN` members
   listed and non-blocked member count populated, but with no caller-specific role (this
   endpoint is `SecurityPublic` — no authentication required at all).
4. **Given** an authenticated user, **When** `ListMyOrganizations` is called, **Then** it
   returns every org the caller belongs to (via `users_orgs`), each with member count and
   the caller's own role/balance.
5. **Given** a valid, unused, unexpired invitation code whose email matches the caller's
   own account email, **When** `JoinOrganizationWithInvite` is called, **Then** the caller
   is added to that org as `MEMBER` with `balance_cents = 0`, the invitation is marked
   used, and all `ADMIN`/`SUPER_ADMIN` members of that org receive an in-app notification
   ("New Member Joined").
6. **Given** the caller already belongs to the invitation's org, **When**
   `JoinOrganizationWithInvite` is called, **Then** it is rejected ("You are already a
   member of this organization").

---

### User Story 2 - Organization Settings & Threshold Broadcast (Priority: P1)

A `SUPER_ADMIN` (or, for non-price fields, an `ADMIN`) updates an organization's profile
and/or its bill-split price thresholds; changing either threshold notifies every active
member.

**Why this priority**: Misconfigured thresholds directly change other domains' behavior
(Bill Split's settlement netting, Rentals' bill-split eligibility cap) — this is the one
`UpdateOrganization` path that is both privileged and correctly guarded, worth specifying
precisely as the contrast case to User Story 3.

**Independent Test**: Call `UpdateOrganization` as an `ADMIN` attempting to change
`billsplit_settlement_threshold_cents` and confirm rejection; call it as `SUPER_ADMIN` with
a genuinely new threshold value and confirm all active members receive a notification,
email, and push notification.

**Acceptance Scenarios**:

1. **Given** the caller holds `ADMIN` or `SUPER_ADMIN` in the target org, **When**
   `UpdateOrganization` is called with only non-price fields changed (`name`,
   `description`, `address`, `metro`, `admin_email`, `admin_phone`), **Then** the update
   succeeds regardless of which of the two roles the caller holds.
2. **Given** the caller holds `ADMIN` (not `SUPER_ADMIN`), **When** `UpdateOrganization` is
   called with a non-zero `billsplit_settlement_threshold_cents` or
   `max_billsplit_rental_cost_cents`, **Then** it is rejected — only `SUPER_ADMIN` may
   change either price field.
3. **Given** the caller is not a member of the target org at all, **When**
   `UpdateOrganization` is called, **Then** it is rejected ("permission denied: not a
   member of this organization") — this endpoint **does** correctly enforce membership and
   role, unlike every `AdminService` endpoint in User Story 3.
4. **Given** either price field is submitted as `0`, **When** `UpdateOrganization` is
   called, **Then** that field's existing value is preserved unchanged — `0` means "no
   change," not "set to zero" (matches `docs/design/grpc_api_business_logic.md`).
5. **Given** a submitted price field differs from the org's current value, **When** the
   update is persisted, **Then** every active (non-`BLOCK`-status) member of the org
   receives an in-app notification, an individual email, and a single FCM multicast push
   describing both new threshold values — dispatched in a background goroutine, so the RPC
   itself returns before the broadcast completes.

---

### User Story 3 - Admin-Only Membership & Join-Request Management (Priority: P1 — was elevated for a live authorization gap; now documents the fixed behavior)

Org admins can: view/search their org's members, view a single member's full profile,
block/unblock a member's renting/lending, and process (approve/reject) pending join
requests or send fresh invitations. Every one of these actions requires the caller to hold
`ADMIN` or `SUPER_ADMIN` in the target org. **This is now enforced as of the 2026-07-22 fix
described in Known Discrepancy 1** — the scenarios below describe the corrected, current
behavior; see Known Discrepancy 1 for exactly what was wrong before and what changed.

**Why P1**: Originally elevated because the gap was a currently-live authorization bypass
in a deployed system. Kept at P1 post-fix because this is the highest-blast-radius
correctness property in the whole domain and deserves the most durable regression coverage.

**Independent Test**: As a user who is a plain `MEMBER` of Org X (or a member of no org at
all), call each of the eight `AdminService` RPCs against an org they have no admin
relationship with, and confirm the call is rejected before any side effect occurs. Covered
by `tests/unit/admin_service_test.go`'s `TestAdminService_RequiresAdminRole` (added with the
fix) for all eight methods at the service layer; still needs an equivalent e2e-level check
through the real gRPC handlers against a live DB (see Current Test Coverage Baseline).

**Acceptance Scenarios**:

1. `ListMembers`, `SearchUsers`, `ListJoinRequests`, and `GetMemberProfile` reject a caller
   who does not hold `ADMIN`/`SUPER_ADMIN` in `organization_id`, checked via
   `adminService.verifyAdminRights` before any repository read of member/join-request data
   occurs. The gRPC handlers (`internal/api/grpc/admin.go`) extract the caller's ID via
   `GetUserIDFromContext` for all four (previously only two of the eight methods did this
   at all).
2. `AdminBlockUserAccount` rejects a caller who does not hold `ADMIN`/`SUPER_ADMIN` in
   `organization_id`, checked before `adminService.BlockUser` reads or writes the target
   user's `users_orgs` row.
3. `ApproveRequestToJoin`, `RejectRequestToJoin`, and `SendInvitation` reject a caller who
   does not hold `ADMIN`/`SUPER_ADMIN` in `organization_id`, checked before any
   `join_requests`/`invitations` row is read or written.
4. A caller with `MEMBER` role, or no membership at all, in the target org is rejected by
   all eight methods before any of their side effects occur — verified for `MEMBER` and
   for "no `users_orgs` row exists at all" in `TestAdminService_RequiresAdminRole`.
5. A caller with `ADMIN` or `SUPER_ADMIN` role continues to reach each method's full
   documented business logic (creating/updating `join_requests`, `invitations`,
   `users_orgs` blocking fields, sending emails) exactly as before the fix — the change is
   additive (a leading guard clause), not a rewrite of existing logic.

---

### Edge Cases

- `ApproveJoinRequest` does not check whether the applicant is already a member of the
  target org before calling `AddUserToOrg`; because `users_orgs` has a composite primary
  key `(user_id, org_id)`, a second approval attempt for an already-joined user fails with
  a raw primary-key-violation error rather than a friendly "already a member" message.
- `RejectJoinRequest` expires any invitation linked to the rejected join request by setting
  `expires_on` to yesterday — this is a real, documented-nowhere-else side effect worth
  knowing about when investigating "why did this invitation stop working" reports.
- `SendInvitation`'s existing-member check uses `GetByEmail` (case-insensitive), so it is
  *not* subject to the same case-sensitivity gap identified in the Users domain spec's
  Known Discrepancy 2 — this endpoint gets it right.
- `SearchOrganizations` is `SecurityPublic` (no authentication required at all, per
  `security_config.go`) and returns each matching org's admin list (names, emails) to an
  unauthenticated caller — separate from, but adjacent to, Known Discrepancy 1; worth
  confirming this public exposure of admin contact info is intentional.

## Known Discrepancies *(code vs. documentation, verified against source)*

1. **CRITICAL, FIXED 2026-07-22 — `AdminService` enforced no caller-authorization at all,
   contrary to documentation for all eight of its RPCs.**
   `docs/design/grpc_api_business_logic.md` documents step 1 of every Administration
   method (`ApproveRequestToJoin`, `RejectRequestToJoin`, `SendInvitation`,
   `AdminBlockUserAccount`, `ListMembers`, `SearchUsers`, `ListJoinRequests`,
   `GetMemberProfile`) as "Verify the caller has 'ADMIN' or 'SUPER_ADMIN' role in the
   given `organization_id`." As found, none of the eight methods in
   `internal/service/admin.go` / `internal/api/grpc/admin.go` did this — any authenticated
   user (any valid access token; `internal/config/security_config.go` set every
   `AdminService` endpoint to `SecurityAccess`, not an org-role-aware level) could read any
   org's full membership/join-request data, and could block/unblock any user's
   renting/lending or approve/reject/invite on behalf of any organization regardless of
   their actual relationship to it.

   **Fix applied same day**: added a `verifyAdminRights(ctx, adminID, orgID)` helper to
   `adminService` (`internal/service/admin.go`), mirroring the pre-existing, already-correct
   pattern in `billSplitService.verifyAdminRights` (`internal/service/bill_split.go`) and
   `organizationService.UpdateOrganization`'s inline check (`internal/service/org.go`).
   Every one of the eight methods now calls it first. For the four methods that previously
   never accepted a caller identity (`ListMembers`, `SearchUsers`, `ListJoinRequests`,
   `GetMemberProfile`), the `AdminService` interface (`internal/service/service.go`) and
   `AdminHandler` (`internal/api/grpc/admin.go`) were updated to extract and pass the
   caller's ID via the existing `GetUserIDFromContext` (same mechanism every other
   authenticated RPC already uses).

   **Verification performed in this session**: `go build ./...` and `go vet ./...` pass;
   the full `tests/unit/...` suite passes, including a new `TestAdminService_RequiresAdminRole`
   covering all eight methods rejecting a `MEMBER`-role and a no-membership caller, and
   accepting both `ADMIN` and `SUPER_ADMIN`; the two pre-existing tests that called these
   methods directly (`tests/unit/admin_service_test.go`,
   `tests/integration/admin_join_request_test.go`) were updated to supply a real
   admin-role caller and still pass. **Not verified in this session** (no live Postgres
   reachable from the sandbox this spec-kit conversion ran in):
   `tests/integration/...` and `tests/e2e/...` against a real database — run
   `make test-integration` and `make test-e2e` (or `make test-precommit`) against your
   Podman DB before considering this closed, since the existing e2e `admin_test.go` suite
   already uses correctly-admin'd callers (it should pass unchanged) but has not been
   executed since the fix.
2. **`ApproveJoinRequest` lacks an already-a-member guard**, producing a raw database error
   instead of a friendly rejection on a double-approval (see Edge Cases). Not
   contradicted by documentation (the doc doesn't mention this case either), but worth
   recording as a rough edge found during verification.

## Current Test Coverage Baseline *(informational — grounds the next /speckit-tasks pass, not a requirement)*

Verified by inspection of `tests/unit/admin_service_test.go`,
`tests/e2e/admin_test.go`, `tests/integration/admin_join_request_test.go`,
`tests/integration/org_member_count_test.go`, and the org/admin portions of
`tests/e2e/org_test.go`:

**Covered**: `BlockUser`'s happy path (blocking effect on `users_orgs`), `ListMembers`
returning the right member set, `ApproveJoinRequest`'s happy path (unit); org member-count
correctness (integration); an admin blocking a member end-to-end (e2e); organization
create/get/update happy paths. **As of 2026-07-22**: all eight `AdminService` methods
rejecting a `MEMBER`-role caller and a no-membership caller, plus both `ADMIN` and
`SUPER_ADMIN` being accepted, via `TestAdminService_RequiresAdminRole` (unit) — this is the
test that closes Known Discrepancy 1 at the unit level.

**Not covered anywhere**:

- **An e2e-level (real gRPC handler + live DB) non-admin-caller rejection test** — the
  unit-level fix is verified, but nothing yet proves the same behavior through the full
  stack (interceptors, handler, service) the way a production request would actually flow.
  Run `make test-e2e` after adding one, ideally as a new `t.Run` alongside the existing
  admin-caller cases in `tests/e2e/admin_test.go`.
- `RejectJoinRequest`'s invitation-expiry side effect.
- `SendInvitation`'s already-a-member rejection path.
- `ApproveJoinRequest`'s double-approval / already-a-member collision (Known Discrepancy
  2).
- `UpdateOrganization`'s non-member rejection and non-`SUPER_ADMIN`-price-field rejection
  (User Story 2 Scenarios 2-3) at more than a superficial level.
- `JoinOrganizationWithInvite`'s already-a-member rejection and the admin-notification
  fan-out it triggers.
- `SearchOrganizations`'s unauthenticated (`SecurityPublic`) access path specifically.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: `CreateOrganization` MUST require no pre-existing authorization — any
  authenticated user may create an org and becomes its `SUPER_ADMIN`.
- **FR-002**: `UpdateOrganization` MUST reject callers who are not members of the target
  org, MUST require `ADMIN` or `SUPER_ADMIN` for any change, and MUST additionally require
  `SUPER_ADMIN` specifically to change either bill-split price threshold field, treating a
  submitted `0` as "no change."
- **FR-003**: `UpdateOrganization` MUST broadcast (notification + email + push) to every
  active member when either price threshold actually changes.
- **FR-004**: `JoinOrganizationWithInvite` MUST validate the invitation belongs to the
  caller's own email, MUST reject if the caller is already a member, and on success MUST
  notify all org admins of the new member.
- **FR-005**: Every `AdminService` RPC (`ApproveRequestToJoin`, `RejectRequestToJoin`,
  `SendInvitation`, `AdminBlockUserAccount`, `ListMembers`, `SearchUsers`,
  `ListJoinRequests`, `GetMemberProfile`) MUST require `ADMIN` or `SUPER_ADMIN` membership
  in the target `organization_id`, checked before any other work in the method. **As of the
  2026-07-22 fix (Known Discrepancy 1), all eight methods satisfy FR-005** — verified by
  `TestAdminService_RequiresAdminRole` (unit) and pending live-DB confirmation via
  `make test-integration`/`make test-e2e`.
- **FR-006**: `RejectRequestToJoin` MUST expire any invitation already linked to the
  rejected join request.

### Key Entities

- **Organization**: `orgs` table — profile fields, `max_replacement_cost_cents`,
  `max_billsplit_rental_cost_cents`, `billsplit_settlement_threshold_cents`.
- **UserOrg (membership)**: `users_orgs` — composite PK `(user_id, org_id)`, `role`
  (`MEMBER`/`ADMIN`/`SUPER_ADMIN`), `status`, blocking fields. This is the record every
  `AdminService` method is documented to check the caller's own row of, and currently does
  not.
- **JoinRequest**: `join_requests` — an applicant's pending request to join an org, with
  `status` (`PENDING`/`INVITED`/`JOINED`/`REJECTED`), optional `reason`, and
  `rejected_by_user_id`.
- **Invitation**: shared with the Authentication domain spec — see
  `specs/002-authentication-legal-consent/spec.md`.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001 (highest priority in this spec) — MET 2026-07-22 (unit), reinforced at e2e
  2026-07-23**: Every one of the eight `AdminService` RPCs rejects a caller who does not hold
  `ADMIN`/`SUPER_ADMIN` in the target org, verified by a dedicated automated test per
  method (`TestAdminService_RequiresAdminRole`). An e2e-level assertion (non-admin caller
  through the real gRPC handler against a live DB) was added 2026-07-23
  (`TestAdminService_E2E > "AdminBlockUserAccount rejects a non-admin caller"`), confirming
  the rejection path end-to-end for at least one representative RPC; the existing e2e suite
  previously only ever exercised admin callers.
- **SC-002 — MET at the unit-test level 2026-07-22**: `UpdateOrganization`'s membership/
  role/price-field gating (already correct) has explicit regression tests locking in its
  current correct behavior (`TestOrganizationService_UpdateOrganization`, 7 subtests
  covering the non-member reject, plain-MEMBER reject, ADMIN-attempts-price-change reject
  for both threshold fields, ADMIN/SUPER_ADMIN success paths, and the "submitted 0 means no
  change" semantics), so it cannot regress silently while Known Discrepancy 1 is being fixed
  elsewhere in the same service layer pattern. Prior to this date, this claim was not
  substantiated by the actual test file — only the SUPER_ADMIN success path was tested; see
  `sbr/rtm/003-organizations-administration.rtm.md` for how the gap was found.
- **SC-003**: `SearchOrganizations`'s public (unauthenticated) exposure of each org's admin
  contact list is either confirmed as an intentional product decision (and documented as
  such) or scoped down — it does not remain an undiscussed side effect of being marked
  `SecurityPublic`.
- **SC-004**: A developer reading only this spec can correctly state, for each of the
  thirteen RPCs across both services, exactly who is allowed to call it successfully today
  — not who the documentation says should be allowed.

## Assumptions

- Fixing Known Discrepancy 1 is expected to follow the exact pattern already proven correct
  twice in this codebase: `billSplitService.verifyAdminRights` (bill_split.go) and
  `organizationService.UpdateOrganization`'s inline role check (org.go) — both fetch the
  caller's own `users_orgs` row for the target org and compare `Role`. This spec documents
  the gap and the existing correct pattern; it does not itself apply the fix, consistent
  with this being a specification, not an implementation task.
- `RequestToJoinOrganization` (applicant side) is out of scope here — see
  `specs/002-authentication-legal-consent/spec.md`.
- General user-profile fields (`GetUser`, `UpdateProfile`) are out of scope here — see
  `specs/003-users/spec.md`.
