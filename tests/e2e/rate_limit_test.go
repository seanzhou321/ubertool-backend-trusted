//go:build ratelimit

package e2e

// This test deliberately exhausts the server's per-IP Login and Verify2FA rate-limit
// buckets (internal/security.IPRateLimiter: burst 3, refill 1/3min, process-lifetime,
// keyed by source IP — see internal/api/grpc/interceptor/rate_limit_interceptor.go). Since
// every e2e test connects from the same loopback IP and shares one running server process,
// running this test in the same `go test ./tests/e2e/...` sweep as any other Login- or
// Verify2FA-calling test would starve that test's budget. It is gated behind the "ratelimit"
// build tag and run in isolation via `make test-e2e-rate-limit`, restarting the server first
// so the buckets start empty — the same pattern the Makefile already uses for other
// isolation-sensitive e2e tests (test-e2e-admin-retrieve, test-e2e-fcm).

import (
	"testing"
	"time"

	pb "ubertool-backend-trusted/api/gen/v1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestAuthService_RateLimit_E2E covers FR-001 (spec 001): Login and Verify2FA MUST be
// rate-limited per client IP with a 3-attempt burst. Requires a freshly (re)started server
// so the shared, process-lifetime bucket starts empty.
func TestAuthService_RateLimit_E2E(t *testing.T) {
	db := PrepareDB(t)
	defer db.Close()
	defer db.Cleanup()

	client := NewGRPCClient(t, "")
	defer client.Close()

	authClient := pb.NewAuthServiceClient(client.Conn())

	t.Run("Login is throttled after a 3-attempt burst", func(t *testing.T) {
		email := "e2e-test-ratelimit-login@test.com"
		db.CreateTestUser(email, "Rate Limit User")
		// Wrong password on every attempt — the rate limiter runs before the handler, so a
		// doomed-to-fail credential check still consumes a token; we only care about the
		// interceptor's own accounting here, not Login's credential logic.
		req := &pb.LoginRequest{Email: email, Password: "wrong-password"}

		var lastErr error
		for i := 0; i < 3; i++ {
			ctx, cancel := ContextWithTimeout(5 * time.Second)
			_, lastErr = authClient.Login(ctx, req)
			cancel()
			require.Error(t, lastErr, "attempt %d: wrong password must still be rejected", i+1)
			require.NotEqual(t, codes.ResourceExhausted, status.Code(lastErr),
				"attempt %d must be within the 3-attempt burst, not rate-limited", i+1)
		}

		// 4th attempt within the same window must be throttled.
		ctx, cancel := ContextWithTimeout(5 * time.Second)
		defer cancel()
		_, err := authClient.Login(ctx, req)
		require.Error(t, err)
		assert.Equal(t, codes.ResourceExhausted, status.Code(err), "4th attempt must be rate-limited")
	})

	t.Run("Verify2FA has its own independent 3-attempt burst", func(t *testing.T) {
		// No valid 2FA session token is sent — every call is rejected before it reaches the
		// code check, but the interceptor still accounts for it against Verify2FA's own bucket.
		req := &pb.Verify2FARequest{TwoFaCode: "00000"}

		var lastErr error
		for i := 0; i < 3; i++ {
			ctx, cancel := ContextWithTimeout(5 * time.Second)
			_, lastErr = authClient.Verify2FA(ctx, req)
			cancel()
			require.Error(t, lastErr, "attempt %d: an unauthenticated Verify2FA call must still be rejected", i+1)
			require.NotEqual(t, codes.ResourceExhausted, status.Code(lastErr),
				"attempt %d must be within Verify2FA's own 3-attempt burst", i+1)
		}

		ctx, cancel := ContextWithTimeout(5 * time.Second)
		defer cancel()
		_, err := authClient.Verify2FA(ctx, req)
		require.Error(t, err)
		assert.Equal(t, codes.ResourceExhausted, status.Code(err), "4th Verify2FA attempt must be rate-limited")
	})
}
