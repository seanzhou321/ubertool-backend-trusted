# Quickstart: Rentals

## Prerequisites
- Go 1.22+, PostgreSQL 16
- `podman-compose -f podman/trusted-group/compose.yaml up -d`
- `make proto-gen`

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

## Manual Validation: Multi-Org Rental Context (FR-008)

> There is no `owner_org_id` on tools and no server-side "current organization" (no
> `SetCurrentOrganization`/`GetCurrentOrganization`, no Redis). The client always passes
> `organization_id` explicitly in `CreateRentalRequest`; the server validates it against the
> renter's and owner's shared active orgs. See `docs/design/multi-org.md`.

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

INSERT INTO tools (id, owner_id, name, status, metro, base_price_cents, duration_unit, duration_value)
VALUES ('tool-123', 'u2', 'Power Drill', 'AVAILABLE', 'NYC', 2000, 'DAY', 1);
```

### Scenario 1: Rejects a non-shared organization_id
```bash
# Renter passes org-1, but the tool owner (u2) is not a member of org-1
grpcurl -plaintext -d '{
  "tool_id": "tool-123",
  "start_date": "2026-08-01",
  "end_date": "2026-08-05",
  "organization_id": "org-1"
}' localhost:50051 ubertool.trusted.backend.v1.RentalService/CreateRentalRequest

# Expected: FAILED_PRECONDITION — "organization org-1 is not shared with the tool owner.
# Shared organizations: [Org Two]"
```

### Scenario 2: Succeeds with a shared organization_id
```bash
grpcurl -plaintext -d '{
  "tool_id": "tool-123",
  "start_date": "2026-08-01",
  "end_date": "2026-08-05",
  "organization_id": "org-2"
}' localhost:50051 ubertool.trusted.backend.v1.RentalService/CreateRentalRequest

# Expected: rental created with org_id=org-2
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
  "organization_id": "org-2"
}' localhost:50051 ubertool.trusted.backend.v1.RentalService/CreateRentalRequest

# Expected: rejected — renter (u1) is no longer a member of org-2 at all
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