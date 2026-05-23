// Package logging provides a small structured-logging API backed by log/slog.
//
// Callers emit messages via Debug/Info/Warn/Error/Fatal/Panic and attach
// structured context via WithField, WithFields, and WithError. An Event type
// lifts a leading typed argument into a structured "event" field, which is
// useful for filtering and aggregating log records by event identifier.
package logging
