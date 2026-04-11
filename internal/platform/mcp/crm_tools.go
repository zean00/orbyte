package mcp

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"orbyte/internal/platform/shared"
)

func (s *Server) crmTicketSummary(actor ActorContext) (map[string]any, error) {
	if s == nil || s.crm == nil {
		return nil, fmt.Errorf("crm is unavailable")
	}
	payload, err := s.crm.SummaryPayload(time.Now().UTC())
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"content": []ContentBlock{{
			Type: "text",
			Text: fmt.Sprintf("CRM summary loaded with %d open tickets and %d overdue tickets.", crmNestedInt(payload, "overview", "open_tickets"), crmNestedInt(payload, "overview", "overdue_tickets")),
		}},
		"structuredContent": payload,
	}, nil
}

func (s *Server) crmTicketSearch(actor ActorContext, arguments map[string]any) (map[string]any, error) {
	if s == nil || s.crm == nil {
		return nil, fmt.Errorf("crm is unavailable")
	}
	filters := map[string]string{}
	for _, key := range []string{"queue_code", "status", "priority", "party_id", "assignee_user_id"} {
		if value := strings.TrimSpace(stringArg(arguments, key)); value != "" {
			filters[key] = value
		}
	}
	page := positiveIntArg(arguments, "page", 1)
	pageSize := positiveIntArg(arguments, "page_size", 20)
	items, total, err := s.crm.SearchTickets(filters, stringArg(arguments, "query"), page, pageSize)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"content": []ContentBlock{{
			Type: "text",
			Text: fmt.Sprintf("Found %d CRM tickets.", total),
		}},
		"structuredContent": map[string]any{
			"items":     items,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		},
	}, nil
}

func (s *Server) crmTicketGet(actor ActorContext, arguments map[string]any) (map[string]any, error) {
	if s == nil || s.crm == nil {
		return nil, fmt.Errorf("crm is unavailable")
	}
	ticketID := strings.TrimSpace(stringArg(arguments, "ticket_id"))
	if ticketID == "" {
		return nil, shared.Validation("ticket_id is required")
	}
	item, err := s.crm.GetTicket(ticketID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"content": []ContentBlock{{
			Type: "text",
			Text: fmt.Sprintf("Loaded CRM ticket %s.", ticketID),
		}},
		"structuredContent": item,
	}, nil
}

func (s *Server) crmTicketCreate(actor ActorContext, arguments map[string]any) (map[string]any, error) {
	if s == nil || s.crm == nil {
		return nil, fmt.Errorf("crm is unavailable")
	}
	if !boolArg(arguments, "confirm_apply") {
		return nil, shared.Validation("confirm_apply must be true to create a crm ticket")
	}
	values := crmMutationValues(arguments, []string{
		"title", "description", "party_id", "party_name", "queue_code", "priority", "severity", "source_channel", "assignee_user_id", "due_at", "first_response_due_at", "issue_category", "tags_json",
	})
	item, err := s.crm.CreateTicket(actor.EffectiveUserID, values)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"content": []ContentBlock{{
			Type: "text",
			Text: fmt.Sprintf("Created CRM ticket %s.", strings.TrimSpace(fmt.Sprintf("%v", item.Values["ticket_number"]))),
		}},
		"structuredContent": item,
	}, nil
}

func (s *Server) crmTicketUpdate(actor ActorContext, arguments map[string]any) (map[string]any, error) {
	if s == nil || s.crm == nil {
		return nil, fmt.Errorf("crm is unavailable")
	}
	if !boolArg(arguments, "confirm_apply") {
		return nil, shared.Validation("confirm_apply must be true to update a crm ticket")
	}
	ticketID := strings.TrimSpace(stringArg(arguments, "ticket_id"))
	if ticketID == "" {
		return nil, shared.Validation("ticket_id is required")
	}
	values := crmMutationValues(arguments, []string{
		"title", "description", "queue_code", "status", "priority", "severity", "assignee_user_id", "first_response_at", "resolved_at", "resolution_notes", "issue_category", "tags_json",
	})
	item, err := s.crm.UpdateTicket(ticketID, actor.EffectiveUserID, values, positiveIntArg(arguments, "expected_version", 0))
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"content": []ContentBlock{{
			Type: "text",
			Text: fmt.Sprintf("Updated CRM ticket %s.", strings.TrimSpace(fmt.Sprintf("%v", item.Values["ticket_number"]))),
		}},
		"structuredContent": item,
	}, nil
}

func (s *Server) crmTicketCommentCreate(actor ActorContext, arguments map[string]any) (map[string]any, error) {
	if s == nil || s.crm == nil {
		return nil, fmt.Errorf("crm is unavailable")
	}
	if !boolArg(arguments, "confirm_apply") {
		return nil, shared.Validation("confirm_apply must be true to add a crm ticket comment")
	}
	ticketID := strings.TrimSpace(stringArg(arguments, "ticket_id"))
	body := strings.TrimSpace(stringArg(arguments, "body"))
	if ticketID == "" {
		return nil, shared.Validation("ticket_id is required")
	}
	if body == "" {
		return nil, shared.Validation("body is required")
	}
	item, err := s.crm.AddTicketComment(ticketID, actor.EffectiveUserID, body, strings.TrimSpace(stringArg(arguments, "comment_type")))
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Added CRM ticket comment for %s.", ticketID)}},
		"structuredContent": item,
	}, nil
}

func (s *Server) crmTicketAssign(actor ActorContext, arguments map[string]any) (map[string]any, error) {
	if s == nil || s.crm == nil {
		return nil, fmt.Errorf("crm is unavailable")
	}
	if !boolArg(arguments, "confirm_apply") {
		return nil, shared.Validation("confirm_apply must be true to assign a crm ticket")
	}
	ticketID := strings.TrimSpace(stringArg(arguments, "ticket_id"))
	assigneeUserID := strings.TrimSpace(stringArg(arguments, "assignee_user_id"))
	if ticketID == "" || assigneeUserID == "" {
		return nil, shared.Validation("ticket_id and assignee_user_id are required")
	}
	item, err := s.crm.AssignTicket(ticketID, actor.EffectiveUserID, assigneeUserID, strings.TrimSpace(stringArg(arguments, "note")), positiveIntArg(arguments, "expected_version", 0))
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Assigned CRM ticket %s.", ticketID)}},
		"structuredContent": item,
	}, nil
}

func (s *Server) crmTicketResolve(actor ActorContext, arguments map[string]any) (map[string]any, error) {
	if s == nil || s.crm == nil {
		return nil, fmt.Errorf("crm is unavailable")
	}
	if !boolArg(arguments, "confirm_apply") {
		return nil, shared.Validation("confirm_apply must be true to resolve a crm ticket")
	}
	ticketID := strings.TrimSpace(stringArg(arguments, "ticket_id"))
	if ticketID == "" {
		return nil, shared.Validation("ticket_id is required")
	}
	item, err := s.crm.ResolveTicket(ticketID, actor.EffectiveUserID, strings.TrimSpace(stringArg(arguments, "resolution_notes")), boolArg(arguments, "close"), positiveIntArg(arguments, "expected_version", 0))
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Resolved CRM ticket %s.", ticketID)}},
		"structuredContent": item,
	}, nil
}

func (s *Server) crmCustomerSummary(actor ActorContext, arguments map[string]any) (map[string]any, error) {
	if s == nil || s.crm == nil {
		return nil, fmt.Errorf("crm is unavailable")
	}
	partyID := strings.TrimSpace(stringArg(arguments, "party_id"))
	if partyID == "" {
		return nil, shared.Validation("party_id is required")
	}
	payload, err := s.crm.Customer360(partyID, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded CRM customer 360 for %s.", partyID)}},
		"structuredContent": payload,
	}, nil
}

func (s *Server) crmCustomerTimeline(actor ActorContext, arguments map[string]any) (map[string]any, error) {
	if s == nil || s.crm == nil {
		return nil, fmt.Errorf("crm is unavailable")
	}
	partyID := strings.TrimSpace(stringArg(arguments, "party_id"))
	if partyID == "" {
		return nil, shared.Validation("party_id is required")
	}
	payload, err := s.crm.Customer360(partyID, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded CRM customer timeline for %s.", partyID)}},
		"structuredContent": map[string]any{
			"party_id":      partyID,
			"tickets":       payload["tickets"],
			"activities":    payload["activities"],
			"opportunities": payload["opportunities"],
		},
	}, nil
}

func (s *Server) crmCustomerHealth(actor ActorContext) (map[string]any, error) {
	if s == nil || s.crm == nil {
		return nil, fmt.Errorf("crm is unavailable")
	}
	payload, err := s.crm.CustomerHealthPayload(time.Now().UTC())
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded CRM customer health for %d at-risk customers.", crmNestedInt(payload, "overview", "customers_with_open_issues"))}},
		"structuredContent": payload,
	}, nil
}

func (s *Server) crmLeadSearch(actor ActorContext, arguments map[string]any) (map[string]any, error) {
	if s == nil || s.crm == nil {
		return nil, fmt.Errorf("crm is unavailable")
	}
	filters := map[string]string{}
	for _, key := range []string{"status", "rating", "party_id", "owner_user_id"} {
		if value := strings.TrimSpace(stringArg(arguments, key)); value != "" {
			filters[key] = value
		}
	}
	page := positiveIntArg(arguments, "page", 1)
	pageSize := positiveIntArg(arguments, "page_size", 20)
	items, total, err := s.crm.SearchLeads(filters, stringArg(arguments, "query"), page, pageSize)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Found %d CRM leads.", total)}},
		"structuredContent": map[string]any{"items": items, "total": total, "page": page, "page_size": pageSize},
	}, nil
}

func (s *Server) crmLeadGet(actor ActorContext, arguments map[string]any) (map[string]any, error) {
	if s == nil || s.crm == nil {
		return nil, fmt.Errorf("crm is unavailable")
	}
	leadID := strings.TrimSpace(stringArg(arguments, "lead_id"))
	if leadID == "" {
		return nil, shared.Validation("lead_id is required")
	}
	item, err := s.crm.GetLead(leadID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded CRM lead %s.", leadID)}},
		"structuredContent": item,
	}, nil
}

func (s *Server) crmLeadCreate(actor ActorContext, arguments map[string]any) (map[string]any, error) {
	if s == nil || s.crm == nil {
		return nil, fmt.Errorf("crm is unavailable")
	}
	if !boolArg(arguments, "confirm_apply") {
		return nil, shared.Validation("confirm_apply must be true to create a crm lead")
	}
	values := crmMutationValues(arguments, []string{"title", "party_id", "party_name", "contact_id", "owner_user_id", "source_channel", "status", "rating", "estimated_value", "expected_close_date", "next_action_at", "notes"})
	item, err := s.crm.CreateLead(actor.EffectiveUserID, values)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Created CRM lead %s.", crmToolTextValue(item.Values["lead_number"]))}},
		"structuredContent": item,
	}, nil
}

func (s *Server) crmLeadUpdate(actor ActorContext, arguments map[string]any) (map[string]any, error) {
	if s == nil || s.crm == nil {
		return nil, fmt.Errorf("crm is unavailable")
	}
	if !boolArg(arguments, "confirm_apply") {
		return nil, shared.Validation("confirm_apply must be true to update a crm lead")
	}
	leadID := strings.TrimSpace(stringArg(arguments, "lead_id"))
	if leadID == "" {
		return nil, shared.Validation("lead_id is required")
	}
	values := crmMutationValues(arguments, []string{"title", "party_id", "party_name", "contact_id", "owner_user_id", "source_channel", "status", "rating", "estimated_value", "expected_close_date", "next_action_at", "notes"})
	item, err := s.crm.UpdateLead(leadID, actor.EffectiveUserID, values, positiveIntArg(arguments, "expected_version", 0))
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Updated CRM lead %s.", crmToolTextValue(item.Values["lead_number"]))}},
		"structuredContent": item,
	}, nil
}

func (s *Server) crmOpportunitySearch(actor ActorContext, arguments map[string]any) (map[string]any, error) {
	if s == nil || s.crm == nil {
		return nil, fmt.Errorf("crm is unavailable")
	}
	filters := map[string]string{}
	for _, key := range []string{"stage", "status", "party_id", "owner_user_id"} {
		if value := strings.TrimSpace(stringArg(arguments, key)); value != "" {
			filters[key] = value
		}
	}
	page := positiveIntArg(arguments, "page", 1)
	pageSize := positiveIntArg(arguments, "page_size", 20)
	items, total, err := s.crm.SearchOpportunities(filters, stringArg(arguments, "query"), page, pageSize)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Found %d CRM opportunities.", total)}},
		"structuredContent": map[string]any{"items": items, "total": total, "page": page, "page_size": pageSize},
	}, nil
}

func (s *Server) crmOpportunityGet(actor ActorContext, arguments map[string]any) (map[string]any, error) {
	if s == nil || s.crm == nil {
		return nil, fmt.Errorf("crm is unavailable")
	}
	opportunityID := strings.TrimSpace(stringArg(arguments, "opportunity_id"))
	if opportunityID == "" {
		return nil, shared.Validation("opportunity_id is required")
	}
	item, err := s.crm.GetOpportunity(opportunityID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded CRM opportunity %s.", opportunityID)}},
		"structuredContent": item,
	}, nil
}

func (s *Server) crmOpportunityCreate(actor ActorContext, arguments map[string]any) (map[string]any, error) {
	if s == nil || s.crm == nil {
		return nil, fmt.Errorf("crm is unavailable")
	}
	if !boolArg(arguments, "confirm_apply") {
		return nil, shared.Validation("confirm_apply must be true to create a crm opportunity")
	}
	values := crmMutationValues(arguments, []string{"title", "party_id", "party_name", "contact_id", "owner_user_id", "source_lead_id", "stage", "estimated_value", "expected_close_date", "next_action_at", "loss_reason", "notes"})
	item, err := s.crm.CreateOpportunity(actor.EffectiveUserID, values)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Created CRM opportunity %s.", crmToolTextValue(item.Values["opportunity_number"]))}},
		"structuredContent": item,
	}, nil
}

func (s *Server) crmOpportunityUpdate(actor ActorContext, arguments map[string]any) (map[string]any, error) {
	if s == nil || s.crm == nil {
		return nil, fmt.Errorf("crm is unavailable")
	}
	if !boolArg(arguments, "confirm_apply") {
		return nil, shared.Validation("confirm_apply must be true to update a crm opportunity")
	}
	opportunityID := strings.TrimSpace(stringArg(arguments, "opportunity_id"))
	if opportunityID == "" {
		return nil, shared.Validation("opportunity_id is required")
	}
	values := crmMutationValues(arguments, []string{"title", "party_id", "party_name", "contact_id", "owner_user_id", "source_lead_id", "stage", "estimated_value", "expected_close_date", "next_action_at", "loss_reason", "notes"})
	item, err := s.crm.UpdateOpportunity(opportunityID, actor.EffectiveUserID, values, positiveIntArg(arguments, "expected_version", 0))
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Updated CRM opportunity %s.", crmToolTextValue(item.Values["opportunity_number"]))}},
		"structuredContent": item,
	}, nil
}

func (s *Server) crmOpportunityPipelineSummary(actor ActorContext) (map[string]any, error) {
	if s == nil || s.crm == nil {
		return nil, fmt.Errorf("crm is unavailable")
	}
	payload, err := s.crm.OpportunityPipelineSummary(time.Now().UTC())
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("CRM pipeline loaded with %d open opportunities.", crmNestedInt(payload, "overview", "open_opportunities"))}},
		"structuredContent": payload,
	}, nil
}

func crmMutationValues(arguments map[string]any, keys []string) map[string]any {
	values := map[string]any{}
	for _, key := range keys {
		raw, ok := arguments[key]
		if !ok {
			continue
		}
		switch typed := raw.(type) {
		case string:
			if strings.TrimSpace(typed) != "" {
				values[key] = strings.TrimSpace(typed)
			}
		default:
			values[key] = typed
		}
	}
	return values
}

func crmNestedInt(payload map[string]any, outer, inner string) int {
	section, _ := payload[outer].(map[string]any)
	value, _ := section[inner]
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return 0
	}
}

func crmToolTextValue(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case fmt.Stringer:
		return strings.TrimSpace(typed.String())
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", value))
	}
}

func positiveIntArg(arguments map[string]any, key string, fallback int) int {
	raw, ok := arguments[key]
	if !ok {
		return fallback
	}
	switch typed := raw.(type) {
	case int:
		if typed > 0 {
			return typed
		}
	case int64:
		if typed > 0 {
			return int(typed)
		}
	case float64:
		if typed > 0 {
			return int(typed)
		}
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		if err == nil && parsed > 0 {
			return parsed
		}
	}
	return fallback
}
