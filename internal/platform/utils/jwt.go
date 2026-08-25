package utils

import (
	"errors"
	"fmt"
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/config"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
	"github.com/golang-jwt/jwt/v5"
)

var ErrInvalidToken = errors.New("invalid token")

type JWTVerifier struct {
	secretKey []byte
	algorithm string
}

func NewJWTVerifier(secretKey, algorithm string) *JWTVerifier {
	return &JWTVerifier{secretKey: []byte(secretKey), algorithm: algorithm}
}

func NewConfiguredJWTVerifier(cfg config.AuthConfig) *JWTVerifier {
	return NewJWTVerifier(cfg.SecretKey, cfg.Algorithm)
}

func (v *JWTVerifier) Verify(tokenString string) (*ports.TokenClaims, error) {
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{v.algorithm}),
		jwt.WithExpirationRequired(),
	)
	token, err := parser.Parse(tokenString, func(token *jwt.Token) (any, error) {
		if token.Method.Alg() != v.algorithm {
			return nil, fmt.Errorf("unexpected signing method: %s", token.Method.Alg())
		}
		return v.secretKey, nil
	})
	if err != nil || token == nil || !token.Valid {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, ErrInvalidToken
	}

	expiresAt, err := claims.GetExpirationTime()
	if err != nil || expiresAt == nil || expiresAt.Time.Before(time.Now()) {
		return nil, ErrInvalidToken
	}

	result := &ports.TokenClaims{
		Subject:   claimsString(claims, "sub"),
		Issuer:    claimsString(claims, "iss"),
		Values:    map[string]any(claims),
		ExpiresAt: expiresAt.Time,
	}
	if issuedAt, err := claims.GetIssuedAt(); err == nil && issuedAt != nil {
		result.IssuedAt = issuedAt.Time
	}
	return result, nil
}

func claimsString(claims jwt.MapClaims, key string) string {
	value, _ := claims[key].(string)
	return value
}
