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
