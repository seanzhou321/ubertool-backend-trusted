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

## 11. Deployment Considerations

### 11.1 Environment Configuration

```go
// config/config.go
package config

import (
    "os"
    "time"
)

type Config struct {
    // Server
    ServerAddress string
    ServiceName   string
    
    // JWT
    JWTAccessSecret  string
    JWTRefreshSecret string
    AccessTokenTTL   time.Duration
    RefreshTokenTTL  time.Duration
    TwoFATokenTTL    time.Duration
    
    // Database
    DatabaseURL string
    
    // Redis (for token blacklist)
    RedisURL string
    
    // TLS
    TLSEnabled  bool
    TLSCertFile string
    TLSKeyFile  string
    
    // Rate Limiting
    RateLimitEnabled bool
    RateLimitPerMin  int
    
    // Logging
    LogLevel string
}

func LoadConfig() *Config {
    return &Config{
        ServerAddress:    getEnv("SERVER_ADDRESS", "localhost:50051"),
        ServiceName:      getEnv("SERVICE_NAME", "auth-service"),
        JWTAccessSecret:  getEnv("JWT_ACCESS_SECRET", ""),
        JWTRefreshSecret: getEnv("JWT_REFRESH_SECRET", ""),
        AccessTokenTTL:   getDurationEnv("ACCESS_TOKEN_TTL", 1*time.Hour),
        RefreshTokenTTL:  getDurationEnv("REFRESH_TOKEN_TTL", 7*24*time.Hour),
        TwoFATokenTTL:    getDurationEnv("2FA_TOKEN_TTL", 10*time.Minute),
        DatabaseURL:      getEnv("DATABASE_URL", ""),
        RedisURL:         getEnv("REDIS_URL", ""),
        TLSEnabled:       getBoolEnv("TLS_ENABLED", true),
        TLSCertFile:      getEnv("TLS_CERT_FILE", ""),
        TLSKeyFile:       getEnv("TLS_KEY_FILE", ""),
        RateLimitEnabled: getBoolEnv("RATE_LIMIT_ENABLED", true),
        RateLimitPerMin:  getIntEnv("RATE_LIMIT_PER_MIN", 60),
        LogLevel:         getEnv("LOG_LEVEL", "info"),
    }
}

func getEnv(key, defaultVal string) string {
    if val := os.Getenv(key); val != "" {
        return val
    }
    return defaultVal
}

func getDurationEnv(key string, defaultVal time.Duration) time.Duration {
    if val := os.Getenv(key); val != "" {
        if d, err := time.ParseDuration(val); err == nil {
            return d
        }
    }
    return defaultVal
}

func getBoolEnv(key string, defaultVal bool) bool {
    if val := os.Getenv(key); val != "" {
        return val == "true"
    }
    return defaultVal
}

func getIntEnv(key string, defaultVal int) int {
    if val := os.Getenv(key); val != "" {
        if i, err := strconv.Atoi(val); err == nil {
            return i
        }
    }
    return defaultVal
}
```

### 11.2 Docker Deployment

```dockerfile
# Dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o server ./cmd/server

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/server .

# Expose gRPC port
EXPOSE 50051

CMD ["./server"]
```

```yaml
# docker-compose.yml
version: '3.8'

services:
  auth-service:
    build: .
    ports:
      - "50051:50051"
    environment:
      - SERVER_ADDRESS=:50051
      - SERVICE_NAME=auth-service
      - JWT_ACCESS_SECRET=${JWT_ACCESS_SECRET}
      - JWT_REFRESH_SECRET=${JWT_REFRESH_SECRET}
      - DATABASE_URL=postgresql://user:pass@postgres:5432/authdb
      - REDIS_URL=redis://redis:6379
      - TLS_ENABLED=true
      - TLS_CERT_FILE=/certs/server.crt
      - TLS_KEY_FILE=/certs/server.key
      - RATE_LIMIT_ENABLED=true
      - LOG_LEVEL=info
    volumes:
      - ./certs:/certs
    depends_on:
      - postgres
      - redis
    restart: unless-stopped

  postgres:
    image: postgres:15-alpine
    environment:
      - POSTGRES_USER=user
      - POSTGRES_PASSWORD=pass
      - POSTGRES_DB=authdb
    volumes:
      - postgres_data:/var/lib/postgresql/data
    ports:
      - "5432:5432"

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data

volumes:
  postgres_data:
  redis_data:
```

### 11.3 Kubernetes Deployment

```yaml
# k8s/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: auth-service
  labels:
    app: auth-service
spec:
  replicas: 3
  selector:
    matchLabels:
      app: auth-service
  template:
    metadata:
      labels:
        app: auth-service
    spec:
      containers:
      - name: auth-service
        image: your-registry/auth-service:latest
        ports:
        - containerPort: 50051
          name: grpc
        env:
        - name: SERVER_ADDRESS
          value: ":50051"
        - name: SERVICE_NAME
          value: "auth-service"
        - name: JWT_ACCESS_SECRET
          valueFrom:
            secretKeyRef:
              name: jwt-secrets
              key: access-secret
        - name: JWT_REFRESH_SECRET
          valueFrom:
            secretKeyRef:
              name: jwt-secrets
              key: refresh-secret
        - name: DATABASE_URL
          valueFrom:
            secretKeyRef:
              name: database-secrets
              key: url
        - name: REDIS_URL
          value: "redis://redis-service:6379"
        - name: TLS_ENABLED
          value: "true"
        - name: TLS_CERT_FILE
          value: "/certs/tls.crt"
        - name: TLS_KEY_FILE
          value: "/certs/tls.key"
        volumeMounts:
        - name: tls-certs
          mountPath: /certs
          readOnly: true
        livenessProbe:
          grpc:
            port: 50051
          initialDelaySeconds: 10
          periodSeconds: 10
        readinessProbe:
          grpc:
            port: 50051
          initialDelaySeconds: 5
          periodSeconds: 5
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 500m
            memory: 512Mi
      volumes:
      - name: tls-certs
        secret:
          secretName: tls-certs

---
apiVersion: v1
kind: Service
metadata:
  name: auth-service
spec:
  type: LoadBalancer
  ports:
  - port: 50051
    targetPort: 50051
    protocol: TCP
    name: grpc
  selector:
    app: auth-service

---
apiVersion: v1
kind: Secret
metadata:
  name: jwt-secrets
type: Opaque
stringData:
  access-secret: "your-access-secret-here"
  refresh-secret: "your-refresh-secret-here"

---
apiVersion: v1
kind: Secret
metadata:
  name: database-secrets
type: Opaque
stringData:
  url: "postgresql://user:pass@postgres:5432/authdb"
```

### 11.4 Monitoring and Observability

```go
// internal/middleware/metrics.go
package middleware

import (
    "context"
    "time"

    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
    "google.golang.org/grpc"
    "google.golang.org/grpc/status"
)

var (
    requestsTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "grpc_requests_total",
            Help: "Total number of gRPC requests",
        },
        []string{"method", "status"},
    )

    requestDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "grpc_request_duration_seconds",
            Help:    "Duration of gRPC requests",
            Buckets: prometheus.DefBuckets,
        },
        []string{"method"},
    )

    authFailures = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "auth_failures_total",
            Help: "Total number of authentication failures",
        },
        []string{"reason"},
    )

    activeTokens = promauto.NewGauge(
        prometheus.GaugeOpts{
            Name: "active_tokens_count",
            Help: "Number of active tokens",
        },
    )
)

func MetricsInterceptor() grpc.UnaryServerInterceptor {
    return func(
        ctx context.Context,
        req interface{},
        info *grpc.UnaryServerInfo,
        handler grpc.UnaryHandler,
    ) (interface{}, error) {
        start := time.Now()

        resp, err := handler(ctx, req)

        duration := time.Since(start).Seconds()
        statusCode := "OK"
        if err != nil {
            statusCode = status.Code(err).String()
        }

        requestsTotal.WithLabelValues(info.FullMethod, statusCode).Inc()
        requestDuration.WithLabelValues(info.FullMethod).Observe(duration)

        return resp, err
    }
}
```

---

## 12. Security Checklist

### 12.1 Pre-Production Checklist

- [ ] **Secrets Management**
  - [ ] JWT secrets stored in secure vault (not in code)
  - [ ] Secrets rotated regularly
  - [ ] Different secrets for different environments
  - [ ] Minimum secret length of 32 characters

- [ ] **Transport Security**
  - [ ] TLS enabled for all gRPC connections
  - [ ] Valid TLS certificates (not self-signed in production)
  - [ ] Minimum TLS version 1.2
  - [ ] Strong cipher suites configured

- [ ] **Token Configuration**
  - [ ] Access token expiry set to 1 hour or less
  - [ ] Refresh token expiry appropriate for use case
  - [ ] 2FA token expiry set to 10 minutes or less
  - [ ] Token rotation implemented for refresh tokens

- [ ] **Authentication**
  - [ ] Password complexity requirements enforced
  - [ ] Passwords hashed with bcrypt/argon2
  - [ ] Account lockout after failed attempts
  - [ ] Rate limiting on auth endpoints
  - [ ] 2FA available and encouraged

- [ ] **Authorization**
  - [ ] Role-based access control (RBAC) implemented
  - [ ] Permission checks on all protected endpoints
  - [ ] Principle of least privilege applied
  - [ ] Token claims validated on each request

- [ ] **Monitoring**
  - [ ] Authentication failures logged
  - [ ] Suspicious activity alerts configured
  - [ ] Token usage metrics tracked
  - [ ] Security events sent to SIEM

- [ ] **Compliance**
  - [ ] GDPR compliance for user data
  - [ ] Data encryption at rest and in transit
  - [ ] Audit logs maintained
  - [ ] Privacy policy updated

---

## 13. Troubleshooting Guide

### 13.1 Common Issues

| Issue | Symptoms | Solution |
|-------|----------|----------|
| Token validation fails | `UNAUTHENTICATED` errors on all requests | Verify JWT secrets match between token generation and validation |
| 2FA token not accepted | Valid 2FA code rejected | Check token type validation - ensure 2FA endpoint accepts 2FA tokens |
| Refresh token fails | Cannot refresh access token | Verify refresh token uses separate secret and audience |
| Public endpoints require auth | Sign-up fails with auth error | Check endpoint security configuration mapping |
| Token expires immediately | Access token invalid right after login | Verify server time synchronization (NTP) |
| Client can't connect | Connection refused errors | Check server address, port, and firewall rules |
| Interceptor not called | Auth bypassed unexpectedly | Verify interceptor registration in server setup |
| Token in wrong format | Parsing errors | Ensure "Bearer " prefix in Authorization header |

### 13.2 Debugging Steps

1. **Enable Debug Logging**
   ```go
   // Add detailed logging to interceptor
   log.Printf("Method: %s, Token: %s (first 20 chars)", info.FullMethod, token[:20])
   ```

2. **Verify Token Claims**
   ```go
   // Decode without validation to inspect claims
   token, _, _ := new(jwt.Parser).ParseUnverified(tokenString, jwt.MapClaims{})
   claims := token.Claims.(jwt.MapClaims)
   log.Printf("Claims: %+v", claims)
   ```

3. **Test with grpcurl**
   ```bash
   # Test public endpoint
   grpcurl -plaintext localhost:50051 api.v1.AuthService/HealthCheck

   # Test with token
   grpcurl -plaintext \
     -H "authorization: Bearer YOUR_TOKEN" \
     localhost:50051 api.v1.UserService/GetProfile
   ```

4. **Check Time Synchronization**
   ```bash
   # Verify server time
   date
   # Sync with NTP
   sudo ntpdate pool.ntp.org
   ```

---

## 14. Appendix

### 14.1 Complete Project Structure

```
yourproject/
├── api/
│   └── v1/
│       ├── auth_service.proto
│       ├── user_service.proto
│       └── data_service.proto
├── cmd/
│   └── server/
│       └── main.go
├── config/
│   ├── config.go
│   └── security_config.go
├── internal/
│   ├── auth/
│   │   ├── token_service.go
│   │   └── token_service_test.go
│   ├── middleware/
│   │   ├── auth_interceptor.go
│   │   ├── auth_interceptor_test.go
│   │   └── metrics.go
│   ├── service/
│   │   ├── auth_service.go
│   │   ├── user_service.go
│   │   └── data_service.go
│   ├── repository/
│   │   └── user_repository.go
│   └── errors/
│       └── auth_errors.go
├── pkg/
│   └── client/
│       ├── grpc_client.go
│       ├── token_manager.go
│       ├── auth_interceptor.go
│       └── error_handler.go
├── test/
│   ├── integration/
│   │   └── auth_flow_test.go
│   └── load/
│       └── auth_load_test.go
├── examples/
│   └── client_example.go
├── deployments/
│   ├── docker/
│   │   ├── Dockerfile
│   │   └── docker-compose.yml
│   └── k8s/
│       ├── deployment.yaml
│       └── service.yaml
├── scripts/
│   ├── generate_certs.sh
│   └── migrate_db.sh
├── docs/
│   └── api/
│       └── openapi.yaml
├── .env.example
├── .gitignore
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

### 14.2 Makefile

```makefile
.PHONY: proto build test run docker-build docker-up clean

# Generate protobuf code
proto:
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		api/v1/*.proto

# Build server
build:
	go build -o bin/server cmd/server/main.go

# Run tests
test:
	go test -v -race -coverprofile=coverage.out ./...

# Run integration tests
test-integration:
	go test -v -tags=integration ./test/integration/...

# Run load tests
test-load:
	go test -v -bench=. ./test/load/...

# Run server locally
run:
	go run cmd/server/main.go

# Docker build
docker-build:
	docker build -t auth-service:latest .

# Docker compose up
docker-up:
	docker-compose up -d

# Docker compose down
docker-down:
	docker-compose down

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f coverage.out

# Generate TLS certificates for development
certs:
	./scripts/generate_certs.sh

# Database migration
migrate:
	./scripts/migrate_db.sh

# Run linter
lint:
	golangci-lint run

# Format code
fmt:
	go fmt ./...

# Install dependencies
deps:
	go mod download
	go mod tidy
```

### 14.3 Environment Variables Reference

```bash
# .env.example

# Server Configuration
SERVER_ADDRESS=:50051
SERVICE_NAME=auth-service

# JWT Configuration
JWT_ACCESS_SECRET=your-super-secret-access-key-change-this-in-production
JWT_REFRESH_SECRET=your-super-secret-refresh-key-change-this-in-production
ACCESS_TOKEN_TTL=1h
REFRESH_TOKEN_TTL=168h  # 7 days
2FA_TOKEN_TTL=10m

# Database
DATABASE_URL=postgresql://user:password@localhost:5432/authdb

# Redis
REDIS_URL=redis://localhost:6379

# TLS
TLS_ENABLED=true
TLS_CERT_FILE=/path/to/cert.pem
TLS_KEY_FILE=/path/to/key.pem

# Rate Limiting
RATE_LIMIT_ENABLED=true
RATE_LIMIT_PER_MIN=60

# Logging
LOG_LEVEL=info

# 2FA
TWILIO_ACCOUNT_SID=your-twilio-sid
TWILIO_AUTH_TOKEN=your-twilio-token
TWILIO_FROM_NUMBER=+1234567890

# Email
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USERNAME=your-email@gmail.com
SMTP_PASSWORD=your-app-password
```

### 14.4 Additional Resources

- **gRPC Documentation**: https://grpc.io/docs/languages/go/
- **JWT Best Practices**: https://tools.ietf.org/html/rfc8725
- **Go JWT Library**: https://github.com/golang-jwt/jwt
- **Prometheus Metrics**: https://prometheus.io/docs/guides/go-application/
- **gRPC Health Checking**: https://github.com/grpc/grpc/blob/master/doc/health-checking.md

---

## 15. Conclusion

This design document provides a comprehensive guide for implementing JWT-based authentication in a Go gRPC microservice. The architecture supports:

1. **Multiple authentication flows**: Standard login, 2FA-protected login, and token refresh
2. **Fine-grained access control**: Different token types for different endpoint categories
3. **Security best practices**: TLS encryption, token expiry, rate limiting, and monitoring
4. **Scalability**: Stateless authentication suitable for microservices
5. **Developer experience**: Clear client SDKs with automatic token management

### Next Steps

1. Review and customize the security configuration for your specific requirements
2. Implement the core components (token service, interceptors, services)
3. Set up comprehensive testing (unit, integration, load tests)
4. Configure monitoring and alerting
5. Deploy to staging environment for testing
6. Conduct security audit before production deployment
7. Document API usage for client developers
8. Set up CI/CD pipeline with security scanning

### Maintenance

- Regularly rotate JWT secrets
- Monitor authentication failure rates
- Update dependencies for security patches
- Review and update rate limits based on usage patterns
- Conduct periodic security audits
- Keep documentation up-to-date with changes

---

**Document Version**: 1.0  
**Last Updated**: January 17, 2026  
**Review Date**: April 17, 2026
    