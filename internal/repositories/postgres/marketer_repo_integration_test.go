package postgres

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
	"gorm.io/gorm"
)

// createIntegrationMarketerNamed inserts a user/marketer pair with a caller-chosen
// display name, so sort tie-break behavior (which orders by name) can be tested.
func createIntegrationMarketerNamed(t *testing.T, db *gorm.DB, name, label string) int32 {
	t.Helper()
	var userID int32
	email := fmt.Sprintf("marketer-repo-%s-%d@example.invalid", label, time.Now().UnixNano())
	if err := db.Raw(`
INSERT INTO users (name, email, created_at)
VALUES (?, ?, ?)
RETURNING user_id`, name, email, time.Now().UTC()).Scan(&userID).Error; err != nil {
		t.Fatalf("create user fixture: %v", err)
	}
	if err := db.Exec(`
INSERT INTO marketers (user_id, bio, experience_years, availability_status, availability_text)
VALUES (?, ?, ?, ?, ?)`, userID, "Test marketer", 1, domain.AvailabilityAvailable, "Available").Error; err != nil {
		t.Fatalf("create marketer fixture: %v", err)
	}
	return userID
}

// TestMarketerRepositorySearchBreaksTiesByNameThenUserID covers the PR#12 review
// request: with no price/rating to sort by, every sort mode must fall back to
// marketer name ascending, and same-named marketers must fall back further to
// user_id ascending for a fully deterministic order.
func TestMarketerRepositorySearchBreaksTiesByNameThenUserID(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	ctx := context.Background()

	zed := createIntegrationMarketerNamed(t, db, "Zed Tiebreak", "zed")
	amy1 := createIntegrationMarketerNamed(t, db, "Amy Tiebreak", "amy1")
	amy2 := createIntegrationMarketerNamed(t, db, "Amy Tiebreak", "amy2")

	// None of these marketers have services or reviews, so lowest_price and
	// average_rating are NULL for all three: every sort mode ties completely
	// on the primary key and must resolve through name, then user_id.
	wantOrder := []int32{amy1, amy2, zed}
	if amy2 < amy1 {
		wantOrder = []int32{amy2, amy1, zed}
	}

	repo := NewMarketerProfileRepository(db)
	for _, sort := range []string{"", "price_asc", "price_desc", "rating_asc", "rating_desc"} {
		t.Run("sort="+sort, func(t *testing.T) {
			page, err := repo.Search(ctx, ports.MarketerSearchQuery{Page: 1, PageSize: 50, Sort: sort})
			if err != nil {
				t.Fatal(err)
			}
			got := filterToFixtureOrder(page.Items, map[int32]bool{zed: true, amy1: true, amy2: true})
			if len(got) != 3 || got[0] != wantOrder[0] || got[1] != wantOrder[1] || got[2] != wantOrder[2] {
				t.Fatalf("expected order %v, got %v", wantOrder, got)
			}
		})
	}
}

func filterToFixtureOrder(items []domain.MarketerSearchResult, ids map[int32]bool) []int32 {
	result := make([]int32, 0, len(ids))
	for _, item := range items {
		if ids[item.Marketer.UserID] {
			result = append(result, item.Marketer.UserID)
		}
	}
	return result
}
