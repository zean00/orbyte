package httpx

import (
	"net/http"

	"orbyte/internal/platform/acp"
	"orbyte/internal/platform/activity"
	"orbyte/internal/platform/analytics"
	application "orbyte/internal/platform/application"
	"orbyte/internal/platform/audit"
	"orbyte/internal/platform/config"
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/eventing"
	"orbyte/internal/platform/featureflags"
	"orbyte/internal/platform/idempotency"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/integration"
	"orbyte/internal/platform/jobs"
	"orbyte/internal/platform/logging"
	"orbyte/internal/platform/mcp"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/monitoring"
	"orbyte/internal/platform/notification"
	"orbyte/internal/platform/observability"
	"orbyte/internal/platform/offline"
	"orbyte/internal/platform/organization"
	"orbyte/internal/platform/otel"
	"orbyte/internal/platform/policy"
	"orbyte/internal/platform/reference"
	"orbyte/internal/platform/reporting"
	"orbyte/internal/platform/runtimehealth"
	"orbyte/internal/platform/search"
	"orbyte/internal/platform/securityfields"
	"orbyte/internal/platform/templateoutput"
	"orbyte/internal/platform/workflow"
)

type RouteRegistrar func(*http.ServeMux)

type RouterConfig struct {
	Registrars    []RouteRegistrar
	CrossCutting  CrossCuttingDeps
	FieldSecurity FieldSecurityDeps
}

type FieldSecurityDeps struct {
	UI        UIDeps
	Models    ModelDeps
	Documents DocumentDeps
}

type PlatformDeps struct {
	Config       *config.Service
	Organization *organization.Service
	Identity     *identity.Service
	Reference    *reference.Service
	Documents    *document.Service
	Workflows    *workflow.Service
	Health       *runtimehealth.Tracker
}

type AuthDeps struct {
	Config        *config.Service
	Identity      *identity.Service
	Audit         *audit.Service
	UIPreferences *UIPreferencesService
}

type ModelDeps struct {
	Identity      *identity.Service
	Models        *model.Service
	Activities    *activity.Service
	Policy        *policy.Service
	FieldSecurity *securityfields.Service
	Actions       *application.ModelActions
}

type DocumentDeps struct {
	Config          *config.Service
	Identity        *identity.Service
	Modules         *module.Service
	Documents       *document.Service
	Actions         *application.DocumentActions
	Commercial      *application.CommercialCoreService
	Procurement     *application.ProcurementCoreService
	Inventory       *application.InventoryCoreService
	Fulfillment     *application.FulfillmentCoreService
	Delivery        *application.DeliveryCoreService
	Returns         *application.ReturnsCoreService
	SupplierReturns *application.SupplierReturnsCoreService
	Planning        *application.PlanningCoreService
	Production      *application.ProductionCoreService
	Traceability    *application.TraceabilityCoreService
	Recall          *application.RecallCoreService
	Audit           *audit.Service
	Policy          *policy.Service
	Search          *search.Service
	FieldSecurity   *securityfields.Service
	Observability   *observability.Service
	Idempotency     *idempotency.Service
}

type OpsDeps struct {
	Identity      *identity.Service
	Audit         *audit.Service
	Eventing      *eventing.Service
	Offline       *offline.Service
	Documents     *document.Service
	Search        *search.Service
	Workflows     *workflow.Service
	Analytics     *analytics.Service
	Monitoring    *monitoring.Service
	Notifications *notification.Service
	Observability *observability.Service
	Integration   *integration.Service
	Jobs          *jobs.Service
	Health        *runtimehealth.Tracker
}

type SearchDeps struct {
	Identity *identity.Service
	Search   *search.Service
	Jobs     *jobs.Service
}

type AdminDeps struct {
	Config        *config.Service
	Flags         *featureflags.Service
	Organization  *organization.Service
	Identity      *identity.Service
	Modules       *module.Service
	Workflows     *workflow.Service
	Audit         *audit.Service
	Policy        *policy.Service
	Observability *observability.Service
	Integration   *integration.Service
	Reference     *reference.Service
	Idempotency   *idempotency.Service
	Health        *runtimehealth.Tracker
	ACP           *acp.Service
}

type TemplateDeps struct {
	Identity  *identity.Service
	Templates *templateoutput.Service
	Documents *document.Service
	Reporting *reporting.Service
}

type UIDeps struct {
	Identity         *identity.Service
	Modules          *module.Service
	Models           *model.Service
	Activities       *activity.Service
	Reporting        *reporting.Service
	Documents        *document.Service
	Workflows        *workflow.Service
	Search           *search.Service
	Analytics        *analytics.Service
	Monitoring       *monitoring.Service
	Commercial       *application.CommercialCoreService
	Procurement      *application.ProcurementCoreService
	Inventory        *application.InventoryCoreService
	Fulfillment      *application.FulfillmentCoreService
	Delivery         *application.DeliveryCoreService
	Planning         *application.PlanningCoreService
	Production       *application.ProductionCoreService
	POS              *application.POSCoreService
	SupplierReturns  *application.SupplierReturnsCoreService
	Traceability     *application.TraceabilityCoreService
	Recall           *application.RecallCoreService
	Finance          *application.FinanceReportingCoreService
	Reconciliation   *application.FinanceReconciliationCoreService
	PeriodEnd        *application.FinancePeriodEndCoreService
	ManualJournals   *application.FinanceManualJournalCoreService
	Collections      *application.FinanceCollectionsCoreService
	FinanceAssets    *application.FinanceAssetCoreService
	InventoryFinance *application.InventoryFinanceCoreService
	RetailFinance    *application.RetailFinanceCoreService
	Policy           *policy.Service
	FieldSecurity    *securityfields.Service
	UIPreferences    *UIPreferencesService
	ACP              *acp.Service
	Notifications    *notification.Service
}

type ACPDeps struct {
	Identity *identity.Service
	Audit    *audit.Service
	Service  *acp.Service
}

type MCPDeps struct {
	Identity         *identity.Service
	Audit            *audit.Service
	Server           *mcp.Server
	Analytics        *analytics.Service
	AnalyticsStream  *mcp.AnalyticsStream
	StreamPath       string
	ScopedStreamPath string
}

type OfflineDeps struct {
	Identity        *identity.Service
	Modules         *module.Service
	Offline         *offline.Service
	Documents       *document.Service
	DocumentActions *application.DocumentActions
	Models          *model.Service
	ModelActions    *application.ModelActions
	Search          *search.Service
	FieldSecurity   *securityfields.Service
	Idempotency     *idempotency.Service
}

type CrossCuttingDeps struct {
	Config        *config.Service
	Identity      *identity.Service
	Logger        *logging.Service
	Observability *observability.Service
	Health        *runtimehealth.Tracker
	OTel          *otel.Service
}

type DocsDeps struct {
	Config    *config.Service
	Modules   *module.Service
	Models    *model.Service
	Documents *document.Service
	Search    *search.Service
}

type DeepLinkDeps struct {
	Identity  *identity.Service
	Documents *document.Service
	Workflows *workflow.Service
	Actions   *application.DocumentActions
	Audit     *audit.Service
}

type NotificationDeps struct {
	Identity      *identity.Service
	Notifications *notification.Service
	Workflows     *workflow.Service
	Documents     *document.Service
}

func RegisterPlatformSurface(deps PlatformDeps) RouteRegistrar {
	return func(mux *http.ServeMux) {
		registerCorePlatformRoutes(mux, deps, deps.Health)
	}
}

func RegisterAuthSurface(deps AuthDeps) RouteRegistrar {
	return func(mux *http.ServeMux) {
		registerAuthRoutes(mux, deps.Config, deps.Identity, deps.Audit, deps.UIPreferences)
	}
}

func RegisterModelSurface(deps ModelDeps) RouteRegistrar {
	return func(mux *http.ServeMux) {
		registerModelRoutes(mux, deps.Identity, deps.Models, deps.Activities, deps.Policy, deps.FieldSecurity, deps.Actions)
	}
}

func RegisterDocumentSurface(deps DocumentDeps) RouteRegistrar {
	return func(mux *http.ServeMux) {
		registerDocumentRoutes(mux, deps.Config, deps.Identity, deps.Modules, deps.Documents, deps.Actions, deps.Commercial, deps.Procurement, deps.Inventory, deps.Fulfillment, deps.Delivery, deps.Returns, deps.SupplierReturns, deps.Production, deps.Traceability, deps.Recall, deps.Audit, deps.Policy, deps.Search, deps.FieldSecurity, deps.Observability)
		registerDocumentFlowRoutes(mux, deps.Identity, deps.Modules, deps.Documents, deps.Actions, deps.Search, deps.FieldSecurity, deps.Idempotency)
	}
}

func RegisterOpsSurface(deps OpsDeps) RouteRegistrar {
	return func(mux *http.ServeMux) {
		registerOpsRoutes(mux, deps.Identity, deps.Audit, deps.Eventing, deps.Offline, deps.Documents, deps.Search, deps.Workflows, deps.Analytics, deps.Monitoring, deps.Observability, deps.Integration, deps.Jobs, deps.Health)
	}
}

func RegisterSearchSurface(deps SearchDeps) RouteRegistrar {
	return func(mux *http.ServeMux) {
		registerSearchRoutes(mux, deps.Identity, deps.Search, deps.Jobs)
	}
}

func RegisterAdminSurface(deps AdminDeps) RouteRegistrar {
	return func(mux *http.ServeMux) {
		registerAdminRoutes(mux, deps.Config, deps.Flags, deps.Organization, deps.Identity, deps.Modules, deps.Workflows, deps.Audit, deps.Policy, deps.Observability, deps.Integration, deps.Reference, deps.Idempotency, deps.Health, deps.ACP)
	}
}

func RegisterACPSurface(deps ACPDeps) RouteRegistrar {
	return func(mux *http.ServeMux) {
		registerACPRoutes(mux, deps.Identity, deps.Audit, deps.Service)
	}
}

func RegisterTemplateSurface(deps TemplateDeps) RouteRegistrar {
	return func(mux *http.ServeMux) {
		registerTemplateRoutes(mux, deps.Identity, deps.Templates, deps.Documents, deps.Reporting)
	}
}

func RegisterMCPSurface(deps MCPDeps) RouteRegistrar {
	return func(mux *http.ServeMux) {
		registerMCPRoutes(mux, deps.Identity, deps.Audit, deps.Server, deps.Analytics, deps.AnalyticsStream, deps.StreamPath, deps.ScopedStreamPath)
	}
}

func RegisterOfflineSurface(deps OfflineDeps) RouteRegistrar {
	return func(mux *http.ServeMux) {
		registerOfflineRoutes(mux, deps.Identity, deps.Modules, deps.Offline, deps.Documents, deps.DocumentActions, deps.Models, deps.ModelActions, deps.Search, deps.FieldSecurity, deps.Idempotency)
	}
}

func RegisterDocsSurface(deps DocsDeps) RouteRegistrar {
	return func(mux *http.ServeMux) {
		registerDocsRoutes(mux, deps.Config, deps.Modules, deps.Models, deps.Documents, deps.Search)
	}
}

func RegisterDeepLinkSurface(deps DeepLinkDeps) RouteRegistrar {
	return func(mux *http.ServeMux) {
		registerDeepLinkRoutes(mux, deps.Identity, deps.Documents, deps.Workflows, deps.Actions, deps.Audit)
	}
}

func RegisterNotificationSurface(deps NotificationDeps) RouteRegistrar {
	return func(mux *http.ServeMux) {
		registerNotificationRoutes(mux, deps.Identity, deps.Notifications, deps.Workflows, deps.Documents)
	}
}

func RegisterUISurface(deps UIDeps) RouteRegistrar {
	return func(mux *http.ServeMux) {
		registerUIRoutes(mux, deps.Identity, deps.Modules, deps.Models, deps.Activities, deps.Reporting, deps.Documents, deps.Workflows, deps.Search, deps.Analytics, deps.Monitoring, deps.Commercial, deps.Procurement, deps.Inventory, deps.Fulfillment, deps.Planning, deps.Production, deps.POS, deps.Traceability, deps.Recall, deps.Finance, deps.Reconciliation, deps.PeriodEnd, deps.ManualJournals, deps.Collections, deps.FinanceAssets, deps.InventoryFinance, deps.RetailFinance, deps.Policy, deps.FieldSecurity, deps.UIPreferences, deps.ACP)
	}
}
