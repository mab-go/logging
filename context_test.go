package logging

import (
	"bytes"
	"context"
	"testing"
)

func TestContextRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	log := NewLogger(Config{Level: InfoLevel, Output: &buf})

	ctx := NewContext(context.Background(), log)
	got, created := FromContext(ctx)

	if created {
		t.Error("expected created=false when a logger is present in context")
	}
	if got != log {
		t.Error("logger returned from context is not the one inserted")
	}
}

func TestContextMissingLogger(t *testing.T) {
	got, created := FromContext(context.Background())

	if !created {
		t.Error("expected created=true when no logger is present in context")
	}
	if got == nil {
		t.Error("expected a non-nil default logger")
	}
}
