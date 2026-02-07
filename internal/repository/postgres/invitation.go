package postgres

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
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
	// Generate a unique invitation code for this email
	// Try up to 10 times to find a unique code
	var invitationCode string
	var err error
	maxAttempts := 10

	for attempt := 0; attempt < maxAttempts; attempt++ {
		invitationCode = generateInvitationCode()

		// Check if this code already exists for this email
		var exists bool
		checkQuery := `SELECT EXISTS(SELECT 1 FROM invitations WHERE invitation_code = $1 AND LOWER(email) = LOWER($2))`
		err = r.db.QueryRowContext(ctx, checkQuery, invitationCode, inv.Email).Scan(&exists)
		if err != nil {
			return err
		}

		// If code doesn't exist for this email, we can use it
		if !exists {
			break
		}

		// If this was the last attempt, return error
		if attempt == maxAttempts-1 {
			return errors.New("failed to generate unique invitation code after maximum attempts")
		}
	}

	query := `INSERT INTO invitations (invitation_code, org_id, email, created_by, expires_on, created_on) 
	          VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`
	err = r.db.QueryRowContext(ctx, query, invitationCode, inv.OrgID, inv.Email, inv.CreatedBy, inv.ExpiresOn, time.Now()).Scan(&inv.ID)
	if err != nil {
		return err
	}
	inv.InvitationCode = invitationCode
	return nil
}

func (r *invitationRepository) GetByInvitationCode(ctx context.Context, invitationCode string) (*domain.Invitation, error) {
	inv := &domain.Invitation{}
	query := `SELECT id, invitation_code, org_id, email, created_by, expires_on, used_on, used_by_user_id, created_on FROM invitations WHERE invitation_code = $1`
	err := r.db.QueryRowContext(ctx, query, invitationCode).Scan(&inv.ID, &inv.InvitationCode, &inv.OrgID, &inv.Email, &inv.CreatedBy, &inv.ExpiresOn, &inv.UsedOn, &inv.UsedByUserID, &inv.CreatedOn)
	if err != nil {
		return nil, err
	}
	return inv, nil
}

func (r *invitationRepository) GetByInvitationCodeAndEmail(ctx context.Context, invitationCode, email string) (*domain.Invitation, error) {
	inv := &domain.Invitation{}
	query := `SELECT id, invitation_code, org_id, email, created_by, expires_on, used_on, used_by_user_id, created_on 
	          FROM invitations 
	          WHERE invitation_code = $1 AND LOWER(email) = LOWER($2)`
	err := r.db.QueryRowContext(ctx, query, invitationCode, email).Scan(&inv.ID, &inv.InvitationCode, &inv.OrgID, &inv.Email, &inv.CreatedBy, &inv.ExpiresOn, &inv.UsedOn, &inv.UsedByUserID, &inv.CreatedOn)
	if err != nil {
		return nil, err
	}
	return inv, nil
}

func (r *invitationRepository) Update(ctx context.Context, inv *domain.Invitation) error {
	query := `UPDATE invitations SET used_on = $1, used_by_user_id = $2 WHERE id = $3`
	_, err := r.db.ExecContext(ctx, query, inv.UsedOn, inv.UsedByUserID, inv.ID)
	return err
}

// generateInvitationCode generates a cryptographically secure random invitation code
// Format: XXX-XXX-XXX (9 uppercase alphanumeric characters with dashes)
func generateInvitationCode() string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	const length = 9
	code := make([]byte, length)
	randomBytes := make([]byte, length)

	// Generate cryptographically secure random bytes
	_, err := rand.Read(randomBytes)
	if err != nil {
		// Fallback to time-based generation if crypto/rand fails
		for i := range code {
			code[i] = charset[time.Now().UnixNano()%int64(len(charset))]
		}
		return formatInvitationCode(string(code))
	}

	// Convert random bytes to characters from charset
	for i := range code {
		code[i] = charset[int(randomBytes[i])%len(charset)]
	}
	return formatInvitationCode(string(code))
}

// formatInvitationCode formats a 9-character code as XXX-XXX-XXX
func formatInvitationCode(code string) string {
	if len(code) != 9 {
		return code
	}
	return code[0:3] + "-" + code[3:6] + "-" + code[6:9]
}
