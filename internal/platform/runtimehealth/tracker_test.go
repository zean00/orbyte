package runtimehealth

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestTrackerSnapshotAndSubsystemTransitions(t *testing.T) {
	tracker := NewTracker()
	tracker.SetBootstrapped(true)
	tracker.SetBackgroundStarted(true)
	tracker.MarkFailure("jobs", errors.New("warming up"))
	tracker.MarkFailure("jobs", errors.New("warming up"))
	snapshot := tracker.Snapshot(context.Background())
	if !snapshot.Ready {
		t.Fatalf("expected still ready before degradation, got %+v", snapshot)
	}
	tracker.MarkFailure("jobs", errors.New("down"))
	snapshot = tracker.Snapshot(context.Background())
	if snapshot.Ready {
		t.Fatalf("expected degraded subsystem to make tracker unready, got %+v", snapshot)
	}
	if len(snapshot.Subsystems) != 1 || snapshot.Subsystems[0].Status != "degraded" {
		t.Fatalf("expected degraded subsystem snapshot, got %+v", snapshot.Subsystems)
	}

	tracker.MarkSuccess("jobs")
	snapshot = tracker.Snapshot(context.Background())
	if !snapshot.Ready || snapshot.Subsystems[0].Status != "healthy" {
		t.Fatalf("expected healthy subsystem snapshot, got %+v", snapshot)
	}
}

func TestTrackerDependencyAndDatabaseStatus(t *testing.T) {
	tracker := NewTracker()
	tracker.SetBootstrapped(true)
	tracker.SetBackgroundStarted(true)
	tracker.SetChecker(func(context.Context) error { return errors.New("db unavailable") })
	tracker.SetDBStatsProvider(func() *DBStats {
		return &DBStats{MaxOpenConnections: 10, OpenConnections: 2}
	})
	snapshot := tracker.Snapshot(context.Background())
	if snapshot.DependencyOK || snapshot.Ready {
		t.Fatalf("expected failed dependency check, got %+v", snapshot)
	}
	if snapshot.Database == nil || snapshot.Database.OpenConnections != 2 {
		t.Fatalf("expected database stats, got %+v", snapshot.Database)
	}

	tracker.SetChecker(nil)
	tracker.SetShuttingDown(true)
	snapshot = tracker.Snapshot(context.Background())
	if snapshot.Ready {
		t.Fatalf("expected shutting down tracker to be unready, got %+v", snapshot)
	}

	var nilTracker *Tracker
	nilSnap := nilTracker.Snapshot(context.Background())
	if !nilSnap.Live || !nilSnap.Ready || !nilSnap.DependencyOK {
		t.Fatalf("expected nil tracker healthy snapshot, got %+v", nilSnap)
	}
}

func TestTrackerTimestampsAreRecorded(t *testing.T) {
	tracker := NewTracker()
	tracker.MarkSuccess("search")
	tracker.MarkFailure("search", errors.New("lag"))
	snapshot := tracker.Snapshot(context.Background())
	if len(snapshot.Subsystems) != 1 {
		t.Fatalf("expected one subsystem, got %+v", snapshot.Subsystems)
	}
	item := snapshot.Subsystems[0]
	if item.LastSuccessAt.IsZero() || item.LastFailureAt.IsZero() {
		t.Fatalf("expected timestamps, got %+v", item)
	}
	if item.LastFailureAt.Before(item.LastSuccessAt.Add(-time.Minute)) {
		t.Fatalf("unexpected timestamp ordering, got %+v", item)
	}
}
