package service

import (
	"context"
	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/repository"
)

type organizationService struct {
	orgRepo repository.OrganizationRepository
}

func NewOrganizationService(orgRepo repository.OrganizationRepository) OrganizationService {
	return &organizationService{orgRepo: orgRepo}
}

func (s *organizationService) ListOrganizations(ctx context.Context) ([]domain.Organization, error) {
	return s.orgRepo.List(ctx)
}

func (s *organizationService) GetOrganization(ctx context.Context, id int32) (*domain.Organization, error) {
	return s.orgRepo.GetByID(ctx, id)
}

func (s *organizationService) SearchOrganizations(ctx context.Context, name, metro string) ([]domain.Organization, error) {
	return s.orgRepo.Search(ctx, name, metro)
}

func (s *organizationService) UpdateOrganization(ctx context.Context, org *domain.Organization) error {
	return s.orgRepo.Update(ctx, org)
}
