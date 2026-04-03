package httpx

import (
	"net/http"

	"orbyte/internal/platform/acp"
	"orbyte/internal/platform/i18n"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/module"
)

func buildWorkspaceBootstrapPayload(r *http.Request, ident *identity.Service, modules *module.Service, p principal, surface module.UISurface, uiPrefs *UIPreferencesService, acpSvc *acp.Service) map[string]any {
	resolver := newUIBootstrapResolver(ident, modules, p)
	menus, actions, views, _, flows := resolver.visibleContracts(surface)
	adminMenus, adminActions, _, _, _ := resolver.visibleContracts(module.UISurfaceAdmin)
	defaultPath := defaultRouteForSurface(ident, p.userID, uiSurfacePreference(surface), menus, actions)
	fallbackPaths := resolver.fallbackPaths()
	adminPath := "/admin"
	if len(adminMenus) > 0 {
		for _, action := range adminActions {
			if action.Key == adminMenus[0].ActionKey {
				adminPath = "/admin#" + action.RoutePath
				break
			}
		}
	}
	return map[string]any{
		"shell_kind": "workspace",
		"surface":    surface,
		"shell": map[string]any{
			"nav_mode":         "collapsible",
			"command_mode":     "route_jump",
			"agent_surface":    "panel",
			"supports_nav_pin": true,
		},
		"page_floorplans":    []string{"worklist", "object", "dashboard", "editor"},
		"available_surfaces": resolver.availableSurfaces(),
		"menus":              menus,
		"actions":            actions,
		"views":              views,
		"flows":              flows,
		"self_service_apis":  visibleSelfServiceAPIs(ident, modules, p),
		"default_path":       defaultPath,
		"preferred_path":     ident.PreferredRoute(p.userID, uiSurfacePreference(surface)),
		"fallback_paths":     fallbackPaths,
		"admin_access":       len(adminMenus) > 0,
		"admin_path":         adminPath,
		"capabilities": map[string]any{
			"ui_preferences":     uiPrefs != nil,
			"saved_filters":      true,
			"column_preferences": true,
			"density_modes":      []string{"comfortable", "compact"},
			"keyboard_shortcuts": true,
			"offline_cache":      true,
			"shell_recovery":     true,
		},
		"locale":            localeFromRequest(r, ident),
		"supported_locales": i18n.SupportedLocales(),
		"auth_context": map[string]any{
			"actor_user_id":       p.userID,
			"effective_user_id":   principalEffectiveUserID(p),
			"delegation_active":   principalHasDelegation(p),
			"delegation_grant_id": principalDelegationGrantID(p),
		},
		"acp": buildACPBootstrap(acpSvc),
	}
}
