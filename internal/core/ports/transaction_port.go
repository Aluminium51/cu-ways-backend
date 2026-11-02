package ports

import (
	"context"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
)

// TransactionQuery selects one page of the signed-in user's transaction
// history. UserID is always the authenticated user, never a request value.
type TransactionQuery struct {
	UserID   int32
	Role     domain.TransactionRole
	Keyword  string
	Page     int
	PageSize int
}

type TransactionPage struct {
	Items    []domain.Transaction
	Page     int
	PageSize int
	Total    int64
}

// TransactionRepository lists Completed Jobs with a Paid payment where the
// user is the owning Creator or the hired Marketer.
type TransactionRepository interface {
	List(ctx context.Context, query TransactionQuery) (TransactionPage, error)
}
