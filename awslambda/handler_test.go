package awslambda

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mab-go/logging"
)

// The slog levels the logging package uses for FATAL and PANIC.
const (
	slogLevelFatal slog.Level = 12
	slogLevelPanic slog.Level = 16
)

// coreOpts returns the options logging.NewLogger passes to a HandlerFunc, so
// tests see the real FATAL and PANIC level names.
func coreOpts(t *testing.T) *slog.HandlerOptions {
	t.Helper()

	var opts *slog.HandlerOptions
	logging.NewLogger(logging.Config{
		Level:  logging.DebugLevel,
		Output: io.Discard,
		Handler: func(w io.Writer, o *slog.HandlerOptions) slog.Handler {
			opts = o

			return NewTextHandler(w, o)
		},
	})

	return opts
}

// outputLines splits buf into lines, logging each so -v shows sample output.
func outputLines(t *testing.T, buf *bytes.Buffer) []string {
	t.Helper()

	out := strings.TrimSuffix(buf.String(), "\n")
	if out == "" {
		return nil
	}
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		t.Log(line)
	}

	return lines
}

// splitHybrid checks that line has four tab-separated fields and a JSON fourth
// field, and returns the fields and the decoded JSON.
func splitHybrid(t *testing.T, line string) ([]string, map[string]any) {
	t.Helper()

	fields := strings.SplitN(line, "\t", 4)
	if len(fields) != 4 {
		t.Fatalf("expected 4 tab-separated fields, got %d: %q", len(fields), line)
	}

	return fields, decodeJSON(t, fields[3])
}

// decodeJSON unmarshals one JSON object.
func decodeJSON(t *testing.T, s string) map[string]any {
	t.Helper()

	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatalf("not a JSON object: %v: %q", err, s)
	}

	return m
}

func TestTextHandlerLevels(t *testing.T) {
	t.Parallel()

	cases := []struct {
		level slog.Level
		want  string
	}{
		{slog.LevelDebug, "DEBUG"},
		{slog.LevelInfo, "INFO"},
		{slog.LevelWarn, "WARN"},
		{slog.LevelError, "ERROR"},
		{slogLevelFatal, "FATAL"},
		{slogLevelPanic, "PANIC"},
	}

	var buf bytes.Buffer
	log := slog.New(NewTextHandler(&buf, coreOpts(t)))
	for _, tc := range cases {
		log.Log(context.Background(), tc.level, "m")
	}

	lines := outputLines(t, &buf)
	if len(lines) != len(cases) {
		t.Fatalf("expected %d lines, got %d", len(cases), len(lines))
	}
	for i, tc := range cases {
		fields, body := splitHybrid(t, lines[i])
		if want := "[" + tc.want + "]"; fields[0] != want {
			t.Errorf("prefix level = %q; want %q", fields[0], want)
		}
		if _, err := time.Parse(timestampLayout, fields[1]); err != nil {
			t.Errorf("prefix timestamp %q: %v", fields[1], err)
		}
		if body["level"] != tc.want {
			t.Errorf("JSON level = %v; want %s", body["level"], tc.want)
		}
	}
}

func TestTextHandlerThroughLogger(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	log := logging.NewLogger(logging.Config{Output: &buf, Handler: NewTextHandler}).
		WithField(RequestIDKey, "req-1").
		WithField("k", "v")

	log.Info(logging.Event("job.start"), "cycle %d", 7)
	func() {
		defer func() { _ = recover() }()
		log.Panic("boom")
	}()

	lines := outputLines(t, &buf)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}

	want := []map[string]any{
		{"level": "INFO", "message": "cycle 7", "event": "job.start", RequestIDKey: "req-1", "k": "v"},
		{"level": "PANIC", "message": "boom", RequestIDKey: "req-1", "k": "v"},
	}
	for i, line := range lines {
		fields, body := splitHybrid(t, line)
		if fields[2] != "req-1" {
			t.Errorf("line %d: prefix request ID = %q; want %q", i, fields[2], "req-1")
		}
		for k, v := range want[i] {
			if body[k] != v {
				t.Errorf("line %d: %s = %v; want %v", i, k, body[k], v)
			}
		}
		if _, ok := body["timestamp"]; !ok {
			t.Errorf("line %d: missing timestamp key", i)
		}
		for _, k := range []string{"time", "msg"} {
			if _, ok := body[k]; ok {
				t.Errorf("line %d: unexpected %q key", i, k)
			}
		}
	}
}

func TestTextHandlerRequestID(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		log  func(l *slog.Logger)
		want string
	}{
		{"from WithAttrs", func(l *slog.Logger) { l.With(RequestIDKey, "req-1").Info("m") }, "req-1"},
		{"record overrides", func(l *slog.Logger) { l.With(RequestIDKey, "req-1").Info("m", RequestIDKey, "req-2") }, "req-2"},
		{"ignored in group", func(l *slog.Logger) { l.WithGroup("g").With(RequestIDKey, "req-1").Info("m") }, ""},
		{"record in group", func(l *slog.Logger) { l.WithGroup("g").Info("m", RequestIDKey, "req-1") }, ""},
		{"absent", func(l *slog.Logger) { l.Info("m") }, ""},
	}
	for _, tc := range cases {
		tc := tc // go.mod is go 1.21: loop variables are shared across iterations
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			tc.log(slog.New(NewTextHandler(&buf, coreOpts(t))))

			lines := outputLines(t, &buf)
			if len(lines) != 1 {
				t.Fatalf("expected 1 line, got %d", len(lines))
			}
			if fields, _ := splitHybrid(t, lines[0]); fields[2] != tc.want {
				t.Errorf("prefix request ID = %q; want %q", fields[2], tc.want)
			}
		})
	}
}

func TestTextHandlerZeroTime(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	r := slog.NewRecord(time.Time{}, slog.LevelInfo, "m", 0)
	if err := NewTextHandler(&buf, nil).Handle(context.Background(), r); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	lines := outputLines(t, &buf)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	fields, body := splitHybrid(t, lines[0])
	if fields[1] != "" {
		t.Errorf("prefix timestamp = %q; want empty", fields[1])
	}
	if _, ok := body["timestamp"]; ok {
		t.Error("unexpected timestamp key for a zero time")
	}
}

func TestTextHandlerConcurrent(t *testing.T) {
	t.Parallel()

	const goroutines, perGoroutine = 8, 100

	var buf bytes.Buffer
	log := logging.NewLogger(logging.Config{Output: &buf, Handler: NewTextHandler}).
		WithField(RequestIDKey, "req-1")

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				log.WithField("g", g).Info("line %d", i)
			}
		}(g)
	}
	wg.Wait()

	lines := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
	if len(lines) != goroutines*perGoroutine {
		t.Fatalf("expected %d lines, got %d", goroutines*perGoroutine, len(lines))
	}
	for _, line := range lines {
		splitHybrid(t, line)
	}
}

func TestJSONHandlerKeys(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	log := slog.New(NewJSONHandler(&buf, coreOpts(t)))
	log.Log(context.Background(), slogLevelPanic, "boom", "k", "v")
	log.Log(context.Background(), slogLevelFatal, "bang")

	lines := outputLines(t, &buf)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}

	for i, msg := range []string{"boom", "bang"} {
		body := decodeJSON(t, lines[i])
		if body["level"] != "FATAL" {
			t.Errorf("line %d: level = %v; want FATAL", i, body["level"])
		}
		if body["message"] != msg {
			t.Errorf("line %d: message = %v; want %s", i, body["message"], msg)
		}
		ts, ok := body["timestamp"].(string)
		if !ok || !strings.HasSuffix(ts, "Z") {
			t.Errorf("line %d: timestamp = %v; want a UTC time", i, body["timestamp"])
		}
		for _, k := range []string{"time", "msg"} {
			if _, ok := body[k]; ok {
				t.Errorf("line %d: unexpected %q key", i, k)
			}
		}
	}
	if body := decodeJSON(t, lines[0]); body["k"] != "v" {
		t.Errorf("k = %v; want v", body["k"])
	}
}

func TestJSONHandlerGroupKeysKept(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	slog.New(NewJSONHandler(&buf, coreOpts(t))).WithGroup("g").Info("m", "time", "t1", "msg", "m1")

	lines := outputLines(t, &buf)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	group, ok := decodeJSON(t, lines[0])["g"].(map[string]any)
	if !ok {
		t.Fatalf("expected group g in %s", lines[0])
	}
	if group["time"] != "t1" || group["msg"] != "m1" {
		t.Errorf("group keys renamed: %v", group)
	}
}

func TestJSONHandlerRenamesTopLevelUserKeys(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	slog.New(NewJSONHandler(&buf, coreOpts(t))).Info("m", "msg", "user")

	out := buf.String()
	t.Log(out)
	if strings.Contains(out, `"msg"`) || !strings.Contains(out, `"message":"user"`) {
		t.Errorf("expected the top-level msg field renamed to message, got: %s", out)
	}
}

func TestNilOptions(t *testing.T) {
	t.Parallel()

	var text, js bytes.Buffer
	slog.New(NewTextHandler(&text, nil)).Info("m")
	slog.New(NewJSONHandler(&js, nil)).Info("m")

	textLines := outputLines(t, &text)
	if len(textLines) != 1 {
		t.Fatalf("text: expected 1 line, got %d", len(textLines))
	}
	if fields, body := splitHybrid(t, textLines[0]); fields[0] != "[INFO]" || body["message"] != "m" {
		t.Errorf("text: unexpected line %q", textLines[0])
	}

	jsLines := outputLines(t, &js)
	if len(jsLines) != 1 {
		t.Fatalf("json: expected 1 line, got %d", len(jsLines))
	}
	if body := decodeJSON(t, jsLines[0]); body["level"] != "INFO" || body["message"] != "m" {
		t.Errorf("json: unexpected line %q", jsLines[0])
	}
}
