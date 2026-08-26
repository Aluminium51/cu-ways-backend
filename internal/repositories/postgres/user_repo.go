package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

// UserRepository is the PostgreSQL adapter for user persistence.
type UserRepository struct {
	db *gorm.DB
}

var _ ports.UserRepository = (*UserRepository)(nil)

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, user *domain.User) error {
	if err := r.db.WithContext(ctx).Create(user).Error; err != nil {
		return mapUserDatabaseError(err)
	}
	return nil
}

func (r *UserRepository) FindByID(ctx context.Context, userID int32) (*domain.User, error) {
	var user domain.User
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND deleted_at IS NULL", userID).
		First(&user).Error
	if err != nil {
		return nil, mapUserDatabaseError(err)
	}
	return &user, nil
}

// FindByEmail intentionally includes soft-deleted users so a deleted user's
// unique email cannot be reused without an explicit account-recovery policy.
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	var user domain.User
	err := r.db.WithContext(ctx).Where("email = ?", email).First(&user).Error
	if err != nil {
		return nil, mapUserDatabaseError(err)
	}
	return &user, nil
}

func (r *UserRepository) List(ctx context.Context, query ports.UserListQuery) (ports.UserPage, error) {
	dbQuery := r.db.WithContext(ctx).
		Model(&domain.User{}).
		Where("deleted_at IS NULL")

	var total int64
	if err := dbQuery.Count(&total).Error; err != nil {
		return ports.UserPage{}, mapUserDatabaseError(err)
	}

	users := make([]domain.User, 0)
	offset := (query.Page - 1) * query.PageSize
	if err := dbQuery.
		Order("user_id ASC").
		Offset(offset).
		Limit(query.PageSize).
		Find(&users).Error; err != nil {
		return ports.UserPage{}, mapUserDatabaseError(err)
	}

	return ports.UserPage{
		Items:    users,
		Page:     query.Page,
		PageSize: query.PageSize,
		Total:    total,
	}, nil
}

func (r *UserRepository) Update(ctx context.Context, userID int32, patch ports.UserPatch) (*domain.User, error) {
	updates := make(map[string]any)
	if patch.Name != nil {
		updates["name"] = *patch.Name
	}
	if patch.Email != nil {
		updates["email"] = *patch.Email
	}
	if patch.PhoneSet {
		updates["phone"] = patch.Phone
	}
	if patch.LineIDSet {
		updates["line_id"] = patch.LineID
	}
	if len(updates) == 0 {
		return nil, domain.ErrNoUserChanges
	}

	result := r.db.WithContext(ctx).
		Model(&domain.User{}).
		Where("user_id = ? AND deleted_at IS NULL", userID).
		Updates(updates)
	if err := result.Error; err != nil {
		return nil, mapUserDatabaseError(err)
	}
	if result.RowsAffected == 0 {
		return nil, domain.ErrUserNotFound
	}

	return r.FindByID(ctx, userID)
}

func (r *UserRepository) SetRole(ctx context.Context, userID int32, role string) error {
	if role != domain.RoleUser && role != domain.RoleAdmin {
		return domain.ErrInvalidUser
	}

	result := r.db.WithContext(ctx).
		Model(&domain.User{}).
		Where("user_id = ? AND deleted_at IS NULL", userID).
		Update("role", role)
	if err := result.Error; err != nil {
		return mapUserDatabaseError(err)
	}
	if result.RowsAffected == 0 {
		return domain.ErrUserNotFound
	}
	return nil
}

func (r *UserRepository) SoftDelete(ctx context.Context, userID int32, deletedAt time.Time) error {
	result := r.db.WithContext(ctx).
		Model(&domain.User{}).
		Where("user_id = ? AND deleted_at IS NULL", userID).
		Update("deleted_at", deletedAt.UTC())
	if err := result.Error; err != nil {
		return mapUserDatabaseError(err)
	}
	if result.RowsAffected == 0 {
		return domain.ErrUserNotFound
	}
	return nil
}

func mapUserDatabaseError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.ErrUserNotFound
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return domain.ErrEmailAlreadyExists
	}
	return err
}
