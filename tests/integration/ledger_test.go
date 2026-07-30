package integration

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"ubertool-backend-trusted/internal/repository/postgres"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLedgerRepository_GetTransactions_IsolationOrderingPagination covers FR-002
// (specs/007-ledger): GetTransactions must return only the caller's own transactions in the
// given org, most recent first, with an accurate total_count independent of the current page.
// Prior to this test, every tier had *some* test that got a non-empty result back, but none
// actually locked in isolation, ordering, or pagination — a second user's rows, distinct
// timestamps, and a second page were never exercised.
func TestLedgerRepository_GetTransactions_IsolationOrderingPagination(t *testing.T) {
	db := prepareDB(t)
	defer db.Close()

	repo := postgres.NewLedgerRepository(db)
	ctx := context.Background()

	orgID := createTestOrgForLedger(t, db)
	userA := createTestUserForLedger(t, db, "ledger-a")
	userB := createTestUserForLedger(t, db, "ledger-b")
	addUserToOrgForLedger(t, db, userA, orgID)
	addUserToOrgForLedger(t, db, userB, orgID)

	defer func() {
		db.Exec("DELETE FROM ledger_transactions WHERE org_id = $1", orgID)
		db.Exec("DELETE FROM users_orgs WHERE org_id = $1", orgID)
		db.Exec("DELETE FROM users WHERE id IN ($1, $2)", userA, userB)
		db.Exec("DELETE FROM orgs WHERE id = $1", orgID)
	}()

	// User A gets 3 transactions on 3 distinct, explicit dates (oldest to newest).
	dates := []string{"2026-01-01", "2026-01-02", "2026-01-03"}
	for i, d := range dates {
		_, err := db.Exec(`
			INSERT INTO ledger_transactions (org_id, user_id, amount, type, description, charged_on, created_on)
			VALUES ($1, $2, $3, 'CHARGE', $4, $5, $5)
		`, orgID, userA, (i+1)*100, fmt.Sprintf("txn-%d", i), d)
		require.NoError(t, err)
	}

	// User B gets one transaction in the same org — must never appear in A's results.
	_, err := db.Exec(`
		INSERT INTO ledger_transactions (org_id, user_id, amount, type, description, charged_on, created_on)
		VALUES ($1, $2, 9999, 'CHARGE', 'not-mine', '2026-01-02', '2026-01-02')
	`, orgID, userB)
	require.NoError(t, err)

	t.Run("Isolation: excludes another user's transactions in the same org", func(t *testing.T) {
		txs, total, err := repo.ListTransactions(ctx, userA, orgID, 1, 10)
		require.NoError(t, err)
		assert.EqualValues(t, 3, total)
		for _, tx := range txs {
			assert.Equal(t, userA, tx.UserID)
			assert.NotEqual(t, "not-mine", tx.Description)
		}
	})

	t.Run("Ordering: most recent first", func(t *testing.T) {
		txs, _, err := repo.ListTransactions(ctx, userA, orgID, 1, 10)
		require.NoError(t, err)
		require.Len(t, txs, 3)
		assert.Equal(t, "2026-01-03", txs[0].CreatedOn, "newest transaction must come first")
		assert.Equal(t, "2026-01-02", txs[1].CreatedOn)
		assert.Equal(t, "2026-01-01", txs[2].CreatedOn, "oldest transaction must come last")
	})

	t.Run("Pagination: total_count stays accurate across pages, page 2 returns the remainder", func(t *testing.T) {
		page1, total1, err := repo.ListTransactions(ctx, userA, orgID, 1, 2)
		require.NoError(t, err)
		assert.EqualValues(t, 3, total1)
		require.Len(t, page1, 2)

		page2, total2, err := repo.ListTransactions(ctx, userA, orgID, 2, 2)
		require.NoError(t, err)
		assert.EqualValues(t, 3, total2, "total_count must not change on a later page")
		require.Len(t, page2, 1)

		// No overlap between the two pages.
		assert.NotEqual(t, page1[0].ID, page2[0].ID)
		assert.NotEqual(t, page1[1].ID, page2[0].ID)
	})
}

// TestLedgerRepository_GetBalance covers FR-001 (specs/007-ledger): GetBalance must return the
// caller's balance_cents for the given org. Prior to this test, the only real-DB ledger
// integration test (TestRentalAndLedger_Integration) never called GetBalance directly — the
// happy path was only proven via a mocked repo (L1) and a before/after-settlement e2e comparison
// (L3), with no test exercising ledgerRepo.GetBalance against a real Postgres instance.
func TestLedgerRepository_GetBalance(t *testing.T) {
	db := prepareDB(t)
	defer db.Close()

	repo := postgres.NewLedgerRepository(db)
	ctx := context.Background()

	orgID := createTestOrgForLedger(t, db)
	user := createTestUserForLedger(t, db, "ledger-balance-user")
	otherOrgUser := createTestUserForLedger(t, db, "ledger-balance-other")
	addUserToOrgForLedgerWithBalance(t, db, user, orgID, 7500)

	defer func() {
		db.Exec("DELETE FROM users_orgs WHERE org_id = $1", orgID)
		db.Exec("DELETE FROM users WHERE id IN ($1, $2)", user, otherOrgUser)
		db.Exec("DELETE FROM orgs WHERE id = $1", orgID)
	}()

	t.Run("Returns the caller's balance_cents for the given org", func(t *testing.T) {
		balance, err := repo.GetBalance(ctx, user, orgID)
		require.NoError(t, err)
		assert.EqualValues(t, 7500, balance)
	})

	t.Run("Errors for a user with no membership in the org", func(t *testing.T) {
		_, err := repo.GetBalance(ctx, otherOrgUser, orgID)
		require.Error(t, err)
	})
}

// TestLedgerRepository_GetSummary covers FR-003 (specs/007-ledger): GetLedgerSummary must
// return the caller's balance and a per-status count of their rentals (as renter OR owner) in
// the given org. Prior to this test, GetLedgerSummary had zero unit or integration coverage —
// only a single e2e happy-path subtest with one concrete org, asserting three individual
// StatusCount entries but never the full status breakdown or the renter+owner union.
func TestLedgerRepository_GetSummary(t *testing.T) {
	db := prepareDB(t)
	defer db.Close()

	repo := postgres.NewLedgerRepository(db)
	ctx := context.Background()

	orgID := createTestOrgForLedger(t, db)
	user := createTestUserForLedger(t, db, "ledger-summary-user")
	otherUser := createTestUserForLedger(t, db, "ledger-summary-other")
	addUserToOrgForLedgerWithBalance(t, db, user, orgID, 4200)
	addUserToOrgForLedgerWithBalance(t, db, otherUser, orgID, 0)

	toolAsOwner := createTestToolForLedger(t, db, user)
	toolAsOtherOwner := createTestToolForLedger(t, db, otherUser)

	defer func() {
		db.Exec("DELETE FROM rentals WHERE org_id = $1", orgID)
		db.Exec("DELETE FROM tools WHERE id IN ($1, $2)", toolAsOwner, toolAsOtherOwner)
		db.Exec("DELETE FROM users_orgs WHERE org_id = $1", orgID)
		db.Exec("DELETE FROM users WHERE id IN ($1, $2)", user, otherUser)
		db.Exec("DELETE FROM orgs WHERE id = $1", orgID)
	}()

	// user as renter: one ACTIVE, one PENDING.
	createTestRentalForLedger(t, db, orgID, toolAsOtherOwner, user, otherUser, "ACTIVE")
	createTestRentalForLedger(t, db, orgID, toolAsOtherOwner, user, otherUser, "PENDING")
	// user as owner (lending): one ACTIVE.
	createTestRentalForLedger(t, db, orgID, toolAsOwner, otherUser, user, "ACTIVE")
	// A rental involving neither user as renter nor owner in a *different* org must never count —
	// simulated here by a rental in the same org between two unrelated participants being absent
	// (isolation is implicit: the query only ever matches rows where the user is renter or owner).

	summary, err := repo.GetSummary(ctx, user, orgID)
	require.NoError(t, err)

	assert.EqualValues(t, 4200, summary.Balance)
	assert.EqualValues(t, 1, summary.ActiveRentalsCount, "one ACTIVE rental as renter")
	assert.EqualValues(t, 1, summary.ActiveLendingsCount, "one ACTIVE rental as owner")
	assert.EqualValues(t, 1, summary.PendingRequestsCount)
	// StatusCount must aggregate across BOTH the renter and owner roles: 2 ACTIVE total (one as
	// renter, one as owner) + 1 PENDING.
	assert.EqualValues(t, 2, summary.StatusCount["ACTIVE"], "StatusCount must combine renter-role and owner-role rentals")
	assert.EqualValues(t, 1, summary.StatusCount["PENDING"])
}

// TestLedgerRepository_GetSummary_CrossOrgRollup covers FR-004 (specs/007-ledger, multi-org):
// when organization_id is omitted (0), GetLedgerSummary MUST roll up balance and per-status
// rental counts across ALL orgs the caller belongs to, not error with "no rows" (Known
// Discrepancy 2 / RTM Gap). Prior to this test, no test anywhere called this code path with
// organization_id=0 — this is the real-DB proof the aggregation SQL is correct, mirroring
// TestLedgerRepository_GetSummary's renter+owner-union pattern above but across two orgs.
func TestLedgerRepository_GetSummary_CrossOrgRollup(t *testing.T) {
	db := prepareDB(t)
	defer db.Close()

	repo := postgres.NewLedgerRepository(db)
	ctx := context.Background()

	orgA := createTestOrgForLedger(t, db)
	orgB := createTestOrgForLedger(t, db)
	orgC := createTestOrgForLedger(t, db) // user is NOT a member here — must never leak in

	user := createTestUserForLedger(t, db, "ledger-rollup-user")
	otherUser := createTestUserForLedger(t, db, "ledger-rollup-other")
	strangerInOrgC := createTestUserForLedger(t, db, "ledger-rollup-stranger")

	addUserToOrgForLedgerWithBalance(t, db, user, orgA, 4200)
	addUserToOrgForLedgerWithBalance(t, db, user, orgB, 800)
	addUserToOrgForLedger(t, db, otherUser, orgA)
	addUserToOrgForLedger(t, db, otherUser, orgB)
	addUserToOrgForLedgerWithBalance(t, db, strangerInOrgC, orgC, 999999)

	toolInA := createTestToolForLedger(t, db, otherUser)
	toolInB := createTestToolForLedger(t, db, user)
	toolInC := createTestToolForLedger(t, db, strangerInOrgC)

	defer func() {
		db.Exec("DELETE FROM rentals WHERE org_id IN ($1, $2, $3)", orgA, orgB, orgC)
		db.Exec("DELETE FROM tools WHERE id IN ($1, $2, $3)", toolInA, toolInB, toolInC)
		db.Exec("DELETE FROM users_orgs WHERE org_id IN ($1, $2, $3)", orgA, orgB, orgC)
		db.Exec("DELETE FROM users WHERE id IN ($1, $2, $3)", user, otherUser, strangerInOrgC)
		db.Exec("DELETE FROM orgs WHERE id IN ($1, $2, $3)", orgA, orgB, orgC)
	}()

	// Org A: user as renter, one ACTIVE.
	createTestRentalForLedger(t, db, orgA, toolInA, user, otherUser, "ACTIVE")
	// Org B: user as owner (lending), one ACTIVE; plus one PENDING as renter.
	createTestRentalForLedger(t, db, orgB, toolInB, otherUser, user, "ACTIVE")
	createTestRentalForLedger(t, db, orgB, toolInA, user, otherUser, "PENDING")
	// Org C: a rental the user has no part in at all — must never count.
	createTestRentalForLedger(t, db, orgC, toolInC, otherUser, strangerInOrgC, "ACTIVE")

	summary, err := repo.GetSummaryAllOrgs(ctx, user)
	require.NoError(t, err, "must roll up across orgs instead of erroring on organization_id=0")

	assert.EqualValues(t, 5000, summary.Balance, "balance must sum across both of the user's orgs (4200+800)")
	assert.EqualValues(t, 1, summary.ActiveRentalsCount, "one ACTIVE rental as renter, in org A")
	assert.EqualValues(t, 1, summary.ActiveLendingsCount, "one ACTIVE rental as owner, in org B")
	assert.EqualValues(t, 1, summary.PendingRequestsCount, "one PENDING rental as renter, in org B")
	assert.EqualValues(t, 2, summary.StatusCount["ACTIVE"], "ACTIVE count must combine both orgs and both roles")
	assert.EqualValues(t, 1, summary.StatusCount["PENDING"])
}

func createTestOrgForLedger(t *testing.T, db *sql.DB) int32 {
	t.Helper()
	var orgID int32
	err := db.QueryRow(`
		INSERT INTO orgs (name, metro, admin_email, admin_phone_number, address)
		VALUES ($1, 'San Jose', 'admin@test.com', '555-0000', '123 Test St')
		RETURNING id
	`, fmt.Sprintf("test-integration-ledger-org-%d", time.Now().UnixNano())).Scan(&orgID)
	require.NoError(t, err)
	return orgID
}

func createTestUserForLedger(t *testing.T, db *sql.DB, prefix string) int32 {
	t.Helper()
	var userID int32
	email := fmt.Sprintf("test-integration-%s-%d@test.com", prefix, time.Now().UnixNano())
	err := db.QueryRow(`
		INSERT INTO users (email, phone_number, password_hash, name)
		VALUES ($1, $2, 'hash', $3) RETURNING id
	`, email, fmt.Sprintf("555-%d", time.Now().UnixNano()), prefix).Scan(&userID)
	require.NoError(t, err)
	return userID
}

func addUserToOrgForLedger(t *testing.T, db *sql.DB, userID, orgID int32) {
	t.Helper()
	addUserToOrgForLedgerWithBalance(t, db, userID, orgID, 0)
}

func addUserToOrgForLedgerWithBalance(t *testing.T, db *sql.DB, userID, orgID, balanceCents int32) {
	t.Helper()
	_, err := db.Exec(`
		INSERT INTO users_orgs (user_id, org_id, role, status, balance_cents)
		VALUES ($1, $2, 'MEMBER', 'ACTIVE', $3)
	`, userID, orgID, balanceCents)
	require.NoError(t, err)
}

func createTestToolForLedger(t *testing.T, db *sql.DB, ownerID int32) int32 {
	t.Helper()
	var toolID int32
	err := db.QueryRow(`
		INSERT INTO tools (owner_id, name, description, price_per_day_cents, price_per_week_cents, price_per_month_cents, duration_unit, condition, metro, status)
		VALUES ($1, $2, 'test tool', 1000, 6000, 20000, 'day', 'EXCELLENT', 'San Jose', 'AVAILABLE')
		RETURNING id
	`, ownerID, fmt.Sprintf("test-integration-ledger-tool-%d", time.Now().UnixNano())).Scan(&toolID)
	require.NoError(t, err)
	return toolID
}

func createTestRentalForLedger(t *testing.T, db *sql.DB, orgID, toolID, renterID, ownerID int32, status string) int32 {
	t.Helper()
	var rentalID int32
	err := db.QueryRow(`
		INSERT INTO rentals (org_id, tool_id, renter_id, owner_id, start_date, end_date, duration_unit,
			daily_price_cents, weekly_price_cents, monthly_price_cents, replacement_cost_cents, total_cost_cents, status)
		VALUES ($1, $2, $3, $4, CURRENT_DATE, CURRENT_DATE + 2, 'day', 1000, 6000, 20000, 5000, 2000, $5)
		RETURNING id
	`, orgID, toolID, renterID, ownerID, status).Scan(&rentalID)
	require.NoError(t, err)
	return rentalID
}
