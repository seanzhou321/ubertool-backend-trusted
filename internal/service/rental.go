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

func (s *rentalService) ApproveRentalRequest(ctx context.Context, ownerID, rentalID int32, pickupNote string) error {
	rt, err := s.rentalRepo.GetByID(ctx, rentalID)
	if err != nil {
		return err
	}
	if rt.OwnerID != ownerID {
		return errors.New("unauthorized")
	}
	if rt.Status != domain.RentalStatusPending {
		return errors.New("rental is not pending")
	}

	rt.Status = domain.RentalStatusApproved
	rt.PickupNote = pickupNote
	return s.rentalRepo.Update(ctx, rt)
}

func (s *rentalService) RejectRentalRequest(ctx context.Context, ownerID, rentalID int32) error {
	rt, err := s.rentalRepo.GetByID(ctx, rentalID)
	if err != nil {
		return err
	}
	if rt.OwnerID != ownerID {
		return errors.New("unauthorized")
	}

	rt.Status = domain.RentalStatusCancelled
	return s.rentalRepo.Update(ctx, rt)
}

func (s *rentalService) FinalizeRentalRequest(ctx context.Context, renterID, rentalID int32) error {
	rt, err := s.rentalRepo.GetByID(ctx, rentalID)
	if err != nil {
		return err
	}
	if rt.RenterID != renterID {
		return errors.New("unauthorized")
	}
	if rt.Status != domain.RentalStatusApproved {
		return errors.New("rental is not approved by owner")
	}

	rt.Status = domain.RentalStatusScheduled
	return s.rentalRepo.Update(ctx, rt)
}

func (s *rentalService) CompleteRental(ctx context.Context, ownerID, rentalID int32) error {
	rt, err := s.rentalRepo.GetByID(ctx, rentalID)
	if err != nil {
		return err
	}
	if rt.OwnerID != ownerID {
		return errors.New("unauthorized")
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
		return err
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
		return err
	}

	now := time.Now()
	rt.EndDate = &now
	rt.Status = domain.RentalStatusCompleted
	return s.rentalRepo.Update(ctx, rt)
}

func (s *rentalService) ListRentals(ctx context.Context, userID, orgID int32, status string, page, pageSize int32) ([]domain.Rental, int32, error) {
	return s.rentalRepo.ListByRenter(ctx, userID, orgID, status, page, pageSize)
}

func (s *rentalService) ListLendings(ctx context.Context, userID, orgID int32, status string, page, pageSize int32) ([]domain.Rental, int32, error) {
	return s.rentalRepo.ListByOwner(ctx, userID, orgID, status, page, pageSize)
}
