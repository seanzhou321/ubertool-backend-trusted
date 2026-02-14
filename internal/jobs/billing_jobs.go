package jobs

import (
	"container/heap"
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
			billCount, err := jr.PerformBillSplittingForOrg(ctx, org.ID, org.Name, lastMonth)
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

// PerformBillSplittingForOrg performs bill splitting for a single organization
func (jr *JobRunner) PerformBillSplittingForOrg(ctx context.Context, orgID int32, orgName, settlementMonth string) (int, error) {
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
	var debtors, creditors []Account
	
	for rows.Next() {
		var userID, balance int
		if err := rows.Scan(&userID, &balance); err != nil {
			logger.Error("Failed to scan user balance", "error", err)
			continue
		}

		acc := Account{UserID: userID, Balance: balance}
		if balance < 0 {
			debtors = append(debtors, acc)
		} else if balance > 0 {
			creditors = append(creditors, acc)
		}
	}

	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("error iterating user balances: %w", err)
	}

	// Calculate transactions using the heap-based greedy algorithm
	thresholdCents := jr.config.Billing.SettlementThresholdCents
	transactions := CalculateTransactions(creditors, debtors, thresholdCents)

	// Save transactions to DB
	billCount := 0
	for _, txn := range transactions {
		insertQuery := `
			INSERT INTO bills (
				org_id, debtor_user_id, creditor_user_id, 
				amount_cents, settlement_month, status, 
				notice_sent_at, created_at, updated_at
			)
			VALUES ($1, $2, $3, $4, $5, 'PENDING', NULL, NOW(), NOW())
			ON CONFLICT (org_id, debtor_user_id, creditor_user_id, settlement_month) DO NOTHING
		`

		_, err := jr.db.ExecContext(ctx, insertQuery,
			orgID, txn.FromUserID, txn.ToUserID,
			txn.Amount, settlementMonth)

		if err != nil {
			logger.Error("Failed to insert bill",
				"org_id", orgID,
				"debtor_id", txn.FromUserID,
				"creditor_id", txn.ToUserID,
				"amount", txn.Amount,
				"error", err)
		} else {
			billCount++
			logger.Debug("Created bill",
				"org_id", orgID,
				"debtor_id", txn.FromUserID,
				"creditor_id", txn.ToUserID,
				"amount_cents", txn.Amount)
		}
	}

	logger.Info("Bill splitting completed for org",
		"org_id", orgID,
		"org_name", orgName,
		"bills_created", billCount,
		"settlement_month", settlementMonth)

	return billCount, nil
}

// Account represents a user account with balance
type Account struct {
	UserID  int
	Balance int // Positive = owed money, Negative = owes money
}

// InternalTransaction represents a calculated payment
type InternalTransaction struct {
	FromUserID int
	ToUserID   int
	Amount     int
}

// AccountHeap implements heap.Interface for Account prioritization
type AccountHeap []Account

func (h AccountHeap) Len() int           { return len(h) }
func (h AccountHeap) Less(i, j int) bool { return abs(h[i].Balance) > abs(h[j].Balance) } // Max heap by absolute balance
func (h AccountHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *AccountHeap) Push(x interface{}) {
	*h = append(*h, x.(Account))
}

func (h *AccountHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[0 : n-1]
	return item
}

// abs returns the absolute value of an integer
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// CalculateTransactions generates optimal transactions to settle bills
func CalculateTransactions(creditors, debtors []Account, threshold int) []InternalTransaction {
	var transactions []InternalTransaction

	// Create max heaps
	creditorHeap := &AccountHeap{}
	debtorHeap := &AccountHeap{}

	heap.Init(creditorHeap)
	heap.Init(debtorHeap)

	// Add all creditors and debtors to heaps
	for _, c := range creditors {
		heap.Push(creditorHeap, c)
	}
	for _, d := range debtors {
		heap.Push(debtorHeap, d)
	}

	// Greedy matching: continue while BOTH tops >= threshold
	for creditorHeap.Len() > 0 && debtorHeap.Len() > 0 {
		// Peek at the tops to check threshold condition
		creditor := heap.Pop(creditorHeap).(Account)
		debtor := heap.Pop(debtorHeap).(Account)

		creditorAmount := creditor.Balance
		debtorAmount := abs(debtor.Balance)

		// Check if BOTH are below threshold - if so, stop settling
		if creditorAmount < threshold && debtorAmount < threshold {
			// Stop settling
			break
		}
		
		// If only one is below threshold, we still settle if the other is large enough?
		// User requirement: "Perform greedy match ... until both the top positive and negative accounts are having the amount less than a threshold amount."
		// This implies the loop continues as long as specific condition is met.
		// "until both ... are less" == "while NOT (both < threshold)" == "while at least one >= threshold".
		// However, standard debt settlement usually stops when you can't satisfy the threshold constraint.
		// The prompt says: "until BOTH the top positive and negative accounts are having the amount less than a threshold amount."
		// This suggests if one is large (e.g. 100) and one is small (e.g. 2), we should still match them? 
		// "Greedy match to pay from the negative balance accounts to positive accounts"
		// If I have Creditor +100 and Debtor -2 (Threshold 5).
		// Creditor is > 5. Debtor is < 5. "Both < 5" is FALSE. So we continue.
		// We pay min(100, 2) = 2.
		// Creditor becomes +98. Debtor becomes 0.
		// The small debtor is cleared. The large creditor remains large.
		// This effectively sweeps up small debts into large creditors, which is good.
		
		// Transaction amount is limited by the smaller of the two availabilities
		transactionAmount := creditorAmount
		if debtorAmount < transactionAmount {
			transactionAmount = debtorAmount
		}

		// Create transaction
		transactions = append(transactions, InternalTransaction{
			FromUserID: debtor.UserID,
			ToUserID:   creditor.UserID,
			Amount:     transactionAmount,
		})

		// Update balances
		creditorRemaining := creditorAmount - transactionAmount
		debtorRemaining := debtorAmount - transactionAmount

		// Push back if remaining balance > 0
		// Note: We don't check threshold here, we just push back. 
		// The loop condition handles the stopping criteria.
		if creditorRemaining > 0 {
			creditor.Balance = creditorRemaining
			heap.Push(creditorHeap, creditor)
		}

		if debtorRemaining > 0 {
			debtor.Balance = -debtorRemaining
			heap.Push(debtorHeap, debtor)
		}
	}

	return transactions
}
