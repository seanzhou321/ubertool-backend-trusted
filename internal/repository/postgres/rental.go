package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/repository"
)

type rentalRepository struct {
	db *sql.DB
}

func NewRentalRepository(db *sql.DB) repository.RentalRepository {
	return &rentalRepository{db: db}
}

func (r *rentalRepository) Create(ctx context.Context, rt *domain.Rental) error {
	query := `INSERT INTO rentals (org_id, tool_id, renter_id, owner_id, start_date, end_date, total_cost_cents, status, created_on, updated_on) 
	          VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) RETURNING id`
	now := time.Now().Format("2006-01-02")
	return r.db.QueryRowContext(ctx, query, rt.OrgID, rt.ToolID, rt.RenterID, rt.OwnerID, rt.StartDate, rt.EndDate, rt.TotalCostCents, rt.Status, now, now).Scan(&rt.ID)
}

func (r *rentalRepository) GetByID(ctx context.Context, id int32) (*domain.Rental, error) {
	rt := &domain.Rental{}
	query := `SELECT id, org_id, tool_id, renter_id, owner_id, start_date, last_agreed_end_date, end_date, COALESCE(total_cost_cents, 0), status, COALESCE(pickup_note, ''), COALESCE(rejection_reason, ''), completed_by, COALESCE(return_condition, ''), COALESCE(surcharge_or_credit_cents, 0), created_on, updated_on FROM rentals WHERE id = $1`

	var startDate, endDate, createdOn, updatedOn time.Time
	var lastAgreedEndDate sql.NullTime

	err := r.db.QueryRowContext(ctx, query, id).Scan(&rt.ID, &rt.OrgID, &rt.ToolID, &rt.RenterID, &rt.OwnerID, &startDate, &lastAgreedEndDate, &endDate, &rt.TotalCostCents, &rt.Status, &rt.PickupNote, &rt.RejectionReason, &rt.CompletedBy, &rt.ReturnCondition, &rt.SurchargeOrCreditCents, &createdOn, &updatedOn)
	if err != nil {
		return nil, err
	}
	rt.StartDate = startDate.Format("2006-01-02")
	rt.EndDate = endDate.Format("2006-01-02")
	rt.CreatedOn = createdOn.Format("2006-01-02")
	rt.UpdatedOn = updatedOn.Format("2006-01-02")
	if lastAgreedEndDate.Valid {
		dateStr := lastAgreedEndDate.Time.Format("2006-01-02")
		rt.LastAgreedEndDate = &dateStr
	}

	return rt, nil
}

func (r *rentalRepository) Update(ctx context.Context, rt *domain.Rental) error {
	query := `UPDATE rentals SET status=$1, pickup_note=$2, start_date=$3, last_agreed_end_date=$4, end_date=$5, total_cost_cents=$6, rejection_reason=$7, completed_by=$8, return_condition=$9, surcharge_or_credit_cents=$10, updated_on=$11 WHERE id=$12`
	_, err := r.db.ExecContext(ctx, query, rt.Status, rt.PickupNote, rt.StartDate, rt.LastAgreedEndDate, rt.EndDate, rt.TotalCostCents, rt.RejectionReason, rt.CompletedBy, rt.ReturnCondition, rt.SurchargeOrCreditCents, time.Now().Format("2006-01-02"), rt.ID)
	return err
}

func (r *rentalRepository) ListByRenter(ctx context.Context, renterID, orgID int32, statuses []string, page, pageSize int32) ([]domain.Rental, int32, error) {
	offset := (page - 1) * pageSize
	query := `SELECT id, org_id, tool_id, renter_id, owner_id, start_date, last_agreed_end_date, end_date, COALESCE(total_cost_cents, 0), status, COALESCE(pickup_note, ''), COALESCE(rejection_reason, ''), completed_by, COALESCE(return_condition, ''), COALESCE(surcharge_or_credit_cents, 0), created_on, updated_on 
	        FROM rentals WHERE renter_id = $1 AND org_id = $2`

	args := []interface{}{renterID, orgID}
	argIdx := 3
	if len(statuses) > 0 {
		placeholders := make([]string, len(statuses))
		for i, status := range statuses {
			placeholders[i] = fmt.Sprintf("$%d", argIdx)
			args = append(args, status)
			argIdx++
		}
		query += " AND status IN (" + strings.Join(placeholders, ", ") + ")"
	}

	var count int32
	countSql := "SELECT count(*) FROM (" + query + ") as sub"
	err := r.db.QueryRowContext(ctx, countSql, args...).Scan(&count)
	if err != nil {
		return nil, 0, err
	}

	query += fmt.Sprintf(" ORDER BY created_on DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, pageSize, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var rentals []domain.Rental
	for rows.Next() {
		var rt domain.Rental
		var startDate, endDate, createdOn, updatedOn time.Time
		var lastAgreedEndDate sql.NullTime

		if err := rows.Scan(&rt.ID, &rt.OrgID, &rt.ToolID, &rt.RenterID, &rt.OwnerID, &startDate, &lastAgreedEndDate, &endDate, &rt.TotalCostCents, &rt.Status, &rt.PickupNote, &rt.RejectionReason, &rt.CompletedBy, &rt.ReturnCondition, &rt.SurchargeOrCreditCents, &createdOn, &updatedOn); err != nil {
			return nil, 0, err
		}
		rt.StartDate = startDate.Format("2006-01-02")
		rt.EndDate = endDate.Format("2006-01-02")
		rt.CreatedOn = createdOn.Format("2006-01-02")
		rt.UpdatedOn = updatedOn.Format("2006-01-02")
		if lastAgreedEndDate.Valid {
			dateStr := lastAgreedEndDate.Time.Format("2006-01-02")
			rt.LastAgreedEndDate = &dateStr
		}
		rentals = append(rentals, rt)
	}
	return rentals, count, nil
}

func (r *rentalRepository) ListByOwner(ctx context.Context, ownerID, orgID int32, statuses []string, page, pageSize int32) ([]domain.Rental, int32, error) {
	offset := (page - 1) * pageSize
	query := `SELECT id, org_id, tool_id, renter_id, owner_id, start_date, last_agreed_end_date, end_date, COALESCE(total_cost_cents, 0), status, COALESCE(pickup_note, ''), COALESCE(rejection_reason, ''), completed_by, COALESCE(return_condition, ''), COALESCE(surcharge_or_credit_cents, 0), created_on, updated_on 
	        FROM rentals WHERE owner_id = $1 AND org_id = $2`

	args := []interface{}{ownerID, orgID}
	argIdx := 3
	if len(statuses) > 0 {
		placeholders := make([]string, len(statuses))
		for i, status := range statuses {
			placeholders[i] = fmt.Sprintf("$%d", argIdx)
			args = append(args, status)
			argIdx++
		}
		query += " AND status IN (" + strings.Join(placeholders, ", ") + ")"
	}

	var count int32
	countSql := "SELECT count(*) FROM (" + query + ") as sub"
	err := r.db.QueryRowContext(ctx, countSql, args...).Scan(&count)
	if err != nil {
		return nil, 0, err
	}

	query += fmt.Sprintf(" ORDER BY created_on DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, pageSize, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var rentals []domain.Rental
	for rows.Next() {
		var rt domain.Rental
		var startDate, endDate, createdOn, updatedOn time.Time
		var lastAgreedEndDate sql.NullTime

		if err := rows.Scan(&rt.ID, &rt.OrgID, &rt.ToolID, &rt.RenterID, &rt.OwnerID, &startDate, &lastAgreedEndDate, &endDate, &rt.TotalCostCents, &rt.Status, &rt.PickupNote, &rt.RejectionReason, &rt.CompletedBy, &rt.ReturnCondition, &rt.SurchargeOrCreditCents, &createdOn, &updatedOn); err != nil {
			return nil, 0, err
		}
		rt.StartDate = startDate.Format("2006-01-02")
		rt.EndDate = endDate.Format("2006-01-02")
		rt.CreatedOn = createdOn.Format("2006-01-02")
		rt.UpdatedOn = updatedOn.Format("2006-01-02")
		if lastAgreedEndDate.Valid {
			dateStr := lastAgreedEndDate.Time.Format("2006-01-02")
			rt.LastAgreedEndDate = &dateStr
		}
		rentals = append(rentals, rt)
	}
	return rentals, count, nil
}

func (r *rentalRepository) ListByTool(ctx context.Context, toolID, orgID int32, statuses []string, page, pageSize int32) ([]domain.Rental, int32, error) {
	offset := (page - 1) * pageSize
	query := `SELECT id, org_id, tool_id, renter_id, owner_id, start_date, last_agreed_end_date, end_date, COALESCE(total_cost_cents, 0), status, COALESCE(pickup_note, ''), COALESCE(rejection_reason, ''), completed_by, COALESCE(return_condition, ''), COALESCE(surcharge_or_credit_cents, 0), created_on, updated_on 
	        FROM rentals WHERE tool_id = $1`

	args := []interface{}{toolID}
	argIdx := 2

	if orgID != 0 {
		query += fmt.Sprintf(" AND org_id = $%d", argIdx)
		args = append(args, orgID)
		argIdx++
	}

	if len(statuses) > 0 {
		placeholders := make([]string, len(statuses))
		for i, status := range statuses {
			placeholders[i] = fmt.Sprintf("$%d", argIdx)
			args = append(args, status)
			argIdx++
		}
		query += " AND status IN (" + strings.Join(placeholders, ", ") + ")"
	}

	var count int32
	countSql := "SELECT count(*) FROM (" + query + ") as sub"
	err := r.db.QueryRowContext(ctx, countSql, args...).Scan(&count)
	if err != nil {
		return nil, 0, err
	}

	query += fmt.Sprintf(" ORDER BY created_on DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, pageSize, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var rentals []domain.Rental
	for rows.Next() {
		var rt domain.Rental
		var startDate, endDate, createdOn, updatedOn time.Time
		var lastAgreedEndDate sql.NullTime

		if err := rows.Scan(&rt.ID, &rt.OrgID, &rt.ToolID, &rt.RenterID, &rt.OwnerID, &startDate, &lastAgreedEndDate, &endDate, &rt.TotalCostCents, &rt.Status, &rt.PickupNote, &rt.RejectionReason, &rt.CompletedBy, &rt.ReturnCondition, &rt.SurchargeOrCreditCents, &createdOn, &updatedOn); err != nil {
			return nil, 0, err
		}
		rt.StartDate = startDate.Format("2006-01-02")
		rt.EndDate = endDate.Format("2006-01-02")
		rt.CreatedOn = createdOn.Format("2006-01-02")
		rt.UpdatedOn = updatedOn.Format("2006-01-02")
		if lastAgreedEndDate.Valid {
			dateStr := lastAgreedEndDate.Time.Format("2006-01-02")
			rt.LastAgreedEndDate = &dateStr
		}
		rentals = append(rentals, rt)
	}
	return rentals, count, nil
}
