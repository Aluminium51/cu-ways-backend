package domain

import "time"

type User struct {
	UserID       int32      `gorm:"column:user_id;primaryKey;autoIncrement"`
	Name         string     `gorm:"column:name;type:varchar(100);not null"`
	Email        string     `gorm:"column:email;type:varchar(255);uniqueIndex;not null"`
	Phone        *string    `gorm:"column:phone;type:varchar(20)"`
	LineID       *string    `gorm:"column:line_id;type:varchar(50)"`
	PasswordHash *string    `gorm:"column:password_hash;type:varchar(255)" json:"-"`
	Role         string     `gorm:"column:role;type:varchar(20);not null" json:"-"`
	CreatedAt    time.Time  `gorm:"column:created_at;type:timestamp;not null"`
	DeletedAt    *time.Time `gorm:"column:deleted_at;index" json:"-"`

	Creator  *Creator  `gorm:"foreignKey:UserID;references:UserID"`
	Marketer *Marketer `gorm:"foreignKey:UserID;references:UserID"`
}

func (User) TableName() string { return "users" }

const (
	RoleUser  = "user"
	RoleAdmin = "admin"
)

type Creator struct {
	UserID int32 `gorm:"column:user_id;primaryKey"`
	User   *User `gorm:"foreignKey:UserID;references:UserID"`

	Surveys []Survey `gorm:"foreignKey:UserID;references:UserID"`
	Jobs    []Job    `gorm:"foreignKey:UserID;references:UserID"`
}

func (Creator) TableName() string { return "creators" }

type Marketer struct {
	UserID             int32              `gorm:"column:user_id;primaryKey"`
	Bio                string             `gorm:"column:bio;type:text;not null"`
	ExperienceYears    int32              `gorm:"column:experience_years;not null"`
	AvailabilityStatus AvailabilityStatus `gorm:"column:availability_status;type:varchar(20);not null"`
	AvailabilityText   string             `gorm:"column:availability_text;type:text;not null"`
	User               *User              `gorm:"foreignKey:UserID;references:UserID"`
	Expertise          []Expertise        `gorm:"many2many:marketer_expertise;joinForeignKey:UserID;joinReferences:ExpertiseID"`
	Campuses           []Campus           `gorm:"many2many:marketer_campuses;joinForeignKey:UserID;joinReferences:CampusID"`

	Services []Service `gorm:"foreignKey:UserID;references:UserID"`
	Offers   []Offer   `gorm:"foreignKey:UserID;references:UserID"`
}

func (Marketer) TableName() string { return "marketers" }

type AvailabilityStatus string

const (
	AvailabilityAvailable   AvailabilityStatus = "available"
	AvailabilityLimited     AvailabilityStatus = "limited"
	AvailabilityUnavailable AvailabilityStatus = "unavailable"
)

type Expertise struct {
	ExpertiseID int32  `gorm:"column:expertise_id;primaryKey;autoIncrement"`
	Slug        string `gorm:"column:slug;type:varchar(80);uniqueIndex;not null"`
	Name        string `gorm:"column:name;type:varchar(120);not null"`
}

func (Expertise) TableName() string { return "expertise_options" }

type Campus struct {
	CampusID int32  `gorm:"column:campus_id;primaryKey;autoIncrement"`
	Slug     string `gorm:"column:slug;type:varchar(80);uniqueIndex;not null"`
	Name     string `gorm:"column:name;type:varchar(120);not null"`
}

func (Campus) TableName() string { return "campus_options" }

type MarketerSearchResult struct {
	Marketer      Marketer
	LowestPrice   *string
	AverageRating *float64
	ReviewCount   int64
}
