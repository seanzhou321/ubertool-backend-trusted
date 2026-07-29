package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/repository/postgres"
	"ubertool-backend-trusted/internal/service"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrganizationService_ListMyOrganizations_MultiOrgMembership(t *testing.T) {
	db := prepareDB(t)
	defer db.Close()

	// Initialize repositories
	orgRepo := postgres.NewOrganizationRepository(db)
	userRepo := postgres.NewUserRepository(db)
	inviteRepo := postgres.NewInvitationRepository(db)

	// Create organization service with nil services
	orgSvc := service.NewOrganizationService(orgRepo, userRepo, inviteRepo, nil, nil, nil)

	ctx := context.Background()

	t.Run("User can belong to multiple organizations", func(t *testing.T) {
		// Clean up test data
		ts := time.Now().UnixNano()
		testPrefix := fmt.Sprintf("multi-org-test-%d", ts)
		db.Exec("DELETE FROM users_orgs WHERE user_id IN (SELECT id FROM users WHERE email LIKE $1)", testPrefix+"-%@test.com")
		db.Exec("DELETE FROM users WHERE email LIKE $1", testPrefix+"-%@test.com")
		db.Exec("DELETE FROM orgs WHERE name LIKE $1", testPrefix+"-%")

		// Create two test organizations
		var orgID1, orgID2 int32
		err := db.QueryRow(`INSERT INTO orgs (name, metro, admin_email, admin_phone_number, address) 
			VALUES ($1, 'San Jose', 'admin@test.com', '555-0000', '123 Test St') RETURNING id`,
			fmt.Sprintf("%s-1", testPrefix)).Scan(&orgID1)
		require.NoError(t, err)

		err = db.QueryRow(`INSERT INTO orgs (name, metro, admin_email, admin_phone_number, address) 
			VALUES ($1, 'San Francisco', 'admin@test.com', '555-0000', '456 Test Ave') RETURNING id`,
			fmt.Sprintf("%s-2", testPrefix)).Scan(&orgID2)
		require.NoError(t, err)
		t.Logf("Created orgs: orgID1=%d, orgID2=%d", orgID1, orgID2)

		// Create test user
		email := fmt.Sprintf("%s-user@test.com", testPrefix)
		var userID int32
		err = db.QueryRow(`INSERT INTO users (email, phone_number, password_hash, name) 
			VALUES ($1, '555-1111', 'hash', 'Multi Org User') RETURNING id`, email).Scan(&userID)
		require.NoError(t, err)
		t.Logf("Created user: userID=%d", userID)

		// Add user to first org as MEMBER
		_, err = db.Exec(`INSERT INTO users_orgs (user_id, org_id, role, status, joined_on, balance_cents) 
			VALUES ($1, $2, 'MEMBER', 'ACTIVE', CURRENT_DATE, 0)`, userID, orgID1)
		require.NoError(t, err)

		// Add user to second org as ADMIN
		_, err = db.Exec(`INSERT INTO users_orgs (user_id, org_id, role, status, joined_on, balance_cents) 
			VALUES ($1, $2, 'ADMIN', 'ACTIVE', CURRENT_DATE, 0)`, userID, orgID2)
		require.NoError(t, err)

		// Call ListMyOrganizations
		orgs, userOrgs, err := orgSvc.ListMyOrganizations(ctx, userID)
		require.NoError(t, err)

		// Verify we get both organizations
		require.Len(t, orgs, 2, "Should return 2 organizations")
		require.Len(t, userOrgs, 2, "Should return 2 user-org relationships")

		// Verify both orgs are returned with correct member counts
		orgMap := make(map[int32]domain.Organization)
		for _, org := range orgs {
			orgMap[org.ID] = org
		}

		// Check first org
		org1, ok := orgMap[orgID1]
		require.True(t, ok, "Should include first org")
		assert.GreaterOrEqual(t, org1.MemberCount, int32(1), "First org should have at least 1 member")

		// Check second org
		org2, ok := orgMap[orgID2]
		require.True(t, ok, "Should include second org")
		assert.GreaterOrEqual(t, org2.MemberCount, int32(1), "Second org should have at least 1 member")

		// Verify userOrg roles
		userOrgMap := make(map[int32]domain.UserOrg)
		for _, uo := range userOrgs {
			userOrgMap[uo.OrgID] = uo
		}

		uo1 := userOrgMap[orgID1]
		assert.Equal(t, domain.UserOrgRoleMember, uo1.Role, "First org role should be MEMBER")
		assert.Equal(t, domain.UserOrgStatusActive, uo1.Status, "First org status should be ACTIVE")

		uo2 := userOrgMap[orgID2]
		assert.Equal(t, domain.UserOrgRoleAdmin, uo2.Role, "Second org role should be ADMIN")
		assert.Equal(t, domain.UserOrgStatusActive, uo2.Status, "Second org status should be ACTIVE")

		t.Logf("✓ Multi-org membership verified: user %d belongs to orgs %d (MEMBER) and %d (ADMIN)", userID, orgID1, orgID2)
	})

	t.Run("ListMyOrganizations excludes BLOCKED orgs", func(t *testing.T) {
		// Clean up test data
		ts := time.Now().UnixNano()
		testPrefix := fmt.Sprintf("blocked-org-test-%d", ts)
		db.Exec("DELETE FROM users_orgs WHERE user_id IN (SELECT id FROM users WHERE email LIKE $1)", testPrefix+"-%@test.com")
		db.Exec("DELETE FROM users WHERE email LIKE $1", testPrefix+"-%@test.com")
		db.Exec("DELETE FROM orgs WHERE name LIKE $1", testPrefix+"-%")

		// Create test organization
		var orgID int32
		err := db.QueryRow(`INSERT INTO orgs (name, metro, admin_email, admin_phone_number, address) 
			VALUES ($1, 'San Jose', 'admin@test.com', '555-0000', '123 Test St') RETURNING id`,
			fmt.Sprintf("%s-org", testPrefix)).Scan(&orgID)
		require.NoError(t, err)

		// Create test user
		email := fmt.Sprintf("%s-user@test.com", testPrefix)
		var userID int32
		err = db.QueryRow(`INSERT INTO users (email, phone_number, password_hash, name) 
			VALUES ($1, '555-1111', 'hash', 'Blocked Org User') RETURNING id`, email).Scan(&userID)
		require.NoError(t, err)

		// Add user to org as BLOCKED
		_, err = db.Exec(`INSERT INTO users_orgs (user_id, org_id, role, status, joined_on, balance_cents) 
			VALUES ($1, $2, 'MEMBER', 'BLOCK', CURRENT_DATE, 0)`, userID, orgID)
		require.NoError(t, err)

		// Call ListMyOrganizations
		orgs, _, err := orgSvc.ListMyOrganizations(ctx, userID)
		require.NoError(t, err)

		// Should return empty list (blocked orgs excluded)
		assert.Len(t, orgs, 0, "Should not include BLOCKED organizations")
		t.Logf("✓ BLOCKED orgs correctly excluded from ListMyOrganizations")
	})

	t.Run("ListMyOrganizations returns empty for user with no orgs", func(t *testing.T) {
		// Create test user with no orgs
		email := fmt.Sprintf("no-org-user-%d@test.com", time.Now().UnixNano())
		var userID int32
		err := db.QueryRow(`INSERT INTO users (email, phone_number, password_hash, name) 
			VALUES ($1, '555-1111', 'hash', 'No Org User') RETURNING id`, email).Scan(&userID)
		require.NoError(t, err)

		// Call ListMyOrganizations
		orgs, userOrgs, err := orgSvc.ListMyOrganizations(ctx, userID)
		require.NoError(t, err)

		// Should return empty lists
		assert.Len(t, orgs, 0, "Should return empty org list for user with no orgs")
		assert.Len(t, userOrgs, 0, "Should return empty user-org list for user with no orgs")
		t.Logf("✓ Empty org list returned for user with no orgs")
	})
}
