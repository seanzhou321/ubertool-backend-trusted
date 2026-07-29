# Quickstart: Tools + Image Storage

## Prerequisites
- Go 1.22+, PostgreSQL 16
- `podman-compose -f podman/trusted-group/compose.yaml up -d`
- `make proto-gen`

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

> No `owner_org_id`, `include_all_my_orgs` flag, or Redis "current org" exist — see
> `docs/design/multi-org.md`. Search is always metro-scoped (explicit `metro`, or the metro of an
> org the caller names) plus a per-tool shared-org filter.

### 1. Metro + Shared-Org Search (FR-008)
```bash
# User in org-1 (NYC) and org-2 (NYC).
# Tools: tool-A owned by u1 (in org-1 + org-2), tool-B owned by another user in org-2 only.

grpcurl -plaintext -d '{"query": "drill", "metro": "NYC"}' \
  localhost:50051 ubertool.trusted.backend.v1.ToolService/SearchTools
# Returns tool-B (shares org-2 with the requester) but never tool-A (SearchTools excludes the
# caller's own tools). A tool whose owner shares zero active orgs with the requester is excluded
# even if it's in the same metro.
```

### 2. Shared-Org Owner Filtering (FR-009)
```bash
# Requester in org-1, org-2; Owner in org-2, org-3
# Shared orgs: org-2 only

grpcurl -plaintext -d '{"tool_id": "tool-owned-by-user-in-org-2-org-3"}' \
  localhost:50051 ubertool.trusted.backend.v1.ToolService/GetTool

# Response.tool.owner.orgs (the pre-existing User.orgs field): only org-2 (shared)
# NOT org-3 (not shared with requester) — no owner_organizations field exists
```

### 3. AddTool → Owner from JWT Only
```bash
grpcurl -plaintext -H "Authorization: Bearer <JWT>" \
  -d '{"name": "New Drill", "daily_price_cents": 500, "metro": "NYC"}' \
  localhost:50051 ubertool.trusted.backend.v1.ToolService/AddTool
# tool.owner_id = JWT user_id
# No org is recorded on the tool at all.
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

-- Tools, owned by users, metro-scoped only (no org column)
INSERT INTO tools (id, owner_id, name, status, metro, daily_price_cents, replacement_cost_cents, condition, duration_unit) VALUES
  ('tool-A', 'u1', 'Drill A', 'AVAILABLE', 'NYC', 500, 50000, 'GOOD', 'DAY'),
  ('tool-B', 'other-user', 'Drill B', 'AVAILABLE', 'NYC', 600, 60000, 'LIKE_NEW', 'DAY');
```