package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
)

type fakeUserRepository struct {
	users map[int32]*domain.User

	findByEmailErr error
	listErr        error
	updateErr      error
	softDeleteErr  error
	lastContext    context.Context
}

func newFakeUserRepository(users ...*domain.User) *fakeUserRepository {
	repo := &fakeUserRepository{users: make(map[int32]*domain.User)}
	for _, user := range users {
		copy := *user
		repo.users[user.UserID] = &copy
	}
	return repo
}

func (f *fakeUserRepository) Create(ctx context.Context, user *domain.User) error {
	f.lastContext = ctx
	if user.UserID == 0 {
		user.UserID = int32(len(f.users) + 1)
	}
	copy := *user
	f.users[user.UserID] = &copy
	return nil
}

func (f *fakeUserRepository) FindByID(ctx context.Context, userID int32) (*domain.User, error) {
	f.lastContext = ctx
	user, ok := f.users[userID]
	if !ok || user.DeletedAt != nil {
		return nil, domain.ErrUserNotFound
	}
	copy := *user
	return &copy, nil
}

func (f *fakeUserRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	f.lastContext = ctx
	if f.findByEmailErr != nil {
		return nil, f.findByEmailErr
	}
	for _, user := range f.users {
		if user.Email == email {
			copy := *user
			return &copy, nil
		}
	}
	return nil, domain.ErrUserNotFound
}

func (f *fakeUserRepository) List(ctx context.Context, query ports.UserListQuery) (ports.UserPage, error) {
	f.lastContext = ctx
	if f.listErr != nil {
		return ports.UserPage{}, f.listErr
	}
	items := make([]domain.User, 0, len(f.users))
	for _, user := range f.users {
		if user.DeletedAt == nil {
			items = append(items, *user)
		}
	}
	return ports.UserPage{Items: items, Page: query.Page, PageSize: query.PageSize, Total: int64(len(items))}, nil
}

func (f *fakeUserRepository) Update(ctx context.Context, userID int32, patch ports.UserPatch) (*domain.User, error) {
	f.lastContext = ctx
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	user, ok := f.users[userID]
	if !ok || user.DeletedAt != nil {
		return nil, domain.ErrUserNotFound
	}
	if patch.Name != nil {
		user.Name = *patch.Name
	}
	if patch.Email != nil {
		user.Email = *patch.Email
	}
	if patch.PhoneSet {
		user.Phone = patch.Phone
	}
	if patch.LineIDSet {
		user.LineID = patch.LineID
	}
	copy := *user
	return &copy, nil
}

func (f *fakeUserRepository) SoftDelete(ctx context.Context, userID int32, deletedAt time.Time) error {
	f.lastContext = ctx
	if f.softDeleteErr != nil {
		return f.softDeleteErr
	}
	user, ok := f.users[userID]
	if !ok || user.DeletedAt != nil {
		return domain.ErrUserNotFound
	}
	user.DeletedAt = &deletedAt
	return nil
}

func TestUserServiceCreateNormalizesAndCreates(t *testing.T) {
	repo := newFakeUserRepository()
	service := NewUserService(repo)
	phone := "0812345678"

	user, err := service.Create(context.Background(), CreateUserInput{
		Name:  " Jane Doe ",
		Email: " JANE@EXAMPLE.COM ",
		Phone: &phone,
	})
	if err != nil {
		t.Fatal(err)
	}
	if user.Name != "Jane Doe" || user.Email != "jane@example.com" {
		t.Fatalf("expected normalized user, got %+v", user)
	}
	if user.Phone == nil || *user.Phone != phone || user.CreatedAt.IsZero() {
		t.Fatalf("expected contact field and created timestamp, got %+v", user)
	}
}

func TestUserServiceCreateRejectsDuplicateEmail(t *testing.T) {
	repo := newFakeUserRepository(&domain.User{UserID: 1, Email: "jane@example.com"})
	service := NewUserService(repo)

	_, err := service.Create(context.Background(), CreateUserInput{Name: "Jane", Email: "JANE@example.com"})
	if !errors.Is(err, domain.ErrEmailAlreadyExists) {
		t.Fatalf("expected duplicate email error, got %v", err)
	}
}

func TestUserServiceAuthorization(t *testing.T) {
	repo := newFakeUserRepository(&domain.User{UserID: 7, Name: "Jane", Email: "jane@example.com"})
	service := NewUserService(repo)

	if _, err := service.Get(context.Background(), Actor{UserID: 7}, 7); err != nil {
		t.Fatalf("owner should access user: %v", err)
	}
	if _, err := service.Get(context.Background(), Actor{UserID: 8}, 7); !errors.Is(err, domain.ErrUserForbidden) {
		t.Fatalf("expected forbidden for another user, got %v", err)
	}
	if _, err := service.Get(context.Background(), Actor{UserID: 8, IsAdmin: true}, 7); err != nil {
		t.Fatalf("admin should access any user: %v", err)
	}
}

func TestUserServiceListRequiresAdminAndAppliesDefaults(t *testing.T) {
	deletedAt := time.Now()
	repo := newFakeUserRepository(
		&domain.User{UserID: 1, Email: "one@example.com"},
		&domain.User{UserID: 2, Email: "two@example.com"},
		&domain.User{UserID: 3, Email: "deleted@example.com", DeletedAt: &deletedAt},
	)
	service := NewUserService(repo)

	if _, err := service.List(context.Background(), Actor{UserID: 1}, ports.UserListQuery{}); !errors.Is(err, domain.ErrUserForbidden) {
		t.Fatalf("expected non-admin list to be forbidden, got %v", err)
	}
	page, err := service.List(context.Background(), Actor{IsAdmin: true}, ports.UserListQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if page.Page != DefaultUserPage || page.PageSize != DefaultUserPageSize || page.Total != 2 || len(page.Items) != 2 {
		t.Fatalf("unexpected user page: %+v", page)
	}
}

func TestUserServiceUpdateCanClearNullableFields(t *testing.T) {
	phone := "0812345678"
	repo := newFakeUserRepository(&domain.User{UserID: 1, Name: "Jane", Email: "jane@example.com", Phone: &phone})
	service := NewUserService(repo)
	name := "Jane Updated"

	user, err := service.Update(context.Background(), Actor{UserID: 1}, 1, UpdateUserInput{
		Name:     &name,
		PhoneSet: true,
		Phone:    nil,
	})
	if err != nil {
		t.Fatal(err)
	}
	if user.Name != name || user.Phone != nil {
		t.Fatalf("expected updated name and cleared phone, got %+v", user)
	}
}

func TestUserServiceUpdateRejectsDuplicateAndEmptyPatch(t *testing.T) {
	repo := newFakeUserRepository(
		&domain.User{UserID: 1, Email: "one@example.com"},
		&domain.User{UserID: 2, Email: "two@example.com"},
	)
	service := NewUserService(repo)
	email := "two@example.com"

	if _, err := service.Update(context.Background(), Actor{UserID: 1}, 1, UpdateUserInput{Email: &email}); !errors.Is(err, domain.ErrEmailAlreadyExists) {
		t.Fatalf("expected duplicate email error, got %v", err)
	}
	if _, err := service.Update(context.Background(), Actor{UserID: 1}, 1, UpdateUserInput{}); !errors.Is(err, domain.ErrNoUserChanges) {
		t.Fatalf("expected no changes error, got %v", err)
	}
}

func TestUserServiceDeleteSoftDeletesUser(t *testing.T) {
	repo := newFakeUserRepository(&domain.User{UserID: 1, Email: "jane@example.com"})
	service := NewUserService(repo)

	deletedAt, err := service.Delete(context.Background(), Actor{UserID: 1}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if deletedAt.IsZero() {
		t.Fatal("expected deletion timestamp")
	}
	if _, err := service.Get(context.Background(), Actor{UserID: 1}, 1); !errors.Is(err, domain.ErrUserNotFound) {
		t.Fatalf("expected deleted user to be hidden, got %v", err)
	}
}

func TestUserServicePropagatesContext(t *testing.T) {
	repo := newFakeUserRepository()
	service := NewUserService(repo)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	repo.findByEmailErr = context.Canceled

	_, err := service.Create(ctx, CreateUserInput{Name: "Jane", Email: "jane@example.com"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
	if repo.lastContext != ctx {
		t.Fatal("expected service to pass the request context to the repository")
	}
}
