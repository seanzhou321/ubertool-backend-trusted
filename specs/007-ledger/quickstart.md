# Quickstart: Ledger

## Prerequisites
- Go 1.22+
- PostgreSQL 16 (local Podman or EC2)
- `podman-compose -f podman/trusted-group/compose.yaml up -d` or EC2 deployment
- Protobuf generation: `make proto` (requires `protoc`, `protoc-gen-go`, `protoc-gen-go-grpc`)

## Run Tests

### L1 Unit (pure logic)
```bash
go test -v ./tests/unit/ledger_service_test.go ./internal/service/ledger.go ./internal/domain/ledger.go
```

### L2 Integration (real DB)
```bash
# Requires testcontainers or running Postgres
go test -v ./tests/integration/ledger_test.go
```

### L3 E2E (full gRPC)
```bash
# Start server
go run ./cmd/server

# In another terminal
go test -v ./tests/e2e/ledger_test.go
```

### Smoke (deploy sanity)
```bash
go test -v ./tests/smoke/smoke_test.go -run Ledger
```

## Manual Validation Scenarios

### 1. Single-Org Balance (FR-001)
```bash
grpcurl -plaintext -d '{"org_id": "550e8400-e29b-41d4-a716-446655440000"}' \
  localhost:50051 ubertool.trusted.backend.v1.LedgerService/GetBalance
# Expected: { "balance_cents": 12345 }
```

### 2. Cross-Org Rollup (FR-004) — **Gap**
```bash
grpcurl -plaintext -d '{"org_id": "0"}' \
  localhost:50051 ubertool.trusted.backend.v1.LedgerService/GetBalance
# Current: ERROR (org_id=0 invalid)
# Target: { "balance_cents": 45678 }  -- sum across all active orgs
```

### 3. Paginated Transactions (FR-002)
```bash
grpcurl -plaintext -d '{"org_id": "550e8400-e29b-41d4-a716-446655440000", "page_size": 10}' \
  localhost:50051 ubertool.trusted.backend.v1.LedgerService/GetTransactions
```

### 4. Ledger Summary with Status Counts (FR-003, FR-004)
```bash
# Single org
grpcurl -plaintext -d '{"org_id": "550e8400-e29b-41d4-a716-446655440000"}' \
  localhost:50051 ubertool.trusted.backend.v1.LedgerService/GetLedgerSummary

# Cross-org (gap)
grpcurl -plaintext -d '{"org_id": "0"}' \
  localhost:50051 ubertool.trusted.backend.v1.LedgerService/GetLedgerSummary
```

## Expected Results After Gap Fix

| RPC | org_id="specific" | org_id="0" (cross-org) |
|-----|-------------------|------------------------|
| GetBalance | Per-org balance | Sum of all active org balances |
| GetTransactions | Filtered to org | **Not supported** (use per-org) |
| GetLedgerSummary | Per-org balance + counts | Aggregated balance + merged counts |

## Test Data Setup
```sql
-- Create test user with 2 orgs
INSERT INTO users (id, email, name) VALUES ('u1', 'test@example.com', 'Test User');
INSERT INTO organizations (id, name, metro) VALUES ('o1', 'Org One', 'NYC'), ('o2', 'Org Two', 'NYC');
INSERT INTO users_orgs (user_id, org_id, role, balance_cents, status) VALUES
  ('u1', 'o1', 'MEMBER',  5000, 'ACTIVE'),   -- $50.00 credit
  ('u1', 'o2', 'ADMIN',  -3000, 'ACTIVE');  -- $30.00 debit

-- Add transactions
INSERT INTO ledger_transactions (org_id, user_id, amount_cents, type, status) VALUES
  ('o1', 'u1',  10000, 'RENTAL_PAYMENT', 'POSTED'),
  ('o1', 'u1',  -5000, 'BILL_SETTLEMENT', 'POSTED'),
  ('o2', 'u1',  -3000, 'RENTAL_PAYMENT', 'POSTED');

-- Verify: GetBalance org_id=o1 → 5000, org_id=o2 → -3000, org_id=0 → 2000
```