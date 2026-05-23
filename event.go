package logging

import "log/slog"

// Event identifies a loggable occurrence. Each package that uses this logging
// library is expected to declare its events as unexported constants (e.g. in an
// events.go file).
//
// When an Event is passed as the first argument to a Logger's level methods
// (Debug, Info, Warn, etc.), the logger lifts it into a structured "event"
// field and uses the event name as the log message (unless additional
// arguments are provided to override it).
type Event string

// String returns the event name as a string.
func (e Event) String() string { return string(e) }

// Attr returns a slog attribute representing this event (key "event").
func (e Event) Attr() slog.Attr { return slog.String("event", string(e)) }
