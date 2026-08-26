package ports

import (
	"context"
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
)

// UserRepository describes the persistence capabilities required by the user service.
// It intentionally exposes no GORM or SQL implementation details.
type UserRepository interface {
	Create(ctx context.Context, user *domain.User) error
	FindByID(ctx context.Context, userID int32) (*domain.User, error)
	FindByEmail(ctx context.Context, email string) (*domain.User, error)
	List(ctx context.Context, query UserListQuery) (UserPage, error)
	Update(ctx context.Context, userID int32, patch UserPatch) (*domain.User, error)
	SoftDelete(ctx context.Context, userID int32, deletedAt time.Time) error
}

type UserListQuery struct {
	Page     int
	PageSize int
}

type UserPage struct {
	Items    []domain.User
	Page     int
	PageSize int
	Total    int64
}

// UserPatch uses the Set flags to distinguish an omitted nullable field from
// an explicit null value that should clear the field.
type UserPatch struct {
	Name  *string
	Email *string

	PhoneSet  bool
	Phone     *string
	LineIDSet bool
	LineID    *string
}
