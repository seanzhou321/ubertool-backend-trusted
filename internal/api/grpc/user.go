package grpc

import (
	"context"

	pb "ubertool-backend-trusted/api/gen/v1"
	"ubertool-backend-trusted/internal/service"
)

type UserHandler struct {
	pb.UnimplementedUserServiceServer
	userSvc service.UserService
}

func NewUserHandler(userSvc service.UserService) *UserHandler {
	return &UserHandler{userSvc: userSvc}
}

func (h *UserHandler) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.GetUserResponse, error) {
	user, userOrgs, err := h.userSvc.GetUserProfile(ctx, req.UserId)
	if err != nil {
		return nil, err
	}
	
	protoOrgs := make([]*pb.Organization, len(userOrgs))
	for i, uo := range userOrgs {
		protoOrgs[i] = &pb.Organization{Id: uo.OrgID}
	}

	return &pb.GetUserResponse{
		User: MapDomainUserToProto(user),
	}, nil
}

func (h *UserHandler) UpdateProfile(ctx context.Context, req *pb.UpdateProfileRequest) (*pb.UpdateProfileResponse, error) {
	err := h.userSvc.UpdateProfile(ctx, req.UserId, req.Name, req.AvatarUrl)
	if err != nil {
		return nil, err
	}
	// Fetch updated user to return it
	user, _, err := h.userSvc.GetUserProfile(ctx, req.UserId)
	if err != nil {
		return nil, err
	}
	return &pb.UpdateProfileResponse{User: MapDomainUserToProto(user)}, nil
}
