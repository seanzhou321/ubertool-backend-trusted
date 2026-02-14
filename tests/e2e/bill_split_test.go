package e2e

import (
	"testing"
	"fmt"
	"time"

	pb "ubertool-backend-trusted/api/gen/v1"
	"ubertool-backend-trusted/internal/config"
	"ubertool-backend-trusted/internal/jobs"
	"ubertool-backend-trusted/internal/repository/postgres"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBillSplitService_PaymentAcknowledgment tests the full payment acknowledgment flow.
// Goal: Verify the standard "Happy Path" for bill splitting where:
// 1. A bill exists in PENDING state.
// 2. The Debtor views the payment details and acknowledges sending the payment.
// 3. The Creditor confirms receiving the payment.
// 4. The system updates the bill status to PAID and updates user balances appropriately.
func TestBillSplitService_PaymentAcknowledgment(t *testing.T) {
	// Setup
	db := PrepareDB(t)
	defer db.Cleanup()
	client := NewGRPCClient(t, "")
	defer client.Close()
	billClient := pb.NewBillSplitServiceClient(client.Conn())

	// Create test organization
	orgID := db.CreateTestOrg("E2E-Test-BillOrg-" + t.Name())

	// Create test users
	debtorEmail := "e2e-test-debtor-" + t.Name() + "@test.com"
	creditorEmail := "e2e-test-creditor-" + t.Name() + "@test.com"

	debtorID := db.CreateTestUser(debtorEmail, "John Debtor")
	creditorID := db.CreateTestUser(creditorEmail, "Jane Creditor")

	// Add users to org with initial balances
	db.AddUserToOrg(debtorID, orgID, "MEMBER", "ACTIVE", 0)
	db.AddUserToOrg(creditorID, orgID, "MEMBER", "ACTIVE", 0)

	// Create a bill (debtor owes creditor $50)
	billID := db.CreateTestBill(debtorID, creditorID, orgID, 5000, "2024-01", "PENDING")

	// Step 1: Get payment detail (debtor's view)
	debtorCtx, cancelDebtor := ContextWithUserIDAndTimeout(debtorID, 5*time.Second)
	defer cancelDebtor()

	detailResp, err := billClient.GetPaymentDetail(debtorCtx, &pb.GetPaymentDetailRequest{
		PaymentId: billID,
	})
	require.NoError(t, err, "GetPaymentDetail should succeed")
	assert.NotNil(t, detailResp.Payment, "Payment should exist")
	assert.Equal(t, int32(5000), detailResp.Payment.AmountCents, "Amount should be 5000 cents")
	assert.True(t, detailResp.CanAcknowledge, "Debtor should be able to acknowledge")

	// Step 2: Debtor acknowledges payment
	ackResp, err := billClient.AcknowledgePayment(debtorCtx, &pb.AcknowledgePaymentRequest{
		PaymentId: billID,
	})
	require.NoError(t, err, "AcknowledgePayment (debtor) should succeed")
	assert.True(t, ackResp.Success, "Acknowledgment should be successful")

	// Verify bill status is still PENDING (waiting for creditor)
	bill := db.GetBillByID(billID)
	assert.Equal(t, "PENDING", bill["status"], "Bill should still be PENDING")
	assert.NotNil(t, bill["debtor_acknowledged_at"], "Debtor acknowledgment timestamp should be set")

	// Verify balance hasn't changed yet
	assert.Equal(t, int32(0), db.GetUserBalance(debtorID, orgID), "Debtor balance should not change yet")
	assert.Equal(t, int32(0), db.GetUserBalance(creditorID, orgID), "Creditor balance should not change yet")

	// Step 3: Creditor acknowledges payment
	creditorCtx, cancelCreditor := ContextWithUserIDAndTimeout(creditorID, 5*time.Second)
	defer cancelCreditor()

	ackResp2, err := billClient.AcknowledgePayment(creditorCtx, &pb.AcknowledgePaymentRequest{
		PaymentId: billID,
	})
	require.NoError(t, err, "AcknowledgePayment (creditor) should succeed")
	assert.True(t, ackResp2.Success, "Acknowledgment should be successful")

	// Verify bill status is now PAID
	bill = db.GetBillByID(billID)
	assert.Equal(t, "PAID", bill["status"], "Bill should be PAID")
	assert.NotNil(t, bill["creditor_acknowledged_at"], "Creditor acknowledgment timestamp should be set")

	// Verify balance updates (debtor: -5000, creditor: +5000)
	assert.Equal(t, int32(-5000), db.GetUserBalance(debtorID, orgID), "Debtor balance should be -5000")
	assert.Equal(t, int32(5000), db.GetUserBalance(creditorID, orgID), "Creditor balance should be +5000")

	// Verify bill actions were recorded
	actionCount := db.CountBillActions(billID)
	assert.GreaterOrEqual(t, actionCount, 2, "Should have at least 2 bill actions")

	latestAction := db.GetLatestBillAction(billID)
	assert.Equal(t, "CREDITOR_ACKNOWLEDGED", latestAction["action_type"], "Latest action should be CREDITOR_ACK")
}

// TestBillSplitService_DebtorDisputesPayment tests debtor disputing a payment.
// Goal: Verify that the system correctly identifies and lists disputed bills.
// 1. Creates a bill and manually moves it to DISPUTED state (simulating a dispute trigger).
// 2. Verifies that an Admin can query `ListDisputedPayments` and see this specific bill.
// 3. Checks that the bill metadata (IsResolved=false, ID match) is correct in the response.
func TestBillSplitService_DebtorDisputesPayment(t *testing.T) {
	// Setup
	db := PrepareDB(t)
	defer db.Cleanup()
	client := NewGRPCClient(t, "")
	defer client.Close()
	billClient := pb.NewBillSplitServiceClient(client.Conn())

	// Create test organization
	orgID := db.CreateTestOrg("E2E-Test-DisputeOrg-" + t.Name())

	// Create test users
	debtorEmail := "e2e-test-debtor-dispute-" + t.Name() + "@test.com"
	creditorEmail := "e2e-test-creditor-dispute-" + t.Name() + "@test.com"

	debtorID := db.CreateTestUser(debtorEmail, "John Disputer")
	creditorID := db.CreateTestUser(creditorEmail, "Jane Creditor")

	// Add users to org
	db.AddUserToOrg(debtorID, orgID, "MEMBER", "ACTIVE", 0)
	db.AddUserToOrg(creditorID, orgID, "MEMBER", "ACTIVE", 0)

	// Create a bill
	billID := db.CreateTestBill(debtorID, creditorID, orgID, 7500, "2024-01", "PENDING")

	// Debtor disputes the payment by acknowledging with a note (simulating dispute)
	// Note: In the actual implementation, dispute is triggered differently,
	// but for E2E we'll test that the system handles disputed bills

	// Manually set bill to disputed state for testing dispute listing and resolution
	_, err := db.Exec(`
		UPDATE bills 
		SET status = 'DISPUTED', 
		    disputed_at = NOW(), 
		    dispute_reason = 'Amount seems incorrect'
		WHERE id = $1
	`, billID)
	require.NoError(t, err, "Should be able to set bill to disputed")

	// List disputed payments as admin
	adminEmail := "e2e-test-admin-" + t.Name() + "@test.com"
	adminID := db.CreateTestUser(adminEmail, "Admin User")
	db.AddUserToOrg(adminID, orgID, "ADMIN", "ACTIVE", 0)

	adminCtx, cancelAdmin := ContextWithUserIDAndTimeout(adminID, 5*time.Second)
	defer cancelAdmin()

	disputesResp, err := billClient.ListDisputedPayments(adminCtx, &pb.ListDisputedPaymentsRequest{
		OrganizationId: orgID,
	})
	require.NoError(t, err, "ListDisputedPayments should succeed")
	assert.Len(t, disputesResp.Disputes, 1, "Should have 1 disputed payment")
	assert.Equal(t, billID, disputesResp.Disputes[0].PaymentId, "Disputed payment ID should match")
	assert.False(t, disputesResp.Disputes[0].IsResolved, "Dispute should not be resolved yet")
}

// TestBillSplitService_AdminResolvesDisputeDebtorFault tests admin resolving dispute with debtor at fault.
// Goal: Verify the dispute resolution logic by an Admin.
// 1. Sets up a bill in DISPUTED state.
// 2. Admin calls `ResolveDispute` with a specific resolution (DEBTOR_AT_FAULT).
// 3. Verifies that the bill status transitions to ADMIN_RESOLVED.
// 4. Verifies that the resolution outcome is correctly persisted (DEBTOR_FAULT).
func TestBillSplitService_AdminResolvesDisputeDebtorFault(t *testing.T) {
	// Setup
	db := PrepareDB(t)
	defer db.Cleanup()
	client := NewGRPCClient(t, "")
	defer client.Close()
	billClient := pb.NewBillSplitServiceClient(client.Conn())

	// Create test organization
	orgID := db.CreateTestOrg("E2E-Test-ResolveOrg-" + t.Name())

	// Create test users
	debtorEmail := "e2e-test-debtor-resolve-" + t.Name() + "@test.com"
	creditorEmail := "e2e-test-creditor-resolve-" + t.Name() + "@test.com"
	adminEmail := "e2e-test-admin-resolve-" + t.Name() + "@test.com"

	debtorID := db.CreateTestUser(debtorEmail, "John Debtor")
	creditorID := db.CreateTestUser(creditorEmail, "Jane Creditor")
	adminID := db.CreateTestUser(adminEmail, "Admin Resolver")

	// Add users to org
	db.AddUserToOrg(debtorID, orgID, "MEMBER", "ACTIVE", 0)
	db.AddUserToOrg(creditorID, orgID, "MEMBER", "ACTIVE", 0)
	db.AddUserToOrg(adminID, orgID, "ADMIN", "ACTIVE", 0)

	// Create a disputed bill
	billID := db.CreateTestBill(debtorID, creditorID, orgID, 10000, "2024-01", "DISPUTED")
	_, err := db.Exec("UPDATE bills SET disputed_at = NOW(), dispute_reason = 'Debtor refused to pay' WHERE id = $1", billID)
	require.NoError(t, err)

	// Admin resolves dispute - debtor at fault
	adminCtx, cancelAdmin := ContextWithUserIDAndTimeout(adminID, 5*time.Second)
	defer cancelAdmin()

	resolveResp, err := billClient.ResolveDispute(adminCtx, &pb.ResolveDisputeRequest{
		PaymentId:  billID,
		Resolution: pb.DisputeResolution_DEBTOR_AT_FAULT,
		Notes:      "Admin resolved: Debtor at fault - unpaid bill",
	})
	require.NoError(t, err, "ResolveDispute should succeed")
	assert.True(t, resolveResp.Success, "Resolution should be successful")

	// Verify bill status
	bill := db.GetBillByID(billID)
	assert.Equal(t, "ADMIN_RESOLVED", bill["status"], "Bill should be ADMIN_RESOLVED")
	assert.Equal(t, "DEBTOR_FAULT", bill["resolution_outcome"], "Resolution outcome should be DEBTOR_FAULT")

	// Verify balance adjustments (debtor pays full amount + penalty)
	debtorBalance := db.GetUserBalance(debtorID, orgID)
	creditorBalance := db.GetUserBalance(creditorID, orgID)

	assert.Less(t, debtorBalance, int32(0), "Debtor balance should be negative")
	assert.Greater(t, creditorBalance, int32(0), "Creditor balance should be positive")

	// Verify action recorded
	latestAction := db.GetLatestBillAction(billID)
	assert.Equal(t, "ADMIN_RESOLUTION", latestAction["action_type"], "Latest action should be ADMIN_RESOLUTION")
}

// TestBillSplitService_AdminResolvesDisputeCreditorFault tests admin resolving dispute with creditor at fault
func TestBillSplitService_AdminResolvesDisputeCreditorFault(t *testing.T) {
	// Setup
	db := PrepareDB(t)
	defer db.Cleanup()
	client := NewGRPCClient(t, "")
	defer client.Close()
	billClient := pb.NewBillSplitServiceClient(client.Conn())

	// Create test organization
	orgID := db.CreateTestOrg("E2E-Test-CreditorFaultOrg-" + t.Name())

	// Create test users
	debtorEmail := "e2e-test-debtor-cfault-" + t.Name() + "@test.com"
	creditorEmail := "e2e-test-creditor-cfault-" + t.Name() + "@test.com"
	adminEmail := "e2e-test-admin-cfault-" + t.Name() + "@test.com"

	debtorID := db.CreateTestUser(debtorEmail, "John Debtor")
	creditorID := db.CreateTestUser(creditorEmail, "Jane BadCreditor")
	adminID := db.CreateTestUser(adminEmail, "Admin Resolver")

	// Add users to org
	db.AddUserToOrg(debtorID, orgID, "MEMBER", "ACTIVE", 0)
	db.AddUserToOrg(creditorID, orgID, "MEMBER", "ACTIVE", 0)
	db.AddUserToOrg(adminID, orgID, "ADMIN", "ACTIVE", 0)

	// Create a disputed bill
	billID := db.CreateTestBill(debtorID, creditorID, orgID, 8000, "2024-01", "DISPUTED")
	_, err := db.Exec("UPDATE bills SET disputed_at = NOW(), dispute_reason = 'Creditor claim is invalid' WHERE id = $1", billID)
	require.NoError(t, err)

	// Admin resolves dispute - creditor at fault
	adminCtx, cancelAdmin := ContextWithUserIDAndTimeout(adminID, 5*time.Second)
	defer cancelAdmin()

	resolveResp, err := billClient.ResolveDispute(adminCtx, &pb.ResolveDisputeRequest{
		PaymentId:  billID,
		Resolution: pb.DisputeResolution_CREDITOR_AT_FAULT,
		Notes:      "Admin resolved: Creditor at fault - invalid bill",
	})
	require.NoError(t, err, "ResolveDispute should succeed")
	assert.True(t, resolveResp.Success, "Resolution should be successful")

	// Verify bill status
	bill := db.GetBillByID(billID)
	assert.Equal(t, "ADMIN_RESOLVED", bill["status"], "Bill should be ADMIN_RESOLVED")
	assert.Equal(t, "CREDITOR_FAULT", bill["resolution_outcome"], "Resolution outcome should be CREDITOR_FAULT")

	// Verify balance adjustments (creditor penalized)
	creditorBalance := db.GetUserBalance(creditorID, orgID)
	assert.Less(t, creditorBalance, int32(0), "Creditor balance should be negative (penalty)")
}

// TestBillSplitService_AdminResolvesBothAtFault tests admin resolving dispute with both parties at fault
func TestBillSplitService_AdminResolvesBothAtFault(t *testing.T) {
	// Setup
	db := PrepareDB(t)
	defer db.Cleanup()
	client := NewGRPCClient(t, "")
	defer client.Close()
	billClient := pb.NewBillSplitServiceClient(client.Conn())

	// Create test organization
	orgID := db.CreateTestOrg("E2E-Test-BothFaultOrg-" + t.Name())

	// Create test users
	debtorEmail := "e2e-test-debtor-both-" + t.Name() + "@test.com"
	creditorEmail := "e2e-test-creditor-both-" + t.Name() + "@test.com"
	adminEmail := "e2e-test-admin-both-" + t.Name() + "@test.com"

	debtorID := db.CreateTestUser(debtorEmail, "John Debtor")
	creditorID := db.CreateTestUser(creditorEmail, "Jane Creditor")
	adminID := db.CreateTestUser(adminEmail, "Admin Resolver")

	// Add users to org
	db.AddUserToOrg(debtorID, orgID, "MEMBER", "ACTIVE", 0)
	db.AddUserToOrg(creditorID, orgID, "MEMBER", "ACTIVE", 0)
	db.AddUserToOrg(adminID, orgID, "ADMIN", "ACTIVE", 0)

	// Create a disputed bill
	billID := db.CreateTestBill(debtorID, creditorID, orgID, 6000, "2024-01", "DISPUTED")
	_, err := db.Exec("UPDATE bills SET disputed_at = NOW(), dispute_reason = 'Both parties misunderstood' WHERE id = $1", billID)
	require.NoError(t, err)

	// Admin resolves dispute - both at fault
	adminCtx, cancelAdmin := ContextWithUserIDAndTimeout(adminID, 5*time.Second)
	defer cancelAdmin()

	resolveResp, err := billClient.ResolveDispute(adminCtx, &pb.ResolveDisputeRequest{
		PaymentId:  billID,
		Resolution: pb.DisputeResolution_BOTH_AT_FAULT,
		Notes:      "Admin resolved: Both parties at fault - miscommunication",
	})
	require.NoError(t, err, "ResolveDispute should succeed")
	assert.True(t, resolveResp.Success, "Resolution should be successful")

	// Verify bill status
	bill := db.GetBillByID(billID)
	assert.Equal(t, "ADMIN_RESOLVED", bill["status"], "Bill should be ADMIN_RESOLVED")
	assert.Equal(t, "BOTH_FAULT", bill["resolution_outcome"], "Resolution outcome should be BOTH_FAULT")

	// Verify both parties have balance adjustments (split penalty)
	debtorBalance := db.GetUserBalance(debtorID, orgID)
	creditorBalance := db.GetUserBalance(creditorID, orgID)

	assert.NotEqual(t, int32(0), debtorBalance, "Debtor should have balance adjustment")
	assert.NotEqual(t, int32(0), creditorBalance, "Creditor should have balance adjustment")
}

// TestBillSplitService_GetGlobalBillSplitSummary tests global summary across organizations
func TestBillSplitService_GetGlobalBillSplitSummary(t *testing.T) {
	// Setup
	db := PrepareDB(t)
	defer db.Cleanup()
	client := NewGRPCClient(t, "")
	defer client.Close()
	billClient := pb.NewBillSplitServiceClient(client.Conn())

	// Create test organization
	orgID := db.CreateTestOrg("E2E-Test-SummaryOrg-" + t.Name())

	// Create test users
	userEmail := "e2e-test-user-summary-" + t.Name() + "@test.com"
	otherEmail := "e2e-test-other-summary-" + t.Name() + "@test.com"

	userID := db.CreateTestUser(userEmail, "John Summary")
	otherID := db.CreateTestUser(otherEmail, "Jane Other")

	// Add users to org
	db.AddUserToOrg(userID, orgID, "MEMBER", "ACTIVE", 0)
	db.AddUserToOrg(otherID, orgID, "MEMBER", "ACTIVE", 0)

	// Create multiple bills (user owes 1000, is owed 500)
	db.CreateTestBill(userID, otherID, orgID, 1000, "2024-01", "PENDING") // User owes 1000
	billID2 := db.CreateTestBill(otherID, userID, orgID, 500, "2024-01", "PENDING")  // User is owed 500

	// Acknowledge the bill where user is creditor, so it appears in "ReceiptsToVerify"
	_, err := db.Exec("UPDATE bills SET debtor_acknowledged_at = NOW() WHERE id = $1", billID2)
	require.NoError(t, err)

	// Set balances manually
	db.SetUserBalance(userID, orgID, -500) // Net: owes 500
	db.SetUserBalance(otherID, orgID, 500) // Net: is owed 500

	// Get global summary
	userCtx, cancelUser := ContextWithUserIDAndTimeout(userID, 5*time.Second)
	defer cancelUser()

	summaryResp, err := billClient.GetGlobalBillSplitSummary(userCtx, &pb.GetGlobalBillSplitSummaryRequest{})
	require.NoError(t, err, "GetGlobalBillSplitSummary should succeed")

	assert.NotNil(t, summaryResp, "Summary response should not be nil")
	assert.NotNil(t, summaryResp.Summary, "Summary field should not be nil")
	// User owes 1000 (payment to make) and is owed 500 (receipt to verify)
	assert.GreaterOrEqual(t, summaryResp.Summary.PaymentsToMake, int32(1), "Should have at least 1 payment to make")
	assert.GreaterOrEqual(t, summaryResp.Summary.ReceiptsToVerify, int32(1), "Should have at least 1 receipt to verify")
}

// TestBillSplitService_GetOrganizationBillSplitSummary tests organization-specific summary
func TestBillSplitService_GetOrganizationBillSplitSummary(t *testing.T) {
	// Setup
	db := PrepareDB(t)
	defer db.Cleanup()
	client := NewGRPCClient(t, "")
	defer client.Close()
	billClient := pb.NewBillSplitServiceClient(client.Conn())

	// Create test organization
	orgID := db.CreateTestOrg("E2E-Test-OrgSummary-" + t.Name())

	// Create test users
	userEmail := "e2e-test-user-org-summary-" + t.Name() + "@test.com"
	otherEmail := "e2e-test-other-org-summary-" + t.Name() + "@test.com"

	userID := db.CreateTestUser(userEmail, "John OrgSummary")
	otherID := db.CreateTestUser(otherEmail, "Jane OrgOther")

	// Add users to org
	db.AddUserToOrg(userID, orgID, "MEMBER", "ACTIVE", 0)
	db.AddUserToOrg(otherID, orgID, "MEMBER", "ACTIVE", 0)

	// Create bills
	db.CreateTestBill(userID, otherID, orgID, 2000, "2024-01", "PENDING")
	db.CreateTestBill(userID, otherID, orgID, 1500, "2024-02", "PAID")

	// Set balance
	db.SetUserBalance(userID, orgID, -3500)
	db.SetUserBalance(otherID, orgID, 3500)

	// Get organization summary
	userCtx, cancelUser := ContextWithUserIDAndTimeout(userID, 5*time.Second)
	defer cancelUser()

	orgSummaryResp, err := billClient.GetOrganizationBillSplitSummary(userCtx, &pb.GetOrganizationBillSplitSummaryRequest{})
	require.NoError(t, err, "GetOrganizationBillSplitSummary should succeed")

	assert.NotNil(t, orgSummaryResp, "Org summary response should not be nil")

	// Find our org in the list
	var foundOrgSummary *pb.OrganizationBillSplitSummary
	for _, s := range orgSummaryResp.OrgSummaries {
		if s.OrganizationId == orgID {
			foundOrgSummary = s
			break
		}
	}
	require.NotNil(t, foundOrgSummary, "Should find summary for test organization")

	// Check summary counts
	// 1 PENDING bill where user is debtor -> 1 Payment To Make
	assert.Equal(t, int32(1), foundOrgSummary.Summary.PaymentsToMake, "Should have 1 payment to make")
}

// TestBillSplitService_ListPayments tests listing payments for a user
func TestBillSplitService_ListPayments(t *testing.T) {
	// Setup
	db := PrepareDB(t)
	defer db.Cleanup()
	client := NewGRPCClient(t, "")
	defer client.Close()
	billClient := pb.NewBillSplitServiceClient(client.Conn())

	// Create test organization
	orgID := db.CreateTestOrg("E2E-Test-ListOrg-" + t.Name())

	// Create test users
	userEmail := "e2e-test-user-list-" + t.Name() + "@test.com"
	otherEmail := "e2e-test-other-list-" + t.Name() + "@test.com"

	userID := db.CreateTestUser(userEmail, "John Lister")
	otherID := db.CreateTestUser(otherEmail, "Jane Other")

	// Add users to org
	db.AddUserToOrg(userID, orgID, "MEMBER", "ACTIVE", 0)
	db.AddUserToOrg(otherID, orgID, "MEMBER", "ACTIVE", 0)

	// Create multiple bills with different categories
	db.CreateTestBill(userID, otherID, orgID, 1000, "2024-01", "PENDING") // User owes
	db.CreateTestBill(otherID, userID, orgID, 500, "2024-01", "PAID")     // User is owed
	db.CreateTestBill(userID, otherID, orgID, 300, "2024-02", "DISPUTED") // User owes (disputed)

	// List active payments (Pending + Disputed)
	userCtx, cancelUser := ContextWithUserIDAndTimeout(userID, 5*time.Second)
	defer cancelUser()

	listResp, err := billClient.ListPayments(userCtx, &pb.ListPaymentsRequest{
		OrganizationId: orgID,
		ShowHistory:    false,
	})
	require.NoError(t, err, "ListPayments (Active) should succeed")
	assert.GreaterOrEqual(t, len(listResp.Payments), 2, "Should have at least 2 active payments")

	// List history payments (Paid)
	listRespHistory, err := billClient.ListPayments(userCtx, &pb.ListPaymentsRequest{
		OrganizationId: orgID,
		ShowHistory:    true,
	})
	require.NoError(t, err, "ListPayments (History) should succeed")
	assert.GreaterOrEqual(t, len(listRespHistory.Payments), 1, "Should have at least 1 history payment")

	// Count payments by category locally (from active list)
	var owedByMeCount, owedToMeCount int
	for _, p := range listResp.Payments {
		if p.Category == pb.PaymentCategory_PAYMENT_TO_MAKE || p.Category == pb.PaymentCategory_PAYMENT_IN_DISPUTE {
			owedByMeCount++
		}
		if p.Category == pb.PaymentCategory_RECEIPT_TO_VERIFY || p.Category == pb.PaymentCategory_RECEIPT_IN_DISPUTE {
			owedToMeCount++
		}
	}

	// 1000 (Pending, owe) + 300 (Disputed, owe) -> 2 owedByMe
	// 500 (Paid, owed) -> Is in History, not here.
	
	assert.GreaterOrEqual(t, owedByMeCount, 2, "Should have at least 2 payments to make/dispute in active list")
}

// TestBillSplitService_ListResolvedDisputes tests listing resolved disputes
func TestBillSplitService_ListResolvedDisputes(t *testing.T) {
	// Setup
	db := PrepareDB(t)
	defer db.Cleanup()
	client := NewGRPCClient(t, "")
	defer client.Close()
	billClient := pb.NewBillSplitServiceClient(client.Conn())

	// Create test organization
	orgID := db.CreateTestOrg("E2E-Test-ResolvedOrg-" + t.Name())

	// Create test users
	adminEmail := "e2e-test-admin-resolved-" + t.Name() + "@test.com"
	debtorEmail := "e2e-test-debtor-resolved-" + t.Name() + "@test.com"
	creditorEmail := "e2e-test-creditor-resolved-" + t.Name() + "@test.com"

	adminID := db.CreateTestUser(adminEmail, "Admin User")
	debtorID := db.CreateTestUser(debtorEmail, "John Debtor")
	creditorID := db.CreateTestUser(creditorEmail, "Jane Creditor")

	// Add users to org
	db.AddUserToOrg(adminID, orgID, "ADMIN", "ACTIVE", 0)
	db.AddUserToOrg(debtorID, orgID, "MEMBER", "ACTIVE", 0)
	db.AddUserToOrg(creditorID, orgID, "MEMBER", "ACTIVE", 0)

	// Create a resolved dispute
	billID := db.CreateTestBill(debtorID, creditorID, orgID, 5000, "2024-01", "ADMIN_RESOLVED")
	_, err := db.Exec(`
		UPDATE bills 
		SET disputed_at = NOW() - INTERVAL '1 day', 
		    resolved_at = NOW(), 
		    dispute_reason = 'Test dispute',
		    resolution_outcome = 'DEBTOR_FAULT',
		    resolution_notes = 'Admin resolved: debtor at fault'
		WHERE id = $1
	`, billID)
	require.NoError(t, err)

	// List resolved disputes
	adminCtx, cancelAdmin := ContextWithUserIDAndTimeout(adminID, 5*time.Second)
	defer cancelAdmin()

	resolvedResp, err := billClient.ListResolvedDisputes(adminCtx, &pb.ListResolvedDisputesRequest{
		OrganizationId: orgID,
	})
	require.NoError(t, err, "ListResolvedDisputes should succeed")
	assert.Len(t, resolvedResp.Disputes, 1, "Should have 1 resolved dispute")
	assert.Equal(t, billID, resolvedResp.Disputes[0].PaymentId, "Resolved dispute ID should match")
	assert.True(t, resolvedResp.Disputes[0].IsResolved, "Dispute should be marked as resolved")
	assert.Equal(t, "DEBTOR_FAULT", resolvedResp.Disputes[0].Resolution, "Resolution should be DEBTOR_FAULT")
}

// TestBillSplitService_UnauthorizedAccess tests that users cannot access other users' bills
func TestBillSplitService_UnauthorizedAccess(t *testing.T) {
	// Setup
	db := PrepareDB(t)
	defer db.Cleanup()
	client := NewGRPCClient(t, "")
	defer client.Close()
	billClient := pb.NewBillSplitServiceClient(client.Conn())

	// Create test organization
	orgID := db.CreateTestOrg("E2E-Test-UnauthorizedOrg-" + t.Name())

	// Create test users
	user1Email := "e2e-test-user1-unauth-" + t.Name() + "@test.com"
	user2Email := "e2e-test-user2-unauth-" + t.Name() + "@test.com"
	user3Email := "e2e-test-user3-unauth-" + t.Name() + "@test.com"

	user1ID := db.CreateTestUser(user1Email, "User One")
	user2ID := db.CreateTestUser(user2Email, "User Two")
	user3ID := db.CreateTestUser(user3Email, "User Three")

	// Add only user1 and user2 to org
	db.AddUserToOrg(user1ID, orgID, "MEMBER", "ACTIVE", 0)
	db.AddUserToOrg(user2ID, orgID, "MEMBER", "ACTIVE", 0)
	// user3 is NOT in the org

	// Create a bill between user1 and user2
	billID := db.CreateTestBill(user1ID, user2ID, orgID, 1000, "2024-01", "PENDING")

	// Try to access payment detail as user3 (unauthorized)
	user3Ctx, cancelUser3 := ContextWithUserIDAndTimeout(user3ID, 5*time.Second)
	defer cancelUser3()

	_, err := billClient.GetPaymentDetail(user3Ctx, &pb.GetPaymentDetailRequest{
		PaymentId: billID,
	})
	assert.Error(t, err, "GetPaymentDetail should fail for unauthorized user")

	// Try to acknowledge as user3 (unauthorized)
	ackResp, err := billClient.AcknowledgePayment(user3Ctx, &pb.AcknowledgePaymentRequest{
		PaymentId: billID,
	})
	// The service handles authorization errors by returning success=false within the response
	// or returns an error. We should check both possibilities.
	if err == nil {
		assert.False(t, ackResp.Success, "AcknowledgePayment should result in success=false for unauthorized user")
	} else {
		assert.Error(t, err, "AcknowledgePayment should fail for unauthorized user")
	}
}

func TestBillSplitAlgorithm_E2E(t *testing.T) {
	// Setup DB
	testDB := PrepareDB(t)
	defer testDB.Cleanup()

	// Scenario: Small church community tool sharing
	// Scenarios from reference implementation
	// John: 4550
	// Mary: -3820
	// Peter: 1275
	// Sarah: -1560
	// David: 320
	// Emma: -280
	// Luke: 2500
	// Anna: -3015
	// Mark: 450
	// Ruth: -420

	orgID := testDB.CreateTestOrg("E2E-Church-Community-" + t.Name())

	users := []struct {
		Name    string
		Balance int32
		ID      int32
	}{
		{"John", 4550, 0},
		{"Mary", -3820, 0},
		{"Peter", 1275, 0},
		{"Sarah", -1560, 0},
		{"David", 320, 0},
		{"Emma", -280, 0},
		{"Luke", 2500, 0},
		{"Anna", -3015, 0},
		{"Mark", 450, 0},
		{"Ruth", -420, 0},
	}

	for i := range users {
		email := fmt.Sprintf("e2e-test-church-%s-%d@test.com", users[i].Name, time.Now().UnixNano())
		users[i].ID = testDB.CreateTestUser(email, users[i].Name)
		testDB.AddUserToOrg(users[i].ID, orgID, "MEMBER", "ACTIVE", users[i].Balance)
	}

	// Setup JobRunner
	// We need a real DB connection (testDB.DB) and a config with threshold
	// Manually construct config since loadConfig might not be available or we want specific settings
	cfg := &config.Config{
		Billing: config.BillingConfig{
			SettlementThresholdCents: 500, // $5.00
		},
	}

	store := postgres.NewStore(testDB.DB)
	// Services can be nil as PerformBillSplitting doesn't use them (confirmed by code analysis)
	jobRunner := jobs.NewJobRunner(testDB.DB, store, nil, cfg)

	// Trigger Bill Splitting
	// We call the job function directly. It will discover the org and run the algorithm.
	// Since PerformBillSplitting runs for ALL orgs, it will pick up our test org.
	// However, we want to ensure it uses the correct "last month" logic.
	// The implementation calculates "last month" as time.Now().AddDate(0, -1, 0).Format("2006-01").
	// We need to verify bills for THAT month.
	
	jobRunner.PerformBillSplitting()

	// Verification
	// We expect 4 bills based on the unit test analysis:
	// 1. Mary(3820) -> John
	// 2. Anna(2500) -> Luke
	// 3. Sarah(1275) -> Peter
	// 4. Anna(515) -> John
	
	lastMonth := time.Now().AddDate(0, -1, 0).Format("2006-01")

	// Helper to find user ID by name
	getUserID := func(name string) int32 {
		for _, u := range users {
			if u.Name == name {
				return u.ID
			}
		}
		return 0
	}

	johnID := getUserID("John")
	maryID := getUserID("Mary")
	peterID := getUserID("Peter")
	sarahID := getUserID("Sarah")
	lukeID := getUserID("Luke")
	annaID := getUserID("Anna")

	// Check bills
	checkBill := func(debtorID, creditorID int32, amount int32) {
		var count int
		err := testDB.QueryRow(`
			SELECT COUNT(*) 
			FROM bills 
			WHERE org_id = $1 
			  AND debtor_user_id = $2 
			  AND creditor_user_id = $3 
			  AND amount_cents = $4
			  AND settlement_month = $5
		`, orgID, debtorID, creditorID, amount, lastMonth).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "Expected bill from %d to %d for %d cents", debtorID, creditorID, amount)
	}

	checkBill(maryID, johnID, 3820)
	checkBill(annaID, lukeID, 2500)
	checkBill(sarahID, peterID, 1275)
	checkBill(annaID, johnID, 515)

	// Verify total count
	var totalBills int
	err := testDB.QueryRow("SELECT COUNT(*) FROM bills WHERE org_id = $1 AND settlement_month = $2", orgID, lastMonth).Scan(&totalBills)
	require.NoError(t, err)
	assert.Equal(t, 4, totalBills, "Expected exactly 4 bills")
}
