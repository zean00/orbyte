package application

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"orbyte/internal/platform/model"
	"orbyte/internal/platform/shared"
)

type CRMCoreService struct {
	models *model.Service
}

func NewCRMCoreService(models *model.Service) *CRMCoreService {
	return &CRMCoreService{models: models}
}

func (s *CRMCoreService) SearchTickets(filters map[string]string, query string, page, pageSize int) ([]model.Record, int, error) {
	items, err := s.listRecords("crm_ticket", filters)
	if err != nil {
		return nil, 0, err
	}
	needle := strings.ToLower(strings.TrimSpace(query))
	filtered := make([]model.Record, 0, len(items))
	for _, item := range items {
		if needle != "" && !recordMatchesNeedle(item, needle, "ticket_number", "title", "description", "party_name", "queue_code", "status", "priority", "issue_category") {
			continue
		}
		filtered = append(filtered, item)
	}
	sort.Slice(filtered, func(i, j int) bool {
		leftPriority := crmPriorityRank(crmTextValue(filtered[i].Values["priority"]))
		rightPriority := crmPriorityRank(crmTextValue(filtered[j].Values["priority"]))
		if leftPriority != rightPriority {
			return leftPriority > rightPriority
		}
		return filtered[i].UpdatedAt.After(filtered[j].UpdatedAt)
	})
	return paginateCRMRecords(filtered, page, pageSize)
}

func (s *CRMCoreService) SearchLeads(filters map[string]string, query string, page, pageSize int) ([]model.Record, int, error) {
	items, err := s.listRecords("crm_lead", filters)
	if err != nil {
		return nil, 0, err
	}
	needle := strings.ToLower(strings.TrimSpace(query))
	filtered := make([]model.Record, 0, len(items))
	for _, item := range items {
		if needle != "" && !recordMatchesNeedle(item, needle, "lead_number", "title", "party_name", "product_name", "product_code", "status", "rating", "owner_user_id", "source_channel") {
			continue
		}
		filtered = append(filtered, item)
	}
	sort.Slice(filtered, func(i, j int) bool {
		left := crmLeadRank(crmTextValue(filtered[i].Values["status"]))
		right := crmLeadRank(crmTextValue(filtered[j].Values["status"]))
		if left != right {
			return left > right
		}
		return filtered[i].UpdatedAt.After(filtered[j].UpdatedAt)
	})
	return paginateCRMRecords(filtered, page, pageSize)
}

func (s *CRMCoreService) SearchOpportunities(filters map[string]string, query string, page, pageSize int) ([]model.Record, int, error) {
	items, err := s.listRecords("crm_opportunity", filters)
	if err != nil {
		return nil, 0, err
	}
	needle := strings.ToLower(strings.TrimSpace(query))
	filtered := make([]model.Record, 0, len(items))
	for _, item := range items {
		if needle != "" && !recordMatchesNeedle(item, needle, "opportunity_number", "title", "party_name", "stage", "status", "owner_user_id") {
			continue
		}
		filtered = append(filtered, item)
	}
	sort.Slice(filtered, func(i, j int) bool {
		left := crmOpportunityStageRank(crmTextValue(filtered[i].Values["stage"]))
		right := crmOpportunityStageRank(crmTextValue(filtered[j].Values["stage"]))
		if left != right {
			return left > right
		}
		return filtered[i].UpdatedAt.After(filtered[j].UpdatedAt)
	})
	return paginateCRMRecords(filtered, page, pageSize)
}

func (s *CRMCoreService) GetTicket(ticketID string) (model.Record, error) {
	return s.getRecord("crm_ticket", ticketID)
}

func (s *CRMCoreService) GetLead(leadID string) (model.Record, error) {
	return s.getRecord("crm_lead", leadID)
}

func (s *CRMCoreService) GetOpportunity(opportunityID string) (model.Record, error) {
	return s.getRecord("crm_opportunity", opportunityID)
}

func (s *CRMCoreService) CreateTicket(actorID string, values map[string]any) (model.Record, error) {
	if err := s.ensureConfigured(); err != nil {
		return model.Record{}, err
	}
	next := cloneAnyMap(values)
	now := time.Now().UTC()
	if strings.TrimSpace(crmTextValue(next["ticket_number"])) == "" {
		next["ticket_number"] = GenerateCRMTicketNumber(now)
	}
	if strings.TrimSpace(crmTextValue(next["status"])) == "" {
		next["status"] = "new"
	}
	if strings.TrimSpace(crmTextValue(next["opened_at"])) == "" {
		next["opened_at"] = now.Format(time.RFC3339)
	}
	if strings.TrimSpace(crmTextValue(next["party_name"])) == "" {
		next["party_name"] = s.resolvePartyName(next["party_id"])
	}
	s.applyAssignmentDefaults(next)
	s.applySLADefaults(next)
	record, err := s.models.Create("crm_ticket", actorID, next)
	if err != nil {
		return model.Record{}, err
	}
	_ = s.createTicketActivityRecord(actorID, map[string]any{
		"ticket_id":         record.ID,
		"ticket_number":     crmTextValue(record.Values["ticket_number"]),
		"activity_type":     "created",
		"actor_user_id":     actorID,
		"assignee_user_id":  crmTextValue(record.Values["assignee_user_id"]),
		"queue_code":        crmTextValue(record.Values["queue_code"]),
		"to_status":         crmTextValue(record.Values["status"]),
		"occurred_at":       now.Format(time.RFC3339),
		"note":              crmTextValue(record.Values["title"]),
		"party_id":          crmTextValue(record.Values["party_id"]),
		"party_name":        crmTextValue(record.Values["party_name"]),
		"severity":          crmTextValue(record.Values["severity"]),
		"priority":          crmTextValue(record.Values["priority"]),
		"sla_breach_risk":   s.ticketRiskLevel(record.Values, now),
		"source_channel":    crmTextValue(record.Values["source_channel"]),
		"issue_category":    crmTextValue(record.Values["issue_category"]),
		"ticket_status_key": crmTextValue(record.Values["status"]),
	})
	_, _ = s.createActivityRecord(actorID, map[string]any{
		"activity_type": "ticket_created",
		"subject":       "Ticket " + crmTextValue(record.Values["ticket_number"]),
		"related_kind":  "ticket",
		"related_id":    record.ID,
		"party_id":      crmTextValue(record.Values["party_id"]),
		"owner_user_id": crmFirstNonEmpty(crmTextValue(record.Values["assignee_user_id"]), actorID),
		"status":        "completed",
		"completed_at":  now.Format(time.RFC3339),
		"note":          crmTextValue(record.Values["title"]),
	})
	return record, nil
}

func (s *CRMCoreService) UpdateTicket(ticketID, actorID string, values map[string]any, expectedVersion int) (model.Record, error) {
	if err := s.ensureConfigured(); err != nil {
		return model.Record{}, err
	}
	current, err := s.models.Get("crm_ticket", strings.TrimSpace(ticketID))
	if err != nil {
		return model.Record{}, err
	}
	next := cloneAnyMap(current.Values)
	for key, value := range values {
		next[key] = value
	}
	if strings.TrimSpace(crmTextValue(next["party_name"])) == "" {
		next["party_name"] = s.resolvePartyName(next["party_id"])
	}
	s.applyAssignmentDefaults(next)
	s.applySLADefaults(next)
	status := strings.TrimSpace(crmTextValue(next["status"]))
	if (status == "resolved" || status == "closed") && strings.TrimSpace(crmTextValue(next["resolved_at"])) == "" {
		next["resolved_at"] = time.Now().UTC().Format(time.RFC3339)
	}
	if expectedVersion <= 0 {
		expectedVersion = current.Version
	}
	updated, err := s.models.Update("crm_ticket", current.ID, actorID, next, expectedVersion)
	if err != nil {
		return model.Record{}, err
	}
	changedStatus := crmTextValue(current.Values["status"]) != crmTextValue(updated.Values["status"])
	changedAssignee := crmTextValue(current.Values["assignee_user_id"]) != crmTextValue(updated.Values["assignee_user_id"])
	if changedStatus || changedAssignee {
		activityType := "updated"
		if changedStatus {
			activityType = "status_changed"
		}
		if changedAssignee && !changedStatus {
			activityType = "assigned"
		}
		_ = s.createTicketActivityRecord(actorID, map[string]any{
			"ticket_id":         updated.ID,
			"ticket_number":     crmTextValue(updated.Values["ticket_number"]),
			"activity_type":     activityType,
			"actor_user_id":     actorID,
			"assignee_user_id":  crmTextValue(updated.Values["assignee_user_id"]),
			"queue_code":        crmTextValue(updated.Values["queue_code"]),
			"from_status":       crmTextValue(current.Values["status"]),
			"to_status":         crmTextValue(updated.Values["status"]),
			"occurred_at":       time.Now().UTC().Format(time.RFC3339),
			"note":              crmFirstNonEmpty(crmTextValue(values["resolution_notes"]), crmTextValue(values["description"])),
			"party_id":          crmTextValue(updated.Values["party_id"]),
			"party_name":        crmTextValue(updated.Values["party_name"]),
			"severity":          crmTextValue(updated.Values["severity"]),
			"priority":          crmTextValue(updated.Values["priority"]),
			"sla_breach_risk":   s.ticketRiskLevel(updated.Values, time.Now().UTC()),
			"source_channel":    crmTextValue(updated.Values["source_channel"]),
			"issue_category":    crmTextValue(updated.Values["issue_category"]),
			"ticket_status_key": crmTextValue(updated.Values["status"]),
		})
	}
	return updated, nil
}

func (s *CRMCoreService) AddTicketComment(ticketID, actorID, body, commentType string) (model.Record, error) {
	if err := s.ensureConfigured(); err != nil {
		return model.Record{}, err
	}
	ticket, err := s.GetTicket(ticketID)
	if err != nil {
		return model.Record{}, err
	}
	now := time.Now().UTC()
	record, err := s.models.Create("crm_ticket_comment", actorID, map[string]any{
		"ticket_id":      ticket.ID,
		"ticket_number":  crmTextValue(ticket.Values["ticket_number"]),
		"comment_type":   crmFirstNonEmpty(commentType, "internal_note"),
		"body":           strings.TrimSpace(body),
		"author_user_id": actorID,
		"created_at":     now.Format(time.RFC3339),
		"party_id":       crmTextValue(ticket.Values["party_id"]),
		"party_name":     crmTextValue(ticket.Values["party_name"]),
	})
	if err != nil {
		return model.Record{}, err
	}
	_ = s.createTicketActivityRecord(actorID, map[string]any{
		"ticket_id":         ticket.ID,
		"ticket_number":     crmTextValue(ticket.Values["ticket_number"]),
		"activity_type":     "commented",
		"actor_user_id":     actorID,
		"assignee_user_id":  crmTextValue(ticket.Values["assignee_user_id"]),
		"queue_code":        crmTextValue(ticket.Values["queue_code"]),
		"to_status":         crmTextValue(ticket.Values["status"]),
		"occurred_at":       now.Format(time.RFC3339),
		"note":              strings.TrimSpace(body),
		"party_id":          crmTextValue(ticket.Values["party_id"]),
		"party_name":        crmTextValue(ticket.Values["party_name"]),
		"severity":          crmTextValue(ticket.Values["severity"]),
		"priority":          crmTextValue(ticket.Values["priority"]),
		"sla_breach_risk":   s.ticketRiskLevel(ticket.Values, now),
		"source_channel":    crmTextValue(ticket.Values["source_channel"]),
		"issue_category":    crmTextValue(ticket.Values["issue_category"]),
		"ticket_status_key": crmTextValue(ticket.Values["status"]),
	})
	return record, nil
}

func (s *CRMCoreService) AssignTicket(ticketID, actorID, assigneeUserID, note string, expectedVersion int) (model.Record, error) {
	values := map[string]any{"assignee_user_id": assigneeUserID}
	if strings.TrimSpace(assigneeUserID) != "" {
		values["status"] = "open"
	}
	updated, err := s.UpdateTicket(ticketID, actorID, values, expectedVersion)
	if err != nil {
		return model.Record{}, err
	}
	trimmedNote := strings.TrimSpace(note)
	if trimmedNote != "" {
		_ = s.createTicketActivityRecord(actorID, map[string]any{
			"ticket_id":         updated.ID,
			"ticket_number":     crmTextValue(updated.Values["ticket_number"]),
			"activity_type":     "assignment_note",
			"actor_user_id":     actorID,
			"assignee_user_id":  crmTextValue(updated.Values["assignee_user_id"]),
			"queue_code":        crmTextValue(updated.Values["queue_code"]),
			"to_status":         crmTextValue(updated.Values["status"]),
			"occurred_at":       time.Now().UTC().Format(time.RFC3339),
			"note":              trimmedNote,
			"party_id":          crmTextValue(updated.Values["party_id"]),
			"party_name":        crmTextValue(updated.Values["party_name"]),
			"severity":          crmTextValue(updated.Values["severity"]),
			"priority":          crmTextValue(updated.Values["priority"]),
			"sla_breach_risk":   s.ticketRiskLevel(updated.Values, time.Now().UTC()),
			"source_channel":    crmTextValue(updated.Values["source_channel"]),
			"issue_category":    crmTextValue(updated.Values["issue_category"]),
			"ticket_status_key": crmTextValue(updated.Values["status"]),
		})
	}
	return updated, nil
}

func (s *CRMCoreService) ResolveTicket(ticketID, actorID, resolutionNotes string, close bool, expectedVersion int) (model.Record, error) {
	status := "resolved"
	if close {
		status = "closed"
	}
	return s.UpdateTicket(ticketID, actorID, map[string]any{
		"status":           status,
		"resolution_notes": resolutionNotes,
		"resolved_at":      time.Now().UTC().Format(time.RFC3339),
	}, expectedVersion)
}

func (s *CRMCoreService) TicketTimeline(ticketID string) ([]map[string]any, error) {
	if err := s.ensureConfigured(); err != nil {
		return nil, err
	}
	items := make([]map[string]any, 0)
	activities, err := s.listRecords("crm_ticket_activity", map[string]string{"ticket_id": strings.TrimSpace(ticketID)})
	if err == nil {
		for _, item := range activities {
			items = append(items, map[string]any{
				"kind":       "activity",
				"id":         item.ID,
				"created_at": crmFirstNonEmpty(crmTextValue(item.Values["occurred_at"]), item.CreatedAt.UTC().Format(time.RFC3339)),
				"payload":    item,
			})
		}
	}
	comments, err := s.listRecords("crm_ticket_comment", map[string]string{"ticket_id": strings.TrimSpace(ticketID)})
	if err == nil {
		for _, item := range comments {
			items = append(items, map[string]any{
				"kind":       "comment",
				"id":         item.ID,
				"created_at": crmFirstNonEmpty(crmTextValue(item.Values["created_at"]), item.CreatedAt.UTC().Format(time.RFC3339)),
				"payload":    item,
			})
		}
	}
	sort.Slice(items, func(i, j int) bool {
		return parseRFC3339(items[i]["created_at"]).Before(parseRFC3339(items[j]["created_at"]))
	})
	return items, nil
}

func (s *CRMCoreService) CreateLead(actorID string, values map[string]any) (model.Record, error) {
	if err := s.ensureConfigured(); err != nil {
		return model.Record{}, err
	}
	next := cloneAnyMap(values)
	now := time.Now().UTC()
	if strings.TrimSpace(crmTextValue(next["lead_number"])) == "" {
		next["lead_number"] = GenerateCRMLeadNumber(now)
	}
	if strings.TrimSpace(crmTextValue(next["status"])) == "" {
		next["status"] = "new"
	}
	if strings.TrimSpace(crmTextValue(next["party_name"])) == "" {
		next["party_name"] = s.resolvePartyName(next["party_id"])
	}
	record, err := s.models.Create("crm_lead", actorID, next)
	if err != nil {
		return model.Record{}, err
	}
	_, _ = s.createActivityRecord(actorID, map[string]any{
		"activity_type": "lead_created",
		"subject":       "Lead " + crmTextValue(record.Values["lead_number"]),
		"related_kind":  "lead",
		"related_id":    record.ID,
		"party_id":      crmTextValue(record.Values["party_id"]),
		"owner_user_id": crmFirstNonEmpty(crmTextValue(record.Values["owner_user_id"]), actorID),
		"status":        "completed",
		"completed_at":  now.Format(time.RFC3339),
		"note":          crmTextValue(record.Values["title"]),
	})
	return record, nil
}

func (s *CRMCoreService) UpdateLead(leadID, actorID string, values map[string]any, expectedVersion int) (model.Record, error) {
	current, err := s.getRecord("crm_lead", leadID)
	if err != nil {
		return model.Record{}, err
	}
	next := cloneAnyMap(current.Values)
	for key, value := range values {
		next[key] = value
	}
	if strings.TrimSpace(crmTextValue(next["party_name"])) == "" {
		next["party_name"] = s.resolvePartyName(next["party_id"])
	}
	if expectedVersion <= 0 {
		expectedVersion = current.Version
	}
	return s.models.Update("crm_lead", current.ID, actorID, next, expectedVersion)
}

func (s *CRMCoreService) CreateOpportunity(actorID string, values map[string]any) (model.Record, error) {
	if err := s.ensureConfigured(); err != nil {
		return model.Record{}, err
	}
	next := cloneAnyMap(values)
	now := time.Now().UTC()
	if strings.TrimSpace(crmTextValue(next["opportunity_number"])) == "" {
		next["opportunity_number"] = GenerateCRMOpportunityNumber(now)
	}
	if strings.TrimSpace(crmTextValue(next["stage"])) == "" {
		next["stage"] = "new"
	}
	next["status"] = crmOpportunityStatusForStage(crmTextValue(next["stage"]))
	if strings.TrimSpace(crmTextValue(next["party_name"])) == "" {
		next["party_name"] = s.resolvePartyName(next["party_id"])
	}
	record, err := s.models.Create("crm_opportunity", actorID, next)
	if err != nil {
		return model.Record{}, err
	}
	_, _ = s.createActivityRecord(actorID, map[string]any{
		"activity_type": "opportunity_created",
		"subject":       "Opportunity " + crmTextValue(record.Values["opportunity_number"]),
		"related_kind":  "opportunity",
		"related_id":    record.ID,
		"party_id":      crmTextValue(record.Values["party_id"]),
		"owner_user_id": crmFirstNonEmpty(crmTextValue(record.Values["owner_user_id"]), actorID),
		"status":        "completed",
		"completed_at":  now.Format(time.RFC3339),
		"note":          crmTextValue(record.Values["title"]),
	})
	return record, nil
}

func (s *CRMCoreService) UpdateOpportunity(opportunityID, actorID string, values map[string]any, expectedVersion int) (model.Record, error) {
	current, err := s.getRecord("crm_opportunity", opportunityID)
	if err != nil {
		return model.Record{}, err
	}
	next := cloneAnyMap(current.Values)
	for key, value := range values {
		next[key] = value
	}
	if strings.TrimSpace(crmTextValue(next["party_name"])) == "" {
		next["party_name"] = s.resolvePartyName(next["party_id"])
	}
	stage := crmTextValue(next["stage"])
	if strings.TrimSpace(stage) != "" {
		next["status"] = crmOpportunityStatusForStage(stage)
	}
	if expectedVersion <= 0 {
		expectedVersion = current.Version
	}
	updated, err := s.models.Update("crm_opportunity", current.ID, actorID, next, expectedVersion)
	if err != nil {
		return model.Record{}, err
	}
	if crmTextValue(current.Values["stage"]) != crmTextValue(updated.Values["stage"]) {
		_, _ = s.createActivityRecord(actorID, map[string]any{
			"activity_type": "opportunity_stage_changed",
			"subject":       "Opportunity " + crmTextValue(updated.Values["opportunity_number"]),
			"related_kind":  "opportunity",
			"related_id":    updated.ID,
			"party_id":      crmTextValue(updated.Values["party_id"]),
			"owner_user_id": crmFirstNonEmpty(crmTextValue(updated.Values["owner_user_id"]), actorID),
			"status":        "completed",
			"completed_at":  time.Now().UTC().Format(time.RFC3339),
			"note":          crmTextValue(updated.Values["stage"]),
		})
	}
	return updated, nil
}

func (s *CRMCoreService) CreateActivity(actorID string, values map[string]any) (model.Record, error) {
	return s.createActivityRecord(actorID, values)
}

func (s *CRMCoreService) Customer360(partyID string, now time.Time) (map[string]any, error) {
	if err := s.ensureConfigured(); err != nil {
		return nil, err
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	party, err := s.models.Get("party", strings.TrimSpace(partyID))
	if err != nil {
		return nil, err
	}
	profiles, _ := s.listRecords("customer_profile", map[string]string{"party_id": party.ID})
	contacts, _ := s.listRecords("party_contact", map[string]string{"party_id": party.ID})
	tickets, _ := s.listRecords("crm_ticket", map[string]string{"party_id": party.ID})
	opps, _ := s.listRecords("crm_opportunity", map[string]string{"party_id": party.ID})
	activities, _ := s.listRecords("crm_activity", map[string]string{"party_id": party.ID})
	partyName := crmTextValue(party.Values["name"])
	if len(tickets) == 0 && strings.TrimSpace(partyName) != "" {
		allTickets, _ := s.listRecords("crm_ticket", nil)
		for _, item := range allTickets {
			if strings.EqualFold(strings.TrimSpace(crmTextValue(item.Values["party_name"])), strings.TrimSpace(partyName)) {
				tickets = append(tickets, item)
			}
		}
	}
	if len(opps) == 0 && strings.TrimSpace(partyName) != "" {
		allOpps, _ := s.listRecords("crm_opportunity", nil)
		for _, item := range allOpps {
			if strings.EqualFold(strings.TrimSpace(crmTextValue(item.Values["party_name"])), strings.TrimSpace(partyName)) {
				opps = append(opps, item)
			}
		}
	}
	if len(activities) == 0 && strings.TrimSpace(partyName) != "" {
		allActivities, _ := s.listRecords("crm_activity", nil)
		for _, item := range allActivities {
			if strings.EqualFold(strings.TrimSpace(crmTextValue(item.Values["party_name"])), strings.TrimSpace(partyName)) {
				activities = append(activities, item)
			}
		}
	}
	sort.Slice(tickets, func(i, j int) bool { return tickets[i].UpdatedAt.After(tickets[j].UpdatedAt) })
	sort.Slice(opps, func(i, j int) bool { return opps[i].UpdatedAt.After(opps[j].UpdatedAt) })
	sort.Slice(activities, func(i, j int) bool { return activities[i].UpdatedAt.After(activities[j].UpdatedAt) })
	openTickets := 0
	overdueTickets := 0
	openOpportunityValue := 0.0
	for _, item := range tickets {
		if crmTicketIsOpen(crmTextValue(item.Values["status"])) {
			openTickets++
			if dueAt := parseRFC3339(item.Values["due_at"]); !dueAt.IsZero() && dueAt.Before(now) {
				overdueTickets++
			}
		}
	}
	for _, item := range opps {
		if crmOpportunityIsOpen(crmTextValue(item.Values["stage"])) {
			openOpportunityValue += crmFloatValue(item.Values["estimated_value"])
		}
	}
	return map[string]any{
		"generated_at":     now.Format(time.RFC3339),
		"party":            party,
		"customer_profile": firstRecordOrNil(profiles),
		"contacts":         contacts,
		"tickets":          tickets,
		"opportunities":    opps,
		"activities":       activities,
		"overview": map[string]any{
			"open_tickets":           openTickets,
			"overdue_tickets":        overdueTickets,
			"open_opportunity_value": roundCRMFloat(openOpportunityValue),
			"member_tier":            crmTextValue(firstRecordValue(profiles, "member_tier")),
			"customer_segment":       crmTextValue(firstRecordValue(profiles, "customer_segment")),
		},
	}, nil
}

func (s *CRMCoreService) CustomerHealthPayload(now time.Time) (map[string]any, error) {
	if err := s.ensureConfigured(); err != nil {
		return nil, err
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	tickets, _ := s.listRecords("crm_ticket", nil)
	opps, _ := s.listRecords("crm_opportunity", nil)
	parties, _ := s.listRecords("party", nil)
	customerProfiles, _ := s.listRecords("customer_profile", nil)
	byParty := map[string]map[string]any{}
	for _, item := range parties {
		byParty[item.ID] = map[string]any{
			"party_id":                item.ID,
			"party_name":              crmTextValue(item.Values["name"]),
			"customer_segment":        "",
			"member_tier":             "",
			"open_tickets":            0,
			"overdue_tickets":         0,
			"open_opportunities":      0,
			"open_opportunity_value":  0.0,
			"health_score":            100,
			"recent_issue_categories": []string{},
		}
	}
	for _, item := range customerProfiles {
		partyID := crmTextValue(item.Values["party_id"])
		row := ensureCRMPartySummary(byParty, partyID, s.resolvePartyName(partyID))
		row["customer_segment"] = crmTextValue(item.Values["customer_segment"])
		row["member_tier"] = crmTextValue(item.Values["member_tier"])
	}
	for _, item := range tickets {
		partyID := crmTextValue(item.Values["party_id"])
		row := ensureCRMPartySummary(byParty, partyID, crmTextValue(item.Values["party_name"]))
		if crmTicketIsOpen(crmTextValue(item.Values["status"])) {
			row["open_tickets"] = crmIntValue(row["open_tickets"]) + 1
			row["health_score"] = crmIntValue(row["health_score"]) - 8
			if dueAt := parseRFC3339(item.Values["due_at"]); !dueAt.IsZero() && dueAt.Before(now) {
				row["overdue_tickets"] = crmIntValue(row["overdue_tickets"]) + 1
				row["health_score"] = crmIntValue(row["health_score"]) - 12
			}
		}
		category := strings.TrimSpace(crmTextValue(item.Values["issue_category"]))
		if category != "" {
			row["recent_issue_categories"] = appendStringIfMissing(row["recent_issue_categories"], category)
		}
	}
	for _, item := range opps {
		if !crmOpportunityIsOpen(crmTextValue(item.Values["stage"])) {
			continue
		}
		partyID := crmTextValue(item.Values["party_id"])
		row := ensureCRMPartySummary(byParty, partyID, crmTextValue(item.Values["party_name"]))
		row["open_opportunities"] = crmIntValue(row["open_opportunities"]) + 1
		row["open_opportunity_value"] = roundCRMFloat(crmFloatValue(row["open_opportunity_value"]) + crmFloatValue(item.Values["estimated_value"]))
	}
	items := make([]map[string]any, 0, len(byParty))
	for _, row := range byParty {
		if crmIntValue(row["open_tickets"]) == 0 && crmIntValue(row["open_opportunities"]) == 0 {
			continue
		}
		score := crmIntValue(row["health_score"])
		if score < 0 {
			score = 0
		}
		row["health_score"] = score
		items = append(items, row)
	}
	sort.Slice(items, func(i, j int) bool {
		left := crmIntValue(items[i]["overdue_tickets"])*10 + crmIntValue(items[i]["open_tickets"])
		right := crmIntValue(items[j]["overdue_tickets"])*10 + crmIntValue(items[j]["open_tickets"])
		if left != right {
			return left > right
		}
		return crmTextValue(items[i]["party_name"]) < crmTextValue(items[j]["party_name"])
	})
	return map[string]any{
		"generated_at": now.Format(time.RFC3339),
		"items":        items,
		"overview": map[string]any{
			"customers_with_open_issues": len(items),
			"total_open_tickets":         sumCRMIntField(items, "open_tickets"),
			"total_overdue_tickets":      sumCRMIntField(items, "overdue_tickets"),
		},
		"charts": map[string]any{
			"at_risk_customers": items,
		},
	}, nil
}

func (s *CRMCoreService) ServiceSummaryPayload(now time.Time) (map[string]any, error) {
	if err := s.ensureConfigured(); err != nil {
		return nil, err
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	tickets, err := s.listRecords("crm_ticket", nil)
	if err != nil {
		return nil, err
	}
	overview := map[string]any{
		"open_tickets":                   0,
		"overdue_tickets":                0,
		"unassigned_tickets":             0,
		"resolved_today":                 0,
		"first_response_breaches":        0,
		"priority_queue_code":            "",
		"priority_queue_open_tickets":    0,
		"priority_customer_name":         "",
		"priority_customer_open_tickets": 0,
		"overdue_ticket_title":           "",
	}
	queueCounts := map[string]int{}
	customerCounts := map[string]int{}
	priorityCounts := map[string]int{}
	channelCounts := map[string]int{}
	statusCounts := map[string]int{}
	trendBuckets := make(map[string]map[string]int)
	for index := 6; index >= 0; index-- {
		day := now.AddDate(0, 0, -index).Format("2006-01-02")
		trendBuckets[day] = map[string]int{"created": 0, "resolved": 0}
	}
	totalResponseHours := 0.0
	responseSamples := 0
	totalResolutionHours := 0.0
	resolutionSamples := 0
	for _, item := range tickets {
		status := strings.TrimSpace(crmTextValue(item.Values["status"]))
		statusCounts[status]++
		queueCode := crmFirstNonEmpty(strings.TrimSpace(crmTextValue(item.Values["queue_code"])), "Unassigned Queue")
		channel := crmFirstNonEmpty(strings.TrimSpace(crmTextValue(item.Values["source_channel"])), "unknown")
		priority := crmFirstNonEmpty(strings.TrimSpace(crmTextValue(item.Values["priority"])), "medium")
		priorityCounts[priority]++
		queueCounts[queueCode]++
		channelCounts[channel]++
		if crmTicketIsOpen(status) {
			overview["open_tickets"] = crmIntValue(overview["open_tickets"]) + 1
			customerName := crmFirstNonEmpty(strings.TrimSpace(crmTextValue(item.Values["party_name"])), "Unknown Customer")
			customerCounts[customerName]++
			if strings.TrimSpace(crmTextValue(item.Values["assignee_user_id"])) == "" {
				overview["unassigned_tickets"] = crmIntValue(overview["unassigned_tickets"]) + 1
			}
			if dueAt := parseRFC3339(item.Values["due_at"]); !dueAt.IsZero() && dueAt.Before(now) {
				overview["overdue_tickets"] = crmIntValue(overview["overdue_tickets"]) + 1
				if strings.TrimSpace(crmTextValue(overview["overdue_ticket_title"])) == "" || dueAt.Before(parseRFC3339(overview["overdue_ticket_due_at"])) {
					overview["overdue_ticket_title"] = crmTextValue(item.Values["title"])
					overview["overdue_ticket_due_at"] = dueAt.Format(time.RFC3339)
				}
			}
			if firstResponseDueAt := parseRFC3339(item.Values["first_response_due_at"]); !firstResponseDueAt.IsZero() && firstResponseDueAt.Before(now) && strings.TrimSpace(crmTextValue(item.Values["first_response_at"])) == "" {
				overview["first_response_breaches"] = crmIntValue(overview["first_response_breaches"]) + 1
			}
		}
		openedAt := item.CreatedAt
		if !parseRFC3339(item.Values["opened_at"]).IsZero() {
			openedAt = parseRFC3339(item.Values["opened_at"])
		}
		createdDay := openedAt.UTC().Format("2006-01-02")
		if bucket, ok := trendBuckets[createdDay]; ok {
			bucket["created"]++
		}
		if firstResponse := parseRFC3339(item.Values["first_response_at"]); !firstResponse.IsZero() && !openedAt.IsZero() {
			totalResponseHours += firstResponse.Sub(openedAt).Hours()
			responseSamples++
		}
		resolvedAt := parseRFC3339(item.Values["resolved_at"])
		if !resolvedAt.IsZero() {
			resolvedDay := resolvedAt.UTC().Format("2006-01-02")
			if bucket, ok := trendBuckets[resolvedDay]; ok {
				bucket["resolved"]++
			}
			if resolvedDay == now.UTC().Format("2006-01-02") {
				overview["resolved_today"] = crmIntValue(overview["resolved_today"]) + 1
			}
			if !openedAt.IsZero() {
				totalResolutionHours += resolvedAt.Sub(openedAt).Hours()
				resolutionSamples++
			}
		}
	}
	queueRows := sortedCountRows(queueCounts, "queue_code", "open_tickets")
	if len(queueRows) > 0 {
		overview["priority_queue_code"] = crmTextValue(queueRows[0]["queue_code"])
		overview["priority_queue_open_tickets"] = crmIntValue(queueRows[0]["open_tickets"])
	}
	customerRows := sortedCountRows(customerCounts, "party_name", "open_tickets")
	if len(customerRows) > 0 {
		overview["priority_customer_name"] = crmTextValue(customerRows[0]["party_name"])
		overview["priority_customer_open_tickets"] = crmIntValue(customerRows[0]["open_tickets"])
	}
	priorityRows := sortedPriorityRows(priorityCounts)
	channelRows := sortedCountRows(channelCounts, "source_channel", "count")
	statusRows := sortedCountRows(statusCounts, "status", "count")
	trendRows := sortedTrendRows(trendBuckets)
	return map[string]any{
		"generated_at": now.Format(time.RFC3339),
		"overview": map[string]any{
			"open_tickets":                   crmIntValue(overview["open_tickets"]),
			"overdue_tickets":                crmIntValue(overview["overdue_tickets"]),
			"unassigned_tickets":             crmIntValue(overview["unassigned_tickets"]),
			"resolved_today":                 crmIntValue(overview["resolved_today"]),
			"first_response_breaches":        crmIntValue(overview["first_response_breaches"]),
			"first_response_hours":           avgCRMFloat(totalResponseHours, responseSamples),
			"resolution_hours":               avgCRMFloat(totalResolutionHours, resolutionSamples),
			"priority_queue_code":            crmTextValue(overview["priority_queue_code"]),
			"priority_queue_open_tickets":    crmIntValue(overview["priority_queue_open_tickets"]),
			"priority_customer_name":         crmTextValue(overview["priority_customer_name"]),
			"priority_customer_open_tickets": crmIntValue(overview["priority_customer_open_tickets"]),
			"overdue_ticket_title":           crmTextValue(overview["overdue_ticket_title"]),
		},
		"tables": map[string]any{
			"queues":   queueRows,
			"statuses": statusRows,
		},
		"charts": map[string]any{
			"queues":     queueRows,
			"priorities": priorityRows,
			"channels":   channelRows,
			"statuses":   statusRows,
		},
		"trends": map[string]any{
			"tickets": trendRows,
		},
	}, nil
}

func (s *CRMCoreService) SummaryPayload(now time.Time) (map[string]any, error) {
	return s.ServiceSummaryPayload(now)
}

func (s *CRMCoreService) SalesSummaryPayload(now time.Time) (map[string]any, error) {
	if err := s.ensureConfigured(); err != nil {
		return nil, err
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	leads, _ := s.listRecords("crm_lead", nil)
	opps, _ := s.listRecords("crm_opportunity", nil)
	activities, _ := s.listRecords("crm_activity", nil)
	overview := map[string]any{
		"open_leads":                 0,
		"open_opportunities":         0,
		"pipeline_value":             0.0,
		"won_value":                  0.0,
		"stale_opportunities":        0,
		"activity_coverage":          0,
		"priority_pipeline_value":    0.0,
		"priority_customer_name":     "",
		"priority_opportunity_title": "",
	}
	stageCounts := map[string]int{}
	stageValues := map[string]float64{}
	ownerCounts := map[string]int{}
	staleRows := make([]map[string]any, 0)
	trendBuckets := make(map[string]map[string]float64)
	for index := 6; index >= 0; index-- {
		day := now.AddDate(0, 0, -index).Format("2006-01-02")
		trendBuckets[day] = map[string]float64{"open": 0, "won": 0}
	}
	for _, item := range leads {
		if crmLeadIsOpen(crmTextValue(item.Values["status"])) {
			overview["open_leads"] = crmIntValue(overview["open_leads"]) + 1
		}
	}
	for _, item := range opps {
		stage := crmFirstNonEmpty(crmTextValue(item.Values["stage"]), "new")
		stageCounts[stage]++
		stageValues[stage] += crmFloatValue(item.Values["estimated_value"])
		if crmOpportunityIsOpen(stage) {
			overview["open_opportunities"] = crmIntValue(overview["open_opportunities"]) + 1
			value := crmFloatValue(item.Values["estimated_value"])
			overview["pipeline_value"] = roundCRMFloat(crmFloatValue(overview["pipeline_value"]) + value)
			if nextActionAt := parseRFC3339(item.Values["next_action_at"]); nextActionAt.IsZero() || nextActionAt.Before(now.Add(-48*time.Hour)) {
				overview["stale_opportunities"] = crmIntValue(overview["stale_opportunities"]) + 1
				staleRows = append(staleRows, map[string]any{
					"opportunity_id":  item.ID,
					"title":           crmTextValue(item.Values["title"]),
					"party_id":        crmTextValue(item.Values["party_id"]),
					"party_name":      crmTextValue(item.Values["party_name"]),
					"stage":           stage,
					"estimated_value": roundCRMFloat(value),
					"next_action_at":  crmTextValue(item.Values["next_action_at"]),
				})
			}
			day := item.UpdatedAt.UTC().Format("2006-01-02")
			if bucket, ok := trendBuckets[day]; ok {
				bucket["open"] += crmFloatValue(item.Values["estimated_value"])
			}
		}
		if stage == "won" {
			overview["won_value"] = roundCRMFloat(crmFloatValue(overview["won_value"]) + crmFloatValue(item.Values["estimated_value"]))
			day := item.UpdatedAt.UTC().Format("2006-01-02")
			if bucket, ok := trendBuckets[day]; ok {
				bucket["won"] += crmFloatValue(item.Values["estimated_value"])
			}
		}
		owner := crmFirstNonEmpty(crmTextValue(item.Values["owner_user_id"]), "unassigned")
		ownerCounts[owner]++
	}
	for _, item := range activities {
		if crmTextValue(item.Values["status"]) == "completed" {
			overview["activity_coverage"] = crmIntValue(overview["activity_coverage"]) + 1
		}
	}
	stageRows := make([]map[string]any, 0, len(stageCounts))
	for stage, count := range stageCounts {
		stageRows = append(stageRows, map[string]any{
			"stage":             stage,
			"opportunity_count": count,
			"pipeline_value":    roundCRMFloat(stageValues[stage]),
			"value":             roundCRMFloat(stageValues[stage]),
		})
	}
	sort.Slice(stageRows, func(i, j int) bool {
		left := crmOpportunityStageRank(crmTextValue(stageRows[i]["stage"]))
		right := crmOpportunityStageRank(crmTextValue(stageRows[j]["stage"]))
		return left > right
	})
	sort.SliceStable(staleRows, func(i, j int) bool {
		left := parseRFC3339(staleRows[i]["next_action_at"])
		right := parseRFC3339(staleRows[j]["next_action_at"])
		if left.Equal(right) {
			return crmFloatValue(staleRows[i]["estimated_value"]) > crmFloatValue(staleRows[j]["estimated_value"])
		}
		if left.IsZero() {
			return true
		}
		if right.IsZero() {
			return false
		}
		return left.Before(right)
	})
	if len(staleRows) > 0 {
		overview["priority_opportunity_title"] = crmTextValue(staleRows[0]["title"])
		overview["priority_customer_name"] = crmTextValue(staleRows[0]["party_name"])
		overview["priority_pipeline_value"] = roundCRMFloat(crmFloatValue(staleRows[0]["estimated_value"]))
	}
	ownerRows := sortedCountRows(ownerCounts, "owner_user_id", "count")
	trendRows := make([]map[string]any, 0, len(trendBuckets))
	days := make([]string, 0, len(trendBuckets))
	for day := range trendBuckets {
		days = append(days, day)
	}
	sort.Strings(days)
	for _, day := range days {
		bucket := trendBuckets[day]
		trendRows = append(trendRows, map[string]any{
			"label": day,
			"open":  roundCRMFloat(bucket["open"]),
			"won":   roundCRMFloat(bucket["won"]),
		})
	}
	return map[string]any{
		"generated_at": now.Format(time.RFC3339),
		"overview":     overview,
		"tables": map[string]any{
			"stages":              stageRows,
			"stale_opportunities": staleRows,
		},
		"charts": map[string]any{
			"pipeline_stages": stageRows,
			"owner_activity":  ownerRows,
		},
		"trends": map[string]any{
			"pipeline": trendRows,
		},
	}, nil
}

func (s *CRMCoreService) OpportunityPipelineSummary(now time.Time) (map[string]any, error) {
	return s.SalesSummaryPayload(now)
}

func (s *CRMCoreService) ensureConfigured() error {
	if s == nil || s.models == nil {
		return shared.Validation("crm service is not configured")
	}
	return nil
}

func (s *CRMCoreService) getRecord(modelKey, recordID string) (model.Record, error) {
	if err := s.ensureConfigured(); err != nil {
		return model.Record{}, err
	}
	return s.models.Get(modelKey, strings.TrimSpace(recordID))
}

func (s *CRMCoreService) listRecords(modelKey string, filters map[string]string) ([]model.Record, error) {
	if err := s.ensureConfigured(); err != nil {
		return nil, err
	}
	query := model.Query{
		Filters:  cloneStringMap(filters),
		Page:     1,
		PageSize: model.MaxPageSize,
	}
	items := make([]model.Record, 0, model.MaxPageSize)
	for {
		pageItems, total, err := s.models.List(modelKey, query)
		if err != nil {
			return nil, err
		}
		items = append(items, pageItems...)
		if len(pageItems) == 0 || len(items) >= total {
			return items, nil
		}
		query.Page++
	}
}

func (s *CRMCoreService) applyAssignmentDefaults(values map[string]any) {
	if strings.TrimSpace(crmTextValue(values["assignee_user_id"])) != "" && strings.TrimSpace(crmTextValue(values["queue_code"])) != "" {
		return
	}
	rules, err := s.listRecords("crm_assignment_rule", nil)
	if err != nil {
		return
	}
	sort.Slice(rules, func(i, j int) bool {
		left := crmIntValue(rules[i].Values["rank"])
		right := crmIntValue(rules[j].Values["rank"])
		if left != right {
			return left < right
		}
		return rules[i].CreatedAt.Before(rules[j].CreatedAt)
	})
	for _, item := range rules {
		if crmTextValue(item.Values["status"]) != "active" {
			continue
		}
		if !crmAssignmentMatches(item.Values, values) {
			continue
		}
		if strings.TrimSpace(crmTextValue(values["queue_code"])) == "" {
			values["queue_code"] = crmFirstNonEmpty(crmTextValue(item.Values["assign_queue_code"]), crmTextValue(item.Values["queue_code"]))
		}
		if strings.TrimSpace(crmTextValue(values["assignee_user_id"])) == "" {
			values["assignee_user_id"] = crmTextValue(item.Values["assign_user_id"])
		}
		return
	}
}

func (s *CRMCoreService) applySLADefaults(values map[string]any) {
	if strings.TrimSpace(crmTextValue(values["first_response_due_at"])) != "" && strings.TrimSpace(crmTextValue(values["due_at"])) != "" {
		return
	}
	now := time.Now().UTC()
	policies, err := s.listRecords("crm_sla_policy", nil)
	if err == nil {
		for _, item := range policies {
			if crmTextValue(item.Values["status"]) != "active" {
				continue
			}
			if !crmSLAMatches(item.Values, values) {
				continue
			}
			if strings.TrimSpace(crmTextValue(values["first_response_due_at"])) == "" {
				values["first_response_due_at"] = now.Add(time.Duration(crmFloatValue(item.Values["first_response_hours"]) * float64(time.Hour))).Format(time.RFC3339)
			}
			if strings.TrimSpace(crmTextValue(values["due_at"])) == "" {
				values["due_at"] = now.Add(time.Duration(crmFloatValue(item.Values["resolution_hours"]) * float64(time.Hour))).Format(time.RFC3339)
			}
			return
		}
	}
	if strings.TrimSpace(crmTextValue(values["queue_code"])) != "" {
		queues, err := s.listRecords("crm_queue", map[string]string{"code": crmTextValue(values["queue_code"])})
		if err == nil && len(queues) > 0 {
			queue := queues[0]
			if strings.TrimSpace(crmTextValue(values["first_response_due_at"])) == "" {
				values["first_response_due_at"] = now.Add(time.Duration(crmFloatValue(queue.Values["triage_sla_hours"]) * float64(time.Hour))).Format(time.RFC3339)
			}
			if strings.TrimSpace(crmTextValue(values["due_at"])) == "" {
				values["due_at"] = now.Add(time.Duration(crmFloatValue(queue.Values["resolution_sla_hours"]) * float64(time.Hour))).Format(time.RFC3339)
			}
		}
	}
}

func (s *CRMCoreService) createTicketActivityRecord(actorID string, values map[string]any) error {
	if s == nil || s.models == nil {
		return nil
	}
	next := cloneAnyMap(values)
	if strings.TrimSpace(crmTextValue(next["occurred_at"])) == "" {
		next["occurred_at"] = time.Now().UTC().Format(time.RFC3339)
	}
	_, err := s.models.Create("crm_ticket_activity", actorID, next)
	return err
}

func (s *CRMCoreService) createActivityRecord(actorID string, values map[string]any) (model.Record, error) {
	if err := s.ensureConfigured(); err != nil {
		return model.Record{}, err
	}
	next := cloneAnyMap(values)
	if strings.TrimSpace(crmTextValue(next["activity_number"])) == "" {
		next["activity_number"] = GenerateCRMActivityNumber(time.Now().UTC())
	}
	if strings.TrimSpace(crmTextValue(next["status"])) == "" {
		next["status"] = "open"
	}
	if strings.TrimSpace(crmTextValue(next["party_name"])) == "" {
		next["party_name"] = s.resolvePartyName(next["party_id"])
	}
	return s.models.Create("crm_activity", actorID, next)
}

func (s *CRMCoreService) ResolveCustomerPartyID(query string) string {
	needle := strings.ToLower(strings.TrimSpace(query))
	if needle == "" || s == nil {
		return ""
	}
	items, err := s.listRecords("party", nil)
	if err != nil {
		return ""
	}
	exactID := ""
	exactName := ""
	containsID := ""
	containsName := ""
	for _, item := range items {
		partyID := strings.TrimSpace(item.ID)
		name := strings.ToLower(strings.TrimSpace(crmTextValue(item.Values["name"])))
		if partyID == "" || name == "" {
			continue
		}
		if partyID == strings.TrimSpace(query) {
			return partyID
		}
		if name == needle {
			if exactName == "" || partyID > exactID {
				exactID = partyID
				exactName = name
			}
			continue
		}
		if strings.Contains(name, needle) {
			if containsName == "" || partyID > containsID {
				containsID = partyID
				containsName = name
			}
		}
	}
	if exactID != "" {
		return exactID
	}
	return containsID
}

func (s *CRMCoreService) resolvePartyName(value any) string {
	partyID := strings.TrimSpace(crmTextValue(value))
	if partyID == "" || s == nil || s.models == nil {
		return ""
	}
	record, err := s.models.Get("party", partyID)
	if err != nil {
		return ""
	}
	return crmTextValue(record.Values["name"])
}

func (s *CRMCoreService) ticketRiskLevel(values map[string]any, now time.Time) string {
	if dueAt := parseRFC3339(values["due_at"]); !dueAt.IsZero() && dueAt.Before(now) {
		return "overdue"
	}
	if firstResponseDueAt := parseRFC3339(values["first_response_due_at"]); !firstResponseDueAt.IsZero() && firstResponseDueAt.Before(now) && strings.TrimSpace(crmTextValue(values["first_response_at"])) == "" {
		return "first_response_breach"
	}
	return "healthy"
}

func crmAssignmentMatches(ruleValues map[string]any, ticketValues map[string]any) bool {
	return crmRuleFieldMatch(ruleValues, "queue_code", ticketValues) &&
		crmRuleFieldMatch(ruleValues, "source_channel", ticketValues) &&
		crmRuleFieldMatch(ruleValues, "issue_category", ticketValues) &&
		crmRuleFieldMatch(ruleValues, "priority", ticketValues) &&
		crmRuleFieldMatch(ruleValues, "severity", ticketValues)
}

func crmSLAMatches(ruleValues map[string]any, ticketValues map[string]any) bool {
	return crmRuleFieldMatch(ruleValues, "queue_code", ticketValues) &&
		crmRuleFieldMatch(ruleValues, "source_channel", ticketValues) &&
		crmRuleFieldMatch(ruleValues, "priority", ticketValues) &&
		crmRuleFieldMatch(ruleValues, "severity", ticketValues)
}

func crmRuleFieldMatch(ruleValues map[string]any, key string, input map[string]any) bool {
	rule := strings.TrimSpace(crmTextValue(ruleValues[key]))
	if rule == "" {
		return true
	}
	return strings.EqualFold(rule, strings.TrimSpace(crmTextValue(input[key])))
}

func paginateCRMRecords(items []model.Record, page, pageSize int) ([]model.Record, int, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > model.MaxPageSize {
		pageSize = model.DefaultPageSize
	}
	total := len(items)
	start := (page - 1) * pageSize
	if start >= total {
		return []model.Record{}, total, nil
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return items[start:end], total, nil
}

func recordMatchesNeedle(item model.Record, needle string, fields ...string) bool {
	for _, key := range fields {
		if strings.Contains(strings.ToLower(crmTextValue(item.Values[key])), needle) {
			return true
		}
	}
	return false
}

func crmTicketIsOpen(status string) bool {
	switch strings.TrimSpace(status) {
	case "resolved", "closed", "cancelled":
		return false
	default:
		return true
	}
}

func crmLeadIsOpen(status string) bool {
	switch strings.TrimSpace(status) {
	case "disqualified", "converted", "closed":
		return false
	default:
		return true
	}
}

func crmOpportunityIsOpen(stage string) bool {
	switch strings.TrimSpace(stage) {
	case "won", "lost":
		return false
	default:
		return true
	}
}

func crmPriorityRank(priority string) int {
	switch strings.TrimSpace(strings.ToLower(priority)) {
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

func crmLeadRank(status string) int {
	switch strings.TrimSpace(strings.ToLower(status)) {
	case "qualified":
		return 4
	case "contacted":
		return 3
	case "new":
		return 2
	default:
		return 1
	}
}

func crmOpportunityStageRank(stage string) int {
	switch strings.TrimSpace(strings.ToLower(stage)) {
	case "negotiation":
		return 6
	case "proposal":
		return 5
	case "qualified":
		return 4
	case "new":
		return 3
	case "won":
		return 2
	case "lost":
		return 1
	default:
		return 0
	}
}

func crmOpportunityStatusForStage(stage string) string {
	switch strings.TrimSpace(strings.ToLower(stage)) {
	case "won":
		return "won"
	case "lost":
		return "lost"
	default:
		return "open"
	}
}

func GenerateCRMTicketNumber(now time.Time) string {
	return fmt.Sprintf("CRM-%s", now.UTC().Format("20060102150405.000"))
}

func GenerateCRMLeadNumber(now time.Time) string {
	return fmt.Sprintf("LEAD-%s", now.UTC().Format("20060102150405.000"))
}

func GenerateCRMOpportunityNumber(now time.Time) string {
	return fmt.Sprintf("OPP-%s", now.UTC().Format("20060102150405.000"))
}

func GenerateCRMActivityNumber(now time.Time) string {
	return fmt.Sprintf("ACT-%s", now.UTC().Format("20060102150405.000"))
}

func parseRFC3339(value any) time.Time {
	text := strings.TrimSpace(crmTextValue(value))
	if text == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339, text)
	if err != nil {
		return time.Time{}
	}
	return parsed
}

func crmTextValue(value any) string {
	if value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	default:
		return fmt.Sprintf("%v", value)
	}
}

func crmIntValue(value any) int {
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

func crmFloatValue(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	default:
		return 0
	}
}

func crmFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func cloneStringMap(input map[string]string) map[string]string {
	if input == nil {
		return nil
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func cloneAnyMap(input map[string]any) map[string]any {
	if input == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func roundCRMFloat(value float64) float64 {
	return float64(int(value*100)) / 100
}

func avgCRMFloat(total float64, count int) float64 {
	if count <= 0 {
		return 0
	}
	return roundCRMFloat(total / float64(count))
}

func sortedCountRows(input map[string]int, keyName, valueName string) []map[string]any {
	rows := make([]map[string]any, 0, len(input))
	for key, count := range input {
		rows = append(rows, map[string]any{keyName: key, valueName: count, "value": count})
	}
	sort.Slice(rows, func(i, j int) bool {
		left := crmIntValue(rows[i][valueName])
		right := crmIntValue(rows[j][valueName])
		if left != right {
			return left > right
		}
		return crmTextValue(rows[i][keyName]) < crmTextValue(rows[j][keyName])
	})
	return rows
}

func sortedPriorityRows(input map[string]int) []map[string]any {
	rows := make([]map[string]any, 0, len(input))
	for key, count := range input {
		rows = append(rows, map[string]any{"priority": key, "count": count, "value": count})
	}
	sort.Slice(rows, func(i, j int) bool {
		left := crmPriorityRank(crmTextValue(rows[i]["priority"]))
		right := crmPriorityRank(crmTextValue(rows[j]["priority"]))
		return left > right
	})
	return rows
}

func sortedTrendRows(input map[string]map[string]int) []map[string]any {
	rows := make([]map[string]any, 0, len(input))
	days := make([]string, 0, len(input))
	for day := range input {
		days = append(days, day)
	}
	sort.Strings(days)
	for _, day := range days {
		bucket := input[day]
		rows = append(rows, map[string]any{
			"label":    day,
			"created":  bucket["created"],
			"resolved": bucket["resolved"],
		})
	}
	return rows
}

func ensureCRMPartySummary(items map[string]map[string]any, partyID, partyName string) map[string]any {
	if item, ok := items[partyID]; ok {
		return item
	}
	item := map[string]any{
		"party_id":                partyID,
		"party_name":              partyName,
		"customer_segment":        "",
		"member_tier":             "",
		"open_tickets":            0,
		"overdue_tickets":         0,
		"open_opportunities":      0,
		"open_opportunity_value":  0.0,
		"health_score":            100,
		"recent_issue_categories": []string{},
	}
	items[partyID] = item
	return item
}

func appendStringIfMissing(value any, input string) []string {
	items, _ := value.([]string)
	for _, item := range items {
		if strings.EqualFold(item, input) {
			return items
		}
	}
	return append(items, input)
}

func sumCRMIntField(items []map[string]any, key string) int {
	total := 0
	for _, item := range items {
		total += crmIntValue(item[key])
	}
	return total
}

func firstRecordOrNil(items []model.Record) any {
	if len(items) == 0 {
		return nil
	}
	return items[0]
}

func firstRecordValue(items []model.Record, key string) any {
	if len(items) == 0 {
		return nil
	}
	return items[0].Values[key]
}
