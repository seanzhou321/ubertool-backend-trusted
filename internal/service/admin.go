package service

import (
	"context"
	"errors"
	"time"

	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/repository"
)

type adminService struct {
	reqRepo    repository.JoinRequestRepository
	userRepo   repository.UserRepository
	ledgerRepo repository.LedgerRepository
}

func NewAdminService(reqRepo repository.JoinRequestRepository, userRepo repository.UserRepository, ledgerRepo repository.LedgerRepository) AdminService {
	return &adminService{
		reqRepo:    reqRepo,
		userRepo:   userRepo,
		ledgerRepo: ledgerRepo,
	}
}

func (s *adminService) ApproveJoinRequest(ctx context.Context, adminID, requestID int32) error {
	req, err := s.reqRepo.GetByID(ctx, requestID)
	if err != nil {
		return err
	}
	if req.Status != "PENDING" {
		return errors.New("request is not pending")
	}

	// In a real app, we'd check if adminID has permission for req.OrgID

	// Create user if not exists? Usually "Request to Join" means user already has an account but wants to join an org.
	// Or it could mean they are signing up. 
	// The PRD says: "If Admin approves, the User receives a confirmation and is added automatically to the organization."
	
	user, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err != nil {
		return errors.New("user must sign up first or we need a way to invite them upon approval")
	}

	userOrg := &domain.UserOrg{
		UserID:   user.ID,
		OrgID:    req.OrgID,
		Role:     domain.UserOrgRoleMember,
		Status:   domain.UserOrgStatusActive,
		JoinedOn: time.Now(),
	}
	if err := s.userRepo.AddUserToOrg(ctx, userOrg); err != nil {
		return err
	}

	req.Status = "APPROVED"
	return s.reqRepo.Update(ctx, req)
}

func (s *adminService) BlockUser(ctx context.Context, adminID, userID, orgID int32, reason string) error {
	uo, err := s.userRepo.GetUserOrg(ctx, userID, orgID)
	if err != nil {
		return err
	}
	uo.Status = domain.UserOrgStatusBlock
	uo.BlockReason = reason
	now := time.Now()
	uo.BlockedDate = &now
	return s.userRepo.UpdateUserOrg(ctx, uo)
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
