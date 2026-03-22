package search

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"orbyte/internal/platform/shared"
)

type sourceRecord struct {
	SourceID       string
	SourceKind     string
	OrganizationID string
	LocationID     string
	Version        int
	UpdatedAt      time.Time
}

func (s *Service) ConsistencyReport(key string) (ConsistencyReport, error) {
	def, ok := s.IndexDefinition(key)
	if !ok {
		return ConsistencyReport{}, shared.NotFound("search index not found")
	}
	sources, err := s.sourceRecords(def)
	if err != nil {
		return ConsistencyReport{}, err
	}
	indexedByOrg, err := s.indexedRecordsByOrganization(def, sources)
	if err != nil {
		return ConsistencyReport{}, err
	}
	report := buildConsistencyReport(def, sources, indexedByOrg)
	s.updateRuntimeFromReport(def, report)
	return report, nil
}

func (s *Service) RepairIndex(key, mode, targetID string) (map[string]any, error) {
	def, ok := s.IndexDefinition(key)
	if !ok {
		return nil, shared.NotFound("search index not found")
	}
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		mode = "reconcile"
	}
	report, err := s.ConsistencyReport(key)
	if err != nil {
		return nil, err
	}
	repaired := 0
	switch def.SourceKind {
	case "document":
		for _, record := range s.documents.List() {
			if def.DocumentType != "" && record.Header.Type != def.DocumentType {
				continue
			}
			if targetID != "" && record.Header.ID != targetID {
				continue
			}
			if shouldRepairDocument(report, mode, record.Header.ID) {
				if err := s.indexDocument(def, record); err != nil {
					s.updateRuntime(key, func(runtime *IndexRuntime) {
						runtime.RuntimeStatus = "failed"
						runtime.LastFailureAt = time.Now().UTC()
						runtime.LastError = err.Error()
					})
					return nil, err
				}
				repaired++
			}
		}
	case "model":
		if s.models == nil {
			return nil, shared.Validation("model source is not configured")
		}
		for _, record := range s.models.Repository().ListRecords(def.ModelKey) {
			if targetID != "" && record.ID != targetID {
				continue
			}
			if shouldRepairDocument(report, mode, record.ID) {
				if err := s.indexModel(def, record); err != nil {
					return nil, err
				}
				repaired++
			}
		}
	case "projection":
		for _, summary := range s.ListDocuments() {
			if targetID != "" && summary.DocumentID != targetID {
				continue
			}
			if shouldRepairDocument(report, mode, summary.DocumentID) {
				if err := s.indexProjectionSummary(def, summary); err != nil {
					return nil, err
				}
				repaired++
			}
		}
	default:
		return nil, shared.Validation("search index source_kind is invalid")
	}
	s.updateRuntime(key, func(runtime *IndexRuntime) {
		runtime.RuntimeStatus = "ready"
		runtime.LastRepairAt = time.Now().UTC()
		runtime.LastRepairMode = mode
		runtime.LastRepairCount = repaired
	})
	updated, err := s.ConsistencyReport(key)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"index_key": key,
		"mode":      mode,
		"target_id": targetID,
		"repaired":  repaired,
		"report":    updated,
	}, nil
}

func (s *Service) ProjectionConsistencyReport() (ConsistencyReport, error) {
	report := ConsistencyReport{
		IndexKey:      "document_summary",
		ProjectionKey: "document_summary",
		SourceKind:    "projection",
	}
	if s.documents == nil {
		return report, shared.Validation("document source is not configured")
	}
	documents := s.documents.List()
	summaries := s.ListDocuments()
	report.SourceCount = len(documents)
	report.IndexedCount = len(summaries)
	byID := make(map[string]DocumentSummary, len(summaries))
	for _, item := range summaries {
		byID[item.DocumentID] = item
		if item.UpdatedAt.After(report.LastIndexedAt) {
			report.LastIndexedAt = item.UpdatedAt
		}
	}
	for _, item := range documents {
		if item.Header.UpdatedAt.After(report.LastSourceUpdatedAt) {
			report.LastSourceUpdatedAt = item.Header.UpdatedAt
		}
		summary, ok := byID[item.Header.ID]
		if !ok {
			report.MissingCount++
			report.Issues = append(report.Issues, ConsistencyIssue{Kind: "missing", SourceID: item.Header.ID, SourceKind: "document"})
			continue
		}
		if summary.Version < item.Header.Version || summary.ETag != item.Header.ETag || summary.Status != item.Header.Status {
			report.StaleCount++
			report.Issues = append(report.Issues, ConsistencyIssue{Kind: "stale", SourceID: item.Header.ID, SourceKind: "document"})
		}
	}
	if report.LastIndexedAt.Before(report.LastSourceUpdatedAt) {
		report.LagSeconds = int64(report.LastSourceUpdatedAt.Sub(report.LastIndexedAt).Seconds())
	}
	report.Status = classifyConsistency(report.MissingCount, report.StaleCount, report.LagSeconds)
	return report, nil
}

func (s *Service) updateRuntime(key string, fn func(runtime *IndexRuntime)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.runtimes[key]
	if !ok {
		return
	}
	fn(&item)
	s.runtimes[key] = item
}

func (s *Service) updateRuntimeFromReport(def IndexDefinition, report ConsistencyReport) {
	s.updateRuntime(def.Key, func(runtime *IndexRuntime) {
		runtime.SourceCount = report.SourceCount
		runtime.IndexedCount = report.IndexedCount
		runtime.MissingCount = report.MissingCount
		runtime.StaleCount = report.StaleCount
		runtime.LastLagSeconds = report.LagSeconds
		runtime.ConsistencyStatus = report.Status
		if report.Status == "ok" {
			runtime.RuntimeStatus = "ready"
			runtime.LastError = ""
			runtime.LastSuccessAt = time.Now().UTC()
		} else {
			runtime.RuntimeStatus = "degraded"
		}
	})
}

func buildConsistencyReport(def IndexDefinition, sources []sourceRecord, indexedByOrg map[string][]IndexedRecord) ConsistencyReport {
	report := ConsistencyReport{
		IndexKey:      def.Key,
		ProjectionKey: projectionKeyForDefinition(def),
		SourceKind:    def.SourceKind,
		SourceCount:   len(sources),
	}
	byIndexedKey := map[string]IndexedRecord{}
	for _, items := range indexedByOrg {
		report.IndexedCount += len(items)
		for _, item := range items {
			byIndexedKey[item.OrganizationID+"|"+item.SourceID] = item
			if item.UpdatedAt.After(report.LastIndexedAt) {
				report.LastIndexedAt = item.UpdatedAt
			}
		}
	}
	for _, source := range sources {
		if source.UpdatedAt.After(report.LastSourceUpdatedAt) {
			report.LastSourceUpdatedAt = source.UpdatedAt
		}
		indexed, ok := byIndexedKey[source.OrganizationID+"|"+source.SourceID]
		if !ok {
			report.MissingCount++
			report.Issues = append(report.Issues, ConsistencyIssue{Kind: "missing", SourceID: source.SourceID, SourceKind: source.SourceKind})
			continue
		}
		if indexed.Version < source.Version || indexed.UpdatedAt.Before(source.UpdatedAt) {
			report.StaleCount++
			report.Issues = append(report.Issues, ConsistencyIssue{Kind: "stale", SourceID: source.SourceID, SourceKind: source.SourceKind})
		}
	}
	if report.LastIndexedAt.Before(report.LastSourceUpdatedAt) {
		report.LagSeconds = int64(report.LastSourceUpdatedAt.Sub(report.LastIndexedAt).Seconds())
	}
	report.Status = classifyConsistency(report.MissingCount, report.StaleCount, report.LagSeconds)
	sort.Slice(report.Issues, func(i, j int) bool {
		if report.Issues[i].Kind == report.Issues[j].Kind {
			return report.Issues[i].SourceID < report.Issues[j].SourceID
		}
		return report.Issues[i].Kind < report.Issues[j].Kind
	})
	return report
}

func (s *Service) sourceRecords(def IndexDefinition) ([]sourceRecord, error) {
	switch def.SourceKind {
	case "document":
		if s.documents == nil {
			return nil, shared.Validation("document source is not configured")
		}
		items := make([]sourceRecord, 0)
		for _, item := range s.documents.List() {
			if def.DocumentType != "" && item.Header.Type != def.DocumentType {
				continue
			}
			items = append(items, sourceRecord{
				SourceID:       item.Header.ID,
				SourceKind:     "document",
				OrganizationID: item.Header.OrganizationID,
				LocationID:     item.Header.LocationID,
				Version:        item.Header.Version,
				UpdatedAt:      item.Header.UpdatedAt,
			})
		}
		return items, nil
	case "model":
		if s.models == nil {
			return nil, shared.Validation("model source is not configured")
		}
		items := make([]sourceRecord, 0)
		for _, item := range s.models.Repository().ListRecords(def.ModelKey) {
			items = append(items, sourceRecord{
				SourceID:       item.ID,
				SourceKind:     "model",
				OrganizationID: stringValue(item.Values["organization_id"]),
				LocationID:     stringValue(item.Values["location_id"]),
				Version:        item.Version,
				UpdatedAt:      item.UpdatedAt,
			})
		}
		return items, nil
	case "projection":
		items := make([]sourceRecord, 0)
		for _, item := range s.ListDocuments() {
			items = append(items, sourceRecord{
				SourceID:       item.DocumentID,
				SourceKind:     "projection",
				OrganizationID: item.OrganizationID,
				LocationID:     item.LocationID,
				Version:        item.Version,
				UpdatedAt:      item.UpdatedAt,
			})
		}
		return items, nil
	default:
		return nil, shared.Validation("search index source_kind is invalid")
	}
}

func (s *Service) indexedRecordsByOrganization(def IndexDefinition, sources []sourceRecord) (map[string][]IndexedRecord, error) {
	orgs := map[string]struct{}{}
	for _, item := range sources {
		org := strings.TrimSpace(item.OrganizationID)
		if org == "" {
			org = "global"
		}
		orgs[org] = struct{}{}
	}
	if len(orgs) == 0 {
		orgs["global"] = struct{}{}
	}
	out := map[string][]IndexedRecord{}
	for org := range orgs {
		items, err := s.backend.List(def, org)
		if err != nil {
			return nil, fmt.Errorf("list indexed records for %s: %w", org, err)
		}
		out[org] = items
	}
	return out, nil
}

func classifyConsistency(missing, stale int, lag int64) string {
	switch {
	case missing > 0:
		return "missing_records"
	case stale > 0:
		return "stale_records"
	case lag > 0:
		return "lagging"
	default:
		return "ok"
	}
}

func shouldRepairDocument(report ConsistencyReport, mode, sourceID string) bool {
	switch mode {
	case "all", "rebuild":
		return true
	case "repair_missing":
		return reportHasIssue(report, "missing", sourceID)
	case "repair_stale":
		return reportHasIssue(report, "stale", sourceID)
	case "reconcile":
		return reportHasIssue(report, "missing", sourceID) || reportHasIssue(report, "stale", sourceID)
	default:
		return true
	}
}

func reportHasIssue(report ConsistencyReport, kind, sourceID string) bool {
	for _, issue := range report.Issues {
		if issue.Kind == kind && issue.SourceID == sourceID {
			return true
		}
	}
	return false
}
