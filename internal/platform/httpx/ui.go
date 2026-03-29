package httpx

import (
	"net/http"

	"orbyte/internal/platform/acp"
	"orbyte/internal/platform/activity"
	"orbyte/internal/platform/analytics"
	"orbyte/internal/platform/application"
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

func registerUIRoutes(mux *http.ServeMux, ident *identity.Service, modules *module.Service, models *model.Service, activities *activity.Service, reportingSvc *reporting.Service, docs *document.Service, workflowSvc *workflow.Service, searchSvc *search.Service, analyticsSvc *analytics.Service, monitoringSvc *monitoring.Service, commercialSvc *application.CommercialCoreService, procurementSvc *application.ProcurementCoreService, inventorySvc *application.InventoryCoreService, fulfillmentSvc *application.FulfillmentCoreService, planningSvc *application.PlanningCoreService, productionSvc *application.ProductionCoreService, posSvc *application.POSCoreService, traceabilitySvc *application.TraceabilityCoreService, recallSvc *application.RecallCoreService, financeSvc *application.FinanceReportingCoreService, policySvc *policy.Service, fieldSecurity *securityfields.Service, uiPrefs *UIPreferencesService, acpSvc *acp.Service) {
	registerUIShellRoutes(mux)
	registerUISurfaceRoutes(mux, ident, modules, docs, policySvc, uiPrefs, acpSvc)
	registerUIDataRoutes(mux, ident, modules, models, activities, reportingSvc, docs, workflowSvc, searchSvc, analyticsSvc, monitoringSvc, commercialSvc, procurementSvc, inventorySvc, fulfillmentSvc, planningSvc, productionSvc, posSvc, traceabilitySvc, recallSvc, financeSvc, policySvc, fieldSecurity)
}

func relatedModelItems(models *model.Service, def model.Definition, recordID, relationKey string) []model.Record {
	for _, relation := range def.Relations {
		if relation.Key != relationKey {
			continue
		}
		items, _, err := models.Related(def.Key, recordID, relationKey, model.Query{Page: 1, PageSize: 100})
		if err != nil {
			return nil
		}
		return items
	}
	return nil
}
