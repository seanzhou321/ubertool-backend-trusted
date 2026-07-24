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
	"github.com/stretchr/testify/require"
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
				{ID: 1, DebtorUserID: 1, Status: domain.BillStatusPending, DebtorAcknowledgedAt: nil},    // Payment to make
				{ID: 2, CreditorUserID: 1, Status: domain.BillStatusPending, DebtorAcknowledgedAt: &now}, // Receipt to verify
				{ID: 3, DebtorUserID: 1, Status: domain.BillStatusDisputed},                              // Payment in dispute
				{ID: 4, CreditorUserID: 1, Status: domain.BillStatusDisputed},                            // Receipt in dispute
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
//   - True for Debtor if they haven't acknowledged yet.
//   - True for Creditor only after Debtor has acknowledged.
//
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

	// FR-011: an org admin who is NOT a party to the bill is a third authorized-caller class
	// distinct from debtor/creditor — previously untested (only debtor/creditor success and a
	// non-member/non-admin rejection were covered).
	t.Run("Success_AsOrgAdmin_NotAParty", func(t *testing.T) {
		bill := &domain.Bill{ID: 2, DebtorUserID: 20, CreditorUserID: 21, OrgID: 1, AmountCents: 1000, Status: domain.BillStatusPending}
		actions := []domain.BillAction{{ID: 1, BillID: 2, ActionType: domain.BillActionTypeNoticeSent}}

		mockBillRepo.On("GetByID", ctx, int32(2)).Return(bill, nil).Once()
		mockUserRepo.On("GetUserOrg", ctx, int32(99), int32(1)).Return(&domain.UserOrg{UserID: 99, OrgID: 1, Role: domain.UserOrgRoleAdmin}, nil).Once()
		mockBillRepo.On("ListActionsByBill", ctx, int32(2)).Return(actions, nil).Once()

		retBill, retActions, canAcknowledge, err := svc.GetPaymentDetail(ctx, 99, 2)
		require.NoError(t, err)
		assert.NotNil(t, retBill)
		assert.Equal(t, 1, len(retActions))
		assert.False(t, canAcknowledge, "an admin who is not a party can never acknowledge")
		mockBillRepo.AssertExpectations(t)
		mockUserRepo.AssertExpectations(t)
	})

	// FR-011's can_acknowledge=false branches — only the two `true` cases were ever asserted.
	t.Run("CanAcknowledge_False_DebtorAlreadyAcknowledged", func(t *testing.T) {
		now := time.Now()
		bill := &domain.Bill{ID: 3, DebtorUserID: 30, CreditorUserID: 31, OrgID: 1, AmountCents: 1000, Status: domain.BillStatusPending, DebtorAcknowledgedAt: &now}
		mockBillRepo.On("GetByID", ctx, int32(3)).Return(bill, nil).Once()
		mockBillRepo.On("ListActionsByBill", ctx, int32(3)).Return([]domain.BillAction{}, nil).Once()

		_, _, canAcknowledge, err := svc.GetPaymentDetail(ctx, 30, 3)
		require.NoError(t, err)
		assert.False(t, canAcknowledge, "a debtor who already acknowledged cannot acknowledge again")
	})

	t.Run("CanAcknowledge_False_CreditorBeforeDebtorAcknowledges", func(t *testing.T) {
		bill := &domain.Bill{ID: 4, DebtorUserID: 40, CreditorUserID: 41, OrgID: 1, AmountCents: 1000, Status: domain.BillStatusPending}
		mockBillRepo.On("GetByID", ctx, int32(4)).Return(bill, nil).Once()
		mockBillRepo.On("ListActionsByBill", ctx, int32(4)).Return([]domain.BillAction{}, nil).Once()

		_, _, canAcknowledge, err := svc.GetPaymentDetail(ctx, 41, 4)
		require.NoError(t, err)
		assert.False(t, canAcknowledge, "the creditor cannot acknowledge before the debtor has")
	})

	t.Run("CanAcknowledge_False_WrongStatus", func(t *testing.T) {
		bill := &domain.Bill{ID: 5, DebtorUserID: 50, CreditorUserID: 51, OrgID: 1, AmountCents: 1000, Status: domain.BillStatusPaid}
		mockBillRepo.On("GetByID", ctx, int32(5)).Return(bill, nil).Once()
		mockBillRepo.On("ListActionsByBill", ctx, int32(5)).Return([]domain.BillAction{}, nil).Once()

		_, _, canAcknowledge, err := svc.GetPaymentDetail(ctx, 50, 5)
		require.NoError(t, err)
		assert.False(t, canAcknowledge, "a PAID bill can no longer be acknowledged by either party")
	})
}

// TestBillSplitService_AcknowledgePayment covers FR-006 and FR-009's GRACEFUL outcome
// (specs/008-bill-split): AcknowledgePayment must accept debtor/creditor calls while status is
// PENDING or DISPUTED, reject any other status, reject double-acknowledgment, and reject
// creditor-before-debtor. On the creditor's acknowledgment (completing the pair), the bill must
// resolve to PAID with resolution_outcome=GRACEFUL and the balance transfer applied. Previously
// deferred entirely to e2e (see git history) — replaced here because the e2e/integration
// coverage that was supposed to fulfill that deferral only ever exercised the happy path.
func TestBillSplitService_AcknowledgePayment(t *testing.T) {
	const debtorID = int32(1)
	const creditorID = int32(2)
	const billID = int32(50)
	const orgID = int32(9)
	const amountCents = int32(2500)

	newSvc := func() (service.BillSplitService, *MockBillRepo, *MockUserRepo) {
		billRepo := new(MockBillRepo)
		userRepo := new(MockUserRepo)
		orgRepo := new(MockOrganizationRepo)
		orgRepo.On("GetByID", mock.Anything, mock.Anything).Return(&domain.Organization{ID: orgID, Name: "Test Org"}, nil).Maybe()
		emailSvc := new(MockEmailService)
		emailSvc.On("SendBillPaymentAcknowledgment", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		emailSvc.On("SendBillReceiptConfirmation", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		noteRepo := new(MockNotificationRepo)
		noteRepo.On("Dispatch", mock.Anything, mock.Anything).Return(nil).Maybe()
		svc := service.NewBillSplitService(billRepo, userRepo, orgRepo, noteRepo, emailSvc)
		return svc, billRepo, userRepo
	}

	t.Run("Debtor acknowledgment succeeds from PENDING", func(t *testing.T) {
		svc, billRepo, userRepo := newSvc()
		ctx := context.Background()
		bill := &domain.Bill{ID: billID, OrgID: orgID, DebtorUserID: debtorID, CreditorUserID: creditorID, AmountCents: amountCents, Status: domain.BillStatusPending}
		billRepo.On("GetByID", ctx, billID).Return(bill, nil)
		userRepo.On("GetByID", ctx, debtorID).Return(&domain.User{ID: debtorID, Name: "Debtor"}, nil)
		userRepo.On("GetByID", ctx, creditorID).Return(&domain.User{ID: creditorID, Name: "Creditor", Email: "creditor@test.com"}, nil)
		billRepo.On("Update", ctx, mock.MatchedBy(func(b *domain.Bill) bool { return b.DebtorAcknowledgedAt != nil })).Return(nil)
		billRepo.On("CreateAction", ctx, mock.Anything).Return(nil)

		err := svc.AcknowledgePayment(ctx, debtorID, billID)
		require.NoError(t, err)
	})

	t.Run("Creditor acknowledgment after debtor resolves to PAID/GRACEFUL with balance transfer", func(t *testing.T) {
		svc, billRepo, userRepo := newSvc()
		ctx := context.Background()
		debtorAckAt := time.Now().Add(-1 * time.Hour)
		bill := &domain.Bill{ID: billID, OrgID: orgID, DebtorUserID: debtorID, CreditorUserID: creditorID, AmountCents: amountCents, Status: domain.BillStatusPending, DebtorAcknowledgedAt: &debtorAckAt}
		billRepo.On("GetByID", ctx, billID).Return(bill, nil)
		userRepo.On("GetByID", ctx, creditorID).Return(&domain.User{ID: creditorID, Name: "Creditor"}, nil)
		userRepo.On("GetByID", ctx, debtorID).Return(&domain.User{ID: debtorID, Name: "Debtor", Email: "debtor@test.com"}, nil)

		var updatedBill *domain.Bill
		billRepo.On("Update", ctx, mock.MatchedBy(func(b *domain.Bill) bool { return true })).
			Run(func(args mock.Arguments) { updatedBill = args.Get(1).(*domain.Bill) }).Return(nil)
		billRepo.On("CreateAction", ctx, mock.Anything).Return(nil)

		userRepo.On("AdjustBalance", ctx, creditorID, orgID, amountCents).Return(nil)
		userRepo.On("AdjustBalance", ctx, debtorID, orgID, -amountCents).Return(nil)

		err := svc.AcknowledgePayment(ctx, creditorID, billID)
		require.NoError(t, err)
		require.NotNil(t, updatedBill)
		assert.Equal(t, domain.BillStatusPaid, updatedBill.Status)
		assert.Equal(t, string(domain.ResolutionOutcomeGraceful), updatedBill.ResolutionOutcome, "FR-009: completing the pair must resolve to GRACEFUL")
		assert.NotNil(t, updatedBill.ResolvedAt)
		userRepo.AssertExpectations(t)
	})

	// FR-007: the DISPUTED-origin graceful-resolution path (both parties acknowledge after the
	// bill was auto-disputed) must resolve identically to the PENDING-origin path: status -> PAID,
	// resolution_outcome -> GRACEFUL, balance transfer applied, and the debtor notified.
	t.Run("Creditor acknowledgment succeeds from DISPUTED (not just PENDING)", func(t *testing.T) {
		billRepo := new(MockBillRepo)
		userRepo := new(MockUserRepo)
		orgRepo := new(MockOrganizationRepo)
		orgRepo.On("GetByID", mock.Anything, mock.Anything).Return(&domain.Organization{ID: orgID, Name: "Test Org"}, nil).Maybe()
		emailSvc := new(MockEmailService)
		noteRepo := new(MockNotificationRepo)
		svc := service.NewBillSplitService(billRepo, userRepo, orgRepo, noteRepo, emailSvc)
		ctx := context.Background()

		debtorAckAt := time.Now().Add(-1 * time.Hour)
		bill := &domain.Bill{ID: billID, OrgID: orgID, DebtorUserID: debtorID, CreditorUserID: creditorID, AmountCents: amountCents, Status: domain.BillStatusDisputed, DebtorAcknowledgedAt: &debtorAckAt}
		billRepo.On("GetByID", ctx, billID).Return(bill, nil)
		userRepo.On("GetByID", ctx, creditorID).Return(&domain.User{ID: creditorID, Name: "Creditor"}, nil)
		userRepo.On("GetByID", ctx, debtorID).Return(&domain.User{ID: debtorID, Name: "Debtor", Email: "debtor@test.com"}, nil)

		var updatedBill *domain.Bill
		billRepo.On("Update", ctx, mock.Anything).
			Run(func(args mock.Arguments) { updatedBill = args.Get(1).(*domain.Bill) }).Return(nil)
		billRepo.On("CreateAction", ctx, mock.Anything).Return(nil)
		userRepo.On("AdjustBalance", ctx, creditorID, orgID, amountCents).Return(nil)
		userRepo.On("AdjustBalance", ctx, debtorID, orgID, -amountCents).Return(nil)
		noteRepo.On("Dispatch", ctx, mock.MatchedBy(func(n *domain.Notification) bool { return n.UserID == debtorID })).Return(nil)
		emailSvc.On("SendBillReceiptConfirmation", ctx, "debtor@test.com", "Debtor", "Creditor", amountCents, mock.Anything, "Test Org").Return(nil)

		err := svc.AcknowledgePayment(ctx, creditorID, billID)
		require.NoError(t, err, "DISPUTED-origin acknowledgment must resolve the same as PENDING-origin (FR-007)")
		require.NotNil(t, updatedBill)
		assert.Equal(t, domain.BillStatusPaid, updatedBill.Status)
		assert.Equal(t, string(domain.ResolutionOutcomeGraceful), updatedBill.ResolutionOutcome)
		assert.NotNil(t, updatedBill.ResolvedAt)
		userRepo.AssertExpectations(t)
		noteRepo.AssertCalled(t, "Dispatch", ctx, mock.MatchedBy(func(n *domain.Notification) bool { return n.UserID == debtorID }))
		emailSvc.AssertCalled(t, "SendBillReceiptConfirmation", ctx, "debtor@test.com", "Debtor", "Creditor", amountCents, mock.Anything, "Test Org")
	})

	t.Run("Rejects a caller uninvolved in the bill", func(t *testing.T) {
		svc, billRepo, userRepo := newSvc()
		ctx := context.Background()
		bill := &domain.Bill{ID: billID, OrgID: orgID, DebtorUserID: debtorID, CreditorUserID: creditorID, Status: domain.BillStatusPending}
		billRepo.On("GetByID", ctx, billID).Return(bill, nil)
		userRepo.On("GetByID", ctx, int32(999)).Return(&domain.User{ID: 999}, nil)

		err := svc.AcknowledgePayment(ctx, int32(999), billID)
		require.Error(t, err)
		billRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
	})

	t.Run("Rejects a status other than PENDING/DISPUTED", func(t *testing.T) {
		svc, billRepo, userRepo := newSvc()
		ctx := context.Background()
		bill := &domain.Bill{ID: billID, OrgID: orgID, DebtorUserID: debtorID, CreditorUserID: creditorID, Status: domain.BillStatusPaid}
		billRepo.On("GetByID", ctx, billID).Return(bill, nil)
		userRepo.On("GetByID", ctx, debtorID).Return(&domain.User{ID: debtorID}, nil)

		err := svc.AcknowledgePayment(ctx, debtorID, billID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not in pending or disputed status")
		billRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
	})

	t.Run("Rejects double-acknowledgment by the debtor", func(t *testing.T) {
		svc, billRepo, userRepo := newSvc()
		ctx := context.Background()
		alreadyAckedAt := time.Now().Add(-1 * time.Hour)
		bill := &domain.Bill{ID: billID, OrgID: orgID, DebtorUserID: debtorID, CreditorUserID: creditorID, Status: domain.BillStatusPending, DebtorAcknowledgedAt: &alreadyAckedAt}
		billRepo.On("GetByID", ctx, billID).Return(bill, nil)
		userRepo.On("GetByID", ctx, debtorID).Return(&domain.User{ID: debtorID}, nil)

		err := svc.AcknowledgePayment(ctx, debtorID, billID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already acknowledged by debtor")
		billRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
	})

	t.Run("Rejects double-acknowledgment by the creditor", func(t *testing.T) {
		svc, billRepo, userRepo := newSvc()
		ctx := context.Background()
		debtorAckAt := time.Now().Add(-2 * time.Hour)
		creditorAckAt := time.Now().Add(-1 * time.Hour)
		bill := &domain.Bill{ID: billID, OrgID: orgID, DebtorUserID: debtorID, CreditorUserID: creditorID, Status: domain.BillStatusPending, DebtorAcknowledgedAt: &debtorAckAt, CreditorAcknowledgedAt: &creditorAckAt}
		billRepo.On("GetByID", ctx, billID).Return(bill, nil)
		userRepo.On("GetByID", ctx, creditorID).Return(&domain.User{ID: creditorID}, nil)

		err := svc.AcknowledgePayment(ctx, creditorID, billID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already acknowledged by creditor")
		billRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
	})

	t.Run("Rejects the creditor acknowledging before the debtor", func(t *testing.T) {
		svc, billRepo, userRepo := newSvc()
		ctx := context.Background()
		bill := &domain.Bill{ID: billID, OrgID: orgID, DebtorUserID: debtorID, CreditorUserID: creditorID, Status: domain.BillStatusPending}
		billRepo.On("GetByID", ctx, billID).Return(bill, nil)
		userRepo.On("GetByID", ctx, creditorID).Return(&domain.User{ID: creditorID}, nil)

		err := svc.AcknowledgePayment(ctx, creditorID, billID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "debtor has not acknowledged")
		billRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
	})
}

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
	// Helper to setup fresh mocks for each subtest
	setup := func() (service.BillSplitService, *MockBillRepo, *MockUserRepo, *MockOrganizationRepo, *MockNotificationRepo, *MockEmailService) {
		mockBillRepo := new(MockBillRepo)
		mockUserRepo := new(MockUserRepo)
		mockOrgRepo := new(MockOrganizationRepo)
		mockNotifRepo := new(MockNotificationRepo)
		mockEmailSvc := new(MockEmailService)
		svc := service.NewBillSplitService(mockBillRepo, mockUserRepo, mockOrgRepo, mockNotifRepo, mockEmailSvc)
		return svc, mockBillRepo, mockUserRepo, mockOrgRepo, mockNotifRepo, mockEmailSvc
	}
	ctx := context.Background()

	t.Run("Success_DebtorFault", func(t *testing.T) {
		svc, mockBillRepo, mockUserRepo, mockOrgRepo, mockNotifRepo, mockEmailSvc := setup()

		bill := &domain.Bill{
			ID: 1, DebtorUserID: 2, CreditorUserID: 3, OrgID: 1,
			AmountCents: 1000, Status: domain.BillStatusDisputed,
			SettlementMonth: "2024-01",
		}
		org := &domain.Organization{ID: 1, Name: "Test Org"}
		userOrg := &domain.UserOrg{UserID: 1, OrgID: 1, Role: domain.UserOrgRoleAdmin}
		debtor := &domain.User{ID: 2, Name: "Debtor", Email: "debtor@test.com"}
		creditor := &domain.User{ID: 3, Name: "Creditor", Email: "creditor@test.com"}

		mockBillRepo.On("GetByID", ctx, int32(1)).Return(bill, nil).Once()
		mockOrgRepo.On("GetByID", ctx, int32(1)).Return(org, nil).Once()
		mockUserRepo.On("GetUserOrg", ctx, int32(1), int32(1)).Return(userOrg, nil).Once()
		mockUserRepo.On("GetByID", ctx, int32(2)).Return(debtor, nil).Once()
		mockUserRepo.On("GetByID", ctx, int32(3)).Return(creditor, nil).Once()

		// updateBalances expectations (enforce payment) — atomic deltas, not read-modify-write
		// (SEC-BILL-004/006, sbr/rtm/009-security.rtm.md).
		mockUserRepo.On("AdjustBalance", ctx, int32(3), int32(1), int32(1000)).Return(nil).Once() // Creditor credited
		mockUserRepo.On("AdjustBalance", ctx, int32(2), int32(1), int32(-1000)).Return(nil).Once() // Debtor debited

		// Update bill
		mockBillRepo.On("Update", ctx, mock.MatchedBy(func(b *domain.Bill) bool {
			return b.Status == domain.BillStatusAdminResolved && b.ResolutionOutcome == string(domain.ResolutionOutcomeDebtorFault)
		})).Return(nil).Once()

		// Create action
		mockBillRepo.On("CreateAction", ctx, mock.MatchedBy(func(a *domain.BillAction) bool {
			return a.ActionType == domain.BillActionTypeAdminResolution && a.ActorUserID != nil && *a.ActorUserID == 1
		})).Return(nil).Once()

		// Block Debtor — column-scoped, does not touch balance_cents. Reason is the fixed
		// string ResolveDispute passes for this branch (internal/service/bill_split.go:337),
		// not the caller-supplied notes.
		mockUserRepo.On("SetRentingBlocked", ctx, int32(2), int32(1), true, "Blocked due to unresolved payment dispute (debtor at fault)", int32(1)).Return(nil).Once()

		// Notifications
		mockNotifRepo.On("Dispatch", ctx, mock.Anything).Return(nil).Times(2)
		mockEmailSvc.On("SendBillDisputeResolutionNotification", ctx, "debtor@test.com", "Debtor", int32(1000), "DEBTOR_FAULT", "Admin resolved: Debtor blocked from renting due to fault", "Test Org").Return(nil).Once()
		mockEmailSvc.On("SendBillDisputeResolutionNotification", ctx, "creditor@test.com", "Creditor", int32(1000), "DEBTOR_FAULT", "Admin resolved: Debtor blocked from renting due to fault", "Test Org").Return(nil).Once()

		err := svc.ResolveDispute(ctx, 1, 1, "DEBTOR_FAULT", "Admin resolved: Debtor blocked from renting due to fault")
		assert.NoError(t, err)
		mockBillRepo.AssertExpectations(t)
		mockUserRepo.AssertExpectations(t)
		mockOrgRepo.AssertExpectations(t)
		mockNotifRepo.AssertExpectations(t)
		mockEmailSvc.AssertExpectations(t)
	})

	t.Run("Success_CreditorFault", func(t *testing.T) {
		svc, mockBillRepo, mockUserRepo, mockOrgRepo, mockNotifRepo, mockEmailSvc := setup()

		bill := &domain.Bill{
			ID: 1, DebtorUserID: 2, CreditorUserID: 3, OrgID: 1,
			AmountCents: 1000, Status: domain.BillStatusDisputed,
			SettlementMonth: "2024-01",
		}
		org := &domain.Organization{ID: 1, Name: "Test Org"}
		userOrg := &domain.UserOrg{UserID: 1, OrgID: 1, Role: domain.UserOrgRoleAdmin}
		debtor := &domain.User{ID: 2, Name: "Debtor", Email: "debtor@test.com"}
		creditor := &domain.User{ID: 3, Name: "Creditor", Email: "creditor@test.com"}

		mockBillRepo.On("GetByID", ctx, int32(1)).Return(bill, nil).Once()
		mockOrgRepo.On("GetByID", ctx, int32(1)).Return(org, nil).Once()
		mockUserRepo.On("GetUserOrg", ctx, int32(1), int32(1)).Return(userOrg, nil).Once()
		mockUserRepo.On("GetByID", ctx, int32(2)).Return(debtor, nil).Once()
		mockUserRepo.On("GetByID", ctx, int32(3)).Return(creditor, nil).Once()

		mockBillRepo.On("Update", ctx, mock.MatchedBy(func(b *domain.Bill) bool {
			return b.Status == domain.BillStatusAdminResolved && b.ResolutionOutcome == string(domain.ResolutionOutcomeCreditorFault)
		})).Return(nil).Once()

		mockBillRepo.On("CreateAction", ctx, mock.Anything).Return(nil).Once()

		// Creditor at fault: penalty applied (atomic -1000 delta) and lending blocked
		// (column-scoped, does not touch balance_cents) — SEC-BILL-004/006. Reason is the fixed
		// string ResolveDispute passes for this branch (internal/service/bill_split.go:340).
		mockUserRepo.On("AdjustBalance", ctx, int32(3), int32(1), int32(-1000)).Return(nil).Once()
		mockUserRepo.On("SetLendingBlocked", ctx, int32(3), int32(1), true, "Blocked due to dispute resolution (creditor at fault)", int32(1)).Return(nil).Once()

		mockNotifRepo.On("Dispatch", ctx, mock.Anything).Return(nil).Times(2)
		mockEmailSvc.On("SendBillDisputeResolutionNotification", ctx, "debtor@test.com", "Debtor", int32(1000), "CREDITOR_FAULT", "Admin resolved: Creditor at fault, payment marked valid", "Test Org").Return(nil).Once()
		mockEmailSvc.On("SendBillDisputeResolutionNotification", ctx, "creditor@test.com", "Creditor", int32(1000), "CREDITOR_FAULT", "Admin resolved: Creditor at fault, payment marked valid", "Test Org").Return(nil).Once()

		err := svc.ResolveDispute(ctx, 1, 1, "CREDITOR_FAULT", "Admin resolved: Creditor at fault, payment marked valid")
		assert.NoError(t, err)
		mockBillRepo.AssertExpectations(t)
		mockUserRepo.AssertExpectations(t)
		mockOrgRepo.AssertExpectations(t)
		mockNotifRepo.AssertExpectations(t)
		mockEmailSvc.AssertExpectations(t)
	})

	t.Run("Success_BothFault", func(t *testing.T) {
		svc, mockBillRepo, mockUserRepo, mockOrgRepo, mockNotifRepo, mockEmailSvc := setup()

		bill := &domain.Bill{
			ID: 1, DebtorUserID: 2, CreditorUserID: 3, OrgID: 1,
			AmountCents: 1000, Status: domain.BillStatusDisputed,
			SettlementMonth: "2024-01",
		}
		org := &domain.Organization{ID: 1, Name: "Test Org"}
		userOrg := &domain.UserOrg{UserID: 1, OrgID: 1, Role: domain.UserOrgRoleAdmin}
		debtor := &domain.User{ID: 2, Name: "Debtor", Email: "debtor@test.com"}
		creditor := &domain.User{ID: 3, Name: "Creditor", Email: "creditor@test.com"}

		mockBillRepo.On("GetByID", ctx, int32(1)).Return(bill, nil).Once()
		mockOrgRepo.On("GetByID", ctx, int32(1)).Return(org, nil).Once()
		mockUserRepo.On("GetUserOrg", ctx, int32(1), int32(1)).Return(userOrg, nil).Once()
		mockUserRepo.On("GetByID", ctx, int32(2)).Return(debtor, nil).Once()
		mockUserRepo.On("GetByID", ctx, int32(3)).Return(creditor, nil).Once()

		mockBillRepo.On("Update", ctx, mock.MatchedBy(func(b *domain.Bill) bool {
			return b.Status == domain.BillStatusAdminResolved && b.ResolutionOutcome == string(domain.ResolutionOutcomeBothFault)
		})).Return(nil).Once()

		mockBillRepo.On("CreateAction", ctx, mock.Anything).Return(nil).Once()

		// Both blocked and penalized — atomic deltas + column-scoped blocking writes
		// (SEC-BILL-004/006). Reason is the fixed string ResolveDispute passes for this branch
		// (internal/service/bill_split.go:343-344).
		mockUserRepo.On("AdjustBalance", ctx, int32(2), int32(1), int32(-1000)).Return(nil).Once() // Debtor penalty
		mockUserRepo.On("SetRentingBlocked", ctx, int32(2), int32(1), true, "Blocked due to unresolved payment dispute (both at fault)", int32(1)).Return(nil).Once()
		mockUserRepo.On("AdjustBalance", ctx, int32(3), int32(1), int32(-1000)).Return(nil).Once() // Creditor penalty
		mockUserRepo.On("SetLendingBlocked", ctx, int32(3), int32(1), true, "Blocked due to unresolved payment dispute (both at fault)", int32(1)).Return(nil).Once()

		mockNotifRepo.On("Dispatch", ctx, mock.Anything).Return(nil).Times(2)
		mockEmailSvc.On("SendBillDisputeResolutionNotification", ctx, "debtor@test.com", "Debtor", int32(1000), "BOTH_FAULT", "Admin resolved: Both parties blocked from renting/lending", "Test Org").Return(nil).Once()
		mockEmailSvc.On("SendBillDisputeResolutionNotification", ctx, "creditor@test.com", "Creditor", int32(1000), "BOTH_FAULT", "Admin resolved: Both parties blocked from renting/lending", "Test Org").Return(nil).Once()

		err := svc.ResolveDispute(ctx, 1, 1, "BOTH_FAULT", "Admin resolved: Both parties blocked from renting/lending")
		assert.NoError(t, err)
		mockBillRepo.AssertExpectations(t)
		mockUserRepo.AssertExpectations(t)
		mockOrgRepo.AssertExpectations(t)
		mockNotifRepo.AssertExpectations(t)
		mockEmailSvc.AssertExpectations(t)
	})

	// TestBillSplitService_ResolveDispute/Success_Graceful covers FR-009 (specs/008-bill-split):
	// GRACEFUL is one of exactly four resolution outcomes SC-001 requires to be tested — it was
	// previously the only one of the four with zero coverage. Unlike AcknowledgePayment's own
	// GRACEFUL path (mutual acknowledgment, resolves to PAID), ResolveDispute's GRACEFUL
	// resolution is admin-driven: it transfers the balance in full with no penalty and resolves
	// to ADMIN_RESOLVED, not PAID — a distinct code path that needs its own test.
	t.Run("Success_Graceful", func(t *testing.T) {
		svc, mockBillRepo, mockUserRepo, mockOrgRepo, mockNotifRepo, mockEmailSvc := setup()

		bill := &domain.Bill{
			ID: 1, DebtorUserID: 2, CreditorUserID: 3, OrgID: 1,
			AmountCents: 1000, Status: domain.BillStatusDisputed,
			SettlementMonth: "2024-01",
		}
		org := &domain.Organization{ID: 1, Name: "Test Org"}
		userOrg := &domain.UserOrg{UserID: 1, OrgID: 1, Role: domain.UserOrgRoleAdmin}
		debtor := &domain.User{ID: 2, Name: "Debtor", Email: "debtor@test.com"}
		creditor := &domain.User{ID: 3, Name: "Creditor", Email: "creditor@test.com"}

		mockBillRepo.On("GetByID", ctx, int32(1)).Return(bill, nil).Once()
		mockOrgRepo.On("GetByID", ctx, int32(1)).Return(org, nil).Once()
		mockUserRepo.On("GetUserOrg", ctx, int32(1), int32(1)).Return(userOrg, nil).Once()
		mockUserRepo.On("GetByID", ctx, int32(2)).Return(debtor, nil).Once()
		mockUserRepo.On("GetByID", ctx, int32(3)).Return(creditor, nil).Once()

		// GRACEFUL only transfers the balance (atomic deltas) — no blocking, no penalty.
		mockUserRepo.On("AdjustBalance", ctx, int32(3), int32(1), int32(1000)).Return(nil).Once()  // Creditor credited
		mockUserRepo.On("AdjustBalance", ctx, int32(2), int32(1), int32(-1000)).Return(nil).Once() // Debtor debited

		mockBillRepo.On("Update", ctx, mock.MatchedBy(func(b *domain.Bill) bool {
			return b.Status == domain.BillStatusAdminResolved && b.ResolutionOutcome == string(domain.ResolutionOutcomeGraceful)
		})).Return(nil).Once()
		mockBillRepo.On("CreateAction", ctx, mock.Anything).Return(nil).Once()

		mockNotifRepo.On("Dispatch", ctx, mock.Anything).Return(nil).Times(2)
		mockEmailSvc.On("SendBillDisputeResolutionNotification", ctx, "debtor@test.com", "Debtor", int32(1000), "GRACEFUL", "Admin resolved gracefully, no fault found", "Test Org").Return(nil).Once()
		mockEmailSvc.On("SendBillDisputeResolutionNotification", ctx, "creditor@test.com", "Creditor", int32(1000), "GRACEFUL", "Admin resolved gracefully, no fault found", "Test Org").Return(nil).Once()

		err := svc.ResolveDispute(ctx, 1, 1, "GRACEFUL", "Admin resolved gracefully, no fault found")
		assert.NoError(t, err)
		mockBillRepo.AssertExpectations(t)
		mockUserRepo.AssertExpectations(t)
		mockOrgRepo.AssertExpectations(t)
		mockNotifRepo.AssertExpectations(t)
		mockEmailSvc.AssertExpectations(t)
	})

	t.Run("Error_NotDisputed", func(t *testing.T) {
		svc, mockBillRepo, mockUserRepo, mockOrgRepo, _, _ := setup()
		
		bill := &domain.Bill{ID: 1, Status: domain.BillStatusPaid, OrgID: 1}
		org := &domain.Organization{ID: 1}
		userOrg := &domain.UserOrg{UserID: 1, OrgID: 1, Role: domain.UserOrgRoleAdmin}

		mockBillRepo.On("GetByID", ctx, int32(1)).Return(bill, nil).Once()
		mockOrgRepo.On("GetByID", ctx, int32(1)).Return(org, nil).Once()
		mockUserRepo.On("GetUserOrg", ctx, int32(1), int32(1)).Return(userOrg, nil).Once()

		err := svc.ResolveDispute(ctx, 1, 1, "DEBTOR_FAULT", "Test notes")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "payment is not in disputed status")
		mockBillRepo.AssertExpectations(t)
		mockUserRepo.AssertExpectations(t)
	})

	t.Run("Error_AdminInvolved", func(t *testing.T) {
		svc, mockBillRepo, mockUserRepo, mockOrgRepo, _, _ := setup()

		bill := &domain.Bill{ID: 1, DebtorUserID: 1, CreditorUserID: 3, OrgID: 1, Status: domain.BillStatusDisputed}
		org := &domain.Organization{ID: 1}
		userOrg := &domain.UserOrg{UserID: 1, OrgID: 1, Role: domain.UserOrgRoleAdmin}

		mockBillRepo.On("GetByID", ctx, int32(1)).Return(bill, nil).Once()
		mockOrgRepo.On("GetByID", ctx, int32(1)).Return(org, nil).Once()
		mockUserRepo.On("GetUserOrg", ctx, int32(1), int32(1)).Return(userOrg, nil).Once()

		err := svc.ResolveDispute(ctx, 1, 1, "DEBTOR_FAULT", "Test notes")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "admins cannot resolve disputes they are involved in")
		mockBillRepo.AssertExpectations(t)
		mockUserRepo.AssertExpectations(t)
	})

	t.Run("Error_InvalidResolution", func(t *testing.T) {
		svc, mockBillRepo, mockUserRepo, mockOrgRepo, _, _ := setup()

		bill := &domain.Bill{ID: 1, DebtorUserID: 2, CreditorUserID: 3, OrgID: 1, Status: domain.BillStatusDisputed}
		org := &domain.Organization{ID: 1}
		userOrg := &domain.UserOrg{UserID: 1, OrgID: 1, Role: domain.UserOrgRoleAdmin}

		mockBillRepo.On("GetByID", ctx, int32(1)).Return(bill, nil).Once()
		mockOrgRepo.On("GetByID", ctx, int32(1)).Return(org, nil).Once()
		mockUserRepo.On("GetUserOrg", ctx, int32(1), int32(1)).Return(userOrg, nil).Once()

		err := svc.ResolveDispute(ctx, 1, 1, "INVALID", "Test notes")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid resolution type")
		mockBillRepo.AssertExpectations(t)
		mockUserRepo.AssertExpectations(t)
	})
}
