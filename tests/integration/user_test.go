package integration

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/repository/postgres"
	"github.com/stretchr/testify/assert"
)

func prepareDB(t *testing.T) *sql.DB {
	connStr := "postgres://ubertool_trusted:ubertool123@localhost:5432/ubertool_db?sslmode=disable"
	var db *sql.DB
	var err error

	// Retry connection as DB might still be starting up
	for i := 0; i < 10; i++ {
		db, err = sql.Open("postgres", connStr)
		if err == nil {
			err = db.Ping()
			if err == nil {
				return db
			}
		}
		time.Sleep(2 * time.Second)
	}
	t.Fatalf("failed to connect to database: %v", err)
	return nil
}

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
			Email:       email,
			PhoneNumber: fmt.Sprintf("%d", time.Now().UnixNano()),
			PasswordHash: "hash",
			Name:        "Integration User",
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
			Email:       email,
			PhoneNumber: fmt.Sprintf("%d", time.Now().UnixNano()),
			PasswordHash: "hash",
			Name:        "Email User",
		}

		repo.Create(ctx, u)

		fetched, err := repo.GetByEmail(ctx, email)
		assert.NoError(t, err)
		assert.Equal(t, u.ID, fetched.ID)
	})

	t.Run("Update", func(t *testing.T) {
		email := fmt.Sprintf("test-integration-update-%d@test.com", time.Now().UnixNano())
		u := &domain.User{
			Email:       email,
			PhoneNumber: fmt.Sprintf("%d", time.Now().UnixNano()),
			PasswordHash: "hash",
			Name:        "Original Name",
		}
		repo.Create(ctx, u)

		u.Name = "Updated Name"
		err := repo.Update(ctx, u)
		assert.NoError(t, err)

		fetched, _ := repo.GetByID(ctx, u.ID)
		assert.Equal(t, "Updated Name", fetched.Name)
	})
}
