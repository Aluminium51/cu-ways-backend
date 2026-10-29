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
	if err := db.Model(&domain.JobRequest{}).
		Where("job_request_id = ?", retried[0].JobRequestID).
		Update("request_status", domain.JobRequestStatusAccepted).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateRequests(ctx, creatorID, job.JobID, []int32{marketerID}, createdAt.Add(2*time.Minute)); !errors.Is(err, domain.ErrJobRequestAlreadyActive) {
		t.Fatalf("expected an Accepted invitation to remain active, got %v", err)
	}
	if err := db.Model(&domain.JobRequest{}).
		Where("job_request_id = ?", retried[0].JobRequestID).
		Update("request_status", domain.JobRequestStatusClosed).Error; err != nil {
		t.Fatal(err)
	}
	closedRetry, err := repo.CreateRequests(ctx, creatorID, job.JobID, []int32{marketerID}, createdAt.Add(3*time.Minute))
	if err != nil {
		t.Fatalf("expected a Closed invitation to allow a later invitation, got %v", err)
	}
	if len(closedRetry) != 1 || closedRetry[0].Status != domain.JobRequestStatusPending {
		t.Fatalf("expected a new Pending row after a Closed invitation, got %+v", closedRetry)
	}
}

func TestJobRepositoryCreatorReadsAreScopedAndReturnSortedNamedHistory(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	ctx := context.Background()
	creatorID := createIntegrationCreator(t, db, "job-read-owner")
	otherCreatorID := createIntegrationCreator(t, db, "job-read-other")
	emptyCreatorID := createIntegrationCreator(t, db, "job-read-empty")
	firstSurvey := createIntegrationSurvey(t, db, creatorID, "read-first")
	secondSurvey := createIntegrationSurvey(t, db, creatorID, "read-second")
	otherSurvey := createIntegrationSurvey(t, db, otherCreatorID, "read-other")
	repo := NewJobRepository(db)
	time1 := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	time2 := time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC)
	time3 := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)

	olderJob, err := repo.CreateWithSurveys(ctx, &domain.Job{UserID: creatorID, JobStatus: domain.JobStatusPending, CreatedAt: time1}, []int32{secondSurvey, firstSurvey})
	if err != nil {
		t.Fatal(err)
	}
	firstTieJob, err := repo.CreateWithSurveys(ctx, &domain.Job{UserID: creatorID, JobStatus: domain.JobStatusAccepted, CreatedAt: time2}, []int32{firstSurvey})
	if err != nil {
		t.Fatal(err)
	}
	secondTieJob, err := repo.CreateWithSurveys(ctx, &domain.Job{UserID: creatorID, JobStatus: domain.JobStatusInProgress, CreatedAt: time2}, []int32{secondSurvey})
	if err != nil {
		t.Fatal(err)
	}
	foreignJob, err := repo.CreateWithSurveys(ctx, &domain.Job{UserID: otherCreatorID, JobStatus: domain.JobStatusCompleted, CreatedAt: time3}, []int32{otherSurvey})
	if err != nil {
		t.Fatal(err)
	}

	jobs, err := repo.ListJobsByCreator(ctx, creatorID)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 3 || jobs[0].JobID != secondTieJob.JobID || jobs[1].JobID != firstTieJob.JobID || jobs[2].JobID != olderJob.JobID {
		t.Fatalf("expected only owned jobs ordered by created_at/job_id descending, got %+v (foreign job %d)", jobs, foreignJob.JobID)
	}
	if jobs[2].JobStatus != domain.JobStatusPending || !jobs[2].CreatedAt.Equal(time1) || len(jobs[2].Surveys) != 2 || jobs[2].Surveys[0].SurveyID != firstSurvey || jobs[2].Surveys[1].SurveyID != secondSurvey || jobs[2].Surveys[0].Title != "Survey read-first" || jobs[2].Surveys[1].Title != "Survey read-second" {
		t.Fatalf("expected survey summaries ordered by ID with titles, got %+v", jobs[2])
	}
	emptyJobs, err := repo.ListJobsByCreator(ctx, emptyCreatorID)
	if err != nil {
		t.Fatal(err)
	}
	if emptyJobs == nil || len(emptyJobs) != 0 {
		t.Fatalf("expected an empty non-nil job list, got %#v", emptyJobs)
	}

	detail, err := repo.GetJobByCreator(ctx, creatorID, olderJob.JobID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.JobID != olderJob.JobID || len(detail.Surveys) != 2 || detail.Surveys[0].SurveyID != firstSurvey || detail.Surveys[1].SurveyID != secondSurvey {
		t.Fatalf("unexpected owned job detail: %+v", detail)
	}
	if _, err := repo.GetJobByCreator(ctx, creatorID, foreignJob.JobID); !errors.Is(err, domain.ErrJobForbidden) {
		t.Fatalf("expected foreign job detail to be forbidden, got %v", err)
	}
	if _, err := repo.GetJobByCreator(ctx, creatorID, int32(2147483647)); !errors.Is(err, domain.ErrJobNotFound) {
		t.Fatalf("expected missing job detail to be not found, got %v", err)
	}

	marketerA := createIntegrationMarketerNamed(t, db, "Avery Marketer", "job-read-a")
	marketerB := createIntegrationMarketerNamed(t, db, "Blake Marketer", "job-read-b")
	marketerC := createIntegrationMarketerNamed(t, db, "Casey Marketer", "job-read-c")
	marketerD := createIntegrationMarketerNamed(t, db, "Drew Marketer", "job-read-d")
	requestA, err := repo.CreateRequests(ctx, creatorID, olderJob.JobID, []int32{marketerA}, time1)
	if err != nil {
		t.Fatal(err)
	}
	requestB, err := repo.CreateRequests(ctx, creatorID, olderJob.JobID, []int32{marketerB}, time2)
	if err != nil {
		t.Fatal(err)
	}
	requestC, err := repo.CreateRequests(ctx, creatorID, olderJob.JobID, []int32{marketerC}, time2)
	if err != nil {
		t.Fatal(err)
	}
	requestD, err := repo.CreateRequests(ctx, creatorID, olderJob.JobID, []int32{marketerD}, time3)
	if err != nil {
		t.Fatal(err)
	}
	respondedAt := time3.Add(time.Hour)
	for _, update := range []struct {
		id     int32
		status domain.JobRequestStatus
	}{{requestA[0].JobRequestID, domain.JobRequestStatusAccepted}, {requestB[0].JobRequestID, domain.JobRequestStatusDeclined}, {requestC[0].JobRequestID, domain.JobRequestStatusClosed}} {
		if err := db.Model(&domain.JobRequest{}).Where("job_request_id = ?", update.id).Updates(map[string]any{
			"request_status": update.status,
			"responded_at":   respondedAt,
		}).Error; err != nil {
			t.Fatal(err)
		}
	}

	history, err := repo.ListJobRequestsByCreator(ctx, creatorID, olderJob.JobID)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 4 {
		t.Fatalf("expected all four invitation statuses, got %+v", history)
	}
	expected := []struct {
		requestID  int32
		marketerID int32
		name       string
		status     domain.JobRequestStatus
	}{{requestD[0].JobRequestID, marketerD, "Drew Marketer", domain.JobRequestStatusPending}, {requestC[0].JobRequestID, marketerC, "Casey Marketer", domain.JobRequestStatusClosed}, {requestB[0].JobRequestID, marketerB, "Blake Marketer", domain.JobRequestStatusDeclined}, {requestA[0].JobRequestID, marketerA, "Avery Marketer", domain.JobRequestStatusAccepted}}
	for index, item := range expected {
		actual := history[index]
		if actual.JobRequestID != item.requestID || actual.JobID != olderJob.JobID || actual.MarketerID != item.marketerID || actual.MarketerName != item.name || actual.Status != item.status {
			t.Errorf("history[%d] expected %+v, got %+v", index, item, actual)
		}
		if item.status == domain.JobRequestStatusPending {
			if actual.RespondedAt != nil {
				t.Errorf("history[%d] expected pending responded_at to be nil, got %s", index, actual.RespondedAt)
			}
		} else if actual.RespondedAt == nil || !actual.RespondedAt.Equal(respondedAt) {
			t.Errorf("history[%d] expected responded_at %s, got %v", index, respondedAt, actual.RespondedAt)
		}
	}
	if _, err := repo.ListJobRequestsByCreator(ctx, creatorID, foreignJob.JobID); !errors.Is(err, domain.ErrJobForbidden) {
		t.Fatalf("expected foreign job invitation history to be forbidden, got %v", err)
	}
	if _, err := repo.ListJobRequestsByCreator(ctx, creatorID, int32(2147483647)); !errors.Is(err, domain.ErrJobNotFound) {
		t.Fatalf("expected missing job invitation history to be not found, got %v", err)
	}
	if emptyHistory, err := repo.ListJobRequestsByCreator(ctx, creatorID, firstTieJob.JobID); err != nil || emptyHistory == nil || len(emptyHistory) != 0 {
		t.Fatalf("expected empty invitation history array, got history=%#v err=%v", emptyHistory, err)
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

func TestJobSummaryCountsOnlyPendingOffers(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	ctx := context.Background()
	creatorID := createIntegrationCreator(t, db, "count-owner")
	otherID := createIntegrationCreator(t, db, "count-other")
	repo := NewJobRepository(db)
	survey := createIntegrationSurvey(t, db, creatorID, "count")
	job, err := repo.CreateWithSurveys(ctx, &domain.Job{UserID: creatorID, JobStatus: domain.JobStatusPending, CreatedAt: time.Now().UTC()}, []int32{survey})
	if err != nil {
		t.Fatal(err)
	}
	statuses := []domain.OfferStatus{domain.OfferStatusPending, domain.OfferStatusPending, domain.OfferStatusWithdrawn, domain.OfferStatusRejected}
	for i, status := range statuses {
		marketer := createIntegrationMarketerNamed(t, db, fmt.Sprintf("Count marketer %d", i), fmt.Sprintf("count-%d", i))
		offer := submitIntegrationOffer(t, db, creatorID, job.JobID, marketer, "100")
		if err := db.Model(&domain.Offer{}).Where("offer_id = ?", offer.OfferID).Update("offer_status", status).Error; err != nil {
			t.Fatal(err)
		}
	}
	summaries, err := repo.ListJobsByCreator(ctx, creatorID)
	if err != nil || len(summaries) != 1 || summaries[0].PendingOfferCount != 2 {
		t.Fatalf("unexpected list count: %+v, %v", summaries, err)
	}
	summary, err := repo.GetJobByCreator(ctx, creatorID, job.JobID)
	if err != nil || summary.PendingOfferCount != 2 {
		t.Fatalf("unexpected detail count: %+v, %v", summary, err)
	}
	foreign, err := repo.ListJobsByCreator(ctx, otherID)
	if err != nil || len(foreign) != 0 {
		t.Fatalf("foreign jobs leaked: %+v, %v", foreign, err)
	}
}
