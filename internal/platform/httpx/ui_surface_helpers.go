package httpx

import (
	"net/http"
	"strings"

	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/shared"
)

func splitCSV(raw string) []string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	parts := strings.Split(trimmed, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func requireInteractivePrincipal(w http.ResponseWriter, r *http.Request) (principal, bool) {
	if err := authError(r); err != nil {
		respondError(w, err)
		return principal{}, false
	}
	p, ok := currentPrincipal(r)
	if !ok {
		respondError(w, shared.Unauthorized("authentication required"))
		return principal{}, false
	}
	if p.kind != userPrincipal {
		respondError(w, shared.Forbidden("interactive user session is required"))
		return principal{}, false
	}
	return p, true
}

func principalAllowsAll(ident *identity.Service, p principal, permissions []string) bool {
	for _, permission := range permissions {
		if strings.TrimSpace(permission) == "" {
			continue
		}
		if !principalAllowsPermission(ident, p, permission, p.currentLocationID) {
			return false
		}
	}
	return true
}
