// Package awslambda formats log output for AWS Lambda functions whose logs go
// to CloudWatch Logs. It provides handlers for the logging package's
// Config.Handler, one for each Lambda log format:
//
//   - NewTextHandler, for the Text log format (Lambda's default), writes
//     "[LEVEL]\t<timestamp>\t<request ID>\t<JSON>" lines, the layout of the
//     Python managed runtime. CloudWatch parses the JSON in the fourth field,
//     so its fields can be filtered and queried.
//   - NewJSONHandler, for the JSON log format, writes one JSON object per line
//     with the "timestamp", "level" and "message" keys Lambda reads, which is
//     what Lambda's application log-level filtering needs.
//
// ConfigFromEnv picks the handler and level from the environment variables
// Lambda sets from the function's logging configuration, so changing the log
// format in Lambda needs no code change:
//
//	log := logging.NewLogger(awslambda.ConfigFromEnv())
//
//	// Per invocation, with lambdacontext from github.com/aws/aws-lambda-go:
//	lc, _ := lambdacontext.FromContext(ctx)
//	ctx = logging.NewContext(ctx, log.WithField(awslambda.RequestIDKey, lc.AwsRequestID))
//
// Both handlers rename slog's built-in "time" and "msg" keys to "timestamp"
// and "message". slog can't tell those from top-level fields with the same
// names, so a field named "time" or "msg" is renamed too; fields inside a
// group are not.
//
// The package uses only the standard library and the logging package.
package awslambda
