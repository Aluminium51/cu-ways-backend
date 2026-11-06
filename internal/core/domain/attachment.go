package domain

import "time"

type AttachmentType string

const (
	AttachmentTypeBrief AttachmentType = "Brief"
	AttachmentTypeProof AttachmentType = "Proof"
	AttachmentTypeOther AttachmentType = "Other"
)

type Attachment struct {
	AttachmentID   int32          `gorm:"column:attachment_id;primaryKey;autoIncrement"`
	JobID          int32          `gorm:"column:job_id;not null"`
	AttachmentType AttachmentType `gorm:"column:attachment_type;type:varchar(30);not null"`
	URL            string         `gorm:"column:url;type:text;not null"`
	UploadAt       time.Time      `gorm:"column:upload_at;type:timestamp;not null"`

	Job *Job `gorm:"foreignKey:JobID;references:JobID"`
}

func (Attachment) TableName() string { return "attachments" }
