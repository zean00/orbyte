package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"orbyte/internal/platform/logging"
	"orbyte/internal/platform/observability"
	"orbyte/internal/platform/policy"
	"orbyte/internal/platform/reporting"
	"orbyte/internal/platform/runtimehealth"
	"orbyte/internal/platform/securityfields"
	"orbyte/internal/platform/shared"
)

const (
	analyticsMCPStreamPath       = "/mcp/events/analytics/snapshot"
	analyticsScopedMCPStreamPath = "/mcp/analytics/events/analytics/snapshot"
)

func BuildRouter(deps RouterDeps) http.Handler {
	mux := http.NewServeMux()
	if deps.UI.FieldSecurity == nil {
		deps.UI.FieldSecurity = newFieldSecurity(deps.UI.Policy, deps.UI.Reporting)
	}
	if deps.Models.FieldSecurity == nil {
		deps.Models.FieldSecurity = deps.UI.FieldSecurity
	}
	if deps.Documents.FieldSecurity == nil {
		deps.Documents.FieldSecurity = deps.UI.FieldSecurity
	}
	registerPlatformRoutes(mux, deps.Platform, deps.CrossCutting.Health)
	registerAuthRoutesWithDeps(mux, deps.Auth)
	registerModelRoutesWithDeps(mux, deps.Models)
	registerDocumentRoutesWithDeps(mux, deps.Documents)
	registerOpsRoutesWithDeps(mux, deps.Ops)
	registerSearchRoutesWithDeps(mux, deps.Search)
	registerAdminRoutesWithDeps(mux, deps.Admin)
	registerACPRoutesWithDeps(mux, deps.ACP)
	registerTemplateRoutesWithDeps(mux, deps.Templates)
	registerMCPRoutesWithDeps(mux, deps.MCP)
	registerOfflineRoutesWithDeps(mux, deps.Offline)
	registerDocsRoutesWithDeps(mux, deps.Docs)
	registerDeepLinkRoutesWithDeps(mux, deps.DeepLinks)
	registerNotificationRoutesWithDeps(mux, deps.Notifications)
	registerLocaleRoutes(mux, deps.CrossCutting.Identity)
	registerUIRoutesWithDeps(mux, deps.UI)

	return withObservability(withAuthentication(withCSRFProtection(mux, deps.CrossCutting.Config), deps.CrossCutting.Identity), deps.CrossCutting.Logger, deps.CrossCutting.Observability)
}

func newFieldSecurity(policySvc *policy.Service, reportingSvc *reporting.Service) *securityfields.Service {
	fieldSecurity := securityfields.NewService(policySvc)
	if reportingSvc != nil {
		reportingSvc.AttachFieldSecurity(fieldSecurity)
	}
	return fieldSecurity
}

func registerCorePlatformRoutes(mux *http.ServeMux, deps PlatformDeps, health *runtimehealth.Tracker) {
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/ui", http.StatusSeeOther)
	})

	mux.HandleFunc("GET /favicon.ico", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		respondJSON(w, http.StatusOK, map[string]any{
			"status": "ok",
			"time":   time.Now().UTC(),
		})
	})

	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		snapshot := runtimehealth.Snapshot{Live: true, Ready: true, DependencyOK: true}
		if health != nil {
			ctx, cancel := context.WithTimeout(r.Context(), 500*time.Millisecond)
			defer cancel()
			snapshot = health.Snapshot(ctx)
		}
		status := http.StatusOK
		if !snapshot.Ready {
			status = http.StatusServiceUnavailable
		}
		respondJSON(w, status, snapshot)
	})

	mux.HandleFunc("GET /platform/context", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, deps.Identity, "platform.context.read", "", "platform.context.read"); !ok {
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{
			"organization":    deps.Organization.Root(),
			"config_keys":     deps.Config.Keys(),
			"reference_types": deps.Reference.Types(),
			"roles":           deps.Identity.Roles(),
			"document_types":  deps.Documents.DocumentTypes(),
			"workflows":       deps.Workflows.ListKeys(),
		})
	})
}

func respondJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func respondError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	kind := "internal_error"
	message := err.Error()

	var platformErr shared.Error
	if errors.As(err, &platformErr) {
		kind = string(platformErr.Kind)
		message = platformErr.Message
		switch platformErr.Kind {
		case shared.KindValidation:
			status = http.StatusBadRequest
		case shared.KindConflict:
			status = http.StatusConflict
		case shared.KindForbidden:
			status = http.StatusForbidden
		case shared.KindUnauthorized:
			status = http.StatusUnauthorized
		case shared.KindNotFound:
			status = http.StatusNotFound
		}
	}

	respondJSON(w, status, map[string]any{
		"error": map[string]any{
			"kind":    kind,
			"message": message,
		},
	})
}

func withObservability(next http.Handler, logger *logging.Service, obs *observability.Service) http.Handler {
	if obs == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		correlationID := r.Header.Get("X-Correlation-ID")
		if correlationID == "" {
			correlationID = time.Now().UTC().Format("20060102150405.000000000")
		}
		ctx := logging.WithCorrelationID(r.Context(), correlationID)
		r = r.WithContext(ctx)
		rw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		rw.Header().Set("X-Correlation-ID", correlationID)
		next.ServeHTTP(rw, r)
		obs.Inc("http.requests.total")
		obs.Inc("http.requests." + r.Method + ".total")
		obs.Inc("http.responses." + strconv.Itoa(rw.status) + ".total")
		routeFamily := routeFamilyForPath(r.URL.Path)
		statusFamily := strconv.Itoa(rw.status / 100)
		obs.Inc("http.route_family." + routeFamily + ".requests.total")
		obs.Inc("http.route_family." + routeFamily + ".responses." + statusFamily + "xx.total")
		duration := time.Since(started)
		obs.Observe("http.request.duration", duration)
		obs.Observe("http.route_family."+routeFamily+".request.duration", duration)
		_ = obs.ObserveMetric("http.request.duration", map[string]string{}, duration)
		_ = obs.EmitLogEvent("http.request.completed", map[string]any{
			"correlation_id": correlationID,
			"method":         r.Method,
			"path":           r.URL.Path,
			"status":         rw.status,
		})
		if logger != nil {
			logger.Info("http request completed", map[string]any{
				"correlation_id": correlationID,
				"method":         r.Method,
				"path":           r.URL.Path,
				"status":         rw.status,
				"duration_ms":    duration.Milliseconds(),
			})
		}
	})
}

func routeFamilyForPath(path string) string {
	switch {
	case path == "/healthz" || path == "/readyz":
		return "health"
	case path == "/platform/context":
		return "platform"
	case path == "/locale":
		return "platform"
	case path == "/metrics":
		return "metrics"
	case len(path) >= 6 && path[:6] == "/auth/":
		return "auth"
	case len(path) >= 6 && path[:6] == "/admin/":
		return "admin"
	case len(path) >= 5 && path[:5] == "/ops/":
		return "ops"
	case len(path) >= 11 && path[:11] == "/documents/" || path == "/documents":
		return "documents"
	case len(path) >= 8 && path[:8] == "/models/" || path == "/models":
		return "models"
	case len(path) >= 8 && path[:8] == "/search/" || path == "/search":
		return "search"
	case len(path) >= 5 && path[:5] == "/mcp/":
		return "mcp"
	case path == "/mcp":
		return "mcp"
	case len(path) >= 9 && path[:9] == "/offline/":
		return "offline"
	case path == "/offline":
		return "offline"
	case len(path) >= 4 && path[:4] == "/ui/":
		return "ui"
	default:
		return "other"
	}
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Flush() {
	flusher, ok := w.ResponseWriter.(http.Flusher)
	if !ok {
		return
	}
	flusher.Flush()
}
