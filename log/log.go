package log

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"time"
)

type ctxKey struct{}

// CloudLoggingHandler is a slog.Handler implementation for Google Cloud Functions.
type CloudLoggingHandler struct {
	attrs []slog.Attr
}

// NewCloudLoggingHandler creates a new handler that writes logs in Google Cloud structured format.
func NewCloudLoggingHandler() *CloudLoggingHandler {
	return &CloudLoggingHandler{}
}

// Handle processes log records.
func (h *CloudLoggingHandler) Handle(ctx context.Context, r slog.Record) error {
	// Extract trace info if available
	traceID := getTraceID(ctx)

	// Prepare log entry in Google Cloud structured logging format
	entry := map[string]any{
		"severity": r.Level.String(),
		"time":     time.Now().Format(time.RFC3339),
		"message":  r.Message,
	}

	// Include trace ID if available
	if traceID != "" {
		entry["logging.googleapis.com/trace"] = traceID
	}

	// Add handler attributes first
	for _, attr := range h.attrs {
		entry[attr.Key] = attr.Value.Any()
	}

	// Add record attributes
	r.Attrs(func(attr slog.Attr) bool {
		entry[attr.Key] = attr.Value.Any()
		return true
	})

	// Encode as JSON and write to stdout
	jsonData, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	os.Stdout.Write(jsonData)
	os.Stdout.Write([]byte("\n"))
	return nil
}

// Enabled always returns true, so all log levels are handled.
func (h *CloudLoggingHandler) Enabled(_ context.Context, _ slog.Level) bool {
	return true
}

// WithAttrs returns a new handler with additional attributes.
func (h *CloudLoggingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := make([]slog.Attr, len(h.attrs)+len(attrs))
	copy(newAttrs, h.attrs)
	copy(newAttrs[len(h.attrs):], attrs)
	return &CloudLoggingHandler{attrs: newAttrs}
}

// WithGroup returns the same handler, as grouping is not implemented.
func (h *CloudLoggingHandler) WithGroup(_ string) slog.Handler {
	return h
}

// getTraceID extracts the Google Cloud Trace ID from the context.
func getTraceID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	traceID, _ := ctx.Value("traceID").(string)
	return traceID
}

func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, logger)
}

func LoggerFromContext(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(ctxKey{}).(*slog.Logger); ok {
		return logger
	}
	return slog.New(NewCloudLoggingHandler())
}
