package analytics

import (
	"sort"
	"time"
)

type MemoryRepository struct {
	snapshots           []Snapshot
	rollups             map[string]Rollup
	facts               FactBundle
	dimensions          DimensionBundle
	dashboards          []Dashboard
	savedMetrics        []SavedMetric
	savedQueries        []SavedQuery
	reports             []ReportDefinition
	runs                []ReportRun
	artifacts           []ReportArtifact
	deliveries          []ReportDelivery
	deliveryDeadLetters []ReportDeliveryDeadLetter
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		snapshots:           []Snapshot{},
		rollups:             map[string]Rollup{},
		facts:               FactBundle{},
		dimensions:          DimensionBundle{},
		dashboards:          []Dashboard{},
		savedMetrics:        []SavedMetric{},
		savedQueries:        []SavedQuery{},
		reports:             []ReportDefinition{},
		runs:                []ReportRun{},
		artifacts:           []ReportArtifact{},
		deliveries:          []ReportDelivery{},
		deliveryDeadLetters: []ReportDeliveryDeadLetter{},
	}
}

func (r *MemoryRepository) SaveDimensions(dimensions DimensionBundle) error {
	r.dimensions = dimensions
	return nil
}

func (r *MemoryRepository) SaveSnapshot(snapshot Snapshot) error {
	r.snapshots = append(r.snapshots, snapshot)
	return nil
}

func (r *MemoryRepository) ListSnapshots() []Snapshot {
	items := append([]Snapshot(nil), r.snapshots...)
	sort.Slice(items, func(i, j int) bool { return items[i].GeneratedAt.Before(items[j].GeneratedAt) })
	return items
}

func (r *MemoryRepository) QuerySnapshots(query Query) []Snapshot {
	items := r.ListSnapshots()
	filtered := make([]Snapshot, 0, len(items))
	for _, snapshot := range items {
		if query.Window != "" && snapshot.Window != query.Window {
			continue
		}
		if !query.From.IsZero() && snapshot.GeneratedAt.Before(query.From) {
			continue
		}
		if !query.To.IsZero() && snapshot.GeneratedAt.After(query.To) {
			continue
		}
		filtered = append(filtered, snapshot)
	}
	if query.Limit > 0 && len(filtered) > query.Limit {
		filtered = filtered[len(filtered)-query.Limit:]
	}
	return filtered
}

func (r *MemoryRepository) ListRecent(limit int) []Snapshot {
	items := r.ListSnapshots()
	if limit <= 0 || len(items) <= limit {
		return items
	}
	return append([]Snapshot(nil), items[len(items)-limit:]...)
}

func (r *MemoryRepository) DeleteOlderThan(cutoff time.Time) error {
	filtered := make([]Snapshot, 0, len(r.snapshots))
	for _, snapshot := range r.snapshots {
		if snapshot.GeneratedAt.Before(cutoff) {
			continue
		}
		filtered = append(filtered, snapshot)
	}
	r.snapshots = filtered
	return nil
}

func (r *MemoryRepository) UpsertRollup(rollup Rollup) error {
	r.rollups[rollup.Granularity+":"+rollup.BucketStart.Format(time.RFC3339)] = rollup
	return nil
}

func (r *MemoryRepository) ListRollups(granularity string, limit int) []Rollup {
	return r.QueryRollups(granularity, time.Time{}, time.Time{}, limit)
}

func (r *MemoryRepository) QueryRollups(granularity string, from, to time.Time, limit int) []Rollup {
	items := make([]Rollup, 0, len(r.rollups))
	for _, rollup := range r.rollups {
		if granularity != "" && rollup.Granularity != granularity {
			continue
		}
		if !from.IsZero() && rollup.BucketStart.Before(from) {
			continue
		}
		if !to.IsZero() && rollup.BucketStart.After(to) {
			continue
		}
		items = append(items, rollup)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].BucketStart.Before(items[j].BucketStart) })
	if limit > 0 && len(items) > limit {
		items = items[len(items)-limit:]
	}
	return items
}

func (r *MemoryRepository) SaveFacts(facts FactBundle) error {
	r.facts.Documents = append(r.facts.Documents, facts.Documents...)
	r.facts.Workflow = append(r.facts.Workflow, facts.Workflow...)
	r.facts.Reliability = append(r.facts.Reliability, facts.Reliability...)
	return nil
}

func (r *MemoryRepository) QueryFacts(query FactQuery) FactBundle {
	bundle := FactBundle{}
	for _, fact := range r.facts.Documents {
		if !query.From.IsZero() && fact.CapturedAt.Before(query.From) {
			continue
		}
		if !query.To.IsZero() && fact.CapturedAt.After(query.To) {
			continue
		}
		if query.LocationID != "" && fact.LocationID != query.LocationID {
			continue
		}
		if query.DocumentType != "" && fact.DocumentType != query.DocumentType {
			continue
		}
		bundle.Documents = append(bundle.Documents, fact)
	}
	for _, fact := range r.facts.Workflow {
		if !query.From.IsZero() && fact.CapturedAt.Before(query.From) {
			continue
		}
		if !query.To.IsZero() && fact.CapturedAt.After(query.To) {
			continue
		}
		bundle.Workflow = append(bundle.Workflow, fact)
	}
	for _, fact := range r.facts.Reliability {
		if !query.From.IsZero() && fact.CapturedAt.Before(query.From) {
			continue
		}
		if !query.To.IsZero() && fact.CapturedAt.After(query.To) {
			continue
		}
		bundle.Reliability = append(bundle.Reliability, fact)
	}
	return bundle
}

func (r *MemoryRepository) ReportingBreakdown(query FactQuery, dimension string) []ReportingRow {
	rows := make(map[string]*ReportingRow)
	for _, fact := range r.QueryFacts(query).Documents {
		var key, label string
		switch dimension {
		case "location":
			key = fact.LocationID
			if key == "" {
				continue
			}
			label = key
		default:
			key = fact.DocumentType
			if key == "" {
				continue
			}
			label = key
			dimension = "document_type"
		}
		row := rows[key]
		if row == nil {
			row = &ReportingRow{DimensionType: dimension, DimensionKey: key, Label: label}
			rows[key] = row
		}
		row.Created += fact.Created
		row.Draft += fact.Draft
		row.Submitted += fact.Submitted
		row.Approved += fact.Approved
		row.Rejected += fact.Rejected
		row.Cancelled += fact.Cancelled
	}
	items := make([]ReportingRow, 0, len(rows))
	for _, row := range rows {
		items = append(items, *row)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].DimensionKey < items[j].DimensionKey })
	return items
}

func (r *MemoryRepository) SaveReportDefinition(def ReportDefinition) error {
	for i := range r.reports {
		if r.reports[i].ID == def.ID {
			r.reports[i] = def
			return nil
		}
	}
	r.reports = append(r.reports, def)
	return nil
}

func (r *MemoryRepository) ListReportDefinitions() []ReportDefinition {
	items := append([]ReportDefinition(nil), r.reports...)
	sort.Slice(items, func(i, j int) bool { return items[i].NextRunAt.Before(items[j].NextRunAt) })
	return items
}

func (r *MemoryRepository) UpdateReportDefinition(def ReportDefinition) error {
	for i := range r.reports {
		if r.reports[i].ID == def.ID {
			r.reports[i] = def
			return nil
		}
	}
	return nil
}

func (r *MemoryRepository) DeleteReportDefinition(id string) error {
	filtered := make([]ReportDefinition, 0, len(r.reports))
	for _, item := range r.reports {
		if item.ID == id {
			continue
		}
		filtered = append(filtered, item)
	}
	r.reports = filtered
	return nil
}

func (r *MemoryRepository) SaveDashboard(item Dashboard) error {
	for i := range r.dashboards {
		if r.dashboards[i].ID == item.ID {
			r.dashboards[i] = item
			return nil
		}
	}
	r.dashboards = append(r.dashboards, item)
	return nil
}

func (r *MemoryRepository) ListDashboards() []Dashboard {
	items := append([]Dashboard(nil), r.dashboards...)
	sort.Slice(items, func(i, j int) bool { return items[i].UpdatedAt.Before(items[j].UpdatedAt) })
	return items
}

func (r *MemoryRepository) GetDashboard(id string) (Dashboard, bool) {
	for _, item := range r.dashboards {
		if item.ID == id {
			return item, true
		}
	}
	return Dashboard{}, false
}

func (r *MemoryRepository) DeleteDashboard(id string) error {
	filtered := make([]Dashboard, 0, len(r.dashboards))
	for _, item := range r.dashboards {
		if item.ID == id {
			continue
		}
		filtered = append(filtered, item)
	}
	r.dashboards = filtered
	return nil
}

func (r *MemoryRepository) SaveSavedMetric(item SavedMetric) error {
	for i := range r.savedMetrics {
		if r.savedMetrics[i].ID == item.ID {
			r.savedMetrics[i] = item
			return nil
		}
	}
	r.savedMetrics = append(r.savedMetrics, item)
	return nil
}

func (r *MemoryRepository) ListSavedMetrics() []SavedMetric {
	items := append([]SavedMetric(nil), r.savedMetrics...)
	sort.Slice(items, func(i, j int) bool { return items[i].UpdatedAt.Before(items[j].UpdatedAt) })
	return items
}

func (r *MemoryRepository) GetSavedMetric(id string) (SavedMetric, bool) {
	for _, item := range r.savedMetrics {
		if item.ID == id {
			return item, true
		}
	}
	return SavedMetric{}, false
}

func (r *MemoryRepository) DeleteSavedMetric(id string) error {
	filtered := make([]SavedMetric, 0, len(r.savedMetrics))
	for _, item := range r.savedMetrics {
		if item.ID == id {
			continue
		}
		filtered = append(filtered, item)
	}
	r.savedMetrics = filtered
	return nil
}

func (r *MemoryRepository) SaveSavedQuery(item SavedQuery) error {
	for i := range r.savedQueries {
		if r.savedQueries[i].ID == item.ID {
			r.savedQueries[i] = item
			return nil
		}
	}
	r.savedQueries = append(r.savedQueries, item)
	return nil
}

func (r *MemoryRepository) ListSavedQueries() []SavedQuery {
	items := append([]SavedQuery(nil), r.savedQueries...)
	sort.Slice(items, func(i, j int) bool { return items[i].UpdatedAt.Before(items[j].UpdatedAt) })
	return items
}

func (r *MemoryRepository) GetSavedQuery(id string) (SavedQuery, bool) {
	for _, item := range r.savedQueries {
		if item.ID == id {
			return item, true
		}
	}
	return SavedQuery{}, false
}

func (r *MemoryRepository) DeleteSavedQuery(id string) error {
	filtered := make([]SavedQuery, 0, len(r.savedQueries))
	for _, item := range r.savedQueries {
		if item.ID == id {
			continue
		}
		filtered = append(filtered, item)
	}
	r.savedQueries = filtered
	return nil
}

func (r *MemoryRepository) SaveReportRun(run ReportRun) error {
	r.runs = append(r.runs, run)
	return nil
}

func (r *MemoryRepository) ListReportRuns() []ReportRun {
	items := append([]ReportRun(nil), r.runs...)
	sort.Slice(items, func(i, j int) bool { return items[i].GeneratedAt.Before(items[j].GeneratedAt) })
	return items
}

func (r *MemoryRepository) SaveReportArtifact(artifact ReportArtifact) error {
	r.artifacts = append(r.artifacts, artifact)
	return nil
}

func (r *MemoryRepository) ListReportArtifacts() []ReportArtifact {
	items := append([]ReportArtifact(nil), r.artifacts...)
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.Before(items[j].CreatedAt) })
	return items
}

func (r *MemoryRepository) GetReportArtifact(id string) (ReportArtifact, bool) {
	for _, artifact := range r.artifacts {
		if artifact.ID == id {
			return artifact, true
		}
	}
	return ReportArtifact{}, false
}

func (r *MemoryRepository) SaveReportDelivery(delivery ReportDelivery) error {
	r.deliveries = append(r.deliveries, delivery)
	return nil
}

func (r *MemoryRepository) ListReportDeliveries() []ReportDelivery {
	items := append([]ReportDelivery(nil), r.deliveries...)
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.Before(items[j].CreatedAt) })
	return items
}

func (r *MemoryRepository) SaveReportDeliveryDeadLetter(record ReportDeliveryDeadLetter) error {
	r.deliveryDeadLetters = append(r.deliveryDeadLetters, record)
	return nil
}

func (r *MemoryRepository) ListReportDeliveryDeadLetters() []ReportDeliveryDeadLetter {
	items := append([]ReportDeliveryDeadLetter(nil), r.deliveryDeadLetters...)
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.Before(items[j].CreatedAt) })
	return items
}

func (r *MemoryRepository) CleanupReportData(cutoff time.Time) error {
	filterRuns := make([]ReportRun, 0, len(r.runs))
	keptRunIDs := map[string]struct{}{}
	for _, run := range r.runs {
		if run.GeneratedAt.Before(cutoff) {
			continue
		}
		filterRuns = append(filterRuns, run)
		keptRunIDs[run.ID] = struct{}{}
	}
	r.runs = filterRuns

	filterArtifacts := make([]ReportArtifact, 0, len(r.artifacts))
	keptArtifactIDs := map[string]struct{}{}
	for _, artifact := range r.artifacts {
		if artifact.CreatedAt.Before(cutoff) {
			continue
		}
		if _, ok := keptRunIDs[artifact.ReportRunID]; !ok {
			continue
		}
		filterArtifacts = append(filterArtifacts, artifact)
		keptArtifactIDs[artifact.ID] = struct{}{}
	}
	r.artifacts = filterArtifacts

	filterDeliveries := make([]ReportDelivery, 0, len(r.deliveries))
	for _, delivery := range r.deliveries {
		if delivery.CreatedAt.Before(cutoff) {
			continue
		}
		if _, ok := keptArtifactIDs[delivery.ArtifactID]; !ok {
			continue
		}
		filterDeliveries = append(filterDeliveries, delivery)
	}
	r.deliveries = filterDeliveries

	filterDead := make([]ReportDeliveryDeadLetter, 0, len(r.deliveryDeadLetters))
	for _, record := range r.deliveryDeadLetters {
		if record.CreatedAt.Before(cutoff) {
			continue
		}
		if _, ok := keptArtifactIDs[record.ArtifactID]; !ok {
			continue
		}
		filterDead = append(filterDead, record)
	}
	r.deliveryDeadLetters = filterDead
	return nil
}
