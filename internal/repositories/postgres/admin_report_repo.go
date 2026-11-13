package postgres

import (
	"context"

	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
	"gorm.io/gorm"
)

type AdminReportRepository struct {
	db *gorm.DB
}

func NewAdminReportRepository(db *gorm.DB) *AdminReportRepository {
	return &AdminReportRepository{db: db}
}

func (r *AdminReportRepository) ListJobs(ctx context.Context, query ports.AdminReportQuery) (ports.AdminReportPage[ports.AdminJob], error) {
	base := r.db.WithContext(ctx).Table("jobs AS j")
	if query.Status != "" {
		base = base.Where("j.job_status = ?", query.Status)
	}
	return readReport[ports.AdminJob](base, query, "j.job_id DESC", func(rows *gorm.DB) *gorm.DB {
		return rows.Select(`j.job_id, j.job_status, creator.name AS creator_name,
			COALESCE((SELECT json_agg(s.title ORDER BY s.survey_id) FROM is_used_in AS l
				JOIN surveys AS s ON s.survey_id = l.survey_id WHERE l.job_id = j.job_id), '[]') AS survey_titles,
			p.amount::text AS amount, p.payment_status, p.paid_at`).
			Joins("JOIN users AS creator ON creator.user_id = j.user_id").
			Joins(`LEFT JOIN LATERAL (
				SELECT amount, payment_status, paid_at FROM payments WHERE job_id = j.job_id
				ORDER BY (payment_status = 'Paid') DESC, payment_id DESC LIMIT 1
			) AS p ON TRUE`)
	})
}

func (r *AdminReportRepository) ListFlaggedMarketers(ctx context.Context, query ports.AdminReportQuery) (ports.AdminReportPage[ports.FlaggedMarketer], error) {
	db := r.db.WithContext(ctx)
	flagged := db.Table("jobs AS j").
		Select(`o.user_id AS marketer_id, COUNT(*) AS flagged_jobs,
			SUM(o.offered_price)::text AS total_value,
			SUM((SELECT COUNT(*) FROM attachments AS a WHERE a.job_id = j.job_id)) AS attachment_count`).
		Joins("JOIN offers AS o ON o.job_id = j.job_id AND o.offer_id = j.accepted_offer_id").
		Where("o.user_id <> j.user_id AND o.offer_status = 'Accepted'").
		Where(`j.job_status = 'Cancelled' OR (j.job_status = 'Completed' AND EXISTS
			(SELECT 1 FROM reviews AS r WHERE r.job_id = j.job_id AND r.rating BETWEEN 1 AND 2))`).
		Group("o.user_id").Having("COUNT(*) >= 2")
	base := db.Table("(?) AS flagged", flagged)
	return readReport[ports.FlaggedMarketer](base, query,
		"flagged.flagged_jobs DESC, flagged.total_value::numeric DESC, flagged.marketer_id ASC",
		func(rows *gorm.DB) *gorm.DB {
			return rows.Select("flagged.*, u.name").Joins("JOIN users AS u ON u.user_id = flagged.marketer_id")
		})
}

func readReport[T any](base *gorm.DB, query ports.AdminReportQuery, order string, rows func(*gorm.DB) *gorm.DB) (ports.AdminReportPage[T], error) {
	page := ports.AdminReportPage[T]{Page: query.Page, PageSize: query.PageSize, Items: []T{}}
	if err := base.Session(&gorm.Session{}).Count(&page.Total).Error; err != nil {
		return page, err
	}
	err := rows(base).Order(order).Limit(query.PageSize).Offset((query.Page - 1) * query.PageSize).Scan(&page.Items).Error
	return page, err
}
