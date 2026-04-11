package app

import (
	"orbyte/internal/platform/httpx"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/search"
)

var (
	crmCustomer360Permissions = []string{
		"crm_ticket.list",
		"crm_opportunity.list",
		"crm_activity.list",
		"party.read",
		"customer.read",
		"party_contact.read",
	}
	crmSalesSummaryPermissions = []string{
		"crm_opportunity.list",
		"crm_lead.list",
		"crm_activity.list",
	}
)

func crmCoreKernelPackManifests() []module.Manifest {
	return []module.Manifest{crmCoreKernelPackManifest()}
}

func crmCoreKernelPackManifest() module.Manifest {
	return module.Manifest{
		Key:                    "crm_core",
		Name:                   "CRM Core",
		NameI18n:               localize("CRM Core", "Inti CRM"),
		Version:                "1.0.0",
		DomainFamily:           "business",
		DependencyRequirements: requiredModuleDependencies("platform.core", "masterdata", "analytics"),
		AdminConsole: module.AdminConsoleDefinition{
			Title:           "CRM Console",
			TitleI18n:       localize("CRM Console", "Konsol CRM"),
			Description:     "Customer service desk, customer 360 review, and sales pipeline operations.",
			DescriptionI18n: localize("Customer service desk, customer 360 review, and sales pipeline operations.", "Service desk pelanggan, tinjauan customer 360, dan operasi pipeline penjualan."),
			Sections: []module.AdminConsoleSectionDefinition{
				{
					Key:       "crm_service_operations",
					Kind:      module.AdminConsoleSectionResourceLinks,
					Title:     "Service CRM",
					TitleI18n: localize("Service CRM", "CRM Layanan"),
					Links: []module.AdminConsoleLinkDefinition{
						adminConsoleLink("crm_tickets", "Issue Tickets", "Tiket Isu", "/ui/crm/tickets", "Review and update customer issue tickets.", "Tinjau dan perbarui tiket isu pelanggan.", "crm_ticket.list"),
						adminConsoleLink("crm_queues", "Ticket Queues", "Queue Tiket", "/ui/crm/queues", "Manage ticket queues and SLA defaults.", "Kelola queue tiket dan default SLA.", "crm_queue.list"),
						adminConsoleLink("crm_sla_policies", "SLA Policies", "Kebijakan SLA", "/ui/crm/sla-policies", "Configure ticket response and resolution targets.", "Konfigurasikan target respons dan resolusi tiket.", "crm_sla_policy.list"),
						adminConsoleLink("crm_assignment_rules", "Assignment Rules", "Aturan Penugasan", "/ui/crm/assignment-rules", "Configure ticket routing and assignment defaults.", "Konfigurasikan routing dan default penugasan tiket.", "crm_assignment_rule.list"),
					},
				},
				{
					Key:       "crm_sales_operations",
					Kind:      module.AdminConsoleSectionResourceLinks,
					Title:     "Sales CRM",
					TitleI18n: localize("Sales CRM", "CRM Penjualan"),
					Links: []module.AdminConsoleLinkDefinition{
						adminConsoleLink("crm_leads", "Leads", "Lead", "/ui/crm/leads", "Open internal lead tracking.", "Buka pelacakan lead internal.", "crm_lead.list"),
						adminConsoleLink("crm_opportunities", "Opportunities", "Peluang", "/ui/crm/opportunities", "Open the opportunity pipeline.", "Buka pipeline peluang.", "crm_opportunity.list"),
						adminConsoleLink("crm_activities", "CRM Activities", "Aktivitas CRM", "/ui/crm/activities", "Review follow-up activities across service and sales.", "Tinjau aktivitas tindak lanjut lintas layanan dan penjualan.", "crm_activity.list"),
						adminConsoleLink("crm_customer_360", "Customer 360", "Customer 360", "/ui/crm/customers/360", "Open the CRM customer 360 workspace.", "Buka workspace customer 360 CRM.", crmCustomer360Permissions[0]),
					},
				},
			},
		},
		Models: []model.Definition{
			crmQueueModelDefinition(),
			crmSLAPolicyModelDefinition(),
			crmAssignmentRuleModelDefinition(),
			crmTicketModelDefinition(),
			crmTicketCommentModelDefinition(),
			crmTicketActivityModelDefinition(),
			crmLeadModelDefinition(),
			crmOpportunityModelDefinition(),
			crmActivityModelDefinition(),
		},
		SearchIndexes: []search.IndexDefinition{
			commercialModelSearchIndex("crm.queues.search", "CRM Queue Search", "crm_queue", "crm.queues.list", []string{"code", "name", "status"}),
			commercialModelSearchIndex("crm.sla_policies.search", "CRM SLA Policy Search", "crm_sla_policy", "crm.sla_policies.list", []string{"code", "name", "queue_code", "status"}),
			commercialModelSearchIndex("crm.assignment_rules.search", "CRM Assignment Rule Search", "crm_assignment_rule", "crm.assignment_rules.list", []string{"code", "name", "queue_code", "status"}),
			commercialModelSearchIndex("crm.tickets.search", "CRM Ticket Search", "crm_ticket", "crm.tickets.list", []string{"ticket_number", "title", "party_name", "queue_code", "status", "priority"}),
			commercialModelSearchIndex("crm.ticket_comments.search", "CRM Ticket Comment Search", "crm_ticket_comment", "crm.ticket_comments.list", []string{"ticket_number", "body", "comment_type"}),
			commercialModelSearchIndex("crm.ticket_activity.search", "CRM Ticket Activity Search", "crm_ticket_activity", "crm.ticket_activity.list", []string{"ticket_number", "activity_type", "party_name"}),
			commercialModelSearchIndex("crm.leads.search", "CRM Lead Search", "crm_lead", "crm.leads.list", []string{"lead_number", "title", "party_name", "status", "rating"}),
			commercialModelSearchIndex("crm.opportunities.search", "CRM Opportunity Search", "crm_opportunity", "crm.opportunities.list", []string{"opportunity_number", "title", "party_name", "stage", "status"}),
			commercialModelSearchIndex("crm.activities.search", "CRM Activity Search", "crm_activity", "crm.activities.list", []string{"activity_number", "subject", "activity_type", "status"}),
		},
		Security: module.SecurityDefinition{
			Permissions: crmSecurityPermissions(),
			RoleTemplates: []module.RoleTemplateDefinition{
				{
					Key:           "crm_agent",
					Name:          "CRM Agent",
					NameI18n:      localize("CRM Agent", "Agen CRM"),
					AllowedScopes: []string{"location", "deployment"},
					PermissionKeys: []string{
						"crm_queue.list", "crm_queue.read",
						"crm_ticket.create", "crm_ticket.list", "crm_ticket.read", "crm_ticket.update",
						"crm_ticket_comment.create", "crm_ticket_comment.list", "crm_ticket_comment.read",
						"crm_ticket_activity.list", "crm_ticket_activity.read",
						"crm_activity.create", "crm_activity.list", "crm_activity.read", "crm_activity.update",
						"crm_lead.create", "crm_lead.list", "crm_lead.read", "crm_lead.update",
						"crm_opportunity.create", "crm_opportunity.list", "crm_opportunity.read", "crm_opportunity.update",
						"party.list", "party.read", "customer.list", "customer.read",
					},
				},
				{
					Key:           "crm_manager",
					Name:          "CRM Manager",
					NameI18n:      localize("CRM Manager", "Manajer CRM"),
					AllowedScopes: []string{"location", "deployment"},
					PermissionKeys: []string{
						"crm_queue.create", "crm_queue.list", "crm_queue.read", "crm_queue.update",
						"crm_sla_policy.create", "crm_sla_policy.list", "crm_sla_policy.read", "crm_sla_policy.update",
						"crm_assignment_rule.create", "crm_assignment_rule.list", "crm_assignment_rule.read", "crm_assignment_rule.update",
						"crm_ticket.create", "crm_ticket.list", "crm_ticket.read", "crm_ticket.update",
						"crm_ticket_comment.create", "crm_ticket_comment.list", "crm_ticket_comment.read",
						"crm_ticket_activity.list", "crm_ticket_activity.read",
						"crm_activity.create", "crm_activity.list", "crm_activity.read", "crm_activity.update",
						"crm_lead.create", "crm_lead.list", "crm_lead.read", "crm_lead.update",
						"crm_opportunity.create", "crm_opportunity.list", "crm_opportunity.read", "crm_opportunity.update",
						"party.list", "party.read", "customer.list", "customer.read",
					},
				},
			},
		},
		Frontend: module.FrontendDefinition{
			Menus: []module.MenuDefinition{
				{Key: "crm.tickets", Label: "Issue Tickets", LabelI18n: localize("Issue Tickets", "Tiket Isu"), ActionKey: "crm.tickets.list", Order: 86, RequiredPermissions: []string{"crm_ticket.list"}},
				{Key: "crm.queues", Label: "Ticket Queues", LabelI18n: localize("Ticket Queues", "Queue Tiket"), ActionKey: "crm.queues.list", Order: 87, RequiredPermissions: []string{"crm_queue.list"}},
				{Key: "crm.customer_360", Label: "Customer 360", LabelI18n: localize("Customer 360", "Customer 360"), ActionKey: "crm.customers.360", Order: 88, RequiredPermissions: crmCustomer360Permissions},
				{Key: "crm.leads", Label: "Leads", LabelI18n: localize("Leads", "Lead"), ActionKey: "crm.leads.list", Order: 89, RequiredPermissions: []string{"crm_lead.list"}},
				{Key: "crm.opportunities", Label: "Opportunities", LabelI18n: localize("Opportunities", "Peluang"), ActionKey: "crm.opportunities.list", Order: 90, RequiredPermissions: []string{"crm_opportunity.list"}},
			},
			Actions: append(
				append(
					append(
						append(
							append(
								append(
									append(
										crmResourceActions("queues", "Ticket Queues", "Ticket Queue Detail", "Ticket Queue Form", "crm_queue"),
										crmResourceActions("sla-policies", "SLA Policies", "SLA Policy Detail", "SLA Policy Form", "crm_sla_policy")...,
									),
									crmResourceActions("assignment-rules", "Assignment Rules", "Assignment Rule Detail", "Assignment Rule Form", "crm_assignment_rule")...,
								),
								crmResourceActions("tickets", "Issue Tickets", "Issue Ticket Detail", "Issue Ticket Form", "crm_ticket")...,
							),
							crmResourceActions("ticket-comments", "Ticket Comments", "Ticket Comment Detail", "Ticket Comment Form", "crm_ticket_comment")...,
						),
						crmResourceActions("ticket-activities", "Ticket Activities", "Ticket Activity Detail", "Ticket Activity Form", "crm_ticket_activity")...,
					),
					crmResourceActions("leads", "Leads", "Lead Detail", "Lead Form", "crm_lead")...,
				),
				append(
					crmResourceActions("opportunities", "Opportunities", "Opportunity Detail", "Opportunity Form", "crm_opportunity"),
					append(
						crmResourceActions("activities", "CRM Activities", "CRM Activity Detail", "CRM Activity Form", "crm_activity"),
						module.ActionDefinition{
							Key:                 "crm.customers.360",
							Label:               "Customer 360",
							LabelI18n:           localize("Customer 360", "Customer 360"),
							Kind:                "navigate",
							RoutePath:           "/crm/customers/360",
							CustomEntryKey:      "crm.customer_360",
							RenderMode:          module.RenderModeCustom,
							RequiredPermissions: crmCustomer360Permissions,
						},
					)...,
				)...,
			),
			Views: []module.ViewDefinition{
				crmQueueListView(), crmQueueDetailView(), crmQueueFormView(),
				crmSLAPolicyListView(), crmSLAPolicyDetailView(), crmSLAPolicyFormView(),
				crmAssignmentRuleListView(), crmAssignmentRuleDetailView(), crmAssignmentRuleFormView(),
				crmTicketListView(), crmTicketDetailView(), crmTicketFormView(),
				crmTicketCommentListView(), crmTicketCommentDetailView(), crmTicketCommentFormView(),
				crmTicketActivityListView(), crmTicketActivityDetailView(), crmTicketActivityFormView(),
				crmLeadListView(), crmLeadDetailView(), crmLeadFormView(),
				crmOpportunityListView(), crmOpportunityDetailView(), crmOpportunityFormView(),
				crmActivityListView(), crmActivityDetailView(), crmActivityFormView(),
			},
			CustomEntries: []module.CustomEntryDefinition{{
				Key:                 "crm.customer_360",
				Title:               "Customer 360",
				TitleI18n:           localize("Customer 360", "Customer 360"),
				RoutePath:           "/crm/customers/360",
				BundleKey:           "crm-customer-360",
				ComponentExport:     "render",
				RequiredPermissions: crmCustomer360Permissions,
			}},
			DashboardWidgets: crmDashboardWidgets(),
		},
		MCP: module.MCPDefinition{
			Tools: crmMCPTools(),
		},
		Bundles: []module.BundleDefinition{{
			Key:    "crm-customer-360",
			Script: httpx.CRMCustomer360Bundle(),
		}},
	}
}

func crmSecurityPermissions() []module.PermissionDefinition {
	keys := []struct {
		key   string
		label string
	}{
		{"crm_queue", "CRM Queue"},
		{"crm_sla_policy", "CRM SLA Policy"},
		{"crm_assignment_rule", "CRM Assignment Rule"},
		{"crm_ticket", "CRM Ticket"},
		{"crm_ticket_comment", "CRM Ticket Comment"},
		{"crm_ticket_activity", "CRM Ticket Activity"},
		{"crm_lead", "CRM Lead"},
		{"crm_opportunity", "CRM Opportunity"},
		{"crm_activity", "CRM Activity"},
	}
	items := make([]module.PermissionDefinition, 0)
	for _, item := range keys {
		items = append(items, commercialModelPermissions(item.key, item.label)...)
	}
	return items
}

func crmMCPTools() []module.MCPToolDefinition {
	return []module.MCPToolDefinition{
		{Key: "crm.ticket.summary", Title: "Get CRM Ticket Summary", TitleI18n: localize("Get CRM Ticket Summary", "Ambil Ringkasan Tiket CRM"), Description: "Summarize CRM service backlog, SLA risk, open-ticket counts, and queue health for issue operations.", DescriptionI18n: localize("Summarize CRM service backlog, SLA risk, open-ticket counts, and queue health for issue operations.", "Ringkas backlog layanan CRM, risiko SLA, jumlah tiket terbuka, dan kesehatan queue untuk operasi isu."), Operation: "crm.ticket.summary", RequiredPermissions: []string{"crm_ticket.list"}, Contract: module.MCPContractMetadata{BusinessDomains: []string{"crm", "service"}}},
		{Key: "crm.ticket.search", Title: "Search CRM Tickets", TitleI18n: localize("Search CRM Tickets", "Cari Tiket CRM"), Description: "Search CRM issue tickets by query, queue, backlog status, priority, assignee, and customer.", DescriptionI18n: localize("Search CRM issue tickets by query, queue, backlog status, priority, assignee, and customer.", "Cari tiket isu CRM berdasarkan kueri, queue, status backlog, prioritas, assignee, dan pelanggan."), Operation: "crm.ticket.search", RequiredPermissions: []string{"crm_ticket.list"}, InputSchema: crmSearchSchema("queue_code", "status", "priority", "party_id", "assignee_user_id"), Contract: module.MCPContractMetadata{BusinessDomains: []string{"crm", "service"}}},
		{Key: "crm.ticket.get", Title: "Get CRM Ticket", TitleI18n: localize("Get CRM Ticket", "Ambil Tiket CRM"), Description: "Get one CRM service issue ticket with its current operational state.", DescriptionI18n: localize("Get one CRM service issue ticket with its current operational state.", "Ambil satu tiket isu layanan CRM beserta kondisi operasionalnya saat ini."), Operation: "crm.ticket.get", RequiredPermissions: []string{"crm_ticket.read"}, InputSchema: crmIDSchema("ticket_id"), Contract: module.MCPContractMetadata{BusinessDomains: []string{"crm", "service"}}},
		{Key: "crm.ticket.create", Title: "Create CRM Ticket", TitleI18n: localize("Create CRM Ticket", "Buat Tiket CRM"), Description: "Create a new CRM issue ticket after explicit confirmation.", DescriptionI18n: localize("Create a new CRM issue ticket after explicit confirmation.", "Buat tiket isu CRM baru setelah konfirmasi eksplisit."), Operation: "crm.ticket.create", RequiredPermissions: []string{"crm_ticket.create"}, InputSchema: map[string]any{"type": "object", "properties": map[string]any{"title": map[string]any{"type": "string"}, "description": map[string]any{"type": "string"}, "party_id": map[string]any{"type": "string"}, "party_name": map[string]any{"type": "string"}, "queue_code": map[string]any{"type": "string"}, "priority": map[string]any{"type": "string"}, "severity": map[string]any{"type": "string"}, "source_channel": map[string]any{"type": "string"}, "assignee_user_id": map[string]any{"type": "string"}, "due_at": map[string]any{"type": "string"}, "first_response_due_at": map[string]any{"type": "string"}, "issue_category": map[string]any{"type": "string"}, "tags_json": map[string]any{"type": "string"}, "confirm_apply": map[string]any{"type": "boolean"}}, "required": []string{"title", "confirm_apply"}}, Contract: module.MCPContractMetadata{ActionClass: "create", RequiresConfirmation: true, BusinessDomains: []string{"crm", "service"}}},
		{Key: "crm.ticket.update", Title: "Update CRM Ticket", TitleI18n: localize("Update CRM Ticket", "Perbarui Tiket CRM"), Description: "Update an existing CRM ticket after explicit confirmation.", DescriptionI18n: localize("Update an existing CRM ticket after explicit confirmation.", "Perbarui tiket CRM yang ada setelah konfirmasi eksplisit."), Operation: "crm.ticket.update", RequiredPermissions: []string{"crm_ticket.update"}, InputSchema: map[string]any{"type": "object", "properties": map[string]any{"ticket_id": map[string]any{"type": "string"}, "title": map[string]any{"type": "string"}, "description": map[string]any{"type": "string"}, "queue_code": map[string]any{"type": "string"}, "status": map[string]any{"type": "string"}, "priority": map[string]any{"type": "string"}, "severity": map[string]any{"type": "string"}, "assignee_user_id": map[string]any{"type": "string"}, "first_response_at": map[string]any{"type": "string"}, "resolved_at": map[string]any{"type": "string"}, "resolution_notes": map[string]any{"type": "string"}, "issue_category": map[string]any{"type": "string"}, "tags_json": map[string]any{"type": "string"}, "expected_version": map[string]any{"type": "integer"}, "confirm_apply": map[string]any{"type": "boolean"}}, "required": []string{"ticket_id", "confirm_apply"}}, Contract: module.MCPContractMetadata{ActionClass: "update", RequiresConfirmation: true, BusinessDomains: []string{"crm", "service"}}},
		{Key: "crm.ticket.comment.create", Title: "Add CRM Ticket Comment", TitleI18n: localize("Add CRM Ticket Comment", "Tambah Komentar Tiket CRM"), Description: "Add a ticket comment or internal note after explicit confirmation.", DescriptionI18n: localize("Add a ticket comment or internal note after explicit confirmation.", "Tambah komentar tiket atau catatan internal setelah konfirmasi eksplisit."), Operation: "crm.ticket.comment.create", RequiredPermissions: []string{"crm_ticket_comment.create"}, InputSchema: map[string]any{"type": "object", "properties": map[string]any{"ticket_id": map[string]any{"type": "string"}, "body": map[string]any{"type": "string"}, "comment_type": map[string]any{"type": "string"}, "confirm_apply": map[string]any{"type": "boolean"}}, "required": []string{"ticket_id", "body", "confirm_apply"}}, Contract: module.MCPContractMetadata{ActionClass: "create", RequiresConfirmation: true, BusinessDomains: []string{"crm", "service"}}},
		{Key: "crm.ticket.assign", Title: "Assign CRM Ticket", TitleI18n: localize("Assign CRM Ticket", "Tugaskan Tiket CRM"), Description: "Assign or reassign a CRM ticket after explicit confirmation.", DescriptionI18n: localize("Assign or reassign a CRM ticket after explicit confirmation.", "Tugaskan atau ubah assignee tiket CRM setelah konfirmasi eksplisit."), Operation: "crm.ticket.assign", RequiredPermissions: []string{"crm_ticket.update"}, InputSchema: map[string]any{"type": "object", "properties": map[string]any{"ticket_id": map[string]any{"type": "string"}, "assignee_user_id": map[string]any{"type": "string"}, "note": map[string]any{"type": "string"}, "expected_version": map[string]any{"type": "integer"}, "confirm_apply": map[string]any{"type": "boolean"}}, "required": []string{"ticket_id", "assignee_user_id", "confirm_apply"}}, Contract: module.MCPContractMetadata{ActionClass: "update", RequiresConfirmation: true, BusinessDomains: []string{"crm", "service"}}},
		{Key: "crm.ticket.resolve", Title: "Resolve CRM Ticket", TitleI18n: localize("Resolve CRM Ticket", "Selesaikan Tiket CRM"), Description: "Resolve or close a CRM ticket after explicit confirmation.", DescriptionI18n: localize("Resolve or close a CRM ticket after explicit confirmation.", "Selesaikan atau tutup tiket CRM setelah konfirmasi eksplisit."), Operation: "crm.ticket.resolve", RequiredPermissions: []string{"crm_ticket.update"}, InputSchema: map[string]any{"type": "object", "properties": map[string]any{"ticket_id": map[string]any{"type": "string"}, "resolution_notes": map[string]any{"type": "string"}, "close": map[string]any{"type": "boolean"}, "expected_version": map[string]any{"type": "integer"}, "confirm_apply": map[string]any{"type": "boolean"}}, "required": []string{"ticket_id", "confirm_apply"}}, Contract: module.MCPContractMetadata{ActionClass: "update", RequiresConfirmation: true, BusinessDomains: []string{"crm", "service"}}},
		{Key: "crm.customer.summary", Title: "Get CRM Customer Summary", TitleI18n: localize("Get CRM Customer Summary", "Ambil Ringkasan Pelanggan CRM"), Description: "Load a CRM customer 360 summary with service tickets, customer health, sales opportunities, and profile data.", DescriptionI18n: localize("Load a CRM customer 360 summary with service tickets, customer health, sales opportunities, and profile data.", "Muat ringkasan customer 360 CRM dengan tiket layanan, kesehatan pelanggan, peluang penjualan, dan data profil."), Operation: "crm.customer.summary", RequiredPermissions: crmCustomer360Permissions, InputSchema: crmIDSchema("party_id"), Contract: module.MCPContractMetadata{BusinessDomains: []string{"crm", "service", "sales"}}},
		{Key: "crm.customer.timeline", Title: "Get CRM Customer Timeline", TitleI18n: localize("Get CRM Customer Timeline", "Ambil Timeline Pelanggan CRM"), Description: "Load CRM activities, service ticket history, and sales follow-up for one customer.", DescriptionI18n: localize("Load CRM activities, service ticket history, and sales follow-up for one customer.", "Muat aktivitas CRM, riwayat tiket layanan, dan tindak lanjut penjualan untuk satu pelanggan."), Operation: "crm.customer.timeline", RequiredPermissions: crmCustomer360Permissions, InputSchema: crmIDSchema("party_id"), Contract: module.MCPContractMetadata{BusinessDomains: []string{"crm", "service", "sales"}}},
		{Key: "crm.customer.health", Title: "Get CRM Customer Health", TitleI18n: localize("Get CRM Customer Health", "Ambil Kesehatan Pelanggan CRM"), Description: "Summarize customer health, at-risk accounts, service issues, and open opportunities.", DescriptionI18n: localize("Summarize customer health, at-risk accounts, service issues, and open opportunities.", "Ringkas kesehatan pelanggan, akun berisiko, isu layanan, dan peluang terbuka."), Operation: "crm.customer.health", RequiredPermissions: crmCustomer360Permissions, Contract: module.MCPContractMetadata{BusinessDomains: []string{"crm", "service", "sales"}}},
		{Key: "crm.lead.search", Title: "Search CRM Leads", TitleI18n: localize("Search CRM Leads", "Cari Lead CRM"), Description: "Search internal CRM sales leads by owner, customer, status, rating, and next step.", DescriptionI18n: localize("Search internal CRM sales leads by owner, customer, status, rating, and next step.", "Cari lead penjualan CRM internal berdasarkan owner, pelanggan, status, rating, dan langkah berikutnya."), Operation: "crm.lead.search", RequiredPermissions: []string{"crm_lead.list"}, InputSchema: crmSearchSchema("status", "rating", "party_id", "owner_user_id"), Contract: module.MCPContractMetadata{BusinessDomains: []string{"crm", "sales"}}},
		{Key: "crm.lead.get", Title: "Get CRM Lead", TitleI18n: localize("Get CRM Lead", "Ambil Lead CRM"), Description: "Get one CRM sales lead.", DescriptionI18n: localize("Get one CRM sales lead.", "Ambil satu lead penjualan CRM."), Operation: "crm.lead.get", RequiredPermissions: []string{"crm_lead.read"}, InputSchema: crmIDSchema("lead_id"), Contract: module.MCPContractMetadata{BusinessDomains: []string{"crm", "sales"}}},
		{Key: "crm.lead.create", Title: "Create CRM Lead", TitleI18n: localize("Create CRM Lead", "Buat Lead CRM"), Description: "Create a CRM lead after explicit confirmation.", DescriptionI18n: localize("Create a CRM lead after explicit confirmation.", "Buat lead CRM setelah konfirmasi eksplisit."), Operation: "crm.lead.create", RequiredPermissions: []string{"crm_lead.create"}, InputSchema: crmMutationSchema("title", "party_id", "party_name", "contact_id", "owner_user_id", "source_channel", "status", "rating", "estimated_value", "expected_close_date", "next_action_at", "notes"), Contract: module.MCPContractMetadata{ActionClass: "create", RequiresConfirmation: true, BusinessDomains: []string{"crm", "sales"}}},
		{Key: "crm.lead.update", Title: "Update CRM Lead", TitleI18n: localize("Update CRM Lead", "Perbarui Lead CRM"), Description: "Update a CRM lead after explicit confirmation.", DescriptionI18n: localize("Update a CRM lead after explicit confirmation.", "Perbarui lead CRM setelah konfirmasi eksplisit."), Operation: "crm.lead.update", RequiredPermissions: []string{"crm_lead.update"}, InputSchema: crmMutationSchema("lead_id", "title", "party_id", "party_name", "contact_id", "owner_user_id", "source_channel", "status", "rating", "estimated_value", "expected_close_date", "next_action_at", "notes", "expected_version"), Contract: module.MCPContractMetadata{ActionClass: "update", RequiresConfirmation: true, BusinessDomains: []string{"crm", "sales"}}},
		{Key: "crm.opportunity.search", Title: "Search CRM Opportunities", TitleI18n: localize("Search CRM Opportunities", "Cari Peluang CRM"), Description: "Search internal CRM sales opportunities by stage, status, customer, owner, and pipeline context.", DescriptionI18n: localize("Search internal CRM sales opportunities by stage, status, customer, owner, and pipeline context.", "Cari peluang penjualan CRM internal berdasarkan stage, status, pelanggan, owner, dan konteks pipeline."), Operation: "crm.opportunity.search", RequiredPermissions: []string{"crm_opportunity.list"}, InputSchema: crmSearchSchema("stage", "status", "party_id", "owner_user_id"), Contract: module.MCPContractMetadata{BusinessDomains: []string{"crm", "sales"}}},
		{Key: "crm.opportunity.get", Title: "Get CRM Opportunity", TitleI18n: localize("Get CRM Opportunity", "Ambil Peluang CRM"), Description: "Get one CRM sales opportunity.", DescriptionI18n: localize("Get one CRM sales opportunity.", "Ambil satu peluang penjualan CRM."), Operation: "crm.opportunity.get", RequiredPermissions: []string{"crm_opportunity.read"}, InputSchema: crmIDSchema("opportunity_id"), Contract: module.MCPContractMetadata{BusinessDomains: []string{"crm", "sales"}}},
		{Key: "crm.opportunity.create", Title: "Create CRM Opportunity", TitleI18n: localize("Create CRM Opportunity", "Buat Peluang CRM"), Description: "Create a CRM opportunity after explicit confirmation.", DescriptionI18n: localize("Create a CRM opportunity after explicit confirmation.", "Buat peluang CRM setelah konfirmasi eksplisit."), Operation: "crm.opportunity.create", RequiredPermissions: []string{"crm_opportunity.create"}, InputSchema: crmMutationSchema("title", "party_id", "party_name", "contact_id", "owner_user_id", "source_lead_id", "stage", "estimated_value", "expected_close_date", "next_action_at", "loss_reason", "notes"), Contract: module.MCPContractMetadata{ActionClass: "create", RequiresConfirmation: true, BusinessDomains: []string{"crm", "sales"}}},
		{Key: "crm.opportunity.update", Title: "Update CRM Opportunity", TitleI18n: localize("Update CRM Opportunity", "Perbarui Peluang CRM"), Description: "Update a CRM opportunity after explicit confirmation.", DescriptionI18n: localize("Update a CRM opportunity after explicit confirmation.", "Perbarui peluang CRM setelah konfirmasi eksplisit."), Operation: "crm.opportunity.update", RequiredPermissions: []string{"crm_opportunity.update"}, InputSchema: crmMutationSchema("opportunity_id", "title", "party_id", "party_name", "contact_id", "owner_user_id", "source_lead_id", "stage", "estimated_value", "expected_close_date", "next_action_at", "loss_reason", "notes", "expected_version"), Contract: module.MCPContractMetadata{ActionClass: "update", RequiresConfirmation: true, BusinessDomains: []string{"crm", "sales"}}},
		{Key: "crm.opportunity.pipeline.summary", Title: "Get Opportunity Pipeline Summary", TitleI18n: localize("Get Opportunity Pipeline Summary", "Ambil Ringkasan Pipeline Peluang"), Description: "Summarize the active CRM sales pipeline by stage, value, stale opportunities, and activity coverage.", DescriptionI18n: localize("Summarize the active CRM sales pipeline by stage, value, stale opportunities, and activity coverage.", "Ringkas pipeline penjualan CRM aktif berdasarkan stage, nilai, peluang stale, dan cakupan aktivitas."), Operation: "crm.opportunity.pipeline.summary", RequiredPermissions: crmSalesSummaryPermissions, Contract: module.MCPContractMetadata{BusinessDomains: []string{"crm", "sales"}}},
	}
}

func crmSearchSchema(extraKeys ...string) map[string]any {
	properties := map[string]any{
		"query":     map[string]any{"type": "string"},
		"page":      map[string]any{"type": "integer"},
		"page_size": map[string]any{"type": "integer"},
	}
	for _, key := range extraKeys {
		properties[key] = map[string]any{"type": "string"}
	}
	return map[string]any{"type": "object", "properties": properties}
}

func crmIDSchema(idKey string) map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{idKey: map[string]any{"type": "string"}}, "required": []string{idKey}}
}

func crmMutationSchema(keys ...string) map[string]any {
	properties := map[string]any{"confirm_apply": map[string]any{"type": "boolean"}}
	required := []string{"confirm_apply"}
	for _, key := range keys {
		properties[key] = map[string]any{"type": "string"}
	}
	if _, ok := properties["expected_version"]; ok {
		properties["expected_version"] = map[string]any{"type": "integer"}
	}
	return map[string]any{"type": "object", "properties": properties, "required": required}
}

func crmDashboardWidgets() []module.DashboardWidgetDefinition {
	return []module.DashboardWidgetDefinition{
		{Key: "crm.ticketing.open_tickets", Title: "Open Tickets", TitleI18n: localize("Open Tickets", "Tiket Terbuka"), Surface: module.UISurfaceDashboard, RendererKind: "metric", RefreshPolicy: "minutes", DataPath: "/ui/data/crm/service/summary", RequiredPermissions: []string{"crm_ticket.list"}, DefaultWidth: 3, DefaultHeight: 1, Metric: &module.DashboardMetricSpec{ValuePath: "overview.open_tickets", Format: "number"}},
		{Key: "crm.ticketing.overdue_tickets", Title: "Overdue Tickets", TitleI18n: localize("Overdue Tickets", "Tiket Lewat SLA"), Surface: module.UISurfaceDashboard, RendererKind: "metric", RefreshPolicy: "minutes", DataPath: "/ui/data/crm/service/summary", RequiredPermissions: []string{"crm_ticket.list"}, DefaultWidth: 3, DefaultHeight: 1, Metric: &module.DashboardMetricSpec{ValuePath: "overview.overdue_tickets", Format: "number"}},
		{Key: "crm.ticketing.first_response_hours", Title: "First Response Hours", TitleI18n: localize("First Response Hours", "Jam Respons Pertama"), Surface: module.UISurfaceDashboard, RendererKind: "metric", RefreshPolicy: "minutes", DataPath: "/ui/data/crm/service/summary", RequiredPermissions: []string{"crm_ticket.list"}, DefaultWidth: 3, DefaultHeight: 1, Metric: &module.DashboardMetricSpec{ValuePath: "overview.first_response_hours", Format: "number"}},
		{Key: "crm.ticketing.queue_backlog", Title: "Queue Backlog", TitleI18n: localize("Queue Backlog", "Backlog Queue"), Surface: module.UISurfaceDashboard, RendererKind: "chart_bar", RefreshPolicy: "minutes", DataPath: "/ui/data/crm/service/summary", RequiredPermissions: []string{"crm_ticket.list"}, DefaultWidth: 6, DefaultHeight: 2, Chart: &module.DashboardChartSpec{SeriesPath: "charts.queues", Category: "queue_code", Value: "open_tickets", Format: "number"}},
		{Key: "crm.ticketing.channel_mix", Title: "Ticket Channel Mix", TitleI18n: localize("Ticket Channel Mix", "Komposisi Kanal Tiket"), Surface: module.UISurfaceDashboard, RendererKind: "chart_bar", RefreshPolicy: "minutes", DataPath: "/ui/data/crm/service/summary", RequiredPermissions: []string{"crm_ticket.list"}, DefaultWidth: 6, DefaultHeight: 2, Chart: &module.DashboardChartSpec{SeriesPath: "charts.channels", Category: "source_channel", Value: "count", Format: "number"}},
		{Key: "crm.ticketing.trend", Title: "Ticket Flow", TitleI18n: localize("Ticket Flow", "Arus Tiket"), Surface: module.UISurfaceDashboard, RendererKind: "chart_line", RefreshPolicy: "minutes", DataPath: "/ui/data/crm/service/summary", RequiredPermissions: []string{"crm_ticket.list"}, DefaultWidth: 6, DefaultHeight: 2, Chart: &module.DashboardChartSpec{SeriesPath: "trends.tickets", Category: "label", Value: "created", Format: "number"}},
		{Key: "crm.sales.pipeline_value", Title: "Pipeline Value", TitleI18n: localize("Pipeline Value", "Nilai Pipeline"), Surface: module.UISurfaceDashboard, RendererKind: "metric", RefreshPolicy: "minutes", DataPath: "/ui/data/crm/sales/summary", RequiredPermissions: crmSalesSummaryPermissions, DefaultWidth: 3, DefaultHeight: 1, Metric: &module.DashboardMetricSpec{ValuePath: "overview.pipeline_value", Format: "currency"}},
		{Key: "crm.sales.open_opportunities", Title: "Open Opportunities", TitleI18n: localize("Open Opportunities", "Peluang Terbuka"), Surface: module.UISurfaceDashboard, RendererKind: "metric", RefreshPolicy: "minutes", DataPath: "/ui/data/crm/sales/summary", RequiredPermissions: crmSalesSummaryPermissions, DefaultWidth: 3, DefaultHeight: 1, Metric: &module.DashboardMetricSpec{ValuePath: "overview.open_opportunities", Format: "number"}},
		{Key: "crm.sales.stale_opportunities", Title: "Stale Opportunities", TitleI18n: localize("Stale Opportunities", "Peluang Stale"), Surface: module.UISurfaceDashboard, RendererKind: "metric", RefreshPolicy: "minutes", DataPath: "/ui/data/crm/sales/summary", RequiredPermissions: crmSalesSummaryPermissions, DefaultWidth: 3, DefaultHeight: 1, Metric: &module.DashboardMetricSpec{ValuePath: "overview.stale_opportunities", Format: "number"}},
		{Key: "crm.sales.pipeline_by_stage", Title: "Pipeline By Stage", TitleI18n: localize("Pipeline By Stage", "Pipeline per Stage"), Surface: module.UISurfaceDashboard, RendererKind: "chart_bar", RefreshPolicy: "minutes", DataPath: "/ui/data/crm/sales/summary", RequiredPermissions: crmSalesSummaryPermissions, DefaultWidth: 6, DefaultHeight: 2, Chart: &module.DashboardChartSpec{SeriesPath: "charts.pipeline_stages", Category: "stage", Value: "pipeline_value", Format: "currency"}},
		{Key: "crm.sales.pipeline_trend", Title: "Pipeline Trend", TitleI18n: localize("Pipeline Trend", "Tren Pipeline"), Surface: module.UISurfaceDashboard, RendererKind: "chart_line", RefreshPolicy: "minutes", DataPath: "/ui/data/crm/sales/summary", RequiredPermissions: crmSalesSummaryPermissions, DefaultWidth: 6, DefaultHeight: 2, Chart: &module.DashboardChartSpec{SeriesPath: "trends.pipeline", Category: "label", Value: "open", Format: "currency"}},
		{Key: "crm.customers.at_risk", Title: "At-Risk Customers", TitleI18n: localize("At-Risk Customers", "Pelanggan Berisiko"), Surface: module.UISurfaceDashboard, RendererKind: "table", RefreshPolicy: "minutes", DataPath: "/ui/data/crm/customers/health", RequiredPermissions: crmCustomer360Permissions, DefaultWidth: 6, DefaultHeight: 2, Table: &module.DashboardTableSpec{RowsPath: "items", Columns: []module.ColumnDefinition{{Key: "party_name", Label: "Customer", LabelI18n: localize("Customer", "Pelanggan"), Path: "party_name"}, {Key: "open_tickets", Label: "Open Tickets", LabelI18n: localize("Open Tickets", "Tiket Terbuka"), Path: "open_tickets"}, {Key: "overdue_tickets", Label: "Overdue", LabelI18n: localize("Overdue", "Lewat SLA"), Path: "overdue_tickets"}}}},
	}
}

func crmQueueModelDefinition() model.Definition {
	return model.Definition{
		Key:                 "crm_queue",
		DisplayName:         "CRM Queue",
		DisplayNameI18n:     localize("CRM Queue", "Queue CRM"),
		OwnerModuleKey:      "crm_core",
		Version:             "v1",
		CreatePermissionKey: "crm_queue.create",
		ListPermissionKey:   "crm_queue.list",
		ReadPermissionKey:   "crm_queue.read",
		UpdatePermissionKey: "crm_queue.update",
		DefaultSort:         "code",
		Fields: []model.FieldDefinition{
			{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Type: "string", Required: true},
			{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Type: "string", Required: true},
			{Key: "description", Label: "Description", LabelI18n: localize("Description", "Deskripsi"), Type: "string"},
			{Key: "manager_user_id", Label: "Manager User", LabelI18n: localize("Manager User", "User Manajer"), Type: "string"},
			{Key: "triage_sla_hours", Label: "Triage SLA Hours", LabelI18n: localize("Triage SLA Hours", "Jam SLA Triase"), Type: "number", DefaultValue: 4},
			{Key: "resolution_sla_hours", Label: "Resolution SLA Hours", LabelI18n: localize("Resolution SLA Hours", "Jam SLA Resolusi"), Type: "number", DefaultValue: 24},
			{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string", DefaultValue: "active", AllowedValues: []string{"active", "inactive"}},
		},
	}
}

func crmSLAPolicyModelDefinition() model.Definition {
	return model.Definition{
		Key:                 "crm_sla_policy",
		DisplayName:         "CRM SLA Policy",
		DisplayNameI18n:     localize("CRM SLA Policy", "Kebijakan SLA CRM"),
		OwnerModuleKey:      "crm_core",
		Version:             "v1",
		CreatePermissionKey: "crm_sla_policy.create",
		ListPermissionKey:   "crm_sla_policy.list",
		ReadPermissionKey:   "crm_sla_policy.read",
		UpdatePermissionKey: "crm_sla_policy.update",
		DefaultSort:         "code",
		Fields: []model.FieldDefinition{
			{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Type: "string", Required: true},
			{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Type: "string", Required: true},
			{Key: "queue_code", Label: "Queue", LabelI18n: localize("Queue", "Queue"), Type: "string", Reference: &model.ReferenceDefinition{ModelKey: "crm_queue", LookupField: "values.code"}},
			{Key: "source_channel", Label: "Source Channel", LabelI18n: localize("Source Channel", "Kanal Sumber"), Type: "string", AllowedValues: []string{"", "web", "email", "phone", "chat", "pos"}},
			{Key: "priority", Label: "Priority", LabelI18n: localize("Priority", "Prioritas"), Type: "string", AllowedValues: []string{"", "low", "medium", "high", "urgent"}},
			{Key: "severity", Label: "Severity", LabelI18n: localize("Severity", "Severity"), Type: "string", AllowedValues: []string{"", "low", "medium", "high", "critical"}},
			{Key: "first_response_hours", Label: "First Response Hours", LabelI18n: localize("First Response Hours", "Jam Respons Pertama"), Type: "number", DefaultValue: 2},
			{Key: "resolution_hours", Label: "Resolution Hours", LabelI18n: localize("Resolution Hours", "Jam Resolusi"), Type: "number", DefaultValue: 24},
			{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string", DefaultValue: "active", AllowedValues: []string{"active", "inactive"}},
		},
	}
}

func crmAssignmentRuleModelDefinition() model.Definition {
	return model.Definition{
		Key:                 "crm_assignment_rule",
		DisplayName:         "CRM Assignment Rule",
		DisplayNameI18n:     localize("CRM Assignment Rule", "Aturan Penugasan CRM"),
		OwnerModuleKey:      "crm_core",
		Version:             "v1",
		CreatePermissionKey: "crm_assignment_rule.create",
		ListPermissionKey:   "crm_assignment_rule.list",
		ReadPermissionKey:   "crm_assignment_rule.read",
		UpdatePermissionKey: "crm_assignment_rule.update",
		DefaultSort:         "code",
		Fields: []model.FieldDefinition{
			{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Type: "string", Required: true},
			{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Type: "string", Required: true},
			{Key: "queue_code", Label: "Queue", LabelI18n: localize("Queue", "Queue"), Type: "string", Reference: &model.ReferenceDefinition{ModelKey: "crm_queue", LookupField: "values.code"}},
			{Key: "assign_queue_code", Label: "Assign Queue", LabelI18n: localize("Assign Queue", "Queue Tujuan"), Type: "string", Reference: &model.ReferenceDefinition{ModelKey: "crm_queue", LookupField: "values.code"}},
			{Key: "assign_user_id", Label: "Assign User", LabelI18n: localize("Assign User", "User Tujuan"), Type: "string"},
			{Key: "source_channel", Label: "Source Channel", LabelI18n: localize("Source Channel", "Kanal Sumber"), Type: "string"},
			{Key: "issue_category", Label: "Issue Category", LabelI18n: localize("Issue Category", "Kategori Isu"), Type: "string"},
			{Key: "priority", Label: "Priority", LabelI18n: localize("Priority", "Prioritas"), Type: "string", AllowedValues: []string{"", "low", "medium", "high", "urgent"}},
			{Key: "severity", Label: "Severity", LabelI18n: localize("Severity", "Severity"), Type: "string", AllowedValues: []string{"", "low", "medium", "high", "critical"}},
			{Key: "rank", Label: "Rank", LabelI18n: localize("Rank", "Peringkat"), Type: "number", DefaultValue: 100},
			{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string", DefaultValue: "active", AllowedValues: []string{"active", "inactive"}},
		},
	}
}

func crmTicketModelDefinition() model.Definition {
	return model.Definition{
		Key:                 "crm_ticket",
		DisplayName:         "CRM Ticket",
		DisplayNameI18n:     localize("CRM Ticket", "Tiket CRM"),
		OwnerModuleKey:      "crm_core",
		Version:             "v1",
		CreatePermissionKey: "crm_ticket.create",
		ListPermissionKey:   "crm_ticket.list",
		ReadPermissionKey:   "crm_ticket.read",
		UpdatePermissionKey: "crm_ticket.update",
		DefaultSort:         "ticket_number",
		Fields: []model.FieldDefinition{
			{Key: "ticket_number", Label: "Ticket Number", LabelI18n: localize("Ticket Number", "Nomor Tiket"), Type: "string", DefaultRuleKey: "crm.ticket_number.default"},
			{Key: "title", Label: "Title", LabelI18n: localize("Title", "Judul"), Type: "string", Required: true},
			{Key: "description", Label: "Description", LabelI18n: localize("Description", "Deskripsi"), Type: "string"},
			{Key: "party_id", Label: "Customer", LabelI18n: localize("Customer", "Pelanggan"), Type: "string", Reference: &model.ReferenceDefinition{ModelKey: "party"}},
			{Key: "party_name", Label: "Customer Name", LabelI18n: localize("Customer Name", "Nama Pelanggan"), Type: "string"},
			{Key: "queue_code", Label: "Queue", LabelI18n: localize("Queue", "Queue"), Type: "string", Reference: &model.ReferenceDefinition{ModelKey: "crm_queue", LookupField: "values.code"}},
			{Key: "source_channel", Label: "Source Channel", LabelI18n: localize("Source Channel", "Kanal Sumber"), Type: "string", DefaultValue: "web", AllowedValues: []string{"web", "email", "phone", "chat", "pos"}},
			{Key: "priority", Label: "Priority", LabelI18n: localize("Priority", "Prioritas"), Type: "string", DefaultValue: "medium", AllowedValues: []string{"low", "medium", "high", "urgent"}},
			{Key: "severity", Label: "Severity", LabelI18n: localize("Severity", "Severity"), Type: "string", DefaultValue: "medium", AllowedValues: []string{"low", "medium", "high", "critical"}},
			{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string", DefaultValue: "new", AllowedValues: []string{"new", "open", "pending_customer", "pending_internal", "resolved", "closed", "cancelled"}},
			{Key: "assignee_user_id", Label: "Assignee User", LabelI18n: localize("Assignee User", "User Assignee"), Type: "string"},
			{Key: "opened_at", Label: "Opened At", LabelI18n: localize("Opened At", "Dibuka Pada"), Type: "string"},
			{Key: "first_response_due_at", Label: "First Response Due At", LabelI18n: localize("First Response Due At", "Batas Respons Pertama"), Type: "string"},
			{Key: "first_response_at", Label: "First Response At", LabelI18n: localize("First Response At", "Respons Pertama Pada"), Type: "string"},
			{Key: "due_at", Label: "Due At", LabelI18n: localize("Due At", "Batas Waktu"), Type: "string"},
			{Key: "resolved_at", Label: "Resolved At", LabelI18n: localize("Resolved At", "Selesai Pada"), Type: "string"},
			{Key: "issue_category", Label: "Issue Category", LabelI18n: localize("Issue Category", "Kategori Isu"), Type: "string"},
			{Key: "resolution_notes", Label: "Resolution Notes", LabelI18n: localize("Resolution Notes", "Catatan Resolusi"), Type: "string"},
			{Key: "tags_json", Label: "Tags JSON", LabelI18n: localize("Tags JSON", "JSON Tag"), Type: "string"},
		},
		Relations: []model.RelationDefinition{
			{Key: "comments", Type: "has_many", TargetModelKey: "crm_ticket_comment", ForeignKey: "ticket_id"},
			{Key: "activities", Type: "has_many", TargetModelKey: "crm_ticket_activity", ForeignKey: "ticket_id"},
		},
	}
}

func crmTicketCommentModelDefinition() model.Definition {
	return model.Definition{
		Key:                 "crm_ticket_comment",
		DisplayName:         "CRM Ticket Comment",
		DisplayNameI18n:     localize("CRM Ticket Comment", "Komentar Tiket CRM"),
		OwnerModuleKey:      "crm_core",
		Version:             "v1",
		CreatePermissionKey: "crm_ticket_comment.create",
		ListPermissionKey:   "crm_ticket_comment.list",
		ReadPermissionKey:   "crm_ticket_comment.read",
		UpdatePermissionKey: "crm_ticket_comment.update",
		DefaultSort:         "created_at",
		Fields: []model.FieldDefinition{
			{Key: "ticket_id", Label: "Ticket", LabelI18n: localize("Ticket", "Tiket"), Type: "string", Required: true, Reference: &model.ReferenceDefinition{ModelKey: "crm_ticket"}},
			{Key: "ticket_number", Label: "Ticket Number", LabelI18n: localize("Ticket Number", "Nomor Tiket"), Type: "string"},
			{Key: "comment_type", Label: "Comment Type", LabelI18n: localize("Comment Type", "Tipe Komentar"), Type: "string", DefaultValue: "internal_note", AllowedValues: []string{"internal_note", "public_reply", "status_update"}},
			{Key: "body", Label: "Body", LabelI18n: localize("Body", "Isi"), Type: "string", Required: true},
			{Key: "author_user_id", Label: "Author User", LabelI18n: localize("Author User", "User Penulis"), Type: "string"},
			{Key: "created_at", Label: "Created At", LabelI18n: localize("Created At", "Dibuat Pada"), Type: "string"},
			{Key: "party_id", Label: "Customer", LabelI18n: localize("Customer", "Pelanggan"), Type: "string", Reference: &model.ReferenceDefinition{ModelKey: "party"}},
			{Key: "party_name", Label: "Customer Name", LabelI18n: localize("Customer Name", "Nama Pelanggan"), Type: "string"},
		},
	}
}

func crmTicketActivityModelDefinition() model.Definition {
	return model.Definition{
		Key:                 "crm_ticket_activity",
		DisplayName:         "CRM Ticket Activity",
		DisplayNameI18n:     localize("CRM Ticket Activity", "Aktivitas Tiket CRM"),
		OwnerModuleKey:      "crm_core",
		Version:             "v1",
		CreatePermissionKey: "crm_ticket_activity.create",
		ListPermissionKey:   "crm_ticket_activity.list",
		ReadPermissionKey:   "crm_ticket_activity.read",
		UpdatePermissionKey: "crm_ticket_activity.update",
		DefaultSort:         "occurred_at",
		Fields: []model.FieldDefinition{
			{Key: "ticket_id", Label: "Ticket", LabelI18n: localize("Ticket", "Tiket"), Type: "string", Required: true, Reference: &model.ReferenceDefinition{ModelKey: "crm_ticket"}},
			{Key: "ticket_number", Label: "Ticket Number", LabelI18n: localize("Ticket Number", "Nomor Tiket"), Type: "string"},
			{Key: "activity_type", Label: "Activity Type", LabelI18n: localize("Activity Type", "Tipe Aktivitas"), Type: "string", Required: true},
			{Key: "actor_user_id", Label: "Actor User", LabelI18n: localize("Actor User", "User Aktor"), Type: "string"},
			{Key: "assignee_user_id", Label: "Assignee User", LabelI18n: localize("Assignee User", "User Assignee"), Type: "string"},
			{Key: "queue_code", Label: "Queue", LabelI18n: localize("Queue", "Queue"), Type: "string"},
			{Key: "from_status", Label: "From Status", LabelI18n: localize("From Status", "Dari Status"), Type: "string"},
			{Key: "to_status", Label: "To Status", LabelI18n: localize("To Status", "Ke Status"), Type: "string"},
			{Key: "occurred_at", Label: "Occurred At", LabelI18n: localize("Occurred At", "Terjadi Pada"), Type: "string"},
			{Key: "note", Label: "Note", LabelI18n: localize("Note", "Catatan"), Type: "string"},
			{Key: "party_id", Label: "Customer", LabelI18n: localize("Customer", "Pelanggan"), Type: "string", Reference: &model.ReferenceDefinition{ModelKey: "party"}},
			{Key: "party_name", Label: "Customer Name", LabelI18n: localize("Customer Name", "Nama Pelanggan"), Type: "string"},
			{Key: "severity", Label: "Severity", LabelI18n: localize("Severity", "Severity"), Type: "string"},
			{Key: "priority", Label: "Priority", LabelI18n: localize("Priority", "Prioritas"), Type: "string"},
			{Key: "sla_breach_risk", Label: "SLA Risk", LabelI18n: localize("SLA Risk", "Risiko SLA"), Type: "string"},
			{Key: "source_channel", Label: "Source Channel", LabelI18n: localize("Source Channel", "Kanal Sumber"), Type: "string"},
			{Key: "issue_category", Label: "Issue Category", LabelI18n: localize("Issue Category", "Kategori Isu"), Type: "string"},
			{Key: "ticket_status_key", Label: "Ticket Status Key", LabelI18n: localize("Ticket Status Key", "Kunci Status Tiket"), Type: "string"},
		},
	}
}

func crmLeadModelDefinition() model.Definition {
	return model.Definition{
		Key:                 "crm_lead",
		DisplayName:         "CRM Lead",
		DisplayNameI18n:     localize("CRM Lead", "Lead CRM"),
		OwnerModuleKey:      "crm_core",
		Version:             "v1",
		CreatePermissionKey: "crm_lead.create",
		ListPermissionKey:   "crm_lead.list",
		ReadPermissionKey:   "crm_lead.read",
		UpdatePermissionKey: "crm_lead.update",
		DefaultSort:         "lead_number",
		Fields: []model.FieldDefinition{
			{Key: "lead_number", Label: "Lead Number", LabelI18n: localize("Lead Number", "Nomor Lead"), Type: "string", DefaultRuleKey: "crm.lead_number.default"},
			{Key: "title", Label: "Title", LabelI18n: localize("Title", "Judul"), Type: "string", Required: true},
			{Key: "party_id", Label: "Customer", LabelI18n: localize("Customer", "Pelanggan"), Type: "string", Reference: &model.ReferenceDefinition{ModelKey: "party"}},
			{Key: "party_name", Label: "Customer Name", LabelI18n: localize("Customer Name", "Nama Pelanggan"), Type: "string"},
			{Key: "contact_id", Label: "Contact", LabelI18n: localize("Contact", "Kontak"), Type: "string", Reference: &model.ReferenceDefinition{ModelKey: "party_contact"}},
			{Key: "owner_user_id", Label: "Owner User", LabelI18n: localize("Owner User", "User Pemilik"), Type: "string"},
			{Key: "source_channel", Label: "Source Channel", LabelI18n: localize("Source Channel", "Kanal Sumber"), Type: "string", AllowedValues: []string{"web", "email", "phone", "chat", "partner", "referral", "walk_in"}},
			{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string", DefaultValue: "new", AllowedValues: []string{"new", "contacted", "qualified", "disqualified", "converted", "closed"}},
			{Key: "rating", Label: "Rating", LabelI18n: localize("Rating", "Rating"), Type: "string", DefaultValue: "warm", AllowedValues: []string{"cold", "warm", "hot"}},
			{Key: "estimated_value", Label: "Estimated Value", LabelI18n: localize("Estimated Value", "Estimasi Nilai"), Type: "number"},
			{Key: "expected_close_date", Label: "Expected Close Date", LabelI18n: localize("Expected Close Date", "Estimasi Tanggal Tutup"), Type: "string"},
			{Key: "next_action_at", Label: "Next Action At", LabelI18n: localize("Next Action At", "Aksi Berikutnya Pada"), Type: "string"},
			{Key: "notes", Label: "Notes", LabelI18n: localize("Notes", "Catatan"), Type: "string"},
		},
	}
}

func crmOpportunityModelDefinition() model.Definition {
	return model.Definition{
		Key:                 "crm_opportunity",
		DisplayName:         "CRM Opportunity",
		DisplayNameI18n:     localize("CRM Opportunity", "Peluang CRM"),
		OwnerModuleKey:      "crm_core",
		Version:             "v1",
		CreatePermissionKey: "crm_opportunity.create",
		ListPermissionKey:   "crm_opportunity.list",
		ReadPermissionKey:   "crm_opportunity.read",
		UpdatePermissionKey: "crm_opportunity.update",
		DefaultSort:         "opportunity_number",
		Fields: []model.FieldDefinition{
			{Key: "opportunity_number", Label: "Opportunity Number", LabelI18n: localize("Opportunity Number", "Nomor Peluang"), Type: "string", DefaultRuleKey: "crm.opportunity_number.default"},
			{Key: "title", Label: "Title", LabelI18n: localize("Title", "Judul"), Type: "string", Required: true},
			{Key: "party_id", Label: "Customer", LabelI18n: localize("Customer", "Pelanggan"), Type: "string", Reference: &model.ReferenceDefinition{ModelKey: "party"}},
			{Key: "party_name", Label: "Customer Name", LabelI18n: localize("Customer Name", "Nama Pelanggan"), Type: "string"},
			{Key: "contact_id", Label: "Contact", LabelI18n: localize("Contact", "Kontak"), Type: "string", Reference: &model.ReferenceDefinition{ModelKey: "party_contact"}},
			{Key: "owner_user_id", Label: "Owner User", LabelI18n: localize("Owner User", "User Pemilik"), Type: "string"},
			{Key: "source_lead_id", Label: "Source Lead", LabelI18n: localize("Source Lead", "Lead Sumber"), Type: "string", Reference: &model.ReferenceDefinition{ModelKey: "crm_lead"}},
			{Key: "stage", Label: "Stage", LabelI18n: localize("Stage", "Stage"), Type: "string", DefaultValue: "new", AllowedValues: []string{"new", "qualified", "proposal", "negotiation", "won", "lost"}},
			{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string", DefaultValue: "open", AllowedValues: []string{"open", "won", "lost"}},
			{Key: "estimated_value", Label: "Estimated Value", LabelI18n: localize("Estimated Value", "Estimasi Nilai"), Type: "number"},
			{Key: "expected_close_date", Label: "Expected Close Date", LabelI18n: localize("Expected Close Date", "Estimasi Tanggal Tutup"), Type: "string"},
			{Key: "next_action_at", Label: "Next Action At", LabelI18n: localize("Next Action At", "Aksi Berikutnya Pada"), Type: "string"},
			{Key: "loss_reason", Label: "Loss Reason", LabelI18n: localize("Loss Reason", "Alasan Kalah"), Type: "string"},
			{Key: "notes", Label: "Notes", LabelI18n: localize("Notes", "Catatan"), Type: "string"},
		},
	}
}

func crmActivityModelDefinition() model.Definition {
	return model.Definition{
		Key:                 "crm_activity",
		DisplayName:         "CRM Activity",
		DisplayNameI18n:     localize("CRM Activity", "Aktivitas CRM"),
		OwnerModuleKey:      "crm_core",
		Version:             "v1",
		CreatePermissionKey: "crm_activity.create",
		ListPermissionKey:   "crm_activity.list",
		ReadPermissionKey:   "crm_activity.read",
		UpdatePermissionKey: "crm_activity.update",
		DefaultSort:         "activity_number",
		Fields: []model.FieldDefinition{
			{Key: "activity_number", Label: "Activity Number", LabelI18n: localize("Activity Number", "Nomor Aktivitas"), Type: "string", DefaultRuleKey: "crm.activity_number.default"},
			{Key: "activity_type", Label: "Activity Type", LabelI18n: localize("Activity Type", "Tipe Aktivitas"), Type: "string", Required: true},
			{Key: "subject", Label: "Subject", LabelI18n: localize("Subject", "Subjek"), Type: "string", Required: true},
			{Key: "related_kind", Label: "Related Kind", LabelI18n: localize("Related Kind", "Jenis Terkait"), Type: "string", AllowedValues: []string{"party", "lead", "opportunity", "ticket"}},
			{Key: "related_id", Label: "Related ID", LabelI18n: localize("Related ID", "ID Terkait"), Type: "string"},
			{Key: "party_id", Label: "Customer", LabelI18n: localize("Customer", "Pelanggan"), Type: "string", Reference: &model.ReferenceDefinition{ModelKey: "party"}},
			{Key: "party_name", Label: "Customer Name", LabelI18n: localize("Customer Name", "Nama Pelanggan"), Type: "string"},
			{Key: "owner_user_id", Label: "Owner User", LabelI18n: localize("Owner User", "User Pemilik"), Type: "string"},
			{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string", DefaultValue: "open", AllowedValues: []string{"open", "completed", "cancelled"}},
			{Key: "due_at", Label: "Due At", LabelI18n: localize("Due At", "Jatuh Tempo"), Type: "string"},
			{Key: "completed_at", Label: "Completed At", LabelI18n: localize("Completed At", "Selesai Pada"), Type: "string"},
			{Key: "note", Label: "Note", LabelI18n: localize("Note", "Catatan"), Type: "string"},
		},
	}
}

func crmResourceActions(routeKey, listLabel, detailLabel, formLabel, modelKey string) []module.ActionDefinition {
	base := "/crm/" + routeKey
	permPrefix := modelKey
	return []module.ActionDefinition{
		{Key: "crm." + routeKey + ".list", Label: listLabel, LabelI18n: localize(listLabel, listLabel), Kind: "navigate", RoutePath: base, ViewKey: "crm." + routeKey + ".list", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{permPrefix + ".list"}},
		{Key: "crm." + routeKey + ".detail", Label: detailLabel, LabelI18n: localize(detailLabel, detailLabel), Kind: "navigate", RoutePath: base + "/detail", ViewKey: "crm." + routeKey + ".detail", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{permPrefix + ".read"}},
		{Key: "crm." + routeKey + ".form", Label: formLabel, LabelI18n: localize(formLabel, formLabel), Kind: "navigate", RoutePath: base + "/form", ViewKey: "crm." + routeKey + ".form", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{permPrefix + ".update"}},
		{Key: "crm." + routeKey + ".new", Label: "New " + listLabel, LabelI18n: localize("New "+listLabel, "Baru"), Kind: "navigate", RoutePath: base + "/new", ViewKey: "crm." + routeKey + ".form", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{permPrefix + ".update"}},
	}
}

func crmResourceListView(routeKey, title, modelKey string, permissions []string, columns []module.ColumnDefinition, filters []module.FilterDefinition) module.ViewDefinition {
	return module.ViewDefinition{
		Key:                 "crm." + routeKey + ".list",
		Title:               title,
		TitleI18n:           localize(title, title),
		Kind:                "list",
		ModelKey:            modelKey,
		RequiredPermissions: permissions,
		DefaultPageSize:     20,
		Columns:             columns,
		Filters:             filters,
	}
}

func crmResourceDetailView(routeKey, title, modelKey string, permissions []string, fields []module.FieldDefinition) module.ViewDefinition {
	return module.ViewDefinition{
		Key:                 "crm." + routeKey + ".detail",
		Title:               title,
		TitleI18n:           localize(title, title),
		Kind:                "detail",
		ModelKey:            modelKey,
		RequiredPermissions: permissions,
		Tabs: []module.TabDefinition{{
			Key:       "summary",
			Title:     "Summary",
			TitleI18n: localize("Summary", "Ringkasan"),
			Sections: []module.SectionDefinition{{
				Key:       "detail",
				Title:     title,
				TitleI18n: localize(title, title),
				Fields:    fields,
			}},
		}},
	}
}

func crmResourceFormView(routeKey, title, modelKey string, permissions []string, fields []module.FieldDefinition) module.ViewDefinition {
	return module.ViewDefinition{
		Key:                 "crm." + routeKey + ".form",
		Title:               title,
		TitleI18n:           localize(title, title),
		Kind:                "form",
		ModelKey:            modelKey,
		RequiredPermissions: permissions,
		Sections: []module.SectionDefinition{{
			Key:       "edit",
			Title:     title,
			TitleI18n: localize(title, title),
			Fields:    fields,
		}},
	}
}

func crmQueueListView() module.ViewDefinition {
	return crmResourceListView("queues", "Ticket Queues", "crm_queue", []string{"crm_queue.list"}, []module.ColumnDefinition{
		{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code"},
		{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name"},
		{Key: "triage_sla_hours", Label: "Triage SLA", LabelI18n: localize("Triage SLA", "SLA Triase"), Path: "values.triage_sla_hours"},
		{Key: "resolution_sla_hours", Label: "Resolution SLA", LabelI18n: localize("Resolution SLA", "SLA Resolusi"), Path: "values.resolution_sla_hours"},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status"},
	}, []module.FilterDefinition{{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "select", Options: []string{"active", "inactive"}}})
}

func crmQueueDetailView() module.ViewDefinition { return crmResourceDetailView("queues", "Ticket Queue Detail", "crm_queue", []string{"crm_queue.read"}, crmQueueFields(false)) }
func crmQueueFormView() module.ViewDefinition   { return crmResourceFormView("queues", "Ticket Queue Form", "crm_queue", []string{"crm_queue.update"}, crmQueueFields(true)) }

func crmSLAPolicyListView() module.ViewDefinition {
	return crmResourceListView("sla-policies", "SLA Policies", "crm_sla_policy", []string{"crm_sla_policy.list"}, []module.ColumnDefinition{
		{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code"},
		{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name"},
		{Key: "queue_code", Label: "Queue", LabelI18n: localize("Queue", "Queue"), Path: "values.queue_code"},
		{Key: "first_response_hours", Label: "First Response", LabelI18n: localize("First Response", "Respons Pertama"), Path: "values.first_response_hours"},
		{Key: "resolution_hours", Label: "Resolution", LabelI18n: localize("Resolution", "Resolusi"), Path: "values.resolution_hours"},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status"},
	}, []module.FilterDefinition{{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "select", Options: []string{"active", "inactive"}}})
}

func crmSLAPolicyDetailView() module.ViewDefinition { return crmResourceDetailView("sla-policies", "SLA Policy Detail", "crm_sla_policy", []string{"crm_sla_policy.read"}, crmSLAPolicyFields(false)) }
func crmSLAPolicyFormView() module.ViewDefinition   { return crmResourceFormView("sla-policies", "SLA Policy Form", "crm_sla_policy", []string{"crm_sla_policy.update"}, crmSLAPolicyFields(true)) }

func crmAssignmentRuleListView() module.ViewDefinition {
	return crmResourceListView("assignment-rules", "Assignment Rules", "crm_assignment_rule", []string{"crm_assignment_rule.list"}, []module.ColumnDefinition{
		{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code"},
		{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name"},
		{Key: "assign_queue_code", Label: "Assign Queue", LabelI18n: localize("Assign Queue", "Queue Tujuan"), Path: "values.assign_queue_code"},
		{Key: "assign_user_id", Label: "Assign User", LabelI18n: localize("Assign User", "User Tujuan"), Path: "values.assign_user_id"},
		{Key: "rank", Label: "Rank", LabelI18n: localize("Rank", "Peringkat"), Path: "values.rank"},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status"},
	}, []module.FilterDefinition{{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "select", Options: []string{"active", "inactive"}}})
}

func crmAssignmentRuleDetailView() module.ViewDefinition {
	return crmResourceDetailView("assignment-rules", "Assignment Rule Detail", "crm_assignment_rule", []string{"crm_assignment_rule.read"}, crmAssignmentRuleFields(false))
}

func crmAssignmentRuleFormView() module.ViewDefinition {
	return crmResourceFormView("assignment-rules", "Assignment Rule Form", "crm_assignment_rule", []string{"crm_assignment_rule.update"}, crmAssignmentRuleFields(true))
}

func crmTicketListView() module.ViewDefinition {
	return crmResourceListView("tickets", "Issue Tickets", "crm_ticket", []string{"crm_ticket.list"}, []module.ColumnDefinition{
		{Key: "ticket_number", Label: "Ticket", LabelI18n: localize("Ticket", "Tiket"), Path: "values.ticket_number"},
		{Key: "title", Label: "Title", LabelI18n: localize("Title", "Judul"), Path: "values.title"},
		{Key: "party_name", Label: "Customer", LabelI18n: localize("Customer", "Pelanggan"), Path: "values.party_name"},
		{Key: "queue_code", Label: "Queue", LabelI18n: localize("Queue", "Queue"), Path: "values.queue_code"},
		{Key: "priority", Label: "Priority", LabelI18n: localize("Priority", "Prioritas"), Path: "values.priority"},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status"},
	}, []module.FilterDefinition{
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "select", Options: []string{"new", "open", "pending_customer", "pending_internal", "resolved", "closed", "cancelled"}},
		{Key: "priority", Label: "Priority", LabelI18n: localize("Priority", "Prioritas"), Type: "select", Options: []string{"low", "medium", "high", "urgent"}},
		{Key: "queue_code", Label: "Queue", LabelI18n: localize("Queue", "Queue"), Type: "text"},
	})
}

func crmTicketDetailView() module.ViewDefinition { return crmResourceDetailView("tickets", "Issue Ticket Detail", "crm_ticket", []string{"crm_ticket.read"}, crmTicketFields(false)) }
func crmTicketFormView() module.ViewDefinition   { return crmResourceFormView("tickets", "Issue Ticket Form", "crm_ticket", []string{"crm_ticket.update"}, crmTicketFields(true)) }

func crmTicketCommentListView() module.ViewDefinition {
	return crmResourceListView("ticket-comments", "Ticket Comments", "crm_ticket_comment", []string{"crm_ticket_comment.list"}, []module.ColumnDefinition{
		{Key: "ticket_number", Label: "Ticket", LabelI18n: localize("Ticket", "Tiket"), Path: "values.ticket_number"},
		{Key: "comment_type", Label: "Type", LabelI18n: localize("Type", "Tipe"), Path: "values.comment_type"},
		{Key: "author_user_id", Label: "Author", LabelI18n: localize("Author", "Penulis"), Path: "values.author_user_id"},
		{Key: "created_at", Label: "Created At", LabelI18n: localize("Created At", "Dibuat Pada"), Path: "values.created_at"},
		{Key: "body", Label: "Body", LabelI18n: localize("Body", "Isi"), Path: "values.body"},
	}, []module.FilterDefinition{{Key: "comment_type", Label: "Type", LabelI18n: localize("Type", "Tipe"), Type: "select", Options: []string{"internal_note", "public_reply", "status_update"}}})
}

func crmTicketCommentDetailView() module.ViewDefinition {
	return crmResourceDetailView("ticket-comments", "Ticket Comment Detail", "crm_ticket_comment", []string{"crm_ticket_comment.read"}, crmTicketCommentFields(false))
}

func crmTicketCommentFormView() module.ViewDefinition {
	return crmResourceFormView("ticket-comments", "Ticket Comment Form", "crm_ticket_comment", []string{"crm_ticket_comment.update"}, crmTicketCommentFields(true))
}

func crmTicketActivityListView() module.ViewDefinition {
	return crmResourceListView("ticket-activities", "Ticket Activities", "crm_ticket_activity", []string{"crm_ticket_activity.list"}, []module.ColumnDefinition{
		{Key: "ticket_number", Label: "Ticket", LabelI18n: localize("Ticket", "Tiket"), Path: "values.ticket_number"},
		{Key: "activity_type", Label: "Activity", LabelI18n: localize("Activity", "Aktivitas"), Path: "values.activity_type"},
		{Key: "actor_user_id", Label: "Actor", LabelI18n: localize("Actor", "Aktor"), Path: "values.actor_user_id"},
		{Key: "to_status", Label: "To Status", LabelI18n: localize("To Status", "Ke Status"), Path: "values.to_status"},
		{Key: "occurred_at", Label: "Occurred At", LabelI18n: localize("Occurred At", "Terjadi Pada"), Path: "values.occurred_at"},
	}, nil)
}

func crmTicketActivityDetailView() module.ViewDefinition {
	return crmResourceDetailView("ticket-activities", "Ticket Activity Detail", "crm_ticket_activity", []string{"crm_ticket_activity.read"}, crmTicketActivityFields(false))
}

func crmTicketActivityFormView() module.ViewDefinition {
	return crmResourceFormView("ticket-activities", "Ticket Activity Form", "crm_ticket_activity", []string{"crm_ticket_activity.update"}, crmTicketActivityFields(true))
}

func crmLeadListView() module.ViewDefinition {
	return crmResourceListView("leads", "Leads", "crm_lead", []string{"crm_lead.list"}, []module.ColumnDefinition{
		{Key: "lead_number", Label: "Lead", LabelI18n: localize("Lead", "Lead"), Path: "values.lead_number"},
		{Key: "title", Label: "Title", LabelI18n: localize("Title", "Judul"), Path: "values.title"},
		{Key: "party_name", Label: "Customer", LabelI18n: localize("Customer", "Pelanggan"), Path: "values.party_name"},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status"},
		{Key: "rating", Label: "Rating", LabelI18n: localize("Rating", "Rating"), Path: "values.rating"},
		{Key: "estimated_value", Label: "Value", LabelI18n: localize("Value", "Nilai"), Path: "values.estimated_value"},
	}, []module.FilterDefinition{{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "select", Options: []string{"new", "contacted", "qualified", "disqualified", "converted", "closed"}}})
}

func crmLeadDetailView() module.ViewDefinition { return crmResourceDetailView("leads", "Lead Detail", "crm_lead", []string{"crm_lead.read"}, crmLeadFields(false)) }
func crmLeadFormView() module.ViewDefinition   { return crmResourceFormView("leads", "Lead Form", "crm_lead", []string{"crm_lead.update"}, crmLeadFields(true)) }

func crmOpportunityListView() module.ViewDefinition {
	return crmResourceListView("opportunities", "Opportunities", "crm_opportunity", []string{"crm_opportunity.list"}, []module.ColumnDefinition{
		{Key: "opportunity_number", Label: "Opportunity", LabelI18n: localize("Opportunity", "Peluang"), Path: "values.opportunity_number"},
		{Key: "title", Label: "Title", LabelI18n: localize("Title", "Judul"), Path: "values.title"},
		{Key: "party_name", Label: "Customer", LabelI18n: localize("Customer", "Pelanggan"), Path: "values.party_name"},
		{Key: "stage", Label: "Stage", LabelI18n: localize("Stage", "Stage"), Path: "values.stage"},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status"},
		{Key: "estimated_value", Label: "Value", LabelI18n: localize("Value", "Nilai"), Path: "values.estimated_value"},
	}, []module.FilterDefinition{{Key: "stage", Label: "Stage", LabelI18n: localize("Stage", "Stage"), Type: "select", Options: []string{"new", "qualified", "proposal", "negotiation", "won", "lost"}}})
}

func crmOpportunityDetailView() module.ViewDefinition {
	return crmResourceDetailView("opportunities", "Opportunity Detail", "crm_opportunity", []string{"crm_opportunity.read"}, crmOpportunityFields(false))
}

func crmOpportunityFormView() module.ViewDefinition {
	return crmResourceFormView("opportunities", "Opportunity Form", "crm_opportunity", []string{"crm_opportunity.update"}, crmOpportunityFields(true))
}

func crmActivityListView() module.ViewDefinition {
	return crmResourceListView("activities", "CRM Activities", "crm_activity", []string{"crm_activity.list"}, []module.ColumnDefinition{
		{Key: "activity_number", Label: "Activity", LabelI18n: localize("Activity", "Aktivitas"), Path: "values.activity_number"},
		{Key: "activity_type", Label: "Type", LabelI18n: localize("Type", "Tipe"), Path: "values.activity_type"},
		{Key: "subject", Label: "Subject", LabelI18n: localize("Subject", "Subjek"), Path: "values.subject"},
		{Key: "owner_user_id", Label: "Owner", LabelI18n: localize("Owner", "Pemilik"), Path: "values.owner_user_id"},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status"},
		{Key: "due_at", Label: "Due At", LabelI18n: localize("Due At", "Jatuh Tempo"), Path: "values.due_at"},
	}, []module.FilterDefinition{{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "select", Options: []string{"open", "completed", "cancelled"}}})
}

func crmActivityDetailView() module.ViewDefinition { return crmResourceDetailView("activities", "CRM Activity Detail", "crm_activity", []string{"crm_activity.read"}, crmActivityFields(false)) }
func crmActivityFormView() module.ViewDefinition   { return crmResourceFormView("activities", "CRM Activity Form", "crm_activity", []string{"crm_activity.update"}, crmActivityFields(true)) }

func crmQueueFields(form bool) []module.FieldDefinition {
	return crmViewFields(form,
		viewField("code", "Code", "values.code", "string", "text", true, nil),
		viewField("name", "Name", "values.name", "string", "text", true, nil),
		viewField("description", "Description", "values.description", "string", "textarea", false, nil),
		viewField("manager_user_id", "Manager User", "values.manager_user_id", "string", "text", false, nil),
		viewField("triage_sla_hours", "Triage SLA Hours", "values.triage_sla_hours", "number", "number", false, nil),
		viewField("resolution_sla_hours", "Resolution SLA Hours", "values.resolution_sla_hours", "number", "number", false, nil),
		viewSelectField("status", "Status", "values.status", []string{"active", "inactive"}),
	)
}

func crmSLAPolicyFields(form bool) []module.FieldDefinition {
	return crmViewFields(form,
		viewField("code", "Code", "values.code", "string", "text", true, nil),
		viewField("name", "Name", "values.name", "string", "text", true, nil),
		viewField("queue_code", "Queue", "values.queue_code", "string", "text", false, nil),
		viewSelectField("source_channel", "Source Channel", "values.source_channel", []string{"", "web", "email", "phone", "chat", "pos"}),
		viewSelectField("priority", "Priority", "values.priority", []string{"", "low", "medium", "high", "urgent"}),
		viewSelectField("severity", "Severity", "values.severity", []string{"", "low", "medium", "high", "critical"}),
		viewField("first_response_hours", "First Response Hours", "values.first_response_hours", "number", "number", false, nil),
		viewField("resolution_hours", "Resolution Hours", "values.resolution_hours", "number", "number", false, nil),
		viewSelectField("status", "Status", "values.status", []string{"active", "inactive"}),
	)
}

func crmAssignmentRuleFields(form bool) []module.FieldDefinition {
	return crmViewFields(form,
		viewField("code", "Code", "values.code", "string", "text", true, nil),
		viewField("name", "Name", "values.name", "string", "text", true, nil),
		viewField("queue_code", "Queue", "values.queue_code", "string", "text", false, nil),
		viewField("assign_queue_code", "Assign Queue", "values.assign_queue_code", "string", "text", false, nil),
		viewField("assign_user_id", "Assign User", "values.assign_user_id", "string", "text", false, nil),
		viewField("source_channel", "Source Channel", "values.source_channel", "string", "text", false, nil),
		viewField("issue_category", "Issue Category", "values.issue_category", "string", "text", false, nil),
		viewSelectField("priority", "Priority", "values.priority", []string{"", "low", "medium", "high", "urgent"}),
		viewSelectField("severity", "Severity", "values.severity", []string{"", "low", "medium", "high", "critical"}),
		viewField("rank", "Rank", "values.rank", "number", "number", false, nil),
		viewSelectField("status", "Status", "values.status", []string{"active", "inactive"}),
	)
}

func crmTicketFields(form bool) []module.FieldDefinition {
	return crmViewFields(form,
		readOnlyField("ticket_number", "Ticket Number", "values.ticket_number"),
		viewField("title", "Title", "values.title", "string", "text", true, nil),
		viewField("description", "Description", "values.description", "string", "textarea", false, nil),
		viewField("party_id", "Customer", "values.party_id", "string", "text", false, nil),
		viewField("party_name", "Customer Name", "values.party_name", "string", "text", false, nil),
		viewField("queue_code", "Queue", "values.queue_code", "string", "text", false, nil),
		viewSelectField("source_channel", "Source Channel", "values.source_channel", []string{"web", "email", "phone", "chat", "pos"}),
		viewSelectField("priority", "Priority", "values.priority", []string{"low", "medium", "high", "urgent"}),
		viewSelectField("severity", "Severity", "values.severity", []string{"low", "medium", "high", "critical"}),
		viewSelectField("status", "Status", "values.status", []string{"new", "open", "pending_customer", "pending_internal", "resolved", "closed", "cancelled"}),
		viewField("assignee_user_id", "Assignee User", "values.assignee_user_id", "string", "text", false, nil),
		viewField("opened_at", "Opened At", "values.opened_at", "string", "text", false, nil),
		viewField("first_response_due_at", "First Response Due At", "values.first_response_due_at", "string", "text", false, nil),
		viewField("first_response_at", "First Response At", "values.first_response_at", "string", "text", false, nil),
		viewField("due_at", "Due At", "values.due_at", "string", "text", false, nil),
		viewField("resolved_at", "Resolved At", "values.resolved_at", "string", "text", false, nil),
		viewField("issue_category", "Issue Category", "values.issue_category", "string", "text", false, nil),
		viewField("resolution_notes", "Resolution Notes", "values.resolution_notes", "string", "textarea", false, nil),
		viewField("tags_json", "Tags JSON", "values.tags_json", "string", "textarea", false, nil),
	)
}

func crmTicketCommentFields(form bool) []module.FieldDefinition {
	return crmViewFields(form,
		viewField("ticket_id", "Ticket", "values.ticket_id", "string", "text", true, nil),
		viewField("ticket_number", "Ticket Number", "values.ticket_number", "string", "text", false, nil),
		viewSelectField("comment_type", "Comment Type", "values.comment_type", []string{"internal_note", "public_reply", "status_update"}),
		viewField("body", "Body", "values.body", "string", "textarea", true, nil),
		viewField("author_user_id", "Author User", "values.author_user_id", "string", "text", false, nil),
		viewField("created_at", "Created At", "values.created_at", "string", "text", false, nil),
		viewField("party_id", "Customer", "values.party_id", "string", "text", false, nil),
		viewField("party_name", "Customer Name", "values.party_name", "string", "text", false, nil),
	)
}

func crmTicketActivityFields(form bool) []module.FieldDefinition {
	return crmViewFields(form,
		viewField("ticket_id", "Ticket", "values.ticket_id", "string", "text", true, nil),
		viewField("ticket_number", "Ticket Number", "values.ticket_number", "string", "text", false, nil),
		viewField("activity_type", "Activity Type", "values.activity_type", "string", "text", true, nil),
		viewField("actor_user_id", "Actor User", "values.actor_user_id", "string", "text", false, nil),
		viewField("assignee_user_id", "Assignee User", "values.assignee_user_id", "string", "text", false, nil),
		viewField("queue_code", "Queue", "values.queue_code", "string", "text", false, nil),
		viewField("from_status", "From Status", "values.from_status", "string", "text", false, nil),
		viewField("to_status", "To Status", "values.to_status", "string", "text", false, nil),
		viewField("occurred_at", "Occurred At", "values.occurred_at", "string", "text", false, nil),
		viewField("note", "Note", "values.note", "string", "textarea", false, nil),
		viewField("party_id", "Customer", "values.party_id", "string", "text", false, nil),
		viewField("party_name", "Customer Name", "values.party_name", "string", "text", false, nil),
	)
}

func crmLeadFields(form bool) []module.FieldDefinition {
	return crmViewFields(form,
		readOnlyField("lead_number", "Lead Number", "values.lead_number"),
		viewField("title", "Title", "values.title", "string", "text", true, nil),
		viewField("party_id", "Customer", "values.party_id", "string", "text", false, nil),
		viewField("party_name", "Customer Name", "values.party_name", "string", "text", false, nil),
		viewField("contact_id", "Contact", "values.contact_id", "string", "text", false, nil),
		viewField("owner_user_id", "Owner User", "values.owner_user_id", "string", "text", false, nil),
		viewSelectField("source_channel", "Source Channel", "values.source_channel", []string{"web", "email", "phone", "chat", "partner", "referral", "walk_in"}),
		viewSelectField("status", "Status", "values.status", []string{"new", "contacted", "qualified", "disqualified", "converted", "closed"}),
		viewSelectField("rating", "Rating", "values.rating", []string{"cold", "warm", "hot"}),
		viewField("estimated_value", "Estimated Value", "values.estimated_value", "number", "number", false, nil),
		viewField("expected_close_date", "Expected Close Date", "values.expected_close_date", "string", "text", false, nil),
		viewField("next_action_at", "Next Action At", "values.next_action_at", "string", "text", false, nil),
		viewField("notes", "Notes", "values.notes", "string", "textarea", false, nil),
	)
}

func crmOpportunityFields(form bool) []module.FieldDefinition {
	return crmViewFields(form,
		readOnlyField("opportunity_number", "Opportunity Number", "values.opportunity_number"),
		viewField("title", "Title", "values.title", "string", "text", true, nil),
		viewField("party_id", "Customer", "values.party_id", "string", "text", false, nil),
		viewField("party_name", "Customer Name", "values.party_name", "string", "text", false, nil),
		viewField("contact_id", "Contact", "values.contact_id", "string", "text", false, nil),
		viewField("owner_user_id", "Owner User", "values.owner_user_id", "string", "text", false, nil),
		viewField("source_lead_id", "Source Lead", "values.source_lead_id", "string", "text", false, nil),
		viewSelectField("stage", "Stage", "values.stage", []string{"new", "qualified", "proposal", "negotiation", "won", "lost"}),
		viewSelectField("status", "Status", "values.status", []string{"open", "won", "lost"}),
		viewField("estimated_value", "Estimated Value", "values.estimated_value", "number", "number", false, nil),
		viewField("expected_close_date", "Expected Close Date", "values.expected_close_date", "string", "text", false, nil),
		viewField("next_action_at", "Next Action At", "values.next_action_at", "string", "text", false, nil),
		viewField("loss_reason", "Loss Reason", "values.loss_reason", "string", "text", false, nil),
		viewField("notes", "Notes", "values.notes", "string", "textarea", false, nil),
	)
}

func crmActivityFields(form bool) []module.FieldDefinition {
	return crmViewFields(form,
		readOnlyField("activity_number", "Activity Number", "values.activity_number"),
		viewField("activity_type", "Activity Type", "values.activity_type", "string", "text", true, nil),
		viewField("subject", "Subject", "values.subject", "string", "text", true, nil),
		viewSelectField("related_kind", "Related Kind", "values.related_kind", []string{"party", "lead", "opportunity", "ticket"}),
		viewField("related_id", "Related ID", "values.related_id", "string", "text", false, nil),
		viewField("party_id", "Customer", "values.party_id", "string", "text", false, nil),
		viewField("party_name", "Customer Name", "values.party_name", "string", "text", false, nil),
		viewField("owner_user_id", "Owner User", "values.owner_user_id", "string", "text", false, nil),
		viewSelectField("status", "Status", "values.status", []string{"open", "completed", "cancelled"}),
		viewField("due_at", "Due At", "values.due_at", "string", "text", false, nil),
		viewField("completed_at", "Completed At", "values.completed_at", "string", "text", false, nil),
		viewField("note", "Note", "values.note", "string", "textarea", false, nil),
	)
}

func crmViewFields(form bool, fields ...module.FieldDefinition) []module.FieldDefinition {
	items := make([]module.FieldDefinition, 0, len(fields))
	for _, item := range fields {
		if !form {
			item.Widget = ""
			item.Required = false
		}
		items = append(items, item)
	}
	return items
}

func viewField(key, label, path, fieldType, widget string, required bool, options []string) module.FieldDefinition {
	return module.FieldDefinition{
		Key:       key,
		Label:     label,
		LabelI18n: localize(label, label),
		Path:      path,
		Type:      fieldType,
		Widget:    widget,
		Required:  required,
		Options:   options,
	}
}

func viewSelectField(key, label, path string, options []string) module.FieldDefinition {
	return viewField(key, label, path, "string", "select", false, options)
}

func readOnlyField(key, label, path string) module.FieldDefinition {
	return module.FieldDefinition{
		Key:       key,
		Label:     label,
		LabelI18n: localize(label, label),
		Path:      path,
		Type:      "string",
		ReadOnly:  true,
		Widget:    "text",
	}
}
