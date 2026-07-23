package integration

import (
	"database/sql"
	"testing"
	"time"

	"ubertool-backend-trusted/internal/config"
	"ubertool-backend-trusted/internal/jobs"
	"ubertool-backend-trusted/internal/repository/postgres"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTakeBalanceSnapshots covers FR-001 (specs/008-bill-split): TakeBalanceSnapshots must
// snapshot every user_org's balance for the current settlement month, and must skip an
// already-settled (user, org, settlement_month) pairing rather than erroring or duplicating.
// Prior to this test, `grep -r TakeBalanceSnapshots tests/` returned nothing at any tier.
func TestTakeBalanceSnapshots(t *testing.T) {
	db := prepareDB(t)
	defer db.Close()

	jr := jobs.NewJobRunner(db, postgres.NewStore(db), nil, &config.Config{})
	orgID := createTestOrgForLedger(t, db)
	userID := createTestUserForLedger(t, db, "snapshot-user")
	addUserToOrgForLedgerWithBalance(t, db, userID, orgID, 12345)

	defer func() {
		db.Exec("DELETE FROM balance_snapshots WHERE org_id = $1", orgID)
		db.Exec("DELETE FROM users_orgs WHERE org_id = $1", orgID)
		db.Exec("DELETE FROM users WHERE id = $1", userID)
		db.Exec("DELETE FROM orgs WHERE id = $1", orgID)
	}()

	settlementMonth := time.Now().Format("2006-01")

	t.Run("Takes a snapshot of the current balance", func(t *testing.T) {
		jr.TakeBalanceSnapshots()

		var balance int32
		err := db.QueryRow(
			"SELECT balance_cents FROM balance_snapshots WHERE user_id = $1 AND org_id = $2 AND settlement_month = $3",
			userID, orgID, settlementMonth,
		).Scan(&balance)
		require.NoError(t, err)
		assert.EqualValues(t, 12345, balance)
	})

	t.Run("Skips an already-settled pairing on a second run", func(t *testing.T) {
		// Change the live balance after the first snapshot; a second run for the same month
		// must NOT overwrite the snapshot (ON CONFLICT DO NOTHING) or error.
		_, err := db.Exec("UPDATE users_orgs SET balance_cents = 99999 WHERE user_id = $1 AND org_id = $2", userID, orgID)
		require.NoError(t, err)

		jr.TakeBalanceSnapshots()

		var count int
		err = db.QueryRow(
			"SELECT COUNT(*) FROM balance_snapshots WHERE user_id = $1 AND org_id = $2 AND settlement_month = $3",
			userID, orgID, settlementMonth,
		).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "must not duplicate the snapshot row for an already-settled pairing")

		var balance int32
		err = db.QueryRow(
			"SELECT balance_cents FROM balance_snapshots WHERE user_id = $1 AND org_id = $2 AND settlement_month = $3",
			userID, orgID, settlementMonth,
		).Scan(&balance)
		require.NoError(t, err)
		assert.EqualValues(t, 12345, balance, "the original snapshot value must be preserved, not overwritten")
	})
}

// TestCheckOverdueBills covers FR-004 (specs/008-bill-split): a PENDING bill whose
// notice_sent_at is more than 10 days in the past must automatically transition to DISPUTED,
// with a dispute reason and a DISPUTE_OPENED bill_actions row — and a bill within the 10-day
// window must be left untouched. `grep -r CheckOverdueBills tests/` previously returned nothing.
func TestCheckOverdueBills(t *testing.T) {
	db := prepareDB(t)
	defer db.Close()

	jr := jobs.NewJobRunner(db, postgres.NewStore(db), nil, &config.Config{})
	orgID := createTestOrgForLedger(t, db)
	debtor := createTestUserForLedger(t, db, "overdue-debtor")
	creditor := createTestUserForLedger(t, db, "overdue-creditor")

	defer func() {
		db.Exec("DELETE FROM bill_actions WHERE bill_id IN (SELECT id FROM bills WHERE org_id = $1)", orgID)
		db.Exec("DELETE FROM bills WHERE org_id = $1", orgID)
		db.Exec("DELETE FROM users WHERE id IN ($1, $2)", debtor, creditor)
		db.Exec("DELETE FROM orgs WHERE id = $1", orgID)
	}()

	overdueBillID := createTestBill(t, db, orgID, debtor, creditor, "2026-01", "PENDING")
	_, err := db.Exec("UPDATE bills SET notice_sent_at = NOW() - INTERVAL '11 days' WHERE id = $1", overdueBillID)
	require.NoError(t, err)

	freshBillID := createTestBill(t, db, orgID, creditor, debtor, "2026-02", "PENDING")
	_, err = db.Exec("UPDATE bills SET notice_sent_at = NOW() - INTERVAL '2 days' WHERE id = $1", freshBillID)
	require.NoError(t, err)

	jr.CheckOverdueBills()

	var status, disputeReason string
	err = db.QueryRow("SELECT status, dispute_reason FROM bills WHERE id = $1", overdueBillID).Scan(&status, &disputeReason)
	require.NoError(t, err)
	assert.Equal(t, "DISPUTED", status, "a bill overdue by 10+ days must transition to DISPUTED")
	assert.Equal(t, "DEBTOR_NO_ACK", disputeReason)

	var actionCount int
	err = db.QueryRow("SELECT COUNT(*) FROM bill_actions WHERE bill_id = $1 AND action_type = 'DISPUTE_OPENED'", overdueBillID).Scan(&actionCount)
	require.NoError(t, err)
	assert.Equal(t, 1, actionCount, "must record a DISPUTE_OPENED action")

	err = db.QueryRow("SELECT status FROM bills WHERE id = $1", freshBillID).Scan(&status)
	require.NoError(t, err)
	assert.Equal(t, "PENDING", status, "a bill within the 10-day window must not be disputed")
}

// TestResolveDisputedBills covers FR-005 (specs/008-bill-split): a still-DISPUTED bill at
// month-end must automatically transition to SYSTEM_DEFAULT_ACTION/BOTH_FAULT, blocking the
// debtor from renting and the creditor from lending, with NO balance change — the detail that
// distinguishes this path from the admin-driven BOTH_FAULT resolution (FR-009), which does apply
// a balance penalty. `grep -r ResolveDisputedBills tests/` previously returned nothing.
func TestResolveDisputedBills(t *testing.T) {
	db := prepareDB(t)
	defer db.Close()

	jr := jobs.NewJobRunner(db, postgres.NewStore(db), nil, &config.Config{})
	orgID := createTestOrgForLedger(t, db)
	debtor := createTestUserForLedger(t, db, "resolve-debtor")
	creditor := createTestUserForLedger(t, db, "resolve-creditor")
	addUserToOrgForLedgerWithBalance(t, db, debtor, orgID, -5000)
	addUserToOrgForLedgerWithBalance(t, db, creditor, orgID, 5000)

	defer func() {
		db.Exec("DELETE FROM bill_actions WHERE bill_id IN (SELECT id FROM bills WHERE org_id = $1)", orgID)
		db.Exec("DELETE FROM bills WHERE org_id = $1", orgID)
		db.Exec("DELETE FROM users_orgs WHERE org_id = $1", orgID)
		db.Exec("DELETE FROM users WHERE id IN ($1, $2)", debtor, creditor)
		db.Exec("DELETE FROM orgs WHERE id = $1", orgID)
	}()

	settlementMonth := time.Now().Format("2006-01")
	billID := createTestBill(t, db, orgID, debtor, creditor, settlementMonth, "DISPUTED")

	jr.ResolveDisputedBills()

	var status, outcome string
	err := db.QueryRow("SELECT status, resolution_outcome FROM bills WHERE id = $1", billID).Scan(&status, &outcome)
	require.NoError(t, err)
	assert.Equal(t, "SYSTEM_DEFAULT_ACTION", status)
	assert.Equal(t, "BOTH_FAULT", outcome)

	var debtorRentingBlocked, creditorLendingBlocked bool
	var debtorBalance, creditorBalance int32
	err = db.QueryRow("SELECT renting_blocked, balance_cents FROM users_orgs WHERE user_id = $1 AND org_id = $2", debtor, orgID).
		Scan(&debtorRentingBlocked, &debtorBalance)
	require.NoError(t, err)
	err = db.QueryRow("SELECT lending_blocked, balance_cents FROM users_orgs WHERE user_id = $1 AND org_id = $2", creditor, orgID).
		Scan(&creditorLendingBlocked, &creditorBalance)
	require.NoError(t, err)

	assert.True(t, debtorRentingBlocked, "debtor must be blocked from renting")
	assert.True(t, creditorLendingBlocked, "creditor must be blocked from lending")
	assert.EqualValues(t, -5000, debtorBalance, "auto-resolution must NOT apply a balance penalty")
	assert.EqualValues(t, 5000, creditorBalance, "auto-resolution must NOT apply a balance penalty")

	var actionCount int
	err = db.QueryRow("SELECT COUNT(*) FROM bill_actions WHERE bill_id = $1 AND action_type = 'SYSTEM_AUTO_RESOLVE'", billID).Scan(&actionCount)
	require.NoError(t, err)
	assert.Equal(t, 1, actionCount)
}

func createTestBill(t *testing.T, db *sql.DB, orgID, debtorID, creditorID int32, settlementMonth, status string) int32 {
	t.Helper()
	var billID int32
	err := db.QueryRow(`
		INSERT INTO bills (org_id, debtor_user_id, creditor_user_id, amount_cents, settlement_month, status)
		VALUES ($1, $2, $3, 1000, $4, $5)
		RETURNING id
	`, orgID, debtorID, creditorID, settlementMonth, status).Scan(&billID)
	require.NoError(t, err)
	return billID
}
