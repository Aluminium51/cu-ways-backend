package httpapi

import (
	"context"
	"errors"
	"net/url"
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/platform/response"
	"github.com/Aluminium51/cu-way-backend/internal/platform/utils"
	"github.com/Aluminium51/cu-way-backend/internal/services"
	"github.com/gofiber/fiber/v2"
)

type jobService interface {
	ListSurveys(context.Context, services.Actor) ([]domain.Survey, error)
	Create(context.Context, services.Actor, services.CreateJobInput) (*domain.Job, error)
	AddSurvey(context.Context, services.Actor, int32, int32) error
	RemoveSurvey(context.Context, services.Actor, int32, int32) error
	AddBrief(context.Context, services.Actor, int32, string) (*domain.Attachment, error)
	ListBriefs(context.Context, services.Actor, int32) ([]domain.Attachment, error)
	CreateRequests(context.Context, services.Actor, int32, services.CreateJobRequestsInput) ([]domain.JobRequest, error)
}

type JobHandler struct {
	service jobService
}

func NewJobHandler(service jobService) *JobHandler {
	return &JobHandler{service: service}
}

type CreateJobDTO struct {
	SurveyIDs []int32 `json:"survey_ids" validate:"required,min=1,dive,gt=0"`
}

type CreateJobRequestsDTO struct {
	MarketerIDs []int32 `json:"marketer_ids" validate:"required,min=1,dive,gt=0"`
}

type CreateBriefDTO struct {
	URL string `json:"url" validate:"required,url"`
}

type JobResponse struct {
	JobID           int32            `json:"job_id"`
	UserID          int32            `json:"user_id"`
	AcceptedOfferID *int32           `json:"accepted_offer_id"`
	JobStatus       domain.JobStatus `json:"job_status"`
	CreatedAt       time.Time        `json:"created_at"`
	SurveyIDs       []int32          `json:"survey_ids"`
}

type JobLinkResponse struct {
	JobID    int32 `json:"job_id"`
	SurveyID int32 `json:"survey_id"`
	Linked   bool  `json:"linked"`
}

type BriefResponse struct {
	AttachmentID   int32                 `json:"attachment_id"`
	AttachmentType domain.AttachmentType `json:"attachment_type"`
	URL            string                `json:"url"`
	UploadAt       time.Time             `json:"upload_at"`
}

type JobRequestResponse struct {
	JobRequestID  int32                   `json:"job_request_id"`
	JobID         int32                   `json:"job_id"`
	MarketerID    int32                   `json:"marketer_id"`
	Status        domain.JobRequestStatus `json:"request_status"`
	CreatedAt     time.Time               `json:"created_at"`
	RespondedAt   *string                 `json:"responded_at"`
	DeclineReason *string                 `json:"decline_reason"`
	DeclineNote   *string                 `json:"decline_note"`
}

func (h *JobHandler) ListSurveys(c *fiber.Ctx) error {
	actor, err := actorFromRequest(c)
	if err != nil {
		return err
	}
	surveys, err := h.service.ListSurveys(c.UserContext(), actor)
	if err != nil {
		return mapJobError(err)
	}
	items := make([]SurveyResponse, 0, len(surveys))
	for index := range surveys {
		items = append(items, toSurveyResponse(&surveys[index]))
	}
	return response.Success(c, fiber.StatusOK, items)
}

func (h *JobHandler) Create(c *fiber.Ctx) error {
	actor, err := actorFromRequest(c)
	if err != nil {
		return err
	}
	var dto CreateJobDTO
	if err := decodeStrictJSON(c, &dto); err != nil {
		return validationError(err)
	}
	if err := utils.Validate(dto); err != nil {
		return validationError(err)
	}
	job, err := h.service.Create(c.UserContext(), actor, services.CreateJobInput{SurveyIDs: dto.SurveyIDs})
	if err != nil {
		return mapJobError(err)
	}
	return response.Success(c, fiber.StatusCreated, toJobResponse(job))
}

func (h *JobHandler) AddSurvey(c *fiber.Ctx) error {
	actor, err := actorFromRequest(c)
	if err != nil {
		return err
	}
	jobID, surveyID, err := parseJobAndSurveyIDs(c)
	if err != nil {
		return validationError(err)
	}
	if err := h.service.AddSurvey(c.UserContext(), actor, jobID, surveyID); err != nil {
		return mapJobError(err)
	}
	return response.Success(c, fiber.StatusOK, JobLinkResponse{JobID: jobID, SurveyID: surveyID, Linked: true})
}

func (h *JobHandler) RemoveSurvey(c *fiber.Ctx) error {
	actor, err := actorFromRequest(c)
	if err != nil {
		return err
	}
	jobID, surveyID, err := parseJobAndSurveyIDs(c)
	if err != nil {
		return validationError(err)
	}
	if err := h.service.RemoveSurvey(c.UserContext(), actor, jobID, surveyID); err != nil {
		return mapJobError(err)
	}
	return response.Success(c, fiber.StatusOK, JobLinkResponse{JobID: jobID, SurveyID: surveyID, Linked: false})
}

func (h *JobHandler) AddBrief(c *fiber.Ctx) error {
	actor, err := actorFromRequest(c)
	if err != nil {
		return err
	}
	jobID, err := parseUserID(c.Params("id"))
	if err != nil {
		return validationError(err)
	}
	var dto CreateBriefDTO
	if err := decodeStrictJSON(c, &dto); err != nil {
		return validationError(err)
	}
	if err := utils.Validate(dto); err != nil {
		return validationError(err)
	}
	if parsed, parseErr := url.ParseRequestURI(dto.URL); parseErr != nil || parsed == nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return validationError(domain.ErrInvalidBriefURL)
	}
	attachment, err := h.service.AddBrief(c.UserContext(), actor, jobID, dto.URL)
	if err != nil {
		return mapJobError(err)
	}
	return response.Success(c, fiber.StatusCreated, toBriefResponse(*attachment))
}

func (h *JobHandler) ListBriefs(c *fiber.Ctx) error {
	actor, err := actorFromRequest(c)
	if err != nil {
		return err
	}
	jobID, err := parseUserID(c.Params("id"))
	if err != nil {
		return validationError(err)
	}
	attachments, err := h.service.ListBriefs(c.UserContext(), actor, jobID)
	if err != nil {
		return mapJobError(err)
	}
	items := make([]BriefResponse, 0, len(attachments))
	for _, attachment := range attachments {
		items = append(items, toBriefResponse(attachment))
	}
	return response.Success(c, fiber.StatusOK, items)
}

func (h *JobHandler) CreateRequests(c *fiber.Ctx) error {
	actor, err := actorFromRequest(c)
	if err != nil {
		return err
	}
	jobID, err := parseUserID(c.Params("id"))
	if err != nil {
		return validationError(err)
	}
	var dto CreateJobRequestsDTO
	if err := decodeStrictJSON(c, &dto); err != nil {
		return validationError(err)
	}
	if err := utils.Validate(dto); err != nil {
		return validationError(err)
	}
	requests, err := h.service.CreateRequests(c.UserContext(), actor, jobID, services.CreateJobRequestsInput{MarketerIDs: dto.MarketerIDs})
	if err != nil {
		return mapJobError(err)
	}
	items := make([]JobRequestResponse, 0, len(requests))
	for _, request := range requests {
		items = append(items, toJobRequestResponse(request))
	}
	return response.Success(c, fiber.StatusCreated, items)
}

func parseJobAndSurveyIDs(c *fiber.Ctx) (int32, int32, error) {
	jobID, err := parseUserID(c.Params("id"))
	if err != nil {
		return 0, 0, err
	}
	surveyID, err := parseUserID(c.Params("surveyId"))
	if err != nil {
		return 0, 0, err
	}
	return jobID, surveyID, nil
}

func toJobResponse(job *domain.Job) JobResponse {
	result := JobResponse{
		JobID:           job.JobID,
		UserID:          job.UserID,
		AcceptedOfferID: job.AcceptedOfferID,
		JobStatus:       job.JobStatus,
		CreatedAt:       job.CreatedAt,
		SurveyIDs:       make([]int32, 0, len(job.Surveys)),
	}
	for _, survey := range job.Surveys {
		result.SurveyIDs = append(result.SurveyIDs, survey.SurveyID)
	}
	return result
}

func toBriefResponse(attachment domain.Attachment) BriefResponse {
	return BriefResponse{AttachmentID: attachment.AttachmentID, AttachmentType: attachment.AttachmentType, URL: attachment.URL, UploadAt: attachment.UploadAt}
}

func toJobRequestResponse(request domain.JobRequest) JobRequestResponse {
	var respondedAt *string
	if request.RespondedAt != nil {
		value := request.RespondedAt.Format(time.RFC3339Nano)
		respondedAt = &value
	}
	return JobRequestResponse{
		JobRequestID:  request.JobRequestID,
		JobID:         request.JobID,
		MarketerID:    request.MarketerID,
		Status:        request.Status,
		CreatedAt:     request.CreatedAt,
		RespondedAt:   respondedAt,
		DeclineReason: request.DeclineReason,
		DeclineNote:   request.DeclineNote,
	}
}

func mapJobError(err error) error {
	switch {
	case errors.Is(err, domain.ErrCreatorRequired):
		return response.NewAppError(fiber.StatusForbidden, "creator_required", "a creator profile is required", err)
	case errors.Is(err, domain.ErrJobForbidden), errors.Is(err, domain.ErrSurveyForbidden):
		return response.NewAppError(fiber.StatusForbidden, "forbidden", "you do not have access to this job or survey", err)
	case errors.Is(err, domain.ErrJobNotFound), errors.Is(err, domain.ErrSurveyNotFound), errors.Is(err, domain.ErrJobSurveyNotFound):
		return response.NewAppError(fiber.StatusNotFound, "resource_not_found", "job or survey was not found", err)
	case errors.Is(err, domain.ErrJobRequestAlreadyActive), errors.Is(err, domain.ErrJobSurveyAlreadyLinked), errors.Is(err, domain.ErrLastJobSurvey):
		return response.NewAppError(fiber.StatusConflict, "job_conflict", err.Error(), err)
	case errors.Is(err, domain.ErrMarketerNotFound):
		return response.NewAppError(fiber.StatusUnprocessableEntity, "marketer_not_found", "one or more selected marketers are unavailable", err)
	case errors.Is(err, domain.ErrInvalidJob), errors.Is(err, domain.ErrInvalidBriefURL), errors.Is(err, domain.ErrInvalidJobRequest), errors.Is(err, domain.ErrNoJobSurveys):
		return validationError(err)
	default:
		return err
	}
}
