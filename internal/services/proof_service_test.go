package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
)

type fakeProofRepository struct {
	calls      int
	userID     int32
	jobID      int32
	attachment *domain.Attachment
	list       []domain.Attachment
	err        error
}

func (f *fakeProofRepository) CreateForAcceptedMarketer(_ context.Context, marketerID, jobID int32, a *domain.Attachment) error {
	f.calls++
	f.userID, f.jobID, f.attachment = marketerID, jobID, a
	if f.err != nil {
		return f.err
	}
	a.AttachmentID = 5
	a.JobID = jobID
	return nil
}

func (f *fakeProofRepository) ListForParticipant(_ context.Context, userID, jobID int32) ([]domain.Attachment, error) {
	f.calls++
	f.userID, f.jobID = userID, jobID
	return f.list, f.err
}

func TestProofServiceAddsTrimmedProofWithServerTimestamp(t *testing.T) {
	for _, attachmentType := range []domain.AttachmentType{"", domain.AttachmentTypeProof} {
		repo := &fakeProofRepository{}
		before := time.Now().UTC()
		a, err := NewProofService(repo, &fakeMembershipRepository{marketer: true}).Add(context.Background(), Actor{UserID: 9}, 12, AddProofInput{URL: "  https://forms.gle/abc123  ", AttachmentType: attachmentType})
		if err != nil {
			t.Fatal(err)
		}
		if repo.userID != 9 || repo.jobID != 12 || a.AttachmentID != 5 || a.JobID != 12 || a.AttachmentType != domain.AttachmentTypeProof || a.URL != "https://forms.gle/abc123" || a.UploadAt.Location() != time.UTC || a.UploadAt.Before(before) || a.UploadAt.After(time.Now().UTC()) {
			t.Fatalf("unexpected attachment: %+v", a)
		}
	}
}

func TestProofServiceRejectsInvalidInputWithoutWriting(t *testing.T) {
	for _, tc := range []struct {
		name           string
		marketer       bool
		actor, job     int32
		url            string
		attachmentType domain.AttachmentType
		want           error
	}{
		{"not_marketer", false, 9, 12, "https://example.com", "", domain.ErrMarketerRequired},
		{"invalid_actor", true, 0, 12, "https://example.com", "", domain.ErrMarketerRequired},
		{"invalid_job", true, 9, 0, "https://example.com", "", domain.ErrInvalidJob},
		{"unknown_type", true, 9, 12, "https://example.com", "Invoice", domain.ErrInvalidAttachmentType},
		{"lowercase_type", true, 9, 12, "https://example.com", "proof", domain.ErrInvalidAttachmentType},
		{"brief_type", true, 9, 12, "https://example.com", domain.AttachmentTypeBrief, domain.ErrInvalidAttachmentType},
		{"other_type", true, 9, 12, "https://example.com", domain.AttachmentTypeOther, domain.ErrInvalidAttachmentType},
		{"blank_url", true, 9, 12, "   ", "", domain.ErrInvalidProofURL},
		{"not_a_url", true, 9, 12, "not a url", "", domain.ErrInvalidProofURL},
		{"ftp_url", true, 9, 12, "ftp://example.com/file", "", domain.ErrInvalidProofURL},
		{"no_host", true, 9, 12, "https://", "", domain.ErrInvalidProofURL},
		{"javascript_url", true, 9, 12, "javascript:alert(1)", "", domain.ErrInvalidProofURL},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeProofRepository{}
			_, err := NewProofService(repo, &fakeMembershipRepository{marketer: tc.marketer}).Add(context.Background(), Actor{UserID: tc.actor}, tc.job, AddProofInput{URL: tc.url, AttachmentType: tc.attachmentType})
			if !errors.Is(err, tc.want) || repo.calls != 0 {
				t.Fatalf("expected %v without a write, got %v (calls=%d)", tc.want, err, repo.calls)
			}
		})
	}
}

func TestProofServicePassesRepositoryErrorsThrough(t *testing.T) {
	repo := &fakeProofRepository{err: domain.ErrJobNotInProgress}
	_, err := NewProofService(repo, &fakeMembershipRepository{marketer: true}).Add(context.Background(), Actor{UserID: 9}, 12, AddProofInput{URL: "https://example.com"})
	if !errors.Is(err, domain.ErrJobNotInProgress) {
		t.Fatalf("expected ErrJobNotInProgress, got %v", err)
	}
}

func TestProofServiceListsForParticipant(t *testing.T) {
	repo := &fakeProofRepository{list: []domain.Attachment{{AttachmentID: 2}, {AttachmentID: 1}}}
	service := NewProofService(repo, &fakeMembershipRepository{})
	items, err := service.List(context.Background(), Actor{UserID: 8}, 12)
	if err != nil || len(items) != 2 || repo.userID != 8 || repo.jobID != 12 {
		t.Fatalf("unexpected list: %+v %v", items, err)
	}
	repo.calls = 0
	if _, err := service.List(context.Background(), Actor{UserID: 8}, 0); !errors.Is(err, domain.ErrInvalidJob) || repo.calls != 0 {
		t.Fatalf("expected ErrInvalidJob without a read, got %v", err)
	}
	if _, err := service.List(context.Background(), Actor{}, 12); !errors.Is(err, domain.ErrJobForbidden) || repo.calls != 0 {
		t.Fatalf("expected ErrJobForbidden without a read, got %v", err)
	}
}
