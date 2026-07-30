package unit

import (
	"context"
	"testing"

	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/service"
	"github.com/stretchr/testify/assert"
)

func TestLedgerService_GetBalance(t *testing.T) {
	repo := new(MockLedgerRepo)
	svc := service.NewLedgerService(repo)
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		repo.On("GetBalance", ctx, int32(1), int32(2)).Return(int32(1000), nil)
		
		bal, err := svc.GetBalance(ctx, 1, 2)
		assert.NoError(t, err)
		assert.Equal(t, int32(1000), bal)
	})
}

func TestLedgerService_GetTransactions(t *testing.T) {
	repo := new(MockLedgerRepo)
	svc := service.NewLedgerService(repo)
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		txs := []domain.LedgerTransaction{{Amount: 100}}
		repo.On("ListTransactions", ctx, int32(1), int32(2), int32(1), int32(10)).Return(txs, int32(1), nil)
		
		res, total, err := svc.GetTransactions(ctx, 1, 2, 1, 10)
		assert.NoError(t, err)
		assert.Equal(t, int32(1), total)
		assert.Equal(t, int32(100), res[0].Amount)
	})
}

// TestLedgerService_GetLedgerSummary covers FR-003 (specs/007-ledger) at L1: GetLedgerSummary
// is a thin pass-through to the repository (all the domain logic — the renter-OR-owner
// StatusCount union — lives in the SQL and is covered at L2), so this confirms the service
// layer forwards the call and result correctly, matching the sibling GetBalance/GetTransactions
// unit tests above.
func TestLedgerService_GetLedgerSummary(t *testing.T) {
	repo := new(MockLedgerRepo)
	svc := service.NewLedgerService(repo)
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		summary := &domain.LedgerSummary{Balance: 1000, StatusCount: map[string]int32{"ACTIVE": 2}}
		repo.On("GetSummary", ctx, int32(1), int32(2), int32(0)).Return(summary, nil)

		res, err := svc.GetLedgerSummary(ctx, 1, 2, 0)
		assert.NoError(t, err)
		assert.Equal(t, int32(1000), res.Balance)
		assert.EqualValues(t, 2, res.StatusCount["ACTIVE"])
	})
}

// TestLedgerService_GetLedgerSummary_FiltersByMonths covers FR-003/FR-004(c) (specs/007-ledger):
// number_of_months must be forwarded to the repository layer unchanged, for both the single-org
// and cross-org rollup paths — this is the service-layer routing half of the fix; the actual
// date-window filtering SQL is proven at L2 (TestLedgerRepository_GetSummary_FiltersByMonths /
// TestLedgerRepository_GetSummary_CrossOrgRollup_FiltersByMonths, tests/integration/ledger_test.go).
func TestLedgerService_GetLedgerSummary_FiltersByMonths(t *testing.T) {
	t.Run("single-org: numberOfMonths forwarded to GetSummary", func(t *testing.T) {
		repo := new(MockLedgerRepo)
		svc := service.NewLedgerService(repo)
		ctx := context.Background()

		summary := &domain.LedgerSummary{Balance: 1000, StatusCount: map[string]int32{"ACTIVE": 1}}
		repo.On("GetSummary", ctx, int32(1), int32(2), int32(3)).Return(summary, nil)

		_, err := svc.GetLedgerSummary(ctx, 1, 2, 3)
		assert.NoError(t, err)
		repo.AssertCalled(t, "GetSummary", ctx, int32(1), int32(2), int32(3))
	})

	t.Run("cross-org rollup: numberOfMonths forwarded to GetSummaryAllOrgs", func(t *testing.T) {
		repo := new(MockLedgerRepo)
		svc := service.NewLedgerService(repo)
		ctx := context.Background()

		summary := &domain.LedgerSummary{Balance: 5000, StatusCount: map[string]int32{"ACTIVE": 1}}
		repo.On("GetSummaryAllOrgs", ctx, int32(1), int32(3)).Return(summary, nil)

		_, err := svc.GetLedgerSummary(ctx, 1, 0, 3)
		assert.NoError(t, err)
		repo.AssertCalled(t, "GetSummaryAllOrgs", ctx, int32(1), int32(3))
	})
}

// TestLedgerService_GetLedgerSummary_RollsUpAcrossOrgs covers FR-004 (specs/007-ledger,
// multi-org): when organization_id is omitted (0), GetLedgerSummary MUST roll up across all of
// the caller's orgs rather than querying a single org_id=0 row (which matches nothing). This is
// the service-layer routing half of the fix — it proves the branch calls the cross-org repo
// method instead of the single-org one; the aggregation SQL itself is proven at L2
// (TestLedgerRepository_GetSummary_CrossOrgRollup, tests/integration/ledger_test.go).
func TestLedgerService_GetLedgerSummary_RollsUpAcrossOrgs(t *testing.T) {
	repo := new(MockLedgerRepo)
	svc := service.NewLedgerService(repo)
	ctx := context.Background()

	t.Run("organization_id omitted calls the cross-org rollup, not the single-org lookup", func(t *testing.T) {
		summary := &domain.LedgerSummary{Balance: 5000, StatusCount: map[string]int32{"ACTIVE": 3}}
		repo.On("GetSummaryAllOrgs", ctx, int32(1), int32(0)).Return(summary, nil)

		res, err := svc.GetLedgerSummary(ctx, 1, 0, 0)
		assert.NoError(t, err)
		assert.Equal(t, int32(5000), res.Balance)
		assert.EqualValues(t, 3, res.StatusCount["ACTIVE"])
		repo.AssertNotCalled(t, "GetSummary", ctx, int32(1), int32(0), int32(0))
	})
}
