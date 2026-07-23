package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/repository/postgres"
	"ubertool-backend-trusted/internal/service"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// failingPushService is a service.PushNotificationService stub whose SendToUser always fails,
// used to confirm Dispatch's error-containment guarantee (FR-003) against a real DB.
type failingPushService struct{}

func (failingPushService) SendToUser(ctx context.Context, userID int32, title, body string, notificationID int64, data map[string]string) error {
	return errors.New("simulated push failure")
}
func (failingPushService) SendMulticastToUsers(ctx context.Context, userIDs []int32, title, body string, data map[string]string) error {
	return nil
}
func (failingPushService) Shutdown(ctx context.Context) error { return nil }

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

// TestNotificationService_Dispatch_Integration covers FR-003 (specs/004-notifications):
// Dispatch must persist the notification before attempting any push send, and must not let a
// push-delivery failure surface as a Dispatch error. Prior to this test, this guarantee was only
// proven at L1 (mocked repo) — never against a real database.
func TestNotificationService_Dispatch_Integration(t *testing.T) {
	db := prepareDB(t)
	defer db.Close()
	ctx := context.Background()

	orgID := createTestOrgForLedger(t, db)
	userID := createTestUserForLedger(t, db, "dispatch-integration")
	defer func() {
		db.Exec("DELETE FROM notifications WHERE user_id = $1", userID)
		db.Exec("DELETE FROM users WHERE id = $1", userID)
		db.Exec("DELETE FROM orgs WHERE id = $1", orgID)
	}()

	noteRepo := postgres.NewNotificationRepository(db)
	svc := service.NewNotificationService(noteRepo, nil)
	svc.SetPushService(failingPushService{})

	n := &domain.Notification{UserID: userID, OrgID: orgID, Title: "Hi", Message: "World"}
	err := svc.Dispatch(ctx, n)
	require.NoError(t, err, "a push-send failure must not surface as a Dispatch error")

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM notifications WHERE id = $1", n.ID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "the notification must be persisted despite the push failure")
}

// TestNotificationRepository_ReportMessageEvent_Idempotent covers FR-007 (specs/004-notifications):
// the DELIVERED/CLICKED timestamp stamps ReportMessageEvent applies are a DB-level
// COALESCE(...) first-write-wins semantic, scoped to (id, user_id). Prior to this test, no test
// exercised MarkDelivered/MarkClicked against a real database at all.
func TestNotificationRepository_ReportMessageEvent_Idempotent(t *testing.T) {
	db := prepareDB(t)
	defer db.Close()

	repo := postgres.NewNotificationRepository(db)
	ctx := context.Background()

	orgID := createTestOrgForLedger(t, db)
	ownerID := createTestUserForLedger(t, db, "reportevent-owner")
	otherID := createTestUserForLedger(t, db, "reportevent-other")
	defer func() {
		db.Exec("DELETE FROM notifications WHERE user_id IN ($1, $2)", ownerID, otherID)
		db.Exec("DELETE FROM users WHERE id IN ($1, $2)", ownerID, otherID)
		db.Exec("DELETE FROM orgs WHERE id = $1", orgID)
	}()

	n := &domain.Notification{UserID: ownerID, OrgID: orgID, Title: "Hi", Message: "World"}
	require.NoError(t, repo.Create(ctx, n))

	t.Run("DELIVERED is first-write-wins", func(t *testing.T) {
		first := time.Now().Add(-1 * time.Hour)
		require.NoError(t, repo.MarkDelivered(ctx, n.ID, ownerID, first))
		var firstDeliveredAt time.Time
		require.NoError(t, db.QueryRow("SELECT delivered_at FROM notifications WHERE id = $1", n.ID).Scan(&firstDeliveredAt))

		require.NoError(t, repo.MarkDelivered(ctx, n.ID, ownerID, time.Now()))
		var secondDeliveredAt time.Time
		require.NoError(t, db.QueryRow("SELECT delivered_at FROM notifications WHERE id = $1", n.ID).Scan(&secondDeliveredAt))
		assert.Equal(t, firstDeliveredAt.Unix(), secondDeliveredAt.Unix(), "delivered_at must not change on a second DELIVERED report")
	})

	t.Run("CLICKED does not stamp a different user's notification", func(t *testing.T) {
		require.NoError(t, repo.MarkClicked(ctx, n.ID, otherID, time.Now()))
		var clickedAt *time.Time
		require.NoError(t, db.QueryRow("SELECT clicked_at FROM notifications WHERE id = $1", n.ID).Scan(&clickedAt))
		assert.Nil(t, clickedAt, "MarkClicked scoped to the wrong user_id must not stamp the row")
	})
}
