package postgres

import (
	"context"
	"database/sql"
	"time"

	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/logger"
	"ubertool-backend-trusted/internal/repository"
)

type joinRequestRepository struct {
	db *sql.DB
}

func NewJoinRequestRepository(db *sql.DB) repository.JoinRequestRepository {
	return &joinRequestRepository{db: db}
}

func (r *joinRequestRepository) Create(ctx context.Context, req *domain.JoinRequest) error {
	logger.EnterMethod("joinRequestRepository.Create", "orgID", req.OrgID, "email", req.Email, "name", req.Name)

	query := `INSERT INTO join_requests (org_id, user_id, name, email, note, status, created_on) 
	          VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`
	logger.DatabaseCall("INSERT", "join_requests", "orgID", req.OrgID, "email", req.Email)

	now := time.Now().Format("2006-01-02")
	err := r.db.QueryRowContext(ctx, query, req.OrgID, req.UserID, req.Name, req.Email, req.Note, req.Status, now).Scan(&req.ID)
	logger.DatabaseResult("INSERT", 1, err, "requestID", req.ID)

	if err != nil {
		logger.ExitMethodWithError("joinRequestRepository.Create", err, "orgID", req.OrgID, "email", req.Email)
	} else {
		logger.ExitMethod("joinRequestRepository.Create", "requestID", req.ID)
	}
	return err
}

func (r *joinRequestRepository) GetByID(ctx context.Context, id int32) (*domain.JoinRequest, error) {
	req := &domain.JoinRequest{}
	query := `SELECT id, org_id, user_id, name, email, note, reason, status, created_on FROM join_requests WHERE id = $1`
	var createdOn time.Time
	var note, reason sql.NullString
	err := r.db.QueryRowContext(ctx, query, id).Scan(&req.ID, &req.OrgID, &req.UserID, &req.Name, &req.Email, &note, &reason, &req.Status, &createdOn)
	if err != nil {
		return nil, err
	}
	req.CreatedOn = createdOn.Format("2006-01-02")
	if note.Valid {
		req.Note = note.String
	}
	if reason.Valid {
		req.Reason = reason.String
	}
	return req, nil
}

func (r *joinRequestRepository) Update(ctx context.Context, req *domain.JoinRequest) error {
	query := `UPDATE join_requests SET status = $1, reason = $2 WHERE id = $3`
	_, err := r.db.ExecContext(ctx, query, req.Status, req.Reason, req.ID)
	return err
}

func (r *joinRequestRepository) ListByOrg(ctx context.Context, orgID int32) ([]domain.JoinRequest, error) {
	query := `
		SELECT jr.id, jr.org_id, jr.user_id, jr.name, jr.email, jr.note, jr.status, jr.created_on, i.used_on
		FROM join_requests jr
		LEFT JOIN LATERAL (
			SELECT used_on
			FROM invitations
			WHERE LOWER(invitations.email) = LOWER(jr.email)
			  AND invitations.org_id = jr.org_id
			  AND invitations.used_on IS NOT NULL
			ORDER BY 
			  CASE 
			    WHEN jr.user_id IS NOT NULL AND invitations.used_by_user_id = jr.user_id THEN 0
			    ELSE 1
			  END,
			  invitations.used_on DESC
			LIMIT 1
		) i ON true
		WHERE jr.org_id = $1
	`
	rows, err := r.db.QueryContext(ctx, query, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reqs []domain.JoinRequest
	for rows.Next() {
		var req domain.JoinRequest
		var createdOn time.Time
		var usedOn sql.NullTime
		if err := rows.Scan(&req.ID, &req.OrgID, &req.UserID, &req.Name, &req.Email, &req.Note, &req.Status, &createdOn, &usedOn); err != nil {
			return nil, err
		}
		req.CreatedOn = createdOn.Format("2006-01-02")
		if usedOn.Valid {
			dateStr := usedOn.Time.Format("2006-01-02")
			req.UsedOn = &dateStr
		}
		reqs = append(reqs, req)
	}
	return reqs, nil
}
