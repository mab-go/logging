package logging

import (
	"context"
	"fmt"
	"log/slog"
	"os"
)

// Fields is a map of strings to any type. It is used to pass to Logger.WithFields.
type Fields map[string]any

// slogLogger is the default implementation of Logger, backed by log/slog.
type slogLogger struct {
	logger *slog.Logger
}

// exitFunc and panicFunc back the Fatal and Panic methods. They are exposed as
// package vars so tests can override them without terminating the test binary.
var (
	exitFunc  = os.Exit
	panicFunc = func(msg string) { panic(msg) }
)

// NewLogger returns a new Logger configured with config.
//
// Parameters:
//   - config: The configuration for the logger.
//
// Returns:
//   - Logger: A new Logger configured with the given configuration.
func NewLogger(config Config) Logger {
	output := config.Output
	if output == nil {
		output = os.Stderr
	}

	handler := slog.NewTextHandler(output, &slog.HandlerOptions{
		Level:       config.Level.slogLevel(),
		ReplaceAttr: replaceLevelName,
	})

	return &slogLogger{logger: slog.New(handler)}
}

// NewDefaultLogger returns a new logger that uses the package's default
// configuration.
//
// Returns:
//   - Logger: A new Logger configured with the default configuration.
func NewDefaultLogger() Logger {
	return NewLogger(defaultConfig)
}

// replaceLevelName rewrites the level attribute for the custom FATAL and PANIC
// levels, which slog would otherwise render as "ERROR+4" / "ERROR+8".
func replaceLevelName(_ []string, a slog.Attr) slog.Attr {
	if a.Key != slog.LevelKey {
		return a
	}

	lvl, ok := a.Value.Any().(slog.Level)
	if !ok {
		return a
	}

	switch lvl {
	case slogLevelFatal:
		return slog.String(slog.LevelKey, "FATAL")
	case slogLevelPanic:
		return slog.String(slog.LevelKey, "PANIC")
	case slog.LevelDebug, slog.LevelInfo, slog.LevelWarn, slog.LevelError:
		return a
	default:
		return a
	}
}

// Ensure that *slogLogger implements Logger.
var _ Logger = (*slogLogger)(nil)

// Debug emits a "DEBUG" level log message. If more than one argument is provided,
// the first argument is used as a string format template and the remaining arguments
// are used as string formatting parameters.
func (l *slogLogger) Debug(args ...any) { l.emit(DebugLevel, args) }

// Info emits an "INFO" level log message. If more than one argument is provided,
// the first argument is used as a string format template and the remaining arguments
// are used as string formatting parameters.
func (l *slogLogger) Info(args ...any) { l.emit(InfoLevel, args) }

// Warn emits a "WARN" level log message. If more than one argument is provided,
// the first argument is used as a string format template and the remaining arguments
// are used as string formatting parameters.
func (l *slogLogger) Warn(args ...any) { l.emit(WarnLevel, args) }

// Error emits an "ERROR" level log message. If more than one argument is provided,
// the first argument is used as a string format template and the remaining arguments
// are used as string formatting parameters.
func (l *slogLogger) Error(args ...any) { l.emit(ErrorLevel, args) }

// Fatal emits a "FATAL" level log message and then calls os.Exit(1). If more than
// one argument is provided, the first argument is used as a string format template
// and the remaining arguments are used as string formatting parameters.
func (l *slogLogger) Fatal(args ...any) {
	l.emit(FatalLevel, args)
	exitFunc(1)
}

// Panic emits a "PANIC" level log message and then panics. If more than one argument
// is provided, the first argument is used as a string format template and the remaining
// arguments are used as string formatting parameters.
func (l *slogLogger) Panic(args ...any) {
	msg := l.emit(PanicLevel, args)
	panicFunc(msg)
}

// emit performs Event extraction, formats the message, and writes the record.
// It returns the formatted message so Panic can pass it to the panic value.
func (l *slogLogger) emit(level Level, args []any) string {
	event, hasEvent, rest := extractEvent(args)
	msg := formatArgs(rest)

	if hasEvent {
		l.logger.LogAttrs(context.Background(), level.slogLevel(), msg, slog.String("event", event))
	} else {
		l.logger.LogAttrs(context.Background(), level.slogLevel(), msg)
	}

	return msg
}

// extractEvent checks whether the first arg is an Event. If so, it returns the
// event name, true, and the remaining args used to build the message:
//
//   - Event alone               -> message args = [eventName] (event doubles as the message)
//   - Event followed by N args  -> message args = those N args (event is the structured field only)
//
// Empty Event("") is treated as no event.
func extractEvent(args []any) (event string, ok bool, rest []any) {
	if len(args) == 0 {
		return "", false, args
	}
	if e, isEvt := args[0].(Event); isEvt && e != "" {
		if len(args) == 1 {
			return string(e), true, []any{string(e)}
		}

		return string(e), true, args[1:]
	}

	return "", false, args
}

// formatArgs mirrors logrus's variadic formatting semantics:
//   - 0 args         -> ""
//   - 1 arg          -> fmt.Sprint(arg)
//   - first is string-> fmt.Sprintf(args[0], args[1:]...)
//   - otherwise      -> fmt.Sprint(args...) (space-joined)
func formatArgs(args []any) string {
	switch len(args) {
	case 0:
		return ""
	case 1:
		return fmt.Sprint(args[0])
	}

	if s, ok := args[0].(string); ok {
		return fmt.Sprintf(s, args[1:]...)
	}

	return fmt.Sprint(args...)
}

// WithField adds a field to the logger and returns a new Logger.
func (l *slogLogger) WithField(key string, value any) Logger {
	return &slogLogger{logger: l.logger.With(key, value)}
}

// WithFields adds multiple fields to the logger and returns a new Logger.
func (l *slogLogger) WithFields(fields Fields) Logger {
	args := make([]any, 0, len(fields)*2)
	for k, v := range fields {
		args = append(args, k, v)
	}

	return &slogLogger{logger: l.logger.With(args...)}
}

// WithError adds a field called "error" to the logger and returns a new Logger.
// If err is nil, the logger is returned unchanged.
func (l *slogLogger) WithError(err error) Logger {
	if err == nil {
		return l
	}

	return &slogLogger{logger: l.logger.With(slog.Any("error", err))}
}

// Copy returns a copy of the logger.
func (l *slogLogger) Copy() Logger {
	return &slogLogger{logger: l.logger}
}
