package postgres

import (
	"context"
	"errors"
	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/shopspring/decimal"
	"testing"
	"time"
)

func TestJobDetailsPersistAndEditAtomicallyWithOwnedSurveys(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	ctx := context.Background()
	owner := createIntegrationCreator(t, db, "details-owner")
	other := createIntegrationCreator(t, db, "details-other")
	survey := createIntegrationSurvey(t, db, owner, "details-survey")
	foreign := createIntegrationSurvey(t, db, other, "details-foreign")
	deadline := time.Date(2030, 10, 28, 0, 0, 0, 0, time.UTC)
	price := decimal.RequireFromString("1200.50")
	repo := NewJobRepository(db)
	job, err := repo.CreateWithSurveys(ctx, &domain.Job{UserID: owner, JobStatus: domain.JobStatusPending, CreatedAt: time.Now().UTC(), Title: "Campus campaign", Description: "Reach students", Budget: &price, Deadline: &deadline}, []int32{survey})
	if err != nil {
		t.Fatal(err)
	}
	saved, err := repo.GetJobByCreator(ctx, owner, job.JobID)
	if err != nil || saved.Title != "Campus campaign" || saved.Description != "Reach students" || saved.Budget == nil || !saved.Budget.Equal(price) || saved.Deadline == nil || !saved.Deadline.Equal(deadline) {
		t.Fatalf("details did not survive reload: %+v, %v", saved, err)
	}
	details := &domain.Job{Title: "Updated campaign", Budget: &price, Deadline: &deadline, Surveys: []domain.Survey{{SurveyID: foreign}}}
	if err := repo.UpdateDetails(ctx, owner, job.JobID, details); !errors.Is(err, domain.ErrSurveyForbidden) {
		t.Fatalf("foreign survey accepted: %v", err)
	}
	saved, err = repo.GetJobByCreator(ctx, owner, job.JobID)
	if err != nil || saved.Title != "Campus campaign" || len(saved.Surveys) != 1 || saved.Surveys[0].SurveyID != survey {
		t.Fatalf("failed edit changed saved scope: %+v, %v", saved, err)
	}
	details.Surveys = []domain.Survey{{SurveyID: survey}}
	if err := repo.UpdateDetails(ctx, other, job.JobID, details); !errors.Is(err, domain.ErrJobForbidden) {
		t.Fatalf("foreign creator edited job: %v", err)
	}
	if err := repo.UpdateDetails(ctx, owner, job.JobID, details); err != nil {
		t.Fatal(err)
	}
	saved, err = repo.GetJobByCreator(ctx, owner, job.JobID)
	if err != nil || saved.Title != details.Title {
		t.Fatalf("edit not persisted: %+v, %v", saved, err)
	}
	if err := db.Model(&domain.Job{}).Where("job_id = ?", job.JobID).Update("job_status", domain.JobStatusAccepted).Error; err != nil {
		t.Fatal(err)
	}
	if err := repo.UpdateDetails(ctx, owner, job.JobID, details); !errors.Is(err, domain.ErrJobNotPending) {
		t.Fatalf("accepted job remained editable: %v", err)
	}
}
