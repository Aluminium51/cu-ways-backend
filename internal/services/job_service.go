package services

import (
	"context"
	"net/url"
	"strings"
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
)

type CreateJobInput struct {
	SurveyIDs []int32
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

func (s *JobService) Create(ctx context.Context, actor Actor, input CreateJobInput) (*domain.Job, error) {
	if err := s.requireCreator(ctx, actor); err != nil {
		return nil, err
	}
	if err := validateIDs(input.SurveyIDs); err != nil {
		return nil, err
	}
	job := &domain.Job{
		UserID:    actor.UserID,
		JobStatus: domain.JobStatusPending,
		CreatedAt: time.Now().UTC(),
	}
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
