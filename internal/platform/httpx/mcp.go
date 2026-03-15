package httpx

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"orbyte/internal/platform/analytics"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/mcp"
	"orbyte/internal/platform/shared"
)

func registerMCPRoutes(mux *http.ServeMux, ident *identity.Service, server *mcp.Server, analyticsSvc *analytics.Service, stream *mcp.AnalyticsStream, streamPath string) {
	mux.HandleFunc("POST /mcp", func(w http.ResponseWriter, r *http.Request) {
		if server == nil {
			respondError(w, shared.NotFound("mcp server is not configured"))
			return
		}
		if err := authError(r); err != nil {
			respondError(w, err)
			return
		}
		p, ok := currentPrincipal(r)
		if !ok {
			respondError(w, shared.Unauthorized("authentication required"))
			return
		}
		var req mcp.JSONRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, shared.Validation("invalid mcp request payload"))
			return
		}
		actor := mcp.ActorContext{
			ActorID:    principalActorID(p),
			SessionID:  p.sessionID,
			LocationID: p.currentLocationID,
			PermissionChecker: func(permissionKey string) bool {
				return principalAllowsAll(ident, p, []string{permissionKey})
			},
		}
		respondJSON(w, http.StatusOK, server.Handle(r.Context(), req, actor))
	})
	if strings.TrimSpace(streamPath) == "" {
		return
	}
	mux.HandleFunc("GET "+streamPath, func(w http.ResponseWriter, r *http.Request) {
		if stream == nil || analyticsSvc == nil {
			respondError(w, shared.NotFound("mcp analytics stream is not configured"))
			return
		}
		if err := authError(r); err != nil {
			respondError(w, err)
			return
		}
		p, ok := currentPrincipal(r)
		if !ok {
			respondError(w, shared.Unauthorized("authentication required"))
			return
		}
		if !principalAllowsAll(ident, p, []string{"analytics.read"}) {
			respondError(w, shared.Forbidden("analytics.read is required"))
			return
		}
		flusher, ok := w.(http.Flusher)
		if !ok {
			respondError(w, shared.Conflict("streaming is not supported"))
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		if latest, ok := stream.Latest(); ok {
			if err := writeAnalyticsEvent(w, "snapshot", latest); err != nil {
				return
			}
			flusher.Flush()
		} else if snapshot, err := analyticsSvc.CaptureSnapshot(); err == nil {
			if err := writeAnalyticsEvent(w, "snapshot", snapshot); err != nil {
				return
			}
			flusher.Flush()
		}

		events, unsubscribe := stream.Subscribe()
		defer unsubscribe()
		heartbeat := time.NewTicker(20 * time.Second)
		defer heartbeat.Stop()

		for {
			select {
			case <-r.Context().Done():
				return
			case snapshot, ok := <-events:
				if !ok {
					return
				}
				if err := writeAnalyticsEvent(w, "snapshot", snapshot); err != nil {
					return
				}
				flusher.Flush()
			case tick := <-heartbeat.C:
				if _, err := fmt.Fprintf(w, "event: heartbeat\ndata: {\"time\":%q}\n\n", tick.UTC().Format(time.RFC3339)); err != nil {
					return
				}
				flusher.Flush()
			}
		}
	})
}

func writeAnalyticsEvent(w http.ResponseWriter, event string, snapshot analytics.Snapshot) error {
	body, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, body)
	return err
}
