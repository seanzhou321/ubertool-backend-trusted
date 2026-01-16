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
	svc := service.NewOrganizationService(mockRepo)
	ctx := context.Background()

	org := &domain.Organization{ID: 1, Name: "Updated"}
	mockRepo.On("Update", ctx, org).Return(nil)

	err := svc.UpdateOrganization(ctx, org)
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}
