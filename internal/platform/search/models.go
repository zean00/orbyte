package search

import "time"

type ProjectionDefinition struct {
	Key           string   `json:"key"`
	SourceTypes   []string `json:"source_types"`
	IndexedFields []string `json:"indexed_fields"`
	DisplayFields []string `json:"display_fields"`
}

type IndexDefinition struct {
	Key                 string                  `json:"key"`
	Title               string                  `json:"title"`
	SourceKind          string                  `json:"source_kind"`
	DocumentType        string                  `json:"document_type,omitempty"`
	ModelKey            string                  `json:"model_key,omitempty"`
	ProjectionKey       string                  `json:"projection_key,omitempty"`
	ViewKey             string                  `json:"view_key,omitempty"`
	ActionKey           string                  `json:"action_key,omitempty"`
	Modes               []string                `json:"modes,omitempty"`
	OrganizationSplit   bool                    `json:"organization_split"`
	RequiredPermissions []string                `json:"required_permissions,omitempty"`
	QueryFilterFields   []string                `json:"query_filter_fields,omitempty"`
	QuerySortFields     []string                `json:"query_sort_fields,omitempty"`
	Fields              []IndexFieldDefinition  `json:"fields,omitempty"`
	VectorFields        []VectorFieldDefinition `json:"vector_fields,omitempty"`
}

type IndexFieldDefinition struct {
	Key        string `json:"key"`
	Path       string `json:"path"`
	Type       string `json:"type"`
	Facet      bool   `json:"facet,omitempty"`
	Sort       bool   `json:"sort,omitempty"`
	Optional   bool   `json:"optional,omitempty"`
	Searchable bool   `json:"searchable,omitempty"`
}

type VectorFieldDefinition struct {
	Key            string   `json:"key"`
	SourcePaths    []string `json:"source_paths,omitempty"`
	EmbeddingMode  string   `json:"embedding_mode"`
	ModelKey       string   `json:"model_key,omitempty"`
	RemoteModel    string   `json:"remote_model,omitempty"`
	Dimensions     int      `json:"dimensions,omitempty"`
	DistanceMetric string   `json:"distance_metric,omitempty"`
}

type IndexedRecord struct {
	ID             string               `json:"id"`
	SourceID       string               `json:"source_id"`
	SourceKind     string               `json:"source_kind"`
	OrganizationID string               `json:"organization_id"`
	LocationID     string               `json:"location_id,omitempty"`
	Version        int                  `json:"version"`
	UpdatedAt      time.Time            `json:"updated_at"`
	Fields         map[string]any       `json:"fields"`
	Vectors        map[string][]float32 `json:"vectors,omitempty"`
}

type QueryRequest struct {
	Mode          string            `json:"mode,omitempty"`
	Query         string            `json:"query,omitempty"`
	VectorText    string            `json:"vector_text,omitempty"`
	Vector        []float32         `json:"vector,omitempty"`
	VectorField   string            `json:"vector_field,omitempty"`
	Filters       map[string]string `json:"filters,omitempty"`
	SortBy        string            `json:"sort_by,omitempty"`
	Desc          bool              `json:"desc,omitempty"`
	Page          int               `json:"page,omitempty"`
	PageSize      int               `json:"page_size,omitempty"`
	IncludeFields []string          `json:"include_fields,omitempty"`
	Limit         int               `json:"limit,omitempty"`
}

type QueryHit struct {
	ID         string         `json:"id"`
	SourceID   string         `json:"source_id"`
	SourceKind string         `json:"source_kind"`
	Score      float64        `json:"score"`
	Fields     map[string]any `json:"fields,omitempty"`
}

type QueryResult struct {
	IndexKey string     `json:"index_key"`
	Mode     string     `json:"mode"`
	Total    int        `json:"total"`
	Page     int        `json:"page"`
	PageSize int        `json:"page_size"`
	Hits     []QueryHit `json:"hits"`
}
