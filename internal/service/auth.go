package service

import (
	"context"
	"errors"
	"fmt"
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
	ErrOrgNotFound        = errors.New("organization not found")
)

type authService struct {
	userRepo   repository.UserRepository
	inviteRepo repository.InvitationRepository
	reqRepo    repository.JoinRequestRepository
	orgRepo    repository.OrganizationRepository
	noteRepo   repository.NotificationRepository
	emailSvc   EmailService
	tm         security.TokenManager
}

func NewAuthService(userRepo repository.UserRepository, inviteRepo repository.InvitationRepository, reqRepo repository.JoinRequestRepository, orgRepo repository.OrganizationRepository, noteRepo repository.NotificationRepository, emailSvc EmailService, secret string) AuthService {
	return &authService{
		userRepo:   userRepo,
		inviteRepo: inviteRepo,
		reqRepo:    reqRepo,
		orgRepo:    orgRepo,
		noteRepo:   noteRepo,
		emailSvc:   emailSvc,
		tm:         security.NewTokenManager(secret),
	}
}

func (s *authService) ValidateInvite(ctx context.Context, inviteCode, email string) (bool, string, *domain.User, error) {
	// 1. Verify the invitation record exists and is not expired
	inv, err := s.inviteRepo.GetByTokenAndEmail(ctx, inviteCode, email)
	if err != nil {
		return false, "invitation code and email pair is invalid or expired", nil, nil
	}
	if inv.UsedOn != nil {
		return false, "invitation already used", nil, nil
	}
	if inv.ExpiresOn.Before(time.Now()) {
		return false, "invitation code and email pair is invalid or expired", nil, nil
	}

	// 2. Check if a user with this email exists
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		// User doesn't exist - validation succeeds but no user object
		return true, "", nil, nil
	}

	// 3. Check if user is currently logged in (has valid JWT token in context)
	// Try to extract user ID from context - if successful, user is authenticated
	// This requires the context to have been validated by the JWT interceptor
	// For this validation to work, the gRPC handler should attempt to extract userID
	// We'll return the user object only if they exist
	// The gRPC layer will determine if they're logged in by checking the context

	return true, "", user, nil
}

func (s *authService) RequestToJoin(ctx context.Context, orgID int32, name, email, note string) error {
	// 1. Verify org exists
	org, err := s.orgRepo.GetByID(ctx, orgID)
	if err != nil {
		return err // Could verify if err is NotFound and return ErrOrgNotFound
	}
	if org == nil {
		return ErrOrgNotFound
	}

	// 2. Search user by email
	user, err := s.userRepo.GetByEmail(ctx, email)
	var userID *int32
	if err == nil && user != nil {
		userID = &user.ID
	}

	// 3. Create join request
	req := &domain.JoinRequest{
		OrgID:  orgID,
		UserID: userID,
		Name:   name,
		Email:  email,
		Note:   note,
		Status: domain.JoinRequestStatusPending,
	}
	if err := s.reqRepo.Create(ctx, req); err != nil {
		return err
	}

	// 4. Find admin users to notify
	users, userOrgs, err := s.userRepo.ListMembersByOrg(ctx, orgID)
	if err != nil {
		// Log error but don't fail the request? Or fail? Better to fail or log.
		// For prototype, returning error is safer to detect issues.
		return fmt.Errorf("failed to list admins for notification: %w", err)
	}

	for i, u := range users {
		uo := userOrgs[i]
		if uo.Role == domain.UserOrgRoleAdmin || uo.Role == domain.UserOrgRoleSuperAdmin {
			// 5. Send Email
			subject := fmt.Sprintf("New Join Request for %s", org.Name)
			message := fmt.Sprintf("User %s (%s) has requested to join %s.\nMessage: %s", name, email, org.Name, note)
			// We act on best effort for notifications?
			_ = s.emailSvc.SendAdminNotification(ctx, u.Email, subject, message)

			// 6. Create Notification
			notif := &domain.Notification{
				UserID:  u.ID,
				OrgID:   orgID,
				Title:   "New Join Request",
				Message: fmt.Sprintf("%s requested to join %s", name, org.Name),
				IsRead:  false,
				Attributes: map[string]string{
					"type":      "JOIN_REQUEST",
					"reference": fmt.Sprintf("join_request:%d", req.ID),
				},
				CreatedOn: time.Now(),
			}
			_ = s.noteRepo.Create(ctx, notif)
		}
	}

	return nil
}

func (s *authService) Signup(ctx context.Context, inviteToken, name, email, phone, password string) error {
	// 1. Validate invitation code and email pair
	valid, errMsg, _, _ := s.ValidateInvite(ctx, inviteToken, email)
	if !valid {
		return errors.New(errMsg)
	}

	// Get the invitation to retrieve org_id
	inv, err := s.inviteRepo.GetByTokenAndEmail(ctx, inviteToken, email)
	if err != nil {
		return err
	}

	// 3. Check if user already exists
	existingUser, err := s.userRepo.GetByEmail(ctx, email)
	if err == nil && existingUser != nil {
		return errors.New("Email already registered. Please log in instead.")
	}

	// 4. Create new user
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	user := &domain.User{
		Email:        email,
		PhoneNumber:  phone,
		PasswordHash: string(hash),
		Name:         name,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return err
	}

	// 6. Mark invite as used
	now := time.Now()
	inv.UsedOn = &now
	inv.UsedByUserID = &user.ID
	if err := s.inviteRepo.Update(ctx, inv); err != nil {
		return err
	}

	// 7. Link user to org
	userOrg := &domain.UserOrg{
		UserID:       user.ID,
		OrgID:        inv.OrgID,
		Role:         domain.UserOrgRoleMember,
		Status:       domain.UserOrgStatusActive,
		JoinedOn:     time.Now(),
		BalanceCents: 0,
	}
	if err := s.userRepo.AddUserToOrg(ctx, userOrg); err != nil {
		return err
	}

	return nil
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

		// Send 2FA code via email
		// For prototype, we might hardcode or log it.
		// "123456" is hardcoded in Verify2FA currently.
		// Real implementation should generate random code and store it in Redis/Session.
		// For now, we can send the hardcoded code via email to demonstrate flow.
		code := "123456"
		subject := "Your 2FA Code"
		message := fmt.Sprintf("Your login code is: %s", code)
		_ = s.emailSvc.SendAdminNotification(ctx, user.Email, subject, message)

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
