// Package smoke contains smoke tests for the EC2-deployed microservice.
//
// These tests verify three things without modifying any data:
//  1. TLS is correctly configured and the certificate is valid.
//  2. The gRPC API is alive and processing requests end-to-end.
//  3. The microservice's database connection is healthy (verified through
//     the API — a Login attempt that triggers a DB lookup and returns a
//     service-level error proves the DB round-trip is working).
//
// No SSH tunnel or direct database credentials are required.
//
// Prerequisites:
//   - config/config.smoke.ec2.yaml with server.host, server.port, tls.enabled
//     (see deploy/ec2-mvp/docs/handoff.md Phase 2b)
//
// Run via Makefile:
//
//	make test-smoke-ec2
//
// Or manually:
//
//	go test -v -count=1 ./tests/smoke/... \
//	  -config=config/config.smoke.ec2.yaml -timeout 30s
package smoke

import (
	"context"
	"crypto/tls"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "ubertool-backend-trusted/api/gen/v1"
)

// TestTLSConnectivity opens a raw TLS connection to the gRPC endpoint and
// inspects the certificate chain without sending any gRPC traffic.
func TestTLSConnectivity(t *testing.T) {
	cfg := loadSmokeConfig(t)
	if !cfg.TLS.Enabled {
		t.Skip("tls.enabled=false in config — skipping TLS certificate check")
	}

	dialer := &net.Dialer{Timeout: 10 * time.Second}
	conn, err := tls.DialWithDialer(dialer, "tcp", cfg.grpcAddr(), &tls.Config{
		MinVersion: tls.VersionTLS12,
	})
	require.NoError(t, err, "TLS handshake failed to %s", cfg.grpcAddr())
	defer conn.Close()

	state := conn.ConnectionState()
	require.True(t, state.HandshakeComplete, "TLS handshake did not complete")
	require.NotEmpty(t, state.PeerCertificates, "server presented no certificates")

	leaf := state.PeerCertificates[0]
	t.Logf("Subject    : %s", leaf.Subject.CommonName)
	t.Logf("Issuer     : %s", leaf.Issuer.CommonName)
	t.Logf("NotBefore  : %s", leaf.NotBefore.Format(time.RFC3339))
	t.Logf("NotAfter   : %s", leaf.NotAfter.Format(time.RFC3339))
	t.Logf("TLS version: 0x%04x", state.Version)

	now := time.Now()
	assert.True(t, now.After(leaf.NotBefore), "certificate is not yet valid (NotBefore=%s)", leaf.NotBefore)
	assert.True(t, now.Before(leaf.NotAfter), "certificate has expired (NotAfter=%s)", leaf.NotAfter)

	if leaf.NotAfter.Sub(now) < 14*24*time.Hour {
		t.Logf("WARNING: certificate expires in less than 14 days (%s)", leaf.NotAfter.Format(time.RFC3339))
	}
}

// TestAPILiveness verifies the gRPC API is reachable and returning service-level
// responses. A Login request with invalid credentials is used because it is
// unauthenticated (no token needed) and always returns a defined error code.
func TestAPILiveness(t *testing.T) {
	cfg := loadSmokeConfig(t)
	conn := newGRPCConn(t, cfg)
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := pb.NewAuthServiceClient(conn).Login(ctx, &pb.LoginRequest{
		Email:    "smoke-probe@test.invalid",
		Password: "not-a-real-password",
	})

	require.Error(t, err, "expected an error response from the service")
	st, ok := status.FromError(err)
	require.True(t, ok,
		"received a non-gRPC error — likely a transport/TLS failure: %v", err)
	require.NotEqual(t, codes.Unavailable, st.Code(),
		"service is unavailable: %s", st.Message())
	require.NotEqual(t, codes.DeadlineExceeded, st.Code(),
		"request timed out before reaching the service: %s", st.Message())

	t.Logf("API responded: code=%s message=%q", st.Code(), st.Message())
}

// TestDatabaseConnectivity verifies that the microservice's database connection
// is healthy using three unauthenticated API calls that together cover multiple
// DB tables and prove the service is transacting, not just reachable:
//
//  1. SearchOrganizations (orgs table) — expects a successful response with at
//     least one organization, providing positive proof of a DB read.
//  2. Login (users table) — expects an application-level error (not Internal),
//     proving the users table was queried.
//  3. ValidateInvite (invitations table) — expects valid=false or an
//     application-level error, proving the invitations table was queried.
//
// No test data is inserted; no SSH tunnel is required.
func TestDatabaseConnectivity(t *testing.T) {
	cfg := loadSmokeConfig(t)
	conn := newGRPCConn(t, cfg)
	defer conn.Close()

	authClient := pb.NewAuthServiceClient(conn)
	orgClient := pb.NewOrganizationServiceClient(conn)

	t.Run("OrgsTableReachable_via_SearchOrganizations", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Search by metro where seed data exists.
		resp, err := orgClient.SearchOrganizations(ctx, &pb.SearchOrganizationsRequest{
			Metro: "San Diego, CA",
		})
		require.NoError(t, err, "SearchOrganizations returned an unexpected error")
		require.NotEmpty(t, resp.Organizations,
			"expected at least one organization in the database — DB may be empty or unreachable")

		t.Logf("orgs table OK — SearchOrganizations returned %d org(s)", len(resp.Organizations))
		for _, org := range resp.Organizations {
			t.Logf("  org id=%-4d name=%q", org.Id, org.Name)
		}
	})

	t.Run("UsersTableReachable_via_Login", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		_, err := authClient.Login(ctx, &pb.LoginRequest{
			Email:    "smoke-probe@test.invalid",
			Password: "not-a-real-password",
		})

		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok, "non-gRPC error — transport or TLS failure: %v", err)
		require.NotEqual(t, codes.Unavailable, st.Code(), "service unavailable: %s", st.Message())
		require.NotEqual(t, codes.Internal, st.Code(), "internal error — DB may be down: %s", st.Message())

		t.Logf("users table OK — Login responded: code=%s message=%q", st.Code(), st.Message())
	})

	t.Run("InvitationsTableReachable_via_ValidateInvite", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		resp, err := authClient.ValidateInvite(ctx, &pb.ValidateInviteRequest{
			InvitationCode: "SMOK-000-000",
			Email:          "smoke-probe@test.invalid",
		})

		if err != nil {
			st, ok := status.FromError(err)
			require.True(t, ok, "non-gRPC error — transport or TLS failure: %v", err)
			require.NotEqual(t, codes.Unavailable, st.Code(), "service unavailable: %s", st.Message())
			require.NotEqual(t, codes.Internal, st.Code(), "internal error — DB may be down: %s", st.Message())
			t.Logf("invitations table OK — ValidateInvite responded: code=%s message=%q", st.Code(), st.Message())
		} else {
			assert.False(t, resp.Valid, "expected valid=false for a nonexistent invitation code")
			t.Logf("invitations table OK — ValidateInvite responded: valid=%v message=%q", resp.Valid, resp.Message)
		}
	})
}

