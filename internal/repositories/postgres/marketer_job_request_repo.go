package postgres

import (
	"context"
	"errors"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type MarketerJobRequestRepository struct {
	db *gorm.DB
}

var _ ports.MarketerJobRequestRepository = (*MarketerJobRequestRepository)(nil)

func NewMarketerJobRequestRepository(db *gorm.DB) *MarketerJobRequestRepository {
	return &MarketerJobRequestRepository{db: db}
}

func (r *MarketerJobRequestRepository) ListForMarketer(ctx context.Context, marketerID int32, status *domain.JobRequestStatus) ([]ports.MarketerJobRequest, error) {
	db := r.db.WithContext(ctx)
	query := db.Where("marketer_id = ?", marketerID)
	if status != nil {
		query = query.Where("request_status = ?", *status)
	}
	return loadMarketerJobRequests(db, query.Order("created_at DESC, job_request_id DESC"))
}

func (r *MarketerJobRequestRepository) FindForMarketer(ctx context.Context, marketerID, requestID int32) (*ports.MarketerJobRequest, error) {
	db := r.db.WithContext(ctx)
	items, err := loadMarketerJobRequests(db, db.Where("job_request_id = ? AND marketer_id = ?", requestID, marketerID))
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, domain.ErrJobRequestNotFound
	}
	return &items[0], nil
}

func (r *MarketerJobRequestRepository) CreateOffer(ctx context.Context, marketerID, requestID int32, offer *domain.Offer) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		request, err := lockMarketerJobRequest(tx, marketerID, requestID)
		if err != nil {
			return err
		}
		if request.Status != domain.JobRequestStatusAccepted {
			return domain.ErrJobRequestNotAccepted
		}
		var active int64
		if err := tx.Model(&domain.Offer{}).
			Where("job_request_id = ? AND offer_status IN ?", requestID,
				[]domain.OfferStatus{domain.OfferStatusPending, domain.OfferStatusAccepted}).
			Count(&active).Error; err != nil {
			return err
		}
		if active > 0 {
			return domain.ErrOfferAlreadyExists
		}
		offer.JobID = request.JobID
		offer.UserID = marketerID
		offer.JobRequestID = &request.JobRequestID
		return tx.Create(offer).Error
	})
	return mapOfferRepositoryError(err)
}

func (r *MarketerJobRequestRepository) ListOffers(ctx context.Context, marketerID, requestID int32) ([]domain.Offer, error) {
	db := r.db.WithContext(ctx)
	var count int64
	if err := db.Model(&domain.JobRequest{}).
		Where("job_request_id = ? AND marketer_id = ?", requestID, marketerID).
		Count(&count).Error; err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, domain.ErrJobRequestNotFound
	}
	var offers []domain.Offer
	err := db.Where("job_request_id = ? AND user_id = ?", requestID, marketerID).
		Order("created_at DESC, offer_id DESC").
		Find(&offers).Error
	return offers, err
}

func lockMarketerJobRequest(tx *gorm.DB, marketerID, requestID int32) (*domain.JobRequest, error) {
	var request domain.JobRequest
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("job_request_id = ? AND marketer_id = ?", requestID, marketerID).
		First(&request).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrJobRequestNotFound
		}
		return nil, err
	}
	return &request, nil
}

func mapOfferRepositoryError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "idx_offers_active_job_request" {
		return domain.ErrOfferAlreadyExists
	}
	return err
}

// loadMarketerJobRequests runs query against job_requests and attaches each
// request's job status, creator name, linked surveys, and newest Brief.
func loadMarketerJobRequests(db, query *gorm.DB) ([]ports.MarketerJobRequest, error) {
	var requests []domain.JobRequest
	if err := query.
		Preload("Job").
		Preload("Job.Surveys", func(tx *gorm.DB) *gorm.DB {
			return tx.Order("surveys.survey_id ASC")
		}).
		Find(&requests).Error; err != nil {
		return nil, err
	}
	if len(requests) == 0 {
		return []ports.MarketerJobRequest{}, nil
	}

	jobIDs := make([]int32, 0, len(requests))
	creatorIDs := make([]int32, 0, len(requests))
	for _, request := range requests {
		if request.Job != nil {
			jobIDs = append(jobIDs, request.JobID)
			creatorIDs = append(creatorIDs, request.Job.UserID)
		}
	}
	creatorNames, err := userNames(db, creatorIDs)
	if err != nil {
		return nil, err
	}
	briefs, err := currentBriefs(db, jobIDs)
	if err != nil {
		return nil, err
	}

	items := make([]ports.MarketerJobRequest, 0, len(requests))
	for _, request := range requests {
		item := ports.MarketerJobRequest{Request: request, Surveys: []domain.Survey{}}
		if job := request.Job; job != nil {
			item.JobStatus = job.JobStatus
			item.CreatorID = job.UserID
			item.CreatorName = creatorNames[job.UserID]
			if job.Surveys != nil {
				item.Surveys = job.Surveys
			}
			item.CurrentBrief = briefs[request.JobID]
		}
		item.Request.Job = nil
		items = append(items, item)
	}
	return items, nil
}

func (r *MarketerJobRequestRepository) RespondToPending(ctx context.Context, marketerID, requestID int32, response ports.JobRequestResponse) (*domain.JobRequest, error) {
	var request domain.JobRequest
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		locked, err := lockMarketerJobRequest(tx, marketerID, requestID)
		if err != nil {
			return err
		}
		request = *locked
		if request.Status != domain.JobRequestStatusPending {
			return domain.ErrJobRequestNotPending
		}
		respondedAt := response.RespondedAt
		request.Status = response.Status
		request.RespondedAt = &respondedAt
		request.DeclineReason = response.DeclineReason
		request.DeclineNote = response.DeclineNote
		return tx.Model(&domain.JobRequest{}).
			Where("job_request_id = ?", requestID).
			Updates(map[string]any{
				"request_status": request.Status,
				"responded_at":   request.RespondedAt,
				"decline_reason": request.DeclineReason,
				"decline_note":   request.DeclineNote,
			}).Error
	})
	if err != nil {
		return nil, err
	}
	return &request, nil
}

func userNames(db *gorm.DB, userIDs []int32) (map[int32]string, error) {
	names := make(map[int32]string, len(userIDs))
	if len(userIDs) == 0 {
		return names, nil
	}
	var users []domain.User
	if err := db.Select("user_id", "name").Where("user_id IN ?", userIDs).Find(&users).Error; err != nil {
		return nil, err
	}
	for _, user := range users {
		names[user.UserID] = user.Name
	}
	return names, nil
}

// currentBriefs returns the newest Brief attachment per job.
func currentBriefs(db *gorm.DB, jobIDs []int32) (map[int32]*domain.Attachment, error) {
	briefs := make(map[int32]*domain.Attachment, len(jobIDs))
	if len(jobIDs) == 0 {
		return briefs, nil
	}
	var attachments []domain.Attachment
	if err := db.Where("job_id IN ? AND attachment_type = ?", jobIDs, domain.AttachmentTypeBrief).
		Order("upload_at DESC, attachment_id DESC").
		Find(&attachments).Error; err != nil {
		return nil, err
	}
	for index := range attachments {
		attachment := attachments[index]
		if _, exists := briefs[attachment.JobID]; !exists {
			briefs[attachment.JobID] = &attachment
		}
	}
	return briefs, nil
}
