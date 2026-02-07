package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/logger"
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
	// 1. Verify the invitation record exists using (invitation_code, email) tuple
	inv, err := s.inviteRepo.GetByInvitationCodeAndEmail(ctx, inviteCode, email)
	if err != nil {
		return false, "invitation code is invalid", nil, err
	}
	if inv.UsedOn != nil {
		return false, "invitation already used", nil, ErrInviteUsed
	}
	// Explicitly check expiration and return error
	if inv.ExpiresOn.Before(time.Now()) {
		return false, "invitation has expired", nil, ErrInviteExpired
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

func (s *authService) RequestToJoin(ctx context.Context, orgID int32, name, email, note, adminEmail string) error {
	logger.EnterMethod("authService.RequestToJoin", "orgID", orgID, "name", name, "email", email, "adminEmail", adminEmail)

	// 1. Verify org exists
	logger.Debug("Fetching organization", "orgID", orgID)
	logger.DatabaseCall("SELECT", "orgs WHERE id = $1")
	org, err := s.orgRepo.GetByID(ctx, orgID)
	logger.DatabaseResult("SELECT", 1, err, "orgID", orgID)
	if err != nil {
		logger.ExitMethodWithError("authService.RequestToJoin", err, "reason", "org not found")
		return err // Could verify if err is NotFound and return ErrOrgNotFound
	}
	if org == nil {
		logger.ExitMethodWithError("authService.RequestToJoin", ErrOrgNotFound, "reason", "org is nil")
		return ErrOrgNotFound
	}
	logger.Debug("Organization found", "orgID", orgID, "orgName", org.Name)

	// 2. Search user by email
	logger.Debug("Searching for user by email", "email", email)
	logger.DatabaseCall("SELECT", "users WHERE email = $1")
	user, err := s.userRepo.GetByEmail(ctx, email)
	logger.DatabaseResult("SELECT", 1, err)
	var userID *int32
	if err == nil && user != nil {
		userID = &user.ID
		logger.Debug("User found", "userID", *userID, "email", email)
	} else {
		logger.Debug("User not found, will create join request without userID", "email", email)
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
	logger.Debug("Creating join request", "orgID", orgID, "email", email, "name", name)
	logger.DatabaseCall("INSERT", "join_requests (org_id, user_id, name, email, note, status)")
	if err := s.reqRepo.Create(ctx, req); err != nil {
		logger.DatabaseResult("INSERT", 0, err)
		logger.ExitMethodWithError("authService.RequestToJoin", err, "reason", "failed to create join request")
		return err
	}
	logger.DatabaseResult("INSERT", 1, nil, "requestID", req.ID)
	logger.Info("Join request created successfully", "requestID", req.ID, "orgID", orgID, "email", email)

	// 4. Verify admin_email is an admin/super_admin in the organization
	logger.Debug("Verifying admin email", "adminEmail", adminEmail, "orgID", orgID)
	logger.DatabaseCall("SELECT", "users WHERE email = $1")
	adminUser, err := s.userRepo.GetByEmail(ctx, adminEmail)
	logger.DatabaseResult("SELECT", 1, err)
	if err != nil {
		logger.ExitMethodWithError("authService.RequestToJoin", err, "reason", "admin user not found")
		return fmt.Errorf("admin email %s not found: %w", adminEmail, err)
	}
	if adminUser == nil {
		logger.ExitMethodWithError("authService.RequestToJoin", fmt.Errorf("admin email not found"), "adminEmail", adminEmail)
		return fmt.Errorf("admin email %s not found", adminEmail)
	}

	// Verify the admin user is a member of the organization with admin/super_admin role
	logger.DatabaseCall("SELECT", "users_orgs WHERE user_id = $1 AND org_id = $2")
	adminUserOrg, err := s.userRepo.GetUserOrg(ctx, adminUser.ID, orgID)
	logger.DatabaseResult("SELECT", 1, err)
	if err != nil {
		logger.ExitMethodWithError("authService.RequestToJoin", err, "reason", "admin not member of org")
		return fmt.Errorf("admin email %s is not a member of organization %d: %w", adminEmail, orgID, err)
	}
	if adminUserOrg.Role != domain.UserOrgRoleAdmin && adminUserOrg.Role != domain.UserOrgRoleSuperAdmin {
		logger.ExitMethodWithError("authService.RequestToJoin", fmt.Errorf("insufficient permissions"), "adminEmail", adminEmail, "role", adminUserOrg.Role)
		return fmt.Errorf("user %s does not have admin permissions in organization %d", adminEmail, orgID)
	}
	logger.Debug("Admin verified", "adminID", adminUser.ID, "adminEmail", adminEmail, "role", adminUserOrg.Role)

	// 5. Send Email to the specified admin
	subject := fmt.Sprintf("New Join Request for %s", org.Name)
	message := fmt.Sprintf("User %s (%s) has requested to join %s.\nMessage: %s", name, email, org.Name, note)

	logger.ExternalServiceCall("email", "SendAdminNotification", "to", adminUser.Email, "subject", subject)
	emailErr := s.emailSvc.SendAdminNotification(ctx, adminUser.Email, subject, message)
	logger.ExternalServiceResult("email", "SendAdminNotification", emailErr, "to", adminUser.Email)
	if emailErr != nil {
		logger.Warn("Failed to send email notification to admin", "adminID", adminUser.ID, "email", adminUser.Email, "error", emailErr)
	}

	// 6. Create Notification for the specified admin
	notif := &domain.Notification{
		UserID:  adminUser.ID,
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

	logger.Debug("Creating notification for admin", "adminID", adminUser.ID, "notifTitle", notif.Title)
	logger.DatabaseCall("INSERT", "notifications (user_id, org_id, title, message, is_read, attributes)")
	notifErr := s.noteRepo.Create(ctx, notif)
	logger.DatabaseResult("INSERT", 1, notifErr, "adminID", adminUser.ID)

	if notifErr != nil {
		logger.Error("CRITICAL: Failed to create notification for admin", "adminID", adminUser.ID, "adminEmail", adminUser.Email, "error", notifErr)
	} else {
		logger.Info("Notification created successfully", "adminID", adminUser.ID, "notificationID", notif.ID)
	}

	logger.Info("Join request processing completed", "requestID", req.ID, "adminEmail", adminEmail)

	logger.ExitMethod("authService.RequestToJoin", "requestID", req.ID)
	return nil
}

func (s *authService) Signup(ctx context.Context, inviteToken, name, email, phone, password string) error {
	// 1. Validate invitation code with (invitation_code, email) tuple
	valid, errMsg, _, err := s.ValidateInvite(ctx, inviteToken, email)
	if err != nil {
		return err
	}
	if !valid {
		return errors.New(errMsg)
	}

	// Get the invitation to retrieve org details
	inv, err := s.inviteRepo.GetByInvitationCodeAndEmail(ctx, inviteToken, email)
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
	logger.EnterMethod("authService.Login", "email", email)

	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		logger.ExitMethodWithError("authService.Login", ErrInvalidCredentials, "reason", "user not found")
		return "", "", "", false, ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		logger.ExitMethodWithError("authService.Login", ErrInvalidCredentials, "reason", "password mismatch")
		return "", "", "", false, ErrInvalidCredentials
	}

	logger.Info("Password validated successfully", "userID", user.ID, "email", email)

	// Assume 2FA is always enabled for trusted backend
	requires2FA := true

	if requires2FA {
		logger.Debug("Generating 2FA token", "userID", user.ID)
		sessionToken, err := s.tm.Generate2FAToken(user.ID, "email")
		if err != nil {
			logger.ExitMethodWithError("authService.Login", err, "reason", "failed to generate 2FA token")
			return "", "", "", false, err
		}
		logger.Debug("2FA token generated", "userID", user.ID, "tokenPrefix", sessionToken[:20])

		// Send 2FA code via email
		// For prototype, we might hardcode or log it.
		// "123456" is hardcoded in Verify2FA currently.
		// Real implementation should generate random code and store it in Redis/Session.
		// For now, we can send the hardcoded code via email to demonstrate flow.
		code := "123456"
		logger.Info("2FA code for testing (HARDCODED)", "userID", user.ID, "code", code)
		subject := "Your 2FA Code"
		message := fmt.Sprintf("Your login code is: %s", code)
		_ = s.emailSvc.SendAdminNotification(ctx, user.Email, subject, message)

		logger.ExitMethod("authService.Login", "userID", user.ID, "requires2FA", true)
		return "", "", sessionToken, true, nil
	}

	// TODO: Retrieve actual roles from database
	access, err := s.tm.GenerateAccessToken(user.ID, user.Email, []string{"user"})
	if err != nil {
		return "", "", "", false, err
	}
	refresh, err := s.tm.GenerateRefreshToken(user.ID, user.Email)
	if err != nil {
		return "", "", "", false, err
	}

	return access, refresh, "", false, nil
}

func (s *authService) Verify2FA(ctx context.Context, userID int32, code string) (string, string, *domain.User, error) {
	logger.EnterMethod("authService.Verify2FA", "userID", userID, "codeProvided", code)

	// Mock 2FA verification
	logger.Debug("Validating 2FA code", "userID", userID, "providedCode", code, "expectedCode", "123456")
	if code != "123456" {
		logger.Warn("2FA code validation FAILED", "userID", userID, "providedCode", code, "expectedCode", "123456")
		logger.ExitMethodWithError("authService.Verify2FA", ErrInvalid2FACode, "userID", userID)
		return "", "", nil, ErrInvalid2FACode
	}
	logger.Info("2FA code validated successfully", "userID", userID)

	// Verify user still exists
	logger.Debug("Verifying user exists", "userID", userID)
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		logger.ExitMethodWithError("authService.Verify2FA", err, "reason", "user not found", "userID", userID)
		return "", "", nil, err
	}

	// Generate tokens
	logger.Debug("Generating access and refresh tokens", "userID", userID)
	// TODO: Retrieve actual roles from database
	access, err := s.tm.GenerateAccessToken(userID, user.Email, []string{"user"})
	if err != nil {
		logger.ExitMethodWithError("authService.Verify2FA", err, "reason", "failed to generate access token")
		return "", "", nil, err
	}
	refresh, err := s.tm.GenerateRefreshToken(userID, user.Email)
	if err != nil {
		logger.ExitMethodWithError("authService.Verify2FA", err, "reason", "failed to generate refresh token")
		return "", "", nil, err
	}

	logger.Info("2FA verification completed successfully", "userID", userID)
	logger.ExitMethod("authService.Verify2FA", "userID", userID)
	return access, refresh, user, nil
}

func (s *authService) RefreshToken(ctx context.Context, refreshToken string) (string, string, error) {
	claims, err := s.tm.ValidateToken(refreshToken)
	if err != nil {
		return "", "", err
	}

	if claims.Type != security.TokenTypeRefresh {
		return "", "", ErrInvalidToken
	}

	// Preserve email from existing token
	access, err := s.tm.GenerateAccessToken(claims.UserID, claims.Email, claims.Roles)
	if err != nil {
		return "", "", err
	}

	refresh, err := s.tm.GenerateRefreshToken(claims.UserID, claims.Email)
	if err != nil {
		return "", "", err
	}

	return access, refresh, nil
}

func (s *authService) Logout(ctx context.Context, refresh string) error {
	// In a real app, we might blacklist the refresh token
	return nil
}
