# Rental Contracts

Source: `api/proto/ubertool_trusted_backend/v1/rental_service.proto`

> **Correction 2026-07-28**: `current_organization_id`/`context_switch_required` (below) were
> never added — no server-side "current org" exists to switch to or from (see
> `docs/design/multi-org.md`). FR-008 is implemented using the pre-existing
> `CreateRentalRequestRequest.organization_id` field, validated directly against
> `isSharedOrganization`.
>
> **Correction 2026-07-29**: `CreateRentalRequestResponse.shared_organization_ids`/
> `shared_organization_names` were REMOVED. A rental-request response is the wrong place for a
> list of alternative orgs to browse — that's a point-in-time write, not a discovery surface.
> The renter is expected to already know which orgs are valid *before* calling
> `CreateRentalRequest`, from `Tool.owner.orgs` on the `ToolService.SearchTools`/`GetTool`
> response (see `specs/006-tools-image-storage/contracts/README.md` FR-009). FR-008's rejection
> is now a plain `FAILED_PRECONDITION` (human-readable shared-org names in the error message,
> not a structured field) — a defensive backstop for the rare case where membership changed
> between search and request, not the primary UX for picking an org. See
> `sbr/rtm/005-rentals.rtm.md` FR-008 for the as-built behavior.

## RentalService RPCs

| RPC | Request | Response | Notes |
|-----|---------|----------|-------|
| `CreateRentalRequest` | `CreateRentalRequestRequest` | `CreateRentalRequestResponse` | **FR-008**: validates the existing `organization_id` field via shared-org membership; no shared-org list in the response |
| `ApproveRentalRequest` | `ApproveRentalRequest` | `ApproveRentalResponse` | Owner only |
| `RejectRentalRequest` | `RejectRentalRequest` | `RejectRentalResponse` | Owner only |
| `FinalizeRentalRequest` | `FinalizeRentalRequest` | `FinalizeRentalResponse` | Renter only |
| `CancelRentalRequest` | `CancelRentalRequest` | `CancelRentalResponse` | Renter (before ACTIVE) or Owner |
| `CompleteRental` | `CompleteRentalRequest` | `CompleteRentalResponse` | Owner only |
| `ChangeRentalDates` | `ChangeRentalDatesRequest` | `ChangeRentalDatesResponse` | Negotiation flow |
| `GetRental` | `GetRentalRequest` | `GetRentalResponse` | Renter or owner |
| `ListMyRentals` | `ListMyRentalsRequest` | `ListMyRentalsResponse` | As renter |
| `ListMyLendings` | `ListMyLendingsRequest` | `ListMyLendingsResponse` | As owner |

## FR-008: No New Fields Needed

```protobuf
message CreateRentalRequestRequest {
  int32 tool_id = 1;
  string start_date = 2;
  string end_date = 3;
  int32 organization_id = 4;   // already existed — the caller's explicit choice, validated by FR-008
}

message CreateRentalRequestResponse {
  RentalRequest rental_request = 1;
  // No shared-org list here — see SearchTools/GetTool's Tool.owner.orgs instead.
}
```

## Rental Message

```protobuf
message Rental {
  int64 id = 1;
  string tool_id = 2;
  string renter_id = 3;
  string owner_id = 4;
  string org_id = 5;              // The caller-supplied, validated shared org (both renter and owner are active members)
  string start_date = 6;
  string end_date = 7;
  RentalStatus status = 8;
  int64 price_snapshot_cents = 9;
  string created_at = 10;
  string updated_at = 11;
}

enum RentalStatus {
  RENTAL_STATUS_UNSPECIFIED = 0;
  RENTAL_STATUS_PENDING = 1;
  RENTAL_STATUS_APPROVED = 2;
  RENTAL_STATUS_SCHEDULED = 3;
  RENTAL_STATUS_ACTIVE = 4;
  RENTAL_STATUS_COMPLETED = 5;
  RENTAL_STATUS_CANCELLED = 6;
  RENTAL_STATUS_REJECTED = 7;
}
```

## Authorization Rules

| RPC | Required Role | Org Scope |
|-----|---------------|-----------|
| `CreateRentalRequest` | Any member | Validates renter AND tool owner are both active members of the caller-supplied `organization_id` |
| `ApproveRentalRequest` | Tool owner | Rental's org (`rental.org_id`) |
| `RejectRentalRequest` | Tool owner | Rental's org |
| `FinalizeRentalRequest` | Renter | Rental's org |
| `CompleteRental` | Tool owner | Rental's org |
| `GetRental` | Renter or owner | Rental's org |
| `ListMyRentals` | Any | `organization_id` passed explicitly in the request (FR-007) |
| `ListMyLendings` | Any | `organization_id` passed explicitly in the request (FR-007) |

## No Breaking Changes

FR-008 required no new fields on `CreateRentalRequestRequest` — it validates the pre-existing
`organization_id` field. `CreateRentalRequestResponse.shared_organization_ids`/
`shared_organization_names` (proto fields 10–11) were removed 2026-07-29 — see the correction
note above.