package services

import (
	"context"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
)

const (
	DefaultTransactionPage     = 1
	DefaultTransactionPageSize = 10
	MaxTransactionPageSize     = 50
	// MaxTransactionPage matches the documented int32 page range and keeps (page-1)*page_size from overflowing into a negative offset.
	MaxTransactionPage = math.MaxInt32
)

type TransactionService struct {
	repo ports.TransactionRepository
}

func NewTransactionService(repo ports.TransactionRepository) *TransactionService {
	return &TransactionService{repo: repo}
}

// List returns the authenticated user's own transaction history. Visibility is
// enforced by the repository query using actor.UserID, so no request value can
// widen it to another user's jobs.
func (s *TransactionService) List(ctx context.Context, actor Actor, query ports.TransactionQuery) (ports.TransactionPage, error) {
	if actor.UserID < 1 {
		return ports.TransactionPage{}, domain.ErrUserForbidden
	}
	query.UserID = actor.UserID
	query.Keyword = strings.TrimSpace(query.Keyword)
	if !query.Role.IsValid() || utf8.RuneCountInString(query.Keyword) > MaxKeywordLength {
		return ports.TransactionPage{}, domain.ErrInvalidTransactionQuery
	}
	if query.Page < 1 {
		query.Page = DefaultTransactionPage
	}
	if query.PageSize < 1 {
		query.PageSize = DefaultTransactionPageSize
	}
	if query.Page > MaxTransactionPage || query.PageSize > MaxTransactionPageSize {
		return ports.TransactionPage{}, domain.ErrInvalidTransactionQuery
	}

	page, err := s.repo.List(ctx, query)
	if err != nil {
		return ports.TransactionPage{}, err
	}
	for i := range page.Items {
		page.Items[i].PayoutStatus = domain.PayoutStatusReleased
	}
	return page, nil
}
