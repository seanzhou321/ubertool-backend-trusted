package grpc

import (
	"context"

	pb "ubertool-backend-trusted/api/gen/v1"
	"ubertool-backend-trusted/internal/service"
)

type NotificationHandler struct {
	pb.UnimplementedNotificationServiceServer
	noteSvc service.NotificationService
}

func NewNotificationHandler(noteSvc service.NotificationService) *NotificationHandler {
	return &NotificationHandler{noteSvc: noteSvc}
}

func (h *NotificationHandler) GetNotifications(ctx context.Context, req *pb.GetNotificationsRequest) (*pb.GetNotificationsResponse, error) {
	userID, err := GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	// Map limit/offset to page/pageSize if needed, or update service
	limit := req.Limit
	if limit <= 0 {
		limit = 10
	}
	page := (req.Offset / limit) + 1
	pageSize := limit
	
	notes, count, err := h.noteSvc.GetNotifications(ctx, userID, page, pageSize)
	if err != nil {
		return nil, err
	}
	protoNotes := make([]*pb.Notification, len(notes))
	for i, n := range notes {
		protoNotes[i] = MapDomainNotificationToProto(&n)
	}
	return &pb.GetNotificationsResponse{
		Notifications: protoNotes,
		TotalCount:    count,
	}, nil
}

func (h *NotificationHandler) MarkNotificationRead(ctx context.Context, req *pb.MarkNotificationReadRequest) (*pb.MarkNotificationReadResponse, error) {
	userID, err := GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	err = h.noteSvc.MarkAsRead(ctx, userID, req.NotificationId)
	if err != nil {
		return nil, err
	}
	return &pb.MarkNotificationReadResponse{Success: true}, nil
}
