package service

import (
	"context"
	"fmt"
	"time"

	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/repository"
)

type adminService struct {
	reqRepo    repository.JoinRequestRepository
	userRepo   repository.UserRepository
	ledgerRepo repository.LedgerRepository
	orgRepo    repository.OrganizationRepository
	inviteRepo repository.InvitationRepository
	emailSvc   EmailService
}

func NewAdminService(
	reqRepo repository.JoinRequestRepository,
	userRepo repository.UserRepository,
	ledgerRepo repository.LedgerRepository,
	orgRepo repository.OrganizationRepository,
	inviteRepo repository.InvitationRepository,
	emailSvc EmailService,
) AdminService {
	return &adminService{
		reqRepo:    reqRepo,
		userRepo:   userRepo,
		ledgerRepo: ledgerRepo,
		orgRepo:    orgRepo,
		inviteRepo: inviteRepo,
		emailSvc:   emailSvc,
	}
}

func (s *adminService) ApproveJoinRequest(ctx context.Context, adminID, orgID int32, email, name string) error {
	// 1. Get Organization for the email context
	org, err := s.orgRepo.GetByID(ctx, orgID)
	if err != nil {
		return fmt.Errorf("failed to get organization: %w", err)
	}

	// 2. Generate and Create Invitation
	inv := &domain.Invitation{
		OrgID:     orgID,
		Email:     email,
		CreatedBy: adminID,
		ExpiresOn: time.Now().Add(7 * 24 * time.Hour), // 7 days expiry
	}
	if err := s.inviteRepo.Create(ctx, inv); err != nil {
		return fmt.Errorf("failed to create invitation: %w", err)
	}

	// 3. Send Email
	if err := s.emailSvc.SendInvitation(ctx, email, name, inv.Token, org.Name); err != nil {
		// We might want to log this but not necessarily fail the whole transaction?
		// But for now, let's return error if email fails if that's critical.
		return fmt.Errorf("failed to send invitation email: %w", err)
	}

	return nil
}

func (s *adminService) BlockUser(ctx context.Context, adminID, userID, orgID int32, isBlock bool, reason string) error {
	uo, err := s.userRepo.GetUserOrg(ctx, userID, orgID)
	if err != nil {
		return err
	}

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	org, err := s.orgRepo.GetByID(ctx, orgID)
	if err != nil {
		return err
	}

	if isBlock {
		uo.Status = domain.UserOrgStatusBlock
		uo.BlockReason = reason
		now := time.Now()
		uo.BlockedDate = &now
	} else {
		uo.Status = domain.UserOrgStatusActive
		uo.BlockReason = ""
		uo.BlockedDate = nil
	}

	if err := s.userRepo.UpdateUserOrg(ctx, uo); err != nil {
		return err
	}

	// Notify user
	statusStr := string(uo.Status)
	_ = s.emailSvc.SendAccountStatusNotification(ctx, user.Email, user.Name, org.Name, statusStr, reason)

	return nil
}

func (s *adminService) ListMembers(ctx context.Context, orgID int32) ([]domain.User, []domain.UserOrg, error) {
	return s.userRepo.ListMembersByOrg(ctx, orgID)
}

func (s *adminService) SearchUsers(ctx context.Context, orgID int32, query string) ([]domain.User, []domain.UserOrg, error) {
	return s.userRepo.SearchMembersByOrg(ctx, orgID, query)
}

func (s *adminService) ListJoinRequests(ctx context.Context, orgID int32) ([]domain.JoinRequest, error) {
	return s.reqRepo.ListByOrg(ctx, orgID)
}
