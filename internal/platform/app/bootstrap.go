package app

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"orbyte/internal/platform/analytics"
	"orbyte/internal/platform/config"
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/observability"
	"orbyte/internal/platform/policy"
	"orbyte/internal/platform/reference"
	"orbyte/internal/platform/reporting"
	"orbyte/internal/platform/search"
	"orbyte/internal/platform/shared"
	"orbyte/internal/platform/templateoutput"
	"orbyte/internal/platform/workflow"
)

type KernelInstallContext struct {
	ConfigSvc         *config.Service
	IdentitySvc       *identity.Service
	ModuleSvc         *module.Service
	ModelSvc          *model.Service
	ReportingSvc      *reporting.Service
	TemplateSvc       *templateoutput.Service
	ReferenceSvc      *reference.Service
	SearchSvc         *search.Service
	DocumentSvc       *document.Service
	WorkflowSvc       *workflow.Service
	PolicySvc         *policy.Service
	BusinessManifests []module.Manifest
	BootstrapPassword string
	Manifests         []module.Manifest
}

type KernelInstaller interface {
	Install(*KernelInstallContext) error
}

func seedPlatformKernel(configSvc *config.Service, identitySvc *identity.Service, moduleSvc *module.Service, modelSvc *model.Service, reportingSvc *reporting.Service, templateSvc *templateoutput.Service, referenceSvc *reference.Service, searchSvc *search.Service, documentSvc *document.Service, workflowSvc *workflow.Service, policySvc *policy.Service, businessManifests []module.Manifest, bootstrapPassword string) error {
	ctx := &KernelInstallContext{
		ConfigSvc:         configSvc,
		IdentitySvc:       identitySvc,
		ModuleSvc:         moduleSvc,
		ModelSvc:          modelSvc,
		ReportingSvc:      reportingSvc,
		TemplateSvc:       templateSvc,
		ReferenceSvc:      referenceSvc,
		SearchSvc:         searchSvc,
		DocumentSvc:       documentSvc,
		WorkflowSvc:       workflowSvc,
		PolicySvc:         policySvc,
		BusinessManifests: append([]module.Manifest(nil), businessManifests...),
		BootstrapPassword: bootstrapPassword,
		Manifests:         append(builtInModuleManifests(), businessManifests...),
	}
	for _, installer := range kernelInstallers() {
		if err := installer.Install(ctx); err != nil {
			return err
		}
	}
	return nil
}

func kernelInstallers() []KernelInstaller {
	return []KernelInstaller{
		moduleAndConfigInstaller{},
		configEntryInstaller{},
		referenceInstaller{},
		contentInstaller{},
		identityBootstrapInstaller{},
		securityInstaller{},
		modelSeedInstaller{},
		documentExtensionInstaller{},
	}
}

type moduleAndConfigInstaller struct{}

func (moduleAndConfigInstaller) Install(ctx *KernelInstallContext) error {
	for _, def := range config.BuiltInDefinitions() {
		if err := ctx.ConfigSvc.RegisterDefinition(def); err != nil {
			return err
		}
	}
	for _, manifest := range ctx.Manifests {
		if err := ctx.ModuleSvc.Register(manifest, "system"); err != nil {
			return err
		}
		for _, def := range manifest.ConfigDefinitions {
			if err := ctx.ConfigSvc.RegisterDefinition(def); err != nil {
				return err
			}
		}
	}
	return nil
}

type configEntryInstaller struct{}

func (configEntryInstaller) Install(ctx *KernelInstallContext) error {
	for _, entry := range config.BuiltInEntries(time.Now().UTC()) {
		if _, ok := ctx.ConfigSvc.Get(entry.Key); ok {
			continue
		}
		if err := ctx.ConfigSvc.Save(entry); err != nil {
			return err
		}
	}
	return nil
}

type referenceInstaller struct{}

func (referenceInstaller) Install(ctx *KernelInstallContext) error {
	for _, manifest := range ctx.Manifests {
		for _, def := range manifest.ReferenceTypes {
			if err := ignoreConflict(ctx.ReferenceSvc.RegisterType(def)); err != nil {
				return err
			}
		}
		for _, record := range manifest.ReferenceRecords {
			if err := ignoreConflict(ctx.ReferenceSvc.UpsertRecord(record)); err != nil {
				return err
			}
		}
	}
	return nil
}

type contentInstaller struct{}

func (contentInstaller) Install(ctx *KernelInstallContext) error {
	for _, manifest := range ctx.Manifests {
		for _, def := range manifest.Models {
			if err := ignoreConflict(ctx.ModelSvc.Register(def)); err != nil {
				return err
			}
		}
		for _, def := range manifest.Documents {
			if err := ignoreConflict(ctx.DocumentSvc.Register(def)); err != nil {
				return err
			}
		}
		for _, def := range manifest.Workflows {
			if err := ignoreConflict(ctx.WorkflowSvc.Register(def)); err != nil {
				return err
			}
		}
		for _, index := range manifest.SearchIndexes {
			if err := ignoreConflict(ctx.SearchSvc.RegisterIndex(index)); err != nil {
				return err
			}
		}
		for _, dataset := range manifest.Datasets {
			if err := ignoreConflict(ctx.ReportingSvc.Register(reportingDatasetDefinition(dataset))); err != nil {
				return err
			}
		}
		for _, def := range manifest.Templates {
			if err := ignoreConflict(ctx.TemplateSvc.RegisterDefinition(templateDefinitionFromModule(def, manifest.Key))); err != nil {
				return err
			}
		}
	}
	return nil
}

type identityBootstrapInstaller struct{}

func (identityBootstrapInstaller) Install(ctx *KernelInstallContext) error {
	if err := ctx.IdentitySvc.SeedBootstrapData(ctx.BootstrapPassword); err != nil {
		return err
	}
	return ctx.IdentitySvc.EnsureBootstrapAdminCredential(ctx.BootstrapPassword)
}

type securityInstaller struct{}

func (securityInstaller) Install(ctx *KernelInstallContext) error {
	return seedModuleContracts(ctx.IdentitySvc, ctx.PolicySvc, ctx.Manifests)
}

type modelSeedInstaller struct{}

func (modelSeedInstaller) Install(ctx *KernelInstallContext) error {
	seedModelRules(ctx.ModelSvc)
	seedModelData(ctx.ModelSvc)
	return nil
}

type documentExtensionInstaller struct{}

func (documentExtensionInstaller) Install(ctx *KernelInstallContext) error {
	for _, manifest := range ctx.Manifests {
		for _, extension := range manifest.DocumentExtensions {
			if err := ignoreConflict(ctx.DocumentSvc.RegisterExtension(documentExtensionDefinition(extension, manifest.Key))); err != nil {
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

func reportingDatasetDefinition(dataset module.DatasetDefinition) reporting.DatasetDefinition {
	return reporting.DatasetDefinition{
		Key:        dataset.Key,
		Title:      dataset.Title,
		SourceKind: dataset.SourceKind,
		ModelKey:   dataset.ModelKey,
		Dimensions: datasetDimensions(dataset.Dimensions),
		Measures:   datasetMeasures(dataset.Measures),
	}
}

func templateDefinitionFromModule(def module.TemplateDefinition, moduleKey string) templateoutput.Definition {
	return templateoutput.FromModule(def, moduleKey)
}

func documentExtensionDefinition(extension module.DocumentExtension, moduleKey string) document.ExtensionDefinition {
	return document.ExtensionDefinition{
		DocumentType:       extension.DocumentType,
		ModuleKey:          moduleKey,
		DisplayName:        extension.DisplayName,
		SchemaVersion:      extension.SchemaVersion,
		ReadPermissionKey:  extension.ReadPermissionKey,
		WritePermissionKey: extension.WritePermissionKey,
	}
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
	modelSvc.SetConstraintEvaluator("accounting_period.date_range", func(input model.RuleInput) error {
		startDate := strings.TrimSpace(stringValue(input.Values["start_date"]))
		endDate := strings.TrimSpace(stringValue(input.Values["end_date"]))
		if startDate != "" && endDate != "" && startDate > endDate {
			return shared.Validation("accounting period start_date cannot be after end_date")
		}
		return nil
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
