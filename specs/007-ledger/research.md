# Research: Ledger Multi-Org Rollup (FR-004)

**Date**: 2026-07-27  
**Context**: Implement FR-004 from `specs/007-ledger/spec.md` — cross-organization ledger rollup when `org_id=0`

## Decision: Use `org_id=0` as sentinel for "all organizations"

**Rationale**: 
- The gRPC proto already uses `int32 organization_id` and the as-built behavior returns an error for `org_id=0`
- Using `0` as a sentinel is a common pattern (e.g., `uid=0` = root, `pid=0` = all processes)
- Requires no proto schema change (backward compatible)
- Minimal client-side change: pass `0` instead of a specific org ID

**Alternatives considered**:
1. **New RPC `GetGlobalLedgerSummary`** — Clean separation but adds surface area; requires proto change + regeneration
2. **Optional `org_id` with `google.protobuf.Int32Value`** — More explicit but requires proto change; `0` sentinel is simpler
3. **Query parameter `rollup=true`** — Not idiomatic for gRPC; would need HTTP gateway mapping

## Decision: Rollup logic lives in repository layer, not service

**Rationale**:
- Per Architecture Constraints: business logic in `internal/service`, but data aggregation queries belong in `internal/repository/postgres`
- The rollup is a pure SQL aggregation across `users_orgs` (balance) + `rentals` (counts) joined on user's memberships
- Keeps service thin: `service.GetLedgerSummary` just validates `orgID == 0` → calls `repo.GetCrossOrgSummary`

## Decision: Use CTE with `users_orgs` as membership anchor

**Rationale**:
- Cross-org rollup must only include orgs where the user has an active membership (`users_orgs.status = 'ACTIVE'`)
- Balance comes from `users_orgs.balance_cents` per membership
- Rental counts come from `rentals` where user is renter/owner AND rental's `org_id` is in user's active orgs
- Single query with CTEs avoids N+1 and ensures consistency

**Query structure**:
```sql
WITH user_orgs AS (
  SELECT org_id, balance_cents
  FROM users_orgs
  WHERE user_id = $1 AND status = 'ACTIVE'
),
balance_agg AS (
  SELECT COALESCE(SUM(balance_cents), 0) AS total_balance
  FROM user_orgs
),
rental_counts AS (
  SELECT r.status, COUNT(*) AS cnt
  FROM rentals r
  JOIN user_orgs uo ON r.org_id = uo.org_id
  WHERE r.renter_id = $1 OR r.owner_id = $1
  GROUP BY r.status
)
SELECT 
  (SELECT total_balance FROM balance_agg) AS total_balance,
  (SELECT COALESCE(SUM(cnt), 0) FROM rental_counts) AS total_rentals,
  json_object_agg(status, cnt) AS status_counts
FROM rental_counts;
```

## Decision: Return shape matches existing `GetLedgerSummaryResponse`

**Rationale**:
- Proto response has `balance` (int32) + `status_count` (map<string, int32>)
- Cross-org rollup returns aggregated balance + merged status counts
- No proto change needed — existing clients can consume both single-org and cross-org responses

## Constitution Check

- **Principle I (Code Is Truth)**: Verified against `internal/repository/postgres/ledger.go` — current impl filters by `org_id` in every query
- **Principle IV (Layered Testing)**: New integration test in `tests/integration/ledger_cross_org_test.go`; e2e test in `tests/e2e/ledger_test.go`
- **Principle V (Deployment Parity)**: Single schema change (none required); query works on both Podman and EC2 PostgreSQL
- **Principle VI (Proto-First)**: No proto change; sentinel `org_id=0` uses existing field

## Open Questions Resolved

| Question | Resolution |
|----------|------------|
| How to identify user's active orgs? | `users_orgs` table with `status = 'ACTIVE'` |
| Should rollup include blocked memberships? | No — only `ACTIVE` status per multi-org membership model |
| What if user has no active orgs? | Return zero balance, empty counts (not an error) |
| Performance for users with 50+ orgs? | CTE with single scan of `rentals` per involved org; index on `rentals(org_id, renter_id, owner_id)` exists |