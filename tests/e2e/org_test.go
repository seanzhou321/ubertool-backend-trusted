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

	client := NewGRPCClient(t, "")
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

	t.Run("SearchOrganizations - Verify Admins Array Populated", func(t *testing.T) {
		// Setup: Create organization with admin users
		orgName := "E2E-Test-AdminCheck-Org"
		orgID := db.CreateTestOrg(orgName)

		// Create admin users with unique emails
		superAdminID := db.CreateTestUser("e2e-test-org-superadmin@test.com", "Super Admin User")
		adminID := db.CreateTestUser("e2e-test-org-admin@test.com", "Admin User")
		memberID := db.CreateTestUser("e2e-test-org-member@test.com", "Member User")

		// Add users to org with different roles
		db.AddUserToOrg(superAdminID, orgID, "SUPER_ADMIN", "ACTIVE", 0)
		db.AddUserToOrg(adminID, orgID, "ADMIN", "ACTIVE", 0)
		db.AddUserToOrg(memberID, orgID, "MEMBER", "ACTIVE", 0)

		// Log the setup for debugging
		t.Logf("Created org: ID=%d, Name=%s", orgID, orgName)
		t.Logf("Added SUPER_ADMIN user: ID=%d", superAdminID)
		t.Logf("Added ADMIN user: ID=%d", adminID)
		t.Logf("Added MEMBER user: ID=%d", memberID)

		// Test: Search for the organization
		ctx, cancel := ContextWithTimeout(5 * time.Second)
		defer cancel()

		req := &pb.SearchOrganizationsRequest{
			Name: orgName,
		}

		resp, err := orgClient.SearchOrganizations(ctx, req)
		require.NoError(t, err)
		t.Logf("SearchOrganizations returned %d organizations", len(resp.Organizations))

		// Find our test organization
		var testOrg *pb.Organization
		for _, org := range resp.Organizations {
			t.Logf("Found org: ID=%d, Name=%s, AdminCount=%d", org.Id, org.Name, len(org.Admins))
			if org.Id == orgID {
				testOrg = org
				break
			}
		}

		require.NotNil(t, testOrg, "Test organization not found in search results")
		t.Logf("Test organization found with %d admins", len(testOrg.Admins))

		// Verify: Admins array should contain 2 users (SUPER_ADMIN and ADMIN, not MEMBER)
		assert.Equal(t, 2, len(testOrg.Admins), "Should have exactly 2 admin users")

		// Verify admin user details
		adminEmails := make(map[string]bool)
		for _, admin := range testOrg.Admins {
			t.Logf("Admin: ID=%d, Name=%s, Email=%s", admin.Id, admin.Name, admin.Email)
			adminEmails[admin.Email] = true
		}

		assert.True(t, adminEmails["e2e-test-org-superadmin@test.com"], "Super admin should be in admins list")
		assert.True(t, adminEmails["e2e-test-org-admin@test.com"], "Admin should be in admins list")
		assert.False(t, adminEmails["e2e-test-org-member@test.com"], "Regular member should NOT be in admins list")
	})

	t.Run("SearchOrganizations - Check All Orgs for Admin Population", func(t *testing.T) {
		// Test: Search all organizations and log admin counts
		ctx, cancel := ContextWithTimeout(5 * time.Second)
		defer cancel()

		req := &pb.SearchOrganizationsRequest{}

		resp, err := orgClient.SearchOrganizations(ctx, req)
		require.NoError(t, err)
		t.Logf("Total organizations found: %d", len(resp.Organizations))

		orgsWithoutAdmins := 0
		orgsWithAdmins := 0

		for _, org := range resp.Organizations {
			if len(org.Admins) == 0 {
				orgsWithoutAdmins++
				t.Logf("Org without admins: ID=%d, Name=%s, Metro=%s", org.Id, org.Name, org.Metro)
			} else {
				orgsWithAdmins++
				t.Logf("Org with %d admins: ID=%d, Name=%s", len(org.Admins), org.Id, org.Name)
			}
		}

		t.Logf("Summary: %d orgs with admins, %d orgs without admins", orgsWithAdmins, orgsWithoutAdmins)
	})
}
