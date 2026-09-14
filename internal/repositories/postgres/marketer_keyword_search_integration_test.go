package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// createIntegrationMarketerNamed is defined in marketer_repo_integration_test.go
// (added by PR#12) and reused here.

// createIntegrationServiceWithScope inserts a service with an optional
// scope_text, and an optional deleted_at so keyword-search tests can cover
// soft-deleted exclusion (createIntegrationService in
// service_repo_integration_test.go has no scope_text/soft-delete knobs).
func createIntegrationServiceWithScope(t *testing.T, db *gorm.DB, userID int32, serviceType, scopeText string, deleted bool) {
	t.Helper()
	var scope *string
	if scopeText != "" {
		scope = &scopeText
	}
	service := &domain.Service{
		UserID:      userID,
		ServiceType: serviceType,
		ScopeText:   scope,
		Price:       decimal.RequireFromString("100.00"),
		CreatedAt:   time.Now().UTC(),
	}
	if err := NewServiceRepository(db).Create(context.Background(), service); err != nil {
		t.Fatalf("create service fixture: %v", err)
	}
	if deleted {
		if err := db.Exec(`UPDATE services SET deleted_at = ? WHERE service_id = ?`, time.Now().UTC(), service.ServiceID).Error; err != nil {
			t.Fatalf("soft delete service fixture: %v", err)
		}
	}
}

// TestMarketerRepositorySearchKeywordEscapesLikeWildcards covers the PR#13
// review request to escape user-supplied % and _ so they are matched
// literally instead of acting as SQL LIKE wildcards.
func TestMarketerRepositorySearchKeywordEscapesLikeWildcards(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	ctx := context.Background()
	repo := NewMarketerProfileRepository(db)

	literal := createIntegrationMarketerNamed(t, db, "Has 100% Guarantee", "literal-percent")
	unrelated := createIntegrationMarketerNamed(t, db, "Unrelated Guarantee", "unrelated")

	page, err := repo.Search(ctx, ports.MarketerSearchQuery{Page: 1, PageSize: 50, Keyword: "100%"})
	if err != nil {
		t.Fatal(err)
	}
	foundLiteral, foundUnrelated := false, false
	for _, item := range page.Items {
		if item.Marketer.UserID == literal {
			foundLiteral = true
		}
		if item.Marketer.UserID == unrelated {
			foundUnrelated = true
		}
	}
	if !foundLiteral {
		t.Fatalf("expected literal '100%%' match to be found")
	}
	if foundUnrelated {
		t.Fatalf("expected '%%' to be treated literally, not as a wildcard matching every name")
	}
}

// TestMarketerRepositorySearchKeyword covers the PR#13 review request: match
// by name, bio, and service type/scope; case-insensitivity; excluding
// soft-deleted services; de-duplicating a marketer with several matching
// services; and correct total/pagination for a keyword search.
func TestMarketerRepositorySearchKeyword(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	ctx := context.Background()
	repo := NewMarketerProfileRepository(db)

	byName := createIntegrationMarketerNamed(t, db, "Keyword Zephyr Alpha", "by-name")
	byBio := createIntegrationMarketerNamed(t, db, "No Match Name", "by-bio")
	if err := db.Exec(`UPDATE marketers SET bio = ? WHERE user_id = ?`, "Specializes in ZEPHYR data collection.", byBio).Error; err != nil {
		t.Fatalf("set bio fixture: %v", err)
	}
	byService := createIntegrationMarketerNamed(t, db, "No Match Either", "by-service")
	createIntegrationServiceWithScope(t, db, byService, "Zephyr Survey Distribution", "", false)
	multiService := createIntegrationMarketerNamed(t, db, "Multi Service", "multi")
	createIntegrationServiceWithScope(t, db, multiService, "Zephyr Type One", "", false)
	createIntegrationServiceWithScope(t, db, multiService, "Other Type", "zephyr scope note", false)
	softDeletedOnly := createIntegrationMarketerNamed(t, db, "Soft Deleted Only", "soft-deleted")
	createIntegrationServiceWithScope(t, db, softDeletedOnly, "Zephyr Hidden Type", "", true)
	noMatch := createIntegrationMarketerNamed(t, db, "Totally Unrelated", "no-match")
	_ = noMatch

	search := func(keyword string) ports.MarketerPage {
		page, err := repo.Search(ctx, ports.MarketerSearchQuery{Page: 1, PageSize: 50, Keyword: keyword})
		if err != nil {
			t.Fatalf("search %q: %v", keyword, err)
		}
		return page
	}
	containsUserID := func(page ports.MarketerPage, userID int32) bool {
		for _, item := range page.Items {
			if item.Marketer.UserID == userID {
				return true
			}
		}
		return false
	}

	t.Run("matches by name", func(t *testing.T) {
		page := search("zephyr alpha")
		if !containsUserID(page, byName) {
			t.Fatalf("expected name match, got %+v", page.Items)
		}
	})

	t.Run("matches by bio", func(t *testing.T) {
		page := search("zephyr")
		if !containsUserID(page, byBio) {
			t.Fatalf("expected bio match")
		}
	})

	t.Run("matches by service type and scope, case-insensitive", func(t *testing.T) {
		page := search("ZEPHYR")
		if !containsUserID(page, byService) {
			t.Fatalf("expected service_type match")
		}
		if !containsUserID(page, multiService) {
			t.Fatalf("expected scope_text match")
		}
	})

	t.Run("excludes soft-deleted services", func(t *testing.T) {
		page := search("hidden type")
		if containsUserID(page, softDeletedOnly) {
			t.Fatalf("expected soft-deleted service not to match")
		}
	})

	t.Run("de-duplicates a marketer with multiple matching services", func(t *testing.T) {
		page := search("zephyr")
		count := 0
		for _, item := range page.Items {
			if item.Marketer.UserID == multiService {
				count++
			}
		}
		if count != 1 {
			t.Fatalf("expected marketer with 2 matching services to appear once, appeared %d times", count)
		}
	})

	t.Run("total and pagination reflect the keyword filter", func(t *testing.T) {
		page := search("zephyr")
		// byName, byBio, byService, multiService all match "zephyr"; noMatch and
		// softDeletedOnly (matching only via its soft-deleted service) do not.
		matching := map[int32]bool{byName: true, byBio: true, byService: true, multiService: true}
		got := 0
		for _, item := range page.Items {
			if matching[item.Marketer.UserID] {
				got++
			}
		}
		if got != len(matching) {
			t.Fatalf("expected all %d known matches present, found %d among %d total", len(matching), got, page.Total)
		}

		small, err := repo.Search(ctx, ports.MarketerSearchQuery{Page: 1, PageSize: 2, Keyword: "zephyr"})
		if err != nil {
			t.Fatal(err)
		}
		if len(small.Items) != 2 {
			t.Fatalf("expected page_size to be honored, got %d items", len(small.Items))
		}
		if small.Total != page.Total {
			t.Fatalf("expected total to stay stable across page sizes, got %d vs %d", small.Total, page.Total)
		}
	})
}
