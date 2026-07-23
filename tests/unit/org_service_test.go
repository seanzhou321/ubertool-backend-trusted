package unit

import (
	"context"
	"errors"
	"testing"

	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func newOrgServiceForTest() (service.OrganizationService, *MockOrganizationRepo, *MockUserRepo) {
	mockRepo := new(MockOrganizationRepo)
	mockUserRepo := new(MockUserRepo)
	mockInviteRepo := new(MockInvitationRepo)
	mockNoteRepo := new(MockNotificationRepo)
	svc := service.NewOrganizationService(mockRepo, mockUserRepo, mockInviteRepo, mockNoteRepo, nil, nil)
	return svc, mockRepo, mockUserRepo
}

// TestOrganizationService_UpdateOrganization covers FR-002 (specs/003-organizations-administration):
// UpdateOrganization must reject non-members, require ADMIN/SUPER_ADMIN for any change, and
// additionally require SUPER_ADMIN specifically to change either price threshold field,
// treating a submitted 0 as "no change." Regression test for a spec-vs-test discrepancy found
// during SBR remediation: spec.md's SC-002 claimed this behavior already had regression tests
// locking it in, but only the SUPER_ADMIN success path was actually tested.
func TestOrganizationService_UpdateOrganization(t *testing.T) {
	const callerID = int32(1)
	const orgID = int32(1)

	t.Run("Success as SUPER_ADMIN", func(t *testing.T) {
		svc, mockRepo, mockUserRepo := newOrgServiceForTest()
		ctx := context.Background()

		callerUserOrg := &domain.UserOrg{UserID: callerID, OrgID: orgID, Role: domain.UserOrgRoleSuperAdmin, Status: domain.UserOrgStatusActive}
		mockUserRepo.On("GetUserOrg", ctx, callerID, orgID).Return(callerUserOrg, nil)

		currentOrg := &domain.Organization{ID: orgID, Name: "Old Name"}
		mockRepo.On("GetByID", ctx, orgID).Return(currentOrg, nil)

		org := &domain.Organization{ID: orgID, Name: "Updated"}
		mockRepo.On("Update", ctx, org).Return(nil)

		err := svc.UpdateOrganization(ctx, callerID, org)
		assert.NoError(t, err)
		mockRepo.AssertExpectations(t)
		mockUserRepo.AssertExpectations(t)
	})

	t.Run("Success as ADMIN changing a non-price field", func(t *testing.T) {
		svc, mockRepo, mockUserRepo := newOrgServiceForTest()
		ctx := context.Background()

		callerUserOrg := &domain.UserOrg{UserID: callerID, OrgID: orgID, Role: domain.UserOrgRoleAdmin, Status: domain.UserOrgStatusActive}
		mockUserRepo.On("GetUserOrg", ctx, callerID, orgID).Return(callerUserOrg, nil)
		mockRepo.On("GetByID", ctx, orgID).Return(&domain.Organization{ID: orgID, Name: "Old Name"}, nil)

		org := &domain.Organization{ID: orgID, Name: "Updated by admin"}
		mockRepo.On("Update", ctx, org).Return(nil)

		err := svc.UpdateOrganization(ctx, callerID, org)
		require.NoError(t, err, "ADMIN must be allowed to update non-price fields")
	})

	t.Run("Rejects a caller who is not a member of the org", func(t *testing.T) {
		svc, mockRepo, mockUserRepo := newOrgServiceForTest()
		ctx := context.Background()

		mockUserRepo.On("GetUserOrg", ctx, callerID, orgID).Return(nil, errors.New("no rows"))

		err := svc.UpdateOrganization(ctx, callerID, &domain.Organization{ID: orgID, Name: "Updated"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not a member")
		mockRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
	})

	t.Run("Rejects a plain MEMBER caller", func(t *testing.T) {
		svc, mockRepo, mockUserRepo := newOrgServiceForTest()
		ctx := context.Background()

		callerUserOrg := &domain.UserOrg{UserID: callerID, OrgID: orgID, Role: domain.UserOrgRoleMember, Status: domain.UserOrgStatusActive}
		mockUserRepo.On("GetUserOrg", ctx, callerID, orgID).Return(callerUserOrg, nil)

		err := svc.UpdateOrganization(ctx, callerID, &domain.Organization{ID: orgID, Name: "Updated"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ADMIN or SUPER_ADMIN role required")
		mockRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
	})

	t.Run("Rejects ADMIN attempting to change the settlement threshold", func(t *testing.T) {
		svc, mockRepo, mockUserRepo := newOrgServiceForTest()
		ctx := context.Background()

		callerUserOrg := &domain.UserOrg{UserID: callerID, OrgID: orgID, Role: domain.UserOrgRoleAdmin, Status: domain.UserOrgStatusActive}
		mockUserRepo.On("GetUserOrg", ctx, callerID, orgID).Return(callerUserOrg, nil)

		org := &domain.Organization{ID: orgID, Name: "Updated", SettlementThresholdCents: 5000}
		err := svc.UpdateOrganization(ctx, callerID, org)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "only SUPER_ADMIN can modify payment threshold values")
		mockRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
	})

	t.Run("Rejects ADMIN attempting to change the max billsplit rental cost", func(t *testing.T) {
		svc, mockRepo, mockUserRepo := newOrgServiceForTest()
		ctx := context.Background()

		callerUserOrg := &domain.UserOrg{UserID: callerID, OrgID: orgID, Role: domain.UserOrgRoleAdmin, Status: domain.UserOrgStatusActive}
		mockUserRepo.On("GetUserOrg", ctx, callerID, orgID).Return(callerUserOrg, nil)

		org := &domain.Organization{ID: orgID, Name: "Updated", MaxBillsplitRentalCostCents: 100000}
		err := svc.UpdateOrganization(ctx, callerID, org)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "only SUPER_ADMIN can modify payment threshold values")
		mockRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
	})

	t.Run("A submitted 0 threshold means 'no change', not 'zero it out'", func(t *testing.T) {
		svc, mockRepo, mockUserRepo := newOrgServiceForTest()
		ctx := context.Background()

		callerUserOrg := &domain.UserOrg{UserID: callerID, OrgID: orgID, Role: domain.UserOrgRoleSuperAdmin, Status: domain.UserOrgStatusActive}
		mockUserRepo.On("GetUserOrg", ctx, callerID, orgID).Return(callerUserOrg, nil)

		currentOrg := &domain.Organization{ID: orgID, Name: "Old Name", SettlementThresholdCents: 5000, MaxBillsplitRentalCostCents: 100000}
		mockRepo.On("GetByID", ctx, orgID).Return(currentOrg, nil)

		// Caller submits 0 for both threshold fields — must be treated as "keep existing",
		// not persisted as a literal zero.
		org := &domain.Organization{ID: orgID, Name: "Updated", SettlementThresholdCents: 0, MaxBillsplitRentalCostCents: 0}
		mockRepo.On("Update", ctx, mock.MatchedBy(func(o *domain.Organization) bool {
			return o.SettlementThresholdCents == 5000 && o.MaxBillsplitRentalCostCents == 100000
		})).Return(nil)

		err := svc.UpdateOrganization(ctx, callerID, org)
		require.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})
}
