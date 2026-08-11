# Research: Rentals Multi-Org Validation (FR-008)

**Date**: 2026-07-27 (original) — **Corrected 2026-07-28**
**Context**: Implement FR-008 from `specs/005-rentals/spec.md` — validating a rental's org context
across a multi-org membership.

> **Status**: The original version of this document proposed a `tool.owner_org_id` field plus a
> Redis-cached "current organization" server-side session, with a `context_switch_required` retry
> flow. Both were implemented, found to break the core multi-org guarantee (a tool follows its
> owner across every org they belong to — see `docs/design/multi-org.md`), and reverted. This
> document now records the corrected decision that was actually built.

## Decision: Validate the Caller-Supplied `organization_id` Directly (FR-008)

**Rationale**:
- Tools belong to users, not orgs (`tools.owner_id`); there is no `tool.owner_org_id` to compare
  against, and adding one re-introduces the tool-to-single-org binding this feature explicitly
  avoids.
- A user's "current organization" is a client/device-local UI preference — a user may have
  several devices, each focused on a different org — so the server has nothing to cache per user
  and must not persist one. See `docs/design/multi-org.md` and
  `docs/design/scratchpad/owner_org_id-for-multi-org.md` (resolution).
- The renter already picks the rental's org explicitly at request time
  (`CreateRentalRequestRequest.organization_id`); the server just needs to validate that choice.

**Flow** (as built, `internal/service/rental.go`):
1. Client calls `CreateRentalRequest` with `organization_id` (the org the renter wants this rental
   billed/governed under).
2. Server validates the renter is an ACTIVE member of `organization_id` (SEC-RENTAL-001).
3. Server validates the tool owner is also an ACTIVE member of `organization_id`
   (`isSharedOrganization`) **and** (2026-07-29) not `lending_blocked` there, and that the renter
   is not `renting_blocked` there — each flag checked against the role that party actually has in
   this rental (owner lends, renter rents). `renting_blocked`/`lending_blocked` are independent
   per-role flags (e.g. set by `BillSplitService` for an unpaid bill): a user blocked from renting
   in an org must still be able to lend there, and vice versa. See
   `TestRentalService_CreateRentalRequest`'s "Blocked-flag business rule" subtests
   (`tests/unit/rental_test.go`).
4. If not shared: reject with `FAILED_PRECONDITION` + the renter's list of orgs shared with the
   owner (`getSharedOrganizations`), so the client can prompt the user to pick one of those
   instead of retrying blind.
5. If shared: rental is created with `org_id = organization_id`.
6. The `rentals_shared_org_check` CHECK constraint on the `rentals` table enforces the
   ACTIVE-membership invariant at the DB layer as a defense-in-depth backstop (the blocked-flag
   rule is service-layer only, not part of the DB constraint).

**Alternatives considered**:
1. **`tool.owner_org_id` + Redis-cached "current org" + context-switch retry** — implemented once,
   reverted. Locks a tool to a single org at creation time, breaking "list once, rent from any
   shared org," and requires the server to track state (whose device, which org) it structurally
   cannot own.
2. **Auto-switch** — Violates least surprise; user may not want to switch.
3. **Allow cross-org rental without any shared-org check** — Lets a renter bill a rental to an
   org the tool owner has no relationship with; breaks the ledger/bill-split scoping model.

## Decision: No `current_organization_id` / `context_switch_required` / Shared-Org-List Proto Fields

**Rationale**:
- The existing `organization_id` field on `CreateRentalRequestRequest` already carries the
  client's explicit choice; no second, redundant "current org" field is needed.
- **Corrected 2026-07-29**: `CreateRentalRequestResponse.shared_organization_ids`/
  `shared_organization_names` (fields 10–11) were removed. A rental-request response is the wrong
  layer for "here are your other options" — that's a discovery concern, not a write-result. The
  renter is expected to already know which orgs are valid *before* calling
  `CreateRentalRequest`, via `Tool.owner.orgs` on the `SearchTools`/`GetTool` response (populated
  by `getSharedOrganizations` at search time — see 006-tools-image-storage FR-009). FR-008's
  rejection stays a plain `FAILED_PRECONDITION` with a human-readable message (still built from
  `getSharedOrganizations`, for debuggability) but no structured field — it's a defensive
  backstop for the rare case where membership changed between search and request, not the
  client's primary way to pick an org.

## Decision: Rental Stays in the Renter's Chosen Org

**Rationale**:
- The rental's `org_id` is whichever shared org the renter names, not implicitly "the owner's
  org" — a shared org is symmetric (both parties are active members), so there is no owner/renter
  distinction to break here.
- Bill Split, Ledger, and Notifications are all scoped to `rental.org_id` as chosen.

## Constitution Check

| Principle | Check |
|-----------|-------|
| I. Reconcile Discrepancies Among Spec, RTM, and Code | Verified against `internal/service/rental.go` `CreateRentalRequest`/`isSharedOrganization`/`getSharedOrganizations` |
| II. Domain Constants | `RentalStatus` constants used; no new status needed |
| III. Push Notifications | Rejection doesn't trigger notifications (rental not created yet) |
| IV. Layered Testing | Unit coverage in `tests/unit/rental_test.go` (`TestRentalService_CreateRentalRequest`); L2/L3 coverage still needed (see `sbr/rtm/005-rentals.rtm.md` FR-008) |
| V. Deployment Parity | No Redis dependency — nothing to keep in parity across deployment targets for this feature |
| VI. Proto-First | No proto change was needed — `organization_id` and the shared-org response fields already existed |

## Open Questions Resolved

| Question | Resolution |
|----------|------------|
| What if the renter has NO membership in the requested org? | Reject with `FAILED_PRECONDITION`-adjacent error before ever loading the tool (SEC-RENTAL-001) |
| What if the tool owner has no ACTIVE membership in the requested org? | Reject with `FAILED_PRECONDITION` + the renter's actual shared orgs with the owner |
| What if membership status is PENDING/BLOCK, not ACTIVE? | Reject — only ACTIVE memberships count on both sides |
| Does org choice affect ListMyRentals? | `ListMyRentals`/`ListMyLendings` take `organization_id` as an explicit request parameter (FR-007), not an implicit server-cached context |
| Retry after rejection — idempotency? | Client retries `CreateRentalRequest` with an `organization_id` from `Tool.owner.orgs` (obtained at search time, not from this response); no special idempotency handling needed since no partial state was created |
