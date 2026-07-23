package integration

import (
	"context"
	"testing"

	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/repository/postgres"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNotificationRepository_MarkAsRead_Idempotent covers FR-002 (specs/004-notifications):
// MarkNotificationRead must be idempotent — a first call sets read_at, and a second call on an
// already-read notification must be a no-op (not an error, and must not change the timestamp).
// Prior to this test, the one test that ever called MarkAsRead called it exactly once; nothing
// asserted the "first-write-wins" clause.
func TestNotificationRepository_MarkAsRead_Idempotent(t *testing.T) {
	db := prepareDB(t)
	defer db.Close()

	repo := postgres.NewNotificationRepository(db)
	ctx := context.Background()

	orgID := createTestOrgForLedger(t, db)
	userID := createTestUserForLedger(t, db, "notif-idempotent")
	defer func() {
		db.Exec("DELETE FROM notifications WHERE user_id = $1", userID)
		db.Exec("DELETE FROM users WHERE id = $1", userID)
		db.Exec("DELETE FROM orgs WHERE id = $1", orgID)
	}()

	n := &domain.Notification{UserID: userID, OrgID: orgID, Title: "Hi", Message: "World"}
	require.NoError(t, repo.Create(ctx, n))

	require.NoError(t, repo.MarkAsRead(ctx, n.ID, userID))
	var firstReadAt string
	require.NoError(t, db.QueryRow("SELECT read_at FROM notifications WHERE id = $1", n.ID).Scan(&firstReadAt))
	require.NotEmpty(t, firstReadAt)

	require.NoError(t, repo.MarkAsRead(ctx, n.ID, userID), "a second call on an already-read notification must be a no-op, not an error")
	var secondReadAt string
	require.NoError(t, db.QueryRow("SELECT read_at FROM notifications WHERE id = $1", n.ID).Scan(&secondReadAt))
	assert.Equal(t, firstReadAt, secondReadAt, "read_at must not change on a second MarkAsRead call (first-write-wins)")
}

// TestNotificationRepository_MarkAsRead_RejectsWrongOwner covers the other half of FR-002:
// MarkNotificationRead must reject a notification_id that does not belong to the caller.
func TestNotificationRepository_MarkAsRead_RejectsWrongOwner(t *testing.T) {
	db := prepareDB(t)
	defer db.Close()

	repo := postgres.NewNotificationRepository(db)
	ctx := context.Background()

	orgID := createTestOrgForLedger(t, db)
	ownerID := createTestUserForLedger(t, db, "notif-owner")
	otherID := createTestUserForLedger(t, db, "notif-other")
	defer func() {
		db.Exec("DELETE FROM notifications WHERE user_id = $1", ownerID)
		db.Exec("DELETE FROM users WHERE id IN ($1, $2)", ownerID, otherID)
		db.Exec("DELETE FROM orgs WHERE id = $1", orgID)
	}()

	n := &domain.Notification{UserID: ownerID, OrgID: orgID, Title: "Hi", Message: "World"}
	require.NoError(t, repo.Create(ctx, n))

	err := repo.MarkAsRead(ctx, n.ID, otherID)
	require.Error(t, err)

	var readAt *string
	require.NoError(t, db.QueryRow("SELECT read_at FROM notifications WHERE id = $1", n.ID).Scan(&readAt))
	assert.Nil(t, readAt, "a wrong-owner call must not mark the notification read")
}
