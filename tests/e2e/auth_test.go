package e2e

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	pb "ubertool-backend-trusted/api/gen/v1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/metadata"
)

// JWTClaims represents the decoded JWT claims used to verify Login/Verify2FA/RefreshToken
// issue tokens with the correct identity and standard claims (FR-001, FR-004).
type JWTClaims struct {
	UserID int32    `json:"user_id"`
	Email  string   `json:"email"`
	Type   string   `json:"type"`
	Roles  []string `json:"roles"`
	Sub    string   `json:"sub"`
	Exp    int64    `json:"exp"`
	Iat    int64    `json:"iat"`
	Iss    string   `json:"iss"`
	Aud    []string `json:"aud"`
	Jti    string   `json:"jti"`
}

// decodeJWT decodes a JWT token without verification (for testing purposes).
func decodeJWT(token string) (*JWTClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, assert.AnError
	}
	payload := parts[1]
	if mod := len(payload) % 4; mod != 0 {
		payload += strings.Repeat("=", 4-mod)
	}
	decoded, err := base64.URLEncoding.DecodeString(payload)
	if err != nil {
		decoded, err = base64.RawURLEncoding.DecodeString(parts[1])
		if err != nil {
			return nil, err
		}
	}
	var claims JWTClaims
	if err := json.Unmarshal(decoded, &claims); err != nil {
		return nil, err
	}
	return &claims, nil
}

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

	t.Run("Login Flow - Canonical Password Success", func(t *testing.T) {
		// Setup: create a user with a real bcrypt hash of the login password — a fake
		// literal hash here would make bcrypt comparison fail unconditionally, and any
		// assertion gated behind "if err == nil" would then pass regardless of whether
		// Login actually works (see git history for why this matters).
		//
		// This subtest also covers FR-004 (RefreshToken) and JWT claim correctness: they were
		// previously a separate Login+Verify2FA cycle in jwt_verification_test.go, folded in
		// here because the server's per-IP Login rate limiter (FR-012, burst 3) can't sustain
		// this file's 3 distinct-scenario logins plus a 4th unrelated one in the same window.
		email := "e2e-test-login@test.com"
		userID := db.CreateTestUser(email, "Login User")

		hash, err := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
		require.NoError(t, err)
		_, err = db.Exec("UPDATE users SET password_hash = $1 WHERE id = $2", string(hash), userID)
		require.NoError(t, err)

		ctx, cancel := ContextWithTimeout(5 * time.Second)
		defer cancel()

		req := &pb.LoginRequest{
			Email:    email,
			Password: "password123",
		}

		resp, err := authClient.Login(ctx, req)
		require.NoError(t, err, "Login must succeed against the canonical password_hash")
		assert.True(t, resp.Success)
		assert.NotEmpty(t, resp.TwoFaToken, "2FA is always required after a successful password check")

		// Complete 2FA (server runs with two_fa.enabled=false — fixed_passcode "47291", see
		// tests/integration/2fa_test.go) and verify the issued JWT's claims.
		ctx2, cancel2 := ContextWithTimeout(5 * time.Second)
		defer cancel2()
		md := metadata.Pairs("authorization", "Bearer "+resp.TwoFaToken)
		ctx2 = metadata.NewOutgoingContext(ctx2, md)
		verify2FAResp, err := authClient.Verify2FA(ctx2, &pb.Verify2FARequest{TwoFaCode: "47291"})
		require.NoError(t, err, "2FA verification should succeed")
		require.NotEmpty(t, verify2FAResp.AccessToken)
		require.NotEmpty(t, verify2FAResp.RefreshToken)

		claims, err := decodeJWT(verify2FAResp.AccessToken)
		require.NoError(t, err, "should be able to decode the access token")
		assert.Equal(t, userID, claims.UserID)
		assert.Equal(t, email, claims.Email)
		assert.Equal(t, "access", claims.Type)
		assert.NotEmpty(t, claims.Roles)
		assert.NotEmpty(t, claims.Sub)
		assert.NotZero(t, claims.Exp)
		assert.NotZero(t, claims.Iat)
		assert.Equal(t, "auth-service", claims.Iss)
		assert.Contains(t, claims.Aud, "api-access")
		assert.NotEmpty(t, claims.Jti)
		now := time.Now().Unix()
		assert.Greater(t, claims.Exp, now, "token should not be expired")
		assert.LessOrEqual(t, claims.Iat, now, "issued-at should not be in the future")

		// FR-004: RefreshToken must issue a new access token preserving identity claims.
		time.Sleep(1 * time.Second)
		ctx3, cancel3 := ContextWithTimeout(5 * time.Second)
		defer cancel3()
		md3 := metadata.Pairs(
			"authorization", "Bearer "+verify2FAResp.RefreshToken,
			"refresh-token", verify2FAResp.RefreshToken,
		)
		ctx3 = metadata.NewOutgoingContext(ctx3, md3)
		refreshResp, err := authClient.RefreshToken(ctx3, &pb.RefreshTokenRequest{})
		require.NoError(t, err, "refresh should succeed")
		require.NotEmpty(t, refreshResp.AccessToken)

		newClaims, err := decodeJWT(refreshResp.AccessToken)
		require.NoError(t, err, "should be able to decode the refreshed access token")
		assert.Equal(t, claims.UserID, newClaims.UserID, "UserID should be preserved after refresh")
		assert.Equal(t, claims.Email, newClaims.Email, "Email should be preserved after refresh")
		assert.Equal(t, "access", newClaims.Type)
		assert.Greater(t, newClaims.Iat, claims.Iat, "refreshed token should have a newer issued-at timestamp")
	})

	t.Run("Login Flow - Pending Credential Fallback", func(t *testing.T) {
		// Canonical password_hash is set to something the login password will never match,
		// so Login must fall back to a valid, unused, unexpired pending_credentials row.
		email := "e2e-test-login-pending@test.com"
		userID := db.CreateTestUser(email, "Pending Cred User")

		canonicalHash, err := bcrypt.GenerateFromPassword([]byte("not-the-login-password"), bcrypt.DefaultCost)
		require.NoError(t, err)
		_, err = db.Exec("UPDATE users SET password_hash = $1 WHERE id = $2", string(canonicalHash), userID)
		require.NoError(t, err)

		tempHash, err := bcrypt.GenerateFromPassword([]byte("temp-password-456"), bcrypt.DefaultCost)
		require.NoError(t, err)
		_, err = db.Exec(
			"INSERT INTO pending_credentials (user_id, temp_password_hash, expires_at) VALUES ($1, $2, NOW() + INTERVAL '48 hours')",
			userID, string(tempHash),
		)
		require.NoError(t, err)

		ctx, cancel := ContextWithTimeout(5 * time.Second)
		defer cancel()

		req := &pb.LoginRequest{
			Email:    email,
			Password: "temp-password-456",
		}

		resp, err := authClient.Login(ctx, req)
		require.NoError(t, err, "Login must succeed via the pending_credentials fallback")
		assert.NotEmpty(t, resp.TwoFaToken)
	})

	t.Run("Login Flow - Invalid Credentials Rejected", func(t *testing.T) {
		// Neither the canonical password nor any pending credential matches — Login must
		// reject with a generic error, not a distinguishing one.
		email := "e2e-test-login-reject@test.com"
		userID := db.CreateTestUser(email, "Reject User")

		hash, err := bcrypt.GenerateFromPassword([]byte("the-real-password"), bcrypt.DefaultCost)
		require.NoError(t, err)
		_, err = db.Exec("UPDATE users SET password_hash = $1 WHERE id = $2", string(hash), userID)
		require.NoError(t, err)

		ctx, cancel := ContextWithTimeout(5 * time.Second)
		defer cancel()

		req := &pb.LoginRequest{
			Email:    email,
			Password: "wrong-password",
		}

		resp, err := authClient.Login(ctx, req)
		require.Error(t, err, "Login must reject a password that matches neither canonical nor pending credentials")
		assert.Nil(t, resp)
	})
}
