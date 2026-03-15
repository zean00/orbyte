package mcp

import (
	"testing"
	"time"

	"orbyte/internal/platform/analytics"
)

func TestAnalyticsStreamReplaysLatestAndPublishesUpdates(t *testing.T) {
	stream := NewAnalyticsStream()
	first := analytics.Snapshot{ID: "snap-1", GeneratedAt: time.Now().UTC()}
	stream.Publish(first)

	latest, ok := stream.Latest()
	if !ok || latest.ID != first.ID {
		t.Fatalf("expected latest snapshot %q, got %+v", first.ID, latest)
	}

	events, unsubscribe := stream.Subscribe()
	defer unsubscribe()

	second := analytics.Snapshot{ID: "snap-2", GeneratedAt: time.Now().UTC()}
	stream.Publish(second)

	select {
	case snapshot := <-events:
		if snapshot.ID != second.ID {
			t.Fatalf("expected live snapshot %q, got %q", second.ID, snapshot.ID)
		}
	case <-time.After(time.Second):
		t.Fatal("expected stream update")
	}
}
