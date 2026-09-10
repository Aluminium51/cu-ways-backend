package services

import (
	"context"
	"errors"
	"testing"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
)

type fakeStatisticsRepository struct {
	result domain.MarketerStatistics
	userID int32
	calls  int
}

func (f *fakeStatisticsRepository) GetByMarketer(_ context.Context, userID int32) (domain.MarketerStatistics, error) {
	f.calls++
	f.userID = userID
	return f.result, nil
}

func TestStatisticsServiceReturnsAuthenticatedMarketerStatistics(t *testing.T) {
	expected := domain.MarketerStatistics{TotalCompletedJobs: 2}
	repo := &fakeStatisticsRepository{result: expected}
	service := NewStatisticsService(repo, &fakeMembershipRepository{marketer: true})

	result, err := service.GetMyStatistics(context.Background(), Actor{UserID: 17})
	if err != nil {
		t.Fatal(err)
	}
	if repo.calls != 1 || repo.userID != 17 || result.TotalCompletedJobs != expected.TotalCompletedJobs {
		t.Fatalf("unexpected statistics lookup: calls=%d user=%d result=%+v", repo.calls, repo.userID, result)
	}
}

func TestStatisticsServiceRejectsNonMarketer(t *testing.T) {
	repo := &fakeStatisticsRepository{}
	service := NewStatisticsService(repo, &fakeMembershipRepository{})

	_, err := service.GetMyStatistics(context.Background(), Actor{UserID: 17})
	if !errors.Is(err, domain.ErrMarketerRequired) {
		t.Fatalf("expected marketer requirement, got %v", err)
	}
	if repo.calls != 0 {
		t.Fatalf("expected no statistics lookup, got %d calls", repo.calls)
	}
}
