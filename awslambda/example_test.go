package awslambda_test

import (
	"github.com/mab-go/logging"
	"github.com/mab-go/logging/awslambda"
)

// Example builds a logger from the function's Lambda logging configuration and
// attaches the invocation's request ID. In a handler, the ID comes from
// lambdacontext.FromContext(ctx) in github.com/aws/aws-lambda-go.
func Example() {
	log := logging.NewLogger(awslambda.ConfigFromEnv())

	log.WithField(awslambda.RequestIDKey, "c6af9ac6-7b61-11e6-9a41-93e8deadbeef").
		Info("handling event")
}

// ExampleNewTextHandler selects the Text-format handler directly.
func ExampleNewTextHandler() {
	log := logging.NewLogger(logging.Config{Handler: awslambda.NewTextHandler})

	log.Info("cold start")
}
