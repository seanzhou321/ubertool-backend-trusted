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

func TestToolRepository_Integration(t *testing.T) {
	db := prepareDB(t)
	defer db.Close()

	userRepo := postgres.NewUserRepository(db)
	repo := postgres.NewToolRepository(db)
	ctx := context.Background()

	// 1. Create an owner first
	ownerEmail := fmt.Sprintf("owner-%d@test.com", time.Now().UnixNano())
	owner := &domain.User{
		Email:        ownerEmail,
		PhoneNumber:  fmt.Sprintf("phone-%d", time.Now().UnixNano()),
		PasswordHash: "hash",
		Name:         "Tool Owner",
	}
	err := userRepo.Create(ctx, owner)
	assert.NoError(t, err)

	t.Run("Create and Get", func(t *testing.T) {
		tool := &domain.Tool{
			OwnerID:            owner.ID,
			Name:               "Hammer",
			Description:        "Heavy hammer",
			Categories:         []string{"Hand Tools"},
			PricePerDayCents:   100,
			PricePerWeekCents:  600,
			PricePerMonthCents: 2000,
			DurationUnit:       domain.ToolDurationUnitDay,
			Condition:          domain.ToolConditionExcellent,
			Metro:              "San Jose",
			Status:             domain.ToolStatusAvailable,
		}

		err := repo.Create(ctx, tool)
		assert.NoError(t, err)
		assert.NotZero(t, tool.ID)

		fetched, err := repo.GetByID(ctx, tool.ID)
		assert.NoError(t, err)
		assert.Equal(t, tool.Name, fetched.Name)
		assert.Equal(t, tool.Metro, fetched.Metro)
	})

	t.Run("Search by Metro", func(t *testing.T) {
		// Metro is set to "San Jose" in previous test
		// Let's create another one in "San Francisco"
		tool2 := &domain.Tool{
			OwnerID:            owner.ID,
			Name:               "Screwdriver",
			Description:        "Manual screwdriver",
			Categories:         []string{"Hand Tools"},
			PricePerDayCents:   50,
			PricePerWeekCents:  300,
			PricePerMonthCents: 1000,
			DurationUnit:       domain.ToolDurationUnitDay,
			Condition:          domain.ToolConditionGood,
			Metro:              "San Francisco",
			Status:             domain.ToolStatusAvailable,
		}
		repo.Create(ctx, tool2)

		// Create an org with metro "San Jose"
		db.Exec("INSERT INTO orgs (name, metro, address, admin_email, admin_phone_number) VALUES ('Org SJ', 'San Jose', '123 Test St', 'admin@test.com', '555-0000')")
		var orgID int32
		db.QueryRow("SELECT id FROM orgs WHERE name = 'Org SJ'").Scan(&orgID)

		tools, total, err := repo.ListByOrg(ctx, orgID, 1, 10)
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, total, int32(1))

		// Check that we only got San Jose tools (Hammer should be there, Screwdriver should not)
		foundHammer := false
		for _, tool := range tools {
			if tool.Name == "Hammer" {
				foundHammer = true
			}
			assert.Equal(t, "San Jose", tool.Metro)
		}
		assert.True(t, foundHammer)
	})
}
