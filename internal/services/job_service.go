package services

import (
	"context"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
)

type CreateJobInput struct {
	SurveyIDs []int32
	JobDetailsInput
}

type JobDetailsInput struct {
	SurveyIDs   []int32
	Title       string
	Description string
	Budget      *string
	Deadline    *string
}

func jobDetails(input JobDetailsInput) (*domain.Job, error) {
	job := &domain.Job{Title: strings.TrimSpace(input.Title), Description: strings.TrimSpace(input.Description)}
	if utf8.RuneCountInString(job.Title) > 200 || utf8.RuneCountInString(job.Description) > 5000 {
		return nil, domain.ErrInvalidJob
	}
	if input.Budget != nil {
		price, err := parsePrice(*input.Budget)
		if err != nil {
			return nil, domain.ErrInvalidJob
		}
		job.Budget = &price
	}
	if input.Deadline != nil {
		date, err := time.Parse(time.DateOnly, *input.Deadline)
		if err != nil {
			return nil, domain.ErrInvalidJob
		}
		job.Deadline = &date
	}
	return job, nil
}

func (s *JobService) UpdateDetails(ctx context.Context, actor Actor, jobID int32, input JobDetailsInput) (*domain.CreatorJobSummary, error) {
	if err := s.requireCreator(ctx, actor); err != nil {
		return nil, err
	}
	if jobID < 1 || strings.TrimSpace(input.Title) == "" {
		return nil, domain.ErrInvalidJob
	}
	details, err := jobDetails(input)
	if err != nil {
		return nil, err
	}
	if input.SurveyIDs != nil {
		if err := validateIDs(input.SurveyIDs); err != nil {
			return nil, err
		}
		for _, id := range input.SurveyIDs {
			details.Surveys = append(details.Surveys, domain.Survey{SurveyID: id})
		}
	}
	if err := s.repo.UpdateDetails(ctx, actor.UserID, jobID, details); err != nil {
		return nil, err
	}
	return s.repo.GetJobByCreator(ctx, actor.UserID, jobID)
}

type CreateJobRequestsInput struct {
	MarketerIDs []int32
}

type JobService struct {
	repo        ports.JobRepository
	memberships ports.MembershipRepository
}

func NewJobService(repo ports.JobRepository, memberships ports.MembershipRepository) *JobService {
	return &JobService{repo: repo, memberships: memberships}
}

func (s *JobService) ListSurveys(ctx context.Context, actor Actor) ([]domain.Survey, error) {
	if err := s.requireCreator(ctx, actor); err != nil {
		return nil, err
	}
	return s.repo.ListSurveysByCreator(ctx, actor.UserID)
}

func (s *JobService) ListMyJobs(ctx context.Context, actor Actor) ([]domain.CreatorJobSummary, error) {
	if err := s.requireCreator(ctx, actor); err != nil {
		return nil, err
	}
	return s.repo.ListJobsByCreator(ctx, actor.UserID)
}

func (s *JobService) GetJob(ctx context.Context, actor Actor, jobID int32) (*domain.CreatorJobSummary, error) {
	if err := s.requireCreator(ctx, actor); err != nil {
		return nil, err
	}
	if jobID < 1 {
		return nil, domain.ErrInvalidJob
	}
	return s.repo.GetJobByCreator(ctx, actor.UserID, jobID)
}

func (s *JobService) ListJobRequests(ctx context.Context, actor Actor, jobID int32) ([]domain.SentJobRequest, error) {
	if err := s.requireCreator(ctx, actor); err != nil {
		return nil, err
	}
	if jobID < 1 {
		return nil, domain.ErrInvalidJob
	}
	return s.repo.ListJobRequestsByCreator(ctx, actor.UserID, jobID)
}

func (s *JobService) Create(ctx context.Context, actor Actor, input CreateJobInput) (*domain.Job, error) {
	if err := s.requireCreator(ctx, actor); err != nil {
		return nil, err
	}
	if err := validateIDs(input.SurveyIDs); err != nil {
		return nil, err
	}
	job, err := jobDetails(input.JobDetailsInput)
	if err != nil {
		return nil, err
	}
	job.UserID = actor.UserID
	job.JobStatus = domain.JobStatusPending
	job.CreatedAt = time.Now().UTC()
	return s.repo.CreateWithSurveys(ctx, job, input.SurveyIDs)
}

func (s *JobService) AddSurvey(ctx context.Context, actor Actor, jobID, surveyID int32) error {
	if err := s.requireCreator(ctx, actor); err != nil {
		return err
	}
	if jobID < 1 || surveyID < 1 {
		return domain.ErrInvalidJob
	}
	return s.repo.AddSurvey(ctx, actor.UserID, jobID, surveyID)
}

func (s *JobService) RemoveSurvey(ctx context.Context, actor Actor, jobID, surveyID int32) error {
	if err := s.requireCreator(ctx, actor); err != nil {
		return err
	}
	if jobID < 1 || surveyID < 1 {
		return domain.ErrInvalidJob
	}
	return s.repo.RemoveSurvey(ctx, actor.UserID, jobID, surveyID)
}

func (s *JobService) AddBrief(ctx context.Context, actor Actor, jobID int32, rawURL string) (*domain.Attachment, error) {
	if err := s.requireCreator(ctx, actor); err != nil {
		return nil, err
	}
	if jobID < 1 {
		return nil, domain.ErrInvalidJob
	}
	briefURL := strings.TrimSpace(rawURL)
	parsed, err := url.ParseRequestURI(briefURL)
	if err != nil || parsed == nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return nil, domain.ErrInvalidBriefURL
	}
	attachment := &domain.Attachment{
		AttachmentType: domain.AttachmentTypeBrief,
		URL:            briefURL,
		UploadAt:       time.Now().UTC(),
	}
	if err := s.repo.CreateBrief(ctx, actor.UserID, jobID, attachment); err != nil {
		return nil, err
	}
	return attachment, nil
}

func (s *JobService) ListBriefs(ctx context.Context, actor Actor, jobID int32) ([]domain.Attachment, error) {
	if err := s.requireCreator(ctx, actor); err != nil {
		return nil, err
	}
	if jobID < 1 {
		return nil, domain.ErrInvalidJob
	}
	return s.repo.ListBriefs(ctx, actor.UserID, jobID)
}

func (s *JobService) CreateRequests(ctx context.Context, actor Actor, jobID int32, input CreateJobRequestsInput) ([]domain.JobRequest, error) {
	if err := s.requireCreator(ctx, actor); err != nil {
		return nil, err
	}
	if jobID < 1 {
		return nil, domain.ErrInvalidJob
	}
	if err := validateIDs(input.MarketerIDs); err != nil {
		return nil, domain.ErrInvalidJobRequest
	}
	return s.repo.CreateRequests(ctx, actor.UserID, jobID, input.MarketerIDs, time.Now().UTC())
}

func (s *JobService) requireCreator(ctx context.Context, actor Actor) error {
	if actor.UserID < 1 {
		return domain.ErrCreatorRequired
	}
	isCreator, err := s.memberships.IsCreator(ctx, actor.UserID)
	if err != nil {
		return err
	}
	if !isCreator {
		return domain.ErrCreatorRequired
	}
	return nil
}

func validateIDs(ids []int32) error {
	if len(ids) == 0 {
		return domain.ErrNoJobSurveys
	}
	seen := make(map[int32]struct{}, len(ids))
	for _, id := range ids {
		if id < 1 {
			return domain.ErrInvalidJob
		}
		if _, exists := seen[id]; exists {
			return domain.ErrInvalidJob
		}
		seen[id] = struct{}{}
	}
	return nil
}
