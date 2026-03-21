package app

import (
	"orbyte/internal/platform/httpx"
	"orbyte/internal/platform/mcp"
)

const (
	analyticsMCPStreamPath       = "/mcp/events/analytics/snapshot"
	analyticsScopedMCPStreamPath = "/mcp/analytics/events/analytics/snapshot"
)

func routerDeps(graph *serviceGraph) httpx.RouterDeps {
	return httpx.RouterDeps{
		Platform: httpx.PlatformDeps{
			Config:       graph.config,
			Organization: graph.organization,
			Identity:     graph.identity,
			Reference:    graph.reference,
			Documents:    graph.documents,
			Workflows:    graph.workflows,
		},
		Auth: httpx.AuthDeps{
			Config:   graph.config,
			Identity: graph.identity,
			Audit:    graph.audit,
		},
		Models: httpx.ModelDeps{
			Identity:      graph.identity,
			Models:        graph.models,
			Activities:    graph.activities,
			Policy:        graph.policy,
			FieldSecurity: graph.fieldSecurity,
			Actions:       graph.modelActions,
		},
		Documents: httpx.DocumentDeps{
			Identity:      graph.identity,
			Modules:       graph.modules,
			Documents:     graph.documents,
			Actions:       graph.docActions,
			Policy:        graph.policy,
			FieldSecurity: graph.fieldSecurity,
			Observability: graph.observability,
			Idempotency:   graph.idempotency,
		},
		Ops: httpx.OpsDeps{
			Identity:      graph.identity,
			Audit:         graph.audit,
			Eventing:      graph.eventing,
			Documents:     graph.documents,
			Search:        graph.search,
			Workflows:     graph.workflows,
			Analytics:     graph.analytics,
			Monitoring:    graph.monitoring,
			Observability: graph.observability,
			Integration:   graph.integration,
			Jobs:          graph.jobs,
			Health:        graph.runtimeHealth,
		},
		Search: httpx.SearchDeps{
			Identity: graph.identity,
			Search:   graph.search,
			Jobs:     graph.jobs,
		},
		Admin: httpx.AdminDeps{
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
		},
		Templates: httpx.TemplateDeps{
			Identity:  graph.identity,
			Templates: graph.templates,
		},
		MCP: httpx.MCPDeps{
			Identity:         graph.identity,
			Server:           mcp.NewServer(graph.modules, graph.analytics, graph.templates, graph.workflows, graph.identity, analyticsMCPStreamPath, analyticsScopedMCPStreamPath),
			AnalyticsStream:  graph.mcpAnalytics,
			Analytics:        graph.analytics,
			StreamPath:       analyticsMCPStreamPath,
			ScopedStreamPath: analyticsScopedMCPStreamPath,
		},
		Offline: httpx.OfflineDeps{
			Identity:        graph.identity,
			Modules:         graph.modules,
			Offline:         graph.offline,
			Documents:       graph.documents,
			DocumentActions: graph.docActions,
			Models:          graph.models,
			ModelActions:    graph.modelActions,
			FieldSecurity:   graph.fieldSecurity,
			Idempotency:     graph.idempotency,
		},
		UI: httpx.UIDeps{
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
		},
		CrossCutting: httpx.CrossCuttingDeps{
			Config:        graph.config,
			Identity:      graph.identity,
			Logger:        graph.logger,
			Observability: graph.observability,
			Health:        graph.runtimeHealth,
		},
	}
}
