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

func newOrgServiceWithBroadcastMocksForTest() (service.OrganizationService, *MockOrganizationRepo, *MockUserRepo, *MockInviteRepo, *MockNotificationRepo, *MockEmailService, *MockPushNotificationService) {
	mockRepo := new(MockOrganizationRepo)
	mockUserRepo := new(MockUserRepo)
	mockInviteRepo := new(MockInviteRepo)
	mockNoteRepo := new(MockNotificationRepo)
	mockEmailSvc := new(MockEmailService)
	mockPushSvc := new(MockPushNotificationService)
	svc := service.NewOrganizationService(mockRepo, mockUserRepo, mockInviteRepo, mockNoteRepo, mockEmailSvc, mockPushSvc)
	return svc, mockRepo, mockUserRepo, mockInviteRepo, mockNoteRepo, mockEmailSvc, mockPushSvc
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

// TestOrganizationService_UpdateOrganization_ThresholdBroadcast covers FR-003: when
// UpdateOrganization actually changes a price threshold, every active member must receive an
// in-app notification, an email, and be included in a single FCM multicast. The broadcast runs
// in a background goroutine, so these tests synchronize on the (last-fired) multicast call via
// a channel rather than asserting immediately after UpdateOrganization returns.
func TestOrganizationService_UpdateOrganization_ThresholdBroadcast(t *testing.T) {
	const callerID = int32(1)
	const orgID = int32(1)
	ctx := context.Background()

	t.Run("Broadcasts to active members when the settlement threshold changes", func(t *testing.T) {
		svc, mockRepo, mockUserRepo, _, mockNoteRepo, mockEmailSvc, mockPushSvc := newOrgServiceWithBroadcastMocksForTest()

		callerUserOrg := &domain.UserOrg{UserID: callerID, OrgID: orgID, Role: domain.UserOrgRoleSuperAdmin, Status: domain.UserOrgStatusActive}
		mockUserRepo.On("GetUserOrg", ctx, callerID, orgID).Return(callerUserOrg, nil)

		currentOrg := &domain.Organization{ID: orgID, Name: "Old Name", SettlementThresholdCents: 1000}
		mockRepo.On("GetByID", ctx, orgID).Return(currentOrg, nil)

		org := &domain.Organization{ID: orgID, Name: "Old Name", SettlementThresholdCents: 2000}
		mockRepo.On("Update", ctx, org).Return(nil)

		members := []domain.User{
			{ID: 10, Email: "member10@test.com", Name: "Member Ten"},
			{ID: 11, Email: "member11@test.com", Name: "Member Eleven"},
			{ID: 12, Email: "member12@test.com", Name: "Blocked Member"},
		}
		userOrgs := []domain.UserOrg{
			{UserID: 10, OrgID: orgID, Status: domain.UserOrgStatusActive},
			{UserID: 11, OrgID: orgID, Status: domain.UserOrgStatusActive},
			{UserID: 12, OrgID: orgID, Status: domain.UserOrgStatusBlock},
		}
		mockUserRepo.On("ListMembersByOrg", mock.Anything, orgID).Return(members, userOrgs, nil)

		mockNoteRepo.On("DispatchSilent", mock.Anything, mock.Anything).Return(nil)
		mockEmailSvc.On("SendAdminNotification", mock.Anything, "member10@test.com", mock.Anything, mock.Anything).Return(nil)
		mockEmailSvc.On("SendAdminNotification", mock.Anything, "member11@test.com", mock.Anything, mock.Anything).Return(nil)

		done := make(chan []int32, 1)
		mockPushSvc.On("SendMulticastToUsers", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) {
				done <- args.Get(1).([]int32)
			}).Return(nil)

		err := svc.UpdateOrganization(ctx, callerID, org)
		require.NoError(t, err)

		select {
		case pushedTo := <-done:
			assert.ElementsMatch(t, []int32{10, 11}, pushedTo, "blocked members must be excluded from the multicast")
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for the async threshold-change broadcast to fire SendMulticastToUsers")
		}

		mockNoteRepo.AssertNumberOfCalls(t, "DispatchSilent", 2)
		mockEmailSvc.AssertCalled(t, "SendAdminNotification", mock.Anything, "member10@test.com", mock.Anything, mock.Anything)
		mockEmailSvc.AssertCalled(t, "SendAdminNotification", mock.Anything, "member11@test.com", mock.Anything, mock.Anything)
	})

	t.Run("Does not broadcast when no threshold field actually changes", func(t *testing.T) {
		svc, mockRepo, mockUserRepo, _, mockNoteRepo, mockEmailSvc, mockPushSvc := newOrgServiceWithBroadcastMocksForTest()

		callerUserOrg := &domain.UserOrg{UserID: callerID, OrgID: orgID, Role: domain.UserOrgRoleSuperAdmin, Status: domain.UserOrgStatusActive}
		mockUserRepo.On("GetUserOrg", ctx, callerID, orgID).Return(callerUserOrg, nil)

		currentOrg := &domain.Organization{ID: orgID, Name: "Old Name", SettlementThresholdCents: 1000, MaxBillsplitRentalCostCents: 50000}
		mockRepo.On("GetByID", ctx, orgID).Return(currentOrg, nil)

		// Renaming only — thresholds submitted as 0, meaning "no change".
		org := &domain.Organization{ID: orgID, Name: "Renamed"}
		mockRepo.On("Update", ctx, mock.Anything).Return(nil)

		err := svc.UpdateOrganization(ctx, callerID, org)
		require.NoError(t, err)

		// Give the (non-existent) goroutine a moment to have fired, if it incorrectly did.
		time.Sleep(150 * time.Millisecond)
		mockUserRepo.AssertNotCalled(t, "ListMembersByOrg", mock.Anything, mock.Anything)
		mockNoteRepo.AssertNotCalled(t, "DispatchSilent", mock.Anything, mock.Anything)
		mockEmailSvc.AssertNotCalled(t, "SendAdminNotification", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		mockPushSvc.AssertNotCalled(t, "SendMulticastToUsers", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})
}

// TestOrganizationService_JoinOrganizationWithInvite covers FR-004: JoinOrganizationWithInvite
// must validate invite-email ownership (via the caller's own email, not a request-supplied one),
// reject if the caller is already a member, and add the caller as MEMBER on success.
func TestOrganizationService_JoinOrganizationWithInvite(t *testing.T) {
	const userID = int32(20)
	const orgID = int32(3)
	const email = "joiner@test.com"
	const token = "JOIN-TOK-001"
	ctx := context.Background()

	t.Run("Success adds the caller as MEMBER", func(t *testing.T) {
		svc, mockRepo, mockUserRepo, mockInviteRepo, _, _, _ := newOrgServiceWithBroadcastMocksForTest()

		mockUserRepo.On("GetByID", ctx, userID).Return(&domain.User{ID: userID, Email: email}, nil)
		invite := &domain.Invitation{
			InvitationCode: token, Email: email, OrgID: orgID,
			ExpiresOn: time.Now().Add(48 * time.Hour).Format("2006-01-02"),
		}
		mockInviteRepo.On("GetByInvitationCodeAndEmail", ctx, token, email).Return(invite, nil)
		mockRepo.On("GetByID", ctx, orgID).Return(&domain.Organization{ID: orgID, Name: "Org"}, nil)
		mockUserRepo.On("ListUserOrgs", ctx, userID).Return([]domain.UserOrg{}, nil)
		mockUserRepo.On("AddUserToOrg", ctx, mock.MatchedBy(func(uo *domain.UserOrg) bool {
			return uo.UserID == userID && uo.OrgID == orgID && uo.Role == domain.UserOrgRoleMember
		})).Return(nil)
		mockInviteRepo.On("Update", ctx, mock.MatchedBy(func(inv *domain.Invitation) bool {
			return inv.UsedOn != nil
		})).Return(nil)
		mockUserRepo.On("ListMembersByOrg", ctx, orgID).Return([]domain.User{}, []domain.UserOrg{}, nil)

		org, user, err := svc.JoinOrganizationWithInvite(ctx, userID, token)
		require.NoError(t, err)
		assert.Equal(t, orgID, org.ID)
		assert.Equal(t, userID, user.ID)
		mockUserRepo.AssertCalled(t, "AddUserToOrg", ctx, mock.Anything)
	})

	t.Run("Rejects when the caller is already a member", func(t *testing.T) {
		svc, mockRepo, mockUserRepo, mockInviteRepo, _, _, _ := newOrgServiceWithBroadcastMocksForTest()

		mockUserRepo.On("GetByID", ctx, userID).Return(&domain.User{ID: userID, Email: email}, nil)
		invite := &domain.Invitation{
			InvitationCode: token, Email: email, OrgID: orgID,
			ExpiresOn: time.Now().Add(48 * time.Hour).Format("2006-01-02"),
		}
		mockInviteRepo.On("GetByInvitationCodeAndEmail", ctx, token, email).Return(invite, nil)
		mockRepo.On("GetByID", ctx, orgID).Return(&domain.Organization{ID: orgID, Name: "Org"}, nil)
		mockUserRepo.On("ListUserOrgs", ctx, userID).Return([]domain.UserOrg{
			{UserID: userID, OrgID: orgID, Role: domain.UserOrgRoleMember, Status: domain.UserOrgStatusActive},
		}, nil)

		_, _, err := svc.JoinOrganizationWithInvite(ctx, userID, token)
		require.Error(t, err)
		mockUserRepo.AssertNotCalled(t, "AddUserToOrg", mock.Anything, mock.Anything)
	})

	t.Run("Rejects an invitation issued to a different email", func(t *testing.T) {
		svc, _, mockUserRepo, mockInviteRepo, _, _, _ := newOrgServiceWithBroadcastMocksForTest()

		mockUserRepo.On("GetByID", ctx, userID).Return(&domain.User{ID: userID, Email: email}, nil)
		// The invitation lookup is keyed on (code, caller's own email) — an invite issued to a
		// different address will not be found under the caller's email, which is the mechanism
		// by which invite-email ownership is enforced.
		mockInviteRepo.On("GetByInvitationCodeAndEmail", ctx, token, email).Return(nil, assert.AnError)

		_, _, err := svc.JoinOrganizationWithInvite(ctx, userID, token)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid or expired")
		mockUserRepo.AssertNotCalled(t, "AddUserToOrg", mock.Anything, mock.Anything)
	})

	t.Run("Rejects an already-used invitation", func(t *testing.T) {
		svc, _, mockUserRepo, mockInviteRepo, _, _, _ := newOrgServiceWithBroadcastMocksForTest()

		mockUserRepo.On("GetByID", ctx, userID).Return(&domain.User{ID: userID, Email: email}, nil)
		usedOn := time.Now().Format("2006-01-02")
		invite := &domain.Invitation{
			InvitationCode: token, Email: email, OrgID: orgID,
			ExpiresOn: time.Now().Add(48 * time.Hour).Format("2006-01-02"),
			UsedOn:    &usedOn,
		}
		mockInviteRepo.On("GetByInvitationCodeAndEmail", ctx, token, email).Return(invite, nil)

		_, _, err := svc.JoinOrganizationWithInvite(ctx, userID, token)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already used")
		mockUserRepo.AssertNotCalled(t, "AddUserToOrg", mock.Anything, mock.Anything)
	})
}
