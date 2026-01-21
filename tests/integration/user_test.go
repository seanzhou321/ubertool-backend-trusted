package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/repository/postgres"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
)

func TestUserRepository_Integration(t *testing.T) {
	db := prepareDB(t)
	defer db.Close()

	repo := postgres.NewUserRepository(db)
	ctx := context.Background()

	// Clean up before tests
	db.Exec("DELETE FROM users WHERE email LIKE 'test-integration-%'")

	t.Run("Create and Get", func(t *testing.T) {
		email := fmt.Sprintf("test-integration-%d@test.com", time.Now().UnixNano())
		u := &domain.User{
			Email:        email,
			PhoneNumber:  fmt.Sprintf("%d", time.Now().UnixNano()),
			PasswordHash: "hash",
			Name:         "Integration User",
		}

		err := repo.Create(ctx, u)
		assert.NoError(t, err)
		assert.NotZero(t, u.ID)

		fetched, err := repo.GetByID(ctx, u.ID)
		assert.NoError(t, err)
		assert.Equal(t, u.Email, fetched.Email)
		assert.Equal(t, u.Name, fetched.Name)
	})

	t.Run("GetByEmail", func(t *testing.T) {
		email := fmt.Sprintf("test-integration-email-%d@test.com", time.Now().UnixNano())
		u := &domain.User{
			Email:        email,
			PhoneNumber:  fmt.Sprintf("%d", time.Now().UnixNano()),
			PasswordHash: "hash",
			Name:         "Email User",
		}

		repo.Create(ctx, u)

		fetched, err := repo.GetByEmail(ctx, email)
		assert.NoError(t, err)
		assert.Equal(t, u.ID, fetched.ID)
	})

	t.Run("Update", func(t *testing.T) {
		email := fmt.Sprintf("test-integration-update-%d@test.com", time.Now().UnixNano())
		u := &domain.User{
			Email:        email,
			PhoneNumber:  fmt.Sprintf("%d", time.Now().UnixNano()),
			PasswordHash: "hash",
			Name:         "Original Name",
		}
		repo.Create(ctx, u)

		u.Name = "Updated Name"
		err := repo.Update(ctx, u)
		assert.NoError(t, err)

		fetched, _ := repo.GetByID(ctx, u.ID)
		assert.Equal(t, "Updated Name", fetched.Name)
	})
}
