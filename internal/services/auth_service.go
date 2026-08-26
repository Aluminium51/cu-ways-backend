package services

import (
	"context"
	"errors"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
)

const (
	MinPasswordLength = 8
	MaxPasswordLength = 128
	AccessTokenTTL    = time.Hour
)

type RegisterInput struct {
	Name     string
	Email    string
	Password string
	Phone    *string
	LineID   *string
}

type LoginInput struct {
	Email    string
	Password string
}

type AuthResult struct {
	User      *domain.User
	Token     string
	ExpiresAt time.Time
}

type AuthService struct {
	repo   ports.UserRepository
	hasher ports.PasswordHasher
	issuer ports.TokenIssuer
	now    func() time.Time
}

func NewAuthService(repo ports.UserRepository, hasher ports.PasswordHasher, issuer ports.TokenIssuer) *AuthService {
	return &AuthService{repo: repo, hasher: hasher, issuer: issuer, now: time.Now}
}

func (s *AuthService) Register(ctx context.Context, input RegisterInput) (*AuthResult, error) {
	name, err := normalizeName(input.Name)
	if err != nil {
		return nil, err
	}
	email, err := normalizeEmail(input.Email)
	if err != nil {
		return nil, err
	}
	if err := validatePassword(input.Password); err != nil {
		return nil, err
	}
	phone, err := normalizeContact(input.Phone, 20)
	if err != nil {
		return nil, err
	}
	lineID, err := normalizeContact(input.LineID, 50)
	if err != nil {
		return nil, err
	}

	if err := s.ensureEmailAvailable(ctx, email); err != nil {
		return nil, err
	}
	passwordHash, err := s.hasher.Hash(ctx, input.Password)
	if err != nil {
		return nil, err
	}

	user := &domain.User{
		Name:         name,
		Email:        email,
		Phone:        phone,
		LineID:       lineID,
		PasswordHash: &passwordHash,
		Role:         domain.RoleUser,
		CreatedAt:    s.now().UTC(),
	}
	if err := s.repo.Create(ctx, user); err != nil {
		return nil, err
	}

	return s.issueResult(ctx, user)
}

func (s *AuthService) Login(ctx context.Context, input LoginInput) (*AuthResult, error) {
	email, err := normalizeEmail(input.Email)
	if err != nil || validatePassword(input.Password) != nil {
		return nil, domain.ErrInvalidCredentials
	}

	user, err := s.repo.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			return nil, domain.ErrInvalidCredentials
		}
		return nil, err
	}
	if user == nil || user.DeletedAt != nil || user.PasswordHash == nil {
		return nil, domain.ErrInvalidCredentials
	}

	if err := s.hasher.Compare(ctx, *user.PasswordHash, input.Password); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		return nil, domain.ErrInvalidCredentials
	}

	return s.issueResult(ctx, user)
}

func (s *AuthService) ensureEmailAvailable(ctx context.Context, email string) error {
	existing, err := s.repo.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			return nil
		}
		return err
	}
	if existing != nil {
		return domain.ErrEmailAlreadyExists
	}
	return nil
}

func (s *AuthService) issueResult(ctx context.Context, user *domain.User) (*AuthResult, error) {
	role := domain.RoleUser
	if user.Role == domain.RoleAdmin {
		role = domain.RoleAdmin
	}
	token, expiresAt, err := s.issuer.Issue(ctx, formatUserID(user.UserID), role, AccessTokenTTL)
	if err != nil {
		return nil, err
	}
	return &AuthResult{User: user, Token: token, ExpiresAt: expiresAt}, nil
}

func validatePassword(password string) error {
	length := utf8.RuneCountInString(password)
	if length < MinPasswordLength || length > MaxPasswordLength {
		return domain.ErrInvalidUser
	}
	return nil
}

func formatUserID(userID int32) string {
	return strconv.FormatInt(int64(userID), 10)
}
