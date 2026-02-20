package repos

import (
	"context"
	"testing"
	"time"

	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/repository/postgres"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/lib/pq"
)

func TestToolRepository_GetByID(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	repo := postgres.NewToolRepository(db)
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"id", "owner_id", "name", "description", "categories", "price_per_day_cents", "price_per_week_cents", "price_per_month_cents", "replacement_cost_cents", "duration_unit", "condition", "metro", "status", "created_on", "deleted_on"}).
			AddRow(1, 2, "Hammer", "A tool", pq.Array([]string{"Hand Tools"}), 100, 500, 1500, 2000, "day", "EXCELLENT", "San Jose", "AVAILABLE", time.Now(), nil)

		mock.ExpectQuery("SELECT (.+) FROM tools WHERE id = \\$1").
			WithArgs(int32(1)).
			WillReturnRows(rows)

		tool, err := repo.GetByID(ctx, 1)
		assert.NoError(t, err)
		assert.NotNil(t, tool)
		assert.Equal(t, int32(1), tool.ID)
		assert.Equal(t, "Hammer", tool.Name)
	})
}

func TestToolRepository_Create(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	repo := postgres.NewToolRepository(db)
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		tool := &domain.Tool{
			OwnerID:          2,
			Name:             "Drill",
			Description:      "Power drill",
			Categories:       []string{"Power Tools"},
			PricePerDayCents: 500,
			DurationUnit:     domain.ToolDurationUnitDay,
			Condition:        domain.ToolConditionExcellent,
			Metro:            "San Jose",
			Status:           domain.ToolStatusAvailable,
		}

		mock.ExpectQuery("INSERT INTO tools").
			WithArgs(tool.OwnerID, tool.Name, tool.Description, pq.Array(tool.Categories), tool.PricePerDayCents, tool.PricePerWeekCents, tool.PricePerMonthCents, tool.ReplacementCostCents, tool.DurationUnit, tool.Condition, tool.Metro, tool.Status, sqlmock.AnyArg()).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

		err := repo.Create(ctx, tool)
		assert.NoError(t, err)
		assert.Equal(t, int32(1), tool.ID)
	})
}
