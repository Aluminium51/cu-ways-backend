package ports

import (
	"context"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
)

// RegistrationRepository describes the persistence capabilities required by
// account registration. The adapter must create the user, the Creator
// membership and, when marketer is non-nil, the Marketer profile atomically.
type RegistrationRepository interface {
	FindByEmail(ctx context.Context, email string) (*domain.User, error)
	CreateWithCreator(ctx context.Context, user *domain.User, marketer *domain.Marketer) error
}
