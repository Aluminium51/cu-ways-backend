package services

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
)

var (
	ErrAdminSeedUserDeleted  = errors.New("admin seed account is soft-deleted")
	ErrAdminSeedPasswordless = errors.New("admin seed account has no password")
)

type SeedAdminInput struct {
	Name     string
	Email    string
	Password string
}

type SeedAdminResult struct {
	User    *domain.User
	Created bool
}

type AdminSeedService struct {
	repo   ports.UserRepository
	hasher ports.PasswordHasher
	now    func() time.Time
}

func NewAdminSeedService(repo ports.UserRepository, hasher ports.PasswordHasher) *AdminSeedService {
	return &AdminSeedService{repo: repo, hasher: hasher, now: time.Now}
}

func (s *AdminSeedService) Seed(ctx context.Context, input SeedAdminInput) (*SeedAdminResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

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

	existing, err := s.repo.FindByEmail(ctx, email)
	if err == nil {
		return s.promoteExisting(ctx, existing)
	}
	if !errors.Is(err, domain.ErrUserNotFound) {
		return nil, err
	}

	passwordHash, err := s.hasher.Hash(ctx, input.Password)
	if err != nil {
		return nil, err
	}
	user := &domain.User{
		Name:         name,
		Email:        email,
		PasswordHash: &passwordHash,
		Role:         domain.RoleAdmin,
		CreatedAt:    s.now().UTC(),
	}
	if err := s.repo.Create(ctx, user); err != nil {
		return nil, err
	}
	return &SeedAdminResult{User: user, Created: true}, nil
}

func (s *AdminSeedService) promoteExisting(ctx context.Context, user *domain.User) (*SeedAdminResult, error) {
	if user == nil {
		return nil, domain.ErrUserNotFound
	}
	if user.DeletedAt != nil {
		return nil, ErrAdminSeedUserDeleted
	}
	if user.PasswordHash == nil || strings.TrimSpace(*user.PasswordHash) == "" {
		return nil, ErrAdminSeedPasswordless
	}
	if user.Role == domain.RoleAdmin {
		return &SeedAdminResult{User: user, Created: false}, nil
	}
	if err := s.repo.SetRole(ctx, user.UserID, domain.RoleAdmin); err != nil {
		return nil, err
	}
	user.Role = domain.RoleAdmin
	return &SeedAdminResult{User: user, Created: false}, nil
}
