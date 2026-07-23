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

// TestUserService_GetUserProfile covers FR-001 and FR-002 (specs/002-users): GetUserProfile
// must return every org membership with its per-org role and balance intact, and must NOT fail
// outright when one membership's org record can't be loaded — it should skip that one and
// return the remaining memberships. Prior to this test, no unit test of userService existed at
// all.
func TestUserService_GetUserProfile(t *testing.T) {
	ctx := context.Background()
	const userID = int32(1)

	newSvc := func() (service.UserService, *MockUserRepo, *MockOrganizationRepo) {
		userRepo := new(MockUserRepo)
		orgRepo := new(MockOrganizationRepo)
		svc := service.NewUserService(userRepo, orgRepo)
		return svc, userRepo, orgRepo
	}

	t.Run("Returns each org membership with its role and balance intact", func(t *testing.T) {
		svc, userRepo, orgRepo := newSvc()
		userRepo.On("GetByID", ctx, userID).Return(&domain.User{ID: userID, Name: "User"}, nil)
		userOrgs := []domain.UserOrg{
			{UserID: userID, OrgID: 1, Role: domain.UserOrgRoleMember, BalanceCents: 1000},
			{UserID: userID, OrgID: 2, Role: domain.UserOrgRoleAdmin, BalanceCents: 2500},
		}
		userRepo.On("ListUserOrgs", ctx, userID).Return(userOrgs, nil)
		orgRepo.On("GetByID", ctx, int32(1)).Return(&domain.Organization{ID: 1, Name: "Org1"}, nil)
		orgRepo.On("GetByID", ctx, int32(2)).Return(&domain.Organization{ID: 2, Name: "Org2"}, nil)

		_, orgs, retUserOrgs, err := svc.GetUserProfile(ctx, userID)
		require.NoError(t, err)
		require.Len(t, orgs, 2)
		require.Len(t, retUserOrgs, 2)
		assert.Equal(t, domain.UserOrgRoleMember, retUserOrgs[0].Role)
		assert.EqualValues(t, 1000, retUserOrgs[0].BalanceCents)
		assert.Equal(t, domain.UserOrgRoleAdmin, retUserOrgs[1].Role)
		assert.EqualValues(t, 2500, retUserOrgs[1].BalanceCents)
	})

	t.Run("Skips a membership whose org record can't be loaded, returning the rest", func(t *testing.T) {
		svc, userRepo, orgRepo := newSvc()
		userRepo.On("GetByID", ctx, userID).Return(&domain.User{ID: userID, Name: "User"}, nil)
		userOrgs := []domain.UserOrg{
			{UserID: userID, OrgID: 1, Role: domain.UserOrgRoleMember, BalanceCents: 1000},
			{UserID: userID, OrgID: 2, Role: domain.UserOrgRoleAdmin, BalanceCents: 2500},
			{UserID: userID, OrgID: 3, Role: domain.UserOrgRoleMember, BalanceCents: 500},
		}
		userRepo.On("ListUserOrgs", ctx, userID).Return(userOrgs, nil)
		orgRepo.On("GetByID", ctx, int32(1)).Return(&domain.Organization{ID: 1, Name: "Org1"}, nil)
		// Org 2's lookup fails — must be skipped, not fail the whole call.
		orgRepo.On("GetByID", ctx, int32(2)).Return(nil, assert.AnError)
		orgRepo.On("GetByID", ctx, int32(3)).Return(&domain.Organization{ID: 3, Name: "Org3"}, nil)

		_, orgs, _, err := svc.GetUserProfile(ctx, userID)
		require.NoError(t, err, "a single failed org lookup must not fail the whole profile fetch")
		require.Len(t, orgs, 2, "must return the 2 orgs that loaded successfully, skipping the failed one")
		var ids []int32
		for _, o := range orgs {
			ids = append(ids, o.ID)
		}
		assert.ElementsMatch(t, []int32{1, 3}, ids)
	})
}

// TestUserService_UpdateProfile covers FR-003 (specs/002-users): UpdateProfile must overwrite
// all 4 fields unconditionally from the request — including clearing a field when the request
// submits it blank — with no partial-update behavior (i.e. no "0 means keep existing" semantics
// like org thresholds have elsewhere in this codebase).
func TestUserService_UpdateProfile(t *testing.T) {
	ctx := context.Background()
	const userID = int32(7)

	t.Run("Blanking a field clears it rather than preserving the prior value", func(t *testing.T) {
		userRepo := new(MockUserRepo)
		orgRepo := new(MockOrganizationRepo)
		svc := service.NewUserService(userRepo, orgRepo)

		existing := &domain.User{
			ID: userID, Name: "Old Name", Email: "old@test.com",
			PhoneNumber: "555-0000", AvatarURL: "https://old.example.com/avatar.png",
		}
		userRepo.On("GetByID", ctx, userID).Return(existing, nil)
		userRepo.On("Update", ctx, mock.MatchedBy(func(u *domain.User) bool {
			return u.Name == "New Name" && u.Email == "new@test.com" &&
				u.PhoneNumber == "" && u.AvatarURL == ""
		})).Return(nil)

		err := svc.UpdateProfile(ctx, userID, "New Name", "new@test.com", "", "")
		require.NoError(t, err)
		userRepo.AssertExpectations(t)
	})
}
