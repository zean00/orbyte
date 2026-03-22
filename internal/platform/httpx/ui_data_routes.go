package httpx

import (
	"net/http"

	"orbyte/internal/platform/activity"
	"orbyte/internal/platform/analytics"
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/monitoring"
	"orbyte/internal/platform/policy"
	"orbyte/internal/platform/reporting"
	"orbyte/internal/platform/search"
	"orbyte/internal/platform/securityfields"
	"orbyte/internal/platform/workflow"
)

func registerUIDataRoutes(mux *http.ServeMux, ident *identity.Service, modules *module.Service, models *model.Service, activities *activity.Service, reportingSvc *reporting.Service, docs *document.Service, workflowSvc *workflow.Service, searchSvc *search.Service, analyticsSvc *analytics.Service, monitoringSvc *monitoring.Service, policySvc *policy.Service, fieldSecurity *securityfields.Service) {
	registerUIDocumentRoutes(mux, ident, modules, docs, searchSvc, policySvc, fieldSecurity)
	registerUIWorklistRoutes(mux, ident, docs, workflowSvc, analyticsSvc, monitoringSvc)
	registerUIModelReportingRoutes(mux, ident, models, activities, reportingSvc, docs, fieldSecurity)
}
