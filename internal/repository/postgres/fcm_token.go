package postgres

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/lib/pq"

	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/repository"
)

type fcmTokenRepository struct {
	db *sql.DB
}

func NewFcmTokenRepository(db *sql.DB) repository.FcmTokenRepository {
	return &fcmTokenRepository{db: db}
}

// Upsert inserts or updates an FCM token record.
// If the token already exists it updates user_id, device_info, status to ACTIVE, and updated_at.
// This handles re-login: the same physical device (same FCM token) may now belong to a new user_id.
func (r *fcmTokenRepository) Upsert(ctx context.Context, t *domain.FcmToken) error {
	info, err := json.Marshal(t.DeviceInfo)
	if err != nil {
		return err
	}
	query := `
		INSERT INTO fcm_tokens (user_id, fcm_token, android_device_id, device_info, status)
		VALUES ($1, $2, $3, $4, 'ACTIVE')
		ON CONFLICT (fcm_token)
		DO UPDATE SET
			user_id          = EXCLUDED.user_id,
			android_device_id = EXCLUDED.android_device_id,
			device_info      = EXCLUDED.device_info,
			status           = 'ACTIVE',
			updated_at       = NOW()
	`
	_, err = r.db.ExecContext(ctx, query, t.UserID, t.Token, t.AndroidDeviceID, info)
	return err
}

// GetActiveByUserID returns all ACTIVE and TESTING FCM tokens for a given user.
// TESTING tokens are sent using Firebase's dry-run (validate-only) mode so that
// fake tokens used in tests never trigger UNREGISTERED errors on real devices.
func (r *fcmTokenRepository) GetActiveByUserID(ctx context.Context, userID int32) ([]domain.FcmToken, error) {
	query := `SELECT id, user_id, fcm_token, android_device_id, device_info, status, created_at, updated_at
	          FROM fcm_tokens WHERE user_id = $1 AND status IN ('ACTIVE', 'TESTING')`
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tokens []domain.FcmToken
	for rows.Next() {
		var t domain.FcmToken
		var infoBytes []byte
		if err := rows.Scan(&t.ID, &t.UserID, &t.Token, &t.AndroidDeviceID, &infoBytes,
			&t.Status, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		if len(infoBytes) > 0 {
			_ = json.Unmarshal(infoBytes, &t.DeviceInfo)
		}
		tokens = append(tokens, t)
	}
	return tokens, nil
}

// MarkObsolete soft-deletes a token by setting its status to OBSOLETE.
// Called when FCM returns an Unregistered (404) error for this token.
func (r *fcmTokenRepository) MarkObsolete(ctx context.Context, token string) error {
	query := `UPDATE fcm_tokens SET status = 'OBSOLETE', updated_at = NOW() WHERE fcm_token = $1`
	_, err := r.db.ExecContext(ctx, query, token)
	return err
}

// MarkObsoleteByDevice marks all ACTIVE FCM tokens for a given user+device as OBSOLETE.
// Called on user logout so the device no longer receives push notifications.
func (r *fcmTokenRepository) MarkObsoleteByDevice(ctx context.Context, userID int32, androidDeviceID string) error {
	query := `UPDATE fcm_tokens SET status = 'OBSOLETE', updated_at = NOW()
	          WHERE user_id = $1 AND android_device_id = $2 AND status = 'ACTIVE'`
	_, err := r.db.ExecContext(ctx, query, userID, androidDeviceID)
	return err
}

// GetActiveByUserIDs returns all ACTIVE FCM tokens for the given set of user IDs in a single query.
func (r *fcmTokenRepository) GetActiveByUserIDs(ctx context.Context, userIDs []int32) ([]domain.FcmToken, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}
	query := `SELECT id, user_id, fcm_token, android_device_id, device_info, status, created_at, updated_at
	          FROM fcm_tokens WHERE user_id = ANY($1) AND status IN ('ACTIVE', 'TESTING')`
	rows, err := r.db.QueryContext(ctx, query, pq.Array(userIDs))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tokens []domain.FcmToken
	for rows.Next() {
		var t domain.FcmToken
		var infoBytes []byte
		if err := rows.Scan(&t.ID, &t.UserID, &t.Token, &t.AndroidDeviceID, &infoBytes,
			&t.Status, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		if len(infoBytes) > 0 {
			_ = json.Unmarshal(infoBytes, &t.DeviceInfo)
		}
		tokens = append(tokens, t)
	}
	return tokens, rows.Err()
}
