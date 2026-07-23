# Feature Specification: Rentals (As-Built)

**Feature Branch**: `006-rentals`

**Created**: 2026-07-22

**Status**: Draft — **contains a HIGH-severity, currently-live data-integrity gap (Known
Discrepancy 1: no double-booking prevention); read that section before relying on schedule
correctness elsewhere in the app.**

**Input**: Retrofit specification for the existing, already-implemented and deployed
Rentals feature — the largest domain in this codebase (16 RPCs). Per project constitution
Principle I ("Code Is Truth"), this document describes verified current behavior of
`internal/service/rental.go` (1058 lines), `internal/api/grpc/rental.go`,
`internal/domain/rental.go`, `internal/utils` (pricing calculation), the `rentals` table in
`podman/trusted-group/postgres/ubertool_schema_trusted.sql`, and
`api/proto/ubertool_trusted_backend/v1/rental_service.proto` — cross-checked against
`docs/design/grpc_api_business_logic.md`'s "Rentals" section and
`docs/design/tool-rental-pricing-algorithm.md`. It is **not** a proposal for new behavior;
every gap found is called out in "Known Discrepancies" below.

Given this domain's size, verification depth here is intentionally shallower than
`specs/001` through `specs/005` for the RPCs in User Stories 3 and 4 (the date-change
negotiation sub-flows and the read/list RPCs) — those match `grpc_api_business_logic.md`
closely on inspection and did not surface discrepancies, but were not traced line-by-line
against every edge case the way User Stories 1 and 2 were. A deeper follow-up pass on those
two stories specifically is a reasonable next step if this domain gets prioritized further.

The tiered pricing algorithm itself (`utils.CalculateRentalCost`) is covered by its own
dedicated test suite (`tests/unit/pricing_test.go`) and design doc
(`docs/design/tool-rental-pricing-algorithm.md`); this spec treats it as a correctly
verified dependency and does not re-derive its correctness, only how rentals consume it
(price snapshots, recalculation triggers).

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Rental Request Lifecycle (Priority: P1)

A renter requests to borrow a tool for a date range; the owner approves or rejects; the
renter finalizes an approved request to confirm it; either party can cancel before pickup.

**Why this priority**: The entry point into every rental — nothing else in this domain has
data to act on without a created, approved, and finalized request.

**Independent Test**: Create a rental request, approve it as owner, finalize it as renter,
and confirm the tool's status becomes `RENTED` and the rental's status becomes `SCHEDULED`.

**Acceptance Scenarios**:

1. **Given** a tool and a date range where `end_date` is strictly after `start_date`,
   **When** `CreateRentalRequest` is called, **Then** the system computes
   `total_cost_cents` via the tiered pricing algorithm against a **price snapshot** taken
   from the tool at that moment (`duration_unit`, per-day/week/month prices,
   `replacement_cost_cents`), inserts a `PENDING` rental carrying that snapshot, and
   notifies the owner (in-app, push, email).
2. **Given** `end_date <= start_date`, **When** `CreateRentalRequest` is called, **Then**
   it is rejected ("end date must be after start date (minimum 1 day rental)").
3. **Given** a `PENDING` rental, **When** the owner calls `ApproveRentalRequest` with a
   `pickup_instructions` note, **Then** status becomes `APPROVED` and the renter is
   notified with the pickup note.
4. **Given** a `PENDING` rental (status is **not** re-checked for `RejectRentalRequest` —
   see Known Discrepancy 2), **When** the owner calls `RejectRentalRequest`, **Then**
   status becomes `REJECTED` and the renter is notified.
5. **Given** an `APPROVED` rental, **When** the renter calls `FinalizeRentalRequest`,
   **Then** `end_date` is copied to `last_agreed_end_date`, status becomes `SCHEDULED`, the
   tool's status becomes `RENTED`, the owner is notified, and the response additionally
   returns the renter's other `APPROVED` and `PENDING` rentals for the same tool (for
   client-side "you have other pending requests for this tool" UX).
6. **Given** an `APPROVED` rental, **When** the renter calls `FinalizeRentalRequest` but
   the rental is not `APPROVED` (any other status), **Then** it is rejected ("rental is not
   approved by owner").
7. **Given** a rental in **any** status (see Known Discrepancy 3 — status is not checked),
   **When** the renter calls `CancelRentalRequest`, **Then** status becomes `CANCELLED` and
   the owner is notified with the caller-supplied `reason` (used only in the
   notification/email; not persisted as a column on the rental record itself).

---

### User Story 2 - Active Rental Lifecycle: Pickup & Return (Priority: P1)

Either party marks a scheduled rental as picked up; either party marks it completed, which
computes final settlement and updates both users' ledgers unless bill-splitting was
declined for this transaction.

**Why this priority**: This is where money actually moves — the highest-consequence part
of the domain, and the part `grpc_api_business_logic.md` documents in the most detail
(21 numbered steps for `CompleteRental` alone).

**Independent Test**: Activate a `SCHEDULED` rental, then complete it with
`charge_billsplit=true` and confirm both parties' `users_orgs.balance_cents` and a
`ledger_transactions` row each are created; repeat with `charge_billsplit=false` and confirm
neither balance nor ledger changes, but both notifications include the direct-settlement
reminder text.

**Acceptance Scenarios**:

1. **Given** a `SCHEDULED` rental, **When** either the renter or the owner calls
   `ActivateRental`, **Then** status becomes `ACTIVE` and the *other* party is notified
   ("Rental Picked Up").
2. **Given** a rental not in `SCHEDULED` status, **When** `ActivateRental` is called,
   **Then** it is rejected ("rental is not in scheduled status").
3. **Given** a rental in `ACTIVE`, `SCHEDULED`, or `OVERDUE` status, **When** either party
   calls `CompleteRental`, **Then** `total_cost_cents` is recalculated from the rental's
   price snapshot (not the tool's current prices) over `start_date`→`end_date`,
   `settlement_cents = total_cost_cents + surcharge_or_credit_cents`, status becomes
   `COMPLETED`, and `completed_by` records the caller.
4. **Given** `charge_billsplit = true`, **When** completion settles, **Then** a
   `LENDING_CREDIT` ledger transaction is created for the owner and a `LENDING_DEBIT` for
   the renter (each for `settlement_cents`); `users_orgs.balance_cents` for both updates
   automatically via the database trigger on `ledger_transactions` — the service layer
   itself never writes `balance_cents` directly for this flow.
5. **Given** `charge_billsplit = false`, **When** completion settles, **Then** no ledger
   transaction is created and no balance changes for either party; both parties' completion
   notifications and emails instead include a highlighted reminder that the rental payment
   must be settled directly between them and is excluded from monthly bill-splitting.
6. **Given** completion succeeds, **When** the tool's remaining rentals are checked,
   **Then** the tool's status is set to `RENTED` if it still has any `ACTIVE` or
   `SCHEDULED` rental, otherwise `AVAILABLE`.
7. **Given** completion succeeds, **When** notifications are dispatched, **Then** the
   credit/debit-update notifications and the completion notifications are sent from
   detached background goroutines (`context.WithoutCancel`) so `CompleteRental` returns to
   the caller without waiting for notification delivery.
8. **Given** a rental not in `ACTIVE`/`SCHEDULED`/`OVERDUE` status, **When**
   `CompleteRental` is called, **Then** it is rejected with the current status named in the
   error.

---

### User Story 3 - Return Date Change Negotiation (Priority: P2)

Either party can propose new dates before pickup; once active, only the renter can request
a return-date extension, which the owner must approve or counter-reject; the renter can
then acknowledge a rejection (rolling back) or cancel their own pending request.

**Why this priority**: A secondary negotiation path layered on top of User Stories 1-2;
every rental goes through the core lifecycle, only some go through a date renegotiation.

**Independent Test**: As the renter on an `ACTIVE` rental, request a return-date extension
(status → `RETURN_DATE_CHANGED`); as owner, reject it with a counter-proposed date (status →
`RETURN_DATE_CHANGE_REJECTED`); as renter, acknowledge the rejection and confirm the rental
rolls back to the last agreed end date and returns to `ACTIVE`/`OVERDUE`.

**Acceptance Scenarios**:

1. **Given** a rental in `PENDING`/`APPROVED`/`SCHEDULED` (pre-pickup) status, **When**
   either party calls `ChangeRentalDates`, **Then** both `start_date` and `end_date` may
   change, cost is recalculated, and status resets to `PENDING` (if the renter changed
   dates, requiring owner re-approval) or stays/becomes `APPROVED` (if the owner changed
   dates, requiring renter confirmation).
2. **Given** a rental in `ACTIVE`/`OVERDUE` status, **When** the renter calls
   `ChangeRentalDates` with a new `end_date` only (changing `start_date` is rejected —
   "cannot change start date of active rental"), **Then** `end_date` and cost update and
   status becomes `RETURN_DATE_CHANGED`; the owner is notified.
3. **Given** a rental already in `RETURN_DATE_CHANGED` status, **When** the renter calls
   `ChangeRentalDates` again (amending their own pending request), **Then** the request is
   updated in place (status stays `RETURN_DATE_CHANGED`) and the owner is re-notified.
4. **Given** a rental in `RETURN_DATE_CHANGED` status, **When** the owner calls
   `ApproveReturnDateChange`, **Then** `end_date` is copied to `last_agreed_end_date`,
   status becomes `ACTIVE` (or `OVERDUE` if the new end date has already passed), and the
   renter is notified.
5. **Given** a rental in `RETURN_DATE_CHANGED` status, **When** the owner calls
   `RejectReturnDateChange` with a mandatory, different-from-requested `new_end_date` and a
   `reason`, **Then** status becomes `RETURN_DATE_CHANGE_REJECTED`, `end_date` is set to
   the owner's counter-proposal, cost is recalculated, and the renter is notified (in-app +
   email) with the reason, new date, and updated cost.
6. **Given** a rental in `RETURN_DATE_CHANGE_REJECTED` status, **When** the renter calls
   `AcknowledgeReturnDateRejection`, **Then** `end_date` rolls back to
   `last_agreed_end_date`, cost is recalculated from that rollback date, the rejection
   reason is cleared, and status becomes `ACTIVE` or `OVERDUE` depending on whether the
   rolled-back end date has already passed.
7. **Given** a rental in `RETURN_DATE_CHANGED` status, **When** the renter calls
   `CancelReturnDateChange`, **Then** the same rollback as Scenario 6 occurs (end date and
   cost revert to the last agreed value, status becomes `ACTIVE`/`OVERDUE`), and the owner
   is notified of the cancellation.

---

### User Story 4 - Rental Visibility (Priority: P3)

Participants and tool owners can look up individual rentals and lists of their own rentals,
lendings, or a specific tool's rental history.

**Why this priority**: Read-only visibility into what the other three stories produce.

**Independent Test**: As a renter, call `ListMyRentals` with a status filter and confirm
only matching rentals return; as the tool's owner, call `ListToolRentals` and confirm the
full history for that tool, including rentals by other renters.

**Acceptance Scenarios**:

1. **Given** a `rental_id`, **When** `GetRental` is called by the renter or the owner,
   **Then** the rental is returned. **As-built**, a caller who is an org admin but neither
   party is rejected (see Known Discrepancy 4).
2. **Given** a status-filter array, **When** `ListMyRentals` (as renter) or
   `ListMyLendings` (as owner) is called, **Then** matching statuses are OR'd together (any
   match returns the rental); an empty filter returns all statuses.
3. **Given** a `tool_id`, **When** `ListToolRentals` is called, **Then** the caller must be
   that tool's owner (rejected otherwise), and the full paginated rental history for the
   tool is returned, optionally filtered by `organization_id` and status.

---

### Edge Cases

- Because `CreateRentalRequest` performs no availability check (Known Discrepancy 1), two
  different renters can each successfully create a `PENDING`/`APPROVED`/`SCHEDULED` rental
  for the same tool over overlapping dates; nothing in the create path prevents this, and
  nothing surfaces it to either owner before they approve.
- `CancelRentalRequest`'s lack of a status check (Known Discrepancy 3) means it can be
  called on an already-`ACTIVE` (picked up) or already-`COMPLETED` rental, leaving a
  physically-in-use or already-settled tool marked `CANCELLED` with no further
  state-machine path back to a sane status.
- The renter-balance check referenced nowhere in `grpc_api_business_logic.md` but present
  as commented-out dead code in `CreateRentalRequest` (`// Check balance - DISABLED FOR
  NOW`) is confirmed genuinely disabled — a corresponding e2e test case
  ("CreateRentalRequest with Insufficient Balance") is also commented out in
  `tests/e2e/rental_test.go`. Not a doc discrepancy (the doc never requires a balance
  check), just confirmation the feature is intentionally inactive, not silently broken.
- `RejectReturnDateChange` requires the owner's counter-proposed `new_end_date` to differ
  from the renter's requested date, but does not require it to be *later* than the
  requested date or even than `start_date` — an owner could counter-propose an earlier date
  than the rental's own `start_date` and the service would accept it (cost recalculation
  would then depend on `utils.CalculateRentalCost`'s own handling of a negative-duration
  range, which this spec does not re-verify — see the pricing algorithm's own test suite).

## Known Discrepancies *(code vs. documentation, verified against source)*

1. **HIGH — No tool availability / overlap check, contrary to documented step 1 of
   `CreateRentalRequest`.** `grpc_api_business_logic.md` documents step 1 as "Verify the
   tool is either available or its rental schedule is free for the specified start_date and
   end_date." **As-built**, `rentalService.CreateRentalRequest`
   (`internal/service/rental.go`) contains only a comment — `// Verify tool availability
   (simplified: check status)` / `// Ideally check if tool is already rented in this
   period.` — with no actual check following it. A repo-wide search for
   overlap/conflict/availability logic (`overlap`, `conflict`, `schedule.*free`,
   `IsAvailable`) found no implementation anywhere in `internal/`. **Net effect**: the
   system currently allows unlimited overlapping rental requests for the same tool; nothing
   prevents two renters from both getting `APPROVED`/`SCHEDULED` rentals for dates that
   overlap. This is a data-integrity gap, not an authorization gap — recommend treating it
   with real urgency, though it is lower severity than
   `specs/004-organizations-administration`'s Known Discrepancy 1 since it doesn't expose
   or corrupt other users' data, only allows a scheduling conflict the app was supposed to
   prevent.
2. **`RejectRentalRequest` does not check the rental is currently `PENDING`.**
   `grpc_api_business_logic.md`'s "Reject Rental Request" doesn't explicitly restrict
   status either, but by contrast `ApproveRentalRequest` (right above it in the same file)
   *does* enforce `PENDING`. **As-built**, `RejectRentalRequest` can transition a rental
   from any status (including `APPROVED`, `SCHEDULED`, or even `COMPLETED`) straight to
   `REJECTED`. Likely an oversight given the sibling method's stricter check, not a
   deliberate design choice.
3. **`CancelRentalRequest` does not check rental status at all** (see Edge Cases) — same
   category of gap as Discrepancy 2, one level more permissive since even `ApproveRentalRequest`-style
   status gating is entirely absent here.
4. **`GetRental` does not grant org-admin access, contrary to documentation.**
   `grpc_api_business_logic.md`'s "Get Rental" business logic says: "check user_id is
   either the renter, the owner, or an admin in the organization of rentals.org_id."
   **As-built**, `rentalService.GetRental` checks only `rt.RenterID != userID && rt.OwnerID
   != userID` — an org admin with no personal stake in the rental is rejected, unlike every
   admin-scoped read elsewhere in the codebase (e.g. Bill Split's `GetPaymentDetail`, which
   does correctly extend access to org admins).

## Current Test Coverage Baseline *(informational — grounds the next /speckit-tasks pass, not a requirement)*

Verified by inspection of `tests/unit/rental_test.go`, `tests/e2e/rental_test.go`,
`tests/e2e/rental_steps_test.go`, and `tests/integration/rental_ledger_test.go`:

**Covered**: `CreateRentalRequest`'s happy path and cost calculation (unit); `CompleteRental`
including ledger/balance effects, snapshot-vs-tool-price recomputation, the participant/status
reject clauses, and the `charge_billsplit` gate (unit + integration — as of 2026-07-22,
`TestRentalService_CompleteRental_Integration` calls `service.CompleteRental` directly against
a real DB; the prior integration test under this same claim bypassed the service method
entirely via direct repository writes, see `sbr/rtm/005-rentals.rtm.md`); `FinalizeRentalRequest`,
`ActivateRental`, `ChangeRentalDates` (multiple sub-cases), `RejectReturnDateChange` (unit);
a full end-to-end lifecycle including reject, an in-flight extension-request update, cancel,
and a `charge_billsplit=false` variant (e2e).

**Not covered anywhere**:

- **The double-booking gap itself (Known Discrepancy 1)** — no test creates two
  overlapping rental requests for the same tool and asserts either rejection or acceptance;
  this is the highest-value test to add (or, more precisely, the fix to build and then
  test) in this entire domain.
- `RejectRentalRequest`/`CancelRentalRequest` called against a non-`PENDING`/non-cancelable
  status (Known Discrepancies 2 and 3) — no test exercises either misuse case.
- `GetRental` called by an org admin who is neither renter nor owner (Known Discrepancy 4).
- `ApproveReturnDateChange` and `AcknowledgeReturnDateRejection` at the unit-test tier
  (only reachable indirectly through e2e today).
- `ListToolRentals`'s ownership-rejection path and its status/organization filtering.
- `ListMyRentals`/`ListMyLendings`'s OR-combination of multiple statuses in one call.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: `CreateRentalRequest` MUST validate `end_date > start_date`, MUST snapshot the
  tool's current price fields onto the rental record, and MUST compute
  `total_cost_cents` from that snapshot. **As-built, it MUST NOT be assumed to reject
  overlapping-date requests for the same tool** (Known Discrepancy 1) — this is the target
  correctness bar for the highest-priority follow-up task in this domain, not current
  behavior.
- **FR-002**: `ApproveRentalRequest` MUST require the caller to be the tool's owner and the
  rental to be `PENDING`.
- **FR-003**: `FinalizeRentalRequest` MUST require the caller to be the renter and the
  rental to be `APPROVED`, and MUST set the tool's status to `RENTED` on success.
- **FR-004**: `CompleteRental` MUST require the caller to be a participant and the rental to
  be `ACTIVE`/`SCHEDULED`/`OVERDUE`, MUST recompute cost from the rental's own price
  snapshot (never the tool's current prices), and MUST only create ledger transactions /
  update balances when `charge_billsplit = true`.
- **FR-005**: Every return-date-change transition (`ChangeRentalDates` in its active-rental
  case, `ApproveReturnDateChange`, `RejectReturnDateChange`, `AcknowledgeReturnDateRejection`,
  `CancelReturnDateChange`) MUST recompute `total_cost_cents` from the rental's price
  snapshot whenever `end_date` changes.
- **FR-006**: `GetRental` MUST grant access to the rental's renter and owner.
  **As-built, it MUST NOT be assumed to also grant access to org admins** (Known
  Discrepancy 4) despite that being documented.

### Key Entities

- **Rental**: `rentals` table — participants (`renter_id`, `owner_id`), `tool_id`,
  `org_id`, date range, a full **immutable price snapshot** taken at creation
  (`duration_unit`, `daily_price_cents`, `weekly_price_cents`, `monthly_price_cents`,
  `replacement_cost_cents`) so later tool price changes never retroactively affect an
  existing rental, `last_agreed_end_date` (rollback anchor for the return-date-change
  sub-flow), `status` (ten values — see the `RentalStatus` constants), settlement fields
  (`return_condition`, `surcharge_or_credit_cents`, `charge_billsplit`, `completed_by`).
- **LedgerTransaction**: created only when a rental completes with `charge_billsplit =
  true`; `users_orgs.balance_cents` updates via a database trigger, not directly from
  service code.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001 (highest priority in this spec)**: `CreateRentalRequest` rejects (or the product
  decision is made and documented to explicitly allow) a new request whose date range
  overlaps an existing `PENDING`/`APPROVED`/`SCHEDULED`/`ACTIVE`/`OVERDUE` rental for the
  same tool — closing Known Discrepancy 1, with a dedicated automated test.
- **SC-002**: `RejectRentalRequest` and `CancelRentalRequest` either gain explicit status
  gating consistent with their sibling methods, or the spec is updated to state plainly
  that they are intentionally unrestricted — Known Discrepancies 2 and 3 do not remain
  undocumented inconsistencies.
- **SC-003**: `GetRental`'s org-admin access is either implemented to match documentation
  or the documentation is corrected — Known Discrepancy 4 does not remain silently
  contradictory.
- **SC-004**: Every one of the ten `RentalStatus` values has at least one automated test
  that reaches it via the RPC(s) that are supposed to produce it.

## Assumptions

- The tiered pricing algorithm (`utils.CalculateRentalCost`) is treated as a verified,
  separately-specified dependency (`docs/design/tool-rental-pricing-algorithm.md` +
  `tests/unit/pricing_test.go`); this spec does not re-verify its internal correctness.
- Fixing Known Discrepancy 1 requires a real design decision (which statuses count as
  "occupying" the schedule, whether back-to-back same-day handoffs are allowed, how to
  query efficiently) that is more involved than the single-pattern fixes in
  `specs/004-organizations-administration`; this spec documents the gap and its severity
  but does not itself propose a specific overlap-detection query or apply a fix.
- General bill-split behavior once a rental's `charge_billsplit=true` ledger transaction
  feeds into the monthly settlement pipeline is specified in
  `specs/001-bill-split/spec.md`, not here.
