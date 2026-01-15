package postgres

import (
	"context"
	"database/sql"
	"time"

	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/repository"
)

type joinRequestRepository struct {
	db *sql.DB
}

func NewJoinRequestRepository(db *sql.DB) repository.JoinRequestRepository {
	return &joinRequestRepository{db: db}
}

func (r *joinRequestRepository) Create(ctx context.Context, req *domain.JoinRequest) error {
	query := `INSERT INTO join_requests (org_id, name, email, note, status, created_on) 
	          VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`
	return r.db.QueryRowContext(ctx, query, req.OrgID, req.Name, req.Email, req.Note, req.Status, time.Now()).Scan(&req.ID)
}

func (r *joinRequestRepository) GetByID(ctx context.Context, id int32) (*domain.JoinRequest, error) {
	req := &domain.JoinRequest{}
	query := `SELECT id, org_id, name, email, note, status, created_on FROM join_requests WHERE id = $1`
	err := r.db.QueryRowContext(ctx, query, id).Scan(&req.ID, &req.OrgID, &req.Name, &req.Email, &req.Note, &req.Status, &req.CreatedOn)
	if err != nil {
		return nil, err
	}
	return req, nil
}

func (r *joinRequestRepository) Update(ctx context.Context, req *domain.JoinRequest) error {
	query := `UPDATE join_requests SET status = $1 WHERE id = $2`
	_, err := r.db.ExecContext(ctx, query, req.Status, req.ID)
	return err
}

func (r *joinRequestRepository) ListByOrg(ctx context.Context, orgID int32) ([]domain.JoinRequest, error) {
	query := `SELECT id, org_id, name, email, note, status, created_on FROM join_requests WHERE org_id = $1`
	rows, err := r.db.QueryContext(ctx, query, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reqs []domain.JoinRequest
	for rows.Next() {
		var req domain.JoinRequest
		if err := rows.Scan(&req.ID, &req.OrgID, &req.Name, &req.Email, &req.Note, &req.Status, &req.CreatedOn); err != nil {
			return nil, err
		}
		reqs = append(reqs, req)
	}
	return reqs, nil
}
