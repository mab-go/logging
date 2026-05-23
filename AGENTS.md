# AGENTS.md — logging

This file is the authoritative briefing for any AI agent working on this
project. Read it in full before writing any code or making any changes.

---

## Project Summary

A small, general-purpose Go library for structured logging, backed by `log/slog`
from the standard library. The module path is `github.com/mab-go/logging`. Go
1.21+ is required (the floor is driven by `log/slog`).

The public surface is intentionally small: a `Logger` interface with level
methods and field-attachment helpers, a typed `Event` value for tagging records
with a stable identifier, context utilities for injecting a logger into a
request, and a package-level default logger for convenience.

The library has **no external dependencies** — only the Go standard library.

---

## Project Structure

```
doc.go                — package-level godoc only; no logic
interface.go          — Logger interface + Fields type
logging.go            — slogLogger implementation, NewLogger, formatArgs, extractEvent,
                        Fatal/Panic test seams (exitFunc, panicFunc)
level.go              — Level type, public constants (DebugLevel..PanicLevel),
                        slogLevel() mapping, custom slog.Level values for FATAL/PANIC
config.go             — Config struct (Level, Output)
event.go              — type Event string with String() and Attr() helpers
default.go            — package-level defaultConfig/defaultLogger, SetDefaultConfig,
                        package-level forwarder functions (Debug, Info, ..., WithError)
context.go            — FromContext / NewContext using an unexported context key

logging_test.go       — level filtering, formatArgs, WithError nil/wrapped,
                        Copy independence, SetDefaultConfig reset, Fatal/Panic via
                        exitFunc/panicFunc seams, Event.Attr() / Event.String(),
                        Example* functions for godoc
event_test.go         — Event behavior across levels, field/message dual role,
                        empty Event handling
context_test.go       — round-trip and missing-key behavior

go.mod                — module github.com/mab-go/logging; go 1.21; no deps
README.md             — usage doc with paste-ready snippet
LICENSE               — MIT
Makefile              — build/test/lint targets (run 'make help' to see all)
.github/workflows/    — CI: test, lint, cyclomatic-complexity
.golangci.yaml        — golangci-lint v2 config
codecov.yml           — informational coverage status (non-blocking)
.editorconfig         — tabs for Go/Makefile, 2-space for everything else
.gitignore            — standard Go ignores
CLAUDE.md             — symlink → AGENTS.md
```

---

## Key Concepts

### Logger interface

```go
type Logger interface {
    Debug(args ...any); Info(args ...any); Warn(args ...any)
    Error(args ...any); Fatal(args ...any); Panic(args ...any)
    WithField(key string, value any) Logger
    WithFields(fields Fields) Logger
    WithError(err error) Logger
    Copy() Logger
}
```

The variadic `args ...any` mirrors logrus-style printf semantics: zero args →
empty message; one arg → `fmt.Sprint`; first arg is a `string` with more to
follow → `fmt.Sprintf`; otherwise → `fmt.Sprint` of all args.

### Event type

```go
type Event string
```

Each consuming package is expected to declare its events as unexported constants
(typically in an `events.go` file):

```go
const (
    eventStart logging.Event = "agent.start"
    eventStop  logging.Event = "agent.stop"
)
```

When an `Event` is the **first** argument to a level method, `extractEvent`
lifts it into a structured `event` field on the record. Two sub-cases:

- **Event alone** (`log.Info(eventStart)`) — the event name doubles as the
  message body.
- **Event with companion args** (`log.Info(eventStart, "cycle %d", 7)`) — the
  companion args determine the message; the event remains a structured field
  only.

An empty `Event("")` is treated as no event (no `event` field is emitted).
Anything other than `Event` as the first arg is treated as ordinary format args.

### Default logger

The package keeps an internal `defaultConfig` (default level: `InfoLevel`) and
`defaultLogger`. Package-level functions (`logging.Info`, `logging.WithField`,
etc.) forward to the default logger. `SetDefaultConfig` rebuilds the default
logger; tests that mutate it must save and restore the originals (see
`TestSetDefaultConfigResets` for the pattern).

### Context

`NewContext(ctx, logger)` stores a logger; `FromContext(ctx)` retrieves it
(returning a new default logger and `created=true` when absent). The context
key is unexported and collision-free.

---

## Calling Conventions

- **Event lifecycle**: declare unexported event constants per consuming package;
  pass the constant as the first arg to a level method. Don't allocate new
  `Event("...")` values inline at call sites — define them as constants.
- **`WithError(nil)` is a no-op** — returns the logger unchanged. This is a
  deliberate departure from logrus (which would emit `error=<nil>`). Callers
  that want to record a nil-error sentinel should attach an explicit field.
- **Format strings follow `fmt` rules**: if you pass extra args without `%`
  verbs in the format string, `fmt` will produce `%!(EXTRA ...)` — that's caller
  error, not library behavior.
- **Source attribution** is not enabled by default
  (`slog.HandlerOptions.AddSource` is false). If a future change enables it,
  note that `LogAttrs` is called from within the wrapper, so the captured PC
  points at this package rather than the caller; fixing that requires building
  records directly via `slog.NewRecord` with `runtime.Callers`.

---

## Testing

- All tests live alongside source files (`*_test.go` in `package logging`).
- Test inputs are inline Go values — no fixtures, no testdata, no helper
  packages.
- The library has zero external dependencies; tests use only `bytes`, `errors`,
  `fmt`, `os`, `strings`, `context`, `testing`.
- **Fatal and Panic** are tested via unexported package vars `exitFunc` and
  `panicFunc`. Tests override them with capturing closures (via `t.Cleanup` to
  restore originals) so the test binary is never terminated. Do not change this
  seam without updating the corresponding tests.
- **`Example*` functions** (`Example`, `ExampleLogger_Info`,
  `ExampleLogger_WithError`) exist for godoc discoverability. They do **not**
  carry `// Output:` comments because timestamps in slog output are
  non-deterministic; the functions still compile-check the API surface.

---

## Build & Lint

Treat any change as incomplete until **`make test lint`** passes.

| Command           | Purpose                                                 |
|-------------------|---------------------------------------------------------|
| `make test`       | `go test -race ./...`                                   |
| `make lint`       | golangci-lint (run `make setup` first if not installed) |
| `make fmt`        | goimports formatting                                    |
| `make vet`        | `go vet ./...`                                          |
| `make cyclo`      | gocyclo, threshold 10                                   |
| `make setup`      | install golangci-lint, goimports, gocyclo into `./bin`  |
| `make test:cover` | coverage report, opens HTML locally                     |
| `make help`       | full target list with descriptions                      |

Tool versions are pinned in the Makefile (golangci-lint v2.11.3, goimports
v0.38.0, gocyclo v0.6.0). If `make lint` surfaces findings, fix the code rather
than relaxing the config.

---

## Constraints

- **No external dependencies.** The module is stdlib-only; do not add imports
  outside `std`.
- **No app-specific sinks.** App-specific transports belong in the consuming
  app, typically as a custom `slog.Handler` that callers wire in.
- **No `cmd/` binaries.** This is a library; demo code lives in `Example*` test
  functions for godoc, plus a snippet in the README.
- **Go 1.21 floor.** Do not raise the `go` directive without a concrete reason;
  the floor exists to maximize consumer compatibility. Note that `go.mod` also
  carries a `toolchain go1.26.1` directive — this is **dev-time only** and does
  not constrain consumers. It exists because the pinned `golangci-lint` v2.11.3
  requires Go ≥ 1.25 to build; if you bump `golangci-lint` and it relaxes that
  requirement, the `toolchain` directive can be lowered or removed.

---

## Documentation Maintenance

After any structural change, update the corresponding documentation:

| Change made                                       | What to review/update                                          |
|---------------------------------------------------|----------------------------------------------------------------|
| New exported symbol added                         | godoc on the symbol; mention in README if user-facing          |
| New file in the package                           | Project Structure tree in this file                            |
| Event semantics change                            | Key Concepts → Event type section and the README               |
| New test seam added (like `exitFunc`/`panicFunc`) | Testing section in this file                                   |
| New Makefile target                               | Build & Lint table in this file                                |
| Go floor raised                                   | go.mod, README install section, and the Constraints note above |

Only update docs when something has genuinely changed — no cosmetic edits.
