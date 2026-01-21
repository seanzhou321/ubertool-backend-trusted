package e2e

import (
	"testing"
	"time"

	pb "ubertool-backend-trusted/api/gen/v1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLedgerService_E2E(t *testing.T) {
	db := PrepareDB(t)
	defer db.Close()
	defer db.Cleanup()

	client := NewGRPCClient(t, "")
	defer client.Close()

	ledgerClient := pb.NewLedgerServiceClient(client.Conn())

	t.Run("GetBalance", func(t *testing.T) {
		// Setup: Create user and org with balance
		userID := db.CreateTestUser("e2e-test-ledger-user@test.com", "Ledger User")
		orgID := db.CreateTestOrg("")
		db.AddUserToOrg(userID, orgID, "MEMBER", "ACTIVE", 5000)

		// Test: Get balance
		ctx, cancel := ContextWithUserIDAndTimeout(userID, 5*time.Second)
		defer cancel()

		req := &pb.GetBalanceRequest{
			OrganizationId: orgID,
		}

		resp, err := ledgerClient.GetBalance(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, int32(5000), resp.Balance)
	})

	t.Run("GetTransactions", func(t *testing.T) {
		// Setup: Create user, org, and transactions
		userID := db.CreateTestUser("e2e-test-ledger-user2@test.com", "Ledger User 2")
		orgID := db.CreateTestOrg("")
		db.AddUserToOrg(userID, orgID, "MEMBER", "ACTIVE", 0)

		// Create test transactions
		_, err := db.Exec(`
			INSERT INTO ledger_transactions (org_id, user_id, amount, type, description)
			VALUES ($1, $2, 1000, 'LENDING_CREDIT', 'Test credit'),
			       ($1, $2, -500, 'RENTAL_DEBIT', 'Test debit')
		`, orgID, userID)
		require.NoError(t, err)

		// Test: Get transactions
		ctx, cancel := ContextWithUserIDAndTimeout(userID, 5*time.Second)
		defer cancel()

		req := &pb.GetTransactionsRequest{
			OrganizationId: orgID,
			Page:           1,
			PageSize:       10,
		}

		resp, err := ledgerClient.GetTransactions(ctx, req)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(resp.Transactions), 2)
		assert.GreaterOrEqual(t, resp.TotalCount, int32(2))
	})

	t.Run("GetLedgerSummary", func(t *testing.T) {
		// Setup: Create user, org, and rental history
		userID := db.CreateTestUser("e2e-test-ledger-user3@test.com", "Ledger User 3")
		ownerID := db.CreateTestUser("e2e-test-owner-ledger@test.com", "Owner")
		orgID := db.CreateTestOrg("")
		db.AddUserToOrg(userID, orgID, "MEMBER", "ACTIVE", 3000)
		db.AddUserToOrg(ownerID, orgID, "MEMBER", "ACTIVE", 0)

		toolID := db.CreateTestTool(ownerID, "Summary Tool", 1000)

		// Create rental records with different statuses
		_, err := db.Exec(`
			INSERT INTO rentals (org_id, tool_id, renter_id, owner_id, start_date, scheduled_end_date, total_cost_cents, status)
			VALUES ($1, $2, $3, $4, CURRENT_DATE, CURRENT_DATE + 1, 1000, 'COMPLETED'),
			       ($1, $2, $3, $4, CURRENT_DATE + 2, CURRENT_DATE + 3, 1000, 'SCHEDULED'),
			       ($1, $2, $3, $4, CURRENT_DATE + 4, CURRENT_DATE + 5, 1000, 'PENDING')
		`, orgID, toolID, userID, ownerID)
		require.NoError(t, err)

		// Test: Get ledger summary
		ctx, cancel := ContextWithUserIDAndTimeout(userID, 5*time.Second)
		defer cancel()

		req := &pb.GetLedgerSummaryRequest{
			OrganizationId: orgID,
			NumberOfMonths: 3,
		}

		resp, err := ledgerClient.GetLedgerSummary(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, int32(3000), resp.Balance)

		// Verify rental counts
		assert.GreaterOrEqual(t, resp.StatusCount["COMPLETED"], int32(1))
		assert.GreaterOrEqual(t, resp.StatusCount["SCHEDULED"], int32(1))
		assert.GreaterOrEqual(t, resp.StatusCount["PENDING"], int32(1))
	})

	t.Run("Ledger Updates After Rental Completion", func(t *testing.T) {
		// This test verifies the ledger is properly updated through the rental workflow
		// Setup
		orgID := db.CreateTestOrg("")
		ownerID := db.CreateTestUser("e2e-test-ledger-owner@test.com", "Ledger Owner")
		renterID := db.CreateTestUser("e2e-test-ledger-renter@test.com", "Ledger Renter")

		db.AddUserToOrg(ownerID, orgID, "MEMBER", "ACTIVE", 0)
		db.AddUserToOrg(renterID, orgID, "MEMBER", "ACTIVE", 10000)

		toolID := db.CreateTestTool(ownerID, "Ledger Test Tool", 2000)

		// Create and finalize rental
		rentalClient := pb.NewRentalServiceClient(client.Conn())

		ctx1, cancel1 := ContextWithUserIDAndTimeout(renterID, 5*time.Second)
		defer cancel1()

		startDate := time.Now().Add(24 * time.Hour).Format("2006-01-02")
		endDate := time.Now().Add(48 * time.Hour).Format("2006-01-02")

		createReq := &pb.CreateRentalRequestRequest{
			ToolId:         toolID,
			StartDate:      startDate,
			EndDate:        endDate,
			OrganizationId: orgID,
		}

		createResp, err := rentalClient.CreateRentalRequest(ctx1, createReq)
		require.NoError(t, err)
		rentalID := createResp.RentalRequest.Id

		// Approve
		ctx2, cancel2 := ContextWithUserIDAndTimeout(ownerID, 5*time.Second)
		defer cancel2()

		approveReq := &pb.ApproveRentalRequestRequest{
			RequestId:          rentalID,
			PickupInstructions: "Test",
		}
		_, err = rentalClient.ApproveRentalRequest(ctx2, approveReq)
		require.NoError(t, err)

		// Finalize
		ctx3, cancel3 := ContextWithUserIDAndTimeout(renterID, 5*time.Second)
		defer cancel3()

		finalizeReq := &pb.FinalizeRentalRequestRequest{
			RequestId: rentalID,
		}
		_, err = rentalClient.FinalizeRentalRequest(ctx3, finalizeReq)
		require.NoError(t, err)

		// Check renter's balance after finalization
		ctx4, cancel4 := ContextWithUserIDAndTimeout(renterID, 5*time.Second)
		defer cancel4()

		balanceReq := &pb.GetBalanceRequest{
			OrganizationId: orgID,
		}
		balanceResp, err := ledgerClient.GetBalance(ctx4, balanceReq)
		require.NoError(t, err)
		// Should be 10000 - 4000 (2 days * 2000/day) = 6000
		assert.Equal(t, int32(6000), balanceResp.Balance)

		// Complete rental
		ctx5, cancel5 := ContextWithUserIDAndTimeout(ownerID, 5*time.Second)
		defer cancel5()

		completeReq := &pb.CompleteRentalRequest{
			RequestId: rentalID,
		}
		_, err = rentalClient.CompleteRental(ctx5, completeReq)
		require.NoError(t, err)

		// Check owner's balance after completion
		ctx6, cancel6 := ContextWithUserIDAndTimeout(ownerID, 5*time.Second)
		defer cancel6()

		ownerBalanceReq := &pb.GetBalanceRequest{
			OrganizationId: orgID,
		}
		ownerBalanceResp, err := ledgerClient.GetBalance(ctx6, ownerBalanceReq)
		require.NoError(t, err)
		// Owner should receive 4000
		assert.Equal(t, int32(4000), ownerBalanceResp.Balance)
	})
}

