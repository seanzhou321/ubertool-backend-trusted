package e2e

import (
	"testing"
	"time"

	pb "ubertool-backend-trusted/api/gen/v1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserService_E2E(t *testing.T) {
	db := PrepareDB(t)
	defer db.Close()
	defer db.Cleanup()

	client := NewGRPCClient(t, "")
	defer client.Close()

	userClient := pb.NewUserServiceClient(client.Conn())

	t.Run("GetUser Profile", func(t *testing.T) {
		// Setup: Create user and add to organizations
		userID := db.CreateTestUser("e2e-test-getuser@test.com", "Test User")
		org1ID := db.CreateTestOrg("E2E-Test-User-Org-1")
		org2ID := db.CreateTestOrg("E2E-Test-User-Org-2")

		db.AddUserToOrg(userID, org1ID, "MEMBER", "ACTIVE", 1000)
		db.AddUserToOrg(userID, org2ID, "ADMIN", "ACTIVE", 2500)

		// Test: Get user profile
		ctx, cancel := ContextWithUserIDAndTimeout(userID, 5*time.Second)
		defer cancel()

		req := &pb.GetUserRequest{}
		resp, err := userClient.GetUser(ctx, req)
		require.NoError(t, err)

		assert.NotNil(t, resp.User)
		assert.Equal(t, userID, resp.User.Id)
		assert.Equal(t, "e2e-test-getuser@test.com", resp.User.Email)
		assert.Equal(t, "Test User", resp.User.Name)

		// Verify: User has organizations listed
		assert.GreaterOrEqual(t, len(resp.User.Orgs), 2)

		// Verify: Organization details include user-specific data
		foundOrg1 := false
		foundOrg2 := false
		for _, org := range resp.User.Orgs {
			if org.Id == org1ID {
				foundOrg1 = true
				assert.Equal(t, int32(1000), org.UserBalance)
			}
			if org.Id == org2ID {
				foundOrg2 = true
				assert.Equal(t, int32(2500), org.UserBalance)
			}
		}
		assert.True(t, foundOrg1, "Organization 1 should be in user's organizations")
		assert.True(t, foundOrg2, "Organization 2 should be in user's organizations")
	})

	t.Run("UpdateProfile", func(t *testing.T) {
		// Setup: Create user
		userID := db.CreateTestUser("e2e-test-updateuser@test.com", "Original Name")

		// Test: Update profile
		ctx, cancel := ContextWithUserIDAndTimeout(userID, 5*time.Second)
		defer cancel()

		req := &pb.UpdateProfileRequest{
			Name:      "Updated Name",
			Email:     "e2e-test-updated@test.com",
			Phone:     "555-9999",
			AvatarUrl: "https://example.com/avatar.jpg",
		}

		resp, err := userClient.UpdateProfile(ctx, req)
		require.NoError(t, err)

		assert.Equal(t, "Updated Name", resp.User.Name)
		assert.Equal(t, "e2e-test-updated@test.com", resp.User.Email)
		assert.Equal(t, "555-9999", resp.User.Phone)
		assert.Equal(t, "https://example.com/avatar.jpg", resp.User.AvatarUrl)

		// Verify: Database was updated
		var name, email, phone, avatarURL string
		err = db.QueryRow("SELECT name, email, phone_number, avatar_url FROM users WHERE id = $1", userID).Scan(&name, &email, &phone, &avatarURL)
		assert.NoError(t, err)
		assert.Equal(t, "Updated Name", name)
		assert.Equal(t, "e2e-test-updated@test.com", email)
		assert.Equal(t, "555-9999", phone)
		assert.Equal(t, "https://example.com/avatar.jpg", avatarURL)
	})

	t.Run("GetUser with No Organizations", func(t *testing.T) {
		// Setup: Create user without any organization membership
		userID := db.CreateTestUser("e2e-test-noorg@test.com", "No Org User")

		// Test: Get user profile
		ctx, cancel := ContextWithUserIDAndTimeout(userID, 5*time.Second)
		defer cancel()

		req := &pb.GetUserRequest{}
		resp, err := userClient.GetUser(ctx, req)
		require.NoError(t, err)

		assert.NotNil(t, resp.User)
		assert.Equal(t, userID, resp.User.Id)
		assert.Equal(t, 0, len(resp.User.Orgs))
	})

	t.Run("UpdateProfile Email Uniqueness", func(t *testing.T) {
		// Setup: Create two users
		_ = db.CreateTestUser("e2e-test-user1-unique@test.com", "User 1")
		user2ID := db.CreateTestUser("e2e-test-user2-unique@test.com", "User 2")

		// Test: Try to update user2's email to user1's email (should fail)
		ctx, cancel := ContextWithUserIDAndTimeout(user2ID, 5*time.Second)
		defer cancel()

		req := &pb.UpdateProfileRequest{
			Name:  "User 2",
			Email: "e2e-test-user1-unique@test.com", // Duplicate email
			Phone: "555-0001",
		}

		_, err := userClient.UpdateProfile(ctx, req)
		// Should fail due to unique constraint on email
		assert.Error(t, err)
	})
}

