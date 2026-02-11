package repos

import (
	"context"
	"testing"
	"time"

	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/repository/postgres"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func TestUserRepository_GetByID(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	repo := postgres.NewUserRepository(db)
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"id", "email", "phone_number", "password_hash", "name", "avatar_url", "created_on", "updated_on"}).
			AddRow(1, "test@test.com", "123", "hash", "Name", "url", time.Now(), time.Now())

		mock.ExpectQuery("SELECT (.+) FROM users WHERE id = \\$1").
			WithArgs(int32(1)).
			WillReturnRows(rows)

		user, err := repo.GetByID(ctx, 1)
		assert.NoError(t, err)
		assert.NotNil(t, user)
		assert.Equal(t, int32(1), user.ID)
	})

	t.Run("NotFound", func(t *testing.T) {
		mock.ExpectQuery("SELECT (.+) FROM users WHERE id = \\$1").
			WithArgs(int32(2)).
			WillReturnError(assert.AnError)

		user, err := repo.GetByID(ctx, 2)
		assert.Error(t, err)
		assert.Nil(t, user)
	})
}

func TestUserRepository_Create(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	repo := postgres.NewUserRepository(db)
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		u := &domain.User{
			Email:        "new@test.com",
			PhoneNumber:  "456",
			PasswordHash: "hash",
			Name:         "New User",
		}

		mock.ExpectQuery("INSERT INTO users").
			WithArgs(u.Email, u.PhoneNumber, u.PasswordHash, u.Name, u.AvatarURL, sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

		err := repo.Create(ctx, u)
		assert.NoError(t, err)
		assert.Equal(t, int32(1), u.ID)
	})
}
func TestUserRepository_ListMembersByOrg(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	repo := postgres.NewUserRepository(db)
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"id", "email", "phone_number", "password_hash", "name", "avatar_url", "created_on", "updated_on",
			"user_id", "org_id", "joined_on", "balance_cents", "last_balance_updated_on", "status", "role", "blocked_on", "blocked_reason",
			"renting_blocked", "lending_blocked", "blocked_due_to_bill_id"}).
			AddRow(1, "u1@test.com", "111", "hash", "User 1", "url", time.Now(), time.Now(), 1, 1, time.Now(), 100, nil, "ACTIVE", "MEMBER", nil, "", false, false, nil)

		mock.ExpectQuery("SELECT (.+) FROM users u JOIN users_orgs uo").
			WithArgs(int32(1)).
			WillReturnRows(rows)

		users, uos, err := repo.ListMembersByOrg(ctx, 1)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(users))
		assert.Equal(t, int32(1), users[0].ID)
		assert.Equal(t, int32(100), uos[0].BalanceCents)
	})
}

func TestUserRepository_SearchMembersByOrg(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	repo := postgres.NewUserRepository(db)
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"id", "email", "phone_number", "password_hash", "name", "avatar_url", "created_on", "updated_on",
			"user_id", "org_id", "joined_on", "balance_cents", "last_balance_updated_on", "status", "role", "blocked_on", "blocked_reason",
			"renting_blocked", "lending_blocked", "blocked_due_to_bill_id"}).
			AddRow(1, "u1@test.com", "111", "hash", "User 1", "url", time.Now(), time.Now(), 1, 1, time.Now(), 100, nil, "ACTIVE", "MEMBER", nil, "", false, false, nil)

		mock.ExpectQuery("SELECT (.+) FROM users u JOIN users_orgs uo").
			WithArgs(int32(1), "%search%").
			WillReturnRows(rows)

		users, _, err := repo.SearchMembersByOrg(ctx, 1, "search")
		assert.NoError(t, err)
		assert.Equal(t, 1, len(users))
	})
}
