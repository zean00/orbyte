package httpx

import (
	"net/http"
	"time"

	"orbyte/internal/platform/acp"
	"orbyte/internal/platform/analytics"
	"orbyte/internal/platform/audit"
	"orbyte/internal/platform/config"
	"orbyte/internal/platform/featureflags"
	"orbyte/internal/platform/idempotency"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/integration"
	"orbyte/internal/platform/mcp"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/observability"
	"orbyte/internal/platform/organization"
	"orbyte/internal/platform/policy"
	"orbyte/internal/platform/reference"
	"orbyte/internal/platform/runtimehealth"
	"orbyte/internal/platform/workflow"
)

type configUpdateRequest struct {
	Scope   string         `json:"scope"`
	ScopeID string         `json:"scope_id"`
	Value   map[string]any `json:"value"`
}

type regoUpdateRequest struct {
	Scope   string `json:"scope"`
	ScopeID string `json:"scope_id"`
	Source  string `json:"source"`
}

type reportingLineRequest struct {
	SubjectUserID    string `json:"subject_user_id"`
	ManagerUserID    string `json:"manager_user_id"`
	RelationshipType string `json:"relationship_type"`
	OrganizationID   string `json:"organization_id"`
	LocationID       string `json:"location_id"`
	OperatingUnitID  string `json:"operating_unit_id"`
	Status           string `json:"status"`
	Priority         int    `json:"priority"`
	EffectiveFrom    string `json:"effective_from"`
	EffectiveTo      string `json:"effective_to"`
}

type authSettingsResponse struct {
	Definition config.Definition     `json:"definition"`
	Entry      config.EffectiveValue `json:"entry"`
}

type adminHierarchyNode struct {
	ID                string `json:"id"`
	Username          string `json:"username"`
	Status            string `json:"status"`
	DefaultLocationID string `json:"default_location_id,omitempty"`
}

type adminHierarchyEdge struct {
	ID               string    `json:"id"`
	SubjectUserID    string    `json:"subject_user_id"`
	ManagerUserID    string    `json:"manager_user_id"`
	RelationshipType string    `json:"relationship_type"`
	Status           string    `json:"status"`
	OrganizationID   string    `json:"organization_id,omitempty"`
	LocationID       string    `json:"location_id,omitempty"`
	OperatingUnitID  string    `json:"operating_unit_id,omitempty"`
	Priority         int       `json:"priority,omitempty"`
	EffectiveFrom    time.Time `json:"effective_from"`
	EffectiveTo      time.Time `json:"effective_to,omitempty"`
}

type adminHierarchySummary struct {
	TotalUsers      int `json:"total_users"`
	ActiveLines     int `json:"active_lines"`
	OrphanUsers     int `json:"orphan_users"`
	ActingOverrides int `json:"acting_overrides"`
}

type adminModuleDependencyGraph struct {
	Nodes   []adminModuleDependencyNode  `json:"nodes"`
	Edges   []adminModuleDependencyEdge  `json:"edges"`
	Summary adminModuleDependencySummary `json:"summary"`
}

type adminModuleDependencyNode struct {
	ModuleKey      string `json:"module_key"`
	Name           string `json:"name"`
	Version        string `json:"version"`
	Enabled        bool   `json:"enabled"`
	LifecycleState string `json:"lifecycle_state"`
	Role           string `json:"role"`
	DomainFamily   string `json:"domain_family,omitempty"`
	Category       string `json:"category,omitempty"`
	Status         string `json:"status"`
	ConsolePath    string `json:"console_path"`
}

type adminModuleDependencyEdge struct {
	SourceModuleKey string `json:"source_module_key"`
	TargetModuleKey string `json:"target_module_key"`
	Kind            string `json:"kind"`
	VersionRange    string `json:"version_range,omitempty"`
	Status          string `json:"status"`
	Enabled         bool   `json:"enabled"`
	Compatible      bool   `json:"compatible"`
	Reason          string `json:"reason,omitempty"`
}

type adminModuleDependencySummary struct {
	TotalModules     int `json:"total_modules"`
	EnabledModules   int `json:"enabled_modules"`
	UnhealthyModules int `json:"unhealthy_modules"`
	TotalEdges       int `json:"total_edges"`
}

func registerAdminRoutes(mux *http.ServeMux, cfg *config.Service, flags *featureflags.Service, org *organization.Service, ident *identity.Service, analyticsSvc *analytics.Service, modules *module.Service, workflowSvc *workflow.Service, auditSvc *audit.Service, policySvc *policy.Service, obsSvc *observability.Service, integrationSvc *integration.Service, referenceSvc *reference.Service, idempotencySvc *idempotency.Service, health *runtimehealth.Tracker, acpSvc *acp.Service, mcpServer *mcp.Server) {
	registerAdminShellRoutes(mux, ident)
	registerAdminCoreRoutes(mux, cfg, org, ident, analyticsSvc, modules, workflowSvc, auditSvc, policySvc, obsSvc, acpSvc, mcpServer)
	registerAdminAuditRoutes(mux, ident, auditSvc)
	registerAdminIntegrationRoutes(mux, ident, auditSvc, integrationSvc, idempotencySvc)
	registerAdminConfigRoutes(mux, cfg, flags, org, ident, modules, auditSvc, policySvc, integrationSvc, referenceSvc, idempotencySvc, health)
}
