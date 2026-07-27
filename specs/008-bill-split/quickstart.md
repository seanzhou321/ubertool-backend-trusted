# Quickstart: Bill Split

## Prerequisites
- Go 1.22+, PostgreSQL 16
- `podman-compose -f podman/trusted-group/compose.yaml up -d`
- `make proto`

## Run Tests

```bash
# L1 Unit
go test -v ./tests/unit/... -run BillSplit

# L2 Integration
go test -v ./tests/integration/... -run BillSplit

# L3 E2E
go run ./cmd/server &
go test -v ./tests/e2e/... -run BillSplit
kill %1

# Smoke
go test -v ./tests/smoke/... -run BillSplit
```

## Manual Validation: Multi-Org Summaries (FR-014, FR-015)

### Setup: User in 2 orgs, bills in both
```sql
INSERT INTO users (id, email, name) VALUES ('u1', 'admin@test.com', 'Admin User');
INSERT INTO orgs (id, name, metro) VALUES 
  ('org-1', 'Org One', 'NYC'),
  ('org-2', 'Org Two', 'NYC');
INSERT INTO users_orgs (user_id, org_id, role, balance_cents, status) VALUES
  ('u1', 'org-1', 'ADMIN', 5000, 'ACTIVE'),
  ('u1', 'org-2', 'SUPER_ADMIN', -3000, 'ACTIVE');

-- Bills in org-1
INSERT INTO bills (org_id, debtor_id, creditor_id, amount_cents, status, settlement_month) VALUES
  ('org-1', 'u2', 'u1', 1000, 'PENDING', '2026-08-01'),
  ('org-1', 'u1', 'u3', 2000, 'ACKNOWLEDGED', '2026-08-01');

-- Bills in org-2
INSERT INTO bills (org_id, debtor_id, creditor_id, amount_cents, status, settlement_month) VALUES
  ('org-2', 'u4', 'u1', 500, 'DISPUTED', '2026-08-01'),
  ('org-2', 'u1', 'u5', 1500, 'RESOLVED', '2026-08-01');
```

### 1. Global Summary (FR-014) — All User's Orgs
```bash
grpcurl -plaintext -d '{}' \
  localhost:50051 ubertool.trusted.backend.v1.BillSplitService/GetGlobalBillSplitSummary

# Expected:
# {
#   "pending_count": 1,
#   "acknowledged_count": 1,
#   "disputed_count": 1,
#   "resolved_count": 1,
#   "total_amount_cents": 4500  -- 1000+2000+500+1500 (excluding RESOLVED)
# }
```

### 2. Per-Org Summary (FR-015) — Admin Only
```bash
# As admin of org-1
grpcurl -plaintext -d '{"organization_id": "org-1"}' \
  localhost:50051 ubertool.trusted.backend.v1.BillSplitService/GetOrganizationBillSplitSummary

# Expected:
# {
#   "pending_count": 1,
#   "acknowledged_count": 1,
#   "disputed_count": 0,
#   "resolved_count": 0,
#   "total_amount_cents": 3000
# }

# Non-admin calling org-2 → PERMISSION_DENIED
grpcurl -plaintext -d '{"organization_id": "org-2"}' \
  localhost:50051 ubertool.trusted.backend.v1.BillSplitService/GetOrganizationBillSplitSummary
```

## Existing Flow Validation

### Monthly Settlement Pipeline
```bash
# Trigger manually (normally cron)
grpcurl -plaintext -d '{}' \
  localhost:50051 ubertool.trusted.backend.v1.BillSplitService/TriggerMonthlySettlement
```

### Bill Lifecycle
```bash
# Acknowledge (debtor or creditor)
grpcurl -plaintext -d '{"bill_id": "1"}' \
  localhost:50051 ubertool.trusted.backend.v1.BillSplitService/AcknowledgePayment

# Dispute (debtor or creditor)
grpcurl -plaintext -d '{"bill_id": "2", "reason": "Amount incorrect"}' \
  localhost:50051 ubertool.trusted.backend.v1.BillSplitService/ResolveDispute

# Admin resolve
grpcurl -plaintext -d '{"bill_id": "2", "resolution": "CREDITOR_WINS"}' \
  localhost:50051 ubertool.trusted.backend.v1.BillSplitService/ResolveDispute
```