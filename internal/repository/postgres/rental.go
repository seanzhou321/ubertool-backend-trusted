package postgres

import (
	"context"
	"database/sql"
	"fmt"
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
	query := `INSERT INTO rentals (org_id, tool_id, renter_id, owner_id, start_date, scheduled_end_date, total_cost_cents, status, created_on, updated_on) 
	          VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) RETURNING id`
	return r.db.QueryRowContext(ctx, query, rt.OrgID, rt.ToolID, rt.RenterID, rt.OwnerID, rt.StartDate, rt.ScheduledEndDate, rt.TotalCostCents, rt.Status, time.Now(), time.Now()).Scan(&rt.ID)
}

func (r *rentalRepository) GetByID(ctx context.Context, id int32) (*domain.Rental, error) {
	rt := &domain.Rental{}
	query := `SELECT id, org_id, tool_id, renter_id, owner_id, start_date, scheduled_end_date, end_date, total_cost_cents, status, pickup_note, completed_by, created_on, updated_on FROM rentals WHERE id = $1`
	err := r.db.QueryRowContext(ctx, query, id).Scan(&rt.ID, &rt.OrgID, &rt.ToolID, &rt.RenterID, &rt.OwnerID, &rt.StartDate, &rt.ScheduledEndDate, &rt.EndDate, &rt.TotalCostCents, &rt.Status, &rt.PickupNote, &rt.CompletedBy, &rt.CreatedOn, &rt.UpdatedOn)
	if err != nil {
		return nil, err
	}
	return rt, nil
}

func (r *rentalRepository) Update(ctx context.Context, rt *domain.Rental) error {
	query := `UPDATE rentals SET status=$1, pickup_note=$2, end_date=$3, completed_by=$4, updated_on=$5 WHERE id=$6`
	_, err := r.db.ExecContext(ctx, query, rt.Status, rt.PickupNote, rt.EndDate, rt.CompletedBy, time.Now(), rt.ID)
	return err
}

func (r *rentalRepository) ListByRenter(ctx context.Context, renterID, orgID int32, status string, page, pageSize int32) ([]domain.Rental, int32, error) {
	offset := (page - 1) * pageSize
	sql := `SELECT id, org_id, tool_id, renter_id, owner_id, start_date, scheduled_end_date, end_date, total_cost_cents, status, pickup_note, completed_by, created_on, updated_on 
	        FROM rentals WHERE renter_id = $1 AND org_id = $2`
	
	args := []interface{}{renterID, orgID}
	argIdx := 3
	if status != "" {
		sql += " AND status = $3"
		args = append(args, status)
		argIdx++
	}

	var count int32
	countSql := "SELECT count(*) FROM (" + sql + ") as sub"
	err := r.db.QueryRowContext(ctx, countSql, args...).Scan(&count)
	if err != nil {
		return nil, 0, err
	}

	sql += fmt.Sprintf(" ORDER BY created_on DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, pageSize, offset)

	rows, err := r.db.QueryContext(ctx, sql, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var rentals []domain.Rental
	for rows.Next() {
		var rt domain.Rental
		if err := rows.Scan(&rt.ID, &rt.OrgID, &rt.ToolID, &rt.RenterID, &rt.OwnerID, &rt.StartDate, &rt.ScheduledEndDate, &rt.EndDate, &rt.TotalCostCents, &rt.Status, &rt.PickupNote, &rt.CompletedBy, &rt.CreatedOn, &rt.UpdatedOn); err != nil {
			return nil, 0, err
		}
		rentals = append(rentals, rt)
	}
	return rentals, count, nil
}

func (r *rentalRepository) ListByOwner(ctx context.Context, ownerID, orgID int32, status string, page, pageSize int32) ([]domain.Rental, int32, error) {
	offset := (page - 1) * pageSize
	sql := `SELECT id, org_id, tool_id, renter_id, owner_id, start_date, scheduled_end_date, end_date, total_cost_cents, status, pickup_note, completed_by, created_on, updated_on 
	        FROM rentals WHERE owner_id = $1 AND org_id = $2`
	
	args := []interface{}{ownerID, orgID}
	argIdx := 3
	if status != "" {
		sql += " AND status = $3"
		args = append(args, status)
		argIdx++
	}

	var count int32
	countSql := "SELECT count(*) FROM (" + sql + ") as sub"
	err := r.db.QueryRowContext(ctx, countSql, args...).Scan(&count)
	if err != nil {
		return nil, 0, err
	}

	sql += fmt.Sprintf(" ORDER BY created_on DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, pageSize, offset)

	rows, err := r.db.QueryContext(ctx, sql, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var rentals []domain.Rental
	for rows.Next() {
		var rt domain.Rental
		if err := rows.Scan(&rt.ID, &rt.OrgID, &rt.ToolID, &rt.RenterID, &rt.OwnerID, &rt.StartDate, &rt.ScheduledEndDate, &rt.EndDate, &rt.TotalCostCents, &rt.Status, &rt.PickupNote, &rt.CompletedBy, &rt.CreatedOn, &rt.UpdatedOn); err != nil {
			return nil, 0, err
		}
		rentals = append(rentals, rt)
	}
	return rentals, count, nil
}
