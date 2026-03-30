package httpx

import (
	"net/http"

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

func registerUIDataRoutes(mux *http.ServeMux, ident *identity.Service, modules *module.Service, models *model.Service, activities *activity.Service, reportingSvc *reporting.Service, docs *document.Service, workflowSvc *workflow.Service, searchSvc *search.Service, analyticsSvc *analytics.Service, monitoringSvc *monitoring.Service, commercialSvc *application.CommercialCoreService, procurementSvc *application.ProcurementCoreService, inventorySvc *application.InventoryCoreService, fulfillmentSvc *application.FulfillmentCoreService, planningSvc *application.PlanningCoreService, productionSvc *application.ProductionCoreService, posSvc *application.POSCoreService, traceabilitySvc *application.TraceabilityCoreService, recallSvc *application.RecallCoreService, financeSvc *application.FinanceReportingCoreService, reconciliationSvc *application.FinanceReconciliationCoreService, periodEndSvc *application.FinancePeriodEndCoreService, manualJournalSvc *application.FinanceManualJournalCoreService, collectionsSvc *application.FinanceCollectionsCoreService, inventoryFinanceSvc *application.InventoryFinanceCoreService, retailFinanceSvc *application.RetailFinanceCoreService, policySvc *policy.Service, fieldSecurity *securityfields.Service) {
	registerUIDocumentRoutes(mux, ident, modules, docs, searchSvc, policySvc, fieldSecurity)
	registerUIWorklistRoutes(mux, ident, docs, workflowSvc, analyticsSvc, monitoringSvc)
	registerUIModelReportingRoutes(mux, ident, models, activities, reportingSvc, docs, inventorySvc, fieldSecurity)
	registerUICommercialRoutes(mux, ident, commercialSvc)
	registerUIProcurementRoutes(mux, ident, procurementSvc)
	registerUIInventoryRoutes(mux, ident, inventorySvc, traceabilitySvc)
	registerUIPlanningRoutes(mux, ident, planningSvc)
	registerUIPosRoutes(mux, ident, posSvc)
	registerUIFinanceRoutes(mux, ident, financeSvc, reconciliationSvc, periodEndSvc, manualJournalSvc, collectionsSvc, inventoryFinanceSvc, retailFinanceSvc)
	_ = fulfillmentSvc
	_ = productionSvc
	_ = recallSvc
}
