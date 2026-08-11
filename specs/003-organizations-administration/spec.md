# Feature Specification: Organizations & Administration (As-Built)

**Feature Branch**: `004-organizations-administration`

**Created**: 2026-07-22

**Status**: Updated 2026-07-29 — this document's own "Known Discrepancies" section
previously misdescribed the as-built code for both listed items: `ApproveJoinRequest`'s
already-a-member guard and `ListJoinRequests`'s `status = 'PENDING'` filter were **already
present in the code** (`internal/service/admin.go:76-82`,
`internal/repository/postgres/join_request.go`'s `ListByOrg` query) but had zero test
coverage, so the gap was in verification, not implementation. Both now have dedicated tests;
7 of 8 `AdminService` RPCs also gained e2e non-admin-rejection coverage (previously 1 of 8).
The "Known Discrepancies" section is kept as a resolved changelog rather than deleted, per
Principle I ("Reconcile Discrepancies Among Spec, RTM, and Code").

**Input**: Retrofit specification for the existing, already-implemented and deployed
Organizations & Administration feature. Per project constitution Principle I ("Reconcile Discrepancies Among Spec, RTM, and Code"), this document describes verified current behavior of
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

Any authenticated user can browse/search organizations and join an existing one they've
been invited to. Creating a new organization (becoming its first `SUPER_ADMIN`) is exposed
via the `CreateOrganization` RPC, but that RPC is gated by the
`features.allow_api_organization_creation` config flag (`internal/config/config.go`),
which is `false` in production (`config/config.ec2.prod.yaml`) — in production, new
organizations are provisioned by the backend team directly (e.g. via
`tests/data-setup/setup.go`-style tooling against the database), not self-service through
the API. The flag is `true` in every non-production config so automated/manual test flows
can still exercise `CreateOrganization` end-to-end.

**Why this priority**: The entry point into every other org-scoped capability in the
system — without an org membership, nothing else in this domain or Bill Split/Rentals/Tools
is reachable for a given user.

**Independent Test**: With `features.allow_api_organization_creation: true`, create an org
as user A (verify A becomes `SUPER_ADMIN` with `balance_cents = 0`); have user B call
`JoinOrganizationWithInvite` with a valid invitation for that org and confirm B becomes a
`MEMBER`. Separately, with the flag `false` (as in production), confirm `CreateOrganization`
is rejected with `PermissionDenied` before any `orgs` row is created.

**Acceptance Scenarios**:

1. **Given** any authenticated user and `features.allow_api_organization_creation: true`,
   **When** `CreateOrganization` is called, **Then** a new `orgs` row is created and the
   caller is added to `users_orgs` as `SUPER_ADMIN` with `balance_cents = 0` — no
   pre-authorization is required or expected to create an org.
1a. **Given** `features.allow_api_organization_creation: false` (the production setting),
    **When** `CreateOrganization` is called by any caller, **Then** it is rejected with
    `PermissionDenied` ("organization creation via the API is disabled in this
    environment...") before any repository read/write occurs — enforced in
    `internal/api/grpc/org.go`'s `CreateOrganization` handler, ahead of the service call.
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

### User Story 3 - Admin-Only Membership & Join-Request Management (Priority: P1)

Org admins can: view/search their org's members, view a single member's full profile,
block/unblock a member's renting/lending, and process (approve/reject) pending join
requests or send fresh invitations. Every one of these actions requires the caller to hold
`ADMIN` or `SUPER_ADMIN` in the target org.

**Why P1**: This is the highest-blast-radius correctness property in the whole domain and
deserves the most durable regression coverage.

**Independent Test**: As a user who is a plain `MEMBER` of Org X (or a member of no org at
all), call each of the eight `AdminService` RPCs against an org they have no admin
relationship with, and confirm the call is rejected before any side effect occurs. Covered
by `tests/unit/admin_service_test.go`'s `TestAdminService_RequiresAdminRole` for all eight
methods at the service layer; still needs an equivalent e2e-level check through the real
gRPC handlers against a live DB (see Current Test Coverage Baseline).

**Acceptance Scenarios**:

1. `ListMembers`, `SearchUsers`, `ListJoinRequests`, and `GetMemberProfile` reject a caller
   who does not hold `ADMIN`/`SUPER_ADMIN` in `organization_id`, checked via
   `adminService.verifyAdminRights` before any repository read of member/join-request data
   occurs. The gRPC handlers (`internal/api/grpc/admin.go`) extract the caller's ID via
   `GetUserIDFromContext` for all four.
2. `AdminBlockUserAccount` rejects a caller who does not hold `ADMIN`/`SUPER_ADMIN` in
   `organization_id`, checked before `adminService.BlockUser` reads or writes the target
   user's `users_orgs` row.
3. `ApproveRequestToJoin`, `RejectRequestToJoin`, and `SendInvitation` reject a caller who
   does not hold `ADMIN`/`SUPER_ADMIN` in `organization_id`, checked before any
   `join_requests`/`invitations` row is read or written.
4. A caller with `MEMBER` role, or no membership at all, in the target org is rejected by
   all eight methods before any of their side effects occur — verified for `MEMBER` and
   for "no `users_orgs` row exists at all" in `TestAdminService_RequiresAdminRole`.
5. A caller with `ADMIN` or `SUPER_ADMIN` role reaches each method's full documented
   business logic (creating/updating `join_requests`, `invitations`, `users_orgs` blocking
   fields, sending emails).

---

### Edge Cases

- **RESOLVED** — `ApproveJoinRequest` checks whether the applicant is already a member of
  the target org (`internal/service/admin.go:76-82`, before calling `AddUserToOrg`) and
  returns a friendly "user is already a member of this organization" error on a
  double-approval attempt, rather than a raw primary-key-violation error from `users_orgs`'
  composite `(user_id, org_id)` key.
- `RejectJoinRequest` expires any invitation linked to the rejected join request by setting
  `expires_on` to yesterday — this is a real, documented-nowhere-else side effect worth
  knowing about when investigating "why did this invitation stop working" reports.
- `SendInvitation`'s existing-member check uses `GetByEmail` (case-insensitive), so it is
  *not* subject to the same case-sensitivity gap identified in the Users domain spec's
  Known Discrepancy 2 — this endpoint gets it right.
- `SearchOrganizations` is `SecurityPublic` (no authentication required at all, per
  `security_config.go`) and returns each matching org's admin list (names, emails) to an
  unauthenticated caller — worth confirming this public exposure of admin contact info is
  intentional.

## Known Discrepancies *(resolved 2026-07-29 — kept as a changelog, not deleted, per Principle I)*

1. **RESOLVED (was never actually broken) — `ApproveJoinRequest`'s already-a-member guard.**
   The guard was already implemented (`internal/service/admin.go:76-82`) but had zero test
   coverage, so this document previously (incorrectly) described it as missing. **Fix**:
   added `TestAdminService_ApproveJoinRequest_AlreadyMember`
   (tests/unit/admin_service_test.go), which asserts the friendly error and that
   `AddUserToOrg`/the join-request `Update` are never called.
2. **RESOLVED (was never actually broken) — `ListJoinRequests`'s `status = 'PENDING'`
   filter.** `docs/design/grpc_api_business_logic.md`'s "List Join Requests" business logic
   says: "Query `join_requests` where `org_id` matches and `status` is `'PENDING'`." The
   filter was already present in `joinRequestRepository.ListByOrg`
   (`internal/repository/postgres/join_request.go`: `AND jr.status = 'PENDING'`), but no test
   ever seeded a non-`PENDING` row to prove it — `TestAdminService_ListJoinRequests_UsedOnField`
   only ever seeded `'PENDING'` rows, so the same result would have appeared whether or not
   the filter existed. **Fix**: added
   `TestAdminService_ListJoinRequests_FiltersToPendingOnly`
   (tests/integration/admin_join_request_test.go), which seeds one `PENDING`, one `INVITED`,
   and one `REJECTED` row and confirms only the `PENDING` one is returned.
3. **RESOLVED — Admin auth e2e coverage.** Only 1 of the 8 `AdminService` RPCs
   (`AdminBlockUserAccount`) had an e2e test proving a non-admin caller is rejected through
   the real gRPC/DB stack; the other 7 relied on unit-tier coverage only
   (`TestAdminService_RequiresAdminRole`). **Fix**: added non-admin-rejection e2e subtests
   for `ApproveRequestToJoin`, `ListMembers`, `SearchUsers`, `ListJoinRequests`,
   `RejectRequestToJoin`, `SendInvitation`, and `GetMemberProfile` (tests/e2e/admin_test.go).

## Current Test Coverage Baseline *(informational — grounds the next /speckit-tasks pass, not a requirement)*

Verified by inspection of `tests/unit/admin_service_test.go`,
`tests/e2e/admin_test.go`, `tests/integration/admin_join_request_test.go`,
`tests/integration/org_member_count_test.go`, and the org/admin portions of
`tests/e2e/org_test.go`:

**Covered**: `BlockUser`'s happy path (blocking effect on `users_orgs`), `ListMembers`
returning the right member set, `ApproveJoinRequest`'s happy path and already-a-member
rejection (unit); org member-count correctness and `ListJoinRequests`'s status filter
(integration); an admin blocking a member end-to-end and non-admin-rejection for all 8
`AdminService` RPCs (e2e); organization create/get/update happy paths; all eight
`AdminService` methods rejecting a `MEMBER`-role caller and a no-membership caller, plus both
`ADMIN` and `SUPER_ADMIN` being accepted, via `TestAdminService_RequiresAdminRole` (unit).

**Not covered anywhere**:

- `RejectJoinRequest`'s invitation-expiry side effect — actually **is** covered
  (`TestAdminService_E2E > "RejectRequestToJoin expires the linked invitation"`); this line
  from an earlier version of this doc was stale.
- `SendInvitation`'s already-a-member rejection path.
- `UpdateOrganization`'s non-member rejection and non-`SUPER_ADMIN`-price-field rejection
  (User Story 2 Scenarios 2-3) at more than a superficial level.
- `JoinOrganizationWithInvite`'s already-a-member rejection and the admin-notification
  fan-out it triggers.
- `SearchOrganizations`'s unauthenticated (`SecurityPublic`) access path specifically.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: `CreateOrganization` MUST require no pre-existing role/authorization beyond
  being authenticated — any authenticated user, once the `features.allow_api_organization_creation`
  config flag is `true`, may create an org and becomes its `SUPER_ADMIN`.
- **FR-001a**: When `features.allow_api_organization_creation` is
  `false` (the production default, `config/config.ec2.prod.yaml`), the
  `OrganizationHandler.CreateOrganization` gRPC handler MUST reject every caller with
  `codes.PermissionDenied` before invoking `OrganizationService.CreateOrganization` — in
  production, organizations are provisioned by the backend team directly, not through the
  API.
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
  in the target `organization_id`, checked before any other work in the method — verified
  by `TestAdminService_RequiresAdminRole` (unit).
- **FR-006**: `RejectRequestToJoin` MUST expire any invitation already linked to the
  rejected join request.
- **FR-007**: `ListMyOrganizations` MUST return every organization the caller belongs to
  (via `users_orgs`), each populated with that organization's member count and the caller's
  own role and balance in it.
- **FR-008**: `ListJoinRequests` (authorization already covered by FR-005) MUST return only
  the `PENDING` `join_requests` rows for the given `organization_id` (Known Discrepancy 2,
  RESOLVED — the filter was already implemented; a test now proves it).
- **FR-009** *(multi-org requirement from PRD 3.1)*: Users MUST be able to belong to multiple
  organizations simultaneously. The `users_orgs` table's composite primary key `(user_id,
  org_id)` enforces this — each row represents membership in one organization with its own
  `role` (`MEMBER`/`ADMIN`/`SUPER_ADMIN`), `status`, `balance_cents`, and blocking flags.
  A user's membership in one org is independent of their membership in another.
- **FR-010** *(multi-org requirement from PRD 3.1, 3.3)*: The server MUST NOT store, cache,
  or derive any per-user "current organization" as session or persisted state. A user may be
  signed in on multiple devices simultaneously, each focused on a different organization, so
  no single server-held value could be correct for all of them — "current organization"
  exists only as a client/device-local UI preference. Every organization-scoped RPC (search,
  rentals, bill split, ledger) MUST receive `organization_id` as an explicit request
  parameter, never inferred from stored state. See `docs/design/multi-org.md`.
- **FR-011** *(multi-org requirement from PRD 3.3)*: `ToolService.SearchTools` MUST include
  tools from ALL organizations the caller belongs to that are in the resolved metro area, not
  just tools owned by members of the single organization identified by the request's
  `organization_id` (which resolves the metro filter for that one request only). See
  `specs/006-tools-image-storage/spec.md` FR-008/FR-009 for test evidence.
- **FR-012** *(multi-org requirement from PRD 3.3, UI-Design 10.3)*: There is no server-side
  organization-context-switch RPC and no server-persisted "last selected organization." The
  client has every organization the caller shares with a tool's owner from `Tool.owner.orgs`
  (FR-011), so it renders the switch prompt itself and calls `CreateRentalRequest` directly
  with the chosen `organization_id`. `RentalService.CreateRentalRequest` validates that the
  supplied `organization_id` is genuinely shared between renter and owner — a defensive
  backstop, not a context-switch step. See `specs/005-rentals/spec.md` FR-008 for test
  evidence.

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

- **SC-001 — CLOSED (2026-07-29)**: Every one of the eight `AdminService` RPCs rejects a
  caller who does not hold `ADMIN`/`SUPER_ADMIN` in the target org, verified by a dedicated
  automated test per method at both L1 (`TestAdminService_RequiresAdminRole`) and now L3 —
  an e2e-level assertion through the real gRPC handler against a live DB exists for all 8 of
  8 RPCs in `TestAdminService_E2E` (previously only 1 of 8).
- **SC-002**: `UpdateOrganization`'s membership/role/price-field gating has explicit
  regression tests locking in its current correct behavior
  (`TestOrganizationService_UpdateOrganization`, 7 subtests covering the non-member reject,
  plain-MEMBER reject, ADMIN-attempts-price-change reject for both threshold fields,
  ADMIN/SUPER_ADMIN success paths, and the "submitted 0 means no change" semantics).
- **SC-003**: `SearchOrganizations`'s public (unauthenticated) exposure of each org's admin
  contact list is either confirmed as an intentional product decision (and documented as
  such) or scoped down — it does not remain an undiscussed side effect of being marked
  `SecurityPublic`.
- **SC-004**: A developer reading only this spec can correctly state, for each of the
  thirteen RPCs across both services, exactly who is allowed to call it successfully today
  — not who the documentation says should be allowed.

## Assumptions

- `RequestToJoinOrganization` (applicant side) is out of scope here — see
  `specs/002-authentication-legal-consent/spec.md`.
- General user-profile fields (`GetUser`, `UpdateProfile`) are out of scope here — see
  `specs/003-users/spec.md`.
