# Research: Organizations & Administration Multi-Org Gaps

**Date**: 2026-07-27  
**Context**: Implement FR-009, FR-010, FR-011, FR-012 from `specs/003-organizations-administration/spec.md`

## Decision: Current Org Context Stored in Redis (not JWT)

**Rationale**:
- JWT is stateless; org context changes frequently (user switches orgs)
- Redis allows TTL-based session expiry, easy invalidation on logout
- No JWT re-issuance needed on org switch
- Query pattern: `GET user:{id}:current_org` — single round-trip

**Alternatives considered**:
1. **Add `current_org_id` to JWT claims** — Requires token refresh on every switch; JWT size grows
2. **Header `X-Current-Org` on every request** — Client must track; no server-side validation of membership
3. **Store in `users` table** — Wrong granularity (user has multiple orgs)

## Decision: `include_all_my_orgs` Boolean Flag on SearchTools (FR-011)

**Rationale**:
- Minimal proto change: adds one optional boolean field
- Default `false` = existing behavior (search current org only)
- `true` = search all user's active orgs in resolved metro
- No new RPC needed; backward compatible

**Alternatives considered**:
1. **New `SearchToolsAcrossOrgs` RPC** — Cleaner separation but adds surface area
2. **`org_ids` repeated field** — More flexible but client must know org IDs; leaks internals
3. **Implicit based on current org** — Magic behavior; hard to debug/test

## Decision: Context Switch Response in CreateRental (FR-012)

**Rationale**:
- Return `context_switch_required=true` + target org info
- Client prompts user: "This tool belongs to [Org B]. Switch context to [Org B] to proceed?"
- User confirms → client calls `SetCurrentOrganization` → retries `CreateRentalRequest`
- Server validates membership in target org before creating rental

**Alternatives considered**:
1. **Auto-switch silently** — Violates Principle of Least Surprise; user may not want to switch
2. **Reject with error** — Forces client to implement switch logic; poor UX
3. **Allow cross-org rental without switch** — Breaks org-scoped ownership model

## Decision: ListMyOrganizations Returns Balance + Role (FR-009)

**Rationale**:
- Single call gives client everything needed for org picker UI
- Balance from `users_orgs.balance_cents` (FR-009)
- Role from `users_orgs.role`
- Member count from `COUNT(*) OVER (PARTITION BY org_id)` in same query

## Constitution Check

| Principle | Check |
|-----------|-------|
| I. Code Is Truth | Verified against `internal/service/org.go`, `admin.go`, `auth.go` (join request) |
| II. Domain Constants | `OrgRole`, `OrgMembershipStatus` constants in `internal/domain/org.go` |
| III. Push Notifications | Org creation/join triggers notifications (handled by Auth service) |
| IV. Layered Testing | Tests map to `tests/unit`, `tests/integration`, `tests/e2e`, `tests/smoke` |
| V. Deployment Parity | Redis for current org (available on both Podman + EC2) |
| VI. Proto-First | New fields added to proto → regenerate → implement |

## Open Questions Resolved

| Question | Resolution |
|----------|------------|
| Where does metro resolve for cross-org search? | From user's **current org** (FR-010) — if no current org, use first active org |
| What if user has orgs in different metros? | Search limited to orgs in **same metro as current org** (PRD 3.3) |
| Does `SetCurrentOrganization` validate membership? | Yes — rejects if user not `ACTIVE` member of target org |
| TTL for current org in Redis? | 24h (aligns with JWT refresh token lifetime) |
| Cleanup on logout? | `Logout` handler deletes `user:{id}:current_org` key |