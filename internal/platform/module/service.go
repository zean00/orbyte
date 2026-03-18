package module

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"orbyte/internal/platform/config"
	"orbyte/internal/platform/shared"
)

type Service struct {
	repo      Repository
	manifests map[string]Manifest
}

func NewService() *Service {
	return NewServiceWithRepository(NewMemoryRepository(nil))
}

func NewServiceWithRepository(repo Repository) *Service {
	return &Service{repo: repo, manifests: map[string]Manifest{}}
}

func (s *Service) Register(manifest Manifest, actorID string) error {
	if manifest.Key == "" {
		return shared.Validation("module key is required")
	}
	if err := validateManifest(s.manifests, manifest); err != nil {
		return err
	}
	s.manifests[manifest.Key] = manifest
	if _, ok := s.repo.Get(manifest.Key); ok {
		return nil
	}
	if actorID == "" {
		actorID = "system"
	}
	return s.repo.Save(InstalledModule{
		Key:       manifest.Key,
		Enabled:   true,
		UpdatedAt: time.Now().UTC(),
		UpdatedBy: actorID,
	})
}

func (s *Service) List() []Detail {
	items := s.repo.List()
	result := make([]Detail, 0, len(items))
	for _, item := range items {
		if manifest, ok := s.manifests[item.Key]; ok {
			result = append(result, s.detail(manifest, item))
		}
	}
	return result
}

func (s *Service) Get(key string) (Detail, bool) {
	item, ok := s.repo.Get(key)
	if !ok {
		return Detail{}, false
	}
	manifest, ok := s.manifests[key]
	if !ok {
		return Detail{}, false
	}
	return s.detail(manifest, item), true
}

func (s *Service) CompatibilityReport() []Detail {
	items := s.List()
	sort.Slice(items, func(i, j int) bool { return items[i].Manifest.Key < items[j].Manifest.Key })
	return items
}

func (s *Service) RoleTemplates() []RoleTemplateAssignment {
	items := make([]RoleTemplateAssignment, 0)
	for _, detail := range s.List() {
		for _, tpl := range detail.Manifest.Security.RoleTemplates {
			items = append(items, RoleTemplateAssignment{
				ModuleKey:     detail.Manifest.Key,
				Template:      tpl,
				RoleID:        roleTemplateRoleID(detail.Manifest.Key, tpl.Key),
				Applied:       true,
				PermissionIDs: append([]string(nil), tpl.PermissionKeys...),
			})
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].ModuleKey == items[j].ModuleKey {
			return items[i].Template.Key < items[j].Template.Key
		}
		return items[i].ModuleKey < items[j].ModuleKey
	})
	return items
}

func (s *Service) Definitions() []config.Definition {
	items := make([]config.Definition, 0)
	for _, manifest := range s.manifests {
		items = append(items, manifest.ConfigDefinitions...)
	}
	return items
}

func (s *Service) SecurityDefinitions() []SecurityDefinition {
	items := make([]SecurityDefinition, 0, len(s.manifests))
	for _, manifest := range s.manifests {
		items = append(items, manifest.Security)
	}
	return items
}

func (s *Service) ObservabilityDefinitions() []ObservabilityDefinition {
	items := make([]ObservabilityDefinition, 0, len(s.manifests))
	for _, manifest := range s.manifests {
		items = append(items, manifest.Observability)
	}
	return items
}

func (s *Service) EnabledMap() map[string]bool {
	items := s.repo.List()
	result := make(map[string]bool, len(items))
	for _, item := range items {
		result[item.Key] = item.Enabled
	}
	return result
}

func (s *Service) Menus() []MenuDefinition {
	return s.MenusForSurface(UISurfaceBoth)
}

func (s *Service) MenusForSurface(surface UISurface) []MenuDefinition {
	items := make([]MenuDefinition, 0)
	for _, manifest := range s.manifests {
		for _, item := range manifest.Frontend.Menus {
			if matchesSurface(item.Surface, surface) {
				items = append(items, item)
			}
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Order == items[j].Order {
			return items[i].Key < items[j].Key
		}
		return items[i].Order < items[j].Order
	})
	return items
}

func (s *Service) Actions() []ActionDefinition {
	return s.ActionsForSurface(UISurfaceBoth)
}

func (s *Service) ActionsForSurface(surface UISurface) []ActionDefinition {
	items := make([]ActionDefinition, 0)
	for _, manifest := range s.manifests {
		for _, item := range manifest.Frontend.Actions {
			if matchesSurface(item.Surface, surface) {
				items = append(items, item)
			}
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Key < items[j].Key })
	return items
}

func (s *Service) Views() []ViewDefinition {
	return s.ViewsForSurface(UISurfaceBoth)
}

func (s *Service) ViewsForSurface(surface UISurface) []ViewDefinition {
	items := make([]ViewDefinition, 0)
	for _, manifest := range s.manifests {
		for _, item := range manifest.Frontend.Views {
			if matchesSurface(item.Surface, surface) {
				items = append(items, item)
			}
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Key < items[j].Key })
	return items
}

func (s *Service) View(key string) (ViewDefinition, bool) {
	return s.ViewForSurface(key, UISurfaceBoth)
}

func (s *Service) ViewForSurface(key string, surface UISurface) (ViewDefinition, bool) {
	for _, manifest := range s.manifests {
		for _, view := range manifest.Frontend.Views {
			if view.Key == key && matchesSurface(view.Surface, surface) {
				return view, true
			}
		}
	}
	return ViewDefinition{}, false
}

func (s *Service) CustomEntries() []CustomEntryDefinition {
	return s.CustomEntriesForSurface(UISurfaceBoth)
}

func (s *Service) CustomEntriesForSurface(surface UISurface) []CustomEntryDefinition {
	items := make([]CustomEntryDefinition, 0)
	for _, manifest := range s.manifests {
		for _, item := range manifest.Frontend.CustomEntries {
			if matchesSurface(item.Surface, surface) {
				items = append(items, item)
			}
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Key < items[j].Key })
	return items
}

func (s *Service) DocumentFlows() []DocumentFlowDefinition {
	return s.DocumentFlowsForSurface(UISurfaceBoth)
}

func (s *Service) DocumentFlowsForSurface(surface UISurface) []DocumentFlowDefinition {
	items := make([]DocumentFlowDefinition, 0)
	for _, manifest := range s.manifests {
		for _, item := range manifest.Frontend.DocumentFlows {
			if matchesSurface(item.Surface, surface) {
				items = append(items, item)
			}
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Key < items[j].Key })
	return items
}

func (s *Service) DocumentFlowForSurface(key string, surface UISurface) (DocumentFlowDefinition, bool) {
	for _, manifest := range s.manifests {
		for _, flow := range manifest.Frontend.DocumentFlows {
			if flow.Key == key && matchesSurface(flow.Surface, surface) {
				return flow, true
			}
		}
	}
	return DocumentFlowDefinition{}, false
}

func (s *Service) Bundle(key string) (BundleDefinition, bool) {
	for _, manifest := range s.manifests {
		for _, bundle := range manifest.Bundles {
			if bundle.Key == key {
				return bundle, true
			}
		}
	}
	return BundleDefinition{}, false
}

func (s *Service) MCPTools() []MCPToolDefinition {
	items := make([]MCPToolDefinition, 0)
	for _, manifest := range s.manifests {
		items = append(items, manifest.MCP.Tools...)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Key < items[j].Key })
	return items
}

func (s *Service) MCPResources() []MCPResourceDefinition {
	items := make([]MCPResourceDefinition, 0)
	for _, manifest := range s.manifests {
		items = append(items, manifest.MCP.Resources...)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Key < items[j].Key })
	return items
}

func (s *Service) MCPApps() []MCPAppDefinition {
	items := make([]MCPAppDefinition, 0)
	for _, manifest := range s.manifests {
		items = append(items, manifest.MCP.Apps...)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Key < items[j].Key })
	return items
}

func (s *Service) Templates() []TemplateDefinition {
	items := make([]TemplateDefinition, 0)
	for _, manifest := range s.manifests {
		items = append(items, manifest.Templates...)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Key < items[j].Key })
	return items
}

func (s *Service) Template(key string) (TemplateDefinition, bool) {
	for _, manifest := range s.manifests {
		for _, item := range manifest.Templates {
			if item.Key == key {
				return item, true
			}
		}
	}
	return TemplateDefinition{}, false
}

func (s *Service) MCPTool(key string) (MCPToolDefinition, bool) {
	for _, manifest := range s.manifests {
		for _, item := range manifest.MCP.Tools {
			if item.Key == key {
				return item, true
			}
		}
	}
	return MCPToolDefinition{}, false
}

func (s *Service) MCPResourceByKey(key string) (MCPResourceDefinition, bool) {
	for _, manifest := range s.manifests {
		for _, item := range manifest.MCP.Resources {
			if item.Key == key {
				return item, true
			}
		}
	}
	return MCPResourceDefinition{}, false
}

func (s *Service) MCPResourceByURI(uri string) (MCPResourceDefinition, bool) {
	trimmed := strings.TrimSpace(uri)
	if trimmed == "" {
		return MCPResourceDefinition{}, false
	}
	for _, manifest := range s.manifests {
		for _, item := range manifest.MCP.Resources {
			if item.URI == trimmed {
				return item, true
			}
		}
	}
	return MCPResourceDefinition{}, false
}

func (s *Service) MCPApp(key string) (MCPAppDefinition, bool) {
	for _, manifest := range s.manifests {
		for _, item := range manifest.MCP.Apps {
			if item.Key == key {
				return item, true
			}
		}
	}
	return MCPAppDefinition{}, false
}

func (s *Service) OfflineReferences() []OfflineReferenceDefinition {
	items := make([]OfflineReferenceDefinition, 0)
	for _, manifest := range s.manifests {
		if !s.IsEnabled(manifest.Key) {
			continue
		}
		items = append(items, manifest.Offline.References...)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].TypeKey < items[j].TypeKey })
	return items
}

func (s *Service) OfflineProjections() []OfflineProjectionDefinition {
	items := make([]OfflineProjectionDefinition, 0)
	for _, manifest := range s.manifests {
		if !s.IsEnabled(manifest.Key) {
			continue
		}
		items = append(items, manifest.Offline.Projections...)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].IndexKey < items[j].IndexKey })
	return items
}

func (s *Service) OfflineDocuments() []OfflineDocumentDefinition {
	items := make([]OfflineDocumentDefinition, 0)
	for _, manifest := range s.manifests {
		if !s.IsEnabled(manifest.Key) {
			continue
		}
		items = append(items, manifest.Offline.Documents...)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Type < items[j].Type })
	return items
}

func (s *Service) OfflineModels() []OfflineModelDefinition {
	items := make([]OfflineModelDefinition, 0)
	for _, manifest := range s.manifests {
		if !s.IsEnabled(manifest.Key) {
			continue
		}
		items = append(items, manifest.Offline.Models...)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ModelKey < items[j].ModelKey })
	return items
}

func (s *Service) OfflineReference(typeKey string) (OfflineReferenceDefinition, bool) {
	for _, manifest := range s.manifests {
		if !s.IsEnabled(manifest.Key) {
			continue
		}
		for _, item := range manifest.Offline.References {
			if item.TypeKey == typeKey {
				return item, true
			}
		}
	}
	return OfflineReferenceDefinition{}, false
}

func (s *Service) OfflineProjection(indexKey string) (OfflineProjectionDefinition, bool) {
	for _, manifest := range s.manifests {
		if !s.IsEnabled(manifest.Key) {
			continue
		}
		for _, item := range manifest.Offline.Projections {
			if item.IndexKey == indexKey {
				return item, true
			}
		}
	}
	return OfflineProjectionDefinition{}, false
}

func (s *Service) OfflineDocument(documentType string) (OfflineDocumentDefinition, bool) {
	for _, manifest := range s.manifests {
		if !s.IsEnabled(manifest.Key) {
			continue
		}
		for _, item := range manifest.Offline.Documents {
			if item.Type == documentType {
				return item, true
			}
		}
	}
	return OfflineDocumentDefinition{}, false
}

func (s *Service) OfflineModel(modelKey string) (OfflineModelDefinition, bool) {
	for _, manifest := range s.manifests {
		if !s.IsEnabled(manifest.Key) {
			continue
		}
		for _, item := range manifest.Offline.Models {
			if item.ModelKey == modelKey {
				return item, true
			}
		}
	}
	return OfflineModelDefinition{}, false
}

func (s *Service) ResolveRoute(path string) (RouteResolution, bool) {
	return s.ResolveRouteForSurface(path, UISurfaceBoth)
}

func (s *Service) ResolveRouteForSurface(path string, surface UISurface) (RouteResolution, bool) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return RouteResolution{}, false
	}
	for _, manifest := range s.manifests {
		if !s.IsEnabled(manifest.Key) {
			continue
		}
		for _, action := range manifest.Frontend.Actions {
			if !matchesSurface(action.Surface, surface) {
				continue
			}
			if action.RoutePath != trimmed {
				continue
			}
			resolution := RouteResolution{
				Path:       trimmed,
				ModuleKey:  manifest.Key,
				RenderMode: action.RenderMode,
				Action:     action,
			}
			if action.ViewKey != "" {
				if view, ok := s.ViewForSurface(action.ViewKey, surface); ok {
					resolution.View = &view
				}
			}
			if action.CustomEntryKey != "" {
				for _, entry := range manifest.Frontend.CustomEntries {
					if entry.Key == action.CustomEntryKey && matchesSurface(entry.Surface, surface) {
						entryCopy := entry
						resolution.CustomEntry = &entryCopy
						break
					}
				}
			}
			if action.FlowKey != "" {
				for _, flow := range manifest.Frontend.DocumentFlows {
					if flow.Key == action.FlowKey && matchesSurface(flow.Surface, surface) {
						flowCopy := flow
						resolution.Flow = &flowCopy
						break
					}
				}
			}
			return resolution, true
		}
	}
	return RouteResolution{}, false
}

func matchesSurface(itemSurface, requested UISurface) bool {
	effective := itemSurface
	if effective == "" {
		effective = UISurfaceUser
	}
	if requested == "" || requested == UISurfaceBoth {
		return true
	}
	return effective == UISurfaceBoth || effective == requested
}

func (s *Service) IsEnabled(key string) bool {
	item, ok := s.repo.Get(key)
	return ok && item.Enabled
}

func (s *Service) Enable(key, actorID string) (InstalledModule, error) {
	return s.setEnabled(key, actorID, true)
}

func (s *Service) Disable(key, actorID string) (InstalledModule, error) {
	return s.setEnabled(key, actorID, false)
}

func (s *Service) setEnabled(key, actorID string, enabled bool) (InstalledModule, error) {
	item, ok := s.repo.Get(key)
	if !ok {
		return InstalledModule{}, shared.NotFound("module not found")
	}
	if enabled {
		for _, dependency := range manifestDependencies(s.manifests[key]) {
			if dependency.Kind == DependencyKindOptional {
				continue
			}
			if !s.IsEnabled(dependency.ModuleKey) {
				return InstalledModule{}, shared.Conflict("module dependency is disabled")
			}
			if dependency.VersionRange != "" {
				current, ok := s.manifests[dependency.ModuleKey]
				if !ok {
					return InstalledModule{}, shared.Conflict("module dependency is missing")
				}
				compatible, err := versionSatisfies(current.Version, dependency.VersionRange)
				if err != nil {
					return InstalledModule{}, shared.Validation(err.Error())
				}
				if !compatible {
					return InstalledModule{}, shared.Conflict("module dependency version is incompatible")
				}
			}
		}
	} else {
		for moduleKey, manifest := range s.manifests {
			if moduleKey == key || !s.IsEnabled(moduleKey) {
				continue
			}
			for _, dependency := range manifestDependencies(manifest) {
				if dependency.Kind == DependencyKindOptional {
					continue
				}
				if dependency.ModuleKey == key {
					return InstalledModule{}, shared.Conflict("module has enabled dependents")
				}
			}
		}
	}
	if actorID == "" {
		actorID = "system"
	}
	item.Enabled = enabled
	item.UpdatedAt = time.Now().UTC()
	item.UpdatedBy = actorID
	if err := s.repo.Save(item); err != nil {
		return InstalledModule{}, err
	}
	return item, nil
}

func (s *Service) detail(manifest Manifest, item InstalledModule) Detail {
	dependencies := manifestDependencies(manifest)
	state := make(map[string]bool, len(dependencies))
	diagnostics := make([]DependencyDiagnostic, 0, len(dependencies))
	for _, dependency := range dependencies {
		enabled := s.IsEnabled(dependency.ModuleKey)
		state[dependency.ModuleKey] = enabled
		diagnostic := DependencyDiagnostic{
			ModuleKey:    dependency.ModuleKey,
			VersionRange: dependency.VersionRange,
			Kind:         dependency.Kind,
			Enabled:      enabled,
			Compatible:   dependency.Kind == DependencyKindOptional,
		}
		if current, ok := s.manifests[dependency.ModuleKey]; ok {
			diagnostic.DependencyVersion = current.Version
			if dependency.VersionRange == "" {
				diagnostic.Compatible = true
			} else if compatible, err := versionSatisfies(current.Version, dependency.VersionRange); err == nil {
				diagnostic.Compatible = compatible
				if !compatible {
					diagnostic.Reason = "version incompatible"
				}
			} else {
				diagnostic.Reason = err.Error()
			}
		} else {
			diagnostic.Reason = "dependency manifest not registered"
		}
		if !enabled && dependency.Kind != DependencyKindOptional {
			diagnostic.Reason = "dependency disabled"
			diagnostic.Compatible = false
		}
		if dependency.Kind == DependencyKindOptional && !enabled && diagnostic.Reason == "" {
			diagnostic.Reason = "optional dependency disabled"
		}
		diagnostics = append(diagnostics, diagnostic)
	}
	sort.Slice(diagnostics, func(i, j int) bool { return diagnostics[i].ModuleKey < diagnostics[j].ModuleKey })
	return Detail{
		Manifest:              manifest,
		Installed:             item,
		DependencyState:       state,
		DependencyDiagnostics: diagnostics,
		LifecycleState:        lifecycleState(item.Enabled, diagnostics),
	}
}

func validateManifest(existing map[string]Manifest, manifest Manifest) error {
	if strings.TrimSpace(manifest.Key) == "" {
		return shared.Validation("module key is required")
	}
	actions := map[string]string{}
	views := map[string]string{}
	customEntries := map[string]string{}
	flows := map[string]string{}
	bundles := map[string]string{}
	menus := map[string]string{}
	mcpTools := map[string]string{}
	mcpResources := map[string]string{}
	mcpURIs := map[string]string{}
	mcpApps := map[string]string{}
	templates := map[string]string{}
	permissions := map[string]string{}
	roleTemplates := map[string]string{}
	policyHooks := map[string]string{}
	projections := map[string]string{}
	dashboards := map[string]string{}
	reports := map[string]string{}
	datasets := map[string]string{}
	metrics := map[string]string{}
	logEvents := map[string]string{}
	domainEvents := map[string]string{}
	models := map[string]string{}
	searchIndexes := map[string]string{}
	referenceTypes := map[string]string{}
	documentTypes := map[string]string{}

	for moduleKey, current := range existing {
		indexFrontendContracts(moduleKey, current, actions, views, customEntries, flows, bundles, menus)
		indexMCPContracts(moduleKey, current, mcpTools, mcpResources, mcpURIs, mcpApps)
		indexTemplateContracts(moduleKey, current, templates)
		indexSecurityContracts(moduleKey, current, permissions, roleTemplates, policyHooks)
		indexObservabilityContracts(moduleKey, current, projections, dashboards, reports, datasets, metrics, logEvents, domainEvents)
		indexModelContracts(moduleKey, current, models)
		indexSearchContracts(moduleKey, current, searchIndexes)
		indexReferenceContracts(moduleKey, current, referenceTypes)
		indexDocumentContracts(moduleKey, current, documentTypes)
	}
	for _, dependency := range manifestDependencies(manifest) {
		if strings.TrimSpace(dependency.ModuleKey) == "" {
			return shared.Validation("dependency module_key is required")
		}
		if dependency.Kind == "" {
			dependency.Kind = DependencyKindRequired
		}
		if dependency.VersionRange != "" {
			if _, err := parseVersion(strings.TrimSpace(strings.TrimPrefix(dependency.VersionRange, ">="))); err != nil && !strings.ContainsAny(dependency.VersionRange, "<,") {
				return shared.Validation("dependency version_range is invalid")
			}
		}
	}
	for _, refType := range manifest.ReferenceTypes {
		if strings.TrimSpace(refType.Key) == "" || strings.TrimSpace(refType.DisplayName) == "" {
			return shared.Validation("reference type key and display_name are required")
		}
		if owner, ok := referenceTypes[refType.Key]; ok && owner != manifest.Key {
			return shared.Conflict("reference type key already registered")
		}
		referenceTypes[refType.Key] = manifest.Key
	}
	for _, documentType := range manifest.OwnedDocumentTypes {
		if strings.TrimSpace(documentType) == "" {
			return shared.Validation("owned document type is required")
		}
		documentTypes[documentType] = manifest.Key
	}
	for _, def := range manifest.Documents {
		if strings.TrimSpace(def.Type) == "" || strings.TrimSpace(def.DisplayName) == "" || strings.TrimSpace(def.SchemaVersion) == "" {
			return shared.Validation("document type, display_name, and schema_version are required")
		}
		if owner, ok := documentTypes[def.Type]; ok && owner != manifest.Key {
			return shared.Conflict("document type already registered")
		}
		documentTypes[def.Type] = manifest.Key
	}
	for _, permission := range manifest.Security.Permissions {
		if strings.TrimSpace(permission.Key) == "" || strings.TrimSpace(permission.Action) == "" || strings.TrimSpace(permission.Resource) == "" {
			return shared.Validation("security permission key, action, and resource are required")
		}
		if owner, ok := permissions[permission.Key]; ok && owner != manifest.Key {
			return shared.Conflict("security permission key already registered")
		}
		permissions[permission.Key] = manifest.Key
	}
	for _, role := range manifest.Security.RoleTemplates {
		if strings.TrimSpace(role.Key) == "" || strings.TrimSpace(role.Name) == "" {
			return shared.Validation("security role template key and name are required")
		}
		if owner, ok := roleTemplates[role.Key]; ok && owner != manifest.Key {
			return shared.Conflict("security role template key already registered")
		}
		for _, permissionKey := range role.PermissionKeys {
			if _, ok := permissions[permissionKey]; !ok {
				return shared.Validation("role template permission key is not registered")
			}
		}
		roleTemplates[role.Key] = manifest.Key
	}
	for _, hook := range manifest.Security.PolicyHooks {
		if strings.TrimSpace(hook.Key) == "" || strings.TrimSpace(hook.Kind) == "" || strings.TrimSpace(hook.Target) == "" {
			return shared.Validation("policy hook key, kind, and target are required")
		}
		if owner, ok := policyHooks[hook.Key]; ok && owner != manifest.Key {
			return shared.Conflict("policy hook key already registered")
		}
		policyHooks[hook.Key] = manifest.Key
	}
	for _, projection := range manifest.Observability.Projections {
		if strings.TrimSpace(projection.Key) == "" {
			return shared.Validation("observability projection key is required")
		}
		if owner, ok := projections[projection.Key]; ok && owner != manifest.Key {
			return shared.Conflict("observability projection key already registered")
		}
		projections[projection.Key] = manifest.Key
	}
	for _, dashboard := range manifest.Observability.Dashboards {
		if strings.TrimSpace(dashboard.Key) == "" || strings.TrimSpace(dashboard.Title) == "" {
			return shared.Validation("observability dashboard key and title are required")
		}
		if owner, ok := dashboards[dashboard.Key]; ok && owner != manifest.Key {
			return shared.Conflict("observability dashboard key already registered")
		}
		dashboards[dashboard.Key] = manifest.Key
	}
	for _, report := range manifest.Observability.Reports {
		if strings.TrimSpace(report.Key) == "" || strings.TrimSpace(report.Title) == "" {
			return shared.Validation("observability report key and title are required")
		}
		if owner, ok := reports[report.Key]; ok && owner != manifest.Key {
			return shared.Conflict("observability report key already registered")
		}
		reports[report.Key] = manifest.Key
	}
	for _, dataset := range manifest.Datasets {
		if strings.TrimSpace(dataset.Key) == "" || strings.TrimSpace(dataset.Title) == "" || strings.TrimSpace(dataset.SourceKind) == "" {
			return shared.Validation("dataset key, title, and source_kind are required")
		}
		if owner, ok := datasets[dataset.Key]; ok && owner != manifest.Key {
			return shared.Conflict("dataset key already registered")
		}
		if dataset.SourceKind == "model" && strings.TrimSpace(dataset.ModelKey) == "" {
			return shared.Validation("model datasets require model_key")
		}
		datasets[dataset.Key] = manifest.Key
	}
	for _, metric := range manifest.Observability.Metrics {
		if strings.TrimSpace(metric.Key) == "" || strings.TrimSpace(metric.Type) == "" {
			return shared.Validation("observability metric key and type are required")
		}
		if owner, ok := metrics[metric.Key]; ok && owner != manifest.Key {
			return shared.Conflict("observability metric key already registered")
		}
		metrics[metric.Key] = manifest.Key
	}
	for _, logEvent := range manifest.Observability.LogEvents {
		if strings.TrimSpace(logEvent.Key) == "" {
			return shared.Validation("observability log event key is required")
		}
		if owner, ok := logEvents[logEvent.Key]; ok && owner != manifest.Key {
			return shared.Conflict("observability log event key already registered")
		}
		logEvents[logEvent.Key] = manifest.Key
	}
	for _, domainEvent := range manifest.Observability.DomainEvents {
		if strings.TrimSpace(domainEvent.Type) == "" {
			return shared.Validation("observability domain event type is required")
		}
		if owner, ok := domainEvents[domainEvent.Type]; ok && owner != manifest.Key {
			return shared.Conflict("observability domain event type already registered")
		}
		domainEvents[domainEvent.Type] = manifest.Key
	}
	for _, def := range manifest.Models {
		if strings.TrimSpace(def.Key) == "" || strings.TrimSpace(def.DisplayName) == "" {
			return shared.Validation("model key and display_name are required")
		}
		if owner, ok := models[def.Key]; ok && owner != manifest.Key {
			return shared.Conflict("model key already registered")
		}
		models[def.Key] = manifest.Key
	}
	for _, index := range manifest.SearchIndexes {
		if strings.TrimSpace(index.Key) == "" || strings.TrimSpace(index.Title) == "" || strings.TrimSpace(index.SourceKind) == "" {
			return shared.Validation("search index key, title, and source_kind are required")
		}
		if owner, ok := searchIndexes[index.Key]; ok && owner != manifest.Key {
			return shared.Conflict("search index key already registered")
		}
		switch index.SourceKind {
		case "document":
			if strings.TrimSpace(index.DocumentType) == "" {
				return shared.Validation("document search index requires document_type")
			}
			if !containsString(manifest.OwnedDocumentTypes, index.DocumentType) {
				return shared.Validation("document search index document_type is not owned by module")
			}
		case "model":
			if strings.TrimSpace(index.ModelKey) == "" {
				return shared.Validation("model search index requires model_key")
			}
			if _, ok := models[index.ModelKey]; !ok {
				return shared.Validation("model search index model_key is not registered")
			}
		case "projection":
			if strings.TrimSpace(index.ProjectionKey) == "" {
				return shared.Validation("projection search index requires projection_key")
			}
			if _, ok := projections[index.ProjectionKey]; !ok {
				return shared.Validation("projection search index projection_key is not registered")
			}
		default:
			return shared.Validation("search index source_kind is invalid")
		}
		searchIndexes[index.Key] = manifest.Key
	}
	for _, item := range manifest.Offline.References {
		if strings.TrimSpace(item.TypeKey) == "" || strings.TrimSpace(item.Title) == "" {
			return shared.Validation("offline reference type_key and title are required")
		}
		if _, ok := referenceTypes[item.TypeKey]; !ok {
			return shared.Validation("offline reference type_key is not registered")
		}
	}
	for _, item := range manifest.Offline.Projections {
		if strings.TrimSpace(item.IndexKey) == "" || strings.TrimSpace(item.Title) == "" {
			return shared.Validation("offline projection index_key and title are required")
		}
		if _, ok := searchIndexes[item.IndexKey]; !ok {
			return shared.Validation("offline projection index_key is not registered")
		}
	}
	for _, item := range manifest.Offline.Documents {
		if strings.TrimSpace(item.Type) == "" || strings.TrimSpace(item.Title) == "" {
			return shared.Validation("offline document type and title are required")
		}
		if _, ok := documentTypes[item.Type]; !ok {
			return shared.Validation("offline document type is not registered")
		}
	}
	for _, item := range manifest.Offline.Models {
		if strings.TrimSpace(item.ModelKey) == "" || strings.TrimSpace(item.Title) == "" {
			return shared.Validation("offline model model_key and title are required")
		}
		if _, ok := models[item.ModelKey]; !ok {
			return shared.Validation("offline model model_key is not registered")
		}
	}

	for _, menu := range manifest.Frontend.Menus {
		if strings.TrimSpace(menu.Key) == "" || strings.TrimSpace(menu.ActionKey) == "" {
			return shared.Validation("menu key and action_key are required")
		}
		if owner, ok := menus[menu.Key]; ok && owner != manifest.Key {
			return shared.Conflict("frontend menu key already registered")
		}
		menus[menu.Key] = manifest.Key
	}
	for _, view := range manifest.Frontend.Views {
		if strings.TrimSpace(view.Key) == "" || strings.TrimSpace(view.Kind) == "" {
			return shared.Validation("view key and kind are required")
		}
		if strings.TrimSpace(view.ModelKey) != "" {
			if _, ok := models[view.ModelKey]; !ok {
				return shared.Validation("view model_key is not registered")
			}
		}
		if strings.TrimSpace(view.DatasetKey) != "" {
			if _, ok := datasets[view.DatasetKey]; !ok {
				return shared.Validation("view dataset_key is not registered")
			}
		}
		for _, column := range view.Columns {
			if strings.TrimSpace(column.Key) == "" || strings.TrimSpace(column.Path) == "" {
				return shared.Validation("view column key and path are required")
			}
		}
		for _, filter := range view.Filters {
			if strings.TrimSpace(filter.Key) == "" || strings.TrimSpace(filter.Type) == "" {
				return shared.Validation("view filter key and type are required")
			}
		}
		for _, section := range view.Sections {
			if strings.TrimSpace(section.Key) == "" || strings.TrimSpace(section.Title) == "" {
				return shared.Validation("view section key and title are required")
			}
		}
		for _, tab := range view.Tabs {
			if strings.TrimSpace(tab.Key) == "" || strings.TrimSpace(tab.Title) == "" {
				return shared.Validation("view tab key and title are required")
			}
		}
		for _, field := range view.Fields {
			if strings.TrimSpace(field.Key) == "" || strings.TrimSpace(field.Path) == "" || strings.TrimSpace(field.Type) == "" {
				return shared.Validation("view field key, path, and type are required")
			}
		}
		for _, related := range view.RelatedViews {
			if strings.TrimSpace(related.Key) == "" || strings.TrimSpace(related.Title) == "" || strings.TrimSpace(related.Source) == "" {
				return shared.Validation("view related view key, title, and source are required")
			}
		}
		for _, placement := range view.ActionPlacements {
			if strings.TrimSpace(placement.ActionKey) == "" || strings.TrimSpace(placement.Zone) == "" {
				return shared.Validation("view action placement action_key and zone are required")
			}
		}
		for _, card := range view.Cards {
			if strings.TrimSpace(card.Key) == "" || strings.TrimSpace(card.Path) == "" {
				return shared.Validation("view card key and path are required")
			}
		}
		if owner, ok := views[view.Key]; ok && owner != manifest.Key {
			return shared.Conflict("frontend view key already registered")
		}
		views[view.Key] = manifest.Key
	}
	for _, entry := range manifest.Frontend.CustomEntries {
		if strings.TrimSpace(entry.Key) == "" || strings.TrimSpace(entry.RoutePath) == "" || strings.TrimSpace(entry.BundleKey) == "" || strings.TrimSpace(entry.ComponentExport) == "" {
			return shared.Validation("custom entry key, route_path, bundle_key, and component_export are required")
		}
		if owner, ok := customEntries[entry.Key]; ok && owner != manifest.Key {
			return shared.Conflict("frontend custom entry key already registered")
		}
		customEntries[entry.Key] = manifest.Key
	}
	for _, flow := range manifest.Frontend.DocumentFlows {
		if strings.TrimSpace(flow.Key) == "" || strings.TrimSpace(flow.Title) == "" || strings.TrimSpace(flow.RoutePath) == "" || strings.TrimSpace(flow.PrimaryDocumentType) == "" {
			return shared.Validation("document flow key, title, route_path, and primary_document_type are required")
		}
		if owner, ok := flows[flow.Key]; ok && owner != manifest.Key {
			return shared.Conflict("document flow key already registered")
		}
		if _, ok := documentTypes[flow.PrimaryDocumentType]; !ok {
			return shared.Validation("document flow primary_document_type is not registered")
		}
		primaryCount := 0
		stepKeys := map[string]bool{}
		for _, step := range flow.Steps {
			if strings.TrimSpace(step.Key) == "" || strings.TrimSpace(step.Title) == "" {
				return shared.Validation("document flow step key and title are required")
			}
			if stepKeys[step.Key] {
				return shared.Validation("document flow step key must be unique")
			}
			stepKeys[step.Key] = true
			if len(step.Documents) == 0 {
				return shared.Validation("document flow step requires at least one document")
			}
			for _, item := range step.Documents {
				if strings.TrimSpace(item.Key) == "" || strings.TrimSpace(item.Title) == "" || strings.TrimSpace(item.DocumentType) == "" {
					return shared.Validation("document flow document key, title, and document_type are required")
				}
				if _, ok := documentTypes[item.DocumentType]; !ok {
					return shared.Validation("document flow document_type is not registered")
				}
				if item.PrimaryOutput {
					primaryCount++
				}
			}
		}
		for _, step := range flow.Steps {
			if strings.TrimSpace(step.NextStepKey) != "" && !stepKeys[step.NextStepKey] {
				return shared.Validation("document flow next_step_key is not registered")
			}
			for _, rule := range step.NextRules {
				if strings.TrimSpace(rule.Path) == "" || strings.TrimSpace(rule.NextStepKey) == "" {
					return shared.Validation("document flow branch rule path and next_step_key are required")
				}
				if !stepKeys[rule.NextStepKey] {
					return shared.Validation("document flow branch next_step_key is not registered")
				}
			}
		}
		if primaryCount != 1 {
			return shared.Validation("document flow requires exactly one primary output document")
		}
		flows[flow.Key] = manifest.Key
	}
	for _, bundle := range manifest.Bundles {
		if strings.TrimSpace(bundle.Key) == "" || strings.TrimSpace(bundle.Script) == "" {
			return shared.Validation("bundle key and script are required")
		}
		if owner, ok := bundles[bundle.Key]; ok && owner != manifest.Key {
			return shared.Conflict("frontend bundle key already registered")
		}
		bundles[bundle.Key] = manifest.Key
	}
	resourceKeys := map[string]struct{}{}
	templateKeys := map[string]struct{}{}
	for _, item := range manifest.MCP.Resources {
		if strings.TrimSpace(item.Key) == "" || strings.TrimSpace(item.Title) == "" || strings.TrimSpace(item.URI) == "" {
			return shared.Validation("mcp resource key, title, and uri are required")
		}
		if owner, ok := mcpResources[item.Key]; ok && owner != manifest.Key {
			return shared.Conflict("mcp resource key already registered")
		}
		if owner, ok := mcpURIs[item.URI]; ok && owner != manifest.Key {
			return shared.Conflict("mcp resource uri already registered")
		}
		mcpResources[item.Key] = manifest.Key
		mcpURIs[item.URI] = manifest.Key
		resourceKeys[item.Key] = struct{}{}
	}
	for _, item := range manifest.Templates {
		if strings.TrimSpace(item.Key) == "" || strings.TrimSpace(item.Title) == "" || strings.TrimSpace(item.TargetKind) == "" || strings.TrimSpace(item.TargetKey) == "" {
			return shared.Validation("template key, title, target_kind, and target_key are required")
		}
		switch strings.TrimSpace(item.RendererKind) {
		case "html", "visual":
		default:
			return shared.Validation("template renderer_kind is invalid")
		}
		if owner, ok := templates[item.Key]; ok && owner != manifest.Key {
			return shared.Conflict("template key already registered")
		}
		templates[item.Key] = manifest.Key
		templateKeys[item.Key] = struct{}{}
	}
	appKeys := map[string]struct{}{}
	for _, item := range manifest.MCP.Apps {
		if strings.TrimSpace(item.Key) == "" || strings.TrimSpace(item.Title) == "" || strings.TrimSpace(item.ResourceKey) == "" {
			return shared.Validation("mcp app key, title, and resource_key are required")
		}
		if owner, ok := mcpApps[item.Key]; ok && owner != manifest.Key {
			return shared.Conflict("mcp app key already registered")
		}
		if strings.TrimSpace(item.ViewKey) == "" && strings.TrimSpace(item.CustomEntryKey) == "" {
			return shared.Validation("mcp app requires view_key or custom_entry_key")
		}
		if strings.TrimSpace(item.ViewKey) != "" && strings.TrimSpace(item.CustomEntryKey) != "" {
			return shared.Validation("mcp app must not declare both view_key and custom_entry_key")
		}
		if strings.TrimSpace(item.ViewKey) != "" {
			if _, ok := views[item.ViewKey]; !ok {
				return shared.Validation("mcp app view_key is not registered")
			}
		}
		if strings.TrimSpace(item.CustomEntryKey) != "" {
			if _, ok := customEntries[item.CustomEntryKey]; !ok {
				return shared.Validation("mcp app custom_entry_key is not registered")
			}
		}
		if _, ok := mcpResources[item.ResourceKey]; !ok {
			if _, ok := resourceKeys[item.ResourceKey]; !ok {
				return shared.Validation("mcp app resource_key is not registered")
			}
		}
		mcpApps[item.Key] = manifest.Key
		appKeys[item.Key] = struct{}{}
	}
	for _, item := range manifest.MCP.Tools {
		if strings.TrimSpace(item.Key) == "" || strings.TrimSpace(item.Title) == "" || strings.TrimSpace(item.Operation) == "" {
			return shared.Validation("mcp tool key, title, and operation are required")
		}
		if owner, ok := mcpTools[item.Key]; ok && owner != manifest.Key {
			return shared.Conflict("mcp tool key already registered")
		}
		if strings.TrimSpace(item.AppKey) != "" {
			if _, ok := mcpApps[item.AppKey]; !ok {
				if _, ok := appKeys[item.AppKey]; !ok {
					return shared.Validation("mcp tool app_key is not registered")
				}
			}
		}
		mcpTools[item.Key] = manifest.Key
	}
	for _, item := range manifest.MCP.Resources {
		if strings.TrimSpace(item.AppKey) != "" {
			if _, ok := mcpApps[item.AppKey]; !ok {
				if _, ok := appKeys[item.AppKey]; !ok {
					return shared.Validation("mcp resource app_key is not registered")
				}
			}
		}
	}
	routePaths := map[string]string{}
	localFlowKeys := map[string]bool{}
	for _, flow := range manifest.Frontend.DocumentFlows {
		localFlowKeys[flow.Key] = true
	}
	for moduleKey, current := range existing {
		for _, action := range current.Frontend.Actions {
			routePaths[action.RoutePath] = moduleKey
		}
		for _, entry := range current.Frontend.CustomEntries {
			routePaths[entry.RoutePath] = moduleKey
		}
		for _, flow := range current.Frontend.DocumentFlows {
			routePaths[flow.RoutePath] = moduleKey
		}
	}
	for _, action := range manifest.Frontend.Actions {
		if strings.TrimSpace(action.Key) == "" || strings.TrimSpace(action.RoutePath) == "" {
			return shared.Validation("action key and route_path are required")
		}
		if owner, ok := actions[action.Key]; ok && owner != manifest.Key {
			return shared.Conflict("frontend action key already registered")
		}
		if owner, ok := routePaths[action.RoutePath]; ok && owner != manifest.Key {
			return shared.Conflict("frontend route path already registered")
		}
		actions[action.Key] = manifest.Key
		routePaths[action.RoutePath] = manifest.Key
		if action.Surface == UISurfaceAdmin && strings.TrimSpace(action.ViewKey) == "" && strings.TrimSpace(action.CustomEntryKey) == "" {
			continue
		}
		switch action.RenderMode {
		case RenderModeGeneric:
			if strings.TrimSpace(action.ViewKey) == "" {
				return shared.Validation("generic actions require view_key")
			}
			if _, ok := views[action.ViewKey]; !ok {
				return shared.Validation("action view_key is not registered")
			}
		case RenderModeCustom:
			if strings.TrimSpace(action.CustomEntryKey) == "" {
				return shared.Validation("custom actions require custom_entry_key")
			}
			if _, ok := customEntries[action.CustomEntryKey]; !ok {
				return shared.Validation("action custom_entry_key is not registered")
			}
		case RenderModeFlow:
			if strings.TrimSpace(action.FlowKey) == "" {
				return shared.Validation("flow actions require flow_key")
			}
			if _, ok := flows[action.FlowKey]; !ok && !localFlowKeys[action.FlowKey] {
				return shared.Validation("action flow_key is not registered")
			}
		default:
			return shared.Validation("action render_mode is invalid")
		}
	}
	for _, menu := range manifest.Frontend.Menus {
		if _, ok := actions[menu.ActionKey]; !ok {
			return shared.Validation("menu action_key is not registered")
		}
	}
	for _, entry := range manifest.Frontend.CustomEntries {
		if _, ok := bundles[entry.BundleKey]; !ok {
			return shared.Validation("custom entry bundle_key is not registered")
		}
		if owner, ok := routePaths[entry.RoutePath]; ok && owner != manifest.Key {
			return shared.Conflict("frontend route path already registered")
		}
	}
	for _, flow := range manifest.Frontend.DocumentFlows {
		if owner, ok := routePaths[flow.RoutePath]; ok && owner != manifest.Key {
			return shared.Conflict("frontend route path already registered")
		}
		routePaths[flow.RoutePath] = manifest.Key
	}
	return nil
}

func roleTemplateRoleID(moduleKey, roleKey string) string {
	return "role:" + moduleKey + ":" + roleKey
}

func lifecycleState(enabled bool, diagnostics []DependencyDiagnostic) string {
	if !enabled {
		return "disabled"
	}
	for _, diagnostic := range diagnostics {
		if diagnostic.Kind == DependencyKindOptional {
			continue
		}
		if !diagnostic.Compatible {
			return "blocked"
		}
	}
	return "healthy"
}

func indexFrontendContracts(moduleKey string, manifest Manifest, actions, views, customEntries, flows, bundles, menus map[string]string) {
	for _, menu := range manifest.Frontend.Menus {
		menus[menu.Key] = moduleKey
	}
	for _, action := range manifest.Frontend.Actions {
		actions[action.Key] = moduleKey
	}
	for _, view := range manifest.Frontend.Views {
		views[view.Key] = moduleKey
	}
	for _, entry := range manifest.Frontend.CustomEntries {
		customEntries[entry.Key] = moduleKey
	}
	for _, flow := range manifest.Frontend.DocumentFlows {
		flows[flow.Key] = moduleKey
	}
	for _, bundle := range manifest.Bundles {
		bundles[bundle.Key] = moduleKey
	}
}

func indexMCPContracts(moduleKey string, manifest Manifest, tools, resources, uris, apps map[string]string) {
	for _, item := range manifest.MCP.Tools {
		tools[item.Key] = moduleKey
	}
	for _, item := range manifest.MCP.Resources {
		resources[item.Key] = moduleKey
		uris[item.URI] = moduleKey
	}
	for _, item := range manifest.MCP.Apps {
		apps[item.Key] = moduleKey
	}
}

func indexTemplateContracts(moduleKey string, manifest Manifest, templates map[string]string) {
	for _, item := range manifest.Templates {
		templates[item.Key] = moduleKey
	}
}

func indexSecurityContracts(moduleKey string, manifest Manifest, permissions, roleTemplates, policyHooks map[string]string) {
	for _, permission := range manifest.Security.Permissions {
		permissions[permission.Key] = moduleKey
	}
	for _, role := range manifest.Security.RoleTemplates {
		roleTemplates[role.Key] = moduleKey
	}
	for _, hook := range manifest.Security.PolicyHooks {
		policyHooks[hook.Key] = moduleKey
	}
}

func indexObservabilityContracts(moduleKey string, manifest Manifest, projections, dashboards, reports, datasets, metrics, logEvents, domainEvents map[string]string) {
	for _, projection := range manifest.Observability.Projections {
		projections[projection.Key] = moduleKey
	}
	for _, dashboard := range manifest.Observability.Dashboards {
		dashboards[dashboard.Key] = moduleKey
	}
	for _, report := range manifest.Observability.Reports {
		reports[report.Key] = moduleKey
	}
	for _, dataset := range manifest.Datasets {
		datasets[dataset.Key] = moduleKey
	}
	for _, metric := range manifest.Observability.Metrics {
		metrics[metric.Key] = moduleKey
	}
	for _, logEvent := range manifest.Observability.LogEvents {
		logEvents[logEvent.Key] = moduleKey
	}
	for _, domainEvent := range manifest.Observability.DomainEvents {
		domainEvents[domainEvent.Type] = moduleKey
	}
}

func indexModelContracts(moduleKey string, manifest Manifest, models map[string]string) {
	for _, def := range manifest.Models {
		models[def.Key] = moduleKey
	}
}

func indexReferenceContracts(moduleKey string, manifest Manifest, referenceTypes map[string]string) {
	for _, def := range manifest.ReferenceTypes {
		referenceTypes[def.Key] = moduleKey
	}
}

func indexDocumentContracts(moduleKey string, manifest Manifest, documentTypes map[string]string) {
	for _, item := range manifest.OwnedDocumentTypes {
		documentTypes[item] = moduleKey
	}
	for _, def := range manifest.Documents {
		documentTypes[def.Type] = moduleKey
	}
}

func indexSearchContracts(moduleKey string, manifest Manifest, indexes map[string]string) {
	for _, index := range manifest.SearchIndexes {
		indexes[index.Key] = moduleKey
	}
}

func containsString(items []string, candidate string) bool {
	for _, item := range items {
		if item == candidate {
			return true
		}
	}
	return false
}

func manifestDependencies(manifest Manifest) []DependencyRequirement {
	if len(manifest.DependencyRequirements) > 0 {
		items := make([]DependencyRequirement, 0, len(manifest.DependencyRequirements))
		for _, dependency := range manifest.DependencyRequirements {
			if dependency.Kind == "" {
				dependency.Kind = DependencyKindRequired
			}
			items = append(items, dependency)
		}
		return items
	}
	items := make([]DependencyRequirement, 0, len(manifest.Dependencies))
	for _, dependency := range manifest.Dependencies {
		if strings.TrimSpace(dependency) == "" {
			continue
		}
		items = append(items, DependencyRequirement{ModuleKey: dependency, Kind: DependencyKindRequired})
	}
	return items
}

func versionSatisfies(version, requirement string) (bool, error) {
	constraints := strings.Split(requirement, ",")
	current, err := parseVersion(version)
	if err != nil {
		return false, err
	}
	for _, constraint := range constraints {
		constraint = strings.TrimSpace(constraint)
		if constraint == "" {
			continue
		}
		operator := "="
		operand := constraint
		switch {
		case strings.HasPrefix(constraint, ">="):
			operator = ">="
			operand = strings.TrimSpace(strings.TrimPrefix(constraint, ">="))
		case strings.HasPrefix(constraint, "<="):
			operator = "<="
			operand = strings.TrimSpace(strings.TrimPrefix(constraint, "<="))
		case strings.HasPrefix(constraint, ">"):
			operator = ">"
			operand = strings.TrimSpace(strings.TrimPrefix(constraint, ">"))
		case strings.HasPrefix(constraint, "<"):
			operator = "<"
			operand = strings.TrimSpace(strings.TrimPrefix(constraint, "<"))
		case strings.HasPrefix(constraint, "="):
			operator = "="
			operand = strings.TrimSpace(strings.TrimPrefix(constraint, "="))
		}
		target, err := parseVersion(operand)
		if err != nil {
			return false, err
		}
		cmp := compareVersions(current, target)
		switch operator {
		case ">=":
			if cmp < 0 {
				return false, nil
			}
		case "<=":
			if cmp > 0 {
				return false, nil
			}
		case ">":
			if cmp <= 0 {
				return false, nil
			}
		case "<":
			if cmp >= 0 {
				return false, nil
			}
		default:
			if cmp != 0 {
				return false, nil
			}
		}
	}
	return true, nil
}

type semanticVersion struct {
	major int
	minor int
	patch int
}

func parseVersion(value string) (semanticVersion, error) {
	trimmed := strings.TrimSpace(strings.TrimPrefix(value, "v"))
	parts := strings.Split(trimmed, ".")
	if len(parts) != 3 {
		return semanticVersion{}, fmt.Errorf("invalid semantic version %q", value)
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return semanticVersion{}, fmt.Errorf("invalid semantic version %q", value)
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return semanticVersion{}, fmt.Errorf("invalid semantic version %q", value)
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return semanticVersion{}, fmt.Errorf("invalid semantic version %q", value)
	}
	return semanticVersion{major: major, minor: minor, patch: patch}, nil
}

func compareVersions(left, right semanticVersion) int {
	switch {
	case left.major != right.major:
		return compareInt(left.major, right.major)
	case left.minor != right.minor:
		return compareInt(left.minor, right.minor)
	default:
		return compareInt(left.patch, right.patch)
	}
}

func compareInt(left, right int) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}
