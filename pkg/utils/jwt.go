package utils

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var (
	ErrInvalidToken = errors.New("invalid or expired token")
	ErrExpiredToken = errors.New("token has expired")
)

type JWTClaims struct {
	UserID   uuid.UUID `json:"sub"`
	Email    string    `json:"email"`
	Username string    `json:"username"`
	jwt.RegisteredClaims
}

// GenerateToken generates a signed HMAC-SHA256 JWT access token.
func GenerateToken(userID uuid.UUID, email, username, secret string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := JWTClaims{
		UserID:   userID,
		Email:    email,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("failed to sign jwt: %w", err)
	}

	return tokenString, nil
}

// ValidateToken parses and validates a signed JWT string.
func ValidateToken(tokenString, secret string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})

	if err != nil {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*JWTClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	if claims.ExpiresAt != nil && time.Now().After(claims.ExpiresAt.Time) {
		return nil, ErrExpiredToken
	}

	return claims, nil
}
