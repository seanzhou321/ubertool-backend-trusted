package grpc

import (
	"context"
	"time"

	pb "ubertool-backend-trusted/api/gen/v1"
	"ubertool-backend-trusted/internal/service"
	"google.golang.org/protobuf/types/known/emptypb"
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

func (h *NotificationHandler) SyncDeviceToken(ctx context.Context, req *pb.SyncTokenRequest) (*emptypb.Empty, error) {
	userID, err := GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.noteSvc.SyncDeviceToken(ctx, userID, req.FcmToken, req.AndroidDeviceId, req.DeviceName); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (h *NotificationHandler) ReportMessageEvent(ctx context.Context, req *pb.ReportEventRequest) (*emptypb.Empty, error) {
	userID, err := GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	var eventTime time.Time
	if req.EventTime != nil {
		eventTime = req.EventTime.AsTime()
	} else {
		eventTime = time.Now()
	}
	if err := h.noteSvc.ReportMessageEvent(ctx, userID, req.NotificationId, req.EventType, eventTime); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}
