package httpx

import (
	"net/http"

	"orbyte/internal/platform/i18n"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/module"
)

func buildWorkspaceBootstrapPayload(r *http.Request, ident *identity.Service, modules *module.Service, p principal, surface module.UISurface, uiPrefs *UIPreferencesService) map[string]any {
	menus, actions, views, _, flows := visibleUIContracts(ident, modules, p, surface)
	adminMenus, adminActions, _, _, _ := visibleUIContracts(ident, modules, p, module.UISurfaceAdmin)
	defaultPath := defaultRouteForSurface(ident, p.userID, uiSurfacePreference(surface), menus, actions)
	fallbackPaths := fallbackPathsForSurfaces(ident, modules, p)
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
		"shell_kind":         "workspace",
		"surface":            surface,
		"available_surfaces": availableUISurfaces(ident, modules, p),
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
	}
}
