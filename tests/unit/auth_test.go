package unit

import (
	"context"
	"testing"
	"time"

	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestAuthService_ValidateInvite(t *testing.T) {
	userRepo := new(MockUserRepo)
	inviteRepo := new(MockInviteRepo)
	reqRepo := new(MockJoinRequestRepo)
	orgRepo := new(MockOrganizationRepo)
	noteRepo := new(MockNotificationRepo)
	emailSvc := new(MockEmailService)
	svc := service.NewAuthService(userRepo, inviteRepo, reqRepo, orgRepo, noteRepo, emailSvc, "secret")

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

	svc := service.NewAuthService(userRepo, inviteRepo, reqRepo, orgRepo, noteRepo, emailSvc, "secret")

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
}
