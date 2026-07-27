# Data Model: Ledger

## Entities

### users_orgs (existing)
| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| user_id | UUID | PK, FK → users.id | User reference |
| org_id | UUID | PK, FK → organizations.id | Organization reference |
| role | TEXT | NOT NULL, CHECK IN ('MEMBER','ADMIN','SUPER_ADMIN') | Org role |
| balance_cents | BIGINT | NOT NULL DEFAULT 0 | Per-org credit/debit balance |
| status | TEXT | NOT NULL DEFAULT 'ACTIVE', CHECK IN ('ACTIVE','BLOCKED','PENDING') | Membership status |
| joined_at | TIMESTAMPTZ | NOT NULL DEFAULT now() | When membership started |

**Indexes**: `(user_id)`, `(org_id)`, `(org_id, status)`

### ledger_transactions (existing)
| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | BIGSERIAL | PK | Transaction ID |
| org_id | UUID | NOT NULL, FK → organizations.id | Organization |
| user_id | UUID | NOT NULL, FK → users.id | User (debtor/creditor) |
| amount_cents | BIGINT | NOT NULL | Signed amount (+credit, -debit) |
| type | TEXT | NOT NULL, CHECK IN ('RENTAL_PAYMENT','BILL_SETTLEMENT','MANUAL_ADJUSTMENT') | Transaction type |
| reference_id | UUID | NULLABLE | Optional link (rental_id, bill_id) |
| status | TEXT | NOT NULL, CHECK IN ('POSTED','VOIDED') | Transaction status |
| created_at | TIMESTAMPTZ | NOT NULL DEFAULT now() | When posted |

**Indexes**: `(org_id, user_id, created_at)`, `(org_id, reference_id)`

## Domain Types (internal/domain/ledger.go)

```go
package domain

type TransactionType string

const (
    TransactionTypeRentalPayment     TransactionType = "RENTAL_PAYMENT"
    TransactionTypeBillSettlement    TransactionType = "BILL_SETTLEMENT"
    TransactionTypeManualAdjustment  TransactionType = "MANUAL_ADJUSTMENT"
)

type TransactionStatus string

const (
    TransactionStatusPosted  TransactionStatus = "POSTED"
    TransactionStatusVoided  TransactionStatus = "VOIDED"
)
```

## Cross-Org Rollup Query (FR-004)

```sql
-- Input: user_id, org_id (0 = all active orgs)
WITH active_orgs AS (
    SELECT org_id FROM users_orgs
    WHERE user_id = $1 AND status = 'ACTIVE'
      AND ($2 = 0 OR org_id = $2)
),
txns AS (
    SELECT lt.*
    FROM ledger_transactions lt
    JOIN active_orgs ao ON lt.org_id = ao.org_id
    WHERE lt.user_id = $1 AND lt.status = 'POSTED'
),
balance AS (
    SELECT COALESCE(SUM(amount_cents), 0) AS balance_cents
    FROM txns
),
status_counts AS (
    SELECT 
        'RENTAL_PAYMENT' AS type,
        COUNT(*) AS count
    FROM txns WHERE type = 'RENTAL_PAYMENT'
    UNION ALL
    SELECT 'BILL_SETTLEMENT', COUNT(*) FROM txns WHERE type = 'BILL_SETTLEMENT'
    UNION ALL
    SELECT 'MANUAL_ADJUSTMENT', COUNT(*) FROM txns WHERE type = 'MANUAL_ADJUSTMENT'
)
SELECT 
    b.balance_cents,
    COALESCE(jsonb_object_agg(sc.type, sc.count), '{}'::jsonb) AS status_count
FROM balance b
CROSS JOIN LATERAL (
    SELECT * FROM status_counts
) sc;
```

## gRPC Contract (api/proto/.../ledger_service.proto)

```protobuf
message GetBalanceRequest {
    string org_id = 1;  // "0" = all active orgs (cross-org rollup)
}

message GetBalanceResponse {
    int64 balance_cents = 1;
}

message GetTransactionsRequest {
    string org_id = 1;
    int32 page_size = 2;
    string page_token = 3;
}

message Transaction {
    int64 id = 1;
    string org_id = 2;
    int64 amount_cents = 3;
    string type = 4;
    string reference_id = 5;
    string status = 6;
    string created_at = 7;
}

message GetTransactionsResponse {
    repeated Transaction transactions = 1;
    string next_page_token = 2;
}

message GetLedgerSummaryRequest {
    string org_id = 1;  // "0" = all active orgs (cross-org rollup)
}

message GetLedgerSummaryResponse {
    int64 balance_cents = 1;
    map<string, int32> status_count = 2;  // type -> count
}
```

**No proto changes required** — `org_id` as string already accepts "0" sentinel.