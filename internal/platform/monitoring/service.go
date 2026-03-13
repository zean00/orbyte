package monitoring

import (
	"time"

	"orbyte/internal/platform/document"
	"orbyte/internal/platform/eventing"
	"orbyte/internal/platform/observability"
	"orbyte/internal/platform/search"
	"orbyte/internal/platform/workflow"
)

type Summary struct {
	GeneratedAt time.Time              `json:"generated_at"`
	Documents   DocumentSummary        `json:"documents"`
	Outbox      OutboxSummary          `json:"outbox"`
	Workflow    WorkflowSummary        `json:"workflow"`
	Projections ProjectionSummary      `json:"projections"`
	Metrics     observability.Snapshot `json:"metrics"`
}

type DocumentSummary struct {
	Total    int            `json:"total"`
	ByStatus map[string]int `json:"by_status"`
}

type OutboxSummary struct {
	Pending     int `json:"pending"`
	Processing  int `json:"processing"`
	Dispatched  int `json:"dispatched"`
	DeadLetter  int `json:"dead_letter"`
	DeadLetters int `json:"dead_letters"`
}

type WorkflowSummary struct {
	OpenTasks         int `json:"open_tasks"`
	CompletedTasks    int `json:"completed_tasks"`
	CancelledTasks    int `json:"cancelled_tasks"`
	PendingApprovals  int `json:"pending_approvals"`
	ApprovedApprovals int `json:"approved_approvals"`
	RejectedApprovals int `json:"rejected_approvals"`
}

type ProjectionSummary struct {
	DocumentSummaries int `json:"document_summaries"`
}

type Service struct {
	documents *document.Service
	eventing  *eventing.Service
	workflows *workflow.Service
	search    *search.Service
	obs       *observability.Service
}

func NewService(documents *document.Service, eventingSvc *eventing.Service, workflowSvc *workflow.Service, searchSvc *search.Service, obs *observability.Service) *Service {
	return &Service{documents: documents, eventing: eventingSvc, workflows: workflowSvc, search: searchSvc, obs: obs}
}

func (s *Service) Summary() Summary {
	docSummary := DocumentSummary{ByStatus: map[string]int{}}
	for _, record := range s.documents.List() {
		docSummary.Total++
		docSummary.ByStatus[record.Header.Status]++
	}
	outboxSummary := OutboxSummary{DeadLetters: len(s.eventing.ListDeadLetters())}
	for _, item := range s.eventing.ListOutbox() {
		switch item.Status {
		case "pending":
			outboxSummary.Pending++
		case "processing":
			outboxSummary.Processing++
		case "dispatched":
			outboxSummary.Dispatched++
		case "dead_letter":
			outboxSummary.DeadLetter++
		}
	}
	workflowSummary := WorkflowSummary{}
	for _, task := range s.workflows.ListTasks() {
		switch task.Status {
		case "open":
			workflowSummary.OpenTasks++
		case "completed":
			workflowSummary.CompletedTasks++
		case "cancelled":
			workflowSummary.CancelledTasks++
		}
	}
	for _, approval := range s.workflows.ListApprovals() {
		switch approval.Status {
		case "pending":
			workflowSummary.PendingApprovals++
		case "approved":
			workflowSummary.ApprovedApprovals++
		case "rejected":
			workflowSummary.RejectedApprovals++
		}
	}
	projectionSummary := ProjectionSummary{DocumentSummaries: len(s.search.ListDocuments())}
	metrics := observability.Snapshot{}
	if s.obs != nil {
		metrics = s.obs.Snapshot()
	}
	return Summary{
		GeneratedAt: time.Now().UTC(),
		Documents:   docSummary,
		Outbox:      outboxSummary,
		Workflow:    workflowSummary,
		Projections: projectionSummary,
		Metrics:     metrics,
	}
}
