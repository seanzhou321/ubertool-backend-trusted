# Design Document: 008-bill-split FR-015 — Membership Auth + Per-Org Summary Fix

**Gap ID**: FR-015 (Convergence C1: CRITICAL, C2: HIGH)
**Status**: Design Phase — Ready for Implementation
**Target**: Close two critical gaps in `GetOrganizationBillSplitSummary`

---

## 1. Problem Analysis

### Current Behavior (Broken)

| Layer | Current Code | Issue |
|-------|--------------|-------|
| **Proto** (`bill_split_service.proto:35-36`) | `message GetOrganizationBillSplitSummaryRequest {}` | Empty request — no `organization_id` field |
| **Handler** (`bill_split.go:55-78`) | No auth check; calls service with only `userID` | **C1 CRITICAL**: Any authenticated user can call; no membership verification |
| **Service** (`bill_split.go:97-138`) | `GetOrganizationBillSplitSummary(ctx, userID)` loops `ListUserOrgs()` | **C2 HIGH**: Returns ALL user's orgs, not the specified org; ignores intended `org_id` parameter |

### Expected Behavior (Per Spec FR-015 + Original Design)

> **FR-015**: `GetOrganizationBillSplitSummary` MUST return the same four count categories as `GetGlobalBillSplitSummary`, but **broken down PER ORGANIZATION the caller belongs to**. Each entry in the list MUST include the `organization_id`, `organization_name`, and the four counts for that specific org. **This enables per-org drill-down from the global summary.**

> **Original Design** (`docs/design/grpc_api_business_logic.md`): "For **each organization the user is a member of**, calculate the BillSplitSummary... Return the list."

**Key Clarification**: This is a **member-facing dashboard RPC**, not an admin tool. Any authenticated member should see **their own** bill-split standing in a specific organization. The auth check is **membership verification** (like `ListPayments`), NOT admin verification.

### Comparison with Other Bill-Split RPCs

| RPC | Auth Required | Data Scope |
|-----|---------------|------------|
| `GetGlobalBillSplitSummary` | Any authenticated member | Caller's aggregated counts across **all** their orgs |
| `GetOrganizationBillSplitSummary` | **Member of target org** | Caller's per-org breakdown (drill-down) |
| `ListPayments` | Member of org | Caller's bills in that org |
| `GetPaymentDetail` | Debtor/Creditor/**Org Admin** | Specific bill (could be someone else's if admin) |
| `ListDisputedPayments` | **Org Admin** | **All** disputed bills in org (admin tool) |
| `ListResolvedDisputes` | **Org Admin** | **All** resolved disputes in org (admin tool) |

**Pattern**: Dashboard/summary RPCs = caller's own data, member access. Admin RPCs = other people's data, admin access.

---

## 2. Design Specification

### 2.1 Proto Changes (Principle VI: Proto-First)

**File**: `api/proto/ubertool_trusted_backend/v1/bill_split_service.proto`

```protobuf
// BEFORE
message GetOrganizationBillSplitSummaryRequest {}

// AFTER
message GetOrganizationBillSplitSummaryRequest {
  int32 organization_id = 1;  // REQUIRED: Target org for per-org breakdown
}
```

**Response unchanged** (already supports list of org summaries):
```protobuf
message OrganizationBillSplitSummary {
  int32 organization_id = 1;
  string organization_name = 2;
  BillSplitSummary summary = 3;
}

message GetOrganizationBillSplitSummaryResponse {
  repeated OrganizationBillSplitSummary org_summaries = 1;
}
```

**Rationale**: 
- Request now accepts `organization_id` for drill-down from global summary
- Response stays as repeated field for future extensibility (batch queries), but handler returns single-element list

---

### 2.2 Handler Changes (Membership Gate)

**File**: `internal/api/grpc/bill_split.go`

```go
func (h *BillSplitHandler) GetOrganizationBillSplitSummary(ctx context.Context, req *pb.GetOrganizationBillSplitSummaryRequest) (*pb.GetOrganizationBillSplitSummaryResponse, error) {
	userID, err := GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// C1 FIX: Validate organization_id is provided
	if req.OrganizationId == 0 {
		return nil, status.Error(codes.InvalidArgument, "organization_id is required")
	}

	// C1 FIX: Verify caller is a MEMBER of the TARGET organization (not admin!)
	userOrg, err := h.userSvc.GetUserOrg(ctx, userID, req.OrganizationId)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to verify organization membership")
	}
	if userOrg == nil {
		return nil, status.Error(codes.PermissionDenied, "not a member of this organization")
	}
	// NO role check — any member (MEMBER, ADMIN, SUPER_ADMIN) can see their own standing

	// C2 FIX: Pass org_id to service (single org, not all orgs)
	orgSummaries, err := h.billSplitSvc.GetOrganizationBillSplitSummary(ctx, userID, req.OrganizationId)
	if err != nil {
		return nil, err
	}

	return &pb.GetOrganizationBillSplitSummaryResponse{
		OrgSummaries: orgSummaries,
	}, nil
}
```

**Error Codes**:
| Condition | gRPC Code | Message |
|-----------|-----------|---------|
| `organization_id == 0` | `InvalidArgument` | "organization_id is required" |
| Not a member of org | `PermissionDenied` | "not a member of this organization" |
| Internal error | `Internal` | "failed to verify organization membership" |

---

### 2.3 Service Changes (Single-Org Logic)

**File**: `internal/service/bill_split.go`

```go
// BEFORE: GetOrganizationBillSplitSummary(ctx, userID int32) ([]domain.Organization, []int32, []int32, []int32, []int32, error)
// AFTER:  GetOrganizationBillSplitSummary(ctx, userID, orgID int32) ([]*pb.OrganizationBillSplitSummary, error)

func (s *billSplitService) GetOrganizationBillSplitSummary(ctx context.Context, userID, orgID int32) ([]*pb.OrganizationBillSplitSummary, error) {
	logger.EnterMethod("billSplitService.GetOrganizationBillSplitSummary", "userID", userID, "orgID", orgID)

	// Defense-in-depth: Verify membership (handler already checked)
	userOrg, err := s.userRepo.GetUserOrg(ctx, userID, orgID)
	if err != nil {
		logger.ExitMethodWithError("billSplitService.GetOrganizationBillSplitSummary", err, "userID", userID, "orgID", orgID)
		return nil, err
	}
	if userOrg == nil {
		return nil, fmt.Errorf("user is not a member of organization %d", orgID)
	}

	// Get org details for name
	org, err := s.orgRepo.GetByID(ctx, orgID)
	if err != nil {
		logger.ExitMethodWithError("billSplitService.GetOrganizationBillSplitSummary", err, "orgID", orgID)
		return nil, err
	}
	if org == nil {
		return nil, fmt.Errorf("organization %d not found", orgID)
	}

	// C2 FIX: Query ONLY the specified org (not all user's orgs)
	p, r, pd, rd, err := s.getOrgSummary(ctx, userID, orgID)
	if err != nil {
		logger.ExitMethodWithError("billSplitService.GetOrganizationBillSplitSummary", err, "userID", userID, "orgID", orgID)
		return nil, err
	}

	summary := &pb.OrganizationBillSplitSummary{
		OrganizationId:   org.ID,
		OrganizationName: org.Name,
		Summary: &pb.BillSplitSummary{
			PaymentsToMake:    p,
			ReceiptsToVerify:  r,
			PaymentsInDispute: pd,
			ReceiptsInDispute: rd,
		},
	}

	logger.ExitMethod("billSplitService.GetOrganizationBillSplitSummary", "userID", userID, "orgID", orgID, "summary", summary)
	return []*pb.OrganizationBillSplitSummary{summary}, nil
}
```

**Key Changes**:
- Signature: Add `orgID int32` parameter
- Remove `ListUserOrgs()` loop — query single org only
- Return `[]*pb.OrganizationBillSplitSummary` (proto type) directly
- Reuse existing `getOrgSummary()` helper (already takes `orgID`)

---

### 2.4 Repository (No Changes Needed)

The existing `getOrgSummary(ctx, userID, orgID)` in service layer already calls:
```go
bills, err := s.billRepo.ListByUser(ctx, userID, orgID, nil)
```
which correctly filters by `org_id`. No repository changes required.

---

## 3. Test Design (TDD — Write Tests First)

### 3.1 Unit Tests — Service (`tests/unit/bill_split_service_test.go`)

```go
func TestBillSplitService_GetOrganizationBillSplitSummary_Success(t *testing.T) {
	// Arrange
	ctx := context.Background()
	userID := int32(1)
	orgID := int32(100)
	
	mockUserRepo.On("GetUserOrg", ctx, userID, orgID).Return(&domain.UserOrg{Role: domain.UserOrgRoleMember}, nil) // MEMBER is fine
	mockOrgRepo.On("GetByID", ctx, orgID).Return(&domain.Organization{ID: orgID, Name: "Test Org"}, nil)
	mockBillRepo.On("ListByUser", ctx, userID, orgID, (*[]domain.BillStatus)(nil)).Return([]domain.Bill{
		{ID: 1, DebtorUserID: userID, CreditorUserID: 2, Status: domain.BillStatusPending, DebtorAcknowledgedAt: nil, AmountCents: 1000, OrgID: orgID},
		{ID: 2, DebtorUserID: 2, CreditorUserID: userID, Status: domain.BillStatusPending, DebtorAcknowledgedAt: &now, AmountCents: 2000, OrgID: orgID},
		{ID: 3, DebtorUserID: userID, CreditorUserID: 3, Status: domain.BillStatusDisputed, AmountCents: 3000, OrgID: orgID},
	}, nil)

	// Act
	summaries, err := svc.GetOrganizationBillSplitSummary(ctx, userID, orgID)

	// Assert
	require.NoError(t, err)
	require.Len(t, summaries, 1)
	assert.Equal(t, orgID, summaries[0].OrganizationId)
	assert.Equal(t, "Test Org", summaries[0].OrganizationName)
	assert.Equal(t, int32(1), summaries[0].Summary.PaymentsToMake)      // Bill 1: debtor, not acked
	assert.Equal(t, int32(1), summaries[0].Summary.ReceiptsToVerify)   // Bill 2: creditor, debtor acked
	assert.Equal(t, int32(1), summaries[0].Summary.PaymentsInDispute)  // Bill 3: debtor, disputed
	assert.Equal(t, int32(0), summaries[0].Summary.ReceiptsInDispute)
	
	// Verify getOrgSummary called with CORRECT orgID (not all orgs)
	mockBillRepo.AssertCalled(t, "ListByUser", ctx, userID, orgID, mock.Anything)
}

func TestBillSplitService_GetOrganizationBillSplitSummary_AdminAlsoWorks(t *testing.T) {
	// Admin should also work (not just MEMBER)
	mockUserRepo.On("GetUserOrg", ctx, userID, orgID).Return(&domain.UserOrg{Role: domain.UserOrgRoleAdmin}, nil)
	// ... same assertions
}

func TestBillSplitService_GetOrganizationBillSplitSummary_NotMember(t *testing.T) {
	mockUserRepo.On("GetUserOrg", ctx, userID, orgID).Return((*domain.UserOrg)(nil), nil)
	
	_, err := svc.GetOrganizationBillSplitSummary(ctx, userID, orgID)
	
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a member")
}

func TestBillSplitService_GetOrganizationBillSplitSummary_OrgNotFound(t *testing.T) {
	mockUserRepo.On("GetUserOrg", ctx, userID, orgID).Return(&domain.UserOrg{Role: domain.UserOrgRoleMember}, nil)
	mockOrgRepo.On("GetByID", ctx, orgID).Return((*domain.Organization)(nil), nil)
	
	_, err := svc.GetOrganizationBillSplitSummary(ctx, userID, orgID)
	
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
```

### 3.2 Unit Tests — Handler (`tests/unit/handlers/bill_split_handler_test.go`)

```go
func TestBillSplitHandler_GetOrganizationBillSplitSummary_MissingOrgID(t *testing.T) {
	req := &pb.GetOrganizationBillSplitSummaryRequest{} // OrganizationId = 0
	_, err := handler.GetOrganizationBillSplitSummary(ctx, req)
	
	assert.GRPCCode(t, err, codes.InvalidArgument)
	assert.Contains(t, err.Error(), "organization_id is required")
}

func TestBillSplitHandler_GetOrganizationBillSplitSummary_NotMember(t *testing.T) {
	mockUserSvc.On("GetUserOrg", ctx, userID, orgID).Return((*domain.UserOrg)(nil), nil)
	
	req := &pb.GetOrganizationBillSplitSummaryRequest{OrganizationId: orgID}
	_, err := handler.GetOrganizationBillSplitSummary(ctx, req)
	
	assert.GRPCCode(t, err, codes.PermissionDenied)
	assert.Contains(t, err.Error(), "not a member")
}

func TestBillSplitHandler_GetOrganizationBillSplitSummary_MemberSuccess(t *testing.T) {
	mockUserSvc.On("GetUserOrg", ctx, userID, orgID).Return(&domain.UserOrg{Role: domain.UserOrgRoleMember}, nil)
	mockBillSplitSvc.On("GetOrganizationBillSplitSummary", ctx, userID, orgID).Return(
		[]*pb.OrganizationBillSplitSummary{{OrganizationId: orgID, OrganizationName: "Test", Summary: &pb.BillSplitSummary{}}},
		nil)
	
	req := &pb.GetOrganizationBillSplitSummaryRequest{OrganizationId: orgID}
	resp, err := handler.GetOrganizationBillSplitSummary(ctx, req)
	
	require.NoError(t, err)
	assert.Len(t, resp.OrgSummaries, 1)
	assert.Equal(t, orgID, resp.OrgSummaries[0].OrganizationId)
}

func TestBillSplitHandler_GetOrganizationBillSplitSummary_AdminSuccess(t *testing.T) {
	// Admin should also work
	mockUserSvc.On("GetUserOrg", ctx, userID, orgID).Return(&domain.UserOrg{Role: domain.UserOrgRoleAdmin}, nil)
	mockBillSplitSvc.On("GetOrganizationBillSplitSummary", ctx, userID, orgID).Return(
		[]*pb.OrganizationBillSplitSummary{{OrganizationId: orgID, OrganizationName: "Test", Summary: &pb.BillSplitSummary{}}},
		nil)
	
	req := &pb.GetOrganizationBillSplitSummaryRequest{OrganizationId: orgID}
	resp, err := handler.GetOrganizationBillSplitSummary(ctx, req)
	
	require.NoError(t, err)
	assert.Len(t, resp.OrgSummaries, 1)
}

func TestBillSplitHandler_GetOrganizationBillSplitSummary_SuperAdminSuccess(t *testing.T) {
	// SuperAdmin should also work
	mockUserSvc.On("GetUserOrg", ctx, userID, orgID).Return(&domain.UserOrg{Role: domain.UserOrgRoleSuperAdmin}, nil)
	mockBillSplitSvc.On("GetOrganizationBillSplitSummary", ctx, userID, orgID).Return(
		[]*pb.OrganizationBillSplitSummary{{OrganizationId: orgID, OrganizationName: "Test", Summary: &pb.BillSplitSummary{}}},
		nil)
	
	req := &pb.GetOrganizationBillSplitSummaryRequest{OrganizationId: orgID}
	resp, err := handler.GetOrganizationBillSplitSummary(ctx, req)
	
	require.NoError(t, err)
	assert.Len(t, resp.OrgSummaries, 1)
}
```

### 3.3 Integration Tests (`tests/integration/bill_split_test.go`)

```go
func TestBillSplitService_Integration_GetOrganizationBillSplitSummary_MemberAccess(t *testing.T) {
	// Seed: user1 = MEMBER in org1
	//       Bills in org1 for user1: 1 debtor-unacked, 1 creditor-acked, 1 disputed
	
	summaries, err := svc.GetOrganizationBillSplitSummary(ctx, user1ID, org1ID)
	
	require.NoError(t, err)
	require.Len(t, summaries, 1)
	assert.Equal(t, org1ID, summaries[0].OrganizationId)
	assert.Equal(t, int32(1), summaries[0].Summary.PaymentsToMake)
	assert.Equal(t, int32(1), summaries[0].Summary.ReceiptsToVerify)
	assert.Equal(t, int32(1), summaries[0].Summary.PaymentsInDispute)
	assert.Equal(t, int32(0), summaries[0].Summary.ReceiptsInDispute)
}

func TestBillSplitService_Integration_GetOrganizationBillSplitSummary_AdminAccess(t *testing.T) {
	// user2 = ADMIN in org1
	summaries, err := svc.GetOrganizationBillSplitSummary(ctx, user2ID, org1ID)
	
	require.NoError(t, err)
	require.Len(t, summaries, 1)
	// Same data accessible
}

func TestBillSplitService_Integration_GetOrganizationBillSplitSummary_NonMemberRejected(t *testing.T) {
	// user3 is NOT a member of org1
	_, err := svc.GetOrganizationBillSplitSummary(ctx, user3ID, org1ID)
	
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a member")
}

func TestBillSplitService_Integration_GetOrganizationBillSplitSummary_CrossOrgRejected(t *testing.T) {
	// user1 is MEMBER in org1, NOT a member of org2
	_, err := svc.GetOrganizationBillSplitSummary(ctx, user1ID, org2ID)
	
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a member")
}
```

### 3.4 E2E Tests (`tests/e2e/bill_split_test.go`)

```go
func TestBillSplitService_E2E_GetOrganizationBillSplitSummary_MemberSuccess(t *testing.T) {
	// Setup: Create org, make user1 MEMBER (not admin)
	// Create bills in org for user1
	// Login as user1
	
	resp, err := client.GetOrganizationBillSplitSummary(ctx, &pb.GetOrganizationBillSplitSummaryRequest{
		OrganizationId: orgID,
	})
	
	require.NoError(t, err)
	assert.Len(t, resp.OrgSummaries, 1)
	assert.Equal(t, orgID, resp.OrgSummaries[0].OrganizationId)
	assert.Equal(t, "Test Org", resp.OrgSummaries[0].OrganizationName)
	// Verify all four counts present
}

func TestBillSplitService_E2E_GetOrganizationBillSplitSummary_AdminSuccess(t *testing.T) {
	// Login as admin user
	resp, err := client.GetOrganizationBillSplitSummary(ctx, &pb.GetOrganizationBillSplitSummaryRequest{
		OrganizationId: orgID,
	})
	
	require.NoError(t, err)
	assert.Len(t, resp.OrgSummaries, 1)
}

func TestBillSplitService_E2E_GetOrganizationBillSplitSummary_NonMemberRejected(t *testing.T) {
	// Login as user3 (not in org at all)
	_, err := client.GetOrganizationBillSplitSummary(ctx, &pb.GetOrganizationBillSplitSummaryRequest{
		OrganizationId: orgID,
	})
	assert.GRPCCode(t, err, codes.PermissionDenied)
}

func TestBillSplitService_E2E_GetOrganizationBillSplitSummary_MultiOrgDrillDown(t *testing.T) {
	// User is MEMBER in org1 AND org2
	// Bills exist in both orgs with DIFFERENT counts
	// Call twice with different orgIDs, verify each returns ONLY that org's data
	
	resp1, _ := client.GetOrganizationBillSplitSummary(ctx, &pb.GetOrganizationBillSplitSummaryRequest{OrganizationId: org1ID})
	resp2, _ := client.GetOrganizationBillSplitSummary(ctx, &pb.GetOrganizationBillSplitSummaryRequest{OrganizationId: org2ID})
	
	assert.Equal(t, org1ID, resp1.OrgSummaries[0].OrganizationId)
	assert.Equal(t, org2ID, resp2.OrgSummaries[0].OrganizationId)
	assert.NotEqual(t, resp1.OrgSummaries[0].Summary.PaymentsToMake, resp2.OrgSummaries[0].Summary.PaymentsToMake)
}
```

---

## 4. Implementation Sequence (TDD Order)

| Step | Action | Command | Verification |
|------|--------|---------|--------------|
| 1 | **Proto First** | Edit proto → `make proto-gen` | Generated code compiles |
| 2 | **Handler Tests** | Write failing handler unit tests | `go test ./tests/unit/handlers -run TestBillSplitHandler_GetOrganizationBillSplitSummary` → FAIL |
| 3 | **Handler Impl** | Implement membership check in handler | Handler tests → PASS |
| 4 | **Service Tests** | Write failing service unit tests | `go test ./tests/unit -run TestBillSplitService_GetOrganizationBillSplitSummary` → FAIL |
| 5 | **Service Impl** | Update service signature + logic | Service tests → PASS |
| 6 | **Integration Tests** | Write integration tests | `go test ./tests/integration -run TestBillSplitService_Integration_GetOrganizationBillSplitSummary` → PASS |
| 7 | **E2E Tests** | Write E2E tests | `go test ./tests/e2e -run TestBillSplitService_E2E_GetOrganizationBillSplitSummary` → PASS |
| 8 | **Full Suite** | `make test-unit && make test-integration && make test-e2e` | All green |
| 9 | **RTM Update** | Update `sbr/rtm/008-bill-split.rtm.md` | FR-015 shows Complete at L1/L2/L3 |

---

## 5. Files to Modify

| File | Change Type | Description |
|------|-------------|-------------|
| `api/proto/ubertool_trusted_backend/v1/bill_split_service.proto` | **Modify** | Add `organization_id` to `GetOrganizationBillSplitSummaryRequest` |
| `internal/api/grpc/bill_split.go` | **Modify** | Add membership auth check + pass `orgID` to service |
| `internal/service/bill_split.go` | **Modify** | Change signature to accept `orgID`, query single org |
| `tests/unit/bill_split_service_test.go` | **Add** | Service unit tests (4 test cases) |
| `tests/unit/handlers/bill_split_handler_test.go` | **Add** | Handler unit tests (5 test cases) |
| `tests/integration/bill_split_test.go` | **Add** | Integration tests (4 test cases) |
| `tests/e2e/bill_split_test.go` | **Add** | E2E tests (4 test cases) |

---

## 6. Rollback Plan

If issues arise post-deployment:
1. Revert proto change → `make proto-gen`
2. Revert handler to old signature (no auth check, no orgID param)
3. Revert service to old signature (loop all orgs)
4. Old behavior: returns all user's orgs with no auth check (current broken state)

No feature flag needed — this is a bug fix, not a new feature rollout.

---

## 7. Verification Checklist

- [ ] Proto compiles after `make proto-gen`
- [ ] Handler returns `InvalidArgument` when `organization_id=0`
- [ ] Handler returns `PermissionDenied` for non-members
- [ ] Handler returns success for MEMBER, ADMIN, SUPER_ADMIN in target org
- [ ] Service queries ONLY the specified `orgID` (not all user's orgs)
- [ ] Response contains correct `organization_id`, `organization_name`, and four counts
- [ ] Unit tests pass (service + handler)
- [ ] Integration tests pass (real DB)
- [ ] E2E tests pass (full gRPC stack)
- [ ] Full test suite passes
- [ ] RTM updated: FR-015 shows Complete at L1/L2/L3

---

## 8. Related Gaps (Out of Scope for This Fix)

| Gap | Tracking | Notes |
|-----|----------|-------|
| FR-012 `ListResolvedDisputes` admin-only rejection test | `specs/008-bill-split/tasks.md` T044 | Separate test task |
| FR-002 Asymmetric email failure path | `specs/008-bill-split/tasks.md` T010-T011 | Integration test for notification jobs |
| KD-1 Pagination/filters on list RPCs | Convergence C3 | Product decision needed |
| KD-2 `ADMIN_COMMENT` dead code | Convergence C4 | Remove or implement |

---

## 9. Design Decision Summary

| Convergence Finding | Original Design | This Fix |
|---------------------|-----------------|----------|
| C1: "Admin auth check missing" | Member access | ✅ Membership check (not admin) |
| C2: "Service iterates all orgs, contradicts spec requiring single org_id + admin check" | No org_id param; returns all orgs | ✅ Single org_id param; returns single org |

**Key correction**: The convergence analysis incorrectly assumed admin-only access. The original design (`grpc_api_business_logic.md`) and FR-015 both confirm this is a **member-facing drill-down** from global summary.

---

**Design Review Status**: Ready for implementation  
**Next Step**: Begin TDD cycle — proto change → handler tests → handler impl → service tests → service impl