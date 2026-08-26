package postgres

import (
	"errors"
	"testing"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

func TestMapUserDatabaseErrorMapsRecordNotFound(t *testing.T) {
	if !errors.Is(mapUserDatabaseError(gorm.ErrRecordNotFound), domain.ErrUserNotFound) {
		t.Fatal("expected record-not-found error to map to domain.ErrUserNotFound")
	}
}

func TestMapUserDatabaseErrorMapsDuplicateEmail(t *testing.T) {
	err := &pgconn.PgError{Code: "23505", ConstraintName: "users_email_key"}
	if !errors.Is(mapUserDatabaseError(err), domain.ErrEmailAlreadyExists) {
		t.Fatal("expected unique violation to map to domain.ErrEmailAlreadyExists")
	}
}

func TestMapUserDatabaseErrorPreservesUnknownErrors(t *testing.T) {
	want := errors.New("database unavailable")
	if !errors.Is(mapUserDatabaseError(want), want) {
		t.Fatal("expected unknown database error to be preserved")
	}
}
