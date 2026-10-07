package domain

import "time"

type JobStatus string

const (
	JobStatusPending    JobStatus = "Pending"
	JobStatusAccepted   JobStatus = "Accepted"
	JobStatusInProgress JobStatus = "In Progress"
	JobStatusCompleted  JobStatus = "Completed"
	JobStatusCancelled  JobStatus = "Cancelled"
)

type Job struct {
	JobID           int32     `gorm:"column:job_id;primaryKey;autoIncrement"`
	UserID          int32     `gorm:"column:user_id;not null"`
	AcceptedOfferID *int32    `gorm:"column:accepted_offer_id;uniqueIndex"`
	JobStatus       JobStatus `gorm:"column:job_status;type:varchar(20);not null"`
	CreatedAt       time.Time `gorm:"column:created_at;type:timestamp;not null"`

	Creator       *Creator     `gorm:"foreignKey:UserID;references:UserID"`
	AcceptedOffer *Offer       `gorm:"foreignKey:JobID,AcceptedOfferID;references:JobID,OfferID"`
	Offers        []Offer      `gorm:"foreignKey:JobID;references:JobID"`
	Surveys       []Survey     `gorm:"many2many:is_used_in;joinForeignKey:JobID;joinReferences:SurveyID"`
	Attachments   []Attachment `gorm:"foreignKey:JobID;references:JobID"`
	Payment       *Payment     `gorm:"foreignKey:JobID;references:JobID"`
	Review        *Review      `gorm:"foreignKey:JobID;references:JobID"`
}

func (Job) TableName() string { return "jobs" }

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

// JobRequest records a Creator's invitation to a Marketer. Accepting this
// invitation is separate from accepting a later offer for the job.
type JobRequest struct {
	JobRequestID int32            `gorm:"column:job_request_id;primaryKey;autoIncrement"`
	JobID        int32            `gorm:"column:job_id;not null"`
	MarketerID   int32            `gorm:"column:marketer_id;not null"`
	Status       JobRequestStatus `gorm:"column:request_status;type:varchar(20);not null"`
	CreatedAt    time.Time        `gorm:"column:created_at;type:timestamp;not null"`
	RespondedAt  *time.Time       `gorm:"column:responded_at;type:timestamp"`

	Job      *Job      `gorm:"foreignKey:JobID;references:JobID"`
	Marketer *Marketer `gorm:"foreignKey:MarketerID;references:UserID"`
}

func (JobRequest) TableName() string { return "job_requests" }
