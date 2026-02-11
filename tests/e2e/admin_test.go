package e2e

import (
	"testing"
	"time"

	pb "ubertool-backend-trusted/api/gen/v1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminService_E2E(t *testing.T) {
	db := PrepareDB(t)
	defer db.Close()
	defer db.Cleanup()

	client := NewGRPCClient(t, "")
	defer client.Close()

	adminClient := pb.NewAdminServiceClient(client.Conn())

	t.Run("ApproveJoinRequest for Existing User", func(t *testing.T) {
		// Setup: Create org, admin, and existing user
		orgID := db.CreateTestOrg("")
		adminID := db.CreateTestUser("e2e-test-admin-approve@test.com", "Admin User")
		existingUserID := db.CreateTestUser("e2e-test-existing@test.com", "Existing User")

		db.AddUserToOrg(adminID, orgID, "ADMIN", "ACTIVE", 0)

		// Create join request
		_, err := db.Exec(`
			INSERT INTO join_requests (org_id, user_id, name, email, note, status)
			VALUES ($1, $2, 'Existing User', 'e2e-test-existing@test.com', 'Please let me join', 'PENDING')
		`, orgID, existingUserID)
		require.NoError(t, err)

		// Test: Admin approves join request
		ctx, cancel := ContextWithUserIDAndTimeout(adminID, 5*time.Second)
		defer cancel()

		req := &pb.ApproveRequestToJoinRequest{
			OrganizationId: orgID,
			ApplicantEmail: "e2e-test-existing@test.com",
			ApplicantName:  "Existing User",
		}

		resp, err := adminClient.ApproveRequestToJoin(ctx, req)
		require.NoError(t, err)
		assert.True(t, resp.Success)

		// Verify: User added to users_orgs
		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM users_orgs WHERE user_id = $1 AND org_id = $2", existingUserID, orgID).Scan(&count)
		assert.NoError(t, err)
		assert.Equal(t, 1, count)

		// Verify: Join request status updated
		var status string
		err = db.QueryRow("SELECT status FROM join_requests WHERE email = $1 AND org_id = $2", "e2e-test-existing@test.com", orgID).Scan(&status)
		assert.NoError(t, err)
		assert.Equal(t, "APPROVED", status)
	})

	t.Run("ApproveJoinRequest for New User (Send Invitation)", func(t *testing.T) {
		// Setup: Create org and admin
		orgID := db.CreateTestOrg("")
		adminID := db.CreateTestUser("e2e-test-admin-invite@test.com", "Admin User 2")
		db.AddUserToOrg(adminID, orgID, "ADMIN", "ACTIVE", 0)

		newUserEmail := "e2e-test-newuser-invite@test.com"

		// Create join request for non-existent user
		_, err := db.Exec(`
			INSERT INTO join_requests (org_id, user_id, name, email, note, status)
			VALUES ($1, NULL, 'New User', $2, 'I want to join', 'PENDING')
		`, orgID, newUserEmail)
		require.NoError(t, err)

		// Test: Admin approves join request
		ctx, cancel := ContextWithUserIDAndTimeout(adminID, 5*time.Second)
		defer cancel()

		req := &pb.ApproveRequestToJoinRequest{
			OrganizationId: orgID,
			ApplicantEmail: newUserEmail,
			ApplicantName:  "New User",
		}

		resp, err := adminClient.ApproveRequestToJoin(ctx, req)
		require.NoError(t, err)
		assert.True(t, resp.Success)

		// Verify: Invitation created
		var invitationCount int
		err = db.QueryRow("SELECT COUNT(*) FROM invitations WHERE email = $1 AND org_id = $2", newUserEmail, orgID).Scan(&invitationCount)
		assert.NoError(t, err)
		assert.Equal(t, 1, invitationCount)

		// Verify: Join request status updated
		var status string
		err = db.QueryRow("SELECT status FROM join_requests WHERE email = $1 AND org_id = $2", newUserEmail, orgID).Scan(&status)
		assert.NoError(t, err)
		assert.Equal(t, "APPROVED", status)
	})

	t.Run("BlockUser", func(t *testing.T) {
		// Setup: Create org, admin, and member
		orgID := db.CreateTestOrg("")
		adminID := db.CreateTestUser("e2e-test-admin-block@test.com", "Admin User 3")
		memberID := db.CreateTestUser("e2e-test-member-block@test.com", "Member to Block")

		db.AddUserToOrg(adminID, orgID, "ADMIN", "ACTIVE", 0)
		db.AddUserToOrg(memberID, orgID, "MEMBER", "ACTIVE", 1000)

		// Test: Admin blocks user
		ctx, cancel := ContextWithUserIDAndTimeout(adminID, 5*time.Second)
		defer cancel()

		req := &pb.AdminBlockUserAccountRequest{
			BlockedUserId:  memberID,
			OrganizationId: orgID,
			Reason:         "Violated community guidelines",
		}

		resp, err := adminClient.AdminBlockUserAccount(ctx, req)
		require.NoError(t, err)
		assert.True(t, resp.Success)

		// Verify: User status updated to BLOCK
		var status, blockReason string
		var blockedOn *time.Time
		err = db.QueryRow("SELECT status, blocked_reason, blocked_on FROM users_orgs WHERE user_id = $1 AND org_id = $2", memberID, orgID).Scan(&status, &blockReason, &blockedOn)
		assert.NoError(t, err)
		assert.Equal(t, "BLOCK", status)
		assert.Equal(t, "Violated community guidelines", blockReason)
		assert.NotNil(t, blockedOn)
	})

	t.Run("ListMembers", func(t *testing.T) {
		// Setup: Create org and members
		orgID := db.CreateTestOrg("")
		adminID := db.CreateTestUser("e2e-test-admin-list@test.com", "Admin User 4")
		member1ID := db.CreateTestUser("e2e-test-member1@test.com", "Member 1")
		member2ID := db.CreateTestUser("e2e-test-member2@test.com", "Member 2")

		db.AddUserToOrg(adminID, orgID, "ADMIN", "ACTIVE", 0)
		db.AddUserToOrg(member1ID, orgID, "MEMBER", "ACTIVE", 1000)
		db.AddUserToOrg(member2ID, orgID, "MEMBER", "ACTIVE", 2000)

		// Test: List members
		ctx, cancel := ContextWithUserIDAndTimeout(adminID, 5*time.Second)
		defer cancel()

		req := &pb.ListMembersRequest{
			OrganizationId: orgID,
		}

		resp, err := adminClient.ListMembers(ctx, req)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(resp.Members), 3) // Admin + 2 members
	})

	t.Run("ListJoinRequests", func(t *testing.T) {
		// Setup: Create org, admin, and join requests
		orgID := db.CreateTestOrg("")
		adminID := db.CreateTestUser("e2e-test-admin-joinreq@test.com", "Admin User 5")
		db.AddUserToOrg(adminID, orgID, "ADMIN", "ACTIVE", 0)

		// Create pending join requests
		_, err := db.Exec(`
			INSERT INTO join_requests (org_id, user_id, name, email, note, status)
			VALUES ($1, NULL, 'Applicant 1', 'e2e-test-applicant1@test.com', 'Note 1', 'PENDING'),
			       ($1, NULL, 'Applicant 2', 'e2e-test-applicant2@test.com', 'Note 2', 'PENDING')
		`, orgID)
		require.NoError(t, err)

		// Test: List join requests
		ctx, cancel := ContextWithUserIDAndTimeout(adminID, 5*time.Second)
		defer cancel()

		req := &pb.ListJoinRequestsRequest{
			OrganizationId: orgID,
		}

		resp, err := adminClient.ListJoinRequests(ctx, req)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(resp.Requests), 2)
	})
}
