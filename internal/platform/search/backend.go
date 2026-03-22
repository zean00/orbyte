package search

type Backend interface {
	EnsureIndex(def IndexDefinition, organizationID string) error
	Upsert(def IndexDefinition, organizationID string, record IndexedRecord) error
	Delete(def IndexDefinition, organizationID, sourceID string) error
	Search(def IndexDefinition, organizationID string, req QueryRequest) (QueryResult, error)
	List(def IndexDefinition, organizationID string) ([]IndexedRecord, error)
	Capabilities(def IndexDefinition) BackendCapabilities
}

type Embedder interface {
	Embed(texts []string, dimensions int) ([][]float32, error)
}
