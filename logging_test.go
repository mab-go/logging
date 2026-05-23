package logging

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"
)

func TestLevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	log := NewLogger(Config{Level: WarnLevel, Output: &buf})

	log.Debug("debug-msg")
	log.Info("info-msg")
	if buf.Len() != 0 {
		t.Errorf("expected debug/info to be suppressed at WarnLevel, got: %s", buf.String())
	}

	log.Warn("warn-msg")
	if !strings.Contains(buf.String(), "warn-msg") {
		t.Errorf("expected warn message to appear, got: %s", buf.String())
	}

	buf.Reset()
	log.Error("error-msg")
	if !strings.Contains(buf.String(), "error-msg") {
		t.Errorf("expected error message to appear, got: %s", buf.String())
	}
}

func TestFormatArgs(t *testing.T) {
	cases := []struct {
		name string
		args []any
		want string
	}{
		{"zero", nil, ""},
		{"single string", []any{"hello"}, "hello"},
		{"single non-string", []any{42}, "42"},
		{"format string", []any{"hello %s, %d", "world", 7}, "hello world, 7"},
		{"multi non-format", []any{1, 2, 3}, "1 2 3"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatArgs(tc.args); got != tc.want {
				t.Errorf("formatArgs(%v) = %q; want %q", tc.args, got, tc.want)
			}
		})
	}
}

func TestWithErrorWrapped(t *testing.T) {
	var buf bytes.Buffer
	log := NewLogger(Config{Level: InfoLevel, Output: &buf})

	inner := errors.New("inner")
	wrapped := fmt.Errorf("wrap: %w", inner)
	log.WithError(wrapped).Info("oops")

	out := buf.String()
	if !strings.Contains(out, "error=") {
		t.Errorf("expected error= field, got: %s", out)
	}
	if !strings.Contains(out, "wrap: inner") {
		t.Errorf("expected wrapped error text, got: %s", out)
	}
}

func TestWithErrorNil(t *testing.T) {
	var buf bytes.Buffer
	log := NewLogger(Config{Level: InfoLevel, Output: &buf})

	log.WithError(nil).Info("no error")

	out := buf.String()
	if strings.Contains(out, "error=") {
		t.Errorf("expected no error field for nil error, got: %s", out)
	}
}

func TestCopyIndependence(t *testing.T) {
	var buf bytes.Buffer
	base := NewLogger(Config{Level: InfoLevel, Output: &buf}).WithField("base", "yes")
	cp := base.Copy().WithField("copy", "yes")

	buf.Reset()
	base.Info("base-msg")
	out1 := buf.String()
	if strings.Contains(out1, "copy=") {
		t.Errorf("base logger leaked copy field: %s", out1)
	}
	if !strings.Contains(out1, "base=yes") {
		t.Errorf("base logger missing base field: %s", out1)
	}

	buf.Reset()
	cp.Info("copy-msg")
	out2 := buf.String()
	if !strings.Contains(out2, "base=yes") || !strings.Contains(out2, "copy=yes") {
		t.Errorf("copy missing fields: %s", out2)
	}
}

func TestSetDefaultConfigResets(t *testing.T) {
	origConfig := defaultConfig
	origLogger := defaultLogger
	t.Cleanup(func() {
		defaultConfig = origConfig
		defaultLogger = origLogger
	})

	var buf bytes.Buffer
	SetDefaultConfig(Config{Level: WarnLevel, Output: &buf})

	Info("should be suppressed")
	if buf.Len() != 0 {
		t.Errorf("expected info to be suppressed at WarnLevel, got: %s", buf.String())
	}

	Warn("should appear")
	if !strings.Contains(buf.String(), "should appear") {
		t.Errorf("expected warn to appear, got: %s", buf.String())
	}
}

func TestFatalCallsExitAndLogs(t *testing.T) {
	origExit := exitFunc
	t.Cleanup(func() { exitFunc = origExit })

	var (
		called bool
		code   int
	)
	exitFunc = func(c int) { called = true; code = c }

	var buf bytes.Buffer
	log := NewLogger(Config{Level: DebugLevel, Output: &buf})
	// exitFunc is overridden above, so control returns here.
	log.Fatal(Event("crash"), "boom") //nolint:revive // see comment above

	if !called {
		t.Fatal("exitFunc not called by Fatal")
	}
	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}

	out := buf.String()
	if !strings.Contains(out, "level=FATAL") {
		t.Errorf("expected level=FATAL, got: %s", out)
	}
	if !strings.Contains(out, "event=crash") {
		t.Errorf("expected event=crash, got: %s", out)
	}
	if !strings.Contains(out, "boom") {
		t.Errorf("expected message text, got: %s", out)
	}
}

// TestPanicDefaultBehavior is the only test that does NOT override panicFunc;
// it verifies the default behavior actually panics with the formatted message.
func TestPanicDefaultBehavior(t *testing.T) {
	var buf bytes.Buffer
	log := NewLogger(Config{Level: DebugLevel, Output: &buf})

	defer func() {
		r := recover()
		if r == nil {
			t.Error("expected panic from default panicFunc, got none")

			return
		}

		msg, ok := r.(string)
		if !ok {
			t.Errorf("expected string panic value, got %T: %v", r, r)

			return
		}
		if msg != "boom 42" {
			t.Errorf("expected panic value %q, got %q", "boom 42", msg)
		}
		if !strings.Contains(buf.String(), "boom 42") {
			t.Errorf("expected log output to contain message, got: %s", buf.String())
		}
	}()

	log.Panic("boom %d", 42)
}

func TestPanicCallsPanicAndLogs(t *testing.T) {
	origPanic := panicFunc
	t.Cleanup(func() { panicFunc = origPanic })

	var captured string
	panicFunc = func(msg string) { captured = msg }

	var buf bytes.Buffer
	log := NewLogger(Config{Level: DebugLevel, Output: &buf})
	// panicFunc is overridden above, so control returns here.
	log.Panic(Event("explode"), "kaboom %d", 7) //nolint:revive // see comment above

	out := buf.String()
	if !strings.Contains(out, "level=PANIC") {
		t.Errorf("expected level=PANIC, got: %s", out)
	}
	if !strings.Contains(out, "event=explode") {
		t.Errorf("expected event=explode, got: %s", out)
	}

	if captured != "kaboom 7" {
		t.Errorf("expected captured panic msg %q, got %q", "kaboom 7", captured)
	}
}

func TestEventAttrHelper(t *testing.T) {
	e := Event("system.ready")
	attr := e.Attr()
	if attr.Key != "event" {
		t.Errorf("expected attr key 'event', got %q", attr.Key)
	}
	if got := attr.Value.String(); got != "system.ready" {
		t.Errorf("expected attr value 'system.ready', got %q", got)
	}

	if e.String() != "system.ready" {
		t.Errorf("Event.String() = %q; want %q", e.String(), "system.ready")
	}
}

func TestLevelString(t *testing.T) {
	cases := []struct {
		lvl  Level
		want string
	}{
		{DebugLevel, "DEBUG"},
		{InfoLevel, "INFO"},
		{WarnLevel, "WARN"},
		{ErrorLevel, "ERROR"},
		{FatalLevel, "FATAL"},
		{PanicLevel, "PANIC"},
	}
	for _, tc := range cases {
		if got := tc.lvl.String(); got != tc.want {
			t.Errorf("Level(%d).String() = %q; want %q", tc.lvl, got, tc.want)
		}
	}
}

func TestLevelStringPanicsOnInvalid(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for invalid Level value")
		}
	}()
	_ = Level(99).String()
}

func TestLevelSlogLevelPanicsOnInvalid(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for invalid Level value")
		}
	}()
	_ = Level(99).slogLevel()
}

func TestReplaceLevelNameIgnoresNonLevelKey(t *testing.T) {
	a := slog.String("other", "value")
	got := replaceLevelName(nil, a)
	if got.Key != "other" || got.Value.String() != "value" {
		t.Errorf("expected attr returned unchanged for non-level key, got %+v", got)
	}
}

func TestReplaceLevelNameIgnoresNonLevelValue(t *testing.T) {
	a := slog.String(slog.LevelKey, "weird")
	got := replaceLevelName(nil, a)
	if got.Key != slog.LevelKey || got.Value.String() != "weird" {
		t.Errorf("expected attr returned unchanged for non-Level value, got %+v", got)
	}
}

func TestReplaceLevelNamePassesThroughStandardLevels(t *testing.T) {
	a := slog.Any(slog.LevelKey, slog.LevelInfo)
	got := replaceLevelName(nil, a)
	if lvl, ok := got.Value.Any().(slog.Level); !ok || lvl != slog.LevelInfo {
		t.Errorf("expected standard level attr unchanged, got %+v", got)
	}
}

func TestReplaceLevelNamePassesThroughUnknownLevel(t *testing.T) {
	a := slog.Any(slog.LevelKey, slog.Level(42))
	got := replaceLevelName(nil, a)
	if lvl, ok := got.Value.Any().(slog.Level); !ok || lvl != slog.Level(42) {
		t.Errorf("expected unknown-level attr unchanged, got %+v", got)
	}
}

func captureDefaultLogger(t *testing.T) *bytes.Buffer {
	t.Helper()
	origCfg := defaultConfig
	origLogger := defaultLogger
	t.Cleanup(func() {
		defaultConfig = origCfg
		defaultLogger = origLogger
	})

	var buf bytes.Buffer
	SetDefaultConfig(Config{Level: DebugLevel, Output: &buf})

	return &buf
}

func TestPackageLevelForwarders(t *testing.T) {
	cases := []struct {
		name string
		emit func()
		want []string
	}{
		{"Debug", func() { Debug("d-msg") }, []string{"d-msg"}},
		{"Info", func() { Info("i-msg") }, []string{"i-msg"}},
		{"Warn", func() { Warn("w-msg") }, []string{"w-msg"}},
		{"Error", func() { Error("e-msg") }, []string{"e-msg"}},
		{"WithField", func() { WithField("k", "v").Info("wf-msg") }, []string{"wf-msg", "k=v"}},
		{"WithFields", func() { WithFields(Fields{"a": 1}).Info("wfs-msg") }, []string{"wfs-msg", "a=1"}},
		{"WithError", func() { WithError(errors.New("boom")).Info("we-msg") }, []string{"we-msg", "error=boom"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			buf := captureDefaultLogger(t)
			tc.emit()
			out := buf.String()
			for _, want := range tc.want {
				if !strings.Contains(out, want) {
					t.Errorf("%s forwarded incorrectly (missing %q): %s", tc.name, want, out)
				}
			}
		})
	}
}

func TestPackageLevelFatal(t *testing.T) {
	buf := captureDefaultLogger(t)
	origExit := exitFunc
	t.Cleanup(func() { exitFunc = origExit })

	var called bool
	exitFunc = func(int) { called = true }
	// exitFunc is overridden above, so control returns here.
	Fatal("f-msg") //nolint:revive // see comment above

	if !called {
		t.Error("Fatal did not invoke exitFunc")
	}

	if !strings.Contains(buf.String(), "f-msg") {
		t.Errorf("Fatal forwarded incorrectly: %s", buf.String())
	}
}

func TestPackageLevelPanic(t *testing.T) {
	buf := captureDefaultLogger(t)
	origPanic := panicFunc
	t.Cleanup(func() { panicFunc = origPanic })

	var captured string
	panicFunc = func(s string) { captured = s }
	// panicFunc is overridden above, so control returns here.
	Panic("p-msg") //nolint:revive // see comment above

	if captured != "p-msg" {
		t.Errorf("Panic forwarded incorrectly: captured=%q", captured)
	}

	if !strings.Contains(buf.String(), "p-msg") {
		t.Errorf("Panic logged incorrectly: %s", buf.String())
	}
}

// Example demonstrates the package-level default logger.
func Example() {
	logger := NewLogger(Config{Level: InfoLevel, Output: os.Stdout})
	logger.Info("hello")
	// Output not asserted (timestamp varies).
}

// ExampleLogger_Info shows logging with an Event and a formatted message.
func ExampleLogger_Info() {
	logger := NewLogger(Config{Level: InfoLevel, Output: os.Stdout})
	logger.Info(Event("agent.wake"), "starting cycle %d", 7)
	// Output not asserted (timestamp varies).
}

// ExampleLogger_WithError shows attaching an error to a structured log.
func ExampleLogger_WithError() {
	logger := NewLogger(Config{Level: InfoLevel, Output: os.Stdout})
	err := errors.New("connection refused")
	logger.WithError(err).Error(Event("db.connect"))
	// Output not asserted (timestamp varies).
}
