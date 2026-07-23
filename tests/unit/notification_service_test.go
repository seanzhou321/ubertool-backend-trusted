package unit

import (
	"context"
	"errors"
	"testing"

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
