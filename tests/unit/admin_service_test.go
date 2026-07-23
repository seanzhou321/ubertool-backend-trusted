package unit

import (
	"context"
	"errors"
	"testing"
	"time"

	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestAdminService_BlockUser(t *testing.T) {
	mockUserRepo := new(MockUserRepo)
	mockJoinRepo := new(MockJoinRequestRepo)
	mockLedgerRepo := new(MockLedgerRepo)
	mockOrgRepo := new(MockOrganizationRepo)
	mockInviteRepo := new(MockInviteRepo)
	mockEmailSvc := new(MockEmailService)
	svc := service.NewAdminService(mockJoinRepo, mockUserRepo, mockLedgerRepo, mockOrgRepo, mockInviteRepo, mockEmailSvc)
	ctx := context.Background()

	t.Run("Block", func(t *testing.T) {
		adminUo := &domain.UserOrg{UserID: 999, OrgID: 1, Role: domain.UserOrgRoleAdmin, Status: domain.UserOrgStatusActive}
		mockUserRepo.On("GetUserOrg", ctx, int32(999), int32(1)).Return(adminUo, nil).Once()

		uo := &domain.UserOrg{UserID: 1, OrgID: 1, Status: domain.UserOrgStatusActive}
		mockUserRepo.On("GetUserOrg", ctx, int32(1), int32(1)).Return(uo, nil).Once()
		mockUserRepo.On("GetByID", ctx, int32(1)).Return(&domain.User{ID: 1, Name: "User 1", Email: "u1@test.com"}, nil).Once()
		mockOrgRepo.On("GetByID", ctx, int32(1)).Return(&domain.Organization{ID: 1, Name: "Test Org"}, nil).Once()
		mockUserRepo.On("UpdateUserOrg", ctx, mock.MatchedBy(func(u *domain.UserOrg) bool {
			return u.Status == domain.UserOrgStatusBlock && u.BlockedReason == "violation" && u.BlockedOn != nil
		})).Return(nil).Once()
		mockEmailSvc.On("SendAccountStatusNotification", ctx, "u1@test.com", "User 1", "Test Org", "BLOCK", "violation").Return(nil).Once()

		err := svc.BlockUser(ctx, 999, 1, 1, true, true, "violation")
		assert.NoError(t, err)
	})

	t.Run("Unblock", func(t *testing.T) {
		adminUo := &domain.UserOrg{UserID: 999, OrgID: 1, Role: domain.UserOrgRoleAdmin, Status: domain.UserOrgStatusActive}
		mockUserRepo.On("GetUserOrg", ctx, int32(999), int32(1)).Return(adminUo, nil).Once()

		dateStr := time.Now().Format("2006-01-02")
		uo := &domain.UserOrg{UserID: 1, OrgID: 1, Status: domain.UserOrgStatusBlock, BlockedReason: "violation", BlockedOn: &dateStr}
		mockUserRepo.On("GetUserOrg", ctx, int32(1), int32(1)).Return(uo, nil).Once()
		mockUserRepo.On("GetByID", ctx, int32(1)).Return(&domain.User{ID: 1, Name: "User 1", Email: "u1@test.com"}, nil).Once()
		mockOrgRepo.On("GetByID", ctx, int32(1)).Return(&domain.Organization{ID: 1, Name: "Test Org"}, nil).Once()
		mockUserRepo.On("UpdateUserOrg", ctx, mock.MatchedBy(func(u *domain.UserOrg) bool {
			return u.Status == domain.UserOrgStatusActive && u.BlockedReason == "" && u.BlockedOn == nil
		})).Return(nil).Once()
		mockEmailSvc.On("SendAccountStatusNotification", ctx, "u1@test.com", "User 1", "Test Org", "ACTIVE", "").Return(nil).Once()

		err := svc.BlockUser(ctx, 999, 1, 1, false, false, "")
		assert.NoError(t, err)
	})

	mockUserRepo.AssertExpectations(t)
	mockEmailSvc.AssertExpectations(t)
}

func TestAdminService_ListMembers(t *testing.T) {
	mockUserRepo := new(MockUserRepo)
	svc := service.NewAdminService(nil, mockUserRepo, nil, nil, nil, nil)
	ctx := context.Background()

	adminUo := &domain.UserOrg{UserID: 999, OrgID: 1, Role: domain.UserOrgRoleAdmin}
	mockUserRepo.On("GetUserOrg", ctx, int32(999), int32(1)).Return(adminUo, nil).Once()

	users := []domain.User{{ID: 1, Name: "User 1"}}
	uos := []domain.UserOrg{{UserID: 1, OrgID: 1}}
	mockUserRepo.On("ListMembersByOrg", ctx, int32(1)).Return(users, uos, nil)

	rUsers, rUos, err := svc.ListMembers(ctx, 999, 1)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(rUsers))
	assert.Equal(t, int32(1), rUos[0].OrgID)
	mockUserRepo.AssertExpectations(t)
}

func TestAdminService_ApproveJoinRequest(t *testing.T) {
	mockUserRepo := new(MockUserRepo)
	mockOrgRepo := new(MockOrganizationRepo)
	mockInviteRepo := new(MockInviteRepo)
	mockEmailSvc := new(MockEmailService)
	mockJoinRepo := new(MockJoinRequestRepo)
	mockLedgerRepo := new(MockLedgerRepo)

	svc := service.NewAdminService(mockJoinRepo, mockUserRepo, mockLedgerRepo, mockOrgRepo, mockInviteRepo, mockEmailSvc)
	ctx := context.Background()

	adminID := int32(1)
	orgID := int32(10)
	joinRequestID := int32(42)
	email := "applicant@test.com"
	name := "Applicant"

	// Caller must hold ADMIN/SUPER_ADMIN in orgID before anything else runs.
	mockUserRepo.On("GetUserOrg", ctx, adminID, orgID).Return(&domain.UserOrg{
		UserID: adminID, OrgID: orgID, Role: domain.UserOrgRoleAdmin,
	}, nil)

	// Mock fetching the join request by ID
	mockJoinRepo.On("GetByID", ctx, joinRequestID).Return(&domain.JoinRequest{
		ID:     joinRequestID,
		OrgID:  orgID,
		Name:   name,
		Email:  email,
		Status: domain.JoinRequestStatusPending,
	}, nil)

	mockOrgRepo.On("GetByID", ctx, orgID).Return(&domain.Organization{ID: orgID, Name: "Test Org"}, nil)
	// Mock admin user for CC
	mockUserRepo.On("GetByID", ctx, adminID).Return(&domain.User{ID: adminID, Name: "Admin", Email: "admin@test.com"}, nil)
	// Mock: user does not exist yet (new user path)
	mockUserRepo.On("GetByEmail", ctx, email).Return(nil, nil)

	mockInviteRepo.On("Create", ctx, mock.MatchedBy(func(inv *domain.Invitation) bool {
		return inv.OrgID == orgID && inv.Email == email && inv.CreatedBy == adminID &&
			inv.JoinRequestID != nil && *inv.JoinRequestID == joinRequestID
	})).Run(func(args mock.Arguments) {
		inv := args.Get(1).(*domain.Invitation)
		inv.ID = 1
		inv.InvitationCode = "ABC12345"
	}).Return(nil)
	mockEmailSvc.On("SendInvitation", ctx, email, name, "ABC12345", "Test Org", "admin@test.com").Return(nil)

	// Mock updating join request status to INVITED
	mockJoinRepo.On("Update", ctx, mock.MatchedBy(func(jr *domain.JoinRequest) bool {
		return jr.ID == joinRequestID && jr.Status == domain.JoinRequestStatusInvited
	})).Return(nil)

	invitationCode, err := svc.ApproveJoinRequest(ctx, adminID, orgID, joinRequestID)
	assert.NoError(t, err)
	assert.Equal(t, "ABC12345", invitationCode, "Should return the invitation code for new users")

	mockOrgRepo.AssertExpectations(t)
	mockJoinRepo.AssertExpectations(t)
	mockInviteRepo.AssertExpectations(t)
	mockEmailSvc.AssertExpectations(t)
}

// TestAdminService_RequiresAdminRole locks in the fix for the authorization gap recorded
// in specs/004-organizations-administration/spec.md (Known Discrepancy 1): every
// AdminService method must reject a caller who is not ADMIN/SUPER_ADMIN in the target org,
// before performing any of its side effects. Regression coverage for a real, previously
// unguarded vulnerability — do not remove without replacing.
func TestAdminService_RequiresAdminRole(t *testing.T) {
	const orgID = int32(1)
	const targetUserID = int32(2)
	const callerID = int32(999)

	t.Run("BlockUser rejects a plain MEMBER caller", func(t *testing.T) {
		mockUserRepo := new(MockUserRepo)
		svc := service.NewAdminService(nil, mockUserRepo, nil, nil, nil, nil)
		ctx := context.Background()

		mockUserRepo.On("GetUserOrg", ctx, callerID, orgID).
			Return(&domain.UserOrg{UserID: callerID, OrgID: orgID, Role: domain.UserOrgRoleMember}, nil).Once()

		err := svc.BlockUser(ctx, callerID, targetUserID, orgID, true, true, "abuse")
		assert.Error(t, err)
		// GetUserOrg for the *target* user must never be reached once authorization fails.
		mockUserRepo.AssertNotCalled(t, "GetUserOrg", ctx, targetUserID, orgID)
		mockUserRepo.AssertExpectations(t)
	})

	t.Run("BlockUser rejects a caller with no membership in the org at all", func(t *testing.T) {
		mockUserRepo := new(MockUserRepo)
		svc := service.NewAdminService(nil, mockUserRepo, nil, nil, nil, nil)
		ctx := context.Background()

		mockUserRepo.On("GetUserOrg", ctx, callerID, orgID).Return(nil, errors.New("sql: no rows")).Once()

		err := svc.BlockUser(ctx, callerID, targetUserID, orgID, true, true, "abuse")
		assert.Error(t, err)
		mockUserRepo.AssertExpectations(t)
	})

	t.Run("ListMembers rejects a plain MEMBER caller", func(t *testing.T) {
		mockUserRepo := new(MockUserRepo)
		svc := service.NewAdminService(nil, mockUserRepo, nil, nil, nil, nil)
		ctx := context.Background()

		mockUserRepo.On("GetUserOrg", ctx, callerID, orgID).
			Return(&domain.UserOrg{UserID: callerID, OrgID: orgID, Role: domain.UserOrgRoleMember}, nil).Once()

		_, _, err := svc.ListMembers(ctx, callerID, orgID)
		assert.Error(t, err)
		mockUserRepo.AssertNotCalled(t, "ListMembersByOrg", ctx, orgID)
		mockUserRepo.AssertExpectations(t)
	})

	t.Run("SearchUsers rejects a plain MEMBER caller", func(t *testing.T) {
		mockUserRepo := new(MockUserRepo)
		svc := service.NewAdminService(nil, mockUserRepo, nil, nil, nil, nil)
		ctx := context.Background()

		mockUserRepo.On("GetUserOrg", ctx, callerID, orgID).
			Return(&domain.UserOrg{UserID: callerID, OrgID: orgID, Role: domain.UserOrgRoleMember}, nil).Once()

		_, _, err := svc.SearchUsers(ctx, callerID, orgID, "query")
		assert.Error(t, err)
		mockUserRepo.AssertExpectations(t)
	})

	t.Run("ListJoinRequests rejects a plain MEMBER caller", func(t *testing.T) {
		mockUserRepo := new(MockUserRepo)
		mockJoinRepo := new(MockJoinRequestRepo)
		svc := service.NewAdminService(mockJoinRepo, mockUserRepo, nil, nil, nil, nil)
		ctx := context.Background()

		mockUserRepo.On("GetUserOrg", ctx, callerID, orgID).
			Return(&domain.UserOrg{UserID: callerID, OrgID: orgID, Role: domain.UserOrgRoleMember}, nil).Once()

		_, err := svc.ListJoinRequests(ctx, callerID, orgID)
		assert.Error(t, err)
		mockJoinRepo.AssertNotCalled(t, "ListByOrg", ctx, orgID)
		mockUserRepo.AssertExpectations(t)
	})

	t.Run("GetMemberProfile rejects a plain MEMBER caller", func(t *testing.T) {
		mockUserRepo := new(MockUserRepo)
		svc := service.NewAdminService(nil, mockUserRepo, nil, nil, nil, nil)
		ctx := context.Background()

		mockUserRepo.On("GetUserOrg", ctx, callerID, orgID).
			Return(&domain.UserOrg{UserID: callerID, OrgID: orgID, Role: domain.UserOrgRoleMember}, nil).Once()

		_, _, err := svc.GetMemberProfile(ctx, callerID, orgID, targetUserID)
		assert.Error(t, err)
		mockUserRepo.AssertNotCalled(t, "GetByID", ctx, targetUserID)
		mockUserRepo.AssertExpectations(t)
	})

	t.Run("ApproveJoinRequest rejects a plain MEMBER caller", func(t *testing.T) {
		mockUserRepo := new(MockUserRepo)
		mockJoinRepo := new(MockJoinRequestRepo)
		svc := service.NewAdminService(mockJoinRepo, mockUserRepo, nil, nil, nil, nil)
		ctx := context.Background()

		mockUserRepo.On("GetUserOrg", ctx, callerID, orgID).
			Return(&domain.UserOrg{UserID: callerID, OrgID: orgID, Role: domain.UserOrgRoleMember}, nil).Once()

		_, err := svc.ApproveJoinRequest(ctx, callerID, orgID, 42)
		assert.Error(t, err)
		mockJoinRepo.AssertNotCalled(t, "GetByID", ctx, int32(42))
		mockUserRepo.AssertExpectations(t)
	})

	t.Run("RejectJoinRequest rejects a plain MEMBER caller", func(t *testing.T) {
		mockUserRepo := new(MockUserRepo)
		mockJoinRepo := new(MockJoinRequestRepo)
		svc := service.NewAdminService(mockJoinRepo, mockUserRepo, nil, nil, nil, nil)
		ctx := context.Background()

		mockUserRepo.On("GetUserOrg", ctx, callerID, orgID).
			Return(&domain.UserOrg{UserID: callerID, OrgID: orgID, Role: domain.UserOrgRoleMember}, nil).Once()

		err := svc.RejectJoinRequest(ctx, callerID, orgID, 42, "no thanks")
		assert.Error(t, err)
		mockJoinRepo.AssertNotCalled(t, "GetByID", ctx, int32(42))
		mockUserRepo.AssertExpectations(t)
	})

	t.Run("SendInvitation rejects a plain MEMBER caller", func(t *testing.T) {
		mockUserRepo := new(MockUserRepo)
		mockOrgRepo := new(MockOrganizationRepo)
		svc := service.NewAdminService(nil, mockUserRepo, nil, mockOrgRepo, nil, nil)
		ctx := context.Background()

		mockUserRepo.On("GetUserOrg", ctx, callerID, orgID).
			Return(&domain.UserOrg{UserID: callerID, OrgID: orgID, Role: domain.UserOrgRoleMember}, nil).Once()

		_, err := svc.SendInvitation(ctx, callerID, orgID, "new@test.com", "New Person")
		assert.Error(t, err)
		mockOrgRepo.AssertNotCalled(t, "GetByID", ctx, orgID)
		mockUserRepo.AssertExpectations(t)
	})

	t.Run("ADMIN and SUPER_ADMIN callers are both accepted by verifyAdminRights", func(t *testing.T) {
		for _, role := range []domain.UserOrgRole{domain.UserOrgRoleAdmin, domain.UserOrgRoleSuperAdmin} {
			mockUserRepo := new(MockUserRepo)
			svc := service.NewAdminService(nil, mockUserRepo, nil, nil, nil, nil)
			ctx := context.Background()

			mockUserRepo.On("GetUserOrg", ctx, callerID, orgID).
				Return(&domain.UserOrg{UserID: callerID, OrgID: orgID, Role: role}, nil).Once()
			mockUserRepo.On("ListMembersByOrg", ctx, orgID).
				Return([]domain.User{}, []domain.UserOrg{}, nil).Once()

			_, _, err := svc.ListMembers(ctx, callerID, orgID)
			assert.NoError(t, err, "role %s should be accepted", role)
			mockUserRepo.AssertExpectations(t)
		}
	})
}

// TestAdminService_RejectJoinRequest_ExpiresLinkedInvitation covers FR-006
// (specs/003-organizations-administration): RejectRequestToJoin must expire any invitation
// already linked to the rejected join request. Prior to this test, the only
// RejectJoinRequest-adjacent test verified the FR-005 authorization gate, not this clause.
func TestAdminService_RejectJoinRequest_ExpiresLinkedInvitation(t *testing.T) {
	const adminID = int32(999)
	const orgID = int32(1)
	joinRequestID := int32(42)
	ctx := context.Background()

	t.Run("Expires a linked, not-yet-used invitation", func(t *testing.T) {
		mockUserRepo := new(MockUserRepo)
		mockJoinRepo := new(MockJoinRequestRepo)
		mockOrgRepo := new(MockOrganizationRepo)
		mockInviteRepo := new(MockInviteRepo)
		mockEmailSvc := new(MockEmailService)
		svc := service.NewAdminService(mockJoinRepo, mockUserRepo, nil, mockOrgRepo, mockInviteRepo, mockEmailSvc)

		mockUserRepo.On("GetUserOrg", ctx, adminID, orgID).Return(&domain.UserOrg{UserID: adminID, OrgID: orgID, Role: domain.UserOrgRoleAdmin}, nil)
		joinReq := &domain.JoinRequest{ID: joinRequestID, OrgID: orgID, Email: "applicant@test.com", Name: "Applicant", Status: domain.JoinRequestStatusPending}
		mockJoinRepo.On("GetByID", ctx, joinRequestID).Return(joinReq, nil)
		mockJoinRepo.On("Update", ctx, mock.MatchedBy(func(r *domain.JoinRequest) bool {
			return r.Status == domain.JoinRequestStatusRejected
		})).Return(nil)

		invite := &domain.Invitation{ID: 7, JoinRequestID: &joinRequestID, ExpiresOn: "2027-01-01"}
		mockInviteRepo.On("GetByJoinRequestID", ctx, joinRequestID).Return(invite, nil)
		mockInviteRepo.On("ExpireInvitation", ctx, int32(7), mock.MatchedBy(func(expiresOn string) bool {
			return expiresOn < time.Now().Format("2006-01-02")
		})).Return(nil)

		mockOrgRepo.On("GetByID", ctx, orgID).Return(&domain.Organization{ID: orgID, Name: "Org"}, nil)
		mockEmailSvc.On("SendAccountStatusNotification", ctx, "applicant@test.com", "Applicant", "Org", "REJECTED", "not a fit").Return(nil)

		err := svc.RejectJoinRequest(ctx, adminID, orgID, joinRequestID, "not a fit")
		assert.NoError(t, err)
		mockInviteRepo.AssertCalled(t, "ExpireInvitation", ctx, int32(7), mock.Anything)
	})

	t.Run("Does not touch an invitation that has already been used", func(t *testing.T) {
		mockUserRepo := new(MockUserRepo)
		mockJoinRepo := new(MockJoinRequestRepo)
		mockOrgRepo := new(MockOrganizationRepo)
		mockInviteRepo := new(MockInviteRepo)
		mockEmailSvc := new(MockEmailService)
		svc := service.NewAdminService(mockJoinRepo, mockUserRepo, nil, mockOrgRepo, mockInviteRepo, mockEmailSvc)

		mockUserRepo.On("GetUserOrg", ctx, adminID, orgID).Return(&domain.UserOrg{UserID: adminID, OrgID: orgID, Role: domain.UserOrgRoleAdmin}, nil)
		joinReq := &domain.JoinRequest{ID: joinRequestID, OrgID: orgID, Email: "applicant@test.com", Name: "Applicant", Status: domain.JoinRequestStatusPending}
		mockJoinRepo.On("GetByID", ctx, joinRequestID).Return(joinReq, nil)
		mockJoinRepo.On("Update", ctx, mock.Anything).Return(nil)

		usedOn := "2026-01-01"
		usedByUserID := int32(55)
		invite := &domain.Invitation{ID: 8, JoinRequestID: &joinRequestID, ExpiresOn: "2027-01-01", UsedOn: &usedOn, UsedByUserID: &usedByUserID}
		mockInviteRepo.On("GetByJoinRequestID", ctx, joinRequestID).Return(invite, nil)

		mockOrgRepo.On("GetByID", ctx, orgID).Return(&domain.Organization{ID: orgID, Name: "Org"}, nil)
		mockEmailSvc.On("SendAccountStatusNotification", ctx, "applicant@test.com", "Applicant", "Org", "REJECTED", "not a fit").Return(nil)

		err := svc.RejectJoinRequest(ctx, adminID, orgID, joinRequestID, "not a fit")
		assert.NoError(t, err)
		mockInviteRepo.AssertNotCalled(t, "ExpireInvitation", mock.Anything, mock.Anything, mock.Anything)
	})
}
