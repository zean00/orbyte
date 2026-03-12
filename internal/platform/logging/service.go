package logging

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"sync"
	"time"
)

type contextKey string

const correlationIDKey contextKey = "correlation_id"

type Service struct {
	mu     sync.Mutex
	writer io.Writer
}

func NewService() *Service {
	return &Service{writer: os.Stdout}
}

func NewServiceWithWriter(writer io.Writer) *Service {
	if writer == nil {
		writer = os.Stdout
	}
	return &Service{writer: writer}
}

func (s *Service) Info(message string, fields map[string]any) {
	s.log("info", message, fields)
}

func (s *Service) Error(message string, fields map[string]any) {
	s.log("error", message, fields)
}

func (s *Service) log(level, message string, fields map[string]any) {
	record := map[string]any{
		"level":   level,
		"message": message,
		"time":    time.Now().UTC().Format(time.RFC3339Nano),
	}
	for key, value := range fields {
		record[key] = value
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, _ = s.writer.Write(append(encoded, '\n'))
}

func WithCorrelationID(ctx context.Context, correlationID string) context.Context {
	return context.WithValue(ctx, correlationIDKey, correlationID)
}

func CorrelationID(ctx context.Context) string {
	value, _ := ctx.Value(correlationIDKey).(string)
	return value
}
