package module

import (
	"time"

	"orbyte/internal/platform/config"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/search"
)

type Manifest struct {
	Key                    string                   `json:"key"`
	Name                   string                   `json:"name"`
	Version                string                   `json:"version"`
	DomainFamily           string                   `json:"domain_family"`
	Category               string                   `json:"category,omitempty"`
	Dependencies           []string                 `json:"dependencies,omitempty"`
	DependencyRequirements []DependencyRequirement  `json:"dependency_requirements,omitempty"`
	OwnedDocumentTypes     []string                 `json:"owned_document_types,omitempty"`
	OwnedEntityTypes       []string                 `json:"owned_entity_types,omitempty"`
	DocumentExtensions     []DocumentExtension      `json:"document_extensions,omitempty"`
	OwnedWorkflowKeys      []string                 `json:"owned_workflow_keys,omitempty"`
	OwnedPermissionKeys    []string                 `json:"owned_permission_keys,omitempty"`
	OwnedProjectionKeys    []string                 `json:"owned_projection_keys,omitempty"`
	OwnedTemplateKeys      []string                 `json:"owned_template_keys,omitempty"`
	FeatureFlags           []string                 `json:"feature_flags,omitempty"`
	ConfigDefinitions      []config.Definition      `json:"config_definitions,omitempty"`
	Models                 []model.Definition       `json:"models,omitempty"`
	Datasets               []DatasetDefinition      `json:"datasets,omitempty"`
	SearchIndexes          []search.IndexDefinition `json:"search_indexes,omitempty"`
	Security               SecurityDefinition       `json:"security,omitempty"`
	Observability          ObservabilityDefinition  `json:"observability,omitempty"`
	Frontend               FrontendDefinition       `json:"frontend,omitempty"`
	Bundles                []BundleDefinition       `json:"-"`
}

type DocumentExtension struct {
	DocumentType       string `json:"document_type"`
	SchemaVersion      string `json:"schema_version"`
	DisplayName        string `json:"display_name"`
	ReadPermissionKey  string `json:"read_permission_key,omitempty"`
	WritePermissionKey string `json:"write_permission_key,omitempty"`
}

type InstalledModule struct {
	Key       string    `json:"key"`
	Enabled   bool      `json:"enabled"`
	UpdatedAt time.Time `json:"updated_at"`
	UpdatedBy string    `json:"updated_by"`
}

type Detail struct {
	Manifest              Manifest               `json:"manifest"`
	Installed             InstalledModule        `json:"installed"`
	DependencyState       map[string]bool        `json:"dependency_state"`
	DependencyDiagnostics []DependencyDiagnostic `json:"dependency_diagnostics,omitempty"`
	LifecycleState        string                 `json:"lifecycle_state,omitempty"`
}

type RoleTemplateAssignment struct {
	ModuleKey     string                 `json:"module_key"`
	Template      RoleTemplateDefinition `json:"template"`
	RoleID        string                 `json:"role_id"`
	Applied       bool                   `json:"applied"`
	PermissionIDs []string               `json:"permission_ids,omitempty"`
}

type DependencyKind string

const (
	DependencyKindRequired    DependencyKind = "required"
	DependencyKindOptional    DependencyKind = "optional"
	DependencyKindUIExtension DependencyKind = "ui_extension"
	DependencyKindIntegration DependencyKind = "integration"
)

type DependencyRequirement struct {
	ModuleKey    string         `json:"module_key"`
	VersionRange string         `json:"version_range,omitempty"`
	Kind         DependencyKind `json:"kind,omitempty"`
}

type DependencyDiagnostic struct {
	ModuleKey         string         `json:"module_key"`
	VersionRange      string         `json:"version_range,omitempty"`
	Kind              DependencyKind `json:"kind,omitempty"`
	Enabled           bool           `json:"enabled"`
	Compatible        bool           `json:"compatible"`
	DependencyVersion string         `json:"dependency_version,omitempty"`
	Reason            string         `json:"reason,omitempty"`
}

type SecurityDefinition struct {
	Permissions   []PermissionDefinition   `json:"permissions,omitempty"`
	RoleTemplates []RoleTemplateDefinition `json:"role_templates,omitempty"`
	PolicyHooks   []PolicyHookDefinition   `json:"policy_hooks,omitempty"`
}

type PermissionDefinition struct {
	Key         string `json:"key"`
	Action      string `json:"action"`
	Resource    string `json:"resource"`
	DisplayName string `json:"display_name,omitempty"`
	Description string `json:"description,omitempty"`
	RiskLevel   string `json:"risk_level,omitempty"`
}

type RoleTemplateDefinition struct {
	Key            string   `json:"key"`
	Name           string   `json:"name"`
	Description    string   `json:"description,omitempty"`
	AllowedScopes  []string `json:"allowed_scopes,omitempty"`
	PermissionKeys []string `json:"permission_keys,omitempty"`
}

type PolicyHookDefinition struct {
	Key               string `json:"key"`
	Kind              string `json:"kind"`
	Target            string `json:"target"`
	InputContractKey  string `json:"input_contract_key,omitempty"`
	OutputContractKey string `json:"output_contract_key,omitempty"`
	Description       string `json:"description,omitempty"`
}

type ObservabilityDefinition struct {
	Projections  []ProjectionDefinition  `json:"projections,omitempty"`
	Dashboards   []DashboardDefinition   `json:"dashboards,omitempty"`
	Reports      []ReportDefinition      `json:"reports,omitempty"`
	Metrics      []MetricDefinition      `json:"metrics,omitempty"`
	LogEvents    []LogEventDefinition    `json:"log_events,omitempty"`
	DomainEvents []DomainEventDefinition `json:"domain_events,omitempty"`
}

type ProjectionDefinition struct {
	Key              string   `json:"key"`
	SourceEventTypes []string `json:"source_event_types,omitempty"`
	RefreshMode      string   `json:"refresh_mode,omitempty"`
	VisibilityPolicy string   `json:"visibility_policy,omitempty"`
}

type DashboardDefinition struct {
	Key                 string   `json:"key"`
	Title               string   `json:"title"`
	ViewKey             string   `json:"view_key,omitempty"`
	RequiredPermissions []string `json:"required_permissions,omitempty"`
}

type ReportDefinition struct {
	Key                 string   `json:"key"`
	Title               string   `json:"title"`
	Dataset             string   `json:"dataset,omitempty"`
	Formats             []string `json:"formats,omitempty"`
	RequiredPermissions []string `json:"required_permissions,omitempty"`
}

type DatasetDefinition struct {
	Key        string             `json:"key"`
	Title      string             `json:"title"`
	SourceKind string             `json:"source_kind"`
	ModelKey   string             `json:"model_key,omitempty"`
	Dimensions []DatasetDimension `json:"dimensions,omitempty"`
	Measures   []DatasetMeasure   `json:"measures,omitempty"`
}

type DatasetDimension struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Path  string `json:"path"`
}

type DatasetMeasure struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Kind  string `json:"kind"`
	Path  string `json:"path,omitempty"`
}

type MetricDefinition struct {
	Key         string   `json:"key"`
	Type        string   `json:"type"`
	Labels      []string `json:"labels,omitempty"`
	Description string   `json:"description,omitempty"`
}

type LogEventDefinition struct {
	Key            string   `json:"key"`
	Category       string   `json:"category,omitempty"`
	Severity       string   `json:"severity,omitempty"`
	RequiredFields []string `json:"required_fields,omitempty"`
}

type DomainEventDefinition struct {
	Type                string `json:"type"`
	Role                string `json:"role,omitempty"`
	CorrelationRequired bool   `json:"correlation_required,omitempty"`
	ExternalPublish     bool   `json:"external_publish,omitempty"`
	Topic               string `json:"topic,omitempty"`
	SchemaVersion       string `json:"schema_version,omitempty"`
}

type FrontendDefinition struct {
	Menus         []MenuDefinition        `json:"menus,omitempty"`
	Actions       []ActionDefinition      `json:"actions,omitempty"`
	Views         []ViewDefinition        `json:"views,omitempty"`
	CustomEntries []CustomEntryDefinition `json:"custom_entries,omitempty"`
}

type MenuDefinition struct {
	Key                 string   `json:"key"`
	Label               string   `json:"label"`
	ParentKey           string   `json:"parent_key,omitempty"`
	Icon                string   `json:"icon,omitempty"`
	ActionKey           string   `json:"action_key"`
	Order               int      `json:"order,omitempty"`
	RequiredPermissions []string `json:"required_permissions,omitempty"`
}

type ActionRenderMode string

const (
	RenderModeGeneric ActionRenderMode = "generic"
	RenderModeCustom  ActionRenderMode = "custom"
)

type ActionDefinition struct {
	Key                 string           `json:"key"`
	Label               string           `json:"label"`
	Kind                string           `json:"kind"`
	RoutePath           string           `json:"route_path"`
	ViewKey             string           `json:"view_key,omitempty"`
	CustomEntryKey      string           `json:"custom_entry_key,omitempty"`
	RenderMode          ActionRenderMode `json:"render_mode"`
	RequiredPermissions []string         `json:"required_permissions,omitempty"`
	ContextDefaults     map[string]any   `json:"context_defaults,omitempty"`
}

type ViewDefinition struct {
	Key                 string                      `json:"key"`
	Title               string                      `json:"title"`
	Kind                string                      `json:"kind"`
	DocumentType        string                      `json:"document_type,omitempty"`
	ModelKey            string                      `json:"model_key,omitempty"`
	ProjectionKey       string                      `json:"projection_key,omitempty"`
	DatasetKey          string                      `json:"dataset_key,omitempty"`
	RequiredPermissions []string                    `json:"required_permissions,omitempty"`
	AllowedActions      []string                    `json:"allowed_actions,omitempty"`
	Columns             []ColumnDefinition          `json:"columns,omitempty"`
	Filters             []FilterDefinition          `json:"filters,omitempty"`
	Sections            []SectionDefinition         `json:"sections,omitempty"`
	Tabs                []TabDefinition             `json:"tabs,omitempty"`
	Fields              []FieldDefinition           `json:"fields,omitempty"`
	Cards               []CardDefinition            `json:"cards,omitempty"`
	RelatedViews        []RelatedViewDefinition     `json:"related_views,omitempty"`
	ActionPlacements    []ActionPlacementDefinition `json:"action_placements,omitempty"`
	EmptyState          string                      `json:"empty_state,omitempty"`
	DefaultPageSize     int                         `json:"default_page_size,omitempty"`
}

type ColumnDefinition struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Path  string `json:"path"`
	Width string `json:"width,omitempty"`
}

type FilterDefinition struct {
	Key     string   `json:"key"`
	Label   string   `json:"label"`
	Type    string   `json:"type"`
	Options []string `json:"options,omitempty"`
}

type SectionDefinition struct {
	Key              string            `json:"key"`
	Title            string            `json:"title"`
	Fields           []FieldDefinition `json:"fields,omitempty"`
	ExtensionSlotKey string            `json:"extension_slot_key,omitempty"`
}

type TabDefinition struct {
	Key      string              `json:"key"`
	Title    string              `json:"title"`
	Sections []SectionDefinition `json:"sections,omitempty"`
}

type FieldDefinition struct {
	Key                string   `json:"key"`
	Label              string   `json:"label"`
	Path               string   `json:"path"`
	Type               string   `json:"type"`
	Widget             string   `json:"widget,omitempty"`
	Placeholder        string   `json:"placeholder,omitempty"`
	HelpText           string   `json:"help_text,omitempty"`
	Options            []string `json:"options,omitempty"`
	ReadOnly           bool     `json:"read_only,omitempty"`
	ExtensionModuleKey string   `json:"extension_module_key,omitempty"`
}

type CardDefinition struct {
	Key       string `json:"key"`
	Label     string `json:"label"`
	Path      string `json:"path"`
	Widget    string `json:"widget,omitempty"`
	ActionKey string `json:"action_key,omitempty"`
}

type RelatedViewDefinition struct {
	Key        string `json:"key"`
	Title      string `json:"title"`
	Source     string `json:"source"`
	EmptyState string `json:"empty_state,omitempty"`
}

type ActionPlacementDefinition struct {
	ActionKey string `json:"action_key"`
	Zone      string `json:"zone"`
	Style     string `json:"style,omitempty"`
}

type CustomEntryDefinition struct {
	Key                 string   `json:"key"`
	Title               string   `json:"title"`
	RoutePath           string   `json:"route_path"`
	BundleKey           string   `json:"bundle_key"`
	ComponentExport     string   `json:"component_export"`
	RequiredPermissions []string `json:"required_permissions,omitempty"`
}

type BundleDefinition struct {
	Key    string `json:"key"`
	Script string `json:"-"`
}

type RouteResolution struct {
	Path        string                 `json:"path"`
	ModuleKey   string                 `json:"module_key"`
	RenderMode  ActionRenderMode       `json:"render_mode"`
	Action      ActionDefinition       `json:"action"`
	View        *ViewDefinition        `json:"view,omitempty"`
	CustomEntry *CustomEntryDefinition `json:"custom_entry,omitempty"`
}
