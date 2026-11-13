package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"gorm.io/gorm"
)

func newProof(url string) *domain.Attachment {
	return &domain.Attachment{AttachmentType: domain.AttachmentTypeProof, URL: url, UploadAt: time.Now().UTC().Truncate(time.Microsecond)}
}

func countProofs(t *testing.T, db *gorm.DB, jobID int32) int64 {
	t.Helper()
	var count int64
	if err := db.Model(&domain.Attachment{}).Where("job_id = ? AND attachment_type = ?", jobID, domain.AttachmentTypeProof).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	return count
}

func TestProofRepositoryStoresProofForAcceptedMarketer(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	creator := createIntegrationCreator(t, db, "proof-success")
	marketer := createIntegrationMarketer(t, db, "proof-success")
	jobID := createStatisticsJob(t, db, creator, marketer, domain.JobStatusInProgress)

	input := newProof("https://forms.gle/abc123")
	if err := NewProofRepository(db).CreateForAcceptedMarketer(context.Background(), marketer, jobID, input); err != nil {
		t.Fatal(err)
	}
	var stored domain.Attachment
	if err := db.First(&stored, "attachment_id = ?", input.AttachmentID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.JobID != jobID || stored.AttachmentType != domain.AttachmentTypeProof || stored.URL != "https://forms.gle/abc123" || !stored.UploadAt.Equal(input.UploadAt) {
		t.Fatalf("unexpected stored proof: %+v", stored)
	}
	var job domain.Job
	if err := db.First(&job, "job_id = ?", jobID).Error; err != nil {
		t.Fatal(err)
	}
	if job.JobStatus != domain.JobStatusInProgress {
		t.Fatalf("saving proof must not change the job status, got %s", job.JobStatus)
	}
}

func TestProofRepositoryRejectsWithoutWriting(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	creator := createIntegrationCreator(t, db, "proof-errors")
	marketer := createIntegrationMarketer(t, db, "proof-errors")
	otherMarketer := createIntegrationMarketer(t, db, "proof-other")
	for _, tc := range []struct {
		name    string
		status  domain.JobStatus
		actor   int32
		noOffer bool
		// offerStatus moves the accepted offer away from Accepted.
		offerStatus domain.OfferStatus
		want        error
	}{
		// A Pending job has not hired anyone yet (the fixture leaves
		// accepted_offer_id NULL), so the hire check refuses it first.
		{name: "pending_job", status: domain.JobStatusPending, actor: marketer, want: domain.ErrJobForbidden},
		{name: "accepted_job", status: domain.JobStatusAccepted, actor: marketer, want: domain.ErrJobNotInProgress},
		{name: "completed_job", status: domain.JobStatusCompleted, actor: marketer, want: domain.ErrJobNotInProgress},
		{name: "cancelled_job", status: domain.JobStatusCancelled, actor: marketer, want: domain.ErrJobNotInProgress},
		{name: "other_marketer", status: domain.JobStatusInProgress, actor: otherMarketer, want: domain.ErrJobForbidden},
		{name: "job_creator", status: domain.JobStatusInProgress, actor: creator, want: domain.ErrJobForbidden},
		{name: "no_accepted_offer", status: domain.JobStatusInProgress, actor: marketer, noOffer: true, want: domain.ErrJobForbidden},
		{name: "accepted_offer_withdrawn", status: domain.JobStatusInProgress, actor: marketer, offerStatus: domain.OfferStatusWithdrawn, want: domain.ErrAcceptedOfferMismatch},
		{name: "accepted_offer_rejected", status: domain.JobStatusInProgress, actor: marketer, offerStatus: domain.OfferStatusRejected, want: domain.ErrAcceptedOfferMismatch},
	} {
		t.Run(tc.name, func(t *testing.T) {
			jobID := createStatisticsJob(t, db, creator, marketer, tc.status)
			if tc.noOffer {
				if err := db.Exec("UPDATE jobs SET accepted_offer_id = NULL WHERE job_id = ?", jobID).Error; err != nil {
					t.Fatal(err)
				}
			}
			if tc.offerStatus != "" {
				if err := db.Exec("UPDATE offers SET offer_status = ? WHERE offer_id = (SELECT accepted_offer_id FROM jobs WHERE job_id = ?)", tc.offerStatus, jobID).Error; err != nil {
					t.Fatal(err)
				}
			}
			before := countProofs(t, db, jobID)
			err := NewProofRepository(db).CreateForAcceptedMarketer(context.Background(), tc.actor, jobID, newProof("https://example.com/proof"))
			if !errors.Is(err, tc.want) {
				t.Fatalf("expected %v, got %v", tc.want, err)
			}
			if after := countProofs(t, db, jobID); after != before {
				t.Fatalf("failed request wrote a proof: before=%d after=%d", before, after)
			}
		})
	}
	if err := NewProofRepository(db).CreateForAcceptedMarketer(context.Background(), marketer, 0, newProof("https://example.com")); !errors.Is(err, domain.ErrJobNotFound) {
		t.Fatalf("missing Job: %v", err)
	}
}

func TestProofRepositoryDatabaseRejectsUnknownAttachmentType(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	creator := createIntegrationCreator(t, db, "proof-type")
	marketer := createIntegrationMarketer(t, db, "proof-type")
	jobID := createStatisticsJob(t, db, creator, marketer, domain.JobStatusInProgress)

	// Bypasses the service check on purpose: the chk_attachment_type
	// constraint must reject the row on its own.
	invalid := newProof("https://example.com/proof")
	invalid.AttachmentType = "Invoice"
	err := NewProofRepository(db).CreateForAcceptedMarketer(context.Background(), marketer, jobID, invalid)
	if !errors.Is(err, domain.ErrInvalidAttachmentType) {
		t.Fatalf("expected ErrInvalidAttachmentType from the database, got %v", err)
	}
	var total int64
	if err := db.Model(&domain.Attachment{}).Where("job_id = ?", jobID).Count(&total).Error; err != nil {
		t.Fatal(err)
	}
	if total != 0 {
		t.Fatalf("the rejected row was stored: %d attachments", total)
	}
}

func TestProofRepositoryListsNewestFirstForParticipants(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	creator := createIntegrationCreator(t, db, "proof-list")
	otherCreator := createIntegrationCreator(t, db, "proof-list-other")
	marketer := createIntegrationMarketer(t, db, "proof-list")
	otherMarketer := createIntegrationMarketer(t, db, "proof-list-other")
	jobID := createStatisticsJob(t, db, creator, marketer, domain.JobStatusInProgress)
	repo := NewProofRepository(db)

	older := newProof("https://example.com/older")
	older.UploadAt = older.UploadAt.Add(-time.Hour)
	newer := newProof("https://example.com/newer")
	for _, proof := range []*domain.Attachment{older, newer} {
		if err := repo.CreateForAcceptedMarketer(context.Background(), marketer, jobID, proof); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Exec(`INSERT INTO attachments (job_id, attachment_type, url, upload_at) VALUES (?, 'Brief', ?, ?)`, jobID, "https://example.com/brief", time.Now().UTC()).Error; err != nil {
		t.Fatal(err)
	}

	for _, viewer := range []int32{creator, marketer} {
		items, err := repo.ListForParticipant(context.Background(), viewer, jobID)
		if err != nil {
			t.Fatal(err)
		}
		if len(items) != 2 || items[0].URL != "https://example.com/newer" || items[1].URL != "https://example.com/older" {
			t.Fatalf("expected Proof rows only, newest first, got %+v", items)
		}
	}
	for _, outsider := range []int32{otherCreator, otherMarketer} {
		if _, err := repo.ListForParticipant(context.Background(), outsider, jobID); !errors.Is(err, domain.ErrJobForbidden) {
			t.Fatalf("expected ErrJobForbidden for an outsider, got %v", err)
		}
	}
	if err := db.Exec("UPDATE offers SET offer_status = ? WHERE offer_id = (SELECT accepted_offer_id FROM jobs WHERE job_id = ?)", domain.OfferStatusWithdrawn, jobID).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ListForParticipant(context.Background(), marketer, jobID); !errors.Is(err, domain.ErrJobForbidden) {
		t.Fatalf("a marketer whose accepted offer was withdrawn must not read Proof, got %v", err)
	}
	if items, err := repo.ListForParticipant(context.Background(), creator, jobID); err != nil || len(items) != 2 {
		t.Fatalf("the creator keeps access after the offer changes, got %+v %v", items, err)
	}
	if _, err := repo.ListForParticipant(context.Background(), creator, 0); !errors.Is(err, domain.ErrJobNotFound) {
		t.Fatalf("missing Job: %v", err)
	}

	emptyJob := createStatisticsJob(t, db, creator, marketer, domain.JobStatusInProgress)
	items, err := repo.ListForParticipant(context.Background(), creator, emptyJob)
	if err != nil || items == nil || len(items) != 0 {
		t.Fatalf("expected an empty, non-nil list, got %+v %v", items, err)
	}
}
