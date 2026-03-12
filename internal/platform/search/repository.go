package search

type Repository interface {
	SaveDocument(summary DocumentSummary) error
	ListDocuments() []DocumentSummary
}
