package document

import (
	"time"

	"orbyte/internal/platform/shared"
)

type Definition struct {
	Type                   string   `json:"type"`
	DisplayName            string   `json:"display_name"`
	SchemaVersion          string   `json:"schema_version"`
	WorkflowKey            string   `json:"workflow_key,omitempty"`
	NumberingKey           string   `json:"numbering_key,omitempty"`
	OwnerModuleKey         string   `json:"owner_module_key,omitempty"`
	AllowedLinkTypes       []string `json:"allowed_link_types,omitempty"`
	AllowedAttachmentTypes []string `json:"allowed_attachment_types,omitempty"`
}

type ExtensionDefinition struct {
	DocumentType       string `json:"document_type"`
	ModuleKey          string `json:"module_key"`
	DisplayName        string `json:"display_name"`
	SchemaVersion      string `json:"schema_version"`
	ReadPermissionKey  string `json:"read_permission_key,omitempty"`
	WritePermissionKey string `json:"write_permission_key,omitempty"`
}

type Header struct {
	ID             string         `json:"id"`
	Type           string         `json:"type"`
	Status         string         `json:"status"`
	Version        int            `json:"version"`
	ETag           string         `json:"etag"`
	OrganizationID string         `json:"organization_id"`
	LocationID     string         `json:"location_id,omitempty"`
	Number         string         `json:"number,omitempty"`
	CreatedBy      string         `json:"created_by"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedBy      string         `json:"updated_by"`
	UpdatedAt      time.Time      `json:"updated_at"`
	SubmittedBy    string         `json:"submitted_by,omitempty"`
	SubmittedAt    time.Time      `json:"submitted_at,omitempty"`
	TotalAmount    shared.Money   `json:"total_amount"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

type Body struct {
	DocumentID    string         `json:"document_id"`
	SchemaVersion string         `json:"schema_version"`
	Payload       map[string]any `json:"payload"`
	ContentHash   string         `json:"content_hash,omitempty"`
}

type Line struct {
	ID         string         `json:"id"`
	DocumentID string         `json:"document_id"`
	LineNo     int            `json:"line_no"`
	LineType   string         `json:"line_type"`
	SchemaRef  string         `json:"schema_ref,omitempty"`
	Payload    map[string]any `json:"payload,omitempty"`
	Amount     shared.Money   `json:"amount"`
}

type Link struct {
	ID               string         `json:"id"`
	DocumentID       string         `json:"document_id"`
	LinkedDocumentID string         `json:"linked_document_id"`
	LinkType         string         `json:"link_type"`
	Metadata         map[string]any `json:"metadata,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
}

type Attachment struct {
	ID             string    `json:"id"`
	DocumentID     string    `json:"document_id"`
	AttachmentType string    `json:"attachment_type"`
	FileName       string    `json:"file_name"`
	ContentType    string    `json:"content_type"`
	StorageKey     string    `json:"storage_key"`
	SizeBytes      int64     `json:"size_bytes"`
	CreatedAt      time.Time `json:"created_at"`
}

type Record struct {
	Header      Header       `json:"header"`
	Body        Body         `json:"body"`
	Lines       []Line       `json:"lines,omitempty"`
	Links       []Link       `json:"links,omitempty"`
	Attachments []Attachment `json:"attachments,omitempty"`
}

type ViewMode string

const (
	ViewNormal   ViewMode = "normal"
	ViewExpanded ViewMode = "expanded"
	ViewRaw      ViewMode = "raw"
)
