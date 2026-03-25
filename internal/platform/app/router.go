package app

import (
	"orbyte/internal/platform/httpx"
)

const (
	analyticsMCPStreamPath       = "/mcp/events/analytics/snapshot"
	analyticsScopedMCPStreamPath = "/mcp/analytics/events/analytics/snapshot"
)

func routerConfig(graph *serviceGraph) httpx.RouterConfig {
	uiPreferences := graph.uiPreferences
	modelDeps := httpx.ModelDeps{
		Identity:      graph.identity,
		Models:        graph.models,
		Activities:    graph.activities,
		Policy:        graph.policy,
		FieldSecurity: graph.fieldSecurity,
		Actions:       graph.modelActions,
	}
	documentDeps := httpx.DocumentDeps{
		Identity:      graph.identity,
		Modules:       graph.modules,
		Documents:     graph.documents,
		Actions:       graph.docActions,
		Policy:        graph.policy,
		Search:        graph.search,
		FieldSecurity: graph.fieldSecurity,
		Observability: graph.observability,
		Idempotency:   graph.idempotency,
	}
	uiDeps := httpx.UIDeps{
		Identity:      graph.identity,
		Modules:       graph.modules,
		Models:        graph.models,
		Activities:    graph.activities,
		Reporting:     graph.reporting,
		Documents:     graph.documents,
		Workflows:     graph.workflows,
		Search:        graph.search,
		Analytics:     graph.analytics,
		Monitoring:    graph.monitoring,
		Policy:        graph.policy,
		FieldSecurity: graph.fieldSecurity,
		UIPreferences: uiPreferences,
		ACP:           graph.acp,
		Notifications: graph.notifications,
	}
	return httpx.RouterConfig{
		Registrars: []httpx.RouteRegistrar{
			httpx.RegisterPlatformSurface(httpx.PlatformDeps{
				Config:       graph.config,
				Organization: graph.organization,
				Identity:     graph.identity,
				Reference:    graph.reference,
				Documents:    graph.documents,
				Workflows:    graph.workflows,
				Health:       graph.runtimeHealth,
			}),
			httpx.RegisterAuthSurface(httpx.AuthDeps{
				Config:        graph.config,
				Identity:      graph.identity,
				Audit:         graph.audit,
				UIPreferences: uiPreferences,
			}),
			httpx.RegisterModelSurface(modelDeps),
			httpx.RegisterDocumentSurface(documentDeps),
			httpx.RegisterOpsSurface(httpx.OpsDeps{
				Identity:      graph.identity,
				Audit:         graph.audit,
				Eventing:      graph.eventing,
				Offline:       graph.offline,
				Documents:     graph.documents,
				Search:        graph.search,
				Workflows:     graph.workflows,
				Analytics:     graph.analytics,
				Monitoring:    graph.monitoring,
				Notifications: graph.notifications,
				Observability: graph.observability,
				Integration:   graph.integration,
				Jobs:          graph.jobs,
				Health:        graph.runtimeHealth,
			}),
			httpx.RegisterSearchSurface(httpx.SearchDeps{
				Identity: graph.identity,
				Search:   graph.search,
				Jobs:     graph.jobs,
			}),
			httpx.RegisterAdminSurface(httpx.AdminDeps{
				Config:        graph.config,
				Flags:         graph.flags,
				Organization:  graph.organization,
				Identity:      graph.identity,
				Modules:       graph.modules,
				Workflows:     graph.workflows,
				Audit:         graph.audit,
				Policy:        graph.policy,
				Observability: graph.observability,
				Integration:   graph.integration,
				Reference:     graph.reference,
				Idempotency:   graph.idempotency,
				Health:        graph.runtimeHealth,
				ACP:           graph.acp,
			}),
			httpx.RegisterACPSurface(httpx.ACPDeps{
				Identity: graph.identity,
				Audit:    graph.audit,
				Service:  graph.acp,
			}),
			httpx.RegisterTemplateSurface(httpx.TemplateDeps{
				Identity:  graph.identity,
				Templates: graph.templates,
			}),
			httpx.RegisterMCPSurface(httpx.MCPDeps{
				Identity:         graph.identity,
				Audit:            graph.audit,
				Server:           graph.mcpServer,
				AnalyticsStream:  graph.mcpAnalytics,
				Analytics:        graph.analytics,
				StreamPath:       analyticsMCPStreamPath,
				ScopedStreamPath: analyticsScopedMCPStreamPath,
			}),
			httpx.RegisterOfflineSurface(httpx.OfflineDeps{
				Identity:        graph.identity,
				Modules:         graph.modules,
				Offline:         graph.offline,
				Documents:       graph.documents,
				DocumentActions: graph.docActions,
				Models:          graph.models,
				ModelActions:    graph.modelActions,
				Search:          graph.search,
				FieldSecurity:   graph.fieldSecurity,
				Idempotency:     graph.idempotency,
			}),
			httpx.RegisterDocsSurface(httpx.DocsDeps{
				Config:    graph.config,
				Modules:   graph.modules,
				Models:    graph.models,
				Documents: graph.documents,
				Search:    graph.search,
			}),
			httpx.RegisterDeepLinkSurface(httpx.DeepLinkDeps{
				Identity:  graph.identity,
				Documents: graph.documents,
				Workflows: graph.workflows,
				Actions:   graph.docActions,
				Audit:     graph.audit,
			}),
			httpx.RegisterNotificationSurface(httpx.NotificationDeps{
				Identity:      graph.identity,
				Notifications: graph.notifications,
				Workflows:     graph.workflows,
				Documents:     graph.documents,
			}),
			httpx.RegisterUISurface(uiDeps),
		},
		FieldSecurity: httpx.FieldSecurityDeps{
			UI:        uiDeps,
			Models:    modelDeps,
			Documents: documentDeps,
		},
		CrossCutting: httpx.CrossCuttingDeps{
			Config:        graph.config,
			Identity:      graph.identity,
			Logger:        graph.logger,
			Observability: graph.observability,
			Health:        graph.runtimeHealth,
			OTel:          graph.otel,
		},
	}
}
