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
			Email:       "new@test.com",
			PhoneNumber: "456",
			PasswordHash: "hash",
			Name:        "New User",
		}

		mock.ExpectQuery("INSERT INTO users").
			WithArgs(u.Email, u.PhoneNumber, u.PasswordHash, u.Name, u.AvatarURL, sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

		err := repo.Create(ctx, u)
		assert.NoError(t, err)
		assert.Equal(t, int32(1), u.ID)
	})
}
