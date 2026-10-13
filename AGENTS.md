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
logging.go            — slogLogger implementation, NewLogger, newHandler (built-in
                        formats), formatArgs, extractEvent, Fatal/Panic test seams
                        (exitFunc, panicFunc)
level.go              — Level type, public constants (DebugLevel..PanicLevel),
                        slogLevel() mapping, custom slog.Level values for FATAL/PANIC
config.go             — Config struct (Level, Output, Format, Handler), Format
                        constants, HandlerFunc type
event.go              — type Event string with String() and Attr() helpers
default.go            — package-level defaultConfig/defaultLogger, SetDefaultConfig,
                        package-level forwarder functions (Debug, Info, ..., WithError)
context.go            — FromContext / NewContext using an unexported context key

logging_test.go       — level filtering, formatArgs, WithError nil/wrapped,
                        Copy independence, SetDefaultConfig reset, Fatal/Panic via
                        exitFunc/panicFunc seams, Event.Attr() / Event.String(),
                        output formats and the Handler hook, Example* functions
                        for godoc
event_test.go         — Event behavior across levels, field/message dual role,
                        empty Event handling
context_test.go       — round-trip and missing-key behavior

awslambda/            — CloudWatch-friendly output for AWS Lambda (stdlib-only)
  doc.go              — package godoc: both Lambda log formats, wiring snippet
  handler.go          — RequestIDKey, NewTextHandler (tab hybrid), NewJSONHandler,
                        hybridHandler, lambdaKeys (timestamp/message, PANIC→FATAL)
  config.go           — ConfigFromEnv / configFrom (AWS_LAMBDA_LOG_FORMAT,
                        AWS_LAMBDA_LOG_LEVEL)
  handler_test.go     — prefix layout, levels, request ID, zero time, key renames,
                        nil options, concurrent writes
  config_test.go      — format and level from the environment
  example_test.go     — Example* functions for godoc (package awslambda_test)

go.mod                — module github.com/mab-go/logging; go 1.21; no deps
README.md             — usage doc with paste-ready snippet
LICENSE               — MIT
Makefile              — build/test/lint targets (run 'make help' to see all)
.github/workflows/    — CI: check (make targets + coverage), go1.21 floor; release
.golangci.yaml        — golangci-lint v2 config
codecov.yml           — informational coverage status (non-blocking)
.editorconfig         — tabs for Go/Makefile, 2-space for everything else
.gitignore            — standard Go ignores, plus Claude Code worktrees and local settings
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

### Output formats

`Config.Format` picks a built-in format: `FormatText` (the zero value, slog's
`key=value` text) or `FormatJSON` (slog JSON). `Config.Handler` is a
`HandlerFunc` that builds any other `slog.Handler`, and it takes precedence over
`Format`. `NewLogger` passes every handler the same `*slog.HandlerOptions`: the
configured level, plus `replaceLevelName` as `ReplaceAttr` so custom levels
render as `FATAL`/`PANIC`. A custom handler should chain its own `ReplaceAttr`
after that one. An invalid `Format` panics, like an invalid `Level`.

The `awslambda` subpackage supplies `HandlerFunc`s for AWS Lambda, one per
Lambda log format:

- **`NewTextHandler`** (Lambda's Text format, the default) writes
  `[LEVEL]\t<timestamp>\t<requestId>\t<JSON>`, the Python runtime's layout;
  CloudWatch parses the JSON in the fourth field. The JSON is the whole record
  (no deduplication with the prefix). Empty columns keep every line at four
  fields.
- **`NewJSONHandler`** (Lambda's JSON format) writes one object per line with
  Lambda's `timestamp`/`level`/`message` keys, and PANIC as FATAL because
  Lambda's level filtering has no PANIC.
- **`ConfigFromEnv`** picks the handler and level from `AWS_LAMBDA_LOG_FORMAT`
  and `AWS_LAMBDA_LOG_LEVEL`.

The request ID comes from a `RequestIDKey` field the app attaches, so the
package needs no aws-lambda-go import.

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

- All tests live alongside source files (`*_test.go` in `package logging`, or
  `package awslambda` under `awslambda/`). The exception is
  `awslambda/example_test.go`, in `package awslambda_test` so its examples read
  as caller code.
- Test inputs are inline Go values — no fixtures, no testdata, no helper
  packages.
- The library has zero external dependencies; tests use only `bytes`, `context`,
  `encoding/json`, `errors`, `fmt`, `io`, `log/slog`, `os`, `strings`,
  `sync`, `testing`, `time`, plus `logging` itself in `awslambda`.
- **Fatal and Panic** are tested via unexported package vars `exitFunc` and
  `panicFunc`. Tests override them with capturing closures (via `t.Cleanup` to
  restore originals) so the test binary is never terminated. Do not change this
  seam without updating the corresponding tests.
- **Tests call `t.Parallel()`** (enforced by `paralleltest` and `tparallel`).
  A test that mutates package state (`exitFunc`, `panicFunc`, the default
  logger) stays sequential, with `//nolint:paralleltest // <reason>` on its own
  line above the `func`. Parallel table subtests copy the loop variable
  (`tc := tc`): `go.mod` is `go 1.21`, where loop variables are shared across
  iterations.
- **`awslambda`** tests get the core's real `*slog.HandlerOptions` (with the
  FATAL/PANIC names) through a capturing `HandlerFunc` (`coreOpts`), and log
  FATAL through `slog` directly, since `Fatal` exits. Environment parsing is
  tested through `configFrom(getenv)` with a map, so only `TestConfigFromEnv`
  uses `t.Setenv` (paralleltest already exempts it; no `//nolint` needed).
- **`Example*` functions** (`Example`, `ExampleLogger_Info`,
  `ExampleLogger_WithError`, and in `awslambda` `Example` and
  `ExampleNewTextHandler`) exist for godoc discoverability. They do **not**
  carry `// Output:` comments because timestamps in slog output are
  non-deterministic; the functions still compile-check the API surface.

---

## Build & Lint

Treat any change as incomplete until **`make check`** passes (`mod:verify`,
`vet`, `lint`, `test`; run `make setup` once first).

| Command           | Purpose                                                 |
|-------------------|---------------------------------------------------------|
| `make check`      | everything CI runs: mod:verify, vet, lint, test         |
| `make test`       | `go test -race ./...`                                   |
| `make lint`       | golangci-lint (run `make setup` first if not installed) |
| `make fmt`        | goimports formatting                                    |
| `make vet`        | `go vet ./...`                                          |
| `make setup`      | install golangci-lint, goimports into `./bin`           |
| `make test:cover` | coverage report, opens HTML locally                     |
| `make help`       | full target list with descriptions                      |

Tool versions are pinned in the Makefile (golangci-lint v2.14.0, goimports
v0.50.0). If `make lint` surfaces findings, fix the code rather than relaxing
the config.

Complexity limits run inside `make lint`: gocyclo 10 and cyclop 12 (tests
included), plus gocognit and nestif (tests exempt). A `//nolint` must name its
linter and give a reason (`//nolint:paralleltest // mutates exitFunc`).

CI runs the same targets (`make setup`, then `mod:verify vet lint` and
`test:cover`), so tool versions are pinned only in the Makefile. Workflow
actions are pinned by commit SHA with the version in a trailing comment
(`@<sha> # v7.0.1`); bump both together.

---

## Constraints

- **No external dependencies.** The module is stdlib-only; do not add imports
  outside `std`.
- **No app-specific sinks.** App-specific transports belong in the consuming
  app, typically as a custom `slog.Handler` that callers wire in. Output
  formats for a platform (like `awslambda`) may live here as stdlib-only
  subpackages; they format records and write to `Config.Output`, nothing
  more.
- **No `cmd/` binaries.** This is a library; demo code lives in `Example*` test
  functions for godoc, plus a snippet in the README.
- **Go 1.21 floor.** Do not raise the `go` directive without a concrete reason;
  the floor exists to maximize consumer compatibility. Note that `go.mod` also
  carries a `toolchain go1.27.2` directive — this is **dev-time only** and does
  not constrain consumers. It exists because the pinned `golangci-lint` v2.14.0
  and `goimports` v0.50.0 require Go ≥ 1.26 to build; if you bump them and they
  relax that requirement, the `toolchain` directive can be lowered or removed.
  CI's `floor` job (go1.21 floor) runs `make vet test` on go1.21.13 with
  `GOTOOLCHAIN=local`, so the floor is tested, not just declared.

---

## Documentation Maintenance

After any structural change, update the corresponding documentation:

| Change made                                       | What to review/update                                                                                                                                 |
|---------------------------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------|
| New exported symbol added                         | godoc on the symbol; mention in README if user-facing                                                                                                 |
| New file in the package                           | Project Structure tree in this file                                                                                                                   |
| Event semantics change                            | Key Concepts → Event type section and the README                                                                                                      |
| New test seam added (like `exitFunc`/`panicFunc`) | Testing section in this file                                                                                                                          |
| New Makefile target                               | Build & Lint table in this file                                                                                                                       |
| Go floor raised                                   | go.mod, README install section, the Constraints note above, the CI `floor` job, and `.golangci.yaml` `run.go` (at 1.22+, enable copyloopvar/intrange) |

Only update docs when something has genuinely changed — no cosmetic edits.
