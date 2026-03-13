package analytics

import (
	"context"
	"time"

	"orbyte/internal/platform/jobs"
	"orbyte/internal/platform/shared"
)

const (
	JobCaptureSnapshot  = "analytics.capture_snapshot"
	JobRunDueReports    = "analytics.run_due_reports"
	JobCleanupSnapshots = "analytics.cleanup_snapshots"
	JobCleanupReports   = "analytics.cleanup_reports"
	JobRecomputeState   = "analytics.recompute.current_state"
)

func (s *Service) AttachJobs(jobSvc *jobs.Service) {
	s.jobs = jobSvc
	if jobSvc == nil {
		return
	}
	jobSvc.RegisterHandler(JobCaptureSnapshot, func(_ context.Context, _ map[string]any) (map[string]any, error) {
		snapshot, err := s.CaptureSnapshot()
		if err != nil {
			return nil, err
		}
		return map[string]any{"snapshot_id": snapshot.ID}, nil
	})
	jobSvc.RegisterHandler(JobRunDueReports, func(_ context.Context, payload map[string]any) (map[string]any, error) {
		now, err := jobTime(payload, "now")
		if err != nil {
			return nil, err
		}
		if err := s.RunDueReports(now); err != nil {
			return nil, err
		}
		return map[string]any{"ran_at": now.Format(time.RFC3339)}, nil
	})
	jobSvc.RegisterHandler(JobCleanupSnapshots, func(_ context.Context, payload map[string]any) (map[string]any, error) {
		cutoff, err := jobTime(payload, "cutoff")
		if err != nil {
			return nil, err
		}
		if err := s.DeleteOlderThan(cutoff); err != nil {
			return nil, err
		}
		return map[string]any{"cutoff": cutoff.Format(time.RFC3339)}, nil
	})
	jobSvc.RegisterHandler(JobCleanupReports, func(_ context.Context, payload map[string]any) (map[string]any, error) {
		cutoff, err := jobTime(payload, "cutoff")
		if err != nil {
			return nil, err
		}
		if err := s.CleanupReportData(cutoff); err != nil {
			return nil, err
		}
		return map[string]any{"cutoff": cutoff.Format(time.RFC3339)}, nil
	})
	jobSvc.RegisterHandler(JobRecomputeState, func(_ context.Context, _ map[string]any) (map[string]any, error) {
		snapshot, err := s.RecomputeCurrentState()
		if err != nil {
			return nil, err
		}
		return map[string]any{"snapshot_id": snapshot.ID}, nil
	})
}

func jobTime(payload map[string]any, key string) (time.Time, error) {
	raw, _ := payload[key].(string)
	if raw == "" {
		return time.Time{}, shared.Validation(key + " is required")
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, shared.Validation("invalid " + key)
	}
	return parsed, nil
}
