package repos

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/repository/postgres"
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
		mock.ExpectQuery("SELECT COALESCE\\(balance_cents, 0\\), COALESCE\\(last_balance_updated_on::text, ''\\) FROM users_orgs").
			WithArgs(int32(1), int32(2)).
			WillReturnRows(sqlmock.NewRows([]string{"balance_cents", "last_balance_updated_on"}).AddRow(1000, "2026-01-15"))

		balance, lastUpdated, err := repo.GetBalance(ctx, 1, 2)
		assert.NoError(t, err)
		assert.Equal(t, int32(1000), balance)
		assert.Equal(t, "2026-01-15", lastUpdated)
	})

	t.Run("empty string when never updated", func(t *testing.T) {
		mock.ExpectQuery("SELECT COALESCE\\(balance_cents, 0\\), COALESCE\\(last_balance_updated_on::text, ''\\) FROM users_orgs").
			WithArgs(int32(1), int32(3)).
			WillReturnRows(sqlmock.NewRows([]string{"balance_cents", "last_balance_updated_on"}).AddRow(0, ""))

		balance, lastUpdated, err := repo.GetBalance(ctx, 1, 3)
		assert.NoError(t, err)
		assert.Equal(t, int32(0), balance)
		assert.Equal(t, "", lastUpdated)
	})
}
