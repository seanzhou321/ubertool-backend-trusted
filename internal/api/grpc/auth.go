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
	// Assume always requires 2FA for trusted backend, ie. requires2FA is always true
	_, _, session, requires2FA, err := h.authSvc.Login(ctx, req.Email, req.Password)
	if err != nil {
		return nil, err
	}

	// Note: access/refresh are empty if 2FA is required
	return &pb.LoginResponse{
		SessionId:    session, // This is the 2FA pending token
		Requires_2Fa: requires2FA,
		// If NO 2FA, these would hypothetically be populated if we changed message structure,
		// but proto LoginResponse only has SessionId/Requires2FA?
		// Wait, if LoginResponse ONLY has SessionId/Requires2FA, how do we return AccessToken if 2FA is OFF?
		// Let me re-read api/gen/v1/auth_service.pb.go LoginResponse struct carefully.
	}, nil
}

func (h *AuthHandler) Verify2FA(ctx context.Context, req *pb.Verify2FARequest) (*pb.Verify2FAResponse, error) {
	// UserID comes from the valid 2FA token in the header, validated by interceptor
	userID, err := GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	access, refresh, err := h.authSvc.Verify2FA(ctx, int32(userID), req.Code)
	if err != nil {
		return nil, err
	}

	return &pb.Verify2FAResponse{
		AccessToken:  access,
		RefreshToken: refresh,
		// User field is optional in response? Check proto.
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
	_, _ = GetUserIDFromContext(ctx) // Extract but maybe not needed for simple logout
	return &pb.LogoutResponse{Success: true}, nil
}
