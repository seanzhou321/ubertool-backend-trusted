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

func (s *adminService) ApproveJoinRequest(ctx context.Context, adminID, orgID, joinRequestID int32) (string, error) {
	// 1. Fetch the join request by ID
	joinReq, err := s.reqRepo.GetByID(ctx, joinRequestID)
	if err != nil {
		return "", fmt.Errorf("failed to get join request: %w", err)
	}
	if joinReq.OrgID != orgID {
		return "", fmt.Errorf("join request does not belong to the given organization")
	}

	// 2. Get Organization
	org, err := s.orgRepo.GetByID(ctx, orgID)
	if err != nil {
		return "", fmt.Errorf("failed to get organization: %w", err)
	}

	var invitationCode string

	// 3. Check if user already exists
	user, err := s.userRepo.GetByEmail(ctx, joinReq.Email)
	if err == nil && user != nil {
		// User exists, add to org
		userOrg := &domain.UserOrg{
			UserID:       user.ID,
			OrgID:        orgID,
			JoinedOn:     time.Now().Format("2006-01-02"),
			Status:       domain.UserOrgStatusActive,
			Role:         domain.UserOrgRoleMember,
			BalanceCents: 0,
		}
		if err := s.userRepo.AddUserToOrg(ctx, userOrg); err != nil {
			return "", fmt.Errorf("failed to add existing user to org: %w", err)
		}

		// Notify user
		_ = s.emailSvc.SendAccountStatusNotification(ctx, joinReq.Email, joinReq.Name, org.Name, "APPROVED", "Your join request has been approved.")

	} else {
		// User does not exist — create an invitation linked to the join request
		inv := &domain.Invitation{
			OrgID:         orgID,
			Email:         joinReq.Email,
			JoinRequestID: &joinReq.ID,
			CreatedBy:     adminID,
			ExpiresOn:     time.Now().Add(7 * 24 * time.Hour).Format("2006-01-02"),
		}
		if err := s.inviteRepo.Create(ctx, inv); err != nil {
			return "", fmt.Errorf("failed to create invitation: %w", err)
		}

		// Get admin email for CC
		admin, err := s.userRepo.GetByID(ctx, adminID)
		if err != nil {
			return "", fmt.Errorf("failed to get admin user: %w", err)
		}

		if err := s.emailSvc.SendInvitation(ctx, joinReq.Email, joinReq.Name, inv.InvitationCode, org.Name, admin.Email); err != nil {
			return "", fmt.Errorf("failed to send invitation email: %w", err)
		}

		invitationCode = inv.InvitationCode
	}

	// 4. Mark the join request as INVITED
	joinReq.Status = domain.JoinRequestStatusInvited
	if err := s.reqRepo.Update(ctx, joinReq); err != nil {
		return "", fmt.Errorf("failed to update join request status: %w", err)
	}

	if invitationCode != "" {
		return invitationCode, nil
	}
	return "", nil
}

func (s *adminService) BlockUser(ctx context.Context, adminID, userID, orgID int32, blockRenting, blockLending bool, reason string) error {
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

	uo.RentingBlocked = blockRenting
	uo.LendingBlocked = blockLending

	if blockRenting || blockLending {
		uo.Status = domain.UserOrgStatusBlock
		uo.BlockedReason = reason
		now := time.Now().Format("2006-01-02")
		uo.BlockedOn = &now
	} else {
		uo.Status = domain.UserOrgStatusActive
		uo.BlockedReason = ""
		uo.BlockedOn = nil
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

func (s *adminService) RejectJoinRequest(ctx context.Context, adminID, orgID, joinRequestID int32, reason string) error {
	// 1. Fetch the join request by ID
	joinReq, err := s.reqRepo.GetByID(ctx, joinRequestID)
	if err != nil {
		return fmt.Errorf("failed to get join request: %w", err)
	}
	if joinReq.OrgID != orgID {
		return fmt.Errorf("join request does not belong to the given organization")
	}

	// 2. Update status to REJECTED, set reason and rejected_by_user_id
	joinReq.Status = domain.JoinRequestStatusRejected
	joinReq.Reason = reason
	joinReq.RejectedByUserID = &adminID
	if err := s.reqRepo.Update(ctx, joinReq); err != nil {
		return fmt.Errorf("failed to update join request: %w", err)
	}

	// 3. Stamp any linked invitation as used (voided by admin rejection), only if not already stamped
	if inv, err := s.inviteRepo.GetByJoinRequestID(ctx, joinRequestID); err == nil && inv != nil && inv.UsedOn == nil && inv.UsedByUserID == nil {
		nowStr := time.Now().Format("2006-01-02")
		inv.UsedOn = &nowStr
		inv.UsedByUserID = &adminID
		_ = s.inviteRepo.Update(ctx, inv)
	}

	// 4. Notify applicant
	org, err := s.orgRepo.GetByID(ctx, orgID)
	if err != nil {
		return fmt.Errorf("failed to get organization: %w", err)
	}
	_ = s.emailSvc.SendAccountStatusNotification(ctx, joinReq.Email, joinReq.Name, org.Name, "REJECTED", reason)

	return nil
}

func (s *adminService) SendInvitation(ctx context.Context, adminID, orgID int32, email, name string) (string, error) {
	// Get organization
	org, err := s.orgRepo.GetByID(ctx, orgID)
	if err != nil {
		return "", fmt.Errorf("failed to get organization: %w", err)
	}

	// Check if user already exists and is already a member
	existingUser, _ := s.userRepo.GetByEmail(ctx, email)
	if existingUser != nil {
		uo, err := s.userRepo.GetUserOrg(ctx, existingUser.ID, orgID)
		if err == nil && uo != nil {
			return "", fmt.Errorf("user is already a member of this organization")
		}
	}

	// Create invitation
	inv := &domain.Invitation{
		OrgID:     orgID,
		Email:     email,
		CreatedBy: adminID,
		ExpiresOn: time.Now().Add(7 * 24 * time.Hour).Format("2006-01-02"),
	}
	if err := s.inviteRepo.Create(ctx, inv); err != nil {
		return "", fmt.Errorf("failed to create invitation: %w", err)
	}

	// Get admin email for CC
	admin, err := s.userRepo.GetByID(ctx, adminID)
	if err != nil {
		return "", fmt.Errorf("failed to get admin user: %w", err)
	}

	if err := s.emailSvc.SendInvitation(ctx, email, name, inv.InvitationCode, org.Name, admin.Email); err != nil {
		return "", fmt.Errorf("failed to send invitation email: %w", err)
	}

	return inv.InvitationCode, nil
}

func (s *adminService) GetMemberProfile(ctx context.Context, orgID, userID int32) (*domain.User, *domain.UserOrg, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get user: %w", err)
	}

	uo, err := s.userRepo.GetUserOrg(ctx, userID, orgID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get user org: %w", err)
	}

	return user, uo, nil
}
