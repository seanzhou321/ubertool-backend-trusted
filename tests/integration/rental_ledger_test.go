package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/repository/postgres"
	"github.com/stretchr/testify/assert"
)

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
		Email:       fmt.Sprintf("owner-%d@t.com", time.Now().UnixNano()),
		PhoneNumber: fmt.Sprintf("p1-%d", time.Now().UnixNano()),
		PasswordHash: "h", Name: "Owner",
	}
	userRepo.Create(ctx, owner)

	renter := &domain.User{
		Email:       fmt.Sprintf("renter-%d@t.com", time.Now().UnixNano()),
		PhoneNumber: fmt.Sprintf("p2-%d", time.Now().UnixNano()),
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
		OwnerID: owner.ID, Name: "Drill", PricePerDayCents: 1000, Condition: domain.ToolConditionExcellent, Metro: "San Jose", Status: domain.ToolStatusAvailable,
	}
	toolRepo.Create(ctx, tool)

	t.Run("Full Lifecycle", func(t *testing.T) {
		// 2. Create Rental Request
		rental := &domain.Rental{
			OrgID: orgID, ToolID: tool.ID, RenterID: renter.ID, OwnerID: owner.ID,
			StartDate: time.Now(), ScheduledEndDate: time.Now().Add(24 * time.Hour),
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
