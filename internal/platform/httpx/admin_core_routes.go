package httpx

import (
	"net/http"

	"orbyte/internal/platform/acp"
	"orbyte/internal/platform/analytics"
	"orbyte/internal/platform/audit"
	"orbyte/internal/platform/config"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/mcp"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/observability"
	"orbyte/internal/platform/organization"
	"orbyte/internal/platform/policy"
	"orbyte/internal/platform/workflow"
)

func registerAdminCoreRoutes(mux *http.ServeMux, cfg *config.Service, org *organization.Service, ident *identity.Service, analyticsSvc *analytics.Service, modules *module.Service, workflowSvc *workflow.Service, auditSvc *audit.Service, policySvc *policy.Service, obsSvc *observability.Service, acpSvc *acp.Service, mcpServer *mcp.Server) {
	registerAdminOverviewRoutes(mux, cfg, org, ident, analyticsSvc, modules, workflowSvc, auditSvc, policySvc, acpSvc, mcpServer)
	registerAdminHierarchyRoutes(mux, ident)
	registerAdminWorkflowRoutes(mux, ident, workflowSvc, auditSvc, policySvc, obsSvc)
	registerAdminDashboardRoutes(mux, org, ident, analyticsSvc, modules)
}
