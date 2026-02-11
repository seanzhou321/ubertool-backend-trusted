package jobs

import (
	"context"
	"fmt"
	"time"

	"ubertool-backend-trusted/internal/logger"
)

// SendOverdueReminders sends email reminders to renters with overdue rentals
func (jr *JobRunner) SendOverdueReminders() {
	jr.runWithRecovery("SendOverdueReminders", func() {
		ctx := context.Background()

		// Find overdue rentals
		query := `
			SELECT r.id, r.renter_id, r.tool_id, r.end_date, 
			       u.email, u.name as renter_name,
			       t.name as tool_name, t.owner_id
			FROM rentals r
			JOIN users u ON r.renter_id = u.id
			JOIN tools t ON r.tool_id = t.id
			WHERE r.status = 'OVERDUE'
		`

		rows, err := jr.db.QueryContext(ctx, query)
		if err != nil {
			logger.Error("Failed to query overdue rentals", "error", err)
			return
		}
		defer rows.Close()

		count := 0
		for rows.Next() {
			var (
				rentalID    int
				renterID    int
				toolID      int
				endDate     string
				email       string
				renterName  string
				toolName    string
				ownerID     int
			)

			if err := rows.Scan(&rentalID, &renterID, &toolID, &endDate, &email, &renterName, &toolName, &ownerID); err != nil {
				logger.Error("Failed to scan overdue rental", "error", err)
				continue
			}

			// Send email reminder
			subject := "Reminder: Overdue Tool Return"
			body := fmt.Sprintf(`Dear %s,

This is a reminder that your rental of "%s" (Rental ID: %d) was due on %s and is now overdue.

Please return the tool as soon as possible to avoid additional charges.

Thank you,
Ubertool Team`, renterName, toolName, rentalID, endDate)

			err := jr.services.Email.SendAdminNotification(ctx, email, subject, body)
			if err != nil {
				logger.Error("Failed to send overdue reminder email",
					"rental_id", rentalID,
					"renter_id", renterID,
					"email", email,
					"error", err)
				continue
			}

			count++
			logger.Debug("Sent overdue reminder",
				"rental_id", rentalID,
				"renter_id", renterID,
				"email", email)
		}

		if err := rows.Err(); err != nil {
			logger.Error("Error iterating overdue rentals", "error", err)
			return
		}

		logger.Info("Overdue reminders sent", "count", count)
	})
}

// SendBillReminders sends reminders to debtors and creditors about unpaid bills
func (jr *JobRunner) SendBillReminders() {
	jr.runWithRecovery("SendBillReminders", func() {
		ctx := context.Background()

		// Find pending bills
		query := `
			SELECT b.id, b.debtor_user_id, b.creditor_user_id, b.amount_cents,
			       b.settlement_month, b.notice_sent_at,
			       debtor.email as debtor_email, debtor.name as debtor_name,
			       creditor.email as creditor_email, creditor.name as creditor_name,
			       o.name as org_name
			FROM bills b
			JOIN users debtor ON b.debtor_user_id = debtor.id
			JOIN users creditor ON b.creditor_user_id = creditor.id
			JOIN orgs o ON b.org_id = o.id
			WHERE b.status = 'PENDING'
			  AND b.notice_sent_at IS NOT NULL
			  AND b.notice_sent_at < $1
		`

		// Send reminders for bills older than 3 days
		threeDaysAgo := time.Now().Add(-72 * time.Hour)
		rows, err := jr.db.QueryContext(ctx, query, threeDaysAgo)
		if err != nil {
			logger.Error("Failed to query pending bills", "error", err)
			return
		}
		defer rows.Close()

		count := 0
		for rows.Next() {
			var (
				billID           int
				debtorID         int
				creditorID       int
				amountCents      int
				settlementMonth  string
				noticeSentAt     time.Time
				debtorEmail      string
				debtorName       string
				creditorEmail    string
				creditorName     string
				orgName          string
			)

			if err := rows.Scan(&billID, &debtorID, &creditorID, &amountCents, &settlementMonth, &noticeSentAt,
				&debtorEmail, &debtorName, &creditorEmail, &creditorName, &orgName); err != nil {
				logger.Error("Failed to scan pending bill", "error", err)
				continue
			}

			// Calculate amount in dollars
			amountDollars := float64(amountCents) / 100.0

			// Send reminder to debtor
			debtorSubject := fmt.Sprintf("Reminder: Payment Due for %s", orgName)
			debtorBody := fmt.Sprintf(`Dear %s,

This is a reminder that you have a pending payment of $%.2f to %s for the settlement period %s.

Please acknowledge your payment in the Ubertool app once completed.

Bill ID: %d

Thank you,
Ubertool Team`, debtorName, amountDollars, creditorName, settlementMonth, billID)

			err := jr.services.Email.SendAdminNotification(ctx, debtorEmail, debtorSubject, debtorBody)
			if err != nil {
				logger.Error("Failed to send reminder to debtor",
					"bill_id", billID,
					"debtor_id", debtorID,
					"error", err)
			}

			// Send reminder to creditor
			creditorSubject := fmt.Sprintf("Reminder: Payment Expected from %s", debtorName)
			creditorBody := fmt.Sprintf(`Dear %s,

This is a reminder that you are expecting a payment of $%.2f from %s for the settlement period %s.

Please confirm receipt in the Ubertool app once you receive the payment.

Bill ID: %d

Thank you,
Ubertool Team`, creditorName, amountDollars, debtorName, settlementMonth, billID)

			err = jr.services.Email.SendAdminNotification(ctx, creditorEmail, creditorSubject, creditorBody)
			if err != nil {
				logger.Error("Failed to send reminder to creditor",
					"bill_id", billID,
					"creditor_id", creditorID,
					"error", err)
			}

			count++
			logger.Debug("Sent bill reminders",
				"bill_id", billID,
				"debtor_id", debtorID,
				"creditor_id", creditorID)
		}

		if err := rows.Err(); err != nil {
			logger.Error("Error iterating pending bills", "error", err)
			return
		}

		logger.Info("Bill reminders sent", "count", count)
	})
}
