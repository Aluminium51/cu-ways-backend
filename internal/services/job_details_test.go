package services

import (
	"context"
	"errors"
	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"strings"
	"testing"
)

func TestJobDetailsValidateBeforeWritingAndPreserveExactCompensation(t *testing.T) {
	budget, deadline := "1200.50", "2030-10-28"
	repo := &fakeJobRepository{}
	service := NewJobService(repo, &fakeMembershipRepository{creator: true})
	job, err := service.Create(context.Background(), Actor{UserID: 8}, CreateJobInput{SurveyIDs: []int32{4}, JobDetailsInput: JobDetailsInput{Title: " Campus campaign ", Description: " Reach students ", Budget: &budget, Deadline: &deadline}})
	if err != nil || job.Title != "Campus campaign" || job.Description != "Reach students" || job.Budget == nil || job.Budget.StringFixed(2) != "1200.50" || job.Deadline == nil || job.Deadline.Format("2006-01-02") != deadline {
		t.Fatalf("unexpected saved details: %+v, %v", job, err)
	}
	for _, input := range []JobDetailsInput{{Title: strings.Repeat("x", 201)}, {Description: strings.Repeat("x", 5001)}, {Budget: textPointer("-1")}, {Budget: textPointer("12.345")}, {Deadline: textPointer("2030-02-30")}} {
		before := repo.createCalls
		_, err := service.Create(context.Background(), Actor{UserID: 8}, CreateJobInput{SurveyIDs: []int32{4}, JobDetailsInput: input})
		if !errors.Is(err, domain.ErrInvalidJob) || repo.createCalls != before {
			t.Fatalf("invalid details reached persistence: %+v, %v", input, err)
		}
	}
}

func textPointer(value string) *string { return &value }
