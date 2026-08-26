package ports

import (
	"context"
	"time"
)

type TokenClaims struct {
	Subject   string
	Issuer    string
	ExpiresAt time.Time
	IssuedAt  time.Time
	Values    map[string]any
}

type TokenVerifier interface {
	Verify(ctx context.Context, token string) (*TokenClaims, error)
}

type TokenIssuer interface {
	Issue(ctx context.Context, subject, role string, ttl time.Duration) (token string, expiresAt time.Time, err error)
}

type PasswordHasher interface {
	Hash(context.Context, string) (string, error)
	Compare(ctx context.Context, encodedHash, password string) error
}
