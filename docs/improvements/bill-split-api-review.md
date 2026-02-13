# Bill Split API Design Review

## Overview

This document reviews the gRPC API design for the bill splitting and dispute resolution workflow defined in `api/proto/ubertool_trusted_backend/v1/bill_split_service.proto` and implemented in the service layer.

## Current API Coverage

### User APIs
| API | Purpose | Status |
|-----|---------|--------|
| GetGlobalBillSplitSummary | Get aggregated counts across all orgs | ✅ Complete |
| GetOrganizationBillSplitSummary | Get counts per organization | ✅ Complete |
| ListPayments | List payments in an organization | ⚠️ Needs enhancement |
| GetPaymentDetail | Get payment details with history | ✅ Complete |
| AcknowledgePayment | Acknowledge payment sent/received | ❌ **Bug identified** |

### Admin APIs
| API | Purpose | Status |
|-----|---------|--------|
| ListDisputedPayments | List active disputes | ⚠️ Needs enhancement |
| ListResolvedDisputes | List resolved disputes | ⚠️ Needs enhancement |
| ResolveDispute | Admin resolves a dispute | ⚠️ Needs enhancement |

## Critical Issues

### 1. **CRITICAL BUG: Graceful Resolution After Dispute Not Possible**

**Issue**: The `AcknowledgePayment` implementation only allows acknowledgments when `status = PENDING`:

```go
if bill.Status != domain.BillStatusPending {
    return fmt.Errorf("payment is not in pending status")
}
```

**Requirement violation**: Per `docs/design/bill-split/requirement.md`:
> "A dispute can be resolved in three ways:
> 1. Both the debtor and creditor acknowledge submission and receipt of payment. This represents a graceful resolution of the dispute."

**Impact**: Once a bill enters `DISPUTED` status (after 10 days), users can NO LONGER acknowledge it, even if they now want to settle gracefully. They're forced to wait for admin intervention or system auto-resolution.

**Fix required**: 
- Allow `AcknowledgePayment` to work on bills with `status = DISPUTED`
- When both parties acknowledge a disputed bill, set:
  - `status = PAID`
  - `resolution_outcome = GRACEFUL`
  - `resolved_at = NOW()`
- This provides the graceful resolution path mentioned in requirements

**Code location**: `internal/service/bill_split.go:237-238` and `line:293-294`

---

## Missing Functionality

### 2. Admin Comments on Disputes

**Issue**: The schema includes `ADMIN_COMMENT` action type, but no API exists to add comments.

**Use case**: Admins mediating disputes need to:
- Document conversations with parties
- Record investigation findings
- Provide context for resolution decisions
- Communicate with other admins

**Suggested API**:
```proto
message AddDisputeCommentRequest {
  int32 payment_id = 1;
  string comment = 2;
}

message AddDisputeCommentResponse {
  bool success = 1;
}

rpc AddDisputeComment(AddDisputeCommentRequest) returns (AddDisputeCommentResponse);
```

**Priority**: Medium (aids admin workflow)

---

### 3. Admin Notes on Resolution

**Issue**: `ResolveDisputeRequest` doesn't allow admins to document their reasoning.

**Current**:
```proto
message ResolveDisputeRequest {
  int32 payment_id = 1;
  DisputeResolution resolution = 2;
}
```

```proto
message ResolveDisputeRequest {
  int32 payment_id = 1;
  DisputeResolution resolution = 2;
  string notes = 3; // Admin's explanation for the resolution
}
```

**Benefit**: Better audit trail and transparency

---

### 4. Settlement Month Filtering

**Status**: ✅ **IMPLEMENTED** in proto

**Change made**:
```proto
message ListPaymentsRequest {
  int32 organization_id = 1;
  bool show_history = 2;
}
```

```proto
message ListPaymentsRequest {
  int32 organization_id = 1;
  bool show_history = 2;
  string settlement_month = 3; // Optional: Format 'YYYY-MM', e.g., '2026-01'
  PageRequest pagination = 4; // Optional: Pagination support
}
```

**Use case**: Users can now see "last month's bills" or "January 2026 bills"

---

### 5. Pagination Support

**Status**: ✅ **IMPLEMENTED** in proto
```proto
message PageRequest {
  int32 page_size = 1;  // Default: 50, Max: 100
  string page_token = 2; // Opaque token for next page
}

message ListPaymentsRequest {
  int32 organization_id = 1;
  bool show_history = 2;
  PageRequest pagination = 3; // Optional
}

message ListPaymentsResponse {
  repeated PaymentItem payments = 1;
  string next_page_token = 2; // Empty if no more pages
  int32 total_count = 3; // Optional: total items
}
```

**Priority**: Medium (can be deferred if orgs remain small)

---

### 6. Filter Resolved Disputes by Outcome

**Issue**: `ListResolvedDisputes` returns all resolved disputes; can't filter by resolution type.

**Use case**: Admin wants to see:
- Change made**:
```proto
message PageRequest {
  int32 page_size = 1;  // Default: 50, Max: 100
  string page_token = 2; // Opaque token for next page
}

message ListPaymentsResponse {
  repeated PaymentItem payments = 1;
  string next_page_token = 2; // Empty if no more pages
  int32 total_count = 3; // Total items
}
```proto
message ListResolvedDisputesRequest {
  int32 organization_id = 1;
  string resolution_outcome = 2; // Optional: GRACEFUL, DEBTOR_FAULT, etc.
  PageRequest pagination = 3;
}
```

**Use case**: Admins can filter for specific resolution types for analytics

---

### 7. View Specific User's Bills (Admin)

**Status**: ⏸️ **DEFERRED** - Not implemented

**Status**: ✅ **IMPLEMENTED** in proto

**Change made
---

### 7. View Specific User's Bills (Admin)

**Issue**: No API for admins to view bills for a specific user during mediation.

**Use case**: Admin mediating a dispute needs to:
- See all of user X's bills in the org
- Determine if user has pattern of disputes
- Priority**: Deferred (admins can ask users to share information; existing APIs sufficient)

---

### 8. View and Manage Blocked Users

**Status**: ❌ **REJECTED** - Redundant with existing AdminService APIs

**Why rejected**: 
- `AdminService.ListMembers()` already returns `MemberProfile` with:
  - `is_blocked` boolean
  - `blocked_on` date
  - `blocked_reason` string
- `AdminService.AdminBlockUserAccount()` already supports:
  - Blocking: `block_renting=true`, `block_lending=true`
  - Unblocking: `block_renting=false`, `block_lending=false`
  - Setting/clearing `reason` field
- Adding `ListBlockedUsers` and `UnblockUser` to BillSplitService would create API duplication
- Admins should use the existing centralized user management APIs

**Recommendation**: Use existing `AdminService` APIs for all blocking/unblocking operations

---

### 9. Payment Method Information

**Status**: ❌ **OUT OF SCOPE**

**Why rejected**
}

message UnblockUserResponse {
  bool success = 1;
}

rpc ListBlockedUsers(ListBlockedUsersRequest) returns (ListBlockedUsersResponse);
rpc UnblockUser(UnblockUserRequest) returns (UnblockUserResponse);
```

**Priority**: Medium (important for admin workflows)

---

### 9. Payment Method Information

**Issue**: Requirements mention "mutually agreed-upon payment method" but there's no field to:
- Store preferred payment method (Venmo, Zelle, Cash, etc.)
- Display payment method in payment details
- Allow users to specify payment method

**Considerations**:
- Payment happens outside the app
- Users might need to know HOW to pay each other
- Different creditors might prefer different methods

**Suggested enhancement**:
```proto
message PaymentItem {
  // ... existing fields ...
  string payment_method_notes = 18; // e.g., "Venmo: @johndoe", "Cash"
}

message PaymentMethodInfo {
  int32 user_id = 1;
  string payment_method = 2; // e.g., "Venmo: @johndoe"
**Why out of scope**:
- Payment methods (Venmo, Zelle, Cash, etc.) are handled **outside the app** between users
- This is intentional by design - app tracks obligations, not actual payment transactions
- Users coordinate payment method through other channels
  string user_name = 2;
  int32 balance_cents = 3;
  int64 snapshot_at = 4; // Epoch milliseconds
  string settlement_month = 5;
}

message GetBalanceSnapshotResponse {
  repeated BalanceSnapshot snapshots = 1;
}

rpc GetBalanceSnapshot(GetBalanceSnapshotRequest) returns (GetBalanceSnapshotResponse);
```

**Authorization**: ADMIN/SUPER_ADMIN only

**Priority**: Low (primarily for debugging)

---

### 11. Manual Dispute Opening

**Issue**: Disputes are only opened automatically after 10 days. No way for:
- Users to manually flag a payment as disputed
- Admins to proactively open a dispute

**Use case**: 
- User realizes creditor gave wrong payment info
- Debtor wants to dispute the amount owed
- Admin receives offline complaint

**Suggested API**:
```proto
message OpenDisputeRequest {
  int32 payment_id = 1;
  Status**: ⏸️ **DEFERRED** - Not implemented

**Why deferred**
}

message OpenDisputeResponse {
  bool success = 1;
}

rpPrimarily needed for debugging bill splitting calculations
- Admins/developers can query database directly when needed
- Not part of regular user or admin workflows
- Can be added later if demand arises

**Priority**: Deferred

---

### 11. Manual Dispute Opening

**Status**: ⏸️ **DEFERRED** - Not implemented

**Why deferred: Add GRACEFUL to DisputeResolution enum**: Complete the resolution types

### Medium Priority (Important Improvements)
3. ✅ **COMPLETED: Add notes to ResolveDispute**: Document resolution reasoning
4. ✅ **COMPLETED: Add pagination**: Prepare for scale (can be iterative)
5. ✅ **COMPLETED: Settlement month filtering**: Improve user experience
6. ✅ **COMPLETED: Filter resolved disputes**: Better analytics

### Low Priority (Nice to Have - Not Implemented)
7. **User bills for admin**: Aid mediation (deferred - admins can use existing tools)
8. **Balance snapshot viewing**: Debugging tool (deferred - direct DB access sufficient)

### Rejected (Redundant with Existing APIs)
9. ~~**Add admin comments API**~~: Not justified by requirements; admin mediation happens offline
10. ~~**Add blocked users management**~~: Redundant with `AdminService.ListMembers` and `AdminBlockUserAccount`
11. ~~**Payment method info**~~: Out of scope; handled offline between users

### Very Low Priority (Defer)
12. **Manual dispute opening**: Current auto-flow may suffice

---

## Implementation Roadmap

### Phase 1: Proto Changes (COMPLETED)
- ✅ Add `GRACEFUL` to `DisputeResolution` enum
- ✅ Add `notes` field to `ResolveDisputeRequest`
- ✅ Add pagination to list endpoints
- ✅ Add settlement month filtering to `ListPaymentsRequest`
- ✅ Add resolution outcome filter to `ListResolvedDisputesRequest`

### Phase 2: Service Implementation (REQUIRED)
- **Fix graceful resolution bug**: Modify `AcknowledgePayment` to accept `DISPUTED` status bills
- Implement pagination logic in list methods
- Implement filtering logic for settlement_month and resolution_outcome
- Update `ResolveDispute` to accept and store admin notes

### Phase 3: Analytics and Tooling (Future - Deferred)
- Consider `GetUserBills` for admin if mediation workflows need it
- Consider `GetBalanceSnapshot` API if direct DB access becomes insufficient

---

## Related Files

- **Proto defimatic 10-day dispute trigger aligns with requirements
- Users can contact admins offline if immediate dispute needed
- Admin can intervene using existing tools (block accounts if needed)
- Could be added later if workflow gaps identified

**Priority**: Deferred
- **Business logic**: `docs/design/grpc_api_business_logic.md`
- **Schema**: `podman/trusted-group/postgres/ubertool_schema_trusted.sql`
