package services

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
)

const (
	MaxSurveyTitleLength       = 200
	MaxSurveyDescriptionLength = 5000
	MaxSurveyTargetGroupLength = 200
)

type CreateSurveyInput struct {
	Title            string
	Description      *string
	SurveyLink       string
	TargetGroup      *string
	DesiredResponses *int32
	Deadline         *time.Time
}

type UpdateSurveyInput struct {
	TitleSet            bool
	Title               string
	DescriptionSet      bool
	Description         *string
	SurveyLinkSet       bool
	SurveyLink          string
	TargetGroupSet      bool
	TargetGroup         *string
	DesiredResponsesSet bool
	DesiredResponses    *int32
	DeadlineSet         bool
	Deadline            *time.Time
}

type SurveyService struct {
	repo ports.SurveyRepository
}

func NewSurveyService(repo ports.SurveyRepository) *SurveyService {
	return &SurveyService{repo: repo}
}

func (s *SurveyService) Create(ctx context.Context, actor Actor, input CreateSurveyInput) (*domain.Survey, error) {
	if actor.UserID < 1 {
		return nil, domain.ErrSurveyForbidden
	}
	survey, err := normalizeSurvey(domain.Survey{
		UserID:           actor.UserID,
		Title:            input.Title,
		Description:      input.Description,
		SurveyLink:       input.SurveyLink,
		TargetGroup:      input.TargetGroup,
		DesiredResponses: input.DesiredResponses,
		Deadline:         input.Deadline,
		CreatedAt:        time.Now().UTC(),
	})
	if err != nil {
		return nil, err
	}
	if err := s.repo.CreateForCreator(ctx, actor.UserID, survey); err != nil {
		return nil, err
	}
	return survey, nil
}

func (s *SurveyService) Get(ctx context.Context, actor Actor, surveyID int32) (*domain.Survey, error) {
	survey, err := s.repo.FindByID(ctx, surveyID)
	if err != nil {
		return nil, err
	}
	if !actor.IsAdmin && survey.UserID != actor.UserID {
		return nil, domain.ErrSurveyForbidden
	}
	return survey, nil
}

func (s *SurveyService) Update(ctx context.Context, actor Actor, surveyID int32, input UpdateSurveyInput) (*domain.Survey, error) {
	survey, err := s.repo.FindByID(ctx, surveyID)
	if err != nil {
		return nil, err
	}
	if !actor.IsAdmin && survey.UserID != actor.UserID {
		return nil, domain.ErrSurveyForbidden
	}
	if !input.TitleSet && !input.DescriptionSet && !input.SurveyLinkSet && !input.TargetGroupSet && !input.DesiredResponsesSet && !input.DeadlineSet {
		return nil, domain.ErrNoSurveyChanges
	}
	patch := ports.SurveyPatch{
		TitleSet:            input.TitleSet,
		Title:               input.Title,
		DescriptionSet:      input.DescriptionSet,
		Description:         input.Description,
		SurveyLinkSet:       input.SurveyLinkSet,
		SurveyLink:          input.SurveyLink,
		TargetGroupSet:      input.TargetGroupSet,
		TargetGroup:         input.TargetGroup,
		DesiredResponsesSet: input.DesiredResponsesSet,
		DesiredResponses:    input.DesiredResponses,
		DeadlineSet:         input.DeadlineSet,
		Deadline:            input.Deadline,
	}
	if input.TitleSet {
		patch.Title = strings.TrimSpace(input.Title)
		if patch.Title == "" || utf8.RuneCountInString(patch.Title) > MaxSurveyTitleLength {
			return nil, domain.ErrInvalidSurvey
		}
	}
	if input.SurveyLinkSet {
		patch.SurveyLink = strings.TrimSpace(input.SurveyLink)
		if patch.SurveyLink == "" {
			return nil, domain.ErrInvalidSurvey
		}
	}
	if input.TargetGroupSet && input.TargetGroup != nil {
		target := strings.TrimSpace(*input.TargetGroup)
		if utf8.RuneCountInString(target) > MaxSurveyTargetGroupLength {
			return nil, domain.ErrInvalidSurvey
		}
		patch.TargetGroup = &target
	}
	if input.DescriptionSet && input.Description != nil {
		description := strings.TrimSpace(*input.Description)
		if utf8.RuneCountInString(description) > MaxSurveyDescriptionLength {
			return nil, domain.ErrInvalidSurvey
		}
		patch.Description = &description
	}
	if input.DesiredResponsesSet && input.DesiredResponses != nil && *input.DesiredResponses < 1 {
		return nil, domain.ErrInvalidSurvey
	}
	return s.repo.Update(ctx, surveyID, patch)
}

func (s *SurveyService) Delete(ctx context.Context, actor Actor, surveyID int32) error {
	survey, err := s.repo.FindByID(ctx, surveyID)
	if err != nil {
		return err
	}
	if !actor.IsAdmin && survey.UserID != actor.UserID {
		return domain.ErrSurveyForbidden
	}
	return s.repo.Delete(ctx, surveyID)
}

func normalizeSurvey(survey domain.Survey) (*domain.Survey, error) {
	survey.Title = strings.TrimSpace(survey.Title)
	survey.SurveyLink = strings.TrimSpace(survey.SurveyLink)
	if survey.Title == "" || utf8.RuneCountInString(survey.Title) > MaxSurveyTitleLength || survey.SurveyLink == "" {
		return nil, domain.ErrInvalidSurvey
	}
	if survey.TargetGroup != nil {
		target := strings.TrimSpace(*survey.TargetGroup)
		if utf8.RuneCountInString(target) > MaxSurveyTargetGroupLength {
			return nil, domain.ErrInvalidSurvey
		}
		survey.TargetGroup = &target
	}
	if survey.DesiredResponses != nil && *survey.DesiredResponses < 1 {
		return nil, domain.ErrInvalidSurvey
	}
	if survey.Description != nil {
		description := strings.TrimSpace(*survey.Description)
		if utf8.RuneCountInString(description) > MaxSurveyDescriptionLength {
			return nil, domain.ErrInvalidSurvey
		}
		survey.Description = &description
	}
	survey.CreatedAt = survey.CreatedAt.UTC()
	return &survey, nil
}
