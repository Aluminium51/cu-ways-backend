package ports

import "context"

type MembershipRepository interface {
	IsCreator(ctx context.Context, userID int32) (bool, error)
	IsMarketer(ctx context.Context, userID int32) (bool, error)
}
