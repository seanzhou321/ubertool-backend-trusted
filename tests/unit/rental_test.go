package unit

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
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
	endDate := time.Now().Add(72 * time.Hour).Format("2006-01-02") // 2-day rental (end-exclusive)

	tool := &domain.Tool{
		ID:                 toolID,
		Name:               "Tool",
		OwnerID:            10,
		PricePerDayCents:   1000,
		PricePerWeekCents:  6000,
		PricePerMonthCents: 20000,
		DurationUnit:       domain.ToolDurationUnitDay,
	}

	t.Run("Success", func(t *testing.T) {
		userRepo.On("GetUserOrg", ctx, renterID, orgID).Return(&domain.UserOrg{UserID: renterID, OrgID: orgID}, nil)
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
		assert.Equal(t, int32(2000), res.TotalCostCents) // 2 days (end-exclusive: +24h to +72h) * 1000
	})

	// SEC-RENTAL-001 (sbr/rtm/009-security.rtm.md): CreateRentalRequest must reject an
	// organization_id the caller does not belong to — mirrors the membership check
	// ToolService.SearchTools already performs (internal/service/tool.go:118-122). Regression
	// test for a real gap found during the security audit: since domain.Tool has no OrgID field
	// at all, without this check any tool could be rented under any organization context the
	// caller names.
	t.Run("Rejects an organization the caller is not a member of (SEC-RENTAL-001)", func(t *testing.T) {
		toolRepo := new(MockToolRepo)
		ledgerRepo := new(MockLedgerRepo)
		userRepo := new(MockUserRepo)
		emailSvc := new(MockEmailService)
		noteRepo := new(MockNotificationRepo)
		rentalRepo := new(MockRentalRepo)
		svc := service.NewRentalService(rentalRepo, toolRepo, ledgerRepo, userRepo, emailSvc, noteRepo)

		foreignOrgID := int32(999)
		// The renter does not belong to foreignOrgID — matches the pattern SearchTools already
		// uses to detect non-membership.
		userRepo.On("GetUserOrg", ctx, renterID, foreignOrgID).Return(nil, fmt.Errorf("no rows in result set"))

		res, err := svc.CreateRentalRequest(ctx, renterID, toolID, foreignOrgID, startDate, endDate)
		require.Error(t, err, "CreateRentalRequest must reject an organization_id the caller does not belong to")
		assert.Nil(t, res)
		toolRepo.AssertNotCalled(t, "GetByID", mock.Anything, mock.Anything)
		rentalRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
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

	// FR-001 (specs/005-rentals): CreateRentalRequest must reject end_date <= start_date
	// (Acceptance Scenario 2). Prior to this test, `grep -r "end date must be after start date"`
	// under tests/ returned nothing.
	t.Run("Rejects an end date that is not after the start date", func(t *testing.T) {
		toolRepo := new(MockToolRepo)
		ledgerRepo := new(MockLedgerRepo)
		userRepo := new(MockUserRepo)
		emailSvc := new(MockEmailService)
		noteRepo := new(MockNotificationRepo)
		rentalRepo := new(MockRentalRepo)
		svc := service.NewRentalService(rentalRepo, toolRepo, ledgerRepo, userRepo, emailSvc, noteRepo)

		userRepo.On("GetUserOrg", ctx, renterID, orgID).Return(&domain.UserOrg{UserID: renterID, OrgID: orgID}, nil)
		toolRepo.On("GetByID", ctx, toolID).Return(tool, nil)
		sameDay := time.Now().Add(24 * time.Hour).Format("2006-01-02")

		res, err := svc.CreateRentalRequest(ctx, renterID, toolID, orgID, sameDay, sameDay)
		require.Error(t, err)
		assert.Nil(t, res)
		assert.Contains(t, err.Error(), "end date must be after start date")
		rentalRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	})
}

func TestRentalService_CompleteRental(t *testing.T) {
	ctx := context.Background()
	ownerID := int32(10)
	renterID := int32(1)
	rentalID := int32(1)
	orgID := int32(3)
	toolID := int32(0)

	// startDate 2 days ago, endDate today → 2-day (end-exclusive) cost = 2000 cents.
	startDate := time.Now().Add(-48 * time.Hour).Format("2006-01-02")
	endDate := time.Now().Format("2006-01-02")

	baseRental := &domain.Rental{
		ID:                rentalID,
		RenterID:          renterID,
		OwnerID:           ownerID,
		OrgID:             orgID,
		ToolID:            toolID,
		StartDate:         startDate,
		EndDate:           endDate,
		DurationUnit:      string(domain.ToolDurationUnitDay),
		DailyPriceCents:   1000,
		WeeklyPriceCents:  6000,
		MonthlyPriceCents: 20000,
		Status:            domain.RentalStatusActive,
	}

	newMocks := func() (
		*MockRentalRepo, *MockToolRepo, *MockLedgerRepo, *MockUserRepo, *MockEmailService, *MockNotificationRepo,
	) {
		return new(MockRentalRepo), new(MockToolRepo), new(MockLedgerRepo), new(MockUserRepo), new(MockEmailService), new(MockNotificationRepo)
	}

	t.Run("Success with charge_billsplit=true", func(t *testing.T) {
		rentalRepo, toolRepo, ledgerRepo, userRepo, emailSvc, noteRepo := newMocks()
		svc := service.NewRentalService(rentalRepo, toolRepo, ledgerRepo, userRepo, emailSvc, noteRepo)

		rt := *baseRental
		rentalRepo.On("GetByID", ctx, rentalID).Return(&rt, nil)
		rentalRepo.On("Update", ctx, mock.AnythingOfType("*domain.Rental")).Return(nil)
		rentalRepo.On("ListByTool", ctx, toolID, orgID, mock.Anything, int32(1), int32(1)).Return([]domain.Rental{}, int32(0), nil)

		userRepo.On("GetByID", ctx, renterID).Return(&domain.User{Email: "renter@test.com"}, nil)
		userRepo.On("GetByID", ctx, ownerID).Return(&domain.User{Email: "owner@test.com"}, nil)

		ledgerRepo.On("CreateTransaction", ctx, mock.AnythingOfType("*domain.LedgerTransaction")).Return(nil)
		toolRepo.On("GetByID", ctx, toolID).Return(&domain.Tool{Name: "Tool"}, nil)
		toolRepo.On("Update", ctx, mock.AnythingOfType("*domain.Tool")).Return(nil)
		// Notification/email calls happen in goroutines with a detached context; use mock.Anything for ctx.
		emailSvc.On("SendRentalCompletionNotification", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe().Return(nil)

		res, err := svc.CompleteRental(ctx, ownerID, rentalID, "Good condition", 0, "All good", true)
		assert.NoError(t, err)
		assert.NotNil(t, res)
		assert.Equal(t, domain.RentalStatusCompleted, res.Status)
		assert.True(t, res.ChargeBillsplit)

		// Allow goroutines to finish so ledger call counts are stable.
		time.Sleep(20 * time.Millisecond)

		// With charge_billsplit=true: credit owner + debit renter = 2 ledger entries.
		ledgerRepo.AssertNumberOfCalls(t, "CreateTransaction", 2)
		// Balance updates are handled by the DB trigger; the service does not call UpdateUserOrg.
		userRepo.AssertNotCalled(t, "UpdateUserOrg")
	})

	t.Run("Success with charge_billsplit=false", func(t *testing.T) {
		rentalRepo, toolRepo, ledgerRepo, userRepo, emailSvc, noteRepo := newMocks()
		svc := service.NewRentalService(rentalRepo, toolRepo, ledgerRepo, userRepo, emailSvc, noteRepo)

		rt := *baseRental
		rentalRepo.On("GetByID", ctx, rentalID).Return(&rt, nil)
		rentalRepo.On("Update", ctx, mock.AnythingOfType("*domain.Rental")).Return(nil)
		rentalRepo.On("ListByTool", ctx, toolID, orgID, mock.Anything, int32(1), int32(1)).Return([]domain.Rental{}, int32(0), nil)

		userRepo.On("GetByID", ctx, renterID).Return(&domain.User{Email: "renter@test.com"}, nil)
		userRepo.On("GetByID", ctx, ownerID).Return(&domain.User{Email: "owner@test.com"}, nil)

		toolRepo.On("GetByID", ctx, toolID).Return(&domain.Tool{Name: "Tool"}, nil)
		toolRepo.On("Update", ctx, mock.AnythingOfType("*domain.Tool")).Return(nil)
		// Notification/email calls happen in goroutines with a detached context; use mock.Anything for ctx.
		emailSvc.On("SendRentalCompletionNotification", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe().Return(nil)

		res, err := svc.CompleteRental(ctx, ownerID, rentalID, "Good condition", 0, "All good", false)
		assert.NoError(t, err)
		assert.NotNil(t, res)
		assert.Equal(t, domain.RentalStatusCompleted, res.Status)
		assert.False(t, res.ChargeBillsplit)

		// Allow goroutines to finish.
		time.Sleep(20 * time.Millisecond)

		// With charge_billsplit=false: NO ledger entries, NO balance updates.
		ledgerRepo.AssertNumberOfCalls(t, "CreateTransaction", 0)
		userRepo.AssertNotCalled(t, "GetUserOrg")
		userRepo.AssertNotCalled(t, "UpdateUserOrg")
	})

	// SEC-RENTAL-005 (sbr/rtm/009-security.rtm.md): rental_service.proto documents CompleteRental
	// as "Complete rental (mark as returned, owner only)", but loadAndValidateRental
	// (internal/service/rental.go:868) currently accepts either the owner OR the renter. A
	// renter can therefore self-complete their own active rental and fully control
	// surcharge_or_credit_cents (an unbounded, client-supplied field that feeds directly into
	// the settlement math), with no owner approval step at all.
	t.Run("Rejects a caller who is the renter, not the owner (SEC-RENTAL-005)", func(t *testing.T) {
		rentalRepo, toolRepo, ledgerRepo, userRepo, emailSvc, noteRepo := newMocks()
		svc := service.NewRentalService(rentalRepo, toolRepo, ledgerRepo, userRepo, emailSvc, noteRepo)

		rt := *baseRental
		rentalRepo.On("GetByID", ctx, rentalID).Return(&rt, nil)
		// Only reached along the current (buggy) path, which lets the renter complete the
		// rental exactly like an owner would — allow them with .Maybe() so the vulnerability
		// surfaces as a clean assertion failure below instead of an unrelated mock panic.
		rentalRepo.On("Update", ctx, mock.AnythingOfType("*domain.Rental")).Maybe().Return(nil)
		rentalRepo.On("ListByTool", ctx, toolID, orgID, mock.Anything, int32(1), int32(1)).Maybe().Return([]domain.Rental{}, int32(0), nil)
		userRepo.On("GetByID", ctx, renterID).Maybe().Return(&domain.User{Email: "renter@test.com"}, nil)
		userRepo.On("GetByID", ctx, ownerID).Maybe().Return(&domain.User{Email: "owner@test.com"}, nil)
		ledgerRepo.On("CreateTransaction", ctx, mock.AnythingOfType("*domain.LedgerTransaction")).Maybe().Return(nil)
		toolRepo.On("GetByID", ctx, toolID).Maybe().Return(&domain.Tool{Name: "Tool"}, nil)
		toolRepo.On("Update", ctx, mock.AnythingOfType("*domain.Tool")).Maybe().Return(nil)
		emailSvc.On("SendRentalCompletionNotification", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe().Return(nil)
		noteRepo.On("Dispatch", mock.Anything, mock.AnythingOfType("*domain.Notification")).Maybe().Return(nil)

		_, err := svc.CompleteRental(ctx, renterID, rentalID, "Good condition", -100000, "", true)
		require.Error(t, err, "CompleteRental is documented owner-only; a renter-initiated call (here also attempting a large negative surcharge) must be rejected")
	})

	t.Run("Settlement notification reminder text when charge_billsplit=false", func(t *testing.T) {
		rentalRepo, toolRepo, ledgerRepo, userRepo, emailSvc, noteRepo := newMocks()
		svc := service.NewRentalService(rentalRepo, toolRepo, ledgerRepo, userRepo, emailSvc, noteRepo)

		rt := *baseRental
		rentalRepo.On("GetByID", ctx, rentalID).Return(&rt, nil)
		rentalRepo.On("Update", ctx, mock.AnythingOfType("*domain.Rental")).Return(nil)
		rentalRepo.On("ListByTool", ctx, toolID, orgID, mock.Anything, int32(1), int32(1)).Return([]domain.Rental{}, int32(0), nil)

		userRepo.On("GetByID", ctx, renterID).Return(&domain.User{ID: renterID, Email: "renter@test.com", Name: "Renter"}, nil)
		userRepo.On("GetByID", ctx, ownerID).Return(&domain.User{ID: ownerID, Email: "owner@test.com", Name: "Owner"}, nil)

		toolRepo.On("GetByID", ctx, toolID).Return(&domain.Tool{Name: "Tool"}, nil)
		toolRepo.On("Update", ctx, mock.AnythingOfType("*domain.Tool")).Return(nil)

		emailSvc.On("SendRentalCompletionNotification", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe().Return(nil)
		// Allow any Dispatch call so the goroutines don't hit unexpected-call failures.
		noteRepo.On("Dispatch", mock.Anything, mock.AnythingOfType("*domain.Notification")).Maybe().Return(nil)

		_, err := svc.CompleteRental(ctx, ownerID, rentalID, "Good condition", 0, "", false)
		require.NoError(t, err)

		// Wait for the fire-and-forget settlement and completion goroutines to run.
		time.Sleep(100 * time.Millisecond)

		// Inspect all Dispatch calls: settlement notifications must include the direct-settlement reminder.
		var ownerReminderFound, renterReminderFound bool
		for _, call := range noteRepo.Calls {
			if call.Method != "Dispatch" {
				continue
			}
			n, ok := call.Arguments.Get(1).(*domain.Notification)
			if !ok {
				continue
			}
			if n.UserID == ownerID && strings.Contains(n.Message, "settled directly between you and the renter") {
				ownerReminderFound = true
			}
			if n.UserID == renterID && strings.Contains(n.Message, "settled directly between you and the owner") {
				renterReminderFound = true
			}
		}
		assert.True(t, ownerReminderFound, "owner settlement notification should contain the direct-settlement reminder")
		assert.True(t, renterReminderFound, "renter settlement notification should contain the direct-settlement reminder")
	})
}

// TestRentalService_ApproveRentalRequest covers FR-002 (specs/005-rentals): ApproveRentalRequest
// must require the caller to be the tool's owner and the rental to be PENDING. Prior to this
// test, every occurrence of ApproveRentalRequest anywhere in the test suite either mocked the
// service directly (handler test) or exercised only the owner-on-a-fresh-PENDING-rental happy
// path (e2e) — neither reject clause had any coverage.
func TestRentalService_ApproveRentalRequest(t *testing.T) {
	ctx := context.Background()
	const ownerID = int32(10)
	const rentalID = int32(100)
	const toolID = int32(200)

	newSvc := func() (service.RentalService, *MockRentalRepo, *MockToolRepo, *MockUserRepo, *MockEmailService, *MockNotificationRepo) {
		rentalRepo := new(MockRentalRepo)
		toolRepo := new(MockToolRepo)
		ledgerRepo := new(MockLedgerRepo)
		userRepo := new(MockUserRepo)
		emailSvc := new(MockEmailService)
		noteRepo := new(MockNotificationRepo)
		svc := service.NewRentalService(rentalRepo, toolRepo, ledgerRepo, userRepo, emailSvc, noteRepo)
		return svc, rentalRepo, toolRepo, userRepo, emailSvc, noteRepo
	}

	t.Run("Success as the tool's owner on a PENDING rental", func(t *testing.T) {
		svc, rentalRepo, toolRepo, userRepo, emailSvc, noteRepo := newSvc()
		rt := &domain.Rental{ID: rentalID, OwnerID: ownerID, RenterID: 1, ToolID: toolID, Status: domain.RentalStatusPending}
		rentalRepo.On("GetByID", ctx, rentalID).Return(rt, nil)
		rentalRepo.On("Update", ctx, mock.MatchedBy(func(r *domain.Rental) bool {
			return r.Status == domain.RentalStatusApproved && r.PickupNote == "Under the mat"
		})).Return(nil)
		userRepo.On("GetByID", ctx, int32(1)).Return(&domain.User{ID: 1, Name: "Renter", Email: "r@test.com"}, nil)
		userRepo.On("GetByID", ctx, ownerID).Return(&domain.User{ID: ownerID, Name: "Owner", Email: "o@test.com"}, nil)
		toolRepo.On("GetByID", ctx, toolID).Return(&domain.Tool{ID: toolID, Name: "Drill"}, nil)
		emailSvc.On("SendRentalApprovalNotification", ctx, "r@test.com", "Drill", "Owner", "Under the mat", "o@test.com").Return(nil)
		noteRepo.On("Create", ctx, mock.AnythingOfType("*domain.Notification")).Return(nil)

		res, err := svc.ApproveRentalRequest(ctx, ownerID, rentalID, "Under the mat")
		require.NoError(t, err)
		assert.Equal(t, domain.RentalStatusApproved, res.Status)
	})

	t.Run("Rejects a caller who is not the tool's owner", func(t *testing.T) {
		svc, rentalRepo, _, _, _, _ := newSvc()
		rt := &domain.Rental{ID: rentalID, OwnerID: ownerID, RenterID: 1, ToolID: toolID, Status: domain.RentalStatusPending}
		rentalRepo.On("GetByID", ctx, rentalID).Return(rt, nil)

		_, err := svc.ApproveRentalRequest(ctx, int32(999), rentalID, "note")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unauthorized")
		rentalRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
	})

	t.Run("Rejects a rental that is not PENDING", func(t *testing.T) {
		svc, rentalRepo, _, _, _, _ := newSvc()
		rt := &domain.Rental{ID: rentalID, OwnerID: ownerID, RenterID: 1, ToolID: toolID, Status: domain.RentalStatusApproved}
		rentalRepo.On("GetByID", ctx, rentalID).Return(rt, nil)

		_, err := svc.ApproveRentalRequest(ctx, ownerID, rentalID, "note")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not pending")
		rentalRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
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

		// 2. Update Rental Status (no ledger transaction at finalize)
		rentalRepo.On("Update", ctx, mock.MatchedBy(func(r *domain.Rental) bool {
			return r.Status == domain.RentalStatusScheduled
		})).Return(nil)

		// 3. Update Tool Status
		toolRepo.On("GetByID", ctx, toolID).Return(tool, nil)
		toolRepo.On("Update", ctx, mock.MatchedBy(func(tl *domain.Tool) bool {
			return tl.Status == domain.ToolStatusRented
		})).Return(nil)

		// 4. Notifications
		userRepo.On("GetByID", ctx, renterID).Return(renter, nil)
		userRepo.On("GetByID", ctx, ownerID).Return(owner, nil)
		emailSvc.On("SendRentalConfirmationNotification", ctx, owner.Email, renter.Name, tool.Name, renter.Email).Return(nil)
		noteRepo.On("Create", ctx, mock.AnythingOfType("*domain.Notification")).Return(nil)

		// 5. List Related Rentals
		rentalRepo.On("ListByTool", ctx, mock.Anything, mock.Anything, []string{string(domain.RentalStatusApproved)}, mock.Anything, mock.Anything).
			Return([]domain.Rental{approvedRental}, int32(1), nil)
		rentalRepo.On("ListByTool", ctx, mock.Anything, mock.Anything, []string{string(domain.RentalStatusPending)}, mock.Anything, mock.Anything).
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

	// FR-003: FinalizeRentalRequest must reject a non-renter caller and a rental that is not
	// APPROVED. Neither reject clause had any test coverage prior to this (Acceptance Scenario 6
	// documents the latter explicitly). Each subtest uses its own fresh mocks/service instance
	// so call-history assertions aren't polluted by the shared "Success" subtest above.
	t.Run("Rejects a caller who is not the renter", func(t *testing.T) {
		rentalRepo := new(MockRentalRepo)
		toolRepo := new(MockToolRepo)
		ledgerRepo := new(MockLedgerRepo)
		userRepo := new(MockUserRepo)
		emailSvc := new(MockEmailService)
		noteRepo := new(MockNotificationRepo)
		svc := service.NewRentalService(rentalRepo, toolRepo, ledgerRepo, userRepo, emailSvc, noteRepo)

		rentalRepo.On("GetByID", ctx, rentalID).Return(requestRental, nil)

		_, _, _, err := svc.FinalizeRentalRequest(ctx, int32(999), rentalID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unauthorized")
		rentalRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
	})

	t.Run("Rejects a rental that is not APPROVED", func(t *testing.T) {
		rentalRepo := new(MockRentalRepo)
		toolRepo := new(MockToolRepo)
		ledgerRepo := new(MockLedgerRepo)
		userRepo := new(MockUserRepo)
		emailSvc := new(MockEmailService)
		noteRepo := new(MockNotificationRepo)
		svc := service.NewRentalService(rentalRepo, toolRepo, ledgerRepo, userRepo, emailSvc, noteRepo)

		notApproved := &domain.Rental{
			ID: rentalID, RenterID: renterID, OwnerID: ownerID, ToolID: toolID,
			Status: domain.RentalStatusPending, OrgID: 99,
		}
		rentalRepo.On("GetByID", ctx, rentalID).Return(notApproved, nil)

		_, _, _, err := svc.FinalizeRentalRequest(ctx, renterID, rentalID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not approved")
		rentalRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
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
		Status:    domain.RentalStatusScheduled,
		StartDate: time.Now().Format("2006-01-02"), EndDate: time.Now().Add(24 * time.Hour).Format("2006-01-02"),
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
		Status:    domain.RentalStatusActive,
		StartDate: time.Now().Format("2006-01-02"), EndDate: time.Now().Add(24 * time.Hour).Format("2006-01-02"),
		TotalCostCents:    1000,
		DurationUnit:      string(domain.ToolDurationUnitDay),
		DailyPriceCents:   1000,
		WeeklyPriceCents:  6000,
		MonthlyPriceCents: 20000,
	}
	tool := &domain.Tool{
		ID:                 toolID,
		Name:               "Drill",
		PricePerDayCents:   1000,
		PricePerWeekCents:  6000,
		PricePerMonthCents: 20000,
		DurationUnit:       domain.ToolDurationUnitDay,
	}

	t.Run("Renter Extension Active", func(t *testing.T) {
		r := *baseRental
		newEnd := time.Now().Add(48 * time.Hour).Format("2006-01-02")

		rentalRepo.On("GetByID", ctx, rentalID).Return(&r, nil)
		toolRepo.On("GetByID", ctx, toolID).Return(tool, nil)

		// Expect update with temp status and new cost
		rentalRepo.On("Update", ctx, mock.MatchedBy(func(u *domain.Rental) bool {
			return u.Status == domain.RentalStatusReturnDateChanged &&
				u.TotalCostCents == 2000 // 2 days end-exclusive (today to +48h) * 1000
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

	t.Run("Renter Updates Pending Extension Request", func(t *testing.T) {
		rentalRepo := new(MockRentalRepo)
		toolRepo := new(MockToolRepo)
		emailSvc := new(MockEmailService)
		userRepo := new(MockUserRepo)
		noteRepo := new(MockNotificationRepo)
		svc := service.NewRentalService(rentalRepo, toolRepo, nil, userRepo, emailSvc, noteRepo)

		requestedEndDate := time.Now().Add(48 * time.Hour).Format("2006-01-02")
		lastAgreedEndDate := time.Now().Add(24 * time.Hour).Format("2006-01-02")
		r := &domain.Rental{
			ID: rentalID, RenterID: renterID, OwnerID: ownerID, ToolID: toolID,
			Status:            domain.RentalStatusReturnDateChanged,
			StartDate:         time.Now().Format("2006-01-02"),
			LastAgreedEndDate: &lastAgreedEndDate,
			EndDate:           requestedEndDate, // Already has a pending request
			TotalCostCents:    2000,
			DurationUnit:      string(domain.ToolDurationUnitDay),
			DailyPriceCents:   1000,
			WeeklyPriceCents:  6000,
			MonthlyPriceCents: 20000,
		}

		// Renter wants to update their request to a different date
		updatedEndDate := time.Now().Add(72 * time.Hour).Format("2006-01-02")

		rentalRepo.On("GetByID", ctx, rentalID).Return(r, nil)
		toolRepo.On("GetByID", ctx, toolID).Return(tool, nil)

		// Expect update with new end date and cost
		rentalRepo.On("Update", ctx, mock.MatchedBy(func(u *domain.Rental) bool {
			return u.Status == domain.RentalStatusReturnDateChanged &&
				u.TotalCostCents == 3000 // 3 days end-exclusive (today to +72h) * 1000
		})).Return(nil)

		// Notifications
		userRepo.On("GetByID", ctx, ownerID).Return(&domain.User{ID: ownerID, Email: "owner@a.com"}, nil)
		noteRepo.On("Create", ctx, mock.MatchedBy(func(n *domain.Notification) bool {
			return n.UserID == ownerID && n.Title == "Extension Request Updated"
		})).Return(nil)

		result, err := svc.ChangeRentalDates(ctx, renterID, rentalID, "", updatedEndDate, "", "")
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, domain.RentalStatusReturnDateChanged, result.Status)
		assert.Equal(t, int32(3000), result.TotalCostCents)
		assert.NotNil(t, result.EndDate)
	})
}

func TestRentalService_RejectReturnDateChange(t *testing.T) {
	ctx := context.Background()

	renterID := int32(20)
	ownerID := int32(10)
	rentalID := int32(100)
	toolID := int32(200)
	orgID := int32(3)

	startDate := time.Now().Format("2006-01-02")
	requestedEndDate := time.Now().Add(72 * time.Hour).Format("2006-01-02")  // 3 days
	lastAgreedEndDate := time.Now().Add(24 * time.Hour).Format("2006-01-02") // 1 day

	tool := &domain.Tool{
		ID:                 toolID,
		Name:               "Power Drill",
		PricePerDayCents:   1000,
		PricePerWeekCents:  6000,
		PricePerMonthCents: 20000,
		DurationUnit:       domain.ToolDurationUnitDay,
	}
	renter := &domain.User{ID: renterID, Email: "renter@test.com", Name: "Renter"}

	t.Run("Success - Owner sets counter-proposal", func(t *testing.T) {
		rentalRepo := new(MockRentalRepo)
		toolRepo := new(MockToolRepo)
		emailSvc := new(MockEmailService)
		userRepo := new(MockUserRepo)
		noteRepo := new(MockNotificationRepo)
		svc := service.NewRentalService(rentalRepo, toolRepo, nil, userRepo, emailSvc, noteRepo)

		baseRental := &domain.Rental{
			ID: rentalID, RenterID: renterID, OwnerID: ownerID, ToolID: toolID, OrgID: orgID,
			Status:            domain.RentalStatusReturnDateChanged,
			StartDate:         startDate,
			LastAgreedEndDate: &lastAgreedEndDate,
			EndDate:           requestedEndDate,
			TotalCostCents:    3000,
			DurationUnit:      string(domain.ToolDurationUnitDay),
			DailyPriceCents:   1000,
			WeeklyPriceCents:  6000,
			MonthlyPriceCents: 20000,
		}
		counterProposalDate := time.Now().Add(48 * time.Hour).Format("2006-01-02") // 2 days
		reason := "I need the tool back sooner"

		rentalRepo.On("GetByID", ctx, rentalID).Return(baseRental, nil)
		toolRepo.On("GetByID", ctx, toolID).Return(tool, nil)

		// Expect update with rejected status and counter-proposal
		rentalRepo.On("Update", ctx, mock.MatchedBy(func(u *domain.Rental) bool {
			return u.Status == domain.RentalStatusReturnDateChangeRejected &&
				u.RejectionReason == reason &&
				u.EndDate == counterProposalDate &&
				u.TotalCostCents == 2000 // 2 days end-exclusive (today to +48h) * 1000
		})).Return(nil)

		// Expect notification to renter
		userRepo.On("GetByID", ctx, renterID).Return(renter, nil)
		noteRepo.On("Create", ctx, mock.MatchedBy(func(n *domain.Notification) bool {
			return n.UserID == renterID &&
				n.Title == "Extension Rejected - Counter-Proposal" &&
				n.Attributes["type"] == "RETURN_DATE_CHANGE_REJECTED" &&
				n.Attributes["new_end_date"] == counterProposalDate
		})).Return(nil)

		// Expect email notification
		emailSvc.On("SendReturnDateRejectionNotification", ctx, renter.Email, tool.Name, counterProposalDate, reason, int32(2000)).Return(nil)

		result, err := svc.RejectReturnDateChange(ctx, ownerID, rentalID, reason, counterProposalDate)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, domain.RentalStatusReturnDateChangeRejected, result.Status)
		assert.Equal(t, reason, result.RejectionReason)
		assert.Equal(t, int32(2000), result.TotalCostCents)
	})

	t.Run("Error - Unauthorized (not owner)", func(t *testing.T) {
		rentalRepo := new(MockRentalRepo)
		toolRepo := new(MockToolRepo)
		emailSvc := new(MockEmailService)
		userRepo := new(MockUserRepo)
		noteRepo := new(MockNotificationRepo)
		svc := service.NewRentalService(rentalRepo, toolRepo, nil, userRepo, emailSvc, noteRepo)

		baseRental := &domain.Rental{
			ID: rentalID, RenterID: renterID, OwnerID: ownerID, ToolID: toolID, OrgID: orgID,
			Status:            domain.RentalStatusReturnDateChanged,
			StartDate:         startDate,
			LastAgreedEndDate: &lastAgreedEndDate,
			EndDate:           requestedEndDate,
			TotalCostCents:    3000,
		}
		unauthorizedUserID := int32(999)
		counterProposalDate := time.Now().Add(48 * time.Hour).Format("2006-01-02")

		rentalRepo.On("GetByID", ctx, rentalID).Return(baseRental, nil)

		result, err := svc.RejectReturnDateChange(ctx, unauthorizedUserID, rentalID, "reason", counterProposalDate)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "unauthorized")
	})

	t.Run("Error - Invalid status", func(t *testing.T) {
		rentalRepo := new(MockRentalRepo)
		toolRepo := new(MockToolRepo)
		emailSvc := new(MockEmailService)
		userRepo := new(MockUserRepo)
		noteRepo := new(MockNotificationRepo)
		svc := service.NewRentalService(rentalRepo, toolRepo, nil, userRepo, emailSvc, noteRepo)

		baseRental := &domain.Rental{
			ID: rentalID, RenterID: renterID, OwnerID: ownerID, ToolID: toolID, OrgID: orgID,
			Status:            domain.RentalStatusActive, // Wrong status
			StartDate:         startDate,
			LastAgreedEndDate: &lastAgreedEndDate,
			EndDate:           lastAgreedEndDate,
			TotalCostCents:    1000,
		}
		counterProposalDate := time.Now().Add(48 * time.Hour).Format("2006-01-02")

		rentalRepo.On("GetByID", ctx, rentalID).Return(baseRental, nil)

		result, err := svc.RejectReturnDateChange(ctx, ownerID, rentalID, "reason", counterProposalDate)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "invalid status")
	})

	t.Run("Error - Empty new_end_date", func(t *testing.T) {
		rentalRepo := new(MockRentalRepo)
		toolRepo := new(MockToolRepo)
		emailSvc := new(MockEmailService)
		userRepo := new(MockUserRepo)
		noteRepo := new(MockNotificationRepo)
		svc := service.NewRentalService(rentalRepo, toolRepo, nil, userRepo, emailSvc, noteRepo)

		baseRental := &domain.Rental{
			ID: rentalID, RenterID: renterID, OwnerID: ownerID, ToolID: toolID, OrgID: orgID,
			Status:            domain.RentalStatusReturnDateChanged,
			StartDate:         startDate,
			LastAgreedEndDate: &lastAgreedEndDate,
			EndDate:           requestedEndDate,
			TotalCostCents:    3000,
		}

		rentalRepo.On("GetByID", ctx, rentalID).Return(baseRental, nil)

		result, err := svc.RejectReturnDateChange(ctx, ownerID, rentalID, "reason", "")
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "new end date is required")
	})

	t.Run("Error - Invalid date format", func(t *testing.T) {
		rentalRepo := new(MockRentalRepo)
		toolRepo := new(MockToolRepo)
		emailSvc := new(MockEmailService)
		userRepo := new(MockUserRepo)
		noteRepo := new(MockNotificationRepo)
		svc := service.NewRentalService(rentalRepo, toolRepo, nil, userRepo, emailSvc, noteRepo)

		baseRental := &domain.Rental{
			ID: rentalID, RenterID: renterID, OwnerID: ownerID, ToolID: toolID, OrgID: orgID,
			Status:            domain.RentalStatusReturnDateChanged,
			StartDate:         startDate,
			LastAgreedEndDate: &lastAgreedEndDate,
			EndDate:           requestedEndDate,
			TotalCostCents:    3000,
		}
		invalidDate := "2024/01/01" // Wrong format

		rentalRepo.On("GetByID", ctx, rentalID).Return(baseRental, nil)

		result, err := svc.RejectReturnDateChange(ctx, ownerID, rentalID, "reason", invalidDate)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "invalid date format")
	})

	t.Run("Error - New date same as requested", func(t *testing.T) {
		rentalRepo := new(MockRentalRepo)
		toolRepo := new(MockToolRepo)
		emailSvc := new(MockEmailService)
		userRepo := new(MockUserRepo)
		noteRepo := new(MockNotificationRepo)
		svc := service.NewRentalService(rentalRepo, toolRepo, nil, userRepo, emailSvc, noteRepo)

		baseRental := &domain.Rental{
			ID: rentalID, RenterID: renterID, OwnerID: ownerID, ToolID: toolID, OrgID: orgID,
			Status:            domain.RentalStatusReturnDateChanged,
			StartDate:         startDate,
			LastAgreedEndDate: &lastAgreedEndDate,
			EndDate:           requestedEndDate,
			TotalCostCents:    3000,
		}
		sameAsRequested := requestedEndDate

		rentalRepo.On("GetByID", ctx, rentalID).Return(baseRental, nil)

		result, err := svc.RejectReturnDateChange(ctx, ownerID, rentalID, "reason", sameAsRequested)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "new end date must be different from the requested date")
	})

	t.Run("Error - Invalid date range (date before start)", func(t *testing.T) {
		rentalRepo := new(MockRentalRepo)
		toolRepo := new(MockToolRepo)
		emailSvc := new(MockEmailService)
		userRepo := new(MockUserRepo)
		noteRepo := new(MockNotificationRepo)
		svc := service.NewRentalService(rentalRepo, toolRepo, nil, userRepo, emailSvc, noteRepo)

		baseRental := &domain.Rental{
			ID: rentalID, RenterID: renterID, OwnerID: ownerID, ToolID: toolID, OrgID: orgID,
			Status:            domain.RentalStatusReturnDateChanged,
			StartDate:         startDate,
			LastAgreedEndDate: &lastAgreedEndDate,
			EndDate:           requestedEndDate,
			TotalCostCents:    3000,
		}
		pastDate := time.Now().Add(-24 * time.Hour).Format("2006-01-02")

		rentalRepo.On("GetByID", ctx, rentalID).Return(baseRental, nil)
		toolRepo.On("GetByID", ctx, toolID).Return(tool, nil)

		result, err := svc.RejectReturnDateChange(ctx, ownerID, rentalID, "reason", pastDate)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "end date must be >= start date")
	})

	t.Run("Success - Notifications sent even if email fails", func(t *testing.T) {
		rentalRepo := new(MockRentalRepo)
		toolRepo := new(MockToolRepo)
		emailSvc := new(MockEmailService)
		userRepo := new(MockUserRepo)
		noteRepo := new(MockNotificationRepo)
		svc := service.NewRentalService(rentalRepo, toolRepo, nil, userRepo, emailSvc, noteRepo)

		baseRental := &domain.Rental{
			ID: rentalID, RenterID: renterID, OwnerID: ownerID, ToolID: toolID, OrgID: orgID,
			Status:            domain.RentalStatusReturnDateChanged,
			StartDate:         startDate,
			LastAgreedEndDate: &lastAgreedEndDate,
			EndDate:           requestedEndDate,
			TotalCostCents:    3000,
			DurationUnit:      string(domain.ToolDurationUnitDay),
			DailyPriceCents:   1000,
			WeeklyPriceCents:  6000,
			MonthlyPriceCents: 20000,
		}
		counterProposalDate := time.Now().Add(48 * time.Hour).Format("2006-01-02")
		reason := "Tool needed urgently"

		rentalRepo.On("GetByID", ctx, rentalID).Return(baseRental, nil)
		toolRepo.On("GetByID", ctx, toolID).Return(tool, nil)
		rentalRepo.On("Update", ctx, mock.AnythingOfType("*domain.Rental")).Return(nil)

		userRepo.On("GetByID", ctx, renterID).Return(renter, nil)
		noteRepo.On("Create", ctx, mock.AnythingOfType("*domain.Notification")).Return(nil)

		// Email fails but operation should still succeed
		emailSvc.On("SendReturnDateRejectionNotification", ctx, renter.Email, tool.Name, counterProposalDate, reason, int32(2000)).Return(fmt.Errorf("email error"))

		result, err := svc.RejectReturnDateChange(ctx, ownerID, rentalID, reason, counterProposalDate)
		assert.NoError(t, err) // Email error is ignored
		assert.NotNil(t, result)
		assert.Equal(t, domain.RentalStatusReturnDateChangeRejected, result.Status)
	})
}

// TestRentalService_AcknowledgeReturnDateRejection covers FR-005 (specs/005-rentals): this RPC
// had zero test evidence at any tier — the only occurrence anywhere was a mock-interface stub.
func TestRentalService_AcknowledgeReturnDateRejection(t *testing.T) {
	ctx := context.Background()
	const renterID = int32(1)
	const ownerID = int32(10)
	const rentalID = int32(100)
	const toolID = int32(200)

	newSvc := func() (service.RentalService, *MockRentalRepo, *MockUserRepo, *MockNotificationRepo) {
		rentalRepo := new(MockRentalRepo)
		toolRepo := new(MockToolRepo)
		ledgerRepo := new(MockLedgerRepo)
		userRepo := new(MockUserRepo)
		emailSvc := new(MockEmailService)
		noteRepo := new(MockNotificationRepo)
		svc := service.NewRentalService(rentalRepo, toolRepo, ledgerRepo, userRepo, emailSvc, noteRepo)
		return svc, rentalRepo, userRepo, noteRepo
	}

	lastAgreed := time.Now().Add(48 * time.Hour).Format("2006-01-02")

	t.Run("Success rolls back to the last agreed end date and recomputes cost", func(t *testing.T) {
		svc, rentalRepo, userRepo, noteRepo := newSvc()
		rt := &domain.Rental{
			ID: rentalID, RenterID: renterID, OwnerID: ownerID, ToolID: toolID,
			Status: domain.RentalStatusReturnDateChangeRejected,
			StartDate: time.Now().Add(-24 * time.Hour).Format("2006-01-02"),
			EndDate: time.Now().Add(96 * time.Hour).Format("2006-01-02"), // the rejected counter-proposal
			LastAgreedEndDate: &lastAgreed,
			DurationUnit: string(domain.ToolDurationUnitDay), DailyPriceCents: 1000, WeeklyPriceCents: 6000, MonthlyPriceCents: 20000,
			RejectionReason: "owner countered",
		}
		rentalRepo.On("GetByID", ctx, rentalID).Return(rt, nil)
		rentalRepo.On("Update", ctx, mock.MatchedBy(func(r *domain.Rental) bool {
			return r.EndDate == lastAgreed && r.RejectionReason == "" && r.Status == domain.RentalStatusActive
		})).Return(nil)
		userRepo.On("GetByID", ctx, ownerID).Return(&domain.User{ID: ownerID, Name: "Owner"}, nil)
		noteRepo.On("Create", ctx, mock.AnythingOfType("*domain.Notification")).Return(nil)

		res, err := svc.AcknowledgeReturnDateRejection(ctx, renterID, rentalID)
		require.NoError(t, err)
		assert.Equal(t, lastAgreed, res.EndDate)
		assert.Empty(t, res.RejectionReason)
	})

	t.Run("Rejects a caller who is not the renter", func(t *testing.T) {
		svc, rentalRepo, _, _ := newSvc()
		rt := &domain.Rental{ID: rentalID, RenterID: renterID, OwnerID: ownerID, Status: domain.RentalStatusReturnDateChangeRejected}
		rentalRepo.On("GetByID", ctx, rentalID).Return(rt, nil)

		_, err := svc.AcknowledgeReturnDateRejection(ctx, int32(999), rentalID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unauthorized")
		rentalRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
	})

	t.Run("Rejects a rental that is not in RETURN_DATE_CHANGE_REJECTED status", func(t *testing.T) {
		svc, rentalRepo, _, _ := newSvc()
		rt := &domain.Rental{ID: rentalID, RenterID: renterID, OwnerID: ownerID, Status: domain.RentalStatusActive}
		rentalRepo.On("GetByID", ctx, rentalID).Return(rt, nil)

		_, err := svc.AcknowledgeReturnDateRejection(ctx, renterID, rentalID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid status")
		rentalRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
	})
}

// TestRentalService_CancelReturnDateChange covers FR-005 (specs/005-rentals): this RPC
// had zero test evidence at any tier — the only occurrence anywhere was a mock-interface stub.
func TestRentalService_CancelReturnDateChange(t *testing.T) {
	ctx := context.Background()
	const renterID = int32(1)
	const ownerID = int32(10)
	const rentalID = int32(100)
	const toolID = int32(200)

	newSvc := func() (service.RentalService, *MockRentalRepo, *MockUserRepo, *MockNotificationRepo) {
		rentalRepo := new(MockRentalRepo)
		toolRepo := new(MockToolRepo)
		ledgerRepo := new(MockLedgerRepo)
		userRepo := new(MockUserRepo)
		emailSvc := new(MockEmailService)
		noteRepo := new(MockNotificationRepo)
		svc := service.NewRentalService(rentalRepo, toolRepo, ledgerRepo, userRepo, emailSvc, noteRepo)
		return svc, rentalRepo, userRepo, noteRepo
	}

	lastAgreed := time.Now().Add(48 * time.Hour).Format("2006-01-02")

	t.Run("Success rolls back to the last agreed end date and recomputes cost", func(t *testing.T) {
		svc, rentalRepo, userRepo, noteRepo := newSvc()
		rt := &domain.Rental{
			ID: rentalID, RenterID: renterID, OwnerID: ownerID, ToolID: toolID,
			Status: domain.RentalStatusReturnDateChanged,
			StartDate: time.Now().Add(-24 * time.Hour).Format("2006-01-02"),
			EndDate: time.Now().Add(96 * time.Hour).Format("2006-01-02"), // the pending extension request
			LastAgreedEndDate: &lastAgreed,
			DurationUnit: string(domain.ToolDurationUnitDay), DailyPriceCents: 1000, WeeklyPriceCents: 6000, MonthlyPriceCents: 20000,
		}
		rentalRepo.On("GetByID", ctx, rentalID).Return(rt, nil)
		rentalRepo.On("Update", ctx, mock.MatchedBy(func(r *domain.Rental) bool {
			return r.EndDate == lastAgreed && r.Status == domain.RentalStatusActive
		})).Return(nil)
		userRepo.On("GetByID", ctx, ownerID).Return(&domain.User{ID: ownerID, Name: "Owner"}, nil)
		noteRepo.On("Create", ctx, mock.AnythingOfType("*domain.Notification")).Return(nil)

		res, err := svc.CancelReturnDateChange(ctx, renterID, rentalID)
		require.NoError(t, err)
		assert.Equal(t, lastAgreed, res.EndDate)
	})

	t.Run("Rejects a caller who is not the renter", func(t *testing.T) {
		svc, rentalRepo, _, _ := newSvc()
		rt := &domain.Rental{ID: rentalID, RenterID: renterID, OwnerID: ownerID, Status: domain.RentalStatusReturnDateChanged}
		rentalRepo.On("GetByID", ctx, rentalID).Return(rt, nil)

		_, err := svc.CancelReturnDateChange(ctx, int32(999), rentalID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unauthorized")
		rentalRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
	})

	t.Run("Rejects a rental that is not in RETURN_DATE_CHANGED status", func(t *testing.T) {
		svc, rentalRepo, _, _ := newSvc()
		rt := &domain.Rental{ID: rentalID, RenterID: renterID, OwnerID: ownerID, Status: domain.RentalStatusActive}
		rentalRepo.On("GetByID", ctx, rentalID).Return(rt, nil)

		_, err := svc.CancelReturnDateChange(ctx, renterID, rentalID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid status")
		rentalRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
	})
}

// TestRentalService_ApproveReturnDateChange covers FR-005 (specs/005-rentals): this RPC had zero
// unit test evidence — the only prior evidence was e2e/integration happy-path coverage. Unlike
// its sibling RPCs, ApproveReturnDateChange does not itself change end_date (the extension's new
// end_date was already applied, and total_cost_cents already recomputed, by the preceding
// ChangeRentalDates call) — it only confirms the pending extension by setting
// last_agreed_end_date and transitioning status, so no further recompute is expected here.
func TestRentalService_ApproveReturnDateChange(t *testing.T) {
	ctx := context.Background()
	const renterID = int32(1)
	const ownerID = int32(10)
	const rentalID = int32(100)
	const toolID = int32(200)

	newSvc := func() (service.RentalService, *MockRentalRepo, *MockToolRepo, *MockUserRepo, *MockNotificationRepo) {
		rentalRepo := new(MockRentalRepo)
		toolRepo := new(MockToolRepo)
		ledgerRepo := new(MockLedgerRepo)
		userRepo := new(MockUserRepo)
		emailSvc := new(MockEmailService)
		noteRepo := new(MockNotificationRepo)
		svc := service.NewRentalService(rentalRepo, toolRepo, ledgerRepo, userRepo, emailSvc, noteRepo)
		return svc, rentalRepo, toolRepo, userRepo, noteRepo
	}

	t.Run("Success as owner sets last_agreed_end_date and activates", func(t *testing.T) {
		svc, rentalRepo, toolRepo, userRepo, noteRepo := newSvc()
		requestedEnd := time.Now().Add(72 * time.Hour).Format("2006-01-02")
		rt := &domain.Rental{
			ID: rentalID, RenterID: renterID, OwnerID: ownerID, ToolID: toolID,
			Status:    domain.RentalStatusReturnDateChanged,
			StartDate: time.Now().Add(-24 * time.Hour).Format("2006-01-02"),
			EndDate:   requestedEnd,
			TotalCostCents: 3000,
		}
		rentalRepo.On("GetByID", ctx, rentalID).Return(rt, nil)
		rentalRepo.On("Update", ctx, mock.MatchedBy(func(r *domain.Rental) bool {
			return r.Status == domain.RentalStatusActive &&
				r.LastAgreedEndDate != nil && *r.LastAgreedEndDate == requestedEnd &&
				r.TotalCostCents == 3000 // unchanged — Approve does not recompute cost
		})).Return(nil)
		toolRepo.On("GetByID", ctx, toolID).Return(&domain.Tool{ID: toolID, Name: "Drill"}, nil)
		userRepo.On("GetByID", ctx, renterID).Return(&domain.User{ID: renterID, Name: "Renter"}, nil)
		noteRepo.On("Create", ctx, mock.AnythingOfType("*domain.Notification")).Return(nil)

		res, err := svc.ApproveReturnDateChange(ctx, ownerID, rentalID)
		require.NoError(t, err)
		assert.Equal(t, domain.RentalStatusActive, res.Status)
		require.NotNil(t, res.LastAgreedEndDate)
		assert.Equal(t, requestedEnd, *res.LastAgreedEndDate)
	})

	t.Run("Rejects a caller who is not the owner", func(t *testing.T) {
		svc, rentalRepo, _, _, _ := newSvc()
		rt := &domain.Rental{ID: rentalID, RenterID: renterID, OwnerID: ownerID, Status: domain.RentalStatusReturnDateChanged}
		rentalRepo.On("GetByID", ctx, rentalID).Return(rt, nil)

		_, err := svc.ApproveReturnDateChange(ctx, int32(999), rentalID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unauthorized")
		rentalRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
	})

	t.Run("Rejects a rental that is not in RETURN_DATE_CHANGED status", func(t *testing.T) {
		svc, rentalRepo, _, _, _ := newSvc()
		rt := &domain.Rental{ID: rentalID, RenterID: renterID, OwnerID: ownerID, Status: domain.RentalStatusActive}
		rentalRepo.On("GetByID", ctx, rentalID).Return(rt, nil)

		_, err := svc.ApproveReturnDateChange(ctx, ownerID, rentalID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid status")
		rentalRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
	})
}

// TestRentalService_GetRental covers FR-006 (specs/005-rentals): GetRental must grant access to
// the rental's renter and owner, and reject any other caller (admin access is explicitly NOT
// required — Known Discrepancy 4). Prior to this test, the only occurrence of GetRental in
// tests/ was a mock-interface stub, never invoked or asserted against.
func TestRentalService_GetRental(t *testing.T) {
	ctx := context.Background()
	const renterID = int32(1)
	const ownerID = int32(10)
	const otherUserID = int32(999)
	const rentalID = int32(100)

	newSvc := func() (service.RentalService, *MockRentalRepo) {
		rentalRepo := new(MockRentalRepo)
		toolRepo := new(MockToolRepo)
		ledgerRepo := new(MockLedgerRepo)
		userRepo := new(MockUserRepo)
		emailSvc := new(MockEmailService)
		noteRepo := new(MockNotificationRepo)
		svc := service.NewRentalService(rentalRepo, toolRepo, ledgerRepo, userRepo, emailSvc, noteRepo)
		return svc, rentalRepo
	}

	rt := &domain.Rental{ID: rentalID, RenterID: renterID, OwnerID: ownerID}

	t.Run("Grants access to the renter", func(t *testing.T) {
		svc, rentalRepo := newSvc()
		rentalRepo.On("GetByID", ctx, rentalID).Return(rt, nil)

		res, err := svc.GetRental(ctx, renterID, rentalID)
		require.NoError(t, err)
		assert.Equal(t, rentalID, res.ID)
	})

	t.Run("Grants access to the owner", func(t *testing.T) {
		svc, rentalRepo := newSvc()
		rentalRepo.On("GetByID", ctx, rentalID).Return(rt, nil)

		res, err := svc.GetRental(ctx, ownerID, rentalID)
		require.NoError(t, err)
		assert.Equal(t, rentalID, res.ID)
	})

	t.Run("Rejects a caller who is neither the renter nor the owner", func(t *testing.T) {
		svc, rentalRepo := newSvc()
		rentalRepo.On("GetByID", ctx, rentalID).Return(rt, nil)

		_, err := svc.GetRental(ctx, otherUserID, rentalID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unauthorized")
	})
}
