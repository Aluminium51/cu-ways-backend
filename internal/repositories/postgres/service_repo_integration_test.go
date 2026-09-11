package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
	"github.com/shopspring/decimal"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestServiceRepositoryRejectsCrossOwnerUpdateAndDelete(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	ctx := context.Background()
	ownerID := createIntegrationMarketer(t, db, "owner")
	otherID := createIntegrationMarketer(t, db, "other")
	service := createIntegrationService(t, db, ownerID, "Original", "500.00")
	repo := NewServiceRepository(db)

	_, err := repo.Update(ctx, otherID, service.ServiceID, ports.ServicePatch{
		PriceSet: true,
		Price:    "900.00",
	})
	if !errors.Is(err, domain.ErrServiceNotFound) {
		t.Fatalf("expected cross-owner update to return service not found, got %v", err)
	}
	if err := repo.Delete(ctx, otherID, service.ServiceID); !errors.Is(err, domain.ErrServiceNotFound) {
		t.Fatalf("expected cross-owner delete to return service not found, got %v", err)
	}

	var stored domain.Service
	if err := db.Where("service_id = ?", service.ServiceID).First(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Price.StringFixed(2) != "500.00" || stored.DeletedAt != nil {
		t.Fatalf("another marketer changed the service: %+v", stored)
	}
}

func TestServiceRepositorySoftDeleteExcludesServiceFromOwnerList(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	ownerID := createIntegrationMarketer(t, db, "list-owner")
	active := createIntegrationService(t, db, ownerID, "Active", "700.00")
	removed := createIntegrationService(t, db, ownerID, "Removed", "300.00")
	repo := NewServiceRepository(db)

	if err := repo.Delete(context.Background(), ownerID, removed.ServiceID); err != nil {
		t.Fatal(err)
	}
	items, err := repo.ListByMarketer(context.Background(), ownerID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ServiceID != active.ServiceID {
		t.Fatalf("expected only active service %d, got %+v", active.ServiceID, items)
	}

	var deletedAt *time.Time
	if err := db.Model(&domain.Service{}).
		Select("deleted_at").
		Where("service_id = ?", removed.ServiceID).
		Scan(&deletedAt).Error; err != nil {
		t.Fatal(err)
	}
	if deletedAt == nil {
		t.Fatal("expected service row to be retained with deleted_at set")
	}
}

func TestMarketerSearchExcludesSoftDeletedService(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	ownerID := createIntegrationMarketer(t, db, "search-owner")
	removed := createIntegrationService(t, db, ownerID, "Removed", "100.00")
	if err := NewServiceRepository(db).Delete(context.Background(), ownerID, removed.ServiceID); err != nil {
		t.Fatal(err)
	}

	maxPrice := decimal.RequireFromString("200.00")
	page, err := NewMarketerProfileRepository(db).Search(context.Background(), ports.MarketerSearchQuery{
		Page:     1,
		PageSize: 20,
		MaxPrice: &maxPrice,
	})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 0 || len(page.Items) != 0 {
		t.Fatalf("soft-deleted service made marketer match price search: %+v", page)
	}
}

func TestMarketerSearchSupportsAllSortDirectionsAndDeterministicOrdering(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	creatorID := createIntegrationCreator(t, db, "sort-creator")
	cheapHighRatingID := createIntegrationMarketer(t, db, "sort-cheap-high")
	cheapHighRatingTieID := createIntegrationMarketer(t, db, "sort-cheap-high-tie")
	expensiveLowRatingID := createIntegrationMarketer(t, db, "sort-expensive-low")
	mediumUnreviewedID := createIntegrationMarketer(t, db, "sort-medium-unreviewed")
	withoutServiceID := createIntegrationMarketer(t, db, "sort-without-service")

	createIntegrationService(t, db, cheapHighRatingID, "Cheap high-rated service", "100.00")
	createIntegrationService(t, db, cheapHighRatingTieID, "Cheap high-rated tie", "100.00")
	createIntegrationService(t, db, expensiveLowRatingID, "Expensive low-rated service", "300.00")
	createIntegrationService(t, db, mediumUnreviewedID, "Medium unreviewed service", "200.00")

	createStatisticsReview(t, db, createStatisticsJob(t, db, creatorID, cheapHighRatingID, domain.JobStatusCompleted), 4)
	createStatisticsReview(t, db, createStatisticsJob(t, db, creatorID, cheapHighRatingTieID, domain.JobStatusCompleted), 4)
	createStatisticsReview(t, db, createStatisticsJob(t, db, creatorID, expensiveLowRatingID, domain.JobStatusCompleted), 2)

	repo := NewMarketerProfileRepository(db)
	tests := []struct {
		name string
		sort string
		want []int32
	}{
		{name: "price ascending", sort: "price_asc", want: []int32{cheapHighRatingID, cheapHighRatingTieID, mediumUnreviewedID, expensiveLowRatingID, withoutServiceID}},
		{name: "price descending", sort: "price_desc", want: []int32{expensiveLowRatingID, mediumUnreviewedID, cheapHighRatingID, cheapHighRatingTieID, withoutServiceID}},
		{name: "rating ascending", sort: "rating_asc", want: []int32{expensiveLowRatingID, cheapHighRatingID, cheapHighRatingTieID, mediumUnreviewedID, withoutServiceID}},
		{name: "rating descending", sort: "rating_desc", want: []int32{cheapHighRatingID, cheapHighRatingTieID, expensiveLowRatingID, mediumUnreviewedID, withoutServiceID}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			page, err := repo.Search(context.Background(), ports.MarketerSearchQuery{Page: 1, PageSize: 20, Sort: tt.sort})
			if err != nil {
				t.Fatal(err)
			}
			if page.Total != int64(len(tt.want)) {
				t.Fatalf("expected total %d, got %d", len(tt.want), page.Total)
			}
			if got := marketerSearchResultIDs(page); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("expected order %v, got %v", tt.want, got)
			}
		})
	}

	page, err := repo.Search(context.Background(), ports.MarketerSearchQuery{Page: 2, PageSize: 2, Sort: "price_desc"})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 5 || page.Page != 2 || page.PageSize != 2 {
		t.Fatalf("unexpected pagination metadata: %+v", page)
	}
	if got, want := marketerSearchResultIDs(page), []int32{cheapHighRatingID, cheapHighRatingTieID}; !reflect.DeepEqual(got, want) {
		t.Fatalf("expected page 2 order %v, got %v", want, got)
	}
}

func marketerSearchResultIDs(page ports.MarketerPage) []int32 {
	ids := make([]int32, 0, len(page.Items))
	for _, item := range page.Items {
		ids = append(ids, item.Marketer.UserID)
	}
	return ids
}

func TestServiceRepositoryUpdateAdvancesUpdatedAt(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	ownerID := createIntegrationMarketer(t, db, "timestamp-owner")
	service := createIntegrationService(t, db, ownerID, "Original", "500.00")
	if err := db.Exec("SELECT pg_sleep(0.01)").Error; err != nil {
		t.Fatal(err)
	}
	updatedScope := "Updated scope"

	updated, err := NewServiceRepository(db).Update(context.Background(), ownerID, service.ServiceID, ports.ServicePatch{
		ScopeTextSet: true,
		ScopeText:    &updatedScope,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !updated.UpdatedAt.After(service.UpdatedAt) {
		t.Fatalf("expected updated_at after %s, got %s", service.UpdatedAt, updated.UpdatedAt)
	}
}

func openRepositoryIntegrationTransaction(t *testing.T) *gorm.DB {
	t.Helper()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set; skipping PostgreSQL integration test")
	}
	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("connect to PostgreSQL: %v", err)
	}
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatalf("begin transaction: %v", tx.Error)
	}
	t.Cleanup(func() {
		if err := tx.Rollback().Error; err != nil && !errors.Is(err, gorm.ErrInvalidTransaction) {
			t.Errorf("rollback integration transaction: %v", err)
		}
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	return tx
}

func createIntegrationMarketer(t *testing.T, db *gorm.DB, label string) int32 {
	t.Helper()
	var userID int32
	email := fmt.Sprintf("service-repo-%s-%d@example.invalid", label, time.Now().UnixNano())
	if err := db.Raw(`
INSERT INTO users (name, email, created_at)
VALUES (?, ?, ?)
RETURNING user_id`, "Repository Test", email, time.Now().UTC()).Scan(&userID).Error; err != nil {
		t.Fatalf("create user fixture: %v", err)
	}
	if err := db.Exec(`
INSERT INTO marketers (user_id, bio, experience_years, availability_status, availability_text)
VALUES (?, ?, ?, ?, ?)`, userID, "Test marketer", 1, domain.AvailabilityAvailable, "Available").Error; err != nil {
		t.Fatalf("create marketer fixture: %v", err)
	}
	return userID
}

func createIntegrationService(t *testing.T, db *gorm.DB, userID int32, serviceType, price string) *domain.Service {
	t.Helper()
	amount := decimal.RequireFromString(price)
	service := &domain.Service{
		UserID:      userID,
		ServiceType: serviceType,
		Price:       amount,
		CreatedAt:   time.Now().UTC(),
	}
	if err := NewServiceRepository(db).Create(context.Background(), service); err != nil {
		t.Fatalf("create service fixture: %v", err)
	}
	return service
}
