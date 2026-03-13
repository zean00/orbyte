package httpx

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"clinic/internal/platform/activity"
	"clinic/internal/platform/analytics"
	application "clinic/internal/platform/application"
	"clinic/internal/platform/audit"
	"clinic/internal/platform/config"
	"clinic/internal/platform/document"
	"clinic/internal/platform/eventing"
	"clinic/internal/platform/identity"
	"clinic/internal/platform/integration"
	"clinic/internal/platform/jobs"
	"clinic/internal/platform/logging"
	"clinic/internal/platform/model"
	"clinic/internal/platform/module"
	"clinic/internal/platform/monitoring"
	"clinic/internal/platform/observability"
	"clinic/internal/platform/organization"
	"clinic/internal/platform/policy"
	"clinic/internal/platform/reporting"
	"clinic/internal/platform/search"
	"clinic/internal/platform/securityfields"
	"clinic/internal/platform/shared"
	"clinic/internal/platform/workflow"
)

func NewRouter(cfg *config.Service, org *organization.Service, ident *identity.Service, modules *module.Service, models *model.Service, activities *activity.Service, reportingSvc *reporting.Service, docs *document.Service, flows *workflow.Service, auditSvc *audit.Service, eventingSvc *eventing.Service, searchSvc *search.Service, loggerSvc *logging.Service, analyticsSvc *analytics.Service, monitoringSvc *monitoring.Service, obsSvc *observability.Service, policySvc *policy.Service, integrationSvc *integration.Service, jobSvc *jobs.Service, docActions *application.DocumentActions, modelActions *application.ModelActions) http.Handler {
	mux := http.NewServeMux()
	fieldSecurity := securityfields.NewService(policySvc)
	reportingSvc.AttachFieldSecurity(fieldSecurity)

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		respondJSON(w, http.StatusOK, map[string]any{
			"status": "ok",
			"time":   time.Now().UTC(),
		})
	})

	mux.HandleFunc("GET /platform/context", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "platform.context.read", "", "platform.context.read"); !ok {
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{
			"organization":   org.Root(),
			"config_keys":    cfg.Keys(),
			"roles":          ident.Roles(),
			"document_types": docs.DocumentTypes(),
			"workflows":      flows.ListKeys(),
		})
	})

	registerAuthRoutes(mux, cfg, ident, auditSvc)
	registerModelRoutes(mux, ident, models, activities, policySvc, fieldSecurity, modelActions)
	registerDocumentRoutes(mux, ident, modules, docs, docActions, policySvc, fieldSecurity, obsSvc)
	registerOpsRoutes(mux, ident, auditSvc, eventingSvc, docs, searchSvc, flows, analyticsSvc, monitoringSvc, obsSvc, jobSvc)
	registerSearchRoutes(mux, ident, searchSvc, jobSvc)
	registerAdminRoutes(mux, cfg, org, ident, modules, auditSvc, policySvc, obsSvc, integrationSvc)
	registerUIRoutes(mux, ident, modules, models, activities, reportingSvc, docs, searchSvc, analyticsSvc, monitoringSvc, policySvc, fieldSecurity)

	return withObservability(withAuthentication(withCSRFProtection(mux, cfg), ident), loggerSvc, obsSvc)
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
		duration := time.Since(started)
		obs.Observe("http.request.duration", duration)
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

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
