package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"
	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/logger"
	"ubertool-backend-trusted/internal/repository"
)

type billRepository struct {
	db *sql.DB
}

func NewBillRepository(db *sql.DB) repository.BillRepository {
	return &billRepository{db: db}
}

func (r *billRepository) Create(ctx context.Context, bill *domain.Bill) error {
	logger.EnterMethod("billRepository.Create", "orgID", bill.OrgID, "debtorID", bill.DebtorUserID, "creditorID", bill.CreditorUserID)

	query := `
		INSERT INTO bills (
			org_id, debtor_user_id, creditor_user_id, amount_cents, settlement_month, 
			status, notice_sent_at, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) 
		RETURNING id, created_at, updated_at
	`
	now := time.Now()
	err := r.db.QueryRowContext(ctx, query,
		bill.OrgID, bill.DebtorUserID, bill.CreditorUserID, bill.AmountCents, bill.SettlementMonth,
		bill.Status, bill.NoticeSentAt, now, now,
	).Scan(&bill.ID, &bill.CreatedAt, &bill.UpdatedAt)

	if err != nil {
		logger.ExitMethodWithError("billRepository.Create", err, "orgID", bill.OrgID)
		return err
	}

	logger.ExitMethod("billRepository.Create", "billID", bill.ID)
	return nil
}

func (r *billRepository) GetByID(ctx context.Context, id int32) (*domain.Bill, error) {
	logger.EnterMethod("billRepository.GetByID", "billID", id)

	query := `
		SELECT id, org_id, debtor_user_id, creditor_user_id, amount_cents, settlement_month,
		       status, notice_sent_at, debtor_acknowledged_at, creditor_acknowledged_at,
		       disputed_at, resolved_at, COALESCE(dispute_reason, ''), 
		       COALESCE(resolution_outcome, ''), COALESCE(resolution_notes, ''),
		       created_at, updated_at
		FROM bills WHERE id = $1
	`

	bill := &domain.Bill{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&bill.ID, &bill.OrgID, &bill.DebtorUserID, &bill.CreditorUserID, &bill.AmountCents, &bill.SettlementMonth,
		&bill.Status, &bill.NoticeSentAt, &bill.DebtorAcknowledgedAt, &bill.CreditorAcknowledgedAt,
		&bill.DisputedAt, &bill.ResolvedAt, &bill.DisputeReason, &bill.ResolutionOutcome, &bill.ResolutionNotes,
		&bill.CreatedAt, &bill.UpdatedAt,
	)

	if err != nil {
		logger.ExitMethodWithError("billRepository.GetByID", err, "billID", id)
		return nil, err
	}

	logger.ExitMethod("billRepository.GetByID", "billID", id)
	return bill, nil
}

func (r *billRepository) Update(ctx context.Context, bill *domain.Bill) error {
	logger.EnterMethod("billRepository.Update", "billID", bill.ID, "status", bill.Status)

	query := `
		UPDATE bills SET 
			status = $1, 
			notice_sent_at = $2,
			debtor_acknowledged_at = $3,
			creditor_acknowledged_at = $4,
			disputed_at = $5,
			resolved_at = $6,
			dispute_reason = $7,
			resolution_outcome = $8,
			resolution_notes = $9,
			updated_at = $10
		WHERE id = $11
	`

	_, err := r.db.ExecContext(ctx, query,
		bill.Status, bill.NoticeSentAt, bill.DebtorAcknowledgedAt, bill.CreditorAcknowledgedAt,
		bill.DisputedAt, bill.ResolvedAt, bill.DisputeReason, bill.ResolutionOutcome, bill.ResolutionNotes,
		time.Now(), bill.ID,
	)

	if err != nil {
		logger.ExitMethodWithError("billRepository.Update", err, "billID", bill.ID)
		return err
	}

	logger.ExitMethod("billRepository.Update", "billID", bill.ID)
	return nil
}

func (r *billRepository) ListByDebtor(ctx context.Context, debtorID int32, orgID int32, statuses []domain.BillStatus) ([]domain.Bill, error) {
	logger.EnterMethod("billRepository.ListByDebtor", "debtorID", debtorID, "orgID", orgID)

	query := `
		SELECT id, org_id, debtor_user_id, creditor_user_id, amount_cents, settlement_month,
		       status, notice_sent_at, debtor_acknowledged_at, creditor_acknowledged_at,
		       disputed_at, resolved_at, COALESCE(dispute_reason, ''), 
		       COALESCE(resolution_outcome, ''), COALESCE(resolution_notes, ''),
		       created_at, updated_at
		FROM bills 
		WHERE debtor_user_id = $1
	`

	args := []interface{}{debtorID}
	argIndex := 2

	if orgID > 0 {
		query += fmt.Sprintf(" AND org_id = $%d", argIndex)
		args = append(args, orgID)
		argIndex++
	}

	if len(statuses) > 0 {
		statusStrs := make([]string, len(statuses))
		for i, s := range statuses {
			statusStrs[i] = string(s)
		}
		query += fmt.Sprintf(" AND status = ANY($%d)", argIndex)
		args = append(args, pq.Array(statusStrs))
	}

	query += " ORDER BY notice_sent_at DESC, created_at DESC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		logger.ExitMethodWithError("billRepository.ListByDebtor", err, "debtorID", debtorID)
		return nil, err
	}
	defer rows.Close()

	bills := []domain.Bill{}
	for rows.Next() {
		var b domain.Bill
		err := rows.Scan(
			&b.ID, &b.OrgID, &b.DebtorUserID, &b.CreditorUserID, &b.AmountCents, &b.SettlementMonth,
			&b.Status, &b.NoticeSentAt, &b.DebtorAcknowledgedAt, &b.CreditorAcknowledgedAt,
			&b.DisputedAt, &b.ResolvedAt, &b.DisputeReason, &b.ResolutionOutcome, &b.ResolutionNotes,
			&b.CreatedAt, &b.UpdatedAt,
		)
		if err != nil {
			logger.ExitMethodWithError("billRepository.ListByDebtor", err, "debtorID", debtorID)
			return nil, err
		}
		bills = append(bills, b)
	}

	logger.ExitMethod("billRepository.ListByDebtor", "debtorID", debtorID, "count", len(bills))
	return bills, nil
}

func (r *billRepository) ListByCreditor(ctx context.Context, creditorID int32, orgID int32, statuses []domain.BillStatus) ([]domain.Bill, error) {
	logger.EnterMethod("billRepository.ListByCreditor", "creditorID", creditorID, "orgID", orgID)

	query := `
		SELECT id, org_id, debtor_user_id, creditor_user_id, amount_cents, settlement_month,
		       status, notice_sent_at, debtor_acknowledged_at, creditor_acknowledged_at,
		       disputed_at, resolved_at, COALESCE(dispute_reason, ''), 
		       COALESCE(resolution_outcome, ''), COALESCE(resolution_notes, ''),
		       created_at, updated_at
		FROM bills 
		WHERE creditor_user_id = $1
	`

	args := []interface{}{creditorID}
	argIndex := 2

	if orgID > 0 {
		query += fmt.Sprintf(" AND org_id = $%d", argIndex)
		args = append(args, orgID)
		argIndex++
	}

	if len(statuses) > 0 {
		statusStrs := make([]string, len(statuses))
		for i, s := range statuses {
			statusStrs[i] = string(s)
		}
		query += fmt.Sprintf(" AND status = ANY($%d)", argIndex)
		args = append(args, pq.Array(statusStrs))
	}

	query += " ORDER BY notice_sent_at DESC, created_at DESC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		logger.ExitMethodWithError("billRepository.ListByCreditor", err, "creditorID", creditorID)
		return nil, err
	}
	defer rows.Close()

	bills := []domain.Bill{}
	for rows.Next() {
		var b domain.Bill
		err := rows.Scan(
			&b.ID, &b.OrgID, &b.DebtorUserID, &b.CreditorUserID, &b.AmountCents, &b.SettlementMonth,
			&b.Status, &b.NoticeSentAt, &b.DebtorAcknowledgedAt, &b.CreditorAcknowledgedAt,
			&b.DisputedAt, &b.ResolvedAt, &b.DisputeReason, &b.ResolutionOutcome, &b.ResolutionNotes,
			&b.CreatedAt, &b.UpdatedAt,
		)
		if err != nil {
			logger.ExitMethodWithError("billRepository.ListByCreditor", err, "creditorID", creditorID)
			return nil, err
		}
		bills = append(bills, b)
	}

	logger.ExitMethod("billRepository.ListByCreditor", "creditorID", creditorID, "count", len(bills))
	return bills, nil
}

func (r *billRepository) ListByUser(ctx context.Context, userID int32, orgID int32, statuses []domain.BillStatus) ([]domain.Bill, error) {
	logger.EnterMethod("billRepository.ListByUser", "userID", userID, "orgID", orgID)

	query := `
		SELECT id, org_id, debtor_user_id, creditor_user_id, amount_cents, settlement_month,
		       status, notice_sent_at, debtor_acknowledged_at, creditor_acknowledged_at,
		       disputed_at, resolved_at, COALESCE(dispute_reason, ''), 
		       COALESCE(resolution_outcome, ''), COALESCE(resolution_notes, ''),
		       created_at, updated_at
		FROM bills 
		WHERE (debtor_user_id = $1 OR creditor_user_id = $1)
	`

	args := []interface{}{userID}
	argIndex := 2

	if orgID > 0 {
		query += fmt.Sprintf(" AND org_id = $%d", argIndex)
		args = append(args, orgID)
		argIndex++
	}

	if len(statuses) > 0 {
		statusStrs := make([]string, len(statuses))
		for i, s := range statuses {
			statusStrs[i] = string(s)
		}
		query += fmt.Sprintf(" AND status = ANY($%d)", argIndex)
		args = append(args, pq.Array(statusStrs))
	}

	query += " ORDER BY notice_sent_at DESC, created_at DESC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		logger.ExitMethodWithError("billRepository.ListByUser", err, "userID", userID)
		return nil, err
	}
	defer rows.Close()

	bills := []domain.Bill{}
	for rows.Next() {
		var b domain.Bill
		err := rows.Scan(
			&b.ID, &b.OrgID, &b.DebtorUserID, &b.CreditorUserID, &b.AmountCents, &b.SettlementMonth,
			&b.Status, &b.NoticeSentAt, &b.DebtorAcknowledgedAt, &b.CreditorAcknowledgedAt,
			&b.DisputedAt, &b.ResolvedAt, &b.DisputeReason, &b.ResolutionOutcome, &b.ResolutionNotes,
			&b.CreatedAt, &b.UpdatedAt,
		)
		if err != nil {
			logger.ExitMethodWithError("billRepository.ListByUser", err, "userID", userID)
			return nil, err
		}
		bills = append(bills, b)
	}

	logger.ExitMethod("billRepository.ListByUser", "userID", userID, "count", len(bills))
	return bills, nil
}

func (r *billRepository) ListDisputedByOrg(ctx context.Context, orgID int32, excludeUserID *int32) ([]domain.Bill, error) {
	logger.EnterMethod("billRepository.ListDisputedByOrg", "orgID", orgID, "excludeUserID", excludeUserID)

	query := `
		SELECT id, org_id, debtor_user_id, creditor_user_id, amount_cents, settlement_month,
		       status, notice_sent_at, debtor_acknowledged_at, creditor_acknowledged_at,
		       disputed_at, resolved_at, COALESCE(dispute_reason, ''), 
		       COALESCE(resolution_outcome, ''), COALESCE(resolution_notes, ''),
		       created_at, updated_at
		FROM bills 
		WHERE org_id = $1 AND status = $2
	`

	args := []interface{}{orgID, domain.BillStatusDisputed}

	if excludeUserID != nil {
		query += " AND debtor_user_id != $3 AND creditor_user_id != $3"
		args = append(args, *excludeUserID)
	}

	query += " ORDER BY disputed_at DESC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		logger.ExitMethodWithError("billRepository.ListDisputedByOrg", err, "orgID", orgID)
		return nil, err
	}
	defer rows.Close()

	bills := []domain.Bill{}
	for rows.Next() {
		var b domain.Bill
		err := rows.Scan(
			&b.ID, &b.OrgID, &b.DebtorUserID, &b.CreditorUserID, &b.AmountCents, &b.SettlementMonth,
			&b.Status, &b.NoticeSentAt, &b.DebtorAcknowledgedAt, &b.CreditorAcknowledgedAt,
			&b.DisputedAt, &b.ResolvedAt, &b.DisputeReason, &b.ResolutionOutcome, &b.ResolutionNotes,
			&b.CreatedAt, &b.UpdatedAt,
		)
		if err != nil {
			logger.ExitMethodWithError("billRepository.ListDisputedByOrg", err, "orgID", orgID)
			return nil, err
		}
		bills = append(bills, b)
	}

	logger.ExitMethod("billRepository.ListDisputedByOrg", "orgID", orgID, "count", len(bills))
	return bills, nil
}

func (r *billRepository) ListResolvedDisputesByOrg(ctx context.Context, orgID int32) ([]domain.Bill, error) {
	logger.EnterMethod("billRepository.ListResolvedDisputesByOrg", "orgID", orgID)

	query := `
		SELECT id, org_id, debtor_user_id, creditor_user_id, amount_cents, settlement_month,
		       status, notice_sent_at, debtor_acknowledged_at, creditor_acknowledged_at,
		       disputed_at, resolved_at, COALESCE(dispute_reason, ''), 
		       COALESCE(resolution_outcome, ''), COALESCE(resolution_notes, ''),
		       created_at, updated_at
		FROM bills 
		WHERE org_id = $1 
		  AND disputed_at IS NOT NULL 
		  AND resolved_at IS NOT NULL
		  AND status IN ($2, $3)
		ORDER BY resolved_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, orgID, domain.BillStatusAdminResolved, domain.BillStatusSystemDefaultAction)
	if err != nil {
		logger.ExitMethodWithError("billRepository.ListResolvedDisputesByOrg", err, "orgID", orgID)
		return nil, err
	}
	defer rows.Close()

	bills := []domain.Bill{}
	for rows.Next() {
		var b domain.Bill
		err := rows.Scan(
			&b.ID, &b.OrgID, &b.DebtorUserID, &b.CreditorUserID, &b.AmountCents, &b.SettlementMonth,
			&b.Status, &b.NoticeSentAt, &b.DebtorAcknowledgedAt, &b.CreditorAcknowledgedAt,
			&b.DisputedAt, &b.ResolvedAt, &b.DisputeReason, &b.ResolutionOutcome, &b.ResolutionNotes,
			&b.CreatedAt, &b.UpdatedAt,
		)
		if err != nil {
			logger.ExitMethodWithError("billRepository.ListResolvedDisputesByOrg", err, "orgID", orgID)
			return nil, err
		}
		bills = append(bills, b)
	}

	logger.ExitMethod("billRepository.ListResolvedDisputesByOrg", "orgID", orgID, "count", len(bills))
	return bills, nil
}

func (r *billRepository) CreateAction(ctx context.Context, action *domain.BillAction) error {
	logger.EnterMethod("billRepository.CreateAction", "billID", action.BillID, "actionType", action.ActionType)

	query := `
		INSERT INTO bill_actions (
			bill_id, actor_user_id, action_type, action_details, notes, created_at
		) VALUES ($1, $2, $3, $4, $5, $6) 
		RETURNING id, created_at
	`

	err := r.db.QueryRowContext(ctx, query,
		action.BillID, action.ActorUserID, action.ActionType,
		nullString(action.ActionDetails), nullString(action.Notes), time.Now(),
	).Scan(&action.ID, &action.CreatedAt)

	if err != nil {
		logger.ExitMethodWithError("billRepository.CreateAction", err, "billID", action.BillID)
		return err
	}

	logger.ExitMethod("billRepository.CreateAction", "actionID", action.ID)
	return nil
}

func (r *billRepository) ListActionsByBill(ctx context.Context, billID int32) ([]domain.BillAction, error) {
	logger.EnterMethod("billRepository.ListActionsByBill", "billID", billID)

	query := `
		SELECT id, bill_id, actor_user_id, action_type, 
		       COALESCE(action_details::text, ''), COALESCE(notes, ''), created_at
		FROM bill_actions 
		WHERE bill_id = $1
		ORDER BY created_at ASC
	`

	rows, err := r.db.QueryContext(ctx, query, billID)
	if err != nil {
		logger.ExitMethodWithError("billRepository.ListActionsByBill", err, "billID", billID)
		return nil, err
	}
	defer rows.Close()

	actions := []domain.BillAction{}
	for rows.Next() {
		var a domain.BillAction
		err := rows.Scan(&a.ID, &a.BillID, &a.ActorUserID, &a.ActionType, &a.ActionDetails, &a.Notes, &a.CreatedAt)
		if err != nil {
			logger.ExitMethodWithError("billRepository.ListActionsByBill", err, "billID", billID)
			return nil, err
		}
		actions = append(actions, a)
	}

	logger.ExitMethod("billRepository.ListActionsByBill", "billID", billID, "count", len(actions))
	return actions, nil
}

// Helper function to convert empty string to SQL NULL
func nullString(s string) interface{} {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}
