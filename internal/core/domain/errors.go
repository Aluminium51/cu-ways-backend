package domain

import "errors"

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrEmailAlreadyExists = errors.New("email already exists")
	ErrUserForbidden      = errors.New("user access forbidden")
	ErrInvalidUser        = errors.New("invalid user")
	ErrNoUserChanges      = errors.New("no user changes supplied")
	ErrInvalidCredentials = errors.New("invalid credentials")
)
