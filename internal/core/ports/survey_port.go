package ports

import (
	"context"
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
)

type SurveyRepository interface {
	CreateForCreator(ctx context.Context, userID int32, survey *domain.Survey) error
	FindByID(ctx context.Context, surveyID int32) (*domain.Survey, error)
	Update(ctx context.Context, surveyID int32, patch SurveyPatch) (*domain.Survey, error)
	Delete(ctx context.Context, surveyID int32) error
}

type SurveyPatch struct {
	TitleSet            bool
	Title               string
	DescriptionSet      bool
	Description         *string
	SurveyLinkSet       bool
	SurveyLink          string
	TargetGroupSet      bool
	TargetGroup         *string
	DesiredResponsesSet bool
	DesiredResponses    *int32
	DeadlineSet         bool
	Deadline            *time.Time
}
