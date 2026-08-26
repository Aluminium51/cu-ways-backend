package domain

import "time"

type User struct {
	UserID    int32     `gorm:"column:user_id;primaryKey;autoIncrement"`
	Name      string    `gorm:"column:name;type:varchar(100);not null"`
	Email     string    `gorm:"column:email;type:varchar(255);uniqueIndex;not null"`
	Phone     *string   `gorm:"column:phone;type:varchar(20)"`
	LineID    *string   `gorm:"column:line_id;type:varchar(50)"`
	CreatedAt time.Time `gorm:"column:created_at;type:timestamp;not null"`

	Creator  *Creator  `gorm:"foreignKey:UserID;references:UserID"`
	Marketer *Marketer `gorm:"foreignKey:UserID;references:UserID"`
}

func (User) TableName() string { return "users" }

type Creator struct {
	UserID int32 `gorm:"column:user_id;primaryKey"`
	User   *User `gorm:"foreignKey:UserID;references:UserID"`

	Surveys []Survey `gorm:"foreignKey:UserID;references:UserID"`
	Jobs    []Job    `gorm:"foreignKey:UserID;references:UserID"`
}

func (Creator) TableName() string { return "creators" }

type Marketer struct {
	UserID           int32   `gorm:"column:user_id;primaryKey"`
	Bio              *string `gorm:"column:bio;type:text"`
	Experience       *string `gorm:"column:experience;type:text"`
	AvailabilityText *string `gorm:"column:availability_text;type:text"`
	User             *User   `gorm:"foreignKey:UserID;references:UserID"`

	Services []Service `gorm:"foreignKey:UserID;references:UserID"`
	Offers   []Offer   `gorm:"foreignKey:UserID;references:UserID"`
}

func (Marketer) TableName() string { return "marketers" }
