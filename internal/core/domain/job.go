package domain

import (
	"github.com/shopspring/decimal"
	"time"
)

type JobStatus string

const (
	JobStatusPending    JobStatus = "Pending"
	JobStatusAccepted   JobStatus = "Accepted"
	JobStatusInProgress JobStatus = "In Progress"
	JobStatusCompleted  JobStatus = "Completed"
	JobStatusCancelled  JobStatus = "Cancelled"
)

type Job struct {
	Title           string           `gorm:"column:title"`
	Description     string           `gorm:"column:description"`
	Budget          *decimal.Decimal `gorm:"column:budget;type:numeric(10,2)"`
	Deadline        *time.Time       `gorm:"column:deadline;type:date"`
	JobID           int32            `gorm:"column:job_id;primaryKey;autoIncrement"`
	UserID          int32            `gorm:"column:user_id;not null"`
	AcceptedOfferID *int32           `gorm:"column:accepted_offer_id;uniqueIndex"`
	JobStatus       JobStatus        `gorm:"column:job_status;type:varchar(20);not null"`
	CreatedAt       time.Time        `gorm:"column:created_at;type:timestamp;not null"`

	Creator       *Creator     `gorm:"foreignKey:UserID;references:UserID"`
	AcceptedOffer *Offer       `gorm:"foreignKey:JobID,AcceptedOfferID;references:JobID,OfferID"`
	Offers        []Offer      `gorm:"foreignKey:JobID;references:JobID"`
	Surveys       []Survey     `gorm:"many2many:is_used_in;joinForeignKey:JobID;joinReferences:SurveyID"`
	Attachments   []Attachment `gorm:"foreignKey:JobID;references:JobID"`
	Payment       *Payment     `gorm:"foreignKey:JobID;references:JobID"`
	Review        *Review      `gorm:"foreignKey:JobID;references:JobID"`
}

func (Job) TableName() string { return "jobs" }

// CreatorJobSummary contains the small set of job fields returned to its owner.
type CreatorJobSummary struct {
	Title             string
	Description       string
	Budget            *decimal.Decimal
	Deadline          *time.Time
	EffectiveDeadline *time.Time
	JobID             int32              `gorm:"column:job_id"`
	JobStatus         JobStatus          `gorm:"column:job_status"`
	CreatedAt         time.Time          `gorm:"column:created_at"`
	Surveys           []JobSurveySummary `gorm:"-"`
}

type JobSurveySummary struct {
	SurveyID int32  `gorm:"column:survey_id"`
	Title    string `gorm:"column:title"`
}

type JobSurvey struct {
	JobID    int32 `gorm:"column:job_id;primaryKey"`
	SurveyID int32 `gorm:"column:survey_id;primaryKey"`

	Job    *Job    `gorm:"foreignKey:JobID;references:JobID"`
	Survey *Survey `gorm:"foreignKey:SurveyID;references:SurveyID"`
}

func (JobSurvey) TableName() string { return "is_used_in" }

type JobRequestStatus string

const (
	JobRequestStatusPending  JobRequestStatus = "Pending"
	JobRequestStatusAccepted JobRequestStatus = "Accepted"
	JobRequestStatusDeclined JobRequestStatus = "Declined"
	JobRequestStatusClosed   JobRequestStatus = "Closed"
)

// IsValid reports whether the status is one the job_requests table accepts.
func (s JobRequestStatus) IsValid() bool {
	switch s {
	case JobRequestStatusPending, JobRequestStatusAccepted, JobRequestStatusDeclined, JobRequestStatusClosed:
		return true
	}
	return false
}

// JobRequestDeclineReasons lists the optional reasons a Marketer may give when
// declining. They match the chk_job_request_decline_reason constraint.
var JobRequestDeclineReasons = []string{
	"Schedule conflict",
	"Budget too low",
	"Target audience mismatch",
	"Fully booked",
}

// MaxJobRequestDeclineNoteLength limits the optional decline note.
const MaxJobRequestDeclineNoteLength = 300

// JobRequest records a Creator's invitation to a Marketer. Accepting this
// invitation is separate from accepting a later offer for the job.
type JobRequest struct {
	JobRequestID  int32            `gorm:"column:job_request_id;primaryKey;autoIncrement"`
	JobID         int32            `gorm:"column:job_id;not null"`
	MarketerID    int32            `gorm:"column:marketer_id;not null"`
	Status        JobRequestStatus `gorm:"column:request_status;type:varchar(20);not null"`
	CreatedAt     time.Time        `gorm:"column:created_at;type:timestamp;not null"`
	RespondedAt   *time.Time       `gorm:"column:responded_at;type:timestamp"`
	DeclineReason *string          `gorm:"column:decline_reason;type:varchar(50)"`
	DeclineNote   *string          `gorm:"column:decline_note;type:varchar(300)"`

	Job      *Job      `gorm:"foreignKey:JobID;references:JobID"`
	Marketer *Marketer `gorm:"foreignKey:MarketerID;references:UserID"`
}

func (JobRequest) TableName() string { return "job_requests" }

// SentJobRequest is an invitation history row with only the Marketer's display
// name added. Contact and profile data are deliberately excluded.
type SentJobRequest struct {
	JobRequestID int32            `gorm:"column:job_request_id"`
	JobID        int32            `gorm:"column:job_id"`
	MarketerID   int32            `gorm:"column:marketer_id"`
	Status       JobRequestStatus `gorm:"column:request_status"`
	CreatedAt    time.Time        `gorm:"column:created_at"`
	RespondedAt  *time.Time       `gorm:"column:responded_at"`
	MarketerName string           `gorm:"column:marketer_name"`
}

// EarliestSurveyDeadline returns the earliest deadline among a job's surveys,
// which is the latest date work for the job can be delivered. It returns nil
// when no linked survey has a deadline.
func EarliestSurveyDeadline(surveys []Survey) *time.Time {
	var earliest *time.Time
	for index := range surveys {
		deadline := surveys[index].Deadline
		if deadline != nil && (earliest == nil || deadline.Before(*earliest)) {
			earliest = deadline
		}
	}
	return earliest
}

// EffectiveJobDeadline preserves the survey delivery limit while allowing a
// Creator to request an earlier date. Both workspaces and offer validation use it.
func EffectiveJobDeadline(deadline *time.Time, surveys []Survey) *time.Time {
	earliest := EarliestSurveyDeadline(surveys)
	if deadline != nil && (earliest == nil || deadline.Before(*earliest)) {
		return deadline
	}
	return earliest
}
