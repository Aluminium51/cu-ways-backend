package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type JobRepository struct {
	db *gorm.DB
}

var _ ports.JobRepository = (*JobRepository)(nil)

func NewJobRepository(db *gorm.DB) *JobRepository {
	return &JobRepository{db: db}
}

func (r *JobRepository) ListJobsByCreator(ctx context.Context, creatorID int32) ([]domain.CreatorJobSummary, error) {
	var jobs []domain.Job
	err := r.db.WithContext(ctx).
		Select("jobs.*, (SELECT COUNT(*) FROM offers WHERE offers.job_id = jobs.job_id AND offers.offer_status = ?) AS pending_offer_count", domain.OfferStatusPending).
		Where("user_id = ?", creatorID).
		Preload("Surveys", func(db *gorm.DB) *gorm.DB {
			return db.Order("surveys.survey_id ASC")
		}).
		Order("created_at DESC, job_id DESC").
		Find(&jobs).Error
	if err != nil {
		return nil, err
	}

	summaries := make([]domain.CreatorJobSummary, 0, len(jobs))
	for _, job := range jobs {
		summaries = append(summaries, toCreatorJobSummary(job))
	}
	return summaries, nil
}

func (r *JobRepository) GetJobByCreator(ctx context.Context, creatorID, jobID int32) (*domain.CreatorJobSummary, error) {
	job, err := findOwnedJob(r.db.WithContext(ctx), creatorID, jobID)
	if err != nil {
		return nil, mapJobRepositoryError(err)
	}
	surveys := make([]domain.Survey, 0)
	err = r.db.WithContext(ctx).
		Table("surveys").
		Select("surveys.*").
		Joins("JOIN is_used_in ON is_used_in.survey_id = surveys.survey_id").
		Where("is_used_in.job_id = ?", jobID).
		Order("surveys.survey_id ASC").
		Scan(&surveys).Error
	if err != nil {
		return nil, err
	}
	job.Surveys = surveys
	summary := toCreatorJobSummary(*job)
	if err := r.db.WithContext(ctx).Model(&domain.Offer{}).Where("job_id = ? AND offer_status = ?", jobID, domain.OfferStatusPending).Count(&summary.PendingOfferCount).Error; err != nil {
		return nil, err
	}
	return &summary, nil
}

func (r *JobRepository) ListJobRequestsByCreator(ctx context.Context, creatorID, jobID int32) ([]domain.SentJobRequest, error) {
	if _, err := findOwnedJob(r.db.WithContext(ctx), creatorID, jobID); err != nil {
		return nil, mapJobRepositoryError(err)
	}
	requests := make([]domain.SentJobRequest, 0)
	err := r.db.WithContext(ctx).
		Table("job_requests").
		Select("job_requests.job_request_id, job_requests.job_id, job_requests.marketer_id, job_requests.request_status, job_requests.created_at, job_requests.responded_at, users.name AS marketer_name").
		Joins("JOIN marketers ON marketers.user_id = job_requests.marketer_id").
		Joins("JOIN users ON users.user_id = marketers.user_id").
		Where("job_requests.job_id = ?", jobID).
		Order("job_requests.created_at DESC, job_requests.job_request_id DESC").
		Scan(&requests).Error
	if err != nil {
		return nil, err
	}
	return requests, nil
}

func toCreatorJobSummary(job domain.Job) domain.CreatorJobSummary {
	summary := domain.CreatorJobSummary{
		PendingOfferCount: job.PendingOfferCount,
		Title:             job.Title, Description: job.Description, Budget: job.Budget,
		Deadline: job.Deadline, EffectiveDeadline: domain.EffectiveJobDeadline(job.Deadline, job.Surveys),
		JobID:     job.JobID,
		JobStatus: job.JobStatus,
		CreatedAt: job.CreatedAt,
		Surveys:   make([]domain.JobSurveySummary, 0, len(job.Surveys)),
	}
	for _, survey := range job.Surveys {
		summary.Surveys = append(summary.Surveys, domain.JobSurveySummary{SurveyID: survey.SurveyID, Title: survey.Title})
	}
	return summary
}

func (r *JobRepository) UpdateDetails(ctx context.Context, creatorID, jobID int32, details *domain.Job) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		job, err := lockOwnedJob(tx, creatorID, jobID)
		if err != nil {
			return err
		}
		if job.JobStatus != domain.JobStatusPending {
			return domain.ErrJobNotPending
		}
		if details.Surveys != nil {
			ids := make([]int32, 0, len(details.Surveys))
			for _, survey := range details.Surveys {
				ids = append(ids, survey.SurveyID)
			}
			if _, err := surveysForCreator(tx, creatorID, ids); err != nil {
				return err
			}
			if err := tx.Where("job_id = ?", jobID).Delete(&domain.JobSurvey{}).Error; err != nil {
				return err
			}
			links := make([]domain.JobSurvey, 0, len(ids))
			for _, id := range ids {
				links = append(links, domain.JobSurvey{JobID: jobID, SurveyID: id})
			}
			if err := tx.Create(&links).Error; err != nil {
				return err
			}
		}
		return tx.Model(job).Updates(map[string]any{
			"title": details.Title, "description": details.Description,
			"budget": details.Budget, "deadline": details.Deadline,
		}).Error
	})
	return mapJobRepositoryError(err)
}

func (r *JobRepository) ListSurveysByCreator(ctx context.Context, creatorID int32) ([]domain.Survey, error) {
	var surveys []domain.Survey
	err := r.db.WithContext(ctx).
		Where("user_id = ?", creatorID).
		Order("created_at DESC, survey_id DESC").
		Find(&surveys).Error
	return surveys, err
}

func (r *JobRepository) CreateWithSurveys(ctx context.Context, job *domain.Job, surveyIDs []int32) (*domain.Job, error) {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		surveys, err := surveysForCreator(tx, job.UserID, surveyIDs)
		if err != nil {
			return err
		}
		if err := tx.Create(job).Error; err != nil {
			return err
		}
		links := make([]domain.JobSurvey, 0, len(surveyIDs))
		for _, surveyID := range surveyIDs {
			links = append(links, domain.JobSurvey{JobID: job.JobID, SurveyID: surveyID})
		}
		if err := tx.Create(&links).Error; err != nil {
			return err
		}
		job.Surveys = surveys
		return nil
	})
	if err != nil {
		return nil, mapJobRepositoryError(err)
	}
	return job, nil
}

func (r *JobRepository) AddSurvey(ctx context.Context, creatorID, jobID, surveyID int32) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := lockOwnedJob(tx, creatorID, jobID); err != nil {
			return err
		}
		var survey domain.Survey
		if err := tx.Where("survey_id = ?", surveyID).First(&survey).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrSurveyNotFound
			}
			return err
		}
		if survey.UserID != creatorID {
			return domain.ErrSurveyForbidden
		}
		var count int64
		if err := tx.Table("is_used_in").Where("job_id = ? AND survey_id = ?", jobID, surveyID).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return domain.ErrJobSurveyAlreadyLinked
		}
		return tx.Create(&domain.JobSurvey{JobID: jobID, SurveyID: surveyID}).Error
	})
	return mapJobRepositoryError(err)
}

func (r *JobRepository) RemoveSurvey(ctx context.Context, creatorID, jobID, surveyID int32) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := lockOwnedJob(tx, creatorID, jobID); err != nil {
			return err
		}
		var links int64
		if err := tx.Table("is_used_in").Where("job_id = ?", jobID).Count(&links).Error; err != nil {
			return err
		}
		var target int64
		if err := tx.Table("is_used_in").Where("job_id = ? AND survey_id = ?", jobID, surveyID).Count(&target).Error; err != nil {
			return err
		}
		if target == 0 {
			return domain.ErrJobSurveyNotFound
		}
		if links <= 1 {
			return domain.ErrLastJobSurvey
		}
		return tx.Delete(&domain.JobSurvey{}, "job_id = ? AND survey_id = ?", jobID, surveyID).Error
	})
	return mapJobRepositoryError(err)
}

func (r *JobRepository) CreateBrief(ctx context.Context, creatorID, jobID int32, attachment *domain.Attachment) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := findOwnedJob(tx, creatorID, jobID); err != nil {
			return err
		}
		attachment.JobID = jobID
		return tx.Create(attachment).Error
	})
	return mapJobRepositoryError(err)
}

func (r *JobRepository) ListBriefs(ctx context.Context, creatorID, jobID int32) ([]domain.Attachment, error) {
	if _, err := findOwnedJob(r.db.WithContext(ctx), creatorID, jobID); err != nil {
		return nil, mapJobRepositoryError(err)
	}
	var attachments []domain.Attachment
	err := r.db.WithContext(ctx).
		Where("job_id = ? AND attachment_type = ?", jobID, domain.AttachmentTypeBrief).
		Order("upload_at DESC, attachment_id DESC").
		Find(&attachments).Error
	return attachments, err
}

func (r *JobRepository) CreateRequests(ctx context.Context, creatorID, jobID int32, marketerIDs []int32, createdAt time.Time) ([]domain.JobRequest, error) {
	requests := make([]domain.JobRequest, 0, len(marketerIDs))
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		job, err := lockOwnedJob(tx, creatorID, jobID)
		if err != nil {
			return err
		}
		// A hired job takes no new invitations. The job lock serializes this
		// check with offer acceptance.
		if job.JobStatus != domain.JobStatusPending || job.AcceptedOfferID != nil {
			return domain.ErrJobNotPending
		}
		var marketerIDsFound []int32
		if err := tx.Table("marketers").
			Joins("JOIN users ON users.user_id = marketers.user_id").
			Where("users.deleted_at IS NULL AND marketers.user_id IN ?", marketerIDs).
			Pluck("marketers.user_id", &marketerIDsFound).Error; err != nil {
			return err
		}
		if len(marketerIDsFound) != len(marketerIDs) {
			return domain.ErrMarketerNotFound
		}

		var active int64
		if err := tx.Model(&domain.JobRequest{}).
			Where("job_id = ? AND marketer_id IN ? AND request_status IN ?", jobID, marketerIDs,
				[]domain.JobRequestStatus{domain.JobRequestStatusPending, domain.JobRequestStatusAccepted}).
			Count(&active).Error; err != nil {
			return err
		}
		if active > 0 {
			return domain.ErrJobRequestAlreadyActive
		}

		for _, marketerID := range marketerIDs {
			requests = append(requests, domain.JobRequest{
				JobID:       jobID,
				MarketerID:  marketerID,
				Status:      domain.JobRequestStatusPending,
				CreatedAt:   createdAt,
				RespondedAt: nil,
			})
		}
		if err := tx.Create(&requests).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, mapJobRepositoryError(err)
	}
	return requests, nil
}

func surveysForCreator(tx *gorm.DB, creatorID int32, surveyIDs []int32) ([]domain.Survey, error) {
	var surveys []domain.Survey
	if err := tx.Where("survey_id IN ?", surveyIDs).Order("survey_id ASC").Find(&surveys).Error; err != nil {
		return nil, err
	}
	if len(surveys) != len(surveyIDs) {
		return nil, domain.ErrSurveyNotFound
	}
	for _, survey := range surveys {
		if survey.UserID != creatorID {
			return nil, domain.ErrSurveyForbidden
		}
	}
	return surveys, nil
}

func findOwnedJob(tx *gorm.DB, creatorID, jobID int32) (*domain.Job, error) {
	var job domain.Job
	if err := tx.Where("job_id = ?", jobID).First(&job).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrJobNotFound
		}
		return nil, err
	}
	if job.UserID != creatorID {
		return nil, domain.ErrJobForbidden
	}
	return &job, nil
}

// lockJob locks the job FOR UPDATE until the transaction ends.
func lockJob(tx *gorm.DB, jobID int32) (*domain.Job, error) {
	var job domain.Job
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("job_id = ?", jobID).First(&job).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrJobNotFound
		}
		return nil, err
	}
	return &job, nil
}

func lockOwnedJob(tx *gorm.DB, creatorID, jobID int32) (*domain.Job, error) {
	job, err := lockJob(tx, jobID)
	if err != nil {
		return nil, err
	}
	if job.UserID != creatorID {
		return nil, domain.ErrJobForbidden
	}
	return job, nil
}

func mapJobRepositoryError(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		switch pgErr.ConstraintName {
		case "idx_job_request_active_unique":
			return domain.ErrJobRequestAlreadyActive
		case "is_used_in_pkey":
			return domain.ErrJobSurveyAlreadyLinked
		}
	}
	return err
}
