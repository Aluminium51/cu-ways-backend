package utils

import "github.com/go-playground/validator/v10"

var defaultValidator = validator.New()

func Validate(value any) error {
	return defaultValidator.Struct(value)
}
