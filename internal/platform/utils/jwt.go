package utils

import (
	"context"
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

var _ ports.TokenVerifier = (*JWTVerifier)(nil)

func NewJWTVerifier(secretKey, algorithm string) *JWTVerifier {
	return &JWTVerifier{secretKey: []byte(secretKey), algorithm: algorithm}
}

func NewConfiguredJWTVerifier(cfg config.AuthConfig) *JWTVerifier {
	return NewJWTVerifier(cfg.SecretKey, cfg.Algorithm)
}

func (v *JWTVerifier) Verify(ctx context.Context, tokenString string) (*ports.TokenClaims, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

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

// JWTIssuer signs short-lived access tokens using the configured algorithm and
// secret. It intentionally does not issue refresh tokens.
type JWTIssuer struct {
	secretKey []byte
	algorithm string
}

var _ ports.TokenIssuer = (*JWTIssuer)(nil)

func NewJWTIssuer(secretKey, algorithm string) *JWTIssuer {
	return &JWTIssuer{secretKey: []byte(secretKey), algorithm: algorithm}
}

func NewConfiguredJWTIssuer(cfg config.AuthConfig) *JWTIssuer {
	return NewJWTIssuer(cfg.SecretKey, cfg.Algorithm)
}

func (i *JWTIssuer) Issue(ctx context.Context, subject, role string, ttl time.Duration) (string, time.Time, error) {
	if err := ctx.Err(); err != nil {
		return "", time.Time{}, err
	}
	if ttl <= 0 {
		return "", time.Time{}, errors.New("token ttl must be positive")
	}

	method := jwt.GetSigningMethod(i.algorithm)
	if method == nil {
		return "", time.Time{}, fmt.Errorf("unsupported signing method: %s", i.algorithm)
	}

	now := time.Now().UTC()
	expiresAt := now.Add(ttl)
	claims := jwt.MapClaims{
		"sub":  subject,
		"role": role,
		"iat":  now.Unix(),
		"exp":  expiresAt.Unix(),
	}
	token := jwt.NewWithClaims(method, claims)
	signed, err := token.SignedString(i.secretKey)
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, expiresAt, nil
}

func claimsString(claims jwt.MapClaims, key string) string {
	value, _ := claims[key].(string)
	return value
}
