# JWT Authentication Design Document for gRPC Microservice

**Version:** 1.0  
**Date:** January 17, 2026  
**Author:** System Architecture Team  
**Status:** Design Document

---

## Table of Contents

1. [Overview](#1-overview)
2. [Architecture](#2-architecture)
3. [Token Structure](#3-token-structure)
4. [Method-to-Token Mapping](#4-method-to-token-mapping)
5. [Server-Side Implementation](#5-server-side-implementation)
6. [Client-Side Implementation](#6-client-side-implementation)
7. [Authentication Flow](#7-authentication-flow)
8. [Security Considerations](#8-security-considerations)
9. [Error Handling](#9-error-handling)
10. [Testing Strategy](#10-testing-strategy)

---

## 1. Overview

### 1.1 Purpose

This document describes the JWT-based authentication system for a gRPC microservice implemented in Go. The system uses different JWT token types to protect different categories of endpoints, ensuring appropriate security boundaries throughout the authentication flow.

### 1.2 Goals

- Implement stateless authentication using JWT tokens
- Support multi-factor authentication (2FA) flow
- Provide fine-grained access control based on token types
- Ensure public endpoints remain accessible without authentication
- Enable token refresh mechanism for seamless user experience

### 1.3 Scope

This design covers:
- JWT token generation and validation
- gRPC interceptors for authentication
- Client-side token management
- Method-level security configuration
- Complete authentication flows (login, 2FA, token refresh)

---

## 2. Architecture

### 2.1 Token Types

The system uses four distinct JWT token types:

| Token Type | Purpose | Expiry | Scope | Audience |
|------------|---------|--------|-------|----------|
| **2FA Token** | Temporary token after password verification | 10 minutes | `verify_2fa` | `2fa-verification` |
| **Access Token** | Full API access after complete authentication | 1 hour | User permissions | `api-access` |
| **Refresh Token** | Long-lived token for renewing access tokens | 7 days | `refresh_token` | `token-refresh` |
| **None** | Public endpoints - no token required | N/A | N/A | N/A |

### 2.2 Component Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      gRPC Client                             │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │ Token Store  │  │ Auth Manager │  │ Auto-Refresh │      │
│  └──────────────┘  └──────────────┘  └──────────────┘      │
└─────────────────────────────────────────────────────────────┘
                            │
                            │ gRPC with metadata
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                    gRPC Interceptor                          │
│  ┌──────────────────────────────────────────────────────┐   │
│  │  Token Extraction → Validation → Context Injection   │   │
│  └──────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                    Service Handlers                          │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │ AuthService  │  │ UserService  │  │ DataService  │      │
│  └──────────────┘  └──────────────┘  └──────────────┘      │
└─────────────────────────────────────────────────────────────┘
```

### 2.3 Endpoint Security Levels

gRPC methods are categorized into four security levels:

1. **Public Endpoints** - No authentication required
   - User registration
   - Password reset initiation
   - Public search methods
   - Health checks

2. **2FA Protected Endpoints** - Requires 2FA token
   - 2FA code verification
   - Resend 2FA code

3. **Refresh Protected Endpoints** - Requires refresh token
   - Token refresh

4. **Protected Endpoints** - Requires access token
   - All business logic methods
   - User profile management
   - CRUD operations

---

## 3. Token Structure

### 3.1 2FA Token Claims

```json
{
  "sub": "user_id_123",
  "type": "2fa_pending",
  "scope": ["verify_2fa"],
  "2fa_method": "totp",
  "jti": "unique_token_id_abc123",
  "exp": 1234567890,
  "iat": 1234567290,
  "iss": "auth-service",
  "aud": "2fa-verification"
}
```

**Field Descriptions:**
- `sub`: Subject (user ID)
- `type`: Token type identifier
- `scope`: Allowed operations
- `2fa_method`: Method used (totp, sms, email)
- `jti`: JWT ID for tracking/revocation
- `exp`: Expiration timestamp
- `iat`: Issued at timestamp
- `iss`: Issuer (service name)
- `aud`: Intended audience

### 3.2 Access Token Claims

```json
{
  "sub": "user_id_123",
  "type": "access",
  "scope": ["read", "write", "delete"],
  "roles": ["user", "admin"],
  "permissions": ["user:read", "user:write", "admin:delete"],
  "jti": "unique_token_id_xyz789",
  "exp": 1234571490,
  "iat": 1234567890,
  "iss": "auth-service",
  "aud": "api-access"
}
```

**Additional Fields:**
- `roles`: User role memberships
- `permissions`: Fine-grained permissions

### 3.3 Refresh Token Claims

```json
{
  "sub": "user_id_123",
  "type": "refresh",
  "scope": ["refresh_token"],
  "jti": "unique_token_id_ref456",
  "exp": 1235172690,
  "iat": 1234567890,
  "iss": "auth-service",
  "aud": "token-refresh"
}
```

---

## 10. Testing Strategy

### 10.1 Unit Tests

#### 10.1.1 Token Service Tests

```go
// internal/auth/token_service_test.go
package auth

import (
    "testing"
    "time"

    "github.com/golang-jwt/jwt/v5"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestTokenService_Generate2FAToken(t *testing.T) {
    ts := NewTokenService("test-secret", "refresh-secret", "test-service")
    
    token, err := ts.Generate2FAToken("user123", "totp")
    require.NoError(t, err)
    assert.NotEmpty(t, token)
    
    // Validate token
    claims, err := ts.ValidateToken(token, TokenType2FA, false)
    require.NoError(t, err)
    assert.Equal(t, "user123", claims.UserID)
    assert.Equal(t, string(TokenType2FA), claims.Type)
    assert.Contains(t, claims.Scope, "verify_2fa")
    assert.Equal(t, "totp", claims.TwoFAMethod)
}

func TestTokenService_GenerateAccessToken(t *testing.T) {
    ts := NewTokenService("test-secret", "refresh-secret", "test-service")
    
    roles := []string{"user", "admin"}
    permissions := []string{"read", "write"}
    
    token, err := ts.GenerateAccessToken("user123", roles, permissions)
    require.NoError(t, err)
    assert.NotEmpty(t, token)
    
    // Validate token
    claims, err := ts.ValidateToken(token, TokenTypeAccess, false)
    require.NoError(t, err)
    assert.Equal(t, "user123", claims.UserID)
    assert.Equal(t, string(TokenTypeAccess), claims.Type)
    assert.Equal(t, roles, claims.Roles)
    assert.Equal(t, permissions, claims.Permissions)
}

func TestTokenService_GenerateRefreshToken(t *testing.T) {
    ts := NewTokenService("test-secret", "refresh-secret", "test-service")
    
    token, err := ts.GenerateRefreshToken("user123")
    require.NoError(t, err)
    assert.NotEmpty(t, token)
    
    // Validate token
    claims, err := ts.ValidateToken(token, TokenTypeRefresh, true)
    require.NoError(t, err)
    assert.Equal(t, "user123", claims.UserID)
    assert.Equal(t, string(TokenTypeRefresh), claims.Type)
}

func TestTokenService_ValidateToken_InvalidType(t *testing.T) {
    ts := NewTokenService("test-secret", "refresh-secret", "test-service")
    
    token, err := ts.Generate2FAToken("user123", "totp")
    require.NoError(t, err)
    
    // Try to validate as access token
    _, err = ts.ValidateToken(token, TokenTypeAccess, false)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "invalid token type")
}

func TestTokenService_ValidateToken_Expired(t *testing.T) {
    ts := NewTokenService("test-secret", "refresh-secret", "test-service")
    
    // Create token that expires immediately
    now := time.Now()
    claims := CustomClaims{
        UserID: "user123",
        Type:   string(TokenTypeAccess),
        Scope:  []string{"api_access"},
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(now.Add(-1 * time.Hour)),
            IssuedAt:  jwt.NewNumericDate(now),
            Issuer:    "test-service",
            Audience:  []string{"api-access"},
        },
    }
    
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    tokenString, err := token.SignedString([]byte("test-secret"))
    require.NoError(t, err)
    
    // Should fail validation
    _, err = ts.ValidateToken(tokenString, TokenTypeAccess, false)
    assert.Error(t, err)
}
```

#### 10.1.2 Auth Interceptor Tests

```go
// internal/middleware/auth_interceptor_test.go
package middleware

import (
    "context"
    "testing"

    "google.golang.org/grpc"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/metadata"
    "google.golang.org/grpc/status"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"

    "yourproject/config"
    "yourproject/internal/auth"
)

func TestAuthInterceptor_PublicEndpoint(t *testing.T) {
    ts := auth.NewTokenService("test-secret", "refresh-secret", "test-service")
    interceptor := NewAuthInterceptor(ts)
    
    // Mock handler
    handler := func(ctx context.Context, req interface{}) (interface{}, error) {
        return "success", nil
    }
    
    info := &grpc.UnaryServerInfo{
        FullMethod: "/api.v1.AuthService/SignUp",
    }
    
    // Should succeed without token
    resp, err := interceptor.UnaryInterceptor()(
        context.Background(),
        nil,
        info,
        handler,
    )
    
    require.NoError(t, err)
    assert.Equal(t, "success", resp)
}

func TestAuthInterceptor_ProtectedEndpoint_NoToken(t *testing.T) {
    ts := auth.NewTokenService("test-secret", "refresh-secret", "test-service")
    interceptor := NewAuthInterceptor(ts)
    
    handler := func(ctx context.Context, req interface{}) (interface{}, error) {
        return "success", nil
    }
    
    info := &grpc.UnaryServerInfo{
        FullMethod: "/api.v1.UserService/GetProfile",
    }
    
    // Should fail without token
    _, err := interceptor.UnaryInterceptor()(
        context.Background(),
        nil,
        info,
        handler,
    )
    
    require.Error(t, err)
    st, ok := status.FromError(err)
    require.True(t, ok)
    assert.Equal(t, codes.Unauthenticated, st.Code())
}

func TestAuthInterceptor_ProtectedEndpoint_ValidToken(t *testing.T) {
    ts := auth.NewTokenService("test-secret", "refresh-secret", "test-service")
    interceptor := NewAuthInterceptor(ts)
    
    // Generate valid token
    token, err := ts.GenerateAccessToken("user123", []string{"user"}, []string{"read"})
    require.NoError(t, err)
    
    // Create context with token
    md := metadata.Pairs("authorization", "Bearer "+token)
    ctx := metadata.NewIncomingContext(context.Background(), md)
    
    handler := func(ctx context.Context, req interface{}) (interface{}, error) {
        // Verify claims in context
        claims, err := GetUserFromContext(ctx)
        if err != nil {
            return nil, err
        }
        assert.Equal(t, "user123", claims.UserID)
        return "success", nil
    }
    
    info := &grpc.UnaryServerInfo{
        FullMethod: "/api.v1.UserService/GetProfile",
    }
    
    resp, err := interceptor.UnaryInterceptor()(ctx, nil, info, handler)
    require.NoError(t, err)
    assert.Equal(t, "success", resp)
}

func TestAuthInterceptor_2FAEndpoint_With2FAToken(t *testing.T) {
    ts := auth.NewTokenService("test-secret", "refresh-secret", "test-service")
    interceptor := NewAuthInterceptor(ts)
    
    // Generate 2FA token
    token, err := ts.Generate2FAToken("user123", "totp")
    require.NoError(t, err)
    
    // Create context with token
    md := metadata.Pairs("authorization", "Bearer "+token)
    ctx := metadata.NewIncomingContext(context.Background(), md)
    
    handler := func(ctx context.Context, req interface{}) (interface{}, error) {
        return "success", nil
    }
    
    info := &grpc.UnaryServerInfo{
        FullMethod: "/api.v1.AuthService/Verify2FA",
    }
    
    resp, err := interceptor.UnaryInterceptor()(ctx, nil, info, handler)
    require.NoError(t, err)
    assert.Equal(t, "success", resp)
}

func TestAuthInterceptor_2FAEndpoint_WithAccessToken(t *testing.T) {
    ts := auth.NewTokenService("test-secret", "refresh-secret", "test-service")
    interceptor := NewAuthInterceptor(ts)
    
    // Generate access token (wrong type)
    token, err := ts.GenerateAccessToken("user123", []string{"user"}, []string{"read"})
    require.NoError(t, err)
    
    // Create context with token
    md := metadata.Pairs("authorization", "Bearer "+token)
    ctx := metadata.NewIncomingContext(context.Background(), md)
    
    handler := func(ctx context.Context, req interface{}) (interface{}, error) {
        return "success", nil
    }
    
    info := &grpc.UnaryServerInfo{
        FullMethod: "/api.v1.AuthService/Verify2FA",
    }
    
    // Should fail with wrong token type
    _, err = interceptor.UnaryInterceptor()(ctx, nil, info, handler)
    require.Error(t, err)
    st, ok := status.FromError(err)
    require.True(t, ok)
    assert.Equal(t, codes.Unauthenticated, st.Code())
}
```

### 10.2 Integration Tests

```go
// test/integration/auth_flow_test.go
package integration

import (
    "context"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"

    pb "yourproject/api/v1"
)

func TestAuthFlow_LoginWithout2FA(t *testing.T) {
    // Setup test server
    server := setupTestServer(t)
    defer server.Stop()
    
    // Create client
    conn, err := grpc.Dial(
        server.Address(),
        grpc.WithTransportCredentials(insecure.NewCredentials()),
    )
    require.NoError(t, err)
    defer conn.Close()
    
    client := pb.NewAuthServiceClient(conn)
    ctx := context.Background()
    
    // Create test user without 2FA
    email := "test@example.com"
    password := "password123"
    createTestUser(t, server, email, password, false)
    
    // Test login
    loginResp, err := client.Login(ctx, &pb.LoginRequest{
        Email:    email,
        Password: password,
    })
    require.NoError(t, err)
    assert.False(t, loginResp.Requires_2Fa)
    assert.NotEmpty(t, loginResp.AccessToken)
    assert.NotEmpty(t, loginResp.RefreshToken)
    
    // Use access token for protected endpoint
    md := metadata.Pairs("authorization", "Bearer "+loginResp.AccessToken)
    ctx = metadata.NewOutgoingContext(ctx, md)
    
    userClient := pb.NewUserServiceClient(conn)
    profile, err := userClient.GetProfile(ctx, &pb.GetProfileRequest{
        UserId: "test-user-id",
    })
    require.NoError(t, err)
    assert.NotNil(t, profile)
}

func TestAuthFlow_LoginWith2FA(t *testing.T) {
    server := setupTestServer(t)
    defer server.Stop()
    
    conn, err := grpc.Dial(
        server.Address(),
        grpc.WithTransportCredentials(insecure.NewCredentials()),
    )
    require.NoError(t, err)
    defer conn.Close()
    
    client := pb.NewAuthServiceClient(conn)
    ctx := context.Background()
    
    // Create test user with 2FA
    email := "test2fa@example.com"
    password := "password123"
    userID := createTestUser(t, server, email, password, true)
    
    // Step 1: Login
    loginResp, err := client.Login(ctx, &pb.LoginRequest{
        Email:    email,
        Password: password,
    })
    require.NoError(t, err)
    assert.True(t, loginResp.Requires_2Fa)
    assert.NotEmpty(t, loginResp.TempToken)
    assert.Empty(t, loginResp.AccessToken)
    
    // Step 2: Get 2FA code (in test, we know the code)
    code := getTest2FACode(t, server, userID)
    
    // Step 3: Verify 2FA
    md := metadata.Pairs("authorization", "Bearer "+loginResp.TempToken)
    ctx = metadata.NewOutgoingContext(ctx, md)
    
    tokenResp, err := client.Verify2FA(ctx, &pb.Verify2FARequest{
        Code: code,
    })
    require.NoError(t, err)
    assert.NotEmpty(t, tokenResp.AccessToken)
    assert.NotEmpty(t, tokenResp.RefreshToken)
    
    // Step 4: Use access token
    md = metadata.Pairs("authorization", "Bearer "+tokenResp.AccessToken)
    ctx = metadata.NewOutgoingContext(context.Background(), md)
    
    userClient := pb.NewUserServiceClient(conn)
    profile, err := userClient.GetProfile(ctx, &pb.GetProfileRequest{
        UserId: userID,
    })
    require.NoError(t, err)
    assert.NotNil(t, profile)
}

func TestAuthFlow_TokenRefresh(t *testing.T) {
    server := setupTestServer(t)
    defer server.Stop()
    
    conn, err := grpc.Dial(
        server.Address(),
        grpc.WithTransportCredentials(insecure.NewCredentials()),
    )
    require.NoError(t, err)
    defer conn.Close()
    
    client := pb.NewAuthServiceClient(conn)
    ctx := context.Background()
    
    // Login
    email := "refresh@example.com"
    password := "password123"
    createTestUser(t, server, email, password, false)
    
    loginResp, err := client.Login(ctx, &pb.LoginRequest{
        Email:    email,
        Password: password,
    })
    require.NoError(t, err)
    
    // Wait for access token to expire (in test, use short expiry)
    time.Sleep(2 * time.Second)
    
    // Refresh token
    md := metadata.Pairs("authorization", "Bearer "+loginResp.RefreshToken)
    ctx = metadata.NewOutgoingContext(ctx, md)
    
    refreshResp, err := client.RefreshToken(ctx, &pb.RefreshTokenRequest{})
    require.NoError(t, err)
    assert.NotEmpty(t, refreshResp.AccessToken)
    assert.NotEmpty(t, refreshResp.RefreshToken)
    assert.NotEqual(t, loginResp.AccessToken, refreshResp.AccessToken)
}

func TestAuthFlow_InvalidCredentials(t *testing.T) {
    server := setupTestServer(t)
    defer server.Stop()
    
    conn, err := grpc.Dial(
        server.Address(),
        grpc.WithTransportCredentials(insecure.NewCredentials()),
    )
    require.NoError(t, err)
    defer conn.Close()
    
    client := pb.NewAuthServiceClient(conn)
    ctx := context.Background()
    
    _, err = client.Login(ctx, &pb.LoginRequest{
        Email:    "nonexistent@example.com",
        Password: "wrongpassword",
    })
    require.Error(t, err)
    
    st, ok := status.FromError(err)
    require.True(t, ok)
    assert.Equal(t, codes.Unauthenticated, st.Code())
}

func TestAuthFlow_ExpiredToken(t *testing.T) {
    server := setupTestServer(t)
    defer server.Stop()
    
    conn, err := grpc.Dial(
        server.Address(),
        grpc.WithTransportCredentials(insecure.NewCredentials()),
    )
    require.NoError(t, err)
    defer conn.Close()
    
    // Create expired token
    expiredToken := createExpiredToken(t, server)
    
    md := metadata.Pairs("authorization", "Bearer "+expiredToken)
    ctx := metadata.NewOutgoingContext(context.Background(), md)
    
    userClient := pb.NewUserServiceClient(conn)
    _, err = userClient.GetProfile(ctx, &pb.GetProfileRequest{
        UserId: "test-user-id",
    })
    require.Error(t, err)
    
    st, ok := status.FromError(err)
    require.True(t, ok)
    assert.Equal(t, codes.Unauthenticated, st.Code())
}

// Helper functions
func setupTestServer(t *testing.T) *TestServer {
    // Implementation of test server setup
    return nil
}

func createTestUser(t *testing.T, server *TestServer, email, password string, enable2FA bool) string {
    // Implementation of test user creation
    return "test-user-id"
}

func getTest2FACode(t *testing.T, server *TestServer, userID string) string {
    // Implementation to get 2FA code
    return "123456"
}

func createExpiredToken(t *testing.T, server *TestServer) string {
    // Implementation to create expired token
    return "expired-token"
}
```

### 10.3 Load Testing

```go
// test/load/auth_load_test.go
package load

import (
    "context"
    "fmt"
    "sync"
    "testing"
    "time"

    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"

    pb "yourproject/api/v1"
)

func BenchmarkLogin(b *testing.B) {
    conn, err := grpc.Dial(
        "localhost:50051",
        grpc.WithTransportCredentials(insecure.NewCredentials()),
    )
    if err != nil {
        b.Fatalf("Failed to connect: %v", err)
    }
    defer conn.Close()
    
    client := pb.NewAuthServiceClient(conn)
    
    b.ResetTimer()
    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            _, err := client.Login(context.Background(), &pb.LoginRequest{
                Email:    "test@example.com",
                Password: "password123",
            })
            if err != nil {
                b.Errorf("Login failed: %v", err)
            }
        }
    })
}

func BenchmarkTokenValidation(b *testing.B) {
    conn, err := grpc.Dial(
        "localhost:50051",
        grpc.WithTransportCredentials(insecure.NewCredentials()),
    )
    if err != nil {
        b.Fatalf("Failed to connect: %v", err)
    }
    defer conn.Close()
    
    // Get a valid token first
    authClient := pb.NewAuthServiceClient(conn)
    loginResp, err := authClient.Login(context.Background(), &pb.LoginRequest{
        Email:    "test@example.com",
        Password: "password123",
    })
    if err != nil {
        b.Fatalf("Failed to login: %v", err)
    }
    
    userClient := pb.NewUserServiceClient(conn)
    md := metadata.Pairs("authorization", "Bearer "+loginResp.AccessToken)
    
    b.ResetTimer()
    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            ctx := metadata.NewOutgoingContext(context.Background(), md)
            _, err := userClient.GetProfile(ctx, &pb.GetProfileRequest{
                UserId: "test-user-id",
            })
            if err != nil {
                b.Errorf("GetProfile failed: %v", err)
            }
        }
    })
}

func TestConcurrentLogins(t *testing.T) {
    conn, err := grpc.Dial(
        "localhost:50051",
        grpc.WithTransportCredentials(insecure.NewCredentials()),
    )
    if err != nil {
        t.Fatalf("Failed to connect: %v", err)
    }
    defer conn.Close()
    
    client := pb.NewAuthServiceClient(conn)
    
    concurrency := 100
    iterations := 1000
    
    var wg sync.WaitGroup
    errors := make(chan error, concurrency*iterations)
    
    start := time.Now()
    
    for i := 0; i < concurrency; i++ {
        wg.Add(1)
        go func(workerID int) {
            defer wg.Done()
            
            for j := 0; j < iterations; j++ {
                _, err := client.Login(context.Background(), &pb.LoginRequest{
                    Email:    fmt.Sprintf("user%d@example.com", workerID),
                    Password: "password123",
                })
                if err != nil {
                    errors <- err
                }
            }
        }(i)
    }
    
    wg.Wait()
    close(errors)
    
    elapsed := time.Since(start)
    totalRequests := concurrency * iterations
    
    errorCount := len(errors)
    successCount := totalRequests - errorCount
    
    t.Logf("Total requests: %d", totalRequests)
    t.Logf("Successful: %d", successCount)
    t.Logf("Failed: %d", errorCount)
    t.Logf("Duration: %s", elapsed)
    t.Logf("Throughput: %.2f req/s", float64(totalRequests)/elapsed.Seconds())
    
    if errorCount > totalRequests/10 {
        t.Errorf("Error rate too high: %d/%d", errorCount, totalRequests)
    }
}
```

---
