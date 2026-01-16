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
	svc := service.NewAuthService(userRepo, inviteRepo, reqRepo, "secret")

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
	svc := service.NewAuthService(userRepo, inviteRepo, reqRepo, "secret")

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		reqRepo.On("Create", ctx, mock.AnythingOfType("*domain.JoinRequest")).Return(nil)
		err := svc.RequestToJoin(ctx, 1, "Name", "email@test.com", "Note")
		assert.NoError(t, err)
	})
}
