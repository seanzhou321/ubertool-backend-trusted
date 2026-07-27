# Quickstart: Tools + Image Storage

## Prerequisites
- Go 1.22+, PostgreSQL 16, Redis
- `podman-compose -f podman/trusted-group/compose.yaml up -d`
- `make proto`

## Run Tests

```bash
# L1 Unit
go test -v ./tests/unit/... -run Tool

# L2 Integration
go test -v ./tests/integration/... -run Tool

# L3 E2E
go run ./cmd/server &
go test -v ./tests/e2e/... -run Tool
kill %1

# Smoke
go test -v ./tests/smoke/... -run Tool
```

## Manual Validation

### 1. Cross-Org Search (FR-008)
```bash
# User in org-1 (NYC) and org-2 (NYC), current org = org-1
# Tools: tool-A in org-1, tool-B in org-2 (both NYC)

# Default: current org only
grpcurl -plaintext -d '{"query": "drill", "include_all_my_orgs": false}' \
  localhost:50051 ubertool.trusted.backend.v1.ToolService/SearchTools
# Returns: tool-A only

# Cross-org
grpcurl -plaintext -d '{"query": "drill", "include_all_my_orgs": true}' \
  localhost:50051 ubertool.trusted.backend.v1.ToolService/SearchTools
# Returns: tool-A + tool-B (both in NYC)
```

### 2. Shared-Org Owner Filtering (FR-009)
```bash
# Requester in org-1, org-2; Owner in org-2, org-3
# Shared orgs: org-2 only

grpcurl -plaintext -d '{"tool_id": "tool-owned-by-user-in-org-2-org-3"}' \
  localhost:50051 ubertool.trusted.backend.v1.ToolService/GetTool

# Response.owner_organizations: only org-2 (shared)
# NOT org-3 (not shared with requester)
```

### 3. AddTool → Owner Org from JWT
```bash
grpcurl -plaintext -H "Authorization: Bearer <JWT>" \
  -d '{"name": "New Drill", "daily_price_cents": 500, "metro": "NYC"}' \
  localhost:50051 ubertool.trusted.backend.v1.ToolService/AddTool
# tool.owner_id = JWT user_id
# tool.owner_org_id = current_org from Redis (FR-010)
```

## Test Data
```sql
-- User with 2 orgs in same metro
INSERT INTO users (id, email, name) VALUES ('u1', 'u1@test.com', 'User One');
INSERT INTO orgs (id, name, metro) VALUES 
  ('org-1', 'Org One', 'NYC'),
  ('org-2', 'Org Two', 'NYC');
INSERT INTO users_orgs (user_id, org_id, role, balance_cents, status) VALUES
  ('u1', 'org-1', 'ADMIN', 0, 'ACTIVE'),
  ('u1', 'org-2', 'MEMBER', 0, 'ACTIVE');

-- Tools in different orgs
INSERT INTO tools (id, owner_id, owner_org_id, name, status, metro, daily_price_cents, replacement_cost_cents, condition, duration_unit) VALUES
  ('tool-A', 'u1', 'org-1', 'Drill A', 'AVAILABLE', 'NYC', 500, 50000, 'GOOD', 'DAY'),
  ('tool-B', 'other-user', 'org-2', 'Drill B', 'AVAILABLE', 'NYC', 600, 60000, 'LIKE_NEW', 'DAY');

-- Redis current org
SET user:u1:current_org "org-1" EX 86400;
```