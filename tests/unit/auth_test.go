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

	t.Run("Valid Token", func(t *testing.T) {
		invite := &domain.Invitation{
			Token:     token,
			ExpiresOn: time.Now().Add(time.Hour),
		}
		inviteRepo.ExpectedCalls = nil
		inviteRepo.On("GetByToken", ctx, token).Return(invite, nil)

		res, err := svc.ValidateInvite(ctx, token)
		assert.NoError(t, err)
		assert.Equal(t, invite, res)
	})

	t.Run("Expired Token", func(t *testing.T) {
		invite := &domain.Invitation{
			Token:     token,
			ExpiresOn: time.Now().Add(-time.Hour),
		}
		inviteRepo.ExpectedCalls = nil
		inviteRepo.On("GetByToken", ctx, token).Return(invite, nil)

		res, err := svc.ValidateInvite(ctx, token)
		assert.Error(t, err)
		assert.Nil(t, res)
		assert.Contains(t, err.Error(), "expired")
	})

	t.Run("Used Token", func(t *testing.T) {
		now := time.Now()
		invite := &domain.Invitation{
			Token:     token,
			ExpiresOn: time.Now().Add(time.Hour),
			UsedOn:    &now,
		}
		inviteRepo.ExpectedCalls = nil
		inviteRepo.On("GetByToken", ctx, token).Return(invite, nil)

		res, err := svc.ValidateInvite(ctx, token)
		assert.Error(t, err)
		assert.Nil(t, res)
		assert.Contains(t, err.Error(), "used")
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
		orgRepo.On("GetByID", ctx, orgID).Return(&domain.Organization{ID: orgID, Name: "Org"}, nil)
		userRepo.On("GetByEmail", ctx, email).Return(nil, nil)
		reqRepo.On("Create", ctx, mock.AnythingOfType("*domain.JoinRequest")).Return(nil)
		
		// Mock ListUsers for admin notification logic
		userRepo.On("ListMembersByOrg", ctx, orgID).Return([]domain.User{}, []domain.UserOrg{}, nil)

		err := svc.RequestToJoin(ctx, orgID, "Name", email, "Note")
		assert.NoError(t, err)
	})
}
