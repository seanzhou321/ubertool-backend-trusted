package repos

import (
	"context"
	"testing"

	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/repository/postgres"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func TestOrganizationRepository_Update(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	repo := postgres.NewOrganizationRepository(db)
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		org := &domain.Organization{
			ID:               1,
			Name:             "Updated Org",
			Description:      "New Desc",
			AdminEmail:       "admin@test.com",
			AdminPhoneNumber: "123",
		}

		mock.ExpectExec("UPDATE orgs SET").
			WithArgs(org.Name, org.Description, org.Address, org.Metro, org.AdminPhoneNumber, org.AdminEmail, org.ID).
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := repo.Update(ctx, org)
		assert.NoError(t, err)
	})
}
