package awslambda

import (
	"os"
	"strings"

	"github.com/mab-go/logging"
)

// Environment variables Lambda sets from the function's logging configuration.
const (
	envLogFormat = "AWS_LAMBDA_LOG_FORMAT"
	envLogLevel  = "AWS_LAMBDA_LOG_LEVEL"
)

// ConfigFromEnv returns a logging.Config that follows the function's Lambda
// logging configuration:
//
//   - AWS_LAMBDA_LOG_FORMAT "JSON" selects NewJSONHandler; anything else,
//     including "Text" or unset, selects NewTextHandler.
//   - AWS_LAMBDA_LOG_LEVEL sets the level: TRACE and DEBUG map to
//     logging.DebugLevel, then INFO, WARN, ERROR and FATAL to their
//     counterparts. Unset or unknown values give logging.InfoLevel.
//
// Both are matched case-insensitively. Output is left nil (standard error);
// Lambda captures standard output and standard error alike.
func ConfigFromEnv() logging.Config {
	return configFrom(os.Getenv)
}

// configFrom builds the Config from getenv, so tests need not set the
// process environment.
func configFrom(getenv func(string) string) logging.Config {
	config := logging.Config{
		Level:   parseLevel(getenv(envLogLevel)),
		Handler: NewTextHandler,
	}
	if strings.EqualFold(getenv(envLogFormat), "JSON") {
		config.Handler = NewJSONHandler
	}

	return config
}

// parseLevel maps an AWS_LAMBDA_LOG_LEVEL value to a logging.Level.
func parseLevel(s string) logging.Level {
	switch strings.ToUpper(s) {
	case "TRACE", "DEBUG":
		return logging.DebugLevel
	case "WARN":
		return logging.WarnLevel
	case "ERROR":
		return logging.ErrorLevel
	case "FATAL":
		return logging.FatalLevel
	default:
		return logging.InfoLevel
	}
}
