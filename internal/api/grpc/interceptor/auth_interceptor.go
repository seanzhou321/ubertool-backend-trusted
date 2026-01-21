package interceptor

import (
	"context"
	"strconv"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"ubertool-backend-trusted/internal/config"
	"ubertool-backend-trusted/internal/security"
)

type AuthInterceptor struct {
	tokenManager security.TokenManager
}

func NewAuthInterceptor(tm security.TokenManager) *AuthInterceptor {
	return &AuthInterceptor{tokenManager: tm}
}

// Unary returns a server interceptor function to authenticate and authorize unary RPCs
func (i *AuthInterceptor) Unary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		level := config.GetSecurityLevel(info.FullMethod)

		// Public endpoint - skip auth
		if level == config.SecurityPublic {
			return handler(ctx, req)
		}

		// Extract token from metadata
		token, err := i.extractToken(ctx)
		if err != nil {
			return nil, err
		}

		// Validate token
		claims, err := i.tokenManager.ValidateToken(token)
		if err != nil {
			return nil, status.Errorf(codes.Unauthenticated, "invalid token: %v", err)
		}

		// Check token type based on security level
		if err := i.checkSecurityLevel(level, claims); err != nil {
			return nil, err
		}

		// Inject user ID into context. We use a Copy to avoid side effects
		// and Set to overwrite any existing "user-id" header from the client for security.
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			md = metadata.New(nil)
		} else {
			md = md.Copy()
		}

		md.Set("user-id", strconv.Itoa(int(claims.UserID)))
		newCtx := metadata.NewIncomingContext(ctx, md)

		return handler(newCtx, req)
	}
}

func (i *AuthInterceptor) extractToken(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "metadata is not provided")
	}

	authHeader := md["authorization"]
	if len(authHeader) == 0 {
		return "", status.Error(codes.Unauthenticated, "authorization token is not provided")
	}

	token := authHeader[0]
	// Remove Bearer prefix if present
	if len(token) > 7 && strings.ToUpper(token[0:7]) == "BEARER " {
		token = token[7:]
	}

	return token, nil
}

func (i *AuthInterceptor) checkSecurityLevel(level config.SecurityLevel, claims *security.UserClaims) error {
	switch level {
	case config.SecurityAccess:
		if claims.Type != security.TokenTypeAccess {
			return status.Error(codes.PermissionDenied, "access token required")
		}
	case config.SecurityRefresh:
		if claims.Type != security.TokenTypeRefresh {
			return status.Error(codes.PermissionDenied, "refresh token required")
		}
	case config.Security2FA:
		// For 2FA endpoint, we expect a 2FA pending token (from Login)
		// OR an access token if they are re-verifying?
		// Design says: Verify2FA requires 2fa_pending token.
		if claims.Type != security.TokenType2FAPending {
			return status.Error(codes.PermissionDenied, "2fa pending token required")
		}
	}
	return nil
}
