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
	// 1. Get Organization
	org, err := s.orgRepo.GetByID(ctx, orgID)
	if err != nil {
		return fmt.Errorf("failed to get organization: %w", err)
	}

	// 2. Check if user already exists
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err == nil && user != nil {
		// User exists, add to org
		userOrg := &domain.UserOrg{
			UserID:       user.ID,
			OrgID:        orgID,
			JoinedOn:     time.Now(),
			Status:       domain.UserOrgStatusActive,
			Role:         domain.UserOrgRoleMember,
			BalanceCents: 0,
		}
		if err := s.userRepo.AddUserToOrg(ctx, userOrg); err != nil {
			return fmt.Errorf("failed to add existing user to org: %w", err)
		}
		
		// Notify user
		_ = s.emailSvc.SendAccountStatusNotification(ctx, email, name, org.Name, "APPROVED", "Your join request has been approved.")

	} else {
		// User does not exist, Invite
		inv := &domain.Invitation{
			OrgID:     orgID,
			Email:     email,
			CreatedBy: adminID,
			ExpiresOn: time.Now().Add(7 * 24 * time.Hour),
		}
		if err := s.inviteRepo.Create(ctx, inv); err != nil {
			return fmt.Errorf("failed to create invitation: %w", err)
		}

		// Get admin email for CC
		admin, err := s.userRepo.GetByID(ctx, adminID)
		if err != nil {
			return fmt.Errorf("failed to get admin user: %w", err)
		}

		if err := s.emailSvc.SendInvitation(ctx, email, name, inv.InvitationCode, org.Name, admin.Email); err != nil {
			return fmt.Errorf("failed to send invitation email: %w", err)
		}
	}

	// 3. Update Join Request Status (if we can find it by email/org or passed ID? Interface passes email/name/orgID)
	// The interface `ApproveJoinRequest` doesn't take RequestID. 
	// But `JoinRequestRepository` likely has `GetByOrg` or we might need to search pending requests.
	// `ListByOrg` exists.
	// Optimization: Ideally `reqID` should be passed. But sticking to interface for now.
	// We will search for pending requests for this email/org and update them.
	
	reqs, err := s.reqRepo.ListByOrg(ctx, orgID)
	if err == nil {
		for _, req := range reqs {
			if req.Email == email && req.Status == domain.JoinRequestStatusPending {
				req.Status = domain.JoinRequestStatusApproved
				_ = s.reqRepo.Update(ctx, &req)
			}
		}
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
