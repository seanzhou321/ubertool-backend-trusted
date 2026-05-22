package security

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// rateLimiterEntry holds a token bucket limiter and the last time it was used.
// The lastSeen field allows a background goroutine to evict stale entries.
type rateLimiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// IPRateLimiter maintains per-IP token bucket limiters for Login and Verify2FA.
// Each bucket starts with a burst of 3 tokens and refills at 1 token per 3 minutes,
// allowing a real user to make 3 rapid attempts before being throttled to at most
// ~20 attempts per hour for sustained attacks.
type IPRateLimiter struct {
	loginLimiters     sync.Map // map[string]*rateLimiterEntry
	verify2FALimiters sync.Map // map[string]*rateLimiterEntry
}

// NewIPRateLimiter creates an IPRateLimiter and starts a background cleanup goroutine.
// Entries not seen within 10 minutes are evicted to prevent unbounded memory growth.
func NewIPRateLimiter() *IPRateLimiter {
	rl := &IPRateLimiter{}
	go rl.cleanupLoop()
	return rl
}

// AllowLogin reports whether the given IP is permitted to attempt a login.
// Burst: 3 tokens. Refill: 1 token per 3 minutes.
func (rl *IPRateLimiter) AllowLogin(ip string) bool {
	return rl.allow(&rl.loginLimiters, ip)
}

// AllowVerify2FA reports whether the given IP is permitted to attempt 2FA verification.
// Burst: 3 tokens. Refill: 1 token per 3 minutes.
func (rl *IPRateLimiter) AllowVerify2FA(ip string) bool {
	return rl.allow(&rl.verify2FALimiters, ip)
}

func (rl *IPRateLimiter) allow(m *sync.Map, ip string) bool {
	v, _ := m.LoadOrStore(ip, &rateLimiterEntry{
		limiter:  rate.NewLimiter(rate.Every(3*time.Minute), 3),
		lastSeen: time.Now(),
	})
	entry := v.(*rateLimiterEntry)
	entry.lastSeen = time.Now()
	return entry.limiter.Allow()
}

// cleanupLoop evicts limiters that have not been used in the last 10 minutes.
// Runs every 5 minutes; the goroutine lives for the lifetime of the process.
func (rl *IPRateLimiter) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		cutoff := time.Now().Add(-10 * time.Minute)
		rl.loginLimiters.Range(func(k, v any) bool {
			if v.(*rateLimiterEntry).lastSeen.Before(cutoff) {
				rl.loginLimiters.Delete(k)
			}
			return true
		})
		rl.verify2FALimiters.Range(func(k, v any) bool {
			if v.(*rateLimiterEntry).lastSeen.Before(cutoff) {
				rl.verify2FALimiters.Delete(k)
			}
			return true
		})
	}
}
