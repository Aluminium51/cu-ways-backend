package postgres

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"gorm.io/gorm"
)

func TestJobRepositoryCreatesJobAndSurveyLinksAtomically(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	ctx := context.Background()
	creatorID := createIntegrationCreator(t, db, "job-create-owner")
	otherCreatorID := createIntegrationCreator(t, db, "job-create-other")
	ownedSurvey := createIntegrationSurvey(t, db, creatorID, "owned")
	foreignSurvey := createIntegrationSurvey(t, db, otherCreatorID, "foreign")
	repo := NewJobRepository(db)

	failedJob := &domain.Job{UserID: creatorID, JobStatus: domain.JobStatusPending, CreatedAt: time.Now().UTC()}
	if _, err := repo.CreateWithSurveys(ctx, failedJob, []int32{ownedSurvey, foreignSurvey}); !errors.Is(err, domain.ErrSurveyForbidden) {
		t.Fatalf("expected foreign survey to reject job creation, got %v", err)
	}
	var jobsAfterFailure int64
	if err := db.Model(&domain.Job{}).Where("user_id = ?", creatorID).Count(&jobsAfterFailure).Error; err != nil {
		t.Fatal(err)
	}
	if jobsAfterFailure != 0 {
		t.Fatalf("failed job creation left %d job rows", jobsAfterFailure)
	}

	linkFailure := errors.New("injected survey link failure")
	callbackName := "test:fail_job_survey_link"
	if err := db.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Table == "is_used_in" {
			_ = tx.AddError(linkFailure)
		}
	}); err != nil {
		t.Fatal(err)
	}
	failedAfterJobInsert := &domain.Job{UserID: creatorID, JobStatus: domain.JobStatusPending, CreatedAt: time.Now().UTC()}
	_, err := repo.CreateWithSurveys(ctx, failedAfterJobInsert, []int32{ownedSurvey})
	if removeErr := db.Callback().Create().Remove(callbackName); removeErr != nil {
		t.Fatal(removeErr)
	}
	if !errors.Is(err, linkFailure) {
		t.Fatalf("expected injected link failure, got %v", err)
	}
	var partiallyCreated int64
	if err := db.Model(&domain.Job{}).Where("job_id = ?", failedAfterJobInsert.JobID).Count(&partiallyCreated).Error; err != nil {
		t.Fatal(err)
	}
	if partiallyCreated != 0 {
		t.Fatalf("link failure left job %d persisted", failedAfterJobInsert.JobID)
	}

	job, err := repo.CreateWithSurveys(ctx, &domain.Job{
		UserID: creatorID, JobStatus: domain.JobStatusPending, CreatedAt: time.Now().UTC(),
	}, []int32{ownedSurvey})
	if err != nil {
		t.Fatal(err)
	}
	if job.JobID == 0 || job.JobStatus != domain.JobStatusPending || job.AcceptedOfferID != nil {
		t.Fatalf("unexpected initial job state: %+v", job)
	}
	var linkedSurveys int64
	if err := db.Table("is_used_in").Where("job_id = ?", job.JobID).Count(&linkedSurveys).Error; err != nil {
		t.Fatal(err)
	}
	if linkedSurveys != 1 {
		t.Fatalf("expected the initial survey link, got %d links", linkedSurveys)
	}
}

func TestJobRepositorySurveyLinksEnforceOwnerAndKeepOneSurvey(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	ctx := context.Background()
	creatorID := createIntegrationCreator(t, db, "job-link-owner")
	otherCreatorID := createIntegrationCreator(t, db, "job-link-other")
	firstSurvey := createIntegrationSurvey(t, db, creatorID, "first")
	secondSurvey := createIntegrationSurvey(t, db, creatorID, "second")
	foreignSurvey := createIntegrationSurvey(t, db, otherCreatorID, "foreign")
	repo := NewJobRepository(db)
	job, err := repo.CreateWithSurveys(ctx, &domain.Job{
		UserID: creatorID, JobStatus: domain.JobStatusPending, CreatedAt: time.Now().UTC(),
	}, []int32{firstSurvey})
	if err != nil {
		t.Fatal(err)
	}

	if err := repo.AddSurvey(ctx, otherCreatorID, job.JobID, secondSurvey); !errors.Is(err, domain.ErrJobForbidden) {
		t.Fatalf("expected another creator to be denied, got %v", err)
	}
	if err := repo.AddSurvey(ctx, creatorID, job.JobID, foreignSurvey); !errors.Is(err, domain.ErrSurveyForbidden) {
		t.Fatalf("expected foreign survey to be denied, got %v", err)
	}
	if err := repo.AddSurvey(ctx, creatorID, job.JobID, secondSurvey); err != nil {
		t.Fatal(err)
	}
	if err := repo.AddSurvey(ctx, creatorID, job.JobID, secondSurvey); !errors.Is(err, domain.ErrJobSurveyAlreadyLinked) {
		t.Fatalf("expected duplicate link conflict, got %v", err)
	}
	if err := repo.RemoveSurvey(ctx, creatorID, job.JobID, secondSurvey); err != nil {
		t.Fatal(err)
	}
	if err := repo.RemoveSurvey(ctx, creatorID, job.JobID, firstSurvey); !errors.Is(err, domain.ErrLastJobSurvey) {
		t.Fatalf("expected last survey to be protected, got %v", err)
	}
	if err := repo.RemoveSurvey(ctx, creatorID, job.JobID, secondSurvey); !errors.Is(err, domain.ErrJobSurveyNotFound) {
		t.Fatalf("expected missing link to return not found, got %v", err)
	}
}

func TestJobRepositoryBriefHistoryAndInvitations(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	ctx := context.Background()
	creatorID := createIntegrationCreator(t, db, "job-brief-owner")
	otherCreatorID := createIntegrationCreator(t, db, "job-brief-other")
	marketerID := createIntegrationMarketer(t, db, "job-request-target")
	secondMarketerID := createIntegrationMarketer(t, db, "job-request-second")
	thirdMarketerID := createIntegrationMarketer(t, db, "job-request-third")
	surveyID := createIntegrationSurvey(t, db, creatorID, "brief")
	repo := NewJobRepository(db)
	job, err := repo.CreateWithSurveys(ctx, &domain.Job{
		UserID: creatorID, JobStatus: domain.JobStatusPending, CreatedAt: time.Now().UTC(),
	}, []int32{surveyID})
	if err != nil {
		t.Fatal(err)
	}
	createdAt := time.Now().UTC().Truncate(time.Microsecond)
	if _, err := repo.CreateRequests(ctx, otherCreatorID, job.JobID, []int32{thirdMarketerID}, createdAt); !errors.Is(err, domain.ErrJobForbidden) {
		t.Fatalf("expected another creator to be denied when inviting marketers, got %v", err)
	}

	firstUpload := time.Now().UTC().Add(-time.Minute)
	latestUpload := time.Now().UTC()
	if err := repo.CreateBrief(ctx, creatorID, job.JobID, &domain.Attachment{
		AttachmentType: domain.AttachmentTypeBrief, URL: "https://example.invalid/brief-v1", UploadAt: firstUpload,
	}); err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateBrief(ctx, creatorID, job.JobID, &domain.Attachment{
		AttachmentType: domain.AttachmentTypeBrief, URL: "https://example.invalid/brief-v2", UploadAt: latestUpload,
	}); err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateBrief(ctx, creatorID, job.JobID, &domain.Attachment{
		AttachmentType: domain.AttachmentTypeProof, URL: "https://example.invalid/proof", UploadAt: latestUpload,
	}); err != nil {
		t.Fatal(err)
	}
	briefs, err := repo.ListBriefs(ctx, creatorID, job.JobID)
	if err != nil {
		t.Fatal(err)
	}
	if len(briefs) != 2 || briefs[0].URL != "https://example.invalid/brief-v2" || briefs[1].URL != "https://example.invalid/brief-v1" {
		t.Fatalf("expected newest Brief first and history retained, got %+v", briefs)
	}
	if err := repo.CreateBrief(ctx, otherCreatorID, job.JobID, &domain.Attachment{
		AttachmentType: domain.AttachmentTypeBrief, URL: "https://example.invalid/unauthorized", UploadAt: latestUpload,
	}); !errors.Is(err, domain.ErrJobForbidden) {
		t.Fatalf("expected cross-owner Brief creation to be denied, got %v", err)
	}
	if _, err := repo.ListBriefs(ctx, otherCreatorID, job.JobID); !errors.Is(err, domain.ErrJobForbidden) {
		t.Fatalf("expected cross-owner Brief list to be denied, got %v", err)
	}

	requests, err := repo.CreateRequests(ctx, creatorID, job.JobID, []int32{marketerID, secondMarketerID}, createdAt)
	if err != nil {
		t.Fatal(err)
	}
	if len(requests) != 2 {
		t.Fatalf("expected two invitations, got %+v", requests)
	}
	for _, request := range requests {
		if request.Status != domain.JobRequestStatusPending || request.RespondedAt != nil || !request.CreatedAt.Equal(createdAt) {
			t.Fatalf("unexpected invitation state: %+v", request)
		}
	}

	if _, err := repo.CreateRequests(ctx, creatorID, job.JobID, []int32{thirdMarketerID, marketerID}, createdAt); !errors.Is(err, domain.ErrJobRequestAlreadyActive) {
		t.Fatalf("expected active invitation to reject a repeated batch, got %v", err)
	}
	var thirdInvitations int64
	if err := db.Model(&domain.JobRequest{}).Where("job_id = ? AND marketer_id = ?", job.JobID, thirdMarketerID).Count(&thirdInvitations).Error; err != nil {
		t.Fatal(err)
	}
	if thirdInvitations != 0 {
		t.Fatalf("failed invitation batch persisted %d new rows", thirdInvitations)
	}

	if err := db.Model(&domain.JobRequest{}).
		Where("job_id = ? AND marketer_id = ?", job.JobID, marketerID).
		Update("request_status", domain.JobRequestStatusDeclined).Error; err != nil {
		t.Fatal(err)
	}
	retried, err := repo.CreateRequests(ctx, creatorID, job.JobID, []int32{marketerID}, createdAt.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(retried) != 1 || retried[0].Status != domain.JobRequestStatusPending {
		t.Fatalf("expected a declined invitation to be resendable, got %+v", retried)
	}
}

func createIntegrationSurvey(t *testing.T, db *gorm.DB, creatorID int32, label string) int32 {
	t.Helper()
	var surveyID int32
	if err := db.Raw(`
INSERT INTO surveys (user_id, title, survey_link, created_at)
VALUES (?, ?, ?, ?)
RETURNING survey_id`, creatorID, fmt.Sprintf("Survey %s", label), "https://example.invalid/survey", time.Now().UTC()).Scan(&surveyID).Error; err != nil {
		t.Fatalf("create survey fixture: %v", err)
	}
	return surveyID
}
