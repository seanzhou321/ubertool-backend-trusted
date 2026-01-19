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

## 4. Method-to-Token Mapping

### 4.1 Proto Service Definition

```protobuf
syntax = "proto3";

package api.v1;

import "google/protobuf/timestamp.proto";
import "google/protobuf/empty.proto";

// Authentication Service
service AuthService {
  // PUBLIC - No token required
  rpc SignUp(SignUpRequest) returns (SignUpResponse);
  rpc InitiatePasswordReset(PasswordResetRequest) returns (PasswordResetResponse);
  rpc HealthCheck(google.protobuf.Empty) returns (HealthCheckResponse);
  
  // PUBLIC - Returns 2FA token or access token
  rpc Login(LoginRequest) returns (LoginResponse);
  
  // 2FA TOKEN REQUIRED
  rpc Verify2FA(Verify2FARequest) returns (TokenResponse);
  rpc ResendCode(ResendCodeRequest) returns (ResendCodeResponse);
  
  // REFRESH TOKEN REQUIRED
  rpc RefreshToken(RefreshTokenRequest) returns (TokenResponse);
  
  // ACCESS TOKEN REQUIRED
  rpc Logout(LogoutRequest) returns (google.protobuf.Empty);
  rpc ChangePassword(ChangePasswordRequest) returns (google.protobuf.Empty);
}

// User Service
service UserService {
  // PUBLIC - Search available to all
  rpc SearchPublicUsers(SearchRequest) returns (SearchResponse);
  
  // ACCESS TOKEN REQUIRED
  rpc GetProfile(GetProfileRequest) returns (UserProfile);
  rpc UpdateProfile(UpdateProfileRequest) returns (UserProfile);
  rpc DeleteAccount(DeleteAccountRequest) returns (google.protobuf.Empty);
  rpc ListUsers(ListUsersRequest) returns (ListUsersResponse);
}

// Data Service
service DataService {
  // ACCESS TOKEN REQUIRED
  rpc CreateRecord(CreateRecordRequest) returns (Record);
  rpc GetRecord(GetRecordRequest) returns (Record);
  rpc UpdateRecord(UpdateRecordRequest) returns (Record);
  rpc DeleteRecord(DeleteRecordRequest) returns (google.protobuf.Empty);
  rpc ListRecords(ListRecordsRequest) returns (ListRecordsResponse);
}

// Message definitions
message SignUpRequest {
  string email = 1;
  string password = 2;
  string username = 3;
}

message SignUpResponse {
  string user_id = 1;
  string message = 2;
}

message LoginRequest {
  string email = 1;
  string password = 2;
}

message LoginResponse {
  bool requires_2fa = 1;
  string temp_token = 2;        // 2FA token if 2FA required
  string access_token = 3;      // Access token if 2FA not required
  string refresh_token = 4;     // Refresh token if 2FA not required
  int64 expires_in = 5;
}

message Verify2FARequest {
  string code = 1;
  // 2FA token sent in metadata
}

message TokenResponse {
  string access_token = 1;
  string refresh_token = 2;
  int64 expires_in = 3;
}

message RefreshTokenRequest {
  // Refresh token sent in metadata
}

message GetProfileRequest {
  string user_id = 1;
}

message UserProfile {
  string user_id = 1;
  string email = 2;
  string username = 3;
  google.protobuf.Timestamp created_at = 4;
}

message SearchRequest {
  string query = 1;
  int32 limit = 2;
  int32 offset = 3;
}

message SearchResponse {
  repeated UserProfile users = 1;
  int32 total_count = 2;
}
```

### 4.2 Endpoint Security Configuration

```go
// config/security_config.go
package config

type SecurityLevel int

const (
    SecurityPublic SecurityLevel = iota  // No authentication
    Security2FA                          // 2FA token required
    SecurityRefresh                      // Refresh token required
    SecurityAccess                       // Access token required
)

// EndpointSecurityConfig maps methods to their required security level
var EndpointSecurityConfig = map[string]SecurityLevel{
    // AuthService - Public
    "/api.v1.AuthService/SignUp":                SecurityPublic,
    "/api.v1.AuthService/InitiatePasswordReset": SecurityPublic,
    "/api.v1.AuthService/HealthCheck":           SecurityPublic,
    "/api.v1.AuthService/Login":                 SecurityPublic,
    
    // AuthService - 2FA Protected
    "/api.v1.AuthService/Verify2FA":    Security2FA,
    "/api.v1.AuthService/ResendCode":   Security2FA,
    
    // AuthService - Refresh Protected
    "/api.v1.AuthService/RefreshToken": SecurityRefresh,
    
    // AuthService - Access Protected
    "/api.v1.AuthService/Logout":         SecurityAccess,
    "/api.v1.AuthService/ChangePassword": SecurityAccess,
    
    // UserService - Public
    "/api.v1.UserService/SearchPublicUsers": SecurityPublic,
    
    // UserService - Access Protected
    "/api.v1.UserService/GetProfile":    SecurityAccess,
    "/api.v1.UserService/UpdateProfile": SecurityAccess,
    "/api.v1.UserService/DeleteAccount": SecurityAccess,
    "/api.v1.UserService/ListUsers":     SecurityAccess,
    
    // DataService - All Access Protected
    "/api.v1.DataService/CreateRecord": SecurityAccess,
    "/api.v1.DataService/GetRecord":    SecurityAccess,
    "/api.v1.DataService/UpdateRecord": SecurityAccess,
    "/api.v1.DataService/DeleteRecord": SecurityAccess,
    "/api.v1.DataService/ListRecords":  SecurityAccess,
}

// GetSecurityLevel returns the security level for a given method
func GetSecurityLevel(method string) SecurityLevel {
    if level, exists := EndpointSecurityConfig[method]; exists {
        return level
    }
    // Default to highest security for unknown endpoints
    return SecurityAccess
}
```

---

## 6. Client-Side Implementation

### 6.1 Token Manager

```go
// pkg/client/token_manager.go
package client

import (
    "sync"
    "time"

    "github.com/golang-jwt/jwt/v5"
)

type TokenManager struct {
    mu           sync.RWMutex
    accessToken  string
    refreshToken string
    twoFAToken   string
    expiresAt    time.Time
}

func NewTokenManager() *TokenManager {
    return &TokenManager{}
}

// Store2FAToken stores the temporary 2FA token
func (tm *TokenManager) Store2FAToken(token string) {
    tm.mu.Lock()
    defer tm.mu.Unlock()
    tm.twoFAToken = token
}

// Get2FAToken retrieves the 2FA token
func (tm *TokenManager) Get2FAToken() string {
    tm.mu.RLock()
    defer tm.mu.RUnlock()
    return tm.twoFAToken
}

// Clear2FAToken clears the 2FA token after successful verification
func (tm *TokenManager) Clear2FAToken() {
    tm.mu.Lock()
    defer tm.mu.Unlock()
    tm.twoFAToken = ""
}

// StoreTokens stores access and refresh tokens
func (tm *TokenManager) StoreTokens(accessToken, refreshToken string, expiresIn int64) {
    tm.mu.Lock()
    defer tm.mu.Unlock()
    tm.accessToken = accessToken
    tm.refreshToken = refreshToken
    tm.expiresAt = time.Now().Add(time.Duration(expiresIn) * time.Second)
}

// GetAccessToken retrieves the current access token
func (tm *TokenManager) GetAccessToken() string {
    tm.mu.RLock()
    defer tm.mu.RUnlock()
    return tm.accessToken
}

// GetRefreshToken retrieves the refresh token
func (tm *TokenManager) GetRefreshToken() string {
    tm.mu.RLock()
    defer tm.mu.RUnlock()
    return tm.refreshToken
}

// IsAccessTokenExpired checks if the access token is expired or about to expire
func (tm *TokenManager) IsAccessTokenExpired() bool {
    tm.mu.RLock()
    defer tm.mu.RUnlock()
    
    if tm.accessToken == "" {
        return true
    }
    
    // Consider token expired if less than 5 minutes remaining
    return time.Now().Add(5 * time.Minute).After(tm.expiresAt)
}

// ClearTokens clears all stored tokens
func (tm *TokenManager) ClearTokens() {
    tm.mu.Lock()
    defer tm.mu.Unlock()
    tm.accessToken = ""
    tm.refreshToken = ""
    tm.twoFAToken = ""
    tm.expiresAt = time.Time{}
}

// GetTokenClaims parses and returns claims from a token
func (tm *TokenManager) GetTokenClaims(token string) (jwt.MapClaims, error) {
    parser := jwt.NewParser()
    parsedToken, _, err := parser.ParseUnverified(token, jwt.MapClaims{})
    if err != nil {
        return nil, err
    }
    
    claims, ok := parsedToken.Claims.(jwt.MapClaims)
    if !ok {
        return nil, jwt.ErrInvalidType
    }
    
    return claims, nil
}
```

### 6.2 Auth Interceptor (Client-Side)

```go
// pkg/client/auth_interceptor.go
package client

import (
    "context"
    "fmt"

    "google.golang.org/grpc"
    "google.golang.org/grpc/metadata"
)

type ClientAuthInterceptor struct {
    tokenManager  *TokenManager
    authClient    AuthClient // Interface for calling auth service
    publicMethods map[string]bool
}

func NewClientAuthInterceptor(
    tokenManager *TokenManager,
    authClient AuthClient,
) *ClientAuthInterceptor {
    return &ClientAuthInterceptor{
        tokenManager: tokenManager,
        authClient:   authClient,
        publicMethods: map[string]bool{
            "/api.v1.AuthService/SignUp":                true,
            "/api.v1.AuthService/InitiatePasswordReset": true,
            "/api.v1.AuthService/HealthCheck":           true,
            "/api.v1.AuthService/Login":                 true,
            "/api.v1.UserService/SearchPublicUsers":     true,
        },
    }
}

// UnaryInterceptor adds authentication to unary calls
func (i *ClientAuthInterceptor) UnaryInterceptor() grpc.UnaryClientInterceptor {
    return func(
        ctx context.Context,
        method string,
        req, reply interface{},
        cc *grpc.ClientConn,
        invoker grpc.UnaryInvoker,
        opts ...grpc.CallOption,
    ) error {
        // Check if method is public
        if i.publicMethods[method] {
            return invoker(ctx, method, req, reply, cc, opts...)
        }

        // Determine which token to use based on method
        token := i.getTokenForMethod(method)
        if token == "" {
            return fmt.Errorf("no valid token available for method %s", method)
        }

        // Check if access token needs refresh
        if method != "/api.v1.AuthService/RefreshToken" && 
           method != "/api.v1.AuthService/Verify2FA" &&
           i.tokenManager.IsAccessTokenExpired() {
            if err := i.refreshAccessToken(ctx); err != nil {
                return fmt.Errorf("failed to refresh token: %w", err)
            }
            token = i.tokenManager.GetAccessToken()
        }

        // Add token to metadata
        ctx = i.attachToken(ctx, token)

        return invoker(ctx, method, req, reply, cc, opts...)
    }
}

// StreamInterceptor adds authentication to streaming calls
func (i *ClientAuthInterceptor) StreamInterceptor() grpc.StreamClientInterceptor {
    return func(
        ctx context.Context,
        desc *grpc.StreamDesc,
        cc *grpc.ClientConn,
        method string,
        streamer grpc.Streamer,
        opts ...grpc.CallOption,
    ) (grpc.ClientStream, error) {
        if i.publicMethods[method] {
            return streamer(ctx, desc, cc, method, opts...)
        }

        token := i.getTokenForMethod(method)
        if token == "" {
            return nil, fmt.Errorf("no valid token available for method %s", method)
        }

        if i.tokenManager.IsAccessTokenExpired() {
            if err := i.refreshAccessToken(ctx); err != nil {
                return nil, fmt.Errorf("failed to refresh token: %w", err)
            }
            token = i.tokenManager.GetAccessToken()
        }

        ctx = i.attachToken(ctx, token)

        return streamer(ctx, desc, cc, method, opts...)
    }
}

// getTokenForMethod determines which token to use based on the method
func (i *ClientAuthInterceptor) getTokenForMethod(method string) string {
    switch method {
    case "/api.v1.AuthService/Verify2FA", "/api.v1.AuthService/ResendCode":
        return i.tokenManager.Get2FAToken()
    case "/api.v1.AuthService/RefreshToken":
        return i.tokenManager.GetRefreshToken()
    default:
        return i.tokenManager.GetAccessToken()
    }
}

// attachToken attaches the token to context metadata
func (i *ClientAuthInterceptor) attachToken(ctx context.Context, token string) context.Context {
    return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)
}

// refreshAccessToken uses refresh token to get new access token
func (i *ClientAuthInterceptor) refreshAccessToken(ctx context.Context) error {
    refreshToken := i.tokenManager.GetRefreshToken()
    if refreshToken == "" {
        return fmt.Errorf("no refresh token available")
    }

    // Call refresh token endpoint
    resp, err := i.authClient.RefreshToken(ctx, refreshToken)
    if err != nil {
        i.tokenManager.ClearTokens()
        return err
    }

    // Store new tokens
    i.tokenManager.StoreTokens(resp.AccessToken, resp.RefreshToken, resp.ExpiresIn)
    return nil
}

// AuthClient interface for auth service operations
type AuthClient interface {
    RefreshToken(ctx context.Context, refreshToken string) (*TokenResponse, error)
}

type TokenResponse struct {
    AccessToken  string
    RefreshToken string
    ExpiresIn    int64
}
```

### 6.3 Client Implementation

```go
// pkg/client/grpc_client.go
package client

import (
    "context"
    "fmt"
    "time"

    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"

    pb "yourproject/api/v1"
)

type GRPCClient struct {
    conn         *grpc.ClientConn
    authClient   pb.AuthServiceClient
    userClient   pb.UserServiceClient
    dataClient   pb.DataServiceClient
    tokenManager *TokenManager
}

func NewGRPCClient(serverAddress string) (*GRPCClient, error) {
    tokenManager := NewTokenManager()
    
    // Create auth client implementation for interceptor
    authClientImpl := &authClientImpl{tokenManager: tokenManager}
    
    // Create interceptor
    authInterceptor := NewClientAuthInterceptor(tokenManager, authClientImpl)

    // Establish connection with interceptors
    conn, err := grpc.Dial(
        serverAddress,
        grpc.WithTransportCredentials(insecure.NewCredentials()),
        grpc.WithUnaryInterceptor(authInterceptor.UnaryInterceptor()),
        grpc.WithStreamInterceptor(authInterceptor.StreamInterceptor()),
    )
    if err != nil {
        return nil, fmt.Errorf("failed to connect: %w", err)
    }

    client := &GRPCClient{
        conn:         conn,
        authClient:   pb.NewAuthServiceClient(conn),
        userClient:   pb.NewUserServiceClient(conn),
        dataClient:   pb.NewDataServiceClient(conn),
        tokenManager: tokenManager,
    }

    // Set the actual auth client for the interceptor
    authClientImpl.client = client

    return client, nil
}

func (c *GRPCClient) Close() error {
    return c.conn.Close()
}

// Authentication Methods

func (c *GRPCClient) SignUp(ctx context.Context, email, password, username string) (string, error) {
    resp, err := c.authClient.SignUp(ctx, &pb.SignUpRequest{
        Email:    email,
        Password: password,
        Username: username,
    })
    if err != nil {
        return "", err
    }
    return resp.UserId, nil
}

func (c *GRPCClient) Login(ctx context.Context, email, password string) (*LoginResult, error) {
    resp, err := c.authClient.Login(ctx, &pb.LoginRequest{
        Email:    email,
        Password: password,
    })
    if err != nil {
        return nil, err
    }

    result := &LoginResult{
        Requires2FA: resp.Requires_2Fa,
    }

    if resp.Requires_2Fa {
        // Store 2FA token
        c.tokenManager.Store2FAToken(resp.TempToken)
        result.ExpiresIn = resp.ExpiresIn
    } else {
        // Store access and refresh tokens
        c.tokenManager.StoreTokens(resp.AccessToken, resp.RefreshToken, resp.ExpiresIn)
    }

    return result, nil
}

func (c *GRPCClient) Verify2FA(ctx context.Context, code string) error {
    resp, err := c.authClient.Verify2FA(ctx, &pb.Verify2FARequest{
        Code: code,
    })
    if err != nil {
        return err
    }

    // Clear 2FA token and store full tokens
    c.tokenManager.Clear2FAToken()
    c.tokenManager.StoreTokens(resp.AccessToken, resp.RefreshToken, resp.ExpiresIn)

    return nil
}

func (c *GRPCClient) Logout(ctx context.Context) error {
    _, err := c.authClient.Logout(ctx, &pb.LogoutRequest{})
    if err != nil {
        return err
    }

    c.tokenManager.ClearTokens()
    return nil
}

// User Methods (Protected)

func (c *GRPCClient) GetProfile(ctx context.Context, userID string) (*pb.UserProfile, error) {
    return c.userClient.GetProfile(ctx, &pb.GetProfileRequest{
        UserId: userID,
    })
}

func (c *GRPCClient) UpdateProfile(ctx context.Context, profile *pb.UserProfile) (*pb.UserProfile, error) {
    return c.userClient.UpdateProfile(ctx, &pb.UpdateProfileRequest{
        Profile: profile,
    })
}

// Public Search

func (c *GRPCClient) SearchPublicUsers(ctx context.Context, query string, limit, offset int32) (*pb.SearchResponse, error) {
    return c.userClient.SearchPublicUsers(ctx, &pb.SearchRequest{
        Query:  query,
        Limit:  limit,
        Offset: offset,
    })
}

// Data Methods (Protected)

func (c *GRPCClient) CreateRecord(ctx context.Context, data string) (*pb.Record, error) {
    return c.dataClient.CreateRecord(ctx, &pb.CreateRecordRequest{
        Data: data,
    })
}

type LoginResult struct {
    Requires2FA bool
    ExpiresIn   int64
}

// authClientImpl implements AuthClient interface for the interceptor
type authClientImpl struct {
    client       *GRPCClient
    tokenManager *TokenManager
}

func (a *authClientImpl) RefreshToken(ctx context.Context, refreshToken string) (*TokenResponse, error) {
    // Add refresh token to context
    ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+refreshToken)
    
    resp, err := a.client.authClient.RefreshToken(ctx, &pb.RefreshTokenRequest{})
    if err != nil {
        return nil, err
    }

    return &TokenResponse{
        AccessToken:  resp.AccessToken,
        RefreshToken: resp.RefreshToken,
        ExpiresIn:    resp.ExpiresIn,
    }, nil
}
```

### 6.4 Client Usage Example

```go
// examples/client_example.go
package main

import (
    "context"
    "log"
    "time"

    "yourproject/pkg/client"
)

func main() {
    // Create client
    grpcClient, err := client.NewGRPCClient("localhost:50051")
    if err != nil {
        log.Fatalf("Failed to create client: %v", err)
    }
    defer grpcClient.Close()

    ctx := context.Background()

    // Example 1: Sign up (public endpoint)
    userID, err := grpcClient.SignUp(ctx, "user@example.com", "password123", "johndoe")
    if err != nil {
        log.Fatalf("SignUp failed: %v", err)
    }
    log.Printf("User created: %s", userID)

    // Example 2: Login without 2FA
    loginResult, err := grpcClient.Login(ctx, "user@example.com", "password123")
    if err != nil {
        log.Fatalf("Login failed: %v", err)
    }

    if !loginResult.Requires2FA {
        log.Println("Login successful, tokens stored")
        
        // Use protected endpoint
        profile, err := grpcClient.GetProfile(ctx, userID)
        if err != nil {
            log.Fatalf("GetProfile failed: %v", err)
        }
        log.Printf("Profile: %+v", profile)
    }

    // Example 3: Login with 2FA
    loginResult, err = grpcClient.Login(ctx, "user-with-2fa@example.com", "password123")
    if err != nil {
        log.Fatalf("Login failed: %v", err)
    }

    if loginResult.Requires2FA {
        log.Println("2FA required, enter code:")
        
        // In real application, get code from user input
        code := "123456"
        
        err = grpcClient.Verify2FA(ctx, code)
        if err != nil {
            log.Fatalf("2FA verification failed: %v", err)
        }
        log.Println("2FA verification successful, tokens stored")
    }

    // Example 4: Using protected endpoints (automatic token refresh)
    for i := 0; i < 5; i++ {
        profile, err := grpcClient.GetProfile(ctx, userID)
        if err != nil {
            log.Fatalf("GetProfile failed: %v", err)
        }
        log.Printf("Profile retrieved: %s", profile.Username)
        time.Sleep(20 * time.Minute) // Token will auto-refresh
    }

    // Example 5: Public search (no authentication)
    searchResults, err := grpcClient.SearchPublicUsers(ctx, "john", 10, 0)
    if err != nil {
        log.Fatalf("SearchPublicUsers failed: %v", err)
    }
    log.Printf("Found %d users", searchResults.TotalCount)

    // Example 6: Logout
    err = grpcClient.Logout(ctx)
    if err != nil {
        log.Fatalf("Logout failed: %v", err)
    }
    log.Println("Logged out successfully")
}
```

