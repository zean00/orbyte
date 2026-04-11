package application

import (
	"testing"
	"time"

	"orbyte/internal/platform/model"
)

func TestCRMCoreServiceSummaryPayload(t *testing.T) {
	models := newCRMTestModelService(t)
	now := time.Date(2026, 4, 11, 10, 0, 0, 0, time.UTC)
	svc := NewCRMCoreService(models)

	if _, err := models.Create("crm_queue", "user_admin", map[string]any{"code": "SUPPORT", "name": "Support"}); err != nil {
		t.Fatalf("create queue: %v", err)
	}
	open, err := svc.CreateTicket("user_admin", map[string]any{
		"title":                 "Coffee machine failure",
		"queue_code":            "SUPPORT",
		"priority":              "urgent",
		"due_at":                now.Add(-time.Hour).Format(time.RFC3339),
		"first_response_due_at": now.Add(-30 * time.Minute).Format(time.RFC3339),
	})
	if err != nil {
		t.Fatalf("create open ticket: %v", err)
	}
	resolved, err := svc.CreateTicket("user_admin", map[string]any{
		"title":       "Receipt paper replaced",
		"queue_code":  "SUPPORT",
		"priority":    "medium",
		"status":      "resolved",
		"resolved_at": now.Format(time.RFC3339),
	})
	if err != nil {
		t.Fatalf("create resolved ticket: %v", err)
	}
	open.CreatedAt = now.Add(-2 * time.Hour)
	resolved.CreatedAt = now.Add(-3 * time.Hour)
	if err := models.WithRawRecordSave(open); err != nil {
		t.Fatalf("save open ticket timestamps: %v", err)
	}
	if err := models.WithRawRecordSave(resolved); err != nil {
		t.Fatalf("save resolved ticket timestamps: %v", err)
	}

	payload, err := svc.SummaryPayload(now)
	if err != nil {
		t.Fatalf("summary payload: %v", err)
	}
	overview := payload["overview"].(map[string]any)
	if got := crmTestIntValue(overview["open_tickets"]); got != 1 {
		t.Fatalf("expected 1 open ticket, got %d", got)
	}
	if got := crmTestIntValue(overview["overdue_tickets"]); got != 1 {
		t.Fatalf("expected 1 overdue ticket, got %d", got)
	}
	if got := crmTestIntValue(overview["first_response_breaches"]); got != 1 {
		t.Fatalf("expected 1 first response breach, got %d", got)
	}
	if got := crmTestIntValue(overview["resolved_today"]); got != 1 {
		t.Fatalf("expected 1 resolved today, got %d", got)
	}
}

func TestCRMCoreServiceUpdateMergesExistingValues(t *testing.T) {
	models := newCRMTestModelService(t)
	svc := NewCRMCoreService(models)

	record, err := svc.CreateTicket("user_admin", map[string]any{"title": "Printer jam"})
	if err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	updated, err := svc.UpdateTicket(record.ID, "user_admin", map[string]any{"status": "open"}, record.Version)
	if err != nil {
		t.Fatalf("update ticket: %v", err)
	}
	if updated.Values["title"] != "Printer jam" {
		t.Fatalf("expected title to be preserved, got %+v", updated.Values["title"])
	}
	if updated.Values["status"] != "open" {
		t.Fatalf("expected status open, got %+v", updated.Values["status"])
	}
}

func TestCRMCoreServiceTicketTimeline(t *testing.T) {
	models := newCRMTestModelService(t)
	svc := NewCRMCoreService(models)
	if _, err := models.Create("crm_queue", "user_admin", map[string]any{"code": "SUPPORT", "name": "Support"}); err != nil {
		t.Fatalf("create queue: %v", err)
	}
	ticket, err := svc.CreateTicket("user_admin", map[string]any{
		"title":      "Need refund review",
		"queue_code": "SUPPORT",
		"priority":   "high",
	})
	if err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	if _, err := svc.AddTicketComment(ticket.ID, "user_admin", "Customer called back", "public_reply"); err != nil {
		t.Fatalf("add comment: %v", err)
	}
	if _, err := svc.ResolveTicket(ticket.ID, "user_admin", "Refund approved", false, 0); err != nil {
		t.Fatalf("resolve ticket: %v", err)
	}

	items, err := svc.TicketTimeline(ticket.ID)
	if err != nil {
		t.Fatalf("ticket timeline: %v", err)
	}
	if len(items) < 3 {
		t.Fatalf("expected activity-rich timeline, got %d items", len(items))
	}
}

func TestCRMCoreServiceCustomer360(t *testing.T) {
	models := newCRMTestModelService(t)
	svc := NewCRMCoreService(models)
	now := time.Date(2026, 4, 11, 10, 0, 0, 0, time.UTC)

	party, err := models.Create("party", "user_admin", map[string]any{"name": "Acme Retail", "status": "active"})
	if err != nil {
		t.Fatalf("create party: %v", err)
	}
	if _, err := models.Create("customer_profile", "user_admin", map[string]any{
		"party_id":         party.ID,
		"customer_name":    "Acme Retail",
		"customer_segment": "strategic",
		"member_tier":      "gold",
		"status":           "active",
	}); err != nil {
		t.Fatalf("create customer profile: %v", err)
	}
	if _, err := models.Create("party_contact", "user_admin", map[string]any{
		"party_id":      party.ID,
		"contact_name":  "Alya",
		"contact_kind":  "account_owner",
		"status":        "active",
		"is_primary":    true,
	}); err != nil {
		t.Fatalf("create party contact: %v", err)
	}
	if _, err := models.Create("crm_queue", "user_admin", map[string]any{"code": "SUPPORT", "name": "Support"}); err != nil {
		t.Fatalf("create queue: %v", err)
	}
	if _, err := svc.CreateTicket("user_admin", map[string]any{
		"title":      "Delivery complaint",
		"party_id":   party.ID,
		"party_name": "Acme Retail",
		"queue_code": "SUPPORT",
		"priority":   "high",
	}); err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	if _, err := svc.CreateOpportunity("user_admin", map[string]any{
		"title":           "Annual coffee supply",
		"party_id":        party.ID,
		"party_name":      "Acme Retail",
		"stage":           "proposal",
		"estimated_value": 12500000,
	}); err != nil {
		t.Fatalf("create opportunity: %v", err)
	}
	if _, err := svc.CreateActivity("user_admin", map[string]any{
		"activity_type": "call",
		"subject":       "Follow-up call",
		"related_kind":  "party",
		"related_id":    party.ID,
		"party_id":      party.ID,
		"party_name":    "Acme Retail",
		"status":        "completed",
		"completed_at":  now.Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("create activity: %v", err)
	}

	payload, err := svc.Customer360(party.ID, now)
	if err != nil {
		t.Fatalf("customer 360: %v", err)
	}
	overview := payload["overview"].(map[string]any)
	if got := crmTestIntValue(overview["open_tickets"]); got != 1 {
		t.Fatalf("expected 1 open ticket, got %d", got)
	}
	if got := crmTestIntValue(overview["open_opportunity_value"]); got != 12500000 {
		t.Fatalf("expected 12500000 open opportunity value, got %d", got)
	}
}

func TestCRMCoreServiceAssignTicketKeepsResolutionNotesEmpty(t *testing.T) {
	models := newCRMTestModelService(t)
	svc := NewCRMCoreService(models)

	if _, err := models.Create("crm_queue", "user_admin", map[string]any{"code": "SUPPORT", "name": "Support"}); err != nil {
		t.Fatalf("create queue: %v", err)
	}
	ticket, err := svc.CreateTicket("user_admin", map[string]any{
		"title":      "Delivery complaint",
		"party_name": "Acme Retail",
		"queue_code": "SUPPORT",
		"priority":   "high",
	})
	if err != nil {
		t.Fatalf("create ticket: %v", err)
	}

	updated, err := svc.AssignTicket(ticket.ID, "user_admin", "user_owner", "Assigned to support lead", ticket.Version)
	if err != nil {
		t.Fatalf("assign ticket: %v", err)
	}
	if got := crmTextValue(updated.Values["resolution_notes"]); got != "" {
		t.Fatalf("expected empty resolution notes after assignment, got %q", got)
	}

	timeline, err := svc.TicketTimeline(ticket.ID)
	if err != nil {
		t.Fatalf("ticket timeline: %v", err)
	}
	foundNote := false
	for _, item := range timeline {
		payload, _ := item["payload"].(model.Record)
		if crmTextValue(payload.Values["activity_type"]) == "assignment_note" && crmTextValue(payload.Values["note"]) == "Assigned to support lead" {
			foundNote = true
			break
		}
	}
	if !foundNote {
		t.Fatalf("expected assignment note activity in timeline")
	}
}

func TestCRMCoreServiceSalesSummaryPayload(t *testing.T) {
	models := newCRMTestModelService(t)
	svc := NewCRMCoreService(models)
	now := time.Date(2026, 4, 11, 10, 0, 0, 0, time.UTC)

	party, err := models.Create("party", "user_admin", map[string]any{"name": "North Roast", "status": "active"})
	if err != nil {
		t.Fatalf("create party: %v", err)
	}
	if _, err := svc.CreateLead("user_admin", map[string]any{
		"title":      "New wholesale lead",
		"party_id":   party.ID,
		"party_name": "North Roast",
		"status":     "qualified",
		"rating":     "hot",
	}); err != nil {
		t.Fatalf("create lead: %v", err)
	}
	opportunity, err := svc.CreateOpportunity("user_admin", map[string]any{
		"title":           "Quarterly supply deal",
		"party_id":        party.ID,
		"party_name":      "North Roast",
		"stage":           "proposal",
		"estimated_value": 22000000,
		"next_action_at":  now.Add(-72 * time.Hour).Format(time.RFC3339),
		"owner_user_id":   "user_admin",
	})
	if err != nil {
		t.Fatalf("create opportunity: %v", err)
	}
	if _, err := svc.CreateActivity("user_admin", map[string]any{
		"activity_type": "meeting",
		"subject":       "Proposal review",
		"related_kind":  "opportunity",
		"related_id":    opportunity.ID,
		"party_id":      party.ID,
		"party_name":    "North Roast",
		"status":        "completed",
		"completed_at":  now.Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("create activity: %v", err)
	}

	payload, err := svc.SalesSummaryPayload(now)
	if err != nil {
		t.Fatalf("sales summary: %v", err)
	}
	overview := payload["overview"].(map[string]any)
	if got := crmTestIntValue(overview["open_leads"]); got != 1 {
		t.Fatalf("expected 1 open lead, got %d", got)
	}
	if got := crmTestIntValue(overview["open_opportunities"]); got != 1 {
		t.Fatalf("expected 1 open opportunity, got %d", got)
	}
	if got := crmTestIntValue(overview["pipeline_value"]); got != 22000000 {
		t.Fatalf("expected 22000000 pipeline value, got %d", got)
	}
	if got := crmTestIntValue(overview["stale_opportunities"]); got != 1 {
		t.Fatalf("expected 1 stale opportunity, got %d", got)
	}
}

func newCRMTestModelService(t *testing.T) *model.Service {
	t.Helper()
	models := model.NewService()
	for _, def := range []model.Definition{
		{
			Key:                 "party",
			DisplayName:         "Party",
			Version:             "v1",
			CreatePermissionKey: "party.create",
			ListPermissionKey:   "party.list",
			ReadPermissionKey:   "party.read",
			UpdatePermissionKey: "party.update",
			Fields: []model.FieldDefinition{
				{Key: "name", Label: "Name", Type: "string", Required: true},
				{Key: "status", Label: "Status", Type: "string"},
			},
		},
		{
			Key:                 "customer_profile",
			DisplayName:         "Customer Profile",
			Version:             "v1",
			CreatePermissionKey: "customer.create",
			ListPermissionKey:   "customer.list",
			ReadPermissionKey:   "customer.read",
			UpdatePermissionKey: "customer.update",
			Fields: []model.FieldDefinition{
				{Key: "party_id", Label: "Party", Type: "string", Required: true},
				{Key: "customer_name", Label: "Name", Type: "string"},
				{Key: "customer_segment", Label: "Segment", Type: "string"},
				{Key: "member_tier", Label: "Tier", Type: "string"},
				{Key: "status", Label: "Status", Type: "string"},
			},
		},
		{
			Key:                 "party_contact",
			DisplayName:         "Party Contact",
			Version:             "v1",
			CreatePermissionKey: "party_contact.create",
			ListPermissionKey:   "party_contact.list",
			ReadPermissionKey:   "party_contact.read",
			UpdatePermissionKey: "party_contact.update",
			Fields: []model.FieldDefinition{
				{Key: "party_id", Label: "Party", Type: "string", Required: true},
				{Key: "contact_name", Label: "Contact", Type: "string"},
				{Key: "contact_kind", Label: "Kind", Type: "string"},
				{Key: "status", Label: "Status", Type: "string"},
				{Key: "is_primary", Label: "Primary", Type: "boolean"},
			},
		},
		{
			Key:                 "crm_queue",
			DisplayName:         "CRM Queue",
			Version:             "v1",
			CreatePermissionKey: "crm_queue.create",
			ListPermissionKey:   "crm_queue.list",
			ReadPermissionKey:   "crm_queue.read",
			UpdatePermissionKey: "crm_queue.update",
			Fields: []model.FieldDefinition{
				{Key: "code", Label: "Code", Type: "string", Required: true},
				{Key: "name", Label: "Name", Type: "string", Required: true},
				{Key: "triage_sla_hours", Label: "Triage SLA", Type: "number"},
				{Key: "resolution_sla_hours", Label: "Resolution SLA", Type: "number"},
				{Key: "status", Label: "Status", Type: "string"},
			},
		},
		{
			Key:                 "crm_sla_policy",
			DisplayName:         "CRM SLA Policy",
			Version:             "v1",
			CreatePermissionKey: "crm_sla_policy.create",
			ListPermissionKey:   "crm_sla_policy.list",
			ReadPermissionKey:   "crm_sla_policy.read",
			UpdatePermissionKey: "crm_sla_policy.update",
			Fields: []model.FieldDefinition{
				{Key: "queue_code", Label: "Queue", Type: "string"},
				{Key: "source_channel", Label: "Channel", Type: "string"},
				{Key: "priority", Label: "Priority", Type: "string"},
				{Key: "severity", Label: "Severity", Type: "string"},
				{Key: "first_response_hours", Label: "First Response", Type: "number"},
				{Key: "resolution_hours", Label: "Resolution", Type: "number"},
				{Key: "status", Label: "Status", Type: "string"},
			},
		},
		{
			Key:                 "crm_assignment_rule",
			DisplayName:         "CRM Assignment Rule",
			Version:             "v1",
			CreatePermissionKey: "crm_assignment_rule.create",
			ListPermissionKey:   "crm_assignment_rule.list",
			ReadPermissionKey:   "crm_assignment_rule.read",
			UpdatePermissionKey: "crm_assignment_rule.update",
			Fields: []model.FieldDefinition{
				{Key: "queue_code", Label: "Queue", Type: "string"},
				{Key: "assign_queue_code", Label: "Assign Queue", Type: "string"},
				{Key: "assign_user_id", Label: "Assign User", Type: "string"},
				{Key: "source_channel", Label: "Channel", Type: "string"},
				{Key: "issue_category", Label: "Category", Type: "string"},
				{Key: "priority", Label: "Priority", Type: "string"},
				{Key: "severity", Label: "Severity", Type: "string"},
				{Key: "rank", Label: "Rank", Type: "number"},
				{Key: "status", Label: "Status", Type: "string"},
			},
		},
		{
			Key:                 "crm_ticket",
			DisplayName:         "CRM Ticket",
			Version:             "v1",
			CreatePermissionKey: "crm_ticket.create",
			ListPermissionKey:   "crm_ticket.list",
			ReadPermissionKey:   "crm_ticket.read",
			UpdatePermissionKey: "crm_ticket.update",
			Fields: []model.FieldDefinition{
				{Key: "ticket_number", Label: "Ticket Number", Type: "string"},
				{Key: "title", Label: "Title", Type: "string", Required: true},
				{Key: "description", Label: "Description", Type: "string"},
				{Key: "party_id", Label: "Party", Type: "string"},
				{Key: "party_name", Label: "Party Name", Type: "string"},
				{Key: "queue_code", Label: "Queue", Type: "string"},
				{Key: "source_channel", Label: "Source", Type: "string"},
				{Key: "priority", Label: "Priority", Type: "string"},
				{Key: "severity", Label: "Severity", Type: "string"},
				{Key: "status", Label: "Status", Type: "string"},
				{Key: "assignee_user_id", Label: "Assignee", Type: "string"},
				{Key: "opened_at", Label: "Opened", Type: "string"},
				{Key: "first_response_due_at", Label: "First Response Due", Type: "string"},
				{Key: "first_response_at", Label: "First Response", Type: "string"},
				{Key: "due_at", Label: "Due At", Type: "string"},
				{Key: "resolved_at", Label: "Resolved At", Type: "string"},
				{Key: "issue_category", Label: "Issue Category", Type: "string"},
				{Key: "resolution_notes", Label: "Resolution", Type: "string"},
				{Key: "tags_json", Label: "Tags", Type: "string"},
			},
		},
		{
			Key:                 "crm_ticket_comment",
			DisplayName:         "CRM Ticket Comment",
			Version:             "v1",
			CreatePermissionKey: "crm_ticket_comment.create",
			ListPermissionKey:   "crm_ticket_comment.list",
			ReadPermissionKey:   "crm_ticket_comment.read",
			UpdatePermissionKey: "crm_ticket_comment.update",
			Fields: []model.FieldDefinition{
				{Key: "ticket_id", Label: "Ticket", Type: "string", Required: true},
				{Key: "ticket_number", Label: "Ticket Number", Type: "string"},
				{Key: "comment_type", Label: "Type", Type: "string"},
				{Key: "body", Label: "Body", Type: "string", Required: true},
				{Key: "author_user_id", Label: "Author", Type: "string"},
				{Key: "created_at", Label: "Created At", Type: "string"},
				{Key: "party_id", Label: "Party", Type: "string"},
				{Key: "party_name", Label: "Party Name", Type: "string"},
			},
		},
		{
			Key:                 "crm_ticket_activity",
			DisplayName:         "CRM Ticket Activity",
			Version:             "v1",
			CreatePermissionKey: "crm_ticket_activity.create",
			ListPermissionKey:   "crm_ticket_activity.list",
			ReadPermissionKey:   "crm_ticket_activity.read",
			UpdatePermissionKey: "crm_ticket_activity.update",
			Fields: []model.FieldDefinition{
				{Key: "ticket_id", Label: "Ticket", Type: "string", Required: true},
				{Key: "ticket_number", Label: "Ticket Number", Type: "string"},
				{Key: "activity_type", Label: "Type", Type: "string", Required: true},
				{Key: "actor_user_id", Label: "Actor", Type: "string"},
				{Key: "assignee_user_id", Label: "Assignee", Type: "string"},
				{Key: "queue_code", Label: "Queue", Type: "string"},
				{Key: "from_status", Label: "From Status", Type: "string"},
				{Key: "to_status", Label: "To Status", Type: "string"},
				{Key: "occurred_at", Label: "Occurred At", Type: "string"},
				{Key: "note", Label: "Note", Type: "string"},
				{Key: "party_id", Label: "Party", Type: "string"},
				{Key: "party_name", Label: "Party Name", Type: "string"},
				{Key: "severity", Label: "Severity", Type: "string"},
				{Key: "priority", Label: "Priority", Type: "string"},
				{Key: "sla_breach_risk", Label: "Risk", Type: "string"},
				{Key: "source_channel", Label: "Source", Type: "string"},
				{Key: "issue_category", Label: "Category", Type: "string"},
				{Key: "ticket_status_key", Label: "Status Key", Type: "string"},
			},
		},
		{
			Key:                 "crm_lead",
			DisplayName:         "CRM Lead",
			Version:             "v1",
			CreatePermissionKey: "crm_lead.create",
			ListPermissionKey:   "crm_lead.list",
			ReadPermissionKey:   "crm_lead.read",
			UpdatePermissionKey: "crm_lead.update",
			Fields: []model.FieldDefinition{
				{Key: "lead_number", Label: "Lead Number", Type: "string"},
				{Key: "title", Label: "Title", Type: "string", Required: true},
				{Key: "party_id", Label: "Party", Type: "string"},
				{Key: "party_name", Label: "Party Name", Type: "string"},
				{Key: "contact_id", Label: "Contact", Type: "string"},
				{Key: "owner_user_id", Label: "Owner", Type: "string"},
				{Key: "source_channel", Label: "Source", Type: "string"},
				{Key: "status", Label: "Status", Type: "string"},
				{Key: "rating", Label: "Rating", Type: "string"},
				{Key: "estimated_value", Label: "Value", Type: "number"},
				{Key: "expected_close_date", Label: "Expected Close", Type: "string"},
				{Key: "next_action_at", Label: "Next Action", Type: "string"},
				{Key: "notes", Label: "Notes", Type: "string"},
			},
		},
		{
			Key:                 "crm_opportunity",
			DisplayName:         "CRM Opportunity",
			Version:             "v1",
			CreatePermissionKey: "crm_opportunity.create",
			ListPermissionKey:   "crm_opportunity.list",
			ReadPermissionKey:   "crm_opportunity.read",
			UpdatePermissionKey: "crm_opportunity.update",
			Fields: []model.FieldDefinition{
				{Key: "opportunity_number", Label: "Opportunity Number", Type: "string"},
				{Key: "title", Label: "Title", Type: "string", Required: true},
				{Key: "party_id", Label: "Party", Type: "string"},
				{Key: "party_name", Label: "Party Name", Type: "string"},
				{Key: "contact_id", Label: "Contact", Type: "string"},
				{Key: "owner_user_id", Label: "Owner", Type: "string"},
				{Key: "source_lead_id", Label: "Lead", Type: "string"},
				{Key: "stage", Label: "Stage", Type: "string"},
				{Key: "status", Label: "Status", Type: "string"},
				{Key: "estimated_value", Label: "Value", Type: "number"},
				{Key: "expected_close_date", Label: "Expected Close", Type: "string"},
				{Key: "next_action_at", Label: "Next Action", Type: "string"},
				{Key: "loss_reason", Label: "Loss Reason", Type: "string"},
				{Key: "notes", Label: "Notes", Type: "string"},
			},
		},
		{
			Key:                 "crm_activity",
			DisplayName:         "CRM Activity",
			Version:             "v1",
			CreatePermissionKey: "crm_activity.create",
			ListPermissionKey:   "crm_activity.list",
			ReadPermissionKey:   "crm_activity.read",
			UpdatePermissionKey: "crm_activity.update",
			Fields: []model.FieldDefinition{
				{Key: "activity_number", Label: "Activity Number", Type: "string"},
				{Key: "activity_type", Label: "Type", Type: "string", Required: true},
				{Key: "subject", Label: "Subject", Type: "string", Required: true},
				{Key: "related_kind", Label: "Related Kind", Type: "string"},
				{Key: "related_id", Label: "Related ID", Type: "string"},
				{Key: "party_id", Label: "Party", Type: "string"},
				{Key: "party_name", Label: "Party Name", Type: "string"},
				{Key: "owner_user_id", Label: "Owner", Type: "string"},
				{Key: "status", Label: "Status", Type: "string"},
				{Key: "due_at", Label: "Due At", Type: "string"},
				{Key: "completed_at", Label: "Completed At", Type: "string"},
				{Key: "note", Label: "Note", Type: "string"},
			},
		},
	} {
		if err := models.Register(def); err != nil {
			t.Fatalf("register %s: %v", def.Key, err)
		}
	}
	return models
}

func crmTestIntValue(value any) int {
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
