package domain

import (
	"errors"
	"testing"
)

func pendingJobWithOffer() (*Job, *Offer) {
	return &Job{JobID: 7, JobStatus: JobStatusPending}, &Offer{OfferID: 31, JobID: 7, UserID: 9, OfferStatus: OfferStatusPending}
}

func TestJobAcceptOfferHiresPendingOffer(t *testing.T) {
	job, offer := pendingJobWithOffer()
	if err := job.AcceptOffer(offer); err != nil {
		t.Fatal(err)
	}
	if job.JobStatus != JobStatusAccepted || job.AcceptedOfferID == nil || *job.AcceptedOfferID != 31 || offer.OfferStatus != OfferStatusAccepted {
		t.Fatalf("unexpected hire: job=%+v offer=%+v", job, offer)
	}
}

func TestJobAcceptOfferRefusesInvalidHires(t *testing.T) {
	accepted := int32(30)
	for _, tt := range []struct {
		name  string
		setup func(*Job, *Offer)
		want  error
	}{
		{"job_not_pending", func(j *Job, _ *Offer) { j.JobStatus = JobStatusCancelled }, ErrJobNotPending},
		{"job_already_hired", func(j *Job, _ *Offer) { j.AcceptedOfferID = &accepted }, ErrJobNotPending},
		{"offer_from_another_job", func(_ *Job, o *Offer) { o.JobID = 8 }, ErrOfferNotFound},
		{"offer_withdrawn", func(_ *Job, o *Offer) { o.OfferStatus = OfferStatusWithdrawn }, ErrOfferNotPending},
	} {
		t.Run(tt.name, func(t *testing.T) {
			job, offer := pendingJobWithOffer()
			tt.setup(job, offer)
			before, beforeOffer := *job, *offer
			if err := job.AcceptOffer(offer); !errors.Is(err, tt.want) {
				t.Fatalf("expected %v, got %v", tt.want, err)
			}
			if job.JobStatus != before.JobStatus || job.AcceptedOfferID != before.AcceptedOfferID || *offer != beforeOffer {
				t.Fatal("a refused hire changed the job or offer")
			}
		})
	}
}

func acceptedJobWithOffer() (*Job, *Offer) {
	offerID := int32(31)
	return &Job{JobID: 7, JobStatus: JobStatusAccepted, AcceptedOfferID: &offerID}, &Offer{OfferID: 31, JobID: 7, UserID: 9, OfferStatus: OfferStatusAccepted}
}

func TestJobStartMovesAcceptedJobToInProgress(t *testing.T) {
	job, offer := acceptedJobWithOffer()
	if err := job.Start(9, offer); err != nil {
		t.Fatal(err)
	}
	if job.JobStatus != JobStatusInProgress || offer.OfferStatus != OfferStatusAccepted {
		t.Fatalf("unexpected start: job=%+v offer=%+v", job, offer)
	}
}

func TestJobStartRefusesInvalidStarts(t *testing.T) {
	for _, tt := range []struct {
		name       string
		marketerID int32
		setup      func(*Job, *Offer) *Offer
		want       error
	}{
		{"not_hired_yet", 9, func(j *Job, _ *Offer) *Offer { j.JobStatus, j.AcceptedOfferID = JobStatusPending, nil; return nil }, ErrJobForbidden},
		{"another_marketer", 10, func(_ *Job, o *Offer) *Offer { return o }, ErrJobForbidden},
		// Ownership comes first, so another Marketer cannot learn the status.
		{"another_marketer_on_started_job", 10, func(j *Job, o *Offer) *Offer { j.JobStatus = JobStatusInProgress; return o }, ErrJobForbidden},
		{"offer_not_the_accepted_one", 9, func(_ *Job, o *Offer) *Offer { o.OfferID = 32; return o }, ErrJobForbidden},
		{"accepted_offer_withdrawn", 9, func(_ *Job, o *Offer) *Offer { o.OfferStatus = OfferStatusWithdrawn; return o }, ErrAcceptedOfferMismatch},
		{"already_started", 9, func(j *Job, o *Offer) *Offer { j.JobStatus = JobStatusInProgress; return o }, ErrJobNotAccepted},
		{"cancelled", 9, func(j *Job, o *Offer) *Offer { j.JobStatus = JobStatusCancelled; return o }, ErrJobNotAccepted},
	} {
		t.Run(tt.name, func(t *testing.T) {
			job, offer := acceptedJobWithOffer()
			accepted := tt.setup(job, offer)
			before := job.JobStatus
			if err := job.Start(tt.marketerID, accepted); !errors.Is(err, tt.want) {
				t.Fatalf("expected %v, got %v", tt.want, err)
			}
			if job.JobStatus != before {
				t.Fatalf("a refused start changed the job to %s", job.JobStatus)
			}
		})
	}
}
