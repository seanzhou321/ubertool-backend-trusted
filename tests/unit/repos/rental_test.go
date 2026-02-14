package repos

import (
	"context"
	"testing"
	"time"

	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/repository/postgres"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func TestRentalRepository_Create(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error opening mock database: %v", err)
	}
	defer db.Close()

	repo := postgres.NewRentalRepository(db)
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		rental := &domain.Rental{
			OrgID:          1,
			ToolID:         2,
			RenterID:       3,
			OwnerID:        4,
			StartDate:      time.Now().Format("2006-01-02"),
			EndDate:        time.Now().Add(24 * time.Hour).Format("2006-01-02"),
			TotalCostCents: 1000,
			Status:         domain.RentalStatusPending,
		}

		mock.ExpectQuery("INSERT INTO rentals").
			WithArgs(rental.OrgID, rental.ToolID, rental.RenterID, rental.OwnerID, rental.StartDate, rental.EndDate, rental.TotalCostCents, rental.Status, sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

		err := repo.Create(ctx, rental)
		assert.NoError(t, err)
		assert.Equal(t, int32(1), rental.ID)
	})
}

func TestRentalRepository_GetByID(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error opening mock database: %v", err)
	}
	defer db.Close()

	repo := postgres.NewRentalRepository(db)
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"id", "org_id", "tool_id", "renter_id", "owner_id", "start_date", "last_agreed_end_date", "end_date", "total_cost_cents", "status", "pickup_note", "rejection_reason", "completed_by", "return_condition", "surcharge_or_credit_cents", "return_note", "created_on", "updated_on"}).
			AddRow(1, 1, 2, 3, 4, time.Now(), time.Now(), time.Now(), 1000, "PENDING", "Note", "", nil, "", 0, "Return Note", time.Now(), time.Now())

		mock.ExpectQuery("SELECT (.+) FROM rentals WHERE id = \\$1").
			WithArgs(int32(1)).
			WillReturnRows(rows)

		rental, err := repo.GetByID(ctx, 1)
		assert.NoError(t, err)
		assert.NotNil(t, rental)
		assert.Equal(t, int32(1), rental.ID)
	})
}
