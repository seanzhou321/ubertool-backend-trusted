# Data Model: Rentals

## Entities

### rentals (existing)
| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | BIGSERIAL | PK | Rental ID |
| tool_id | UUID | NOT NULL, FK → tools.id | Tool being rented |
| renter_id | UUID | NOT NULL, FK → users.id | Borrower |
| owner_id | UUID | NOT NULL, FK → users.id | Tool owner |
| org_id | UUID | NOT NULL, FK → orgs.id | **Rental org (shared between renter + owner)** |
| start_date | DATE | NOT NULL | Rental start |
| end_date | DATE | NOT NULL | Rental end |
| status | TEXT | NOT NULL, CHECK IN ('PENDING','APPROVED','SCHEDULED','ACTIVE','COMPLETED','CANCELLED','REJECTED') | Rental status |
| price_snapshot_cents | BIGINT | NOT NULL | Frozen price at creation |
| idempotency_key | UUID | UNIQUE | Client-provided for retries |
| created_at | TIMESTAMPTZ | NOT NULL DEFAULT now() | |
| updated_at | TIMESTAMPTZ | NOT NULL DEFAULT now() | |

**Indexes**: `(org_id, status)`, `(renter_id, status)`, `(owner_id, status)`, `(tool_id, status)`

**New Constraint (FR-008)**:
```sql
-- BEFORE INSERT trigger or CHECK constraint verifying shared active membership
ALTER TABLE rentals ADD CONSTRAINT rentals_shared_org_check
CHECK (
  EXISTS (
    SELECT 1 FROM users_orgs uo_renter
    JOIN users_orgs uo_owner ON uo_renter.org_id = uo_owner.org_id
    WHERE uo_renter.user_id = NEW.renter_id
      AND uo_owner.user_id = NEW.owner_id
      AND uo_renter.org_id = NEW.org_id
      AND uo_renter.status = 'ACTIVE'
      AND uo_owner.status = 'ACTIVE'
  )
);
```

### tools (from Tools domain — referenced)
| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | UUID | PK | Tool ID |
| owner_id | UUID | NOT NULL, FK → users.id | Owner |
| owner_org_id | UUID | **REMOVED** — tools belong to users, not orgs | |
| status | TEXT | NOT NULL, CHECK IN ('AVAILABLE','RENTED','MAINTENANCE','UNAVAILABLE') | |

> **Note**: The `owner_org_id` column previously added to `tools` is REMOVED per the corrected multi-org design (tools belong to users, rental org chosen from shared orgs).

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

// FR-008 — no new fields; organization_id already existed
message CreateRentalRequestRequest {
  int32 tool_id = 1;
  string start_date = 2;
  string end_date = 3;
  int32 organization_id = 4;  // pre-existing, required — the caller's explicit choice
}

message CreateRentalRequestResponse {
  RentalRequest rental_request = 1;
  // No shared-org list here (removed 2026-07-29) — see Tool.owner.orgs on
  // SearchTools/GetTool (006-tools-image-storage FR-009) for org discovery instead.
}
```

## Multi-Org Shared-Org Validation Logic (FR-008 — Corrected)

**No `owner_org_id` on tools.** Tools belong to users (`owner_id`). Rental `org_id` must be a
shared org. There is no middleware/Redis "effective org" — `organization_id` comes directly from
the request, exactly as the caller sent it.

```sql
-- 1. Verify caller (renter) is an active member of the requested organization_id
SELECT 1 FROM users_orgs 
WHERE user_id = $renter_id AND org_id = $organization_id AND status = 'ACTIVE';

-- 2. Verify tool owner is ALSO an active member of that same organization_id
SELECT 1 FROM users_orgs 
WHERE user_id = $tool_owner_id AND org_id = $organization_id AND status = 'ACTIVE';

-- 3. If step 2 fails, find shared orgs for the human-readable error message only
SELECT o.id, o.name FROM orgs o
JOIN users_orgs uo_renter ON uo_renter.org_id = o.id AND uo_renter.user_id = $renter_id AND uo_renter.status = 'ACTIVE'
JOIN users_orgs uo_owner ON uo_owner.org_id = o.id AND uo_owner.user_id = $tool_owner_id AND uo_owner.status = 'ACTIVE';

-- 4. Create rental in the requested org (step 2 passed)
INSERT INTO rentals (tool_id, renter_id, owner_id, org_id, ...)
VALUES ($1, $2, $3, $organization_id, ...);
```

**Error response (when step 2 fails):**
```
code: FAILED_PRECONDITION
message: "organization [Church A] is not shared with the tool owner. Shared organizations: [Church B]"
```
No structured `shared_org_ids`/`shared_org_names` field — this is a defensive backstop, not the
primary way to discover valid orgs. The client should already know which orgs are valid from
`Tool.owner.orgs` on the `SearchTools`/`GetTool` response it used to find this tool in the first
place, and retries `CreateRentalRequest` with one of those.

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