package httpx

import (
	"sort"
	"strings"

	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/module"
)

type uiContractsSnapshot struct {
	menus   []module.MenuDefinition
	actions []module.ActionDefinition
	views   []module.ViewDefinition
	entries []module.CustomEntryDefinition
	flows   []module.DocumentFlowDefinition
}

type uiBootstrapResolver struct {
	ident            *identity.Service
	modules          *module.Service
	principal        principal
	moduleDetails    []module.Detail
	contracts        map[module.UISurface]uiContractsSnapshot
	routeResolutions map[string]uiResolvedRoute
	permissionAllows map[string]bool
	fallbacks        map[module.UISurface]string
	surfaces         []string
	surfacesReady    bool
}

type uiResolvedRoute struct {
	resolution module.RouteResolution
	ok         bool
}

func newUIBootstrapResolver(ident *identity.Service, modules *module.Service, p principal) *uiBootstrapResolver {
	return &uiBootstrapResolver{
		ident:            ident,
		modules:          modules,
		principal:        p,
		moduleDetails:    modules.List(),
		contracts:        map[module.UISurface]uiContractsSnapshot{},
		routeResolutions: map[string]uiResolvedRoute{},
		permissionAllows: map[string]bool{},
		fallbacks:        map[module.UISurface]string{},
	}
}

func (r *uiBootstrapResolver) allowsPermission(permissionKey string) bool {
	permissionKey = strings.TrimSpace(permissionKey)
	if permissionKey == "" {
		return true
	}
	cacheKey := r.principal.currentLocationID + "::" + permissionKey
	allowed, ok := r.permissionAllows[cacheKey]
	if ok {
		return allowed
	}
	allowed = principalAllowsPermission(r.ident, r.principal, permissionKey, r.principal.currentLocationID)
	r.permissionAllows[cacheKey] = allowed
	return allowed
}

func (r *uiBootstrapResolver) allowsAll(permissions []string) bool {
	for _, permission := range permissions {
		if !r.allowsPermission(permission) {
			return false
		}
	}
	return true
}

func (r *uiBootstrapResolver) visibleContracts(surface module.UISurface) ([]module.MenuDefinition, []module.ActionDefinition, []module.ViewDefinition, []module.CustomEntryDefinition, []module.DocumentFlowDefinition) {
	if snapshot, ok := r.contracts[surface]; ok {
		return snapshot.menus, snapshot.actions, snapshot.views, snapshot.entries, snapshot.flows
	}

	allowedMenus := make([]module.MenuDefinition, 0)
	allowedActions := make([]module.ActionDefinition, 0)
	allowedViews := make([]module.ViewDefinition, 0)
	allowedEntries := make([]module.CustomEntryDefinition, 0)
	allowedFlows := make([]module.DocumentFlowDefinition, 0)
	actionKeys := map[string]bool{}
	viewKeys := map[string]bool{}
	entryKeys := map[string]bool{}
	flowKeys := map[string]bool{}

	for _, detail := range r.moduleDetails {
		if !detail.Installed.Enabled {
			continue
		}
		for _, action := range detail.Manifest.Frontend.Actions {
			if !surfaceMatches(action.Surface, surface) {
				continue
			}
			if !r.allowsAll(action.RequiredPermissions) {
				continue
			}
			switch action.RenderMode {
			case module.RenderModeGeneric:
				if action.ViewKey != "" {
					view, ok := r.modules.ViewForSurface(action.ViewKey, surface)
					if !ok || !r.allowsAll(view.RequiredPermissions) {
						continue
					}
					if !viewKeys[view.Key] {
						allowedViews = append(allowedViews, view)
						viewKeys[view.Key] = true
					}
				}
			case module.RenderModeCustom:
				entryAllowed := false
				for _, entry := range detail.Manifest.Frontend.CustomEntries {
					if entry.Key == action.CustomEntryKey && surfaceMatches(entry.Surface, surface) && r.allowsAll(entry.RequiredPermissions) {
						entryAllowed = true
						if !entryKeys[entry.Key] {
							allowedEntries = append(allowedEntries, entry)
							entryKeys[entry.Key] = true
						}
						break
					}
				}
				if !entryAllowed {
					continue
				}
			case module.RenderModeFlow:
				flowAllowed := false
				for _, flow := range detail.Manifest.Frontend.DocumentFlows {
					if flow.Key == action.FlowKey && surfaceMatches(flow.Surface, surface) && r.allowsAll(flow.RequiredPermissions) {
						flowAllowed = true
						if !flowKeys[flow.Key] {
							allowedFlows = append(allowedFlows, flow)
							flowKeys[flow.Key] = true
						}
						break
					}
				}
				if !flowAllowed {
					continue
				}
			}
			if !actionKeys[action.Key] {
				allowedActions = append(allowedActions, action)
				actionKeys[action.Key] = true
			}
		}
	}

	for _, detail := range r.moduleDetails {
		if !detail.Installed.Enabled {
			continue
		}
		for _, menuDef := range detail.Manifest.Frontend.Menus {
			if !surfaceMatches(menuDef.Surface, surface) {
				continue
			}
			if !r.allowsAll(menuDef.RequiredPermissions) || !actionKeys[menuDef.ActionKey] {
				continue
			}
			allowedMenus = append(allowedMenus, menuDef)
		}
	}

	sort.Slice(allowedMenus, func(i, j int) bool {
		if allowedMenus[i].Order == allowedMenus[j].Order {
			return allowedMenus[i].Key < allowedMenus[j].Key
		}
		return allowedMenus[i].Order < allowedMenus[j].Order
	})
	sort.Slice(allowedActions, func(i, j int) bool { return allowedActions[i].Key < allowedActions[j].Key })
	sort.Slice(allowedViews, func(i, j int) bool { return allowedViews[i].Key < allowedViews[j].Key })
	sort.Slice(allowedEntries, func(i, j int) bool { return allowedEntries[i].Key < allowedEntries[j].Key })
	sort.Slice(allowedFlows, func(i, j int) bool { return allowedFlows[i].Key < allowedFlows[j].Key })

	snapshot := uiContractsSnapshot{
		menus:   allowedMenus,
		actions: allowedActions,
		views:   allowedViews,
		entries: allowedEntries,
		flows:   allowedFlows,
	}
	r.contracts[surface] = snapshot
	return snapshot.menus, snapshot.actions, snapshot.views, snapshot.entries, snapshot.flows
}

func (r *uiBootstrapResolver) resolveRoute(surface module.UISurface, path string) (module.RouteResolution, bool) {
	cacheKey := string(surface) + "::" + strings.TrimSpace(path)
	if cached, ok := r.routeResolutions[cacheKey]; ok {
		return cached.resolution, cached.ok
	}
	resolution, ok := r.modules.ResolveRouteForSurface(path, surface)
	r.routeResolutions[cacheKey] = uiResolvedRoute{resolution: resolution, ok: ok}
	return resolution, ok
}

func (r *uiBootstrapResolver) fallbackPath(surface module.UISurface) string {
	if path, ok := r.fallbacks[surface]; ok {
		return path
	}
	menus, actions, _, _, _ := r.visibleContracts(surface)
	path := defaultRouteForSurface(r.ident, r.principal.userID, uiSurfacePreference(surface), menus, actions)
	r.fallbacks[surface] = path
	return path
}

func (r *uiBootstrapResolver) availableSurfaces() []string {
	if r.surfacesReady {
		return r.surfaces
	}
	items := make([]string, 0, 5)
	for _, surface := range []module.UISurface{
		module.UISurfaceBackoffice,
		module.UISurfaceWorklist,
		module.UISurfaceSelfService,
		module.UISurfaceAgent,
		module.UISurfacePOS,
		module.UISurfaceDashboard,
	} {
		menus, actions, _, _, _ := r.visibleContracts(surface)
		if len(menus) > 0 || len(actions) > 0 {
			items = append(items, string(surface))
		}
	}
	if len(items) == 0 {
		items = append(items, string(module.UISurfaceBackoffice))
	}
	r.surfaces = items
	r.surfacesReady = true
	return items
}

func (r *uiBootstrapResolver) fallbackPaths() map[string]string {
	items := map[string]string{}
	for _, surface := range r.availableSurfaces() {
		if path := r.fallbackPath(module.UISurface(surface)); path != "" {
			items[surface] = path
		}
	}
	return items
}
