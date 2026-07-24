package integration

import (
	"context"
	"sync"
	"testing"

	"ubertool-backend-trusted/internal/repository/postgres"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// TestUserOrgRepository_AdjustBalance_ConcurrentCallsDoNotLoseAnUpdate is the regression test
// for the fix to SEC-BILL-004/006 (sbr/rtm/009-security.rtm.md): billSplitService.updateBalances
// and its penalize-and-block siblings (internal/service/bill_split.go) used to do a
// GetUserOrg-then-UpdateUserOrg read-modify-write of users_orgs.balance_cents — vulnerable to a
// lost update when two settlements touched the same row concurrently, since UpdateUserOrg wrote
// balance_cents as an absolute value computed from a possibly-stale in-memory read. They now
// call UserRepository.AdjustBalance, which applies its delta via a single atomic
// "balance_cents = balance_cents + $1" UPDATE (internal/repository/postgres/user.go) instead.
//
// This test fires two goroutines concurrently against the real Postgres instance — a genuine
// race, not a simulated one — and asserts the final balance reflects both deltas. It is not
// flaky despite being timing-based: unlike the old read-modify-write pattern, a single atomic
// UPDATE statement is serialized by Postgres's own row-level locking regardless of which
// goroutine's statement reaches the row first, so the two deltas always compose correctly no
// matter how the scheduler interleaves them.
func TestUserOrgRepository_AdjustBalance_ConcurrentCallsDoNotLoseAnUpdate(t *testing.T) {
	db := prepareDB(t)
	defer db.Close()
	ctx := context.Background()

	userRepo := postgres.NewUserRepository(db)

	orgID := createTestOrgForLedger(t, db)
	userID := createTestUserForLedger(t, db, "balance-race")
	addUserToOrgForLedgerWithBalance(t, db, userID, orgID, 1000)

	defer func() {
		db.Exec("DELETE FROM users_orgs WHERE user_id = $1 AND org_id = $2", userID, orgID)
		db.Exec("DELETE FROM users WHERE id = $1", userID)
		db.Exec("DELETE FROM orgs WHERE id = $1", orgID)
	}()

	// Simulate two settlements landing on the same balance at the same time: this user credited
	// +500 on one bill (e.g. as a creditor whose counterparty just acknowledged) and debited
	// -300 on a different bill (e.g. as a debtor on a bill an admin just resolved) — exactly
	// what a real concurrent AcknowledgePayment + ResolveDispute pair would do.
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		errs <- userRepo.AdjustBalance(ctx, userID, orgID, 500)
	}()
	go func() {
		defer wg.Done()
		errs <- userRepo.AdjustBalance(ctx, userID, orgID, -300)
	}()
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}

	final, err := userRepo.GetUserOrg(ctx, userID, orgID)
	require.NoError(t, err)

	const correctBalance = int32(1000 + 500 - 300) // 1200 — both concurrent deltas must compose
	require.Equal(t, correctBalance, final.BalanceCents,
		"AdjustBalance must not lose an update under real concurrent calls — expected both the +500 and -300 deltas to apply (1200), got %d", final.BalanceCents)
}
