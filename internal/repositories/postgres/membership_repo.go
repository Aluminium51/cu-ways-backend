package postgres

import (
	"context"

	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
	"gorm.io/gorm"
)

type MembershipRepository struct {
	db *gorm.DB
}

var _ ports.MembershipRepository = (*MembershipRepository)(nil)

func NewMembershipRepository(db *gorm.DB) *MembershipRepository {
	return &MembershipRepository{db: db}
}

func (r *MembershipRepository) IsCreator(ctx context.Context, userID int32) (bool, error) {
	return r.exists(ctx, "creators", userID)
}

func (r *MembershipRepository) IsMarketer(ctx context.Context, userID int32) (bool, error) {
	return r.exists(ctx, "marketers", userID)
}

func (r *MembershipRepository) exists(ctx context.Context, table string, userID int32) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Table(table).
		Where("user_id = ? AND EXISTS (SELECT 1 FROM users WHERE users.user_id = ? AND users.deleted_at IS NULL)", userID, userID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}
