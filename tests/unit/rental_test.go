package unit

import (
	"context"
	"testing"
	"time"

	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestRentalService_CreateRentalRequest(t *testing.T) {
	rentalRepo := new(MockRentalRepo)
	toolRepo := new(MockToolRepo)
	ledgerRepo := new(MockLedgerRepo)
	userRepo := new(MockUserRepo)
	svc := service.NewRentalService(rentalRepo, toolRepo, ledgerRepo, userRepo)

	ctx := context.Background()
	renterID := int32(1)
	toolID := int32(2)
	orgID := int32(3)
	startDate := time.Now().Add(24 * time.Hour).Format("2006-01-02")
	endDate := time.Now().Add(48 * time.Hour).Format("2006-01-02")

	tool := &domain.Tool{
		ID:               toolID,
		OwnerID:          10,
		PricePerDayCents: 1000,
	}

	t.Run("Success", func(t *testing.T) {
		toolRepo.On("GetByID", ctx, toolID).Return(tool, nil)
		ledgerRepo.On("GetBalance", ctx, renterID, orgID).Return(int32(5000), nil)
		rentalRepo.On("Create", ctx, mock.AnythingOfType("*domain.Rental")).Return(nil)

		res, err := svc.CreateRentalRequest(ctx, renterID, toolID, orgID, startDate, endDate)
		assert.NoError(t, err)
		assert.NotNil(t, res)
		assert.Equal(t, toolID, res.ToolID)
		assert.Equal(t, renterID, res.RenterID)
		assert.Equal(t, int32(1000), res.TotalCostCents) // 1 day * 1000
	})

	t.Run("Insufficient Balance", func(t *testing.T) {
		toolRepo.ExpectedCalls = nil
		toolRepo.On("GetByID", ctx, toolID).Return(tool, nil)
		ledgerRepo.ExpectedCalls = nil
		ledgerRepo.On("GetBalance", ctx, renterID, orgID).Return(int32(500), nil) // Cost is 1000

		res, err := svc.CreateRentalRequest(ctx, renterID, toolID, orgID, startDate, endDate)
		assert.Error(t, err)
		assert.Nil(t, res)
		assert.Contains(t, err.Error(), "insufficient balance")
	})
}

func TestRentalService_CompleteRental(t *testing.T) {
	rentalRepo := new(MockRentalRepo)
	toolRepo := new(MockToolRepo)
	ledgerRepo := new(MockLedgerRepo)
	userRepo := new(MockUserRepo)
	svc := service.NewRentalService(rentalRepo, toolRepo, ledgerRepo, userRepo)

	ctx := context.Background()
	ownerID := int32(10)
	rentalID := int32(1)

	rental := &domain.Rental{
		ID:             rentalID,
		RenterID:       1,
		OwnerID:        ownerID,
		OrgID:          3,
		TotalCostCents: 1000,
		Status:         domain.RentalStatusActive,
	}

	t.Run("Success", func(t *testing.T) {
		rentalRepo.On("GetByID", ctx, rentalID).Return(rental, nil)
		ledgerRepo.On("CreateTransaction", ctx, mock.AnythingOfType("*domain.LedgerTransaction")).Return(nil)
		rentalRepo.On("Update", ctx, mock.AnythingOfType("*domain.Rental")).Return(nil)

		err := svc.CompleteRental(ctx, ownerID, rentalID)
		assert.NoError(t, err)
		
		// Two transactions should be created: debit renter, credit owner
		ledgerRepo.AssertNumberOfCalls(t, "CreateTransaction", 2)
	})
}
