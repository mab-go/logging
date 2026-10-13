package awslambda

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"sync"
)

// RequestIDKey is the field name the handlers read the Lambda request ID from.
// Attach it once per invocation, e.g.
// log.WithField(awslambda.RequestIDKey, lc.AwsRequestID).
const RequestIDKey = "requestId"

// timestampLayout is the prefix timestamp: RFC 3339 in UTC with milliseconds,
// as the Lambda managed runtimes write it.
const timestampLayout = "2006-01-02T15:04:05.000Z"

// NewJSONHandler returns a handler for functions using Lambda's JSON log
// format. Each record is one JSON object with the keys Lambda expects
// ("timestamp", "level", "message") plus the record's fields. PANIC is
// written as FATAL, because Lambda has no PANIC level and would otherwise
// treat it as INFO for log-level filtering. A nil opts is treated as the zero
// slog.HandlerOptions.
func NewJSONHandler(w io.Writer, opts *slog.HandlerOptions) slog.Handler {
	return newJSON(w, opts, true)
}

// NewTextHandler returns a handler for functions using Lambda's Text log
// format. Each line is four tab-separated fields:
//
//	[LEVEL]	<timestamp>	<request ID>	<JSON>
//
// The timestamp and request ID columns are empty when the record has no time
// or no RequestIDKey field. The JSON is the whole record, in the same shape
// as NewJSONHandler writes except that PANIC stays PANIC. A nil opts is
// treated as the zero slog.HandlerOptions.
func NewTextHandler(w io.Writer, opts *slog.HandlerOptions) slog.Handler {
	if opts == nil {
		opts = &slog.HandlerOptions{}
	}

	buf := &bytes.Buffer{}

	return &hybridHandler{
		inner:   newJSON(buf, opts, false),
		buf:     buf,
		mu:      &sync.Mutex{},
		out:     w,
		replace: opts.ReplaceAttr,
	}
}

// newJSON returns a slog JSON handler whose ReplaceAttr runs opts.ReplaceAttr
// and then lambdaKeys.
func newJSON(w io.Writer, opts *slog.HandlerOptions, panicAsFatal bool) slog.Handler {
	var o slog.HandlerOptions
	if opts != nil {
		o = *opts
	}

	prev := o.ReplaceAttr
	o.ReplaceAttr = func(groups []string, a slog.Attr) slog.Attr {
		if prev != nil {
			a = prev(groups, a)
		}
		if len(groups) > 0 {
			return a
		}

		return lambdaKeys(a, panicAsFatal)
	}

	return slog.NewJSONHandler(w, &o)
}

// lambdaKeys renames slog's built-in time and msg keys to Lambda's
// "timestamp" and "message", writes the time in UTC, and, if panicAsFatal is
// set, rewrites the PANIC level as FATAL. ReplaceAttr can't tell built-in
// attributes from top-level fields with the same key, so those are renamed
// too.
func lambdaKeys(a slog.Attr, panicAsFatal bool) slog.Attr {
	switch a.Key {
	case slog.TimeKey:
		a.Key = "timestamp"
		if a.Value.Kind() == slog.KindTime {
			a.Value = slog.TimeValue(a.Value.Time().UTC())
		}
	case slog.MessageKey:
		a.Key = "message"
	case slog.LevelKey:
		if panicAsFatal && a.Value.String() == "PANIC" {
			a.Value = slog.StringValue("FATAL")
		}
	}

	return a
}

// hybridHandler writes the Text-format prefix and then the record's JSON. All
// clones share buf and mu, so each line reaches out in a single Write.
type hybridHandler struct {
	inner     slog.Handler // JSON handler writing into buf
	buf       *bytes.Buffer
	mu        *sync.Mutex
	out       io.Writer
	replace   func(groups []string, a slog.Attr) slog.Attr // names the level
	requestID string
	grouped   bool // a group is open; its attrs are not top-level
}

// Enabled reports whether the inner JSON handler handles records at level.
func (h *hybridHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

// Handle writes one line: the prefix, then the record as JSON.
func (h *hybridHandler) Handle(ctx context.Context, r slog.Record) error {
	requestID := h.requestID
	if !h.grouped {
		r.Attrs(func(a slog.Attr) bool {
			if a.Key == RequestIDKey {
				requestID = a.Value.Resolve().String()
			}

			return true
		})
	}

	var timestamp string
	if !r.Time.IsZero() {
		timestamp = r.Time.UTC().Format(timestampLayout)
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	h.buf.Reset()
	if err := h.inner.Handle(ctx, r); err != nil {
		return err
	}

	line := make([]byte, 0, len(timestamp)+len(requestID)+h.buf.Len()+16)
	line = append(line, '[')
	line = append(line, h.levelText(r.Level)...)
	line = append(line, "]\t"...)
	line = append(line, timestamp...)
	line = append(line, '\t')
	line = append(line, requestID...)
	line = append(line, '\t')
	line = append(line, h.buf.Bytes()...)

	_, err := h.out.Write(line)

	return err
}

// WithAttrs returns a clone with attrs added, picking up a top-level
// RequestIDKey attribute for the prefix.
func (h *hybridHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	c := *h
	c.inner = h.inner.WithAttrs(attrs)
	if !h.grouped {
		for _, a := range attrs {
			if a.Key == RequestIDKey {
				c.requestID = a.Value.Resolve().String()
			}
		}
	}

	return &c
}

// WithGroup returns a clone with the group opened. Attributes added after it
// are inside the group, so they no longer set the request ID.
func (h *hybridHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}

	c := *h
	c.inner = h.inner.WithGroup(name)
	c.grouped = true

	return &c
}

// levelText names level the way the JSON body does, using the ReplaceAttr
// from the options (which names the custom FATAL and PANIC levels).
func (h *hybridHandler) levelText(level slog.Level) string {
	if h.replace == nil {
		return level.String()
	}

	return h.replace(nil, slog.Any(slog.LevelKey, level)).Value.String()
}
