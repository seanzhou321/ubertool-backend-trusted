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
	// Map limit/offset to page/pageSize if needed, or update service
	page := (req.Offset / req.Limit) + 1
	pageSize := req.Limit
	
	notes, count, err := h.noteSvc.GetNotifications(ctx, req.UserId, page, pageSize)
	if err != nil {
		return nil, err
	}
	protoNotes := make([]*pb.Notification, len(notes))
	for i, n := range notes {
		protoNotes[i] = mapDomainNotificationToProto(&n)
	}
	return &pb.GetNotificationsResponse{
		Notifications: protoNotes,
		TotalCount:    count,
	}, nil
}

func (h *NotificationHandler) MarkNotificationRead(ctx context.Context, req *pb.MarkNotificationReadRequest) (*pb.MarkNotificationReadResponse, error) {
	err := h.noteSvc.MarkAsRead(ctx, req.UserId, req.NotificationId)
	if err != nil {
		return nil, err
	}
	return &pb.MarkNotificationReadResponse{Success: true}, nil
}
