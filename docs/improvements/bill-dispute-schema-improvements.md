# Bill Dispute Schema Improvements

## Overview

The current bill dispute design uses an explicit `DISPUTED` status in the `bills` table with supporting metadata fields. This document outlines suggested improvements to enhance data consistency and reduce redundancy.

## Current Design Strengths

- ✅ Query simplicity: `WHERE status = 'DISPUTED'` is straightforward
- ✅ Clear business intent: Explicit domain modeling
- ✅ Indexed performance: Fast queries via status indexes
- ✅ Admin workflows: Simple dispute listing and management

## Suggested Improvements

### 1. Add Constraint Validation

**Problem**: Currently, there's no database-level enforcement ensuring that dispute-related fields are consistent with the bill status.

**Solution**: Add a CHECK constraint to ensure data integrity:

```sql
ALTER TABLE bills ADD CONSTRAINT check_disputed_consistency
CHECK (
    (status = 'DISPUTED' AND disputed_at IS NOT NULL AND dispute_reason IS NOT NULL)
    OR (status != 'DISPUTED' AND disputed_at IS NULL AND dispute_reason IS NULL)
);
```

**Benefits**:
- Prevents invalid states (e.g., `status = 'DISPUTED'` but `dispute_reason` is NULL)
- Catches application bugs at the database level
- Ensures data integrity across all access patterns
- Self-documenting: constraint name explains the business rule

**Implementation Notes**:
- Apply this constraint after migrating existing data to ensure consistency
- Update existing records if needed:
  ```sql
  -- Find and fix any inconsistent records before adding constraint
  SELECT id, status, disputed_at, dispute_reason 
  FROM bills 
  WHERE (status = 'DISPUTED' AND (disputed_at IS NULL OR dispute_reason IS NULL))
     OR (status != 'DISPUTED' AND (disputed_at IS NOT NULL OR dispute_reason IS NOT NULL));
  ```

**Trade-offs**:
- Adds minimal overhead on INSERT/UPDATE operations
- May require application code updates if currently inserting disputed bills in multiple steps
- Makes status transitions more rigid (must update all fields atomically)

### 2. Alternative: Computed Column Approach

**Problem**: The `disputed_at` timestamp is somewhat redundant with the status field, creating potential for inconsistency.

**Solution**: Use a PostgreSQL generated column (requires PostgreSQL 12+):

```sql
-- Migration approach:
-- 1. Drop existing disputed_at column
ALTER TABLE bills DROP COLUMN disputed_at;

-- 2. Add as a generated column
ALTER TABLE bills ADD COLUMN disputed_at TIMESTAMP 
GENERATED ALWAYS AS (
    CASE WHEN status = 'DISPUTED' THEN updated_at ELSE NULL END
) STORED;

-- 3. Update indexes
DROP INDEX IF EXISTS idx_bills_disputed;
CREATE INDEX idx_bills_disputed ON bills(disputed_at) WHERE status = 'DISPUTED';
```

**Benefits**:
- Eliminates redundancy: `disputed_at` automatically derived from status
- Impossible to have inconsistent data
- Single source of truth for dispute timing
- Simplifies application code (one less field to manage)

**Trade-offs**:
- ⚠️ **Loss of granularity**: Uses `updated_at` instead of exact dispute transition time
- ⚠️ **Breaking change**: Requires application code updates
- ⚠️ **Semantic difference**: `disputed_at` would reflect last update, not initial dispute time
- ⚠️ **PostgreSQL version requirement**: Needs PostgreSQL 12+
- ⚠️ **Migration complexity**: Need to preserve historical `disputed_at` values

**Alternative Computed Approach**: If exact dispute timing matters, use the `bill_actions` table as the source of truth:

```sql
-- Create a view for dispute timing
CREATE VIEW bill_dispute_info AS
SELECT 
    b.*,
    ba.created_at as disputed_at
FROM bills b
LEFT JOIN bill_actions ba ON ba.bill_id = b.id 
    AND ba.action_type = 'DISPUTE_OPENED'
WHERE b.status = 'DISPUTED';
```

## Recommendation

**Option 1 (Constraint Validation)** is recommended because:
- Low risk, high benefit
- No breaking changes to application code
- Preserves exact dispute timing
- Easy to implement immediately
- Provides data integrity without sacrificing flexibility

**Option 2 (Computed Column)** should be considered only if:
- You're refactoring the schema anyway
- The timing granularity trade-off is acceptable
- You want maximum consistency guarantees
- You prefer relying on `bill_actions` for historical audit

## Implementation Priority

1. **High Priority**: Add constraint validation (Option 1)
   - Can be implemented immediately
   - Minimal application impact
   - Significant integrity benefit

2. **Low Priority**: Consider computed column (Option 2)
   - Evaluate during next major schema refactoring
   - Requires careful migration planning
   - Consider using view approach for dispute timing instead

## Related Files

- Schema: `podman/trusted-group/postgres/ubertool_schema_trusted.sql`
- Domain model: `internal/domain/bill.go`
- Service layer: `internal/service/bill_split.go`
- Requirements: `docs/design/bill-split/requirement.md`
