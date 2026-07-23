package unit

import (
	"context"
	"errors"
	"testing"
	"time"

	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/service"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestNotificationService_Dispatch covers FR-003 (specs/004-notifications): Dispatch must
// persist the notification before attempting any push send, and must NOT let a push-delivery
// failure surface as a Dispatch error. Prior to this test, no unit test existed for
// notificationService at all — the e2e suite only ever exercised the happy path where the push
// send succeeds, never simulating a failure to confirm the containment guarantee.
func TestNotificationService_Dispatch(t *testing.T) {
	ctx := context.Background()

	t.Run("Persists the notification and swallows a push-send failure", func(t *testing.T) {
		noteRepo := new(MockNotificationDBRepo)
		fcmRepo := new(MockFcmTokenRepo)
		pushSvc := new(MockPushNotificationService)

		svc := service.NewNotificationService(noteRepo, fcmRepo)
		svc.SetPushService(pushSvc)

		n := &domain.Notification{UserID: 42, OrgID: 1, Title: "Hi", Message: "World"}
		noteRepo.On("Create", ctx, n).Run(func(args mock.Arguments) {
			note := args.Get(1).(*domain.Notification)
			note.ID = 100 // simulate DB-assigned ID, as the real repo would on insert
		}).Return(nil)
		pushSvc.On("SendToUser", ctx, int32(42), "Hi", "World", int64(100), mock.Anything).
			Return(errors.New("fcm: service unavailable"))

		err := svc.Dispatch(ctx, n)
		require.NoError(t, err, "a push-send failure must not surface as a Dispatch error")
		noteRepo.AssertCalled(t, "Create", ctx, n)
		pushSvc.AssertCalled(t, "SendToUser", ctx, int32(42), "Hi", "World", int64(100), mock.Anything)
	})

	t.Run("Propagates a persistence failure and never attempts a push", func(t *testing.T) {
		noteRepo := new(MockNotificationDBRepo)
		fcmRepo := new(MockFcmTokenRepo)
		pushSvc := new(MockPushNotificationService)

		svc := service.NewNotificationService(noteRepo, fcmRepo)
		svc.SetPushService(pushSvc)

		n := &domain.Notification{UserID: 42, OrgID: 1, Title: "Hi", Message: "World"}
		dbErr := errors.New("db connection lost")
		noteRepo.On("Create", ctx, n).Return(dbErr)

		err := svc.Dispatch(ctx, n)
		require.ErrorIs(t, err, dbErr)
		pushSvc.AssertNotCalled(t, "SendToUser", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})
}

// TestNotificationService_GetNotifications_NonAlignedOffset covers FR-001 (specs/004-notifications):
// GetNotifications must honor a true offset rather than round-tripping it through a page number.
// This is a regression test for spec.md's own documented Known Discrepancy 1 / SC-001: the prior
// gRPC handler computed `page := (offset / limit) + 1` (integer division), which silently
// rounded any non-page-aligned offset (e.g. offset=5, limit=10) down to offset=0. Fixed by having
// the service accept limit/offset directly instead of page/pageSize.
func TestNotificationService_GetNotifications_NonAlignedOffset(t *testing.T) {
	ctx := context.Background()
	noteRepo := new(MockNotificationDBRepo)
	fcmRepo := new(MockFcmTokenRepo)
	svc := service.NewNotificationService(noteRepo, fcmRepo)

	noteRepo.On("List", ctx, int32(7), int32(10), int32(5)).Return([]domain.Notification{}, int32(0), nil)

	_, _, err := svc.GetNotifications(ctx, 7, 10, 5)
	require.NoError(t, err)
	noteRepo.AssertCalled(t, "List", ctx, int32(7), int32(10), int32(5))
}

// TestNotificationService_ReportMessageEvent covers FR-007 (specs/004-notifications):
// ReportMessageEvent must reject an event_type outside {DELIVERED, CLICKED}, and must route
// DELIVERED/CLICKED to the corresponding repo method. Prior to this test, ReportMessageEvent
// appeared in tests/ only as a no-op mock-interface stub — never a real invocation.
func TestNotificationService_ReportMessageEvent(t *testing.T) {
	ctx := context.Background()
	const userID = int32(3)
	const notificationID = int64(500)
	eventTime := time.Now()

	t.Run("Routes DELIVERED to MarkDelivered", func(t *testing.T) {
		noteRepo := new(MockNotificationDBRepo)
		fcmRepo := new(MockFcmTokenRepo)
		svc := service.NewNotificationService(noteRepo, fcmRepo)
		noteRepo.On("MarkDelivered", ctx, notificationID, userID, eventTime).Return(nil)

		err := svc.ReportMessageEvent(ctx, userID, notificationID, "DELIVERED", eventTime)
		require.NoError(t, err)
		noteRepo.AssertCalled(t, "MarkDelivered", ctx, notificationID, userID, eventTime)
	})

	t.Run("Routes CLICKED to MarkClicked", func(t *testing.T) {
		noteRepo := new(MockNotificationDBRepo)
		fcmRepo := new(MockFcmTokenRepo)
		svc := service.NewNotificationService(noteRepo, fcmRepo)
		noteRepo.On("MarkClicked", ctx, notificationID, userID, eventTime).Return(nil)

		err := svc.ReportMessageEvent(ctx, userID, notificationID, "CLICKED", eventTime)
		require.NoError(t, err)
		noteRepo.AssertCalled(t, "MarkClicked", ctx, notificationID, userID, eventTime)
	})

	t.Run("Rejects an event_type outside DELIVERED/CLICKED", func(t *testing.T) {
		noteRepo := new(MockNotificationDBRepo)
		fcmRepo := new(MockFcmTokenRepo)
		svc := service.NewNotificationService(noteRepo, fcmRepo)

		err := svc.ReportMessageEvent(ctx, userID, notificationID, "OPENED", eventTime)
		require.Error(t, err)
		noteRepo.AssertNotCalled(t, "MarkDelivered", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		noteRepo.AssertNotCalled(t, "MarkClicked", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})
}
