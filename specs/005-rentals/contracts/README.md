# Rental Contracts

Source: `api/proto/ubertool_trusted_backend/v1/rental_service.proto`

## RentalService RPCs

| RPC | Request | Response | Notes |
|-----|---------|----------|-------|
| `CreateRentalRequest` | `CreateRentalRequest` | `CreateRentalResponse` | **FR-008**: `current_organization_id`, `context_switch_required` |
| `ApproveRentalRequest` | `ApproveRentalRequest` | `ApproveRentalResponse` | Owner only |
| `RejectRentalRequest` | `RejectRentalRequest` | `RejectRentalResponse` | Owner only |
| `FinalizeRentalRequest` | `FinalizeRentalRequest` | `FinalizeRentalResponse` | Renter only |
| `CancelRentalRequest` | `CancelRentalRequest` | `CancelRentalResponse` | Renter (before ACTIVE) or Owner |
| `CompleteRental` | `CompleteRentalRequest` | `CompleteRentalResponse` | Owner only |
| `ChangeRentalDates` | `ChangeRentalDatesRequest` | `ChangeRentalDatesResponse` | Negotiation flow |
| `GetRental` | `GetRentalRequest` | `GetRentalResponse` | Renter or owner |
| `ListMyRentals` | `ListMyRentalsRequest` | `ListMyRentalsResponse` | As renter |
| `ListMyLendings` | `ListMyLendingsRequest` | `ListMyLendingsResponse` | As owner |

## FR-008 New Fields

```protobuf
message CreateRentalRequest {
  string tool_id = 1;
  string start_date = 2;
  string end_date = 3;
  string current_organization_id = 20;  // NEW: client's current org
  string idempotency_key = 21;
}

message CreateRentalResponse {
  Rental rental = 1;
  bool context_switch_required = 10;      // NEW
  string target_organization_id = 11;     // NEW
  string target_organization_name = 12;   // NEW
}
```

## Rental Message

```protobuf
message Rental {
  int64 id = 1;
  string tool_id = 2;
  string renter_id = 3;
  string owner_id = 4;
  string org_id = 5;              // Owner's org (not renter's!)
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
| `CreateRentalRequest` | Any member | Validates membership in `current_organization_id` |
| `ApproveRentalRequest` | Tool owner | Owner's org (rental.org_id) |
| `RejectRentalRequest` | Tool owner | Owner's org |
| `FinalizeRentalRequest` | Renter | Rental's org |
| `CompleteRental` | Tool owner | Rental's org |
| `GetRental` | Renter or owner | Rental's org |
| `ListMyRentals` | Any | Current org (or all if admin?) |
| `ListMyLendings` | Any | Current org |

## No Breaking Changes

All new fields are optional with sensible defaults:
- `context_switch_required` defaults to `false`
- `current_organization_id` optional; if omitted, uses Redis current org