package httpx

import (
	"net/http"

	"orbyte/internal/platform/activity"
	"orbyte/internal/platform/analytics"
	application "orbyte/internal/platform/application"
	"orbyte/internal/platform/audit"
	"orbyte/internal/platform/config"
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/eventing"
	"orbyte/internal/platform/featureflags"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/idempotency"
	"orbyte/internal/platform/integration"
	"orbyte/internal/platform/jobs"
	"orbyte/internal/platform/logging"
	"orbyte/internal/platform/mcp"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/monitoring"
	"orbyte/internal/platform/observability"
	"orbyte/internal/platform/offline"
	"orbyte/internal/platform/organization"
	"orbyte/internal/platform/policy"
	"orbyte/internal/platform/reference"
	"orbyte/internal/platform/reporting"
	"orbyte/internal/platform/runtimehealth"
	"orbyte/internal/platform/search"
	"orbyte/internal/platform/securityfields"
	"orbyte/internal/platform/templateoutput"
	"orbyte/internal/platform/workflow"
)

type RouterDeps struct {
	Platform     PlatformDeps
	Auth         AuthDeps
	Models       ModelDeps
	Documents    DocumentDeps
	Ops          OpsDeps
	Search       SearchDeps
	Admin        AdminDeps
	MCP          MCPDeps
	Offline      OfflineDeps
	Templates    TemplateDeps
	UI           UIDeps
	Docs         DocsDeps
	CrossCutting CrossCuttingDeps
}

type PlatformDeps struct {
	Config       *config.Service
	Organization *organization.Service
	Identity     *identity.Service
	Reference    *reference.Service
	Documents    *document.Service
	Workflows    *workflow.Service
}

type AuthDeps struct {
	Config   *config.Service
	Identity *identity.Service
	Audit    *audit.Service
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
	Identity      *identity.Service
	Modules       *module.Service
	Documents     *document.Service
	Actions       *application.DocumentActions
	Policy        *policy.Service
	FieldSecurity *securityfields.Service
	Observability *observability.Service
	Idempotency   *idempotency.Service
}

type OpsDeps struct {
	Identity      *identity.Service
	Audit         *audit.Service
	Eventing      *eventing.Service
	Documents     *document.Service
	Search        *search.Service
	Workflows     *workflow.Service
	Analytics     *analytics.Service
	Monitoring    *monitoring.Service
	Observability *observability.Service
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
	Audit         *audit.Service
	Policy        *policy.Service
	Observability *observability.Service
	Integration   *integration.Service
	Reference     *reference.Service
	Idempotency   *idempotency.Service
}

type TemplateDeps struct {
	Identity  *identity.Service
	Templates *templateoutput.Service
}

type UIDeps struct {
	Identity      *identity.Service
	Modules       *module.Service
	Models        *model.Service
	Activities    *activity.Service
	Reporting     *reporting.Service
	Documents     *document.Service
	Search        *search.Service
	Analytics     *analytics.Service
	Monitoring    *monitoring.Service
	Policy        *policy.Service
	FieldSecurity *securityfields.Service
}

type MCPDeps struct {
	Identity        *identity.Service
	Server          *mcp.Server
	Analytics       *analytics.Service
	AnalyticsStream *mcp.AnalyticsStream
	StreamPath      string
}

type OfflineDeps struct {
	Identity        *identity.Service
	Modules         *module.Service
	Offline         *offline.Service
	Documents       *document.Service
	DocumentActions *application.DocumentActions
	Models          *model.Service
	ModelActions    *application.ModelActions
	FieldSecurity   *securityfields.Service
	Idempotency     *idempotency.Service
}

type CrossCuttingDeps struct {
	Config        *config.Service
	Identity      *identity.Service
	Logger        *logging.Service
	Observability *observability.Service
	Health        *runtimehealth.Tracker
}

type DocsDeps struct {
	Config    *config.Service
	Modules   *module.Service
	Models    *model.Service
	Documents *document.Service
	Search    *search.Service
}

func registerPlatformRoutes(mux *http.ServeMux, deps PlatformDeps, health *runtimehealth.Tracker) {
	registerCorePlatformRoutes(mux, deps, health)
}

func registerAuthRoutesWithDeps(mux *http.ServeMux, deps AuthDeps) {
	registerAuthRoutes(mux, deps.Config, deps.Identity, deps.Audit)
}

func registerModelRoutesWithDeps(mux *http.ServeMux, deps ModelDeps) {
	registerModelRoutes(mux, deps.Identity, deps.Models, deps.Activities, deps.Policy, deps.FieldSecurity, deps.Actions)
}

func registerDocumentRoutesWithDeps(mux *http.ServeMux, deps DocumentDeps) {
	registerDocumentRoutes(mux, deps.Identity, deps.Modules, deps.Documents, deps.Actions, deps.Policy, deps.FieldSecurity, deps.Observability)
	registerDocumentFlowRoutes(mux, deps.Identity, deps.Modules, deps.Documents, deps.Actions, deps.FieldSecurity, deps.Idempotency)
}

func registerOpsRoutesWithDeps(mux *http.ServeMux, deps OpsDeps) {
	registerOpsRoutes(mux, deps.Identity, deps.Audit, deps.Eventing, deps.Documents, deps.Search, deps.Workflows, deps.Analytics, deps.Monitoring, deps.Observability, deps.Jobs, deps.Health)
}

func registerSearchRoutesWithDeps(mux *http.ServeMux, deps SearchDeps) {
	registerSearchRoutes(mux, deps.Identity, deps.Search, deps.Jobs)
}

func registerAdminRoutesWithDeps(mux *http.ServeMux, deps AdminDeps) {
	registerAdminRoutes(mux, deps.Config, deps.Flags, deps.Organization, deps.Identity, deps.Modules, deps.Audit, deps.Policy, deps.Observability, deps.Integration, deps.Reference, deps.Idempotency)
}

func registerMCPRoutesWithDeps(mux *http.ServeMux, deps MCPDeps) {
	registerMCPRoutes(mux, deps.Identity, deps.Server, deps.Analytics, deps.AnalyticsStream, deps.StreamPath)
}

func registerOfflineRoutesWithDeps(mux *http.ServeMux, deps OfflineDeps) {
	registerOfflineRoutes(mux, deps.Identity, deps.Modules, deps.Offline, deps.Documents, deps.DocumentActions, deps.Models, deps.ModelActions, deps.FieldSecurity, deps.Idempotency)
}

func registerTemplateRoutesWithDeps(mux *http.ServeMux, deps TemplateDeps) {
	registerTemplateRoutes(mux, deps.Identity, deps.Templates)
}

func registerUIRoutesWithDeps(mux *http.ServeMux, deps UIDeps) {
	registerUIRoutes(mux, deps.Identity, deps.Modules, deps.Models, deps.Activities, deps.Reporting, deps.Documents, deps.Search, deps.Analytics, deps.Monitoring, deps.Policy, deps.FieldSecurity)
}

func registerDocsRoutesWithDeps(mux *http.ServeMux, deps DocsDeps) {
	registerDocsRoutes(mux, deps.Config, deps.Modules, deps.Models, deps.Documents, deps.Search)
}
