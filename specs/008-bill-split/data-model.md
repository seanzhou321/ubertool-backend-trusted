# Data Model: Bill Split

## Entities

### bills (existing)
| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | BIGSERIAL | PK | Bill ID |
| org_id | UUID | NOT NULL, FK → orgs.id | Organization |
| debtor_id | UUID | NOT NULL, FK → users.id | Owes money |
| creditor_id | UUID | NOT NULL, FK → users.id | Owed money |
| amount_cents | BIGINT | NOT NULL | Amount in cents |
| status | TEXT | NOT NULL, CHECK IN ('PENDING','ACKNOWLEDGED','DISPUTED','RESOLVED') | Bill status |
| settlement_month | DATE | NOT NULL | YYYY-MM (month of settlement) |
| notice_sent_at | TIMESTAMPTZ | | When notice email sent |
| acknowledged_at | TIMESTAMPTZ | | When both parties acknowledged |
| disputed_at | TIMESTAMPTZ | | When dispute filed |
| resolved_at | TIMESTAMPTZ | | When resolved |
| created_at | TIMESTAMPTZ | NOT NULL DEFAULT now() | |

**Indexes**: `(org_id, status)`, `(debtor_id, status)`, `(creditor_id, status)`, `(settlement_month, org_id)`

### bill_actions (existing)
| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | BIGSERIAL | PK | Action ID |
| bill_id | BIGINT | NOT NULL, FK → bills.id | Parent bill |
| actor_id | UUID | NOT NULL, FK → users.id | User who acted |
| action_type | TEXT | NOT NULL, CHECK IN ('CREATED','NOTICE_SENT','ACKNOWLEDGED','DISPUTED','RESOLVED') | Action |
| metadata | JSONB | | Extra data |
| created_at | TIMESTAMPTZ | NOT NULL DEFAULT now() | |

## Domain Types (internal/domain/bill.go)

```go
package domain

type BillStatus string

const (
    BillStatusPending       BillStatus = "PENDING"
    BillStatusAcknowledged  BillStatus = "ACKNOWLEDGED"
    BillStatusDisputed      BillStatus = "DISPUTED"
    BillStatusResolved      BillStatus = "RESOLVED"
)

type Bill struct {
    ID                int64
    OrgID             string
    DebtorID          string
    CreditorID        string
    AmountCents       int64
    Status            BillStatus
    SettlementMonth   time.Time
    NoticeSentAt      *time.Time
    AcknowledgedAt    *time.Time
    DisputedAt        *time.Time
    ResolvedAt        *time.Time
    CreatedAt         time.Time
}

type BillAction struct {
    ID        int64
    BillID    int64
    ActorID   string
    Type      string
    Metadata  map[string]interface{}
    CreatedAt time.Time
}
```

## gRPC Contract (bill_split_service.proto)

```protobuf
service BillSplitService {
  // ... existing 13 RPCs ...
  
  // NEW FR-014
  rpc GetGlobalBillSplitSummary(GetGlobalBillSplitSummaryRequest) returns (GetGlobalBillSplitSummaryResponse);
  
  // NEW FR-015
  rpc GetOrganizationBillSplitSummary(GetOrganizationBillSplitSummaryRequest) returns (GetOrganizationBillSplitSummaryResponse);
}

// FR-014
message GetGlobalBillSplitSummaryRequest {}
message GetGlobalBillSplitSummaryResponse {
  int32 pending_count = 1;
  int32 acknowledged_count = 2;
  int32 disputed_count = 3;
  int32 resolved_count = 4;
  int64 total_amount_cents = 5;
}

// FR-015
message GetOrganizationBillSplitSummaryRequest {
  string organization_id = 1;
}
message GetOrganizationBillSplitSummaryResponse {
  int32 pending_count = 1;
  int32 acknowledged_count = 2;
  int32 disputed_count = 3;
  int32 resolved_count = 4;
  int64 total_amount_cents = 5;
}
```

## Multi-Org Queries

### FR-014: Global Summary (all user's active orgs)
```sql
WITH user_orgs AS (
  SELECT org_id FROM users_orgs 
  WHERE user_id = $1 AND status = 'ACTIVE'
)
SELECT 
  COUNT(*) FILTER (WHERE status = 'PENDING') AS pending_count,
  COUNT(*) FILTER (WHERE status = 'ACKNOWLEDGED') AS acknowledged_count,
  COUNT(*) FILTER (WHERE status = 'DISPUTED') AS disputed_count,
  COUNT(*) FILTER (WHERE status = 'RESOLVED') AS resolved_count,
  COALESCE(SUM(amount_cents) FILTER (WHERE status != 'RESOLVED'), 0) AS total_amount_cents
FROM bills
WHERE (debtor_id = $1 OR creditor_id = $1)
  AND org_id IN (SELECT org_id FROM user_orgs);
```

### FR-015: Org Summary (admin only)
```sql
SELECT 
  COUNT(*) FILTER (WHERE status = 'PENDING') AS pending_count,
  COUNT(*) FILTER (WHERE status = 'ACKNOWLEDGED') AS acknowledged_count,
  COUNT(*) FILTER (WHERE status = 'DISPUTED') AS disputed_count,
  COUNT(*) FILTER (WHERE status = 'RESOLVED') AS resolved_count,
  COALESCE(SUM(amount_cents) FILTER (WHERE status != 'RESOLVED'), 0) AS total_amount_cents
FROM bills
WHERE org_id = $1;
```

## Proto Regeneration
```bash
make proto
# api/gen/v1/bill_split_service.pb.go
```