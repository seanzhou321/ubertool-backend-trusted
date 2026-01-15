package grpc

import (
	"context"

	pb "ubertool-backend-trusted/api/gen/v1"
	"ubertool-backend-trusted/internal/service"
)

type AdminHandler struct {
	pb.UnimplementedAdminServiceServer
	adminSvc service.AdminService
}

func NewAdminHandler(adminSvc service.AdminService) *AdminHandler {
	return &AdminHandler{adminSvc: adminSvc}
}

func (h *AdminHandler) ApproveRequestToJoin(ctx context.Context, req *pb.ApproveRequestToJoinRequest) (*pb.ApproveRequestToJoinResponse, error) {
	// The proto doesn't have requestId, it has email/name. 
	// This might mean it approves based on email.
	// We'll use a placeholder for adminID.
	adminID := req.UserId
	err := h.adminSvc.ApproveJoinRequest(ctx, adminID, 0) // Placeholder
	if err != nil {
		return nil, err
	}
	return &pb.ApproveRequestToJoinResponse{Success: true}, nil
}

func (h *AdminHandler) AdminAdjustBalance(ctx context.Context, req *pb.AdminAdjustBalanceRequest) (*pb.AdminAdjustBalanceResponse, error) {
	adminID := int32(1) // Placeholder
	err := h.adminSvc.AdjustBalance(ctx, adminID, req.UserId, req.OrganizationId, req.Amount, req.Reason)
	if err != nil {
		return nil, err
	}
	return &pb.AdminAdjustBalanceResponse{}, nil
}

func (h *AdminHandler) AdminBlockUserAccount(ctx context.Context, req *pb.AdminBlockUserAccountRequest) (*pb.AdminBlockUserAccountResponse, error) {
	adminID := int32(1) // Placeholder
	err := h.adminSvc.BlockUser(ctx, adminID, req.UserId, req.OrganizationId)
	if err != nil {
		return nil, err
	}
	return &pb.AdminBlockUserAccountResponse{Success: true}, nil
}
