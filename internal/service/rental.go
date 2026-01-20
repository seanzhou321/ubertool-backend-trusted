package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/repository"
)

type rentalService struct {
	rentalRepo repository.RentalRepository
	toolRepo   repository.ToolRepository
	ledgerRepo repository.LedgerRepository
	userRepo   repository.UserRepository
}

func NewRentalService(rentalRepo repository.RentalRepository, toolRepo repository.ToolRepository, ledgerRepo repository.LedgerRepository, userRepo repository.UserRepository) RentalService {
	return &rentalService{
		rentalRepo: rentalRepo,
		toolRepo:   toolRepo,
		ledgerRepo: ledgerRepo,
		userRepo:   userRepo,
	}
}

func (s *rentalService) CreateRentalRequest(ctx context.Context, renterID, toolID, orgID int32, startDateStr, endDateStr string) (*domain.Rental, error) {
	tool, err := s.toolRepo.GetByID(ctx, toolID)
	if err != nil {
		return nil, err
	}

	start, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		return nil, err
	}
	end, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		return nil, err
	}

	days := int32(end.Sub(start).Hours() / 24)
	if days <= 0 {
		return nil, errors.New("end date must be after start date")
	}

	// Calculate cost (simplified: daily price only for now)
	totalCost := tool.PricePerDayCents * days

	// Check balance
	balance, err := s.ledgerRepo.GetBalance(ctx, renterID, orgID)
	if err != nil {
		return nil, err
	}
	if balance < totalCost {
		return nil, errors.New("insufficient balance")
	}

	rental := &domain.Rental{
		OrgID:            orgID,
		ToolID:           toolID,
		RenterID:         renterID,
		OwnerID:          tool.OwnerID,
		StartDate:        start,
		ScheduledEndDate: end,
		TotalCostCents:   totalCost,
		Status:           domain.RentalStatusPending,
	}

	if err := s.rentalRepo.Create(ctx, rental); err != nil {
		return nil, err
	}

	return rental, nil
}

func (s *rentalService) ApproveRentalRequest(ctx context.Context, ownerID, rentalID int32, pickupNote string) (*domain.Rental, error) {
	rt, err := s.rentalRepo.GetByID(ctx, rentalID)
	if err != nil {
		return nil, err
	}
	if rt.OwnerID != ownerID {
		return nil, errors.New("unauthorized")
	}
	if rt.Status != domain.RentalStatusPending {
		return nil, errors.New("rental is not pending")
	}

	rt.Status = domain.RentalStatusApproved
	rt.PickupNote = pickupNote
	if err := s.rentalRepo.Update(ctx, rt); err != nil {
		return nil, err
	}
	return rt, nil
}

func (s *rentalService) RejectRentalRequest(ctx context.Context, ownerID, rentalID int32) (*domain.Rental, error) {
	rt, err := s.rentalRepo.GetByID(ctx, rentalID)
	if err != nil {
		return nil, err
	}
	if rt.OwnerID != ownerID {
		return nil, errors.New("unauthorized")
	}

	rt.Status = domain.RentalStatusRejected
	if err := s.rentalRepo.Update(ctx, rt); err != nil {
		return nil, err
	}
	return rt, nil
}

func (s *rentalService) CancelRental(ctx context.Context, renterID, rentalID int32, reason string) (*domain.Rental, error) {
	rt, err := s.rentalRepo.GetByID(ctx, rentalID)
	if err != nil {
		return nil, err
	}
	if rt.RenterID != renterID {
		return nil, errors.New("unauthorized")
	}

	// Can only cancel if PENDING or APPROVED or SCHEDULED (maybe?)
	// Logic says: if cancel, status -> CANCELLED.
	// NOTE: If paid, we might need refund logic. For now, just change status.
	
	rt.Status = domain.RentalStatusCancelled
	// rt.CancelReason = reason // User request mentioned reason, but domain struct doesn't have it? 
	// The schema doesn't have cancel_reason column for rental table.
	// So we ignore reason for now or log it.

	if err := s.rentalRepo.Update(ctx, rt); err != nil {
		return nil, err
	}
	return rt, nil
}

func (s *rentalService) FinalizeRentalRequest(ctx context.Context, renterID, rentalID int32) (*domain.Rental, error) {
	rt, err := s.rentalRepo.GetByID(ctx, rentalID)
	if err != nil {
		return nil, err
	}
	if rt.RenterID != renterID {
		return nil, errors.New("unauthorized")
	}
	if rt.Status != domain.RentalStatusApproved {
		return nil, errors.New("rental is not approved by owner")
	}

	rt.Status = domain.RentalStatusScheduled
	if err := s.rentalRepo.Update(ctx, rt); err != nil {
		return nil, err
	}
	return rt, nil
}

func (s *rentalService) CompleteRental(ctx context.Context, ownerID, rentalID int32) (*domain.Rental, error) {
	rt, err := s.rentalRepo.GetByID(ctx, rentalID)
	if err != nil {
		return nil, err
	}
	if rt.OwnerID != ownerID {
		return nil, errors.New("unauthorized")
	}
	
	// Transaction: Renter -> Owner
	debit := &domain.LedgerTransaction{
		OrgID:           rt.OrgID,
		UserID:          rt.RenterID,
		Amount:          -rt.TotalCostCents,
		Type:            domain.TransactionTypeRentalDebit,
		RelatedRentalID: &rt.ID,
		Description:     fmt.Sprintf("Rental for tool %d", rt.ToolID),
	}
	if err := s.ledgerRepo.CreateTransaction(ctx, debit); err != nil {
		return nil, err
	}

	credit := &domain.LedgerTransaction{
		OrgID:           rt.OrgID,
		UserID:          rt.OwnerID,
		Amount:          rt.TotalCostCents,
		Type:            domain.TransactionTypeLendingCredit,
		RelatedRentalID: &rt.ID,
		Description:     fmt.Sprintf("Lending for tool %d", rt.ToolID),
	}
	if err := s.ledgerRepo.CreateTransaction(ctx, credit); err != nil {
		return nil, err
	}

	now := time.Now()
	rt.EndDate = &now
	rt.Status = domain.RentalStatusCompleted
	if err := s.rentalRepo.Update(ctx, rt); err != nil {
		return nil, err
	}
	return rt, nil
}

func (s *rentalService) ListRentals(ctx context.Context, userID, orgID int32, status string, page, pageSize int32) ([]domain.Rental, int32, error) {
	return s.rentalRepo.ListByRenter(ctx, userID, orgID, status, page, pageSize)
}

func (s *rentalService) ListLendings(ctx context.Context, userID, orgID int32, status string, page, pageSize int32) ([]domain.Rental, int32, error) {
	return s.rentalRepo.ListByOwner(ctx, userID, orgID, status, page, pageSize)
}

func (s *rentalService) GetRental(ctx context.Context, userID, rentalID int32) (*domain.Rental, error) {
	rt, err := s.rentalRepo.GetByID(ctx, rentalID)
	if err != nil {
		return nil, err
	}
	if rt.RenterID != userID && rt.OwnerID != userID {
		return nil, errors.New("unauthorized")
	}
	return rt, nil
}
