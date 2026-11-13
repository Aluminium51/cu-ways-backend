package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
)

type fakeReviewRepository struct {
	creatorID int32
	jobID     int32
	review    *domain.Review
	current   *domain.Review
	calls     int
	err       error

	listQuery ports.MarketerReviewQuery
	listCalls int
	listPage  ports.MarketerReviewPage
	listErr   error
}

func (f *fakeReviewRepository) FindForCreatorJob(_ context.Context, creatorID, jobID int32) (*domain.Review, error) {
	f.calls++
	f.creatorID, f.jobID = creatorID, jobID
	if f.err != nil {
		return nil, f.err
	}
	return f.current, nil
}

func (f *fakeReviewRepository) CreateForCreatorJob(_ context.Context, creatorID, jobID int32, review *domain.Review) (*domain.Review, error) {
	f.calls++
	f.creatorID, f.jobID, f.review = creatorID, jobID, review
	if f.err != nil {
		return nil, f.err
	}
	review.ReviewID = 41
	return review, nil
}

func (f *fakeReviewRepository) ListForMarketer(_ context.Context, query ports.MarketerReviewQuery) (ports.MarketerReviewPage, error) {
	f.listCalls++
	f.listQuery = query
	if f.listErr != nil {
		return ports.MarketerReviewPage{}, f.listErr
	}
	return f.listPage, nil
}

func TestReviewServiceSubmitsAsAuthenticatedCreator(t *testing.T) {
	repo := &fakeReviewRepository{}
	service := NewReviewService(repo, &fakeMembershipRepository{creator: true})
	comment := "Clear communication"
	before := time.Now().UTC()
	review, err := service.Submit(context.Background(), Actor{UserID: 8}, 12, SubmitReviewInput{Rating: 5, Comment: &comment})
	if err != nil {
		t.Fatal(err)
	}
	if repo.calls != 1 || repo.creatorID != 8 || repo.jobID != 12 || repo.review.Rating != 5 || repo.review.Comment == nil || *repo.review.Comment != comment {
		t.Fatalf("wrong review submission: %+v", repo)
	}
	if review.ReviewID != 41 || review.CreatedAt.Before(before) || review.CreatedAt.After(time.Now().UTC()) {
		t.Fatalf("review ID or timestamp was not set: %+v", review)
	}
}

func TestReviewServiceRejectsInvalidSubmissionBeforePersistence(t *testing.T) {
	for _, tc := range []struct {
		name    string
		creator bool
		actorID int32
		jobID   int32
		rating  int32
		want    error
	}{
		{name: "not_creator", actorID: 8, jobID: 12, rating: 5, want: domain.ErrCreatorRequired},
		{name: "invalid_job", creator: true, actorID: 8, rating: 5, want: domain.ErrInvalidJob},
		{name: "zero_rating", creator: true, actorID: 8, jobID: 12, want: domain.ErrInvalidReview},
		{name: "rating_above_five", creator: true, actorID: 8, jobID: 12, rating: 6, want: domain.ErrInvalidReview},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeReviewRepository{}
			service := NewReviewService(repo, &fakeMembershipRepository{creator: tc.creator})
			_, err := service.Submit(context.Background(), Actor{UserID: tc.actorID}, tc.jobID, SubmitReviewInput{Rating: tc.rating})
			if !errors.Is(err, tc.want) || repo.calls != 0 {
				t.Fatalf("expected %v without persistence, got %v and %d calls", tc.want, err, repo.calls)
			}
		})
	}
}

func TestReviewServicePassesThroughJobAndDuplicateErrors(t *testing.T) {
	for _, want := range []error{domain.ErrJobNotFound, domain.ErrJobForbidden, domain.ErrJobNotCompleted, domain.ErrReviewAlreadyExists} {
		repo := &fakeReviewRepository{err: want}
		service := NewReviewService(repo, &fakeMembershipRepository{creator: true})
		_, err := service.Submit(context.Background(), Actor{UserID: 8}, 12, SubmitReviewInput{Rating: 3})
		if !errors.Is(err, want) {
			t.Fatalf("expected %v, got %v", want, err)
		}
	}
}

func TestReviewServiceCurrentScopesToCreatorJob(t *testing.T) {
	current := &domain.Review{ReviewID: 41, JobID: 12, Rating: 4}
	repo := &fakeReviewRepository{current: current}
	review, err := NewReviewService(repo, &fakeMembershipRepository{creator: true}).Current(context.Background(), Actor{UserID: 8}, 12)
	if err != nil || review != current || repo.creatorID != 8 || repo.jobID != 12 {
		t.Fatalf("got %+v, %v; repo %+v", review, err, repo)
	}
	none, err := NewReviewService(&fakeReviewRepository{}, &fakeMembershipRepository{creator: true}).Current(context.Background(), Actor{UserID: 8}, 12)
	if err != nil || none != nil {
		t.Fatalf("expected no review, got %+v, %v", none, err)
	}
	failing := &fakeReviewRepository{err: domain.ErrJobForbidden}
	if _, err := NewReviewService(failing, &fakeMembershipRepository{creator: true}).Current(context.Background(), Actor{UserID: 8}, 12); !errors.Is(err, domain.ErrJobForbidden) {
		t.Fatalf("expected repository error, got %v", err)
	}
}

func TestReviewServiceCurrentRejectsBeforeReading(t *testing.T) {
	for _, tc := range []struct {
		name       string
		creator    bool
		actor, job int32
		want       error
	}{
		{"not_creator", false, 8, 12, domain.ErrCreatorRequired},
		{"invalid_actor", true, 0, 12, domain.ErrCreatorRequired},
		{"invalid_job", true, 8, 0, domain.ErrInvalidJob},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeReviewRepository{}
			_, err := NewReviewService(repo, &fakeMembershipRepository{creator: tc.creator}).Current(context.Background(), Actor{UserID: tc.actor}, tc.job)
			if !errors.Is(err, tc.want) || repo.calls != 0 {
				t.Fatalf("got %v, calls %d", err, repo.calls)
			}
		})
	}
}
