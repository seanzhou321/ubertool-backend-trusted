package postgres

import (
	"context"
	"database/sql"
	"time"

	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/repository"
)

type userRepository struct {
	db *sql.DB
}


func NewUserRepository(db *sql.DB) repository.UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) Create(ctx context.Context, u *domain.User) error {
	query := `INSERT INTO users (email, phone_number, password_hash, name, avatar_url, created_on, updated_on) 
	          VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`
	return r.db.QueryRowContext(ctx, query, u.Email, u.PhoneNumber, u.PasswordHash, u.Name, u.AvatarURL, time.Now(), time.Now()).Scan(&u.ID)
}

func (r *userRepository) GetByID(ctx context.Context, id int32) (*domain.User, error) {
	u := &domain.User{}
	query := `SELECT id, email, phone_number, password_hash, name, avatar_url, created_on, updated_on FROM users WHERE id = $1`
	err := r.db.QueryRowContext(ctx, query, id).Scan(&u.ID, &u.Email, &u.PhoneNumber, &u.PasswordHash, &u.Name, &u.AvatarURL, &u.CreatedOn, &u.UpdatedOn)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (r *userRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	u := &domain.User{}
	query := `SELECT id, email, phone_number, password_hash, name, avatar_url, created_on, updated_on FROM users WHERE email = $1`
	err := r.db.QueryRowContext(ctx, query, email).Scan(&u.ID, &u.Email, &u.PhoneNumber, &u.PasswordHash, &u.Name, &u.AvatarURL, &u.CreatedOn, &u.UpdatedOn)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (r *userRepository) Update(ctx context.Context, u *domain.User) error {
	query := `UPDATE users SET email=$1, phone_number=$2, name=$3, avatar_url=$4, updated_on=$5 WHERE id=$6`
	_, err := r.db.ExecContext(ctx, query, u.Email, u.PhoneNumber, u.Name, u.AvatarURL, time.Now(), u.ID)
	return err
}

func (r *userRepository) AddUserToOrg(ctx context.Context, uo *domain.UserOrg) error {
	query := `INSERT INTO users_orgs (user_id, org_id, joined_on, balance_cents, status, role) 
	          VALUES ($1, $2, $3, $4, $5, $6)`
	_, err := r.db.ExecContext(ctx, query, uo.UserID, uo.OrgID, time.Now(), uo.BalanceCents, uo.Status, uo.Role)
	return err
}

func (r *userRepository) GetUserOrg(ctx context.Context, userID, orgID int32) (*domain.UserOrg, error) {
	uo := &domain.UserOrg{}
	query := `SELECT user_id, org_id, joined_on, balance_cents, status, role FROM users_orgs WHERE user_id = $1 AND org_id = $2`
	err := r.db.QueryRowContext(ctx, query, userID, orgID).Scan(&uo.UserID, &uo.OrgID, &uo.JoinedOn, &uo.BalanceCents, &uo.Status, &uo.Role)
	if err != nil {
		return nil, err
	}
	return uo, nil
}

func (r *userRepository) ListUserOrgs(ctx context.Context, userID int32) ([]domain.UserOrg, error) {
	query := `SELECT user_id, org_id, joined_on, balance_cents, status, role FROM users_orgs WHERE user_id = $1`
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orgs []domain.UserOrg
	for rows.Next() {
		var uo domain.UserOrg
		if err := rows.Scan(&uo.UserID, &uo.OrgID, &uo.JoinedOn, &uo.BalanceCents, &uo.Status, &uo.Role); err != nil {
			return nil, err
		}
		orgs = append(orgs, uo)
	}
	return orgs, nil
}

func (r *userRepository) UpdateUserOrg(ctx context.Context, uo *domain.UserOrg) error {
	query := `UPDATE users_orgs SET balance_cents=$1, status=$2, role=$3 WHERE user_id=$4 AND org_id=$5`
	_, err := r.db.ExecContext(ctx, query, uo.BalanceCents, uo.Status, uo.Role, uo.UserID, uo.OrgID)
	return err
}
