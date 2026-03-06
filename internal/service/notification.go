package service

import (
	"context"
	"fmt"
	"time"

	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/logger"
	"ubertool-backend-trusted/internal/repository"
)

type notificationService struct {
	noteRepo repository.NotificationRepository
	fcmRepo  repository.FcmTokenRepository
	pushSvc  PushNotificationService // nil when FCM is not configured
}

func NewNotificationService(noteRepo repository.NotificationRepository, fcmRepo repository.FcmTokenRepository) NotificationService {
	return &notificationService{noteRepo: noteRepo, fcmRepo: fcmRepo}
}

// SetPushService wires in the FCM push service after construction (avoids circular init).
func (s *notificationService) SetPushService(pushSvc PushNotificationService) {
	s.pushSvc = pushSvc
}

func (s *notificationService) GetNotifications(ctx context.Context, userID int32, page, pageSize int32) ([]domain.Notification, int32, error) {
	offset := (page - 1) * pageSize
	return s.noteRepo.List(ctx, userID, pageSize, offset)
}

func (s *notificationService) MarkAsRead(ctx context.Context, userID int32, notificationID int64) error {
	return s.noteRepo.MarkAsRead(ctx, notificationID, userID)
}

// Dispatch inserts a notification into the database and asynchronously sends an FCM push if configured.
func (s *notificationService) Dispatch(ctx context.Context, n *domain.Notification) error {
	if err := s.noteRepo.Create(ctx, n); err != nil {
		return err
	}
	if s.pushSvc != nil && n.ID > 0 {
		logger.Debug("Dispatching push notification", "userID", n.UserID, "notificationID", n.ID, "title", n.Title)
		s.pushSvc.SendToUser(ctx, n.UserID, n.Title, n.Message, n.ID, n.Attributes) //nolint:errcheck
	} else if s.pushSvc == nil {
		logger.Debug("Push service not configured, skipping push for notification", "notificationID", n.ID)
	}
	return nil
}

// DispatchSilent inserts a notification into the database without firing a push notification.
// Use this when the caller handles push delivery separately (e.g. via FCM multicast broadcast).
func (s *notificationService) DispatchSilent(ctx context.Context, n *domain.Notification) error {
	return s.noteRepo.Create(ctx, n)
}

// SyncDeviceToken upserts an FCM token for the user's device.
func (s *notificationService) SyncDeviceToken(ctx context.Context, userID int32, fcmToken, androidDeviceID, deviceName string) error {
	t := &domain.FcmToken{
		UserID:          userID,
		Token:           fcmToken,
		AndroidDeviceID: androidDeviceID,
		DeviceInfo:      map[string]string{"device_name": deviceName},
		Status:          "ACTIVE",
	}
	return s.fcmRepo.Upsert(ctx, t)
}

// ReportMessageEvent stamps the appropriate timestamp column (first-write-wins) for the notification.
func (s *notificationService) ReportMessageEvent(ctx context.Context, userID int32, notificationID int64, eventType string, eventTime time.Time) error {
	switch eventType {
	case "DELIVERED":
		return s.noteRepo.MarkDelivered(ctx, notificationID, userID, eventTime)
	case "CLICKED":
		return s.noteRepo.MarkClicked(ctx, notificationID, userID, eventTime)
	default:
		return fmt.Errorf("unknown event_type: %s", eventType)
	}
}

