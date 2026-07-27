# Research: Tools + Image Storage Multi-Org Gaps

**Date**: 2026-07-27  
**Context**: Implement FR-008 (cross-org search) + FR-009 (shared-org owner filtering)

## Decision: FR-008 Cross-Org Search = `include_all_my_orgs` Flag on SearchTools

**Rationale**:
- Minimal proto change: one boolean field
- Default `false` = existing behavior (current org only)
- `true` = search all user's active orgs in same metro
- Metro resolution: from user's **current org** (FR-010) or first active org

**Alternatives considered**:
1. New RPC `SearchToolsAcrossOrgs` — adds surface area
2. `org_ids` repeated field — client must know IDs; leaks internals
3. Implicit based on auth — no explicit control; hard to test

## Decision: FR-009 Owner Filtering = Intersection of (Requester's Orgs) ∩ (Owner's Orgs)

**Rationale**:
- PRD 3.3: "owner's org info returned MUST be limited to only the organizations shared between the tool's owner and the requesting caller"
- Implementation: Join `users_orgs` twice — once for requester, once for owner
- Returns only orgs where BOTH have `ACTIVE` membership

**Query pattern**:
```sql
SELECT o.id, o.name, o.metro
FROM orgs o
JOIN users_orgs uo_requester ON uo_requester.org_id = o.id 
    AND uo_requester.user_id = $requester_id AND uo_requester.status = 'ACTIVE'
JOIN users_orgs uo_owner ON uo_owner.org_id = o.id 
    AND uo_owner.user_id = $owner_id AND uo_owner.status = 'ACTIVE'
```

## Decision: SearchTools Metro Resolution

**Rationale**:
- If `include_all_my_orgs = true`: metro = requester's **current org's metro** (or first active org)
- If `include_all_my_orgs = false`: metro = current org's metro (existing behavior)
- Tools have `metro` column (via owner's org or tool's own metro)

## Constitution Check

| Principle | Check |
|-----------|-------|
| I. Code Is Truth | Verified against `internal/service/tool.go`, `internal/api/grpc/tool.go` |
| II. Domain Constants | `ToolStatus` constants in `internal/domain/tool.go` |
| III. Push Notifications | AddTool/UpdateTool/DeleteTool → notify owner + shared org members? (N/A for search) |
| IV. Layered Testing | New integration test for cross-org search |
| V. Deployment Parity | No schema change; uses existing `users_orgs` |
| VI. Proto-First | `include_all_my_orgs` field added to `SearchToolsRequest` |

## Open Questions Resolved

| Question | Resolution |
|----------|------------|
| Does FR-009 apply to GetTool? | Yes — same shared-org filtering for owner info |
| What if user has no current org? | Use first active org from `users_orgs` (fallback) |
| What if no shared orgs with owner? | Return tool with empty `owner_organizations` list |
| Performance for users with 50+ orgs? | Index on `users_orgs(user_id, status)` + `orgs(metro)` — query is indexed nested loop |