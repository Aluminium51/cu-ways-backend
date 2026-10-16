package httpapi

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
	"github.com/Aluminium51/cu-way-backend/internal/platform/response"
	"github.com/Aluminium51/cu-way-backend/internal/platform/utils"
	"github.com/Aluminium51/cu-way-backend/internal/services"
	"github.com/gofiber/fiber/v2"
)

type marketerJobRequestService interface {
	List(context.Context, services.Actor, *domain.JobRequestStatus) ([]ports.MarketerJobRequest, error)
	Accept(context.Context, services.Actor, int32) (*domain.JobRequest, error)
	Decline(context.Context, services.Actor, int32, services.DeclineJobRequestInput) (*domain.JobRequest, error)
	SubmitOffer(context.Context, services.Actor, int32, services.SubmitOfferInput) (*domain.Offer, error)
	ListOffers(context.Context, services.Actor, int32) ([]domain.Offer, error)
	WithdrawOffer(context.Context, services.Actor, int32) (*domain.Offer, error)
}

type MarketerJobRequestHandler struct {
	service marketerJobRequestService
}

func NewMarketerJobRequestHandler(service marketerJobRequestService) *MarketerJobRequestHandler {
	return &MarketerJobRequestHandler{service: service}
}

type DeclineJobRequestDTO struct {
	Reason *string `json:"reason" validate:"omitempty,max=50"`
	Note   *string `json:"note" validate:"omitempty,max=300"`
}

type SubmitOfferDTO struct {
	OfferedPrice string  `json:"offered_price" validate:"required"`
	DeliveryDate string  `json:"delivery_date" validate:"required"`
	Message      *string `json:"message" validate:"omitempty,max=300"`
}

type OfferResponse struct {
	OfferID      int32              `json:"offer_id"`
	JobID        int32              `json:"job_id"`
	JobRequestID *int32             `json:"job_request_id"`
	MarketerID   int32              `json:"marketer_id"`
	OfferedPrice string             `json:"offered_price"`
	DeliveryDate *string            `json:"delivery_date"`
	Message      *string            `json:"message"`
	OfferStatus  domain.OfferStatus `json:"offer_status"`
	CreatedAt    time.Time          `json:"created_at"`
}

type MarketerJobRequestResponse struct {
	JobRequestResponse
	Job MarketerJobRequestJobResponse `json:"job"`
}

type MarketerJobRequestJobResponse struct {
	JobID           int32                      `json:"job_id"`
	JobStatus       domain.JobStatus           `json:"job_status"`
	Creator         MarketerJobRequestCreator  `json:"creator"`
	Deadline        *time.Time                 `json:"deadline"`
	TargetResponses *int32                     `json:"target_responses"`
	Surveys         []MarketerJobRequestSurvey `json:"surveys"`
	Brief           *BriefResponse             `json:"brief"`
}

type MarketerJobRequestCreator struct {
	UserID int32  `json:"user_id"`
	Name   string `json:"name"`
}

type MarketerJobRequestSurvey struct {
	SurveyID         int32      `json:"survey_id"`
	Title            string     `json:"title"`
	Description      *string    `json:"description"`
	SurveyLink       string     `json:"survey_link"`
	TargetGroup      *string    `json:"target_group"`
	DesiredResponses *int32     `json:"desired_responses"`
	Deadline         *time.Time `json:"deadline"`
}

func (h *MarketerJobRequestHandler) List(c *fiber.Ctx) error {
	actor, err := actorFromRequest(c)
	if err != nil {
		return err
	}
	var status *domain.JobRequestStatus
	if raw := strings.TrimSpace(c.Query("status")); raw != "" {
		value := domain.JobRequestStatus(raw)
		if !value.IsValid() {
			return validationError(errors.New("status must be Pending, Accepted, Declined, or Closed"))
		}
		status = &value
	}
	items, err := h.service.List(c.UserContext(), actor, status)
	if err != nil {
		return mapMarketerJobRequestError(err)
	}
	result := make([]MarketerJobRequestResponse, 0, len(items))
	for index := range items {
		result = append(result, toMarketerJobRequestResponse(&items[index]))
	}
	return response.Success(c, fiber.StatusOK, result)
}

func (h *MarketerJobRequestHandler) Accept(c *fiber.Ctx) error {
	actor, err := actorFromRequest(c)
	if err != nil {
		return err
	}
	requestID, err := parseUserID(c.Params("id"))
	if err != nil {
		return validationError(err)
	}
	request, err := h.service.Accept(c.UserContext(), actor, requestID)
	if err != nil {
		return mapMarketerJobRequestError(err)
	}
	return response.Success(c, fiber.StatusOK, toJobRequestResponse(*request))
}

func (h *MarketerJobRequestHandler) Decline(c *fiber.Ctx) error {
	actor, err := actorFromRequest(c)
	if err != nil {
		return err
	}
	requestID, err := parseUserID(c.Params("id"))
	if err != nil {
		return validationError(err)
	}
	var dto DeclineJobRequestDTO
	// The body is optional: a decline without a reason or note is valid.
	if len(strings.TrimSpace(string(c.Body()))) > 0 {
		if err := decodeStrictJSON(c, &dto); err != nil {
			return validationError(err)
		}
		if err := utils.Validate(dto); err != nil {
			return validationError(err)
		}
	}
	request, err := h.service.Decline(c.UserContext(), actor, requestID, services.DeclineJobRequestInput{
		Reason: dto.Reason,
		Note:   dto.Note,
	})
	if err != nil {
		return mapMarketerJobRequestError(err)
	}
	return response.Success(c, fiber.StatusOK, toJobRequestResponse(*request))
}

func (h *MarketerJobRequestHandler) SubmitOffer(c *fiber.Ctx) error {
	actor, err := actorFromRequest(c)
	if err != nil {
		return err
	}
	requestID, err := parseUserID(c.Params("id"))
	if err != nil {
		return validationError(err)
	}
	var dto SubmitOfferDTO
	if err := decodeStrictJSON(c, &dto); err != nil {
		return validationError(err)
	}
	if err := utils.Validate(dto); err != nil {
		return validationError(err)
	}
	offer, err := h.service.SubmitOffer(c.UserContext(), actor, requestID, services.SubmitOfferInput{
		OfferedPrice: dto.OfferedPrice,
		DeliveryDate: dto.DeliveryDate,
		Message:      dto.Message,
	})
	if err != nil {
		return mapMarketerJobRequestError(err)
	}
	return response.Success(c, fiber.StatusCreated, toOfferResponse(offer))
}

func (h *MarketerJobRequestHandler) WithdrawOffer(c *fiber.Ctx) error {
	actor, err := actorFromRequest(c)
	if err != nil {
		return err
	}
	offerID, err := parseUserID(c.Params("id"))
	if err != nil {
		return validationError(err)
	}
	offer, err := h.service.WithdrawOffer(c.UserContext(), actor, offerID)
	if err != nil {
		return mapMarketerJobRequestError(err)
	}
	return response.Success(c, fiber.StatusOK, toOfferResponse(offer))
}

func (h *MarketerJobRequestHandler) ListOffers(c *fiber.Ctx) error {
	actor, err := actorFromRequest(c)
	if err != nil {
		return err
	}
	requestID, err := parseUserID(c.Params("id"))
	if err != nil {
		return validationError(err)
	}
	offers, err := h.service.ListOffers(c.UserContext(), actor, requestID)
	if err != nil {
		return mapMarketerJobRequestError(err)
	}
	result := make([]OfferResponse, 0, len(offers))
	for index := range offers {
		result = append(result, toOfferResponse(&offers[index]))
	}
	return response.Success(c, fiber.StatusOK, result)
}

func toOfferResponse(offer *domain.Offer) OfferResponse {
	var deliveryDate *string
	if offer.DeliveryDate != nil {
		value := offer.DeliveryDate.Format(time.DateOnly)
		deliveryDate = &value
	}
	return OfferResponse{
		OfferID:      offer.OfferID,
		JobID:        offer.JobID,
		JobRequestID: offer.JobRequestID,
		MarketerID:   offer.UserID,
		OfferedPrice: offer.OfferedPrice.StringFixed(2),
		DeliveryDate: deliveryDate,
		Message:      offer.Message,
		OfferStatus:  offer.OfferStatus,
		CreatedAt:    offer.CreatedAt,
	}
}

// offerValidationError returns the client-safe reason joined after
// domain.ErrInvalidOffer so the frontend can show which field is wrong.
func offerValidationError(err error) error {
	appErr := response.NewAppError(fiber.StatusUnprocessableEntity, "invalid_offer", "offer validation failed", err)
	if lines := strings.Split(err.Error(), "\n"); len(lines) > 1 {
		appErr.Message = lines[1]
	}
	return appErr
}

func toMarketerJobRequestResponse(item *ports.MarketerJobRequest) MarketerJobRequestResponse {
	job := MarketerJobRequestJobResponse{
		JobID:     item.Request.JobID,
		JobStatus: item.JobStatus,
		Creator:   MarketerJobRequestCreator{UserID: item.CreatorID, Name: item.CreatorName},
		Surveys:   make([]MarketerJobRequestSurvey, 0, len(item.Surveys)),
	}
	var target int32
	hasTarget := false
	for index := range item.Surveys {
		survey := &item.Surveys[index]
		job.Surveys = append(job.Surveys, MarketerJobRequestSurvey{
			SurveyID:         survey.SurveyID,
			Title:            survey.Title,
			Description:      survey.Description,
			SurveyLink:       survey.SurveyLink,
			TargetGroup:      survey.TargetGroup,
			DesiredResponses: survey.DesiredResponses,
			Deadline:         survey.Deadline,
		})
		if survey.DesiredResponses != nil {
			target += *survey.DesiredResponses
			hasTarget = true
		}
	}
	if hasTarget {
		job.TargetResponses = &target
	}
	job.Deadline = domain.EarliestSurveyDeadline(item.Surveys)
	if item.CurrentBrief != nil {
		brief := toBriefResponse(*item.CurrentBrief)
		job.Brief = &brief
	}
	return MarketerJobRequestResponse{JobRequestResponse: toJobRequestResponse(item.Request), Job: job}
}

func mapMarketerJobRequestError(err error) error {
	switch {
	case errors.Is(err, domain.ErrMarketerRequired):
		return response.NewAppError(fiber.StatusForbidden, "marketer_profile_required", "a marketer profile is required", err)
	case errors.Is(err, domain.ErrJobRequestNotFound):
		return response.NewAppError(fiber.StatusNotFound, "job_request_not_found", "job request was not found", err)
	case errors.Is(err, domain.ErrJobRequestNotPending):
		return response.NewAppError(fiber.StatusConflict, "job_request_not_pending", "this job request is no longer pending", err)
	case errors.Is(err, domain.ErrJobRequestNotAccepted):
		return response.NewAppError(fiber.StatusConflict, "job_request_not_accepted", "accept this job request before submitting an offer", err)
	case errors.Is(err, domain.ErrJobNotPending):
		return response.NewAppError(fiber.StatusConflict, "job_not_pending", "this job has already hired a marketer", err)
	case errors.Is(err, domain.ErrOfferAlreadyExists):
		return response.NewAppError(fiber.StatusConflict, "offer_already_exists", "an active offer already exists for this job request", err)
	case errors.Is(err, domain.ErrOfferNotFound):
		return response.NewAppError(fiber.StatusNotFound, "offer_not_found", "offer was not found", err)
	case errors.Is(err, domain.ErrOfferNotPending):
		return response.NewAppError(fiber.StatusConflict, "offer_not_pending", "only a pending offer can be withdrawn", err)
	case errors.Is(err, domain.ErrInvalidOffer):
		return offerValidationError(err)
	case errors.Is(err, domain.ErrInvalidJobRequest):
		return validationError(err)
	default:
		return err
	}
}
