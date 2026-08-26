package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
)

type fakePasswordHasher struct {
	hashValue  string
	hashErr    error
	compareErr error
	lastInput  string
}

func (f *fakePasswordHasher) Hash(ctx context.Context, password string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	f.lastInput = password
	if f.hashErr != nil {
		return "", f.hashErr
	}
	if f.hashValue == "" {
		f.hashValue = "argon2id-hash"
	}
	return f.hashValue, nil
}

func (f *fakePasswordHasher) Compare(ctx context.Context, _, _ string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return f.compareErr
}

type fakeTokenIssuer struct {
	token     string
	expiresAt time.Time
	subject   string
	role      string
	ttl       time.Duration
}

func (f *fakeTokenIssuer) Issue(ctx context.Context, subject, role string, ttl time.Duration) (string, time.Time, error) {
	if err := ctx.Err(); err != nil {
		return "", time.Time{}, err
	}
	f.subject = subject
	f.role = role
	f.ttl = ttl
	if f.token == "" {
		f.token = "test-token"
	}
	if f.expiresAt.IsZero() {
		f.expiresAt = time.Now().Add(ttl)
	}
	return f.token, f.expiresAt, nil
}

func TestAuthServiceRegisterCreatesUserAndIssuesUserToken(t *testing.T) {
	repo := newFakeUserRepository()
	hasher := &fakePasswordHasher{}
	issuer := &fakeTokenIssuer{}
	service := NewAuthService(repo, hasher, issuer)

	phone := "0812345678"
	result, err := service.Register(context.Background(), RegisterInput{
		Name:     " Jane Doe ",
		Email:    " JANE@EXAMPLE.COM ",
		Password: "correct horse battery staple",
		Phone:    &phone,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.User.Name != "Jane Doe" || result.User.Email != "jane@example.com" {
		t.Fatalf("expected normalized user, got %+v", result.User)
	}
	if result.User.Role != domain.RoleUser || result.User.PasswordHash == nil || *result.User.PasswordHash != "argon2id-hash" {
		t.Fatalf("expected user role and password hash, got %+v", result.User)
	}
	if result.Token != "test-token" || issuer.subject != "1" || issuer.role != domain.RoleUser || issuer.ttl != AccessTokenTTL {
		t.Fatalf("unexpected issued token: %+v, issuer=%+v", result, issuer)
	}
	if hasher.lastInput != "correct horse battery staple" {
		t.Fatal("expected password to be passed to the hasher")
	}
}

func TestAuthServiceRegisterRejectsDuplicateEmail(t *testing.T) {
	repo := newFakeUserRepository(&domain.User{UserID: 1, Email: "jane@example.com"})
	service := NewAuthService(repo, &fakePasswordHasher{}, &fakeTokenIssuer{})

	_, err := service.Register(context.Background(), RegisterInput{
		Name:     "Jane",
		Email:    "JANE@example.com",
		Password: "correct horse battery staple",
	})
	if !errors.Is(err, domain.ErrEmailAlreadyExists) {
		t.Fatalf("expected duplicate email error, got %v", err)
	}
}

func TestAuthServiceLoginIssuesAdminToken(t *testing.T) {
	hash := "stored-hash"
	repo := newFakeUserRepository(&domain.User{
		UserID:       7,
		Name:         "Admin",
		Email:        "admin@example.com",
		Role:         domain.RoleAdmin,
		PasswordHash: &hash,
	})
	issuer := &fakeTokenIssuer{}
	service := NewAuthService(repo, &fakePasswordHasher{}, issuer)

	result, err := service.Login(context.Background(), LoginInput{
		Email:    " ADMIN@EXAMPLE.COM ",
		Password: "correct horse battery staple",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.User.UserID != 7 || result.Token != "test-token" || issuer.subject != "7" || issuer.role != domain.RoleAdmin {
		t.Fatalf("unexpected login result: %+v, issuer=%+v", result, issuer)
	}
}

func TestAuthServiceLoginUsesGenericInvalidCredentials(t *testing.T) {
	wrongPassword := errors.New("wrong password")
	hash := "stored-hash"
	repo := newFakeUserRepository(&domain.User{UserID: 1, Email: "jane@example.com", PasswordHash: &hash})
	service := NewAuthService(repo, &fakePasswordHasher{compareErr: wrongPassword}, &fakeTokenIssuer{})

	_, err := service.Login(context.Background(), LoginInput{Email: "jane@example.com", Password: "correct horse battery staple"})
	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("expected generic invalid credentials, got %v", err)
	}

	_, err = service.Login(context.Background(), LoginInput{Email: "missing@example.com", Password: "correct horse battery staple"})
	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("expected generic missing-user credentials error, got %v", err)
	}
}

func TestAuthServiceLoginRejectsDeletedAndPasswordlessUsers(t *testing.T) {
	deletedAt := time.Now()
	hash := "stored-hash"
	repo := newFakeUserRepository(
		&domain.User{UserID: 1, Email: "deleted@example.com", PasswordHash: &hash, DeletedAt: &deletedAt},
		&domain.User{UserID: 2, Email: "passwordless@example.com"},
	)
	service := NewAuthService(repo, &fakePasswordHasher{}, &fakeTokenIssuer{})

	for _, email := range []string{"deleted@example.com", "passwordless@example.com"} {
		if _, err := service.Login(context.Background(), LoginInput{Email: email, Password: "correct horse battery staple"}); !errors.Is(err, domain.ErrInvalidCredentials) {
			t.Fatalf("expected invalid credentials for %s, got %v", email, err)
		}
	}
}

func TestAuthServiceRegisterPropagatesCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	service := NewAuthService(newFakeUserRepository(), &fakePasswordHasher{}, &fakeTokenIssuer{})

	_, err := service.Register(ctx, RegisterInput{
		Name:     "Jane",
		Email:    "jane@example.com",
		Password: "correct horse battery staple",
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
}
