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

func (m *MockNotificationRepo) GetNotifications(ctx context.Context, userID int32, page, pageSize int32) ([]domain.Notification, int32, error) {
	return nil, 0, nil
}
func (m *MockNotificationRepo) MarkAsRead(ctx context.Context, userID int32, notificationID int64) error {
	return nil
}
func (m *MockNotificationRepo) Dispatch(ctx context.Context, n *domain.Notification) error {
	return nil
}
func (m *MockNotificationRepo) DispatchSilent(ctx context.Context, n *domain.Notification) error {
	return nil
}
func (m *MockNotificationRepo) SyncDeviceToken(ctx context.Context, userID int32, fcmToken, androidDeviceID, deviceName string) error {
	return nil
}
func (m *MockNotificationRepo) ReportMessageEvent(ctx context.Context, userID int32, notificationID int64, eventType string, eventTime time.Time) error {
	return nil
}
func (m *MockNotificationRepo) SetPushService(pushSvc service.PushNotificationService) {}

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
	db.Exec("INSERT INTO orgs (name, metro, address, admin_email, admin_phone_number) VALUES ($1, 'San Jose', '123 Test St', 'admin@test.com', '555-0000')", orgName)
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

// TestRentalService_CompleteRental_Integration covers FR-004 (specs/005-rentals):
// CompleteRental must require a participant caller and ACTIVE/SCHEDULED/OVERDUE status, must
// recompute cost from the rental's own price snapshot (never the tool's current prices), and
// must only create ledger transactions / update balances when charge_billsplit=true. Regression
// test for a spec-vs-test discrepancy found during SBR remediation: the only prior "integration"
// coverage of this behavior bypassed service.CompleteRental entirely (direct repo writes),
// while spec.md's own Coverage Baseline claimed the ledger/balance effects were integration-tested.
func TestRentalService_CompleteRental_Integration(t *testing.T) {
	db := prepareDB(t)
	defer db.Close()

	userRepo := postgres.NewUserRepository(db)
	toolRepo := postgres.NewToolRepository(db)
	rentalRepo := postgres.NewRentalRepository(db)
	ledgerRepo := postgres.NewLedgerRepository(db)
	emailSvc := new(MockEmailService)
	noteRepo := new(MockNotificationRepo)
	svc := service.NewRentalService(rentalRepo, toolRepo, ledgerRepo, userRepo, emailSvc, noteRepo)
	ctx := context.Background()

	orgName := fmt.Sprintf("Org-Complete-%d", time.Now().UnixNano())
	_, err := db.Exec("INSERT INTO orgs (name, metro, address, admin_email, admin_phone_number) VALUES ($1, 'San Jose', '123 Test St', 'admin@test.com', '555-0000')", orgName)
	require.NoError(t, err)
	var orgID int32
	require.NoError(t, db.QueryRow("SELECT id FROM orgs WHERE name = $1", orgName).Scan(&orgID))

	newUser := func(prefix string) *domain.User {
		u := &domain.User{
			Email:        fmt.Sprintf("%s-%d@t.com", prefix, time.Now().UnixNano()),
			PhoneNumber:  fmt.Sprintf("+1555%d", time.Now().UnixNano()%10000000),
			PasswordHash: "h", Name: prefix,
		}
		require.NoError(t, userRepo.Create(ctx, u))
		require.NoError(t, userRepo.AddUserToOrg(ctx, &domain.UserOrg{UserID: u.ID, OrgID: orgID, BalanceCents: 0, Status: domain.UserOrgStatusActive, Role: domain.UserOrgRoleMember}))
		return u
	}

	// Tool's CURRENT price is set deliberately different from the rental's own price
	// snapshot below, so a test that (incorrectly) recomputed cost from the tool's live
	// price instead of the snapshot would be caught.
	newTool := func(owner *domain.User) *domain.Tool {
		tl := &domain.Tool{
			OwnerID: owner.ID, Name: "Drill", PricePerDayCents: 9999, PricePerWeekCents: 60000, PricePerMonthCents: 200000,
			DurationUnit: domain.ToolDurationUnitDay, Condition: domain.ToolConditionExcellent, Metro: "San Jose", Status: domain.ToolStatusRented,
		}
		require.NoError(t, toolRepo.Create(ctx, tl))
		return tl
	}

	newActiveRental := func(owner, renter *domain.User, tool *domain.Tool, status domain.RentalStatus) *domain.Rental {
		rt := &domain.Rental{
			OrgID: orgID, ToolID: tool.ID, RenterID: renter.ID, OwnerID: owner.ID,
			StartDate: time.Now().Add(-48 * time.Hour).Format("2006-01-02"), EndDate: time.Now().Format("2006-01-02"),
			// Price snapshot deliberately differs from the tool's current price (9999/day) above.
			DurationUnit: string(domain.ToolDurationUnitDay), DailyPriceCents: 1000, WeeklyPriceCents: 6000, MonthlyPriceCents: 20000,
			TotalCostCents: 2000, Status: domain.RentalStatusPending,
		}
		require.NoError(t, rentalRepo.Create(ctx, rt))
		rt.Status = status
		require.NoError(t, rentalRepo.Update(ctx, rt))
		return rt
	}

	getBalance := func(userID int32) int32 {
		var balance int32
		require.NoError(t, db.QueryRow("SELECT balance_cents FROM users_orgs WHERE user_id = $1 AND org_id = $2", userID, orgID).Scan(&balance))
		return balance
	}

	t.Run("Recomputes cost from the rental's own price snapshot, not the tool's current price", func(t *testing.T) {
		owner, renter := newUser("owner-cr"), newUser("renter-cr")
		tool := newTool(owner)
		rt := newActiveRental(owner, renter, tool, domain.RentalStatusActive)

		completed, err := svc.CompleteRental(ctx, owner.ID, rt.ID, "Good", 0, "", true)
		require.NoError(t, err)
		// 2 days at the snapshot's 1000/day = 2000, NOT 2 * 9999 (the tool's live price).
		assert.Equal(t, int32(2000), completed.TotalCostCents)
	})

	t.Run("charge_billsplit=true creates ledger transactions and updates balances", func(t *testing.T) {
		owner, renter := newUser("owner-cb-true"), newUser("renter-cb-true")
		tool := newTool(owner)
		rt := newActiveRental(owner, renter, tool, domain.RentalStatusActive)

		ownerBefore, renterBefore := getBalance(owner.ID), getBalance(renter.ID)
		// CompleteRental is owner-only (SEC-RENTAL-005, sbr/rtm/009-security.rtm.md) — this
		// subtest is about the ledger/balance side effects of a completion, not about who may
		// call it (see "Rejects a caller who is the renter, not the owner" below for that), so
		// it calls as the owner like its sibling subtest above.
		_, err := svc.CompleteRental(ctx, owner.ID, rt.ID, "Good", 0, "", true)
		require.NoError(t, err)

		assert.Equal(t, ownerBefore+2000, getBalance(owner.ID), "owner should be credited the settlement")
		assert.Equal(t, renterBefore-2000, getBalance(renter.ID), "renter should be debited the settlement")

		_, total, err := ledgerRepo.ListTransactions(ctx, owner.ID, orgID, 1, 10)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, total, int32(1))
	})

	t.Run("charge_billsplit=false does not create ledger transactions or change balances", func(t *testing.T) {
		owner, renter := newUser("owner-cb-false"), newUser("renter-cb-false")
		tool := newTool(owner)
		rt := newActiveRental(owner, renter, tool, domain.RentalStatusActive)

		ownerBefore, renterBefore := getBalance(owner.ID), getBalance(renter.ID)
		_, err := svc.CompleteRental(ctx, owner.ID, rt.ID, "Good", 0, "", false)
		require.NoError(t, err)

		assert.Equal(t, ownerBefore, getBalance(owner.ID), "balance must not change when charge_billsplit=false")
		assert.Equal(t, renterBefore, getBalance(renter.ID), "balance must not change when charge_billsplit=false")
	})

	t.Run("Rejects a non-participant caller", func(t *testing.T) {
		owner, renter := newUser("owner-np"), newUser("renter-np")
		outsider := newUser("outsider-np")
		tool := newTool(owner)
		rt := newActiveRental(owner, renter, tool, domain.RentalStatusActive)

		_, err := svc.CompleteRental(ctx, outsider.ID, rt.ID, "Good", 0, "", true)
		require.Error(t, err)
	})

	// SEC-RENTAL-005 (sbr/rtm/009-security.rtm.md): CompleteRental is documented owner-only
	// ("Complete rental (mark as returned, owner only)", rental_service.proto) — the renter is a
	// participant but must not be able to self-complete their own rental against the real DB
	// (balance must be left untouched, matching the unit-level regression test in
	// tests/unit/rental_test.go).
	t.Run("Rejects the renter — CompleteRental is owner-only", func(t *testing.T) {
		owner, renter := newUser("owner-renter-reject"), newUser("renter-renter-reject")
		tool := newTool(owner)
		rt := newActiveRental(owner, renter, tool, domain.RentalStatusActive)

		ownerBefore, renterBefore := getBalance(owner.ID), getBalance(renter.ID)
		_, err := svc.CompleteRental(ctx, renter.ID, rt.ID, "Good", 0, "", true)
		require.Error(t, err)
		assert.Equal(t, ownerBefore, getBalance(owner.ID), "a rejected completion must not touch balances")
		assert.Equal(t, renterBefore, getBalance(renter.ID), "a rejected completion must not touch balances")
	})

	t.Run("Rejects a rental that is not ACTIVE/SCHEDULED/OVERDUE", func(t *testing.T) {
		owner, renter := newUser("owner-status"), newUser("renter-status")
		tool := newTool(owner)
		// Left in PENDING — never transitioned to a completable status.
		rt := newActiveRental(owner, renter, tool, domain.RentalStatusPending)

		_, err := svc.CompleteRental(ctx, owner.ID, rt.ID, "Good", 0, "", true)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be completed")
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
	_, err := db.Exec("INSERT INTO orgs (name, metro, address, admin_email, admin_phone_number) VALUES ($1, 'San Jose', '123 Test St', 'admin@test.com', '555-0000')", orgName)
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
