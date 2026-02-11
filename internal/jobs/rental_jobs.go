package jobs

import (
	"context"
	"time"

	"ubertool-backend-trusted/internal/logger"
)

// MarkOverdueRentals marks rentals as OVERDUE if they are past their end_date
func (jr *JobRunner) MarkOverdueRentals() {
	jr.runWithRecovery("MarkOverdueRentals", func() {
		ctx := context.Background()

		// Find rentals that are past their end date and still in ACTIVE status
		query := `
			UPDATE rentals
			SET status = 'OVERDUE',
			    updated_on = NOW()
			WHERE status = 'ACTIVE'
			  AND end_date < $1
			RETURNING id, renter_id, tool_id, end_date
		`

		rows, err := jr.db.QueryContext(ctx, query, time.Now().Format("2006-01-02"))
		if err != nil {
			logger.Error("Failed to mark overdue rentals", "error", err)
			return
		}
		defer rows.Close()

		count := 0
		var overdueRentals []struct {
			ID       int
			RenterID int
			ToolID   int
			EndDate  string
		}

		for rows.Next() {
			var rental struct {
				ID       int
				RenterID int
				ToolID   int
				EndDate  string
			}
			if err := rows.Scan(&rental.ID, &rental.RenterID, &rental.ToolID, &rental.EndDate); err != nil {
				logger.Error("Failed to scan overdue rental", "error", err)
				continue
			}
			overdueRentals = append(overdueRentals, rental)
			count++
		}

		if err := rows.Err(); err != nil {
			logger.Error("Error iterating overdue rentals", "error", err)
			return
		}

		logger.Info("Marked rentals as overdue", "count", count)

		// Log details for each overdue rental
		for _, rental := range overdueRentals {
			logger.Debug("Marked rental as overdue",
				"rental_id", rental.ID,
				"renter_id", rental.RenterID,
				"tool_id", rental.ToolID,
				"end_date", rental.EndDate)
		}
	})
}
