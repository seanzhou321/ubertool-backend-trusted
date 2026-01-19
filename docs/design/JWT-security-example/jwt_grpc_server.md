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

## 5. Server-Side Implementation

### 5.1 Token Service

```go
// internal/auth/token_service.go
package auth

import (
    "fmt"
    "time"

    "github.com/golang-jwt/jwt/v5"
)

type TokenType string

const (
    TokenType2FA     TokenType = "2fa_pending"
    TokenTypeAccess  TokenType = "access"
    TokenTypeRefresh TokenType = "refresh"
)

type CustomClaims struct {
    UserID      string   `json:"sub"`
    Type        string   `json:"type"`
    Scope       []string `json:"scope"`
    Roles       []string `json:"roles,omitempty"`
    Permissions []string `json:"permissions,omitempty"`
    TwoFAMethod string   `json:"2fa_method,omitempty"`
    jwt.RegisteredClaims
}

type TokenService struct {
    accessSecret  []byte
    refreshSecret []byte
    issuer        string
}

func NewTokenService(accessSecret, refreshSecret, issuer string) *TokenService {
    return &TokenService{
        accessSecret:  []byte(accessSecret),
        refreshSecret: []byte(refreshSecret),
        issuer:        issuer,
    }
}

// Generate2FAToken creates a temporary token for 2FA verification
func (s *TokenService) Generate2FAToken(userID, method string) (string, error) {
    now := time.Now()
    claims := CustomClaims{
        UserID:      userID,
        Type:        string(TokenType2FA),
        Scope:       []string{"verify_2fa"},
        TwoFAMethod: method,
        RegisteredClaims: jwt.RegisteredClaims{
            Subject:   userID,
            ExpiresAt: jwt.NewNumericDate(now.Add(10 * time.Minute)),
            IssuedAt:  jwt.NewNumericDate(now),
            NotBefore: jwt.NewNumericDate(now),
            Issuer:    s.issuer,
            Audience:  []string{"2fa-verification"},
            ID:        generateJTI(),
        },
    }

    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString(s.accessSecret)
}

// GenerateAccessToken creates a full access token
func (s *TokenService) GenerateAccessToken(userID string, roles, permissions []string) (string, error) {
    now := time.Now()
    claims := CustomClaims{
        UserID:      userID,
        Type:        string(TokenTypeAccess),
        Scope:       []string{"api_access"},
        Roles:       roles,
        Permissions: permissions,
        RegisteredClaims: jwt.RegisteredClaims{
            Subject:   userID,
            ExpiresAt: jwt.NewNumericDate(now.Add(1 * time.Hour)),
            IssuedAt:  jwt.NewNumericDate(now),
            NotBefore: jwt.NewNumericDate(now),
            Issuer:    s.issuer,
            Audience:  []string{"api-access"},
            ID:        generateJTI(),
        },
    }

    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString(s.accessSecret)
}

// GenerateRefreshToken creates a long-lived refresh token
func (s *TokenService) GenerateRefreshToken(userID string) (string, error) {
    now := time.Now()
    claims := CustomClaims{
        UserID: userID,
        Type:   string(TokenTypeRefresh),
        Scope:  []string{"refresh_token"},
        RegisteredClaims: jwt.RegisteredClaims{
            Subject:   userID,
            ExpiresAt: jwt.NewNumericDate(now.Add(7 * 24 * time.Hour)),
            IssuedAt:  jwt.NewNumericDate(now),
            NotBefore: jwt.NewNumericDate(now),
            Issuer:    s.issuer,
            Audience:  []string{"token-refresh"},
            ID:        generateJTI(),
        },
    }

    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString(s.refreshSecret)
}

// ValidateToken validates a token and returns claims
func (s *TokenService) ValidateToken(tokenString string, expectedType TokenType, useRefreshSecret bool) (*CustomClaims, error) {
    secret := s.accessSecret
    if useRefreshSecret {
        secret = s.refreshSecret
    }

    token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
        if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
            return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
        }
        return secret, nil
    })

    if err != nil {
        return nil, fmt.Errorf("failed to parse token: %w", err)
    }

    claims, ok := token.Claims.(*CustomClaims)
    if !ok || !token.Valid {
        return nil, fmt.Errorf("invalid token")
    }

    // Validate token type
    if claims.Type != string(expectedType) {
        return nil, fmt.Errorf("invalid token type: expected %s, got %s", expectedType, claims.Type)
    }

    // Validate issuer
    if claims.Issuer != s.issuer {
        return nil, fmt.Errorf("invalid issuer")
    }

    return claims, nil
}

// Helper function to generate unique JWT ID
func generateJTI() string {
    return fmt.Sprintf("%d-%s", time.Now().UnixNano(), randomString(16))
}

func randomString(length int) string {
    // Implementation of random string generation
    const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
    b := make([]byte, length)
    for i := range b {
        b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
    }
    return string(b)
}
```

### 5.2 Authentication Interceptor

```go
// internal/middleware/auth_interceptor.go
package middleware

import (
    "context"
    "strings"

    "google.golang.org/grpc"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/metadata"
    "google.golang.org/grpc/status"

    "yourproject/config"
    "yourproject/internal/auth"
)

type contextKey string

const (
    UserContextKey contextKey = "user"
)

type AuthInterceptor struct {
    tokenService *auth.TokenService
}

func NewAuthInterceptor(tokenService *auth.TokenService) *AuthInterceptor {
    return &AuthInterceptor{
        tokenService: tokenService,
    }
}

// UnaryInterceptor validates JWT tokens for unary RPCs
func (i *AuthInterceptor) UnaryInterceptor() grpc.UnaryServerInterceptor {
    return func(
        ctx context.Context,
        req interface{},
        info *grpc.UnaryServerInfo,
        handler grpc.UnaryHandler,
    ) (interface{}, error) {
        // Get security level for this method
        securityLevel := config.GetSecurityLevel(info.FullMethod)

        // Public endpoints - no authentication required
        if securityLevel == config.SecurityPublic {
            return handler(ctx, req)
        }

        // Extract token from metadata
        token, err := i.extractToken(ctx)
        if err != nil {
            return nil, status.Error(codes.Unauthenticated, "missing or invalid authorization header")
        }

        // Validate token based on security level
        claims, err := i.validateTokenForSecurityLevel(token, securityLevel)
        if err != nil {
            return nil, status.Error(codes.Unauthenticated, err.Error())
        }

        // Add claims to context
        ctx = context.WithValue(ctx, UserContextKey, claims)

        // Call the handler
        return handler(ctx, req)
    }
}

// StreamInterceptor validates JWT tokens for streaming RPCs
func (i *AuthInterceptor) StreamInterceptor() grpc.StreamServerInterceptor {
    return func(
        srv interface{},
        ss grpc.ServerStream,
        info *grpc.StreamServerInfo,
        handler grpc.StreamHandler,
    ) error {
        ctx := ss.Context()
        securityLevel := config.GetSecurityLevel(info.FullMethod)

        if securityLevel == config.SecurityPublic {
            return handler(srv, ss)
        }

        token, err := i.extractToken(ctx)
        if err != nil {
            return status.Error(codes.Unauthenticated, "missing or invalid authorization header")
        }

        claims, err := i.validateTokenForSecurityLevel(token, securityLevel)
        if err != nil {
            return status.Error(codes.Unauthenticated, err.Error())
        }

        // Wrap the stream with new context
        wrappedStream := &wrappedStream{
            ServerStream: ss,
            ctx:          context.WithValue(ctx, UserContextKey, claims),
        }

        return handler(srv, wrappedStream)
    }
}

// extractToken extracts JWT token from gRPC metadata
func (i *AuthInterceptor) extractToken(ctx context.Context) (string, error) {
    md, ok := metadata.FromIncomingContext(ctx)
    if !ok {
        return "", status.Error(codes.Unauthenticated, "no metadata provided")
    }

    authHeaders := md.Get("authorization")
    if len(authHeaders) == 0 {
        return "", status.Error(codes.Unauthenticated, "no authorization header")
    }

    // Expected format: "Bearer <token>"
    authHeader := authHeaders[0]
    if !strings.HasPrefix(authHeader, "Bearer ") {
        return "", status.Error(codes.Unauthenticated, "invalid authorization format")
    }

    return strings.TrimPrefix(authHeader, "Bearer "), nil
}

// validateTokenForSecurityLevel validates token based on required security level
func (i *AuthInterceptor) validateTokenForSecurityLevel(
    token string,
    level config.SecurityLevel,
) (*auth.CustomClaims, error) {
    switch level {
    case config.Security2FA:
        return i.tokenService.ValidateToken(token, auth.TokenType2FA, false)
    case config.SecurityRefresh:
        return i.tokenService.ValidateToken(token, auth.TokenTypeRefresh, true)
    case config.SecurityAccess:
        return i.tokenService.ValidateToken(token, auth.TokenTypeAccess, false)
    default:
        return nil, status.Error(codes.Internal, "unknown security level")
    }
}

// wrappedStream wraps grpc.ServerStream with custom context
type wrappedStream struct {
    grpc.ServerStream
    ctx context.Context
}

func (w *wrappedStream) Context() context.Context {
    return w.ctx
}

// GetUserFromContext extracts user claims from context
func GetUserFromContext(ctx context.Context) (*auth.CustomClaims, error) {
    claims, ok := ctx.Value(UserContextKey).(*auth.CustomClaims)
    if !ok {
        return nil, status.Error(codes.Unauthenticated, "user not found in context")
    }
    return claims, nil
}
```

### 5.3 Service Implementation Example

```go
// internal/service/auth_service.go
package service

import (
    "context"
    "fmt"

    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"

    pb "yourproject/api/v1"
    "yourproject/internal/auth"
    "yourproject/internal/middleware"
)

type AuthService struct {
    pb.UnimplementedAuthServiceServer
    tokenService *auth.TokenService
    userRepo     UserRepository
    twoFAService TwoFactorAuthService
}

func NewAuthService(
    tokenService *auth.TokenService,
    userRepo UserRepository,
    twoFAService TwoFactorAuthService,
) *AuthService {
    return &AuthService{
        tokenService: tokenService,
        userRepo:     userRepo,
        twoFAService: twoFAService,
    }
}

// SignUp - Public endpoint
func (s *AuthService) SignUp(ctx context.Context, req *pb.SignUpRequest) (*pb.SignUpResponse, error) {
    // Validate input
    if err := validateSignUpRequest(req); err != nil {
        return nil, status.Error(codes.InvalidArgument, err.Error())
    }

    // Create user
    userID, err := s.userRepo.CreateUser(req.Email, req.Password, req.Username)
    if err != nil {
        return nil, status.Error(codes.Internal, "failed to create user")
    }

    return &pb.SignUpResponse{
        UserId:  userID,
        Message: "User created successfully",
    }, nil
}

// Login - Public endpoint, returns 2FA token or full tokens
func (s *AuthService) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
    // Authenticate user
    user, err := s.userRepo.AuthenticateUser(req.Email, req.Password)
    if err != nil {
        return nil, status.Error(codes.Unauthenticated, "invalid credentials")
    }

    // Check if 2FA is enabled
    if user.TwoFactorEnabled {
        // Send 2FA code
        if err := s.twoFAService.SendCode(user.ID, user.TwoFactorMethod); err != nil {
            return nil, status.Error(codes.Internal, "failed to send 2FA code")
        }

        // Generate 2FA token
        twoFAToken, err := s.tokenService.Generate2FAToken(user.ID, user.TwoFactorMethod)
        if err != nil {
            return nil, status.Error(codes.Internal, "failed to generate 2FA token")
        }

        return &pb.LoginResponse{
            Requires_2Fa: true,
            TempToken:    twoFAToken,
            ExpiresIn:    600, // 10 minutes
        }, nil
    }

    // No 2FA - generate full tokens
    accessToken, err := s.tokenService.GenerateAccessToken(user.ID, user.Roles, user.Permissions)
    if err != nil {
        return nil, status.Error(codes.Internal, "failed to generate access token")
    }

    refreshToken, err := s.tokenService.GenerateRefreshToken(user.ID)
    if err != nil {
        return nil, status.Error(codes.Internal, "failed to generate refresh token")
    }

    return &pb.LoginResponse{
        Requires_2Fa: false,
        AccessToken:  accessToken,
        RefreshToken: refreshToken,
        ExpiresIn:    3600, // 1 hour
    }, nil
}

// Verify2FA - Requires 2FA token in metadata
func (s *AuthService) Verify2FA(ctx context.Context, req *pb.Verify2FARequest) (*pb.TokenResponse, error) {
    // Get user from context (extracted by interceptor)
    claims, err := middleware.GetUserFromContext(ctx)
    if err != nil {
        return nil, err
    }

    // Verify 2FA code
    valid, err := s.twoFAService.VerifyCode(claims.UserID, req.Code)
    if err != nil || !valid {
        return nil, status.Error(codes.Unauthenticated, "invalid 2FA code")
    }

    // Get user details
    user, err := s.userRepo.GetUserByID(claims.UserID)
    if err != nil {
        return nil, status.Error(codes.Internal, "failed to get user")
    }

    // Generate full tokens
    accessToken, err := s.tokenService.GenerateAccessToken(user.ID, user.Roles, user.Permissions)
    if err != nil {
        return nil, status.Error(codes.Internal, "failed to generate access token")
    }

    refreshToken, err := s.tokenService.GenerateRefreshToken(user.ID)
    if err != nil {
        return nil, status.Error(codes.Internal, "failed to generate refresh token")
    }

    return &pb.TokenResponse{
        AccessToken:  accessToken,
        RefreshToken: refreshToken,
        ExpiresIn:    3600,
    }, nil
}

// RefreshToken - Requires refresh token in metadata
func (s *AuthService) RefreshToken(ctx context.Context, req *pb.RefreshTokenRequest) (*pb.TokenResponse, error) {
    // Get user from context (refresh token validated by interceptor)
    claims, err := middleware.GetUserFromContext(ctx)
    if err != nil {
        return nil, err
    }

    // Get user details
    user, err := s.userRepo.GetUserByID(claims.UserID)
    if err != nil {
        return nil, status.Error(codes.Internal, "failed to get user")
    }

    // Generate new access token
    accessToken, err := s.tokenService.GenerateAccessToken(user.ID, user.Roles, user.Permissions)
    if err != nil {
        return nil, status.Error(codes.Internal, "failed to generate access token")
    }

    // Optionally rotate refresh token
    refreshToken, err := s.tokenService.GenerateRefreshToken(user.ID)
    if err != nil {
        return nil, status.Error(codes.Internal, "failed to generate refresh token")
    }

    return &pb.TokenResponse{
        AccessToken:  accessToken,
        RefreshToken: refreshToken,
        ExpiresIn:    3600,
    }, nil
}

// Logout - Requires access token
func (s *AuthService) Logout(ctx context.Context, req *pb.LogoutRequest) (*emptypb.Empty, error) {
    claims, err := middleware.GetUserFromContext(ctx)
    if err != nil {
        return nil, err
    }

    // Optionally blacklist the token JTI
    // This requires a token blacklist/revocation mechanism
    if err := s.tokenService.BlacklistToken(claims.ID); err != nil {
        return nil, status.Error(codes.Internal, "failed to logout")
    }

    return &emptypb.Empty{}, nil
}
```

### 5.4 Server Setup

```go
// cmd/server/main.go
package main

import (
    "log"
    "net"
    "os"

    "google.golang.org/grpc"
    "google.golang.org/grpc/reflection"

    pb "yourproject/api/v1"
    "yourproject/config"
    "yourproject/internal/auth"
    "yourproject/internal/middleware"
    "yourproject/internal/service"
)

func main() {
    // Load configuration
    cfg := config.LoadConfig()

    // Initialize token service
    tokenService := auth.NewTokenService(
        cfg.JWTAccessSecret,
        cfg.JWTRefreshSecret,
        cfg.ServiceName,
    )

    // Initialize repositories and services
    userRepo := initUserRepository(cfg)
    twoFAService := initTwoFactorAuthService(cfg)

    // Create auth interceptor
    authInterceptor := middleware.NewAuthInterceptor(tokenService)

    // Create gRPC server with interceptors
    grpcServer := grpc.NewServer(
        grpc.UnaryInterceptor(authInterceptor.UnaryInterceptor()),
        grpc.StreamInterceptor(authInterceptor.StreamInterceptor()),
    )

    // Register services
    authService := service.NewAuthService(tokenService, userRepo, twoFAService)
    pb.RegisterAuthServiceServer(grpcServer, authService)

    userService := service.NewUserService(userRepo)
    pb.RegisterUserServiceServer(grpcServer, userService)

    dataService := service.NewDataService(/* dependencies */)
    pb.RegisterDataServiceServer(grpcServer, dataService)

    // Register reflection service for development
    reflection.Register(grpcServer)

    // Start server
    listener, err := net.Listen("tcp", cfg.ServerAddress)
    if err != nil {
        log.Fatalf("Failed to listen: %v", err)
    }

    log.Printf("Server listening on %s", cfg.ServerAddress)
    if err := grpcServer.Serve(listener); err != nil {
        log.Fatalf("Failed to serve: %v", err)
    }
}

func initUserRepository(cfg *config.Config) service.UserRepository {
    // Initialize your user repository (database connection, etc.)
    return nil // Replace with actual implementation
}

func initTwoFactorAuthService(cfg *config.Config) service.TwoFactorAuthService {
    // Initialize your 2FA service
    return nil // Replace with actual implementation
}
```

---
