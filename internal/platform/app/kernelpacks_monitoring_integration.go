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
