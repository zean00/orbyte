package httpx

import (
	"strings"
	"sync"
)

type UIPreferencesService struct {
	mu    sync.RWMutex
	items map[string]UIPreferences
}

type UIPreferences struct {
	Surface     string         `json:"surface"`
	RoutePath   string         `json:"route_path"`
	ViewKey     string         `json:"view_key,omitempty"`
	Filters     map[string]any `json:"filters,omitempty"`
	Columns     []string       `json:"columns,omitempty"`
	ColumnOrder []string       `json:"column_order,omitempty"`
	Density     string         `json:"density,omitempty"`
}

func NewUIPreferencesService() *UIPreferencesService {
	return &UIPreferencesService{items: map[string]UIPreferences{}}
}

func (s *UIPreferencesService) Get(userID, surface, routePath string) UIPreferences {
	if s == nil {
		return normalizeUIPreferences(UIPreferences{Surface: surface, RoutePath: routePath})
	}
	key := uiPreferencesKey(userID, surface, routePath)
	s.mu.RLock()
	item, ok := s.items[key]
	s.mu.RUnlock()
	if !ok {
		return normalizeUIPreferences(UIPreferences{Surface: surface, RoutePath: routePath})
	}
	return normalizeUIPreferences(item)
}

func (s *UIPreferencesService) Put(userID string, prefs UIPreferences) UIPreferences {
	normalized := normalizeUIPreferences(prefs)
	if s == nil {
		return normalized
	}
	key := uiPreferencesKey(userID, normalized.Surface, normalized.RoutePath)
	s.mu.Lock()
	s.items[key] = normalized
	s.mu.Unlock()
	return normalized
}

func uiPreferencesKey(userID, surface, routePath string) string {
	return strings.TrimSpace(userID) + "|" + strings.TrimSpace(surface) + "|" + normalizeUIRoutePath(routePath)
}

func normalizeUIPreferences(prefs UIPreferences) UIPreferences {
	prefs.Surface = strings.ToLower(strings.TrimSpace(prefs.Surface))
	prefs.RoutePath = normalizeUIRoutePath(prefs.RoutePath)
	prefs.ViewKey = strings.TrimSpace(prefs.ViewKey)
	if prefs.Filters == nil {
		prefs.Filters = map[string]any{}
	}
	if prefs.Columns == nil {
		prefs.Columns = []string{}
	}
	if prefs.ColumnOrder == nil {
		prefs.ColumnOrder = []string{}
	}
	switch strings.ToLower(strings.TrimSpace(prefs.Density)) {
	case "compact", "comfortable":
		prefs.Density = strings.ToLower(strings.TrimSpace(prefs.Density))
	default:
		prefs.Density = "comfortable"
	}
	return prefs
}

func normalizeUIRoutePath(routePath string) string {
	value := strings.TrimSpace(routePath)
	if value == "" {
		return ""
	}
	if !strings.HasPrefix(value, "/") {
		value = "/" + value
	}
	return value
}
