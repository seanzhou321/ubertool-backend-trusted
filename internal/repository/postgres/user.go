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
	query := `INSERT INTO users_orgs (user_id, org_id, joined_on, balance_cents, status, role, blocked_date, block_reason) 
	          VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
	_, err := r.db.ExecContext(ctx, query, uo.UserID, uo.OrgID, time.Now(), uo.BalanceCents, uo.Status, uo.Role, uo.BlockedDate, uo.BlockReason)
	return err
}

func (r *userRepository) GetUserOrg(ctx context.Context, userID, orgID int32) (*domain.UserOrg, error) {
	uo := &domain.UserOrg{}
	query := `SELECT user_id, org_id, joined_on, balance_cents, status, role, blocked_date, block_reason FROM users_orgs WHERE user_id = $1 AND org_id = $2`
	err := r.db.QueryRowContext(ctx, query, userID, orgID).Scan(&uo.UserID, &uo.OrgID, &uo.JoinedOn, &uo.BalanceCents, &uo.Status, &uo.Role, &uo.BlockedDate, &uo.BlockReason)
	if err != nil {
		return nil, err
	}
	return uo, nil
}

func (r *userRepository) ListUserOrgs(ctx context.Context, userID int32) ([]domain.UserOrg, error) {
	query := `SELECT user_id, org_id, joined_on, balance_cents, status, role, blocked_date, block_reason FROM users_orgs WHERE user_id = $1`
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orgs []domain.UserOrg
	for rows.Next() {
		var uo domain.UserOrg
		if err := rows.Scan(&uo.UserID, &uo.OrgID, &uo.JoinedOn, &uo.BalanceCents, &uo.Status, &uo.Role, &uo.BlockedDate, &uo.BlockReason); err != nil {
			return nil, err
		}
		orgs = append(orgs, uo)
	}
	return orgs, nil
}

func (r *userRepository) UpdateUserOrg(ctx context.Context, uo *domain.UserOrg) error {
	query := `UPDATE users_orgs SET balance_cents=$1, status=$2, role=$3, blocked_date=$4, block_reason=$5 WHERE user_id=$6 AND org_id=$7`
	_, err := r.db.ExecContext(ctx, query, uo.BalanceCents, uo.Status, uo.Role, uo.BlockedDate, uo.BlockReason, uo.UserID, uo.OrgID)
	return err
}

func (r *userRepository) ListMembersByOrg(ctx context.Context, orgID int32) ([]domain.User, []domain.UserOrg, error) {
	query := `SELECT u.id, u.email, u.phone_number, u.password_hash, u.name, u.avatar_url, u.created_on, u.updated_on,
	                 uo.user_id, uo.org_id, uo.joined_on, uo.balance_cents, uo.status, uo.role, uo.blocked_date, uo.block_reason
	          FROM users u
	          JOIN users_orgs uo ON u.id = uo.user_id
	          WHERE uo.org_id = $1`
	rows, err := r.db.QueryContext(ctx, query, orgID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var users []domain.User
	var uos []domain.UserOrg
	for rows.Next() {
		var u domain.User
		var uo domain.UserOrg
		err := rows.Scan(
			&u.ID, &u.Email, &u.PhoneNumber, &u.PasswordHash, &u.Name, &u.AvatarURL, &u.CreatedOn, &u.UpdatedOn,
			&uo.UserID, &uo.OrgID, &uo.JoinedOn, &uo.BalanceCents, &uo.Status, &uo.Role, &uo.BlockedDate, &uo.BlockReason,
		)
		if err != nil {
			return nil, nil, err
		}
		users = append(users, u)
		uos = append(uos, uo)
	}
	return users, uos, nil
}

func (r *userRepository) SearchMembersByOrg(ctx context.Context, orgID int32, query string) ([]domain.User, []domain.UserOrg, error) {
	sqlQuery := `SELECT u.id, u.email, u.phone_number, u.password_hash, u.name, u.avatar_url, u.created_on, u.updated_on,
	                 uo.user_id, uo.org_id, uo.joined_on, uo.balance_cents, uo.status, uo.role, uo.blocked_date, uo.block_reason
	          FROM users u
	          JOIN users_orgs uo ON u.id = uo.user_id
	          WHERE uo.org_id = $1 AND (u.name ILIKE $2 OR u.email ILIKE $2)`
	rows, err := r.db.QueryContext(ctx, sqlQuery, orgID, "%"+query+"%")
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var users []domain.User
	var uos []domain.UserOrg
	for rows.Next() {
		var u domain.User
		var uo domain.UserOrg
		err := rows.Scan(
			&u.ID, &u.Email, &u.PhoneNumber, &u.PasswordHash, &u.Name, &u.AvatarURL, &u.CreatedOn, &u.UpdatedOn,
			&uo.UserID, &uo.OrgID, &uo.JoinedOn, &uo.BalanceCents, &uo.Status, &uo.Role, &uo.BlockedDate, &uo.BlockReason,
		)
		if err != nil {
			return nil, nil, err
		}
		users = append(users, u)
		uos = append(uos, uo)
	}
	return users, uos, nil
}
