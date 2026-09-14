package services

import (
	"context"
	"errors"
	"testing"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
)

type fakeSurveyRepository struct {
	survey    *domain.Survey
	createdBy int32
	deleted   bool
}

func (f *fakeSurveyRepository) CreateForCreator(_ context.Context, userID int32, survey *domain.Survey) error {
	f.createdBy = userID
	f.survey = survey
	return nil
}

func (f *fakeSurveyRepository) FindByID(context.Context, int32) (*domain.Survey, error) {
	if f.survey == nil {
		return nil, domain.ErrSurveyNotFound
	}
	return f.survey, nil
}

func (f *fakeSurveyRepository) Update(_ context.Context, _ int32, patch ports.SurveyPatch) (*domain.Survey, error) {
	if patch.TitleSet {
		f.survey.Title = patch.Title
	}
	if patch.SurveyLinkSet {
		f.survey.SurveyLink = patch.SurveyLink
	}
	return f.survey, nil
}

func (f *fakeSurveyRepository) Delete(context.Context, int32) error {
	f.deleted = true
	return nil
}

func TestSurveyServiceCreatesSurveyAndProvisionsCreator(t *testing.T) {
	repo := &fakeSurveyRepository{}
	service := NewSurveyService(repo)
	result, err := service.Create(context.Background(), Actor{UserID: 9}, CreateSurveyInput{Title: "  Student survey ", SurveyLink: " https://example.com/survey "})
	if err != nil {
		t.Fatal(err)
	}
	if repo.createdBy != 9 || result.Title != "Student survey" || result.SurveyLink != "https://example.com/survey" {
		t.Fatalf("unexpected survey: %+v", result)
	}
}

func TestSurveyServiceRejectsInvalidMetadataAndForbiddenOwner(t *testing.T) {
	service := NewSurveyService(&fakeSurveyRepository{})
	_, err := service.Create(context.Background(), Actor{UserID: 1}, CreateSurveyInput{Title: "", SurveyLink: "https://example.com"})
	if !errors.Is(err, domain.ErrInvalidSurvey) {
		t.Fatalf("expected invalid survey, got %v", err)
	}
	repo := &fakeSurveyRepository{survey: &domain.Survey{SurveyID: 2, UserID: 10, Title: "Survey", SurveyLink: "https://example.com"}}
	service = NewSurveyService(repo)
	if _, err := service.Get(context.Background(), Actor{UserID: 11}, 2); !errors.Is(err, domain.ErrSurveyForbidden) {
		t.Fatalf("expected forbidden survey access, got %v", err)
	}
}
