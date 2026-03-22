package httpx

import (
	"net/http"

	"orbyte/internal/platform/acp"
	"orbyte/internal/platform/i18n"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/organization"
)

func buildAdminBootstrapPayload(r *http.Request, org *organization.Service, ident *identity.Service, modules *module.Service, p principal, acpSvc *acp.Service) map[string]any {
	menus, actions, views, entries, _ := visibleUIContracts(ident, modules, p, module.UISurfaceAdmin)
	uiMenus, uiActions, _, _, _ := visibleUIContracts(ident, modules, p, module.UISurfaceUser)
	defaultPath := defaultRouteForSurface(ident, p.userID, "admin", menus, actions)
	uiPath := "/ui"
	if len(uiMenus) > 0 {
		for _, action := range uiActions {
			if action.Key == uiMenus[0].ActionKey {
				uiPath = "/ui#" + action.RoutePath
				break
			}
		}
	}
	return map[string]any{
		"shell_kind":      "admin",
		"organization":    org.Root(),
		"locations":       org.Locations(),
		"operating_units": org.OperatingUnits(),
		"roles":           ident.Roles(),
		"menus":           menus,
		"actions":         actions,
		"user_actions":    uiActions,
		"views":           views,
		"custom_entries":  entries,
		"default_path":    defaultPath,
		"preferred_path":  ident.PreferredRoute(p.userID, "admin"),
		"ui_access":       len(uiMenus) > 0,
		"ui_path":         uiPath,
		"current_user_id": p.userID,
		"capabilities": map[string]any{
			"workspace_link": true,
			"shell_recovery": true,
		},
		"locale":            localeFromRequest(r, ident),
		"supported_locales": i18n.SupportedLocales(),
		"acp":               buildACPBootstrap(acpSvc),
	}
}
