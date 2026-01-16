package unit

import (
	"context"
	"testing"

	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestAdminService_BlockUser(t *testing.T) {
	mockUserRepo := new(MockUserRepo)
	mockJoinRepo := new(MockJoinRequestRepo)
	mockLedgerRepo := new(MockLedgerRepo)
	svc := service.NewAdminService(mockJoinRepo, mockUserRepo, mockLedgerRepo)
	ctx := context.Background()

	uo := &domain.UserOrg{UserID: 1, OrgID: 1, Status: domain.UserOrgStatusActive}
	mockUserRepo.On("GetUserOrg", ctx, int32(1), int32(1)).Return(uo, nil)
	mockUserRepo.On("UpdateUserOrg", ctx, mock.MatchedBy(func(u *domain.UserOrg) bool {
		return u.Status == domain.UserOrgStatusBlock && u.BlockReason == "violation"
	})).Return(nil)

	err := svc.BlockUser(ctx, 999, 1, 1, "violation")
	assert.NoError(t, err)
	mockUserRepo.AssertExpectations(t)
}

func TestAdminService_ListMembers(t *testing.T) {
	mockUserRepo := new(MockUserRepo)
	svc := service.NewAdminService(nil, mockUserRepo, nil)
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
