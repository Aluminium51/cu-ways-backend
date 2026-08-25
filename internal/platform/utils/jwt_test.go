package utils

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestJWTVerifierAcceptsValidToken(t *testing.T) {
	verifier := NewJWTVerifier("test-secret", "HS256")
	token := signedToken(t, jwt.SigningMethodHS256, "test-secret", jwt.MapClaims{
		"sub": "user-1",
		"exp": time.Now().Add(time.Minute).Unix(),
	})

	claims, err := verifier.Verify(token)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Subject != "user-1" {
		t.Fatalf("expected user-1, got %q", claims.Subject)
	}
}

func TestJWTVerifierRejectsExpiredToken(t *testing.T) {
	verifier := NewJWTVerifier("test-secret", "HS256")
	token := signedToken(t, jwt.SigningMethodHS256, "test-secret", jwt.MapClaims{
		"exp": time.Now().Add(-time.Minute).Unix(),
	})

	if _, err := verifier.Verify(token); err != ErrInvalidToken {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

func TestJWTVerifierRejectsWrongSignature(t *testing.T) {
	verifier := NewJWTVerifier("test-secret", "HS256")
	token := signedToken(t, jwt.SigningMethodHS256, "wrong-secret", jwt.MapClaims{
		"exp": time.Now().Add(time.Minute).Unix(),
	})

	if _, err := verifier.Verify(token); err != ErrInvalidToken {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

func TestJWTVerifierRejectsUnsupportedAlgorithm(t *testing.T) {
	verifier := NewJWTVerifier("test-secret", "HS256")
	token := signedToken(t, jwt.SigningMethodHS384, "test-secret", jwt.MapClaims{
		"exp": time.Now().Add(time.Minute).Unix(),
	})

	if _, err := verifier.Verify(token); err != ErrInvalidToken {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

func signedToken(t *testing.T, method jwt.SigningMethod, secret string, claims jwt.MapClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(method, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	return signed
}
