package app

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
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
	"orbyte/internal/platform/observability"
	"orbyte/internal/platform/organization"
	otelplatform "orbyte/internal/platform/otel"
	"orbyte/internal/platform/policy"
	"orbyte/internal/platform/reference"
	"orbyte/internal/platform/reporting"
	"orbyte/internal/platform/runtimeconfig"
	"orbyte/internal/platform/runtimehealth"
	"orbyte/internal/platform/search"
	"orbyte/internal/platform/shared"
	"orbyte/internal/platform/store"
	"orbyte/internal/platform/templateoutput"
	"orbyte/internal/platform/workflow"
	"os"
	"runtime/debug"
	"strings"
	"time"
)

type App struct {
	address            string
	handler            http.Handler
	postgres           *store.Postgres
	closers            []func() error
	profile            string
	businessModuleKeys []string
	Config             *config.Service
	Flags              *featureflags.Service
	Organization       *organization.Service
	Identity           *identity.Service
	Documents          *document.Service
	Workflows          *workflow.Service
	Audit              *audit.Service
	Eventing           *eventing.Service
	Search             *search.Service
	Logger             *logging.Service
	Analytics          *analytics.Service
	AnalyticsScheduler *analytics.Scheduler
	Monitoring         *monitoring.Service
	RuntimeHealth      *runtimehealth.Tracker
	Modules            *module.Service
	Models             *model.Service
	Activities         *activity.Service
	Reporting          *reporting.Service
	Reference          *reference.Service
	Templates          *templateoutput.Service
	Observability      *observability.Service
	Policy             *policy.Service
	Integration        *integration.Service
	Idempotency        *idempotency.Service
	Jobs               *jobs.Service
	ACP                *acp.Service
	MCP                *mcp.Server
	DocActions         *application.DocumentActions
	ModelActions       *application.ModelActions
	Dispatcher         *eventing.Dispatcher
}

type Options struct {
	Profile           string
	BusinessManifests []module.Manifest
}

func New(opts Options) (*App, error) {
	runtimeSettings := runtimeconfig.Current()
	traceCloser, err := initializeTracing()
	if err != nil {
		return nil, fmt.Errorf("initialize tracing: %w", err)
	}
	keepTracing := true
	defer func() {
		if keepTracing && traceCloser != nil {
			_ = traceCloser()
		}
	}()
	databaseURLConfigured := runtimeSettings.DatabaseURL() != ""
	postgres, err := store.OpenFromEnv()
	if err != nil {
		if databaseURLConfigured {
			return nil, fmt.Errorf("postgres unavailable while DATABASE_URL is configured: %w", err)
		}
		log.Printf("postgres unavailable, using memory repositories: %v", err)
	}
	if err := ensureJWTSecret(databaseURLConfigured); err != nil {
		return nil, err
	}

	profile := strings.TrimSpace(opts.Profile)
	if profile == "" {
		profile = "all"
	}
	businessManifests := append([]module.Manifest(nil), opts.BusinessManifests...)
	if err := validateBusinessManifests(builtInModuleManifests(), businessManifests); err != nil {
		return nil, err
	}
	graph := constructServiceGraph(postgres, businessManifests)
	if err := seedPlatformKernel(graph.config, graph.identity, graph.modules, graph.models, graph.reporting, graph.templates, graph.reference, graph.search, graph.documents, graph.workflows, graph.policy, businessManifests, runtimeSettings.BootstrapAdminPassword()); err != nil {
		return nil, err
	}
	graph.runtimeHealth.SetBootstrapped(true)
	if report := graph.config.ValidateAll("", ""); !report.Valid {
		return nil, fmt.Errorf("configuration validation failed: %v", report.Issues)
	}
	if err := graph.policy.ValidateConfiguredModules(); err != nil {
		return nil, err
	}
	if typesenseCfg := graph.config.TypesensePolicy(); typesenseCfg.Enabled && typesenseCfg.Endpoint != "" && typesenseCfg.APIKey != "" {
		graph.search.SetBackend(search.NewTypesenseBackend(typesenseCfg.Endpoint, typesenseCfg.APIKey, time.Duration(typesenseCfg.TimeoutSeconds)*time.Second))
	}
	runtime := configureRuntime(graph)
	closers := configureAdapters(graph)
	if traceCloser != nil {
		closers = append(closers, traceCloser)
	}
	router := httpx.BuildRouter(routerDeps(graph))

	addr := runtimeSettings.Address()
	keepTracing = false

	return &App{
		address:            addr,
		handler:            router,
		postgres:           postgres,
		closers:            closers,
		profile:            profile,
		businessModuleKeys: manifestKeys(businessManifests),
		Config:             graph.config,
		Flags:              graph.flags,
		Organization:       graph.organization,
		Identity:           graph.identity,
		Documents:          graph.documents,
		Workflows:          graph.workflows,
		Audit:              graph.audit,
		Eventing:           graph.eventing,
		Search:             graph.search,
		Logger:             graph.logger,
		Analytics:          graph.analytics,
		AnalyticsScheduler: runtime.analyticsScheduler,
		Monitoring:         graph.monitoring,
		RuntimeHealth:      graph.runtimeHealth,
		Modules:            graph.modules,
		Models:             graph.models,
		Activities:         graph.activities,
		Reporting:          graph.reporting,
		Reference:          graph.reference,
		Templates:          graph.templates,
		Observability:      graph.observability,
		Policy:             graph.policy,
		Integration:        graph.integration,
		Idempotency:        graph.idempotency,
		Jobs:               graph.jobs,
		ACP:                graph.acp,
		MCP:                graph.mcpServer,
		DocActions:         graph.docActions,
		ModelActions:       graph.modelActions,
		Dispatcher:         runtime.dispatcher,
	}, nil
}

func initializeTracing() (func() error, error) {
	tp, err := otelplatform.InitStdoutTracerProvider(context.Background(), "orbyte", buildVersion())
	if err != nil {
		return nil, err
	}
	return func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return otelplatform.Shutdown(ctx, tp)
	}, nil
}

func buildVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok && strings.TrimSpace(info.Main.Version) != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}

func ensureJWTSecret(databaseURLConfigured bool) error {
	runtime := runtimeconfig.Current()
	if runtime.JWTSecret() != "" {
		return nil
	}
	if databaseURLConfigured || !runtime.AuthDevMode() {
		return fmt.Errorf("APP_JWT_SECRET is required unless APP_AUTH_DEV_MODE=true")
	}
	secret, err := generateDevelopmentJWTSecret()
	if err != nil {
		return fmt.Errorf("generate development jwt secret: %w", err)
	}
	_ = os.Setenv("APP_JWT_SECRET", secret)
	log.Printf("APP_AUTH_DEV_MODE enabled; seeded ephemeral JWT secret for this process")
	return nil
}

func generateDevelopmentJWTSecret() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func ignoreConflict(err error) error {
	var platformErr shared.Error
	if errors.As(err, &platformErr) && platformErr.Kind == shared.KindConflict {
		return nil
	}
	return err
}

func BuiltInModuleManifests() []module.Manifest {
	return append([]module.Manifest(nil), builtInModuleManifests()...)
}

func (a *App) Address() string {
	return a.address
}

func (a *App) Profile() string {
	return a.profile
}

func (a *App) BusinessModuleKeys() []string {
	return append([]string(nil), a.businessModuleKeys...)
}

func (a *App) Handler() http.Handler {
	return a.handler
}

func (a *App) StartBackground(ctx context.Context) {
	if a.RuntimeHealth != nil {
		a.RuntimeHealth.SetBackgroundStarted(true)
		a.RuntimeHealth.SetShuttingDown(false)
	}
	if a.Jobs != nil {
		a.Jobs.Start(ctx)
	}
	if a.Dispatcher != nil {
		a.Dispatcher.Start(ctx)
	}
	if a.AnalyticsScheduler != nil {
		a.AnalyticsScheduler.Start(ctx)
	}
}

func (a *App) PrepareShutdown() {
	if a.RuntimeHealth != nil {
		a.RuntimeHealth.SetShuttingDown(true)
		a.RuntimeHealth.SetBackgroundStarted(false)
	}
}

func (a *App) Close() error {
	a.PrepareShutdown()
	if a.AnalyticsScheduler != nil {
		a.AnalyticsScheduler.Stop()
	}
	if a.Dispatcher != nil {
		a.Dispatcher.Stop()
	}
	if a.Jobs != nil {
		a.Jobs.Stop()
	}
	if a.ACP != nil {
		_ = a.ACP.Close()
	}
	for _, closeFn := range a.closers {
		if closeFn == nil {
			continue
		}
		if err := closeFn(); err != nil {
			return err
		}
	}
	if a.postgres == nil {
		return nil
	}
	return a.postgres.Close()
}
