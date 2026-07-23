package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"database/sql"

	"ubertool-backend-trusted/internal/config"
	"ubertool-backend-trusted/internal/repository/postgres"
	"ubertool-backend-trusted/internal/service"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func createTestUserForAuth(t *testing.T, db *sql.DB, prefix string) (int32, string) {
	t.Helper()
	email := fmt.Sprintf("test-integration-auth-%s-%d@test.com", prefix, time.Now().UnixNano())
	var userID int32
	require.NoError(t, db.QueryRow(
		`INSERT INTO users (email, phone_number, password_hash, name) VALUES ($1, $2, 'placeholder-hash', $3) RETURNING id`,
		email, fmt.Sprintf("555-%s-%d", prefix, time.Now().UnixNano()), prefix,
	).Scan(&userID))
	return userID, email
}

// TestAuthService_Logout_Integration covers FR-005 (specs/001-authentication-legal-consent):
// Logout must mark the caller's FCM token(s) OBSOLETE. Prior to this test, the only evidence was
// a mocked L1 unit test (the fcmRepo.MarkObsoleteByDevice call was asserted, but never confirmed
// to actually flip the row's status in Postgres).
func TestAuthService_Logout_Integration(t *testing.T) {
	db := prepareDB(t)
	defer db.Close()
	ctx := context.Background()

	userID, _ := createTestUserForAuth(t, db, "logout")
	fcmRepo := postgres.NewFcmTokenRepository(db)
	svc := service.NewAuthService(nil, nil, nil, nil, nil, nil, "secret", fcmRepo, nil, nil, config.TwoFAConfig{})

	deviceID := fmt.Sprintf("device-%d", time.Now().UnixNano())
	token := fmt.Sprintf("token-%d", time.Now().UnixNano())
	_, err := db.Exec(
		`INSERT INTO fcm_tokens (user_id, fcm_token, android_device_id, status) VALUES ($1, $2, $3, 'ACTIVE')`,
		userID, token, deviceID,
	)
	require.NoError(t, err)
	defer func() {
		db.Exec("DELETE FROM fcm_tokens WHERE user_id = $1", userID)
		db.Exec("DELETE FROM users WHERE id = $1", userID)
	}()

	err = svc.Logout(ctx, userID, "some-refresh-token", deviceID)
	require.NoError(t, err)

	var status string
	err = db.QueryRow("SELECT status FROM fcm_tokens WHERE user_id = $1 AND android_device_id = $2", userID, deviceID).Scan(&status)
	require.NoError(t, err)
	assert.Equal(t, "OBSOLETE", status, "Logout must mark the device's FCM token OBSOLETE in the database")
}

// TestAuthService_ChangePassword_Integration covers FR-008 (specs/001-authentication-legal-consent):
// ChangePassword must update the canonical password_hash on success and stamp any outstanding
// pending_credentials.used_at. Prior to this test, the only evidence was a mocked L1 unit test.
func TestAuthService_ChangePassword_Integration(t *testing.T) {
	db := prepareDB(t)
	defer db.Close()
	ctx := context.Background()

	userID, _ := createTestUserForAuth(t, db, "changepwd")
	oldHash, err := bcrypt.GenerateFromPassword([]byte("old-password-1"), bcrypt.DefaultCost)
	require.NoError(t, err)
	_, err = db.Exec("UPDATE users SET password_hash = $1 WHERE id = $2", string(oldHash), userID)
	require.NoError(t, err)

	// A still-outstanding pending credential must be stamped used on a successful canonical change.
	_, err = db.Exec(
		`INSERT INTO pending_credentials (user_id, temp_password_hash, expires_at) VALUES ($1, 'unused-temp-hash', $2)`,
		userID, time.Now().Add(20*time.Minute),
	)
	require.NoError(t, err)
	defer func() {
		db.Exec("DELETE FROM pending_credentials WHERE user_id = $1", userID)
		db.Exec("DELETE FROM users WHERE id = $1", userID)
	}()

	userRepo := postgres.NewUserRepository(db)
	pendingCredsRepo := postgres.NewPendingCredentialsRepository(db)
	svc := service.NewAuthService(userRepo, nil, nil, nil, nil, nil, "secret", nil, pendingCredsRepo, nil, config.TwoFAConfig{})

	err = svc.ChangePassword(ctx, userID, "old-password-1", "new-password-2")
	require.NoError(t, err)

	var newHash string
	err = db.QueryRow("SELECT password_hash FROM users WHERE id = $1", userID).Scan(&newHash)
	require.NoError(t, err)
	assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(newHash), []byte("new-password-2")),
		"the stored password_hash must verify against the new password")

	var usedAt *time.Time
	err = db.QueryRow("SELECT used_at FROM pending_credentials WHERE user_id = $1", userID).Scan(&usedAt)
	require.NoError(t, err)
	assert.NotNil(t, usedAt, "an outstanding pending credential must be stamped used_at on a successful change")
}

// TestAuthService_ResetPassword_Integration covers FR-009 (specs/001-authentication-legal-consent):
// ResetPassword must create/upsert a pending_credentials row for an existing account. Prior to
// this test, the only evidence was a mocked L1 unit test (regression-locking the real
// user-enumeration bug fixed in Phase 1) — this confirms the actual DB write against Postgres.
func TestAuthService_ResetPassword_Integration(t *testing.T) {
	db := prepareDB(t)
	defer db.Close()
	ctx := context.Background()

	userID, email := createTestUserForAuth(t, db, "resetpwd")
	defer func() {
		db.Exec("DELETE FROM pending_credentials WHERE user_id = $1", userID)
		db.Exec("DELETE FROM users WHERE id = $1", userID)
	}()

	userRepo := postgres.NewUserRepository(db)
	pendingCredsRepo := postgres.NewPendingCredentialsRepository(db)
	emailSvc := new(MockEmailService)
	svc := service.NewAuthService(userRepo, nil, nil, nil, nil, emailSvc, "secret", nil, pendingCredsRepo, nil, config.TwoFAConfig{})

	t.Run("Existing account gets a real pending_credentials row", func(t *testing.T) {
		err := svc.ResetPassword(ctx, email)
		require.NoError(t, err)

		var tempHash string
		var expiresAt time.Time
		var usedAt *time.Time
		err = db.QueryRow(
			"SELECT temp_password_hash, expires_at, used_at FROM pending_credentials WHERE user_id = $1", userID,
		).Scan(&tempHash, &expiresAt, &usedAt)
		require.NoError(t, err)
		assert.NotEmpty(t, tempHash)
		assert.Nil(t, usedAt)
		assert.True(t, expiresAt.After(time.Now()))
	})

	t.Run("Non-existent email creates no row and returns no error", func(t *testing.T) {
		err := svc.ResetPassword(ctx, "nobody-integration@example.com")
		require.NoError(t, err, "must not be distinguishable from the found-user case")

		var count int
		err = db.QueryRow(
			`SELECT COUNT(*) FROM pending_credentials pc JOIN users u ON pc.user_id = u.id WHERE u.email = $1`,
			"nobody-integration@example.com",
		).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})
}
