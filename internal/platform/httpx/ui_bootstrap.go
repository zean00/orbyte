package httpx

import (
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"orbyte/internal/platform/acp"
	"orbyte/internal/platform/i18n"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/module"
)

const workspaceBootstrapCacheTTL = 15 * time.Second
const workspaceBootstrapCacheSweepInterval = time.Minute

type cachedWorkspaceBootstrap struct {
	payload   map[string]any
	expiresAt time.Time
}

var workspaceBootstrapCache sync.Map
var workspaceBootstrapCacheSweep struct {
	mu       sync.Mutex
	lastScan time.Time
}

func loadCachedWorkspaceBootstrap(cacheKey string, now time.Time) (map[string]any, bool) {
	cached, ok := workspaceBootstrapCache.Load(cacheKey)
	if !ok {
		return nil, false
	}
	entry, _ := cached.(cachedWorkspaceBootstrap)
	if now.Before(entry.expiresAt) && entry.payload != nil {
		return entry.payload, true
	}
	workspaceBootstrapCache.Delete(cacheKey)
	return nil, false
}

func sweepExpiredWorkspaceBootstrapEntries(now time.Time) {
	workspaceBootstrapCacheSweep.mu.Lock()
	if !workspaceBootstrapCacheSweep.lastScan.IsZero() && now.Sub(workspaceBootstrapCacheSweep.lastScan) < workspaceBootstrapCacheSweepInterval {
		workspaceBootstrapCacheSweep.mu.Unlock()
		return
	}
	workspaceBootstrapCacheSweep.lastScan = now
	workspaceBootstrapCacheSweep.mu.Unlock()

	workspaceBootstrapCache.Range(func(key, value any) bool {
		entry, _ := value.(cachedWorkspaceBootstrap)
		if !now.Before(entry.expiresAt) {
			workspaceBootstrapCache.Delete(key)
		}
		return true
	})
}

func buildWorkspaceBootstrapPayload(r *http.Request, ident *identity.Service, modules *module.Service, p principal, surface module.UISurface, uiPrefs *UIPreferencesService, acpSvc *acp.Service) map[string]any {
	locale := localeFromRequest(r, ident)
	if workspaceBootstrapCachingEnabled() {
		now := time.Now()
		sweepExpiredWorkspaceBootstrapEntries(now)
		cacheKey := workspaceBootstrapCacheKey(p, surface, locale)
		if payload, ok := loadCachedWorkspaceBootstrap(cacheKey, now); ok {
			return payload
		}
	}

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
	payload := map[string]any{
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
		"locale":            locale,
		"supported_locales": i18n.SupportedLocales(),
		"auth_context": map[string]any{
			"actor_user_id":       p.userID,
			"effective_user_id":   principalEffectiveUserID(p),
			"delegation_active":   principalHasDelegation(p),
			"delegation_grant_id": principalDelegationGrantID(p),
		},
		"acp": buildACPBootstrap(acpSvc),
	}
	if workspaceBootstrapCachingEnabled() {
		workspaceBootstrapCache.Store(workspaceBootstrapCacheKey(p, surface, locale), cachedWorkspaceBootstrap{
			payload:   payload,
			expiresAt: time.Now().Add(workspaceBootstrapCacheTTL),
		})
	}
	return payload
}

func workspaceBootstrapCacheKey(p principal, surface module.UISurface, locale string) string {
	return strings.Join([]string{
		string(surface),
		locale,
		p.userID,
		principalEffectiveUserID(p),
		principalDelegationGrantID(p),
		principalDeepLinkGrantID(p),
		p.currentLocationID,
	}, "::")
}

func workspaceBootstrapCachingEnabled() bool {
	return !strings.HasSuffix(os.Args[0], ".test")
}
