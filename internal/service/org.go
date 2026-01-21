package service

import (
	"context"
	"time"

	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/repository"
)

type organizationService struct {
	orgRepo  repository.OrganizationRepository
	userRepo repository.UserRepository
}

func NewOrganizationService(orgRepo repository.OrganizationRepository, userRepo repository.UserRepository) OrganizationService {
	return &organizationService{
		orgRepo:  orgRepo,
		userRepo: userRepo,
	}
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

func (s *organizationService) CreateOrganization(ctx context.Context, userID int32, org *domain.Organization) error {
	if err := s.orgRepo.Create(ctx, org); err != nil {
		return err
	}

	// Add creator as SUPER_ADMIN
	userOrg := &domain.UserOrg{
		UserID:       userID,
		OrgID:        org.ID,
		JoinedOn:     time.Now(),
		BalanceCents: 0,
		Status:       domain.UserOrgStatusActive,
		Role:         domain.UserOrgRoleSuperAdmin,
	}

	return s.userRepo.AddUserToOrg(ctx, userOrg)
}

func (s *organizationService) UpdateOrganization(ctx context.Context, org *domain.Organization) error {
	return s.orgRepo.Update(ctx, org)
}

func (s *organizationService) ListMyOrganizations(ctx context.Context, userID int32) ([]domain.Organization, []domain.UserOrg, error) {
	userOrgs, err := s.userRepo.ListUserOrgs(ctx, userID)
	if err != nil {
		return nil, nil, err
	}

	var orgs []domain.Organization
	for _, uo := range userOrgs {
		org, err := s.orgRepo.GetByID(ctx, uo.OrgID)
		if err != nil {
			continue
		}
		if org != nil {
			orgs = append(orgs, *org)
		}
	}
	return orgs, userOrgs, nil
}
