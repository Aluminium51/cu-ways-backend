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
	if !repo.creatorIDs[result.User.UserID] {
		t.Fatalf("expected Creator membership for user %d", result.User.UserID)
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
		Phone:    stringPointer("0812345678"),
	})
	if !errors.Is(err, domain.ErrEmailAlreadyExists) {
		t.Fatalf("expected duplicate email error, got %v", err)
	}
	if len(repo.creatorIDs) != 0 {
		t.Fatal("duplicate registration must not create a Creator membership")
	}
}

func TestAuthServiceRegisterPropagatesDuplicatePhoneError(t *testing.T) {
	repo := newFakeUserRepository()
	repo.createWithCreatorErr = domain.ErrPhoneAlreadyExists
	service := NewAuthService(repo, &fakePasswordHasher{}, &fakeTokenIssuer{})

	phone := "0812345678"
	_, err := service.Register(context.Background(), RegisterInput{
		Name:     "Jane",
		Email:    "jane@example.com",
		Password: "correct horse battery staple",
		Phone:    &phone,
	})
	if !errors.Is(err, domain.ErrPhoneAlreadyExists) {
		t.Fatalf("expected duplicate phone error, got %v", err)
	}
}

func TestAuthServiceRegisterPropagatesRegistrationRepositoryError(t *testing.T) {
	repo := newFakeUserRepository()
	repo.createWithCreatorErr = errors.New("registration persistence failed")
	service := NewAuthService(repo, &fakePasswordHasher{}, &fakeTokenIssuer{})

	_, err := service.Register(context.Background(), RegisterInput{
		Name:     "Jane",
		Email:    "jane@example.com",
		Password: "correct horse battery staple",
		Phone:    stringPointer("0812345678"),
	})
	if !errors.Is(err, repo.createWithCreatorErr) {
		t.Fatalf("expected registration repository error, got %v", err)
	}
	if len(repo.users) != 0 || len(repo.creatorIDs) != 0 {
		t.Fatal("failed registration must not persist user or Creator membership")
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
		Phone:    stringPointer("0812345678"),
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
}

func TestAuthServiceRegisterRequiresPhone(t *testing.T) {
	repo := newFakeUserRepository()
	service := NewAuthService(repo, &fakePasswordHasher{}, &fakeTokenIssuer{})

	for name, phone := range map[string]*string{"missing": nil, "blank": stringPointer("   ")} {
		t.Run(name, func(t *testing.T) {
			_, err := service.Register(context.Background(), RegisterInput{
				Name:     "Jane",
				Email:    "jane@example.com",
				Password: "correct horse battery staple",
				Phone:    phone,
			})
			if !errors.Is(err, domain.ErrInvalidUser) {
				t.Fatalf("expected invalid user for %s phone, got %v", name, err)
			}
		})
	}
	if len(repo.users) != 0 {
		t.Fatal("registration without a phone must not persist a user")
	}
}

func TestAuthServiceRegisterCreatorHasNoMarketerProfile(t *testing.T) {
	repo := newFakeUserRepository()
	service := NewAuthService(repo, &fakePasswordHasher{}, &fakeTokenIssuer{})

	result, err := service.Register(context.Background(), RegisterInput{
		Name:     "Jane",
		Email:    "jane@example.com",
		Password: "correct horse battery staple",
		Phone:    stringPointer("0812345678"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := repo.marketers[result.User.UserID]; ok {
		t.Fatal("a creator sign-up must not create a marketer profile")
	}
}

func TestAuthServiceRegisterSavesMarketerProfileWithCreatorMembership(t *testing.T) {
	repo := newFakeUserRepository()
	service := NewAuthService(repo, &fakePasswordHasher{}, &fakeTokenIssuer{})
	years := int32(3)

	result, err := service.Register(context.Background(), RegisterInput{
		Name:     "Jane",
		Email:    "jane@example.com",
		Password: "correct horse battery staple",
		Phone:    stringPointer("0812345678"),
		MarketerProfile: &RegisterMarketerProfileInput{
			Bio:              "  Survey recruiter  ",
			ExperienceYears:  &years,
			AvailabilityText: " Weekends ",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	userID := result.User.UserID
	if !repo.creatorIDs[userID] {
		t.Fatal("a marketer sign-up keeps the Creator membership")
	}
	profile := repo.marketers[userID]
	if profile == nil {
		t.Fatal("expected a marketer profile")
	}
	if profile.Bio != "Survey recruiter" || profile.ExperienceYears != 3 || profile.AvailabilityText != "Weekends" {
		t.Fatalf("expected trimmed profile fields, got %+v", profile)
	}
	if profile.AvailabilityStatus != domain.AvailabilityAvailable {
		t.Fatalf("expected a new marketer to start available, got %q", profile.AvailabilityStatus)
	}
}

func TestAuthServiceRegisterDefaultsMissingExperienceToZero(t *testing.T) {
	repo := newFakeUserRepository()
	service := NewAuthService(repo, &fakePasswordHasher{}, &fakeTokenIssuer{})

	result, err := service.Register(context.Background(), RegisterInput{
		Name:            "Jane",
		Email:           "jane@example.com",
		Password:        "correct horse battery staple",
		Phone:           stringPointer("0812345678"),
		MarketerProfile: &RegisterMarketerProfileInput{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if profile := repo.marketers[result.User.UserID]; profile == nil || profile.ExperienceYears != 0 {
		t.Fatalf("expected a marketer profile with 0 years, got %+v", profile)
	}
}

func TestAuthServiceRegisterRejectsInvalidMarketerProfile(t *testing.T) {
	repo := newFakeUserRepository()
	hasher := &fakePasswordHasher{}
	service := NewAuthService(repo, hasher, &fakeTokenIssuer{})
	years := int32(81)

	_, err := service.Register(context.Background(), RegisterInput{
		Name:            "Jane",
		Email:           "jane@example.com",
		Password:        "correct horse battery staple",
		Phone:           stringPointer("0812345678"),
		MarketerProfile: &RegisterMarketerProfileInput{ExperienceYears: &years},
	})
	if !errors.Is(err, domain.ErrInvalidMarketerProfile) {
		t.Fatalf("expected invalid marketer profile, got %v", err)
	}
	if len(repo.users) != 0 || hasher.lastInput != "" {
		t.Fatal("an invalid marketer profile must be rejected before hashing or persisting")
	}
}

func TestAuthServiceRegisterRequiresTenDigitPhone(t *testing.T) {
	repo := newFakeUserRepository()
	service := NewAuthService(repo, &fakePasswordHasher{}, &fakeTokenIssuer{})

	for _, phone := range []string{"081234567", "08123456789"} {
		_, err := service.Register(context.Background(), RegisterInput{
			Name:     "Jane",
			Email:    "jane@example.com",
			Password: "correct horse battery staple",
			Phone:    stringPointer(phone),
		})
		if !errors.Is(err, domain.ErrInvalidUser) {
			t.Fatalf("expected %q to be rejected, got %v", phone, err)
		}
	}
	if len(repo.users) != 0 {
		t.Fatal("an invalid phone must not create an account")
	}
}
