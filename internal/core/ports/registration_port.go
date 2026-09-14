package ports

import (
	"context"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
)

// RegistrationRepository describes the persistence capabilities required by
// account registration. The adapter must create the user and Creator
// membership atomically.
type RegistrationRepository interface {
	FindByEmail(ctx context.Context, email string) (*domain.User, error)
	CreateWithCreator(ctx context.Context, user *domain.User) error
}
