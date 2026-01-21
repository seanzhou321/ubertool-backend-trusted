package repos

import (
	"context"
	"testing"

	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/repository/postgres"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func TestLedgerRepository_CreateTransaction(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error opening mock database: %v", err)
	}
	defer db.Close()

	repo := postgres.NewLedgerRepository(db)
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		tx := &domain.LedgerTransaction{
			OrgID:       1,
			UserID:      2,
			Amount:      -500,
			Type:        domain.TransactionTypeRentalDebit,
			Description: "Test",
		}

		mock.ExpectQuery("INSERT INTO ledger_transactions").
			WithArgs(tx.OrgID, tx.UserID, tx.Amount, tx.Type, tx.RelatedRentalID, tx.Description, sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

		err := repo.CreateTransaction(ctx, tx)
		assert.NoError(t, err)
		assert.Equal(t, int32(1), tx.ID)
	})
}

func TestLedgerRepository_GetBalance(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error opening mock database: %v", err)
	}
	defer db.Close()

	repo := postgres.NewLedgerRepository(db)
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		mock.ExpectQuery("SELECT COALESCE\\(balance_cents, 0\\) FROM users_orgs").
			WithArgs(int32(1), int32(2)).
			WillReturnRows(sqlmock.NewRows([]string{"balance_cents"}).AddRow(1000))

		balance, err := repo.GetBalance(ctx, 1, 2)
		assert.NoError(t, err)
		assert.Equal(t, int32(1000), balance)
	})
}
