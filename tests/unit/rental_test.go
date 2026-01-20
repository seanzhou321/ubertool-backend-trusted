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
	emailSvc := new(MockEmailService)
	noteRepo := new(MockNotificationRepo)
	
	svc := service.NewRentalService(rentalRepo, toolRepo, ledgerRepo, userRepo, emailSvc, noteRepo)

	ctx := context.Background()
	renterID := int32(1)
	toolID := int32(2)
	orgID := int32(3)
	startDate := time.Now().Add(24 * time.Hour).Format("2006-01-02")
	endDate := time.Now().Add(48 * time.Hour).Format("2006-01-02")

	tool := &domain.Tool{
		ID:               toolID,
		Name:             "Tool",
		OwnerID:          10,
		PricePerDayCents: 1000,
	}

	t.Run("Success", func(t *testing.T) {
		toolRepo.On("GetByID", ctx, toolID).Return(tool, nil)
		ledgerRepo.On("GetBalance", ctx, renterID, orgID).Return(int32(5000), nil)
		rentalRepo.On("Create", ctx, mock.AnythingOfType("*domain.Rental")).Return(nil)


		// Setup expectations for email notification
		userRepo.On("GetByID", ctx, int32(10)).Return(&domain.User{ID: 10, Email: "owner@test.com", Name: "Owner"}, nil)
		userRepo.On("GetByID", ctx, renterID).Return(&domain.User{ID: renterID, Email: "renter@test.com", Name: "Renter"}, nil)
		emailSvc.On("SendRentalRequestNotification", ctx, "owner@test.com", "Renter", "Tool").Return(nil)
		noteRepo.On("Create", ctx, mock.AnythingOfType("*domain.Notification")).Return(nil)

		res, err := svc.CreateRentalRequest(ctx, renterID, toolID, orgID, startDate, endDate)
		assert.NoError(t, err)
		assert.NotNil(t, res)
		assert.Equal(t, toolID, res.ToolID)
		assert.Equal(t, renterID, res.RenterID)
		assert.Equal(t, int32(2000), res.TotalCostCents) // 2 days inclusive (24h to 48h) * 1000
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
	emailSvc := new(MockEmailService)
	noteRepo := new(MockNotificationRepo)
	svc := service.NewRentalService(rentalRepo, toolRepo, ledgerRepo, userRepo, emailSvc, noteRepo)

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
		toolRepo.On("GetByID", ctx, int32(0)).Return(&domain.Tool{Name: "Tool"}, nil) // ToolID is 0 in setup
		toolRepo.On("Update", ctx, mock.AnythingOfType("*domain.Tool")).Return(nil)

		userRepo.On("GetByID", ctx, int32(1)).Return(&domain.User{Email: "renter@test.com"}, nil)
		userRepo.On("GetByID", ctx, ownerID).Return(&domain.User{Email: "owner@test.com"}, nil)
		emailSvc.On("SendRentalCompletionNotification", ctx, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

		res, err := svc.CompleteRental(ctx, ownerID, rentalID)
		assert.NoError(t, err)
		assert.NotNil(t, res)
		
		// One transaction should be created: credit owner (debit was at finalize)
		ledgerRepo.AssertNumberOfCalls(t, "CreateTransaction", 1)
	})
}
