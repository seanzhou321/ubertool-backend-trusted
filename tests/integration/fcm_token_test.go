package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/repository/postgres"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFcmTokenRepository_Upsert_Reassignment covers FR-006 (specs/004-notifications):
// SyncDeviceToken (backed by fcmTokenRepository.Upsert) must upsert on the FCM token's own
// uniqueness, reassigning user_id when the same physical token re-registers under a different
// user (e.g. user A logs out, user B logs in on the same device). This is a DB-level ON CONFLICT
// semantic that a mocked unit test cannot exercise — the two existing e2e subtests only ever
// re-sync a token onto the SAME user, so the reassignment clause itself had zero coverage.
func TestFcmTokenRepository_Upsert_Reassignment(t *testing.T) {
	db := prepareDB(t)
	defer db.Close()

	repo := postgres.NewFcmTokenRepository(db)
	ctx := context.Background()

	userAEmail := fmt.Sprintf("test-integration-fcm-a-%d@test.com", time.Now().UnixNano())
	userBEmail := fmt.Sprintf("test-integration-fcm-b-%d@test.com", time.Now().UnixNano())
	var userAID, userBID int32
	require.NoError(t, db.QueryRow(
		`INSERT INTO users (email, phone_number, password_hash, name) VALUES ($1, $2, 'hash', 'User A') RETURNING id`,
		userAEmail, fmt.Sprintf("555-a-%d", time.Now().UnixNano()),
	).Scan(&userAID))
	require.NoError(t, db.QueryRow(
		`INSERT INTO users (email, phone_number, password_hash, name) VALUES ($1, $2, 'hash', 'User B') RETURNING id`,
		userBEmail, fmt.Sprintf("555-b-%d", time.Now().UnixNano()),
	).Scan(&userBID))
	defer func() {
		db.Exec("DELETE FROM fcm_tokens WHERE user_id IN ($1, $2)", userAID, userBID)
		db.Exec("DELETE FROM users WHERE id IN ($1, $2)", userAID, userBID)
	}()

	sharedToken := fmt.Sprintf("shared-device-token-%d", time.Now().UnixNano())
	deviceID := "physical-device-xyz"

	// User A registers the device first.
	err := repo.Upsert(ctx, &domain.FcmToken{
		UserID: userAID, Token: sharedToken, AndroidDeviceID: deviceID,
		DeviceInfo: map[string]string{"device_name": "Shared Phone"},
	})
	require.NoError(t, err)

	tokensA, err := repo.GetActiveByUserID(ctx, userAID)
	require.NoError(t, err)
	require.Len(t, tokensA, 1, "user A must own the token immediately after their own sync")

	// User B logs in on the same physical device — the same FCM token re-registers under B.
	err = repo.Upsert(ctx, &domain.FcmToken{
		UserID: userBID, Token: sharedToken, AndroidDeviceID: deviceID,
		DeviceInfo: map[string]string{"device_name": "Shared Phone"},
	})
	require.NoError(t, err)

	// The token row must now be reassigned to B, not duplicated.
	tokensB, err := repo.GetActiveByUserID(ctx, userBID)
	require.NoError(t, err)
	require.Len(t, tokensB, 1, "user B must own the token after re-registering it")
	assert.Equal(t, sharedToken, tokensB[0].Token)

	tokensA, err = repo.GetActiveByUserID(ctx, userAID)
	require.NoError(t, err)
	assert.Empty(t, tokensA, "user A must no longer see the reassigned token as their own")

	var rowCount int
	err = db.QueryRow("SELECT COUNT(*) FROM fcm_tokens WHERE fcm_token = $1", sharedToken).Scan(&rowCount)
	require.NoError(t, err)
	assert.Equal(t, 1, rowCount, "reassignment must update the existing row, not insert a duplicate")
}

// TestFcmTokenRepository_GetActiveByUserIDs covers FR-008 (specs/004-notifications):
// SendMulticastToUsers fetches its recipients via GetActiveByUserIDs, which must return only
// ACTIVE/TESTING tokens across the given set of users, excluding OBSOLETE ones. Prior to this
// test, this repository method — the actual SQL query the multicast batching logic depends
// on — had zero coverage at any tier (only ever mocked in the L1 push-notification tests).
func TestFcmTokenRepository_GetActiveByUserIDs(t *testing.T) {
	db := prepareDB(t)
	defer db.Close()

	repo := postgres.NewFcmTokenRepository(db)
	ctx := context.Background()

	userAEmail := fmt.Sprintf("test-integration-fcmulti-a-%d@test.com", time.Now().UnixNano())
	userBEmail := fmt.Sprintf("test-integration-fcmulti-b-%d@test.com", time.Now().UnixNano())
	userCEmail := fmt.Sprintf("test-integration-fcmulti-c-%d@test.com", time.Now().UnixNano())
	var userAID, userBID, userCID int32
	require.NoError(t, db.QueryRow(
		`INSERT INTO users (email, phone_number, password_hash, name) VALUES ($1, $2, 'hash', 'User A') RETURNING id`,
		userAEmail, fmt.Sprintf("555-ma-%d", time.Now().UnixNano()),
	).Scan(&userAID))
	require.NoError(t, db.QueryRow(
		`INSERT INTO users (email, phone_number, password_hash, name) VALUES ($1, $2, 'hash', 'User B') RETURNING id`,
		userBEmail, fmt.Sprintf("555-mb-%d", time.Now().UnixNano()),
	).Scan(&userBID))
	require.NoError(t, db.QueryRow(
		`INSERT INTO users (email, phone_number, password_hash, name) VALUES ($1, $2, 'hash', 'User C') RETURNING id`,
		userCEmail, fmt.Sprintf("555-mc-%d", time.Now().UnixNano()),
	).Scan(&userCID))
	defer func() {
		db.Exec("DELETE FROM fcm_tokens WHERE user_id IN ($1, $2, $3)", userAID, userBID, userCID)
		db.Exec("DELETE FROM users WHERE id IN ($1, $2, $3)", userAID, userBID, userCID)
	}()

	require.NoError(t, repo.Upsert(ctx, &domain.FcmToken{UserID: userAID, Token: fmt.Sprintf("tok-a-%d", time.Now().UnixNano()), AndroidDeviceID: "dev-a"}))
	require.NoError(t, repo.Upsert(ctx, &domain.FcmToken{UserID: userBID, Token: fmt.Sprintf("tok-b-%d", time.Now().UnixNano()), AndroidDeviceID: "dev-b"}))
	// User C's token is explicitly marked obsolete and must be excluded.
	obsoleteToken := fmt.Sprintf("tok-c-%d", time.Now().UnixNano())
	require.NoError(t, repo.Upsert(ctx, &domain.FcmToken{UserID: userCID, Token: obsoleteToken, AndroidDeviceID: "dev-c"}))
	require.NoError(t, repo.MarkObsolete(ctx, obsoleteToken))

	tokens, err := repo.GetActiveByUserIDs(ctx, []int32{userAID, userBID, userCID})
	require.NoError(t, err)

	var gotUserIDs []int32
	for _, tok := range tokens {
		gotUserIDs = append(gotUserIDs, tok.UserID)
	}
	assert.ElementsMatch(t, []int32{userAID, userBID}, gotUserIDs, "must return A and B's active tokens but exclude C's obsolete one")
}
