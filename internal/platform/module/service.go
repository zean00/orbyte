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
	items := make([]MenuDefinition, 0)
	for _, manifest := range s.manifests {
		items = append(items, manifest.Frontend.Menus...)
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
	items := make([]ActionDefinition, 0)
	for _, manifest := range s.manifests {
		items = append(items, manifest.Frontend.Actions...)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Key < items[j].Key })
	return items
}

func (s *Service) Views() []ViewDefinition {
	items := make([]ViewDefinition, 0)
	for _, manifest := range s.manifests {
		items = append(items, manifest.Frontend.Views...)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Key < items[j].Key })
	return items
}

func (s *Service) View(key string) (ViewDefinition, bool) {
	for _, manifest := range s.manifests {
		for _, view := range manifest.Frontend.Views {
			if view.Key == key {
				return view, true
			}
		}
	}
	return ViewDefinition{}, false
}

func (s *Service) CustomEntries() []CustomEntryDefinition {
	items := make([]CustomEntryDefinition, 0)
	for _, manifest := range s.manifests {
		items = append(items, manifest.Frontend.CustomEntries...)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Key < items[j].Key })
	return items
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

func (s *Service) ResolveRoute(path string) (RouteResolution, bool) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return RouteResolution{}, false
	}
	for _, manifest := range s.manifests {
		if !s.IsEnabled(manifest.Key) {
			continue
		}
		for _, action := range manifest.Frontend.Actions {
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
				if view, ok := s.View(action.ViewKey); ok {
					resolution.View = &view
				}
			}
			if action.CustomEntryKey != "" {
				for _, entry := range manifest.Frontend.CustomEntries {
					if entry.Key == action.CustomEntryKey {
						entryCopy := entry
						resolution.CustomEntry = &entryCopy
						break
					}
				}
			}
			return resolution, true
		}
	}
	return RouteResolution{}, false
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
	bundles := map[string]string{}
	menus := map[string]string{}
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

	for moduleKey, current := range existing {
		indexFrontendContracts(moduleKey, current, actions, views, customEntries, bundles, menus)
		indexSecurityContracts(moduleKey, current, permissions, roleTemplates, policyHooks)
		indexObservabilityContracts(moduleKey, current, projections, dashboards, reports, datasets, metrics, logEvents, domainEvents)
		indexModelContracts(moduleKey, current, models)
		indexSearchContracts(moduleKey, current, searchIndexes)
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
	for _, bundle := range manifest.Bundles {
		if strings.TrimSpace(bundle.Key) == "" || strings.TrimSpace(bundle.Script) == "" {
			return shared.Validation("bundle key and script are required")
		}
		if owner, ok := bundles[bundle.Key]; ok && owner != manifest.Key {
			return shared.Conflict("frontend bundle key already registered")
		}
		bundles[bundle.Key] = manifest.Key
	}
	routePaths := map[string]string{}
	for moduleKey, current := range existing {
		for _, action := range current.Frontend.Actions {
			routePaths[action.RoutePath] = moduleKey
		}
		for _, entry := range current.Frontend.CustomEntries {
			routePaths[entry.RoutePath] = moduleKey
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

func indexFrontendContracts(moduleKey string, manifest Manifest, actions, views, customEntries, bundles, menus map[string]string) {
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
	for _, bundle := range manifest.Bundles {
		bundles[bundle.Key] = moduleKey
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
