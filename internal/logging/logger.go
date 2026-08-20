package logging

import (
	"os"

	"github.com/rs/zerolog"
)

func New(environment string) zerolog.Logger {
	writer := zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "2006-01-02T15:04:05Z07:00"}
	if environment == "production" {
		return zerolog.New(os.Stderr).With().Timestamp().Caller().Logger()
	}
	return zerolog.New(writer).With().Timestamp().Caller().Logger()
}
