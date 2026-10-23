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

	ErrJobNotFound             = errors.New("job not found")
	ErrJobForbidden            = errors.New("job access forbidden")
	ErrInvalidJob              = errors.New("invalid job")
	ErrNoJobSurveys            = errors.New("a job must have at least one survey")
	ErrJobSurveyNotFound       = errors.New("survey is not linked to this job")
	ErrJobSurveyAlreadyLinked  = errors.New("survey is already linked to this job")
	ErrLastJobSurvey           = errors.New("cannot remove the last survey from a job")
	ErrInvalidBriefURL         = errors.New("invalid brief URL")
	ErrInvalidJobRequest       = errors.New("invalid job request")
	ErrJobRequestAlreadyActive = errors.New("an active request already exists for this job and marketer")
	ErrMarketerNotFound        = errors.New("marketer not found")
	ErrJobRequestNotFound      = errors.New("job request not found")
	ErrJobRequestNotPending    = errors.New("job request is no longer pending")
	ErrJobRequestNotAccepted   = errors.New("accept the job request before submitting an offer")

	ErrInvalidOffer       = errors.New("invalid offer")
	ErrOfferAlreadyExists = errors.New("an active offer already exists for this job request")
	ErrOfferNotFound      = errors.New("offer not found")
	ErrOfferNotPending    = errors.New("offer is no longer pending")
	ErrJobNotPending      = errors.New("job is no longer open for offers")

	ErrJobNotAccepted        = errors.New("only an accepted job can be started")
	ErrAcceptedOfferMismatch = errors.New("accepted offer does not match the job")
)
