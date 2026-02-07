# API Guide: Updating Extension Requests While Pending

## Overview

This guide demonstrates how to update a rental extension request while it's still pending owner approval (status: `RETURN_DATE_CHANGED`).

## Scenario

A renter submits an extension request to extend the rental return date. Before the owner approves it, the renter realizes they need to extend it even further (or less). The renter can update their pending extension request by calling `ChangeRentalDates()` again.

## API Workflow

### 1. Submit Initial Extension Request

**Prerequisites:**
- Rental must be in `ACTIVE` or `OVERDUE` status
- Only the renter can submit extension requests

**API Call:**
```protobuf
ChangeRentalDatesRequest {
  request_id: <rental_id>
  new_end_date: "2026-02-09"  // Extend by 1 day
}
```

**Result:**
- Status changes from `ACTIVE` → `RETURN_DATE_CHANGED`
- `end_date` field is populated with the requested date
- `total_cost_cents` is recalculated
- Owner receives notification

### 2. Update the Extension Request (While Still Pending)

**Prerequisites:**
- Rental must be in `RETURN_DATE_CHANGED` status
- Only the renter can update their own extension request
- Owner has not yet approved/rejected the request

**API Call:**
```protobuf
ChangeRentalDatesRequest {
  request_id: <rental_id>
  new_end_date: "2026-02-10"  // Update to extend by 2 days instead
}
```

**Result:**
- Status remains `RETURN_DATE_CHANGED`
- ✅ **`end_date` field is UPDATED with the new requested date**
- `total_cost_cents` is recalculated based on the new date
- Owner receives notification about the update

### 3. Owner Approves the (Updated) Extension

**API Call:**
```protobuf
ApproveReturnDateChangeRequest {
  request_id: <rental_id>
}
```

**Result:**
- Status changes back to `ACTIVE` (or `OVERDUE` if past due)
- `end_date` remains with the approved date
- `last_agreed_end_date` is updated to store the agreed date for potential rollback
- Renter receives approval notification

## Important Notes

### Database Field Behavior

| Field | Initial State (ACTIVE) | After Extension Request | After Update Request | After Approval |
|-------|----------------------|------------------------|---------------------|----------------|
| `status` | ACTIVE | RETURN_DATE_CHANGED | RETURN_DATE_CHANGED | ACTIVE |
| `last_agreed_end_date` | 2026-02-08 | 2026-02-08 (unchanged) | 2026-02-08 (unchanged) | 2026-02-10 (updated) |
| `end_date` | 2026-02-08 | 2026-02-09 (requested) | 2026-02-10 (UPDATED) | 2026-02-10 (approved) |
| `total_cost_cents` | 2000 | 3000 (recalculated) | 4000 (recalculated) | 4000 |

**Note:** 
- `end_date` (NOT NULL) is the primary working field that always contains the current/requested date
- `last_agreed_end_date` (nullable) stores the last confirmed date for potential rollback scenarios

### Client Implementation Guidelines

1. **Check rental status** before calling the API:
   - `ACTIVE` or `OVERDUE` → Can submit initial extension request
   - `RETURN_DATE_CHANGED` → Can update existing extension request

2. **Display the pending request** to the user:
   - Show `end_date` field as the "Current/Requested Return Date"
   - Show `last_agreed_end_date` as the "Last Agreed Return Date" (if available)

3. **Allow updates** while status is `RETURN_DATE_CHANGED`:
   - Show an "Update Extension Request" button
   - Call `ChangeRentalDates()` with the new date
   - No need to cancel and recreate - just send the updated date

4. **Handle the waiting period**:
   - Inform the user their request is pending owner approval
   - Poll for status changes or use notifications
   - Status will change to `ACTIVE` when approved, or `RETURN_DATE_CHANGE_REJECTED` when rejected

## Code Example (E2E Test)

See the complete working example in:
```
tests/e2e/rental_test.go
Test: "Update Extension Request While Pending (RETURN_DATE_CHANGED)"
```

This test demonstrates:
1. Creating and activating a rental
2. Submitting an initial extension request (1 day)
3. Updating the extension request (2 days)
4. Verifying the `end_date` field was modified in the database
5. Owner approving the updated extension

## Test Output

```
✓ Initial extension request submitted successfully. 
  Status: RETURN_DATE_CHANGED, EndDate: 2026-02-09, Cost: 3000 cents

✓ Extension request updated successfully!
  - Status: RENTAL_STATUS_RETURN_DATE_CHANGED
  - First extension date:  2026-02-09 (cost: 3000 cents)
  - Updated extension date: 2026-02-10 (cost: 4000 cents)
  - EndDate field was successfully modified from 2026-02-09 to 2026-02-10

✓ Extension approved by owner. EndDate updated to: 2026-02-10
```

## API Reference

### ChangeRentalDates

**Request:**
```protobuf
message ChangeRentalDatesRequest {
  int32 request_id = 1;
  string new_start_date = 2;  // Optional, format: YYYY-MM-DD
  string new_end_date = 3;    // Required for extensions, format: YYYY-MM-DD
  string old_start_date = 4;  // Optional, for optimistic locking
  string old_end_date = 5;    // Optional, for optimistic locking
}
```

**Response:**
```protobuf
message ChangeRentalDatesResponse {
  RentalRequest rental_request = 1;  // Updated rental with new dates
}
```

**Status Codes:**
- `OK` - Successfully updated
- `PERMISSION_DENIED` - User is not the renter
- `FAILED_PRECONDITION` - Invalid status (not ACTIVE, OVERDUE, or RETURN_DATE_CHANGED)
- `INVALID_ARGUMENT` - Invalid date format

## Troubleshooting

### "Cannot change dates in current status and role"
- **Cause:** Rental is not in ACTIVE, OVERDUE, or RETURN_DATE_CHANGED status
- **Solution:** Check the rental status before calling the API

### "Unauthorized"
- **Cause:** The user is not the renter of this rental
- **Solution:** Ensure you're making the API call with the renter's user ID

### End date not updating in database
- **Cause:** This was a bug that has been fixed
- **Solution:** Ensure you're using the latest version of the backend service
- **Verification:** Run the E2E test to confirm the functionality works

## Version History

- **v1.0** (2026-02-06): Added support for updating extension requests while in RETURN_DATE_CHANGED status
