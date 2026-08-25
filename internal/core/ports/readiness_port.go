package ports

import "context"

type ReadinessChecker interface {
	Ping(context.Context) error
}
