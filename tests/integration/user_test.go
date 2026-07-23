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
	"github.com/stretchr/testify/require"
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

	// FR-004 (specs/002-users): UpdateProfile (via userRepository.Update) must reject a write
	// that would violate the users.email UNIQUE constraint. Previously this rejection was only
	// verified at e2e via a generic assert.Error — this isolates the real Postgres constraint
	// violation directly against the repository, and confirms the first user's row is untouched.
	t.Run("Update rejects a duplicate email", func(t *testing.T) {
		ts := time.Now().UnixNano()
		email1 := fmt.Sprintf("test-integration-unique-1-%d@test.com", ts)
		email2 := fmt.Sprintf("test-integration-unique-2-%d@test.com", ts)

		u1 := &domain.User{Email: email1, PhoneNumber: fmt.Sprintf("%d1", ts), PasswordHash: "hash", Name: "User One"}
		require.NoError(t, repo.Create(ctx, u1))

		u2 := &domain.User{Email: email2, PhoneNumber: fmt.Sprintf("%d2", ts), PasswordHash: "hash", Name: "User Two"}
		require.NoError(t, repo.Create(ctx, u2))

		u2.Email = email1 // collides with u1's exact-case email
		err := repo.Update(ctx, u2)
		require.Error(t, err, "Update must reject a write that violates the users.email UNIQUE constraint")

		// u1's row must be untouched by the failed attempt.
		fetched1, err := repo.GetByID(ctx, u1.ID)
		require.NoError(t, err)
		assert.Equal(t, email1, fetched1.Email)

		fetched2, err := repo.GetByID(ctx, u2.ID)
		require.NoError(t, err)
		assert.Equal(t, email2, fetched2.Email, "u2's email must remain unchanged after the rejected update")
	})
}
