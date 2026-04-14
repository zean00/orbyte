package app

import "orbyte/internal/platform/module"

func monitoringKernelPackManifest() module.Manifest {
	return module.Manifest{
		Key:          "monitoring",
		Name:         "Monitoring",
		NameI18n:     localize("Monitoring", "Pemantauan"),
		Version:      "1.0.0",
		DomainFamily: "platform",
		DependencyRequirements: []module.DependencyRequirement{{
			ModuleKey: "platform.core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired,
		}},
		AdminConsole: module.AdminConsoleDefinition{
			Title:           "Monitoring Console",
			TitleI18n:       localize("Monitoring Console", "Konsol Pemantauan"),
			Description:     "Monitoring dashboards and observability shortcuts.",
			DescriptionI18n: localize("Monitoring dashboards and observability shortcuts.", "Dashboard pemantauan dan pintasan observabilitas."),
			Sections: []module.AdminConsoleSectionDefinition{
				{
					Key:       "monitoring_operations",
					Title:     "Monitoring Operations",
					TitleI18n: localize("Monitoring Operations", "Operasi Pemantauan"),
					Kind:      module.AdminConsoleSectionResourceLinks,
					Links: []module.AdminConsoleLinkDefinition{
						adminConsoleLink("monitoring_dashboard", "Monitoring Dashboard", "Dashboard Pemantauan", "/ui/monitoring", "Open the monitoring dashboard.", "Buka dashboard pemantauan.", "monitoring.read"),
						adminConsoleLink("observability", "Observability", "Observabilitas", "/admin/observability", "Open the observability admin page.", "Buka halaman admin observabilitas.", "module.read"),
						adminConsoleLink("security", "Security", "Keamanan", "/admin/security", "Open security policies related to monitoring access.", "Buka kebijakan keamanan terkait akses pemantauan.", "configuration.read"),
					},
				},
			},
		},
		Security: module.SecurityDefinition{
			Permissions: []module.PermissionDefinition{
				{Key: "metrics.read", Action: "read", Resource: "metrics", DisplayName: "Read Metrics", DisplayNameI18n: localize("Read Metrics", "Lihat Metrik")},
				{Key: "monitoring.read", Action: "read", Resource: "dashboard", DisplayName: "Read Monitoring Dashboard", DisplayNameI18n: localize("Read Monitoring Dashboard", "Lihat Dashboard Pemantauan")},
				{Key: "audit.read", Action: "read", Resource: "audit_event", DisplayName: "Read Audit Events", DisplayNameI18n: localize("Read Audit Events", "Lihat Event Audit")},
				{Key: "event.read", Action: "read", Resource: "domain_event", DisplayName: "Read Domain Events", DisplayNameI18n: localize("Read Domain Events", "Lihat Event Domain")},
				{Key: "outbox.read", Action: "read", Resource: "outbox", DisplayName: "Read Outbox", DisplayNameI18n: localize("Read Outbox", "Lihat Outbox")},
				{Key: "outbox.dispatch", Action: "dispatch", Resource: "outbox", DisplayName: "Dispatch Outbox", DisplayNameI18n: localize("Dispatch Outbox", "Kirim Outbox"), RiskLevel: "high"},
				{Key: "deadletter.read", Action: "read", Resource: "dead_letter", DisplayName: "Read Dead Letters", DisplayNameI18n: localize("Read Dead Letters", "Lihat Dead Letter")},
			},
		},
		Observability: module.ObservabilityDefinition{
			Dashboards: []module.DashboardDefinition{
				{Key: "monitoring.overview", Title: "Monitoring Overview", TitleI18n: localize("Monitoring Overview", "Ikhtisar Pemantauan"), ViewKey: "monitoring.overview", RequiredPermissions: []string{"monitoring.read"}},
			},
			Metrics: []module.MetricDefinition{
				{Key: "http.responses.404.total", Type: "counter", Description: "HTTP 404 responses"},
			},
		},
		Frontend: module.FrontendDefinition{
			Menus: []module.MenuDefinition{{
				Key: "monitoring.overview", Label: "Monitoring", LabelI18n: localize("Monitoring", "Pemantauan"), ActionKey: "monitoring.overview", Order: 30, RequiredPermissions: []string{"monitoring.read"},
			}},
			Actions: []module.ActionDefinition{{
				Key: "monitoring.overview", Label: "Monitoring Overview", LabelI18n: localize("Monitoring Overview", "Ikhtisar Pemantauan"), Kind: "navigate", RoutePath: "/monitoring", ViewKey: "monitoring.overview", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"monitoring.read"},
			}},
			Views: []module.ViewDefinition{{
				Key: "monitoring.overview", Title: "Monitoring Overview", TitleI18n: localize("Monitoring Overview", "Ikhtisar Pemantauan"), Kind: "dashboard", ProjectionKey: "monitoring.summary", RequiredPermissions: []string{"monitoring.read"},
				Cards: []module.CardDefinition{
					{Key: "documents_total", Label: "Documents", LabelI18n: localize("Documents", "Dokumen"), Path: "documents.total"},
					{Key: "outbox_pending", Label: "Outbox Pending", LabelI18n: localize("Outbox Pending", "Outbox Tertunda"), Path: "outbox.pending"},
					{Key: "pending_approvals", Label: "Pending Approvals", LabelI18n: localize("Pending Approvals", "Persetujuan Tertunda"), Path: "workflow.pending_approvals"},
					{Key: "projections", Label: "Document Summaries", LabelI18n: localize("Document Summaries", "Ringkasan Dokumen"), Path: "projections.document_summaries"},
				},
			}},
			DashboardWidgets: []module.DashboardWidgetDefinition{
				{
					Key:                 "monitoring.outbox.pending",
					Title:               "Outbox Pending",
					TitleI18n:           localize("Outbox Pending", "Outbox Tertunda"),
					Surface:             module.UISurfaceDashboard,
					RendererKind:        "gauge",
					RefreshPolicy:       "minutes",
					DataPath:            "/ui/data/monitoring/summary",
					RequiredPermissions: []string{"monitoring.read"},
					DefaultWidth:        3,
					DefaultHeight:       2,
					Gauge: &module.DashboardGaugeSpec{
						ValuePath:  "outbox.pending",
						MinValue:   0,
						MaxValue:   50,
						Thresholds: []float64{5, 15, 30},
						Format:     "number",
					},
				},
			},
		},
	}
}

func integrationKernelPackManifest() module.Manifest {
	return module.Manifest{
		Key:          "integration",
		Name:         "Integration Kernel",
		NameI18n:     localize("Integration Kernel", "Kernel Integrasi"),
		Version:      "1.0.0",
		DomainFamily: "platform",
		DependencyRequirements: []module.DependencyRequirement{
			{ModuleKey: "platform.core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "documents", VersionRange: ">=1.1.0,<2.0.0", Kind: module.DependencyKindOptional},
		},
		AdminConsole: module.AdminConsoleDefinition{
			Title:           "Integration Console",
			TitleI18n:       localize("Integration Console", "Konsol Integrasi"),
			Description:     "Integration policy hooks and observability entry points.",
			DescriptionI18n: localize("Integration policy hooks and observability entry points.", "Hook kebijakan integrasi dan pintu masuk observabilitas."),
			Sections: []module.AdminConsoleSectionDefinition{
				{
					Key:       "integration_operations",
					Title:     "Integration Operations",
					TitleI18n: localize("Integration Operations", "Operasi Integrasi"),
					Kind:      module.AdminConsoleSectionResourceLinks,
					Links: []module.AdminConsoleLinkDefinition{
						adminConsoleLink("observability", "Observability", "Observabilitas", "/admin/observability", "Inspect integration metrics and logs.", "Periksa metrik dan log integrasi.", "module.read"),
						adminConsoleLink("security", "Security", "Keamanan", "/admin/security", "Open policy hooks and security rules.", "Buka hook kebijakan dan aturan keamanan.", "configuration.read"),
					},
				},
			},
		},
		Security: module.SecurityDefinition{
			PolicyHooks: []module.PolicyHookDefinition{
				{Key: "integration.submission.preflight", Kind: "integration", Target: "submission_preflight", InputContractKey: "integration.submission.v1", OutputContractKey: "decision.v1", Description: "Validates integration submissions before they are queued."},
			},
		},
		Observability: module.ObservabilityDefinition{
			Metrics: []module.MetricDefinition{
				{Key: "integration.submissions.queued.total", Type: "counter", Description: "Queued integration submissions"},
				{Key: "integration.submissions.succeeded.total", Type: "counter", Description: "Succeeded integration submissions"},
				{Key: "integration.submissions.failed.total", Type: "counter", Description: "Failed integration submissions"},
				{Key: "analytics.scheduler.enqueued.total", Type: "counter", Description: "Scheduled analytics jobs enqueued"},
				{Key: "analytics.scheduler.already_claimed.total", Type: "counter", Description: "Scheduled analytics work already claimed through shared job deduplication"},
				{Key: "analytics.scheduler.enqueue_failed.total", Type: "counter", Description: "Scheduled analytics enqueue failures"},
			},
			LogEvents: []module.LogEventDefinition{
				{Key: "integration.submission.succeeded", Category: "integration", Severity: "info", RequiredFields: []string{"submission_id", "system_key", "operation", "status"}},
				{Key: "integration.submission.failed", Category: "integration", Severity: "error", RequiredFields: []string{"submission_id", "system_key", "operation", "status"}},
			},
		},
	}
}
