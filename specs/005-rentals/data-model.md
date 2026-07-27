# Data Model: Rentals

## Entities

### rentals (existing)
| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | BIGSERIAL | PK | Rental ID |
| tool_id | UUID | NOT NULL, FK → tools.id | Tool being rented |
| renter_id | UUID | NOT NULL, FK → users.id | Borrower |
| owner_id | UUID | NOT NULL, FK → users.id | Tool owner |
| org_id | UUID | NOT NULL, FK → orgs.id | **Org context (owner's org)** |
| start_date | DATE | NOT NULL | Rental start |
| end_date | DATE | NOT NULL | Rental end |
| status | TEXT | NOT NULL, CHECK IN ('PENDING','APPROVED','SCHEDULED','ACTIVE','COMPLETED','CANCELLED','REJECTED') | Rental status |
| price_snapshot_cents | BIGINT | NOT NULL | Frozen price at creation |
| idempotency_key | UUID | UNIQUE | Client-provided for retries |
| created_at | TIMESTAMPTZ | NOT NULL DEFAULT now() | |
| updated_at | TIMESTAMPTZ | NOT NULL DEFAULT now() | |

**Indexes**: `(org_id, status)`, `(renter_id, status)`, `(owner_id, status)`, `(tool_id, status)`

### tools (from Tools domain — referenced)
| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | UUID | PK | Tool ID |
| owner_id | UUID | NOT NULL, FK → users.id | Owner |
| owner_org_id | UUID | NOT NULL, FK → orgs.id | **Owner's org (rental org)** |
| status | TEXT | NOT NULL, CHECK IN ('AVAILABLE','RENTED','MAINTENANCE','UNAVAILABLE') | |

## Domain Types (internal/domain/rental.go)

```go
package domain

type RentalStatus string

const (
    RentalStatusPending    RentalStatus = "PENDING"
    RentalStatusApproved   RentalStatus = "APPROVED"
    RentalStatusScheduled  RentalStatus = "SCHEDULED"
    RentalStatusActive     RentalStatus = "ACTIVE"
    RentalStatusCompleted  RentalStatus = "COMPLETED"
    RentalStatusCancelled  RentalStatus = "CANCELLED"
    RentalStatusRejected   RentalStatus = "REJECTED"
)

type Rental struct {
    ID                   int64
    ToolID               string
    RenterID             string
    OwnerID              string
    OrgID                string          // Owner's org (not renter's!)
    StartDate            time.Time
    EndDate              time.Time
    Status               RentalStatus
    PriceSnapshotCents   int64
    IdempotencyKey       string
    CreatedAt            time.Time
    UpdatedAt            time.Time
}

type PricingSnapshot struct {
    ToolID             string
    BasePriceCents     int64
    DurationUnit       string
    DurationValue      int32
    CalculatedPrice    int64
    CalculatedAt       time.Time
}
```

## gRPC Contract (rental_service.proto)

```protobuf
service RentalService {
  rpc CreateRentalRequest(CreateRentalRequest) returns (CreateRentalResponse);
  rpc ApproveRentalRequest(ApproveRentalRequest) returns (ApproveRentalResponse);
  rpc RejectRentalRequest(RejectRentalRequest) returns (RejectRentalResponse);
  rpc FinalizeRentalRequest(FinalizeRentalRequest) returns (FinalizeRentalResponse);
  rpc CancelRentalRequest(CancelRentalRequest) returns (CancelRentalResponse);
  rpc CompleteRental(CompleteRentalRequest) returns (CompleteRentalResponse);
  rpc ChangeRentalDates(ChangeRentalDatesRequest) returns (ChangeRentalDatesResponse);
  rpc GetRental(GetRentalRequest) returns (GetRentalResponse);
  rpc ListMyRentals(ListMyRentalsRequest) returns (ListMyRentalsResponse);
  rpc ListMyLendings(ListMyLendingsRequest) returns (ListMyLendingsResponse);
  // ... 6 more RPCs
}

// FR-008 NEW FIELDS
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

## Multi-Org Context Switch Logic (FR-008)

```sql
-- 1. Validate current org membership
SELECT 1 FROM users_orgs 
WHERE user_id = $1 AND org_id = $2 AND status = 'ACTIVE';

-- 2. Get tool's owner org
SELECT owner_org_id FROM tools WHERE id = $1;

-- 3. If different, check membership in owner's org
SELECT 1 FROM users_orgs 
WHERE user_id = $1 AND org_id = $2 AND status = 'ACTIVE';
-- If found → context switch required to $2 (owner_org_id)

-- 4. Create rental in OWNER's org (not requester's!)
INSERT INTO rentals (tool_id, renter_id, owner_id, org_id, ...)
VALUES ($1, $2, $3, $4, ...);  -- $4 = tool.owner_org_id
```

## Rental Status Transitions (existing — verified)

```
PENDING → APPROVED (owner approves)
PENDING → REJECTED (owner rejects)
PENDING → CANCELLED (renter cancels before approve)
APPROVED → SCHEDULED (renter finalizes)
SCHEDULED → ACTIVE (start_date reached)
ACTIVE → COMPLETED (owner calls CompleteRental)
ANY → CANCELLED (by renter before ACTIVE, or owner before ACTIVE)
```

## Known Discrepancy 1 (HIGH): No Double-Booking Prevention

**As-built**: Two rentals for same tool with overlapping dates can both reach ACTIVE
**Gap**: No DB constraint or service-level check
**Fix needed**: Partial unique index `CREATE UNIQUE INDEX ON rentals (tool_id) WHERE status IN ('SCHEDULED','ACTIVE') AND daterange(start_date, end_date, '[]') && daterange(...)`