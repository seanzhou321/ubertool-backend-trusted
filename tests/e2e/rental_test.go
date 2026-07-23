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
		env := setupRentalTestEnv(t, db, "lifecycle", 5000)
		start := time.Now().Add(24 * time.Hour)
		end := start.Add(48 * time.Hour) // 2-day rental (2 * 1000 = 2000 cents)

		rentalID := doCreateRentalRequest(t, rentalClient, env, start, end)
		assertNotifiedAtLeastOnce(t, db, env.ownerID, env.orgID)

		doApproveRentalRequest(t, rentalClient, env.ownerID, rentalID, "Pick up at my garage")
		assertNotifiedAtLeastOnce(t, db, env.renterID, env.orgID)

		doFinalizeRentalRequest(t, rentalClient, env.renterID, rentalID)
		assertBalance(t, db, env.renterID, env.orgID, 5000) // balance unchanged before completion
		assertLedgerCount(t, db, env.renterID, env.orgID, "LENDING_DEBIT", 0)
		assertToolStatus(t, db, env.toolID, "RENTED")

		doCompleteRental(t, rentalClient, env.ownerID, rentalID, true)
		assertBalance(t, db, env.renterID, env.orgID, 3000) // 5000 - 2000 cents
		assertBalance(t, db, env.ownerID, env.orgID, 2000)  // 2 days * 1000 cents/day
		assertLedgerCount(t, db, env.renterID, env.orgID, "LENDING_DEBIT", 1)
		assertLedgerCount(t, db, env.ownerID, env.orgID, "LENDING_CREDIT", 1)
		assertToolStatus(t, db, env.toolID, "AVAILABLE")
	})

	t.Run("Reject Rental Request", func(t *testing.T) {
		env := setupRentalTestEnv(t, db, "reject", 5000)
		start := time.Now().Add(24 * time.Hour)
		end := start.Add(24 * time.Hour)

		rentalID := doCreateRentalRequest(t, rentalClient, env, start, end)

		ctx, cancel := ContextWithUserIDAndTimeout(env.ownerID, 5*time.Second)
		defer cancel()
		resp, err := rentalClient.RejectRentalRequest(ctx, &pb.RejectRentalRequestRequest{
			RequestId: rentalID,
			Reason:    "Tool is not available",
		})
		require.NoError(t, err)
		assert.Equal(t, pb.RentalStatus_RENTAL_STATUS_REJECTED, resp.RentalRequest.Status)
		assertNotifiedAtLeastOnce(t, db, env.renterID, env.orgID)
	})

	t.Run("Update Extension Request While Pending (RETURN_DATE_CHANGED)", func(t *testing.T) {
		env := setupRentalTestEnv(t, db, "ext", 10000)
		start := time.Now().Add(24 * time.Hour)
		end := start.Add(24 * time.Hour) // initial: 1-day rental

		rentalID := doCreateRentalRequest(t, rentalClient, env, start, end)
		doApproveRentalRequest(t, rentalClient, env.ownerID, rentalID, "Pick up location")
		doFinalizeRentalRequest(t, rentalClient, env.renterID, rentalID)
		doActivateRental(t, rentalClient, env.ownerID, rentalID)

		// First extension: extend to 2-day duration (start -> start+2d = 2000 cents).
		ext1 := start.Add(48 * time.Hour)
		doChangeRentalDates(t, rentalClient, env.renterID, rentalID, ext1)
		assertExtensionDatesInDB(t, db, rentalID, ext1, 2000)
		ext1Str := ext1.Format("2006-01-02")

		// Second extension: overwrite to 3-day duration (start -> start+3d = 3000 cents).
		ext2 := start.Add(72 * time.Hour)
		doChangeRentalDates(t, rentalClient, env.renterID, rentalID, ext2)
		assertExtensionDatesInDB(t, db, rentalID, ext2, 3000)
		assert.NotEqual(t, ext1Str, ext2.Format("2006-01-02"), "end_date must update between extension requests")
		assertNotifiedAtLeastOnce(t, db, env.ownerID, env.orgID)

		// Owner approves the final extension; last_agreed_end_date must be set.
		doApproveReturnDateChange(t, rentalClient, env.ownerID, rentalID)
		var lastAgreed *time.Time
		var finalEnd time.Time
		err := db.QueryRow("SELECT last_agreed_end_date, end_date FROM rentals WHERE id = $1", rentalID).Scan(&lastAgreed, &finalEnd)
		require.NoError(t, err)
		require.NotNil(t, lastAgreed)
		assert.Equal(t, ext2.Format("2006-01-02"), finalEnd.Format("2006-01-02"))
		assert.Equal(t, ext2.Format("2006-01-02"), lastAgreed.Format("2006-01-02"))
	})

	// FR-005 (specs/005-rentals): RejectReturnDateChange and AcknowledgeReturnDateRejection are
	// exercised end-to-end for the first time here — prior to this test, neither RPC was ever
	// called from an integration or e2e test (only unit-tested with mocks).
	t.Run("Reject Extension Request with Counter-Proposal, Renter Acknowledges", func(t *testing.T) {
		env := setupRentalTestEnv(t, db, "rejectext", 10000)
		start := time.Now().Add(24 * time.Hour)
		end := start.Add(24 * time.Hour) // initial: 1-day rental (1000 cents)

		rentalID := doCreateRentalRequest(t, rentalClient, env, start, end)
		doApproveRentalRequest(t, rentalClient, env.ownerID, rentalID, "Pick up location")
		doFinalizeRentalRequest(t, rentalClient, env.renterID, rentalID)
		doActivateRental(t, rentalClient, env.ownerID, rentalID)

		// Renter requests a 3-day extension (3000 cents).
		requested := start.Add(72 * time.Hour)
		doChangeRentalDates(t, rentalClient, env.renterID, rentalID, requested)
		assertExtensionDatesInDB(t, db, rentalID, requested, 3000)

		// Owner rejects with a 2-day counter-proposal (2000 cents) instead.
		counter := start.Add(48 * time.Hour)
		doRejectReturnDateChange(t, rentalClient, env.ownerID, rentalID, "Tool needed sooner", counter)
		assertExtensionDatesInDB(t, db, rentalID, counter, 2000)
		assertNotifiedAtLeastOnce(t, db, env.renterID, env.orgID)

		// Renter acknowledges the rejection — rolls back to the last-agreed end date (the
		// original 1-day rental, since no extension was ever approved) and recomputes cost.
		rt := doAcknowledgeReturnDateRejection(t, rentalClient, env.renterID, rentalID)
		assert.Equal(t, pb.RentalStatus_RENTAL_STATUS_ACTIVE, rt.Status)
		assertExtensionDatesInDB(t, db, rentalID, start.Add(24*time.Hour), 1000)
	})

	// FR-005 (specs/005-rentals): CancelReturnDateChange is exercised end-to-end for the first
	// time here — prior to this test, no integration or e2e test ever called this RPC.
	t.Run("Cancel Extension Request Before Owner Acts", func(t *testing.T) {
		env := setupRentalTestEnv(t, db, "cancelext", 10000)
		start := time.Now().Add(24 * time.Hour)
		end := start.Add(24 * time.Hour) // initial: 1-day rental (1000 cents)

		rentalID := doCreateRentalRequest(t, rentalClient, env, start, end)
		doApproveRentalRequest(t, rentalClient, env.ownerID, rentalID, "Pick up location")
		doFinalizeRentalRequest(t, rentalClient, env.renterID, rentalID)
		doActivateRental(t, rentalClient, env.ownerID, rentalID)

		// Renter requests a 2-day extension (2000 cents), then changes their mind.
		requested := start.Add(48 * time.Hour)
		doChangeRentalDates(t, rentalClient, env.renterID, rentalID, requested)
		assertExtensionDatesInDB(t, db, rentalID, requested, 2000)

		// Cancelling rolls back to the last-agreed (original) end date and cost.
		rt := doCancelReturnDateChange(t, rentalClient, env.renterID, rentalID)
		assert.Equal(t, pb.RentalStatus_RENTAL_STATUS_ACTIVE, rt.Status)
		assertExtensionDatesInDB(t, db, rentalID, start.Add(24*time.Hour), 1000)
	})

	t.Run("Cancel Rental Request", func(t *testing.T) {
		env := setupRentalTestEnv(t, db, "cancel", 5000)
		start := time.Now().Add(24 * time.Hour)
		end := start.Add(24 * time.Hour)

		rentalID := doCreateRentalRequest(t, rentalClient, env, start, end)

		ctx, cancel := ContextWithUserIDAndTimeout(env.renterID, 5*time.Second)
		defer cancel()
		resp, err := rentalClient.CancelRental(ctx, &pb.CancelRentalRequest{
			RequestId: rentalID,
			Reason:    "Changed my mind",
		})
		require.NoError(t, err)
		assert.Equal(t, pb.RentalStatus_RENTAL_STATUS_CANCELLED, resp.RentalRequest.Status)
		assertNotifiedAtLeastOnce(t, db, env.ownerID, env.orgID)
	})

	// FR-006 (specs/005-rentals): GetRental had thorough L1 coverage but was never exercised
	// through the real gRPC handler against a live DB — prior to this test,
	// `grep -r GetRental tests/e2e tests/integration` returned nothing.
	t.Run("GetRental grants access to renter and owner, rejects others", func(t *testing.T) {
		env := setupRentalTestEnv(t, db, "getrental", 5000)
		start := time.Now().Add(24 * time.Hour)
		end := start.Add(24 * time.Hour)
		rentalID := doCreateRentalRequest(t, rentalClient, env, start, end)

		outsiderID := db.CreateTestUser("e2e-test-getrental-outsider@test.com", "Outsider")

		renterCtx, cancel := ContextWithUserIDAndTimeout(env.renterID, 5*time.Second)
		defer cancel()
		resp, err := rentalClient.GetRental(renterCtx, &pb.GetRentalRequest{RequestId: rentalID})
		require.NoError(t, err, "the renter must be granted access")
		assert.Equal(t, rentalID, resp.RentalRequest.Id)

		ownerCtx, cancel2 := ContextWithUserIDAndTimeout(env.ownerID, 5*time.Second)
		defer cancel2()
		resp, err = rentalClient.GetRental(ownerCtx, &pb.GetRentalRequest{RequestId: rentalID})
		require.NoError(t, err, "the owner must be granted access")
		assert.Equal(t, rentalID, resp.RentalRequest.Id)

		outsiderCtx, cancel3 := ContextWithUserIDAndTimeout(outsiderID, 5*time.Second)
		defer cancel3()
		_, err = rentalClient.GetRental(outsiderCtx, &pb.GetRentalRequest{RequestId: rentalID})
		require.Error(t, err, "a caller who is neither renter nor owner must be rejected")
	})

	// Balance check is disabled for now
	// t.Run("CreateRentalRequest with Insufficient Balance", func(t *testing.T) { ... })

	t.Run("Full Rental Lifecycle with charge_billsplit=false", func(t *testing.T) {
		env := setupRentalTestEnv(t, db, "nobs", 5000)
		start := time.Now().Add(24 * time.Hour)
		end := start.Add(48 * time.Hour) // 2-day rental (2 * 1000 = 2000 cents)

		rentalID := doCreateRentalRequest(t, rentalClient, env, start, end)
		doApproveRentalRequest(t, rentalClient, env.ownerID, rentalID, "Leave at front door")
		doFinalizeRentalRequest(t, rentalClient, env.renterID, rentalID)
		assertBalance(t, db, env.renterID, env.orgID, 5000) // unchanged before completion

		rt := doCompleteRental(t, rentalClient, env.ownerID, rentalID, false)
		assert.False(t, rt.ChargeBillsplit, "charge_billsplit must be persisted as false on the rental record")

		var chargeBillsplitInDB bool
		err := db.QueryRow("SELECT charge_billsplit FROM rentals WHERE id = $1", rentalID).Scan(&chargeBillsplitInDB)
		require.NoError(t, err)
		assert.False(t, chargeBillsplitInDB)

		// No balance changes and no ledger transactions when charge_billsplit=false.
		assertBalance(t, db, env.renterID, env.orgID, 5000)
		assertBalance(t, db, env.ownerID, env.orgID, 0)
		var txCount int
		err = db.QueryRow(
			"SELECT COUNT(*) FROM ledger_transactions WHERE org_id = $1 AND (user_id = $2 OR user_id = $3)",
			env.orgID, env.ownerID, env.renterID,
		).Scan(&txCount)
		require.NoError(t, err)
		assert.Equal(t, 0, txCount)
		assertToolStatus(t, db, env.toolID, "AVAILABLE")

		// assertNotifiedAtLeastOnce passes immediately (prior workflow steps already inserted
		// notifications). assertDirectSettlementReminders polls internally (up to 5 s) so the
		// fire-and-forget settlement goroutine has time to finish its DB writes.
		assertNotifiedAtLeastOnce(t, db, env.ownerID, env.orgID)
		assertNotifiedAtLeastOnce(t, db, env.renterID, env.orgID)
		assertDirectSettlementReminders(t, db, env.ownerID, env.renterID, env.orgID)
	})
}
