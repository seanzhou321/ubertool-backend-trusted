package service

import (
	"context"
	"errors"
	"time"

	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/repository"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrInviteExpired      = errors.New("invitation has expired")
	ErrInviteUsed         = errors.New("invitation already used")
	ErrInvalidToken       = errors.New("invalid token")
)

type authService struct {
	userRepo   repository.UserRepository
	inviteRepo repository.InvitationRepository
	reqRepo    repository.JoinRequestRepository
	jwtSecret  []byte
}

func NewAuthService(userRepo repository.UserRepository, inviteRepo repository.InvitationRepository, reqRepo repository.JoinRequestRepository, secret string) AuthService {
	return &authService{
		userRepo:   userRepo,
		inviteRepo: inviteRepo,
		reqRepo:    reqRepo,
		jwtSecret:  []byte(secret),
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

	access, refresh, err := s.generateTokens(user.ID)
	return user, access, refresh, err
}

func (s *authService) Login(ctx context.Context, email, password string) (string, string, error) {
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return "", "", ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", "", ErrInvalidCredentials
	}

	return s.generateTokens(user.ID)
}

func (s *authService) Verify2FA(ctx context.Context, email, code string) (string, string, error) {
	// Mock 2FA verification - for demonstration, "123456" is always valid
	if code != "123456" {
		return "", "", errors.New("invalid 2fa code")
	}

	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return "", "", err
	}

	return s.generateTokens(user.ID)
}

func (s *authService) RefreshToken(ctx context.Context, refresh string) (string, string, error) {
	token, err := jwt.Parse(refresh, func(token *jwt.Token) (interface{}, error) {
		return s.jwtSecret, nil
	})

	if err != nil || !token.Valid {
		return "", "", ErrInvalidToken
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || claims["type"] != "refresh" {
		return "", "", ErrInvalidToken
	}

	userID := int32(claims["sub"].(float64))
	return s.generateTokens(userID)
}

func (s *authService) Logout(ctx context.Context, refresh string) error {
	// In a real app, we might blacklist the refresh token
	return nil
}

func (s *authService) generateTokens(userID int32) (string, string, error) {
	// Access Token (15 mins)
	accessClaims := jwt.MapClaims{
		"sub":  userID,
		"exp":  time.Now().Add(time.Minute * 15).Unix(),
		"type": "access",
	}
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	access, err := accessToken.SignedString(s.jwtSecret)
	if err != nil {
		return "", "", err
	}

	// Refresh Token (7 days)
	refreshClaims := jwt.MapClaims{
		"sub":  userID,
		"exp":  time.Now().Add(time.Hour * 24 * 7).Unix(),
		"type": "refresh",
	}
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refresh, err := refreshToken.SignedString(s.jwtSecret)
	if err != nil {
		return "", "", err
	}

	return access, refresh, nil
}
