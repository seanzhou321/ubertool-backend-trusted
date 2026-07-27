# Quickstart: Organizations + Admin

## Prerequisites
- Go 1.22+
- PostgreSQL 16 (Podman: `podman-compose -f podman/trusted-group/compose.yaml up -d`)
- Protobuf: `make proto` (requires `protoc`, `protoc-gen-go`, `protoc-gen-go-grpc`)

## Run Tests

### L1 Unit (pure logic)
```bash
go test -v ./tests/unit/... -run Org
go test -v ./tests/unit/... -run Admin
```

### L2 Integration (real DB)
```bash
go test -v ./tests/integration/... -run Org
go test -v ./tests/integration/... -run Admin
```

### L3 E2E (full gRPC)
```bash
# Terminal 1: start server
go run ./cmd/server

# Terminal 2: run e2e tests
go test -v ./tests/e2e/... -run Org
go test -v ./tests/e2e/... -run Admin
```

### Smoke (deploy sanity)
```bash
go test -v ./tests/smoke/... -run Org
go test -v ./tests/smoke/... -run Admin
```

## Manual Validation Scenarios

### 1. Create Organization (FR-001)
```bash
grpcurl -plaintext -d '{"name": "Test Org", "metro": "NYC"}' \
  localhost:50051 ubertool.trusted.backend.v1.OrganizationService/CreateOrganization
# Expected: { organization: { id, name, metro, ... }, membership: { role: "SUPER_ADMIN" } }
```

### 2. List My Organizations (FR-007)
```bash
grpcurl -plaintext -d '{}' \
  localhost:50051 ubertool.trusted.backend.v1.OrganizationService/ListMyOrganizations
# Expected: { organizations: [{ id, name, member_count, my_role, my_balance_cents }, ...] }
```

### 3. Join Organization with Invite (FR-004)
```bash
# First create invite as admin
grpcurl -plaintext -d '{"organization_id": "<org-id>", "email": "user@test.com", "role": "MEMBER"}' \
  localhost:50051 ubertool.trusted.backend.v1.AdminService/SendInvitation

# Then join as invited user
grpcurl -plaintext -d '{"token": "<token-from-invite>", "organization_id": "<org-id>"}' \
  localhost:50051 ubertool.trusted.backend.v1.OrganizationService/JoinOrganizationWithInvite
```

### 4. **Multi-Org FR-009**: Verify Per-Membership Balance
```bash
# User belongs to Org A ($50) and Org B (-$30)
grpcurl -plaintext -d '{}' \
  localhost:50051 ubertool.trusted.backend.v1.OrganizationService/ListMyOrganizations
# Expected: Org A shows my_balance_cents: 5000, Org B shows -3000
```

### 5. **Multi-Org FR-010**: Set/Get Current Org Context
```bash
# Set current org
grpcurl -plaintext -d '{"organization_id": "<org-a-id>"}' \
  localhost:50051 ubertool.trusted.backend.v1.OrganizationService/SetCurrentOrganization

# Get current org
grpcurl -plaintext -d '{}' \
  localhost:50051 ubertool.trusted.backend.v1.OrganizationService/GetCurrentOrganization
# Expected: { organization_id: "<org-a-id>" }
```

### 6. **Multi-Org FR-011**: Cross-Org Tool Search
```bash
# Search only current org (default)
grpcurl -plaintext -d '{"query": "drill", "include_all_my_orgs": false}' \
  localhost:50051 ubertool.trusted.backend.v1.ToolService/SearchTools

# Search ALL user's orgs in same metro
grpcurl -plaintext -d '{"query": "drill", "include_all_my_orgs": true}' \
  localhost:50051 ubertool.trusted.backend.v1.ToolService/SearchTools
# Expected: tools from Org A AND Org B (if both in NYC metro)
```

### 7. **Multi-Org FR-012**: Rental Context Switch Prompt
```bash
# User in Org A tries to rent tool owned by Org B member (user also in Org B)
grpcurl -plaintext -d '{
  "tool_id": "<tool-in-org-b>",
  "start_date": "2026-08-01",
  "end_date": "2026-08-05",
  "current_organization_id": "<org-a-id>"
}' \
  localhost:50051 ubertool.trusted.backend.v1.RentalService/CreateRentalRequest
# Expected: { context_switch_required: true, target_organization_id: "<org-b-id>", target_organization_name: "Org B" }
# Client shows prompt, user confirms, client calls SetCurrentOrganization(org-b-id), then retries rental
```

### 8. Admin: Approve Join Request
```bash
grpcurl -plaintext -d '{"request_id": 123}' \
  localhost:50051 ubertool.trusted.backend.v1.AdminService/ApproveRequestToJoin
```

### 9. Admin: List All Join Requests (FR-008 - Known Discrepancy 3)
```bash
grpcurl -plaintext -d '{"organization_id": "<org-id>"}' \
  localhost:50051 ubertool.trusted.backend.v1.AdminService/ListJoinRequests
# Expected: returns ALL statuses (PENDING, APPROVED, REJECTED, EXPIRED) not just PENDING
```

## Test Data Setup
```sql
-- User with 2 org memberships
INSERT INTO users (id, email, name) VALUES ('u1', 'multi@test.com', 'Multi Org User');
INSERT INTO orgs (id, name, metro) VALUES 
  ('org-a', 'Org A', 'NYC'),
  ('org-b', 'Org B', 'NYC');
INSERT INTO users_orgs (user_id, org_id, role, balance_cents, status) VALUES
  ('u1', 'org-a', 'SUPER_ADMIN',  5000, 'ACTIVE'),  -- $50 credit
  ('u1', 'org-b', 'MEMBER',     -3000, 'ACTIVE');  -- $30 debit

-- Tools in both orgs
INSERT INTO tools (id, owner_id, org_id, name, metro, status) VALUES
  ('t1', 'u2', 'org-a', 'Drill', 'NYC', 'AVAILABLE'),
  ('t2', 'u3', 'org-b', 'Saw', 'NYC', 'AVAILABLE');

-- Join request for admin testing
INSERT INTO join_requests (id, user_id, org_id, status) VALUES (1, 'u4', 'org-a', 'PENDING');
```