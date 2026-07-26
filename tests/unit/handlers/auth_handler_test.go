package handlers

import (
	"context"
	"testing"

	pb "ubertool-backend-trusted/api/gen/v1"
	"ubertool-backend-trusted/internal/api/grpc"
	"ubertool-backend-trusted/internal/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
)

// FR-013 (specs/001-authentication-legal-consent): ValidateInvite MUST only include a User
// object in the response when the caller's own authenticated session (JWT-derived user-id,
// injected into gRPC metadata by the auth interceptor) identifies the same user as the one
// matching the invite's email — never merely because a user with that email exists.
// authService.ValidateInvite itself returns the matched user whenever one exists by email
// (see internal/service/auth.go:80-94); AuthHandler.ValidateInvite is the layer that applies
// the login-match gate (internal/api/grpc/auth.go:36-42), so this must be a handler-level
// test, not a service-level one.
func TestAuthHandler_ValidateInvite(t *testing.T) {
	const email = "invitee@example.com"
	matchedUser := &domain.User{ID: 7, Email: email, Name: "Invitee"}
	req := &pb.ValidateInviteRequest{InvitationCode: "valid-code", Email: email}

	t.Run("No User object when the caller is unauthenticated, even though a user with that email exists", func(t *testing.T) {
		svc := new(MockAuthService)
		handler := grpc.NewAuthHandler(svc)
		ctx := context.Background() // no "user-id" metadata at all

		svc.On("ValidateInvite", ctx, "valid-code", email).Return(true, "", matchedUser, nil)

		resp, err := handler.ValidateInvite(ctx, req)
		require.NoError(t, err)
		assert.True(t, resp.Valid)
		assert.Nil(t, resp.User, "an unauthenticated caller must never receive the matched user's profile")
	})

	t.Run("No User object when the caller is authenticated as a different user", func(t *testing.T) {
		svc := new(MockAuthService)
		handler := grpc.NewAuthHandler(svc)
		md := metadata.Pairs("user-id", "999") // a different user than matchedUser.ID (7)
		ctx := metadata.NewIncomingContext(context.Background(), md)

		svc.On("ValidateInvite", ctx, "valid-code", email).Return(true, "", matchedUser, nil)

		resp, err := handler.ValidateInvite(ctx, req)
		require.NoError(t, err)
		assert.True(t, resp.Valid)
		assert.Nil(t, resp.User, "a caller logged in as a different user must not receive someone else's profile")
	})

	t.Run("Returns the User object when the caller is authenticated as the matching user", func(t *testing.T) {
		svc := new(MockAuthService)
		handler := grpc.NewAuthHandler(svc)
		md := metadata.Pairs("user-id", "7") // matches matchedUser.ID
		ctx := metadata.NewIncomingContext(context.Background(), md)

		svc.On("ValidateInvite", ctx, "valid-code", email).Return(true, "", matchedUser, nil)

		resp, err := handler.ValidateInvite(ctx, req)
		require.NoError(t, err)
		assert.True(t, resp.Valid)
		require.NotNil(t, resp.User, "the caller logged in as the matching user must receive their own profile")
		assert.Equal(t, int32(7), resp.User.Id)
		assert.Equal(t, email, resp.User.Email)
	})

	t.Run("No User object when no user exists for the email, regardless of caller identity", func(t *testing.T) {
		svc := new(MockAuthService)
		handler := grpc.NewAuthHandler(svc)
		md := metadata.Pairs("user-id", "7")
		ctx := metadata.NewIncomingContext(context.Background(), md)

		svc.On("ValidateInvite", ctx, "valid-code", email).Return(true, "", nil, nil)

		resp, err := handler.ValidateInvite(ctx, req)
		require.NoError(t, err)
		assert.True(t, resp.Valid)
		assert.Nil(t, resp.User)
	})
}
