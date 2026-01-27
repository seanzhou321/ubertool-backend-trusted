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
	userID, err := GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	user, orgs, userOrgs, err := h.userSvc.GetUserProfile(ctx, userID)
	if err != nil {
		return nil, err
	}

	protoUser := MapDomainUserToProto(user)
	protoUser.Orgs = make([]*pb.Organization, len(orgs))
	for i, o := range orgs {
		// Find matching userOrg to get role and balance
		var userRole string
		for _, uo := range userOrgs {
			if uo.OrgID == o.ID {
				userRole = string(uo.Role)
				protoUser.Orgs[i] = MapDomainOrgToProto(&o, userRole)
				protoUser.Orgs[i].UserBalance = uo.BalanceCents
				break
			}
		}
		if userRole == "" {
			// Fallback in case userOrg not found
			protoUser.Orgs[i] = MapDomainOrgToProto(&o, "")
		}
	}

	return &pb.GetUserResponse{
		User: protoUser,
	}, nil
}

func (h *UserHandler) UpdateProfile(ctx context.Context, req *pb.UpdateProfileRequest) (*pb.UpdateProfileResponse, error) {
	userID, err := GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	err = h.userSvc.UpdateProfile(ctx, userID, req.Name, req.Email, req.Phone, req.AvatarUrl)
	if err != nil {
		return nil, err
	}
	// Fetch updated user to return it
	user, _, _, err := h.userSvc.GetUserProfile(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &pb.UpdateProfileResponse{User: MapDomainUserToProto(user)}, nil
}
