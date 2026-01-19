package service

import (
	"context"
	"errors"
	"time"

	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/repository"
	"ubertool-backend-trusted/internal/security"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrInviteExpired      = errors.New("invitation has expired")
	ErrInviteUsed         = errors.New("invitation already used")
	ErrInvalidToken       = errors.New("invalid token")
	ErrInvalid2FACode     = errors.New("invalid 2fa code")
)

type authService struct {
	userRepo   repository.UserRepository
	inviteRepo repository.InvitationRepository
	reqRepo    repository.JoinRequestRepository
	tm         security.TokenManager
}

func NewAuthService(userRepo repository.UserRepository, inviteRepo repository.InvitationRepository, reqRepo repository.JoinRequestRepository, secret string) AuthService {
	return &authService{
		userRepo:   userRepo,
		inviteRepo: inviteRepo,
		reqRepo:    reqRepo,
		tm:         security.NewTokenManager(secret),
	}
}

func (s *authService) ValidateInvite(ctx context.Context, token string) (*domain.Invitation, error) {
	inv, err := s.inviteRepo.GetByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	if inv.UsedOn != nil {
		return nil, ErrInviteUsed
	}
	if inv.ExpiresOn.Before(time.Now()) {
		return nil, ErrInviteExpired
	}
	return inv, nil
}

func (s *authService) RequestToJoin(ctx context.Context, orgID int32, name, email, note string) error {
	req := &domain.JoinRequest{
		OrgID:  orgID,
		Name:   name,
		Email:  email,
		Note:   note,
		Status: domain.JoinRequestStatusPending,
	}
	return s.reqRepo.Create(ctx, req)
}

func (s *authService) Signup(ctx context.Context, inviteToken, name, email, phone, password string) (*domain.User, string, string, error) {
	inv, err := s.ValidateInvite(ctx, inviteToken)
	if err != nil {
		return nil, "", "", err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", "", err
	}

	user := &domain.User{
		Email:        email,
		PhoneNumber:  phone,
		PasswordHash: string(hash),
		Name:         name,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, "", "", err
	}

	// Link user to org
	userOrg := &domain.UserOrg{
		UserID:   user.ID,
		OrgID:    inv.OrgID,
		Role:     domain.UserOrgRoleMember,
		Status:   domain.UserOrgStatusActive,
		JoinedOn: time.Now(),
	}
	if err := s.userRepo.AddUserToOrg(ctx, userOrg); err != nil {
		return nil, "", "", err
	}

	// Mark invite as used
	now := time.Now()
	inv.UsedOn = &now
	if err := s.inviteRepo.Update(ctx, inv); err != nil {
		return nil, "", "", err
	}

	access, err := s.tm.GenerateAccessToken(user.ID, []string{"user"})
	if err != nil {
		return nil, "", "", err
	}
	refresh, err := s.tm.GenerateRefreshToken(user.ID)
	if err != nil {
		return nil, "", "", err
	}

	return user, access, refresh, nil
}

func (s *authService) Login(ctx context.Context, email, password string) (string, string, string, bool, error) {
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return "", "", "", false, ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", "", "", false, ErrInvalidCredentials
	}

	// Assume 2FA is always enabled for trusted backend
	requires2FA := true

	if requires2FA {
		sessionToken, err := s.tm.Generate2FAToken(user.ID, "email")
		if err != nil {
			return "", "", "", false, err
		}
		// In a real implementation, we would send the email here.
		// emailService.Send2FACode(user.Email, generatedCode)
		return "", "", sessionToken, true, nil
	}

	access, err := s.tm.GenerateAccessToken(user.ID, []string{"user"}) // Retrieve roles
	if err != nil {
		return "", "", "", false, err
	}
	refresh, err := s.tm.GenerateRefreshToken(user.ID)
	if err != nil {
		return "", "", "", false, err
	}

	return access, refresh, "", false, nil
}

func (s *authService) Verify2FA(ctx context.Context, userID int32, code string) (string, string, error) {
	// Mock 2FA verification
	if code != "123456" {
		return "", "", ErrInvalid2FACode
	}

	// Verify user still exists
	_, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return "", "", err
	}

	// Generate tokens
	access, err := s.tm.GenerateAccessToken(userID, []string{"user"})
	if err != nil {
		return "", "", err
	}
	refresh, err := s.tm.GenerateRefreshToken(userID)
	if err != nil {
		return "", "", err
	}

	return access, refresh, nil
}

func (s *authService) RefreshToken(ctx context.Context, refreshToken string) (string, string, error) {
	claims, err := s.tm.ValidateToken(refreshToken)
	if err != nil {
		return "", "", err
	}

	if claims.Type != security.TokenTypeRefresh {
		return "", "", ErrInvalidToken
	}

	access, err := s.tm.GenerateAccessToken(claims.UserID, claims.Roles)
	if err != nil {
		return "", "", err
	}

	refresh, err := s.tm.GenerateRefreshToken(claims.UserID)
	if err != nil {
		return "", "", err
	}

	return access, refresh, nil
}

func (s *authService) Logout(ctx context.Context, refresh string) error {
	// In a real app, we might blacklist the refresh token
	return nil
}
