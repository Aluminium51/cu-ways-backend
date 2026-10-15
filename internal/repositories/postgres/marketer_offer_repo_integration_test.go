package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func acceptIntegrationJobRequest(t *testing.T, db *gorm.DB, marketerID, requestID int32) {
	t.Helper()
	if _, err := NewMarketerJobRequestRepository(db).RespondToPending(context.Background(), marketerID, requestID, ports.JobRequestResponse{
		Status: domain.JobRequestStatusAccepted, RespondedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("accept job request fixture: %v", err)
	}
}

func newIntegrationOffer(price string) *domain.Offer {
	delivery := time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC)
	message := "Campus booth and faculty LINE groups."
	return &domain.Offer{
		OfferedPrice: decimal.RequireFromString(price),
		DeliveryDate: &delivery,
		Message:      &message,
		OfferStatus:  domain.OfferStatusPending,
		CreatedAt:    time.Now().UTC(),
	}
}

func TestMarketerOfferRepositoryStoresPendingOfferForAcceptedRequest(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	ctx := context.Background()
	creatorID := createIntegrationCreator(t, db, "offer-create-creator")
	marketerID := createIntegrationMarketer(t, db, "offer-create-marketer")
	surveyID := createIntegrationSurvey(t, db, creatorID, "offer-create")
	jobID, requestID := createIntegrationJobRequest(t, db, creatorID, marketerID, surveyID)
	repo := NewMarketerJobRequestRepository(db)

	if err := repo.CreateOffer(ctx, marketerID, requestID, newIntegrationOffer("100")); !errors.Is(err, domain.ErrJobRequestNotAccepted) {
		t.Fatalf("expected a Pending request to reject offers, got %v", err)
	}

	acceptIntegrationJobRequest(t, db, marketerID, requestID)
	offer := newIntegrationOffer("9000.50")
	if err := repo.CreateOffer(ctx, marketerID, requestID, offer); err != nil {
		t.Fatal(err)
	}
	if offer.OfferID == 0 || offer.JobID != jobID || offer.UserID != marketerID || offer.JobRequestID == nil || *offer.JobRequestID != requestID {
		t.Fatalf("unexpected stored offer identity: %+v", offer)
	}
	var stored domain.Offer
	if err := db.First(&stored, "offer_id = ?", offer.OfferID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.OfferStatus != domain.OfferStatusPending || stored.OfferedPrice.StringFixed(2) != "9000.50" {
		t.Fatalf("unexpected stored offer: %+v", stored)
	}
	if stored.DeliveryDate == nil || stored.DeliveryDate.Format(time.DateOnly) != "2026-10-25" || stored.Message == nil {
		t.Fatalf("delivery date and message were not persisted: %+v", stored)
	}
}

func TestMarketerOfferRepositoryAllowsOneActiveOfferPerRequest(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	ctx := context.Background()
	creatorID := createIntegrationCreator(t, db, "offer-dup-creator")
	marketerID := createIntegrationMarketer(t, db, "offer-dup-marketer")
	surveyID := createIntegrationSurvey(t, db, creatorID, "offer-dup")
	_, requestID := createIntegrationJobRequest(t, db, creatorID, marketerID, surveyID)
	acceptIntegrationJobRequest(t, db, marketerID, requestID)
	repo := NewMarketerJobRequestRepository(db)

	first := newIntegrationOffer("100")
	if err := repo.CreateOffer(ctx, marketerID, requestID, first); err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateOffer(ctx, marketerID, requestID, newIntegrationOffer("90")); !errors.Is(err, domain.ErrOfferAlreadyExists) {
		t.Fatalf("expected a second active offer to be rejected, got %v", err)
	}

	// The partial unique index is the final safeguard if two submissions race
	// past the service check.
	err := db.Transaction(func(tx *gorm.DB) error {
		duplicate := newIntegrationOffer("80")
		duplicate.JobID = first.JobID
		duplicate.UserID = marketerID
		duplicate.JobRequestID = &requestID
		return mapOfferRepositoryError(tx.Create(duplicate).Error)
	})
	if !errors.Is(err, domain.ErrOfferAlreadyExists) {
		t.Fatalf("expected the active offer index to reject a duplicate, got %v", err)
	}

	if err := db.Model(&domain.Offer{}).Where("offer_id = ?", first.OfferID).Update("offer_status", domain.OfferStatusWithdrawn).Error; err != nil {
		t.Fatal(err)
	}
	replacement := newIntegrationOffer("95")
	if err := repo.CreateOffer(ctx, marketerID, requestID, replacement); err != nil {
		t.Fatalf("a withdrawn offer must allow a new offer, got %v", err)
	}

	offers, err := repo.ListOffers(ctx, marketerID, requestID)
	if err != nil {
		t.Fatal(err)
	}
	if len(offers) != 2 || offers[0].OfferID != replacement.OfferID || offers[1].OfferStatus != domain.OfferStatusWithdrawn {
		t.Fatalf("expected the new offer first and the withdrawn one kept, got %+v", offers)
	}
}

func TestMarketerOfferRepositoryHidesOtherMarketersRequests(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	ctx := context.Background()
	creatorID := createIntegrationCreator(t, db, "offer-owner-creator")
	marketerID := createIntegrationMarketer(t, db, "offer-owner-marketer")
	otherMarketerID := createIntegrationMarketer(t, db, "offer-owner-other")
	surveyID := createIntegrationSurvey(t, db, creatorID, "offer-owner")
	_, requestID := createIntegrationJobRequest(t, db, creatorID, marketerID, surveyID)
	acceptIntegrationJobRequest(t, db, marketerID, requestID)
	repo := NewMarketerJobRequestRepository(db)

	if err := repo.CreateOffer(ctx, otherMarketerID, requestID, newIntegrationOffer("100")); !errors.Is(err, domain.ErrJobRequestNotFound) {
		t.Fatalf("expected another marketer's request to be hidden on create, got %v", err)
	}
	if _, err := repo.ListOffers(ctx, otherMarketerID, requestID); !errors.Is(err, domain.ErrJobRequestNotFound) {
		t.Fatalf("expected another marketer's request to be hidden on list, got %v", err)
	}
	if _, err := repo.FindForMarketer(ctx, otherMarketerID, requestID); !errors.Is(err, domain.ErrJobRequestNotFound) {
		t.Fatalf("expected another marketer's request to be hidden on find, got %v", err)
	}
	found, err := repo.FindForMarketer(ctx, marketerID, requestID)
	if err != nil {
		t.Fatal(err)
	}
	if found.Request.Status != domain.JobRequestStatusAccepted || len(found.Surveys) != 1 {
		t.Fatalf("unexpected request context: %+v", found)
	}
}
