package services

import (
	"context"
	"errors"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
)

const (
	DefaultUserPage     = 1
	DefaultUserPageSize = 20
	MaxUserPageSize     = 100
)

type Actor struct {
	UserID  int32
	IsAdmin bool
}

type CreateUserInput struct {
	Name   string
	Email  string
	Phone  *string
	LineID *string
}

// UpdateUserInput uses Set flags for nullable fields so the service can
// distinguish an omitted field from an explicit null that clears it.
type UpdateUserInput struct {
	Name  *string
	Email *string

	PhoneSet  bool
	Phone     *string
	LineIDSet bool
	LineID    *string
}

type UserService struct {
	repo ports.UserRepository
	now  func() time.Time
}

func NewUserService(repo ports.UserRepository) *UserService {
	return &UserService{repo: repo, now: time.Now}
}

func (s *UserService) Create(ctx context.Context, input CreateUserInput) (*domain.User, error) {
	name, err := normalizeName(input.Name)
	if err != nil {
		return nil, err
	}
	email, err := normalizeEmail(input.Email)
	if err != nil {
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

	if err := s.ensureEmailAvailable(ctx, email, 0); err != nil {
		return nil, err
	}

	user := &domain.User{
		Name:      name,
		Email:     email,
		Phone:     phone,
		LineID:    lineID,
		CreatedAt: s.now().UTC(),
	}
	if err := s.repo.Create(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *UserService) Get(ctx context.Context, actor Actor, userID int32) (*domain.User, error) {
	if err := authorize(actor, userID); err != nil {
		return nil, err
	}
	return s.repo.FindByID(ctx, userID)
}

func (s *UserService) List(ctx context.Context, actor Actor, query ports.UserListQuery) (ports.UserPage, error) {
	if !actor.IsAdmin {
		return ports.UserPage{}, domain.ErrUserForbidden
	}

	if query.Page == 0 {
		query.Page = DefaultUserPage
	}
	if query.PageSize == 0 {
		query.PageSize = DefaultUserPageSize
	}
	if query.Page < 1 || query.PageSize < 1 || query.PageSize > MaxUserPageSize {
		return ports.UserPage{}, domain.ErrInvalidUser
	}

	return s.repo.List(ctx, query)
}

func (s *UserService) Update(ctx context.Context, actor Actor, userID int32, input UpdateUserInput) (*domain.User, error) {
	if err := authorize(actor, userID); err != nil {
		return nil, err
	}
	if input.Name == nil && input.Email == nil && !input.PhoneSet && !input.LineIDSet {
		return nil, domain.ErrNoUserChanges
	}

	patch := ports.UserPatch{
		PhoneSet:  input.PhoneSet,
		LineIDSet: input.LineIDSet,
		Phone:     input.Phone,
		LineID:    input.LineID,
	}
	if input.Name != nil {
		name, err := normalizeName(*input.Name)
		if err != nil {
			return nil, err
		}
		patch.Name = &name
	}
	if input.Email != nil {
		email, err := normalizeEmail(*input.Email)
		if err != nil {
			return nil, err
		}
		if err := s.ensureEmailAvailable(ctx, email, userID); err != nil {
			return nil, err
		}
		patch.Email = &email
	}
	if input.PhoneSet {
		phone, err := normalizeContact(input.Phone, 20)
		if err != nil {
			return nil, err
		}
		patch.Phone = phone
	}
	if input.LineIDSet {
		lineID, err := normalizeContact(input.LineID, 50)
		if err != nil {
			return nil, err
		}
		patch.LineID = lineID
	}

	return s.repo.Update(ctx, userID, patch)
}

func (s *UserService) Delete(ctx context.Context, actor Actor, userID int32) (time.Time, error) {
	if err := authorize(actor, userID); err != nil {
		return time.Time{}, err
	}

	deletedAt := s.now().UTC()
	if err := s.repo.SoftDelete(ctx, userID, deletedAt); err != nil {
		return time.Time{}, err
	}
	return deletedAt, nil
}

func (s *UserService) ensureEmailAvailable(ctx context.Context, email string, currentUserID int32) error {
	existing, err := s.repo.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			return nil
		}
		return err
	}
	if existing != nil && existing.UserID != currentUserID {
		return domain.ErrEmailAlreadyExists
	}
	return nil
}

func authorize(actor Actor, userID int32) error {
	if actor.IsAdmin || actor.UserID == userID {
		return nil
	}
	return domain.ErrUserForbidden
}

func normalizeName(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || utf8.RuneCountInString(value) > 100 {
		return "", domain.ErrInvalidUser
	}
	return value, nil
}

func normalizeEmail(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" || len(value) > 255 {
		return "", domain.ErrInvalidUser
	}
	parsed, err := mail.ParseAddress(value)
	if err != nil || parsed.Address != value {
		return "", domain.ErrInvalidUser
	}
	return value, nil
}

func normalizeContact(value *string, maxRunes int) (*string, error) {
	if value == nil {
		return nil, nil
	}
	normalized := strings.TrimSpace(*value)
	if normalized == "" || utf8.RuneCountInString(normalized) > maxRunes {
		return nil, domain.ErrInvalidUser
	}
	return &normalized, nil
}
