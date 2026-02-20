package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/repository/postgres"
	"ubertool-backend-trusted/internal/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Mocks for Integration Test
type MockEmailService struct {
	mock.Mock
}

func (m *MockEmailService) SendRentalRequestNotification(ctx context.Context, ownerEmail, renterName, toolName, renterEmail string) error {
	return nil
}
func (m *MockEmailService) SendRentalConfirmationNotification(ctx context.Context, ownerEmail, renterName, toolName, renterEmail string) error {
	return nil
}
func (m *MockEmailService) SendRentalPickupNotification(ctx context.Context, email, name, toolName, startDate, endDate string) error {
	return nil
}
func (m *MockEmailService) SendRentalCompletionNotification(ctx context.Context, email, role, toolName string, amount int32) error {
	return nil
}
func (m *MockEmailService) SendRentalCancellationNotification(ctx context.Context, ownerEmail, renterName, toolName, reason, ccEmail string) error {
	return nil
}
func (m *MockEmailService) SendAdminNotification(ctx context.Context, adminEmail, subject, message string) error {
	return nil
}
func (m *MockEmailService) SendInvitation(ctx context.Context, email, name, token string, orgName string, ccEmail string) error {
	return nil
}
func (m *MockEmailService) SendAccountStatusNotification(ctx context.Context, email, name, orgName, status, reason string) error {
	return nil
}
func (m *MockEmailService) SendRentalApprovalNotification(ctx context.Context, renterEmail, toolName, ownerName, pickupNote string, ccEmail string) error {
	return nil
}
func (m *MockEmailService) SendRentalRejectionNotification(ctx context.Context, renterEmail, toolName, ownerName string, ccEmail string) error {
	return nil
}
func (m *MockEmailService) SendReturnDateRejectionNotification(ctx context.Context, renterEmail, toolName, newEndDate, reason string, totalCostCents int32) error {
	return nil
}

// Bill Split Notifications
func (m *MockEmailService) SendBillPaymentNotice(ctx context.Context, debtorEmail, debtorName, creditorName string, amountCents int32, settlementMonth string, orgName string) error {
	return nil
}
func (m *MockEmailService) SendBillPaymentAcknowledgment(ctx context.Context, creditorEmail, creditorName, debtorName string, amountCents int32, settlementMonth string, orgName string) error {
	return nil
}
func (m *MockEmailService) SendBillReceiptConfirmation(ctx context.Context, debtorEmail, debtorName, creditorName string, amountCents int32, settlementMonth string, orgName string) error {
	return nil
}
func (m *MockEmailService) SendBillDisputeNotification(ctx context.Context, email, name, otherPartyName string, amountCents int32, reason string, orgName string) error {
	return nil
}
func (m *MockEmailService) SendBillDisputeResolutionNotification(ctx context.Context, email, name string, amountCents int32, resolution, notes string, orgName string) error {
	return nil
}

type MockNotificationRepo struct {
	mock.Mock
}

func (m *MockNotificationRepo) Create(ctx context.Context, n *domain.Notification) error {
	return nil
}
func (m *MockNotificationRepo) List(ctx context.Context, userID int32, limit, offset int32) ([]domain.Notification, int32, error) {
	return nil, 0, nil
}
func (m *MockNotificationRepo) MarkAsRead(ctx context.Context, id, userID int32) error {
	return nil
}

func TestRentalAndLedger_Integration(t *testing.T) {
	db := prepareDB(t)
	defer db.Close()

	userRepo := postgres.NewUserRepository(db)
	toolRepo := postgres.NewToolRepository(db)
	rentalRepo := postgres.NewRentalRepository(db)
	ledgerRepo := postgres.NewLedgerRepository(db)
	ctx := context.Background()

	// 1. Setup Data
	orgName := fmt.Sprintf("Org-%d", time.Now().UnixNano())
	db.Exec("INSERT INTO orgs (name, metro) VALUES ($1, 'San Jose')", orgName)
	var orgID int32
	db.QueryRow("SELECT id FROM orgs WHERE name = $1", orgName).Scan(&orgID)

	owner := &domain.User{
		Email:        fmt.Sprintf("owner-%d@t.com", time.Now().UnixNano()),
		PhoneNumber:  fmt.Sprintf("p1-%d", time.Now().UnixNano()),
		PasswordHash: "h", Name: "Owner",
	}
	userRepo.Create(ctx, owner)

	renter := &domain.User{
		Email:        fmt.Sprintf("renter-%d@t.com", time.Now().UnixNano()),
		PhoneNumber:  fmt.Sprintf("p2-%d", time.Now().UnixNano()),
		PasswordHash: "h", Name: "Renter",
	}
	userRepo.Create(ctx, renter)

	// Set initial balance for renter
	userRepo.AddUserToOrg(ctx, &domain.UserOrg{
		UserID: renter.ID, OrgID: orgID, BalanceCents: 5000, Status: domain.UserOrgStatusActive, Role: domain.UserOrgRoleMember,
	})
	userRepo.AddUserToOrg(ctx, &domain.UserOrg{
		UserID: owner.ID, OrgID: orgID, BalanceCents: 0, Status: domain.UserOrgStatusActive, Role: domain.UserOrgRoleMember,
	})

	tool := &domain.Tool{
		OwnerID: owner.ID, Name: "Drill", PricePerDayCents: 1000, PricePerWeekCents: 6000, PricePerMonthCents: 20000, DurationUnit: domain.ToolDurationUnitDay, Condition: domain.ToolConditionExcellent, Metro: "San Jose", Status: domain.ToolStatusAvailable,
	}
	toolRepo.Create(ctx, tool)

	t.Run("Full Lifecycle", func(t *testing.T) {
		// 2. Create Rental Request
		rental := &domain.Rental{
			OrgID: orgID, ToolID: tool.ID, RenterID: renter.ID, OwnerID: owner.ID,
			StartDate: time.Now().Format("2006-01-02"), EndDate: time.Now().Add(24 * time.Hour).Format("2006-01-02"),
			TotalCostCents: 1000, Status: domain.RentalStatusPending,
		}
		err := rentalRepo.Create(ctx, rental)
		assert.NoError(t, err)

		// 3. Update Status (Simulation of approval/completion)
		rental.Status = domain.RentalStatusCompleted
		err = rentalRepo.Update(ctx, rental)
		assert.NoError(t, err)

		// 4. Create Transactions
		tx1 := &domain.LedgerTransaction{
			OrgID: orgID, UserID: renter.ID, Amount: -1000, Type: domain.TransactionTypeRentalDebit, RelatedRentalID: &rental.ID, Description: "Rental",
		}
		err = ledgerRepo.CreateTransaction(ctx, tx1)
		assert.NoError(t, err)

		tx2 := &domain.LedgerTransaction{
			OrgID: orgID, UserID: owner.ID, Amount: 1000, Type: domain.TransactionTypeLendingCredit, RelatedRentalID: &rental.ID, Description: "Lending",
		}
		err = ledgerRepo.CreateTransaction(ctx, tx2)
		assert.NoError(t, err)

		// 5. Verify Transactions
		txs, total, err := ledgerRepo.ListTransactions(ctx, renter.ID, orgID, 1, 10)
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, total, int32(1))
		assert.Equal(t, int32(-1000), txs[0].Amount)
	})
}

func TestRentalDateChange_Integration(t *testing.T) {
	db := prepareDB(t)
	defer db.Close()

	// 1. Setup Service and Repos
	userRepo := postgres.NewUserRepository(db)
	toolRepo := postgres.NewToolRepository(db)
	rentalRepo := postgres.NewRentalRepository(db)
	ledgerRepo := postgres.NewLedgerRepository(db)
	emailSvc := new(MockEmailService)
	noteRepo := new(MockNotificationRepo)

	svc := service.NewRentalService(rentalRepo, toolRepo, ledgerRepo, userRepo, emailSvc, noteRepo)
	ctx := context.Background()

	// 2. Setup Data
	orgName := fmt.Sprintf("Org-Date-%d", time.Now().UnixNano())
	_, err := db.Exec("INSERT INTO orgs (name, metro) VALUES ($1, 'San Jose')", orgName)
	require.NoError(t, err)
	var orgID int32
	err = db.QueryRow("SELECT id FROM orgs WHERE name = $1", orgName).Scan(&orgID)
	require.NoError(t, err)

	owner := &domain.User{
		Email:        fmt.Sprintf("owner-d-%d@t.com", time.Now().UnixNano()),
		PhoneNumber:  fmt.Sprintf("+1555%d", time.Now().UnixNano()%10000000),
		PasswordHash: "h", Name: "Owner",
	}
	err = userRepo.Create(ctx, owner)
	require.NoError(t, err)
	require.NotZero(t, owner.ID, "Owner ID should be set after creation")

	renter := &domain.User{
		Email:        fmt.Sprintf("renter-d-%d@t.com", time.Now().UnixNano()),
		PhoneNumber:  fmt.Sprintf("+1666%d", time.Now().UnixNano()%10000000),
		PasswordHash: "h", Name: "Renter",
	}
	err = userRepo.Create(ctx, renter)
	require.NoError(t, err)
	require.NotZero(t, renter.ID, "Renter ID should be set after creation")

	err = userRepo.AddUserToOrg(ctx, &domain.UserOrg{UserID: renter.ID, OrgID: orgID, BalanceCents: 50000, Status: domain.UserOrgStatusActive})
	require.NoError(t, err)
	err = userRepo.AddUserToOrg(ctx, &domain.UserOrg{UserID: owner.ID, OrgID: orgID, BalanceCents: 0, Status: domain.UserOrgStatusActive})
	require.NoError(t, err)

	tool := &domain.Tool{
		OwnerID: owner.ID, Name: "Drill", PricePerDayCents: 1000, PricePerWeekCents: 6000, PricePerMonthCents: 20000, DurationUnit: domain.ToolDurationUnitDay, Metro: "San Jose", Status: domain.ToolStatusAvailable,
	}
	err = toolRepo.Create(ctx, tool)
	require.NoError(t, err)
	require.NotZero(t, tool.ID, "Tool ID should be set after creation")

	t.Run("Activate -> Extend -> Approve", func(t *testing.T) {
		// Use date-only format to match service parsing
		startDate := "2025-01-01"
		endDate := "2025-01-02"

		// Create Scheduled Rental manually
		rental := &domain.Rental{
			OrgID: orgID, ToolID: tool.ID, RenterID: renter.ID, OwnerID: owner.ID,
			StartDate: startDate, EndDate: endDate,
			TotalCostCents:       1000,
			Status:               domain.RentalStatusScheduled,
			DurationUnit:         string(tool.DurationUnit),
			DailyPriceCents:      tool.PricePerDayCents,
			WeeklyPriceCents:     tool.PricePerWeekCents,
			MonthlyPriceCents:    tool.PricePerMonthCents,
			ReplacementCostCents: tool.ReplacementCostCents,
		}
		err = rentalRepo.Create(ctx, rental)
		assert.NoError(t, err)
		require.NotZero(t, rental.ID)

		// Force status to SCHEDULED to ensure setup is correct
		_, err = db.Exec("UPDATE rentals SET status = 'SCHEDULED' WHERE id = $1", rental.ID)
		require.NoError(t, err)

		// Debug: check Tool Price
		var price int32
		err = db.QueryRow("SELECT price_per_day_cents FROM tools WHERE id = $1", tool.ID).Scan(&price)
		require.NoError(t, err)
		t.Logf("Tool Price from DB: %d", price)

		// 1. Activate
		actRental, err := svc.ActivateRental(ctx, owner.ID, rental.ID)
		if err != nil {
			t.Logf("Activate Error: %v", err)
		}
		require.NoError(t, err)
		require.NotNil(t, actRental)
		assert.Equal(t, domain.RentalStatusActive, actRental.Status)

		// 2. Change Dates (Extend by 1 day: 2025-01-02 -> 2025-01-03)
		newEnd := "2025-01-03"
		t.Logf("Calling ChangeRentalDates: rentalID=%d, renterID=%d, newEnd=%s", rental.ID, renter.ID, newEnd)
		chgRental, err := svc.ChangeRentalDates(ctx, renter.ID, rental.ID, "", newEnd, "", "")
		if err != nil {
			t.Logf("ChangeDates Error: %v", err)
			t.FailNow()
		}
		require.NoError(t, err)
		require.NotNil(t, chgRental)
		t.Logf("ChangeRental Result: Status=%s, Cost=%d, EndDate=%v, LastAgreedEndDate=%v",
			chgRental.Status, chgRental.TotalCostCents, chgRental.EndDate, chgRental.LastAgreedEndDate)
		assert.Equal(t, domain.RentalStatusReturnDateChanged, chgRental.Status)
		// Cost should be 2 days (2025-01-01 to 2025-01-03 end-exclusive) * 1000 = 2000
		assert.Equal(t, int32(2000), chgRental.TotalCostCents)

		// 3. Approve Extension
		appRental, err := svc.ApproveReturnDateChange(ctx, owner.ID, rental.ID)
		if err != nil {
			t.Logf("Approve Error: %v", err)
		}
		require.NoError(t, err)
		require.NotNil(t, appRental)
		// Note: Status becomes OVERDUE because the rental dates are in the past
		assert.Equal(t, domain.RentalStatusOverdue, appRental.Status)
		assert.NotNil(t, appRental.LastAgreedEndDate) // Should be set to approved date
		// Verify EndDate and LastAgreedEndDate are updated (check logic persistence)
		// Since we use DB, let's fetch fresh
		finalRental, _ := rentalRepo.GetByID(ctx, rental.ID)
		assert.Equal(t, newEnd, finalRental.EndDate)
		assert.NotNil(t, finalRental.LastAgreedEndDate)
		assert.Equal(t, newEnd, *finalRental.LastAgreedEndDate)
	})
}
