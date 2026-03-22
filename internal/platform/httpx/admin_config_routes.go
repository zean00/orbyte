package httpx

import (
	"net/http"

	"orbyte/internal/platform/audit"
	"orbyte/internal/platform/config"
	"orbyte/internal/platform/featureflags"
	"orbyte/internal/platform/idempotency"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/integration"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/organization"
	"orbyte/internal/platform/policy"
	"orbyte/internal/platform/reference"
	"orbyte/internal/platform/runtimehealth"
)

func registerAdminConfigRoutes(mux *http.ServeMux, cfg *config.Service, flags *featureflags.Service, org *organization.Service, ident *identity.Service, modules *module.Service, auditSvc *audit.Service, policySvc *policy.Service, integrationSvc *integration.Service, referenceSvc *reference.Service, idempotencySvc *idempotency.Service, health *runtimehealth.Tracker) {
	registerAdminFeatureFlagRoutes(mux, flags, org, ident, auditSvc)
	registerAdminSecurityModuleRoutes(mux, ident, modules, auditSvc, policySvc, integrationSvc, referenceSvc, idempotencySvc)
	registerAdminConfigRuntimeRoutes(mux, cfg, flags, ident, modules, auditSvc, health)
}
