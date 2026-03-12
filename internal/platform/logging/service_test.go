package logging

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestStructuredLogAndCorrelationContext(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := NewServiceWithWriter(buf)
	ctx := WithCorrelationID(context.Background(), "corr-1")
	logger.Info("request completed", map[string]any{"correlation_id": CorrelationID(ctx), "status": 200})
	output := buf.String()
	if !strings.Contains(output, "request completed") || !strings.Contains(output, "corr-1") {
		t.Fatalf("expected structured output with correlation id, got %s", output)
	}
}
