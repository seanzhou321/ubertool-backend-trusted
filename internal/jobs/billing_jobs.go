package jobs

import (
	"context"
	"fmt"
	"time"

	"ubertool-backend-trusted/internal/logger"
)

// CheckOverdueBills checks for bills overdue by 10+ days and marks them as DISPUTED
func (jr *JobRunner) CheckOverdueBills() {
	jr.runWithRecovery("CheckOverdueBills", func() {
		ctx := context.Background()

		// Call the database function to check overdue bills
		_, err := jr.db.ExecContext(ctx, "SELECT check_overdue_bills()")
		if err != nil {
			logger.Error("Failed to check overdue bills", "error", err)
			return
		}

		logger.Info("Successfully checked overdue bills and initiated disputes")
	})
}

// ResolveDisputedBills applies system default action to unresolved disputed bills
func (jr *JobRunner) ResolveDisputedBills() {
	jr.runWithRecovery("ResolveDisputedBills", func() {
		ctx := context.Background()

		// Get all organizations
		orgs, err := jr.store.OrganizationRepository.List(ctx)
		if err != nil {
			logger.Error("Failed to get organizations", "error", err)
			return
		}

		// Get current settlement month (format: 'YYYY-MM')
		currentMonth := time.Now().Format("2006-01")

		totalResolved := 0
		for _, org := range orgs {
			// Call the database function for each organization
			result, err := jr.db.ExecContext(
				ctx,
				"SELECT auto_resolve_disputed_bills($1, $2)",
				org.ID,
				currentMonth,
			)
			if err != nil {
				logger.Error("Failed to resolve disputed bills for org",
					"org_id", org.ID,
					"org_name", org.Name,
					"error", err)
				continue
			}

			logger.Info("Resolved disputed bills for org",
				"org_id", org.ID,
				"org_name", org.Name,
				"settlement_month", currentMonth)

			if result != nil {
				totalResolved++
			}
		}

		logger.Info("Completed resolving disputed bills",
			"total_orgs_processed", totalResolved,
			"settlement_month", currentMonth)
	})
}

// TakeBalanceSnapshots takes a snapshot of all user balances before bill splitting
func (jr *JobRunner) TakeBalanceSnapshots() {
	jr.runWithRecovery("TakeBalanceSnapshots", func() {
		ctx := context.Background()

		// Get current settlement month (format: 'YYYY-MM')
		settlementMonth := time.Now().Format("2006-01")

		// Insert balance snapshots for all users in all orgs
		query := `
			INSERT INTO balance_snapshots (user_id, org_id, balance_cents, settlement_month, snapshot_at)
			SELECT user_id, org_id, balance_cents, $1, NOW()
			FROM users_orgs
			ON CONFLICT (user_id, org_id, settlement_month) DO NOTHING
		`

		result, err := jr.db.ExecContext(ctx, query, settlementMonth)
		if err != nil {
			logger.Error("Failed to take balance snapshots", "error", err)
			return
		}

		rowsAffected, _ := result.RowsAffected()
		logger.Info("Balance snapshots taken",
			"count", rowsAffected,
			"settlement_month", settlementMonth)
	})
}

// PerformBillSplitting performs the monthly bill splitting calculation
func (jr *JobRunner) PerformBillSplitting() {
	jr.runWithRecovery("PerformBillSplitting", func() {
		ctx := context.Background()

		// Get all organizations
		orgs, err := jr.store.OrganizationRepository.List(ctx)
		if err != nil {
			logger.Error("Failed to get organizations", "error", err)
			return
		}

		// Get previous month for settlement (format: 'YYYY-MM')
		lastMonth := time.Now().AddDate(0, -1, 0).Format("2006-01")

		totalBills := 0
		for _, org := range orgs {
			billCount, err := jr.performBillSplittingForOrg(ctx, org.ID, org.Name, lastMonth)
			if err != nil {
				logger.Error("Failed to perform bill splitting for org",
					"org_id", org.ID,
					"org_name", org.Name,
					"error", err)
				continue
			}
			totalBills += billCount
		}

		logger.Info("Bill splitting completed",
			"total_bills_created", totalBills,
			"settlement_month", lastMonth)
	})
}

// performBillSplittingForOrg performs bill splitting for a single organization
func (jr *JobRunner) performBillSplittingForOrg(ctx context.Context, orgID int32, orgName, settlementMonth string) (int, error) {
	// Get all users in the organization with their balances
	query := `
		SELECT user_id, balance_cents
		FROM users_orgs
		WHERE org_id = $1
		  AND status = 'ACTIVE'
		  AND balance_cents != 0
	`

	rows, err := jr.db.QueryContext(ctx, query, orgID)
	if err != nil {
		return 0, fmt.Errorf("failed to get user balances: %w", err)
	}
	defer rows.Close()

	// Collect debtors (negative balance) and creditors (positive balance)
	var debtors, creditors []struct {
		UserID  int
		Balance int
	}

	for rows.Next() {
		var userID, balance int
		if err := rows.Scan(&userID, &balance); err != nil {
			logger.Error("Failed to scan user balance", "error", err)
			continue
		}

		if balance < 0 {
			debtors = append(debtors, struct {
				UserID  int
				Balance int
			}{userID, -balance}) // Store as positive amount owed
		} else if balance > 0 {
			creditors = append(creditors, struct {
				UserID  int
				Balance int
			}{userID, balance})
		}
	}

	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("error iterating user balances: %w", err)
	}

	// Simple greedy algorithm: match largest debtor with largest creditor
	billCount := 0
	for len(debtors) > 0 && len(creditors) > 0 {
		debtor := &debtors[0]
		creditor := &creditors[0]

		// Determine payment amount
		paymentAmount := debtor.Balance
		if creditor.Balance < paymentAmount {
			paymentAmount = creditor.Balance
		}

		// Insert bill
		insertQuery := `
			INSERT INTO bills (
				org_id, debtor_user_id, creditor_user_id, 
				amount_cents, settlement_month, status, 
				notice_sent_at, created_at, updated_at
			)
			VALUES ($1, $2, $3, $4, $5, 'PENDING', NOW(), NOW(), NOW())
			ON CONFLICT (org_id, debtor_user_id, creditor_user_id, settlement_month) DO NOTHING
		`

		_, err := jr.db.ExecContext(ctx, insertQuery,
			orgID, debtor.UserID, creditor.UserID,
			paymentAmount, settlementMonth)

		if err != nil {
			logger.Error("Failed to insert bill",
				"org_id", orgID,
				"debtor_id", debtor.UserID,
				"creditor_id", creditor.UserID,
				"amount", paymentAmount,
				"error", err)
		} else {
			billCount++
			logger.Debug("Created bill",
				"org_id", orgID,
				"debtor_id", debtor.UserID,
				"creditor_id", creditor.UserID,
				"amount_cents", paymentAmount)
		}

		// Update balances
		debtor.Balance -= paymentAmount
		creditor.Balance -= paymentAmount

		// Remove settled parties
		if debtor.Balance == 0 {
			debtors = debtors[1:]
		}
		if creditor.Balance == 0 {
			creditors = creditors[1:]
		}
	}

	logger.Info("Bill splitting completed for org",
		"org_id", orgID,
		"org_name", orgName,
		"bills_created", billCount,
		"settlement_month", settlementMonth)

	return billCount, nil
}
