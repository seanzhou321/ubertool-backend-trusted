package interceptor

import (
	"context"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	"ubertool-backend-trusted/internal/logger"
	"ubertool-backend-trusted/internal/security"
)

const (
	methodLogin     = "/ubertool.trusted.api.v1.AuthService/Login"
	methodVerify2FA = "/ubertool.trusted.api.v1.AuthService/Verify2FA"
)

// RateLimitInterceptor guards Login and Verify2FA with per-IP token buckets.
// All other endpoints are passed through without restriction.
type RateLimitInterceptor struct {
	limiter *security.IPRateLimiter
}

// NewRateLimitInterceptor returns a RateLimitInterceptor backed by the given limiter.
func NewRateLimitInterceptor(limiter *security.IPRateLimiter) *RateLimitInterceptor {
	return &RateLimitInterceptor{limiter: limiter}
}

// Unary returns a gRPC unary server interceptor that enforces rate limits.
func (i *RateLimitInterceptor) Unary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		var allowed bool
		switch info.FullMethod {
		case methodLogin:
			ip := peerIP(ctx)
			allowed = i.limiter.AllowLogin(ip)
			if !allowed {
				logger.Warn("Login rate limit exceeded", "ip", ip)
				return nil, status.Errorf(codes.ResourceExhausted, "too many login attempts, please try again later")
			}
		case methodVerify2FA:
			ip := peerIP(ctx)
			allowed = i.limiter.AllowVerify2FA(ip)
			if !allowed {
				logger.Warn("Verify2FA rate limit exceeded", "ip", ip)
				return nil, status.Errorf(codes.ResourceExhausted, "too many attempts, please try again later")
			}
		}
		return handler(ctx, req)
	}
}

// peerIP extracts the client IP address from the gRPC peer context, stripping the port.
func peerIP(ctx context.Context) string {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return "unknown"
	}
	host, _, err := net.SplitHostPort(p.Addr.String())
	if err != nil {
		return p.Addr.String()
	}
	return host
}
