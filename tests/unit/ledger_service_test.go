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
