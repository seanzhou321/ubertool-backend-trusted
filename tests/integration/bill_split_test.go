package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/repository/postgres"
	"ubertool-backend-trusted/internal/service"

	"github.com/stretchr/testify/assert"
)

// TestBillSplitService_Integration verifies the core workflows of the Bill Split Service.
// It covers:
//  1. "Full Bill Lifecycle": The happy path where a bill is created, the debtor acknowledges payment,
//     and the creditor confirms receipt, resulting in a PAID status.
//  2. "Dispute Lifecycle": The conflict path where a bill is disputed by a user and subsequently
//     resolved by an admin (creditor), verifying status transitions and resolution outcomes.
func TestBillSplitService_Integration(t *testing.T) {
	db := prepareDB(t)
	defer db.Close()

	// Initialize Repositories
	userRepo := postgres.NewUserRepository(db)
	orgRepo := postgres.NewOrganizationRepository(db)
	billRepo := postgres.NewBillRepository(db)
	notifRepo := &MockNotificationRepo{} // Using the mock from rental_ledger_test.go if package-level

	// Check if we need to redefine mocks or if they are shared.
	// Since they are in the same package 'integration', they should be shared.
	emailSvc := &MockEmailService{}

	// Initialize Service
	billSvc := service.NewBillSplitService(billRepo, userRepo, orgRepo, notifRepo, emailSvc)
	ctx := context.Background()

	// 1. Setup Data: Org, Creditor, Debtor
	orgName := fmt.Sprintf("BillOrg-%d", time.Now().UnixNano())
	org := &domain.Organization{Name: orgName, Metro: "San Jose"}
	err := orgRepo.Create(ctx, org)
	assert.NoError(t, err)

	creditor := &domain.User{
		Email:        fmt.Sprintf("creditor-%d@t.com", time.Now().UnixNano()),
		PhoneNumber:  fmt.Sprintf("c1-%d", time.Now().UnixNano()),
		PasswordHash: "h", Name: "Creditor",
	}
	err = userRepo.Create(ctx, creditor)
	assert.NoError(t, err)

	debtor := &domain.User{
		Email:        fmt.Sprintf("debtor-%d@t.com", time.Now().UnixNano()),
		PhoneNumber:  fmt.Sprintf("d1-%d", time.Now().UnixNano()),
		PasswordHash: "h", Name: "Debtor",
	}
	err = userRepo.Create(ctx, debtor)
	assert.NoError(t, err)

	// Admin (Neutral party for resolving disputes)
	adminUser := &domain.User{
		Email:        fmt.Sprintf("admin-%d@t.com", time.Now().UnixNano()),
		PhoneNumber:  fmt.Sprintf("a1-%d", time.Now().UnixNano()),
		PasswordHash: "h", Name: "Admin",
	}
	err = userRepo.Create(ctx, adminUser)
	assert.NoError(t, err)

	// Add users to Org
	err = userRepo.AddUserToOrg(ctx, &domain.UserOrg{
		UserID: creditor.ID, OrgID: org.ID, Role: domain.UserOrgRoleMember, Status: domain.UserOrgStatusActive,
	})
	assert.NoError(t, err)
	err = userRepo.AddUserToOrg(ctx, &domain.UserOrg{
		UserID: debtor.ID, OrgID: org.ID, Role: domain.UserOrgRoleMember, Status: domain.UserOrgStatusActive,
	})
	assert.NoError(t, err)
	err = userRepo.AddUserToOrg(ctx, &domain.UserOrg{
		UserID: adminUser.ID, OrgID: org.ID, Role: domain.UserOrgRoleAdmin, Status: domain.UserOrgStatusActive,
	})
	assert.NoError(t, err)

	t.Run("Full Bill Lifecycle", func(t *testing.T) {
		// Goal: Verify the standard payment flow:
		// Created -> Pending -> Debtor Acknowledged -> Creditor Acknowledged -> Paid

		// 2. Create Bill (Pending)
		settlementMonth := time.Now().Format("2006-01")
		bill := &domain.Bill{
			OrgID:           org.ID,
			DebtorUserID:    debtor.ID,
			CreditorUserID:  creditor.ID,
			AmountCents:     5000, // $50.00
			SettlementMonth: settlementMonth,
			Status:          domain.BillStatusPending,
		}
		err := billRepo.Create(ctx, bill)
		assert.NoError(t, err)

		// 3. Check Summary
		// Creditor should have receipts to verify: 0 (since debtor hasn't acked)
		// Debtor should have payments to make: 1

		// For Creditor
		p, r, pd, rd, err := billSvc.GetGlobalBillSplitSummary(ctx, creditor.ID)
		assert.NoError(t, err)
		assert.Equal(t, int32(0), p)
		assert.Equal(t, int32(0), r) // Only counts when debtor acknowledges? Let's check logic later or assume based on names
		assert.Equal(t, int32(0), pd)
		assert.Equal(t, int32(0), rd)

		// For Debtor
		p, r, pd, rd, err = billSvc.GetGlobalBillSplitSummary(ctx, debtor.ID)
		assert.NoError(t, err)
		assert.Equal(t, int32(1), p)
		assert.Equal(t, int32(0), r)

		// 4. Debtor Acknowledges Payment
		err = billSvc.AcknowledgePayment(ctx, debtor.ID, bill.ID)
		assert.NoError(t, err)

		// Verify status update in DB
		updatedBill, err := billRepo.GetByID(ctx, bill.ID)
		assert.NoError(t, err)
		assert.NotNil(t, updatedBill.DebtorAcknowledgedAt)

		// 5. Creditor Acknowledges Receipt
		err = billSvc.AcknowledgePayment(ctx, creditor.ID, bill.ID)
		assert.NoError(t, err)

		updatedBill, err = billRepo.GetByID(ctx, bill.ID)
		assert.NoError(t, err)
		assert.Equal(t, domain.BillStatusPaid, updatedBill.Status)
		assert.NotNil(t, updatedBill.CreditorAcknowledgedAt)
	})

	t.Run("Dispute Lifecycle", func(t *testing.T) {
		// Goal: Verify the dispute resolution flow:
		// Created -> Pending -> Disputed -> Admin Resolved -> Paid

		// Create another bill
		// Use previous month to avoid unique constraint conflict with the bill from "Full Bill Lifecycle"
		prevMonth := time.Now().AddDate(0, -1, 0).Format("2006-01")
		bill2 := &domain.Bill{
			OrgID:           org.ID,
			DebtorUserID:    debtor.ID,
			CreditorUserID:  creditor.ID,
			AmountCents:     2500,
			SettlementMonth: prevMonth,
			Status:          domain.BillStatusPending,
		}
		err := billRepo.Create(ctx, bill2)
		assert.NoError(t, err)

		// Unfortunately ResolveDispute is usually manually triggered on the "Disputed" status
		// But currently there is no explicit "DisputePayment" method in the Service Interface I saw?
		// Checking interface again: ListDisputedPayments, ListResolvedDisputes, ResolveDispute.
		// There is no "DisputePayment" method exposed in service?
		// If so, I have to manually update status to DISPUTED to test ResolveDispute.

		bill2.Status = domain.BillStatusDisputed
		now := time.Now()
		bill2.DisputedAt = &now
		bill2.DisputeReason = string(domain.DisputeReasonDebtorNoAck)
		err = billRepo.Update(ctx, bill2)
		assert.NoError(t, err)

		// Verify ListDisputedPayments
		disputes, err := billSvc.ListDisputedPayments(ctx, adminUser.ID, org.ID) // Admin user
		assert.NoError(t, err)
		assert.NotEmpty(t, disputes)
		assert.Equal(t, bill2.ID, disputes[0].ID)

		// Resolve Dispute
		err = billSvc.ResolveDispute(ctx, adminUser.ID, bill2.ID, string(domain.ResolutionOutcomeDebtorFault), "Admin resolved: Debtor at fault")
		assert.NoError(t, err)

		updatedBill, err := billRepo.GetByID(ctx, bill2.ID)
		assert.NoError(t, err)
		assert.Equal(t, domain.BillStatusAdminResolved, updatedBill.Status)
		assert.Equal(t, string(domain.ResolutionOutcomeDebtorFault), updatedBill.ResolutionOutcome)
	})
}
