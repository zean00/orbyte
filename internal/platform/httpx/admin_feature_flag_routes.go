package httpx

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"orbyte/internal/platform/audit"
	"orbyte/internal/platform/featureflags"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/organization"
	"orbyte/internal/platform/shared"
)

func registerAdminFeatureFlagRoutes(mux *http.ServeMux, flags *featureflags.Service, org *organization.Service, ident *identity.Service, auditSvc *audit.Service) {
	mux.HandleFunc("GET /admin/api/feature-flags/definitions", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "configuration.read", "", "configuration.read"); !ok {
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": flags.Definitions()})
	})

	mux.HandleFunc("GET /admin/api/feature-flags/values", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "configuration.read", "", "configuration.read"); !ok {
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": flags.Values()})
	})

	mux.HandleFunc("GET /admin/api/feature-flags/effective", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "configuration.read", "", "configuration.read"); !ok {
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{
			"items": flags.ResolveAllWithOperatingUnit(strings.TrimSpace(r.URL.Query().Get("organization_id")), strings.TrimSpace(r.URL.Query().Get("location_id")), strings.TrimSpace(r.URL.Query().Get("operating_unit_id")), time.Now().UTC()),
		})
	})

	mux.HandleFunc("GET /admin/api/feature-flags/targeting", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "configuration.read", "", "configuration.read"); !ok {
			return
		}
		flagKey := strings.TrimSpace(r.URL.Query().Get("flag_key"))
		if flagKey == "" {
			respondError(w, shared.Validation("flag_key is required"))
			return
		}
		view, ok := flags.TargetingView(flagKey, strings.TrimSpace(r.URL.Query().Get("organization_id")), strings.TrimSpace(r.URL.Query().Get("location_id")), strings.TrimSpace(r.URL.Query().Get("operating_unit_id")), time.Now().UTC())
		if !ok {
			respondError(w, shared.NotFound("feature flag definition not found"))
			return
		}
		respondJSON(w, http.StatusOK, view)
	})

	mux.HandleFunc("GET /admin/api/operating-units", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "configuration.read", "", "configuration.read"); !ok {
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": org.OperatingUnits()})
	})

	mux.HandleFunc("PUT /admin/api/operating-units/", func(w http.ResponseWriter, r *http.Request) {
		unitID, ok := adminOperatingUnitPath(r.URL.Path)
		if !ok {
			respondError(w, shared.NotFound("operating unit not found"))
			return
		}
		p, ok := requireAuthorization(w, r, ident, "configuration.manage", "", "configuration.manage")
		if !ok {
			return
		}
		var req struct {
			OrganizationID string `json:"organization_id,omitempty"`
			LocationID     string `json:"location_id,omitempty"`
			Key            string `json:"key"`
			Name           string `json:"name"`
			Status         string `json:"status,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, shared.Validation("invalid operating unit request"))
			return
		}
		unit, err := org.UpsertOperatingUnit(organization.OperatingUnit{
			ID:             unitID,
			OrganizationID: strings.TrimSpace(req.OrganizationID),
			LocationID:     strings.TrimSpace(req.LocationID),
			Key:            strings.TrimSpace(req.Key),
			Name:           strings.TrimSpace(req.Name),
			Status:         strings.TrimSpace(req.Status),
		})
		if err != nil {
			respondError(w, err)
			return
		}
		recordAudit(auditSvc, audit.Event{
			ID:             "audit:organization:operating_unit:update:" + unit.ID + ":" + time.Now().UTC().Format("20060102150405.000000000"),
			Action:         "organization.operating_unit.update",
			TargetType:     "operating_unit",
			TargetID:       unit.ID,
			ActorID:        principalActorID(p),
			ActorKind:      principalActorKind(p),
			OrganizationID: unit.OrganizationID,
			LocationID:     unit.LocationID,
			OccurredAt:     time.Now().UTC(),
			ChangeSummary:  map[string]any{"fields": []string{"key", "name", "status", "location_id"}},
			Metadata:       map[string]any{"key": unit.Key, "name": unit.Name, "status": unit.Status},
		})
		respondJSON(w, http.StatusOK, unit)
	})

	mux.HandleFunc("PUT /admin/api/feature-flags/", func(w http.ResponseWriter, r *http.Request) {
		flagKey, ok := adminFeatureFlagPath(r.URL.Path)
		if !ok {
			respondError(w, shared.NotFound("feature flag not found"))
			return
		}
		p, ok := requireAuthorization(w, r, ident, "configuration.manage", "", "configuration.manage")
		if !ok {
			return
		}
		var req struct {
			Scope         string    `json:"scope"`
			ScopeID       string    `json:"scope_id,omitempty"`
			Enabled       bool      `json:"enabled"`
			Status        string    `json:"status,omitempty"`
			EffectiveFrom time.Time `json:"effective_from,omitempty"`
			EffectiveTo   time.Time `json:"effective_to,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, shared.Validation("invalid feature flag request"))
			return
		}
		if err := flags.UpsertValue(featureflags.Value{
			FlagKey:       flagKey,
			Scope:         strings.TrimSpace(req.Scope),
			ScopeID:       strings.TrimSpace(req.ScopeID),
			Enabled:       req.Enabled,
			Status:        strings.TrimSpace(req.Status),
			UpdatedBy:     principalActorID(p),
			EffectiveFrom: req.EffectiveFrom,
			EffectiveTo:   req.EffectiveTo,
		}); err != nil {
			respondError(w, err)
			return
		}
		recordAudit(auditSvc, audit.Event{
			ID:            "audit:feature_flag:update:" + flagKey + ":" + time.Now().UTC().Format("20060102150405.000000000"),
			Action:        "feature_flag.update",
			TargetType:    "feature_flag",
			TargetID:      flagKey,
			ActorID:       principalActorID(p),
			ActorKind:     principalActorKind(p),
			OccurredAt:    time.Now().UTC(),
			ChangeSummary: map[string]any{"fields": []string{"scope", "scope_id", "enabled", "status"}},
			Metadata:      map[string]any{"scope": req.Scope, "scope_id": req.ScopeID, "enabled": req.Enabled, "status": req.Status},
		})
		orgID := ""
		locationID := ""
		operatingUnitID := ""
		if req.Scope == "organization" {
			orgID = req.ScopeID
		}
		if req.Scope == "location" {
			locationID = req.ScopeID
		}
		if req.Scope == "operating_unit" {
			operatingUnitID = req.ScopeID
		}
		effective, _ := flags.ResolveWithOperatingUnit(flagKey, orgID, locationID, operatingUnitID, time.Now().UTC())
		respondJSON(w, http.StatusOK, effective)
	})

}
