package services

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
)

type fakeTransactionRepository struct {
	page  ports.TransactionPage
	err   error
	query ports.TransactionQuery
	calls int
}

func (f *fakeTransactionRepository) List(_ context.Context, query ports.TransactionQuery) (ports.TransactionPage, error) {
	f.calls++
	f.query = query
	return f.page, f.err
}

func TestTransactionServiceScopesQueryToAuthenticatedUser(t *testing.T) {
	repo := &fakeTransactionRepository{page: ports.TransactionPage{Items: []domain.Transaction{{JobID: 1}, {JobID: 2}}, Total: 2}}
	service := NewTransactionService(repo)

	// A caller-supplied UserID must never survive: only the actor decides whose history is read.
	page, err := service.List(context.Background(), Actor{UserID: 17}, ports.TransactionQuery{UserID: 99, Role: domain.TransactionRoleCreator, Keyword: "  survey  "})
	if err != nil {
		t.Fatal(err)
	}
	if repo.query.UserID != 17 || repo.query.Role != domain.TransactionRoleCreator || repo.query.Keyword != "survey" {
		t.Fatalf("unexpected repository query: %+v", repo.query)
	}
	if repo.query.Page != DefaultTransactionPage || repo.query.PageSize != DefaultTransactionPageSize {
		t.Fatalf("expected default paging, got %+v", repo.query)
	}
	for _, item := range page.Items {
		if item.PayoutStatus != domain.PayoutStatusReleased {
			t.Fatalf("expected derived payout status, got %q", item.PayoutStatus)
		}
	}
}

func TestTransactionServiceRejectsInvalidQueries(t *testing.T) {
	for name, query := range map[string]ports.TransactionQuery{
		"unknown role":       {Role: "admin"},
		"keyword too long":   {Keyword: strings.Repeat("a", MaxKeywordLength+1)},
		"page size too high": {PageSize: MaxTransactionPageSize + 1},
	} {
		t.Run(name, func(t *testing.T) {
			repo := &fakeTransactionRepository{}
			_, err := NewTransactionService(repo).List(context.Background(), Actor{UserID: 17}, query)
			if !errors.Is(err, domain.ErrInvalidTransactionQuery) {
				t.Fatalf("expected invalid query, got %v", err)
			}
			if repo.calls != 0 {
				t.Fatal("repository must not be called for an invalid query")
			}
		})
	}
}

func TestTransactionServiceRequiresAuthenticatedUser(t *testing.T) {
	repo := &fakeTransactionRepository{}
	_, err := NewTransactionService(repo).List(context.Background(), Actor{}, ports.TransactionQuery{})
	if !errors.Is(err, domain.ErrUserForbidden) || repo.calls != 0 {
		t.Fatalf("expected forbidden without a repository call, got err=%v calls=%d", err, repo.calls)
	}
}

func TestTransactionServicePassesRepositoryErrorsThrough(t *testing.T) {
	boom := errors.New("database unavailable")
	_, err := NewTransactionService(&fakeTransactionRepository{err: boom}).List(context.Background(), Actor{UserID: 17}, ports.TransactionQuery{})
	if !errors.Is(err, boom) {
		t.Fatalf("expected repository error, got %v", err)
	}
}
