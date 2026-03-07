package postgres

import (
	"context"
	"database/sql"
	"time"

	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/repository"
)

type pendingCredentialsRepository struct {
	db *sql.DB
}

func NewPendingCredentialsRepository(db *sql.DB) repository.PendingCredentialsRepository {
	return &pendingCredentialsRepository{db: db}
}

func (r *pendingCredentialsRepository) Upsert(ctx context.Context, cred *domain.PendingCredential) error {
	query := `
		INSERT INTO pending_credentials (user_id, temp_password_hash, expires_at, used_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id) DO UPDATE
			SET temp_password_hash = EXCLUDED.temp_password_hash,
			    expires_at         = EXCLUDED.expires_at,
			    used_at            = EXCLUDED.used_at`
	_, err := r.db.ExecContext(ctx, query, cred.UserID, cred.TempPasswordHash, cred.ExpiresAt, cred.UsedAt)
	return err
}

func (r *pendingCredentialsRepository) GetByUserID(ctx context.Context, userID int32) (*domain.PendingCredential, error) {
	cred := &domain.PendingCredential{}
	var usedAt sql.NullTime
	query := `SELECT user_id, temp_password_hash, expires_at, used_at FROM pending_credentials WHERE user_id = $1`
	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&cred.UserID,
		&cred.TempPasswordHash,
		&cred.ExpiresAt,
		&usedAt,
	)
	if err != nil {
		return nil, err
	}
	if usedAt.Valid {
		t := usedAt.Time
		cred.UsedAt = &t
	}
	return cred, nil
}

func (r *pendingCredentialsRepository) StampUsedAt(ctx context.Context, userID int32) error {
	query := `UPDATE pending_credentials SET used_at = $1 WHERE user_id = $2 AND used_at IS NULL`
	_, err := r.db.ExecContext(ctx, query, time.Now(), userID)
	return err
}
