# Research: Rentals Multi-Org Context Switch (FR-008)

**Date**: 2026-07-27  
**Context**: Implement FR-008 from `specs/005-rentals/spec.md` — org context switch when renting from different org

## Decision: Context Switch Handled at Rental Creation (FR-008)

**Rationale**:
- PRD 3.1: "When user attempts to rent from different org they belong to, prompt to switch"
- Current org stored in Redis (per Organizations FR-010)
- Tool's owner org = `tools.owner_org_id`
- If `current_org != tool.owner_org_id` AND user has membership in `tool.owner_org_id` → context switch required

**Flow**:
1. Client calls `CreateRentalRequest` with `current_organization_id`
2. Server validates: user has membership in current_org
3. Server checks tool's owner_org_id
4. If different AND user has ACTIVE membership in owner_org:
   - Return `CONTEXT_SWITCH_REQUIRED` with target org info
5. Client prompts user → calls `SetCurrentOrganization(target_org)` → retries
6. On retry: rental created in target org context

**Alternatives considered**:
1. **Auto-switch** — Violates least surprise; user may not want to switch
2. **Reject with error** — Forces manual switch first; extra round-trip
3. **Allow cross-org rental without switch** — Breaks org-scoped ownership model

## Decision: `current_organization_id` in CreateRentalRequest (Proto Change)

**Rationale**:
- Explicit client-side context — no hidden server state
- Enables server validation of membership in stated org
- Required for FR-012 in Tools (same pattern)

**Proto change**:
```protobuf
message CreateRentalRequest {
  // ... existing fields ...
  string current_organization_id = 20;  // NEW
}

message CreateRentalResponse {
  // ... existing fields ...
  bool context_switch_required = 10;     // NEW
  string target_organization_id = 11;    // NEW
  string target_organization_name = 12;  // NEW
}
```

## Decision: Rental Stays in Owner's Org (Not Requester's)

**Rationale**:
- Tool belongs to owner's org; rental lifecycle tied to that org
- Bill Split, Ledger, Notifications all scoped to rental's org
- Requester's membership in owner's org validated at creation

## Constitution Check

| Principle | Check |
|-----------|-------|
| I. Code Is Truth | Verified against `internal/service/rental.go` CreateRental flow |
| II. Domain Constants | `RentalStatus` constants used; new status not needed |
| III. Push Notifications | Context switch doesn't trigger notifications (rental not created yet) |
| IV. Layered Testing | New integration test for context-switch flow |
| V. Deployment Parity | Redis for current org available on both targets |
| VI. Proto-First | Proto change required → regenerate → implement |

## Open Questions Resolved

| Question | Resolution |
|----------|------------|
| What if user has NO membership in tool's org? | Reject with `PERMISSION_DENIED` (not context switch) |
| What if user has PENDING membership? | Reject — only ACTIVE memberships count |
| Does switch affect ListMyRentals? | Yes — `ListMyRentals` filters by current org (FR-010) |
| Retry after switch — idempotency? | Client retries handled by existing `idempotency_key` |