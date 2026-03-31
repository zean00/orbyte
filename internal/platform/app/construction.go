package app

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"sync"
	"time"

	"orbyte/internal/platform/acp"
	"orbyte/internal/platform/activity"
	"orbyte/internal/platform/analytics"
	application "orbyte/internal/platform/application"
	"orbyte/internal/platform/audit"
	"orbyte/internal/platform/config"
	"orbyte/internal/platform/dataops"
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/engagement"
	"orbyte/internal/platform/eventing"
	"orbyte/internal/platform/featureflags"
	"orbyte/internal/platform/httpx"
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
	"orbyte/internal/platform/secretstore"
	"orbyte/internal/platform/securityfields"
	"orbyte/internal/platform/store"
	"orbyte/internal/platform/templateoutput"
	"orbyte/internal/platform/workflow"
)

type serviceGraph struct {
	config              *config.Service
	flags               *featureflags.Service
	secrets             *secretstore.Service
	organization        *organization.Service
	identity            *identity.Service
	modules             *module.Service
	models              *model.Service
	activities          *activity.Service
	reporting           *reporting.Service
	reference           *reference.Service
	documents           *document.Service
	workflows           *workflow.Service
	audit               *audit.Service
	logger              *logging.Service
	observability       *observability.Service
	policy              *policy.Service
	fieldSecurity       *securityfields.Service
	integration         *integration.Service
	jobs                *jobs.Service
	eventing            *eventing.Service
	search              *search.Service
	analytics           *analytics.Service
	acp                 *acp.Service
	mcpAnalytics        *mcp.AnalyticsStream
	mcpServer           *mcp.Server
	offline             *offline.Service
	monitoring          *monitoring.Service
	notifications       *notification.Service
	templates           *templateoutput.Service
	dataops             *dataops.Service
	engagement          *engagement.Service
	idempotency         *idempotency.Service
	uiPreferences       *httpx.UIPreferencesService
	runtimeHealth       *runtimehealth.Tracker
	docActions          *application.DocumentActions
	approvalPolicies    *application.ApprovalPolicyService
	workforceAttendance *application.WorkforceAttendanceCoreService
	employeeSpend       *application.EmployeeSpendCoreService
	modelActions        *application.ModelActions
	commercialCore      *application.CommercialCoreService
	procurementCore     *application.ProcurementCoreService
	inventoryCore       *application.InventoryCoreService
	fulfillmentCore     *application.FulfillmentCoreService
	deliveryCore        *application.DeliveryCoreService
	returnsCore         *application.ReturnsCoreService
	supplierReturns     *application.SupplierReturnsCoreService
	planningCore        *application.PlanningCoreService
	productionCore      *application.ProductionCoreService
	productionCosting   *application.ProductionCostingCoreService
	posCore             *application.POSCoreService
	traceabilityCore    *application.TraceabilityCoreService
	recallCore          *application.RecallCoreService
	financeReporting    *application.FinanceReportingCoreService
	financeReconcile    *application.FinanceReconciliationCoreService
	financePeriodEnd    *application.FinancePeriodEndCoreService
	financeManual       *application.FinanceManualJournalCoreService
	financeCollections  *application.FinanceCollectionsCoreService
	financeAssets       *application.FinanceAssetCoreService
	inventoryFinance    *application.InventoryFinanceCoreService
	retailFinance       *application.RetailFinanceCoreService
	treasuryCore        *application.TreasuryCoreService
	analyticsRepo       analytics.Repository
	submitStore         application.SubmitStore
	queryMonitor        *store.QueryMonitor
	otel                *otel.Service
	businessManifests   []module.Manifest
}

func constructServiceGraph(postgres *store.Postgres, businessManifests []module.Manifest) *serviceGraph {
	graph := buildCoreGraph(businessManifests)
	installPersistence(graph, postgres)
	finalizeServiceGraph(graph, postgres)
	return graph
}

func buildCoreGraph(businessManifests []module.Manifest) *serviceGraph {
	graph := &serviceGraph{
		secrets:           secretstore.NewService(),
		flags:             featureflags.NewService(),
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
		uiPreferences:     httpx.NewUIPreferencesService(),
		runtimeHealth:     runtimehealth.NewTracker(),
		analyticsRepo:     analytics.NewMemoryRepository(),
		notifications:     notification.NewService(),
		businessManifests: append([]module.Manifest(nil), businessManifests...),
	}
	graph.otel = otel.NewService("orbyte")
	acpInstr := acp.NewInstrumentation(graph.observability, graph.otel.Tracer())
	graph.config = config.NewServiceWithRepositoryAndSecrets(config.NewMemoryRepository(nil), graph.secrets)
	for _, def := range config.BuiltInDefinitions() {
		_ = graph.config.RegisterDefinition(def)
	}
	for _, entry := range config.BuiltInEntries(time.Now().UTC()) {
		_ = graph.config.Save(entry)
	}
	graph.identity = identity.NewService(graph.organization)
	graph.acp = acp.NewService(graph.config, acpInstr)
	graph.reporting = reporting.NewService(graph.models)
	graph.templates = templateoutput.NewService(graph.documents, graph.reporting)
	graph.policy = policy.NewServiceWithConfig(graph.config)
	graph.fieldSecurity = securityfields.NewService(graph.policy)
	graph.reporting.AttachFieldSecurity(graph.fieldSecurity)
	graph.integration = integration.NewService(graph.observability, graph.logger)
	graph.jobs = jobs.NewService()
	graph.jobs.AttachObservability(graph.observability)
	graph.eventing = eventing.NewServiceWithRepository(eventing.NewMemoryRepository(), graph.observability, graph.logger)
	graph.search = search.NewService()
	configureSearchEmbedding(graph.search, graph.config)
	graph.search.AttachSources(graph.documents, graph.models)
	graph.search.AttachFieldSecurity(graph.fieldSecurity)
	graph.reporting.AttachDocumentSources(graph.documents, graph.search)
	graph.integration.AttachPolicy(graph.policy)
	graph.submitStore = application.NewMemorySubmitStore(graph.documents, graph.workflows, graph.audit, graph.eventing)
	graph.modelActions = application.NewMemoryModelActions(graph.models, graph.activities, graph.audit, graph.eventing)
	graph.idempotency = idempotency.NewService()
	return graph
}

func installPersistence(graph *serviceGraph, postgres *store.Postgres) {
	if postgres == nil || postgres.DB == nil {
		return
	}
	acpInstr := acp.NewInstrumentation(graph.observability, graph.otel.Tracer())
	graph.secrets = secretstore.NewServiceWithRepository(secretstore.NewPostgresRepository(postgres.DB))
	graph.config = config.NewServiceWithRepositoryAndSecrets(config.NewPostgresRepository(postgres.DB), graph.secrets)
	dbPolicy := databaseInstrumentationPolicy(graph.config)
	graph.queryMonitor = store.NewQueryMonitor(graph.observability, store.QueryMonitorOptions{
		SlowThreshold: dbPolicy.SlowThreshold,
		TopOperations: dbPolicy.TopOperationsLimit,
		SlowQueries:   dbPolicy.SlowQueriesLimit,
	})
	graph.observability.RegisterLogEventDefinition(observability.LogEventDefinition{
		Key:            "db.query.slow",
		Category:       "database",
		Severity:       "warning",
		RequiredFields: []string{"operation", "fingerprint", "duration_millis"},
		ModuleKey:      "platform.core",
	})
	flagsDB := store.InstrumentDB(postgres.DB, graph.queryMonitor, "featureflags", "repository")
	secretsDB := store.InstrumentDB(postgres.DB, graph.queryMonitor, "secretstore", "repository")
	configDB := store.InstrumentDB(postgres.DB, graph.queryMonitor, "config", "repository")
	organizationDB := store.InstrumentDB(postgres.DB, graph.queryMonitor, "organization", "repository")
	identityDB := store.InstrumentDB(postgres.DB, graph.queryMonitor, "identity", "repository")
	moduleDB := store.InstrumentDB(postgres.DB, graph.queryMonitor, "module", "repository")
	modelDB := store.InstrumentDB(postgres.DB, graph.queryMonitor, "model", "repository")
	templateDB := store.InstrumentDB(postgres.DB, graph.queryMonitor, "templateoutput", "repository")
	referenceDB := store.InstrumentDB(postgres.DB, graph.queryMonitor, "reference", "repository")
	documentDB := store.InstrumentDB(postgres.DB, graph.queryMonitor, "document", "repository")
	workflowDB := store.InstrumentDB(postgres.DB, graph.queryMonitor, "workflow", "repository")
	auditDB := store.InstrumentDB(postgres.DB, graph.queryMonitor, "audit", "repository")
	eventingDB := store.InstrumentDB(postgres.DB, graph.queryMonitor, "eventing", "repository")
	jobsDB := store.InstrumentDB(postgres.DB, graph.queryMonitor, "jobs", "repository")
	searchDB := store.InstrumentDB(postgres.DB, graph.queryMonitor, "search", "repository")
	analyticsDB := store.InstrumentDB(postgres.DB, graph.queryMonitor, "analytics", "repository")
	integrationDB := store.InstrumentDB(postgres.DB, graph.queryMonitor, "integration", "repository")
	idempotencyDB := store.InstrumentDB(postgres.DB, graph.queryMonitor, "idempotency", "repository")
	engagementDB := store.InstrumentDB(postgres.DB, graph.queryMonitor, "engagement", "repository")
	submitDB := store.InstrumentDB(postgres.DB, graph.queryMonitor, "application", "submit_store")
	modelActionsDB := store.InstrumentDB(postgres.DB, graph.queryMonitor, "application", "model_actions")

	graph.flags = featureflags.NewServiceWithRepository(featureflags.NewPostgresRepositoryWithDB(flagsDB))
	graph.secrets = secretstore.NewServiceWithRepository(secretstore.NewPostgresRepositoryWithDB(secretsDB))
	graph.config = config.NewServiceWithRepositoryAndSecrets(config.NewPostgresRepositoryWithDB(configDB), graph.secrets)
	graph.policy = policy.NewServiceWithConfig(graph.config)
	graph.fieldSecurity = securityfields.NewService(graph.policy)
	graph.organization = organization.NewServiceWithRepository(organization.NewPostgresRepositoryWithDB(organizationDB))
	graph.identity = identity.NewServiceWithRepository(graph.organization, identity.NewPostgresRepositoryWithDB(identityDB))
	graph.acp = acp.NewService(graph.config, acpInstr)
	graph.modules = module.NewServiceWithRepository(module.NewPostgresRepositoryWithDB(moduleDB))
	graph.models = model.NewServiceWithRepository(model.NewPostgresRepositoryWithDB(modelDB))
	graph.activities = activity.NewService()
	graph.reporting = reporting.NewService(graph.models)
	graph.templates = templateoutput.NewServiceWithRepository(templateoutput.NewPostgresRepositoryWithDB(templateDB), graph.documents, graph.reporting)
	graph.reference = reference.NewServiceWithRepository(reference.NewPostgresRepositoryWithDB(referenceDB))
	graph.documents = document.NewServiceWithRepository(document.NewPostgresRepositoryWithDB(documentDB))
	graph.workflows = workflow.NewServiceWithRepository(workflow.NewPostgresRepositoryWithDB(workflowDB))
	graph.audit = audit.NewServiceWithRepository(audit.NewPostgresRepositoryWithDB(auditDB))
	graph.eventing = eventing.NewServiceWithRepository(eventing.NewPostgresRepositoryWithDB(eventingDB), graph.observability, graph.logger)
	graph.jobs = jobs.NewServiceWithRepository(jobs.NewPostgresRepositoryWithDB(jobsDB))
	graph.jobs.AttachObservability(graph.observability)
	graph.search = search.NewServiceWithRepository(search.NewPostgresRepositoryWithDB(searchDB))
	configureSearchEmbedding(graph.search, graph.config)
	analyticsRepo := analytics.NewPostgresRepositoryWithDB(analyticsDB)
	analyticsRepo.SetReadStrategyResolver(func(operation string) string {
		return dbPolicy.ReadStrategies[operation]
	})
	graph.analyticsRepo = analyticsRepo
	graph.integration = integration.NewServiceWithRepository(integration.NewPostgresRepositoryWithDB(integrationDB), graph.observability, graph.logger)
	graph.idempotency = idempotency.NewServiceWithRepository(idempotency.NewPostgresRepositoryWithDB(idempotencyDB))
	graph.engagement = engagement.NewServiceWithRepository(engagement.NewPostgresRepositoryWithDB(engagementDB))
	graph.integration.AttachPolicy(graph.policy)
	graph.reporting.AttachFieldSecurity(graph.fieldSecurity)
	graph.search.AttachSources(graph.documents, graph.models)
	graph.search.AttachFieldSecurity(graph.fieldSecurity)
	graph.reporting.AttachDocumentSources(graph.documents, graph.search)
	graph.submitStore = application.NewPostgresSubmitStoreWithDB(submitDB)
	graph.modelActions = application.NewPostgresModelActionsWithDB(modelActionsDB, graph.models, graph.activities, graph.audit, graph.eventing)
	notificationsDB := store.InstrumentDB(postgres.DB, graph.queryMonitor, "notification", "repository")
	graph.notifications = notification.NewServiceWithRepository(notification.NewPostgresRepositoryWithDB(notificationsDB))
}

func finalizeServiceGraph(graph *serviceGraph, postgres *store.Postgres) {
	graph.approvalPolicies = application.NewApprovalPolicyService(graph.models)
	graph.workforceAttendance = application.NewWorkforceAttendanceCoreService(graph.models, application.NewEmployeeWorkforceCoreService(graph.models))
	graph.employeeSpend = application.NewEmployeeSpendCoreService(graph.documents, graph.models)
	graph.docActions = application.NewDocumentActions(graph.documents, graph.workflows, graph.identity, graph.policy, graph.submitStore)
	graph.docActions.AttachApprovalPolicies(graph.approvalPolicies)
	graph.docActions.AttachActivities(graph.activities)
	graph.docActions.AttachNotifications(graph.notifications)
	graph.commercialCore = application.NewCommercialCoreService(graph.documents, graph.config, graph.models, graph.search)
	graph.procurementCore = application.NewProcurementCoreService(graph.documents, graph.config, graph.models, graph.search)
	graph.inventoryCore = application.NewInventoryCoreService(graph.documents, graph.config, graph.models, graph.search)
	graph.fulfillmentCore = application.NewFulfillmentCoreService(graph.documents, graph.search, graph.inventoryCore)
	graph.deliveryCore = application.NewDeliveryCoreService(graph.documents, graph.search)
	graph.returnsCore = application.NewReturnsCoreService(graph.documents, graph.search, graph.inventoryCore, graph.commercialCore, graph.fulfillmentCore)
	graph.supplierReturns = application.NewSupplierReturnsCoreService(graph.documents, graph.search, graph.inventoryCore, graph.procurementCore)
	graph.planningCore = application.NewPlanningCoreService(graph.documents, graph.models, graph.search, graph.inventoryCore, graph.fulfillmentCore, graph.procurementCore)
	graph.productionCore = application.NewProductionCoreService(graph.documents, graph.models, graph.search, graph.inventoryCore)
	graph.productionCosting = application.NewProductionCostingCoreService(graph.documents, graph.models, graph.inventoryCore, graph.financeReporting)
	graph.posCore = application.NewPOSCoreService(graph.documents, graph.models, graph.search, graph.docActions, graph.commercialCore, graph.inventoryCore, graph.fulfillmentCore, graph.returnsCore)
	graph.posCore.AttachWorkforceAttendance(graph.workforceAttendance)
	graph.traceabilityCore = application.NewTraceabilityCoreService(graph.documents, graph.models, graph.inventoryCore)
	graph.recallCore = application.NewRecallCoreService(graph.documents, graph.models, graph.search, graph.inventoryCore, graph.traceabilityCore)
	graph.financeReporting = application.NewFinanceReportingCoreService(graph.documents, graph.models, graph.config)
	graph.financeReconcile = application.NewFinanceReconciliationCoreService(graph.documents, graph.models, graph.financeReporting)
	graph.financePeriodEnd = application.NewFinancePeriodEndCoreService(graph.documents, graph.models, graph.financeReporting)
	graph.financeManual = application.NewFinanceManualJournalCoreService(graph.documents, graph.financeReporting)
	graph.financeCollections = application.NewFinanceCollectionsCoreService(graph.documents, graph.models, graph.financeReconcile, graph.commercialCore, graph.procurementCore, graph.financeReporting)
	graph.financeAssets = application.NewFinanceAssetCoreService(graph.documents, graph.models, graph.config, graph.financeReporting)
	graph.inventoryFinance = application.NewInventoryFinanceCoreService(graph.documents, graph.models, graph.inventoryCore, graph.financeReporting)
	graph.retailFinance = application.NewRetailFinanceCoreService(graph.documents, graph.models, graph.config, graph.financeReporting)
	graph.treasuryCore = application.NewTreasuryCoreService(graph.documents, graph.models, graph.config, graph.financeReporting, graph.retailFinance)
	graph.commercialCore.SetFinanceReporting(graph.financeReporting)
	graph.commercialCore.SetPeriodEnd(graph.financePeriodEnd)
	graph.commercialCore.SetManualJournals(graph.financeManual)
	graph.commercialCore.SetFinanceAssets(graph.financeAssets)
	graph.financeManual.SetPeriodEnd(graph.financePeriodEnd)
	graph.procurementCore.SetFinanceReporting(graph.financeReporting)
	graph.procurementCore.SetInventoryCore(graph.inventoryCore)
	graph.inventoryCore.SetFinanceReporting(graph.financeReporting)
	graph.productionCore.SetFinanceReporting(graph.financeReporting)
	graph.productionCore.SetCosting(graph.productionCosting)
	graph.posCore.SetRetailFinance(graph.retailFinance)
	graph.analytics = analytics.NewServiceWithRepository(graph.documents, graph.workflows, graph.eventing, graph.search, graph.audit, graph.observability, graph.analyticsRepo)
	graph.mcpAnalytics = mcp.NewAnalyticsStream()
	graph.analytics.SetCaptureHook(graph.mcpAnalytics.Publish)
	graph.offline = offline.NewService(graph.modules, graph.reference, graph.search)
	graph.monitoring = monitoring.NewService(graph.documents, graph.eventing, graph.workflows, graph.search, graph.observability)
	graph.monitoring.AttachQueryMonitor(graph.queryMonitor)
	graph.dataops = dataops.NewService(graph.config, graph.flags, graph.modules, graph.reference, graph.identity, graph.documents, graph.integration)
	if graph.engagement == nil {
		graph.engagement = engagement.NewService()
	}
	graph.mcpServer = mcp.NewServer(mcp.ServerDeps{
		Modules:                   graph.modules,
		Analytics:                 graph.analytics,
		Templates:                 graph.templates,
		Workflows:                 graph.workflows,
		Identity:                  graph.identity,
		Config:                    graph.config,
		Flags:                     graph.flags,
		Integration:               graph.integration,
		Documents:                 graph.documents,
		DocumentActions:           graph.docActions,
		Models:                    graph.models,
		Reference:                 graph.reference,
		Search:                    graph.search,
		FieldSecurity:             graph.fieldSecurity,
		Policy:                    graph.policy,
		Eventing:                  graph.eventing,
		Jobs:                      graph.jobs,
		Health:                    graph.runtimeHealth,
		Audit:                     graph.audit,
		Observability:             graph.observability,
		Offline:                   graph.offline,
		Dataops:                   graph.dataops,
		Engagement:                graph.engagement,
		AnalyticsStreamPath:       analyticsMCPStreamPath,
		AnalyticsScopedStreamPath: analyticsScopedMCPStreamPath,
		OTel:                      graph.otel,
	})
	configureDatabaseHealth(graph.runtimeHealth, postgres, graph.observability)
}

func configureDatabaseHealth(health *runtimehealth.Tracker, postgres *store.Postgres, obs *observability.Service) {
	if health == nil || postgres == nil || postgres.DB == nil {
		return
	}
	var (
		waitCountMu          sync.Mutex
		lastObservedWaits    int64
		waitCountInitialized bool
	)
	health.SetChecker(func(ctx context.Context) error {
		return postgres.DB.PingContext(ctx)
	})
	health.SetDBStatsProvider(func() *runtimehealth.DBStats {
		stats := postgres.DB.Stats()
		if obs != nil {
			obs.SetGauge("db.pool.connections.open", int64(stats.OpenConnections))
			obs.SetGauge("db.pool.connections.in_use", int64(stats.InUse))
			obs.SetGauge("db.pool.connections.idle", int64(stats.Idle))
			waitCountMu.Lock()
			waitDelta := stats.WaitCount
			if waitCountInitialized {
				waitDelta = stats.WaitCount - lastObservedWaits
				if waitDelta < 0 {
					waitDelta = stats.WaitCount
				}
			}
			lastObservedWaits = stats.WaitCount
			waitCountInitialized = true
			waitCountMu.Unlock()
			obs.Add("db.pool.connections.wait_count", waitDelta)
		}
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

func configureSearchEmbedding(searchSvc *search.Service, configSvc *config.Service) {
	if searchSvc == nil {
		return
	}
	policy := config.EmbeddingPolicy{Provider: "hash", Dimensions: 8}
	if configSvc != nil {
		policy = configSvc.EmbeddingPolicy()
	}
	switch strings.ToLower(strings.TrimSpace(policy.Provider)) {
	case "", "hash", "development_hash":
		searchSvc.SetEmbedder(search.NewDevelopmentHashEmbedder(policy.Dimensions))
	case "disabled", "none":
		searchSvc.SetEmbedder(search.NewDisabledEmbedder())
	default:
		searchSvc.SetEmbedder(search.NewFallbackEmbedder(strings.ToLower(strings.TrimSpace(policy.Provider)), policy.Dimensions))
	}
}

type dbInstrumentationConfig struct {
	SlowThreshold      time.Duration
	TopOperationsLimit int
	SlowQueriesLimit   int
	ReadStrategies     map[string]string
}

func databaseInstrumentationPolicy(cfg *config.Service) dbInstrumentationConfig {
	policy := dbInstrumentationConfig{
		SlowThreshold:      250 * time.Millisecond,
		TopOperationsLimit: 20,
		SlowQueriesLimit:   50,
		ReadStrategies:     map[string]string{},
	}
	if cfg == nil {
		return policy
	}
	entry, ok := cfg.Get("platform.db")
	if !ok {
		return policy
	}
	policy.SlowThreshold = time.Duration(intValue(entry.Value["slow_query_threshold_ms"], 250)) * time.Millisecond
	policy.TopOperationsLimit = intValue(entry.Value["top_operations_limit"], 20)
	policy.SlowQueriesLimit = intValue(entry.Value["slow_queries_limit"], 50)
	if raw := strings.TrimSpace(stringValue(entry.Value["read_strategies_json"])); raw != "" {
		var parsed map[string]string
		if err := json.Unmarshal([]byte(raw), &parsed); err == nil {
			policy.ReadStrategies = parsed
		}
	}
	return policy
}

func intValue(value any, fallback int) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		if err == nil {
			return parsed
		}
	}
	return fallback
}
