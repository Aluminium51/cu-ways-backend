package domain

import "time"

type AttachmentType string

const (
	AttachmentTypeBrief AttachmentType = "Brief"
	AttachmentTypeProof AttachmentType = "Proof"
	AttachmentTypeOther AttachmentType = "Other"
)

// IsValid reports whether the type is one the attachments table accepts.
func (t AttachmentType) IsValid() bool {
	switch t {
	case AttachmentTypeBrief, AttachmentTypeProof, AttachmentTypeOther:
		return true
	default:
		return false
	}
}

type Attachment struct {
	AttachmentID   int32          `gorm:"column:attachment_id;primaryKey;autoIncrement"`
	JobID          int32          `gorm:"column:job_id;not null"`
	AttachmentType AttachmentType `gorm:"column:attachment_type;type:varchar(30);not null"`
	URL            string         `gorm:"column:url;type:text;not null"`
	UploadAt       time.Time      `gorm:"column:upload_at;type:timestamp;not null"`

	Job *Job `gorm:"foreignKey:JobID;references:JobID"`
}

func (Attachment) TableName() string { return "attachments" }
