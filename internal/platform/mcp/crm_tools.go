package mcp

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"orbyte/internal/platform/model"
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
	summary := fmt.Sprintf("CRM summary loaded with %d open tickets and %d overdue tickets.", crmNestedInt(payload, "overview", "open_tickets"), crmNestedInt(payload, "overview", "overdue_tickets"))
	if queueCode := crmNestedString(payload, "overview", "priority_queue_code"); strings.TrimSpace(queueCode) != "" {
		summary += fmt.Sprintf(" Priority queue: %s", queueCode)
		if count := crmNestedInt(payload, "overview", "priority_queue_open_tickets"); count > 0 {
			summary += fmt.Sprintf(" with %d open tickets", count)
		}
		summary += "."
	}
	if customerName := crmNestedString(payload, "overview", "priority_customer_name"); strings.TrimSpace(customerName) != "" {
		summary += fmt.Sprintf(" Priority customer: %s", customerName)
		if count := crmNestedInt(payload, "overview", "priority_customer_open_tickets"); count > 0 {
			summary += fmt.Sprintf(" with %d open tickets", count)
		}
		summary += "."
	}
	if overdueTitle := crmNestedString(payload, "overview", "overdue_ticket_title"); strings.TrimSpace(overdueTitle) != "" {
		summary += fmt.Sprintf(" Overdue example ticket: %s.", overdueTitle)
	}
	return map[string]any{
		"content": []ContentBlock{{
			Type: "text",
			Text: summary,
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
	summary := fmt.Sprintf("Found %d CRM tickets.", total)
	if top := crmTopRecordSummary(items, 3, func(record any) string {
		values := crmRecordValues(record)
		return fmt.Sprintf("%s [%s] (%s, %s, queue %s, id %s)", crmToolTextValue(values["title"]), crmToolTextValue(values["ticket_number"]), crmToolTextValue(values["party_name"]), crmToolTextValue(values["status"]), crmToolTextValue(values["queue_code"]), crmRecordID(record))
	}); top != "" {
		summary += " Top matches: " + top
	}
	return map[string]any{
		"content": []ContentBlock{{
			Type: "text",
			Text: summary,
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
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Added CRM ticket comment for %s.", ticketID)}},
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
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Assigned CRM ticket %s.", ticketID)}},
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
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Resolved CRM ticket %s.", ticketID)}},
		"structuredContent": item,
	}, nil
}

func (s *Server) crmCustomerSummary(actor ActorContext, arguments map[string]any) (map[string]any, error) {
	if s == nil || s.crm == nil {
		return nil, fmt.Errorf("crm is unavailable")
	}
	partyID := crmResolvePartyID(s, arguments)
	if partyID == "" {
		return nil, shared.Validation("party_id or query is required")
	}
	payload, err := s.crm.Customer360(partyID, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	party := payload["party"]
	query := strings.TrimSpace(stringArg(arguments, "query"))
	partyName := crmToolTextValue(crmRecordValues(party)["name"])
	if partyName == "" {
		partyName = query
	}
	ticketItems := crmRecordSlice(payload["tickets"])
	opportunityItems := crmRecordSlice(payload["opportunities"])
	if len(ticketItems) == 0 {
		ticketItems, _, _ = s.crm.SearchTickets(map[string]string{"party_id": partyID}, partyName, 1, 20)
	}
	if len(opportunityItems) == 0 {
		opportunityItems, _, _ = s.crm.SearchOpportunities(map[string]string{"party_id": partyID}, partyName, 1, 20)
	}
	summary := fmt.Sprintf("Loaded CRM customer 360 for %s.", partyName)
	if ticket := crmPreferredCurrentTicket(ticketItems, time.Now().UTC()); strings.TrimSpace(ticket.ID) != "" {
		summary += fmt.Sprintf(" Current service issue: %s (%s).", crmToolTextValue(ticket.Values["title"]), crmToolTextValue(ticket.Values["status"]))
	}
	if len(opportunityItems) > 0 {
		first := opportunityItems[0]
		summary += fmt.Sprintf(" Active opportunity: %s (%s, value %s).", crmToolTextValue(first.Values["title"]), crmToolTextValue(first.Values["stage"]), crmToolTextValue(first.Values["estimated_value"]))
	}
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: summary}},
		"structuredContent": payload,
	}, nil
}

func (s *Server) crmCustomerTimeline(actor ActorContext, arguments map[string]any) (map[string]any, error) {
	if s == nil || s.crm == nil {
		return nil, fmt.Errorf("crm is unavailable")
	}
	partyID := crmResolvePartyID(s, arguments)
	if partyID == "" {
		return nil, shared.Validation("party_id or query is required")
	}
	payload, err := s.crm.Customer360(partyID, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	party := payload["party"]
	summary := fmt.Sprintf("Loaded CRM customer timeline for %s.", crmToolTextValue(crmRecordValues(party)["name"]))
	return map[string]any{
		"content": []ContentBlock{{Type: "text", Text: summary}},
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
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded CRM customer health for %d at-risk customers.", crmNestedInt(payload, "overview", "customers_with_open_issues"))}},
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
	summary := fmt.Sprintf("Found %d CRM leads.", total)
	if top := crmTopRecordSummary(items, 3, func(record any) string {
		values := crmRecordValues(record)
		return fmt.Sprintf("%s (%s, rating %s, id %s)", crmToolTextValue(values["title"]), crmToolTextValue(values["status"]), crmToolTextValue(values["rating"]), crmRecordID(record))
	}); top != "" {
		summary += " Top matches: " + top
	}
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: summary}},
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
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded CRM lead %s.", leadID)}},
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
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Created CRM lead %s.", crmToolTextValue(item.Values["lead_number"]))}},
		"structuredContent": item,
	}, nil
}

func (s *Server) crmLeadFindOrCreateForProductInterest(actor ActorContext, arguments map[string]any) (map[string]any, error) {
	if s == nil || s.crm == nil {
		return nil, fmt.Errorf("crm is unavailable")
	}
	partyID := strings.TrimSpace(stringArg(arguments, "party_id"))
	partyName := strings.TrimSpace(stringArg(arguments, "party_name"))
	if partyID == "" && partyName != "" {
		partyID = s.crm.ResolveCustomerPartyID(partyName)
	}
	if partyID == "" {
		return nil, shared.Validation("party_id or party_name is required")
	}
	_, product, err := s.resolveCommercialItem(actor, map[string]any{
		"record_id":    strings.TrimSpace(stringArg(arguments, "product_record_id")),
		"product_code": strings.TrimSpace(stringArg(arguments, "product_code")),
		"sku":          strings.TrimSpace(stringArg(arguments, "sku")),
		"name":         strings.TrimSpace(stringArg(arguments, "product_name")),
	})
	if err != nil {
		return nil, err
	}
	lead, total, err := s.findReusableLeadForProductInterest(partyID, product)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(lead.ID) != "" {
		return map[string]any{
			"content": []ContentBlock{{
				Type: "text",
				Text: fmt.Sprintf("Found existing CRM lead %s for %s.", crmToolTextValue(lead.Values["lead_number"]), crmToolTextValue(product.Values["name"])),
			}},
			"structuredContent": map[string]any{
				"action":  "found_existing",
				"lead":    lead,
				"product": commercialItemDetailPayload(product),
				"total":   total,
			},
		}, nil
	}

	leadValues := map[string]any{
		"title":               firstNonEmpty(strings.TrimSpace(stringArg(arguments, "title")), "Interest in "+firstNonEmpty(stringValue(product.Values["name"]), product.ID)),
		"party_id":            partyID,
		"party_name":          partyName,
		"owner_user_id":       strings.TrimSpace(stringArg(arguments, "owner_user_id")),
		"source_channel":      strings.TrimSpace(stringArg(arguments, "source_channel")),
		"estimated_value":     strings.TrimSpace(stringArg(arguments, "estimated_value")),
		"expected_close_date": strings.TrimSpace(stringArg(arguments, "expected_close_date")),
		"next_action_at":      strings.TrimSpace(stringArg(arguments, "next_action_at")),
		"notes":               strings.TrimSpace(stringArg(arguments, "notes")),
		"product_record_id":   product.ID,
		"product_code":        firstNonEmpty(stringValue(product.Values["product_code"]), strings.TrimSpace(stringArg(arguments, "product_code"))),
		"product_name":        firstNonEmpty(stringValue(product.Values["name"]), strings.TrimSpace(stringArg(arguments, "product_name"))),
	}
	if !boolArg(arguments, "confirm_apply") {
		return map[string]any{
			"content": []ContentBlock{{
				Type: "text",
				Text: fmt.Sprintf("Prepared a CRM lead preview for %s about %s.", firstNonEmpty(partyName, partyID), crmToolTextValue(product.Values["name"])),
			}},
			"structuredContent": map[string]any{
				"action":  "would_create",
				"preview": leadValues,
				"product": commercialItemDetailPayload(product),
			},
		}, nil
	}
	item, err := s.crm.CreateLead(actor.EffectiveUserID, leadValues)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"content": []ContentBlock{{
			Type: "text",
			Text: fmt.Sprintf("Created CRM lead %s for %s.", crmToolTextValue(item.Values["lead_number"]), crmToolTextValue(product.Values["name"])),
		}},
		"structuredContent": map[string]any{
			"action":  "created",
			"lead":    item,
			"product": commercialItemDetailPayload(product),
		},
	}, nil
}

func (s *Server) findReusableLeadForProductInterest(partyID string, product model.Record) (model.Record, int, error) {
	page := 1
	total := 0
	for {
		leads, searchTotal, err := s.crm.SearchLeads(map[string]string{"party_id": partyID}, "", page, model.MaxPageSize)
		if err != nil {
			return model.Record{}, 0, err
		}
		if total == 0 {
			total = searchTotal
		}
		for _, lead := range leads {
			if !crmLeadReusableStatus(crmToolTextValue(lead.Values["status"])) {
				continue
			}
			if crmLeadMatchesProductInterest(lead, product) {
				return lead, total, nil
			}
		}
		if len(leads) == 0 || page*model.MaxPageSize >= searchTotal {
			return model.Record{}, total, nil
		}
		page++
	}
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
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Updated CRM lead %s.", crmToolTextValue(item.Values["lead_number"]))}},
		"structuredContent": item,
	}, nil
}

func crmLeadReusableStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case "new", "contacted", "qualified":
		return true
	default:
		return false
	}
}

func crmLeadMatchesProductInterest(lead model.Record, product model.Record) bool {
	leadProductRecordID := strings.TrimSpace(crmToolTextValue(lead.Values["product_record_id"]))
	productRecordID := strings.TrimSpace(product.ID)
	if leadProductRecordID != "" && productRecordID != "" && leadProductRecordID == productRecordID {
		return true
	}
	leadProductCode := normalizeCRMLeadProductMatchValue(lead.Values["product_code"])
	productCode := normalizeCRMLeadProductMatchValue(product.Values["product_code"])
	if leadProductCode != "" && productCode != "" && leadProductCode == productCode {
		return true
	}
	leadProductName := normalizeCRMLeadProductMatchValue(lead.Values["product_name"])
	productName := normalizeCRMLeadProductMatchValue(product.Values["name"])
	return leadProductName != "" && productName != "" && leadProductName == productName
}

func normalizeCRMLeadProductMatchValue(value any) string {
	return strings.ToLower(strings.TrimSpace(crmToolTextValue(value)))
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
	summary := fmt.Sprintf("Found %d CRM opportunities.", total)
	if top := crmTopRecordSummary(items, 5, func(record any) string {
		values := crmRecordValues(record)
		return fmt.Sprintf("%s (%s, %s, value %s, id %s)", crmToolTextValue(values["title"]), crmToolTextValue(values["party_name"]), crmToolTextValue(values["stage"]), crmToolTextValue(values["estimated_value"]), crmRecordID(record))
	}); top != "" {
		summary += " Top matches: " + top
	}
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: summary}},
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
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded CRM opportunity %s.", opportunityID)}},
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
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Created CRM opportunity %s.", crmToolTextValue(item.Values["opportunity_number"]))}},
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
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Updated CRM opportunity %s.", crmToolTextValue(item.Values["opportunity_number"]))}},
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
	priorityTitle := crmNestedString(payload, "overview", "priority_opportunity_title")
	priorityCustomer := crmNestedString(payload, "overview", "priority_customer_name")
	priorityValue := crmNestedInt(payload, "overview", "priority_pipeline_value")
	summary := fmt.Sprintf("CRM pipeline loaded with %d open opportunities.", crmNestedInt(payload, "overview", "open_opportunities"))
	if strings.TrimSpace(priorityTitle) != "" {
		summary += fmt.Sprintf(" Priority stale opportunity: %s", priorityTitle)
		if strings.TrimSpace(priorityCustomer) != "" {
			summary += fmt.Sprintf(" for %s", priorityCustomer)
		}
		if priorityValue > 0 {
			summary += fmt.Sprintf(" valued at %d", priorityValue)
		}
		summary += "."
	}
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: summary}},
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

func crmMapValue(value any, key ...string) map[string]any {
	mapped, _ := value.(map[string]any)
	if len(key) == 0 {
		return mapped
	}
	if mapped == nil {
		return nil
	}
	nested, _ := mapped[key[0]].(map[string]any)
	return nested
}

func crmRecordValues(value any) map[string]any {
	switch typed := value.(type) {
	case model.Record:
		return typed.Values
	case *model.Record:
		if typed == nil {
			return nil
		}
		return typed.Values
	case map[string]any:
		if values, ok := typed["values"].(map[string]any); ok {
			return values
		}
		return typed
	default:
		return nil
	}
}

func crmRecordSlice(value any) []model.Record {
	switch typed := value.(type) {
	case []model.Record:
		return append([]model.Record(nil), typed...)
	case []any:
		records := make([]model.Record, 0, len(typed))
		for _, item := range typed {
			switch record := item.(type) {
			case model.Record:
				records = append(records, record)
			case *model.Record:
				if record != nil {
					records = append(records, *record)
				}
			}
		}
		return records
	default:
		return nil
	}
}

func crmRecordID(value any) string {
	switch typed := value.(type) {
	case model.Record:
		return typed.ID
	case *model.Record:
		if typed == nil {
			return ""
		}
		return typed.ID
	case map[string]any:
		return crmToolTextValue(typed["id"])
	default:
		return ""
	}
}

func crmResolvePartyID(s *Server, arguments map[string]any) string {
	partyID := strings.TrimSpace(stringArg(arguments, "party_id"))
	if partyID != "" {
		return partyID
	}
	query := strings.TrimSpace(stringArg(arguments, "query"))
	if query == "" || s == nil || s.crm == nil {
		return ""
	}
	return s.crm.ResolveCustomerPartyID(query)
}

func crmTopRecordSummary(items any, limit int, builder func(any) string) string {
	parts := make([]string, 0, limit)
	for _, item := range crmTopRecordItems(items) {
		if len(parts) >= limit {
			break
		}
		part := strings.TrimSpace(builder(item))
		if part == "" {
			continue
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, "; ")
}

func crmTopRecordItems(items any) []any {
	switch typed := items.(type) {
	case []any:
		return typed
	case []model.Record:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, item)
		}
		return out
	case []*model.Record:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			if item != nil {
				out = append(out, item)
			}
		}
		return out
	default:
		return nil
	}
}

func crmPreferredCurrentTicket(items []model.Record, now time.Time) model.Record {
	filtered := make([]model.Record, 0, len(items))
	for _, item := range items {
		status := strings.TrimSpace(crmToolTextValue(item.Values["status"]))
		if status == "open" || status == "new" || status == "in_progress" {
			filtered = append(filtered, item)
		}
	}
	if len(filtered) == 0 {
		return model.Record{}
	}
	best := filtered[0]
	for _, item := range filtered[1:] {
		bestDue := parseRFC3339(best.Values["due_at"])
		itemDue := parseRFC3339(item.Values["due_at"])
		bestFuture := !bestDue.IsZero() && bestDue.After(now)
		itemFuture := !itemDue.IsZero() && itemDue.After(now)
		if itemFuture != bestFuture {
			if itemFuture {
				best = item
			}
			continue
		}
		bestPriority := crmPriorityRank(strings.TrimSpace(crmToolTextValue(best.Values["priority"])))
		itemPriority := crmPriorityRank(strings.TrimSpace(crmToolTextValue(item.Values["priority"])))
		if itemPriority != bestPriority {
			if itemPriority > bestPriority {
				best = item
			}
			continue
		}
		if item.UpdatedAt.After(best.UpdatedAt) {
			best = item
		}
	}
	return best
}

func crmPriorityRank(priority string) int {
	switch strings.ToLower(strings.TrimSpace(priority)) {
	case "urgent":
		return 4
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

func parseRFC3339(value any) time.Time {
	text := strings.TrimSpace(crmToolTextValue(value))
	if text == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339, text)
	if err != nil {
		return time.Time{}
	}
	return parsed
}

func crmNestedString(payload map[string]any, outer, inner string) string {
	section, _ := payload[outer].(map[string]any)
	return crmToolTextValue(section[inner])
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
