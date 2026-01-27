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
	"ubertool-backend-trusted/internal/logger"
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

		logger.Debug("Auth interceptor processing request", "method", info.FullMethod, "securityLevel", level)

		// Public endpoint - skip auth
		if level == config.SecurityPublic {
			logger.Debug("Public endpoint - skipping authentication", "method", info.FullMethod)
			return handler(ctx, req)
		}

		// Extract token from metadata
		logger.Debug("Extracting token from metadata", "method", info.FullMethod)
		token, err := i.extractToken(ctx)
		if err != nil {
			logger.Warn("Token extraction failed", "method", info.FullMethod, "error", err)
			return nil, err
		}
		logger.Debug("Token extracted", "method", info.FullMethod, "tokenPrefix", token[:min(20, len(token))])

		// Validate token
		logger.Debug("Validating token", "method", info.FullMethod)
		claims, err := i.tokenManager.ValidateToken(token)
		if err != nil {
			logger.Error("Token validation failed", "method", info.FullMethod, "error", err)
			return nil, status.Errorf(codes.Unauthenticated, "invalid token: %v", err)
		}
		logger.Info("Token validated successfully", "method", info.FullMethod, "userID", claims.UserID, "tokenType", claims.Type)

		// Check token type based on security level
		logger.Debug("Checking security level requirements", "method", info.FullMethod, "requiredLevel", level, "tokenType", claims.Type)
		if err := i.checkSecurityLevel(level, claims); err != nil {
			logger.Error("Security level check failed", "method", info.FullMethod, "requiredLevel", level, "tokenType", claims.Type, "error", err)
			return nil, err
		}
		logger.Debug("Security level check passed", "method", info.FullMethod)

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
		logger.Debug("User ID injected into context", "method", info.FullMethod, "userID", claims.UserID)

		return handler(newCtx, req)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (i *AuthInterceptor) extractToken(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		logger.Warn("No metadata in request")
		return "", status.Error(codes.Unauthenticated, "metadata is not provided")
	}

	// Log all available metadata keys for debugging
	logger.Debug("Request metadata", "keys", getAllMetadataKeys(md))

	authHeader := md["authorization"]
	if len(authHeader) == 0 {
		logger.Warn("No authorization header found in metadata", "availableHeaders", getAllMetadataKeys(md))
		return "", status.Error(codes.Unauthenticated, "authorization token is not provided")
	}

	logger.Debug("Authorization header found", "headerValue", authHeader[0][:min(50, len(authHeader[0]))])

	token := authHeader[0]
	// Remove Bearer prefix if present
	if len(token) > 7 && strings.ToUpper(token[0:7]) == "BEARER " {
		token = token[7:]
		logger.Debug("Bearer prefix removed from token")
	}

	return token, nil
}

func getAllMetadataKeys(md metadata.MD) []string {
	keys := make([]string, 0, len(md))
	for k := range md {
		keys = append(keys, k)
	}
	return keys
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
