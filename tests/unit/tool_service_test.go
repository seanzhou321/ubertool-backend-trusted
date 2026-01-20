package unit

import (
	"context"
	"testing"

	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/service"
	"github.com/stretchr/testify/assert"
)

func TestToolService_AddTool(t *testing.T) {
	repo := new(MockToolRepo)
	userRepo := new(MockUserRepo)
	svc := service.NewToolService(repo, userRepo)
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		tool := &domain.Tool{Name: "Tool"}
		repo.On("Create", ctx, tool).Return(nil)
		
		err := svc.AddTool(ctx, tool, []string{})
		assert.NoError(t, err)
	})
}

func TestToolService_SearchTools(t *testing.T) {
	repo := new(MockToolRepo)
	userRepo := new(MockUserRepo) // Need user repo now
	svc := service.NewToolService(repo, userRepo)
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		tools := []domain.Tool{{Name: "Hammer"}}
		repo.On("Search", ctx, int32(1), "query", []string{"cat"}, int32(100), "condition", int32(1), int32(10)).
			Return(tools, int32(1), nil)
		
		// Mock GetUserOrg logic if orgID != 0
		if 1 != 0 {
			userRepo.On("GetUserOrg", ctx, int32(1), int32(1)).Return(&domain.UserOrg{}, nil)
		}

		res, total, err := svc.SearchTools(ctx, 1, 1, "query", []string{"cat"}, 100, "condition", 1, 10)
		assert.NoError(t, err)
		assert.Equal(t, int32(1), total)
		assert.Equal(t, "Hammer", res[0].Name)
	})
}
