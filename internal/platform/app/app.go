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
	"orbyte/internal/platform/i18n"
	"orbyte/internal/platform/idempotency"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/integration"
	"orbyte/internal/platform/jobs"
	"orbyte/internal/platform/logging"
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
	"orbyte/internal/platform/shared"
	"orbyte/internal/platform/store"
	"orbyte/internal/platform/templateoutput"
	"orbyte/internal/platform/workflow"
	"os"
	"sort"
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
	DocActions         *application.DocumentActions
	ModelActions       *application.ModelActions
	Dispatcher         *eventing.Dispatcher
}

func localize(en, id string) i18n.LocalizedText {
	return i18n.LocalizedText{"en": en, "id": id}
}

type Options struct {
	Profile           string
	BusinessManifests []module.Manifest
}

func New(opts Options) (*App, error) {
	databaseURLConfigured := strings.TrimSpace(os.Getenv("DATABASE_URL")) != ""
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
	if err := seedPlatformKernel(graph.config, graph.identity, graph.modules, graph.models, graph.reporting, graph.templates, graph.reference, graph.search, graph.documents, graph.workflows, graph.policy, businessManifests, strings.TrimSpace(os.Getenv("APP_BOOTSTRAP_ADMIN_PASSWORD"))); err != nil {
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
	router := httpx.BuildRouter(routerDeps(graph))

	addr := os.Getenv("APP_ADDRESS")
	if addr == "" {
		addr = ":8080"
	}

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
		DocActions:         graph.docActions,
		ModelActions:       graph.modelActions,
		Dispatcher:         runtime.dispatcher,
	}, nil
}

func ensureJWTSecret(databaseURLConfigured bool) error {
	if strings.TrimSpace(os.Getenv("APP_JWT_SECRET")) != "" {
		return nil
	}
	if databaseURLConfigured || !boolFromEnv("APP_AUTH_DEV_MODE") {
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

func boolFromEnv(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func ignoreConflict(err error) error {
	var platformErr shared.Error
	if errors.As(err, &platformErr) && platformErr.Kind == shared.KindConflict {
		return nil
	}
	return err
}

func seedPlatformKernel(configSvc *config.Service, identitySvc *identity.Service, moduleSvc *module.Service, modelSvc *model.Service, reportingSvc *reporting.Service, templateSvc *templateoutput.Service, referenceSvc *reference.Service, searchSvc *search.Service, documentSvc *document.Service, workflowSvc *workflow.Service, policySvc *policy.Service, businessManifests []module.Manifest, bootstrapPassword string) error {
	manifests := append(builtInModuleManifests(), businessManifests...)
	for _, def := range config.BuiltInDefinitions() {
		if err := configSvc.RegisterDefinition(def); err != nil {
			return err
		}
	}
	for _, manifest := range manifests {
		if err := moduleSvc.Register(manifest, "system"); err != nil {
			return err
		}
		for _, def := range manifest.ConfigDefinitions {
			if err := configSvc.RegisterDefinition(def); err != nil {
				return err
			}
		}
	}
	for _, entry := range config.BuiltInEntries(time.Now().UTC()) {
		if _, ok := configSvc.Get(entry.Key); !ok {
			if err := configSvc.Save(entry); err != nil {
				return err
			}
		}
	}
	for _, manifest := range manifests {
		for _, def := range manifest.ReferenceTypes {
			if err := ignoreConflict(referenceSvc.RegisterType(def)); err != nil {
				return err
			}
		}
		for _, record := range manifest.ReferenceRecords {
			if err := ignoreConflict(referenceSvc.UpsertRecord(record)); err != nil {
				return err
			}
		}
		for _, def := range manifest.Models {
			if err := ignoreConflict(modelSvc.Register(def)); err != nil {
				return err
			}
		}
		for _, def := range manifest.Documents {
			if err := ignoreConflict(documentSvc.Register(def)); err != nil {
				return err
			}
		}
		for _, def := range manifest.Workflows {
			if err := ignoreConflict(workflowSvc.Register(def)); err != nil {
				return err
			}
		}
		for _, index := range manifest.SearchIndexes {
			if err := ignoreConflict(searchSvc.RegisterIndex(index)); err != nil {
				return err
			}
		}
		for _, dataset := range manifest.Datasets {
			if err := ignoreConflict(reportingSvc.Register(reporting.DatasetDefinition{
				Key:        dataset.Key,
				Title:      dataset.Title,
				SourceKind: dataset.SourceKind,
				ModelKey:   dataset.ModelKey,
				Dimensions: datasetDimensions(dataset.Dimensions),
				Measures:   datasetMeasures(dataset.Measures),
			})); err != nil {
				return err
			}
		}
		for _, def := range manifest.Templates {
			if err := ignoreConflict(templateSvc.RegisterDefinition(templateoutput.FromModule(def, manifest.Key))); err != nil {
				return err
			}
		}
	}
	if err := identitySvc.SeedBootstrapData(bootstrapPassword); err != nil {
		return err
	}
	if err := identitySvc.EnsureBootstrapAdminCredential(bootstrapPassword); err != nil {
		return err
	}
	if err := seedModuleContracts(identitySvc, policySvc, manifests); err != nil {
		return err
	}
	seedModelRules(modelSvc)
	seedModelData(modelSvc)
	for _, manifest := range manifests {
		for _, extension := range manifest.DocumentExtensions {
			if err := ignoreConflict(documentSvc.RegisterExtension(document.ExtensionDefinition{
				DocumentType:       extension.DocumentType,
				ModuleKey:          manifest.Key,
				DisplayName:        extension.DisplayName,
				SchemaVersion:      extension.SchemaVersion,
				ReadPermissionKey:  extension.ReadPermissionKey,
				WritePermissionKey: extension.WritePermissionKey,
			})); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateBusinessManifests(builtIn []module.Manifest, business []module.Manifest) error {
	svc := module.NewService()
	known := make(map[string]module.Manifest, len(builtIn)+len(business))
	for _, manifest := range builtIn {
		known[manifest.Key] = manifest
		if err := svc.Register(manifest, "system"); err != nil {
			return err
		}
	}
	for _, manifest := range business {
		if strings.TrimSpace(manifest.Key) == "" {
			return fmt.Errorf("business manifest key is required")
		}
		if _, exists := known[manifest.Key]; exists {
			return fmt.Errorf("duplicate module key %q in selected manifests", manifest.Key)
		}
		known[manifest.Key] = manifest
		if err := svc.Register(manifest, "system"); err != nil {
			return err
		}
	}
	for _, manifest := range business {
		for _, requirement := range manifest.DependencyRequirements {
			if requirement.Kind == module.DependencyKindOptional || requirement.Kind == module.DependencyKindUIExtension {
				continue
			}
			if _, ok := known[requirement.ModuleKey]; !ok {
				return fmt.Errorf("module %q requires %q but it is not included in the selected profile", manifest.Key, requirement.ModuleKey)
			}
		}
		if manifest.Role == module.ModuleRoleLocalExtension {
			baseModuleKey := strings.TrimSpace(manifest.LocalExtension.BaseModuleKey)
			baseManifest, ok := known[baseModuleKey]
			if !ok {
				return fmt.Errorf("module %q targets base module %q but it is not included in the selected profile", manifest.Key, baseModuleKey)
			}
			if baseManifest.Role != module.ModuleRoleBase {
				return fmt.Errorf("module %q targets base module %q but that module does not use role base", manifest.Key, baseModuleKey)
			}
		}
	}
	return nil
}

func manifestKeys(manifests []module.Manifest) []string {
	keys := make([]string, 0, len(manifests))
	for _, manifest := range manifests {
		keys = append(keys, manifest.Key)
	}
	sort.Strings(keys)
	return keys
}

func datasetDimensions(input []module.DatasetDimension) []reporting.DimensionDefinition {
	items := make([]reporting.DimensionDefinition, 0, len(input))
	for _, item := range input {
		items = append(items, reporting.DimensionDefinition{Key: item.Key, Label: item.Label, Path: item.Path})
	}
	return items
}

func datasetMeasures(input []module.DatasetMeasure) []reporting.MeasureDefinition {
	items := make([]reporting.MeasureDefinition, 0, len(input))
	for _, item := range input {
		items = append(items, reporting.MeasureDefinition{Key: item.Key, Label: item.Label, Kind: item.Kind, Path: item.Path})
	}
	return items
}

func seedModuleContracts(identitySvc *identity.Service, policySvc *policy.Service, manifests []module.Manifest) error {
	for _, manifest := range manifests {
		for _, permission := range manifest.Security.Permissions {
			if err := identitySvc.UpsertPermission(identity.Permission{
				Key:         permission.Key,
				Module:      manifest.Key,
				Action:      permission.Action,
				Resource:    permission.Resource,
				Description: permission.Description,
			}); err != nil {
				return err
			}
			if err := identitySvc.GrantRolePermission(identity.RolePermission{RoleID: "role_admin", PermissionKey: permission.Key}); err != nil {
				return err
			}
		}
		for _, roleTemplate := range manifest.Security.RoleTemplates {
			roleID := "role:" + manifest.Key + ":" + roleTemplate.Key
			if err := identitySvc.UpsertRole(identity.Role{
				ID:        roleID,
				Key:       roleTemplate.Key,
				Name:      roleTemplate.Name,
				ScopeType: firstScope(roleTemplate.AllowedScopes),
			}); err != nil {
				return err
			}
			for _, permissionKey := range roleTemplate.PermissionKeys {
				if err := identitySvc.GrantRolePermission(identity.RolePermission{RoleID: roleID, PermissionKey: permissionKey}); err != nil {
					return err
				}
			}
		}
		for _, hook := range manifest.Security.PolicyHooks {
			if err := policySvc.Register(policy.HookDefinition{
				Key:               hook.Key,
				Kind:              hook.Kind,
				Target:            hook.Target,
				InputContractKey:  hook.InputContractKey,
				OutputContractKey: hook.OutputContractKey,
				Description:       hook.Description,
				Engine:            policyEngineForHook(hook.Key),
				RegoPackage:       policyRegoPackageForHook(hook.Key),
				RegoQuery:         policyRegoQueryForHook(hook.Key),
				DefaultRegoSource: defaultPolicyModule(hook.Key),
				AllowedScopes:     []string{"deployment", "organization", "location"},
				DefaultRule:       defaultPolicyRule(hook.Key),
			}); err != nil {
				return err
			}
		}
	}
	if err := policySvc.SetEvaluator("documents.extension.view", func(req policy.Request) policy.Decision {
		moduleKey, _ := req.Inputs["module_key"].(string)
		if strings.TrimSpace(moduleKey) == "" {
			return policy.Decision{Allowed: false, Code: "missing_module", Reason: "module key is required"}
		}
		if denied := stringSliceRule(req.Rule, "denied_statuses"); containsValue(denied, stringValue(req.Inputs["document_status"])) {
			return policy.Decision{Allowed: false, Code: "status_blocked", Reason: "document status blocked by policy"}
		}
		if allowed := stringSliceRule(req.Rule, "allowed_modules"); len(allowed) > 0 && !containsValue(allowed, moduleKey) {
			return policy.Decision{Allowed: false, Code: "module_blocked", Reason: "module extension view blocked by policy"}
		}
		if required := stringSliceRule(req.Rule, "required_permissions"); len(required) > 0 {
			return policy.Decision{Allowed: true, Code: "allowed_with_permissions", Output: map[string]any{"required_permissions": required}}
		}
		return policy.Decision{Allowed: true}
	}); err != nil {
		return err
	}
	if err := policySvc.SetEvaluator("documents.extension.write", func(req policy.Request) policy.Decision {
		moduleKey, _ := req.Inputs["module_key"].(string)
		if strings.TrimSpace(moduleKey) == "" {
			return policy.Decision{Allowed: false, Code: "missing_module", Reason: "module key is required"}
		}
		if denied := stringSliceRule(req.Rule, "denied_statuses"); containsValue(denied, stringValue(req.Inputs["document_status"])) {
			return policy.Decision{Allowed: false, Code: "status_blocked", Reason: "document status blocked by policy"}
		}
		if allowed := stringSliceRule(req.Rule, "allowed_modules"); len(allowed) > 0 && !containsValue(allowed, moduleKey) {
			return policy.Decision{Allowed: false, Code: "module_blocked", Reason: "module extension write blocked by policy"}
		}
		if required := stringSliceRule(req.Rule, "required_permissions"); len(required) > 0 {
			return policy.Decision{Allowed: true, Code: "allowed_with_permissions", Output: map[string]any{"required_permissions": required}}
		}
		return policy.Decision{Allowed: true}
	}); err != nil {
		return err
	}
	if err := policySvc.SetEvaluator("documents.workflow.transition", func(req policy.Request) policy.Decision {
		action, _ := req.Inputs["action"].(string)
		if strings.TrimSpace(action) == "" {
			return policy.Decision{Allowed: false, Code: "missing_action", Reason: "action is required"}
		}
		if blocked := stringSliceRule(req.Rule, "blocked_actions"); containsValue(blocked, action) {
			return policy.Decision{Allowed: false, Code: "action_blocked", Reason: "workflow action blocked by policy"}
		}
		if allowed := stringSliceRule(req.Rule, "allowed_actions"); len(allowed) > 0 && !containsValue(allowed, action) {
			return policy.Decision{Allowed: false, Code: "action_not_allowed", Reason: "workflow action not allowed by policy"}
		}
		if statuses := stringSliceRule(req.Rule, "allowed_statuses"); len(statuses) > 0 && !containsValue(statuses, stringValue(req.Inputs["status"])) {
			return policy.Decision{Allowed: false, Code: "status_not_allowed", Reason: "document status not allowed by policy"}
		}
		if minimum := intRule(req.Rule, "minimum_amount_minor"); minimum > 0 && intRule(req.Inputs, "amount_minor") < minimum {
			return policy.Decision{Allowed: false, Code: "amount_below_threshold", Reason: "document amount below policy threshold"}
		}
		if boolRule(req.Rule, "require_number") && strings.TrimSpace(stringValue(req.Inputs["number"])) == "" {
			return policy.Decision{Allowed: false, Code: "number_required", Reason: "document number is required by policy"}
		}
		return policy.Decision{Allowed: true}
	}); err != nil {
		return err
	}
	if err := policySvc.SetEvaluator("documents.workflow.assignment", func(req policy.Request) policy.Decision {
		output := map[string]any{}
		if role := strings.TrimSpace(stringValue(req.Rule["assignee_role_key"])); role != "" {
			output["assignee_role_key"] = role
		}
		if mode := strings.TrimSpace(stringValue(req.Rule["assignment_mode"])); mode != "" {
			output["assignment_mode"] = mode
		}
		if userID := strings.TrimSpace(stringValue(req.Rule["assignee_user_id"])); userID != "" {
			output["assignee_user_id"] = userID
		}
		if candidates := stringSliceRule(req.Rule, "candidate_role_keys"); len(candidates) > 0 {
			output["candidate_role_keys"] = candidates
		}
		return policy.Decision{Allowed: true, Output: output}
	}); err != nil {
		return err
	}
	if err := policySvc.SetEvaluator("documents.workflow.sla", func(req policy.Request) policy.Decision {
		output := map[string]any{}
		if due := intRule(req.Rule, "due_after_seconds"); due > 0 {
			output["due_after_seconds"] = due
		}
		if escalate := intRule(req.Rule, "escalate_after_seconds"); escalate > 0 {
			output["escalate_after_seconds"] = escalate
		}
		return policy.Decision{Allowed: true, Output: output}
	}); err != nil {
		return err
	}
	if err := policySvc.SetEvaluator("documents.search.visibility", func(req policy.Request) policy.Decision {
		if hidden := stringSliceRule(req.Rule, "hidden_statuses"); containsValue(hidden, stringValue(req.Inputs["status"])) {
			return policy.Decision{Allowed: false, Code: "status_hidden", Reason: "document status hidden by policy"}
		}
		if allowed := stringSliceRule(req.Rule, "allowed_types"); len(allowed) > 0 && !containsValue(allowed, stringValue(req.Inputs["document_type"])) {
			return policy.Decision{Allowed: false, Code: "type_hidden", Reason: "document type hidden by policy"}
		}
		if allowlist := stringSliceRule(req.Rule, "location_allowlist"); len(allowlist) > 0 && !containsValue(allowlist, stringValue(req.Inputs["location_id"])) {
			return policy.Decision{Allowed: false, Code: "location_hidden", Reason: "document location hidden by policy"}
		}
		return policy.Decision{Allowed: true}
	}); err != nil {
		return err
	}
	if err := policySvc.SetEvaluator("documents.numbering.assign", func(req policy.Request) policy.Decision {
		numberingKey, _ := req.Inputs["numbering_key"].(string)
		documentType, _ := req.Inputs["document_type"].(string)
		locationID, _ := req.Inputs["location_id"].(string)
		prefix := strings.ToUpper(strings.TrimSpace(stringValue(req.Rule["prefix"])))
		if prefix == "" {
			prefix = strings.ToUpper(strings.TrimSpace(numberingKey))
		}
		if prefix == "" {
			prefix = strings.ToUpper(strings.TrimSpace(documentType))
		}
		if boolRule(req.Rule, "include_location") && locationID != "" {
			prefix += "-" + strings.ToUpper(locationID)
		}
		number := prefix
		if boolRule(req.Rule, "include_date") {
			number += "-" + time.Now().UTC().Format("20060102")
		}
		padding := intRule(req.Rule, "sequence_padding")
		sequence := time.Now().UTC().Format("150405")
		if padding > 0 {
			sequence = fmt.Sprintf("%0*s", padding, sequence)
		}
		return policy.Decision{
			Allowed: true,
			Output: map[string]any{
				"number": number + "-" + sequence,
			},
		}
	}); err != nil {
		return err
	}
	if err := policySvc.SetEvaluator("documents.action.render", func(req policy.Request) policy.Decision {
		action := stringValue(req.Inputs["action"])
		if containsValue(stringSliceRule(req.Rule, "hidden_actions"), action) {
			return policy.Decision{Allowed: false, Code: "action_hidden", Reason: "action hidden by policy"}
		}
		if containsValue(stringSliceRule(req.Rule, "primary_actions"), action) {
			return policy.Decision{Allowed: true, Code: "primary", Output: map[string]any{"placement": "primary"}}
		}
		return policy.Decision{Allowed: true, Code: "secondary", Output: map[string]any{"placement": "secondary"}}
	}); err != nil {
		return err
	}
	if err := policySvc.SetEvaluator("integration.submission.preflight", func(req policy.Request) policy.Decision {
		if blocked := stringSliceRule(req.Rule, "blocked_operation_types"); containsValue(blocked, stringValue(req.Inputs["operation_type"])) {
			return policy.Decision{Allowed: false, Code: "operation_blocked", Reason: "integration operation blocked by policy"}
		}
		requiredStatus := stringValue(req.Rule["required_system_status"])
		if requiredStatus != "" && requiredStatus != stringValue(req.Inputs["system_status"]) {
			return policy.Decision{Allowed: false, Code: "system_status_blocked", Reason: "integration system status does not satisfy policy"}
		}
		return policy.Decision{Allowed: true}
	}); err != nil {
		return err
	}
	return nil
}

func firstScope(scopes []string) string {
	if len(scopes) == 0 {
		return "deployment"
	}
	return scopes[0]
}

func bootstrapRuntimeModuleContracts(moduleSvc *module.Service, obsSvc *observability.Service, analyticsSvc *analytics.Service) {
	for _, detail := range moduleSvc.List() {
		manifest := detail.Manifest
		for _, metric := range manifest.Observability.Metrics {
			obsSvc.RegisterMetricDefinition(observability.MetricDefinition{
				Key:         metric.Key,
				Type:        metric.Type,
				Labels:      append([]string(nil), metric.Labels...),
				Description: metric.Description,
				ModuleKey:   manifest.Key,
			})
		}
		for _, event := range manifest.Observability.LogEvents {
			obsSvc.RegisterLogEventDefinition(observability.LogEventDefinition{
				Key:            event.Key,
				Category:       event.Category,
				Severity:       event.Severity,
				RequiredFields: append([]string(nil), event.RequiredFields...),
				ModuleKey:      manifest.Key,
			})
		}
		for _, event := range manifest.Observability.DomainEvents {
			obsSvc.RegisterDomainEventDefinition(observability.DomainEventDefinition{
				Type:                event.Type,
				Role:                event.Role,
				CorrelationRequired: event.CorrelationRequired,
				ModuleKey:           manifest.Key,
			})
		}
		for _, report := range manifest.Observability.Reports {
			_, _ = analyticsSvc.EnsureReportDefinition(analytics.ReportDefinition{
				ID:        "report:" + manifest.Key + ":" + report.Key,
				Name:      report.Title,
				Dimension: firstValue(report.Dataset, "document_type"),
				Format:    firstReportFormat(report.Formats),
				Schedule:  "daily",
				Window:    "current_state",
			})
		}
	}
}

func externalBrokerRoutes(moduleSvc *module.Service, subjectPrefix string) map[string]string {
	routes := map[string]string{}
	for _, detail := range moduleSvc.List() {
		if !detail.Installed.Enabled {
			continue
		}
		for _, event := range detail.Manifest.Observability.DomainEvents {
			if !event.ExternalPublish || strings.TrimSpace(event.Topic) == "" {
				continue
			}
			routes[event.Type] = brokerTopic(subjectPrefix, event.Topic)
		}
	}
	return routes
}

func brokerTopic(prefix, topic string) string {
	prefix = strings.Trim(strings.TrimSpace(prefix), ".")
	topic = strings.Trim(strings.TrimSpace(topic), ".")
	if prefix == "" {
		return topic
	}
	if topic == "" {
		return prefix
	}
	return prefix + "." + topic
}

func seedModelRules(modelSvc *model.Service) {
	modelSvc.SetDefaultEvaluator("party.status.default", func(_ model.RuleInput) (any, error) {
		return "active", nil
	})
	modelSvc.SetComputeEvaluator("party.display_name.compute", func(input model.RuleInput) (any, error) {
		return firstValue(strings.TrimSpace(stringValue(input.Values["name"])), strings.TrimSpace(stringValue(input.Existing["name"]))), nil
	})
	modelSvc.SetConstraintEvaluator("party.status.allowed", func(input model.RuleInput) error {
		switch strings.TrimSpace(stringValue(input.Values["status"])) {
		case "active", "inactive", "blocked":
			return nil
		default:
			return shared.Validation("party status must be active, inactive, or blocked")
		}
	})
}

func seedModelData(modelSvc *model.Service) {
	items, _, err := modelSvc.List("party", model.Query{Page: 1, PageSize: 1})
	if err == nil && len(items) > 0 {
		return
	}
	_, _ = modelSvc.Create("party", "system", map[string]any{
		"name":   "Walk In Customer",
		"email":  "walkin@clinic.local",
		"status": "active",
	})
	parties, _, err := modelSvc.List("party", model.Query{Page: 1, PageSize: 1})
	if err == nil && len(parties) > 0 {
		_, _ = modelSvc.Create("party_contact", "system", map[string]any{
			"party_id": parties[0].ID,
			"name":     "Reception Desk",
			"phone":    "+62-21-000000",
			"role":     "primary",
		})
	}
}

func defaultPolicyRule(hookKey string) map[string]any {
	switch hookKey {
	case "documents.extension.view", "documents.extension.write":
		return map[string]any{"allowed_modules": []string{}, "denied_statuses": []string{"cancelled"}}
	case "documents.workflow.transition":
		return map[string]any{"blocked_actions": []string{}, "allowed_actions": []string{}, "allowed_statuses": []string{}}
	case "documents.search.visibility":
		return map[string]any{"hidden_statuses": []string{}, "allowed_types": []string{}}
	case "documents.numbering.assign":
		return map[string]any{"prefix": "", "include_location": true, "include_date": true}
	case "documents.action.render":
		return map[string]any{"hidden_actions": []string{}, "primary_actions": []string{"submit", "approve"}}
	case "integration.submission.preflight":
		return map[string]any{"blocked_operation_types": []string{}, "required_system_status": "active"}
	default:
		return map[string]any{}
	}
}

func defaultPolicyModule(hookKey string) string {
	switch hookKey {
	case "documents.search.visibility":
		return `package orbyte.policy.documents.search.visibility

import rego.v1

default decision := {"allowed": true}

decision := {"allowed": false, "code": "status_hidden", "reason": "document status hidden by policy"} if {
	input.inputs.status != ""
	input.inputs.status in object.get(input.rule, "hidden_statuses", [])
} else := {"allowed": false, "code": "type_hidden", "reason": "document type hidden by policy"} if {
	count(object.get(input.rule, "allowed_types", [])) > 0
	not input.inputs.document_type in object.get(input.rule, "allowed_types", [])
} else := {"allowed": false, "code": "location_hidden", "reason": "document location hidden by policy"} if {
	count(object.get(input.rule, "location_allowlist", [])) > 0
	not input.inputs.location_id in object.get(input.rule, "location_allowlist", [])
}`
	case "documents.workflow.transition":
		return `package orbyte.policy.documents.workflow.transition

import rego.v1

default decision := {"allowed": true}

decision := {"allowed": false, "code": "missing_action", "reason": "action is required"} if {
	trim_space(object.get(input.inputs, "action", "")) == ""
} else := {"allowed": false, "code": "action_blocked", "reason": "workflow action blocked by policy"} if {
	input.inputs.action in object.get(input.rule, "blocked_actions", [])
} else := {"allowed": false, "code": "action_not_allowed", "reason": "workflow action not allowed by policy"} if {
	count(object.get(input.rule, "allowed_actions", [])) > 0
	not input.inputs.action in object.get(input.rule, "allowed_actions", [])
} else := {"allowed": false, "code": "status_not_allowed", "reason": "document status not allowed by policy"} if {
	count(object.get(input.rule, "allowed_statuses", [])) > 0
	not input.inputs.status in object.get(input.rule, "allowed_statuses", [])
} else := {"allowed": false, "code": "amount_below_threshold", "reason": "document amount below policy threshold"} if {
	object.get(input.rule, "minimum_amount_minor", 0) > 0
	object.get(input.inputs, "amount_minor", 0) < object.get(input.rule, "minimum_amount_minor", 0)
} else := {"allowed": false, "code": "number_required", "reason": "document number is required by policy"} if {
	object.get(input.rule, "require_number", false)
	trim_space(object.get(input.inputs, "number", "")) == ""
}`
	case "integration.submission.preflight":
		return `package orbyte.policy.integration.submission.preflight

import rego.v1

default decision := {"allowed": true}

decision := {"allowed": false, "code": "operation_blocked", "reason": "integration operation blocked by policy"} if {
	input.inputs.operation_type in object.get(input.rule, "blocked_operation_types", [])
} else := {"allowed": false, "code": "system_status_blocked", "reason": "integration system status does not satisfy policy"} if {
	object.get(input.rule, "required_system_status", "") != ""
	object.get(input.rule, "required_system_status", "") != object.get(input.inputs, "system_status", "")
}`
	default:
		return ""
	}
}

func policyEngineForHook(hookKey string) string {
	switch hookKey {
	case "documents.search.visibility", "documents.workflow.transition", "integration.submission.preflight":
		return policy.EngineRego
	default:
		return policy.EngineGo
	}
}

func policyRegoPackageForHook(hookKey string) string {
	if policyEngineForHook(hookKey) != policy.EngineRego {
		return ""
	}
	return policy.RegoPackageForHook(hookKey)
}

func policyRegoQueryForHook(hookKey string) string {
	if policyEngineForHook(hookKey) != policy.EngineRego {
		return ""
	}
	return "data." + policy.RegoPackageForHook(hookKey) + ".decision"
}

func stringSliceRule(rule map[string]any, key string) []string {
	raw, _ := rule[key]
	switch typed := raw.(type) {
	case []string:
		return append([]string(nil), typed...)
	case []any:
		items := make([]string, 0, len(typed))
		for _, item := range typed {
			if text, ok := item.(string); ok && strings.TrimSpace(text) != "" {
				items = append(items, strings.TrimSpace(text))
			}
		}
		return items
	default:
		return nil
	}
}

func boolRule(rule map[string]any, key string) bool {
	if value, ok := rule[key].(bool); ok {
		return value
	}
	return false
}

func intRule(rule map[string]any, key string) int {
	switch value := rule[key].(type) {
	case int:
		return value
	case int32:
		return int(value)
	case int64:
		return int(value)
	case float64:
		return int(value)
	default:
		return 0
	}
}

func stringValue(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func containsValue(items []string, candidate string) bool {
	for _, item := range items {
		if item == candidate {
			return true
		}
	}
	return false
}

func firstReportFormat(formats []string) string {
	if len(formats) == 0 {
		return "csv"
	}
	return formats[0]
}

func firstValue(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func builtInModuleManifests() []module.Manifest {
	authDefinition, _ := config.NewService().Definition("identity.auth")
	httpDefinition, _ := config.NewService().Definition("platform.http")
	seededAt := time.Now().UTC()
	return []module.Manifest{
		{
			Key:          "reference_masterdata",
			Name:         "Reference Master Data",
			NameI18n:     localize("Reference Master Data", "Data Referensi"),
			Version:      "1.0.0",
			DomainFamily: "platform",
			DependencyRequirements: []module.DependencyRequirement{
				{ModuleKey: "platform.core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			},
			ReferenceTypes: []reference.TypeDefinition{
				{Key: "currency", DisplayName: "Currency", DisplayNameI18n: localize("Currency", "Mata Uang"), OwnerModuleKey: "reference_masterdata"},
				{Key: "uom", DisplayName: "Unit of Measure", DisplayNameI18n: localize("Unit of Measure", "Satuan Ukur"), OwnerModuleKey: "reference_masterdata"},
				{Key: "party_type", DisplayName: "Party Type", DisplayNameI18n: localize("Party Type", "Tipe Pihak"), OwnerModuleKey: "reference_masterdata"},
				{Key: "location_type", DisplayName: "Location Type", DisplayNameI18n: localize("Location Type", "Tipe Lokasi"), OwnerModuleKey: "reference_masterdata"},
				{Key: "document_reason", DisplayName: "Document Reason", DisplayNameI18n: localize("Document Reason", "Alasan Dokumen"), OwnerModuleKey: "reference_masterdata"},
				{Key: "appointment_type", DisplayName: "Appointment Type", DisplayNameI18n: localize("Appointment Type", "Tipe Janji Temu"), OwnerModuleKey: "reference_masterdata"},
				{Key: "patient_identifier_type", DisplayName: "Patient Identifier Type", DisplayNameI18n: localize("Patient Identifier Type", "Tipe Identitas Pasien"), OwnerModuleKey: "reference_masterdata"},
				{Key: "practitioner_type", DisplayName: "Practitioner Type", DisplayNameI18n: localize("Practitioner Type", "Tipe Praktisi"), OwnerModuleKey: "reference_masterdata"},
				{Key: "payer_type", DisplayName: "Payer Type", DisplayNameI18n: localize("Payer Type", "Tipe Penjamin"), OwnerModuleKey: "reference_masterdata"},
				{Key: "visit_priority", DisplayName: "Visit Priority", DisplayNameI18n: localize("Visit Priority", "Prioritas Kunjungan"), OwnerModuleKey: "reference_masterdata"},
				{Key: "shipment_method", DisplayName: "Shipment Method", DisplayNameI18n: localize("Shipment Method", "Metode Pengiriman"), OwnerModuleKey: "reference_masterdata"},
				{Key: "item_category", DisplayName: "Item Category", DisplayNameI18n: localize("Item Category", "Kategori Item"), OwnerModuleKey: "reference_masterdata"},
			},
			ReferenceRecords: []reference.Record{
				{TypeKey: "currency", Key: "IDR", DisplayName: "Indonesian Rupiah", DisplayNameI18n: localize("Indonesian Rupiah", "Rupiah Indonesia"), Scope: "deployment", UpdatedAt: seededAt, UpdatedBy: "system", Value: map[string]any{"currency_code": "IDR", "minor_unit_scale": 2, "display_symbol": "Rp"}},
				{TypeKey: "uom", Key: "ea", DisplayName: "Each", DisplayNameI18n: localize("Each", "Per Unit"), Scope: "deployment", UpdatedAt: seededAt, UpdatedBy: "system", Value: map[string]any{"uom_code": "ea", "dimension": "count", "precision_scale": 0}},
				{TypeKey: "party_type", Key: "patient", DisplayName: "Patient", DisplayNameI18n: localize("Patient", "Pasien"), Scope: "deployment", UpdatedAt: seededAt, UpdatedBy: "system", Value: map[string]any{"reference_type": "party_type"}},
				{TypeKey: "party_type", Key: "payer", DisplayName: "Payer", DisplayNameI18n: localize("Payer", "Penjamin"), Scope: "deployment", UpdatedAt: seededAt, UpdatedBy: "system", Value: map[string]any{"reference_type": "party_type"}},
				{TypeKey: "party_type", Key: "practitioner", DisplayName: "Practitioner", DisplayNameI18n: localize("Practitioner", "Praktisi"), Scope: "deployment", UpdatedAt: seededAt, UpdatedBy: "system", Value: map[string]any{"reference_type": "party_type"}},
				{TypeKey: "location_type", Key: "clinic", DisplayName: "Clinic", DisplayNameI18n: localize("Clinic", "Klinik"), Scope: "deployment", UpdatedAt: seededAt, UpdatedBy: "system", Value: map[string]any{"reference_type": "location_type"}},
				{TypeKey: "document_reason", Key: "walk_in", DisplayName: "Walk-In Visit", DisplayNameI18n: localize("Walk-In Visit", "Kunjungan Langsung"), Scope: "deployment", UpdatedAt: seededAt, UpdatedBy: "system", Value: map[string]any{"reference_type": "document_reason"}},
				{TypeKey: "document_reason", Key: "follow_up", DisplayName: "Follow Up", DisplayNameI18n: localize("Follow Up", "Kontrol Lanjutan"), Scope: "deployment", UpdatedAt: seededAt, UpdatedBy: "system", Value: map[string]any{"reference_type": "document_reason"}},
				{TypeKey: "appointment_type", Key: "consultation", DisplayName: "Consultation", DisplayNameI18n: localize("Consultation", "Konsultasi"), Scope: "deployment", UpdatedAt: seededAt, UpdatedBy: "system", Value: map[string]any{"reference_type": "appointment_type"}},
				{TypeKey: "patient_identifier_type", Key: "mrn", DisplayName: "Medical Record Number", DisplayNameI18n: localize("Medical Record Number", "Nomor Rekam Medis"), Scope: "deployment", UpdatedAt: seededAt, UpdatedBy: "system", Value: map[string]any{"reference_type": "patient_identifier_type"}},
				{TypeKey: "practitioner_type", Key: "doctor", DisplayName: "Doctor", DisplayNameI18n: localize("Doctor", "Dokter"), Scope: "deployment", UpdatedAt: seededAt, UpdatedBy: "system", Value: map[string]any{"reference_type": "practitioner_type"}},
				{TypeKey: "practitioner_type", Key: "nurse", DisplayName: "Nurse", DisplayNameI18n: localize("Nurse", "Perawat"), Scope: "deployment", UpdatedAt: seededAt, UpdatedBy: "system", Value: map[string]any{"reference_type": "practitioner_type"}},
				{TypeKey: "payer_type", Key: "self_pay", DisplayName: "Self Pay", DisplayNameI18n: localize("Self Pay", "Bayar Sendiri"), Scope: "deployment", UpdatedAt: seededAt, UpdatedBy: "system", Value: map[string]any{"reference_type": "payer_type"}},
				{TypeKey: "payer_type", Key: "insurance", DisplayName: "Insurance", DisplayNameI18n: localize("Insurance", "Asuransi"), Scope: "deployment", UpdatedAt: seededAt, UpdatedBy: "system", Value: map[string]any{"reference_type": "payer_type"}},
				{TypeKey: "visit_priority", Key: "routine", DisplayName: "Routine", DisplayNameI18n: localize("Routine", "Rutin"), Scope: "deployment", UpdatedAt: seededAt, UpdatedBy: "system", Value: map[string]any{"reference_type": "visit_priority"}},
				{TypeKey: "visit_priority", Key: "urgent", DisplayName: "Urgent", DisplayNameI18n: localize("Urgent", "Mendesak"), Scope: "deployment", UpdatedAt: seededAt, UpdatedBy: "system", Value: map[string]any{"reference_type": "visit_priority"}},
			},
			Offline: module.OfflineDefinition{
				References: []module.OfflineReferenceDefinition{
					{TypeKey: "appointment_type", Title: "Appointment Types", TitleI18n: localize("Appointment Types", "Jenis Janji Temu")},
					{TypeKey: "party_type", Title: "Party Types", TitleI18n: localize("Party Types", "Jenis Pihak")},
				},
			},
		},
		{
			Key:          "masterdata",
			Name:         "Master Data",
			NameI18n:     localize("Master Data", "Data Master"),
			Version:      "1.0.0",
			DomainFamily: "platform",
			DependencyRequirements: []module.DependencyRequirement{
				{ModuleKey: "platform.core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
				{ModuleKey: "reference_masterdata", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			},
			Models: []model.Definition{{
				Key:                 "party",
				DisplayName:         "Party",
				DisplayNameI18n:     localize("Party", "Pihak"),
				OwnerModuleKey:      "masterdata",
				Version:             "v1",
				CreatePermissionKey: "party.create",
				ListPermissionKey:   "party.list",
				ReadPermissionKey:   "party.read",
				UpdatePermissionKey: "party.update",
				DefaultSort:         "name",
				Fields: []model.FieldDefinition{
					{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Type: "string", Required: true},
					{Key: "display_name", Label: "Display Name", LabelI18n: localize("Display Name", "Nama Tampil"), Type: "string", ReadOnly: true, ComputeRuleKey: "party.display_name.compute"},
					{Key: "email", Label: "Email", LabelI18n: localize("Email", "Email"), Type: "string"},
					{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string", DefaultRuleKey: "party.status.default", ConstraintRuleKeys: []string{"party.status.allowed"}},
				},
				Relations: []model.RelationDefinition{
					{Key: "contacts", Type: "has_many", TargetModelKey: "party_contact", ForeignKey: "party_id"},
				},
			}, {
				Key:                 "party_contact",
				DisplayName:         "Party Contact",
				DisplayNameI18n:     localize("Party Contact", "Kontak Pihak"),
				OwnerModuleKey:      "masterdata",
				Version:             "v1",
				CreatePermissionKey: "party.update",
				ListPermissionKey:   "party.read",
				ReadPermissionKey:   "party.read",
				UpdatePermissionKey: "party.update",
				DefaultSort:         "name",
				Fields: []model.FieldDefinition{
					{Key: "party_id", Label: "Party ID", LabelI18n: localize("Party ID", "ID Pihak"), Type: "string", Required: true},
					{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Type: "string", Required: true},
					{Key: "phone", Label: "Phone", LabelI18n: localize("Phone", "Telepon"), Type: "string"},
					{Key: "role", Label: "Role", LabelI18n: localize("Role", "Peran"), Type: "string"},
				},
			}},
			Datasets: []module.DatasetDefinition{{
				Key:        "masterdata.party.summary",
				Title:      "Party Summary",
				TitleI18n:  localize("Party Summary", "Ringkasan Pihak"),
				SourceKind: "model",
				ModelKey:   "party",
				Dimensions: []module.DatasetDimension{{Key: "by_status", Label: "By Status", LabelI18n: localize("By Status", "Berdasarkan Status"), Path: "status"}},
				Measures:   []module.DatasetMeasure{{Key: "total", Label: "Total", LabelI18n: localize("Total", "Total"), Kind: "count"}},
			}},
			SearchIndexes: []search.IndexDefinition{{
				Key:                 "masterdata.party.search",
				Title:               "Party Search",
				SourceKind:          "model",
				ModelKey:            "party",
				ViewKey:             "masterdata.parties.list",
				Modes:               []string{"keyword", "vector", "hybrid"},
				OrganizationSplit:   true,
				RequiredPermissions: []string{"party.list"},
				QueryFilterFields:   []string{"status", "location_id"},
				QuerySortFields:     []string{"name", "updated_at"},
				Fields: []search.IndexFieldDefinition{
					{Key: "name", Path: "name", Type: "string", Searchable: true, Sort: true},
					{Key: "email", Path: "email", Type: "string", Searchable: true},
					{Key: "status", Path: "status", Type: "string", Facet: true, Sort: true},
				},
				VectorFields: []search.VectorFieldDefinition{{
					Key: "semantic", SourcePaths: []string{"name", "email"}, EmbeddingMode: "external", Dimensions: 8, DistanceMetric: "cosine",
				}},
			}},
			Security: module.SecurityDefinition{
				Permissions: []module.PermissionDefinition{
					{Key: "party.create", Action: "create", Resource: "party", DisplayName: "Create Parties", DisplayNameI18n: localize("Create Parties", "Buat Pihak")},
					{Key: "party.list", Action: "list", Resource: "party", DisplayName: "List Parties", DisplayNameI18n: localize("List Parties", "Daftar Pihak")},
					{Key: "party.read", Action: "read", Resource: "party", DisplayName: "Read Parties", DisplayNameI18n: localize("Read Parties", "Lihat Pihak")},
					{Key: "party.update", Action: "update", Resource: "party", DisplayName: "Update Parties", DisplayNameI18n: localize("Update Parties", "Perbarui Pihak")},
				},
				RoleTemplates: []module.RoleTemplateDefinition{{
					Key: "party_manager", Name: "Party Manager", NameI18n: localize("Party Manager", "Pengelola Pihak"), AllowedScopes: []string{"deployment", "location"}, PermissionKeys: []string{"party.create", "party.list", "party.read", "party.update"},
				}},
			},
			Frontend: module.FrontendDefinition{
				Menus: []module.MenuDefinition{{
					Key:                 "masterdata.parties",
					Label:               "Parties",
					LabelI18n:           localize("Parties", "Pihak"),
					ActionKey:           "masterdata.parties.list",
					Order:               5,
					RequiredPermissions: []string{"party.list"},
				}},
				Actions: []module.ActionDefinition{
					{Key: "masterdata.parties.list", Label: "Parties", LabelI18n: localize("Parties", "Pihak"), Kind: "navigate", RoutePath: "/masterdata/parties", ViewKey: "masterdata.parties.list", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"party.list"}},
					{Key: "masterdata.parties.detail", Label: "Party Detail", LabelI18n: localize("Party Detail", "Detail Pihak"), Kind: "navigate", RoutePath: "/masterdata/parties/detail", ViewKey: "masterdata.parties.detail", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"party.read"}},
					{Key: "masterdata.parties.form", Label: "Party Form", LabelI18n: localize("Party Form", "Form Pihak"), Kind: "navigate", RoutePath: "/masterdata/parties/form", ViewKey: "masterdata.parties.form", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"party.update"}},
				},
				Views: []module.ViewDefinition{
					{
						Key: "masterdata.parties.list", Title: "Parties", TitleI18n: localize("Parties", "Pihak"), Kind: "list", ModelKey: "party", RequiredPermissions: []string{"party.list"},
						Columns: []module.ColumnDefinition{
							{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name"},
							{Key: "email", Label: "Email", LabelI18n: localize("Email", "Email"), Path: "values.email"},
							{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status"},
						},
						Filters:         []module.FilterDefinition{{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "enum", Options: []string{"active", "inactive", "blocked"}}},
						DefaultPageSize: 10,
						EmptyState:      "No parties registered yet.",
						EmptyStateI18n:  localize("No parties registered yet.", "Belum ada pihak terdaftar."),
					},
					{
						Key: "masterdata.parties.detail", Title: "Party Detail", TitleI18n: localize("Party Detail", "Detail Pihak"), Kind: "detail", ModelKey: "party", RequiredPermissions: []string{"party.read"},
						Tabs: []module.TabDefinition{{
							Key: "summary", Title: "Summary", TitleI18n: localize("Summary", "Ringkasan"), Sections: []module.SectionDefinition{{
								Key: "core", Title: "Core Fields", TitleI18n: localize("Core Fields", "Field Utama"), Fields: []module.FieldDefinition{
									{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name", Type: "string"},
									{Key: "display_name", Label: "Display Name", LabelI18n: localize("Display Name", "Nama Tampil"), Path: "values.display_name", Type: "string"},
									{Key: "email", Label: "Email", LabelI18n: localize("Email", "Email"), Path: "values.email", Type: "string"},
									{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string"},
								},
							}},
						}},
						RelatedViews: []module.RelatedViewDefinition{
							{Key: "timeline", Title: "Timeline", TitleI18n: localize("Timeline", "Linimasa"), Source: "timeline", EmptyState: "No activity yet", EmptyStateI18n: localize("No activity yet", "Belum ada aktivitas")},
							{Key: "contacts", Title: "Contacts", TitleI18n: localize("Contacts", "Kontak"), Source: "contacts", EmptyState: "No related contacts", EmptyStateI18n: localize("No related contacts", "Belum ada kontak terkait")},
						},
					},
					{
						Key: "masterdata.parties.form", Title: "Party Form", TitleI18n: localize("Party Form", "Form Pihak"), Kind: "form", ModelKey: "party", RequiredPermissions: []string{"party.update"},
						Sections: []module.SectionDefinition{{
							Key: "edit", Title: "Edit Party", TitleI18n: localize("Edit Party", "Ubah Pihak"), Fields: []module.FieldDefinition{
								{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name", Type: "string", Widget: "text", Placeholder: "Party name", PlaceholderI18n: localize("Party name", "Nama pihak")},
								{Key: "email", Label: "Email", LabelI18n: localize("Email", "Email"), Path: "values.email", Type: "string", Widget: "text", Placeholder: "Email address", PlaceholderI18n: localize("Email address", "Alamat email")},
								{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string", Widget: "select", Options: []string{"active", "inactive", "blocked"}},
							},
						}},
						RelatedViews: []module.RelatedViewDefinition{
							{Key: "contacts", Title: "Contacts", TitleI18n: localize("Contacts", "Kontak"), Source: "contacts", EmptyState: "No related contacts", EmptyStateI18n: localize("No related contacts", "Belum ada kontak terkait")},
						},
					},
				},
			},
			Offline: module.OfflineDefinition{
				Projections: []module.OfflineProjectionDefinition{{
					IndexKey:             "masterdata.party.search",
					Title:                "Party Search",
					TitleI18n:            localize("Party Search", "Pencarian Pihak"),
					RequiredPermissions:  []string{"party.list"},
					DefaultIncludeFields: []string{"name", "email", "status"},
				}},
				Models: []module.OfflineModelDefinition{{
					ModelKey:            "party",
					Title:               "Party",
					TitleI18n:           localize("Party", "Pihak"),
					CreatePermissionKey: "party.create",
					UpdatePermissionKey: "party.update",
					RequiredPermissions: []string{"party.read"},
				}},
			},
			Templates: []module.TemplateDefinition{{
				Key:                 "documents.generic_request.default",
				Title:               "Generic Request Print",
				TitleI18n:           localize("Generic Request Print", "Cetak Permintaan Generik"),
				Description:         "Default printable request template.",
				DescriptionI18n:     localize("Default printable request template.", "Template cetak default untuk permintaan."),
				TargetKind:          "document",
				TargetKey:           "generic_request",
				RendererKind:        "html",
				DefaultFormat:       "html",
				Formats:             []string{"html", "pdf"},
				Purpose:             "official",
				Channel:             "print",
				AllowedScopes:       []string{"deployment", "organization", "location"},
				RequiredPermissions: []string{"template.read", "template.render"},
				DefaultBody: `<article class="print-sheet">
  <header>
    <h1>Generic Request</h1>
    <p>{{ .document.Header.Number }}</p>
  </header>
  <dl>
    <dt>Status</dt><dd>{{ .document.Header.Status }}</dd>
    <dt>Title</dt><dd>{{ index .document.Body.Payload "title" }}</dd>
    <dt>Organization</dt><dd>{{ .document.Header.OrganizationID }}</dd>
    <dt>Location</dt><dd>{{ .document.Header.LocationID }}</dd>
  </dl>
</article>`,
				DefaultStyle: `.print-sheet{font-family:Arial,sans-serif;padding:24px;color:#0f172a}.print-sheet h1{margin:0 0 8px}.print-sheet dl{display:grid;grid-template-columns:160px 1fr;gap:8px 12px}`,
			}},
		},
		{
			Key:                 "platform.core",
			Name:                "Platform Core",
			NameI18n:            localize("Platform Core", "Inti Platform"),
			Version:             "1.0.0",
			DomainFamily:        "platform",
			OwnedPermissionKeys: []string{"platform.context.read", "module.read", "module.manage", "configuration.read", "configuration.manage", "search.manage", "template.read", "template.manage", "template.publish", "template.bind", "template.render"},
			ConfigDefinitions:   []config.Definition{httpDefinition},
			Security: module.SecurityDefinition{
				Permissions: []module.PermissionDefinition{
					{Key: "platform.context.read", Action: "read", Resource: "context", DisplayName: "Read Platform Context", DisplayNameI18n: localize("Read Platform Context", "Lihat Konteks Platform")},
					{Key: "module.read", Action: "read", Resource: "module", DisplayName: "Read Modules", DisplayNameI18n: localize("Read Modules", "Lihat Modul")},
					{Key: "module.manage", Action: "manage", Resource: "module", DisplayName: "Manage Modules", DisplayNameI18n: localize("Manage Modules", "Kelola Modul"), RiskLevel: "high"},
					{Key: "configuration.read", Action: "read", Resource: "configuration", DisplayName: "Read Configuration", DisplayNameI18n: localize("Read Configuration", "Lihat Konfigurasi")},
					{Key: "configuration.manage", Action: "manage", Resource: "configuration", DisplayName: "Manage Configuration", DisplayNameI18n: localize("Manage Configuration", "Kelola Konfigurasi"), RiskLevel: "high"},
					{Key: "search.manage", Action: "manage", Resource: "search", DisplayName: "Manage Search Indexes", DisplayNameI18n: localize("Manage Search Indexes", "Kelola Indeks Pencarian"), RiskLevel: "high"},
					{Key: "template.read", Action: "read", Resource: "template", DisplayName: "Read Templates", DisplayNameI18n: localize("Read Templates", "Lihat Template")},
					{Key: "template.manage", Action: "manage", Resource: "template", DisplayName: "Manage Templates", DisplayNameI18n: localize("Manage Templates", "Kelola Template"), RiskLevel: "high"},
					{Key: "template.publish", Action: "publish", Resource: "template", DisplayName: "Publish Templates", DisplayNameI18n: localize("Publish Templates", "Publikasikan Template"), RiskLevel: "high"},
					{Key: "template.bind", Action: "bind", Resource: "template", DisplayName: "Bind Templates", DisplayNameI18n: localize("Bind Templates", "Atur Binding Template"), RiskLevel: "high"},
					{Key: "template.render", Action: "render", Resource: "template_output", DisplayName: "Render Template Outputs", DisplayNameI18n: localize("Render Template Outputs", "Render Output Template")},
				},
				RoleTemplates: []module.RoleTemplateDefinition{{
					Key:            "platform_operator",
					Name:           "Platform Operator",
					NameI18n:       localize("Platform Operator", "Operator Platform"),
					AllowedScopes:  []string{"deployment"},
					PermissionKeys: []string{"platform.context.read", "module.read", "configuration.read", "template.read", "template.render"},
				}},
			},
			Frontend: module.FrontendDefinition{
				Menus: []module.MenuDefinition{
					{
						Key:                 "admin.modules",
						Label:               "Modules",
						LabelI18n:           localize("Modules", "Modul"),
						ActionKey:           "admin.modules",
						Order:               10,
						Surface:             module.UISurfaceAdmin,
						RequiredPermissions: []string{"module.read"},
					},
					{
						Key:                 "admin.auth",
						Label:               "Authentication",
						LabelI18n:           localize("Authentication", "Autentikasi"),
						ActionKey:           "admin.auth",
						Order:               20,
						Surface:             module.UISurfaceAdmin,
						RequiredPermissions: []string{"configuration.read"},
					},
					{
						Key:                 "admin.configuration",
						Label:               "Configuration",
						LabelI18n:           localize("Configuration", "Konfigurasi"),
						ActionKey:           "admin.configuration",
						Order:               30,
						Surface:             module.UISurfaceAdmin,
						RequiredPermissions: []string{"configuration.read"},
					},
					{
						Key:                 "admin.definitions",
						Label:               "Definitions",
						LabelI18n:           localize("Definitions", "Definisi"),
						ActionKey:           "admin.definitions",
						Order:               40,
						Surface:             module.UISurfaceAdmin,
						RequiredPermissions: []string{"configuration.read"},
					},
					{
						Key:                 "admin.templates",
						Label:               "Templates",
						LabelI18n:           localize("Templates", "Template"),
						ActionKey:           "admin.templates",
						Order:               45,
						Surface:             module.UISurfaceAdmin,
						RequiredPermissions: []string{"template.read"},
					},
					{
						Key:                 "admin.security",
						Label:               "Security",
						LabelI18n:           localize("Security", "Keamanan"),
						ActionKey:           "admin.security",
						Order:               50,
						Surface:             module.UISurfaceAdmin,
						RequiredPermissions: []string{"configuration.read"},
					},
					{
						Key:                 "admin.observability",
						Label:               "Observability",
						LabelI18n:           localize("Observability", "Observabilitas"),
						ActionKey:           "admin.observability",
						Order:               60,
						Surface:             module.UISurfaceAdmin,
						RequiredPermissions: []string{"module.read"},
					},
				},
				Actions: []module.ActionDefinition{
					{
						Key:                 "admin.modules",
						Label:               "Modules",
						LabelI18n:           localize("Modules", "Modul"),
						Kind:                "navigate",
						RoutePath:           "/admin/modules",
						Surface:             module.UISurfaceAdmin,
						RequiredPermissions: []string{"module.read"},
					},
					{
						Key:                 "admin.auth",
						Label:               "Authentication",
						LabelI18n:           localize("Authentication", "Autentikasi"),
						Kind:                "navigate",
						RoutePath:           "/admin/auth",
						Surface:             module.UISurfaceAdmin,
						RequiredPermissions: []string{"configuration.read"},
					},
					{
						Key:                 "admin.configuration",
						Label:               "Configuration",
						LabelI18n:           localize("Configuration", "Konfigurasi"),
						Kind:                "navigate",
						RoutePath:           "/admin/config",
						Surface:             module.UISurfaceAdmin,
						RequiredPermissions: []string{"configuration.read"},
					},
					{
						Key:                 "admin.definitions",
						Label:               "Definitions",
						LabelI18n:           localize("Definitions", "Definisi"),
						Kind:                "navigate",
						RoutePath:           "/admin/definitions",
						Surface:             module.UISurfaceAdmin,
						RequiredPermissions: []string{"configuration.read"},
					},
					{
						Key:                 "admin.templates",
						Label:               "Templates",
						LabelI18n:           localize("Templates", "Template"),
						Kind:                "navigate",
						RoutePath:           "/admin/templates",
						Surface:             module.UISurfaceAdmin,
						RequiredPermissions: []string{"template.read"},
					},
					{
						Key:                 "admin.security",
						Label:               "Security",
						LabelI18n:           localize("Security", "Keamanan"),
						Kind:                "navigate",
						RoutePath:           "/admin/security",
						Surface:             module.UISurfaceAdmin,
						RequiredPermissions: []string{"configuration.read"},
					},
					{
						Key:                 "admin.observability",
						Label:               "Observability",
						LabelI18n:           localize("Observability", "Observabilitas"),
						Kind:                "navigate",
						RoutePath:           "/admin/observability",
						Surface:             module.UISurfaceAdmin,
						RequiredPermissions: []string{"module.read"},
					},
				},
			},
			Observability: module.ObservabilityDefinition{
				Metrics: []module.MetricDefinition{
					{Key: "http.requests.total", Type: "counter", Description: "Total HTTP requests"},
					{Key: "http.request.duration", Type: "timing", Description: "HTTP request duration"},
				},
				LogEvents: []module.LogEventDefinition{{
					Key: "http.request.completed", Category: "http", Severity: "info", RequiredFields: []string{"correlation_id", "method", "path", "status"},
				}},
			},
		},
		{
			Key:          "identity",
			Name:         "Identity and Access",
			NameI18n:     localize("Identity and Access", "Identitas dan Akses"),
			Version:      "1.0.0",
			DomainFamily: "platform",
			DependencyRequirements: []module.DependencyRequirement{{
				ModuleKey: "platform.core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired,
			}},
			OwnedPermissionKeys: []string{"identity.manage_sessions", "identity.manage_users"},
			ConfigDefinitions:   []config.Definition{authDefinition},
			Security: module.SecurityDefinition{
				Permissions: []module.PermissionDefinition{
					{Key: "identity.manage_sessions", Action: "manage", Resource: "session", DisplayName: "Manage Sessions", DisplayNameI18n: localize("Manage Sessions", "Kelola Sesi"), RiskLevel: "high"},
					{Key: "identity.manage_users", Action: "manage", Resource: "user", DisplayName: "Manage Users", DisplayNameI18n: localize("Manage Users", "Kelola Pengguna"), RiskLevel: "high"},
				},
			},
		},
		{
			Key:          "documents",
			Name:         "Document Kernel",
			NameI18n:     localize("Document Kernel", "Kernel Dokumen"),
			Version:      "1.1.0",
			DomainFamily: "platform",
			DependencyRequirements: []module.DependencyRequirement{
				{ModuleKey: "platform.core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
				{ModuleKey: "identity", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			},
			OwnedDocumentTypes: []string{"generic_request"},
			OwnedWorkflowKeys:  []string{"generic_request_flow"},
			Documents: []document.Definition{{
				Type:                   "generic_request",
				DisplayName:            "Generic Request",
				SchemaVersion:          "v1",
				WorkflowKey:            "generic_request_flow",
				NumberingKey:           "generic_request_number",
				OwnerModuleKey:         "documents",
				AllowedLinkTypes:       []string{"related_to", "amends"},
				AllowedAttachmentTypes: []string{"note", "image", "document"},
			}},
			Workflows: []workflow.Definition{{
				Key:    "generic_request_flow",
				States: []string{"draft", "submitted", "approved", "rejected", "cancelled"},
				Actions: []workflow.ActionRule{
					{Action: "submit", FromState: "draft", ToState: "submitted", PermissionKey: "document.submit", TaskType: "review", CreateApproval: true},
					{Action: "approve", FromState: "submitted", ToState: "approved", PermissionKey: "document.approve"},
					{Action: "reject", FromState: "submitted", ToState: "rejected", PermissionKey: "document.reject"},
					{Action: "reopen", FromState: "rejected", ToState: "draft", PermissionKey: "document.reopen"},
					{Action: "reopen", FromState: "approved", ToState: "draft", PermissionKey: "document.reopen"},
					{Action: "cancel", FromState: "draft", ToState: "cancelled", PermissionKey: "document.cancel"},
					{Action: "cancel", FromState: "submitted", ToState: "cancelled", PermissionKey: "document.cancel"},
				},
			}},
			Security: module.SecurityDefinition{
				Permissions: []module.PermissionDefinition{
					{Key: "document.create", Action: "create", Resource: "document", DisplayName: "Create Documents", DisplayNameI18n: localize("Create Documents", "Buat Dokumen")},
					{Key: "document.list", Action: "list", Resource: "document", DisplayName: "List Documents", DisplayNameI18n: localize("List Documents", "Daftar Dokumen")},
					{Key: "document.read", Action: "read", Resource: "document", DisplayName: "Read Documents", DisplayNameI18n: localize("Read Documents", "Lihat Dokumen")},
					{Key: "document.update_draft", Action: "update_draft", Resource: "document", DisplayName: "Update Draft Documents", DisplayNameI18n: localize("Update Draft Documents", "Perbarui Dokumen Draf")},
					{Key: "document.submit", Action: "submit", Resource: "document", DisplayName: "Submit Documents", DisplayNameI18n: localize("Submit Documents", "Ajukan Dokumen")},
					{Key: "document.approve", Action: "approve", Resource: "document", DisplayName: "Approve Documents", DisplayNameI18n: localize("Approve Documents", "Setujui Dokumen"), RiskLevel: "medium"},
					{Key: "document.reject", Action: "reject", Resource: "document", DisplayName: "Reject Documents", DisplayNameI18n: localize("Reject Documents", "Tolak Dokumen"), RiskLevel: "medium"},
					{Key: "document.reopen", Action: "reopen", Resource: "document", DisplayName: "Reopen Documents", DisplayNameI18n: localize("Reopen Documents", "Buka Ulang Dokumen"), RiskLevel: "high"},
					{Key: "document.cancel", Action: "cancel", Resource: "document", DisplayName: "Cancel Documents", DisplayNameI18n: localize("Cancel Documents", "Batalkan Dokumen"), RiskLevel: "high"},
				},
				RoleTemplates: []module.RoleTemplateDefinition{
					{
						Key:            "document_clerk",
						Name:           "Document Clerk",
						NameI18n:       localize("Document Clerk", "Petugas Dokumen"),
						AllowedScopes:  []string{"location"},
						PermissionKeys: []string{"document.create", "document.list", "document.read", "document.update_draft", "document.submit"},
					},
					{
						Key:            "document_reviewer",
						Name:           "Document Reviewer",
						NameI18n:       localize("Document Reviewer", "Peninjau Dokumen"),
						AllowedScopes:  []string{"location"},
						PermissionKeys: []string{"document.list", "document.read", "document.approve", "document.reject", "document.reopen", "document.cancel"},
					},
				},
				PolicyHooks: []module.PolicyHookDefinition{
					{Key: "documents.extension.view", Kind: "access", Target: "document_extension_view", InputContractKey: "document.extension.access.v1", OutputContractKey: "decision.v1", Description: "Controls extension visibility in expanded/raw document reads."},
					{Key: "documents.extension.write", Kind: "access", Target: "document_extension_write", InputContractKey: "document.extension.access.v1", OutputContractKey: "decision.v1", Description: "Controls extension writes for draft documents."},
					{Key: "documents.workflow.transition", Kind: "workflow", Target: "document_transition", InputContractKey: "document.transition.v1", OutputContractKey: "decision.v1", Description: "Controls workflow transitions before they are committed."},
					{Key: "documents.workflow.assignment", Kind: "workflow", Target: "document_assignment", InputContractKey: "document.assignment.v1", OutputContractKey: "decision.v1", Description: "Controls workflow assignment before side effects are persisted."},
					{Key: "documents.workflow.sla", Kind: "workflow", Target: "document_sla", InputContractKey: "document.sla.v1", OutputContractKey: "decision.v1", Description: "Controls workflow task due and escalation timing before side effects are persisted."},
					{Key: "documents.search.visibility", Kind: "search", Target: "document_search", InputContractKey: "document.search.v1", OutputContractKey: "decision.v1", Description: "Controls document visibility in search and list views."},
					{Key: "documents.numbering.assign", Kind: "numbering", Target: "document_numbering", InputContractKey: "document.numbering.v1", OutputContractKey: "decision.v1", Description: "Assigns document numbers when numbering is policy-bound."},
					{Key: "documents.action.render", Kind: "ui", Target: "document_action_render", InputContractKey: "document.action.render.v1", OutputContractKey: "decision.v1", Description: "Controls action placement and visibility in generic detail views."},
				},
			},
			Observability: module.ObservabilityDefinition{
				Projections: []module.ProjectionDefinition{{
					Key: "document_summary", SourceEventTypes: []string{"document.updated", "document.submitted", "document.approved", "document.reject", "document.reopened", "document.cancelled"}, RefreshMode: "event_driven",
				}},
				DomainEvents: []module.DomainEventDefinition{
					{Type: "document.updated", Role: "producer", CorrelationRequired: true},
					{Type: "document.submitted", Role: "producer", CorrelationRequired: true, ExternalPublish: true, Topic: "documents.lifecycle.submitted", SchemaVersion: "v1"},
					{Type: "document.approved", Role: "producer", CorrelationRequired: true, ExternalPublish: true, Topic: "documents.lifecycle.approved", SchemaVersion: "v1"},
					{Type: "document.reject", Role: "producer", CorrelationRequired: true},
					{Type: "document.reopened", Role: "producer", CorrelationRequired: true},
					{Type: "document.cancelled", Role: "producer", CorrelationRequired: true},
					{Type: "document.extension.updated", Role: "producer", CorrelationRequired: true},
				},
				Metrics: []module.MetricDefinition{
					{Key: "document.actions.total", Type: "counter", Labels: []string{"action", "outcome"}},
				},
				LogEvents: []module.LogEventDefinition{{
					Key: "document.action", Category: "document", Severity: "info", RequiredFields: []string{"document_id", "action", "actor_id"},
				}},
			},
			SelfService: module.SelfServiceDefinition{
				APIs: []module.SelfServiceAPIDefinition{
					{
						Key:                 "documents.self_service.requests.list",
						Title:               "List Self-Service Requests",
						TitleI18n:           localize("List Self-Service Requests", "Daftar Permintaan Self-Service"),
						Method:              "GET",
						RoutePath:           "/ui/data/documents?type=generic_request",
						HandlerKind:         "ui_data",
						DocumentType:        "generic_request",
						RequiredPermissions: []string{"document.list"},
						AudienceKinds:       []string{"customer", "employee", "patient"},
						ResponseContractKey: "documents.generic_request.list",
					},
					{
						Key:                 "documents.self_service.requests.get",
						Title:               "Get Self-Service Request",
						TitleI18n:           localize("Get Self-Service Request", "Lihat Permintaan Self-Service"),
						Method:              "GET",
						RoutePath:           "/ui/data/documents/{documentID}",
						HandlerKind:         "ui_data",
						DocumentType:        "generic_request",
						RequiredPermissions: []string{"document.read"},
						AudienceKinds:       []string{"customer", "employee", "patient"},
						ResponseContractKey: "documents.generic_request.detail",
					},
					{
						Key:                 "documents.self_service.requests.create",
						Title:               "Create Self-Service Request",
						TitleI18n:           localize("Create Self-Service Request", "Buat Permintaan Self-Service"),
						Method:              "POST",
						RoutePath:           "/document-flows/documents.self_service.requests.intake/commit",
						HandlerKind:         "flow_commit",
						DocumentType:        "generic_request",
						FlowKey:             "documents.self_service.requests.intake",
						RequiredPermissions: []string{"document.create"},
						AudienceKinds:       []string{"customer", "employee", "patient"},
						RequestContractKey:  "documents.generic_request.create",
						ResponseContractKey: "documents.generic_request.detail",
						Idempotent:          true,
					},
					{
						Key:                 "documents.self_service.requests.submit",
						Title:               "Submit Self-Service Request",
						TitleI18n:           localize("Submit Self-Service Request", "Ajukan Permintaan Self-Service"),
						Method:              "POST",
						RoutePath:           "/documents/{documentID}/actions",
						HandlerKind:         "document_action",
						DocumentType:        "generic_request",
						RequiredPermissions: []string{"document.submit"},
						AudienceKinds:       []string{"customer", "employee", "patient"},
						RequestContractKey:  "documents.generic_request.action.submit",
						ResponseContractKey: "documents.generic_request.detail",
						Idempotent:          true,
					},
				},
			},
			Frontend: module.FrontendDefinition{
				Menus: []module.MenuDefinition{{
					Key:                 "documents.requests",
					Label:               "Requests",
					LabelI18n:           localize("Requests", "Permintaan"),
					ActionKey:           "documents.requests.list",
					Order:               10,
					RequiredPermissions: []string{"document.list"},
				}, {
					Key:                 "documents.self_service.requests",
					Label:               "My Requests",
					LabelI18n:           localize("My Requests", "Permintaan Saya"),
					Surface:             module.UISurfaceSelfService,
					ActionKey:           "documents.self_service.requests.list",
					Order:               8,
					RequiredPermissions: []string{"document.list"},
				}, {
					Key:                 "documents.worklist",
					Label:               "Worklist",
					LabelI18n:           localize("Worklist", "Antrian Kerja"),
					Surface:             module.UISurfaceWorklist,
					ActionKey:           "documents.worklist.tasks",
					Order:               5,
					RequiredPermissions: []string{"document.list"},
				}},
				Actions: []module.ActionDefinition{
					{
						Key:                 "documents.requests.create",
						Label:               "New Request",
						LabelI18n:           localize("New Request", "Permintaan Baru"),
						Kind:                "navigate",
						RoutePath:           "/documents/new",
						FlowKey:             "documents.requests.intake",
						RenderMode:          module.RenderModeFlow,
						RequiredPermissions: []string{"document.create"},
					},
					{
						Key:                 "documents.requests.list",
						Label:               "Requests",
						LabelI18n:           localize("Requests", "Permintaan"),
						Kind:                "navigate",
						RoutePath:           "/documents",
						ViewKey:             "documents.requests.list",
						RenderMode:          module.RenderModeGeneric,
						RequiredPermissions: []string{"document.list"},
					},
					{
						Key:                 "documents.requests.detail",
						Label:               "Request Detail",
						LabelI18n:           localize("Request Detail", "Detail Permintaan"),
						Kind:                "navigate",
						RoutePath:           "/documents/detail",
						ViewKey:             "documents.requests.detail",
						RenderMode:          module.RenderModeGeneric,
						RequiredPermissions: []string{"document.read"},
					},
					{
						Key:                 "documents.self_service.requests.create",
						Label:               "New Request",
						LabelI18n:           localize("New Request", "Permintaan Baru"),
						Surface:             module.UISurfaceSelfService,
						Kind:                "navigate",
						RoutePath:           "/self-service/requests/new",
						FlowKey:             "documents.self_service.requests.intake",
						RenderMode:          module.RenderModeFlow,
						RequiredPermissions: []string{"document.create"},
					},
					{
						Key:                 "documents.self_service.requests.list",
						Label:               "My Requests",
						LabelI18n:           localize("My Requests", "Permintaan Saya"),
						Surface:             module.UISurfaceSelfService,
						Kind:                "navigate",
						RoutePath:           "/self-service/requests",
						ViewKey:             "documents.self_service.requests.list",
						RenderMode:          module.RenderModeGeneric,
						RequiredPermissions: []string{"document.list"},
					},
					{
						Key:                 "documents.self_service.requests.detail",
						Label:               "Request Detail",
						LabelI18n:           localize("Request Detail", "Detail Permintaan"),
						Surface:             module.UISurfaceSelfService,
						Kind:                "navigate",
						RoutePath:           "/self-service/requests/detail",
						ViewKey:             "documents.self_service.requests.detail",
						RenderMode:          module.RenderModeGeneric,
						RequiredPermissions: []string{"document.read"},
					},
					{
						Key:                 "documents.self_service.requests.form",
						Label:               "Request Draft",
						LabelI18n:           localize("Request Draft", "Draf Permintaan"),
						Surface:             module.UISurfaceSelfService,
						Kind:                "navigate",
						RoutePath:           "/self-service/requests/form",
						ViewKey:             "documents.self_service.requests.form",
						RenderMode:          module.RenderModeGeneric,
						RequiredPermissions: []string{"document.update_draft"},
					},
					{
						Key:                 "documents.worklist.tasks",
						Label:               "Task Queue",
						LabelI18n:           localize("Task Queue", "Antrian Tugas"),
						Surface:             module.UISurfaceWorklist,
						Kind:                "navigate",
						RoutePath:           "/worklist",
						ViewKey:             "documents.worklist.tasks",
						RenderMode:          module.RenderModeGeneric,
						RequiredPermissions: []string{"document.list"},
					},
					{
						Key:                 "documents.worklist.approvals",
						Label:               "Approval Queue",
						LabelI18n:           localize("Approval Queue", "Antrian Persetujuan"),
						Surface:             module.UISurfaceWorklist,
						Kind:                "navigate",
						RoutePath:           "/worklist/approvals",
						ViewKey:             "documents.worklist.approvals",
						RenderMode:          module.RenderModeGeneric,
						RequiredPermissions: []string{"document.list"},
					},
				},
				Views: []module.ViewDefinition{
					{
						Key:                 "documents.requests.list",
						Title:               "Requests",
						TitleI18n:           localize("Requests", "Permintaan"),
						Kind:                "list",
						DocumentType:        "generic_request",
						ProjectionKey:       "document_summary",
						RequiredPermissions: []string{"document.list"},
						Columns: []module.ColumnDefinition{
							{Key: "id", Label: "Document", LabelI18n: localize("Document", "Dokumen"), Path: "header.id"},
							{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "header.status"},
							{Key: "updated_at", Label: "Updated", LabelI18n: localize("Updated", "Diperbarui"), Path: "header.updated_at"},
						},
						Filters: []module.FilterDefinition{
							{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "enum", Options: []string{"draft", "submitted", "approved", "rejected", "cancelled"}},
						},
						DefaultPageSize: 10,
						EmptyState:      "No governed request documents exist yet.",
						EmptyStateI18n:  localize("No governed request documents exist yet.", "Belum ada dokumen permintaan yang dikelola."),
					},
					{
						Key:                 "documents.requests.detail",
						Title:               "Request Detail",
						TitleI18n:           localize("Request Detail", "Detail Permintaan"),
						Kind:                "detail",
						DocumentType:        "generic_request",
						Printable:           true,
						PrintPurpose:        "official",
						PrintChannel:        "print",
						RequiredPermissions: []string{"document.read"},
						AllowedActions:      []string{"submit", "approve", "reject", "reopen", "cancel"},
						Tabs: []module.TabDefinition{
							{
								Key:       "summary",
								Title:     "Summary",
								TitleI18n: localize("Summary", "Ringkasan"),
								Sections: []module.SectionDefinition{
									{
										Key:       "header",
										Title:     "Header",
										TitleI18n: localize("Header", "Header"),
										Fields: []module.FieldDefinition{
											{Key: "doc_id", Label: "Document ID", LabelI18n: localize("Document ID", "ID Dokumen"), Path: "header.id", Type: "string", ReadOnly: true},
											{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "header.status", Type: "string", ReadOnly: true},
											{Key: "updated_by", Label: "Updated By", LabelI18n: localize("Updated By", "Diperbarui Oleh"), Path: "header.updated_by", Type: "string", ReadOnly: true},
										},
									},
									{
										Key:       "payload",
										Title:     "Payload",
										TitleI18n: localize("Payload", "Payload"),
										Fields: []module.FieldDefinition{
											{Key: "title", Label: "Title", LabelI18n: localize("Title", "Judul"), Path: "body.payload.title", Type: "string"},
										},
									},
								},
							},
							{
								Key:       "extensions",
								Title:     "Extensions",
								TitleI18n: localize("Extensions", "Ekstensi"),
								Sections: []module.SectionDefinition{{
									Key: "analytics_extension", Title: "Analytics Extension", TitleI18n: localize("Analytics Extension", "Ekstensi Analitik"), ExtensionSlotKey: "analytics",
								}},
							},
						},
						RelatedViews: []module.RelatedViewDefinition{
							{Key: "lines", Title: "Lines", TitleI18n: localize("Lines", "Baris"), Source: "lines", EmptyState: "No lines", EmptyStateI18n: localize("No lines", "Belum ada baris")},
							{Key: "links", Title: "Links", TitleI18n: localize("Links", "Tautan"), Source: "links", EmptyState: "No links", EmptyStateI18n: localize("No links", "Belum ada tautan")},
							{Key: "attachments", Title: "Attachments", TitleI18n: localize("Attachments", "Lampiran"), Source: "attachments", EmptyState: "No attachments", EmptyStateI18n: localize("No attachments", "Belum ada lampiran")},
						},
						ActionPlacements: []module.ActionPlacementDefinition{
							{ActionKey: "submit", Zone: "primary"},
							{ActionKey: "approve", Zone: "primary"},
							{ActionKey: "reject", Zone: "secondary", Style: "warn"},
							{ActionKey: "reopen", Zone: "secondary"},
							{ActionKey: "cancel", Zone: "secondary", Style: "warn"},
						},
					},
					{
						Key:                 "documents.self_service.requests.list",
						Title:               "My Requests",
						TitleI18n:           localize("My Requests", "Permintaan Saya"),
						Surface:             module.UISurfaceSelfService,
						Kind:                "list",
						DocumentType:        "generic_request",
						ProjectionKey:       "document_summary",
						RequiredPermissions: []string{"document.list"},
						Columns: []module.ColumnDefinition{
							{Key: "id", Label: "Request", LabelI18n: localize("Request", "Permintaan"), Path: "header.id"},
							{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "header.status"},
							{Key: "updated_at", Label: "Updated", LabelI18n: localize("Updated", "Diperbarui"), Path: "header.updated_at"},
						},
						Filters: []module.FilterDefinition{
							{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "enum", Options: []string{"draft", "submitted", "approved", "rejected", "cancelled"}},
						},
						DefaultPageSize: 10,
						EmptyState:      "No self-service requests yet.",
						EmptyStateI18n:  localize("No self-service requests yet.", "Belum ada permintaan self-service."),
					},
					{
						Key:                 "documents.self_service.requests.detail",
						Title:               "My Request",
						TitleI18n:           localize("My Request", "Permintaan Saya"),
						Surface:             module.UISurfaceSelfService,
						Kind:                "detail",
						DocumentType:        "generic_request",
						RequiredPermissions: []string{"document.read"},
						AllowedActions:      []string{"submit", "reopen", "cancel"},
						Tabs: []module.TabDefinition{
							{
								Key:       "summary",
								Title:     "Summary",
								TitleI18n: localize("Summary", "Ringkasan"),
								Sections: []module.SectionDefinition{
									{
										Key:       "header",
										Title:     "Header",
										TitleI18n: localize("Header", "Header"),
										Fields: []module.FieldDefinition{
											{Key: "doc_id", Label: "Request ID", LabelI18n: localize("Request ID", "ID Permintaan"), Path: "header.id", Type: "string", ReadOnly: true},
											{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "header.status", Type: "string", ReadOnly: true},
										},
									},
									{
										Key:       "payload",
										Title:     "Payload",
										TitleI18n: localize("Payload", "Payload"),
										Fields: []module.FieldDefinition{
											{Key: "title", Label: "Title", LabelI18n: localize("Title", "Judul"), Path: "body.payload.title", Type: "string", ReadOnly: true},
										},
									},
								},
							},
						},
						ActionPlacements: []module.ActionPlacementDefinition{
							{ActionKey: "submit", Zone: "primary"},
							{ActionKey: "reopen", Zone: "secondary"},
							{ActionKey: "cancel", Zone: "secondary", Style: "warn"},
						},
					},
					{
						Key:                 "documents.self_service.requests.form",
						Title:               "Request Form",
						TitleI18n:           localize("Request Form", "Form Permintaan"),
						Surface:             module.UISurfaceSelfService,
						Kind:                "form",
						DocumentType:        "generic_request",
						RequiredPermissions: []string{"document.update_draft"},
						Sections: []module.SectionDefinition{{
							Key: "request_fields", Title: "Request", TitleI18n: localize("Request", "Permintaan"), Fields: []module.FieldDefinition{
								{Key: "title", Label: "Title", LabelI18n: localize("Title", "Judul"), Path: "body.payload.title", Type: "string", Widget: "textarea", Placeholder: "Describe your request", PlaceholderI18n: localize("Describe your request", "Jelaskan permintaan Anda")},
							},
						}},
					},
					{
						Key:                 "documents.worklist.tasks",
						Title:               "Task Queue",
						TitleI18n:           localize("Task Queue", "Antrian Tugas"),
						Surface:             module.UISurfaceWorklist,
						Kind:                "queue",
						ProjectionKey:       "workflow.tasks",
						RequiredPermissions: []string{"document.list"},
						EmptyState:          "No open workflow tasks.",
						EmptyStateI18n:      localize("No open workflow tasks.", "Belum ada tugas workflow terbuka."),
					},
					{
						Key:                 "documents.worklist.approvals",
						Title:               "Approval Queue",
						TitleI18n:           localize("Approval Queue", "Antrian Persetujuan"),
						Surface:             module.UISurfaceWorklist,
						Kind:                "queue",
						ProjectionKey:       "workflow.approvals",
						RequiredPermissions: []string{"document.list"},
						EmptyState:          "No pending workflow approvals.",
						EmptyStateI18n:      localize("No pending workflow approvals.", "Belum ada persetujuan workflow tertunda."),
					},
					{
						Key:                 "documents.requests.form",
						Title:               "Request Draft Form",
						TitleI18n:           localize("Request Draft Form", "Form Draf Permintaan"),
						Kind:                "form",
						DocumentType:        "generic_request",
						RequiredPermissions: []string{"document.update_draft"},
						Fields: []module.FieldDefinition{
							{Key: "title", Label: "Title", LabelI18n: localize("Title", "Judul"), Path: "body.payload.title", Type: "string", Widget: "text", Placeholder: "Enter request title", PlaceholderI18n: localize("Enter request title", "Masukkan judul permintaan"), HelpText: "Short summary used in lists.", HelpTextI18n: localize("Short summary used in lists.", "Ringkasan singkat yang dipakai di daftar.")},
						},
						Sections: []module.SectionDefinition{{
							Key: "draft_fields", Title: "Draft Fields", TitleI18n: localize("Draft Fields", "Field Draf"), Fields: []module.FieldDefinition{
								{Key: "title", Label: "Title", LabelI18n: localize("Title", "Judul"), Path: "body.payload.title", Type: "string", Widget: "textarea", Placeholder: "Describe the request", PlaceholderI18n: localize("Describe the request", "Jelaskan permintaan")},
							},
						}},
					},
				},
				DocumentFlows: []module.DocumentFlowDefinition{{
					Key:                 "documents.requests.intake",
					Title:               "Request Intake",
					TitleI18n:           localize("Request Intake", "Intake Permintaan"),
					RoutePath:           "/documents/new",
					PrimaryDocumentType: "generic_request",
					RequiredPermissions: []string{"document.create"},
					Steps: []module.DocumentFlowStepDefinition{
						{
							Key:       "intake",
							Title:     "Primary Request",
							TitleI18n: localize("Primary Request", "Permintaan Utama"),
							Documents: []module.DocumentFlowDocumentDefinition{{
								Key:           "request",
								Title:         "Request",
								TitleI18n:     localize("Request", "Permintaan"),
								DocumentType:  "generic_request",
								PrimaryOutput: true,
								Sections: []module.SectionDefinition{{
									Key: "request_core", Title: "Request Details", TitleI18n: localize("Request Details", "Detail Permintaan"), Fields: []module.FieldDefinition{
										{Key: "title", Label: "Title", LabelI18n: localize("Title", "Judul"), Path: "body.payload.title", Type: "string", Widget: "text", Placeholder: "Enter request title", PlaceholderI18n: localize("Enter request title", "Masukkan judul permintaan")},
										{Key: "request_kind", Label: "Request Kind", LabelI18n: localize("Request Kind", "Jenis Permintaan"), Path: "body.payload.request_kind", Type: "string", Widget: "select", Options: []string{"review", "followup"}},
									},
								}},
							}},
							NextRules: []module.DocumentFlowBranchRule{
								{Path: "documents.request.payload.request_kind", Equals: "review", NextStepKey: "review_packet"},
								{Path: "documents.request.payload.request_kind", Equals: "followup", NextStepKey: "followup_plan"},
							},
						},
						{
							Key:       "review_packet",
							Title:     "Review Packet",
							TitleI18n: localize("Review Packet", "Paket Tinjauan"),
							Documents: []module.DocumentFlowDocumentDefinition{
								{
									Key:          "review_note",
									Title:        "Review Note",
									TitleI18n:    localize("Review Note", "Catatan Tinjauan"),
									DocumentType: "generic_request",
									LinkType:     "related_to",
									Sections: []module.SectionDefinition{{
										Key: "review_note", Title: "Review Note", TitleI18n: localize("Review Note", "Catatan Tinjauan"), Fields: []module.FieldDefinition{
											{Key: "title", Label: "Title", LabelI18n: localize("Title", "Judul"), Path: "body.payload.title", Type: "string", Widget: "text"},
											{Key: "notes", Label: "Notes", LabelI18n: localize("Notes", "Catatan"), Path: "body.payload.notes", Type: "string", Widget: "textarea"},
										},
									}},
								},
								{
									Key:          "review_checklist",
									Title:        "Review Checklist",
									TitleI18n:    localize("Review Checklist", "Daftar Periksa Tinjauan"),
									DocumentType: "generic_request",
									LinkType:     "related_to",
									Sections: []module.SectionDefinition{{
										Key: "review_checklist", Title: "Checklist", TitleI18n: localize("Checklist", "Daftar Periksa"), Fields: []module.FieldDefinition{
											{Key: "title", Label: "Title", LabelI18n: localize("Title", "Judul"), Path: "body.payload.title", Type: "string", Widget: "text"},
											{Key: "owner", Label: "Owner", LabelI18n: localize("Owner", "Penanggung Jawab"), Path: "body.payload.owner", Type: "string", Widget: "text"},
										},
									}},
								},
							},
						},
						{
							Key:       "followup_plan",
							Title:     "Follow-up Plan",
							TitleI18n: localize("Follow-up Plan", "Rencana Tindak Lanjut"),
							Documents: []module.DocumentFlowDocumentDefinition{{
								Key:          "followup_plan",
								Title:        "Follow-up Plan",
								TitleI18n:    localize("Follow-up Plan", "Rencana Tindak Lanjut"),
								DocumentType: "generic_request",
								LinkType:     "related_to",
								Sections: []module.SectionDefinition{{
									Key: "followup_plan", Title: "Plan", TitleI18n: localize("Plan", "Rencana"), Fields: []module.FieldDefinition{
										{Key: "title", Label: "Title", LabelI18n: localize("Title", "Judul"), Path: "body.payload.title", Type: "string", Widget: "text"},
										{Key: "next_action", Label: "Next Action", LabelI18n: localize("Next Action", "Tindak Lanjut"), Path: "body.payload.next_action", Type: "string", Widget: "textarea"},
									},
								}},
							}},
						},
					},
				}, {
					Key:                 "documents.self_service.requests.intake",
					Title:               "Self-Service Request",
					TitleI18n:           localize("Self-Service Request", "Permintaan Self-Service"),
					Surface:             module.UISurfaceSelfService,
					RoutePath:           "/self-service/requests/new",
					PrimaryDocumentType: "generic_request",
					RequiredPermissions: []string{"document.create"},
					Steps: []module.DocumentFlowStepDefinition{{
						Key:       "request",
						Title:     "Request",
						TitleI18n: localize("Request", "Permintaan"),
						Documents: []module.DocumentFlowDocumentDefinition{{
							Key:           "request",
							Title:         "Request",
							TitleI18n:     localize("Request", "Permintaan"),
							DocumentType:  "generic_request",
							PrimaryOutput: true,
							Sections: []module.SectionDefinition{{
								Key: "request_core", Title: "Request", TitleI18n: localize("Request", "Permintaan"), Fields: []module.FieldDefinition{
									{Key: "title", Label: "Title", LabelI18n: localize("Title", "Judul"), Path: "body.payload.title", Type: "string", Widget: "textarea", Placeholder: "Describe your request", PlaceholderI18n: localize("Describe your request", "Jelaskan permintaan Anda")},
								},
							}},
						}},
					}},
				}},
			},
			SearchIndexes: []search.IndexDefinition{{
				Key:                 "documents.requests.search",
				Title:               "Request Search",
				SourceKind:          "document",
				DocumentType:        "generic_request",
				ViewKey:             "documents.requests.list",
				Modes:               []string{"keyword", "vector", "hybrid"},
				OrganizationSplit:   true,
				RequiredPermissions: []string{"document.list"},
				QueryFilterFields:   []string{"status", "location_id", "document_type"},
				QuerySortFields:     []string{"status", "updated_at", "title"},
				Fields: []search.IndexFieldDefinition{
					{Key: "document_id", Path: "header.id", Type: "string", Searchable: true},
					{Key: "document_type", Path: "header.type", Type: "string", Facet: true},
					{Key: "status", Path: "header.status", Type: "string", Facet: true, Sort: true},
					{Key: "title", Path: "body.payload.title", Type: "string", Searchable: true, Sort: true},
				},
				VectorFields: []search.VectorFieldDefinition{{
					Key: "semantic", SourcePaths: []string{"body.payload.title"}, EmbeddingMode: "external", Dimensions: 8, DistanceMetric: "cosine",
				}},
			}, {
				Key:                 "documents.summary.search",
				Title:               "Request Summary Projection Search",
				SourceKind:          "projection",
				ProjectionKey:       "document_summary",
				ViewKey:             "documents.requests.list",
				Modes:               []string{"keyword"},
				OrganizationSplit:   true,
				RequiredPermissions: []string{"document.list"},
				QueryFilterFields:   []string{"status", "location_id", "document_type"},
				QuerySortFields:     []string{"status", "updated_at"},
				Fields: []search.IndexFieldDefinition{
					{Key: "document_id", Path: "document_id", Type: "string", Searchable: true},
					{Key: "document_type", Path: "document_type", Type: "string", Facet: true},
					{Key: "status", Path: "status", Type: "string", Facet: true, Sort: true},
				},
			}},
			Offline: module.OfflineDefinition{
				Projections: []module.OfflineProjectionDefinition{{
					IndexKey:             "documents.requests.search",
					Title:                "Requests",
					TitleI18n:            localize("Requests", "Permintaan"),
					RequiredPermissions:  []string{"document.list"},
					DefaultFilters:       []string{"status=draft"},
					DefaultIncludeFields: []string{"document_id", "document_type", "status", "title"},
				}},
				Documents: []module.OfflineDocumentDefinition{{
					Type:                "generic_request",
					Title:               "Generic Request",
					TitleI18n:           localize("Generic Request", "Permintaan Generik"),
					CreatePermissionKey: "document.create",
					UpdatePermissionKey: "document.update_draft",
					RequiredPermissions: []string{"document.read"},
				}},
			},
		},
		{
			Key:          "analytics",
			Name:         "Analytics",
			NameI18n:     localize("Analytics", "Analitik"),
			Version:      "1.0.0",
			DomainFamily: "platform",
			DependencyRequirements: []module.DependencyRequirement{
				{ModuleKey: "documents", VersionRange: ">=1.1.0,<2.0.0", Kind: module.DependencyKindRequired},
				{ModuleKey: "monitoring", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindOptional},
			},
			OwnedProjectionKeys: []string{"document_summary"},
			DocumentExtensions: []module.DocumentExtension{{
				DocumentType:       "generic_request",
				SchemaVersion:      "v1",
				DisplayName:        "Analytics Extension",
				DisplayNameI18n:    localize("Analytics Extension", "Ekstensi Analitik"),
				ReadPermissionKey:  "analytics.read",
				WritePermissionKey: "analytics.manage_reports",
			}},
			Security: module.SecurityDefinition{
				Permissions: []module.PermissionDefinition{
					{Key: "analytics.read", Action: "read", Resource: "analytics", DisplayName: "Read Analytics", DisplayNameI18n: localize("Read Analytics", "Lihat Analitik")},
					{Key: "analytics.author", Action: "author", Resource: "analytics_runtime", DisplayName: "Author Runtime Analytics", DisplayNameI18n: localize("Author Runtime Analytics", "Kelola Analitik Runtime"), RiskLevel: "high"},
					{Key: "analytics.manage_reports", Action: "manage_reports", Resource: "analytics_report", DisplayName: "Manage Analytics Reports", DisplayNameI18n: localize("Manage Analytics Reports", "Kelola Laporan Analitik"), RiskLevel: "high"},
					{Key: "analytics.deliver_reports", Action: "deliver_reports", Resource: "analytics_report", DisplayName: "Deliver Analytics Reports", DisplayNameI18n: localize("Deliver Analytics Reports", "Kirim Laporan Analitik"), RiskLevel: "high"},
				},
				RoleTemplates: []module.RoleTemplateDefinition{{
					Key: "analytics_viewer", Name: "Analytics Viewer", NameI18n: localize("Analytics Viewer", "Pembaca Analitik"), AllowedScopes: []string{"deployment", "location"}, PermissionKeys: []string{"analytics.read"},
				}},
			},
			Observability: module.ObservabilityDefinition{
				Dashboards: []module.DashboardDefinition{
					{Key: "analytics.cockpit", Title: "Analytics Cockpit", TitleI18n: localize("Analytics Cockpit", "Kokpit Analitik"), RequiredPermissions: []string{"analytics.read"}},
				},
				Reports: []module.ReportDefinition{
					{Key: "analytics.documents.reporting", Title: "Document Reporting", TitleI18n: localize("Document Reporting", "Pelaporan Dokumen"), Dataset: "document_reporting", Formats: []string{"csv", "xlsx", "pdf"}, RequiredPermissions: []string{"analytics.read"}},
				},
				Metrics: []module.MetricDefinition{
					{Key: "analytics.snapshots.total", Type: "counter", Description: "Captured analytics snapshots"},
				},
				LogEvents: []module.LogEventDefinition{{
					Key: "analytics.report.delivery", Category: "analytics", Severity: "info", RequiredFields: []string{"artifact_id", "channel", "recipient"},
				}},
				DomainEvents: []module.DomainEventDefinition{
					{Type: "analytics.snapshot.captured", Role: "producer", CorrelationRequired: true},
				},
			},
			Templates: []module.TemplateDefinition{{
				Key:                 "analytics.document_reporting.default",
				Title:               "Document Reporting Print",
				TitleI18n:           localize("Document Reporting Print", "Cetak Pelaporan Dokumen"),
				Description:         "Default printable analytics reporting template.",
				DescriptionI18n:     localize("Default printable analytics reporting template.", "Template cetak default untuk pelaporan analitik."),
				TargetKind:          "report",
				TargetKey:           "document_reporting",
				RendererKind:        "visual",
				DefaultFormat:       "html",
				Formats:             []string{"html", "pdf"},
				Purpose:             "report",
				Channel:             "print",
				AllowedScopes:       []string{"deployment", "organization", "location"},
				RequiredPermissions: []string{"template.read", "template.render", "analytics.read"},
				DefaultBody:         `{"schema_version":"visual-grid/v1","title":"Document Reporting","settings":{"paper_preset":"a4","orientation":"portrait","density":"comfortable"},"sections":[{"id":"header","title":"Summary","rows":[{"columns":[{"span":8,"blocks":[{"type":"text","text":"Document Reporting","font_size":"xl","emphasis":"strong"}]},{"span":4,"blocks":[{"type":"field","label":"Total Rows","path":"report.total","align":"right"}]}]}]},{"id":"body","title":"Rows","rows":[{"columns":[{"span":12,"blocks":[{"type":"table","rows_path":"report.rows","columns":[{"label":"Dimension","path":"dimension_key"},{"label":"Label","path":"label"},{"label":"Total","path":"total"}]}]}]}]},{"id":"footer","title":"Notes","rows":[{"columns":[{"span":12,"blocks":[{"type":"text","text":"Generated from the reporting workspace.","align":"right","emphasis":"muted"}]}]}]}]}`,
				DefaultStyle:        `.template-visual{font-family:Arial,sans-serif;color:#0f172a}`,
			}},
			Frontend: module.FrontendDefinition{
				Menus: []module.MenuDefinition{{
					Key:                 "analytics.cockpit",
					Label:               "Analytics Cockpit",
					LabelI18n:           localize("Analytics Cockpit", "Kokpit Analitik"),
					ActionKey:           "analytics.cockpit",
					Order:               20,
					RequiredPermissions: []string{"analytics.read"},
				}},
				Actions: []module.ActionDefinition{{
					Key:                 "analytics.cockpit",
					Label:               "Analytics Cockpit",
					LabelI18n:           localize("Analytics Cockpit", "Kokpit Analitik"),
					Kind:                "navigate",
					RoutePath:           "/analytics/cockpit",
					CustomEntryKey:      "analytics.cockpit",
					RenderMode:          module.RenderModeCustom,
					RequiredPermissions: []string{"analytics.read"},
				}},
				CustomEntries: []module.CustomEntryDefinition{{
					Key:                 "analytics.cockpit",
					Title:               "Analytics Cockpit",
					TitleI18n:           localize("Analytics Cockpit", "Kokpit Analitik"),
					RoutePath:           "/analytics/cockpit",
					BundleKey:           "analytics-cockpit",
					ComponentExport:     "render",
					RequiredPermissions: []string{"analytics.read"},
				}},
			},
			MCP: module.MCPDefinition{
				Tools: []module.MCPToolDefinition{{
					Key:                 "analytics.snapshot.get",
					Title:               "Get Analytics Snapshot",
					TitleI18n:           localize("Get Analytics Snapshot", "Ambil Snapshot Analitik"),
					Description:         "Returns the latest analytics snapshot and links the cockpit app.",
					DescriptionI18n:     localize("Returns the latest analytics snapshot and links the cockpit app.", "Mengembalikan snapshot analitik terbaru dan menautkan aplikasi kokpit."),
					Operation:           "analytics.snapshot.get",
					RequiredPermissions: []string{"analytics.read"},
					AppKey:              "analytics.cockpit",
				}},
				Resources: []module.MCPResourceDefinition{
					{
						Key:                 "analytics.snapshot.current",
						Title:               "Current Analytics Snapshot",
						TitleI18n:           localize("Current Analytics Snapshot", "Snapshot Analitik Saat Ini"),
						Description:         "Structured current-state analytics snapshot.",
						DescriptionI18n:     localize("Structured current-state analytics snapshot.", "Snapshot analitik terstruktur untuk kondisi saat ini."),
						URI:                 "orbyte://analytics/snapshot/current",
						MIMEType:            "application/json",
						Provider:            "analytics.snapshot.current",
						RequiredPermissions: []string{"analytics.read"},
					},
					{
						Key:                 "analytics.cockpit.app",
						Title:               "Analytics Cockpit App",
						TitleI18n:           localize("Analytics Cockpit App", "Aplikasi Kokpit Analitik"),
						Description:         "Inline MCP app resource for the analytics cockpit.",
						DescriptionI18n:     localize("Inline MCP app resource for the analytics cockpit.", "Resource aplikasi MCP inline untuk kokpit analitik."),
						URI:                 "orbyte://apps/analytics.cockpit",
						MIMEType:            "text/html",
						Provider:            "mcp.app",
						RequiredPermissions: []string{"analytics.read"},
						AppKey:              "analytics.cockpit",
					},
				},
				Apps: []module.MCPAppDefinition{{
					Key:                 "analytics.cockpit",
					Title:               "Analytics Cockpit",
					TitleI18n:           localize("Analytics Cockpit", "Kokpit Analitik"),
					Description:         "Interactive analytics cockpit for MCP-capable hosts.",
					DescriptionI18n:     localize("Interactive analytics cockpit for MCP-capable hosts.", "Kokpit analitik interaktif untuk host yang mendukung MCP."),
					ResourceKey:         "analytics.cockpit.app",
					CustomEntryKey:      "analytics.cockpit",
					RequiredPermissions: []string{"analytics.read"},
				}},
			},
			Bundles: []module.BundleDefinition{{
				Key:    "analytics-cockpit",
				Script: httpx.AnalyticsCockpitBundle(),
			}},
		},
		{
			Key:          "monitoring",
			Name:         "Monitoring",
			NameI18n:     localize("Monitoring", "Pemantauan"),
			Version:      "1.0.0",
			DomainFamily: "platform",
			DependencyRequirements: []module.DependencyRequirement{{
				ModuleKey: "platform.core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired,
			}},
			Security: module.SecurityDefinition{
				Permissions: []module.PermissionDefinition{
					{Key: "metrics.read", Action: "read", Resource: "metrics", DisplayName: "Read Metrics", DisplayNameI18n: localize("Read Metrics", "Lihat Metrik")},
					{Key: "monitoring.read", Action: "read", Resource: "dashboard", DisplayName: "Read Monitoring Dashboard", DisplayNameI18n: localize("Read Monitoring Dashboard", "Lihat Dashboard Pemantauan")},
					{Key: "audit.read", Action: "read", Resource: "audit_event", DisplayName: "Read Audit Events", DisplayNameI18n: localize("Read Audit Events", "Lihat Event Audit")},
					{Key: "event.read", Action: "read", Resource: "domain_event", DisplayName: "Read Domain Events", DisplayNameI18n: localize("Read Domain Events", "Lihat Event Domain")},
					{Key: "outbox.read", Action: "read", Resource: "outbox", DisplayName: "Read Outbox", DisplayNameI18n: localize("Read Outbox", "Lihat Outbox")},
					{Key: "outbox.dispatch", Action: "dispatch", Resource: "outbox", DisplayName: "Dispatch Outbox", DisplayNameI18n: localize("Dispatch Outbox", "Kirim Outbox"), RiskLevel: "high"},
					{Key: "deadletter.read", Action: "read", Resource: "dead_letter", DisplayName: "Read Dead Letters", DisplayNameI18n: localize("Read Dead Letters", "Lihat Dead Letter")},
				},
			},
			Observability: module.ObservabilityDefinition{
				Dashboards: []module.DashboardDefinition{
					{Key: "monitoring.overview", Title: "Monitoring Overview", TitleI18n: localize("Monitoring Overview", "Ikhtisar Pemantauan"), ViewKey: "monitoring.overview", RequiredPermissions: []string{"monitoring.read"}},
				},
				Metrics: []module.MetricDefinition{
					{Key: "http.responses.404.total", Type: "counter", Description: "HTTP 404 responses"},
				},
			},
			Frontend: module.FrontendDefinition{
				Menus: []module.MenuDefinition{{
					Key: "monitoring.overview", Label: "Monitoring", LabelI18n: localize("Monitoring", "Pemantauan"), ActionKey: "monitoring.overview", Order: 30, RequiredPermissions: []string{"monitoring.read"},
				}},
				Actions: []module.ActionDefinition{{
					Key: "monitoring.overview", Label: "Monitoring Overview", LabelI18n: localize("Monitoring Overview", "Ikhtisar Pemantauan"), Kind: "navigate", RoutePath: "/monitoring", ViewKey: "monitoring.overview", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"monitoring.read"},
				}},
				Views: []module.ViewDefinition{{
					Key: "monitoring.overview", Title: "Monitoring Overview", TitleI18n: localize("Monitoring Overview", "Ikhtisar Pemantauan"), Kind: "dashboard", ProjectionKey: "monitoring.summary", RequiredPermissions: []string{"monitoring.read"},
					Cards: []module.CardDefinition{
						{Key: "documents_total", Label: "Documents", LabelI18n: localize("Documents", "Dokumen"), Path: "documents.total"},
						{Key: "outbox_pending", Label: "Outbox Pending", LabelI18n: localize("Outbox Pending", "Outbox Tertunda"), Path: "outbox.pending"},
						{Key: "pending_approvals", Label: "Pending Approvals", LabelI18n: localize("Pending Approvals", "Persetujuan Tertunda"), Path: "workflow.pending_approvals"},
						{Key: "projections", Label: "Document Summaries", LabelI18n: localize("Document Summaries", "Ringkasan Dokumen"), Path: "projections.document_summaries"},
					},
				}},
			},
		},
		{
			Key:          "integration",
			Name:         "Integration Kernel",
			NameI18n:     localize("Integration Kernel", "Kernel Integrasi"),
			Version:      "1.0.0",
			DomainFamily: "platform",
			DependencyRequirements: []module.DependencyRequirement{
				{ModuleKey: "platform.core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
				{ModuleKey: "documents", VersionRange: ">=1.1.0,<2.0.0", Kind: module.DependencyKindOptional},
			},
			Security: module.SecurityDefinition{
				PolicyHooks: []module.PolicyHookDefinition{
					{Key: "integration.submission.preflight", Kind: "integration", Target: "submission_preflight", InputContractKey: "integration.submission.v1", OutputContractKey: "decision.v1", Description: "Validates integration submissions before they are queued."},
				},
			},
			Observability: module.ObservabilityDefinition{
				Metrics: []module.MetricDefinition{
					{Key: "integration.submissions.queued.total", Type: "counter", Description: "Queued integration submissions"},
					{Key: "integration.submissions.succeeded.total", Type: "counter", Description: "Succeeded integration submissions"},
					{Key: "integration.submissions.failed.total", Type: "counter", Description: "Failed integration submissions"},
					{Key: "analytics.scheduler.enqueued.total", Type: "counter", Description: "Scheduled analytics jobs enqueued"},
					{Key: "analytics.scheduler.already_claimed.total", Type: "counter", Description: "Scheduled analytics work already claimed through shared job deduplication"},
					{Key: "analytics.scheduler.enqueue_failed.total", Type: "counter", Description: "Scheduled analytics enqueue failures"},
				},
				LogEvents: []module.LogEventDefinition{
					{Key: "integration.submission.succeeded", Category: "integration", Severity: "info", RequiredFields: []string{"submission_id", "system_key", "operation", "status"}},
					{Key: "integration.submission.failed", Category: "integration", Severity: "error", RequiredFields: []string{"submission_id", "system_key", "operation", "status"}},
				},
			},
		},
	}
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
