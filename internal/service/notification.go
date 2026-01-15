package service

import (
	"context"
	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/repository"
)

type notificationService struct {
	noteRepo repository.NotificationRepository
}

func NewNotificationService(noteRepo repository.NotificationRepository) NotificationService {
	return &notificationService{noteRepo: noteRepo}
}

func (s *notificationService) GetNotifications(ctx context.Context, userID int32, page, pageSize int32) ([]domain.Notification, int32, error) {
	offset := (page - 1) * pageSize
	return s.noteRepo.List(ctx, userID, pageSize, offset)
}

func (s *notificationService) MarkAsRead(ctx context.Context, userID, notificationID int32) error {
	return s.noteRepo.MarkAsRead(ctx, notificationID, userID)
}
