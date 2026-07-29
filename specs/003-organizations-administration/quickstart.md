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

### 5. **REMOVED**: FR-010 "Current Org Context" (`SetCurrentOrganization`/`GetCurrentOrganization`)
There is no server-side "current organization" — see `docs/design/multi-org.md`. A user's active
org is a client/device-local UI preference (a user may have several devices, each focused on a
different org), so the server never caches or persists it. Every request that needs an org
context takes `organization_id` explicitly.

### 6. **Multi-Org FR-011**: Cross-Org Tool Search (metro + shared-org filter, no `include_all_my_orgs`)
```bash
grpcurl -plaintext -d '{"query": "drill", "metro": "NYC"}' \
  localhost:50051 ubertool.trusted.backend.v1.ToolService/SearchTools
# Expected: tools owned by users who share an active org with the caller in the NYC metro
# (e.g. an Org B tool, if the caller is also in Org B) — never the caller's own tools.
# Each returned tool's owner.orgs lists exactly the orgs shared with the caller, e.g.:
#   tools[0].owner.orgs = [{ id: "<org-b-id>", name: "Org Two", ... }]
# This is how the client learns which organization_id to use for CreateRentalRequest (§7).
```

### 7. **Multi-Org FR-012 / Rentals FR-008**: Shared-Org Rental Validation

Org discovery happens at search time, not here: `SearchTools`/`GetTool` on this tool already
returned `owner.orgs = [{id: "<org-b-id>", ...}]` (§6 above shows the shared-org search that
surfaces this), so the client already knows `<org-b-id>` is the only valid `organization_id`
before ever calling `CreateRentalRequest`.

```bash
# Correct call, using the org_id learned from SearchTools/GetTool's owner.orgs:
grpcurl -plaintext -d '{
  "tool_id": "<tool-in-org-b>",
  "start_date": "2026-08-01",
  "end_date": "2026-08-05",
  "organization_id": "<org-b-id>"
}' \
  localhost:50051 ubertool.trusted.backend.v1.RentalService/CreateRentalRequest
# Expected: success.

# Defensive backstop — if the client (incorrectly, or due to stale data) sends a non-shared org:
grpcurl -plaintext -d '{
  "tool_id": "<tool-in-org-b>",
  "start_date": "2026-08-01",
  "end_date": "2026-08-05",
  "organization_id": "<org-a-id>"
}' \
  localhost:50051 ubertool.trusted.backend.v1.RentalService/CreateRentalRequest
# Expected: FAILED_PRECONDITION, human-readable message only (no structured shared-org list in
# the response — the client should re-fetch owner.orgs via GetTool, not rely on this error).
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