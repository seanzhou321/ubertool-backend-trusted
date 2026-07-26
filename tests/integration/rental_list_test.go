package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/repository/postgres"

	"github.com/stretchr/testify/require"
)

// TestRentalRepository_ListByRenterAndOwner_OrgFilter covers FR-007 (specs/005-rentals):
// ListMyRentals/ListMyLendings' organization_id filter MUST be optional — when supplied it
// filters to that org, when omitted (0) it MUST return the caller's rentals across all their
// orgs. Per product decision (2026-07-25, confirmed with the repo owner): organization_id was
// never meant to be a mandatory exact-match filter (Known Discrepancy 6,
// specs/005-rentals/spec.md) — this test regression-locks the corrected behavior at the
// repository layer, where the SQL predicate actually lives.
func TestRentalRepository_ListByRenterAndOwner_OrgFilter(t *testing.T) {
	db := prepareDB(t)
	defer db.Close()

	userRepo := postgres.NewUserRepository(db)
	toolRepo := postgres.NewToolRepository(db)
	rentalRepo := postgres.NewRentalRepository(db)
	ctx := context.Background()

	newOrg := func(prefix string) int32 {
		name := fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
		_, err := db.Exec("INSERT INTO orgs (name, metro, address, admin_email, admin_phone_number) VALUES ($1, 'San Jose', '123 Test St', 'admin@test.com', '555-0000')", name)
		require.NoError(t, err)
		var orgID int32
		require.NoError(t, db.QueryRow("SELECT id FROM orgs WHERE name = $1", name).Scan(&orgID))
		return orgID
	}

	newUserInOrg := func(prefix string, orgID int32) *domain.User {
		u := &domain.User{
			Email:        fmt.Sprintf("%s-%d@t.com", prefix, time.Now().UnixNano()),
			PhoneNumber:  fmt.Sprintf("+1555%d", time.Now().UnixNano()%10000000),
			PasswordHash: "h", Name: prefix,
		}
		require.NoError(t, userRepo.Create(ctx, u))
		require.NoError(t, userRepo.AddUserToOrg(ctx, &domain.UserOrg{UserID: u.ID, OrgID: orgID, BalanceCents: 0, Status: domain.UserOrgStatusActive, Role: domain.UserOrgRoleMember}))
		return u
	}

	org1 := newOrg("Org-ListFilter-1")
	org2 := newOrg("Org-ListFilter-2")

	owner := newUserInOrg("owner-lf", org1)
	renter := newUserInOrg("renter-lf", org1)
	require.NoError(t, userRepo.AddUserToOrg(ctx, &domain.UserOrg{UserID: renter.ID, OrgID: org2, BalanceCents: 0, Status: domain.UserOrgStatusActive, Role: domain.UserOrgRoleMember}))

	tool := &domain.Tool{
		OwnerID: owner.ID, Name: "Ladder", PricePerDayCents: 500, PricePerWeekCents: 3000, PricePerMonthCents: 10000,
		DurationUnit: domain.ToolDurationUnitDay, Condition: domain.ToolConditionExcellent, Metro: "San Jose", Status: domain.ToolStatusAvailable,
	}
	require.NoError(t, toolRepo.Create(ctx, tool))

	newRental := func(orgID int32) *domain.Rental {
		rt := &domain.Rental{
			OrgID: orgID, ToolID: tool.ID, RenterID: renter.ID, OwnerID: owner.ID,
			StartDate: time.Now().Format("2006-01-02"), EndDate: time.Now().Add(48 * time.Hour).Format("2006-01-02"),
			DurationUnit: string(domain.ToolDurationUnitDay), DailyPriceCents: 500, WeeklyPriceCents: 3000, MonthlyPriceCents: 10000,
			TotalCostCents: 1000, Status: domain.RentalStatusPending,
		}
		require.NoError(t, rentalRepo.Create(ctx, rt))
		return rt
	}

	rentalOrg1 := newRental(org1)
	rentalOrg2 := newRental(org2)

	t.Run("A specific organization_id filters to only that org's rentals", func(t *testing.T) {
		rentals, count, err := rentalRepo.ListByRenter(ctx, renter.ID, org1, nil, 1, 10)
		require.NoError(t, err)
		require.Len(t, rentals, 1)
		require.Equal(t, int32(1), count)
		require.Equal(t, rentalOrg1.ID, rentals[0].ID)
	})

	t.Run("organization_id=0 (omitted) returns the renter's rentals across all their orgs", func(t *testing.T) {
		rentals, count, err := rentalRepo.ListByRenter(ctx, renter.ID, 0, nil, 1, 10)
		require.NoError(t, err)
		require.Equal(t, int32(2), count, "omitting organization_id must return rentals from every org the caller belongs to, not zero")

		gotIDs := map[int32]bool{}
		for _, r := range rentals {
			gotIDs[r.ID] = true
		}
		require.True(t, gotIDs[rentalOrg1.ID])
		require.True(t, gotIDs[rentalOrg2.ID])
	})

	t.Run("ListByOwner: a specific organization_id filters to only that org's lendings", func(t *testing.T) {
		lendings, count, err := rentalRepo.ListByOwner(ctx, owner.ID, org1, nil, 1, 10)
		require.NoError(t, err)
		require.Len(t, lendings, 1)
		require.Equal(t, int32(1), count)
		require.Equal(t, rentalOrg1.ID, lendings[0].ID)
	})

	t.Run("ListByOwner: organization_id=0 (omitted) returns the owner's lendings across all their orgs", func(t *testing.T) {
		lendings, count, err := rentalRepo.ListByOwner(ctx, owner.ID, 0, nil, 1, 10)
		require.NoError(t, err)
		require.Equal(t, int32(2), count)

		gotIDs := map[int32]bool{}
		for _, r := range lendings {
			gotIDs[r.ID] = true
		}
		require.True(t, gotIDs[rentalOrg1.ID])
		require.True(t, gotIDs[rentalOrg2.ID])
	})
}
