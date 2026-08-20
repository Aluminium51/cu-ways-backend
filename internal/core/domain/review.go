package domain

import "time"

type Review struct {
	ReviewID  int32     `gorm:"column:review_id;primaryKey;autoIncrement"`
	JobID     int32     `gorm:"column:job_id;not null;uniqueIndex"`
	Rating    int32     `gorm:"column:rating;not null"`
	Comment   *string   `gorm:"column:comment;type:text"`
	CreatedAt time.Time `gorm:"column:created_at;type:timestamp;not null"`

	Job *Job `gorm:"foreignKey:JobID;references:JobID"`
}

func (Review) TableName() string { return "reviews" }
