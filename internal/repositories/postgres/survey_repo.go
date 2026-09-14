package postgres

import (
	"context"
	"errors"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SurveyRepository struct {
	db *gorm.DB
}

var _ ports.SurveyRepository = (*SurveyRepository)(nil)

func NewSurveyRepository(db *gorm.DB) *SurveyRepository {
	return &SurveyRepository{db: db}
}

func (r *SurveyRepository) CreateForCreator(ctx context.Context, userID int32, survey *domain.Survey) error {
	tx := r.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return tx.Error
	}
	rollback := func(err error) error {
		tx.Rollback()
		return err
	}
	if err := tx.Exec("INSERT INTO creators (user_id) VALUES (?) ON CONFLICT (user_id) DO NOTHING", userID).Error; err != nil {
		return rollback(err)
	}
	if err := tx.Create(survey).Error; err != nil {
		return rollback(err)
	}
	if err := tx.Commit().Error; err != nil {
		return err
	}
	return nil
}

func (r *SurveyRepository) FindByID(ctx context.Context, surveyID int32) (*domain.Survey, error) {
	var survey domain.Survey
	if err := r.db.WithContext(ctx).Where("survey_id = ?", surveyID).First(&survey).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrSurveyNotFound
		}
		return nil, err
	}
	return &survey, nil
}

func (r *SurveyRepository) Update(ctx context.Context, surveyID int32, patch ports.SurveyPatch) (*domain.Survey, error) {
	updates := make(map[string]any)
	if patch.TitleSet {
		updates["title"] = patch.Title
	}
	if patch.DescriptionSet {
		updates["description"] = patch.Description
	}
	if patch.SurveyLinkSet {
		updates["survey_link"] = patch.SurveyLink
	}
	if patch.TargetGroupSet {
		updates["target_group"] = patch.TargetGroup
	}
	if patch.DesiredResponsesSet {
		updates["desired_responses"] = patch.DesiredResponses
	}
	if patch.DeadlineSet {
		updates["deadline"] = patch.Deadline
	}
	if len(updates) == 0 {
		return nil, domain.ErrNoSurveyChanges
	}
	result := r.db.WithContext(ctx).Model(&domain.Survey{}).Where("survey_id = ?", surveyID).Updates(updates)
	if err := result.Error; err != nil {
		return nil, err
	}
	if result.RowsAffected == 0 {
		return nil, domain.ErrSurveyNotFound
	}
	return r.FindByID(ctx, surveyID)
}

func (r *SurveyRepository) Delete(ctx context.Context, surveyID int32) error {
	tx := r.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return tx.Error
	}
	rollback := func(err error) error {
		tx.Rollback()
		return err
	}

	var survey domain.Survey
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("survey_id = ?", surveyID).First(&survey).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return rollback(domain.ErrSurveyNotFound)
		}
		return rollback(err)
	}
	var references int64
	if err := tx.Table("is_used_in").Where("survey_id = ?", surveyID).Count(&references).Error; err != nil {
		return rollback(err)
	}
	if references > 0 {
		return rollback(domain.ErrSurveyInUse)
	}
	result := tx.Where("survey_id = ?", surveyID).Delete(&domain.Survey{})
	if err := result.Error; err != nil {
		return rollback(err)
	}
	if result.RowsAffected == 0 {
		return rollback(domain.ErrSurveyNotFound)
	}
	return tx.Commit().Error
}
