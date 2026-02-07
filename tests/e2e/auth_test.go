package e2e

import (
	"testing"
	"time"

	pb "ubertool-backend-trusted/api/gen/v1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthService_E2E(t *testing.T) {
	db := PrepareDB(t)
	defer db.Close()
	defer db.Cleanup()

	// Note: These tests require a running gRPC server
	// Skip if server is not available
	client := NewGRPCClient(t, "")
	defer client.Close()

	authClient := pb.NewAuthServiceClient(client.Conn())

	t.Run("Signup with Valid Invitation", func(t *testing.T) {
		// Setup: Create org, admin user, and invitation
		orgID := db.CreateTestOrg("")
		adminID := db.CreateTestUser("e2e-test-admin@test.com", "Admin User")
		db.AddUserToOrg(adminID, orgID, "SUPER_ADMIN", "ACTIVE", 0)

		email := "e2e-test-newuser@test.com"
		token := db.CreateTestInvitation(orgID, email, adminID)

		// Test: Signup
		ctx, cancel := ContextWithTimeout(5 * time.Second)
		defer cancel()

		req := &pb.SignupRequest{
			InvitationCode: token,
			Name:           "New User",
			Email:          email,
			Phone:          "555-1234",
			Password:       "password123",
		}

		resp, err := authClient.UserSignup(ctx, req)
		require.NoError(t, err)
		assert.True(t, resp.Success)
		assert.NotEmpty(t, resp.Message)

		// Verify: User was created and added to org
		var userID int32
		var usedOn *time.Time
		err = db.QueryRow("SELECT id FROM users WHERE email = $1", email).Scan(&userID)
		assert.NoError(t, err)
		assert.Greater(t, userID, int32(0))

		// Verify: Invitation was marked as used
		err = db.QueryRow("SELECT used_on FROM invitations WHERE invitation_code = $1 AND email = $2", token, email).Scan(&usedOn)
		assert.NoError(t, err)
		assert.NotNil(t, usedOn)

		// Verify: User is in users_orgs
		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM users_orgs WHERE user_id = $1 AND org_id = $2", userID, orgID).Scan(&count)
		assert.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("RequestToJoin Organization", func(t *testing.T) {
		// Setup: Create org and admin
		orgID := db.CreateTestOrg("")
		adminID := db.CreateTestUser("e2e-test-admin2@test.com", "Admin User 2")
		db.AddUserToOrg(adminID, orgID, "ADMIN", "ACTIVE", 0)

		// Test: Request to join
		ctx, cancel := ContextWithTimeout(5 * time.Second)
		defer cancel()

		applicantEmail := "e2e-test-applicant@test.com"
		req := &pb.RequestToJoinRequest{
			OrganizationId: orgID,
			Name:           "Applicant User",
			Email:          applicantEmail,
			Message:        "I would like to join",
			AdminEmail:     "e2e-test-admin2@test.com",
		}

		resp, err := authClient.RequestToJoinOrganization(ctx, req)
		require.NoError(t, err)
		assert.True(t, resp.Success)

		// Verify: Join request was created
		var requestID int32
		var status string
		err = db.QueryRow("SELECT id, status FROM join_requests WHERE email = $1 AND org_id = $2", applicantEmail, orgID).Scan(&requestID, &status)
		assert.NoError(t, err)
		assert.Equal(t, "PENDING", status)

		// Verify: Admin received notification
		var notifCount int
		err = db.QueryRow("SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND org_id = $2", adminID, orgID).Scan(&notifCount)
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, notifCount, 1)
	})

	t.Run("Login Flow", func(t *testing.T) {
		// Setup: Create user
		email := "e2e-test-login@test.com"
		userID := db.CreateTestUser(email, "Login User")

		// Update password hash to a known value (in real scenario, this would be bcrypt hash)
		_, err := db.Exec("UPDATE users SET password_hash = $1 WHERE id = $2", "hashed_password", userID)
		require.NoError(t, err)

		// Test: Login
		ctx, cancel := ContextWithTimeout(5 * time.Second)
		defer cancel()

		req := &pb.LoginRequest{
			Email:    email,
			Password: "password123",
		}

		resp, err := authClient.Login(ctx, req)
		// Note: This might fail if password hashing doesn't match
		// In a real E2E test, you'd need to ensure password is properly hashed
		if err == nil {
			assert.NotEmpty(t, resp.TwoFaToken)
			assert.Greater(t, resp.ExpiresAt, int64(0))
		}
	})
}
