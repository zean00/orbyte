package httpx

import (
	"sort"
	"strings"

	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/module"
)

func visibleUIContracts(ident *identity.Service, modules *module.Service, p principal, surface module.UISurface) ([]module.MenuDefinition, []module.ActionDefinition, []module.ViewDefinition, []module.CustomEntryDefinition, []module.DocumentFlowDefinition) {
	allowedMenus := make([]module.MenuDefinition, 0)
	allowedActions := make([]module.ActionDefinition, 0)
	allowedViews := make([]module.ViewDefinition, 0)
	allowedEntries := make([]module.CustomEntryDefinition, 0)
	allowedFlows := make([]module.DocumentFlowDefinition, 0)
	actionKeys := map[string]bool{}
	viewKeys := map[string]bool{}
	entryKeys := map[string]bool{}
	flowKeys := map[string]bool{}

	for _, detail := range modules.List() {
		if !detail.Installed.Enabled {
			continue
		}
		for _, action := range detail.Manifest.Frontend.Actions {
			if !surfaceMatches(action.Surface, surface) {
				continue
			}
			if !principalAllowsAll(ident, p, action.RequiredPermissions) {
				continue
			}
			switch action.RenderMode {
			case module.RenderModeGeneric:
				if action.ViewKey != "" {
					view, ok := modules.ViewForSurface(action.ViewKey, surface)
					if !ok || !principalAllowsAll(ident, p, view.RequiredPermissions) {
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
					if entry.Key == action.CustomEntryKey && surfaceMatches(entry.Surface, surface) && principalAllowsAll(ident, p, entry.RequiredPermissions) {
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
					if flow.Key == action.FlowKey && surfaceMatches(flow.Surface, surface) && principalAllowsAll(ident, p, flow.RequiredPermissions) {
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

	for _, detail := range modules.List() {
		if !detail.Installed.Enabled {
			continue
		}
		for _, menuDef := range detail.Manifest.Frontend.Menus {
			if !surfaceMatches(menuDef.Surface, surface) {
				continue
			}
			if !principalAllowsAll(ident, p, menuDef.RequiredPermissions) || !actionKeys[menuDef.ActionKey] {
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

	return allowedMenus, allowedActions, allowedViews, allowedEntries, allowedFlows
}

func surfaceMatches(itemSurface, requested module.UISurface) bool {
	effective := itemSurface
	if effective == "" {
		effective = module.UISurfaceUser
	}
	if requested == "" || requested == module.UISurfaceBoth {
		return true
	}
	if effective == module.UISurfaceBoth {
		return true
	}
	if requested == module.UISurfaceBackoffice && effective == module.UISurfaceUser {
		return true
	}
	if requested == module.UISurfaceUser && effective == module.UISurfaceBackoffice {
		return true
	}
	return effective == requested
}

func viewKeyFromPath(path string) string {
	const prefix = "/ui/views/"
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(path, prefix))
}

func bundleKeyFromPath(path string) string {
	const prefix = "/ui/assets/modules/"
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	return strings.TrimSuffix(strings.TrimSpace(strings.TrimPrefix(path, prefix)), ".js")
}
