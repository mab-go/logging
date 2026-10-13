package awslambda

import (
	"bytes"
	"testing"

	"github.com/mab-go/logging"
)

// mapEnv returns a getenv backed by m.
func mapEnv(m map[string]string) func(string) string {
	return func(key string) string { return m[key] }
}

// firstByte logs one INFO line through config and returns its first byte:
// '{' for NewJSONHandler, '[' for NewTextHandler.
func firstByte(t *testing.T, config logging.Config) byte {
	t.Helper()

	var buf bytes.Buffer
	config.Output = &buf
	config.Level = logging.InfoLevel
	logging.NewLogger(config).Info("m")
	if buf.Len() == 0 {
		t.Fatal("no output")
	}

	return buf.Bytes()[0]
}

func TestConfigFromFormat(t *testing.T) {
	t.Parallel()

	cases := []struct {
		format string
		want   byte
	}{
		{"JSON", '{'},
		{"json", '{'},
		{"Text", '['},
		{"", '['},
		{"garbage", '['},
	}
	for _, tc := range cases {
		tc := tc // go.mod is go 1.21: loop variables are shared across iterations
		t.Run(tc.format, func(t *testing.T) {
			t.Parallel()

			config := configFrom(mapEnv(map[string]string{envLogFormat: tc.format}))
			if got := firstByte(t, config); got != tc.want {
				t.Errorf("format %q: output starts with %q; want %q", tc.format, got, tc.want)
			}
		})
	}
}

func TestConfigFromLevel(t *testing.T) {
	t.Parallel()

	cases := []struct {
		level string
		want  logging.Level
	}{
		{"TRACE", logging.DebugLevel},
		{"debug", logging.DebugLevel},
		{"INFO", logging.InfoLevel},
		{"WARN", logging.WarnLevel},
		{"ERROR", logging.ErrorLevel},
		{"FATAL", logging.FatalLevel},
		{"", logging.InfoLevel},
		{"garbage", logging.InfoLevel},
	}
	for _, tc := range cases {
		config := configFrom(mapEnv(map[string]string{envLogLevel: tc.level}))
		if config.Level != tc.want {
			t.Errorf("level %q: got %v; want %v", tc.level, config.Level, tc.want)
		}
	}
}

func TestConfigFromEnv(t *testing.T) {
	t.Setenv(envLogFormat, "JSON")
	t.Setenv(envLogLevel, "WARN")

	config := ConfigFromEnv()
	if config.Level != logging.WarnLevel {
		t.Errorf("level = %v; want WARN", config.Level)
	}
	if got := firstByte(t, config); got != '{' {
		t.Errorf("output starts with %q; want '{'", got)
	}
}
