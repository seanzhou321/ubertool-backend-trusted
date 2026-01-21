package e2e

import (
	"testing"
	"time"

	pb "ubertool-backend-trusted/api/gen/v1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrganizationService_E2E(t *testing.T) {
	db := PrepareDB(t)
	defer db.Close()
	defer db.Cleanup()

	client := NewGRPCClient(t, "localhost:50051")
	defer client.Close()

	orgClient := pb.NewOrganizationServiceClient(client.Conn())

	t.Run("CreateOrganization with SUPER_ADMIN Assignment", func(t *testing.T) {
		// Setup: Create user
		userID := db.CreateTestUser("e2e-test-creator@test.com", "Creator User")

		// Test: Create organization
		ctx, cancel := ContextWithUserIDAndTimeout(userID, 5*time.Second)
		defer cancel()

		req := &pb.CreateOrganizationRequest{
			Name:        "E2E Test Organization",
			Description: "Test organization for E2E tests",
			Address:     "123 Test St",
			Metro:       "San Jose",
			AdminEmail:  "admin@e2etest.com",
			AdminPhone:  "555-9999",
		}

		resp, err := orgClient.CreateOrganization(ctx, req)
		require.NoError(t, err)
		assert.NotNil(t, resp.Organization)
		assert.Greater(t, resp.Organization.Id, int32(0))

		// Verify: Creator is SUPER_ADMIN
		var role string
		err = db.QueryRow("SELECT role FROM users_orgs WHERE user_id = $1 AND org_id = $2", userID, resp.Organization.Id).Scan(&role)
		assert.NoError(t, err)
		assert.Equal(t, "SUPER_ADMIN", role)
	})

	t.Run("ListMyOrganizations", func(t *testing.T) {
		// Setup: Create user and add to multiple orgs
		userID := db.CreateTestUser("e2e-test-member@test.com", "Member User")
		org1ID := db.CreateTestOrg("E2E-Test-Org-1")
		org2ID := db.CreateTestOrg("E2E-Test-Org-2")
		db.AddUserToOrg(userID, org1ID, "MEMBER", "ACTIVE", 1000)
		db.AddUserToOrg(userID, org2ID, "ADMIN", "ACTIVE", 2000)

		// Test: List my organizations
		ctx, cancel := ContextWithUserIDAndTimeout(userID, 5*time.Second)
		defer cancel()

		req := &pb.ListMyOrganizationsRequest{}
		resp, err := orgClient.ListMyOrganizations(ctx, req)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(resp.Organizations), 2)

		// Verify: Organizations include user-specific data
		foundOrg1 := false
		foundOrg2 := false
		for _, org := range resp.Organizations {
			if org.Id == org1ID {
				foundOrg1 = true
				assert.Equal(t, int32(1000), org.UserBalance)
			}
			if org.Id == org2ID {
				foundOrg2 = true
				assert.Equal(t, int32(2000), org.UserBalance)
			}
		}
		assert.True(t, foundOrg1, "Organization 1 should be in the list")
		assert.True(t, foundOrg2, "Organization 2 should be in the list")
	})

	t.Run("SearchOrganizations", func(t *testing.T) {
		// Setup: Create test organizations
		db.CreateTestOrg("E2E-Test-SearchOrg-SanJose")
		db.CreateTestOrg("E2E-Test-SearchOrg-Oakland")

		// Test: Search by metro
		ctx, cancel := ContextWithTimeout(5 * time.Second)
		defer cancel()

		req := &pb.SearchOrganizationsRequest{
			Metro: "San Jose",
		}

		resp, err := orgClient.SearchOrganizations(ctx, req)
		require.NoError(t, err)
		assert.Greater(t, len(resp.Organizations), 0)
	})
}
