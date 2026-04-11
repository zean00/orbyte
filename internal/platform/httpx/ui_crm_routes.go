package httpx

import (
	"net/http"
	"time"

	"orbyte/internal/platform/application"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/shared"
)

func registerUICRMRoutes(mux *http.ServeMux, ident *identity.Service, crmSvc *application.CRMCoreService) {
	if crmSvc == nil {
		return
	}

	mux.HandleFunc("GET /ui/data/crm/tickets/summary", func(w http.ResponseWriter, r *http.Request) {
		handleCRMServiceSummary(w, r, ident, crmSvc)
	})
	mux.HandleFunc("GET /ui/data/crm/service/summary", func(w http.ResponseWriter, r *http.Request) {
		handleCRMServiceSummary(w, r, ident, crmSvc)
	})
	mux.HandleFunc("GET /ui/data/crm/sales/summary", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		if !principalAllowsAll(ident, p, []string{"crm_opportunity.list", "crm_lead.list", "crm_activity.list"}) {
			respondError(w, shared.Forbidden("crm sales summary is not allowed"))
			return
		}
		payload, err := crmSvc.SalesSummaryPayload(time.Now().UTC())
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, payload)
	})
	mux.HandleFunc("GET /ui/data/crm/customers/health", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		if !principalAllowsAll(ident, p, []string{"crm_ticket.list", "crm_opportunity.list", "crm_activity.list", "party.read", "customer.read", "party_contact.read"}) {
			respondError(w, shared.Forbidden("crm customer health is not allowed"))
			return
		}
		payload, err := crmSvc.CustomerHealthPayload(time.Now().UTC())
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, payload)
	})
	mux.HandleFunc("GET /ui/data/crm/customers/360/{party_id}", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		if !principalAllowsAll(ident, p, []string{"crm_ticket.list", "crm_opportunity.list", "crm_activity.list", "party.read", "customer.read", "party_contact.read"}) {
			respondError(w, shared.Forbidden("crm customer summary is not allowed"))
			return
		}
		partyID := r.PathValue("party_id")
		if partyID == "" {
			respondError(w, shared.Validation("party_id is required"))
			return
		}
		payload, err := crmSvc.Customer360(partyID, time.Now().UTC())
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, payload)
	})
	mux.HandleFunc("GET /ui/data/crm/tickets/{ticket_id}/timeline", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		if !principalAllowsAll(ident, p, []string{"crm_ticket.list"}) {
			respondError(w, shared.Forbidden("crm ticket timeline is not allowed"))
			return
		}
		ticketID := r.PathValue("ticket_id")
		if ticketID == "" {
			respondError(w, shared.Validation("ticket_id is required"))
			return
		}
		payload, err := crmSvc.TicketTimeline(ticketID)
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{
			"ticket_id": ticketID,
			"items":     payload,
		})
	})
}

func handleCRMServiceSummary(w http.ResponseWriter, r *http.Request, ident *identity.Service, crmSvc *application.CRMCoreService) {
	p, ok := requireInteractivePrincipal(w, r)
	if !ok {
		return
	}
	if !principalAllowsAll(ident, p, []string{"crm_ticket.list"}) {
		respondError(w, shared.Forbidden("crm ticket summary is not allowed"))
		return
	}
	payload, err := crmSvc.SummaryPayload(time.Now().UTC())
	if err != nil {
		respondError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, payload)
}
