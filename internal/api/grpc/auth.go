package grpc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	pb "ubertool-backend-trusted/api/gen/v1"
	"ubertool-backend-trusted/internal/logger"
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
	valid, errMsg, user, err := h.authSvc.ValidateInvite(ctx, req.InvitationCode, req.Email)
	if err != nil {
		return nil, err
	}
	if !valid {
		return &pb.ValidateInviteResponse{
			Valid:   false,
			Message: errMsg,
		}, nil
	}

	// Check if user is logged in by trying to extract user ID from context
	// If user exists AND is logged in, return user object
	var protoUser *pb.User
	if user != nil {
		userIDFromContext, err := GetUserIDFromContext(ctx)
		if err == nil && userIDFromContext == user.ID {
			// User is logged in
			protoUser = MapDomainUserToProto(user)
		}
	}

	return &pb.ValidateInviteResponse{
		Valid: true,
		User:  protoUser,
	}, nil
}

func (h *AuthHandler) RequestToJoinOrganization(ctx context.Context, req *pb.RequestToJoinRequest) (*pb.RequestToJoinResponse, error) {
	logger.Info("=== API RequestToJoinOrganization called ===",
		"organizationID", req.OrganizationId,
		"name", req.Name,
		"email", req.Email)
	logger.EnterMethod("AuthHandler.RequestToJoinOrganization", "organizationID", req.OrganizationId, "email", req.Email)

	err := h.authSvc.RequestToJoin(ctx, req.OrganizationId, req.Name, req.Email, req.Message)
	if err != nil {
		logger.ExitMethodWithError("AuthHandler.RequestToJoinOrganization", err, "organizationID", req.OrganizationId, "email", req.Email)
		logger.Error("=== API RequestToJoinOrganization FAILED ===", "error", err)
		return nil, err
	}

	logger.ExitMethod("AuthHandler.RequestToJoinOrganization", "organizationID", req.OrganizationId)
	logger.Info("=== API RequestToJoinOrganization completed successfully ===", "organizationID", req.OrganizationId)
	return &pb.RequestToJoinResponse{Success: true}, nil
}

func (h *AuthHandler) UserSignup(ctx context.Context, req *pb.SignupRequest) (*pb.SignupResponse, error) {
	err := h.authSvc.Signup(ctx, req.InvitationCode, req.Name, req.Email, req.Phone, req.Password)
	if err != nil {
		return nil, err
	}
	return &pb.SignupResponse{
		Success: true,
		Message: "Your account has been created. Please log in.",
	}, nil
}

func (h *AuthHandler) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	// Assume always requires 2FA for trusted backend, ie. requires2FA is always true
	_, _, session, _, err := h.authSvc.Login(ctx, req.Email, req.Password)
	if err != nil {
		return nil, err
	}

	// Note: access/refresh are empty if 2FA is required
	return &pb.LoginResponse{
		Success:    true,
		TwoFaToken: session,
		Message:    "2FA Required",
	}, nil
}

func (h *AuthHandler) Verify2FA(ctx context.Context, req *pb.Verify2FARequest) (*pb.Verify2FAResponse, error) {
	logger.Info("=== API Verify2FA called ===", "codeProvided", req.TwoFaCode)
	logger.EnterMethod("AuthHandler.Verify2FA", "codeLength", len(req.TwoFaCode))

	// UserID comes from the valid 2FA token in the header, validated by interceptor
	logger.Debug("Extracting userID from context (2FA token)")
	userID, err := GetUserIDFromContext(ctx)
	if err != nil {
		logger.Error("CRITICAL: Failed to get userID from context - 2FA token missing or invalid?", "error", err)
		logger.ExitMethodWithError("AuthHandler.Verify2FA", err, "reason", "no userID in context")
		return nil, err
	}
	logger.Info("UserID extracted from 2FA token", "userID", userID)

	logger.Debug("Calling authService.Verify2FA", "userID", userID, "code", req.TwoFaCode)
	access, refresh, err := h.authSvc.Verify2FA(ctx, int32(userID), req.TwoFaCode)
	if err != nil {
		logger.Error("=== API Verify2FA FAILED ===", "userID", userID, "error", err)
		logger.ExitMethodWithError("AuthHandler.Verify2FA", err, "userID", userID)
		return nil, err
	}

	logger.Info("=== API Verify2FA completed successfully ===", "userID", userID)
	logger.ExitMethod("AuthHandler.Verify2FA", "userID", userID)
	return &pb.Verify2FAResponse{
		Success:      true,
		AccessToken:  access,
		RefreshToken: refresh,
		// User field is optional in response? Check proto.
	}, nil
}

func (h *AuthHandler) RefreshToken(ctx context.Context, req *pb.RefreshTokenRequest) (*pb.RefreshTokenResponse, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Errorf(codes.Unauthenticated, "metadata is not provided")
	}
	tokens := md.Get("refresh-token")
	if len(tokens) == 0 {
		return nil, status.Errorf(codes.Unauthenticated, "refresh-token is not provided in metadata")
	}
	refreshToken := tokens[0]

	access, refresh, err := h.authSvc.RefreshToken(ctx, refreshToken)
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
