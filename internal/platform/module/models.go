package module

import (
	"time"

	"orbyte/internal/platform/config"
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/i18n"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/reference"
	"orbyte/internal/platform/search"
	"orbyte/internal/platform/workflow"
)

type Manifest struct {
	Key                    string                     `json:"key"`
	Name                   string                     `json:"name"`
	NameI18n               i18n.LocalizedText         `json:"name_i18n,omitempty"`
	Version                string                     `json:"version"`
	Role                   ModuleRole                 `json:"role,omitempty"`
	LocalExtension         LocalExtensionDefinition   `json:"local_extension,omitempty"`
	KernelVersionRange     string                     `json:"kernel_version_range,omitempty"`
	RequiredCapabilities   []string                   `json:"required_capabilities,omitempty"`
	DomainFamily           string                     `json:"domain_family"`
	Category               string                     `json:"category,omitempty"`
	Dependencies           []string                   `json:"dependencies,omitempty"`
	DependencyRequirements []DependencyRequirement    `json:"dependency_requirements,omitempty"`
	OwnedDocumentTypes     []string                   `json:"owned_document_types,omitempty"`
	OwnedEntityTypes       []string                   `json:"owned_entity_types,omitempty"`
	DocumentExtensions     []DocumentExtension        `json:"document_extensions,omitempty"`
	OwnedWorkflowKeys      []string                   `json:"owned_workflow_keys,omitempty"`
	OwnedPermissionKeys    []string                   `json:"owned_permission_keys,omitempty"`
	OwnedProjectionKeys    []string                   `json:"owned_projection_keys,omitempty"`
	OwnedTemplateKeys      []string                   `json:"owned_template_keys,omitempty"`
	FeatureFlags           []string                   `json:"feature_flags,omitempty"`
	ConfigDefinitions      []config.Definition        `json:"config_definitions,omitempty"`
	ReferenceTypes         []reference.TypeDefinition `json:"reference_types,omitempty"`
	ReferenceRecords       []reference.Record         `json:"reference_records,omitempty"`
	Models                 []model.Definition         `json:"models,omitempty"`
	Documents              []document.Definition      `json:"documents,omitempty"`
	Workflows              []workflow.Definition      `json:"workflows,omitempty"`
	Datasets               []DatasetDefinition        `json:"datasets,omitempty"`
	SearchIndexes          []search.IndexDefinition   `json:"search_indexes,omitempty"`
	Security               SecurityDefinition         `json:"security,omitempty"`
	Observability          ObservabilityDefinition    `json:"observability,omitempty"`
	Frontend               FrontendDefinition         `json:"frontend,omitempty"`
	SelfService            SelfServiceDefinition      `json:"self_service,omitempty"`
	Offline                OfflineDefinition          `json:"offline,omitempty"`
	MCP                    MCPDefinition              `json:"mcp,omitempty"`
	Templates              []TemplateDefinition       `json:"templates,omitempty"`
	Bundles                []BundleDefinition         `json:"-"`
}

type ModuleRole string

const (
	ModuleRoleStandard       ModuleRole = "standard"
	ModuleRoleBase           ModuleRole = "base"
	ModuleRoleExtension      ModuleRole = "extension"
	ModuleRoleLocalExtension ModuleRole = "local_extension"
)

type LocalExtensionDefinition struct {
	BaseModuleKey string `json:"base_module_key,omitempty"`
	LocalityType  string `json:"locality_type,omitempty"`
	LocalityCode  string `json:"locality_code,omitempty"`
	LocalityLabel string `json:"locality_label,omitempty"`
}

type DocumentExtension struct {
	DocumentType       string             `json:"document_type"`
	SchemaVersion      string             `json:"schema_version"`
	DisplayName        string             `json:"display_name"`
	DisplayNameI18n    i18n.LocalizedText `json:"display_name_i18n,omitempty"`
	ReadPermissionKey  string             `json:"read_permission_key,omitempty"`
	WritePermissionKey string             `json:"write_permission_key,omitempty"`
}

type InstalledModule struct {
	Key       string    `json:"key"`
	Enabled   bool      `json:"enabled"`
	UpdatedAt time.Time `json:"updated_at"`
	UpdatedBy string    `json:"updated_by"`
}

type LocalExtensionActivation struct {
	BaseModuleKey      string    `json:"base_module_key"`
	ExtensionModuleKey string    `json:"extension_module_key"`
	Scope              string    `json:"scope"`
	ScopeID            string    `json:"scope_id,omitempty"`
	UpdatedAt          time.Time `json:"updated_at"`
	UpdatedBy          string    `json:"updated_by"`
}

type Detail struct {
	Manifest              Manifest               `json:"manifest"`
	Installed             InstalledModule        `json:"installed"`
	DependencyState       map[string]bool        `json:"dependency_state"`
	DependencyDiagnostics []DependencyDiagnostic `json:"dependency_diagnostics,omitempty"`
	KernelDiagnostics     []Diagnostic           `json:"kernel_diagnostics,omitempty"`
	LifecycleState        string                 `json:"lifecycle_state,omitempty"`
}

type ScopedDetail struct {
	Detail
	LocalExtensionState *LocalExtensionState `json:"local_extension_state,omitempty"`
}

type LocalExtensionState struct {
	Eligible             bool   `json:"eligible"`
	Active               bool   `json:"active"`
	BaseModuleKey        string `json:"base_module_key,omitempty"`
	LocalityType         string `json:"locality_type,omitempty"`
	LocalityCode         string `json:"locality_code,omitempty"`
	LocalityLabel        string `json:"locality_label,omitempty"`
	SourceScope          string `json:"source_scope,omitempty"`
	SourceScopeID        string `json:"source_scope_id,omitempty"`
	ActivatedModuleKey   string `json:"activated_module_key,omitempty"`
	ActivationConsistent bool   `json:"activation_consistent,omitempty"`
}

type DiagnosticSeverity string

const (
	SeverityError   DiagnosticSeverity = "error"
	SeverityWarning DiagnosticSeverity = "warning"
)

type Diagnostic struct {
	Severity  DiagnosticSeverity `json:"severity"`
	Code      string             `json:"code"`
	Message   string             `json:"message"`
	ModuleKey string             `json:"module_key,omitempty"`
	Path      string             `json:"path,omitempty"`
}

type LintReport struct {
	Diagnostics []Diagnostic `json:"diagnostics,omitempty"`
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
	Key             string             `json:"key"`
	Action          string             `json:"action"`
	Resource        string             `json:"resource"`
	DisplayName     string             `json:"display_name,omitempty"`
	DisplayNameI18n i18n.LocalizedText `json:"display_name_i18n,omitempty"`
	Description     string             `json:"description,omitempty"`
	DescriptionI18n i18n.LocalizedText `json:"description_i18n,omitempty"`
	RiskLevel       string             `json:"risk_level,omitempty"`
}

type RoleTemplateDefinition struct {
	Key             string             `json:"key"`
	Name            string             `json:"name"`
	NameI18n        i18n.LocalizedText `json:"name_i18n,omitempty"`
	Description     string             `json:"description,omitempty"`
	DescriptionI18n i18n.LocalizedText `json:"description_i18n,omitempty"`
	AllowedScopes   []string           `json:"allowed_scopes,omitempty"`
	PermissionKeys  []string           `json:"permission_keys,omitempty"`
}

type PolicyHookDefinition struct {
	Key               string             `json:"key"`
	Kind              string             `json:"kind"`
	Target            string             `json:"target"`
	InputContractKey  string             `json:"input_contract_key,omitempty"`
	OutputContractKey string             `json:"output_contract_key,omitempty"`
	Description       string             `json:"description,omitempty"`
	DescriptionI18n   i18n.LocalizedText `json:"description_i18n,omitempty"`
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
	Key                 string             `json:"key"`
	Title               string             `json:"title"`
	TitleI18n           i18n.LocalizedText `json:"title_i18n,omitempty"`
	ViewKey             string             `json:"view_key,omitempty"`
	RequiredPermissions []string           `json:"required_permissions,omitempty"`
}

type ReportDefinition struct {
	Key                 string             `json:"key"`
	Title               string             `json:"title"`
	TitleI18n           i18n.LocalizedText `json:"title_i18n,omitempty"`
	Dataset             string             `json:"dataset,omitempty"`
	Formats             []string           `json:"formats,omitempty"`
	RequiredPermissions []string           `json:"required_permissions,omitempty"`
}

type DatasetDefinition struct {
	Key        string             `json:"key"`
	Title      string             `json:"title"`
	TitleI18n  i18n.LocalizedText `json:"title_i18n,omitempty"`
	SourceKind string             `json:"source_kind"`
	ModelKey   string             `json:"model_key,omitempty"`
	Dimensions []DatasetDimension `json:"dimensions,omitempty"`
	Measures   []DatasetMeasure   `json:"measures,omitempty"`
}

type DatasetDimension struct {
	Key       string             `json:"key"`
	Label     string             `json:"label"`
	LabelI18n i18n.LocalizedText `json:"label_i18n,omitempty"`
	Path      string             `json:"path"`
}

type DatasetMeasure struct {
	Key       string             `json:"key"`
	Label     string             `json:"label"`
	LabelI18n i18n.LocalizedText `json:"label_i18n,omitempty"`
	Kind      string             `json:"kind"`
	Path      string             `json:"path,omitempty"`
}

type MetricDefinition struct {
	Key             string             `json:"key"`
	Type            string             `json:"type"`
	Labels          []string           `json:"labels,omitempty"`
	Description     string             `json:"description,omitempty"`
	DescriptionI18n i18n.LocalizedText `json:"description_i18n,omitempty"`
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
	Menus         []MenuDefinition         `json:"menus,omitempty"`
	Actions       []ActionDefinition       `json:"actions,omitempty"`
	Views         []ViewDefinition         `json:"views,omitempty"`
	CustomEntries []CustomEntryDefinition  `json:"custom_entries,omitempty"`
	DocumentFlows []DocumentFlowDefinition `json:"document_flows,omitempty"`
}

type UISurface string

const (
	UISurfaceUser        UISurface = "user"
	UISurfaceAdmin       UISurface = "admin"
	UISurfaceBoth        UISurface = "both"
	UISurfaceBackoffice  UISurface = "backoffice"
	UISurfaceWorklist    UISurface = "worklist"
	UISurfaceSelfService UISurface = "self_service"
	UISurfacePOS         UISurface = "pos"
	UISurfaceMobile      UISurface = "mobile"
)

type SelfServiceDefinition struct {
	APIs []SelfServiceAPIDefinition `json:"apis,omitempty"`
}

type SelfServiceAPIDefinition struct {
	Key                 string             `json:"key"`
	Title               string             `json:"title"`
	TitleI18n           i18n.LocalizedText `json:"title_i18n,omitempty"`
	Description         string             `json:"description,omitempty"`
	DescriptionI18n     i18n.LocalizedText `json:"description_i18n,omitempty"`
	Method              string             `json:"method"`
	RoutePath           string             `json:"route_path"`
	HandlerKind         string             `json:"handler_kind"`
	DocumentType        string             `json:"document_type,omitempty"`
	ModelKey            string             `json:"model_key,omitempty"`
	FlowKey             string             `json:"flow_key,omitempty"`
	RequiredPermissions []string           `json:"required_permissions,omitempty"`
	AudienceKinds       []string           `json:"audience_kinds,omitempty"`
	RequestContractKey  string             `json:"request_contract_key,omitempty"`
	ResponseContractKey string             `json:"response_contract_key,omitempty"`
	Idempotent          bool               `json:"idempotent,omitempty"`
	OfflineCapable      bool               `json:"offline_capable,omitempty"`
}

type OfflineDefinition struct {
	References  []OfflineReferenceDefinition  `json:"references,omitempty"`
	Projections []OfflineProjectionDefinition `json:"projections,omitempty"`
	Documents   []OfflineDocumentDefinition   `json:"documents,omitempty"`
	Models      []OfflineModelDefinition      `json:"models,omitempty"`
}

type OfflineReferenceDefinition struct {
	TypeKey             string             `json:"type_key"`
	Title               string             `json:"title"`
	TitleI18n           i18n.LocalizedText `json:"title_i18n,omitempty"`
	RequiredPermissions []string           `json:"required_permissions,omitempty"`
}

type OfflineProjectionDefinition struct {
	IndexKey             string             `json:"index_key"`
	Title                string             `json:"title"`
	TitleI18n            i18n.LocalizedText `json:"title_i18n,omitempty"`
	RequiredPermissions  []string           `json:"required_permissions,omitempty"`
	DefaultFilters       []string           `json:"default_filters,omitempty"`
	DefaultIncludeFields []string           `json:"default_include_fields,omitempty"`
}

type OfflineDocumentDefinition struct {
	Type                string             `json:"type"`
	Title               string             `json:"title"`
	TitleI18n           i18n.LocalizedText `json:"title_i18n,omitempty"`
	CreatePermissionKey string             `json:"create_permission_key,omitempty"`
	UpdatePermissionKey string             `json:"update_permission_key,omitempty"`
	RequiredPermissions []string           `json:"required_permissions,omitempty"`
}

type OfflineModelDefinition struct {
	ModelKey            string             `json:"model_key"`
	Title               string             `json:"title"`
	TitleI18n           i18n.LocalizedText `json:"title_i18n,omitempty"`
	CreatePermissionKey string             `json:"create_permission_key,omitempty"`
	UpdatePermissionKey string             `json:"update_permission_key,omitempty"`
	RequiredPermissions []string           `json:"required_permissions,omitempty"`
}

type MCPDefinition struct {
	Tools     []MCPToolDefinition     `json:"tools,omitempty"`
	Resources []MCPResourceDefinition `json:"resources,omitempty"`
	Apps      []MCPAppDefinition      `json:"apps,omitempty"`
}

type MCPContractMetadata struct {
	Version         string   `json:"version,omitempty"`
	Stability       string   `json:"stability,omitempty"`
	SideEffectClass string   `json:"side_effect_class,omitempty"`
	Idempotency     string   `json:"idempotency,omitempty"`
	AuditAction     string   `json:"audit_action,omitempty"`
	RequiredScopes  []string `json:"required_scopes,omitempty"`
	Deprecated      bool     `json:"deprecated,omitempty"`
	DeprecationNote string   `json:"deprecation_note,omitempty"`
}

type TemplateDefinition struct {
	Key                 string             `json:"key"`
	Title               string             `json:"title"`
	TitleI18n           i18n.LocalizedText `json:"title_i18n,omitempty"`
	Description         string             `json:"description,omitempty"`
	DescriptionI18n     i18n.LocalizedText `json:"description_i18n,omitempty"`
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

type MCPToolDefinition struct {
	Key                 string              `json:"key"`
	Title               string              `json:"title"`
	TitleI18n           i18n.LocalizedText  `json:"title_i18n,omitempty"`
	Description         string              `json:"description,omitempty"`
	DescriptionI18n     i18n.LocalizedText  `json:"description_i18n,omitempty"`
	Operation           string              `json:"operation"`
	RequiredPermissions []string            `json:"required_permissions,omitempty"`
	InputSchema         map[string]any      `json:"input_schema,omitempty"`
	OutputSchema        map[string]any      `json:"output_schema,omitempty"`
	AppKey              string              `json:"app_key,omitempty"`
	Contract            MCPContractMetadata `json:"contract,omitempty"`
}

type MCPResourceDefinition struct {
	Key                 string              `json:"key"`
	Title               string              `json:"title"`
	TitleI18n           i18n.LocalizedText  `json:"title_i18n,omitempty"`
	Description         string              `json:"description,omitempty"`
	DescriptionI18n     i18n.LocalizedText  `json:"description_i18n,omitempty"`
	URI                 string              `json:"uri"`
	MIMEType            string              `json:"mime_type,omitempty"`
	Provider            string              `json:"provider,omitempty"`
	RequiredPermissions []string            `json:"required_permissions,omitempty"`
	AppKey              string              `json:"app_key,omitempty"`
	Contract            MCPContractMetadata `json:"contract,omitempty"`
}

type MCPAppDefinition struct {
	Key                 string              `json:"key"`
	Title               string              `json:"title"`
	TitleI18n           i18n.LocalizedText  `json:"title_i18n,omitempty"`
	Description         string              `json:"description,omitempty"`
	DescriptionI18n     i18n.LocalizedText  `json:"description_i18n,omitempty"`
	ResourceKey         string              `json:"resource_key"`
	ViewKey             string              `json:"view_key,omitempty"`
	CustomEntryKey      string              `json:"custom_entry_key,omitempty"`
	RequiredPermissions []string            `json:"required_permissions,omitempty"`
	Contract            MCPContractMetadata `json:"contract,omitempty"`
}

type MenuDefinition struct {
	Key                 string             `json:"key"`
	Label               string             `json:"label"`
	LabelI18n           i18n.LocalizedText `json:"label_i18n,omitempty"`
	Surface             UISurface          `json:"surface,omitempty"`
	ParentKey           string             `json:"parent_key,omitempty"`
	Icon                string             `json:"icon,omitempty"`
	ActionKey           string             `json:"action_key"`
	Order               int                `json:"order,omitempty"`
	RequiredPermissions []string           `json:"required_permissions,omitempty"`
}

type ActionRenderMode string

const (
	RenderModeGeneric ActionRenderMode = "generic"
	RenderModeCustom  ActionRenderMode = "custom"
	RenderModeFlow    ActionRenderMode = "flow"
)

type ActionDefinition struct {
	Key                 string             `json:"key"`
	Label               string             `json:"label"`
	LabelI18n           i18n.LocalizedText `json:"label_i18n,omitempty"`
	Surface             UISurface          `json:"surface,omitempty"`
	Kind                string             `json:"kind"`
	RoutePath           string             `json:"route_path"`
	ViewKey             string             `json:"view_key,omitempty"`
	CustomEntryKey      string             `json:"custom_entry_key,omitempty"`
	FlowKey             string             `json:"flow_key,omitempty"`
	RenderMode          ActionRenderMode   `json:"render_mode"`
	RequiredPermissions []string           `json:"required_permissions,omitempty"`
	ContextDefaults     map[string]any     `json:"context_defaults,omitempty"`
}

type DocumentFlowDefinition struct {
	Key                 string                       `json:"key"`
	Title               string                       `json:"title"`
	TitleI18n           i18n.LocalizedText           `json:"title_i18n,omitempty"`
	Surface             UISurface                    `json:"surface,omitempty"`
	RoutePath           string                       `json:"route_path"`
	PrimaryDocumentType string                       `json:"primary_document_type"`
	RequiredPermissions []string                     `json:"required_permissions,omitempty"`
	Steps               []DocumentFlowStepDefinition `json:"steps,omitempty"`
}

type DocumentFlowStepDefinition struct {
	Key         string                           `json:"key"`
	Title       string                           `json:"title"`
	TitleI18n   i18n.LocalizedText               `json:"title_i18n,omitempty"`
	Documents   []DocumentFlowDocumentDefinition `json:"documents,omitempty"`
	NextRules   []DocumentFlowBranchRule         `json:"next_rules,omitempty"`
	NextStepKey string                           `json:"next_step_key,omitempty"`
}

type DocumentFlowDocumentDefinition struct {
	Key                 string              `json:"key"`
	Title               string              `json:"title"`
	TitleI18n           i18n.LocalizedText  `json:"title_i18n,omitempty"`
	DocumentType        string              `json:"document_type"`
	PrimaryOutput       bool                `json:"primary_output,omitempty"`
	LinkType            string              `json:"link_type,omitempty"`
	RequiredPermissions []string            `json:"required_permissions,omitempty"`
	Tabs                []TabDefinition     `json:"tabs,omitempty"`
	Sections            []SectionDefinition `json:"sections,omitempty"`
	Fields              []FieldDefinition   `json:"fields,omitempty"`
}

type DocumentFlowBranchRule struct {
	Path        string   `json:"path"`
	Equals      string   `json:"equals,omitempty"`
	In          []string `json:"in,omitempty"`
	Truthy      bool     `json:"truthy,omitempty"`
	NextStepKey string   `json:"next_step_key"`
}

type ViewDefinition struct {
	Key                 string                      `json:"key"`
	Title               string                      `json:"title"`
	TitleI18n           i18n.LocalizedText          `json:"title_i18n,omitempty"`
	Surface             UISurface                   `json:"surface,omitempty"`
	Kind                string                      `json:"kind"`
	DocumentType        string                      `json:"document_type,omitempty"`
	ModelKey            string                      `json:"model_key,omitempty"`
	ProjectionKey       string                      `json:"projection_key,omitempty"`
	DatasetKey          string                      `json:"dataset_key,omitempty"`
	Printable           bool                        `json:"printable,omitempty"`
	PrintPurpose        string                      `json:"print_purpose,omitempty"`
	PrintChannel        string                      `json:"print_channel,omitempty"`
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
	EmptyStateI18n      i18n.LocalizedText          `json:"empty_state_i18n,omitempty"`
	DefaultPageSize     int                         `json:"default_page_size,omitempty"`
}

type ColumnDefinition struct {
	Key       string             `json:"key"`
	Label     string             `json:"label"`
	LabelI18n i18n.LocalizedText `json:"label_i18n,omitempty"`
	Path      string             `json:"path"`
	Width     string             `json:"width,omitempty"`
}

type FilterDefinition struct {
	Key       string             `json:"key"`
	Label     string             `json:"label"`
	LabelI18n i18n.LocalizedText `json:"label_i18n,omitempty"`
	Type      string             `json:"type"`
	Options   []string           `json:"options,omitempty"`
}

type SectionDefinition struct {
	Key              string             `json:"key"`
	Title            string             `json:"title"`
	TitleI18n        i18n.LocalizedText `json:"title_i18n,omitempty"`
	Fields           []FieldDefinition  `json:"fields,omitempty"`
	ExtensionSlotKey string             `json:"extension_slot_key,omitempty"`
}

type TabDefinition struct {
	Key       string              `json:"key"`
	Title     string              `json:"title"`
	TitleI18n i18n.LocalizedText  `json:"title_i18n,omitempty"`
	Sections  []SectionDefinition `json:"sections,omitempty"`
}

type FieldDefinition struct {
	Key                string             `json:"key"`
	Label              string             `json:"label"`
	LabelI18n          i18n.LocalizedText `json:"label_i18n,omitempty"`
	Path               string             `json:"path"`
	Type               string             `json:"type"`
	Widget             string             `json:"widget,omitempty"`
	Placeholder        string             `json:"placeholder,omitempty"`
	PlaceholderI18n    i18n.LocalizedText `json:"placeholder_i18n,omitempty"`
	HelpText           string             `json:"help_text,omitempty"`
	HelpTextI18n       i18n.LocalizedText `json:"help_text_i18n,omitempty"`
	Options            []string           `json:"options,omitempty"`
	ReadOnly           bool               `json:"read_only,omitempty"`
	ExtensionModuleKey string             `json:"extension_module_key,omitempty"`
}

type CardDefinition struct {
	Key       string             `json:"key"`
	Label     string             `json:"label"`
	LabelI18n i18n.LocalizedText `json:"label_i18n,omitempty"`
	Path      string             `json:"path"`
	Widget    string             `json:"widget,omitempty"`
	ActionKey string             `json:"action_key,omitempty"`
}

type RelatedViewDefinition struct {
	Key            string             `json:"key"`
	Title          string             `json:"title"`
	TitleI18n      i18n.LocalizedText `json:"title_i18n,omitempty"`
	Source         string             `json:"source"`
	EmptyState     string             `json:"empty_state,omitempty"`
	EmptyStateI18n i18n.LocalizedText `json:"empty_state_i18n,omitempty"`
}

type ActionPlacementDefinition struct {
	ActionKey string `json:"action_key"`
	Zone      string `json:"zone"`
	Style     string `json:"style,omitempty"`
}

type CustomEntryDefinition struct {
	Key                 string             `json:"key"`
	Title               string             `json:"title"`
	TitleI18n           i18n.LocalizedText `json:"title_i18n,omitempty"`
	Surface             UISurface          `json:"surface,omitempty"`
	RoutePath           string             `json:"route_path"`
	BundleKey           string             `json:"bundle_key"`
	ComponentExport     string             `json:"component_export"`
	Printable           bool               `json:"printable,omitempty"`
	PrintTargetKind     string             `json:"print_target_kind,omitempty"`
	PrintTargetKey      string             `json:"print_target_key,omitempty"`
	PrintPurpose        string             `json:"print_purpose,omitempty"`
	PrintChannel        string             `json:"print_channel,omitempty"`
	RequiredPermissions []string           `json:"required_permissions,omitempty"`
}

type BundleDefinition struct {
	Key    string `json:"key"`
	Script string `json:"-"`
}

type RouteResolution struct {
	Path        string                  `json:"path"`
	ModuleKey   string                  `json:"module_key"`
	RenderMode  ActionRenderMode        `json:"render_mode"`
	Action      ActionDefinition        `json:"action"`
	View        *ViewDefinition         `json:"view,omitempty"`
	CustomEntry *CustomEntryDefinition  `json:"custom_entry,omitempty"`
	Flow        *DocumentFlowDefinition `json:"flow,omitempty"`
}
