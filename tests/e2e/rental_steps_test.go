package e2e

import (
	"testing"
	"time"

	pb "ubertool-backend-trusted/api/gen/v1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rentalTestEnv holds the shared setup state for a rental sub-test.
type rentalTestEnv struct {
	orgID, ownerID, renterID, toolID int32
}

// setupRentalTestEnv creates a fresh org, owner, renter, and tool for a sub-test.
// emailSuffix must be unique per sub-test and is embedded in the email addresses as
// e2e-test-<suffix>-{owner,renter}@test.com, which the Cleanup helper will sweep up.
func setupRentalTestEnv(t *testing.T, db *TestDB, emailSuffix string, renterInitialBalance int32) rentalTestEnv {
	t.Helper()
	orgID := db.CreateTestOrg("")
	ownerID := db.CreateTestUser("e2e-test-"+emailSuffix+"-owner@test.com", "Owner")
	renterID := db.CreateTestUser("e2e-test-"+emailSuffix+"-renter@test.com", "Renter")
	db.AddUserToOrg(ownerID, orgID, "MEMBER", "ACTIVE", 0)
	db.AddUserToOrg(renterID, orgID, "MEMBER", "ACTIVE", renterInitialBalance)
	toolID := db.CreateTestTool(ownerID, "Tool "+emailSuffix, 1000) // 1000 cents/day
	return rentalTestEnv{orgID, ownerID, renterID, toolID}
}

// doCreateRentalRequest calls CreateRentalRequest as the renter and returns the rentalID.
func doCreateRentalRequest(t *testing.T, client pb.RentalServiceClient, env rentalTestEnv, startDate, endDate time.Time) int32 {
	t.Helper()
	ctx, cancel := ContextWithUserIDAndTimeout(env.renterID, 5*time.Second)
	defer cancel()
	resp, err := client.CreateRentalRequest(ctx, &pb.CreateRentalRequestRequest{
		ToolId:         env.toolID,
		StartDate:      startDate.Format("2006-01-02"),
		EndDate:        endDate.Format("2006-01-02"),
		OrganizationId: env.orgID,
	})
	require.NoError(t, err)
	require.Equal(t, pb.RentalStatus_RENTAL_STATUS_PENDING, resp.RentalRequest.Status)
	return resp.RentalRequest.Id
}

// doApproveRentalRequest calls ApproveRentalRequest as the owner.
func doApproveRentalRequest(t *testing.T, client pb.RentalServiceClient, ownerID, rentalID int32, pickupNote string) {
	t.Helper()
	ctx, cancel := ContextWithUserIDAndTimeout(ownerID, 5*time.Second)
	defer cancel()
	resp, err := client.ApproveRentalRequest(ctx, &pb.ApproveRentalRequestRequest{
		RequestId:          rentalID,
		PickupInstructions: pickupNote,
	})
	require.NoError(t, err)
	require.Equal(t, pb.RentalStatus_RENTAL_STATUS_APPROVED, resp.RentalRequest.Status)
}

// doFinalizeRentalRequest calls FinalizeRentalRequest as the renter.
func doFinalizeRentalRequest(t *testing.T, client pb.RentalServiceClient, renterID, rentalID int32) {
	t.Helper()
	ctx, cancel := ContextWithUserIDAndTimeout(renterID, 5*time.Second)
	defer cancel()
	resp, err := client.FinalizeRentalRequest(ctx, &pb.FinalizeRentalRequestRequest{RequestId: rentalID})
	require.NoError(t, err)
	require.Equal(t, pb.RentalStatus_RENTAL_STATUS_SCHEDULED, resp.RentalRequest.Status)
}

// doActivateRental calls ActivateRental as the owner.
func doActivateRental(t *testing.T, client pb.RentalServiceClient, ownerID, rentalID int32) {
	t.Helper()
	ctx, cancel := ContextWithUserIDAndTimeout(ownerID, 5*time.Second)
	defer cancel()
	resp, err := client.ActivateRental(ctx, &pb.ActivateRentalRequest{RequestId: rentalID})
	require.NoError(t, err)
	require.Equal(t, pb.RentalStatus_RENTAL_STATUS_ACTIVE, resp.RentalRequest.Status)
}

// doCompleteRental calls CompleteRental as the owner with a standard return condition.
func doCompleteRental(t *testing.T, client pb.RentalServiceClient, ownerID, rentalID int32, chargeBillsplit bool) *pb.RentalRequest {
	t.Helper()
	ctx, cancel := ContextWithUserIDAndTimeout(ownerID, 5*time.Second)
	defer cancel()
	resp, err := client.CompleteRental(ctx, &pb.CompleteRentalRequest{
		RequestId:              rentalID,
		ReturnCondition:        "Good condition",
		SurchargeOrCreditCents: 0,
		ChargeBillsplit:        chargeBillsplit,
	})
	require.NoError(t, err)
	require.Equal(t, pb.RentalStatus_RENTAL_STATUS_COMPLETED, resp.RentalRequest.Status)
	return resp.RentalRequest
}

// doChangeRentalDates calls ChangeRentalDates as the renter and asserts the rental enters
// RETURN_DATE_CHANGED status.
func doChangeRentalDates(t *testing.T, client pb.RentalServiceClient, renterID, rentalID int32, newEndDate time.Time) *pb.RentalRequest {
	t.Helper()
	ctx, cancel := ContextWithUserIDAndTimeout(renterID, 5*time.Second)
	defer cancel()
	resp, err := client.ChangeRentalDates(ctx, &pb.ChangeRentalDatesRequest{
		RequestId:  rentalID,
		NewEndDate: newEndDate.Format("2006-01-02"),
	})
	require.NoError(t, err)
	require.Equal(t, pb.RentalStatus_RENTAL_STATUS_RETURN_DATE_CHANGED, resp.RentalRequest.Status)
	return resp.RentalRequest
}

// doApproveReturnDateChange calls ApproveReturnDateChange as the owner.
func doApproveReturnDateChange(t *testing.T, client pb.RentalServiceClient, ownerID, rentalID int32) {
	t.Helper()
	ctx, cancel := ContextWithUserIDAndTimeout(ownerID, 5*time.Second)
	defer cancel()
	_, err := client.ApproveReturnDateChange(ctx, &pb.ApproveReturnDateChangeRequest{RequestId: rentalID})
	require.NoError(t, err)
}

// ── Assertion helpers ──────────────────────────────────────────────────────────

// assertBalance verifies a user's balance_cents in the given org.
func assertBalance(t *testing.T, db *TestDB, userID, orgID int32, expected int32) {
	t.Helper()
	var got int32
	err := db.QueryRow(
		"SELECT balance_cents FROM users_orgs WHERE user_id = $1 AND org_id = $2",
		userID, orgID,
	).Scan(&got)
	require.NoError(t, err)
	assert.Equal(t, expected, got)
}

// assertToolStatus verifies the status column of a tool.
func assertToolStatus(t *testing.T, db *TestDB, toolID int32, wantStatus string) {
	t.Helper()
	var got string
	err := db.QueryRow("SELECT status FROM tools WHERE id = $1", toolID).Scan(&got)
	require.NoError(t, err)
	assert.Equal(t, wantStatus, got)
}

// assertLedgerCount verifies the number of ledger transactions of a given type for a user/org.
func assertLedgerCount(t *testing.T, db *TestDB, userID, orgID int32, txType string, want int) {
	t.Helper()
	var got int
	err := db.QueryRow(
		"SELECT COUNT(*) FROM ledger_transactions WHERE user_id = $1 AND org_id = $2 AND type = $3",
		userID, orgID, txType,
	).Scan(&got)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

// assertNotifiedAtLeastOnce verifies there is at least one notification for a user/org pair.
func assertNotifiedAtLeastOnce(t *testing.T, db *TestDB, userID, orgID int32) {
	t.Helper()
	var got int
	err := db.QueryRow(
		"SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND org_id = $2",
		userID, orgID,
	).Scan(&got)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, got, 1)
}

// pollDB retries f until it returns true or timeout elapses, sleeping pollInterval between attempts.
// Use this to wait for fire-and-forget server goroutines to persist data to the DB without a fixed sleep.
func pollDB(timeout, pollInterval time.Duration, f func() bool) bool {
	deadline := time.Now().Add(timeout)
	for {
		if f() {
			return true
		}
		if time.Now().Add(pollInterval).After(deadline) {
			return false
		}
		time.Sleep(pollInterval)
	}
}

// assertDirectSettlementReminders verifies that the settlement notifications for a
// charge_billsplit=false rental contain the reminder text directing parties to settle directly.
// Polls the DB for up to 5 seconds so that fire-and-forget goroutines have time to commit
// their DB writes before the assertion is evaluated — no fixed sleep required at the call site.
func assertDirectSettlementReminders(t *testing.T, db *TestDB, ownerID, renterID, orgID int32) {
	t.Helper()

	ownerOK := pollDB(5*time.Second, 100*time.Millisecond, func() bool {
		var count int
		db.QueryRow(`
			SELECT COUNT(*) FROM notifications
			WHERE user_id = $1 AND org_id = $2
			  AND message LIKE '%settled directly between you and the renter%'
		`, ownerID, orgID).Scan(&count) //nolint:errcheck
		return count >= 1
	})
	assert.True(t, ownerOK, "Owner credit notification must contain the direct-settlement reminder")

	renterOK := pollDB(5*time.Second, 100*time.Millisecond, func() bool {
		var count int
		db.QueryRow(`
			SELECT COUNT(*) FROM notifications
			WHERE user_id = $1 AND org_id = $2
			  AND message LIKE '%settled directly between you and the owner%'
		`, renterID, orgID).Scan(&count) //nolint:errcheck
		return count >= 1
	})
	assert.True(t, renterOK, "Renter debit notification must contain the direct-settlement reminder")
}

// assertExtensionDatesInDB checks that end_date and total_cost_cents stored in the DB
// match the expected values after a ChangeRentalDates call.
func assertExtensionDatesInDB(t *testing.T, db *TestDB, rentalID int32, expectedEndDate time.Time, expectedCostCents int32) {
	t.Helper()
	var endDate *time.Time
	var cost int32
	err := db.QueryRow(
		"SELECT end_date, total_cost_cents FROM rentals WHERE id = $1", rentalID,
	).Scan(&endDate, &cost)
	require.NoError(t, err)
	require.NotNil(t, endDate, "end_date must not be null after an extension request")
	assert.Equal(t, expectedEndDate.Format("2006-01-02"), endDate.Format("2006-01-02"))
	assert.Equal(t, expectedCostCents, cost)
}
