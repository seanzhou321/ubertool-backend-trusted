# JWT Authentication Design Document for gRPC Microservice

**Version:** 1.0  
**Date:** January 17, 2026  
**Author:** System Architecture Team  
**Status:** Design Document

---

## 8. Security Considerations

### 8.1 Token Security

**Access Token:**
- Short expiry (1 hour) limits exposure window
- Stored in memory only (not persisted)
- Contains minimal sensitive information
- Should be transmitted only over TLS/HTTPS

**Refresh Token:**
- Longer expiry (7 days) for better UX
- Should be stored securely (encrypted storage)
- Used only for token refresh endpoint
- Rotation on each use prevents replay attacks
- Can be revoked server-side

**2FA Token:**
- Very short expiry (10 minutes)
- Single-use preferred (track JTI)
- Limited scope (only 2FA verification)
- Should be cleared after successful verification

### 8.2 Best Practices

1. **Transport Security**
   ```go
   // Always use TLS in production
   creds, err := credentials.NewClientTLSFromFile("cert.pem", "")
   conn, err := grpc.Dial(serverAddress, grpc.WithTransportCredentials(creds))
   ```

2. **Token Storage**
   - Never store tokens in URL parameters
   - Use secure storage mechanisms (encrypted)
   - Clear tokens on logout
   - Implement token rotation

3. **Rate Limiting**
   ```go
   // Implement rate limiting for auth endpoints
   limiter := ratelimit.NewLimiter(rate.Limit(5), 10) // 5 req/sec, burst 10
   ```

4. **Token Revocation**
   ```go
   // Maintain token blacklist for immediate revocation
   type TokenBlacklist interface {
       Add(jti string, expiry time.Time) error
       IsBlacklisted(jti string) (bool, error)
   }
   ```

5. **Input Validation**
   ```go
   func validateLoginRequest(req *pb.LoginRequest) error {
       if !isValidEmail(req.Email) {
           return errors.New("invalid email format")
       }
       if len(req.Password) < 8 {
           return errors.New("password too short")
       }
       return nil
   }
   ```

### 8.3 Common Vulnerabilities and Mitigations

| Vulnerability | Mitigation |
|--------------|------------|
| Token theft | Short expiry, TLS encryption, secure storage |
| Replay attacks | Single-use 2FA tokens, token rotation, JTI tracking |
| Token leakage | Never log tokens, use secure headers only |
| Brute force | Rate limiting, account lockout, CAPTCHA |
| Session fixation | Generate new tokens after 2FA, token rotation |
| XSS attacks | HTTPOnly cookies (if using cookies), CSP headers |
| CSRF | SameSite cookies, CSRF tokens for state-changing operations |

---

## 9. Error Handling

### 9.1 Error Code Mapping

```go
// internal/errors/auth_errors.go
package errors

import (
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
)

var (
    ErrInvalidCredentials    = status.Error(codes.Unauthenticated, "invalid credentials")
    ErrInvalidToken          = status.Error(codes.Unauthenticated, "invalid or expired token")
    ErrInvalidTokenType      = status.Error(codes.Unauthenticated, "invalid token type")
    ErrMissingToken          = status.Error(codes.Unauthenticated, "missing authentication token")
    ErrInvalid2FACode        = status.Error(codes.Unauthenticated, "invalid 2FA code")
    Err2FACodeExpired        = status.Error(codes.DeadlineExceeded, "2FA code expired")
    ErrInsufficientPermission = status.Error(codes.PermissionDenied, "insufficient permissions")
    ErrUserNotFound          = status.Error(codes.NotFound, "user not found")
    ErrUserAlreadyExists     = status.Error(codes.AlreadyExists, "user already exists")
    ErrInvalidInput          = status.Error(codes.InvalidArgument, "invalid input")
    ErrRateLimitExceeded     = status.Error(codes.ResourceExhausted, "rate limit exceeded")
    ErrInternal              = status.Error(codes.Internal, "internal server error")
)

// IsUnauthenticatedError checks if error is authentication-related
func IsUnauthenticatedError(err error) bool {
    st, ok := status.FromError(err)
    if !ok {
        return false
    }
    return st.Code() == codes.Unauthenticated
}
```

### 9.2 Error Scenarios and Responses

| Scenario | Error Code | Response | Client Action |
|----------|-----------|----------|---------------|
| Invalid credentials | `UNAUTHENTICATED` | "Invalid credentials" | Show error, allow retry |
| Expired access token | `UNAUTHENTICATED` | "Invalid or expired token" | Auto-refresh with refresh token |
| Expired refresh token | `UNAUTHENTICATED` | "Invalid or expired token" | Redirect to login |
| Invalid 2FA code | `UNAUTHENTICATED` | "Invalid 2FA code" | Allow retry (with limit) |
| 2FA code expired | `DEADLINE_EXCEEDED` | "2FA code expired" | Request new code |
| Wrong token type | `UNAUTHENTICATED` | "Invalid token type" | Clear tokens, redirect to login |
| Insufficient permissions | `PERMISSION_DENIED` | "Insufficient permissions" | Show error, don't retry |
| Rate limit exceeded | `RESOURCE_EXHAUSTED` | "Rate limit exceeded" | Implement backoff, retry later |

### 9.3 Client Error Handling Example

```go
// pkg/client/error_handler.go
package client

import (
    "context"
    "time"

    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
)

type ErrorHandler struct {
    tokenManager *TokenManager
    authClient   pb.AuthServiceClient
}

func NewErrorHandler(tokenManager *TokenManager, authClient pb.AuthServiceClient) *ErrorHandler {
    return &ErrorHandler{
        tokenManager: tokenManager,
        authClient:   authClient,
    }
}

// HandleError processes gRPC errors and takes appropriate actions
func (h *ErrorHandler) HandleError(ctx context.Context, err error) error {
    st, ok := status.FromError(err)
    if !ok {
        return err
    }

    switch st.Code() {
    case codes.Unauthenticated:
        return h.handleUnauthenticated(ctx, st)
    case codes.PermissionDenied:
        return h.handlePermissionDenied(st)
    case codes.ResourceExhausted:
        return h.handleRateLimitExceeded(st)
    case codes.DeadlineExceeded:
        return h.handleDeadlineExceeded(st)
    default:
        return err
    }
}

func (h *ErrorHandler) handleUnauthenticated(ctx context.Context, st *status.Status) error {
    msg := st.Message()
    
    // Check if it's an expired token that can be refreshed
    if msg == "invalid or expired token" && h.tokenManager.GetRefreshToken() != "" {
        // Attempt to refresh token
        refreshCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
        defer cancel()
        
        // This would be handled by the interceptor
        return status.Error(codes.Unauthenticated, "token refresh needed")
    }
    
    // Clear all tokens and require re-authentication
    h.tokenManager.ClearTokens()
    return status.Error(codes.Unauthenticated, "authentication required")
}

func (h *ErrorHandler) handlePermissionDenied(st *status.Status) error {
    // Log permission denied for auditing
    return status.Error(codes.PermissionDenied, "insufficient permissions for this operation")
}

func (h *ErrorHandler) handleRateLimitExceeded(st *status.Status) error {
    return status.Error(codes.ResourceExhausted, "rate limit exceeded, please try again later")
}

func (h *ErrorHandler) handleDeadlineExceeded(st *status.Status) error {
    return status.Error(codes.DeadlineExceeded, "request timeout, please try again")
}

// RetryableError checks if an error can be retried
func RetryableError(err error) bool {
    st, ok := status.FromError(err)
    if !ok {
        return false
    }
    
    switch st.Code() {
    case codes.Unavailable, codes.DeadlineExceeded, codes.ResourceExhausted:
        return true
    default:
        return false
    }
}

// WithRetry wraps a function with retry logic
func WithRetry(ctx context.Context, maxRetries int, fn func() error) error {
    var err error
    backoff := time.Second
    
    for i := 0; i <= maxRetries; i++ {
        err = fn()
        if err == nil {
            return nil
        }
        
        if !RetryableError(err) {
            return err
        }
        
        if i < maxRetries {
            select {
            case <-time.After(backoff):
                backoff *= 2 // Exponential backoff
            case <-ctx.Done():
                return ctx.Err()
            }
        }
    }
    
    return err
}
```

---

