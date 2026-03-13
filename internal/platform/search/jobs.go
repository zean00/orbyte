package search

import (
	"context"

	"clinic/internal/platform/jobs"
	"clinic/internal/platform/shared"
)

const (
	JobRefreshDocument = "search.refresh_document"
	JobRefreshModel    = "search.refresh_model"
	JobRebuildIndex    = "search.rebuild_index"
	JobRebuildSummary  = "search.rebuild_document_summary"
)

func (s *Service) registerJobHandlers(jobSvc *jobs.Service) {
	jobSvc.RegisterHandler(JobRefreshDocument, func(_ context.Context, payload map[string]any) (map[string]any, error) {
		if s.documents == nil {
			return nil, shared.Validation("document source is not configured")
		}
		documentID := stringValue(payload["document_id"])
		if documentID == "" {
			return nil, shared.Validation("document_id is required")
		}
		record, err := s.documents.Get(documentID)
		if err != nil {
			return nil, err
		}
		if err := s.indexDocumentAndProjection(record); err != nil {
			return nil, err
		}
		return map[string]any{"document_id": documentID}, nil
	})
	jobSvc.RegisterHandler(JobRefreshModel, func(_ context.Context, payload map[string]any) (map[string]any, error) {
		if s.models == nil {
			return nil, shared.Validation("model source is not configured")
		}
		modelKey := stringValue(payload["model_key"])
		recordID := stringValue(payload["record_id"])
		if modelKey == "" || recordID == "" {
			return nil, shared.Validation("model_key and record_id are required")
		}
		record, err := s.models.Get(modelKey, recordID)
		if err != nil {
			return nil, err
		}
		if err := s.indexModelByRecord(record); err != nil {
			return nil, err
		}
		return map[string]any{"model_key": modelKey, "record_id": recordID}, nil
	})
	jobSvc.RegisterHandler(JobRebuildIndex, func(_ context.Context, payload map[string]any) (map[string]any, error) {
		indexKey := stringValue(payload["index_key"])
		if indexKey == "" {
			return nil, shared.Validation("index_key is required")
		}
		return s.RebuildIndex(indexKey)
	})
	jobSvc.RegisterHandler(JobRebuildSummary, func(_ context.Context, payload map[string]any) (map[string]any, error) {
		if s.documents == nil {
			return nil, shared.Validation("document source is not configured")
		}
		documentID := stringValue(payload["document_id"])
		if documentID != "" {
			record, err := s.documents.Get(documentID)
			if err != nil {
				return nil, err
			}
			s.RebuildDocument(record)
			return map[string]any{"document_id": documentID}, nil
		}
		records := s.documents.List()
		s.RebuildAll(records)
		return map[string]any{"count": len(records)}, nil
	})
}
