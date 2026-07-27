# Quickstart: Rentals

## Prerequisites
- Go 1.22+, PostgreSQL 16, Redis
- `podman-compose -f podman/trusted-group/compose.yaml up -d`
- `make proto`

## Run Tests

```bash
# L1 Unit
go test -v ./tests/unit/... -run Rental

# L2 Integration
go test -v ./tests/integration/... -run Rental

# L3 E2E
go run ./cmd/server &
go test -v ./tests/e2e/... -run Rental
kill %1

# Smoke
go test -v ./tests/smoke/... -run Rental
```

## Manual Validation: Multi-Org Context Switch (FR-008)

### Setup: User in 2 orgs, tool owned by org-2 member
```sql
INSERT INTO users (id, email, name) VALUES 
  ('u1', 'renter@example.com', 'Renter'),
  ('u2', 'owner@example.com', 'Owner');

INSERT INTO orgs (id, name, metro) VALUES 
  ('org-1', 'Org One', 'NYC'),
  ('org-2', 'Org Two', 'NYC');

INSERT INTO users_orgs (user_id, org_id, role, balance_cents, status) VALUES
  ('u1', 'org-1', 'ADMIN',  5000, 'ACTIVE'),
  ('u1', 'org-2', 'MEMBER', -3000, 'ACTIVE'),
  ('u2', 'org-2', 'ADMIN',  10000, 'ACTIVE');

INSERT INTO tools (id, owner_id, owner_org_id, name, status, metro, base_price_cents, duration_unit, duration_value)
VALUES ('tool-123', 'u2', 'org-2', 'Power Drill', 'AVAILABLE', 'NYC', 2000, 'DAY', 1);
```

### Scenario 1: Context Switch Required
```bash
# Current org = org-1, tool owned by org-2 member
grpcurl -plaintext -d '{
  "tool_id": "tool-123",
  "start_date": "2026-08-01",
  "end_date": "2026-08-05",
  "current_organization_id": "org-1"
}' localhost:50051 ubertool.trusted.backend.v1.RentalService/CreateRentalRequest

# Expected: 
# {
#   "context_switch_required": true,
#   "target_organization_id": "org-2",
#   "target_organization_name": "Org Two"
# }
```

### Scenario 2: Client Switches Context + Retries
```bash
# 1. Switch context
grpcurl -plaintext -d '{"organization_id": "org-2"}' \
  localhost:50051 ubertool.trusted.backend.v1.OrganizationService/SetCurrentOrganization

# 2. Retry rental request (with same idempotency_key)
grpcurl -plaintext -d '{
  "tool_id": "tool-123",
  "start_date": "2026-08-01",
  "end_date": "2026-08-05",
  "current_organization_id": "org-2",
  "idempotency_key": "same-key-as-before"
}' localhost:50051 ubertool.trusted.backend.v1.RentalService/CreateRentalRequest

# Expected: rental created in org-2 context
```

### Scenario 3: No Membership in Tool's Org → Reject
```sql
-- Remove u1's membership in org-2
DELETE FROM users_orgs WHERE user_id = 'u1' AND org_id = 'org-2';
```

```bash
grpcurl -plaintext -d '{
  "tool_id": "tool-123",
  "start_date": "2026-08-01",
  "end_date": "2026-08-05",
  "current_organization_id": "org-1"
}' localhost:50051 ubertool.trusted.backend.v1.RentalService/CreateRentalRequest

# Expected: PERMISSION_DENIED (not context switch)
```

## Existing Flow Validation (US1-US4)

```bash
# US1: Create → Approve → Finalize → Complete
grpcurl ... CreateRentalRequest
grpcurl ... ApproveRentalRequest {rental_id: "1"}
grpcurl ... FinalizeRentalRequest {rental_id: "1"}
grpcurl ... CompleteRental {rental_id: "1"}

# US2: ListMyRentals (renter) + ListMyLendings (owner)
grpcurl ... ListMyRentals
grpcurl ... ListMyLendings
```