package logging

import (
	"io"
	"log/slog"
)

// A Format selects one of the built-in output formats.
type Format uint8

const (
	// FormatText writes slog's text format (key=value pairs). It is the default.
	FormatText Format = iota

	// FormatJSON writes slog's JSON format, one object per line.
	FormatJSON
)

// A HandlerFunc builds the slog.Handler a logger writes through, for output
// formats other than the built-in ones. opts carries the configured level and
// a ReplaceAttr that names the custom FATAL and PANIC levels; implementations
// should honour both, running any ReplaceAttr of their own after
// opts.ReplaceAttr.
type HandlerFunc func(w io.Writer, opts *slog.HandlerOptions) slog.Handler

// Config is a logging configuration.
type Config struct {
	// Level is the lowest level of log message that should be emitted. Any log
	// messages logged at the specified level or any level more severe will be
	// emitted. The default level is INFO.
	Level Level

	// Output is the destination for log messages. By default, it is os.Stderr.
	Output io.Writer

	// Format is the built-in output format. The default is FormatText. It is
	// ignored when Handler is set.
	Format Format

	// Handler, if set, builds the handler for any other output format. It takes
	// precedence over Format.
	Handler HandlerFunc
}
