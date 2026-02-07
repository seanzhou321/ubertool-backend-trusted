package e2e

import (
	"testing"
	"time"

	pb "ubertool-backend-trusted/api/gen/v1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRentalService_E2E(t *testing.T) {
	db := PrepareDB(t)
	defer db.Close()
	defer db.Cleanup()

	client := NewGRPCClient(t, "")
	defer client.Close()

	rentalClient := pb.NewRentalServiceClient(client.Conn())

	t.Run("Full Rental Lifecycle", func(t *testing.T) {
		// Setup: Create org, owner, renter, and tool
		orgID := db.CreateTestOrg("")
		ownerID := db.CreateTestUser("e2e-test-owner@test.com", "Tool Owner")
		renterID := db.CreateTestUser("e2e-test-renter@test.com", "Tool Renter")

		db.AddUserToOrg(ownerID, orgID, "MEMBER", "ACTIVE", 0)
		db.AddUserToOrg(renterID, orgID, "MEMBER", "ACTIVE", 5000) // Renter has 5000 cents balance

		toolID := db.CreateTestTool(ownerID, "E2E Rental Tool", 1000)

		// Step 1: Create Rental Request
		ctx1, cancel1 := ContextWithUserIDAndTimeout(renterID, 5*time.Second)
		defer cancel1()

		startDate := time.Now().Add(24 * time.Hour)
		endDate := startDate.Add(48 * time.Hour) // 2 days rental

		createReq := &pb.CreateRentalRequestRequest{
			ToolId:         toolID,
			StartDate:      startDate.Format("2006-01-02"),
			EndDate:        endDate.Format("2006-01-02"),
			OrganizationId: orgID,
		}

		createResp, err := rentalClient.CreateRentalRequest(ctx1, createReq)
		require.NoError(t, err)
		assert.NotNil(t, createResp.RentalRequest)
		assert.Equal(t, pb.RentalStatus_RENTAL_STATUS_PENDING, createResp.RentalRequest.Status)
		rentalID := createResp.RentalRequest.Id

		// Verify: Notification sent to owner
		var ownerNotifCount int
		err = db.QueryRow("SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND org_id = $2", ownerID, orgID).Scan(&ownerNotifCount)
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, ownerNotifCount, 1)

		// Step 2: Owner Approves Rental Request
		ctx2, cancel2 := ContextWithUserIDAndTimeout(ownerID, 5*time.Second)
		defer cancel2()

		approveReq := &pb.ApproveRentalRequestRequest{
			RequestId:          rentalID,
			PickupInstructions: "Pick up at my garage",
		}

		approveResp, err := rentalClient.ApproveRentalRequest(ctx2, approveReq)
		require.NoError(t, err)
		assert.Equal(t, pb.RentalStatus_RENTAL_STATUS_APPROVED, approveResp.RentalRequest.Status)
		assert.Equal(t, "Pick up at my garage", approveResp.RentalRequest.PickupInstructions)

		// Verify: Notification sent to renter
		var renterNotifCount int
		err = db.QueryRow("SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND org_id = $2", renterID, orgID).Scan(&renterNotifCount)
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, renterNotifCount, 1)

		// Step 3: Renter Finalizes Rental (Payment)
		ctx3, cancel3 := ContextWithUserIDAndTimeout(renterID, 5*time.Second)
		defer cancel3()

		finalizeReq := &pb.FinalizeRentalRequestRequest{
			RequestId: rentalID,
		}

		finalizeResp, err := rentalClient.FinalizeRentalRequest(ctx3, finalizeReq)
		require.NoError(t, err)
		assert.Equal(t, pb.RentalStatus_RENTAL_STATUS_SCHEDULED, finalizeResp.RentalRequest.Status)

		// Verify: Renter's balance unchanged (no transaction at finalize)
		var renterBalance int32
		err = db.QueryRow("SELECT balance_cents FROM users_orgs WHERE user_id = $1 AND org_id = $2", renterID, orgID).Scan(&renterBalance)
		assert.NoError(t, err)
		assert.Equal(t, int32(5000), renterBalance) // Balance unchanged

		// Verify: No ledger transaction at finalize
		var txCount int
		err = db.QueryRow("SELECT COUNT(*) FROM ledger_transactions WHERE user_id = $1 AND org_id = $2", renterID, orgID).Scan(&txCount)
		assert.NoError(t, err)
		assert.Equal(t, 0, txCount) // No transactions yet

		// Verify: Tool status updated to RENTED
		var toolStatus string
		err = db.QueryRow("SELECT status FROM tools WHERE id = $1", toolID).Scan(&toolStatus)
		assert.NoError(t, err)
		assert.Equal(t, "RENTED", toolStatus)

		// Step 4: Complete Rental
		ctx4, cancel4 := ContextWithUserIDAndTimeout(ownerID, 5*time.Second)
		defer cancel4()

		completeReq := &pb.CompleteRentalRequest{
			RequestId:              rentalID,
			ReturnCondition:        "Good condition",
			SurchargeOrCreditCents: 0,
		}

		completeResp, err := rentalClient.CompleteRental(ctx4, completeReq)
		require.NoError(t, err)
		assert.Equal(t, pb.RentalStatus_RENTAL_STATUS_COMPLETED, completeResp.RentalRequest.Status)

		// Verify: Renter's balance was debited at completion
		err = db.QueryRow("SELECT balance_cents FROM users_orgs WHERE user_id = $1 AND org_id = $2", renterID, orgID).Scan(&renterBalance)
		assert.NoError(t, err)
		assert.Equal(t, int32(2000), renterBalance) // 5000 - 3000 = 2000

		// Verify: Owner's balance was credited
		var ownerBalance int32
		err = db.QueryRow("SELECT balance_cents FROM users_orgs WHERE user_id = $1 AND org_id = $2", ownerID, orgID).Scan(&ownerBalance)
		assert.NoError(t, err)
		assert.Equal(t, int32(3000), ownerBalance) // Owner receives the rental cost

		// Verify: Ledger transactions created (2 paired transactions: owner credit + renter debit)
		var renterDebitCount int
		err = db.QueryRow("SELECT COUNT(*) FROM ledger_transactions WHERE user_id = $1 AND org_id = $2 AND type = 'LENDING_DEBIT'", renterID, orgID).Scan(&renterDebitCount)
		assert.NoError(t, err)
		assert.Equal(t, 1, renterDebitCount)

		// Verify: Ledger transaction created (credit)
		var creditCount int
		err = db.QueryRow("SELECT COUNT(*) FROM ledger_transactions WHERE user_id = $1 AND org_id = $2 AND type = 'LENDING_CREDIT'", ownerID, orgID).Scan(&creditCount)
		assert.NoError(t, err)
		assert.Equal(t, 1, creditCount)

		// Verify: Tool status back to AVAILABLE
		err = db.QueryRow("SELECT status FROM tools WHERE id = $1", toolID).Scan(&toolStatus)
		assert.NoError(t, err)
		assert.Equal(t, "AVAILABLE", toolStatus)
	})

	t.Run("Reject Rental Request", func(t *testing.T) {
		// Setup
		orgID := db.CreateTestOrg("")
		ownerID := db.CreateTestUser("e2e-test-owner2@test.com", "Owner 2")
		renterID := db.CreateTestUser("e2e-test-renter2@test.com", "Renter 2")

		db.AddUserToOrg(ownerID, orgID, "MEMBER", "ACTIVE", 0)
		db.AddUserToOrg(renterID, orgID, "MEMBER", "ACTIVE", 5000)

		toolID := db.CreateTestTool(ownerID, "Tool for Rejection", 1000)

		// Create rental request
		ctx1, cancel1 := ContextWithUserIDAndTimeout(renterID, 5*time.Second)
		defer cancel1()

		startDate := time.Now().Add(24 * time.Hour)
		endDate := startDate.Add(24 * time.Hour)

		createReq := &pb.CreateRentalRequestRequest{
			ToolId:         toolID,
			StartDate:      startDate.Format("2006-01-02"),
			EndDate:        endDate.Format("2006-01-02"),
			OrganizationId: orgID,
		}

		createResp, err := rentalClient.CreateRentalRequest(ctx1, createReq)
		require.NoError(t, err)
		assert.NotNil(t, createResp.RentalRequest)
		assert.Equal(t, pb.RentalStatus_RENTAL_STATUS_PENDING, createResp.RentalRequest.Status)
		rentalID := createResp.RentalRequest.Id

		// Owner rejects
		ctx2, cancel2 := ContextWithUserIDAndTimeout(ownerID, 5*time.Second)
		defer cancel2()

		rejectReq := &pb.RejectRentalRequestRequest{
			RequestId: rentalID,
			Reason:    "Tool is not available",
		}

		rejectResp, err := rentalClient.RejectRentalRequest(ctx2, rejectReq)
		require.NoError(t, err)
		assert.Equal(t, pb.RentalStatus_RENTAL_STATUS_REJECTED, rejectResp.RentalRequest.Status)

		// Verify: Notification sent to renter
		var notifCount int
		err = db.QueryRow("SELECT COUNT(*) FROM notifications WHERE user_id = $1", renterID).Scan(&notifCount)
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, notifCount, 1)
	})

	t.Run("Update Extension Request While Pending (RETURN_DATE_CHANGED)", func(t *testing.T) {
		// Setup
		orgID := db.CreateTestOrg("")
		ownerID := db.CreateTestUser("e2e-test-owner-ext@test.com", "Owner Extension")
		renterID := db.CreateTestUser("e2e-test-renter-ext@test.com", "Renter Extension")

		db.AddUserToOrg(ownerID, orgID, "MEMBER", "ACTIVE", 0)
		db.AddUserToOrg(renterID, orgID, "MEMBER", "ACTIVE", 10000) // Sufficient balance

		toolID := db.CreateTestTool(ownerID, "Tool for Extension Test", 1000) // $10/day

		// Step 1: Create rental request
		ctx1, cancel1 := ContextWithUserIDAndTimeout(renterID, 5*time.Second)
		defer cancel1()

		startDate := time.Now().Add(24 * time.Hour)
		endDate := startDate.Add(24 * time.Hour) // 2 days rental initially

		createReq := &pb.CreateRentalRequestRequest{
			ToolId:         toolID,
			StartDate:      startDate.Format("2006-01-02"),
			EndDate:        endDate.Format("2006-01-02"),
			OrganizationId: orgID,
		}

		createResp, err := rentalClient.CreateRentalRequest(ctx1, createReq)
		require.NoError(t, err)
		rentalID := createResp.RentalRequest.Id

		// Step 2: Owner approves
		ctx2, cancel2 := ContextWithUserIDAndTimeout(ownerID, 5*time.Second)
		defer cancel2()

		approveReq := &pb.ApproveRentalRequestRequest{
			RequestId:          rentalID,
			PickupInstructions: "Pick up location",
		}

		_, err = rentalClient.ApproveRentalRequest(ctx2, approveReq)
		require.NoError(t, err)

		// Step 3: Renter finalizes (payment)
		ctx3, cancel3 := ContextWithUserIDAndTimeout(renterID, 5*time.Second)
		defer cancel3()

		finalizeReq := &pb.FinalizeRentalRequestRequest{
			RequestId: rentalID,
		}

		_, err = rentalClient.FinalizeRentalRequest(ctx3, finalizeReq)
		require.NoError(t, err)

		// Step 4: Activate rental (owner marks as picked up)
		ctx4, cancel4 := ContextWithUserIDAndTimeout(ownerID, 5*time.Second)
		defer cancel4()

		activateReq := &pb.ActivateRentalRequest{
			RequestId: rentalID,
		}

		activateResp, err := rentalClient.ActivateRental(ctx4, activateReq)
		require.NoError(t, err)
		assert.Equal(t, pb.RentalStatus_RENTAL_STATUS_ACTIVE, activateResp.RentalRequest.Status)

		// Step 5: Renter submits INITIAL extension request to extend by 1 day
		ctx5, cancel5 := ContextWithUserIDAndTimeout(renterID, 5*time.Second)
		defer cancel5()

		firstExtensionDate := endDate.Add(24 * time.Hour) // Extend by 1 day (3 days total)

		changeReq1 := &pb.ChangeRentalDatesRequest{
			RequestId:  rentalID,
			NewEndDate: firstExtensionDate.Format("2006-01-02"),
		}

		changeResp1, err := rentalClient.ChangeRentalDates(ctx5, changeReq1)
		require.NoError(t, err, "Failed to submit initial extension request")
		assert.Equal(t, pb.RentalStatus_RENTAL_STATUS_RETURN_DATE_CHANGED, changeResp1.RentalRequest.Status)
		assert.NotEmpty(t, changeResp1.RentalRequest.EndDate, "EndDate should be populated with requested extension date")
		
		// Verify the first extension date was stored in end_date field
		var firstEndDate *time.Time
		var firstTotalCost int32
		err = db.QueryRow("SELECT end_date, total_cost_cents FROM rentals WHERE id = $1", rentalID).Scan(&firstEndDate, &firstTotalCost)
		require.NoError(t, err)
		require.NotNil(t, firstEndDate, "EndDate should not be null after extension request")
		
		firstEndDateStr := firstEndDate.Format("2006-01-02")
		expectedFirstDate := firstExtensionDate.Format("2006-01-02")
		assert.Equal(t, expectedFirstDate, firstEndDateStr, "First extension date should match requested date")
		// First extension: 3 days inclusive = 3000 cents
		assert.Equal(t, int32(3000), firstTotalCost, "Cost should be calculated for 3 days")

		t.Logf("✓ Initial extension request submitted successfully. Status: RETURN_DATE_CHANGED, EndDate: %s, Cost: %d cents", 
			firstEndDateStr, firstTotalCost)

		// Step 6: Renter UPDATES the extension request to extend by 2 days instead
		ctx6, cancel6 := ContextWithUserIDAndTimeout(renterID, 5*time.Second)
		defer cancel6()

		secondExtensionDate := endDate.Add(48 * time.Hour) // Extend by 2 days (4 days total)

		changeReq2 := &pb.ChangeRentalDatesRequest{
			RequestId:  rentalID,
			NewEndDate: secondExtensionDate.Format("2006-01-02"),
		}

		changeResp2, err := rentalClient.ChangeRentalDates(ctx6, changeReq2)
		require.NoError(t, err, "Failed to update extension request")
		assert.Equal(t, pb.RentalStatus_RENTAL_STATUS_RETURN_DATE_CHANGED, changeResp2.RentalRequest.Status, 
			"Status should remain RETURN_DATE_CHANGED")
		assert.NotEmpty(t, changeResp2.RentalRequest.EndDate, 
			"EndDate should be populated with updated extension date")

		// Step 7: VERIFY the end_date field was UPDATED in the database
		var updatedEndDate *time.Time
		var updatedTotalCost int32
		err = db.QueryRow("SELECT end_date, total_cost_cents FROM rentals WHERE id = $1", rentalID).Scan(&updatedEndDate, &updatedTotalCost)
		require.NoError(t, err)
		require.NotNil(t, updatedEndDate, "EndDate should still not be null after update")

		updatedEndDateStr := updatedEndDate.Format("2006-01-02")
		expectedSecondDate := secondExtensionDate.Format("2006-01-02")
		assert.Equal(t, expectedSecondDate, updatedEndDateStr, 
			"CRITICAL: EndDate should be UPDATED to the new requested date, not remain the old date")
		
		// Updated extension: 4 days inclusive = 4000 cents
		assert.Equal(t, int32(4000), updatedTotalCost, 
			"Cost should be recalculated for 4 days")

		// Verify it's NOT the old date
		assert.NotEqual(t, firstEndDateStr, updatedEndDateStr, 
			"EndDate MUST change from first extension to second extension")

		t.Logf("✓ Extension request updated successfully!")
		t.Logf("  - Status: %s", changeResp2.RentalRequest.Status)
		t.Logf("  - First extension date:  %s (cost: %d cents)", firstEndDateStr, firstTotalCost)
		t.Logf("  - Updated extension date: %s (cost: %d cents)", updatedEndDateStr, updatedTotalCost)
		t.Logf("  - EndDate field was successfully modified from %s to %s", firstEndDateStr, updatedEndDateStr)

		// Verify notification was sent to owner about the update
		var ownerNotifCount int
		err = db.QueryRow("SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND org_id = $2", ownerID, orgID).Scan(&ownerNotifCount)
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, ownerNotifCount, 2, "Owner should receive notifications for both the initial request and the update")

		// Optional: Owner can now approve the updated extension
		ctx7, cancel7 := ContextWithUserIDAndTimeout(ownerID, 5*time.Second)
		defer cancel7()

		approveExtensionReq := &pb.ApproveReturnDateChangeRequest{
			RequestId: rentalID,
		}

		approveExtensionResp, err := rentalClient.ApproveReturnDateChange(ctx7, approveExtensionReq)
		require.NoError(t, err, "Owner should be able to approve the updated extension")
		assert.NotNil(t, approveExtensionResp, "Approval response should not be nil")
		
		// After approval, last_agreed_end_date should be updated to the new end_date
		var lastAgreedEndDate *time.Time
		var finalEndDate time.Time
		err = db.QueryRow("SELECT last_agreed_end_date, end_date FROM rentals WHERE id = $1", rentalID).Scan(&lastAgreedEndDate, &finalEndDate)
		require.NoError(t, err)
		assert.NotNil(t, lastAgreedEndDate, "LastAgreedEndDate should be set after approval")
		assert.Equal(t, expectedSecondDate, finalEndDate.Format("2006-01-02"), 
			"EndDate should be the final agreed date")
		assert.Equal(t, expectedSecondDate, lastAgreedEndDate.Format("2006-01-02"), 
			"LastAgreedEndDate should match EndDate after approval")

		t.Logf("✓ Extension approved by owner. EndDate updated to: %s", finalEndDate.Format("2006-01-02"))
		t.Logf("\n=== TEST SUMMARY ===")
		t.Logf("This test demonstrates the complete workflow for updating an extension request:")
		t.Logf("1. Create rental and activate it (ACTIVE status)")
		t.Logf("2. Submit initial extension request (ACTIVE -> RETURN_DATE_CHANGED)")
		t.Logf("3. Update the extension request while still pending (RETURN_DATE_CHANGED -> RETURN_DATE_CHANGED)")
		t.Logf("4. Verify end_date field was modified in database")
		t.Logf("5. Owner approves the updated extension")
		t.Logf("\nCLIENT IMPLEMENTATION GUIDE:")
		t.Logf("- To submit extension: Call ChangeRentalDates() with new end date")
		t.Logf("- To update extension: Call ChangeRentalDates() AGAIN with different end date")
		t.Logf("- Check rental status is RETURN_DATE_CHANGED to know extension is pending")
		t.Logf("- The end_date field contains the current working date")
		t.Logf("- After owner approval, last_agreed_end_date is set to the agreed date")
	})

	t.Run("Cancel Rental Request", func(t *testing.T) {
		// Setup
		orgID := db.CreateTestOrg("")
		ownerID := db.CreateTestUser("e2e-test-owner3@test.com", "Owner 3")
		renterID := db.CreateTestUser("e2e-test-renter3@test.com", "Renter 3")

		db.AddUserToOrg(ownerID, orgID, "MEMBER", "ACTIVE", 0)
		db.AddUserToOrg(renterID, orgID, "MEMBER", "ACTIVE", 5000)

		toolID := db.CreateTestTool(ownerID, "Tool for Cancellation", 1000)

		// Create rental request
		ctx1, cancel1 := ContextWithUserIDAndTimeout(renterID, 5*time.Second)
		defer cancel1()

		startDate := time.Now().Add(24 * time.Hour)
		endDate := startDate.Add(24 * time.Hour)

		createReq := &pb.CreateRentalRequestRequest{
			ToolId:         toolID,
			StartDate:      startDate.Format("2006-01-02"),
			EndDate:        endDate.Format("2006-01-02"),
			OrganizationId: orgID,
		}

		createResp, err := rentalClient.CreateRentalRequest(ctx1, createReq)
		require.NoError(t, err)
		assert.NotNil(t, createResp.RentalRequest)
		rentalID := createResp.RentalRequest.Id

		// Renter cancels
		ctx2, cancel2 := ContextWithUserIDAndTimeout(renterID, 5*time.Second)
		defer cancel2()

		cancelReq := &pb.CancelRentalRequest{
			RequestId: rentalID,
			Reason:    "Changed my mind",
		}

		cancelResp, err := rentalClient.CancelRental(ctx2, cancelReq)
		require.NoError(t, err)
		assert.Equal(t, pb.RentalStatus_RENTAL_STATUS_CANCELLED, cancelResp.RentalRequest.Status)

		// Verify: Notification sent to owner
		var notifCount int
		err = db.QueryRow("SELECT COUNT(*) FROM notifications WHERE user_id = $1", ownerID).Scan(&notifCount)
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, notifCount, 1)
	})

	// Balance check is disabled for now
	// t.Run("CreateRentalRequest with Insufficient Balance", func(t *testing.T) {
	// 	// Setup: Renter with low balance
	// 	orgID := db.CreateTestOrg("")
	// 	ownerID := db.CreateTestUser("e2e-test-owner4@test.com", "Owner 4")
	// 	renterID := db.CreateTestUser("e2e-test-renter4@test.com", "Renter 4")

	// 	db.AddUserToOrg(ownerID, orgID, "MEMBER", "ACTIVE", 0)
	// 	db.AddUserToOrg(renterID, orgID, "MEMBER", "ACTIVE", 100) // Only 100 cents

	// 	toolID := db.CreateTestTool(ownerID, "Expensive Tool", 5000) // 5000 cents per day

	// 	// Try to create rental request
	// 	ctx, cancel := ContextWithUserIDAndTimeout(renterID, 5*time.Second)
	// 	defer cancel()

	// 	startDate := time.Now().Add(24 * time.Hour)
	// 	endDate := startDate.Add(24 * time.Hour)

	// 	createReq := &pb.CreateRentalRequestRequest{
	// 		ToolId:         toolID,
	// 		StartDate:      startDate.Format("2006-01-02"),
	// 		EndDate:        endDate.Format("2006-01-02"),
	// 		OrganizationId: orgID,
	// 	}

	// 	_, err := rentalClient.CreateRentalRequest(ctx, createReq)
	// 	// Should fail due to insufficient balance
	// 	assert.Error(t, err)
	// })
}
