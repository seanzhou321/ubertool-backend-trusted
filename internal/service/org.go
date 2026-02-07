package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/logger"
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
	orgs, err := s.orgRepo.Search(ctx, name, metro)
	if err != nil {
		return nil, err
	}

	// Populate admins for each organization
	for i := range orgs {
		users, userOrgs, err := s.userRepo.ListMembersByOrg(ctx, orgs[i].ID)
		if err != nil {
			logger.Warn("Failed to fetch members for org", "orgID", orgs[i].ID, "error", err)
			continue
		}

		// Filter for SUPER_ADMIN and ADMIN roles
		var admins []domain.User
		for j, userOrg := range userOrgs {
			if userOrg.Role == domain.UserOrgRoleSuperAdmin || userOrg.Role == domain.UserOrgRoleAdmin {
				admins = append(admins, users[j])
			}
		}
		orgs[i].Admins = admins
	}

	return orgs, nil
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

	// 2. Validate the invitation code using (invitation_code, email) tuple
	inv, err := s.inviteRepo.GetByInvitationCodeAndEmail(ctx, inviteCode, user.Email)
	if err != nil {
		return nil, nil, errors.New("invitation code is invalid or expired")
	}
	if inv.UsedOn != nil {
		return nil, nil, errors.New("invitation already used")
	}
	if inv.ExpiresOn.Before(time.Now()) {
		return nil, nil, errors.New("invitation code is invalid or expired")
	}

	// 3. Retrieve the organization from the invitation
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
	logger.Debug("Creating notifications for admins about new member", "userID", userID, "orgID", inv.OrgID)
	users, userOrgsAll, err := s.userRepo.ListMembersByOrg(ctx, inv.OrgID)
	if err == nil {
		adminCount := 0
		notificationsSent := 0
		notificationsFailed := 0

		for i, u := range users {
			uo := userOrgsAll[i]
			if uo.Role == domain.UserOrgRoleAdmin || uo.Role == domain.UserOrgRoleSuperAdmin {
				adminCount++
				logger.Debug("Creating member joined notification for admin", "adminID", u.ID, "adminEmail", u.Email, "role", uo.Role)

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

				notifErr := s.noteRepo.Create(ctx, notif)
				if notifErr != nil {
					logger.Error("CRITICAL: Failed to create member joined notification", "adminID", u.ID, "error", notifErr)
					notificationsFailed++
				} else {
					logger.Info("Member joined notification created", "adminID", u.ID, "notificationID", notif.ID)
					notificationsSent++
				}
			}
		}

		logger.Info("Admin notifications completed", "adminsFound", adminCount, "notificationsSent", notificationsSent, "notificationsFailed", notificationsFailed)
	} else {
		logger.Warn("Failed to fetch admins for member joined notifications", "error", err)
	}

	return org, user, nil
}
