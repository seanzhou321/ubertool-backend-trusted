package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/repository"
)

type organizationService struct {
	orgRepo    repository.OrganizationRepository
	userRepo   repository.UserRepository
	inviteRepo repository.InvitationRepository
	noteRepo   repository.NotificationRepository
}

func NewOrganizationService(orgRepo repository.OrganizationRepository, userRepo repository.UserRepository, inviteRepo repository.InvitationRepository, noteRepo repository.NotificationRepository) OrganizationService {
	return &organizationService{
		orgRepo:    orgRepo,
		userRepo:   userRepo,
		inviteRepo: inviteRepo,
		noteRepo:   noteRepo,
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

func (s *organizationService) JoinOrganizationWithInvite(ctx context.Context, userID int32, inviteCode string) (*domain.Organization, *domain.User, error) {
	// 1. Get authenticated user
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, nil, err
	}

	// 2. Validate the invitation code
	inv, err := s.inviteRepo.GetByTokenAndEmail(ctx, inviteCode, user.Email)
	if err != nil {
		return nil, nil, errors.New("invitation code is invalid or expired")
	}
	if inv.UsedOn != nil {
		return nil, nil, errors.New("invitation already used")
	}
	if inv.ExpiresOn.Before(time.Now()) {
		return nil, nil, errors.New("invitation code is invalid or expired")
	}

	// 3. Retrieve the organization_id from the invitation
	org, err := s.orgRepo.GetByID(ctx, inv.OrgID)
	if err != nil {
		return nil, nil, err
	}

	// 5. Check if user is already a member
	userOrgs, err := s.userRepo.ListUserOrgs(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	for _, uo := range userOrgs {
		if uo.OrgID == inv.OrgID {
			return nil, nil, errors.New("You are already a member of this organization")
		}
	}

	// 7. Update the invitations record
	now := time.Now()
	inv.UsedOn = &now
	inv.UsedByUserID = &userID
	if err := s.inviteRepo.Update(ctx, inv); err != nil {
		return nil, nil, err
	}

	// 8. Create a record in users_orgs table
	userOrg := &domain.UserOrg{
		UserID:       userID,
		OrgID:        inv.OrgID,
		Role:         domain.UserOrgRoleMember,
		Status:       domain.UserOrgStatusActive,
		JoinedOn:     time.Now(),
		BalanceCents: 0,
	}
	if err := s.userRepo.AddUserToOrg(ctx, userOrg); err != nil {
		return nil, nil, err
	}

	// 10. Create notification to org admins
	users, userOrgsAll, err := s.userRepo.ListMembersByOrg(ctx, inv.OrgID)
	if err == nil {
		for i, u := range users {
			uo := userOrgsAll[i]
			if uo.Role == domain.UserOrgRoleAdmin || uo.Role == domain.UserOrgRoleSuperAdmin {
				notif := &domain.Notification{
					UserID:  u.ID,
					OrgID:   inv.OrgID,
					Title:   "New Member Joined",
					Message: fmt.Sprintf("%s joined %s", user.Name, org.Name),
					IsRead:  false,
					Attributes: map[string]string{
						"type":      "MEMBER_JOINED",
						"reference": fmt.Sprintf("user:%d", userID),
					},
					CreatedOn: time.Now(),
				}
				_ = s.noteRepo.Create(ctx, notif)
			}
		}
	}

	return org, user, nil
}
