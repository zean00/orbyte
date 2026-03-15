package app

import (
	"context"

	"orbyte/internal/platform/activity"
	"orbyte/internal/platform/analytics"
	application "orbyte/internal/platform/application"
	"orbyte/internal/platform/audit"
	"orbyte/internal/platform/config"
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/eventing"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/integration"
	"orbyte/internal/platform/jobs"
	"orbyte/internal/platform/logging"
	"orbyte/internal/platform/mcp"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/monitoring"
	"orbyte/internal/platform/observability"
	"orbyte/internal/platform/organization"
	"orbyte/internal/platform/policy"
	"orbyte/internal/platform/reference"
	"orbyte/internal/platform/reporting"
	"orbyte/internal/platform/runtimehealth"
	"orbyte/internal/platform/search"
	"orbyte/internal/platform/securityfields"
	"orbyte/internal/platform/store"
	"orbyte/internal/platform/workflow"
)

type serviceGraph struct {
	config            *config.Service
	organization      *organization.Service
	identity          *identity.Service
	modules           *module.Service
	models            *model.Service
	activities        *activity.Service
	reporting         *reporting.Service
	reference         *reference.Service
	documents         *document.Service
	workflows         *workflow.Service
	audit             *audit.Service
	logger            *logging.Service
	observability     *observability.Service
	policy            *policy.Service
	fieldSecurity     *securityfields.Service
	integration       *integration.Service
	jobs              *jobs.Service
	eventing          *eventing.Service
	search            *search.Service
	analytics         *analytics.Service
	mcpAnalytics      *mcp.AnalyticsStream
	monitoring        *monitoring.Service
	runtimeHealth     *runtimehealth.Tracker
	docActions        *application.DocumentActions
	modelActions      *application.ModelActions
	analyticsRepo     analytics.Repository
	submitStore       application.SubmitStore
	businessManifests []module.Manifest
}

func constructServiceGraph(postgres *store.Postgres, businessManifests []module.Manifest) *serviceGraph {
	graph := &serviceGraph{
		config:            config.NewService(),
		organization:      organization.NewService(),
		modules:           module.NewService(),
		models:            model.NewService(),
		activities:        activity.NewService(),
		reference:         reference.NewService(),
		documents:         document.NewService(),
		workflows:         workflow.NewService(),
		audit:             audit.NewService(),
		logger:            logging.NewService(),
		observability:     observability.NewService(),
		runtimeHealth:     runtimehealth.NewTracker(),
		analyticsRepo:     analytics.NewMemoryRepository(),
		businessManifests: append([]module.Manifest(nil), businessManifests...),
	}
	graph.identity = identity.NewService(graph.organization)
	graph.reporting = reporting.NewService(graph.models)
	graph.policy = policy.NewServiceWithConfig(graph.config)
	graph.fieldSecurity = securityfields.NewService(graph.policy)
	graph.reporting.AttachFieldSecurity(graph.fieldSecurity)
	graph.integration = integration.NewService(graph.observability, graph.logger)
	graph.jobs = jobs.NewService()
	graph.eventing = eventing.NewServiceWithRepository(eventing.NewMemoryRepository(), graph.observability, graph.logger)
	graph.search = search.NewService()
	graph.search.AttachSources(graph.documents, graph.models)
	graph.search.AttachFieldSecurity(graph.fieldSecurity)
	graph.reporting.AttachDocumentSources(graph.documents, graph.search)
	graph.integration.AttachPolicy(graph.policy)
	graph.submitStore = application.NewMemorySubmitStore(graph.documents, graph.workflows, graph.audit, graph.eventing)
	graph.modelActions = application.NewMemoryModelActions(graph.models, graph.activities, graph.audit, graph.eventing)

	if postgres != nil && postgres.DB != nil {
		graph.config = config.NewServiceWithRepository(config.NewPostgresRepository(postgres.DB))
		graph.policy = policy.NewServiceWithConfig(graph.config)
		graph.fieldSecurity = securityfields.NewService(graph.policy)
		graph.organization = organization.NewServiceWithRepository(organization.NewPostgresRepository(postgres.DB))
		graph.identity = identity.NewServiceWithRepository(graph.organization, identity.NewPostgresRepository(postgres.DB))
		graph.modules = module.NewServiceWithRepository(module.NewPostgresRepository(postgres.DB))
		graph.models = model.NewServiceWithRepository(model.NewPostgresRepository(postgres.DB))
		graph.activities = activity.NewService()
		graph.reporting = reporting.NewService(graph.models)
		graph.reference = reference.NewServiceWithRepository(reference.NewPostgresRepository(postgres.DB))
		graph.documents = document.NewServiceWithRepository(document.NewPostgresRepository(postgres.DB))
		graph.workflows = workflow.NewServiceWithRepository(workflow.NewPostgresRepository(postgres.DB))
		graph.audit = audit.NewServiceWithRepository(audit.NewPostgresRepository(postgres.DB))
		graph.eventing = eventing.NewServiceWithRepository(eventing.NewPostgresRepository(postgres.DB), graph.observability, graph.logger)
		graph.jobs = jobs.NewServiceWithRepository(jobs.NewPostgresRepository(postgres.DB))
		graph.search = search.NewServiceWithRepository(search.NewPostgresRepository(postgres.DB))
		graph.analyticsRepo = analytics.NewPostgresRepository(postgres.DB)
		graph.integration = integration.NewServiceWithRepository(integration.NewPostgresRepository(postgres.DB), graph.observability, graph.logger)
		graph.integration.AttachPolicy(graph.policy)
		graph.reporting.AttachFieldSecurity(graph.fieldSecurity)
		graph.search.AttachSources(graph.documents, graph.models)
		graph.search.AttachFieldSecurity(graph.fieldSecurity)
		graph.reporting.AttachDocumentSources(graph.documents, graph.search)
		graph.submitStore = application.NewPostgresSubmitStore(postgres.DB)
		graph.modelActions = application.NewPostgresModelActions(postgres.DB, graph.models, graph.activities, graph.audit, graph.eventing)
	}

	graph.docActions = application.NewDocumentActions(graph.documents, graph.workflows, graph.policy, graph.submitStore)
	graph.analytics = analytics.NewServiceWithRepository(graph.documents, graph.workflows, graph.eventing, graph.search, graph.audit, graph.observability, graph.analyticsRepo)
	graph.mcpAnalytics = mcp.NewAnalyticsStream()
	graph.analytics.SetCaptureHook(graph.mcpAnalytics.Publish)
	graph.monitoring = monitoring.NewService(graph.documents, graph.eventing, graph.workflows, graph.search, graph.observability)
	configureDatabaseHealth(graph.runtimeHealth, postgres)
	return graph
}

func configureDatabaseHealth(health *runtimehealth.Tracker, postgres *store.Postgres) {
	if health == nil || postgres == nil || postgres.DB == nil {
		return
	}
	health.SetChecker(func(ctx context.Context) error {
		return postgres.DB.PingContext(ctx)
	})
	health.SetDBStatsProvider(func() *runtimehealth.DBStats {
		stats := postgres.DB.Stats()
		return &runtimehealth.DBStats{
			MaxOpenConnections: stats.MaxOpenConnections,
			OpenConnections:    stats.OpenConnections,
			InUse:              stats.InUse,
			Idle:               stats.Idle,
			WaitCount:          stats.WaitCount,
			WaitDurationMillis: stats.WaitDuration.Milliseconds(),
			MaxIdleClosed:      stats.MaxIdleClosed,
			MaxIdleTimeClosed:  stats.MaxIdleTimeClosed,
			MaxLifetimeClosed:  stats.MaxLifetimeClosed,
		}
	})
}
