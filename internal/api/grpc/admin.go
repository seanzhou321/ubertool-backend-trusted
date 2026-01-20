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
	adminID, err := GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	err = h.adminSvc.ApproveJoinRequest(ctx, adminID, req.OrganizationId, req.ApplicantEmail, req.ApplicantName)
	if err != nil {
		return nil, err
	}
	return &pb.ApproveRequestToJoinResponse{Success: true}, nil
}

func (h *AdminHandler) AdminBlockUserAccount(ctx context.Context, req *pb.AdminBlockUserAccountRequest) (*pb.AdminBlockUserAccountResponse, error) {
	adminID, err := GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	err = h.adminSvc.BlockUser(ctx, adminID, req.BlockedUserId, req.OrganizationId, true, req.Reason)
	if err != nil {
		return nil, err
	}
	return &pb.AdminBlockUserAccountResponse{Success: true}, nil
}

func (h *AdminHandler) ListMembers(ctx context.Context, req *pb.ListMembersRequest) (*pb.ListMembersResponse, error) {
	users, uos, err := h.adminSvc.ListMembers(ctx, req.OrganizationId)
	if err != nil {
		return nil, err
	}
	members := make([]*pb.MemberProfile, len(users))
	for i := range users {
		members[i] = MapDomainMemberProfileToProto(users[i], uos[i])
	}
	return &pb.ListMembersResponse{Members: members}, nil
}

func (h *AdminHandler) SearchUsers(ctx context.Context, req *pb.SearchUsersRequest) (*pb.SearchUsersResponse, error) {
	users, uos, err := h.adminSvc.SearchUsers(ctx, req.OrganizationId, req.Query)
	if err != nil {
		return nil, err
	}
	protoUsers := make([]*pb.MemberProfile, len(users))
	for i := range users {
		protoUsers[i] = MapDomainMemberProfileToProto(users[i], uos[i])
	}
	return &pb.SearchUsersResponse{Users: protoUsers}, nil
}

func (h *AdminHandler) ListJoinRequests(ctx context.Context, req *pb.ListJoinRequestsRequest) (*pb.ListJoinRequestsResponse, error) {
	reqs, err := h.adminSvc.ListJoinRequests(ctx, req.OrganizationId)
	if err != nil {
		return nil, err
	}
	protoReqs := make([]*pb.JoinRequestProfile, len(reqs))
	for i, r := range reqs {
		protoReqs[i] = MapDomainJoinRequestProfileToProto(&r)
	}
	return &pb.ListJoinRequestsResponse{Requests: protoReqs}, nil
}
