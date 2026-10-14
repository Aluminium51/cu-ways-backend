package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
	"gorm.io/gorm"
)

func createIntegrationJobRequest(t *testing.T, db *gorm.DB, creatorID, marketerID int32, surveyIDs ...int32) (int32, int32) {
	t.Helper()
	job, err := NewJobRepository(db).CreateWithSurveys(context.Background(), &domain.Job{
		UserID: creatorID, JobStatus: domain.JobStatusPending, CreatedAt: time.Now().UTC(),
	}, surveyIDs)
	if err != nil {
		t.Fatalf("create job fixture: %v", err)
	}
	requests, err := NewJobRepository(db).CreateRequests(context.Background(), creatorID, job.JobID, []int32{marketerID}, time.Now().UTC())
	if err != nil {
		t.Fatalf("create job request fixture: %v", err)
	}
	return job.JobID, requests[0].JobRequestID
}

func TestMarketerJobRequestRepositoryListsOnlyOwnRequestsWithJobContext(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	ctx := context.Background()
	creatorID := createIntegrationCreator(t, db, "job-request-list-creator")
	marketerID := createIntegrationMarketer(t, db, "job-request-list-marketer")
	otherMarketerID := createIntegrationMarketer(t, db, "job-request-list-other")
	firstSurvey := createIntegrationSurvey(t, db, creatorID, "list-first")
	secondSurvey := createIntegrationSurvey(t, db, creatorID, "list-second")
	jobID, requestID := createIntegrationJobRequest(t, db, creatorID, marketerID, firstSurvey, secondSurvey)
	createIntegrationJobRequest(t, db, creatorID, otherMarketerID, firstSurvey)
	for _, brief := range []domain.Attachment{
		{AttachmentType: domain.AttachmentTypeBrief, URL: "https://example.invalid/old", UploadAt: time.Now().UTC().Add(-time.Hour)},
		{AttachmentType: domain.AttachmentTypeBrief, URL: "https://example.invalid/current", UploadAt: time.Now().UTC()},
	} {
		if err := NewJobRepository(db).CreateBrief(ctx, creatorID, jobID, &brief); err != nil {
			t.Fatal(err)
		}
	}

	repo := NewMarketerJobRequestRepository(db)
	items, err := repo.ListForMarketer(ctx, marketerID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("expected only the marketer's own request, got %d", len(items))
	}
	item := items[0]
	if item.Request.JobRequestID != requestID || item.Request.MarketerID != marketerID || item.JobStatus != domain.JobStatusPending {
		t.Fatalf("unexpected request: %+v", item.Request)
	}
	if item.CreatorID != creatorID || item.CreatorName == "" {
		t.Fatalf("expected creator details, got id=%d name=%q", item.CreatorID, item.CreatorName)
	}
	if len(item.Surveys) != 2 || item.Surveys[0].SurveyID != firstSurvey || item.Surveys[1].SurveyID != secondSurvey {
		t.Fatalf("expected both linked surveys in order, got %+v", item.Surveys)
	}
	if item.CurrentBrief == nil || item.CurrentBrief.URL != "https://example.invalid/current" {
		t.Fatalf("expected the newest Brief, got %+v", item.CurrentBrief)
	}

	accepted := domain.JobRequestStatusAccepted
	filtered, err := repo.ListForMarketer(ctx, marketerID, &accepted)
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered) != 0 {
		t.Fatalf("status filter returned %d non-matching requests", len(filtered))
	}
}

func TestMarketerJobRequestRepositoryRespondsOnlyToOwnPendingRequest(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	ctx := context.Background()
	creatorID := createIntegrationCreator(t, db, "job-request-respond-creator")
	marketerID := createIntegrationMarketer(t, db, "job-request-respond-marketer")
	otherMarketerID := createIntegrationMarketer(t, db, "job-request-respond-other")
	surveyID := createIntegrationSurvey(t, db, creatorID, "respond")
	_, acceptID := createIntegrationJobRequest(t, db, creatorID, marketerID, surveyID)
	_, declineID := createIntegrationJobRequest(t, db, creatorID, marketerID, surveyID)
	repo := NewMarketerJobRequestRepository(db)
	respondedAt := time.Now().UTC().Truncate(time.Microsecond)

	if _, err := repo.RespondToPending(ctx, otherMarketerID, acceptID, ports.JobRequestResponse{Status: domain.JobRequestStatusAccepted, RespondedAt: respondedAt}); !errors.Is(err, domain.ErrJobRequestNotFound) {
		t.Fatalf("expected another marketer's request to be hidden, got %v", err)
	}

	accepted, err := repo.RespondToPending(ctx, marketerID, acceptID, ports.JobRequestResponse{Status: domain.JobRequestStatusAccepted, RespondedAt: respondedAt})
	if err != nil {
		t.Fatal(err)
	}
	if accepted.Status != domain.JobRequestStatusAccepted || accepted.RespondedAt == nil {
		t.Fatalf("unexpected accepted request: %+v", accepted)
	}
	var stored domain.JobRequest
	if err := db.First(&stored, "job_request_id = ?", acceptID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Status != domain.JobRequestStatusAccepted || stored.RespondedAt == nil || !stored.RespondedAt.Equal(respondedAt) {
		t.Fatalf("response was not persisted: %+v", stored)
	}

	if _, err := repo.RespondToPending(ctx, marketerID, acceptID, ports.JobRequestResponse{Status: domain.JobRequestStatusDeclined, RespondedAt: respondedAt}); !errors.Is(err, domain.ErrJobRequestNotPending) {
		t.Fatalf("expected a resolved request to reject another response, got %v", err)
	}

	reason, note := "Schedule conflict", "Busy with exams."
	if _, err := repo.RespondToPending(ctx, marketerID, declineID, ports.JobRequestResponse{
		Status: domain.JobRequestStatusDeclined, RespondedAt: respondedAt, DeclineReason: &reason, DeclineNote: &note,
	}); err != nil {
		t.Fatal(err)
	}
	var declined domain.JobRequest
	if err := db.First(&declined, "job_request_id = ?", declineID).Error; err != nil {
		t.Fatal(err)
	}
	if declined.Status != domain.JobRequestStatusDeclined || declined.DeclineReason == nil || *declined.DeclineReason != reason || declined.DeclineNote == nil || *declined.DeclineNote != note {
		t.Fatalf("decline details were not persisted: %+v", declined)
	}
}

func TestJobRequestDeclineDetailsAreConstrainedByTheDatabase(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	creatorID := createIntegrationCreator(t, db, "job-request-constraint-creator")
	marketerID := createIntegrationMarketer(t, db, "job-request-constraint-marketer")
	surveyID := createIntegrationSurvey(t, db, creatorID, "constraint")
	_, requestID := createIntegrationJobRequest(t, db, creatorID, marketerID, surveyID)

	for name, statement := range map[string]string{
		"unknown_reason":          `UPDATE job_requests SET request_status = 'Declined', decline_reason = 'Too far' WHERE job_request_id = ?`,
		"details_on_non_declined": `UPDATE job_requests SET decline_note = 'note' WHERE job_request_id = ?`,
	} {
		err := db.Transaction(func(tx *gorm.DB) error {
			return tx.Exec(statement, requestID).Error
		})
		if err == nil {
			t.Fatalf("%s: expected the database to reject the update", name)
		}
	}
}
