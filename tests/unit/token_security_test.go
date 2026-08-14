package unit

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"

	"ubertool-backend-trusted/internal/security"
)

// SBR-Trace: SEC-AUTH-001 — JWT algorithm-confusion forgery (alg: none, RS256-key-substitution)
// is rejected by ValidateToken; the defense exists in token.go:113 via the HMAC type-assertion.
// This test constructs forged tokens with wrong algorithms and asserts they are rejected.
func TestTokenManager_AlgorithmConfusionRejection(t *testing.T) {
	const secret = "test-secret-32-bytes-minimum!!" // HS256 requires 32+ bytes
	tm := security.NewTokenManager(secret)

	ctx := t.Context()
	_ = ctx

	// First, generate a valid HS256 token to establish baseline behavior
	validToken, err := tm.GenerateAccessToken(int32(42), "victim@example.com", []string{"user"})
	require.NoError(t, err, "valid HS256 token generation must succeed")

	claims, err := tm.ValidateToken(validToken)
	require.NoError(t, err, "valid HS256 token must validate")
	require.Equal(t, int32(42), claims.UserID)
	require.Equal(t, security.TokenTypeAccess, claims.Type)

	// --- Attack 1: alg: none token ---
	// A token with "alg": "none" and an empty signature should be rejected.
	noneToken := jwt.NewWithClaims(jwt.SigningMethodNone, security.UserClaims{
		UserID: 999,
		Email:  "attacker@example.com",
		Type:   security.TokenTypeAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "999",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "auth-service",
			Audience:  jwt.ClaimStrings{"api-access"},
			ID:        "none-attack",
		},
	})
	// SigningMethodNone produces a token with no signature part
	noneTokenString, err := noneToken.SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err, "alg: none token construction must succeed")

	_, err = tm.ValidateToken(noneTokenString)
	require.Error(t, err, "alg: none token must be rejected")
	require.Equal(t, security.ErrInvalidToken, err, "must return ErrInvalidToken for alg: none")

	// --- Attack 2: RS256-key-substitution (HMAC using public key as secret) ---
	// An attacker who knows the RS256 public key can craft an RS256 token where the
	// "signature" is actually an HMAC computed with that public key as the secret.
	// Our code only accepts HMAC methods (HS256/384/512), so RS256 must be rejected.
	// We use a pre-crafted RS256 token string (header declares "alg":"RS256") with a fake signature.
	rsaTokenString := "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjo4ODgsImVtYWlsIjoiYXR0YWNrZXItcnNhQGV4YW1wbGUuY29tIiwidHlwZSI6ImFjY2VzcyIsInN1YiI6Ijg4OCIsImV4cCI6OTk5OTk5OTk5OSwiaWF0IjoxNzI0MTQ3MjAwLCJpc3MiOiJhdXRoLXNlcnZpY2UiLCJhdWQiOlsiYXBpLWFjY2VzcyJdLCJpZCI6InJzYS1hdHRhY2sifQ.fake-signature"
	_, err = tm.ValidateToken(rsaTokenString)
	require.Error(t, err, "RS256 token must be rejected (algorithm confusion)")
	require.Equal(t, security.ErrInvalidToken, err, "must return ErrInvalidToken for RS256")

	// --- Attack 3: HS384/HS512 (other HMAC methods) ---
	// The type-assertion `token.Method.(*jwt.SigningMethodHMAC)` accepts ALL HMAC methods,
	// not just HS256. This is actually correct behavior — HS384/HS512 are also HMAC.
	// But our token generator only ever produces HS256. If we ever changed the generator
	// to use HS384, the validator would still accept it. This test documents that
	// HS384/HS512 are NOT rejected by the current defense (they are valid HMAC).
	// If this ever needs to be restricted to HS256-only, that would be a separate change.
	hs384Token := jwt.NewWithClaims(jwt.SigningMethodHS384, security.UserClaims{
		UserID: 777,
		Email:  "hs384-test@example.com",
		Type:   security.TokenTypeAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "777",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "auth-service",
			Audience:  jwt.ClaimStrings{"api-access"},
			ID:        "hs384-test",
		},
	})
	hs384TokenString, err := hs384Token.SignedString([]byte(secret))
	require.NoError(t, err, "HS384 token construction must succeed")

	// HS384 with the same secret SHOULD validate (it's a valid HMAC method)
	// This confirms the type-assertion accepts all HMAC, not just HS256
	claims, err = tm.ValidateToken(hs384TokenString)
	require.NoError(t, err, "HS384 with correct secret must validate (all HMAC accepted)")
	require.Equal(t, int32(777), claims.UserID)

	// But HS384 with WRONG secret must fail (signature verification fails)
	hs384TokenWrongSecret, err := hs384Token.SignedString([]byte("wrong-secret-32-bytes-minimum!!"))
	require.NoError(t, err)
	_, err = tm.ValidateToken(hs384TokenWrongSecret)
	require.Error(t, err, "HS384 with wrong secret must be rejected")
	require.Equal(t, security.ErrInvalidToken, err)

	// --- Attack 4: Malformed/garbage token ---
	_, err = tm.ValidateToken("not.a.valid.token")
	require.Error(t, err, "malformed token must be rejected")
	require.Equal(t, security.ErrInvalidToken, err)

	// --- Attack 5: Empty token ---
	_, err = tm.ValidateToken("")
	require.Error(t, err, "empty token must be rejected")
	require.Equal(t, security.ErrInvalidToken, err)

	// Note: The test with SigningMethodNone above already covers the alg: none case.
	// A manually crafted token with "alg":"none" header and a signature would be
	// parsed as SigningMethodNone by jwt-go and rejected by our HMAC type-assertion.
}
