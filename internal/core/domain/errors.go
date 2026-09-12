package domain

import "errors"

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrEmailAlreadyExists = errors.New("email already exists")
	ErrPhoneAlreadyExists = errors.New("phone already exists")
	ErrUserForbidden      = errors.New("user access forbidden")
	ErrInvalidUser        = errors.New("invalid user")
	ErrNoUserChanges      = errors.New("no user changes supplied")
	ErrInvalidCredentials = errors.New("invalid credentials")

	ErrMarketerProfileNotFound = errors.New("marketer profile not found")
	ErrMarketerRequired        = errors.New("marketer profile required")
	ErrCreatorRequired         = errors.New("creator profile required")
	ErrInvalidMarketerProfile  = errors.New("invalid marketer profile")
	ErrInvalidAvailability     = errors.New("invalid availability")
	ErrCatalogOptionNotFound   = errors.New("catalog option not found")

	ErrServiceNotFound  = errors.New("service not found")
	ErrServiceForbidden = errors.New("service access forbidden")
	ErrInvalidService   = errors.New("invalid service")
	ErrNoServiceChanges = errors.New("no service changes supplied")
	ErrInvalidPrice     = errors.New("invalid price")

	ErrSurveyNotFound  = errors.New("survey not found")
	ErrSurveyForbidden = errors.New("survey access forbidden")
	ErrInvalidSurvey   = errors.New("invalid survey")
	ErrNoSurveyChanges = errors.New("no survey changes supplied")
	ErrSurveyInUse     = errors.New("survey is referenced by a job")
)
