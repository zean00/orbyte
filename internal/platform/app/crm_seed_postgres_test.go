package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"orbyte/internal/modules"
	"orbyte/internal/platform/analytics"
	"orbyte/internal/platform/store"
)

type crmSeedManifest struct {
	SeededAt       string `json:"seeded_at"`
	OrganizationID string `json:"organization_id"`
	Routes         struct {
		Tickets      string `json:"tickets"`
		Queues       string `json:"queues"`
		Leads        string `json:"leads"`
		Opportunities string `json:"opportunities"`
		Customer360  string `json:"customer_360"`
		Dashboard    string `json:"dashboard"`
		Agent        string `json:"agent"`
	} `json:"routes"`
	Queue struct {
		Code string `json:"code"`
		Name string `json:"name"`
	} `json:"queue"`
	SLA struct {
		Code string `json:"code"`
		Name string `json:"name"`
	} `json:"sla"`
	Customer struct {
		PartyID string `json:"party_id"`
		Name    string `json:"name"`
		Tier    string `json:"tier"`
	} `json:"customer"`
	Ticket struct {
		ID           string `json:"id"`
		TicketNumber string `json:"ticket_number"`
		Status       string `json:"status"`
	} `json:"ticket"`
	Lead struct {
		ID         string `json:"id"`
		LeadNumber string `json:"lead_number"`
		Status     string `json:"status"`
	} `json:"lead"`
	Opportunity struct {
		ID                string  `json:"id"`
		OpportunityNumber string  `json:"opportunity_number"`
		Stage             string  `json:"stage"`
		Value             float64 `json:"value"`
	} `json:"opportunity"`
	Dashboard struct {
		BoardID   string   `json:"board_id"`
		BoardName string   `json:"board_name"`
		Widgets   []string `json:"widgets"`
	} `json:"dashboard"`
	AgentPrompts []string `json:"agent_prompts"`
}

func TestSeedCRMSyntheticScenario(t *testing.T) {
	if os.Getenv("CRM_SEED") != "1" {
		t.Skip("set CRM_SEED=1 to seed the postgres-backed CRM scenario")
	}
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("DATABASE_URL is required for postgres-backed CRM seed")
	}

	postgres, err := store.OpenFromEnv()
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	defer func() { _ = postgres.Close() }()

	manifests, err := modules.ForProfile(modules.ProfileAll)
	if err != nil {
		t.Fatalf("load business manifests: %v", err)
	}
	graph := constructServiceGraph(postgres, manifests)
	if err := seedPlatformKernel(graph.config, graph.identity, graph.modules, graph.models, graph.reporting, graph.templates, graph.reference, graph.search, graph.documents, graph.workflows, graph.policy, manifests, testBootstrapAdminPassword); err != nil {
		t.Fatalf("seed platform kernel: %v", err)
	}

	actorID := "user_admin"
	suffix := time.Now().UTC().Format("20060102150405")
	now := time.Now().UTC()

	queue := ensureModelByCode(t, graph.models, "crm_queue", "code", "CRM-SUPPORT-"+suffix, map[string]any{
		"code":                 "CRM-SUPPORT-" + suffix,
		"name":                 "CRM Support " + suffix,
		"triage_sla_hours":     2,
		"resolution_sla_hours": 12,
		"status":               "active",
	}, actorID)
	sla := ensureModelByCode(t, graph.models, "crm_sla_policy", "code", "CRM-SLA-"+suffix, map[string]any{
		"code":                 "CRM-SLA-" + suffix,
		"name":                 "CRM Priority SLA " + suffix,
		"queue_code":           textValue(queue.Values["code"]),
		"priority":             "high",
		"first_response_hours": 1,
		"resolution_hours":     8,
		"status":               "active",
	}, actorID)
	_ = ensureModelByCode(t, graph.models, "crm_assignment_rule", "code", "CRM-ASSIGN-"+suffix, map[string]any{
		"code":              "CRM-ASSIGN-" + suffix,
		"name":              "Support Queue Default " + suffix,
		"queue_code":        textValue(queue.Values["code"]),
		"assign_queue_code": textValue(queue.Values["code"]),
		"assign_user_id":    actorID,
		"rank":              10,
		"status":            "active",
	}, actorID)

	party := ensureModelByCode(t, graph.models, "party", "name", "CRM Demo Customer "+suffix, map[string]any{
		"party_type": "organization",
		"name":       "CRM Demo Customer " + suffix,
		"status":     "active",
	}, actorID)
	_ = ensureModelByCode(t, graph.models, "customer_profile", "party_id", party.ID, map[string]any{
		"party_id":         party.ID,
		"customer_name":    "CRM Demo Customer " + suffix,
		"customer_type":    "member",
		"customer_segment": "strategic",
		"member_status":    "active",
		"member_tier":      "gold",
		"status":           "active",
	}, actorID)
	contact := ensureModelByCode(t, graph.models, "party_contact", "party_id", party.ID, map[string]any{
		"party_id":      party.ID,
		"name":          "Alya CRM " + suffix,
		"contact_kind":  "person",
		"email":         "alya+" + suffix + "@example.com",
		"status":        "active",
		"is_primary":    true,
	}, actorID)

	ticket, err := graph.crmCore.CreateTicket(actorID, map[string]any{
		"title":          "Damaged shipment replacement",
		"description":    "Customer reports damaged cartons on arrival.",
		"party_id":       party.ID,
		"party_name":     textValue(party.Values["name"]),
		"queue_code":     textValue(queue.Values["code"]),
		"priority":       "high",
		"severity":       "high",
		"source_channel": "email",
		"issue_category": "logistics",
	})
	if err != nil {
		t.Fatalf("create crm ticket: %v", err)
	}
	if _, err := graph.crmCore.AddTicketComment(ticket.ID, actorID, "Replacement review started.", "internal_note"); err != nil {
		t.Fatalf("add ticket comment: %v", err)
	}
	if _, err := graph.crmCore.AssignTicket(ticket.ID, actorID, actorID, "Assigned to support lead", ticket.Version); err != nil {
		t.Fatalf("assign ticket: %v", err)
	}

	lead, err := graph.crmCore.CreateLead(actorID, map[string]any{
		"title":               "Upsell catering program",
		"party_id":            party.ID,
		"party_name":          textValue(party.Values["name"]),
		"contact_id":          contact.ID,
		"owner_user_id":       actorID,
		"source_channel":      "referral",
		"status":              "qualified",
		"rating":              "hot",
		"estimated_value":     18000000,
		"expected_close_date": now.AddDate(0, 1, 0).Format("2006-01-02"),
		"next_action_at":      now.Add(24 * time.Hour).Format(time.RFC3339),
		"notes":               "Customer open to quarterly contract.",
	})
	if err != nil {
		t.Fatalf("create crm lead: %v", err)
	}
	opportunity, err := graph.crmCore.CreateOpportunity(actorID, map[string]any{
		"title":               "Quarterly catering contract",
		"party_id":            party.ID,
		"party_name":          textValue(party.Values["name"]),
		"contact_id":          contact.ID,
		"owner_user_id":       actorID,
		"source_lead_id":      lead.ID,
		"stage":               "proposal",
		"estimated_value":     24000000,
		"expected_close_date": now.AddDate(0, 1, 15).Format("2006-01-02"),
		"next_action_at":      now.Add(48 * time.Hour).Format(time.RFC3339),
		"notes":               "Proposal draft in review.",
	})
	if err != nil {
		t.Fatalf("create crm opportunity: %v", err)
	}
	if _, err := graph.crmCore.CreateActivity(actorID, map[string]any{
		"activity_type": "meeting",
		"subject":       "Review service recovery and catering proposal",
		"related_kind":  "opportunity",
		"related_id":    opportunity.ID,
		"party_id":      party.ID,
		"party_name":    textValue(party.Values["name"]),
		"owner_user_id": actorID,
		"status":        "open",
		"due_at":        now.Add(72 * time.Hour).Format(time.RFC3339),
		"note":          "Prepare both service recovery and upsell summary.",
	}); err != nil {
		t.Fatalf("create crm activity: %v", err)
	}

	board, err := graph.analytics.SaveDashboard(analytics.Dashboard{
		Name:        "CRM Operations Board " + suffix,
		Description: "Seeded CRM dashboard for ticketing, customer health, and sales pipeline.",
		Surface:     "dashboard",
		IsDefault:   true,
		Visibility:  "private",
		Status:      "active",
		RuntimeScope: analytics.RuntimeScope{
			ScopeType: "deployment",
		},
		Widgets: []analytics.DashboardWidget{
			{WidgetKey: "crm.ticketing.open_tickets", Title: "Open Tickets", Kind: "metric", Width: 3, Height: 1, Order: 1},
			{WidgetKey: "crm.ticketing.overdue_tickets", Title: "Overdue Tickets", Kind: "metric", Width: 3, Height: 1, Order: 2},
			{WidgetKey: "crm.ticketing.queue_backlog", Title: "Queue Backlog", Kind: "chart_bar", Width: 6, Height: 2, Order: 3},
			{WidgetKey: "crm.sales.pipeline_value", Title: "Pipeline Value", Kind: "metric", Width: 3, Height: 1, Order: 4},
			{WidgetKey: "crm.sales.pipeline_by_stage", Title: "Pipeline by Stage", Kind: "chart_bar", Width: 6, Height: 2, Order: 5},
			{WidgetKey: "crm.customers.at_risk", Title: "At-Risk Customers", Kind: "table", Width: 6, Height: 2, Order: 6},
		},
	})
	if err != nil {
		t.Fatalf("save crm dashboard: %v", err)
	}

	manifest := crmSeedManifest{
		SeededAt:       time.Now().UTC().Format(time.RFC3339),
		OrganizationID: "org_default",
		AgentPrompts: []string{
			"Summarize the CRM service backlog, customer health, and active pipeline for the seeded CRM demo customer. Show me the most relevant dashboard widgets too.",
			"Create a CRM service recovery plan for the seeded customer, then propose the best next sales opportunity action without executing anything.",
			"Open the CRM customer 360 context for the seeded customer and explain the link between the open ticket and the current opportunity.",
		},
	}
	manifest.Routes.Tickets = "/ui/crm/tickets"
	manifest.Routes.Queues = "/ui/crm/queues"
	manifest.Routes.Leads = "/ui/crm/leads"
	manifest.Routes.Opportunities = "/ui/crm/opportunities"
	manifest.Routes.Customer360 = "/ui/crm/customers/360?party_id=" + party.ID
	manifest.Routes.Dashboard = "/ui/dashboard"
	manifest.Routes.Agent = "/ui/agent/workspace"
	manifest.Queue.Code = textValue(queue.Values["code"])
	manifest.Queue.Name = textValue(queue.Values["name"])
	manifest.SLA.Code = textValue(sla.Values["code"])
	manifest.SLA.Name = textValue(sla.Values["name"])
	manifest.Customer.PartyID = party.ID
	manifest.Customer.Name = textValue(party.Values["name"])
	manifest.Customer.Tier = "gold"
	manifest.Ticket.ID = ticket.ID
	manifest.Ticket.TicketNumber = textValue(ticket.Values["ticket_number"])
	manifest.Ticket.Status = textValue(ticket.Values["status"])
	manifest.Lead.ID = lead.ID
	manifest.Lead.LeadNumber = textValue(lead.Values["lead_number"])
	manifest.Lead.Status = textValue(lead.Values["status"])
	manifest.Opportunity.ID = opportunity.ID
	manifest.Opportunity.OpportunityNumber = textValue(opportunity.Values["opportunity_number"])
	manifest.Opportunity.Stage = textValue(opportunity.Values["stage"])
	manifest.Opportunity.Value = floatValue(opportunity.Values["estimated_value"])
	manifest.Dashboard.BoardID = board.ID
	manifest.Dashboard.BoardName = board.Name
	manifest.Dashboard.Widgets = []string{
		"crm.ticketing.open_tickets",
		"crm.ticketing.overdue_tickets",
		"crm.ticketing.queue_backlog",
		"crm.sales.pipeline_value",
		"crm.sales.pipeline_by_stage",
		"crm.customers.at_risk",
	}

	body, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	manifestPath := filepath.Join(os.TempDir(), "orbyte-crm-seed.json")
	if err := os.WriteFile(manifestPath, body, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	t.Logf("wrote crm seed manifest to %s", manifestPath)
}

func floatValue(value any) float64 {
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
