package analytics

import (
	"fmt"
	"sort"
	"time"

	"clinic/internal/platform/audit"
	"clinic/internal/platform/document"
	"clinic/internal/platform/eventing"
	"clinic/internal/platform/jobs"
	"clinic/internal/platform/observability"
	"clinic/internal/platform/search"
	"clinic/internal/platform/workflow"
)

type Snapshot struct {
	ID          string             `json:"id"`
	GeneratedAt time.Time          `json:"generated_at"`
	Window      string             `json:"window"`
	Documents   DocumentKPI        `json:"documents"`
	Segments    SegmentKPI         `json:"segments"`
	Workflow    WorkflowKPI        `json:"workflow"`
	Reliability ReliabilityKPI     `json:"reliability"`
	Coverage    CoverageKPI        `json:"coverage"`
	Metrics     map[string]float64 `json:"metrics"`
}

type DocumentKPI struct {
	Created   int `json:"created"`
	Draft     int `json:"draft"`
	Submitted int `json:"submitted"`
	Approved  int `json:"approved"`
	Rejected  int `json:"rejected"`
	Cancelled int `json:"cancelled"`
}

type WorkflowKPI struct {
	OpenTasks        int     `json:"open_tasks"`
	PendingApprovals int     `json:"pending_approvals"`
	CompletedTasks   int     `json:"completed_tasks"`
	ApprovalRate     float64 `json:"approval_rate"`
	RejectionRate    float64 `json:"rejection_rate"`
}

type ReliabilityKPI struct {
	OutboxPending     int     `json:"outbox_pending"`
	OutboxDeadLetters int     `json:"outbox_dead_letters"`
	DispatchSuccess   int64   `json:"dispatch_success"`
	DispatchRetries   int64   `json:"dispatch_retries"`
	DeadLetterRate    float64 `json:"dead_letter_rate"`
}

type CoverageKPI struct {
	DocumentSummaries  int     `json:"document_summaries"`
	ProjectionCoverage float64 `json:"projection_coverage"`
	AuditEvents        int     `json:"audit_events"`
}

type SegmentKPI struct {
	ByDocumentType map[string]DocumentKPI `json:"by_document_type"`
	ByLocation     map[string]DocumentKPI `json:"by_location"`
}

type TrendPoint struct {
	SnapshotID         string    `json:"snapshot_id"`
	GeneratedAt        time.Time `json:"generated_at"`
	SubmittedDocuments int       `json:"submitted_documents"`
	ApprovedDocuments  int       `json:"approved_documents"`
	PendingApprovals   int       `json:"pending_approvals"`
	DeadLetters        int       `json:"dead_letters"`
}

type Rollup struct {
	ID            string         `json:"id"`
	Granularity   string         `json:"granularity"`
	BucketStart   time.Time      `json:"bucket_start"`
	BucketEnd     time.Time      `json:"bucket_end"`
	SnapshotCount int            `json:"snapshot_count"`
	Documents     DocumentKPI    `json:"documents"`
	Segments      SegmentKPI     `json:"segments"`
	Workflow      WorkflowKPI    `json:"workflow"`
	Reliability   ReliabilityKPI `json:"reliability"`
}

type Comparison struct {
	Current  SnapshotDeltaBase `json:"current"`
	Previous SnapshotDeltaBase `json:"previous"`
	Delta    SnapshotDelta     `json:"delta"`
}

type SnapshotDeltaBase struct {
	WindowFrom  time.Time      `json:"window_from"`
	WindowTo    time.Time      `json:"window_to"`
	Documents   DocumentKPI    `json:"documents"`
	Workflow    WorkflowKPI    `json:"workflow"`
	Reliability ReliabilityKPI `json:"reliability"`
}

type SnapshotDelta struct {
	SubmittedDocuments int `json:"submitted_documents"`
	ApprovedDocuments  int `json:"approved_documents"`
	RejectedDocuments  int `json:"rejected_documents"`
	PendingApprovals   int `json:"pending_approvals"`
	DeadLetters        int `json:"dead_letters"`
}

type Query struct {
	Window string
	From   time.Time
	To     time.Time
	Limit  int
}

type FactQuery struct {
	From         time.Time
	To           time.Time
	LocationID   string
	DocumentType string
}

type DocumentFact struct {
	SnapshotID   string    `json:"snapshot_id"`
	CapturedAt   time.Time `json:"captured_at"`
	LocationID   string    `json:"location_id,omitempty"`
	DocumentType string    `json:"document_type,omitempty"`
	Created      int       `json:"created"`
	Draft        int       `json:"draft"`
	Submitted    int       `json:"submitted"`
	Approved     int       `json:"approved"`
	Rejected     int       `json:"rejected"`
	Cancelled    int       `json:"cancelled"`
}

type WorkflowFact struct {
	SnapshotID       string    `json:"snapshot_id"`
	CapturedAt       time.Time `json:"captured_at"`
	PendingApprovals int       `json:"pending_approvals"`
	OpenTasks        int       `json:"open_tasks"`
	CompletedTasks   int       `json:"completed_tasks"`
}

type ReliabilityFact struct {
	SnapshotID      string    `json:"snapshot_id"`
	CapturedAt      time.Time `json:"captured_at"`
	OutboxPending   int       `json:"outbox_pending"`
	DeadLetters     int       `json:"dead_letters"`
	DispatchSuccess int64     `json:"dispatch_success"`
	DispatchRetries int64     `json:"dispatch_retries"`
}

type FactBundle struct {
	Documents   []DocumentFact    `json:"documents"`
	Workflow    []WorkflowFact    `json:"workflow"`
	Reliability []ReliabilityFact `json:"reliability"`
}

type DocumentTypeDimension struct {
	DocumentType string `json:"document_type"`
	DisplayName  string `json:"display_name"`
}

type LocationDimension struct {
	LocationID string `json:"location_id"`
	Name       string `json:"name"`
}

type DimensionBundle struct {
	DocumentTypes []DocumentTypeDimension `json:"document_types"`
	Locations     []LocationDimension     `json:"locations"`
}

type ReportingRow struct {
	DimensionType string `json:"dimension_type"`
	DimensionKey  string `json:"dimension_key"`
	Label         string `json:"label"`
	Created       int    `json:"created"`
	Draft         int    `json:"draft"`
	Submitted     int    `json:"submitted"`
	Approved      int    `json:"approved"`
	Rejected      int    `json:"rejected"`
	Cancelled     int    `json:"cancelled"`
}

type ReportDefinition struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Dimension       string    `json:"dimension"`
	Format          string    `json:"format"`
	Window          string    `json:"window"`
	LocationID      string    `json:"location_id,omitempty"`
	DocumentType    string    `json:"document_type,omitempty"`
	DeliveryChannel string    `json:"delivery_channel,omitempty"`
	DeliveryTarget  string    `json:"delivery_target,omitempty"`
	Schedule        string    `json:"schedule"`
	NextRunAt       time.Time `json:"next_run_at"`
	Enabled         bool      `json:"enabled"`
}

type ReportRun struct {
	ID          string    `json:"id"`
	ReportID    string    `json:"report_id"`
	ArtifactID  string    `json:"artifact_id,omitempty"`
	Format      string    `json:"format"`
	Status      string    `json:"status"`
	GeneratedAt time.Time `json:"generated_at"`
	RowCount    int       `json:"row_count"`
}

type ReportArtifact struct {
	ID          string    `json:"id"`
	ReportID    string    `json:"report_id"`
	ReportRunID string    `json:"report_run_id"`
	FileName    string    `json:"file_name"`
	ContentType string    `json:"content_type"`
	SizeBytes   int       `json:"size_bytes"`
	CreatedAt   time.Time `json:"created_at"`
	Content     []byte    `json:"-"`
}

type ReportDelivery struct {
	ID           string    `json:"id"`
	ArtifactID   string    `json:"artifact_id"`
	Channel      string    `json:"channel"`
	Recipient    string    `json:"recipient,omitempty"`
	Status       string    `json:"status"`
	AttemptCount int       `json:"attempt_count"`
	LastError    string    `json:"last_error,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	DeliveredAt  time.Time `json:"delivered_at,omitempty"`
}

type ReportDeliveryDeadLetter struct {
	ID           string    `json:"id"`
	ArtifactID   string    `json:"artifact_id"`
	Channel      string    `json:"channel"`
	Recipient    string    `json:"recipient,omitempty"`
	Reason       string    `json:"reason"`
	AttemptCount int       `json:"attempt_count"`
	CreatedAt    time.Time `json:"created_at"`
}

type Service struct {
	documents *document.Service
	workflow  *workflow.Service
	eventing  *eventing.Service
	search    *search.Service
	audit     *audit.Service
	obs       *observability.Service
	repo      Repository
	jobs      *jobs.Service
}

type ConsistencyReport struct {
	GeneratedAt        time.Time               `json:"generated_at"`
	DocumentCount      int                     `json:"document_count"`
	ProjectionCount    int                     `json:"projection_count"`
	SnapshotCount      int                     `json:"snapshot_count"`
	FactDocumentCount  int                     `json:"fact_document_count"`
	LatestSnapshotAt   time.Time               `json:"latest_snapshot_at,omitempty"`
	ProjectionCoverage float64                 `json:"projection_coverage"`
	Observations       map[string]any          `json:"observations,omitempty"`
}

func NewService(documents *document.Service, workflowSvc *workflow.Service, eventingSvc *eventing.Service, searchSvc *search.Service, auditSvc *audit.Service, obs *observability.Service) *Service {
	return NewServiceWithRepository(documents, workflowSvc, eventingSvc, searchSvc, auditSvc, obs, NewMemoryRepository())
}

func NewServiceWithRepository(documents *document.Service, workflowSvc *workflow.Service, eventingSvc *eventing.Service, searchSvc *search.Service, auditSvc *audit.Service, obs *observability.Service, repo Repository) *Service {
	return &Service{documents: documents, workflow: workflowSvc, eventing: eventingSvc, search: searchSvc, audit: auditSvc, obs: obs, repo: repo}
}

func (s *Service) Snapshot() Snapshot {
	var docs DocumentKPI
	segments := SegmentKPI{ByDocumentType: map[string]DocumentKPI{}, ByLocation: map[string]DocumentKPI{}}
	allDocs := s.documents.List()
	for _, record := range allDocs {
		docs.Created++
		incrementDocumentKPI(&docs, record.Header.Status)
		typeKPI := segments.ByDocumentType[record.Header.Type]
		typeKPI.Created++
		incrementDocumentKPI(&typeKPI, record.Header.Status)
		segments.ByDocumentType[record.Header.Type] = typeKPI
		locationKey := record.Header.LocationID
		if locationKey == "" {
			locationKey = "unscoped"
		}
		locationKPI := segments.ByLocation[locationKey]
		locationKPI.Created++
		incrementDocumentKPI(&locationKPI, record.Header.Status)
		segments.ByLocation[locationKey] = locationKPI
	}

	var wf WorkflowKPI
	for _, task := range s.workflow.ListTasks() {
		switch task.Status {
		case "open":
			wf.OpenTasks++
		case "completed":
			wf.CompletedTasks++
		}
	}
	approvals := s.workflow.ListApprovals()
	approved, rejected := 0, 0
	for _, approval := range approvals {
		switch approval.Status {
		case "pending":
			wf.PendingApprovals++
		case "approved":
			approved++
		case "rejected":
			rejected++
		}
	}
	if totalResolved := approved + rejected; totalResolved > 0 {
		wf.ApprovalRate = float64(approved) / float64(totalResolved)
		wf.RejectionRate = float64(rejected) / float64(totalResolved)
	}

	outbox := s.eventing.ListOutbox()
	deadLetters := s.eventing.ListDeadLetters()
	rel := ReliabilityKPI{OutboxDeadLetters: len(deadLetters)}
	for _, item := range outbox {
		if item.Status == "pending" {
			rel.OutboxPending++
		}
	}
	if s.obs != nil {
		snap := s.obs.Snapshot()
		rel.DispatchSuccess = snap.Counters["outbox.dispatch.success.total"]
		rel.DispatchRetries = snap.Counters["outbox.dispatch.retry.total"]
		totalTerminal := rel.DispatchSuccess + int64(rel.OutboxDeadLetters)
		if totalTerminal > 0 {
			rel.DeadLetterRate = float64(rel.OutboxDeadLetters) / float64(totalTerminal)
		}
	}

	coverage := CoverageKPI{
		DocumentSummaries: len(s.search.ListDocuments()),
		AuditEvents:       len(s.audit.List()),
	}
	if len(allDocs) > 0 {
		coverage.ProjectionCoverage = float64(coverage.DocumentSummaries) / float64(len(allDocs))
	}

	metricMap := map[string]float64{}
	if s.obs != nil {
		for k, v := range s.obs.Snapshot().Counters {
			metricMap[k] = float64(v)
		}
	}

	return Snapshot{
		ID:          fmt.Sprintf("analytics:%d", time.Now().UTC().UnixNano()),
		GeneratedAt: time.Now().UTC(),
		Window:      "current_state",
		Documents:   docs,
		Segments:    segments,
		Workflow:    wf,
		Reliability: rel,
		Coverage:    coverage,
		Metrics:     metricMap,
	}
}

func incrementDocumentKPI(kpi *DocumentKPI, status string) {
	switch status {
	case "draft":
		kpi.Draft++
	case "submitted":
		kpi.Submitted++
	case "approved":
		kpi.Approved++
	case "rejected":
		kpi.Rejected++
	case "cancelled":
		kpi.Cancelled++
	}
}

func (s *Service) CaptureSnapshot() (Snapshot, error) {
	snapshot := s.Snapshot()
	if s.repo != nil {
		if err := s.repo.SaveDimensions(s.buildDimensions(snapshot)); err != nil {
			return Snapshot{}, err
		}
		if err := s.repo.SaveSnapshot(snapshot); err != nil {
			return Snapshot{}, err
		}
		if err := s.repo.SaveFacts(s.buildFacts(snapshot)); err != nil {
			return Snapshot{}, err
		}
		if err := s.RefreshRollups(); err != nil {
			return Snapshot{}, err
		}
	}
	return snapshot, nil
}

func (s *Service) QueryFacts(query FactQuery) FactBundle {
	if s.repo == nil {
		return FactBundle{}
	}
	return s.repo.QueryFacts(query)
}

func (s *Service) ReportingBreakdown(query FactQuery, dimension string) []ReportingRow {
	if s.repo == nil {
		return nil
	}
	return s.repo.ReportingBreakdown(query, dimension)
}

func (s *Service) ListSnapshots() []Snapshot {
	if s.repo == nil {
		return nil
	}
	return s.repo.ListSnapshots()
}

func (s *Service) QuerySnapshots(query Query) []Snapshot {
	if s.repo == nil {
		return nil
	}
	return s.repo.QuerySnapshots(query)
}

func (s *Service) ListRecent(limit int) []Snapshot {
	if s.repo == nil {
		return nil
	}
	return s.repo.ListRecent(limit)
}

func (s *Service) DeleteOlderThan(cutoff time.Time) error {
	if s.repo == nil {
		return nil
	}
	return s.repo.DeleteOlderThan(cutoff)
}

func (s *Service) Trends(limit int) []TrendPoint {
	return s.TrendsByQuery(Query{Limit: limit})
}

func (s *Service) TrendsByQuery(query Query) []TrendPoint {
	snapshots := s.QuerySnapshots(query)
	points := make([]TrendPoint, 0, len(snapshots))
	for _, snapshot := range snapshots {
		points = append(points, TrendPoint{
			SnapshotID:         snapshot.ID,
			GeneratedAt:        snapshot.GeneratedAt,
			SubmittedDocuments: snapshot.Documents.Submitted,
			ApprovedDocuments:  snapshot.Documents.Approved,
			PendingApprovals:   snapshot.Workflow.PendingApprovals,
			DeadLetters:        snapshot.Reliability.OutboxDeadLetters,
		})
	}
	return points
}

func (s *Service) LatestSnapshot(query Query) (Snapshot, bool) {
	snapshots := s.QuerySnapshots(query)
	if len(snapshots) == 0 {
		return Snapshot{}, false
	}
	return snapshots[len(snapshots)-1], true
}

func (s *Service) Breakdown(query Query, groupBy string) (map[string]DocumentKPI, bool) {
	snapshot, ok := s.LatestSnapshot(query)
	if !ok {
		return nil, false
	}
	switch groupBy {
	case "document_type":
		return snapshot.Segments.ByDocumentType, true
	case "location":
		return snapshot.Segments.ByLocation, true
	default:
		return nil, false
	}
}

func (s *Service) RefreshRollups() error {
	for _, granularity := range []string{"daily", "weekly", "monthly"} {
		for _, rollup := range s.buildRollups(granularity, s.ListSnapshots()) {
			if err := s.repo.UpsertRollup(rollup); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Service) RecomputeCurrentState() (Snapshot, error) {
	return s.CaptureSnapshot()
}

func (s *Service) Consistency() ConsistencyReport {
	report := ConsistencyReport{
		GeneratedAt:   time.Now().UTC(),
		DocumentCount: len(s.documents.List()),
		ProjectionCount: len(s.search.ListDocuments()),
		SnapshotCount: len(s.ListSnapshots()),
		Observations:  map[string]any{},
	}
	if report.DocumentCount > 0 {
		report.ProjectionCoverage = float64(report.ProjectionCount) / float64(report.DocumentCount)
	}
	facts := s.QueryFacts(FactQuery{})
	report.FactDocumentCount = len(facts.Documents)
	if snapshots := s.ListSnapshots(); len(snapshots) > 0 {
		report.LatestSnapshotAt = snapshots[len(snapshots)-1].GeneratedAt
	}
	report.Observations["outbox_pending"] = len(s.eventing.ListOutbox())
	report.Observations["audit_events"] = len(s.audit.List())
	return report
}

func (s *Service) ListRollups(granularity string, limit int) []Rollup {
	if s.repo == nil {
		return nil
	}
	return s.repo.QueryRollups(granularity, time.Time{}, time.Time{}, limit)
}

func (s *Service) buildRollups(granularity string, snapshots []Snapshot) []Rollup {
	buckets := map[string]*Rollup{}
	for _, snapshot := range snapshots {
		start, end := bucketRange(granularity, snapshot.GeneratedAt)
		key := granularity + ":" + start.Format(time.RFC3339)
		rollup, ok := buckets[key]
		if !ok {
			rollup = &Rollup{ID: key, Granularity: granularity, BucketStart: start, BucketEnd: end, Segments: SegmentKPI{ByDocumentType: map[string]DocumentKPI{}, ByLocation: map[string]DocumentKPI{}}}
			buckets[key] = rollup
		}
		rollup.SnapshotCount++
		rollup.Documents.Created += snapshot.Documents.Created
		rollup.Documents.Draft += snapshot.Documents.Draft
		rollup.Documents.Submitted += snapshot.Documents.Submitted
		rollup.Documents.Approved += snapshot.Documents.Approved
		rollup.Documents.Rejected += snapshot.Documents.Rejected
		rollup.Documents.Cancelled += snapshot.Documents.Cancelled
		mergeSegments(&rollup.Segments.ByDocumentType, snapshot.Segments.ByDocumentType)
		mergeSegments(&rollup.Segments.ByLocation, snapshot.Segments.ByLocation)
		rollup.Workflow.OpenTasks += snapshot.Workflow.OpenTasks
		rollup.Workflow.PendingApprovals += snapshot.Workflow.PendingApprovals
		rollup.Workflow.CompletedTasks += snapshot.Workflow.CompletedTasks
		rollup.Reliability.OutboxPending += snapshot.Reliability.OutboxPending
		rollup.Reliability.OutboxDeadLetters += snapshot.Reliability.OutboxDeadLetters
		rollup.Reliability.DispatchSuccess += snapshot.Reliability.DispatchSuccess
		rollup.Reliability.DispatchRetries += snapshot.Reliability.DispatchRetries
	}
	items := make([]Rollup, 0, len(buckets))
	for _, rollup := range buckets {
		items = append(items, *rollup)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].BucketStart.Before(items[j].BucketStart) })
	return items
}

func mergeSegments(target *map[string]DocumentKPI, source map[string]DocumentKPI) {
	if *target == nil {
		*target = map[string]DocumentKPI{}
	}
	for key, value := range source {
		current := (*target)[key]
		current.Created += value.Created
		current.Draft += value.Draft
		current.Submitted += value.Submitted
		current.Approved += value.Approved
		current.Rejected += value.Rejected
		current.Cancelled += value.Cancelled
		(*target)[key] = current
	}
}

func (s *Service) RollupBreakdown(granularity, groupBy string, limit int) (map[string]DocumentKPI, bool) {
	rollups := s.ListRollups(granularity, limit)
	if len(rollups) == 0 {
		return nil, false
	}
	latest := rollups[len(rollups)-1]
	switch groupBy {
	case "document_type":
		return latest.Segments.ByDocumentType, true
	case "location":
		return latest.Segments.ByLocation, true
	default:
		return nil, false
	}
}

func (s *Service) Compare(query Query) (Comparison, bool) {
	current, ok := s.LatestSnapshot(query)
	if !ok {
		return Comparison{}, false
	}
	currentFrom, currentTo := comparisonWindow(query, current.GeneratedAt)
	windowDuration := currentTo.Sub(currentFrom)
	previousQuery := query
	previousQuery.From = currentFrom.Add(-windowDuration)
	previousQuery.To = currentFrom
	previous, ok := s.LatestSnapshot(previousQuery)
	if !ok {
		return Comparison{
			Current: SnapshotDeltaBase{WindowFrom: currentFrom, WindowTo: currentTo, Documents: current.Documents, Workflow: current.Workflow, Reliability: current.Reliability},
		}, true
	}
	return Comparison{
		Current:  SnapshotDeltaBase{WindowFrom: currentFrom, WindowTo: currentTo, Documents: current.Documents, Workflow: current.Workflow, Reliability: current.Reliability},
		Previous: SnapshotDeltaBase{WindowFrom: previousQuery.From, WindowTo: previousQuery.To, Documents: previous.Documents, Workflow: previous.Workflow, Reliability: previous.Reliability},
		Delta: SnapshotDelta{
			SubmittedDocuments: current.Documents.Submitted - previous.Documents.Submitted,
			ApprovedDocuments:  current.Documents.Approved - previous.Documents.Approved,
			RejectedDocuments:  current.Documents.Rejected - previous.Documents.Rejected,
			PendingApprovals:   current.Workflow.PendingApprovals - previous.Workflow.PendingApprovals,
			DeadLetters:        current.Reliability.OutboxDeadLetters - previous.Reliability.OutboxDeadLetters,
		},
	}, true
}

func comparisonWindow(query Query, fallback time.Time) (time.Time, time.Time) {
	to := query.To
	if to.IsZero() {
		to = fallback
	}
	from := query.From
	if from.IsZero() {
		from = to.Add(-24 * time.Hour)
	}
	return from, to
}

func bucketRange(granularity string, ts time.Time) (time.Time, time.Time) {
	t := ts.UTC()
	switch granularity {
	case "weekly":
		weekday := int(t.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		start := time.Date(t.Year(), t.Month(), t.Day()-(weekday-1), 0, 0, 0, 0, time.UTC)
		return start, start.AddDate(0, 0, 7)
	case "monthly":
		start := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
		return start, start.AddDate(0, 1, 0)
	default:
		start := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
		return start, start.AddDate(0, 0, 1)
	}
}

func (s *Service) buildFacts(snapshot Snapshot) FactBundle {
	bundle := FactBundle{}
	for documentType, kpi := range snapshot.Segments.ByDocumentType {
		bundle.Documents = append(bundle.Documents, DocumentFact{
			SnapshotID:   snapshot.ID,
			CapturedAt:   snapshot.GeneratedAt,
			DocumentType: documentType,
			Created:      kpi.Created,
			Draft:        kpi.Draft,
			Submitted:    kpi.Submitted,
			Approved:     kpi.Approved,
			Rejected:     kpi.Rejected,
			Cancelled:    kpi.Cancelled,
		})
	}
	for locationID, kpi := range snapshot.Segments.ByLocation {
		bundle.Documents = append(bundle.Documents, DocumentFact{
			SnapshotID: snapshot.ID,
			CapturedAt: snapshot.GeneratedAt,
			LocationID: locationID,
			Created:    kpi.Created,
			Draft:      kpi.Draft,
			Submitted:  kpi.Submitted,
			Approved:   kpi.Approved,
			Rejected:   kpi.Rejected,
			Cancelled:  kpi.Cancelled,
		})
	}
	bundle.Workflow = append(bundle.Workflow, WorkflowFact{
		SnapshotID:       snapshot.ID,
		CapturedAt:       snapshot.GeneratedAt,
		PendingApprovals: snapshot.Workflow.PendingApprovals,
		OpenTasks:        snapshot.Workflow.OpenTasks,
		CompletedTasks:   snapshot.Workflow.CompletedTasks,
	})
	bundle.Reliability = append(bundle.Reliability, ReliabilityFact{
		SnapshotID:      snapshot.ID,
		CapturedAt:      snapshot.GeneratedAt,
		OutboxPending:   snapshot.Reliability.OutboxPending,
		DeadLetters:     snapshot.Reliability.OutboxDeadLetters,
		DispatchSuccess: snapshot.Reliability.DispatchSuccess,
		DispatchRetries: snapshot.Reliability.DispatchRetries,
	})
	return bundle
}

func (s *Service) buildDimensions(snapshot Snapshot) DimensionBundle {
	bundle := DimensionBundle{}
	for documentType := range snapshot.Segments.ByDocumentType {
		bundle.DocumentTypes = append(bundle.DocumentTypes, DocumentTypeDimension{DocumentType: documentType, DisplayName: documentType})
	}
	for locationID := range snapshot.Segments.ByLocation {
		bundle.Locations = append(bundle.Locations, LocationDimension{LocationID: locationID, Name: locationID})
	}
	sort.Slice(bundle.DocumentTypes, func(i, j int) bool {
		return bundle.DocumentTypes[i].DocumentType < bundle.DocumentTypes[j].DocumentType
	})
	sort.Slice(bundle.Locations, func(i, j int) bool { return bundle.Locations[i].LocationID < bundle.Locations[j].LocationID })
	return bundle
}

func nextRun(schedule string, from time.Time) time.Time {
	switch schedule {
	case "hourly":
		return from.Add(time.Hour)
	case "weekly":
		return from.AddDate(0, 0, 7)
	case "monthly":
		return from.AddDate(0, 1, 0)
	default:
		return from.Add(24 * time.Hour)
	}
}
