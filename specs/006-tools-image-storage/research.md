# Research: Tools + Image Storage Multi-Org Gaps

**Date**: 2026-07-27 (original) — **Corrected 2026-07-28**
**Context**: Implement FR-008 (cross-org search) + FR-009 (shared-org owner filtering)

> **Status**: The `include_all_my_orgs` flag and its dependency on a Redis-cached "current org"
> (FR-010) described below were implemented once and reverted — tools are metro-scoped
> (`tools.metro`), never org-scoped, so there is no single-org "default" for a flag to expand from,
> and the server has no per-user "current org" to resolve a metro against in the first place (a
> user may have several devices, each focused on a different org — see
> `docs/design/multi-org.md`). FR-008 was implemented as a metro-scoped search plus the FR-009
> shared-org filter below, with no proto change.

## Decision: FR-008 Cross-Org Search = Metro Filter + Shared-Org Post-Filter (no `include_all_my_orgs`)

**Rationale**:
- No proto change needed: `SearchToolsRequest` already has `metro` (explicit) and `orgID`
  (resolves a metro) as request parameters
- Tools are metro-scoped, not org-scoped, so there is no "current org only" default behavior to
  gate behind a flag — search is always metro-wide, filtered down to shared-org owners (FR-009)
- No dependency on a "current org" of any kind, cached or otherwise

**Alternatives considered**:
1. New RPC `SearchToolsAcrossOrgs` — adds surface area
2. `org_ids` repeated field — client must know IDs; leaks internals
3. `include_all_my_orgs` boolean gating a Redis-cached "current org" default — implemented once,
   reverted; reintroduces exactly the server-side org-context state this design avoids

## Decision: FR-009 Owner Filtering = Intersection of (Requester's Orgs) ∩ (Owner's Orgs), Surfaced via the Existing `User.orgs` Field

**Rationale**:
- PRD 3.3: "owner's org info returned MUST be limited to only the organizations shared between the tool's owner and the requesting caller"
- Implementation: Join `users_orgs` twice — once for requester, once for owner
- Returns only orgs where BOTH have `ACTIVE` membership, **and** (2026-07-29) where the owner is
  not `lending_blocked` and the requester is not `renting_blocked` in that org — each flag
  checked against the role that party actually has here (owner lends, requester rents).
  `renting_blocked`/`lending_blocked` are independent per-role flags (e.g. set by
  `BillSplitService.SetRentingBlocked`/`SetLendingBlocked` for an unpaid bill): a user blocked
  from renting in an org must still be able to lend there, and vice versa — one flag must never
  disqualify the other role. See `TestToolService_GetSharedOrganizations_RespectsBlockedFlags`
  (`tests/unit/tool_service_test.go`) for the regression coverage.
- **Corrected 2026-07-29**: rather than adding a new `owner_organizations`/`OrganizationSummary`
  field to `Tool`, this reuses `Tool.owner.orgs` — `Tool.owner` is already typed as the shared
  `User` message, which already has `repeated Organization orgs = 6`
  (`ubertool_schema.proto`). `toolService.populateToolOwner` sets `owner.Orgs` to the shared-org
  result and the existing mapper (`internal/api/grpc/mapper.go`, `MapDomainUserToProto`) already
  serializes it — no proto change was needed at all. This is also why
  `CreateRentalRequestResponse.shared_organization_ids`/`shared_organization_names`
  (005-rentals) were removed: the client already has this exact information from `SearchTools`/
  `GetTool` by the time it calls `CreateRentalRequest`, so duplicating it there was redundant and
  in the wrong layer (a write-result response, not a discovery surface).

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
- `metro` is either passed explicitly by the caller, or resolved from an `organization_id` the
  caller is an active member of — there is no cached "current org" of any kind
- `tools.metro` is the tool's own column (tools are owned by users, not orgs)

## Constitution Check

| Principle | Check |
|-----------|-------|
| I. Code Is Truth | Verified against `internal/service/tool.go`, `internal/api/grpc/tool.go` |
| II. Domain Constants | `ToolStatus` constants in `internal/domain/tool.go` |
| III. Push Notifications | AddTool/UpdateTool/DeleteTool → notify owner + shared org members? (N/A for search) |
| IV. Layered Testing | New integration test for cross-org search |
| V. Deployment Parity | No schema change; uses existing `users_orgs` |
| VI. Proto-First | No proto change for FR-008 (reuses existing `metro`/`organization_id` on `SearchToolsRequest`) or FR-009 (reuses existing `User.orgs` via `Tool.owner`) |

## Open Questions Resolved

| Question | Resolution |
|----------|------------|
| Does FR-009 apply to GetTool? | Yes — same shared-org filtering for owner info |
| What if the caller doesn't pass a metro or org? | `SearchTools` requires an explicit `metro` (or an `organization_id` it resolves one from) — there is no cached "current org" to fall back to |
| What if no shared orgs with owner? | `GetTool` returns the tool with `owner.orgs` empty; `SearchTools` excludes the tool entirely (see FR-008 filter) |
| Performance for users with 50+ orgs? | Index on `users_orgs(user_id, status)` + `orgs(metro)` — query is indexed nested loop |