package postgres

import (
	"context"
	"database/sql"
	"time"

	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/repository"
)

type ledgerRepository struct {
	db *sql.DB
}

func NewLedgerRepository(db *sql.DB) repository.LedgerRepository {
	return &ledgerRepository{db: db}
}

func (r *ledgerRepository) CreateTransaction(ctx context.Context, tx *domain.LedgerTransaction) error {
	query := `INSERT INTO ledger_transactions (org_id, user_id, amount, type, related_rental_id, description, charged_on, created_on) 
	          VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`
	return r.db.QueryRowContext(ctx, query, tx.OrgID, tx.UserID, tx.Amount, tx.Type, tx.RelatedRentalID, tx.Description, time.Now(), time.Now()).Scan(&tx.ID)
}

func (r *ledgerRepository) GetBalance(ctx context.Context, userID, orgID int32) (int32, error) {
	var balance int32
	query := `SELECT balance_cents FROM users_orgs WHERE user_id = $1 AND org_id = $2`
	err := r.db.QueryRowContext(ctx, query, userID, orgID).Scan(&balance)
	return balance, err
}

func (r *ledgerRepository) ListTransactions(ctx context.Context, userID, orgID int32, page, pageSize int32) ([]domain.LedgerTransaction, int32, error) {
	offset := (page - 1) * pageSize
	query := `SELECT id, org_id, user_id, amount, type, related_rental_id, description, charged_on, created_on 
	          FROM ledger_transactions WHERE user_id = $1 AND org_id = $2 ORDER BY created_on DESC LIMIT $3 OFFSET $4`
	rows, err := r.db.QueryContext(ctx, query, userID, orgID, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var count int32
	countQuery := `SELECT count(*) FROM ledger_transactions WHERE user_id = $1 AND org_id = $2`
	err = r.db.QueryRowContext(ctx, countQuery, userID, orgID).Scan(&count)
	if err != nil {
		return nil, 0, err
	}

	var txs []domain.LedgerTransaction
	for rows.Next() {
		var tx domain.LedgerTransaction
		if err := rows.Scan(&tx.ID, &tx.OrgID, &tx.UserID, &tx.Amount, &tx.Type, &tx.RelatedRentalID, &tx.Description, &tx.ChargedOn, &tx.CreatedOn); err != nil {
			return nil, 0, err
		}
		txs = append(txs, tx)
	}
	return txs, count, nil
}
