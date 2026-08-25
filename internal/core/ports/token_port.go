package ports

import "time"

type TokenClaims struct {
	Subject   string
	Issuer    string
	ExpiresAt time.Time
	IssuedAt  time.Time
	Values    map[string]any
}

type TokenVerifier interface {
	Verify(string) (*TokenClaims, error)
}
