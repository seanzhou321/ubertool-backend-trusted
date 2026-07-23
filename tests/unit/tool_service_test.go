package unit

import (
	"context"
	"testing"

	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestToolService_AddTool(t *testing.T) {
	repo := new(MockToolRepo)
	userRepo := new(MockUserRepo)
	orgRepo := new(MockOrganizationRepo)
	svc := service.NewToolService(repo, userRepo, orgRepo)
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		tool := &domain.Tool{Name: "Tool"}
		repo.On("Create", ctx, tool).Return(nil)

		err := svc.AddTool(ctx, tool, []string{})
		assert.NoError(t, err)
	})
}

// TestToolService_UpdateDelete_RequiresOwnership locks in the fix for the ownership gap
// found while writing specs/007-tools-image-storage/spec.md: UpdateTool and DeleteTool
// previously performed no ownership check at any layer (handler, service, or repository),
// so any authenticated user could modify or soft-delete any tool by ID. Regression coverage
// for a real, previously unguarded vulnerability — do not remove without replacing.
func TestToolService_UpdateDelete_RequiresOwnership(t *testing.T) {
	const toolID = int32(5)
	const ownerID = int32(1)
	const otherUserID = int32(2)

	t.Run("UpdateTool rejects a non-owner caller", func(t *testing.T) {
		repo := new(MockToolRepo)
		svc := service.NewToolService(repo, new(MockUserRepo), new(MockOrganizationRepo))
		ctx := context.Background()

		repo.On("GetByID", ctx, toolID).Return(&domain.Tool{ID: toolID, OwnerID: ownerID}, nil).Once()

		err := svc.UpdateTool(ctx, otherUserID, &domain.Tool{ID: toolID, Name: "Hijacked"})
		assert.Error(t, err)
		repo.AssertNotCalled(t, "Update", ctx, mock.Anything)
		repo.AssertExpectations(t)
	})

	t.Run("UpdateTool accepts the actual owner", func(t *testing.T) {
		repo := new(MockToolRepo)
		svc := service.NewToolService(repo, new(MockUserRepo), new(MockOrganizationRepo))
		ctx := context.Background()

		tool := &domain.Tool{ID: toolID, Name: "Updated"}
		repo.On("GetByID", ctx, toolID).Return(&domain.Tool{ID: toolID, OwnerID: ownerID}, nil).Once()
		repo.On("Update", ctx, tool).Return(nil).Once()

		err := svc.UpdateTool(ctx, ownerID, tool)
		assert.NoError(t, err)
		repo.AssertExpectations(t)
	})

	t.Run("DeleteTool rejects a non-owner caller", func(t *testing.T) {
		repo := new(MockToolRepo)
		svc := service.NewToolService(repo, new(MockUserRepo), new(MockOrganizationRepo))
		ctx := context.Background()

		repo.On("GetByID", ctx, toolID).Return(&domain.Tool{ID: toolID, OwnerID: ownerID}, nil).Once()

		err := svc.DeleteTool(ctx, otherUserID, toolID)
		assert.Error(t, err)
		repo.AssertNotCalled(t, "Delete", ctx, toolID)
		repo.AssertExpectations(t)
	})

	t.Run("DeleteTool accepts the actual owner", func(t *testing.T) {
		repo := new(MockToolRepo)
		svc := service.NewToolService(repo, new(MockUserRepo), new(MockOrganizationRepo))
		ctx := context.Background()

		repo.On("GetByID", ctx, toolID).Return(&domain.Tool{ID: toolID, OwnerID: ownerID}, nil).Once()
		repo.On("Delete", ctx, toolID).Return(nil).Once()

		err := svc.DeleteTool(ctx, ownerID, toolID)
		assert.NoError(t, err)
		repo.AssertExpectations(t)
	})
}

func TestToolService_SearchTools(t *testing.T) {
	repo := new(MockToolRepo)
	userRepo := new(MockUserRepo) // Need user repo now
	orgRepo := new(MockOrganizationRepo)
	svc := service.NewToolService(repo, userRepo, orgRepo)
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		tools := []domain.Tool{{ID: 1, OwnerID: 5, Name: "Hammer"}}
		repo.On("Search", ctx, int32(1), "San Jose", "query", []string{"cat"}, int32(100), "NOT_DAMAGED", int32(1), int32(10)).
			Return(tools, int32(1), nil)

		// Mock GetUserOrg logic if orgID != 0
		if 1 != 0 {
			userRepo.On("GetUserOrg", ctx, int32(1), int32(1)).Return(&domain.UserOrg{}, nil)
			orgRepo.On("GetByID", ctx, int32(1)).Return(&domain.Organization{ID: 1, Name: "Test Org", Metro: "San Jose"}, nil)
		}

		// Mock owner population calls
		owner := &domain.User{ID: 5, Name: "Owner User"}
		userRepo.On("GetByID", ctx, int32(5)).Return(owner, nil)
		userRepo.On("ListUserOrgs", ctx, int32(5)).Return([]domain.UserOrg{{OrgID: 1}}, nil)
		userRepo.On("ListUserOrgs", ctx, int32(1)).Return([]domain.UserOrg{{OrgID: 1}}, nil)
		orgRepo.On("GetByID", ctx, int32(1)).Return(&domain.Organization{ID: 1, Name: "Test Org"}, nil)

		res, total, err := svc.SearchTools(ctx, 1, 1, "", "query", []string{"cat"}, 100, "", 1, 10)
		assert.NoError(t, err)
		assert.Equal(t, int32(1), total)
		assert.Equal(t, "Hammer", res[0].Name)
		assert.NotNil(t, res[0].Owner)
		assert.Equal(t, "Owner User", res[0].Owner.Name)
	})

	t.Run("SharedOrgFiltering_ReturnsToolWhenUsersShareOrg", func(t *testing.T) {
		// Scenario: Tool 141 owned by user 1, requesting user 301, both in org 1
		tools := []domain.Tool{{ID: 141, OwnerID: 1, Name: "Shared Tool"}}
		repo.On("Search", ctx, int32(301), "San Francisco", "st", []string(nil), int32(0), "NOT_DAMAGED", int32(1), int32(10)).
			Return(tools, int32(1), nil)

		// Mock owner population - both users in org 1
		owner := &domain.User{ID: 1, Name: "Owner 1"}
		userRepo.On("GetByID", ctx, int32(1)).Return(owner, nil)
		userRepo.On("ListUserOrgs", ctx, int32(1)).Return([]domain.UserOrg{{OrgID: 1}}, nil)
		userRepo.On("ListUserOrgs", ctx, int32(301)).Return([]domain.UserOrg{{OrgID: 1}}, nil)
		orgRepo.On("GetByID", ctx, int32(1)).Return(&domain.Organization{ID: 1, Name: "Shared Org", Metro: "San Francisco"}, nil)

		res, total, err := svc.SearchTools(ctx, 301, 0, "San Francisco", "st", nil, 0, "", 1, 10)
		assert.NoError(t, err)
		assert.Equal(t, int32(1), total, "Should return 1 tool since users share org")
		assert.Len(t, res, 1)
		assert.Equal(t, int32(141), res[0].ID)
		assert.Equal(t, "Shared Tool", res[0].Name)
		assert.NotNil(t, res[0].Owner)
		assert.Len(t, res[0].Owner.Orgs, 1, "Should have 1 shared org")
		assert.Equal(t, int32(1), res[0].Owner.Orgs[0].ID)
	})

	t.Run("SharedOrgFiltering_FiltersOutToolWhenNoSharedOrg", func(t *testing.T) {
		// Create fresh mocks for this test to avoid conflicts with previous mocks
		repo2 := new(MockToolRepo)
		userRepo2 := new(MockUserRepo)
		orgRepo2 := new(MockOrganizationRepo)
		svc2 := service.NewToolService(repo2, userRepo2, orgRepo2)

		// Scenario: Tool owned by user 5 in org 2, requesting user 301 in org 3 - no shared org
		tools := []domain.Tool{{ID: 200, OwnerID: 5, Name: "Different Org Tool"}}
		repo2.On("Search", ctx, int32(301), "San Francisco", "tool", []string(nil), int32(0), "NOT_DAMAGED", int32(1), int32(10)).
			Return(tools, int32(1), nil)

		// Mock owner population - different orgs (no overlap)
		owner := &domain.User{ID: 5, Name: "Owner 5"}
		userRepo2.On("GetByID", ctx, int32(5)).Return(owner, nil)
		userRepo2.On("ListUserOrgs", ctx, int32(5)).Return([]domain.UserOrg{{OrgID: 2}}, nil)   // Owner in org 2
		userRepo2.On("ListUserOrgs", ctx, int32(301)).Return([]domain.UserOrg{{OrgID: 3}}, nil) // Requesting user in org 3 (different!)

		res, total, err := svc2.SearchTools(ctx, 301, 0, "San Francisco", "tool", nil, 0, "", 1, 10)
		assert.NoError(t, err)
		assert.Equal(t, int32(0), total, "Should return 0 tools since users don't share org")
		assert.Len(t, res, 0, "Tool should be filtered out")
	})

	// FR-003 (specs/006-tools-image-storage): SearchTools must require a non-empty query, and
	// must require an explicit metro when organization_id is not specified. Neither reject
	// clause had any test coverage at any tier prior to this.
	t.Run("Rejects an empty query", func(t *testing.T) {
		repo3 := new(MockToolRepo)
		userRepo3 := new(MockUserRepo)
		orgRepo3 := new(MockOrganizationRepo)
		svc3 := service.NewToolService(repo3, userRepo3, orgRepo3)

		_, _, err := svc3.SearchTools(ctx, 301, 1, "San Jose", "", nil, 0, "", 1, 10)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "query parameter is required")
		repo3.AssertNotCalled(t, "Search", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("Rejects an empty metro when organization_id is not specified", func(t *testing.T) {
		repo3 := new(MockToolRepo)
		userRepo3 := new(MockUserRepo)
		orgRepo3 := new(MockOrganizationRepo)
		svc3 := service.NewToolService(repo3, userRepo3, orgRepo3)

		_, _, err := svc3.SearchTools(ctx, 301, 0, "", "drill", nil, 0, "", 1, 10)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "metro parameter is required")
		repo3.AssertNotCalled(t, "Search", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})
}

func TestToolService_GetSharedOrganizations(t *testing.T) {
	ctx := context.Background()

	t.Run("ReturnsSharedOrgs_WhenUsersShareOrg", func(t *testing.T) {
		userRepo := new(MockUserRepo)
		orgRepo := new(MockOrganizationRepo)
		toolRepo := new(MockToolRepo)
		svc := service.NewToolService(toolRepo, userRepo, orgRepo)

		// Setup: User 1 and User 301 both in org 1
		userRepo.On("ListUserOrgs", ctx, int32(1)).Return([]domain.UserOrg{{OrgID: 1}}, nil)
		userRepo.On("ListUserOrgs", ctx, int32(301)).Return([]domain.UserOrg{{OrgID: 1}}, nil)
		orgRepo.On("GetByID", ctx, int32(1)).Return(&domain.Organization{ID: 1, Name: "Shared Org"}, nil)

		// This is a workaround to test the private method getSharedOrganizations
		// We test it indirectly through populateToolOwner
		tool := &domain.Tool{ID: 141, OwnerID: 1, Name: "Test Tool"}
		owner := &domain.User{ID: 1, Name: "Owner 1"}
		userRepo.On("GetByID", ctx, int32(1)).Return(owner, nil)

		// Access the unexported method via reflection or test through SearchTools
		// For now, we'll create a minimal SearchTools scenario
		toolRepo.On("Search", ctx, int32(301), "San Diego, CA", "test", []string(nil), int32(0), "NOT_DAMAGED", int32(1), int32(10)).
			Return([]domain.Tool{*tool}, int32(1), nil)

		res, total, err := svc.SearchTools(ctx, 301, 0, "San Diego, CA", "test", nil, 0, "", 1, 10)
		assert.NoError(t, err)
		assert.Equal(t, int32(1), total)
		assert.NotNil(t, res[0].Owner)
		assert.NotNil(t, res[0].Owner.Orgs, "Owner.Orgs should not be nil")
		assert.Len(t, res[0].Owner.Orgs, 1, "Should have exactly 1 shared org")
		assert.Equal(t, int32(1), res[0].Owner.Orgs[0].ID)
	})

	t.Run("ReturnsEmptyOrgs_WhenUsersDoNotShareOrg", func(t *testing.T) {
		userRepo := new(MockUserRepo)
		orgRepo := new(MockOrganizationRepo)
		toolRepo := new(MockToolRepo)
		svc := service.NewToolService(toolRepo, userRepo, orgRepo)

		// Setup: Owner in org 2, Requester in org 3
		userRepo.On("ListUserOrgs", ctx, int32(5)).Return([]domain.UserOrg{{OrgID: 2}}, nil)
		userRepo.On("ListUserOrgs", ctx, int32(301)).Return([]domain.UserOrg{{OrgID: 3}}, nil)

		tool := &domain.Tool{ID: 200, OwnerID: 5, Name: "Different Org Tool"}
		owner := &domain.User{ID: 5, Name: "Owner 5"}
		userRepo.On("GetByID", ctx, int32(5)).Return(owner, nil)

		toolRepo.On("Search", ctx, int32(301), "San Diego, CA", "test", []string(nil), int32(0), "NOT_DAMAGED", int32(1), int32(10)).
			Return([]domain.Tool{*tool}, int32(1), nil)

		res, total, err := svc.SearchTools(ctx, 301, 0, "San Diego, CA", "test", nil, 0, "", 1, 10)
		assert.NoError(t, err)
		assert.Equal(t, int32(0), total, "Tool should be filtered out")
		assert.Len(t, res, 0, "No tools should be returned")
	})

	t.Run("HandlesMultipleOrgs_ReturnsOnlyShared", func(t *testing.T) {
		userRepo := new(MockUserRepo)
		orgRepo := new(MockOrganizationRepo)
		toolRepo := new(MockToolRepo)
		svc := service.NewToolService(toolRepo, userRepo, orgRepo)

		// Setup: Owner in orgs 1,2,3  Requester in orgs 2,4
		// Should only return org 2
		userRepo.On("ListUserOrgs", ctx, int32(10)).Return([]domain.UserOrg{
			{OrgID: 1},
			{OrgID: 2},
			{OrgID: 3},
		}, nil)
		userRepo.On("ListUserOrgs", ctx, int32(301)).Return([]domain.UserOrg{
			{OrgID: 2},
			{OrgID: 4},
		}, nil)
		orgRepo.On("GetByID", ctx, int32(2)).Return(&domain.Organization{ID: 2, Name: "Shared Org 2"}, nil)

		tool := &domain.Tool{ID: 300, OwnerID: 10, Name: "Multi Org Tool"}
		owner := &domain.User{ID: 10, Name: "Owner 10"}
		userRepo.On("GetByID", ctx, int32(10)).Return(owner, nil)

		toolRepo.On("Search", ctx, int32(301), "San Diego, CA", "multi", []string(nil), int32(0), "NOT_DAMAGED", int32(1), int32(10)).
			Return([]domain.Tool{*tool}, int32(1), nil)

		res, total, err := svc.SearchTools(ctx, 301, 0, "San Diego, CA", "multi", nil, 0, "", 1, 10)
		assert.NoError(t, err)
		assert.Equal(t, int32(1), total)
		assert.NotNil(t, res[0].Owner)
		assert.Len(t, res[0].Owner.Orgs, 1, "Should have exactly 1 shared org")
		assert.Equal(t, int32(2), res[0].Owner.Orgs[0].ID, "Should return org 2 only")
	})
}
