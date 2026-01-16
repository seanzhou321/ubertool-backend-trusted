package grpc

import (
	"context"

	pb "ubertool-backend-trusted/api/gen/v1"
	"ubertool-backend-trusted/internal/service"
)

type AuthHandler struct {
	pb.UnimplementedAuthServiceServer
	authSvc service.AuthService
}

func NewAuthHandler(authSvc service.AuthService) *AuthHandler {
	return &AuthHandler{authSvc: authSvc}
}

func (h *AuthHandler) ValidateInvite(ctx context.Context, req *pb.ValidateInviteRequest) (*pb.ValidateInviteResponse, error) {
	_, err := h.authSvc.ValidateInvite(ctx, req.InvitationCode)
	if err != nil {
		return &pb.ValidateInviteResponse{Valid: false}, nil
	}
	return &pb.ValidateInviteResponse{Valid: true}, nil
}

func (h *AuthHandler) RequestToJoinOrganization(ctx context.Context, req *pb.RequestToJoinRequest) (*pb.RequestToJoinResponse, error) {
	err := h.authSvc.RequestToJoin(ctx, req.OrganizationId, req.Name, req.Email, req.Message)
	if err != nil {
		return nil, err
	}
	return &pb.RequestToJoinResponse{Success: true}, nil
}

func (h *AuthHandler) UserSignup(ctx context.Context, req *pb.SignupRequest) (*pb.SignupResponse, error) {
	user, access, refresh, err := h.authSvc.Signup(ctx, req.InvitationCode, req.Name, req.Email, req.Phone, req.Password)
	if err != nil {
		return nil, err
	}
	return &pb.SignupResponse{
		User:         MapDomainUserToProto(user),
		AccessToken:  access,
		RefreshToken: refresh,
	}, nil
}

func (h *AuthHandler) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	_, _, err := h.authSvc.Login(ctx, req.Email, req.Password)
	if err != nil {
		return nil, err
	}
	
	return &pb.LoginResponse{
		SessionId:   "mock-session-" + req.Email,
		Requires_2Fa: true,
	}, nil
}

func (h *AuthHandler) Verify2FA(ctx context.Context, req *pb.Verify2FARequest) (*pb.Verify2FAResponse, error) {
	email := "mock@example.com"
	access, refresh, err := h.authSvc.Verify2FA(ctx, email, req.Code)
	if err != nil {
		return nil, err
	}
	
	return &pb.Verify2FAResponse{
		AccessToken:  access,
		RefreshToken: refresh,
	}, nil
}

func (h *AuthHandler) RefreshToken(ctx context.Context, req *pb.RefreshTokenRequest) (*pb.RefreshTokenResponse, error) {
	access, refresh, err := h.authSvc.RefreshToken(ctx, req.RefreshToken)
	if err != nil {
		return nil, err
	}
	return &pb.RefreshTokenResponse{
		AccessToken:  access,
		RefreshToken: refresh,
	}, nil
}

func (h *AuthHandler) Logout(ctx context.Context, req *pb.LogoutRequest) (*pb.LogoutResponse, error) {
	return &pb.LogoutResponse{Success: true}, nil
}
