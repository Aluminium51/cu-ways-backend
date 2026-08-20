package domain

import "time"

type Survey struct {
	SurveyID         int32      `gorm:"column:survey_id;primaryKey;autoIncrement"`
	UserID           int32      `gorm:"column:user_id;not null"`
	Title            string     `gorm:"column:title;type:varchar(200);not null"`
	Description      *string    `gorm:"column:description;type:text"`
	SurveyLink       string     `gorm:"column:survey_link;type:text;not null"`
	TargetGroup      *string    `gorm:"column:target_group;type:varchar(200)"`
	DesiredResponses *int32     `gorm:"column:desired_responses"`
	Deadline         *time.Time `gorm:"column:deadline;type:timestamp"`
	CreatedAt        time.Time  `gorm:"column:created_at;type:timestamp;not null"`

	Creator *Creator `gorm:"foreignKey:UserID;references:UserID"`
	Jobs    []Job    `gorm:"many2many:is_used_in;joinForeignKey:SurveyID;joinReferences:JobID"`
}

func (Survey) TableName() string { return "surveys" }
