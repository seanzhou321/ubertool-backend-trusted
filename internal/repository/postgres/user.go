package postgres

import (
	"context"
	"database/sql"
	"time"

	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/logger"
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
	now := time.Now().Format("2006-01-02")
	u.CreatedOn = now
	u.UpdatedOn = now
	return r.db.QueryRowContext(ctx, query, u.Email, u.PhoneNumber, u.PasswordHash, u.Name, u.AvatarURL, u.CreatedOn, u.UpdatedOn).Scan(&u.ID)
}

func (r *userRepository) GetByID(ctx context.Context, id int32) (*domain.User, error) {
	u := &domain.User{}
	query := `SELECT id, email, phone_number, password_hash, name, COALESCE(avatar_url, ''), created_on, updated_on FROM users WHERE id = $1`
	var createdOn, updatedOn time.Time
	err := r.db.QueryRowContext(ctx, query, id).Scan(&u.ID, &u.Email, &u.PhoneNumber, &u.PasswordHash, &u.Name, &u.AvatarURL, &createdOn, &updatedOn)
	if err != nil {
		return nil, err
	}
	u.CreatedOn = createdOn.Format("2006-01-02")
	u.UpdatedOn = updatedOn.Format("2006-01-02")
	return u, nil
}

func (r *userRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	u := &domain.User{}
	query := `SELECT id, email, phone_number, password_hash, name, COALESCE(avatar_url, ''), created_on, updated_on FROM users WHERE LOWER(email) = LOWER($1)`
	var createdOn, updatedOn time.Time
	err := r.db.QueryRowContext(ctx, query, email).Scan(&u.ID, &u.Email, &u.PhoneNumber, &u.PasswordHash, &u.Name, &u.AvatarURL, &createdOn, &updatedOn)
	if err != nil {
		return nil, err
	}
	u.CreatedOn = createdOn.Format("2006-01-02")
	u.UpdatedOn = updatedOn.Format("2006-01-02")
	return u, nil
}

func (r *userRepository) Update(ctx context.Context, u *domain.User) error {
	query := `UPDATE users SET email=$1, phone_number=$2, name=$3, avatar_url=$4, updated_on=$5 WHERE id=$6`
	now := time.Now().Format("2006-01-02")
	u.UpdatedOn = now
	_, err := r.db.ExecContext(ctx, query, u.Email, u.PhoneNumber, u.Name, u.AvatarURL, u.UpdatedOn, u.ID)
	return err
}

func (r *userRepository) AddUserToOrg(ctx context.Context, uo *domain.UserOrg) error {
	query := `INSERT INTO users_orgs (user_id, org_id, joined_on, balance_cents, last_balance_updated_on, status, role, blocked_on, blocked_reason, renting_blocked, lending_blocked, blocked_due_to_bill_id) 
	          VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`
	now := time.Now().Format("2006-01-02")
	uo.JoinedOn = now
	_, err := r.db.ExecContext(ctx, query, uo.UserID, uo.OrgID, uo.JoinedOn, uo.BalanceCents, uo.LastBalanceUpdateOn, uo.Status, uo.Role, uo.BlockedOn, uo.BlockedReason, uo.RentingBlocked, uo.LendingBlocked, uo.BlockedDueToBillID)
	return err
}

func (r *userRepository) GetUserOrg(ctx context.Context, userID, orgID int32) (*domain.UserOrg, error) {
	uo := &domain.UserOrg{}
	query := `SELECT user_id, org_id, joined_on, balance_cents, last_balance_updated_on, status, role, blocked_on, COALESCE(blocked_reason, ''), renting_blocked, lending_blocked, blocked_due_to_bill_id FROM users_orgs WHERE user_id = $1 AND org_id = $2`

	var lastBalanceUpdateOn sql.NullTime
	var blockedDate sql.NullTime
	var joinedOn time.Time

	err := r.db.QueryRowContext(ctx, query, userID, orgID).Scan(
		&uo.UserID, &uo.OrgID, &joinedOn, &uo.BalanceCents, &lastBalanceUpdateOn,
		&uo.Status, &uo.Role, &blockedDate, &uo.BlockedReason, &uo.RentingBlocked,
		&uo.LendingBlocked, &uo.BlockedDueToBillID,
	)
	if err != nil {
		return nil, err
	}
	uo.JoinedOn = joinedOn.Format("2006-01-02")

	if lastBalanceUpdateOn.Valid {
		dateStr := lastBalanceUpdateOn.Time.Format("2006-01-02")
		uo.LastBalanceUpdateOn = &dateStr
	}
	if blockedDate.Valid {
		dateStr := blockedDate.Time.Format("2006-01-02")
		uo.BlockedOn = &dateStr
	}

	return uo, nil
}

func (r *userRepository) ListUserOrgs(ctx context.Context, userID int32) ([]domain.UserOrg, error) {
	query := `SELECT user_id, org_id, joined_on, balance_cents, last_balance_updated_on, status, role, blocked_on, COALESCE(blocked_reason, ''), renting_blocked, lending_blocked, blocked_due_to_bill_id FROM users_orgs WHERE user_id = $1`
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orgs []domain.UserOrg
	for rows.Next() {
		var uo domain.UserOrg
		var joinedOn time.Time
		var lastBalanceUpdateOn sql.NullTime
		var blockedDate sql.NullTime

		if err := rows.Scan(
			&uo.UserID, &uo.OrgID, &joinedOn, &uo.BalanceCents, &lastBalanceUpdateOn,
			&uo.Status, &uo.Role, &blockedDate, &uo.BlockedReason, &uo.RentingBlocked,
			&uo.LendingBlocked, &uo.BlockedDueToBillID,
		); err != nil {
			return nil, err
		}
		uo.JoinedOn = joinedOn.Format("2006-01-02")

		if lastBalanceUpdateOn.Valid {
			dateStr := lastBalanceUpdateOn.Time.Format("2006-01-02")
			uo.LastBalanceUpdateOn = &dateStr
		}
		if blockedDate.Valid {
			dateStr := blockedDate.Time.Format("2006-01-02")
			uo.BlockedOn = &dateStr
		}

		orgs = append(orgs, uo)
	}
	return orgs, nil
}

func (r *userRepository) UpdateUserOrg(ctx context.Context, uo *domain.UserOrg) error {
	query := `UPDATE users_orgs SET balance_cents=$1, last_balance_updated_on=$2, status=$3, role=$4, blocked_on=$5, blocked_reason=$6, renting_blocked=$7, lending_blocked=$8, blocked_due_to_bill_id=$9 WHERE user_id=$10 AND org_id=$11`

	var lastBalanceUpdateOn interface{} = nil
	if uo.LastBalanceUpdateOn != nil {
		lastBalanceUpdateOn = *uo.LastBalanceUpdateOn
	}

	_, err := r.db.ExecContext(ctx, query, uo.BalanceCents, lastBalanceUpdateOn, uo.Status, uo.Role, uo.BlockedOn, uo.BlockedReason, uo.RentingBlocked, uo.LendingBlocked, uo.BlockedDueToBillID, uo.UserID, uo.OrgID)
	return err
}

func (r *userRepository) ListMembersByOrg(ctx context.Context, orgID int32) ([]domain.User, []domain.UserOrg, error) {
	logger.EnterMethod("userRepository.ListMembersByOrg", "orgID", orgID)

	query := `SELECT u.id, u.email, u.phone_number, u.password_hash, u.name, COALESCE(u.avatar_url, ''), u.created_on, u.updated_on,
	                 uo.user_id, uo.org_id, uo.joined_on, uo.balance_cents, uo.last_balance_updated_on, uo.status, uo.role, uo.blocked_on, COALESCE(uo.blocked_reason, ''), uo.renting_blocked, uo.lending_blocked, uo.blocked_due_to_bill_id
	          FROM users u
	          JOIN users_orgs uo ON u.id = uo.user_id
	          WHERE uo.org_id = $1`
	logger.DatabaseCall("SELECT", "users JOIN users_orgs", "orgID", orgID)

	rows, err := r.db.QueryContext(ctx, query, orgID)
	if err != nil {
		logger.DatabaseResult("SELECT", 0, err, "orgID", orgID)
		logger.ExitMethodWithError("userRepository.ListMembersByOrg", err, "orgID", orgID)
		return nil, nil, err
	}
	defer rows.Close()

	var users []domain.User
	var uos []domain.UserOrg
	for rows.Next() {
		var u domain.User
		var uo domain.UserOrg
		var createdOn, updatedOn, joinedOn time.Time
		var lastBalanceUpdateOn sql.NullTime
		var blockedDate sql.NullTime

		err := rows.Scan(
			&u.ID, &u.Email, &u.PhoneNumber, &u.PasswordHash, &u.Name, &u.AvatarURL, &createdOn, &updatedOn,
			&uo.UserID, &uo.OrgID, &joinedOn, &uo.BalanceCents, &lastBalanceUpdateOn,
			&uo.Status, &uo.Role, &blockedDate, &uo.BlockedReason, &uo.RentingBlocked,
			&uo.LendingBlocked, &uo.BlockedDueToBillID,
		)
		if err != nil {
			logger.DatabaseResult("SELECT", int64(len(users)), err, "orgID", orgID)
			logger.ExitMethodWithError("userRepository.ListMembersByOrg", err, "orgID", orgID)
			return nil, nil, err
		}
		u.CreatedOn = createdOn.Format("2006-01-02")
		u.UpdatedOn = updatedOn.Format("2006-01-02")
		uo.JoinedOn = joinedOn.Format("2006-01-02")

		if lastBalanceUpdateOn.Valid {
			dateStr := lastBalanceUpdateOn.Time.Format("2006-01-02")
			uo.LastBalanceUpdateOn = &dateStr
		}
		if blockedDate.Valid {
			dateStr := blockedDate.Time.Format("2006-01-02")
			uo.BlockedOn = &dateStr
		}

		users = append(users, u)
		uos = append(uos, uo)
	}

	logger.DatabaseResult("SELECT", int64(len(users)), nil, "orgID", orgID, "membersFound", len(users))
	logger.ExitMethod("userRepository.ListMembersByOrg", "orgID", orgID, "count", len(users))
	return users, uos, nil
}

func (r *userRepository) CountMembersByOrg(ctx context.Context, orgID int32) (int32, error) {
	query := `SELECT COUNT(*) FROM users_orgs WHERE org_id = $1 AND status != 'BLOCK'`
	var count int32
	err := r.db.QueryRowContext(ctx, query, orgID).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (r *userRepository) SearchMembersByOrg(ctx context.Context, orgID int32, query string) ([]domain.User, []domain.UserOrg, error) {
	sqlQuery := `SELECT u.id, u.email, u.phone_number, u.password_hash, u.name, COALESCE(u.avatar_url, ''), u.created_on, u.updated_on,
	                 uo.user_id, uo.org_id, uo.joined_on, uo.balance_cents, uo.last_balance_updated_on, uo.status, uo.role, uo.blocked_on, COALESCE(uo.blocked_reason, ''), uo.renting_blocked, uo.lending_blocked, uo.blocked_due_to_bill_id
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
		var lastBalanceUpdateOn sql.NullTime
		var blockedDate sql.NullTime
		var createdOn, updatedOn, joinedOn time.Time

		err := rows.Scan(
			&u.ID, &u.Email, &u.PhoneNumber, &u.PasswordHash, &u.Name, &u.AvatarURL, &createdOn, &updatedOn,
			&uo.UserID, &uo.OrgID, &joinedOn, &uo.BalanceCents, &lastBalanceUpdateOn, &uo.Status, &uo.Role, &blockedDate, &uo.BlockedReason, &uo.RentingBlocked, &uo.LendingBlocked, &uo.BlockedDueToBillID,
		)
		if err != nil {
			return nil, nil, err
		}
		u.CreatedOn = createdOn.Format("2006-01-02")
		u.UpdatedOn = updatedOn.Format("2006-01-02")
		uo.JoinedOn = joinedOn.Format("2006-01-02")

		if lastBalanceUpdateOn.Valid {
			dateStr := lastBalanceUpdateOn.Time.Format("2006-01-02")
			uo.LastBalanceUpdateOn = &dateStr
		}
		if blockedDate.Valid {
			dateStr := blockedDate.Time.Format("2006-01-02")
			uo.BlockedOn = &dateStr
		}

		users = append(users, u)
		uos = append(uos, uo)
	}
	return users, uos, nil
}
