# Research: Bill Split Multi-Org Summaries (FR-014, FR-015)

**Date**: 2026-07-27  
**Context**: Implement FR-014 (global summary) + FR-015 (per-org summary)

## Decision: Two Separate RPCs

**Rationale**:
- Different auth: Global = any member; Org = admin only
- Different aggregation: Global = union across user's orgs; Org = single org breakdown
- Explicit API > flag magic

## Decision: Global Summary = All Active Orgs

```sql
-- FR-014: User's bills across all active orgs
WITH user_orgs AS (
  SELECT org_id FROM users_orgs 
  WHERE user_id = $1 AND status = 'ACTIVE'
)
SELECT 
  COUNT(*) FILTER (WHERE status = 'PENDING') AS pending_count,
  COUNT(*) FILTER (WHERE status = 'ACKNOWLEDGED') AS acknowledged_count,
  COUNT(*) FILTER (WHERE status = 'DISPUTED') AS disputed_count,
  COUNT(*) FILTER (WHERE status = 'RESOLVED') AS resolved_count,
  COALESCE(SUM(amount_cents) FILTER (WHERE status != 'RESOLVED'), 0) AS total_amount_cents
FROM bills
WHERE (debtor_id = $1 OR creditor_id = $1)
  AND org_id IN (SELECT org_id FROM user_orgs);
```

## Decision: Org Summary = Admin Only

```sql
-- FR-015: Admin view of single org
SELECT 
  COUNT(*) FILTER (WHERE status = 'PENDING') AS pending_count,
  COUNT(*) FILTER (WHERE status = 'ACKNOWLEDGED') AS acknowledged_count,
  COUNT(*) FILTER (WHERE status = 'DISPUTED') AS disputed_count,
  COUNT(*) FILTER (WHERE status = 'RESOLVED') AS resolved_count,
  COALESCE(SUM(amount_cents) FILTER (WHERE status != 'RESOLVED'), 0) AS total_amount_cents
FROM bills
WHERE org_id = $1;
```

## Constitution Check

| Principle | Check |
|-----------|-------|
| I. Code Is Truth | Verified against `internal/service/bill_split.go`, `billing_jobs.go` |
| II. Domain Constants | `BillStatus` constants used |
| III. Push Notifications | Summary reads don't trigger notifications |
| IV. Layered Testing | New integration + e2e tests |
| V. Deployment Parity | No schema change; uses existing `bills` table |
| VI. Proto-First | Two new RPCs added → regenerate |

## Open Questions Resolved

| Question | Resolution |
|----------|------------|
| Does global include resolved bills? | Yes — but `total_amount_cents` excludes RESOLVED |
| What if user has 0 active orgs? | Return all zeros |
| Per-org: admin in multiple orgs? | Each call targets one org; caller chooses |
| Per-org: non-admin calls? | `PERMISSION_DENIED` |