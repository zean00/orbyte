package httpx

import (
	"strings"

	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/module"
)

func defaultRouteForSurface(ident *identity.Service, userID, surface string, menus []module.MenuDefinition, actions []module.ActionDefinition) string {
	allowed := make(map[string]string, len(actions))
	for _, action := range actions {
		if route := strings.TrimSpace(action.RoutePath); route != "" {
			allowed[route] = action.Key
		}
	}
	if ident != nil && userID != "" {
		for _, candidate := range []string{
			ident.PreferredRoute(userID, surface),
			ident.DefaultRoute(userID, surface),
		} {
			candidate = strings.TrimSpace(candidate)
			if candidate == "" {
				continue
			}
			if _, ok := allowed[candidate]; ok {
				return candidate
			}
		}
	}
	if len(menus) == 0 {
		return ""
	}
	for _, action := range actions {
		if action.Key == menus[0].ActionKey {
			return action.RoutePath
		}
	}
	return ""
}
