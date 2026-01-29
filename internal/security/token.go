package security

import (
	"errors"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrExpiredToken = errors.New("token has expired")
	ErrWrongTokenType = errors.New("wrong token type for this endpoint")
)

type TokenType string

const (
	TokenTypeAccess     TokenType = "access"
	TokenTypeRefresh    TokenType = "refresh"
	TokenType2FAPending TokenType = "2fa_pending"
)

// UserClaims defines the standard claims for our application
type UserClaims struct {
	UserID    int32     `json:"user_id"` // Standard field for our application
	Email     string    `json:"email,omitempty"`
	Type      TokenType `json:"type"`
	Scope     []string  `json:"scope,omitempty"`
	Roles     []string  `json:"roles,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
	AuthMethod string    `json:"2fa_method,omitempty"` 
	jwt.RegisteredClaims
}

type TokenManager interface {
	GenerateAccessToken(userID int32, email string, roles []string) (string, error)
	GenerateRefreshToken(userID int32, email string) (string, error)
	Generate2FAToken(userID int32, method string) (string, error)
	ValidateToken(tokenString string) (*UserClaims, error)
}

type tokenManager struct {
	secret []byte
}

func NewTokenManager(secret string) TokenManager {
	return &tokenManager{
		secret: []byte(secret),
	}
}

func (m *tokenManager) GenerateAccessToken(userID int32, email string, roles []string) (string, error) {
	claims := UserClaims{
		UserID:  userID,
		Email:   email,
		Type:    TokenTypeAccess,
		Roles:   roles,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.Itoa(int(userID)),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)), // 1 hour
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "auth-service",
			Audience:  jwt.ClaimStrings{"api-access"},
			ID:        generateJTI(),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

func (m *tokenManager) GenerateRefreshToken(userID int32, email string) (string, error) {
	claims := UserClaims{
		UserID: userID,
		Email:  email,
		Type:   TokenTypeRefresh,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.Itoa(int(userID)),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * 7 * time.Hour)), // 7 days
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "auth-service",
			Audience:  jwt.ClaimStrings{"token-refresh"},
			ID:        generateJTI(),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

func (m *tokenManager) Generate2FAToken(userID int32, method string) (string, error) {
	claims := UserClaims{
		UserID:     userID,
		Type:       TokenType2FAPending,
		AuthMethod: method,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.Itoa(int(userID)),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(10 * time.Minute)), // 10 minutes
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "auth-service",
			Audience:  jwt.ClaimStrings{"2fa-verification"},
			ID:        generateJTI(),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

func (m *tokenManager) ValidateToken(tokenString string) (*UserClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &UserClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return m.secret, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}

	if claims, ok := token.Claims.(*UserClaims); ok && token.Valid {
		// Populate UserID from Subject if it was lost (though we set both)
		if claims.UserID == 0 && claims.Subject != "" {
			uid, _ := strconv.Atoi(claims.Subject)
			claims.UserID = int32(uid)
		}
		return claims, nil
	}

	return nil, ErrInvalidToken
}

// Simple unique ID generator
func generateJTI() string {
	return strconv.FormatInt(time.Now().UnixNano(), 16)
}
