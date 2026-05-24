# mab-go/logging

<p align="center">
  <a href="https://github.com/mab-go/logging/actions"><img src="https://img.shields.io/github/check-runs/mab-go/logging/main?style=flat&labelColor=555555&label=checks" alt="Build Status" /></a>
  <a href="https://goreportcard.com/report/github.com/mab-go/logging"><img src="https://goreportcard.com/badge/github.com/mab-go/logging" alt="Go Report Card" /></a>
  <a href="https://pkg.go.dev/github.com/mab-go/logging"><img src="https://img.shields.io/badge/-reference-00ADD8?style=flat&logo=go&logoColor=white&labelColor=555555" alt="Go Reference" /></a>
  <a href="https://deepwiki.com/mab-go/logging"><img src="https://img.shields.io/badge/DeepWiki-logging-blue?style=flat&logoColor=white&labelColor=555555" alt="Ask DeepWiki"></a>
  <a href="LICENSE"><img src="https://img.shields.io/github/license/mab-go/logging" alt="License: MIT" /></a>
</p>

A small structured-logging library built on `log/slog` (Go 1.21+ stdlib).
Provides a `Logger` interface with structured field support, context injection,
and an `Event` type for tagging records with a stable event identifier.

## Install

```
go get github.com/mab-go/logging
```

Requires Go 1.21 or newer.

## Quick start

```go
package main

import (
    "errors"

    "github.com/mab-go/logging"
)

const (
    eventAgentWake     logging.Event = "agent.wake"
    eventAgentFailed   logging.Event = "agent.failed"
)

func main() {
    // Optional: tweak the default config (level, output).
    logging.SetDefaultConfig(logging.Config{Level: logging.InfoLevel})

    // Package-level shortcuts use the default logger.
    logging.Info(eventAgentWake, "starting cycle %d", 7)

    // Or build your own logger and pass it around.
    log := logging.NewLogger(logging.Config{Level: logging.DebugLevel})
    log.WithField("cycle", 7).Info(eventAgentWake)

    if err := errors.New("connection refused"); err != nil {
        log.WithError(err).Error(eventAgentFailed)
    }
}
```

Output (timestamps elided):

```
level=INFO msg="starting cycle 7" event=agent.wake
level=INFO msg=agent.wake cycle=7 event=agent.wake
level=ERROR msg=agent.failed error="connection refused" event=agent.failed
```

## Events

Declare events as unexported constants in each package that uses logging,
typically in an `events.go` file:

```go
// internal/agent/events.go
package agent

import "github.com/mab-go/logging"

const (
    eventStart logging.Event = "agent.start"
    eventStop  logging.Event = "agent.stop"
    eventWake  logging.Event = "agent.wake"
)
```

When an `Event` is the first argument to a level method (`Debug`, `Info`,
etc.), it is lifted into a structured `event` field. If no additional
arguments are provided, the event name is also used as the message body.

```go
log.Info(eventWake)                  // msg=agent.wake event=agent.wake
log.Info(eventWake, "cycle %d", 7)   // msg="cycle 7" event=agent.wake
log.WithError(err).Error(eventStop)  // msg=agent.stop event=agent.stop error=...
```

## Context

Carry a logger through a request via context:

```go
ctx = logging.NewContext(ctx, log)

// later, deep in the call stack:
log, _ := logging.FromContext(ctx)
log.Info(eventStep, "doing work")
```

## API surface

```go
type Logger interface {
    Debug(args ...any); Info(args ...any); Warn(args ...any)
    Error(args ...any); Fatal(args ...any); Panic(args ...any)
    WithField(key string, value any) Logger
    WithFields(fields Fields) Logger
    WithError(err error) Logger
    Copy() Logger
}

type Config  struct{ Level Level; Output io.Writer }
type Event   string                                   // e.Attr(), e.String()
type Fields  map[string]any
type Level   uint8                                    // DebugLevel..PanicLevel

func NewLogger(Config) Logger
func NewDefaultLogger() Logger
func SetDefaultConfig(Config)
func FromContext(ctx) (Logger, bool)
func NewContext(ctx, Logger) ctx
// Plus package-level Debug/Info/.../Panic/WithField/WithFields/WithError
// forwarding to the default logger.
```
