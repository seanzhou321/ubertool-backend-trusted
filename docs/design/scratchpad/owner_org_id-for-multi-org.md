# `owner_org_id` Implementation Details (Scratchpad)

> **Status**: Captured for reference before potential revert. This documents the current state of the `owner_org_id` field added to support FR-012 (hard org-context enforcement at rental creation).

---

## Database Schema Changes

### `podman/trusted-group/postgres/ubertool_schema_trusted.sql`

```sql
CREATE TABLE tools (
    id SERIAL PRIMARY KEY,
    owner_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    owner_org_id INTEGER REFERENCES orgs(id) ON DELETE SET NULL,  -- ADDED
    name TEXT NOT NULL,
    ...
);
```

- **Type**: `INTEGER` (FK to `orgs.id`)
- **Nullability**: `NULL` allowed (`ON DELETE SET NULL`)
- **Default**: Not set (must be provided at insert)
- **Index**: None explicitly created (FK creates implicit index)

---

## Domain Model Changes

### `internal/domain/tool.go`

```go
type Tool struct {
    ID                   int32            `json:"id"`
    OwnerID              int32            `json:"owner_id"`
    OwnerOrgID           int32            `json:"owner_org_id"`  // ADDED
    Owner                *User            `json:"owner,omitempty"`
    ...
}
```

- Field: `OwnerOrgID int32`
- JSON tag: `owner_org_id`
- Zero value (`0`) = not set (maps to `NULL` in DB)

---

## Repository Changes

### `internal/repository/postgres/tool.go`

| Method | Change |
|--------|--------|
| `Create` | INSERT includes `owner_org_id` as `$2` (after `owner_id`) |
| `GetByID` | SELECT includes `owner_org_id`; scans into `t.OwnerOrgID` |
| `ListByOrg` | SELECT includes `owner_org_id` |
| `ListByOwner` | SELECT includes `owner_org_id` |
| `Search` | SELECT includes `owner_org_id` |

**Note**: The `Create` method signature unchanged; caller must populate `tool.OwnerOrgID` before calling.

```go
// Create - updated query
query := `INSERT INTO tools (owner_id, owner_org_id, name, description, categories, 
          price_per_day_cents, price_per_week_cents, price_per_month_cents, 
          replacement_cost_cents, duration_unit, condition, metro, status, created_on) 
          VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14) RETURNING id`
// ...
return r.db.QueryRowContext(ctx, query, t.OwnerID, t.OwnerOrgID, t.Name, ...).Scan(&t.ID)
```

---

## Service Layer (Incomplete — Not Yet Implemented)

### `internal/service/tool.go` — **No Changes**
- `AddTool` does **not** set `tool.OwnerOrgID`
- `GetTool`, `ListTools`, `SearchTools` do **not** filter or validate by `OwnerOrgID`
- `populateToolOwner` does **not** include `OwnerOrgID` in shared orgs logic

### `internal/service/rental.go` — **No Enforcement Yet**
- `CreateRentalRequest` accepts `orgID` parameter but does **not** compare against `tool.OwnerOrgID`
- No `context_switch_required` error returned
- Auth interceptor injects `current-org-id` into metadata, but rental service doesn't read it

---

## Auth Interceptor Context Injection

### `internal/api/grpc/interceptor/auth_interceptor.go`

```go
// Get current org from Redis if available
if i.orgCtxRepo != nil {
    orgID, err := i.orgCtxRepo.GetCurrentOrg(ctx, claims.UserID)
    if err != nil {
        logger.Warn("Failed to get current org from Redis", "userID", claims.UserID, "error", err)
    } else if orgID != 0 {
        md.Set("current-org-id", strconv.Itoa(int(orgID)))
        logger.Debug("Current org ID injected into context", "method", info.FullMethod, "orgID", orgID)
    }
}
```

- Injects `current-org-id` metadata header from Redis (`user:{user_id}:current_org`)
- Set via `OrganizationService.SetCurrentOrganization` (validates membership)

---

## Organization Service — Context Switching

### `internal/service/org.go`

```go
func (s *organizationService) SetCurrentOrganization(ctx context.Context, userID int32, orgID int32) error {
    // Verify user is a member of the organization
    userOrg, err := s.userRepo.GetUserOrg(ctx, userID, orgID)
    if err != nil || userOrg == nil {
        return fmt.Errorf("user is not a member of this organization")
    }
    if userOrg.Status == domain.UserOrgStatusBlock {
        return fmt.Errorf("user is blocked in this organization")
    }
    // Store in Redis
    if s.orgCtxRepo != nil {
        if err := s.orgCtxRepo.SetCurrentOrg(ctx, userID, orgID); err != nil {
            return fmt.Errorf("failed to set current organization: %w", err)
        }
    }
    // Send push notification for context switch
    ...
}
```

- Validates membership before setting
- Stores in Redis with 24hr TTL
- Sends push notification (Constitution III)

---

## gRPC Handler — Rental

### `internal/api/grpc/rental.go`

```go
func (h *RentalHandler) CreateRentalRequest(ctx context.Context, req *pb.CreateRentalRequestRequest) (*pb.CreateRentalRequestResponse, error) {
    userID, err := GetUserIDFromContext(ctx)
    if err != nil {
        return nil, err
    }
    rt, err := h.rentalSvc.CreateRentalRequest(ctx, userID, req.ToolId, req.OrganizationId, req.StartDate, req.EndDate)
    ...
}
```

- Passes `req.OrganizationId` directly to service
- **Does not** read `current-org-id` from metadata
- **Does not** validate against tool's `OwnerOrgID`

---

## Proto Definitions (Unchanged)

### `api/proto/ubertool_trusted_backend/v1/rental_service.proto`

```protobuf
message CreateRentalRequestRequest {
  int32 tool_id = 1;
  string start_date = 2;
  string end_date = 3;
  int32 organization_id = 4;  // Still explicit in request
}
```

- No `context_switch_required` error code defined
- No `owner_org_id` field in `Tool` or `RentalRequest` messages

### `api/proto/ubertool_trusted_backend/v1/tool_service.proto`

```protobuf
message Tool {
  int32 id = 1;
  int32 owner_id = 2;
  // owner_org_id NOT present
  ...
}
```

---

## Intended Flow (Not Implemented)

```
1. Tool Created
   → AddTool(ctx, tool) where tool.OwnerOrgID = caller's current org (from Redis)
   
2. User Calls CreateRentalRequest(tool_id, organization_id)
   → Auth interceptor injects current_org_id from Redis into metadata
   
3. Rental Service Compares
   → tool.OwnerOrgID vs current_org_id (from metadata, NOT request.organization_id)
   
4. Mismatch → Return context_switch_required error (gRPC status code)
   
5. Client Calls SetCurrentOrganization(org_id) → Retries → Succeeds
```

---

## Current Gaps / Missing Pieces

| Component | Missing |
|-----------|---------|
| `toolService.AddTool` | Set `tool.OwnerOrgID` from caller's current org |
| `toolService.SearchTools` | Filter by `owner_org_id` or include in results |
| `rentalService.CreateRentalRequest` | Read `current-org-id` from metadata; compare to `tool.OwnerOrgID`; return `context_switch_required` |
| `rentalService.CreateRentalRequest` | Ignore `req.OrganizationId` (or validate it matches `current_org_id`) |
| Proto | Add `owner_org_id` to `Tool` message; add `context_switch_required` error |
| Rental Handler | Extract `current-org-id` from metadata; pass to service |
| Tests | Coverage for org mismatch flow |

---

## Migration Considerations (If Reverting)

### Database
```sql
-- Remove column
ALTER TABLE tools DROP COLUMN owner_org_id;
```

### Code Files to Revert
1. `podman/trusted-group/postgres/ubertool_schema_trusted.sql` — remove column
2. `internal/domain/tool.go` — remove `OwnerOrgID` field
3. `internal/repository/postgres/tool.go` — remove from all SELECT/INSERT/SCAN
4. Any migration scripts referencing `owner_org_id`

### Data Impact
- Column is `NULL` for all existing tools (no default was set)
- No data loss on drop
- If any tools were created with `OwnerOrgID` set (unlikely, service doesn't populate), those values would be lost

---

## What the `owner_org_id` Change Breaks (Transferred from multi-org.md)

| Broken Use Case | Original Behavior | With `owner_org_id` |
|-----------------|-------------------|---------------------|
| User lists tool once, rents in any shared org | Tool visible in all shared orgs; renter picks org at rental time | Tool locked to single `owner_org_id`; rental only allowed in that org |
| Owner in Org A & B, renter in Org B only | Renter sees tool, creates rental in Org B | Tool must be created in Org B; if created in Org A, renter blocked |
| Tool moves with user across orgs | Tool follows user (ownership = user) | Tool stuck in creation org; requires delete + recreate |
| Admin of Org A wants to see all tools their members own | `SearchTools` + `getSharedOrganizations` shows cross-org tools | Only tools with `owner_org_id = Org_A` visible |

---

## Related Discussions

- Original design: `docs/design/multi-org.md`
- FR-012 / 005-rentals: Hard org-context enforcement
- Session notes: `owner_org_id` breaks multi-org tool sharing use cases

---

## Decision Log (from multi-org.md)

| Date | Decision | Rationale |
|------|----------|-----------|
| Original | Tool owned by user, metro-scoped, rental org chosen at request | Supports multi-org membership naturally; no tool duplication |
| 2025-04-17 | Added `owner_org_id` to tools (FR-012) | Attempt to enforce org context at rental creation via hard error |
| **This doc** | Revert `owner_org_id`; enforce context via soft prompt + validation | Preserve original multi-org flexibility; fix rental org validation differently |

---

## Decision

> **Resolved 2026-07-28**: Reverted. `tools` has no `owner_org_id` column, and no Redis-backed
> "current organization" server-side cache exists. Rental org-context validation was implemented
> per the original design: `rentalService.CreateRentalRequest` validates the caller's requested
> `organization_id` against `isSharedOrganization`/`getSharedOrganizations` (both renter and tool
> owner must be active members of that org), and `rentals_shared_org_check` enforces the same
> invariant as a DB CHECK constraint on the `rentals` table
> (`podman/trusted-group/postgres/ubertool_schema_trusted.sql`). The `current-org-id` gRPC
> metadata header, `OrgContextRepository`, `SetCurrentOrganization`/`GetCurrentOrganization` RPCs,
> and the Redis dependency itself were all removed — "current organization" is a client/device-local
> concept only and is never cached or persisted server-side. See `docs/design/multi-org.md` for the
> current ground truth.