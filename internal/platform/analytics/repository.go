package analytics

import "time"

type Repository interface {
	SaveDimensions(dimensions DimensionBundle) error
	SaveSnapshot(snapshot Snapshot) error
	ListSnapshots() []Snapshot
	QuerySnapshots(query Query) []Snapshot
	ListRecent(limit int) []Snapshot
	DeleteOlderThan(cutoff time.Time) error
	UpsertRollup(rollup Rollup) error
	ListRollups(granularity string, limit int) []Rollup
	QueryRollups(granularity string, from, to time.Time, limit int) []Rollup
	SaveFacts(facts FactBundle) error
	QueryFacts(query FactQuery) FactBundle
	ReportingBreakdown(query FactQuery, dimension string) []ReportingRow
	SaveDashboard(item Dashboard) error
	ListDashboards() []Dashboard
	GetDashboard(id string) (Dashboard, bool)
	DeleteDashboard(id string) error
	SaveSavedMetric(item SavedMetric) error
	ListSavedMetrics() []SavedMetric
	GetSavedMetric(id string) (SavedMetric, bool)
	DeleteSavedMetric(id string) error
	SaveSavedQuery(item SavedQuery) error
	ListSavedQueries() []SavedQuery
	GetSavedQuery(id string) (SavedQuery, bool)
	DeleteSavedQuery(id string) error
	SaveReportDefinition(def ReportDefinition) error
	ListReportDefinitions() []ReportDefinition
	UpdateReportDefinition(def ReportDefinition) error
	DeleteReportDefinition(id string) error
	SaveReportRun(run ReportRun) error
	ListReportRuns() []ReportRun
	SaveReportArtifact(artifact ReportArtifact) error
	ListReportArtifacts() []ReportArtifact
	GetReportArtifact(id string) (ReportArtifact, bool)
	SaveReportDelivery(delivery ReportDelivery) error
	ListReportDeliveries() []ReportDelivery
	SaveReportDeliveryDeadLetter(record ReportDeliveryDeadLetter) error
	ListReportDeliveryDeadLetters() []ReportDeliveryDeadLetter
	CleanupReportData(cutoff time.Time) error
}
