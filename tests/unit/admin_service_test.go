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
		uo := &domain.UserOrg{UserID: 1, OrgID: 1, Status: domain.UserOrgStatusActive}
		mockUserRepo.On("GetUserOrg", ctx, int32(1), int32(1)).Return(uo, nil).Once()
		mockUserRepo.On("GetByID", ctx, int32(1)).Return(&domain.User{ID: 1, Name: "User 1", Email: "u1@test.com"}, nil).Once()
		mockOrgRepo.On("GetByID", ctx, int32(1)).Return(&domain.Organization{ID: 1, Name: "Test Org"}, nil).Once()
		mockUserRepo.On("UpdateUserOrg", ctx, mock.MatchedBy(func(u *domain.UserOrg) bool {
			return u.Status == domain.UserOrgStatusBlock && u.BlockReason == "violation" && u.BlockedDate != nil
		})).Return(nil).Once()
		mockEmailSvc.On("SendAccountStatusNotification", ctx, "u1@test.com", "User 1", "Test Org", "BLOCK", "violation").Return(nil).Once()

		err := svc.BlockUser(ctx, 999, 1, 1, true, "violation")
		assert.NoError(t, err)
	})

	t.Run("Unblock", func(t *testing.T) {
		now := time.Now()
		uo := &domain.UserOrg{UserID: 1, OrgID: 1, Status: domain.UserOrgStatusBlock, BlockReason: "violation", BlockedDate: &now}
		mockUserRepo.On("GetUserOrg", ctx, int32(1), int32(1)).Return(uo, nil).Once()
		mockUserRepo.On("GetByID", ctx, int32(1)).Return(&domain.User{ID: 1, Name: "User 1", Email: "u1@test.com"}, nil).Once()
		mockOrgRepo.On("GetByID", ctx, int32(1)).Return(&domain.Organization{ID: 1, Name: "Test Org"}, nil).Once()
		mockUserRepo.On("UpdateUserOrg", ctx, mock.MatchedBy(func(u *domain.UserOrg) bool {
			return u.Status == domain.UserOrgStatusActive && u.BlockReason == "" && u.BlockedDate == nil
		})).Return(nil).Once()
		mockEmailSvc.On("SendAccountStatusNotification", ctx, "u1@test.com", "User 1", "Test Org", "ACTIVE", "").Return(nil).Once()

		err := svc.BlockUser(ctx, 999, 1, 1, false, "")
		assert.NoError(t, err)
	})

	mockUserRepo.AssertExpectations(t)
	mockEmailSvc.AssertExpectations(t)
}

func TestAdminService_ListMembers(t *testing.T) {
	mockUserRepo := new(MockUserRepo)
	svc := service.NewAdminService(nil, mockUserRepo, nil, nil, nil, nil)
	ctx := context.Background()

	users := []domain.User{{ID: 1, Name: "User 1"}}
	uos := []domain.UserOrg{{UserID: 1, OrgID: 1}}
	mockUserRepo.On("ListMembersByOrg", ctx, int32(1)).Return(users, uos, nil)

	rUsers, rUos, err := svc.ListMembers(ctx, 1)
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
	// Add other missing repos: reqRepo, ledgerRepo
	mockJoinRepo := new(MockJoinRequestRepo)
	mockLedgerRepo := new(MockLedgerRepo)
	
	svc := service.NewAdminService(mockJoinRepo, mockUserRepo, mockLedgerRepo, mockOrgRepo, mockInviteRepo, mockEmailSvc)
	ctx := context.Background()

	adminID := int32(1)
	orgID := int32(10)
	email := "applicant@test.com"
	name := "Applicant"

	mockOrgRepo.On("GetByID", ctx, orgID).Return(&domain.Organization{ID: orgID, Name: "Test Org"}, nil)
	// Mock admin user for CC
	mockUserRepo.On("GetByID", ctx, adminID).Return(&domain.User{ID: adminID, Name: "Admin", Email: "admin@test.com"}, nil)
	// Mock Check if user exists (false for this test case)
	mockUserRepo.On("GetByEmail", ctx, email).Return(nil, nil)
	
	mockInviteRepo.On("Create", ctx, mock.MatchedBy(func(inv *domain.Invitation) bool {
		return inv.OrgID == orgID && inv.Email == email && inv.CreatedBy == adminID
	})).Run(func(args mock.Arguments) {
		inv := args.Get(1).(*domain.Invitation)
		inv.Token = "uuid-token"
	}).Return(nil)
	mockEmailSvc.On("SendInvitation", ctx, email, name, "uuid-token", "Test Org", "admin@test.com").Return(nil)
	
	// Mock ListJoinRequests lookup update
	mockJoinRepo.On("ListByOrg", ctx, orgID).Return([]domain.JoinRequest{}, nil)

	err := svc.ApproveJoinRequest(ctx, adminID, orgID, email, name)
	assert.NoError(t, err)

	mockOrgRepo.AssertExpectations(t)
	mockInviteRepo.AssertExpectations(t)
	mockEmailSvc.AssertExpectations(t)
}
