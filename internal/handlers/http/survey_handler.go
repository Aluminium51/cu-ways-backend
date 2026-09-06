package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/platform/response"
	"github.com/Aluminium51/cu-way-backend/internal/platform/utils"
	"github.com/Aluminium51/cu-way-backend/internal/services"
	"github.com/gofiber/fiber/v2"
)

type surveyService interface {
	Create(context.Context, services.Actor, services.CreateSurveyInput) (*domain.Survey, error)
	Get(context.Context, services.Actor, int32) (*domain.Survey, error)
	Update(context.Context, services.Actor, int32, services.UpdateSurveyInput) (*domain.Survey, error)
	Delete(context.Context, services.Actor, int32) error
}

type SurveyHandler struct {
	service surveyService
}

func NewSurveyHandler(service surveyService) *SurveyHandler {
	return &SurveyHandler{service: service}
}

type CreateSurveyDTO struct {
	Title            string     `json:"title" validate:"required,max=200"`
	Description      *string    `json:"description" validate:"omitempty,max=5000"`
	SurveyLink       string     `json:"survey_link" validate:"required"`
	TargetGroup      *string    `json:"target_group" validate:"omitempty,max=200"`
	DesiredResponses *int32     `json:"desired_responses" validate:"omitempty,gte=1"`
	Deadline         *time.Time `json:"deadline"`
}

type OptionalInt32 struct {
	Set   bool
	Value *int32
}

func (o *OptionalInt32) UnmarshalJSON(data []byte) error {
	o.Set = true
	data = bytes.TrimSpace(data)
	if bytes.Equal(data, []byte("null")) {
		o.Value = nil
		return nil
	}
	var value int32
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	o.Value = &value
	return nil
}

type OptionalTime struct {
	Set   bool
	Value *time.Time
}

func (o *OptionalTime) UnmarshalJSON(data []byte) error {
	o.Set = true
	data = bytes.TrimSpace(data)
	if bytes.Equal(data, []byte("null")) {
		o.Value = nil
		return nil
	}
	var value time.Time
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	o.Value = &value
	return nil
}

type UpdateSurveyDTO struct {
	Title            OptionalString `json:"title"`
	Description      OptionalString `json:"description"`
	SurveyLink       OptionalString `json:"survey_link"`
	TargetGroup      OptionalString `json:"target_group"`
	DesiredResponses OptionalInt32  `json:"desired_responses"`
	Deadline         OptionalTime   `json:"deadline"`
}

type SurveyResponse struct {
	SurveyID         int32      `json:"survey_id"`
	UserID           int32      `json:"user_id"`
	Title            string     `json:"title"`
	Description      *string    `json:"description"`
	SurveyLink       string     `json:"survey_link"`
	TargetGroup      *string    `json:"target_group"`
	DesiredResponses *int32     `json:"desired_responses"`
	Deadline         *time.Time `json:"deadline"`
	CreatedAt        time.Time  `json:"created_at"`
}

func (h *SurveyHandler) Create(c *fiber.Ctx) error {
	actor, err := actorFromRequest(c)
	if err != nil {
		return err
	}
	var dto CreateSurveyDTO
	if err := c.BodyParser(&dto); err != nil {
		return validationError(err)
	}
	if err := utils.Validate(dto); err != nil {
		return validationError(err)
	}
	if strings.TrimSpace(dto.SurveyLink) == "" {
		return validationError(domain.ErrInvalidSurvey)
	}
	survey, err := h.service.Create(c.UserContext(), actor, services.CreateSurveyInput{
		Title:            dto.Title,
		Description:      dto.Description,
		SurveyLink:       dto.SurveyLink,
		TargetGroup:      dto.TargetGroup,
		DesiredResponses: dto.DesiredResponses,
		Deadline:         dto.Deadline,
	})
	if err != nil {
		return mapSurveyError(err)
	}
	return response.Success(c, fiber.StatusCreated, toSurveyResponse(survey))
}

func (h *SurveyHandler) Get(c *fiber.Ctx) error {
	actor, err := actorFromRequest(c)
	if err != nil {
		return err
	}
	surveyID, err := parseUserID(c.Params("id"))
	if err != nil {
		return validationError(err)
	}
	survey, err := h.service.Get(c.UserContext(), actor, surveyID)
	if err != nil {
		return mapSurveyError(err)
	}
	return response.Success(c, fiber.StatusOK, toSurveyResponse(survey))
}

func (h *SurveyHandler) Update(c *fiber.Ctx) error {
	actor, err := actorFromRequest(c)
	if err != nil {
		return err
	}
	surveyID, err := parseUserID(c.Params("id"))
	if err != nil {
		return validationError(err)
	}
	var dto UpdateSurveyDTO
	if err := c.BodyParser(&dto); err != nil {
		return validationError(err)
	}
	if !dto.Title.Set && !dto.Description.Set && !dto.SurveyLink.Set && !dto.TargetGroup.Set && !dto.DesiredResponses.Set && !dto.Deadline.Set {
		return validationError(domain.ErrNoSurveyChanges)
	}
	if (dto.Title.Set && dto.Title.Value == nil) || (dto.SurveyLink.Set && dto.SurveyLink.Value == nil) {
		return validationError(domain.ErrInvalidSurvey)
	}
	if dto.DesiredResponses.Set && dto.DesiredResponses.Value != nil && *dto.DesiredResponses.Value < 1 {
		return validationError(domain.ErrInvalidSurvey)
	}
	if dto.Description.Set && dto.Description.Value != nil && utf8.RuneCountInString(strings.TrimSpace(*dto.Description.Value)) > services.MaxSurveyDescriptionLength {
		return validationError(domain.ErrInvalidSurvey)
	}
	input := services.UpdateSurveyInput{
		TitleSet:            dto.Title.Set,
		Title:               optionalStringValue(dto.Title),
		DescriptionSet:      dto.Description.Set,
		Description:         dto.Description.Value,
		SurveyLinkSet:       dto.SurveyLink.Set,
		SurveyLink:          optionalStringValue(dto.SurveyLink),
		TargetGroupSet:      dto.TargetGroup.Set,
		TargetGroup:         dto.TargetGroup.Value,
		DesiredResponsesSet: dto.DesiredResponses.Set,
		DesiredResponses:    dto.DesiredResponses.Value,
		DeadlineSet:         dto.Deadline.Set,
		Deadline:            dto.Deadline.Value,
	}
	survey, err := h.service.Update(c.UserContext(), actor, surveyID, input)
	if err != nil {
		return mapSurveyError(err)
	}
	return response.Success(c, fiber.StatusOK, toSurveyResponse(survey))
}

func (h *SurveyHandler) Delete(c *fiber.Ctx) error {
	actor, err := actorFromRequest(c)
	if err != nil {
		return err
	}
	surveyID, err := parseUserID(c.Params("id"))
	if err != nil {
		return validationError(err)
	}
	if err := h.service.Delete(c.UserContext(), actor, surveyID); err != nil {
		return mapSurveyError(err)
	}
	return response.Success(c, fiber.StatusOK, map[string]any{"survey_id": surveyID, "deleted": true})
}

func toSurveyResponse(survey *domain.Survey) SurveyResponse {
	return SurveyResponse{
		SurveyID:         survey.SurveyID,
		UserID:           survey.UserID,
		Title:            survey.Title,
		Description:      survey.Description,
		SurveyLink:       survey.SurveyLink,
		TargetGroup:      survey.TargetGroup,
		DesiredResponses: survey.DesiredResponses,
		Deadline:         survey.Deadline,
		CreatedAt:        survey.CreatedAt,
	}
}

func mapSurveyError(err error) error {
	switch {
	case errors.Is(err, domain.ErrSurveyNotFound):
		return response.NewAppError(fiber.StatusNotFound, "survey_not_found", "survey not found", err)
	case errors.Is(err, domain.ErrSurveyForbidden):
		return response.NewAppError(fiber.StatusForbidden, "forbidden", "you do not have access to this survey", err)
	case errors.Is(err, domain.ErrSurveyInUse):
		return response.NewAppError(fiber.StatusConflict, "survey_in_use", "survey is referenced by a job and cannot be deleted", err)
	case errors.Is(err, domain.ErrInvalidSurvey), errors.Is(err, domain.ErrNoSurveyChanges):
		return validationError(err)
	default:
		return err
	}
}
