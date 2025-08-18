package bbgo

import (
	"os"

	log "github.com/sirupsen/logrus"
	prefixed "github.com/x-cray/logrus-prefixed-formatter"
)

func GetCurrentEnv() string {
	env := os.Getenv("BBGO_ENV")
	if env == "" {
		env = "development"
	}

	return env
}

func NewLogFormatterWithEnv(env string) log.Formatter {
	switch env {
	case "production", "prod", "stag", "staging":
		// always use json formatter for production and staging
		return &log.JSONFormatter{}
	}

	return &prefixed.TextFormatter{}
}

type LogFormatterType string

const (
	LogFormatterTypePrefixed LogFormatterType = "prefixed"
	LogFormatterTypeText     LogFormatterType = "text"
	LogFormatterTypeJson     LogFormatterType = "json"
)

func NewLogFormatter(logFormatter LogFormatterType) log.Formatter {
	switch logFormatter {
	case LogFormatterTypePrefixed:
		return &prefixed.TextFormatter{}
	case LogFormatterTypeText:
		return &log.TextFormatter{}
	case LogFormatterTypeJson:
		return &log.JSONFormatter{}
	}

	return &prefixed.TextFormatter{}
}
