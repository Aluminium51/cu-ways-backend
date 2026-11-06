package httpapi

import "context"

// fakeMembership is the shared MembershipRepository fake for handler tests.
type fakeMembership struct{ creator, marketer bool }

func (f fakeMembership) IsCreator(context.Context, int32) (bool, error)  { return f.creator, nil }
func (f fakeMembership) IsMarketer(context.Context, int32) (bool, error) { return f.marketer, nil }
