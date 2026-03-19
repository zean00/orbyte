package search

type Repository interface {
	SaveDocument(summary DocumentSummary) error
	ListDocuments() []DocumentSummary
	SaveProjectionStatus(status ProjectionStatus) error
	ListProjectionStatuses() []ProjectionStatus
}
