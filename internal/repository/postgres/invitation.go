package postgres

import (
	"context"
	"database/sql"
	"time"

	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/repository"
)

type invitationRepository struct {
	db *sql.DB
}

func NewInvitationRepository(db *sql.DB) repository.InvitationRepository {
	return &invitationRepository{db: db}
}

func (r *invitationRepository) Create(ctx context.Context, inv *domain.Invitation) error {
	query := `INSERT INTO invitations (org_id, email, created_by, expires_on, created_on) 
	          VALUES ($1, $2, $3, $4, $5) RETURNING token`
	return r.db.QueryRowContext(ctx, query, inv.OrgID, inv.Email, inv.CreatedBy, inv.ExpiresOn, time.Now()).Scan(&inv.Token)
}

func (r *invitationRepository) GetByToken(ctx context.Context, token string) (*domain.Invitation, error) {
	inv := &domain.Invitation{}
	query := `SELECT token, org_id, email, created_by, expires_on, used_on, used_by_user_id, created_on FROM invitations WHERE token = $1`
	err := r.db.QueryRowContext(ctx, query, token).Scan(&inv.Token, &inv.OrgID, &inv.Email, &inv.CreatedBy, &inv.ExpiresOn, &inv.UsedOn, &inv.UsedByUserID, &inv.CreatedOn)
	if err != nil {
		return nil, err
	}
	return inv, nil
}

func (r *invitationRepository) GetByTokenAndEmail(ctx context.Context, token, email string) (*domain.Invitation, error) {
	inv := &domain.Invitation{}
	query := `SELECT token, org_id, email, created_by, expires_on, used_on, used_by_user_id, created_on FROM invitations WHERE token = $1 AND email = $2`
	err := r.db.QueryRowContext(ctx, query, token, email).Scan(&inv.Token, &inv.OrgID, &inv.Email, &inv.CreatedBy, &inv.ExpiresOn, &inv.UsedOn, &inv.UsedByUserID, &inv.CreatedOn)
	if err != nil {
		return nil, err
	}
	return inv, nil
}

func (r *invitationRepository) Update(ctx context.Context, inv *domain.Invitation) error {
	query := `UPDATE invitations SET used_on = $1, used_by_user_id = $2 WHERE token = $3`
	_, err := r.db.ExecContext(ctx, query, inv.UsedOn, inv.UsedByUserID, inv.Token)
	return err
}
