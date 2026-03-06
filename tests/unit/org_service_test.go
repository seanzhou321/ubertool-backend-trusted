package unit

import (
	"context"
	"testing"

	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/service"

	"github.com/stretchr/testify/assert"
)

func TestOrganizationService_UpdateOrganization(t *testing.T) {
	mockRepo := new(MockOrganizationRepo)
	mockUserRepo := new(MockUserRepo)
	mockInviteRepo := new(MockInvitationRepo)
	mockNoteRepo := new(MockNotificationRepo)
	svc := service.NewOrganizationService(mockRepo, mockUserRepo, mockInviteRepo, mockNoteRepo, nil, nil)
	ctx := context.Background()

	const callerID = int32(1)
	const orgID = int32(1)

	// Caller is SUPER_ADMIN
	callerUserOrg := &domain.UserOrg{UserID: callerID, OrgID: orgID, Role: domain.UserOrgRoleSuperAdmin, Status: domain.UserOrgStatusActive}
	mockUserRepo.On("GetUserOrg", ctx, callerID, orgID).Return(callerUserOrg, nil)

	// Current org returned for change detection
	currentOrg := &domain.Organization{ID: orgID, Name: "Old Name"}
	mockRepo.On("GetByID", ctx, orgID).Return(currentOrg, nil)

	org := &domain.Organization{ID: orgID, Name: "Updated"}
	mockRepo.On("Update", ctx, org).Return(nil)

	err := svc.UpdateOrganization(ctx, callerID, org)
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
	mockUserRepo.AssertExpectations(t)
}
