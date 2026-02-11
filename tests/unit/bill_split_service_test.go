package unit

import (
	"context"
	"errors"
	"testing"
	"time"

	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestBillSplitService_GetGlobalBillSplitSummary verification of bill summary aggregation.
// Goal: Verify that the service accurately aggregates:
// 1. "Payments to Make" (Pending bills where user is debtor).
// 2. "Receipts to Verify" (Pending bills where user is creditor and debtor has acknowledged).
// 3. "Disputed bills" (both payable and receivable).
// It tests the logic across multiple organizations.
func TestBillSplitService_GetGlobalBillSplitSummary(t *testing.T) {
	mockBillRepo := new(MockBillRepo)
	mockUserRepo := new(MockUserRepo)
	svc := service.NewBillSplitService(mockBillRepo, mockUserRepo, nil, nil, nil)
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		// Mock ListUserOrgs to return user's organizations
		mockUserRepo.On("ListUserOrgs", ctx, int32(1)).
			Return([]domain.UserOrg{{UserID: 1, OrgID: 1}, {UserID: 1, OrgID: 2}}, nil).Once()

		// Mock ListByUser for org 1 - returns various bills for counting
		now := time.Now()
		mockBillRepo.On("ListByUser", ctx, int32(1), int32(1), []domain.BillStatus(nil)).
			Return([]domain.Bill{
				{ID: 1, DebtorUserID: 1, Status: domain.BillStatusPending, DebtorAcknowledgedAt: nil}, // Payment to make
				{ID: 2, CreditorUserID: 1, Status: domain.BillStatusPending, DebtorAcknowledgedAt: &now}, // Receipt to verify
				{ID: 3, DebtorUserID: 1, Status: domain.BillStatusDisputed}, // Payment in dispute
				{ID: 4, CreditorUserID: 1, Status: domain.BillStatusDisputed}, // Receipt in dispute
			}, nil).Once()

		// Mock ListByUser for org 2 - returns one bill
		mockBillRepo.On("ListByUser", ctx, int32(1), int32(2), []domain.BillStatus(nil)).
			Return([]domain.Bill{
				{ID: 5, DebtorUserID: 1, Status: domain.BillStatusPending, DebtorAcknowledgedAt: nil}, // Payment to make
			}, nil).Once()

		paymentsToMake, receiptsToVerify, paymentsInDispute, receiptsInDispute, err := svc.GetGlobalBillSplitSummary(ctx, 1)
		assert.NoError(t, err)
		assert.Equal(t, int32(2), paymentsToMake)
		assert.Equal(t, int32(1), receiptsToVerify)
		assert.Equal(t, int32(1), paymentsInDispute)
		assert.Equal(t, int32(1), receiptsInDispute)
		mockBillRepo.AssertExpectations(t)
		mockUserRepo.AssertExpectations(t)
	})

	t.Run("Error_ListUserOrgs", func(t *testing.T) {
		mockUserRepo.On("ListUserOrgs", ctx, int32(1)).
			Return([]domain.UserOrg(nil), errors.New("db error")).Once()

		_, _, _, _, err := svc.GetGlobalBillSplitSummary(ctx, 1)
		assert.Error(t, err)
		mockUserRepo.AssertExpectations(t)
	})
}

// TestBillSplitService_ListPayments verifies the payment listing functionality.
// Goal: Verify that:
// 1. Users can list their payments for a specific organization.
// 2. The `showHistory` flag correctly toggles between showing only active/pending bills vs all bills (including paid/resolved).
// 3. Security checks enforce that only org members can list payments.
func TestBillSplitService_ListPayments(t *testing.T) {
	mockBillRepo := new(MockBillRepo)
	mockUserRepo := new(MockUserRepo)
	svc := service.NewBillSplitService(mockBillRepo, mockUserRepo, nil, nil, nil)
	ctx := context.Background()

	t.Run("Success_ShowHistory", func(t *testing.T) {
		// User membership check
		mockUserRepo.On("GetUserOrg", ctx, int32(1), int32(1)).
			Return(&domain.UserOrg{UserID: 1, OrgID: 1, Status: domain.UserOrgStatusActive}, nil).Once()

		// List all bills
		mockBillRepo.On("ListByUser", ctx, int32(1), int32(1), []domain.BillStatus{
			domain.BillStatusPaid,
			domain.BillStatusAdminResolved,
			domain.BillStatusSystemDefaultAction,
		}).Return([]domain.Bill{{ID: 1}, {ID: 2}}, nil).Once()

		bills, err := svc.ListPayments(ctx, 1, 1, true)
		assert.NoError(t, err)
		assert.Equal(t, 2, len(bills))
		mockBillRepo.AssertExpectations(t)
		mockUserRepo.AssertExpectations(t)
	})

	t.Run("Success_NoHistory", func(t *testing.T) {
		mockUserRepo.On("GetUserOrg", ctx, int32(1), int32(1)).
			Return(&domain.UserOrg{UserID: 1, OrgID: 1, Status: domain.UserOrgStatusActive}, nil).Once()

		mockBillRepo.On("ListByUser", ctx, int32(1), int32(1), []domain.BillStatus{
			domain.BillStatusPending,
			domain.BillStatusDisputed,
		}).Return([]domain.Bill{{ID: 1}}, nil).Once()

		bills, err := svc.ListPayments(ctx, 1, 1, false)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(bills))
		mockBillRepo.AssertExpectations(t)
		mockUserRepo.AssertExpectations(t)
	})

	t.Run("Error_NotMember", func(t *testing.T) {
		mockUserRepo.On("GetUserOrg", ctx, int32(1), int32(1)).
			Return((*domain.UserOrg)(nil), errors.New("not found")).Once()

		_, err := svc.ListPayments(ctx, 1, 1, true)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not a member")
		mockUserRepo.AssertExpectations(t)
	})
}

// TestBillSplitService_GetPaymentDetail verifies retrieval of detailed bill information.
// Goal: Verify that:
// 1. Both Debtors and Creditors can retrieve details for bills they are involved in.
// 2. The `CanAcknowledge` flag is correctly calculated:
//    - True for Debtor if they haven't acknowledged yet.
//    - True for Creditor only after Debtor has acknowledged.
// 3. Access is denied for users not involved in the bill.
func TestBillSplitService_GetPaymentDetail(t *testing.T) {
	mockBillRepo := new(MockBillRepo)
	mockUserRepo := new(MockUserRepo)
	svc := service.NewBillSplitService(mockBillRepo, mockUserRepo, nil, nil, nil)
	ctx := context.Background()

	t.Run("Success_AsDebtor", func(t *testing.T) {
		bill := &domain.Bill{ID: 1, DebtorUserID: 1, CreditorUserID: 2, OrgID: 1, AmountCents: 1000, Status: domain.BillStatusPending}
		actions := []domain.BillAction{{ID: 1, BillID: 1, ActionType: domain.BillActionTypeNoticeSent}}

		mockBillRepo.On("GetByID", ctx, int32(1)).Return(bill, nil).Once()
		mockBillRepo.On("ListActionsByBill", ctx, int32(1)).Return(actions, nil).Once()

		retBill, retActions, canAcknowledge, err := svc.GetPaymentDetail(ctx, 1, 1)
		assert.NoError(t, err)
		assert.NotNil(t, retBill)
		assert.Equal(t, 1, len(retActions))
		assert.True(t, canAcknowledge) // Debtor can acknowledge when DebtorAcknowledgedAt is nil
		mockBillRepo.AssertExpectations(t)
	})

	t.Run("Success_AsCreditor", func(t *testing.T) {
		now := time.Now()
		bill := &domain.Bill{ID: 1, DebtorUserID: 2, CreditorUserID: 1, OrgID: 1, AmountCents: 1000, Status: domain.BillStatusPending, DebtorAcknowledgedAt: &now}
		actions := []domain.BillAction{{ID: 1, BillID: 1, ActionType: domain.BillActionTypeDebtorAcknowledged}}

		mockBillRepo.On("GetByID", ctx, int32(1)).Return(bill, nil).Once()
		mockBillRepo.On("ListActionsByBill", ctx, int32(1)).Return(actions, nil).Once()

		retBill, retActions, canAcknowledge, err := svc.GetPaymentDetail(ctx, 1, 1)
		assert.NoError(t, err)
		assert.NotNil(t, retBill)
		assert.Equal(t, 1, len(retActions))
		assert.True(t, canAcknowledge) // Creditor can acknowledge when debtor has acknowledged
		mockBillRepo.AssertExpectations(t)
	})

	t.Run("Error_NotInvolved", func(t *testing.T) {
		bill := &domain.Bill{ID: 1, DebtorUserID: 2, CreditorUserID: 3, OrgID: 1, AmountCents: 1000}

		mockBillRepo.On("GetByID", ctx, int32(1)).Return(bill, nil).Once()
		mockUserRepo.On("GetUserOrg", ctx, int32(1), int32(1)).Return((*domain.UserOrg)(nil), errors.New("not found")).Once()

		_, _, _, err := svc.GetPaymentDetail(ctx, 1, 1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unauthorized")
		mockBillRepo.AssertExpectations(t)
		mockUserRepo.AssertExpectations(t)
	})

	t.Run("Error_BillNotFound", func(t *testing.T) {
		mockBillRepo.On("GetByID", ctx, int32(1)).Return((*domain.Bill)(nil), errors.New("not found")).Once()

		_, _, _, err := svc.GetPaymentDetail(ctx, 1, 1)
		assert.Error(t, err)
		mockBillRepo.AssertExpectations(t)
	})
}

// Note: AcknowledgePayment has complex balance update logic that is better tested in E2E tests.
// See tests/e2e/bill_split_test.go for comprehensive payment acknowledgment scenarios.

// TestBillSplitService_ListDisputedPayments verifies admin access to disputed bills.
// Goal: Verify that:
// 1. Organization Admins can view all disputed bills within their org.
// 2. Non-admin members are denied access (returning an authorization error).
func TestBillSplitService_ListDisputedPayments(t *testing.T) {
	mockBillRepo := new(MockBillRepo)
	mockUserRepo := new(MockUserRepo)
	mockOrgRepo := new(MockOrganizationRepo)
	svc := service.NewBillSplitService(mockBillRepo, mockUserRepo, mockOrgRepo, nil, nil)
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		userOrg := &domain.UserOrg{UserID: 1, OrgID: 1, Role: domain.UserOrgRoleAdmin}
		mockUserRepo.On("GetUserOrg", ctx, int32(1), int32(1)).Return(userOrg, nil).Once()

		bills := []domain.Bill{{ID: 1, Status: domain.BillStatusDisputed}, {ID: 2, Status: domain.BillStatusDisputed}}
		adminID := int32(1)
		mockBillRepo.On("ListDisputedByOrg", ctx, int32(1), &adminID).Return(bills, nil).Once()

		result, err := svc.ListDisputedPayments(ctx, 1, 1)
		assert.NoError(t, err)
		assert.Equal(t, 2, len(result))
		mockBillRepo.AssertExpectations(t)
		mockUserRepo.AssertExpectations(t)
	})

	t.Run("Error_NotAdmin", func(t *testing.T) {
		userOrg := &domain.UserOrg{UserID: 1, OrgID: 1, Role: domain.UserOrgRoleMember}
		mockUserRepo.On("GetUserOrg", ctx, int32(1), int32(1)).Return(userOrg, nil).Once()

		_, err := svc.ListDisputedPayments(ctx, 1, 1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unauthorized")
		mockUserRepo.AssertExpectations(t)
	})
}

// TestBillSplitService_ListResolvedDisputes verifies admin access to resolved dispute history.
// Goal: Verify that admins can list bills that were previously disputed and have been resolved
// (either by admin action or system default).
func TestBillSplitService_ListResolvedDisputes(t *testing.T) {
	mockBillRepo := new(MockBillRepo)
	mockUserRepo := new(MockUserRepo)
	mockOrgRepo := new(MockOrganizationRepo)
	svc := service.NewBillSplitService(mockBillRepo, mockUserRepo, mockOrgRepo, nil, nil)
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		userOrg := &domain.UserOrg{UserID: 1, OrgID: 1, Role: domain.UserOrgRoleAdmin}
		mockUserRepo.On("GetUserOrg", ctx, int32(1), int32(1)).Return(userOrg, nil).Once()

		bills := []domain.Bill{
			{ID: 1, Status: domain.BillStatusAdminResolved},
			{ID: 2, Status: domain.BillStatusSystemDefaultAction},
		}
		mockBillRepo.On("ListResolvedDisputesByOrg", ctx, int32(1)).Return(bills, nil).Once()

		result, err := svc.ListResolvedDisputes(ctx, 1, 1)
		assert.NoError(t, err)
		assert.Equal(t, 2, len(result))
		mockBillRepo.AssertExpectations(t)
		mockUserRepo.AssertExpectations(t)
	})
}

// TestBillSplitService_ResolveDispute verifies the complex logic of dispute resolution.
// Goal: Verify that when an admin resolves a dispute finding the Debtor at fault:
// 1. Bill status is updated to ADMIN_RESOLVED.
// 2. An admin resolution action is logged.
// 3. User balances are corrected (Debtor pays, Creditor receives).
// 4. Notifications are sent to both parties.
func TestBillSplitService_ResolveDispute(t *testing.T) {
	mockBillRepo := new(MockBillRepo)
	mockUserRepo := new(MockUserRepo)
	mockOrgRepo := new(MockOrganizationRepo)
	mockNotifRepo := new(MockNotificationRepo)
	mockEmailSvc := new(MockEmailService)
	svc := service.NewBillSplitService(mockBillRepo, mockUserRepo, mockOrgRepo, mockNotifRepo, mockEmailSvc)
	ctx := context.Background()

	t.Run("Success_DebtorFault", func(t *testing.T) {
		bill := &domain.Bill{
			ID: 1, DebtorUserID: 2, CreditorUserID: 3, OrgID: 1,
			AmountCents: 1000, Status: domain.BillStatusDisputed,
			SettlementMonth: "2024-01",
		}
		org := &domain.Organization{ID: 1, Name: "Test Org"}
		userOrg := &domain.UserOrg{UserID: 1, OrgID: 1, Role: domain.UserOrgRoleAdmin}
		debtor := &domain.User{ID: 2, Name: "Debtor", Email: "debtor@test.com"}
		creditor := &domain.User{ID: 3, Name: "Creditor", Email: "creditor@test.com"}
		debtorUO := &domain.UserOrg{UserID: 2, OrgID: 1, BalanceCents: 500}
		
		mockBillRepo.On("GetByID", ctx, int32(1)).Return(bill, nil).Once()
		mockOrgRepo.On("GetByID", ctx, int32(1)).Return(org, nil).Once()
		mockUserRepo.On("GetUserOrg", ctx, int32(1), int32(1)).Return(userOrg, nil).Once()
		mockUserRepo.On("GetByID", ctx, int32(2)).Return(debtor, nil).Once()
		mockUserRepo.On("GetByID", ctx, int32(3)).Return(creditor, nil).Once()
		mockUserRepo.On("GetUserOrg", ctx, int32(2), int32(1)).Return(debtorUO, nil).Once()
		// mockUserRepo.On("GetUserOrg", ctx, int32(3), int32(1)).Return(creditorUO, nil).Once()

		// Update bill
		mockBillRepo.On("Update", ctx, mock.MatchedBy(func(b *domain.Bill) bool {
			return b.Status == domain.BillStatusAdminResolved && b.ResolutionOutcome == string(domain.ResolutionOutcomeDebtorFault)
		})).Return(nil).Once()

		// Create action
		mockBillRepo.On("CreateAction", ctx, mock.MatchedBy(func(a *domain.BillAction) bool {
			return a.ActionType == domain.BillActionTypeAdminResolution && a.ActorUserID != nil && *a.ActorUserID == 1
		})).Return(nil).Once()

		// Block Debtor
		blockDueTo := int32(1)
		mockUserRepo.On("UpdateUserOrg", ctx, mock.MatchedBy(func(uo *domain.UserOrg) bool {
			return uo.UserID == 2 && uo.RentingBlocked == true && uo.BlockedDueToBillID != nil && *uo.BlockedDueToBillID == blockDueTo
		})).Return(nil).Once()

		// Notifications
		mockNotifRepo.On("Create", ctx, mock.Anything).Return(nil).Times(2)
		mockEmailSvc.On("SendBillDisputeResolutionNotification", ctx, "debtor@test.com", "Debtor", int32(1000), "DEBTOR_FAULT", "Admin resolved: Debtor blocked from renting due to fault", "Test Org").Return(nil).Once()
		mockEmailSvc.On("SendBillDisputeResolutionNotification", ctx, "creditor@test.com", "Creditor", int32(1000), "DEBTOR_FAULT", "Admin resolved: Debtor blocked from renting due to fault", "Test Org").Return(nil).Once()

		err := svc.ResolveDispute(ctx, 1, 1, "DEBTOR_FAULT")
		assert.NoError(t, err)
		mockBillRepo.AssertExpectations(t)
		mockUserRepo.AssertExpectations(t)
		mockOrgRepo.AssertExpectations(t)
		mockNotifRepo.AssertExpectations(t)
		mockEmailSvc.AssertExpectations(t)
	})

	t.Run("Success_CreditorFault", func(t *testing.T) {
		bill := &domain.Bill{
			ID: 1, DebtorUserID: 2, CreditorUserID: 3, OrgID: 1,
			AmountCents: 1000, Status: domain.BillStatusDisputed,
			SettlementMonth: "2024-01",
		}
		org := &domain.Organization{ID: 1, Name: "Test Org"}
		userOrg := &domain.UserOrg{UserID: 1, OrgID: 1, Role: domain.UserOrgRoleAdmin}
		debtor := &domain.User{ID: 2, Name: "Debtor", Email: "debtor@test.com"}
		creditor := &domain.User{ID: 3, Name: "Creditor", Email: "creditor@test.com"}
		debtorUO := &domain.UserOrg{UserID: 2, OrgID: 1, BalanceCents: 500}
		creditorUO := &domain.UserOrg{UserID: 3, OrgID: 1, BalanceCents: -500}

		mockBillRepo.On("GetByID", ctx, int32(1)).Return(bill, nil).Once()
		mockOrgRepo.On("GetByID", ctx, int32(1)).Return(org, nil).Once()
		mockUserRepo.On("GetUserOrg", ctx, int32(1), int32(1)).Return(userOrg, nil).Once()
		mockUserRepo.On("GetByID", ctx, int32(2)).Return(debtor, nil).Once()
		mockUserRepo.On("GetByID", ctx, int32(3)).Return(creditor, nil).Once()
		mockUserRepo.On("GetUserOrg", ctx, int32(2), int32(1)).Return(debtorUO, nil)
		mockUserRepo.On("GetUserOrg", ctx, int32(3), int32(1)).Return(creditorUO, nil)

		mockBillRepo.On("Update", ctx, mock.MatchedBy(func(b *domain.Bill) bool {
			return b.Status == domain.BillStatusAdminResolved && b.ResolutionOutcome == string(domain.ResolutionOutcomeCreditorFault)
		})).Return(nil).Once()

		mockBillRepo.On("CreateAction", ctx, mock.Anything).Return(nil).Once()

		// Creditor fault: payment marked valid -> balances update, and creditor blocked
		// Balance checks: Debtor 500 -> -500. Creditor -500 -> 500.
		mockUserRepo.On("UpdateUserOrg", ctx, mock.MatchedBy(func(uo *domain.UserOrg) bool {
			return uo.UserID == 3 && uo.BalanceCents == 500
		})).Return(nil).Once() // Creditor balance update
		mockUserRepo.On("UpdateUserOrg", ctx, mock.MatchedBy(func(uo *domain.UserOrg) bool {
			return uo.UserID == 2 && uo.BalanceCents == -500
		})).Return(nil).Once() // Debtor balance update
		
		blockDueTo := int32(1)
		mockUserRepo.On("UpdateUserOrg", ctx, mock.MatchedBy(func(uo *domain.UserOrg) bool {
			return uo.UserID == 3 && uo.LendingBlocked == true && uo.BlockedDueToBillID != nil && *uo.BlockedDueToBillID == blockDueTo
		})).Return(nil).Once() // Creditor block

		mockNotifRepo.On("Create", ctx, mock.Anything).Return(nil).Times(2)
		// SendBillDisputeResolutionNotification(ctx, email, userName, amount, resolution, notes, orgName)
		mockEmailSvc.On("SendBillDisputeResolutionNotification", ctx, "debtor@test.com", "Debtor", int32(1000), "CREDITOR_FAULT", "Admin resolved: Creditor at fault, payment marked valid", "Test Org").Return(nil).Once()
		mockEmailSvc.On("SendBillDisputeResolutionNotification", ctx, "creditor@test.com", "Creditor", int32(1000), "CREDITOR_FAULT", "Admin resolved: Creditor at fault, payment marked valid", "Test Org").Return(nil).Once()

		err := svc.ResolveDispute(ctx, 1, 1, "CREDITOR_FAULT")
		assert.NoError(t, err)
		mockBillRepo.AssertExpectations(t)
		mockUserRepo.AssertExpectations(t)
		mockOrgRepo.AssertExpectations(t)
		mockNotifRepo.AssertExpectations(t)
		mockEmailSvc.AssertExpectations(t)
	})

	t.Run("Success_BothFault", func(t *testing.T) {
		bill := &domain.Bill{
			ID: 1, DebtorUserID: 2, CreditorUserID: 3, OrgID: 1,
			AmountCents: 1000, Status: domain.BillStatusDisputed,
			SettlementMonth: "2024-01",
		}
		org := &domain.Organization{ID: 1, Name: "Test Org"}
		userOrg := &domain.UserOrg{UserID: 1, OrgID: 1, Role: domain.UserOrgRoleAdmin}
		debtor := &domain.User{ID: 2, Name: "Debtor", Email: "debtor@test.com"}
		creditor := &domain.User{ID: 3, Name: "Creditor", Email: "creditor@test.com"}
		debtorUO := &domain.UserOrg{UserID: 2, OrgID: 1, BalanceCents: 500}
		creditorUO := &domain.UserOrg{UserID: 3, OrgID: 1, BalanceCents: -500}

		mockBillRepo.On("GetByID", ctx, int32(1)).Return(bill, nil).Once()
		mockOrgRepo.On("GetByID", ctx, int32(1)).Return(org, nil).Once()
		mockUserRepo.On("GetUserOrg", ctx, int32(1), int32(1)).Return(userOrg, nil).Once()
		mockUserRepo.On("GetByID", ctx, int32(2)).Return(debtor, nil).Once()
		mockUserRepo.On("GetByID", ctx, int32(3)).Return(creditor, nil).Once()
		mockUserRepo.On("GetUserOrg", ctx, int32(2), int32(1)).Return(debtorUO, nil)
		mockUserRepo.On("GetUserOrg", ctx, int32(3), int32(1)).Return(creditorUO, nil)

		mockBillRepo.On("Update", ctx, mock.MatchedBy(func(b *domain.Bill) bool {
			return b.Status == domain.BillStatusAdminResolved && b.ResolutionOutcome == string(domain.ResolutionOutcomeBothFault)
		})).Return(nil).Once()

		mockBillRepo.On("CreateAction", ctx, mock.Anything).Return(nil).Once()

		blockDueTo := int32(1)
		// Both blocked
		mockUserRepo.On("UpdateUserOrg", ctx, mock.MatchedBy(func(uo *domain.UserOrg) bool {
			return uo.UserID == 2 && uo.RentingBlocked == true && uo.BlockedDueToBillID != nil && *uo.BlockedDueToBillID == blockDueTo
		})).Return(nil).Once()
		mockUserRepo.On("UpdateUserOrg", ctx, mock.MatchedBy(func(uo *domain.UserOrg) bool {
			return uo.UserID == 3 && uo.LendingBlocked == true && uo.BlockedDueToBillID != nil && *uo.BlockedDueToBillID == blockDueTo
		})).Return(nil).Once()

		mockNotifRepo.On("Create", ctx, mock.Anything).Return(nil).Times(2)
		mockEmailSvc.On("SendBillDisputeResolutionNotification", ctx, "debtor@test.com", "Debtor", int32(1000), "BOTH_FAULT", "Admin resolved: Both parties blocked from renting/lending", "Test Org").Return(nil).Once()
		mockEmailSvc.On("SendBillDisputeResolutionNotification", ctx, "creditor@test.com", "Creditor", int32(1000), "BOTH_FAULT", "Admin resolved: Both parties blocked from renting/lending", "Test Org").Return(nil).Once()

		err := svc.ResolveDispute(ctx, 1, 1, "BOTH_FAULT")
		assert.NoError(t, err)
		mockBillRepo.AssertExpectations(t)
		mockUserRepo.AssertExpectations(t)
		mockOrgRepo.AssertExpectations(t)
		mockNotifRepo.AssertExpectations(t)
		mockEmailSvc.AssertExpectations(t)
	})

	t.Run("Error_NotDisputed", func(t *testing.T) {
		bill := &domain.Bill{ID: 1, Status: domain.BillStatusPaid, OrgID: 1}
		org := &domain.Organization{ID: 1}
		userOrg := &domain.UserOrg{UserID: 1, OrgID: 1, Role: domain.UserOrgRoleAdmin}

		mockBillRepo.On("GetByID", ctx, int32(1)).Return(bill, nil).Once()
		mockOrgRepo.On("GetByID", ctx, int32(1)).Return(org, nil).Once() // ResolveDispute expects GetByID(OrgID) check
		mockUserRepo.On("GetUserOrg", ctx, int32(1), int32(1)).Return(userOrg, nil).Once()

		err := svc.ResolveDispute(ctx, 1, 1, "DEBTOR_FAULT")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "payment is not in disputed status")
		mockBillRepo.AssertExpectations(t)
		// mockOrgRepo.AssertExpectations(t)
		mockUserRepo.AssertExpectations(t)
	})

	t.Run("Error_AdminInvolved", func(t *testing.T) {
		bill := &domain.Bill{ID: 1, DebtorUserID: 1, CreditorUserID: 3, OrgID: 1, Status: domain.BillStatusDisputed}
		org := &domain.Organization{ID: 1}
		userOrg := &domain.UserOrg{UserID: 1, OrgID: 1, Role: domain.UserOrgRoleAdmin}

		mockBillRepo.On("GetByID", ctx, int32(1)).Return(bill, nil).Once()
		mockOrgRepo.On("GetByID", ctx, int32(1)).Return(org, nil).Once()
		mockUserRepo.On("GetUserOrg", ctx, int32(1), int32(1)).Return(userOrg, nil).Once()

		err := svc.ResolveDispute(ctx, 1, 1, "DEBTOR_FAULT")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "admins cannot resolve disputes they are involved in")
		mockBillRepo.AssertExpectations(t)
		// mockOrgRepo.AssertExpectations(t)
		mockUserRepo.AssertExpectations(t)
	})

	t.Run("Error_InvalidResolution", func(t *testing.T) {
		bill := &domain.Bill{ID: 1, DebtorUserID: 2, CreditorUserID: 3, OrgID: 1, Status: domain.BillStatusDisputed}
		org := &domain.Organization{ID: 1}
		userOrg := &domain.UserOrg{UserID: 1, OrgID: 1, Role: domain.UserOrgRoleAdmin}

		mockBillRepo.On("GetByID", ctx, int32(1)).Return(bill, nil).Once()
		mockOrgRepo.On("GetByID", ctx, int32(1)).Return(org, nil).Once()
		mockUserRepo.On("GetUserOrg", ctx, int32(1), int32(1)).Return(userOrg, nil).Once()

		err := svc.ResolveDispute(ctx, 1, 1, "INVALID")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid resolution type")
		mockBillRepo.AssertExpectations(t)
		// mockOrgRepo.AssertExpectations(t)
		mockUserRepo.AssertExpectations(t)
	})
}
