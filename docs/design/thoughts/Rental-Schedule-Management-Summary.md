# Rental Schedule Management - Design Summary

## Overview

Added comprehensive rental schedule management functionality to the Ubertool platform with **day-based scheduling** as the core requirement. All rental periods are measured in full days (minimum 1 day, maximum 30 days) to simplify scheduling logic and prevent complex hourly conflicts.

## Key Changes Made

### 1. Backend PRD (`docs/backend-design/PRD-Backend.md`)

**Added Section 4.4: Rental Schedule Management**
- Day-based scheduling system (DATE type, not TIMESTAMP)
- Availability calendar with 90-day default view
- Owner blocking functionality (maintenance, personal use, vacation, other)
- Overlap prevention algorithm
- Conflict detection for rentals and blocked dates
- Multiple pending requests allowed (only accepted/finalized block dates)

**New API Endpoints:**
```
GET    /api/v1/tools/:toolId/schedule
GET    /api/v1/tools/:toolId/schedule/availability
PUT    /api/v1/tools/:toolId/schedule/block
DELETE /api/v1/tools/:toolId/schedule/unblock
GET    /api/v1/tools/:toolId/schedule/stats
```

**Business Rules:**
1. **Request Creation:** Advisory availability check, multiple pending allowed
2. **Request Acceptance:** Strict validation, re-check for conflicts
3. **Request Finalization:** Auto-cancel other requests for same tool
4. **Owner Blocking:** Cannot block dates with finalized/accepted rentals
5. **Cancellation:** Frees up dates immediately

**Error Codes:**
- `RENTAL_DATES_OVERLAP`: Dates conflict with existing rental
- `RENTAL_DATES_BLOCKED`: Dates are owner-blocked
- `RENTAL_PERIOD_INVALID`: Invalid date range
- `RENTAL_PAST_DATES`: Cannot request past dates
- `RENTAL_CANNOT_UNBLOCK`: Cannot unblock confirmed rentals

---

### 2. Database Schema (`docs/backend-design/Database-Schema.md`)

**Modified `rental_requests` Table:**
```sql
-- Changed from TIMESTAMP to DATE
start_date DATE NOT NULL,
end_date DATE NOT NULL,

-- Added composite index for performance
CREATE INDEX idx_rental_requests_tool_dates_status 
  ON rental_requests(tool_id, start_date, end_date, status);
```

**New `blocked_dates` Table:**
```sql
CREATE TABLE blocked_dates (
    id BIGSERIAL PRIMARY KEY,
    tool_id BIGINT NOT NULL REFERENCES tools(id) ON DELETE CASCADE,
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    reason VARCHAR(100) CHECK (reason IN ('maintenance', 'personal_use', 'vacation', 'other')),
    notes TEXT,
    created_by BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT valid_blocked_date_range CHECK (end_date >= start_date)
);

CREATE INDEX idx_blocked_dates_tool_id ON blocked_dates(tool_id);
CREATE INDEX idx_blocked_dates_dates ON blocked_dates(start_date, end_date);
CREATE INDEX idx_blocked_dates_tool_dates ON blocked_dates(tool_id, start_date, end_date);
```

**New Sample Queries:**
```sql
-- Check for overlapping rentals (simplified with DATE type)
SELECT COUNT(*) 
FROM rental_requests
WHERE tool_id = $1
  AND status IN ('accepted', 'finalized')
  AND start_date <= $3  -- new end_date
  AND end_date >= $2;   -- new start_date

-- Get tool schedule (rentals + blocks)
SELECT 'rental' AS type, rr.id, rr.start_date, rr.end_date, rr.status, u.name AS renter_name
FROM rental_requests rr
JOIN users u ON rr.renter_id = u.id
WHERE rr.tool_id = $1 AND rr.start_date <= $3 AND rr.end_date >= $2

UNION ALL

SELECT 'blocked' AS type, bd.id, bd.start_date, bd.end_date, bd.reason AS status, NULL AS renter_name
FROM blocked_dates bd
WHERE bd.tool_id = $1 AND bd.start_date <= $3 AND bd.end_date >= $2
ORDER BY start_date;
```

---

### 3. API Specification (`docs/backend-design/API-Specification.md`)

**New Section 5A: Rental Schedule Endpoints**

#### 5A.1 Get Tool Schedule
```
GET /tools/:toolId/schedule?startDate=2026-01-15&endDate=2026-02-15
```
Returns all rentals (pending, accepted, finalized) and blocked dates for the tool within the date range.

#### 5A.2 Check Availability
```
GET /tools/:toolId/schedule/availability?startDate=2026-01-15&endDate=2026-01-17
```
Returns `available: true/false` and list of conflicts if any.

#### 5A.3 Block Dates (Owner Only)
```
PUT /tools/:toolId/schedule/block
Body: { startDate, endDate, reason, notes }
```
Allows owner to block dates for maintenance, personal use, vacation, or other reasons.

#### 5A.4 Unblock Dates (Owner Only)
```
DELETE /tools/:toolId/schedule/unblock
Body: { startDate, endDate }
```
Removes blocked dates if no confirmed rentals exist.

#### 5A.5 Get Rental Statistics
```
GET /tools/:toolId/schedule/stats
```
Returns utilization rate, rented days, blocked days, revenue estimates.

**Updated Section 6: Rental Request Endpoints**

Changed date format from ISO 8601 timestamp to simple date:
- **Before:** `"startDate": "2026-01-15T09:00:00Z"`
- **After:** `"startDate": "2026-01-15"`

Added validation rules:
- Dates must be in YYYY-MM-DD format
- `startDate` must be >= today
- `endDate` must be > `startDate`
- Rental period must be <= 30 days

Added error responses:
- `422 Unprocessable Entity` for invalid date ranges
- `409 Conflict` for overlapping rentals when accepting requests

---

## Conflict Detection Algorithm

```
For new rental request [start_date, end_date]:
  Check if ANY existing rental/block satisfies:
    (existing.start_date <= new.end_date) AND 
    (existing.end_date >= new.start_date)
  If match found → CONFLICT
  Else → AVAILABLE
```

This simple algorithm works because:
1. Dates are inclusive (both start and end dates count)
2. Using DATE type eliminates time-of-day complexity
3. Single comparison covers all overlap scenarios

---

## Example Scenarios

### Scenario 1: Valid Request Flow
```
Tool has no bookings
Request A: Jan 15-17 → ALLOWED (pending)
Request B: Jan 20-22 → ALLOWED (pending)
Owner accepts Request A → SUCCESS (dates now reserved)
Request C: Jan 16-18 → REJECTED (overlaps with accepted)
```

### Scenario 2: Owner Blocking
```
Owner blocks: Jan 10-12 (maintenance)
Request: Jan 11-13 → REJECTED (overlaps blocked dates)
Request: Jan 13-15 → ALLOWED (no overlap)
```

### Scenario 3: Multiple Pending Requests
```
Request A: Jan 15-17 (pending)
Request B: Jan 15-17 (pending) → ALLOWED (pending doesn't block)
Request C: Jan 16-18 (pending) → ALLOWED
Owner accepts Request A → SUCCESS
Requests B and C auto-rejected (conflict with accepted)
```

---

## Performance Optimizations

1. **Composite Index:** `(tool_id, start_date, end_date, status)` for fast conflict checks
2. **Caching:** Tool schedules cached for 5 minutes (frequently viewed tools)
3. **PostgreSQL DATERANGE:** Can use native date range types for even better performance
4. **Batch Checks:** Availability checks batched for multiple tools in search results

---

## Migration Path

### Database Migration Order:
1. Add `blocked_dates` table
2. Modify `rental_requests` table:
   - Change `start_date` from TIMESTAMP to DATE
   - Change `end_date` from TIMESTAMP to DATE
   - Add composite index
3. Update ERD diagram
4. Create sample queries

### API Migration:
1. Add new schedule endpoints (Section 5A)
2. Update rental request endpoints to use DATE format
3. Add new error codes
4. Update API documentation

---

## Testing Checklist

- [ ] Create rental request with valid dates
- [ ] Create rental request with invalid dates (past, end before start, > 30 days)
- [ ] Accept request with no conflicts
- [ ] Accept request with overlapping rental (should fail)
- [ ] Accept request with overlapping block (should fail)
- [ ] Multiple pending requests for same dates (should succeed)
- [ ] Finalize request (should cancel other pending/accepted)
- [ ] Block dates with no rentals (should succeed)
- [ ] Block dates with finalized rental (should fail)
- [ ] Unblock dates with no rentals (should succeed)
- [ ] Unblock dates with finalized rental (should fail)
- [ ] Get tool schedule with mixed rentals and blocks
- [ ] Check availability for available dates
- [ ] Check availability for blocked dates
- [ ] Get rental statistics with utilization calculations

---

## Summary

The rental schedule management system provides:
✅ **Day-based scheduling** (simplified, no hourly conflicts)  
✅ **Owner blocking** (maintenance, personal use, vacation)  
✅ **Overlap prevention** (strict validation on acceptance)  
✅ **Multiple pending requests** (flexible for renters)  
✅ **Availability calendar** (90-day default view)  
✅ **Rental statistics** (utilization, revenue estimates)  
✅ **Clear error handling** (specific error codes)  
✅ **Performance optimized** (indexed queries, caching)

All design documents have been updated to reflect these requirements.
