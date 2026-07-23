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
		var joinRequestID int32
		err := db.QueryRow(`
			INSERT INTO join_requests (org_id, user_id, name, email, note, status)
			VALUES ($1, $2, 'Existing User', 'e2e-test-existing@test.com', 'Please let me join', 'PENDING')
			RETURNING id
		`, orgID, existingUserID).Scan(&joinRequestID)
		require.NoError(t, err)

		// Test: Admin approves join request
		ctx, cancel := ContextWithUserIDAndTimeout(adminID, 5*time.Second)
		defer cancel()

		req := &pb.ApproveRequestToJoinRequest{
			OrganizationId: orgID,
			JoinRequestId:  joinRequestID,
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
		assert.Equal(t, "INVITED", status)
	})

	t.Run("ApproveJoinRequest for New User (Send Invitation)", func(t *testing.T) {
		// Setup: Create org and admin
		orgID := db.CreateTestOrg("")
		adminID := db.CreateTestUser("e2e-test-admin-invite@test.com", "Admin User 2")
		db.AddUserToOrg(adminID, orgID, "ADMIN", "ACTIVE", 0)

		newUserEmail := "e2e-test-newuser-invite@test.com"

		// Create join request for non-existent user
		var joinRequestID int32
		err := db.QueryRow(`
			INSERT INTO join_requests (org_id, user_id, name, email, note, status)
			VALUES ($1, NULL, 'New User', $2, 'I want to join', 'PENDING')
			RETURNING id
		`, orgID, newUserEmail).Scan(&joinRequestID)
		require.NoError(t, err)

		// Test: Admin approves join request
		ctx, cancel := ContextWithUserIDAndTimeout(adminID, 5*time.Second)
		defer cancel()

		req := &pb.ApproveRequestToJoinRequest{
			OrganizationId: orgID,
			JoinRequestId:  joinRequestID,
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
		assert.Equal(t, "INVITED", status)
	})

	// FR-006 (specs/003-organizations-administration): RejectRequestToJoin's invitation-expiry
	// side effect had thorough L1 coverage but was never exercised through the real gRPC handler
	// against a live DB — prior to this test, `grep -r RejectRequestToJoin tests/e2e
	// tests/integration` returned nothing.
	t.Run("RejectRequestToJoin expires the linked invitation", func(t *testing.T) {
		orgID := db.CreateTestOrg("")
		adminID := db.CreateTestUser("e2e-test-admin-reject@test.com", "Admin User")
		db.AddUserToOrg(adminID, orgID, "ADMIN", "ACTIVE", 0)

		applicantEmail := "e2e-test-reject-applicant@test.com"
		var joinRequestID int32
		err := db.QueryRow(`
			INSERT INTO join_requests (org_id, user_id, name, email, note, status)
			VALUES ($1, NULL, 'Applicant', $2, 'Please let me join', 'PENDING')
			RETURNING id
		`, orgID, applicantEmail).Scan(&joinRequestID)
		require.NoError(t, err)

		var invitationID int32
		err = db.QueryRow(`
			INSERT INTO invitations (invitation_code, org_id, email, join_request_id, created_by, expires_on)
			VALUES ($1, $2, $3, $4, $5, CURRENT_DATE + INTERVAL '7 days')
			RETURNING id
		`, "REJ-TEST-INV-CODE", orgID, applicantEmail, joinRequestID, adminID).Scan(&invitationID)
		require.NoError(t, err)

		ctx, cancel := ContextWithUserIDAndTimeout(adminID, 5*time.Second)
		defer cancel()

		resp, err := adminClient.RejectRequestToJoin(ctx, &pb.RejectRequestToJoinRequest{
			OrganizationId: orgID,
			JoinRequestId:  joinRequestID,
			Reason:         "not a fit",
		})
		require.NoError(t, err)
		assert.True(t, resp.Success)

		var status string
		err = db.QueryRow("SELECT status FROM join_requests WHERE id = $1", joinRequestID).Scan(&status)
		require.NoError(t, err)
		assert.Equal(t, "REJECTED", status)

		var expiresOn time.Time
		err = db.QueryRow("SELECT expires_on FROM invitations WHERE id = $1", invitationID).Scan(&expiresOn)
		require.NoError(t, err)
		assert.True(t, expiresOn.Before(time.Now()), "the linked invitation must be expired")
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
			BlockRenting:   true,
			BlockLending:   true,
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

	// FR-005 (specs/003-organizations-administration): every AdminService RPC must require
	// ADMIN/SUPER_ADMIN membership in the target org. The L1 unit suite
	// (TestAdminService_RequiresAdminRole) already regression-locks this per-RPC at the service
	// layer — this is a reinforcement of that guarantee at the contract level, previously absent
	// (spec.md's own SC-001 self-documents this as a known, accepted gap).
	t.Run("AdminBlockUserAccount rejects a non-admin caller", func(t *testing.T) {
		orgID := db.CreateTestOrg("")
		memberID := db.CreateTestUser("e2e-test-nonadmin-block@test.com", "Plain Member")
		targetID := db.CreateTestUser("e2e-test-blocktarget@test.com", "Target User")
		db.AddUserToOrg(memberID, orgID, "MEMBER", "ACTIVE", 0)
		db.AddUserToOrg(targetID, orgID, "MEMBER", "ACTIVE", 0)

		ctx, cancel := ContextWithUserIDAndTimeout(memberID, 5*time.Second)
		defer cancel()

		req := &pb.AdminBlockUserAccountRequest{
			BlockedUserId:  targetID,
			OrganizationId: orgID,
			BlockRenting:   true,
			BlockLending:   true,
			Reason:         "should never apply",
		}

		_, err := adminClient.AdminBlockUserAccount(ctx, req)
		require.Error(t, err, "a plain MEMBER caller must be rejected")

		var status string
		err2 := db.QueryRow("SELECT status FROM users_orgs WHERE user_id = $1 AND org_id = $2", targetID, orgID).Scan(&status)
		require.NoError(t, err2)
		assert.Equal(t, "ACTIVE", status, "the rejected caller's request must not have applied any block")
	})
}
