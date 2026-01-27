package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	pb "ubertool-backend-trusted/api/gen/v1"
)

// Test2FAFlow tests the complete 2FA authentication flow:
// 1. Login with email/password -> get 2FA token
// 2. Verify 2FA code with token -> get access/refresh tokens
func Test2FAFlow(t *testing.T) {
	// Setup test database and create test user
	db := prepareDB(t)
	defer db.Close()

	// Test credentials
	email := "test-2fa-user@example.com"
	password := "testpass123"
	userName := "Test 2FA User"
	expectedCode := "123456" // Hardcoded in the backend

	// Create test user with password
	t.Log("Setting up test user...")
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	require.NoError(t, err, "Failed to generate password hash")

	// Clean up any existing test user
	_, _ = db.Exec("DELETE FROM users WHERE email = $1", email)

	// Insert test user
	var userID int32
	err = db.QueryRow(`
		INSERT INTO users (email, phone_number, password_hash, name, created_on, updated_on)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id`,
		email, "", string(hash), userName, time.Now(), time.Now(),
	).Scan(&userID)
	require.NoError(t, err, "Failed to create test user")
	t.Logf("Test user created with ID: %d", userID)

	// Cleanup at the end
	defer func() {
		_, _ = db.Exec("DELETE FROM users WHERE id = $1", userID)
		t.Log("Test user cleaned up")
	}()

	// Connect to gRPC server
	serverAddr := "localhost:50052"
	conn, err := grpc.Dial(serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err, "Failed to connect to gRPC server at %s", serverAddr)
	defer conn.Close()

	authClient := pb.NewAuthServiceClient(conn)

	var twoFAToken string
	var accessToken string

	t.Run("Step1_Login_Should_Return_2FA_Token", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		t.Logf("Testing login for: %s", email)

		req := &pb.LoginRequest{
			Email:    email,
			Password: password,
		}

		resp, err := authClient.Login(ctx, req)
		require.NoError(t, err, "Login should succeed with valid credentials")
		require.NotNil(t, resp, "Login response should not be nil")

		t.Logf("Login Response: Success=%v, Message=%s", resp.Success, resp.Message)

		assert.True(t, resp.Success, "Login success should be true")
		assert.NotEmpty(t, resp.TwoFaToken, "2FA token should be returned")
		assert.Equal(t, "2FA Required", resp.Message, "Message should indicate 2FA is required")

		if resp.TwoFaToken != "" {
			t.Logf("✅ 2FA Token received (length: %d)", len(resp.TwoFaToken))
			t.Logf("   Token prefix: %s...", resp.TwoFaToken[:min(30, len(resp.TwoFaToken))])
			twoFAToken = resp.TwoFaToken // Save for next test
		}
	})

	t.Run("Step2_Verify2FA_With_Invalid_Code_Should_Fail", func(t *testing.T) {
		if twoFAToken == "" {
			t.Skip("No 2FA token available from previous test")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Add 2FA token to context metadata
		md := metadata.New(map[string]string{
			"authorization": "Bearer " + twoFAToken,
		})
		ctx = metadata.NewOutgoingContext(ctx, md)

		t.Log("Testing 2FA verification with INVALID code")

		req := &pb.Verify2FARequest{
			TwoFaCode: "999999", // Invalid code
		}

		resp, err := authClient.Verify2FA(ctx, req)

		// Should fail with invalid code
		assert.Error(t, err, "Verify2FA should fail with invalid code")
		assert.Nil(t, resp, "Response should be nil on error")

		if err != nil {
			t.Logf("❌ Expected error received: %v", err)
		}
	})

	t.Run("Step3_Verify2FA_Without_Token_Should_Fail", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// NO token in metadata - should fail at interceptor level
		t.Log("Testing 2FA verification WITHOUT 2FA token in header")

		req := &pb.Verify2FARequest{
			TwoFaCode: expectedCode,
		}

		resp, err := authClient.Verify2FA(ctx, req)

		// Should fail with unauthenticated error
		assert.Error(t, err, "Verify2FA should fail without token")
		assert.Nil(t, resp, "Response should be nil on error")

		if err != nil {
			t.Logf("❌ Expected error received: %v", err)
		}
	})

	t.Run("Step4_Verify2FA_With_Valid_Code_Should_Succeed", func(t *testing.T) {
		if twoFAToken == "" {
			t.Skip("No 2FA token available from previous test")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Add 2FA token to context metadata (Authorization header)
		md := metadata.New(map[string]string{
			"authorization": "Bearer " + twoFAToken,
		})
		ctx = metadata.NewOutgoingContext(ctx, md)

		t.Logf("Testing 2FA verification with VALID code: %s", expectedCode)
		t.Log("Authorization header includes 2FA token")

		req := &pb.Verify2FARequest{
			TwoFaCode: expectedCode,
		}

		resp, err := authClient.Verify2FA(ctx, req)
		require.NoError(t, err, "Verify2FA should succeed with valid code and token")
		require.NotNil(t, resp, "Verify2FA response should not be nil")

		t.Logf("Verify2FA Response: Success=%v", resp.Success)

		assert.True(t, resp.Success, "Verify2FA success should be true")
		assert.NotEmpty(t, resp.AccessToken, "Access token should be returned")
		assert.NotEmpty(t, resp.RefreshToken, "Refresh token should be returned")

		t.Log("✅ 2FA Verification SUCCESSFUL!")
		t.Logf("   Access Token (prefix): %s...", resp.AccessToken[:min(30, len(resp.AccessToken))])
		t.Logf("   Refresh Token (prefix): %s...", resp.RefreshToken[:min(30, len(resp.RefreshToken))])

		accessToken = resp.AccessToken // Save for next test
	})

	t.Run("Step5_Use_Access_Token_To_Get_Notifications", func(t *testing.T) {
		if accessToken == "" {
			t.Skip("No access token available from previous test")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Add access token to context metadata
		md := metadata.New(map[string]string{
			"authorization": "Bearer " + accessToken,
		})
		ctx = metadata.NewOutgoingContext(ctx, md)

		t.Log("Testing protected endpoint with access token")

		// Try to call a protected endpoint - e.g., GetNotifications
		notificationClient := pb.NewNotificationServiceClient(conn)
		notifResp, err := notificationClient.GetNotifications(ctx, &pb.GetNotificationsRequest{
			Limit:  10,
			Offset: 0,
		})

		require.NoError(t, err, "Protected endpoint should be accessible with valid access token")
		require.NotNil(t, notifResp, "Notification response should not be nil")

		t.Log("✅ Protected endpoint access SUCCESSFUL!")
		t.Logf("   Notifications count: %d", len(notifResp.Notifications))
		t.Logf("   Total notifications: %d", notifResp.TotalCount)
	})
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
