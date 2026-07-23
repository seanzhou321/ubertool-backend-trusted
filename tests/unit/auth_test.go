package unit

import (
	"context"
	"regexp"
	"testing"
	"time"

	"ubertool-backend-trusted/internal/config"
	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestAuthService_ValidateInvite(t *testing.T) {
	userRepo := new(MockUserRepo)
	inviteRepo := new(MockInviteRepo)
	reqRepo := new(MockJoinRequestRepo)
	orgRepo := new(MockOrganizationRepo)
	noteRepo := new(MockNotificationRepo)
	emailSvc := new(MockEmailService)
	fcmRepo := new(MockFcmTokenRepo)
	pendingCredsRepo := new(MockPendingCredentialsRepo)
	legalConsentRepo := new(MockLegalConsentRepo)
	svc := service.NewAuthService(userRepo, inviteRepo, reqRepo, orgRepo, noteRepo, emailSvc, "secret", fcmRepo, pendingCredsRepo, legalConsentRepo,
		config.TwoFAConfig{Enabled: false, FixedPasscode: "00000"})

	ctx := context.Background()
	token := "valid-token"
	email := "test@example.com"

	t.Run("Valid Token", func(t *testing.T) {
		invite := &domain.Invitation{
			InvitationCode: token,
			Email:          email,
			OrgID:          1,
			ExpiresOn:      time.Now().Add(48 * time.Hour).Format("2006-01-02"), // Future
		}
		inviteRepo.ExpectedCalls = nil
		userRepo.ExpectedCalls = nil
		inviteRepo.On("GetByInvitationCodeAndEmail", ctx, token, email).Return(invite, nil)
		userRepo.On("GetByEmail", ctx, email).Return(nil, assert.AnError)

		valid, msg, user, err := svc.ValidateInvite(ctx, token, email)
		assert.NoError(t, err)
		assert.True(t, valid)
		assert.Empty(t, msg)
		assert.Nil(t, user)
	})

	t.Run("Expired Token", func(t *testing.T) {
		invite := &domain.Invitation{
			InvitationCode: token,
			Email:          email,
			OrgID:          1,
			ExpiresOn:      time.Now().Add(-24 * time.Hour).Format("2006-01-02"), // Yesterday
		}
		inviteRepo.ExpectedCalls = nil
		inviteRepo.On("GetByInvitationCodeAndEmail", ctx, token, email).Return(invite, nil)

		valid, msg, user, err := svc.ValidateInvite(ctx, token, email)
		assert.Error(t, err, "Expected error for expired invitation")
		assert.Equal(t, service.ErrInviteExpired, err, "Expected ErrInviteExpired")
		assert.False(t, valid)
		assert.Contains(t, msg, "expired")
		assert.Nil(t, user)
	})

	t.Run("Used Token", func(t *testing.T) {
		now := time.Now().Format("2006-01-02")
		invite := &domain.Invitation{
			InvitationCode: token,
			Email:          email,
			OrgID:          1,
			ExpiresOn:      time.Now().Add(48 * time.Hour).Format("2006-01-02"),
			UsedOn:         &now,
		}
		inviteRepo.ExpectedCalls = nil
		inviteRepo.On("GetByInvitationCodeAndEmail", ctx, token, email).Return(invite, nil)

		valid, msg, user, err := svc.ValidateInvite(ctx, token, email)
		assert.Error(t, err, "Expected error for used invitation")
		assert.Equal(t, service.ErrInviteUsed, err, "Expected ErrInviteUsed")
		assert.False(t, valid)
		assert.Contains(t, msg, "already used")
		assert.Nil(t, user)
	})
}

func TestAuthService_RequestToJoin(t *testing.T) {
	userRepo := new(MockUserRepo)
	inviteRepo := new(MockInviteRepo)
	reqRepo := new(MockJoinRequestRepo)
	orgRepo := new(MockOrganizationRepo)
	noteRepo := new(MockNotificationRepo)
	emailSvc := new(MockEmailService)
	fcmRepo := new(MockFcmTokenRepo)
	pendingCredsRepo := new(MockPendingCredentialsRepo)
	legalConsentRepo := new(MockLegalConsentRepo)

	svc := service.NewAuthService(userRepo, inviteRepo, reqRepo, orgRepo, noteRepo, emailSvc, "secret", fcmRepo, pendingCredsRepo, legalConsentRepo,
		config.TwoFAConfig{Enabled: false, FixedPasscode: "00000"})

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		orgID := int32(1)
		email := "email@test.com"
		adminEmail := "admin@test.com"
		orgRepo.On("GetByID", ctx, orgID).Return(&domain.Organization{ID: orgID, Name: "Org"}, nil)
		userRepo.On("GetByEmail", ctx, email).Return(nil, nil)
		reqRepo.On("Create", ctx, mock.AnythingOfType("*domain.JoinRequest")).Return(nil)

		// Mock admin verification
		adminUser := &domain.User{ID: 2, Email: adminEmail, Name: "Admin"}
		userRepo.On("GetByEmail", ctx, adminEmail).Return(adminUser, nil)
		adminUserOrg := &domain.UserOrg{UserID: 2, OrgID: orgID, Role: domain.UserOrgRoleAdmin}
		userRepo.On("GetUserOrg", ctx, int32(2), orgID).Return(adminUserOrg, nil)

		// Mock notification creation
		emailSvc.On("SendAdminNotification", ctx, adminEmail, mock.Anything, mock.Anything).Return(nil)
		noteRepo.On("Create", ctx, mock.AnythingOfType("*domain.Notification")).Return(nil)

		err := svc.RequestToJoin(ctx, orgID, "Name", email, "Note", adminEmail)
		assert.NoError(t, err)
	})

	// FR-007: RequestToJoinOrganization must verify the org exists before doing anything else.
	t.Run("Rejects when the organization does not exist", func(t *testing.T) {
		userRepo := new(MockUserRepo)
		inviteRepo := new(MockInviteRepo)
		reqRepo := new(MockJoinRequestRepo)
		orgRepo := new(MockOrganizationRepo)
		noteRepo := new(MockNotificationRepo)
		emailSvc := new(MockEmailService)
		fcmRepo := new(MockFcmTokenRepo)
		pendingCredsRepo := new(MockPendingCredentialsRepo)
		legalConsentRepo := new(MockLegalConsentRepo)
		svc := service.NewAuthService(userRepo, inviteRepo, reqRepo, orgRepo, noteRepo, emailSvc, "secret", fcmRepo, pendingCredsRepo, legalConsentRepo,
			config.TwoFAConfig{Enabled: false, FixedPasscode: "00000"})

		orgID := int32(999)
		orgRepo.On("GetByID", ctx, orgID).Return(nil, assert.AnError)

		err := svc.RequestToJoin(ctx, orgID, "Name", "email@test.com", "Note", "admin@test.com")
		require.Error(t, err)
		reqRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	})

	// FR-007's most distinctive clause (spec.md Edge Cases / US3 Scenario 6): the join_requests
	// row must persist even when the subsequent admin-verification step fails.
	t.Run("Persists the join request even when admin verification subsequently fails", func(t *testing.T) {
		userRepo := new(MockUserRepo)
		inviteRepo := new(MockInviteRepo)
		reqRepo := new(MockJoinRequestRepo)
		orgRepo := new(MockOrganizationRepo)
		noteRepo := new(MockNotificationRepo)
		emailSvc := new(MockEmailService)
		fcmRepo := new(MockFcmTokenRepo)
		pendingCredsRepo := new(MockPendingCredentialsRepo)
		legalConsentRepo := new(MockLegalConsentRepo)
		svc := service.NewAuthService(userRepo, inviteRepo, reqRepo, orgRepo, noteRepo, emailSvc, "secret", fcmRepo, pendingCredsRepo, legalConsentRepo,
			config.TwoFAConfig{Enabled: false, FixedPasscode: "00000"})

		orgID := int32(1)
		email := "applicant@test.com"
		adminEmail := "not-an-admin@test.com"
		orgRepo.On("GetByID", ctx, orgID).Return(&domain.Organization{ID: orgID, Name: "Org"}, nil)
		userRepo.On("GetByEmail", ctx, email).Return(nil, nil)
		reqRepo.On("Create", ctx, mock.AnythingOfType("*domain.JoinRequest")).Return(nil)
		// The admin-email lookup fails (not found) — RequestToJoin must still have persisted
		// the join request created above before reaching this step.
		userRepo.On("GetByEmail", ctx, adminEmail).Return(nil, assert.AnError)

		err := svc.RequestToJoin(ctx, orgID, "Name", email, "Note", adminEmail)
		require.Error(t, err)
		reqRepo.AssertCalled(t, "Create", ctx, mock.AnythingOfType("*domain.JoinRequest"))
	})
}

// TestAuthService_Login_TwoFA_ProductionPath covers FR-001's spec.md requirement that when
// two_fa.enabled=true, Login must take the random-code+email path (not the fixed-passcode
// path used only for automated tests) — the production 2FA behavior real users experience.
func TestAuthService_Login_TwoFA_ProductionPath(t *testing.T) {
	userRepo := new(MockUserRepo)
	inviteRepo := new(MockInviteRepo)
	reqRepo := new(MockJoinRequestRepo)
	orgRepo := new(MockOrganizationRepo)
	noteRepo := new(MockNotificationRepo)
	emailSvc := new(MockEmailService)
	fcmRepo := new(MockFcmTokenRepo)
	pendingCredsRepo := new(MockPendingCredentialsRepo)
	legalConsentRepo := new(MockLegalConsentRepo)

	svc := service.NewAuthService(userRepo, inviteRepo, reqRepo, orgRepo, noteRepo, emailSvc, "secret", fcmRepo, pendingCredsRepo, legalConsentRepo,
		config.TwoFAConfig{Enabled: true, FixedPasscode: "00000"})

	ctx := context.Background()
	const userID = int32(7)
	const email = "prod-2fa-user@example.com"
	const password = "correct-horse-battery-staple"

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	require.NoError(t, err)
	user := &domain.User{ID: userID, Email: email, PasswordHash: string(hash)}
	userRepo.On("GetByEmail", ctx, email).Return(user, nil)
	userRepo.On("GetByID", ctx, userID).Return(user, nil)

	codeRe := regexp.MustCompile(`Your login code is: (\d{5})`)
	var emailedCode string
	emailSvc.On("SendAdminNotification", ctx, email, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			message := args.Get(3).(string)
			matches := codeRe.FindStringSubmatch(message)
			require.Len(t, matches, 2, "email body must contain a 5-digit login code: %q", message)
			emailedCode = matches[1]
		}).Return(nil)

	sessionToken, requires2FA, tempPwd, err := svc.Login(ctx, email, password)
	require.NoError(t, err)
	assert.True(t, requires2FA)
	assert.False(t, tempPwd)
	assert.NotEmpty(t, sessionToken)
	emailSvc.AssertCalled(t, "SendAdminNotification", ctx, email, mock.Anything, mock.Anything)
	require.NotEmpty(t, emailedCode, "a random 5-digit code must have been emailed")
	require.Len(t, emailedCode, 5)
	assert.NotEqual(t, "00000", emailedCode, "must not fall back to the fixed test passcode when enabled=true")

	// The emailed code — not the configured FixedPasscode — must be the one Verify2FA accepts.
	accessToken, refreshToken, verifiedUser, verifiedTempPwd, err := svc.Verify2FA(ctx, userID, emailedCode, tempPwd)
	require.NoError(t, err, "Verify2FA must accept the code that was actually emailed")
	assert.NotEmpty(t, accessToken)
	assert.NotEmpty(t, refreshToken)
	assert.Equal(t, userID, verifiedUser.ID)
	assert.False(t, verifiedTempPwd)
}

func newAuthServiceForTest() (service.AuthService, *MockUserRepo, *MockEmailService, *MockPendingCredentialsRepo) {
	userRepo := new(MockUserRepo)
	inviteRepo := new(MockInviteRepo)
	reqRepo := new(MockJoinRequestRepo)
	orgRepo := new(MockOrganizationRepo)
	noteRepo := new(MockNotificationRepo)
	emailSvc := new(MockEmailService)
	fcmRepo := new(MockFcmTokenRepo)
	pendingCredsRepo := new(MockPendingCredentialsRepo)
	legalConsentRepo := new(MockLegalConsentRepo)
	svc := service.NewAuthService(userRepo, inviteRepo, reqRepo, orgRepo, noteRepo, emailSvc, "secret", fcmRepo, pendingCredsRepo, legalConsentRepo,
		config.TwoFAConfig{Enabled: false, FixedPasscode: "00000"})
	return svc, userRepo, emailSvc, pendingCredsRepo
}

// TestAuthService_ChangePassword covers FR-008: ChangePassword must accept either the
// canonical password or a valid pending (temporary) credential as proof of the old password,
// and on success must update the canonical hash and stamp any outstanding temp credential used.
func TestAuthService_ChangePassword(t *testing.T) {
	ctx := context.Background()
	const userID = int32(11)
	const oldPassword = "old-password-1"
	const newPassword = "new-password-2"

	t.Run("Success via canonical password", func(t *testing.T) {
		svc, userRepo, _, pendingCredsRepo := newAuthServiceForTest()
		hash, err := bcrypt.GenerateFromPassword([]byte(oldPassword), bcrypt.DefaultCost)
		require.NoError(t, err)
		userRepo.On("GetByID", ctx, userID).Return(&domain.User{ID: userID, PasswordHash: string(hash)}, nil)
		userRepo.On("UpdatePassword", ctx, userID, mock.MatchedBy(func(newHash string) bool {
			return bcrypt.CompareHashAndPassword([]byte(newHash), []byte(newPassword)) == nil
		})).Return(nil)
		pendingCredsRepo.On("StampUsedAt", ctx, userID).Return(nil)

		err = svc.ChangePassword(ctx, userID, oldPassword, newPassword)
		require.NoError(t, err)
		userRepo.AssertCalled(t, "UpdatePassword", ctx, userID, mock.Anything)
		pendingCredsRepo.AssertCalled(t, "StampUsedAt", ctx, userID)
	})

	t.Run("Success via valid pending credential", func(t *testing.T) {
		svc, userRepo, _, pendingCredsRepo := newAuthServiceForTest()
		// Canonical hash deliberately does not match oldPassword — only the pending credential does.
		canonicalHash, err := bcrypt.GenerateFromPassword([]byte("unrelated-password"), bcrypt.DefaultCost)
		require.NoError(t, err)
		tempHash, err := bcrypt.GenerateFromPassword([]byte(oldPassword), bcrypt.DefaultCost)
		require.NoError(t, err)
		userRepo.On("GetByID", ctx, userID).Return(&domain.User{ID: userID, PasswordHash: string(canonicalHash)}, nil)
		pendingCredsRepo.On("GetByUserID", ctx, userID).Return(&domain.PendingCredential{
			UserID: userID, TempPasswordHash: string(tempHash), ExpiresAt: time.Now().Add(20 * time.Minute),
		}, nil)
		userRepo.On("UpdatePassword", ctx, userID, mock.Anything).Return(nil)
		pendingCredsRepo.On("StampUsedAt", ctx, userID).Return(nil)

		err = svc.ChangePassword(ctx, userID, oldPassword, newPassword)
		require.NoError(t, err, "must fall back to the pending credential when the canonical password doesn't match")
	})

	t.Run("Rejects a wrong old password", func(t *testing.T) {
		svc, userRepo, _, pendingCredsRepo := newAuthServiceForTest()
		hash, err := bcrypt.GenerateFromPassword([]byte(oldPassword), bcrypt.DefaultCost)
		require.NoError(t, err)
		userRepo.On("GetByID", ctx, userID).Return(&domain.User{ID: userID, PasswordHash: string(hash)}, nil)
		pendingCredsRepo.On("GetByUserID", ctx, userID).Return(nil, nil)

		err = svc.ChangePassword(ctx, userID, "totally-wrong-password", newPassword)
		require.Error(t, err)
		userRepo.AssertNotCalled(t, "UpdatePassword", mock.Anything, mock.Anything, mock.Anything)
	})
}

// TestAuthService_ResetPassword covers FR-009: ResetPassword must return an identical generic
// outcome (no error) regardless of whether the account exists, only creating/emailing a
// temporary credential when it does. This is a regression test for a real bug found during
// SBR remediation: the not-found branch previously returned a non-nil error, which the gRPC
// handler propagated as a distinguishable error response — a user-enumeration vulnerability.
func TestAuthService_ResetPassword(t *testing.T) {
	ctx := context.Background()

	t.Run("Non-existent email returns no error", func(t *testing.T) {
		svc, userRepo, emailSvc, _ := newAuthServiceForTest()
		userRepo.On("GetByEmail", ctx, "nobody@example.com").Return(nil, nil)

		err := svc.ResetPassword(ctx, "nobody@example.com")
		require.NoError(t, err, "must not be distinguishable from the found-user case at the caller")
		emailSvc.AssertNotCalled(t, "SendAdminNotification", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("Existing email creates and emails a temporary credential", func(t *testing.T) {
		svc, userRepo, emailSvc, pendingCredsRepo := newAuthServiceForTest()
		user := &domain.User{ID: 22, Email: "real-user@example.com"}
		userRepo.On("GetByEmail", ctx, "real-user@example.com").Return(user, nil)
		pendingCredsRepo.On("Upsert", ctx, mock.MatchedBy(func(cred *domain.PendingCredential) bool {
			return cred.UserID == user.ID && cred.TempPasswordHash != "" && cred.UsedAt == nil &&
				cred.ExpiresAt.After(time.Now())
		})).Return(nil)
		emailSvc.On("SendAdminNotification", ctx, user.Email, mock.Anything, mock.Anything).Return(nil)

		err := svc.ResetPassword(ctx, "real-user@example.com")
		require.NoError(t, err)
		pendingCredsRepo.AssertCalled(t, "Upsert", ctx, mock.Anything)
		emailSvc.AssertCalled(t, "SendAdminNotification", ctx, user.Email, mock.Anything, mock.Anything)
	})
}

// TestAuthService_Signup covers FR-006: UserSignup must reject an already-registered email,
// reject an expired/used/invalid invitation, and on success create the user, mark the
// invitation used, link any originating join request JOINED, and add the user as MEMBER.
func TestAuthService_Signup(t *testing.T) {
	ctx := context.Background()
	const orgID = int32(5)
	const email = "signup-target@example.com"
	const token = "SIG-NUP-TOK"

	newSvc := func() (service.AuthService, *MockUserRepo, *MockInviteRepo, *MockJoinRequestRepo) {
		userRepo := new(MockUserRepo)
		inviteRepo := new(MockInviteRepo)
		reqRepo := new(MockJoinRequestRepo)
		orgRepo := new(MockOrganizationRepo)
		noteRepo := new(MockNotificationRepo)
		emailSvc := new(MockEmailService)
		fcmRepo := new(MockFcmTokenRepo)
		pendingCredsRepo := new(MockPendingCredentialsRepo)
		legalConsentRepo := new(MockLegalConsentRepo)
		svc := service.NewAuthService(userRepo, inviteRepo, reqRepo, orgRepo, noteRepo, emailSvc, "secret", fcmRepo, pendingCredsRepo, legalConsentRepo,
			config.TwoFAConfig{Enabled: false, FixedPasscode: "00000"})
		return svc, userRepo, inviteRepo, reqRepo
	}

	t.Run("Rejects when the user already exists", func(t *testing.T) {
		svc, userRepo, inviteRepo, _ := newSvc()
		invite := &domain.Invitation{
			InvitationCode: token, Email: email, OrgID: orgID,
			ExpiresOn: time.Now().Add(48 * time.Hour).Format("2006-01-02"),
		}
		inviteRepo.On("GetByInvitationCodeAndEmail", ctx, token, email).Return(invite, nil)
		userRepo.On("GetByEmail", ctx, email).Return(&domain.User{ID: 99, Email: email}, nil)

		err := svc.Signup(ctx, token, "New User", email, "555-1234", "password123")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already registered")
		userRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	})

	t.Run("Rejects an expired invitation", func(t *testing.T) {
		svc, userRepo, inviteRepo, _ := newSvc()
		invite := &domain.Invitation{
			InvitationCode: token, Email: email, OrgID: orgID,
			ExpiresOn: time.Now().Add(-24 * time.Hour).Format("2006-01-02"),
		}
		inviteRepo.On("GetByInvitationCodeAndEmail", ctx, token, email).Return(invite, nil)

		err := svc.Signup(ctx, token, "New User", email, "555-1234", "password123")
		require.Error(t, err)
		assert.Equal(t, service.ErrInviteExpired, err)
		userRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	})

	t.Run("Rejects an already-used invitation", func(t *testing.T) {
		svc, userRepo, inviteRepo, _ := newSvc()
		usedOn := time.Now().Format("2006-01-02")
		invite := &domain.Invitation{
			InvitationCode: token, Email: email, OrgID: orgID,
			ExpiresOn: time.Now().Add(48 * time.Hour).Format("2006-01-02"),
			UsedOn:    &usedOn,
		}
		inviteRepo.On("GetByInvitationCodeAndEmail", ctx, token, email).Return(invite, nil)

		err := svc.Signup(ctx, token, "New User", email, "555-1234", "password123")
		require.Error(t, err)
		assert.Equal(t, service.ErrInviteUsed, err)
		userRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	})

	t.Run("Rejects an invalid invitation code", func(t *testing.T) {
		svc, userRepo, inviteRepo, _ := newSvc()
		inviteRepo.On("GetByInvitationCodeAndEmail", ctx, "bogus-token", email).Return(nil, assert.AnError)

		err := svc.Signup(ctx, "bogus-token", "New User", email, "555-1234", "password123")
		require.Error(t, err)
		userRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	})

	t.Run("Success links an originating join request as JOINED", func(t *testing.T) {
		svc, userRepo, inviteRepo, reqRepo := newSvc()
		joinReqID := int32(42)
		invite := &domain.Invitation{
			InvitationCode: token, Email: email, OrgID: orgID, JoinRequestID: &joinReqID,
			ExpiresOn: time.Now().Add(48 * time.Hour).Format("2006-01-02"),
		}
		inviteRepo.On("GetByInvitationCodeAndEmail", ctx, token, email).Return(invite, nil)
		userRepo.On("GetByEmail", ctx, email).Return(nil, assert.AnError)
		userRepo.On("Create", ctx, mock.MatchedBy(func(u *domain.User) bool {
			u.ID = 101 // simulate DB-assigned ID, as the real repo would on insert
			return u.Email == email
		})).Return(nil)
		inviteRepo.On("Update", ctx, mock.MatchedBy(func(inv *domain.Invitation) bool {
			return inv.UsedOn != nil && inv.UsedByUserID != nil && *inv.UsedByUserID == 101
		})).Return(nil)
		joinReq := &domain.JoinRequest{ID: joinReqID, OrgID: orgID, Email: email, Status: domain.JoinRequestStatusPending}
		reqRepo.On("GetByID", ctx, joinReqID).Return(joinReq, nil)
		reqRepo.On("Update", ctx, mock.MatchedBy(func(r *domain.JoinRequest) bool {
			return r.Status == domain.JoinRequestStatusJoined
		})).Return(nil)
		userRepo.On("AddUserToOrg", ctx, mock.MatchedBy(func(uo *domain.UserOrg) bool {
			return uo.OrgID == orgID && uo.Role == domain.UserOrgRoleMember
		})).Return(nil)

		err := svc.Signup(ctx, token, "New User", email, "555-1234", "password123")
		require.NoError(t, err)
		reqRepo.AssertCalled(t, "Update", ctx, mock.MatchedBy(func(r *domain.JoinRequest) bool {
			return r.Status == domain.JoinRequestStatusJoined
		}))
	})
}

// TestAuthService_Verify2FA_CodeReuse covers FR-003: a 2FA code MUST be deleted immediately
// after successful use, so a second Verify2FA call with the same (now-consumed) code must fail
// even though it was valid moments earlier.
func TestAuthService_Verify2FA_CodeReuse(t *testing.T) {
	svc, userRepo, _, _ := newAuthServiceForTest()
	ctx := context.Background()
	const userID = int32(55)
	const email = "reuse-2fa@example.com"
	const password = "correct-horse-battery-staple"

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	require.NoError(t, err)
	user := &domain.User{ID: userID, Email: email, PasswordHash: string(hash)}
	userRepo.On("GetByEmail", ctx, email).Return(user, nil)
	userRepo.On("GetByID", ctx, userID).Return(user, nil)

	_, requires2FA, tempPwd, err := svc.Login(ctx, email, password)
	require.NoError(t, err)
	require.True(t, requires2FA)

	// The fixed test passcode ("00000") is used since this service was constructed with
	// TwoFAConfig{Enabled: false}.
	accessToken, refreshToken, verifiedUser, _, err := svc.Verify2FA(ctx, userID, "00000", tempPwd)
	require.NoError(t, err, "the first use of a valid code must succeed")
	assert.NotEmpty(t, accessToken)
	assert.NotEmpty(t, refreshToken)
	assert.Equal(t, userID, verifiedUser.ID)

	_, _, _, _, err = svc.Verify2FA(ctx, userID, "00000", tempPwd)
	require.Error(t, err, "re-submitting an already-consumed code must fail")
	assert.Equal(t, service.ErrInvalid2FACode, err)
}

// TestAuthService_Logout covers FR-005: Logout must mark the caller's FCM token(s) OBSOLETE
// for the given device. Prior to this test, `grep -ri Logout tests/` returned zero matches.
func TestAuthService_Logout(t *testing.T) {
	ctx := context.Background()
	const userID = int32(60)
	const androidDeviceID = "device-abc-123"

	t.Run("Marks the device's FCM tokens obsolete", func(t *testing.T) {
		userRepo := new(MockUserRepo)
		inviteRepo := new(MockInviteRepo)
		reqRepo := new(MockJoinRequestRepo)
		orgRepo := new(MockOrganizationRepo)
		noteRepo := new(MockNotificationRepo)
		emailSvc := new(MockEmailService)
		fcmRepo := new(MockFcmTokenRepo)
		pendingCredsRepo := new(MockPendingCredentialsRepo)
		legalConsentRepo := new(MockLegalConsentRepo)
		svc := service.NewAuthService(userRepo, inviteRepo, reqRepo, orgRepo, noteRepo, emailSvc, "secret", fcmRepo, pendingCredsRepo, legalConsentRepo,
			config.TwoFAConfig{Enabled: false, FixedPasscode: "00000"})

		fcmRepo.On("MarkObsoleteByDevice", ctx, userID, androidDeviceID).Return(nil)

		err := svc.Logout(ctx, userID, "some-refresh-token", androidDeviceID)
		require.NoError(t, err)
		fcmRepo.AssertCalled(t, "MarkObsoleteByDevice", ctx, userID, androidDeviceID)
	})

	t.Run("A failure to mark tokens obsolete does not fail the logout", func(t *testing.T) {
		userRepo := new(MockUserRepo)
		inviteRepo := new(MockInviteRepo)
		reqRepo := new(MockJoinRequestRepo)
		orgRepo := new(MockOrganizationRepo)
		noteRepo := new(MockNotificationRepo)
		emailSvc := new(MockEmailService)
		fcmRepo := new(MockFcmTokenRepo)
		pendingCredsRepo := new(MockPendingCredentialsRepo)
		legalConsentRepo := new(MockLegalConsentRepo)
		svc := service.NewAuthService(userRepo, inviteRepo, reqRepo, orgRepo, noteRepo, emailSvc, "secret", fcmRepo, pendingCredsRepo, legalConsentRepo,
			config.TwoFAConfig{Enabled: false, FixedPasscode: "00000"})

		fcmRepo.On("MarkObsoleteByDevice", ctx, userID, androidDeviceID).Return(assert.AnError)

		err := svc.Logout(ctx, userID, "some-refresh-token", androidDeviceID)
		require.NoError(t, err, "logout must succeed even if marking FCM tokens obsolete fails")
	})
}

// TestAuthService_RecordLegalConsent covers FR-010: RecordLegalConsent must forward the given
// doc names and version to the legal consent repository and propagate any repository error.
// (INVALID_ARGUMENT validation on empty doc_names/version happens at the gRPC handler layer —
// see internal/api/grpc/auth.go — and is exercised at e2e in tests/e2e/auth_test.go.) Prior to
// this test, `grep -r RecordLegalConsent tests/` returned zero calls to the method.
func TestAuthService_RecordLegalConsent(t *testing.T) {
	ctx := context.Background()
	const userID = int32(70)
	docNames := []string{"01_terms_of_service", "02_privacy_policy"}
	const version = "2026-01"

	t.Run("Forwards to the repository and succeeds", func(t *testing.T) {
		svc, _, _, _, legalConsentRepo := newAuthServiceForTestWithLegalConsent()
		legalConsentRepo.On("Record", ctx, userID, docNames, version).Return(nil)

		err := svc.RecordLegalConsent(ctx, userID, docNames, version)
		require.NoError(t, err)
		legalConsentRepo.AssertCalled(t, "Record", ctx, userID, docNames, version)
	})

	t.Run("Propagates a repository error", func(t *testing.T) {
		svc, _, _, _, legalConsentRepo := newAuthServiceForTestWithLegalConsent()
		legalConsentRepo.On("Record", ctx, userID, docNames, version).Return(assert.AnError)

		err := svc.RecordLegalConsent(ctx, userID, docNames, version)
		require.Error(t, err)
	})
}

// TestAuthService_GetUserConsentStatus covers FR-011: GetUserConsentStatus must compute
// pending_docs against the fixed domain.KnownLegalDocs list, considering only consent rows
// matching currentVersion. (INVALID_ARGUMENT validation on empty current_version happens at the
// gRPC handler layer and is exercised at e2e.) Prior to this test, no test anywhere called
// GetUserConsentStatus.
func TestAuthService_GetUserConsentStatus(t *testing.T) {
	ctx := context.Background()
	const userID = int32(71)
	const currentVersion = "2026-01"

	t.Run("All current when every known doc is consented at the current version", func(t *testing.T) {
		svc, _, _, _, legalConsentRepo := newAuthServiceForTestWithLegalConsent()
		var existing []domain.LegalConsent
		for _, doc := range domain.KnownLegalDocs {
			existing = append(existing, domain.LegalConsent{UserID: userID, DocName: doc, Version: currentVersion})
		}
		legalConsentRepo.On("ListByUser", ctx, userID).Return(existing, nil)

		allCurrent, pending, err := svc.GetUserConsentStatus(ctx, userID, currentVersion)
		require.NoError(t, err)
		assert.True(t, allCurrent)
		assert.Empty(t, pending)
	})

	t.Run("Returns docs not yet consented at the current version", func(t *testing.T) {
		svc, _, _, _, legalConsentRepo := newAuthServiceForTestWithLegalConsent()
		// Consented to the first known doc only, and at a stale version for a second.
		existing := []domain.LegalConsent{
			{UserID: userID, DocName: domain.KnownLegalDocs[0], Version: currentVersion},
			{UserID: userID, DocName: domain.KnownLegalDocs[1], Version: "2025-06"},
		}
		legalConsentRepo.On("ListByUser", ctx, userID).Return(existing, nil)

		allCurrent, pending, err := svc.GetUserConsentStatus(ctx, userID, currentVersion)
		require.NoError(t, err)
		assert.False(t, allCurrent)
		assert.Contains(t, pending, domain.KnownLegalDocs[1], "stale-version consent must not count as current")
		for _, doc := range domain.KnownLegalDocs[2:] {
			assert.Contains(t, pending, doc)
		}
		assert.NotContains(t, pending, domain.KnownLegalDocs[0])
	})

	t.Run("Propagates a repository error", func(t *testing.T) {
		svc, _, _, _, legalConsentRepo := newAuthServiceForTestWithLegalConsent()
		legalConsentRepo.On("ListByUser", ctx, userID).Return(nil, assert.AnError)

		_, _, err := svc.GetUserConsentStatus(ctx, userID, currentVersion)
		require.Error(t, err)
	})
}

func newAuthServiceForTestWithLegalConsent() (service.AuthService, *MockUserRepo, *MockEmailService, *MockPendingCredentialsRepo, *MockLegalConsentRepo) {
	userRepo := new(MockUserRepo)
	inviteRepo := new(MockInviteRepo)
	reqRepo := new(MockJoinRequestRepo)
	orgRepo := new(MockOrganizationRepo)
	noteRepo := new(MockNotificationRepo)
	emailSvc := new(MockEmailService)
	fcmRepo := new(MockFcmTokenRepo)
	pendingCredsRepo := new(MockPendingCredentialsRepo)
	legalConsentRepo := new(MockLegalConsentRepo)
	svc := service.NewAuthService(userRepo, inviteRepo, reqRepo, orgRepo, noteRepo, emailSvc, "secret", fcmRepo, pendingCredsRepo, legalConsentRepo,
		config.TwoFAConfig{Enabled: false, FixedPasscode: "00000"})
	return svc, userRepo, emailSvc, pendingCredsRepo, legalConsentRepo
}
