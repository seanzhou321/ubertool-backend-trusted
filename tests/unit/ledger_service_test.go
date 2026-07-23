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
		repo.On("GetSummary", ctx, int32(1), int32(2)).Return(summary, nil)

		res, err := svc.GetLedgerSummary(ctx, 1, 2)
		assert.NoError(t, err)
		assert.Equal(t, int32(1000), res.Balance)
		assert.EqualValues(t, 2, res.StatusCount["ACTIVE"])
	})
}
