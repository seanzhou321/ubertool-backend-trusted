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
		emailSvc.On("SendRentalRequestNotification", ctx, "owner@test.com", "Renter", "Tool", "renter@test.com").Return(nil)
		noteRepo.On("Create", ctx, mock.AnythingOfType("*domain.Notification")).Return(nil)

		res, err := svc.CreateRentalRequest(ctx, renterID, toolID, orgID, startDate, endDate)
		assert.NoError(t, err)
		assert.NotNil(t, res)
		assert.Equal(t, toolID, res.ToolID)
		assert.Equal(t, renterID, res.RenterID)
		assert.Equal(t, int32(2000), res.TotalCostCents) // 2 days inclusive (24h to 48h) * 1000
	})

	// Balance check is disabled for now
	// t.Run("Insufficient Balance", func(t *testing.T) {
	// 	toolRepo.ExpectedCalls = nil
	// 	toolRepo.On("GetByID", ctx, toolID).Return(tool, nil)
	// 	ledgerRepo.ExpectedCalls = nil
	// 	ledgerRepo.On("GetBalance", ctx, renterID, orgID).Return(int32(500), nil) // Cost is 1000

	// 	res, err := svc.CreateRentalRequest(ctx, renterID, toolID, orgID, startDate, endDate)
	// 	assert.Error(t, err)
	// 	assert.Nil(t, res)
	// 	assert.Contains(t, err.Error(), "insufficient balance")
	// })
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

func TestRentalService_FinalizeRentalRequest(t *testing.T) {
	rentalRepo := new(MockRentalRepo)
	toolRepo := new(MockToolRepo)
	ledgerRepo := new(MockLedgerRepo)
	userRepo := new(MockUserRepo)
	emailSvc := new(MockEmailService)
	noteRepo := new(MockNotificationRepo)
	
	svc := service.NewRentalService(rentalRepo, toolRepo, ledgerRepo, userRepo, emailSvc, noteRepo)
	ctx := context.Background()

	renterID := int32(1)
	ownerID := int32(10)
	rentalID := int32(100)
	toolID := int32(200)

	// Status must be Approved to Finalize
	requestRental := &domain.Rental{
		ID: rentalID, RenterID: renterID, OwnerID: ownerID, ToolID: toolID, 
		Status: domain.RentalStatusApproved, TotalCostCents: 5000,
		OrgID: 99,
	}
	approvedRental := domain.Rental{ID: 101, ToolID: toolID, Status: domain.RentalStatusApproved}
	pendingRental := domain.Rental{ID: 102, ToolID: toolID, Status: domain.RentalStatusPending}
	
	tool := &domain.Tool{ID: toolID, Name: "Hammer", Status: domain.ToolStatusAvailable}
	renter := &domain.User{ID: renterID, Name: "Renter", Email: "r@test.com"}
	owner := &domain.User{ID: ownerID, Name: "Owner", Email: "o@test.com"}

	t.Run("Success", func(t *testing.T) {
		// 1. Get Rental
		rentalRepo.On("GetByID", ctx, rentalID).Return(requestRental, nil)
		
		// 2. Debit Ledger
		ledgerRepo.On("CreateTransaction", ctx, mock.MatchedBy(func(tx *domain.LedgerTransaction) bool {
			return tx.Amount == -5000 && tx.UserID == renterID
		})).Return(nil)
		
		// 3. Update Rental Status
		rentalRepo.On("Update", ctx, mock.MatchedBy(func(r *domain.Rental) bool {
			return r.Status == domain.RentalStatusScheduled
		})).Return(nil)
		
		// 4. Update Tool Status
		toolRepo.On("GetByID", ctx, toolID).Return(tool, nil)
		toolRepo.On("Update", ctx, mock.MatchedBy(func(tl *domain.Tool) bool {
			return tl.Status == domain.ToolStatusRented
		})).Return(nil)
		
		// 5. Notifications
		userRepo.On("GetByID", ctx, renterID).Return(renter, nil)
		userRepo.On("GetByID", ctx, ownerID).Return(owner, nil)
		emailSvc.On("SendRentalConfirmationNotification", ctx, owner.Email, renter.Name, tool.Name, renter.Email).Return(nil)
		noteRepo.On("Create", ctx, mock.AnythingOfType("*domain.Notification")).Return(nil)

		// 6. List Related Rentals
		rentalRepo.On("ListByTool", ctx, mock.Anything, mock.Anything, string(domain.RentalStatusApproved), mock.Anything, mock.Anything).
			Return([]domain.Rental{approvedRental}, int32(1), nil)
		rentalRepo.On("ListByTool", ctx, mock.Anything, mock.Anything, string(domain.RentalStatusPending), mock.Anything, mock.Anything).
			Return([]domain.Rental{pendingRental}, int32(1), nil)

		res, approved, pending, err := svc.FinalizeRentalRequest(ctx, renterID, rentalID)
		if err != nil {
			t.Logf("Computed Error: %v", err)
		}
		assert.NoError(t, err)
		assert.Equal(t, rentalID, res.ID)
		assert.Len(t, approved, 1)
		assert.Len(t, pending, 1)
		assert.Equal(t, approvedRental.ID, approved[0].ID)
		assert.Equal(t, pendingRental.ID, pending[0].ID)
	})
}

func TestRentalService_ActivateRental(t *testing.T) {
	rentalRepo := new(MockRentalRepo)
	toolRepo := new(MockToolRepo)
	userRepo := new(MockUserRepo)
	emailSvc := new(MockEmailService)
	noteRepo := new(MockNotificationRepo)
	
	svc := service.NewRentalService(rentalRepo, toolRepo, nil, userRepo, emailSvc, noteRepo)
	ctx := context.Background()

	ownerID := int32(10)
	renterID := int32(20)
	rentalID := int32(100)
	toolID := int32(200)

	rental := &domain.Rental{
		ID: rentalID, OwnerID: ownerID, RenterID: renterID, ToolID: toolID,
		Status: domain.RentalStatusScheduled,
		StartDate: time.Now(), ScheduledEndDate: time.Now().Add(24*time.Hour),
		OrgID: 3,
	}
	tool := &domain.Tool{ID: toolID, Name: "Tool"}
	renter := &domain.User{ID: renterID, Email: "renter@a.com", Name: "Renter"}
	owner := &domain.User{ID: ownerID, Email: "owner@a.com", Name: "Owner"}

	t.Run("Success", func(t *testing.T) {
		rentalRepo.On("GetByID", ctx, rentalID).Return(rental, nil)
		rentalRepo.On("Update", ctx, mock.MatchedBy(func(r *domain.Rental) bool {
			return r.Status == domain.RentalStatusActive
		})).Return(nil)
		
		toolRepo.On("GetByID", ctx, toolID).Return(tool, nil)
		userRepo.On("GetByID", ctx, renterID).Return(renter, nil)
		userRepo.On("GetByID", ctx, ownerID).Return(owner, nil)
		
		emailSvc.On("SendRentalPickupNotification", ctx, renter.Email, renter.Name, tool.Name, mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(nil)
		noteRepo.On("Create", ctx, mock.AnythingOfType("*domain.Notification")).Return(nil)

		res, err := svc.ActivateRental(ctx, ownerID, rentalID)
		if err != nil {
			t.Logf("Computed Error: %v", err)
		}
		assert.NoError(t, err)
		assert.Equal(t, domain.RentalStatusActive, res.Status)
	})
}


func TestRentalService_ChangeRentalDates(t *testing.T) {
	rentalRepo := new(MockRentalRepo)
	toolRepo := new(MockToolRepo)
	emailSvc := new(MockEmailService)
	userRepo := new(MockUserRepo)
	noteRepo := new(MockNotificationRepo)
	
	svc := service.NewRentalService(rentalRepo, toolRepo, nil, userRepo, emailSvc, noteRepo)
	ctx := context.Background()

	renterID := int32(20)
	ownerID := int32(10)
	rentalID := int32(100)
	toolID := int32(200)

	baseRental := &domain.Rental{
		ID: rentalID, RenterID: renterID, OwnerID: ownerID, ToolID: toolID,
		Status: domain.RentalStatusActive,
		StartDate: time.Now(), ScheduledEndDate: time.Now().Add(24*time.Hour),
		TotalCostCents: 1000,
	}
	tool := &domain.Tool{ID: toolID, Name: "Drill", PricePerDayCents: 1000}

	t.Run("Renter Extension Active", func(t *testing.T) {
		r := *baseRental
		newEnd := time.Now().Add(48*time.Hour).Format("2006-01-02")
		
		rentalRepo.On("GetByID", ctx, rentalID).Return(&r, nil)
		toolRepo.On("GetByID", ctx, toolID).Return(tool, nil)
		
		// Expect update with temp status and new cost
		rentalRepo.On("Update", ctx, mock.MatchedBy(func(u *domain.Rental) bool {
			return u.Status == domain.RentalStatusReturnDateChanged &&
				u.EndDate != nil && 
				u.TotalCostCents == 2000 // 2 days * 1000
		})).Return(nil)

		// Notifications
		userRepo.On("GetByID", ctx, ownerID).Return(&domain.User{ID: ownerID, Email: "owner@a.com"}, nil)
		// Expect notification via NotificationRepo
		noteRepo.On("Create", ctx, mock.MatchedBy(func(n *domain.Notification) bool {
			return n.UserID == ownerID && n.Title == "Return Date Extension Request"
		})).Return(nil)

		_, err := svc.ChangeRentalDates(ctx, renterID, rentalID, "", newEnd, "", "")
		assert.NoError(t, err)
	})
}
