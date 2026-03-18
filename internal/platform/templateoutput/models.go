package templateoutput

import (
	"time"

	"orbyte/internal/platform/i18n"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/reporting"
)

type Definition struct {
	Key                 string             `json:"key"`
	Title               string             `json:"title"`
	TitleI18n           i18n.LocalizedText `json:"title_i18n,omitempty"`
	Description         string             `json:"description,omitempty"`
	DescriptionI18n     i18n.LocalizedText `json:"description_i18n,omitempty"`
	OwnerModuleKey      string             `json:"owner_module_key,omitempty"`
	TargetKind          string             `json:"target_kind"`
	TargetKey           string             `json:"target_key"`
	RendererKind        string             `json:"renderer_kind"`
	DefaultFormat       string             `json:"default_format,omitempty"`
	Formats             []string           `json:"formats,omitempty"`
	Purpose             string             `json:"purpose,omitempty"`
	Channel             string             `json:"channel,omitempty"`
	AllowedScopes       []string           `json:"allowed_scopes,omitempty"`
	RequiredPermissions []string           `json:"required_permissions,omitempty"`
	DefaultBody         string             `json:"default_body,omitempty"`
	DefaultStyle        string             `json:"default_style,omitempty"`
}

type Version struct {
	TemplateKey  string    `json:"template_key"`
	Version      int       `json:"version"`
	Status       string    `json:"status"`
	RendererKind string    `json:"renderer_kind"`
	Body         string    `json:"body"`
	Style        string    `json:"style,omitempty"`
	UpdatedAt    time.Time `json:"updated_at"`
	UpdatedBy    string    `json:"updated_by"`
	PublishedAt  time.Time `json:"published_at,omitempty"`
	PublishedBy  string    `json:"published_by,omitempty"`
}

type Binding struct {
	ID          string    `json:"id"`
	TemplateKey string    `json:"template_key"`
	ScopeType   string    `json:"scope_type"`
	ScopeID     string    `json:"scope_id,omitempty"`
	TargetKind  string    `json:"target_kind"`
	TargetKey   string    `json:"target_key"`
	Purpose     string    `json:"purpose,omitempty"`
	Channel     string    `json:"channel,omitempty"`
	IsDefault   bool      `json:"is_default,omitempty"`
	IsOfficial  bool      `json:"is_official,omitempty"`
	UpdatedAt   time.Time `json:"updated_at"`
	UpdatedBy   string    `json:"updated_by"`
}

type RenderRequest struct {
	TemplateKey    string                 `json:"template_key,omitempty"`
	RendererKind   string                 `json:"renderer_kind,omitempty"`
	Body           string                 `json:"body,omitempty"`
	Style          string                 `json:"style,omitempty"`
	TargetKind     string                 `json:"target_kind"`
	TargetKey      string                 `json:"target_key,omitempty"`
	TargetID       string                 `json:"target_id,omitempty"`
	Sample         bool                   `json:"sample,omitempty"`
	OrganizationID string                 `json:"organization_id,omitempty"`
	LocationID     string                 `json:"location_id,omitempty"`
	ScopeType      string                 `json:"scope_type,omitempty"`
	ScopeID        string                 `json:"scope_id,omitempty"`
	Purpose        string                 `json:"purpose,omitempty"`
	Channel        string                 `json:"channel,omitempty"`
	Format         string                 `json:"format,omitempty"`
	Draft          bool                   `json:"draft,omitempty"`
	Query          model.Query            `json:"query,omitempty"`
	ReportView     reporting.QueryRequest `json:"report_view,omitempty"`
}

type RenderedOutput struct {
	TemplateKey string    `json:"template_key"`
	Version     int       `json:"version"`
	Format      string    `json:"format"`
	ContentType string    `json:"content_type"`
	FileName    string    `json:"file_name"`
	HTML        string    `json:"html,omitempty"`
	Bytes       []byte    `json:"-"`
	GeneratedAt time.Time `json:"generated_at"`
	Official    bool      `json:"official"`
}

type VisualTemplate struct {
	SchemaVersion string          `json:"schema_version,omitempty"`
	Title         string          `json:"title,omitempty"`
	Settings      VisualSettings  `json:"settings,omitempty"`
	Sections      []VisualSection `json:"sections,omitempty"`
}

type VisualSettings struct {
	PaperPreset string `json:"paper_preset,omitempty"`
	Orientation string `json:"orientation,omitempty"`
	Density     string `json:"density,omitempty"`
	Margins     string `json:"margins,omitempty"`
	ShowGrid    bool   `json:"show_grid,omitempty"`
}

type VisualSection struct {
	ID    string      `json:"id,omitempty"`
	Title string      `json:"title,omitempty"`
	Kind  string      `json:"kind,omitempty"`
	Rows  []VisualRow `json:"rows,omitempty"`
}

type VisualRow struct {
	ID      string       `json:"id,omitempty"`
	Columns []VisualCell `json:"columns,omitempty"`
}

type VisualCell struct {
	ID     string        `json:"id,omitempty"`
	Span   int           `json:"span,omitempty"`
	Blocks []VisualBlock `json:"blocks,omitempty"`
}

type VisualBlock struct {
	ID        string         `json:"id,omitempty"`
	Type      string         `json:"type"`
	Label     string         `json:"label,omitempty"`
	Text      string         `json:"text,omitempty"`
	Path      string         `json:"path,omitempty"`
	RowsPath  string         `json:"rows_path,omitempty"`
	Columns   []VisualColumn `json:"columns,omitempty"`
	Align     string         `json:"align,omitempty"`
	FontSize  string         `json:"font_size,omitempty"`
	Emphasis  string         `json:"emphasis,omitempty"`
	VisibleIf string         `json:"visible_if,omitempty"`
	Value     string         `json:"value,omitempty"`
	ImageURL  string         `json:"image_url,omitempty"`
	Alt       string         `json:"alt,omitempty"`
	Format    string         `json:"format,omitempty"`
}

type VisualColumn struct {
	Label string `json:"label"`
	Path  string `json:"path"`
}
