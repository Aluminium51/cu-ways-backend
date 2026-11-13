package postgres

import (
	"context"
	"strings"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
	"gorm.io/gorm"
)

type TransactionRepository struct {
	db *gorm.DB
}

var _ ports.TransactionRepository = (*TransactionRepository)(nil)

func NewTransactionRepository(db *gorm.DB) *TransactionRepository {
	return &TransactionRepository{db: db}
}

// transactionBase selects Completed Jobs whose accepted offer has a Paid
// payment. A Job can hold failed or refunded payments as history, so only the
// latest Paid one is joined, looked up per Job after the user filter narrows the
// rows. The user filter is applied here, from the authenticated user id, so
// another user's Jobs can never be returned.
func (r *TransactionRepository) transactionBase(ctx context.Context, query ports.TransactionQuery) *gorm.DB {
	base := r.db.WithContext(ctx).Table("jobs AS j").
		Joins("JOIN offers AS o ON o.job_id = j.job_id AND o.offer_id = j.accepted_offer_id").
		Joins(`JOIN LATERAL (
SELECT p.method, p.paid_at FROM payments AS p
WHERE p.job_id = j.job_id AND p.payment_status = ?
ORDER BY p.paid_at DESC NULLS LAST, p.payment_id DESC
LIMIT 1
) AS pay ON TRUE`, domain.PaymentStatusPaid).
		Joins("JOIN users AS cu ON cu.user_id = j.user_id").
		Joins("JOIN users AS mu ON mu.user_id = o.user_id").
		Where("j.job_status = ?", domain.JobStatusCompleted)

	switch query.Role {
	case domain.TransactionRoleCreator:
		base = base.Where("j.user_id = ?", query.UserID)
	case domain.TransactionRoleMarketer:
		base = base.Where("o.user_id = ?", query.UserID)
	default:
		base = base.Where("(j.user_id = ? OR o.user_id = ?)", query.UserID, query.UserID)
	}

	if query.Keyword != "" {
		// Job numbers may be typed as "#12" and match exactly, so "12" does not also return job 112.
		jobNumber := strings.TrimPrefix(query.Keyword, "#")
		pattern := "%" + escapeLikePattern(strings.ToLower(query.Keyword)) + "%"
		base = base.Where(`(
LOWER(j.title) LIKE ? ESCAPE '\'
OR CAST(j.job_id AS text) = ?
OR LOWER(cu.name) LIKE ? ESCAPE '\'
OR LOWER(mu.name) LIKE ? ESCAPE '\'
OR LOWER(pay.method) LIKE ? ESCAPE '\'
)`, pattern, jobNumber, pattern, pattern, pattern)
	}
	return base
}

func (r *TransactionRepository) List(ctx context.Context, query ports.TransactionQuery) (ports.TransactionPage, error) {
	page := ports.TransactionPage{Page: query.Page, PageSize: query.PageSize, Items: []domain.Transaction{}}

	if err := r.transactionBase(ctx, query).Count(&page.Total).Error; err != nil {
		return ports.TransactionPage{}, err
	}
	if page.Total == 0 {
		return page, nil
	}

	includeReview := query.Role != domain.TransactionRoleMarketer
	err := r.transactionBase(ctx, query).
		Select(`j.job_id AS job_id,
COALESCE(j.title, '') AS job_title,
j.user_id AS creator_id,
cu.name AS creator_name,
o.user_id AS marketer_id,
mu.name AS marketer_name,
o.offered_price AS accepted_price,
pay.method AS payment_method,
pay.paid_at AS paid_at,
j.completed_at,
CASE WHEN ? AND j.user_id = ?
THEN EXISTS (SELECT 1 FROM reviews AS r WHERE r.job_id = j.job_id) END AS has_review`, includeReview, query.UserID).
		Order("j.completed_at DESC NULLS LAST, j.job_id DESC").
		Limit(query.PageSize).
		Offset((query.Page - 1) * query.PageSize).
		Scan(&page.Items).Error
	if err != nil {
		return ports.TransactionPage{}, err
	}
	return page, nil
}
