package service

import (
	"context"
	"fmt"
	"time"

	"firebase.google.com/go/v4/messaging"

	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/logger"
	"ubertool-backend-trusted/internal/repository"
)

const fcmSendTimeout = 10 * time.Second

type pushNotificationService struct {
	fcmClient *messaging.Client
	fcmRepo   repository.FcmTokenRepository
}

// NewPushNotificationService creates a PushNotificationService backed by the Firebase Messaging
// client. When fcmClient is nil (e.g. in dev/test without credentials) all sends are no-ops.
func NewPushNotificationService(fcmClient *messaging.Client, fcmRepo repository.FcmTokenRepository) PushNotificationService {
	return &pushNotificationService{fcmClient: fcmClient, fcmRepo: fcmRepo}
}

// SendToUser looks up all ACTIVE FCM tokens for userID and sends a push notification to each
// device asynchronously. The notificationID is embedded in the data payload so the client can
// call ReportMessageEvent to report delivery/click events back to the server.
func (s *pushNotificationService) SendToUser(ctx context.Context, userID int32, title, body string, notificationID int64, data map[string]string) error {
	if s.fcmClient == nil {
		logger.Debug("FCM client not configured, skipping push", "userID", userID, "notificationID", notificationID)
		return nil
	}

	tokens, err := s.fcmRepo.GetActiveByUserID(ctx, userID)
	if err != nil {
		logger.Error("Failed to get FCM tokens for user", "userID", userID, "error", err)
		return err
	}
	if len(tokens) == 0 {
		return nil
	}

	for _, t := range tokens {
		payload := buildPayload(data, notificationID)
		msg := &messaging.Message{
			Token: t.Token,
			Notification: &messaging.Notification{
				Title: title,
				Body:  body,
			},
			Data: payload,
		}
		go s.sendAsync(t, msg)
	}
	return nil
}

func (s *pushNotificationService) sendAsync(t domain.FcmToken, msg *messaging.Message) {
	ctx, cancel := context.WithTimeout(context.Background(), fcmSendTimeout)
	defer cancel()

	_, err := s.fcmClient.Send(ctx, msg)
	if err == nil {
		return
	}
	if messaging.IsUnregistered(err) {
		logger.Info("FCM token unregistered, marking obsolete", "token_prefix", tokenPrefix(t.Token))
		if obsErr := s.fcmRepo.MarkObsolete(context.Background(), t.Token); obsErr != nil {
			logger.Error("Failed to mark FCM token obsolete", "error", obsErr)
		}
		return
	}
	logger.Warn("FCM send failed", "userID", t.UserID, "error", err)
}

func buildPayload(extra map[string]string, notificationID int64) map[string]string {
	payload := make(map[string]string, len(extra)+1)
	for k, v := range extra {
		payload[k] = v
	}
	payload["notification_id"] = fmt.Sprintf("%d", notificationID)
	return payload
}

// tokenPrefix returns the first 8 chars of a token for safe log output.
func tokenPrefix(token string) string {
	if len(token) <= 8 {
		return token
	}
	return token[:8] + "..."
}
