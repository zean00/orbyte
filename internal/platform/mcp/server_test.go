package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"orbyte/internal/platform/analytics"
	application "orbyte/internal/platform/application"
	"orbyte/internal/platform/audit"
	"orbyte/internal/platform/config"
	"orbyte/internal/platform/dataops"
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/engagement"
	"orbyte/internal/platform/eventing"
	"orbyte/internal/platform/featureflags"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/integration"
	"orbyte/internal/platform/jobs"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/observability"
	"orbyte/internal/platform/offline"
	"orbyte/internal/platform/organization"
	"orbyte/internal/platform/policy"
	"orbyte/internal/platform/reference"
	"orbyte/internal/platform/reporting"
	"orbyte/internal/platform/runtimehealth"
	"orbyte/internal/platform/search"
	"orbyte/internal/platform/securityfields"
	"orbyte/internal/platform/templateoutput"
	"orbyte/internal/platform/workflow"
)

func TestHandleInitializeAndUnknownMethod(t *testing.T) {
	server := newTestServer(t)

	resp := server.Handle(context.Background(), JSONRPCRequest{ID: 1, Method: "initialize"}, ActorContext{})
	if resp.Error != nil {
		t.Fatalf("initialize failed: %+v", resp.Error)
	}
	if resp.JSONRPC != "2.0" {
		t.Fatalf("expected default jsonrpc version, got %q", resp.JSONRPC)
	}
	result := resp.Result.(map[string]any)
	if result["protocolVersion"] != ProtocolVersion {
		t.Fatalf("expected protocol version %q, got %+v", ProtocolVersion, result)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{JSONRPC: "2.0", ID: 2, Method: "does/not/exist"}, ActorContext{})
	if resp.Error == nil || resp.Error.Code != -32601 {
		t.Fatalf("expected method not found error, got %+v", resp.Error)
	}
}

func TestServerToolsAlwaysExposeInputSchema(t *testing.T) {
	server := newTestServer(t)
	resp := server.Handle(context.Background(), JSONRPCRequest{JSONRPC: "2.0", ID: 1, Method: "tools/list"}, ActorContext{
		PermissionChecker: func(string) bool { return true },
	})
	if resp.Error != nil {
		t.Fatalf("tools/list failed: %+v", resp.Error)
	}
	tools := resp.Result.(map[string]any)["tools"].([]ToolDescriptor)
	for _, tool := range tools {
		if tool.InputSchema == nil {
			t.Fatalf("expected inputSchema for %q", tool.Name)
		}
		if schemaType, _ := tool.InputSchema["type"].(string); strings.TrimSpace(schemaType) == "" {
			t.Fatalf("expected schema type for %q, got %+v", tool.Name, tool.InputSchema)
		}
	}
}

func TestServerListsToolsAndResourcesByPermission(t *testing.T) {
	server := newTestServer(t)
	resp := server.Handle(context.Background(), JSONRPCRequest{JSONRPC: "2.0", ID: 1, Method: "tools/list"}, ActorContext{
		PermissionChecker: func(permissionKey string) bool {
			return permissionKey == "analytics.read" || permissionKey == "template.read"
		},
	})
	if resp.Error != nil {
		t.Fatalf("tools/list failed: %+v", resp.Error)
	}
	payload := resp.Result.(map[string]any)
	tools := payload["tools"].([]ToolDescriptor)
	toolNames := make([]string, 0, len(tools))
	for _, tool := range tools {
		toolNames = append(toolNames, tool.Name)
	}
	for _, name := range []string{
		"template.definition.list",
		"template.definition.get",
		"template.draft.get",
		"analytics.snapshot.get",
		"analytics.dashboard.list",
		"analytics.dashboard.widget_catalog",
		"analytics.dashboard.widget.preview",
		"analytics.dashboard.widgets.preview",
		"analytics.dashboard.board.preview",
		"analytics.query.execute",
		"analytics.report.definition.list",
	} {
		if !contains(toolNames, name) {
			t.Fatalf("expected tool %q in %+v", name, toolNames)
		}
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{JSONRPC: "2.0", ID: 2, Method: "resources/list"}, ActorContext{
		PermissionChecker: func(permissionKey string) bool {
			return permissionKey == "analytics.read" || permissionKey == "template.read"
		},
	})
	if resp.Error != nil {
		t.Fatalf("resources/list failed: %+v", resp.Error)
	}
	resources := resp.Result.(map[string]any)["resources"].([]ResourceDescriptor)
	resourceURIs := make([]string, 0, len(resources))
	for _, resource := range resources {
		resourceURIs = append(resourceURIs, resource.URI)
	}
	for _, uri := range []string{
		"orbyte://analytics/snapshot/current",
		"orbyte://apps/analytics.cockpit",
		"orbyte://apps/analytics.studio",
		"orbyte://apps/template.designer",
	} {
		if !contains(resourceURIs, uri) {
			t.Fatalf("expected resource %q in %+v", uri, resourceURIs)
		}
	}
}

func TestServerMinimalExposureListsOnlyMetaToolsAndSupportsPlaybooks(t *testing.T) {
	server := newTestServer(t)
	if err := server.config.Save(config.Entry{
		Key:   "platform.mcp",
		Scope: "deployment",
		Value: map[string]any{
			"enabled":                            true,
			"exposure_mode":                      "minimal",
			"governance_enabled":                 false,
			"default_action_mode":                "draft_only",
			"tool_states_json":                   "{}",
			"blocked_action_classes_json":        "[]",
			"blocked_tool_keys_json":             "[]",
			"blocked_document_types_json":        "[]",
			"allowed_submit_document_types_json": "[]",
			"domain_policy_overrides_json":       "{}",
			"playbooks_json": `[
				{
					"id":"retail_recovery_dashboard",
					"name":"Retail Recovery Dashboard",
					"description":"Use dashboard widgets to explain branch recovery priorities.",
					"domains":["analytics","retail"],
					"labels":["dashboard","recovery"],
					"keywords":["store","branch","performance"],
					"use_when":"The user asks about branch underperformance and wants dashboard evidence.",
					"workflow_steps":["Search the widget catalog","Preview the most relevant widgets","Cite the widget artifact in the answer"],
					"tool_sequence":[
						{"step":"discover_widgets","tool_id":"analytics.dashboard.widget_catalog","required":true,"output":"Candidate widgets"},
						{"step":"preview_widgets","tool_id":"analytics.dashboard.widgets.preview","required":true,"output":"Dashboard artifacts"}
					],
					"tool_ids":["analytics.dashboard.widget_catalog","analytics.dashboard.widgets.preview"],
					"required_final_facts":["benchmark branch","underperforming branches"],
					"required_artifacts":["dashboard_widget artifact blocks"],
					"required_draft_outputs":["generic_request draft id"],
					"guardrails":["do not submit"],
					"success_checks":["names benchmark branch explicitly"],
					"pitfalls":["do not summarize widgets without artifact blocks"]
				}
			]`,
		},
	}); err != nil {
		t.Fatalf("save platform.mcp config failed: %v", err)
	}
	actor := ActorContext{
		ActorID:         "user_admin",
		EffectiveUserID: "user_admin",
		PermissionChecker: func(string) bool {
			return true
		},
	}

	resp := server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/list",
		Params:  mustJSON(t, map[string]any{"exposure_mode": "minimal", "include_summary": true}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("tools/list failed: %+v", resp.Error)
	}
	tools := resp.Result.(map[string]any)["tools"].([]ToolDescriptor)
	names := make([]string, 0, len(tools))
	for _, item := range tools {
		names = append(names, item.Name)
	}
	for _, name := range []string{
		"tools.list",
		"tools.search",
		"tools.describe",
		"tools.call",
		"playbooks.list",
		"playbooks.search",
		"playbooks.describe",
	} {
		if !contains(names, name) {
			t.Fatalf("expected meta tool %q in %+v", name, names)
		}
	}
	if contains(names, "analytics.dashboard.widget_catalog") {
		t.Fatalf("did not expect direct dashboard tool in minimal exposure: %+v", names)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/search",
		Params:  mustJSON(t, map[string]any{"query": "dashboard widget"}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("tools/search failed: %+v", resp.Error)
	}
	searchItems := resp.Result.(map[string]any)["structuredContent"].(map[string]any)["items"].([]toolSummary)
	if len(searchItems) == 0 || searchItems[0].ToolID == "" {
		t.Fatalf("expected discoverable tool summaries, got %+v", searchItems)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "playbooks/list",
	}, actor)
	if resp.Error != nil {
		t.Fatalf("playbooks/list failed: %+v", resp.Error)
	}
	playbookItems := resp.Result.(map[string]any)["structuredContent"].(map[string]any)["items"].([]PlaybookSummary)
	if len(playbookItems) != 1 || playbookItems[0].ID != "retail_recovery_dashboard" {
		t.Fatalf("expected configured playbook, got %+v", playbookItems)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      31,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "playbooks.describe",
			"arguments": map[string]any{
				"playbook_id": "retail_recovery_dashboard",
			},
		}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("playbooks.describe failed: %+v", resp.Error)
	}
	detail := resp.Result.(map[string]any)["structuredContent"].(map[string]any)
	playbook := detail["playbook"].(PlaybookDefinition)
	if len(playbook.ToolSequence) != 2 || playbook.ToolSequence[0].ToolID != "analytics.dashboard.widget_catalog" {
		t.Fatalf("expected structured tool sequence, got %+v", playbook.ToolSequence)
	}
	if !contains(playbook.RequiredFinalFacts, "benchmark branch") {
		t.Fatalf("expected required final facts, got %+v", playbook.RequiredFinalFacts)
	}
	if !contains(playbook.RequiredArtifacts, "dashboard_widget artifact blocks") {
		t.Fatalf("expected required artifacts, got %+v", playbook.RequiredArtifacts)
	}
	if !contains(playbook.RequiredDraftOutputs, "generic_request draft id") {
		t.Fatalf("expected required draft outputs, got %+v", playbook.RequiredDraftOutputs)
	}
	if !contains(playbook.Guardrails, "do not submit") {
		t.Fatalf("expected guardrails, got %+v", playbook.Guardrails)
	}
	if !contains(playbook.SuccessChecks, "names benchmark branch explicitly") {
		t.Fatalf("expected success checks, got %+v", playbook.SuccessChecks)
	}
	if !contains(playbook.Pitfalls, "do not summarize widgets without artifact blocks") {
		t.Fatalf("expected pitfalls, got %+v", playbook.Pitfalls)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      4,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "tools.call",
			"arguments": map[string]any{
				"tool_id": "analytics.dashboard.widget_catalog",
				"payload": map[string]any{"surface": "dashboard"},
			},
		}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("tools.call meta failed: %+v", resp.Error)
	}
	if resp.Result.(map[string]any)["structuredContent"] == nil {
		t.Fatalf("expected structured content from delegated tools.call, got %+v", resp.Result)
	}
}

func TestServerMinimalExposureKeepsMetaToolsVisibleForCapabilityFilteredCompactRequests(t *testing.T) {
	server := newTestServer(t)
	if err := server.config.Save(config.Entry{
		Key:   "platform.mcp",
		Scope: "deployment",
		Value: map[string]any{
			"enabled":                            true,
			"exposure_mode":                      "minimal",
			"governance_enabled":                 false,
			"default_action_mode":                "draft_only",
			"tool_states_json":                   "{}",
			"blocked_action_classes_json":        "[]",
			"blocked_tool_keys_json":             "[]",
			"blocked_document_types_json":        "[]",
			"allowed_submit_document_types_json": "[]",
			"domain_policy_overrides_json":       "{}",
			"playbooks_json":                     "[]",
		},
	}); err != nil {
		t.Fatalf("save platform.mcp config failed: %v", err)
	}
	actor := ActorContext{
		ActorID:         "user_admin",
		EffectiveUserID: "user_admin",
		PermissionChecker: func(string) bool {
			return true
		},
	}
	resp := server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/list",
		Params: mustJSON(t, map[string]any{
			"catalog_mode":          "compact",
			"exposure_mode":         "minimal",
			"capabilities":          []string{"cross_domain_analytics"},
			"include_summary":       true,
			"include_hidden_counts": true,
			"max_tools":             8,
		}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("minimal tools/list failed: %+v", resp.Error)
	}
	tools := resp.Result.(map[string]any)["tools"].([]ToolDescriptor)
	if len(tools) == 0 {
		t.Fatal("expected discovery meta-tools to remain visible in minimal mode")
	}
	if !containsToolNamed(tools, "tools.search") || !containsToolNamed(tools, "tools.call") {
		t.Fatalf("expected discovery meta-tools in %+v", tools)
	}
}

func TestServerMinimalExposureCatalogSummaryUsesUnderlyingDiscoverableToolCount(t *testing.T) {
	server := newTestServer(t)
	if err := server.config.Save(config.Entry{
		Key:   "platform.mcp",
		Scope: "deployment",
		Value: map[string]any{
			"enabled":                            true,
			"exposure_mode":                      "minimal",
			"governance_enabled":                 false,
			"default_action_mode":                "draft_only",
			"tool_states_json":                   "{}",
			"blocked_action_classes_json":        "[]",
			"blocked_tool_keys_json":             "[]",
			"blocked_document_types_json":        "[]",
			"allowed_submit_document_types_json": "[]",
			"domain_policy_overrides_json":       "{}",
			"playbooks_json":                     "[]",
		},
	}); err != nil {
		t.Fatalf("save platform.mcp config failed: %v", err)
	}
	actor := ActorContext{
		ActorID:         "user_admin",
		EffectiveUserID: "user_admin",
		PermissionChecker: func(permissionKey string) bool {
			switch permissionKey {
			case "module.read", "analytics.read", "document.list", "document.read", "document.create", "document.update_draft":
				return true
			default:
				return false
			}
		},
	}
	resp := server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/list",
		Params: mustJSON(t, map[string]any{
			"catalog_mode":          "compact",
			"exposure_mode":         "minimal",
			"capabilities":          []string{"cross_domain_analytics"},
			"include_summary":       true,
			"include_hidden_counts": true,
			"max_tools":             8,
		}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("minimal tools/list failed: %+v", resp.Error)
	}
	payload := resp.Result.(map[string]any)
	catalog := payload["catalog"].(map[string]any)
	totalMatching := int(catalog["total_matching_tools"].(int))
	hiddenTools := int(catalog["hidden_tools"].(int))
	if totalMatching == 0 {
		t.Fatal("expected underlying discoverable catalog count to be non-zero")
	}
	if hiddenTools == 0 {
		t.Fatalf("expected hidden tools count to reflect underlying discoverable tools, got %+v", catalog)
	}
	if hiddenTools != totalMatching {
		t.Fatalf("expected hidden_tools to match the underlying discoverable count in minimal mode, got %+v", catalog)
	}
}

func TestServerFullAndCompactExposureAdvertiseDiscoveryMetaTools(t *testing.T) {
	server := newTestServer(t)
	if err := server.config.Save(config.Entry{
		Key:   "platform.mcp",
		Scope: "deployment",
		Value: map[string]any{
			"enabled":                            true,
			"exposure_mode":                      "full",
			"governance_enabled":                 false,
			"default_action_mode":                "draft_only",
			"tool_states_json":                   "{}",
			"blocked_action_classes_json":        "[]",
			"blocked_tool_keys_json":             "[]",
			"blocked_document_types_json":        "[]",
			"allowed_submit_document_types_json": "[]",
			"domain_policy_overrides_json":       "{}",
			"playbooks_json":                     "[]",
		},
	}); err != nil {
		t.Fatalf("save platform.mcp config failed: %v", err)
	}
	actor := ActorContext{
		ActorID:         "user_admin",
		EffectiveUserID: "user_admin",
		PermissionChecker: func(permissionKey string) bool {
			return true
		},
	}
	for _, tc := range []struct {
		name   string
		params map[string]any
	}{
		{name: "full", params: map[string]any{"exposure_mode": "full"}},
		{name: "compact", params: map[string]any{"exposure_mode": "compact", "catalog_mode": "compact", "capabilities": []string{"discovery"}, "include_summary": true}},
	} {
		resp := server.Handle(context.Background(), JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      1,
			Method:  "tools/list",
			Params:  mustJSON(t, tc.params),
		}, actor)
		if resp.Error != nil {
			t.Fatalf("%s tools/list failed: %+v", tc.name, resp.Error)
		}
		tools := resp.Result.(map[string]any)["tools"].([]ToolDescriptor)
		for _, name := range []string{"tools.search", "tools.describe", "tools.call", "playbooks.search", "playbooks.describe"} {
			if !containsToolNamed(tools, name) {
				t.Fatalf("%s expected discovery meta-tool %q in catalog", tc.name, name)
			}
		}
	}
}

func TestServerAnalyticsAuthoringAndAdHocQueryFlow(t *testing.T) {
	server := newTestServer(t)
	actor := ActorContext{
		ActorID:         "user_admin",
		EffectiveUserID: "user_admin",
		LocationID:      "loc_hq",
		PermissionChecker: func(permissionKey string) bool {
			switch permissionKey {
			case "analytics.read", "analytics.author", "analytics.manage_reports", "analytics.deliver_reports":
				return true
			default:
				return false
			}
		},
	}
	if _, err := server.analytics.CaptureSnapshot(); err != nil {
		t.Fatalf("capture analytics snapshot failed: %v", err)
	}

	resp := server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "analytics.query.execute",
			"arguments": map[string]any{
				"query": map[string]any{
					"source_kind": "snapshot",
					"measures":    []string{"submitted", "approved"},
				},
			},
		}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("analytics.query.execute failed: %+v", resp.Error)
	}
	execResult := resp.Result.(map[string]any)
	if execResult["_meta"] == nil {
		t.Fatal("expected analytics execution app metadata")
	}
	structured := execResult["structuredContent"].(map[string]any)
	result := structured["result"].(analytics.QueryExecution)
	if result.Chart.Type == "" || len(result.Rows) != 1 {
		t.Fatalf("expected ad hoc result and chart, got %+v", result)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "analytics.query.save",
			"arguments": map[string]any{
				"query": map[string]any{
					"name": "Current Performance",
					"spec": map[string]any{
						"source_kind": "snapshot",
						"measures":    []string{"submitted", "approved"},
					},
				},
			},
		}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("analytics.query.save failed: %+v", resp.Error)
	}
	savedQuery := resp.Result.(map[string]any)["structuredContent"].(analytics.SavedQuery)
	if savedQuery.ID == "" {
		t.Fatalf("expected saved query id, got %+v", savedQuery)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "analytics.dashboard.save",
			"arguments": map[string]any{
				"dashboard": map[string]any{
					"name": "Sales Performance",
					"widgets": []map[string]any{{
						"title":    "Current Sales",
						"kind":     "chart",
						"query_id": savedQuery.ID,
					}},
				},
			},
		}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("analytics.dashboard.save failed: %+v", resp.Error)
	}
	savedDashboard := resp.Result.(map[string]any)["structuredContent"].(map[string]any)["dashboard"].(analytics.Dashboard)
	if savedDashboard.ID == "" || len(savedDashboard.Widgets) != 1 {
		t.Fatalf("expected saved dashboard with widget, got %+v", savedDashboard)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      4,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "analytics.report.definition.save",
			"arguments": map[string]any{
				"report": map[string]any{
					"name":      "Current Sales Report",
					"dimension": "document_type",
					"format":    "csv",
				},
			},
		}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("analytics.report.definition.save failed: %+v", resp.Error)
	}
	reportDef := resp.Result.(map[string]any)["structuredContent"].(analytics.ReportDefinition)
	if reportDef.ID == "" {
		t.Fatalf("expected report definition id, got %+v", reportDef)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      5,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "analytics.report.run", "arguments": map[string]any{"report_id": reportDef.ID}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("analytics.report.run failed: %+v", resp.Error)
	}
	runPayload := resp.Result.(map[string]any)["structuredContent"].(map[string]any)
	run := runPayload["run"].(analytics.ReportRun)
	if run.ArtifactID == "" {
		t.Fatalf("expected report run artifact, got %+v", run)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      6,
		Method:  "resources/read",
		Params:  mustJSON(t, map[string]any{"uri": "orbyte://apps/analytics.studio?kind=dashboard&id=" + savedDashboard.ID}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("analytics studio resource failed: %+v", resp.Error)
	}
	contents := resp.Result.(map[string]any)["contents"].([]ResourceContent)
	if len(contents) != 1 || !strings.Contains(contents[0].Text, "Analytics Studio") {
		t.Fatalf("expected analytics studio app html, got %+v", contents)
	}
}

func TestServerMinimalToolSearchFindsDedicatedCRMTools(t *testing.T) {
	server := newTestServer(t)
	if err := server.modules.Register(module.Manifest{
		Key: "crm_core",
		MCP: module.MCPDefinition{
			Tools: []module.MCPToolDefinition{
				{Key: "crm.ticket.summary", Title: "Get CRM Ticket Summary", Description: "Summarize CRM service backlog and queue health.", Operation: "crm.ticket.summary", RequiredPermissions: []string{"crm_ticket.list"}, Contract: module.MCPContractMetadata{BusinessDomains: []string{"crm", "service"}}},
				{Key: "crm.customer.health", Title: "Get CRM Customer Health", Description: "Summarize customer health and at-risk accounts.", Operation: "crm.customer.health", RequiredPermissions: []string{"crm_ticket.list"}, Contract: module.MCPContractMetadata{BusinessDomains: []string{"crm", "service", "sales"}}},
				{Key: "crm.opportunity.pipeline.summary", Title: "Get Opportunity Pipeline Summary", Description: "Summarize the active CRM sales pipeline.", Operation: "crm.opportunity.pipeline.summary", RequiredPermissions: []string{"crm_opportunity.list"}, Contract: module.MCPContractMetadata{BusinessDomains: []string{"crm", "sales"}}},
			},
		},
	}, "system"); err != nil {
		t.Fatalf("register crm test manifest failed: %v", err)
	}
	if err := server.config.Save(config.Entry{
		Key:   "platform.mcp",
		Scope: "deployment",
		Value: map[string]any{
			"enabled":                            true,
			"exposure_mode":                      "minimal",
			"governance_enabled":                 false,
			"default_action_mode":                "draft_only",
			"tool_states_json":                   "{}",
			"blocked_action_classes_json":        "[]",
			"blocked_tool_keys_json":             "[]",
			"blocked_document_types_json":        "[]",
			"allowed_submit_document_types_json": "[]",
			"domain_policy_overrides_json":       "{}",
			"playbooks_json": `[
				{
					"id":"crm_service_sales_overview",
					"name":"CRM Service and Sales Overview",
					"description":"Summarize CRM ticket backlog, customer health, and active pipeline.",
					"domains":["crm","service","sales"],
					"labels":["crm","backlog","pipeline"]
				}
			]`,
		},
	}); err != nil {
		t.Fatalf("save platform.mcp config failed: %v", err)
	}
	actor := ActorContext{
		ActorID:         "user_admin",
		EffectiveUserID: "user_admin",
		PermissionChecker: func(string) bool {
			return true
		},
	}

	resp := server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "tools.search",
			"arguments": map[string]any{
				"query":  "crm backlog customer health active pipeline",
				"domain": "crm",
				"limit":  10,
			},
		}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("tools.search failed: %+v", resp.Error)
	}
	structured := resp.Result.(map[string]any)["structuredContent"].(map[string]any)
	rawItems := structured["items"].([]toolSummary)
	if len(rawItems) == 0 {
		t.Fatal("expected CRM search results")
	}
	names := make([]string, 0, len(rawItems))
	for _, item := range rawItems {
		names = append(names, item.ToolID)
	}
	for _, name := range []string{"crm.ticket.summary", "crm.customer.health", "crm.opportunity.pipeline.summary"} {
		if !contains(names, name) {
			t.Fatalf("expected dedicated CRM tool %q in %+v", name, names)
		}
	}

	playbookResp := server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "playbooks.search",
			"arguments": map[string]any{
				"query": "crm backlog pipeline",
				"limit": 5,
			},
		}),
	}, actor)
	if playbookResp.Error != nil {
		t.Fatalf("playbooks.search failed: %+v", playbookResp.Error)
	}
	playbookStructured := playbookResp.Result.(map[string]any)["structuredContent"].(map[string]any)
	playbooks := playbookStructured["items"].([]PlaybookSummary)
	if len(playbooks) == 0 || playbooks[0].ID != "crm_service_sales_overview" {
		t.Fatalf("expected CRM playbook search result, got %+v", playbooks)
	}
}

func TestServerDataOpsFlow(t *testing.T) {
	server := newTestServer(t)
	actor := ActorContext{
		ActorID: "user_admin",
		PermissionChecker: func(permissionKey string) bool {
			return permissionKey == "configuration.read" || permissionKey == "configuration.manage"
		},
	}

	resp := server.Handle(context.Background(), JSONRPCRequest{JSONRPC: "2.0", ID: 1, Method: "tools/list"}, actor)
	if resp.Error != nil {
		t.Fatalf("tools/list failed: %+v", resp.Error)
	}
	tools := resp.Result.(map[string]any)["tools"].([]ToolDescriptor)
	names := make([]string, 0, len(tools))
	for _, item := range tools {
		names = append(names, item.Name)
	}
	for _, name := range []string{"dataops.backup.plan", "dataops.restore.run", "dataops.migration.register"} {
		if !contains(names, name) {
			t.Fatalf("expected tool %s in %v", name, names)
		}
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "dataops.backup.plan",
			"arguments": map[string]any{
				"selected_data_classes": []string{"configuration"},
			},
		}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("backup plan failed: %+v", resp.Error)
	}
	plan := resp.Result.(map[string]any)["structuredContent"].(dataops.BackupPlan)
	if plan.ArtifactType != dataops.ArtifactTypeBackup {
		t.Fatalf("expected backup artifact type, got %+v", plan.ArtifactType)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "dataops.backup.run",
			"arguments": map[string]any{
				"selected_data_classes": []string{"configuration"},
				"confirm_apply":         true,
			},
		}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("backup run failed: %+v", resp.Error)
	}
	result := resp.Result.(map[string]any)["structuredContent"].(map[string]any)
	run := result["run"].(dataops.OperationRun)
	job := result["job"].(jobs.Job)
	if strings.TrimSpace(run.ID) == "" || strings.TrimSpace(job.ID) == "" {
		t.Fatal("expected queued run and job from backup run")
	}
}

func TestBusinessModuleToolsAndSyntheticWrappers(t *testing.T) {
	server := newTestServer(t)
	if err := server.documents.Register(document.Definition{
		Type:          "promotion_plan",
		DisplayName:   "Promotion Plan",
		SchemaVersion: "v1",
	}); err != nil {
		t.Fatalf("register promotion_plan failed: %v", err)
	}
	if err := server.modules.Register(module.Manifest{
		Key:                  "pricing",
		Name:                 "Pricing",
		Description:          "Pricing and promotion setup",
		DomainFamily:         "commercial",
		Category:             "pricing",
		BusinessCapabilities: []string{"discounting", "promotions"},
		DependencyRequirements: []module.DependencyRequirement{{
			ModuleKey: "analytics",
			Kind:      module.DependencyKindOptional,
		}},
		OwnedDocumentTypes: []string{"promotion_plan"},
		Documents: []document.Definition{{
			Type:          "promotion_plan",
			DisplayName:   "Promotion Plan",
			SchemaVersion: "v1",
		}},
	}, "system"); err != nil {
		t.Fatalf("register pricing manifest failed: %v", err)
	}
	actor := ActorContext{
		ActorID: "user_admin",
		PermissionChecker: func(permissionKey string) bool {
			switch permissionKey {
			case "module.read", "document.list", "document.read", "document.create", "document.update_draft":
				return true
			default:
				return false
			}
		},
	}

	resp := server.Handle(context.Background(), JSONRPCRequest{JSONRPC: "2.0", ID: 1, Method: "tools/list"}, actor)
	if resp.Error != nil {
		t.Fatalf("tools/list failed: %+v", resp.Error)
	}
	tools := resp.Result.(map[string]any)["tools"].([]ToolDescriptor)
	names := make([]string, 0, len(tools))
	for _, item := range tools {
		names = append(names, item.Name)
	}
	for _, expected := range []string{"business.module.list", "business.document.draft.create", "pricing.business.info.get", "pricing.business.document.draft.create"} {
		if !contains(names, expected) {
			t.Fatalf("expected tool %q in %+v", expected, names)
		}
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "business.module.get", "arguments": map[string]any{"module_key": "pricing"}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("business.module.get failed: %+v", resp.Error)
	}
	info := resp.Result.(map[string]any)["structuredContent"].(businessModuleInfo)
	if info.Key != "pricing" || len(info.OwnedDocumentTypes) != 1 || info.OwnedDocumentTypes[0] != "promotion_plan" {
		t.Fatalf("unexpected business module info: %+v", info)
	}
	if len(info.BusinessCapabilities) != 2 {
		t.Fatalf("expected capabilities, got %+v", info)
	}
}

func TestBusinessDocumentDraftCreateRequiresConfirmationAndSupportsSyntheticModuleTool(t *testing.T) {
	server := newTestServer(t)
	if err := server.documents.Register(document.Definition{
		Type:          "promotion_plan",
		DisplayName:   "Promotion Plan",
		SchemaVersion: "v1",
	}); err != nil {
		t.Fatalf("register promotion_plan failed: %v", err)
	}
	if err := server.modules.Register(module.Manifest{
		Key:                "pricing",
		Name:               "Pricing",
		DomainFamily:       "commercial",
		Category:           "pricing",
		OwnedDocumentTypes: []string{"promotion_plan"},
		Documents: []document.Definition{{
			Type:          "promotion_plan",
			DisplayName:   "Promotion Plan",
			SchemaVersion: "v1",
		}},
	}, "system"); err != nil {
		t.Fatalf("register pricing manifest failed: %v", err)
	}
	actor := ActorContext{
		ActorID:         "user_admin",
		EffectiveUserID: "user_admin",
		LocationID:      "loc_hq",
		PermissionChecker: func(permissionKey string) bool {
			switch permissionKey {
			case "module.read", "document.create", "document.list", "document.read", "document.update_draft":
				return true
			default:
				return false
			}
		},
	}

	resp := server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "business.document.draft.create",
			"arguments": map[string]any{
				"module_key":      "pricing",
				"document_type":   "promotion_plan",
				"location_id":     "loc_hq",
				"organization_id": "org_default",
				"payload":         map[string]any{"name": "Ramadan Promo"},
			},
		}),
	}, actor)
	if resp.Error == nil || !strings.Contains(resp.Error.Message, "confirm_apply") {
		t.Fatalf("expected confirm_apply validation error, got %+v", resp.Error)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "pricing.business.document.draft.create",
			"arguments": map[string]any{
				"document_type":   "promotion_plan",
				"location_id":     "loc_hq",
				"organization_id": "org_default",
				"payload":         map[string]any{"name": "Ramadan Promo"},
				"confirm_apply":   true,
			},
		}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("synthetic business draft create failed: %+v", resp.Error)
	}
	structured := resp.Result.(map[string]any)["structuredContent"].(map[string]any)
	record := structured["record"].(document.Record)
	if record.Header.Type != "promotion_plan" || record.Header.Status != "draft" {
		t.Fatalf("expected promotion draft, got %+v", record)
	}
	expectedOpenPath := "/ui/documents/detail?id=" + url.QueryEscape(record.Header.ID)
	if structured["document_id"] != record.Header.ID {
		t.Fatalf("expected document_id in structured content, got %+v", structured)
	}
	if structured["open_path"] != expectedOpenPath {
		t.Fatalf("expected open_path %s, got %+v", expectedOpenPath, structured["open_path"])
	}
	content := resp.Result.(map[string]any)["content"].([]ContentBlock)
	if len(content) == 0 || !strings.Contains(content[0].Text, expectedOpenPath) {
		t.Fatalf("expected draft response text to include deep link, got %+v", content)
	}
}

func TestBusinessModelReadIsSanitized(t *testing.T) {
	server := newTestServer(t)
	if err := server.models.Register(model.Definition{
		Key:                 "discount_policy",
		DisplayName:         "Discount Policy",
		OwnerModuleKey:      "pricing",
		ListPermissionKey:   "pricing.policy.list",
		ReadPermissionKey:   "pricing.policy.read",
		CreatePermissionKey: "pricing.policy.create",
		UpdatePermissionKey: "pricing.policy.update",
		Fields: []model.FieldDefinition{
			{Key: "name", Type: "string", Label: "Name"},
			{Key: "internal_formula", Type: "string", Label: "Internal Formula", ReadPermissionKey: "pricing.policy.formula.read"},
		},
	}); err != nil {
		t.Fatalf("register model failed: %v", err)
	}
	if _, err := server.models.Create("discount_policy", "user_admin", map[string]any{
		"name":             "VIP Discount",
		"internal_formula": "margin > 10%",
	}); err != nil {
		t.Fatalf("create model record failed: %v", err)
	}
	actor := ActorContext{
		ActorID: "user_admin",
		PermissionChecker: func(permissionKey string) bool {
			switch permissionKey {
			case "module.read", "pricing.policy.list", "pricing.policy.read":
				return true
			default:
				return false
			}
		},
	}

	resp := server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "business.record.search",
			"arguments": map[string]any{
				"resource_kind":        "model",
				"model_key":            "discount_policy",
				"include_full_payload": true,
			},
		}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("business.record.search failed: %+v", resp.Error)
	}
	items := resp.Result.(map[string]any)["structuredContent"].(map[string]any)["items"].([]businessRecordSummary)
	if len(items) != 1 {
		t.Fatalf("expected one model record, got %+v", items)
	}
	values := items[0].Record["record"].(model.Record).Values
	if _, ok := values["internal_formula"]; ok {
		t.Fatalf("expected internal_formula to be sanitized, got %+v", values)
	}
}

func TestPOSPromotionStrategyTools(t *testing.T) {
	server := newTestServer(t)
	modelDefs := []model.Definition{
		{Key: "commercial_item", DisplayName: "Commercial Item", OwnerModuleKey: "pos_core", ListPermissionKey: "commercial_item.list", ReadPermissionKey: "commercial_item.read", Fields: []model.FieldDefinition{{Key: "sku", Type: "string"}, {Key: "name", Type: "string"}, {Key: "status", Type: "string"}}},
		{Key: "customer_profile", DisplayName: "Customer Profile", OwnerModuleKey: "pos_core", ListPermissionKey: "customer_profile.list", ReadPermissionKey: "customer_profile.read", Fields: []model.FieldDefinition{{Key: "party_id", Type: "string"}, {Key: "customer_name", Type: "string"}, {Key: "member_tier", Type: "string"}, {Key: "member_status", Type: "string"}, {Key: "status", Type: "string"}}},
		{Key: "pos_sale", DisplayName: "POS Sale", OwnerModuleKey: "pos_core", ListPermissionKey: "pos_sale.list", ReadPermissionKey: "pos_sale.read", Fields: []model.FieldDefinition{{Key: "store_code", Type: "string"}, {Key: "register_code", Type: "string"}, {Key: "status", Type: "string"}, {Key: "party_id", Type: "string"}, {Key: "total_amount", Type: "number"}, {Key: "lines_json", Type: "string"}}},
		{Key: "promotion_campaign", DisplayName: "Promotion Campaign", OwnerModuleKey: "promotion_core", ListPermissionKey: "promotion_campaign.list", ReadPermissionKey: "promotion_campaign.read", Fields: []model.FieldDefinition{{Key: "code", Type: "string"}, {Key: "name", Type: "string"}, {Key: "status", Type: "string"}, {Key: "trigger_mode", Type: "string"}}},
		{Key: "promotion_code", DisplayName: "Promotion Code", OwnerModuleKey: "promotion_core", ListPermissionKey: "promotion_code.list", ReadPermissionKey: "promotion_code.read", Fields: []model.FieldDefinition{{Key: "code", Type: "string"}, {Key: "promotion_campaign_code", Type: "string"}, {Key: "status", Type: "string"}}},
		{Key: "promotion_redemption", DisplayName: "Promotion Redemption", OwnerModuleKey: "promotion_core", ListPermissionKey: "promotion_redemption.list", ReadPermissionKey: "promotion_redemption.read", Fields: []model.FieldDefinition{{Key: "promotion_campaign_code", Type: "string"}, {Key: "promotion_code", Type: "string"}, {Key: "store_code", Type: "string"}, {Key: "discount_amount_total", Type: "number"}, {Key: "status", Type: "string"}}},
		{Key: "discount_rule", DisplayName: "Discount Rule", OwnerModuleKey: "promotion_core", ListPermissionKey: "discount_rule.list", ReadPermissionKey: "discount_rule.read", Fields: []model.FieldDefinition{{Key: "code", Type: "string"}, {Key: "name", Type: "string"}, {Key: "promotion_campaign_code", Type: "string"}, {Key: "item_codes", Type: "string"}, {Key: "status", Type: "string"}}},
	}
	if err := server.modules.Register(module.Manifest{
		Key:    "pos_core",
		Name:   "POS Core",
		Models: []model.Definition{modelDefs[0], modelDefs[1], modelDefs[2]},
	}, "system"); err != nil {
		t.Fatalf("register pos_core manifest failed: %v", err)
	}
	if err := server.modules.Register(module.Manifest{
		Key:    "promotion_core",
		Name:   "Promotion Core",
		Models: []model.Definition{modelDefs[3], modelDefs[4], modelDefs[5], modelDefs[6]},
	}, "system"); err != nil {
		t.Fatalf("register promotion_core manifest failed: %v", err)
	}
	for _, def := range modelDefs {
		if err := server.models.Register(def); err != nil {
			t.Fatalf("register model %s failed: %v", def.Key, err)
		}
	}
	create := func(modelKey string, values map[string]any) {
		t.Helper()
		if _, err := server.models.Create(modelKey, "user_admin", values); err != nil {
			t.Fatalf("create %s failed: %v", modelKey, err)
		}
	}
	create("commercial_item", map[string]any{"sku": "ESP-1", "name": "Espresso Double", "status": "active"})
	create("commercial_item", map[string]any{"sku": "CRO-1", "name": "Butter Croissant", "status": "active"})
	create("commercial_item", map[string]any{"sku": "BEAN-1", "name": "House Beans 1kg", "status": "active"})
	create("customer_profile", map[string]any{"party_id": "party-gold-1", "customer_name": "Alya Santoso", "member_tier": "gold", "member_status": "active", "status": "active"})
	create("customer_profile", map[string]any{"party_id": "party-gold-2", "customer_name": "Bima Pratama", "member_tier": "gold", "member_status": "active", "status": "active"})
	create("customer_profile", map[string]any{"party_id": "party-silver-1", "customer_name": "Citra Lestari", "member_tier": "silver", "member_status": "active", "status": "active"})
	for _, sale := range []map[string]any{
		{"store_code": "PROMO-STORE", "register_code": "PROMO-REG", "status": "completed", "party_id": "party-gold-1", "total_amount": 50000.0, "lines_json": `[{"item_code":"ESP-1","quantity":1,"line_total":28000},{"item_code":"CRO-1","quantity":1,"line_total":22000}]`},
		{"store_code": "PROMO-STORE", "register_code": "PROMO-REG", "status": "completed", "party_id": "party-gold-2", "total_amount": 50000.0, "lines_json": `[{"item_code":"ESP-1","quantity":1,"line_total":28000},{"item_code":"CRO-1","quantity":1,"line_total":22000}]`},
		{"store_code": "PROMO-STORE", "register_code": "PROMO-REG", "status": "completed", "party_id": "party-gold-1", "total_amount": 85500.0, "lines_json": `[{"item_code":"BEAN-1","quantity":1,"line_total":85500}]`},
		{"store_code": "PROMO-STORE", "register_code": "PROMO-REG", "status": "completed", "party_id": "party-silver-1", "total_amount": 18000.0, "lines_json": `[{"item_code":"ESP-1","quantity":1,"line_total":18000}]`},
	} {
		create("pos_sale", sale)
	}
	create("promotion_campaign", map[string]any{"code": "BEANS-BOOST", "name": "Beans Boost", "status": "active", "trigger_mode": "code"})
	create("promotion_code", map[string]any{"code": "BEANS10", "promotion_campaign_code": "BEANS-BOOST", "status": "active"})
	create("discount_rule", map[string]any{"code": "BEANS-RULE", "name": "Beans 10 Percent", "promotion_campaign_code": "BEANS-BOOST", "item_codes": "BEAN-1", "status": "active"})
	create("promotion_redemption", map[string]any{"promotion_campaign_code": "BEANS-BOOST", "promotion_code": "BEANS10", "store_code": "PROMO-STORE", "discount_amount_total": 9500.0, "status": "active"})
	actor := ActorContext{
		ActorID:         "user_admin",
		EffectiveUserID: "user_admin",
		LocationID:      "loc_hq",
		PermissionChecker: func(permissionKey string) bool {
			switch permissionKey {
			case "module.read", "document.create", "commercial_item.list", "customer_profile.list", "pos_sale.list", "promotion_campaign.list", "promotion_code.list", "promotion_redemption.list", "discount_rule.list":
				return true
			default:
				return false
			}
		},
	}
	listResp := server.Handle(context.Background(), JSONRPCRequest{JSONRPC: "2.0", ID: 1, Method: "tools/list"}, actor)
	if listResp.Error != nil {
		t.Fatalf("tools/list failed: %+v", listResp.Error)
	}
	tools := listResp.Result.(map[string]any)["tools"].([]ToolDescriptor)
	for _, expected := range []string{"pos_core.sales.strategy.summary", "promotion_core.performance.summary", "promotion_core.strategy.plan.draft.create"} {
		if !containsToolNamed(tools, expected) {
			t.Fatalf("expected tool %s to be listed", expected)
		}
	}
	salesResp := server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: mustJSON(t, map[string]any{"name": "pos_core.sales.strategy.summary", "arguments": map[string]any{"store_code": "PROMO-STORE"}}),
	}, actor)
	if salesResp.Error != nil {
		t.Fatalf("pos strategy summary failed: %+v", salesResp.Error)
	}
	salesStructured := salesResp.Result.(map[string]any)["structuredContent"].(map[string]any)
	recommendation := salesStructured["recommendation_signal"].(map[string]any)
	if recommendation["target_segment"] != "gold members" {
		t.Fatalf("expected gold members recommendation, got %+v", recommendation)
	}
	targetProducts := recommendation["target_products"].([]string)
	if len(targetProducts) != 2 || targetProducts[0] != "Butter Croissant" && targetProducts[1] != "Butter Croissant" {
		t.Fatalf("expected espresso/croissant recommendation, got %+v", targetProducts)
	}
	promoResp := server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0", ID: 3, Method: "tools/call",
		Params: mustJSON(t, map[string]any{"name": "promotion_core.performance.summary", "arguments": map[string]any{"store_code": "PROMO-STORE"}}),
	}, actor)
	if promoResp.Error != nil {
		t.Fatalf("promotion performance summary failed: %+v", promoResp.Error)
	}
	underperforming := promoResp.Result.(map[string]any)["structuredContent"].(map[string]any)["underperforming_signal"].(map[string]any)
	if underperforming["campaign_name"] != "Beans Boost" {
		t.Fatalf("expected Beans Boost underperforming signal, got %+v", underperforming)
	}
	if int(numberValue(underperforming["redemption_count"])) != 1 {
		t.Fatalf("expected one redemption, got %+v", underperforming)
	}
	draftResp := server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0", ID: 4, Method: "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "promotion_core.strategy.plan.draft.create",
			"arguments": map[string]any{
				"title":             "Promotion Plan Test",
				"summary":           "Bundle espresso and croissant for gold members. Replace Beans Boost.",
				"target_products":   []string{"Espresso Double", "Butter Croissant"},
				"target_segment":    "gold members",
				"replaced_campaign": "Beans Boost",
				"confirm_apply":     true,
			},
		}),
	}, actor)
	if draftResp.Error != nil {
		t.Fatalf("promotion draft create failed: %+v", draftResp.Error)
	}
	structured := draftResp.Result.(map[string]any)["structuredContent"].(map[string]any)
	record := structured["record"].(document.Record)
	if record.Header.Type != "generic_request" || stringValue(record.Body.Payload["title"]) != "Promotion Plan Test" {
		t.Fatalf("expected generic_request promotion plan draft, got %+v", record)
	}
	if stringValue(record.Body.Payload["request_kind"]) != "promotion_plan" || stringValue(record.Body.Payload["viewer_hint"]) != "promotion.plan" {
		t.Fatalf("expected promotion plan routing payload, got %+v", record.Body.Payload)
	}
	expectedOpenPath := "/ui/promotion/plans/form?id=" + url.QueryEscape(record.Header.ID)
	if structured["open_path"] != expectedOpenPath {
		t.Fatalf("expected promotion draft open_path %s, got %+v", expectedOpenPath, structured["open_path"])
	}
}

func TestPOSPromotionStrategySummaryTargetsComboAudience(t *testing.T) {
	server := newTestServer(t)
	modelDefs := []model.Definition{
		{Key: "commercial_item", DisplayName: "Commercial Item", OwnerModuleKey: "pos_core", ListPermissionKey: "commercial_item.list", ReadPermissionKey: "commercial_item.read", Fields: []model.FieldDefinition{{Key: "sku", Type: "string"}, {Key: "name", Type: "string"}, {Key: "status", Type: "string"}}},
		{Key: "customer_profile", DisplayName: "Customer Profile", OwnerModuleKey: "pos_core", ListPermissionKey: "customer_profile.list", ReadPermissionKey: "customer_profile.read", Fields: []model.FieldDefinition{{Key: "party_id", Type: "string"}, {Key: "customer_name", Type: "string"}, {Key: "member_tier", Type: "string"}, {Key: "member_status", Type: "string"}, {Key: "status", Type: "string"}}},
		{Key: "pos_sale", DisplayName: "POS Sale", OwnerModuleKey: "pos_core", ListPermissionKey: "pos_sale.list", ReadPermissionKey: "pos_sale.read", Fields: []model.FieldDefinition{{Key: "store_code", Type: "string"}, {Key: "register_code", Type: "string"}, {Key: "status", Type: "string"}, {Key: "party_id", Type: "string"}, {Key: "total_amount", Type: "number"}, {Key: "lines_json", Type: "string"}}},
	}
	if err := server.modules.Register(module.Manifest{
		Key:    "pos_core",
		Name:   "POS Core",
		Models: []model.Definition{modelDefs[0], modelDefs[1], modelDefs[2]},
	}, "system"); err != nil {
		t.Fatalf("register pos_core manifest failed: %v", err)
	}
	for _, def := range modelDefs {
		if err := server.models.Register(def); err != nil {
			t.Fatalf("register model %s failed: %v", def.Key, err)
		}
	}
	create := func(modelKey string, values map[string]any) {
		t.Helper()
		if _, err := server.models.Create(modelKey, "user_admin", values); err != nil {
			t.Fatalf("create %s failed: %v", modelKey, err)
		}
	}
	create("commercial_item", map[string]any{"sku": "ESP-1", "name": "Espresso Double", "status": "active"})
	create("commercial_item", map[string]any{"sku": "CRO-1", "name": "Butter Croissant", "status": "active"})
	create("commercial_item", map[string]any{"sku": "BEAN-1", "name": "House Beans 1kg", "status": "active"})
	create("customer_profile", map[string]any{"party_id": "party-gold-1", "customer_name": "Gold One", "member_tier": "gold", "member_status": "active", "status": "active"})
	create("customer_profile", map[string]any{"party_id": "party-gold-2", "customer_name": "Gold Two", "member_tier": "gold", "member_status": "active", "status": "active"})
	create("customer_profile", map[string]any{"party_id": "party-gold-3", "customer_name": "Gold Three", "member_tier": "gold", "member_status": "active", "status": "active"})
	create("customer_profile", map[string]any{"party_id": "party-silver-1", "customer_name": "Silver One", "member_tier": "silver", "member_status": "active", "status": "active"})
	create("customer_profile", map[string]any{"party_id": "party-silver-2", "customer_name": "Silver Two", "member_tier": "silver", "member_status": "active", "status": "active"})
	create("customer_profile", map[string]any{"party_id": "party-silver-3", "customer_name": "Silver Three", "member_tier": "silver", "member_status": "active", "status": "active"})
	create("customer_profile", map[string]any{"party_id": "party-silver-4", "customer_name": "Silver Four", "member_tier": "silver", "member_status": "active", "status": "active"})
	for _, sale := range []map[string]any{
		{"store_code": "PROMO-STORE-MIXED", "register_code": "PROMO-REG", "status": "completed", "party_id": "party-gold-1", "total_amount": 50000.0, "lines_json": `[{"item_code":"ESP-1","quantity":1,"line_total":28000},{"item_code":"CRO-1","quantity":1,"line_total":22000}]`},
		{"store_code": "PROMO-STORE-MIXED", "register_code": "PROMO-REG", "status": "completed", "party_id": "party-gold-2", "total_amount": 50000.0, "lines_json": `[{"item_code":"ESP-1","quantity":1,"line_total":28000},{"item_code":"CRO-1","quantity":1,"line_total":22000}]`},
		{"store_code": "PROMO-STORE-MIXED", "register_code": "PROMO-REG", "status": "completed", "party_id": "party-gold-3", "total_amount": 50000.0, "lines_json": `[{"item_code":"ESP-1","quantity":1,"line_total":28000},{"item_code":"CRO-1","quantity":1,"line_total":22000}]`},
		{"store_code": "PROMO-STORE-MIXED", "register_code": "PROMO-REG", "status": "completed", "party_id": "party-silver-1", "total_amount": 113500.0, "lines_json": `[{"item_code":"ESP-1","quantity":1,"line_total":28000},{"item_code":"BEAN-1","quantity":1,"line_total":85500}]`},
		{"store_code": "PROMO-STORE-MIXED", "register_code": "PROMO-REG", "status": "completed", "party_id": "party-silver-2", "total_amount": 113500.0, "lines_json": `[{"item_code":"ESP-1","quantity":1,"line_total":28000},{"item_code":"BEAN-1","quantity":1,"line_total":85500}]`},
		{"store_code": "PROMO-STORE-MIXED", "register_code": "PROMO-REG", "status": "completed", "party_id": "party-silver-3", "total_amount": 107500.0, "lines_json": `[{"item_code":"CRO-1","quantity":1,"line_total":22000},{"item_code":"BEAN-1","quantity":1,"line_total":85500}]`},
		{"store_code": "PROMO-STORE-MIXED", "register_code": "PROMO-REG", "status": "completed", "party_id": "party-silver-4", "total_amount": 107500.0, "lines_json": `[{"item_code":"CRO-1","quantity":1,"line_total":22000},{"item_code":"BEAN-1","quantity":1,"line_total":85500}]`},
	} {
		create("pos_sale", sale)
	}
	actor := ActorContext{
		ActorID:         "user_admin",
		EffectiveUserID: "user_admin",
		LocationID:      "loc_hq",
		PermissionChecker: func(permissionKey string) bool {
			switch permissionKey {
			case "module.read", "commercial_item.list", "customer_profile.list", "pos_sale.list":
				return true
			default:
				return false
			}
		},
	}
	resp := server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: mustJSON(t, map[string]any{"name": "pos_core.sales.strategy.summary", "arguments": map[string]any{"store_code": "PROMO-STORE-MIXED"}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("pos strategy summary failed: %+v", resp.Error)
	}
	recommendation := resp.Result.(map[string]any)["structuredContent"].(map[string]any)["recommendation_signal"].(map[string]any)
	if recommendation["target_segment"] != "gold members" {
		t.Fatalf("expected combo audience to drive recommendation, got %+v", recommendation)
	}
}

func TestBusinessRecordSearchValidationSuggestsStrategyTools(t *testing.T) {
	server := newTestServer(t)
	resp := server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: mustJSON(t, map[string]any{"name": "business.record.search", "arguments": map[string]any{"resource_kind": "sales"}}),
	}, ActorContext{ActorID: "user_admin", PermissionChecker: func(permissionKey string) bool { return permissionKey == "module.read" }})
	if resp.Error == nil || !strings.Contains(resp.Error.Message, "pos_core.sales.strategy.summary") {
		t.Fatalf("expected strategy summary guidance in validation error, got %+v", resp.Error)
	}
}

func TestBusinessTimelineGetReturnsModelAuditEvents(t *testing.T) {
	server := newTestServer(t)
	if err := server.models.Register(model.Definition{
		Key:                 "discount_policy",
		DisplayName:         "Discount Policy",
		OwnerModuleKey:      "pricing",
		ListPermissionKey:   "pricing.policy.list",
		ReadPermissionKey:   "pricing.policy.read",
		CreatePermissionKey: "pricing.policy.create",
		UpdatePermissionKey: "pricing.policy.update",
		Fields: []model.FieldDefinition{
			{Key: "name", Type: "string", Label: "Name"},
		},
	}); err != nil {
		t.Fatalf("register model failed: %v", err)
	}
	record, err := server.models.Create("discount_policy", "user_admin", map[string]any{
		"name": "VIP Discount",
	})
	if err != nil {
		t.Fatalf("create model record failed: %v", err)
	}
	if err := server.audit.Record(audit.Event{
		ID:         "audit-model-1",
		Action:     "model.updated",
		TargetType: "model:discount_policy",
		TargetID:   record.ID,
		ActorID:    "user_admin",
		OccurredAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("record model audit event failed: %v", err)
	}
	actor := ActorContext{
		ActorID: "user_admin",
		PermissionChecker: func(permissionKey string) bool {
			switch permissionKey {
			case "pricing.policy.read":
				return true
			default:
				return false
			}
		},
	}

	resp := server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "business.timeline.get",
			"arguments": map[string]any{
				"resource_kind": "model",
				"model_key":     "discount_policy",
				"record_id":     record.ID,
			},
		}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("business.timeline.get failed: %+v", resp.Error)
	}
	timeline := resp.Result.(map[string]any)["structuredContent"].(map[string]any)
	if len(timeline["audit_events"].([]audit.Event)) != 1 {
		t.Fatalf("expected model audit event in timeline, got %+v", timeline)
	}
}

func TestToolsListHidesBusinessTimelineGetWithoutReadableResources(t *testing.T) {
	server := newTestServer(t)
	actor := ActorContext{
		ActorID: "user_admin",
		PermissionChecker: func(permissionKey string) bool {
			return permissionKey == "module.read"
		},
	}

	resp := server.Handle(context.Background(), JSONRPCRequest{JSONRPC: "2.0", ID: 1, Method: "tools/list"}, actor)
	if resp.Error != nil {
		t.Fatalf("tools/list failed: %+v", resp.Error)
	}
	tools := resp.Result.(map[string]any)["tools"].([]ToolDescriptor)
	names := make([]string, 0, len(tools))
	for _, item := range tools {
		names = append(names, item.Name)
	}
	if contains(names, "business.timeline.get") {
		t.Fatalf("did not expect business.timeline.get for module.read-only actor: %+v", names)
	}
}

func TestToolsListIncludesBusinessTimelineGetForModelReaders(t *testing.T) {
	server := newTestServer(t)
	if err := server.models.Register(model.Definition{
		Key:                 "discount_policy",
		DisplayName:         "Discount Policy",
		OwnerModuleKey:      "pricing",
		ListPermissionKey:   "pricing.policy.list",
		ReadPermissionKey:   "pricing.policy.read",
		CreatePermissionKey: "pricing.policy.create",
		UpdatePermissionKey: "pricing.policy.update",
		Fields: []model.FieldDefinition{
			{Key: "name", Type: "string", Label: "Name"},
		},
	}); err != nil {
		t.Fatalf("register model failed: %v", err)
	}
	actor := ActorContext{
		ActorID: "user_admin",
		PermissionChecker: func(permissionKey string) bool {
			return permissionKey == "pricing.policy.read"
		},
	}

	resp := server.Handle(context.Background(), JSONRPCRequest{JSONRPC: "2.0", ID: 1, Method: "tools/list"}, actor)
	if resp.Error != nil {
		t.Fatalf("tools/list failed: %+v", resp.Error)
	}
	tools := resp.Result.(map[string]any)["tools"].([]ToolDescriptor)
	names := make([]string, 0, len(tools))
	for _, item := range tools {
		names = append(names, item.Name)
	}
	if !contains(names, "business.timeline.get") {
		t.Fatalf("expected business.timeline.get for model reader, got %+v", names)
	}
}

func TestBusinessRecordSearchDoesNotLeakDocumentsWithoutDocumentList(t *testing.T) {
	server := newTestServer(t)
	if err := server.documents.Register(document.Definition{
		Type:          "promotion_plan",
		DisplayName:   "Promotion Plan",
		SchemaVersion: "v1",
	}); err != nil {
		t.Fatalf("register promotion_plan failed: %v", err)
	}
	if err := server.modules.Register(module.Manifest{
		Key:                "pricing",
		Name:               "Pricing",
		DomainFamily:       "commercial",
		Category:           "pricing",
		OwnedDocumentTypes: []string{"promotion_plan"},
		Documents: []document.Definition{{
			Type:          "promotion_plan",
			DisplayName:   "Promotion Plan",
			SchemaVersion: "v1",
		}},
	}, "system"); err != nil {
		t.Fatalf("register pricing manifest failed: %v", err)
	}
	if _, err := server.documents.Create("promotion_plan", "org_default", "loc_hq", "user_admin", map[string]any{"name": "Secret Promo"}); err != nil {
		t.Fatalf("create promotion draft failed: %v", err)
	}
	actor := ActorContext{
		ActorID: "user_admin",
		PermissionChecker: func(permissionKey string) bool {
			return permissionKey == "module.read"
		},
	}

	resp := server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "business.record.search",
			"arguments": map[string]any{
				"module_key": "pricing",
			},
		}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("business.record.search failed: %+v", resp.Error)
	}
	items := resp.Result.(map[string]any)["structuredContent"].(map[string]any)["items"].([]businessRecordSummary)
	if len(items) != 0 {
		t.Fatalf("expected no document leakage without document.list, got %+v", items)
	}
}

func TestBusinessDocumentDraftUpdateValidatesMergedPayload(t *testing.T) {
	server := newTestServer(t)
	if err := server.documents.Register(document.Definition{
		Type:          "promotion_plan",
		DisplayName:   "Promotion Plan",
		SchemaVersion: "v1",
	}); err != nil {
		t.Fatalf("register promotion_plan failed: %v", err)
	}
	if err := server.modules.Register(module.Manifest{
		Key:                "pricing",
		Name:               "Pricing",
		DomainFamily:       "commercial",
		Category:           "pricing",
		OwnedDocumentTypes: []string{"promotion_plan"},
		Documents: []document.Definition{{
			Type:          "promotion_plan",
			DisplayName:   "Promotion Plan",
			SchemaVersion: "v1",
		}},
		Frontend: module.FrontendDefinition{
			Views: []module.ViewDefinition{{
				Key:          "pricing.promotion_plan.form",
				Title:        "Promotion Plan Form",
				Kind:         "form",
				DocumentType: "promotion_plan",
				Fields: []module.FieldDefinition{
					{Key: "name", Label: "Name", Path: "body.payload.name", Type: "string", Required: true},
					{Key: "description", Label: "Description", Path: "body.payload.description", Type: "string"},
				},
			}},
		},
	}, "system"); err != nil {
		t.Fatalf("register pricing manifest failed: %v", err)
	}
	created, err := server.documents.Create("promotion_plan", "org_default", "loc_hq", "user_admin", map[string]any{
		"name":        "Ramadan Promo",
		"description": "Before update",
	})
	if err != nil {
		t.Fatalf("create promotion draft failed: %v", err)
	}
	actor := ActorContext{
		ActorID:         "user_admin",
		EffectiveUserID: "user_admin",
		LocationID:      "loc_hq",
		PermissionChecker: func(permissionKey string) bool {
			switch permissionKey {
			case "document.update_draft":
				return true
			default:
				return false
			}
		},
	}

	resp := server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "business.document.draft.update",
			"arguments": map[string]any{
				"document_id":   created.Header.ID,
				"payload":       map[string]any{"description": "After update"},
				"confirm_apply": true,
			},
		}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("business.document.draft.update failed: %+v", resp.Error)
	}
	record := resp.Result.(map[string]any)["structuredContent"].(map[string]any)["record"].(document.Record)
	if got := record.Body.Payload["description"]; got != "After update" {
		t.Fatalf("expected description patch applied, got %+v", record.Body.Payload)
	}
}

func TestToolsListIncludesGovernanceMetadataAndSourceTypes(t *testing.T) {
	server := newTestServer(t)
	server.config = config.NewService()
	if err := server.documents.Register(document.Definition{
		Type:          "promotion_plan",
		DisplayName:   "Promotion Plan",
		SchemaVersion: "v1",
	}); err != nil {
		t.Fatalf("register promotion_plan failed: %v", err)
	}
	if err := server.modules.Register(module.Manifest{
		Key:                "pricing",
		Name:               "Pricing",
		DomainFamily:       "commercial",
		Category:           "pricing",
		OwnedDocumentTypes: []string{"promotion_plan"},
		Documents: []document.Definition{{
			Type:          "promotion_plan",
			DisplayName:   "Promotion Plan",
			SchemaVersion: "v1",
		}},
		MCP: module.MCPDefinition{
			Tools: []module.MCPToolDefinition{{
				Key:                 "pricing.promotion.special.review",
				Title:               "Special Promotion Review",
				Description:         "Review promotion conditions.",
				Operation:           "analytics.snapshot.get",
				RequiredPermissions: []string{"module.read"},
			}},
		},
	}, "system"); err != nil {
		t.Fatalf("register pricing manifest failed: %v", err)
	}
	actor := ActorContext{
		ActorID: "user_admin",
		PermissionChecker: func(permissionKey string) bool {
			switch permissionKey {
			case "module.read", "document.list", "document.read", "document.create", "document.update_draft", "analytics.read":
				return true
			default:
				return false
			}
		},
	}

	resp := server.Handle(context.Background(), JSONRPCRequest{JSONRPC: "2.0", ID: 1, Method: "tools/list"}, actor)
	if resp.Error != nil {
		t.Fatalf("tools/list failed: %+v", resp.Error)
	}
	tools := resp.Result.(map[string]any)["tools"].([]ToolDescriptor)
	byName := map[string]ToolDescriptor{}
	for _, item := range tools {
		byName[item.Name] = item
	}

	health := byName["business.health.summary"]
	if health.SourceType != "built_in" || health.ModuleKey != "platform.core" {
		t.Fatalf("expected built-in source metadata, got %+v", health)
	}
	if health.Contract.ActionClass != "analyze" || health.Contract.RiskClass != "low" {
		t.Fatalf("expected analyze metadata, got %+v", health.Contract)
	}
	overview := byName["business.analytics.overview"]
	if overview.SourceType != "built_in" || overview.Contract.ActionClass != "analyze" || overview.Contract.RiskClass != "low" {
		t.Fatalf("expected analytical overview metadata, got %+v", overview)
	}
	if overview.PolicyState != "allowed" {
		t.Fatalf("expected allowed policy state, got %+v", overview)
	}
	if !contains(overview.Contract.GovernanceTags, "analytics") {
		t.Fatalf("expected analytics governance tag, got %+v", overview.Contract)
	}

	synthetic := byName["pricing.business.document.draft.create"]
	if synthetic.SourceType != "synthetic" || synthetic.ModuleKey != "pricing" {
		t.Fatalf("expected synthetic source metadata, got %+v", synthetic)
	}
	if synthetic.Contract.ActionClass != "draft" || !synthetic.Contract.RequiresConfirmation || !synthetic.Contract.DraftOnly {
		t.Fatalf("expected draft governance metadata, got %+v", synthetic.Contract)
	}
	if synthetic.PolicyState != "confirmation_required" {
		t.Fatalf("expected confirmation-required policy state, got %+v", synthetic)
	}

	moduleTool := byName["pricing.promotion.special.review"]
	if moduleTool.SourceType != "module" || moduleTool.ModuleKey != "pricing" {
		t.Fatalf("expected module source metadata, got %+v", moduleTool)
	}
	if moduleTool.Contract.ActionClass == "" || moduleTool.Contract.RiskClass == "" {
		t.Fatalf("expected populated contract metadata, got %+v", moduleTool.Contract)
	}
}

func TestToolsListCompactCatalogFiltersByCapability(t *testing.T) {
	server := newTestServer(t)
	actor := ActorContext{
		ActorID:         "user_admin",
		EffectiveUserID: "user_admin",
		PermissionChecker: func(permissionKey string) bool {
			switch permissionKey {
			case "module.read", "document.list", "document.read", "document.create", "document.update_draft":
				return true
			default:
				return false
			}
		},
	}

	resp := server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/list",
		Params: mustJSON(t, map[string]any{
			"catalog_mode":          "compact",
			"capabilities":          []string{"cross_domain_analytics"},
			"include_summary":       true,
			"include_hidden_counts": true,
			"max_tools":             8,
		}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("compact tools/list failed: %+v", resp.Error)
	}
	payload := resp.Result.(map[string]any)
	tools := payload["tools"].([]ToolDescriptor)
	if len(tools) == 0 || len(tools) > 8 {
		t.Fatalf("expected compact tools within max_tools, got %d", len(tools))
	}
	for _, item := range tools {
		if !containsString(item.CapabilityKeys, "cross_domain_analytics") {
			t.Fatalf("expected compact tool to match requested capability, got %+v", item)
		}
	}
	if containsToolNamed(tools, "module.enable") {
		t.Fatalf("did not expect platform admin tool in analytics compact catalog, got %+v", tools)
	}
	catalog := payload["catalog"].(map[string]any)
	if anyString(catalog["mode"]) != "compact" {
		t.Fatalf("expected compact mode metadata, got %+v", catalog)
	}
}

func TestToolsCallCatalogContextBlocksOutOfScopeTool(t *testing.T) {
	server := newTestServer(t)
	actor := ActorContext{
		ActorID:         "user_admin",
		EffectiveUserID: "user_admin",
		PermissionChecker: func(permissionKey string) bool {
			switch permissionKey {
			case "module.read", "module.manage":
				return true
			default:
				return false
			}
		},
	}

	resp := server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "module.enable",
			"arguments": map[string]any{
				"module_key":    "analytics",
				"confirm_apply": true,
			},
			"catalog_context": map[string]any{
				"catalog_mode": "compact",
				"capabilities": []string{"cross_domain_analytics"},
				"max_tools":    8,
			},
		}),
	}, actor)
	if resp.Error == nil || !strings.Contains(resp.Error.Message, "out of scope") {
		t.Fatalf("expected out-of-scope catalog block, got %+v", resp.Error)
	}
}

func TestToolsListCompactCatalogAllowsExplicitDomainFilterWithoutCapabilities(t *testing.T) {
	server := newTestServer(t)
	actor := ActorContext{
		ActorID:         "user_admin",
		EffectiveUserID: "user_admin",
		PermissionChecker: func(permissionKey string) bool {
			return permissionKey == "module.read"
		},
	}

	resp := server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/list",
		Params: mustJSON(t, map[string]any{
			"catalog_mode":    "compact",
			"domains":         []string{"pricing"},
			"include_summary": true,
			"max_tools":       16,
		}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("compact domain-filtered tools/list failed: %+v", resp.Error)
	}
	payload := resp.Result.(map[string]any)
	tools := payload["tools"].([]ToolDescriptor)
	if !containsToolNamed(tools, "pricing.promotion.advisor.review") {
		t.Fatalf("expected pricing advisor tool in pricing compact catalog, got %+v", tools)
	}
	catalog := payload["catalog"].(map[string]any)
	if capabilities, ok := catalog["capabilities"].([]string); ok && len(capabilities) != 0 {
		t.Fatalf("expected no implicit capabilities for explicit domain filter, got %+v", capabilities)
	}
}

func TestToolsCallCatalogContextIgnoresMaxToolsTruncation(t *testing.T) {
	server := newTestServer(t)
	actor := ActorContext{
		ActorID:         "user_admin",
		EffectiveUserID: "user_admin",
		PermissionChecker: func(permissionKey string) bool {
			switch permissionKey {
			case "module.read", "analytics.read":
				return true
			default:
				return false
			}
		},
	}

	resp := server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "business.analytics.trend",
			"arguments": map[string]any{
				"limit": 1,
			},
			"catalog_context": map[string]any{
				"catalog_mode": "compact",
				"capabilities": []string{"cross_domain_analytics"},
				"max_tools":    1,
			},
		}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("expected in-scope tool call to ignore max_tools truncation, got %+v", resp.Error)
	}
}

func TestMCPGovernanceBlocksSubmitAndMutationByDefault(t *testing.T) {
	server := newTestServer(t)
	server.config = config.NewService()

	actor := ActorContext{
		ActorID:         "user_admin",
		EffectiveUserID: "user_admin",
		PermissionChecker: func(permissionKey string) bool {
			switch permissionKey {
			case "configuration.read", "configuration.manage", "module.manage":
				return true
			default:
				return false
			}
		},
	}

	draft, err := server.workflows.CreateDraft("generic_request_flow", "user_admin")
	if err != nil {
		t.Fatalf("create workflow draft failed: %v", err)
	}

	resp := server.Handle(context.Background(), JSONRPCRequest{JSONRPC: "2.0", ID: 1, Method: "tools/list"}, actor)
	if resp.Error != nil {
		t.Fatalf("tools/list failed: %+v", resp.Error)
	}
	tools := resp.Result.(map[string]any)["tools"].([]ToolDescriptor)
	byName := map[string]ToolDescriptor{}
	for _, item := range tools {
		byName[item.Name] = item
	}
	if byName["workflow.draft.publish"].PolicyState != "blocked" {
		t.Fatalf("expected blocked publish policy state, got %+v", byName["workflow.draft.publish"])
	}
	if byName["module.enable"].PolicyState != "blocked" {
		t.Fatalf("expected blocked module.enable policy state, got %+v", byName["module.enable"])
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "workflow.draft.publish",
			"arguments": map[string]any{
				"workflow_key":    "generic_request_flow",
				"version":         draft.Version,
				"confirm_publish": true,
			},
		}),
	}, actor)
	if resp.Error == nil || !strings.Contains(resp.Error.Message, "blocked by policy") {
		t.Fatalf("expected publish policy block, got %+v", resp.Error)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "module.enable",
			"arguments": map[string]any{
				"module_key":    "analytics",
				"confirm_apply": true,
			},
		}),
	}, actor)
	if resp.Error == nil || !strings.Contains(resp.Error.Message, "blocked by policy") {
		t.Fatalf("expected module.enable policy block, got %+v", resp.Error)
	}
}

func TestMCPGovernanceAllowsSubmitDocumentTypesInDraftOnlyMode(t *testing.T) {
	server := newTestServer(t)
	server.config = config.NewService()
	if err := server.config.Save(config.Entry{
		Key:   "platform.mcp",
		Scope: "deployment",
		Value: map[string]any{
			"enabled":                            true,
			"governance_enabled":                 true,
			"default_action_mode":                "draft_only",
			"tool_states_json":                   "{}",
			"blocked_action_classes_json":        "[]",
			"blocked_tool_keys_json":             "[]",
			"blocked_document_types_json":        "[]",
			"allowed_submit_document_types_json": `["promotion_plan"]`,
			"domain_policy_overrides_json":       "{}",
		},
	}); err != nil {
		t.Fatalf("save platform.mcp config failed: %v", err)
	}

	allowed := server.evaluateToolGovernance(ToolDescriptor{
		Name: "business.promotion.submit",
		Contract: ContractDescriptor{
			ActionClass: "submit",
		},
	}, map[string]any{"document_type": "promotion_plan"})
	if !allowed.Allowed || allowed.PolicyState == "blocked" {
		t.Fatalf("expected allowlisted submit document type to be allowed, got %+v", allowed)
	}

	blocked := server.evaluateToolGovernance(ToolDescriptor{
		Name: "business.promotion.submit",
		Contract: ContractDescriptor{
			ActionClass: "submit",
		},
	}, map[string]any{"document_type": "tax_structure"})
	if blocked.Allowed || blocked.PolicyState != "blocked" {
		t.Fatalf("expected non-allowlisted submit document type to be blocked, got %+v", blocked)
	}
}

func TestBusinessComprehensionTools(t *testing.T) {
	server := newTestServer(t)
	if err := server.documents.Register(document.Definition{
		Type:          "promotion_plan",
		DisplayName:   "Promotion Plan",
		SchemaVersion: "v1",
	}); err != nil {
		t.Fatalf("register promotion_plan failed: %v", err)
	}
	if err := server.modules.Register(module.Manifest{
		Key:                  "pricing",
		Name:                 "Pricing",
		Description:          "Pricing and promotions",
		DomainFamily:         "commercial",
		Category:             "pricing",
		BusinessCapabilities: []string{"promotion planning", "discount strategy"},
		OwnedDocumentTypes:   []string{"promotion_plan"},
		Documents: []document.Definition{{
			Type:          "promotion_plan",
			DisplayName:   "Promotion Plan",
			SchemaVersion: "v1",
		}},
	}, "system"); err != nil {
		t.Fatalf("register pricing manifest failed: %v", err)
	}
	record, err := server.documents.Create("promotion_plan", "org_default", "loc_hq", "user_admin", map[string]any{"name": "Ramadan Promo"})
	if err != nil {
		t.Fatalf("create promotion draft failed: %v", err)
	}
	if err := server.audit.Record(audit.Event{
		ID:             "audit-1",
		Action:         "document.created",
		TargetType:     "document",
		TargetID:       record.Header.ID,
		ActorID:        "user_admin",
		OrganizationID: "org_default",
		LocationID:     "loc_hq",
		OccurredAt:     time.Now().UTC(),
	}); err != nil {
		t.Fatalf("record audit event failed: %v", err)
	}
	transition, err := server.workflows.Execute("generic_request_flow", "draft", "submit")
	if err != nil {
		t.Fatalf("prepare workflow transition failed: %v", err)
	}
	if err := server.workflows.CreateSideEffects(transition, "document", record.Header.ID, time.Now().UTC()); err != nil {
		t.Fatalf("create workflow side effects failed: %v", err)
	}
	if _, err := server.analytics.CaptureSnapshot(); err != nil {
		t.Fatalf("capture analytics snapshot failed: %v", err)
	}
	actor := ActorContext{
		ActorID:         "user_admin",
		EffectiveUserID: "user_admin",
		LocationID:      "loc_hq",
		PermissionChecker: func(permissionKey string) bool {
			switch permissionKey {
			case "module.read", "document.list", "document.read", "analytics.read", "configuration.read":
				return true
			default:
				return false
			}
		},
	}

	resp := server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "business.topology.map", "arguments": map[string]any{}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("business.topology.map failed: %+v", resp.Error)
	}
	topology := resp.Result.(map[string]any)["structuredContent"].(map[string]any)
	if len(topology["nodes"].([]map[string]any)) == 0 {
		t.Fatalf("expected topology nodes, got %+v", topology)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "business.timeline.get", "arguments": map[string]any{"resource_kind": "document", "document_id": record.Header.ID}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("business.timeline.get failed: %+v", resp.Error)
	}
	timeline := resp.Result.(map[string]any)["structuredContent"].(map[string]any)
	if len(timeline["audit_events"].([]audit.Event)) != 1 {
		t.Fatalf("expected audit event in timeline, got %+v", timeline)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "business.health.summary", "arguments": map[string]any{}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("business.health.summary failed: %+v", resp.Error)
	}
	health := resp.Result.(map[string]any)["structuredContent"].(map[string]any)
	documentsSection := health["documents"].(map[string]any)
	if documentsSection["count"].(int) < 1 {
		t.Fatalf("expected document count in health summary, got %+v", health)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      4,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "pricing.promotion.advisor.review", "arguments": map[string]any{}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("pricing.promotion.advisor.review failed: %+v", resp.Error)
	}
	advisor := resp.Result.(map[string]any)["structuredContent"].(map[string]any)
	if len(advisor["relevant_modules"].([]businessModuleInfo)) == 0 {
		t.Fatalf("expected relevant modules in advisor review, got %+v", advisor)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      5,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "business.analytics.overview", "arguments": map[string]any{}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("business.analytics.overview failed: %+v", resp.Error)
	}
	overview := resp.Result.(map[string]any)["structuredContent"].(map[string]any)
	if len(overview["summary"].([]map[string]any)) == 0 {
		t.Fatalf("expected analytical summary cards, got %+v", overview)
	}
	if len(overview["drilldowns"].([]map[string]any)) == 0 {
		t.Fatalf("expected drilldowns in analytical overview, got %+v", overview)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      6,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "business.analytics.trend", "arguments": map[string]any{"bucket": "day"}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("business.analytics.trend failed: %+v", resp.Error)
	}
	trend := resp.Result.(map[string]any)["structuredContent"].(map[string]any)
	if len(trend["series"].([]map[string]any)) == 0 {
		t.Fatalf("expected analytical trend series, got %+v", trend)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      7,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "business.analytics.anomaly.search", "arguments": map[string]any{}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("business.analytics.anomaly.search failed: %+v", resp.Error)
	}
	anomalies := resp.Result.(map[string]any)["structuredContent"].(map[string]any)
	if len(anomalies["items"].([]map[string]any)) == 0 {
		t.Fatalf("expected analytical anomalies, got %+v", anomalies)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      8,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "business.analytics.exception.cluster", "arguments": map[string]any{}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("business.analytics.exception.cluster failed: %+v", resp.Error)
	}
	clustered := resp.Result.(map[string]any)["structuredContent"].(map[string]any)
	if len(clustered["items"].([]map[string]any)) == 0 {
		t.Fatalf("expected clustered exceptions, got %+v", clustered)
	}

	firstDrilldown := overview["drilldowns"].([]map[string]any)[0]
	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      9,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "business.analytics.drilldown", "arguments": map[string]any{"handle": firstDrilldown["handle"]}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("business.analytics.drilldown failed: %+v", resp.Error)
	}
	drilldown := resp.Result.(map[string]any)["structuredContent"].(map[string]any)
	if strings.TrimSpace(anyString(drilldown["target_tool"])) == "" {
		t.Fatalf("expected resolved drilldown target, got %+v", drilldown)
	}
}

func TestBusinessRecordSearchDocumentFiltersSkipModelValidation(t *testing.T) {
	server := newTestServer(t)
	if err := server.documents.Register(document.Definition{
		Type:          "expense_claim",
		DisplayName:   "Expense Claim",
		SchemaVersion: "v1",
	}); err != nil {
		t.Fatalf("register expense_claim failed: %v", err)
	}
	if err := server.modules.Register(module.Manifest{
		Key:                "employee_spend_core",
		Name:               "Employee Spend Core",
		OwnedDocumentTypes: []string{"expense_claim"},
		Documents: []document.Definition{{
			Type:          "expense_claim",
			DisplayName:   "Expense Claim",
			SchemaVersion: "v1",
		}},
	}, "system"); err != nil {
		t.Fatalf("register employee_spend_core failed: %v", err)
	}
	record, err := server.documents.Create("expense_claim", "org_default", "loc_hq", "user_admin", map[string]any{
		"employee_code": "EMP-SPEND-001",
		"title":         "Travel Claim",
		"amount":        170,
	})
	if err != nil {
		t.Fatalf("create expense claim failed: %v", err)
	}
	actor := ActorContext{
		ActorID:         "user_admin",
		EffectiveUserID: "user_admin",
		LocationID:      "loc_hq",
		PermissionChecker: func(permissionKey string) bool {
			return permissionKey == "document.list" || permissionKey == "module.read"
		},
	}
	resp := server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "employee_spend_core.business.records.search",
			"arguments": map[string]any{
				"resource_kind": "document",
				"filters": map[string]any{
					"employee_code": "EMP-SPEND-001",
				},
			},
		}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("business.records.search failed: %+v", resp.Error)
	}
	result := resp.Result.(map[string]any)
	content := result["content"].([]ContentBlock)
	if len(content) == 0 || !strings.Contains(content[0].Text, record.Header.ID) || !strings.Contains(content[0].Text, "expense_claim") {
		t.Fatalf("expected search text summary with matching record, got %+v", content)
	}
	items := result["structuredContent"].(map[string]any)["items"].([]businessRecordSummary)
	if len(items) != 1 || items[0].RecordID != record.Header.ID {
		t.Fatalf("expected matching document summary, got %+v", items)
	}
}

func TestBusinessDocumentGetIncludesReadableSummaryText(t *testing.T) {
	server := newTestServer(t)
	if err := server.documents.Register(document.Definition{
		Type:          "expense_claim",
		DisplayName:   "Expense Claim",
		SchemaVersion: "v1",
	}); err != nil {
		t.Fatalf("register expense_claim failed: %v", err)
	}
	record, err := server.documents.Create("expense_claim", "org_default", "loc_hq", "user_admin", map[string]any{
		"employee_code":         "EMP-SPEND-001",
		"title":                 "Travel Claim",
		"claim_total_amount":    170,
		"net_settlement_amount": 140,
		"settlement_direction":  "company_owes_employee",
		"reimbursement_status":  "paid",
	})
	if err != nil {
		t.Fatalf("create expense claim failed: %v", err)
	}
	record.Header.Status = "approved"
	if err := server.documents.Save(record); err != nil {
		t.Fatalf("save expense claim failed: %v", err)
	}
	actor := ActorContext{
		ActorID:         "user_admin",
		EffectiveUserID: "user_admin",
		LocationID:      "loc_hq",
		PermissionChecker: func(permissionKey string) bool {
			return permissionKey == "document.read"
		},
	}
	resp := server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "business.document.get",
			"arguments": map[string]any{
				"document_id":          record.Header.ID,
				"include_full_payload": false,
			},
		}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("business.document.get failed: %+v", resp.Error)
	}
	content := resp.Result.(map[string]any)["content"].([]ContentBlock)
	if len(content) == 0 {
		t.Fatal("expected text content")
	}
	text := content[0].Text
	for _, fragment := range []string{
		record.Header.ID,
		"Type: expense_claim",
		"Title: Travel Claim",
		"Status: approved",
		"Business summary: expense claim total is 170 and status is approved.",
		`"employee_code":"EMP-SPEND-001"`,
		`"claim_total_amount":170`,
	} {
		if !strings.Contains(text, fragment) {
			t.Fatalf("expected %q in %q", fragment, text)
		}
	}
}

func TestBusinessDocumentGetIncludesSettlementInterpretation(t *testing.T) {
	server := newTestServer(t)
	if err := server.documents.Register(document.Definition{
		Type:          "advance_liquidation",
		DisplayName:   "Advance Liquidation",
		SchemaVersion: "v1",
	}); err != nil {
		t.Fatalf("register advance_liquidation failed: %v", err)
	}
	record, err := server.documents.Create("advance_liquidation", "org_default", "loc_hq", "user_admin", map[string]any{
		"claim_total_amount":     170.0,
		"advance_amount":         30.0,
		"advance_applied_amount": 30.0,
		"net_settlement_amount":  140.0,
		"settlement_direction":   "company_owes_employee",
	})
	if err != nil {
		t.Fatalf("create liquidation failed: %v", err)
	}
	record.Header.Status = "approved"
	if err := server.documents.Save(record); err != nil {
		t.Fatalf("save liquidation failed: %v", err)
	}
	actor := ActorContext{
		ActorID:         "user_admin",
		EffectiveUserID: "user_admin",
		LocationID:      "loc_hq",
		PermissionChecker: func(permissionKey string) bool {
			return permissionKey == "document.read"
		},
	}
	resp := server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "business.document.get",
			"arguments": map[string]any{
				"document_id": record.Header.ID,
			},
		}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("business.document.get failed: %+v", resp.Error)
	}
	text := resp.Result.(map[string]any)["content"].([]ContentBlock)[0].Text
	for _, fragment := range []string{
		"Settlement summary: claim total 170, advance amount 30, advance applied 30, net settlement 140.",
		"Interpretation: the company owes the employee 140.",
	} {
		if !strings.Contains(text, fragment) {
			t.Fatalf("expected %q in %q", fragment, text)
		}
	}
}

func TestBusinessAnalyticsTrendUsesLatestSnapshotPerBucket(t *testing.T) {
	server := newTestServer(t)
	if err := server.documents.Register(document.Definition{
		Type:          "promotion_plan",
		DisplayName:   "Promotion Plan",
		SchemaVersion: "v1",
	}); err != nil {
		t.Fatalf("register promotion_plan failed: %v", err)
	}
	first, err := server.documents.Create("promotion_plan", "org_default", "loc_hq", "user_admin", map[string]any{"name": "Promo 1"})
	if err != nil {
		t.Fatalf("create first document failed: %v", err)
	}
	first.Header.Status = "submitted"
	if err := server.documents.Save(first); err != nil {
		t.Fatalf("save first submitted document failed: %v", err)
	}
	if _, err := server.analytics.CaptureSnapshot(); err != nil {
		t.Fatalf("capture first snapshot failed: %v", err)
	}
	second, err := server.documents.Create("promotion_plan", "org_default", "loc_hq", "user_admin", map[string]any{"name": "Promo 2"})
	if err != nil {
		t.Fatalf("create second document failed: %v", err)
	}
	second.Header.Status = "submitted"
	if err := server.documents.Save(second); err != nil {
		t.Fatalf("save second submitted document failed: %v", err)
	}
	if _, err := server.analytics.CaptureSnapshot(); err != nil {
		t.Fatalf("capture second snapshot failed: %v", err)
	}
	actor := ActorContext{
		ActorID: "user_admin",
		PermissionChecker: func(permissionKey string) bool {
			return permissionKey == "analytics.read"
		},
	}

	resp := server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "business.analytics.trend", "arguments": map[string]any{"bucket": "day"}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("business.analytics.trend failed: %+v", resp.Error)
	}
	series := resp.Result.(map[string]any)["structuredContent"].(map[string]any)["series"].([]map[string]any)
	if len(series) != 1 {
		t.Fatalf("expected one trend bucket, got %+v", series)
	}
	if got := anyInt(series[0]["submitted_documents"]); got != 2 {
		t.Fatalf("expected latest snapshot submitted_documents=2, got %+v", series[0])
	}
	if got := anyInt(series[0]["approved_documents"]); got != 0 {
		t.Fatalf("expected latest snapshot approved_documents=0, got %+v", series[0])
	}
}

func TestBusinessAnalyticsExceptionClusterHonorsStatusFilterAndMixedStatusDrilldown(t *testing.T) {
	server := newTestServer(t)
	if err := server.documents.Register(document.Definition{
		Type:          "promotion_plan",
		DisplayName:   "Promotion Plan",
		SchemaVersion: "v1",
	}); err != nil {
		t.Fatalf("register promotion_plan failed: %v", err)
	}
	recordSubmitted, err := server.documents.Create("promotion_plan", "org_default", "loc_hq", "user_admin", map[string]any{"name": "Submitted Promo"})
	if err != nil {
		t.Fatalf("create submitted document failed: %v", err)
	}
	recordRejected, err := server.documents.Create("promotion_plan", "org_default", "loc_hq", "user_admin", map[string]any{"name": "Rejected Promo"})
	if err != nil {
		t.Fatalf("create rejected document failed: %v", err)
	}
	recordSubmitted.Header.Status = "submitted"
	recordRejected.Header.Status = "rejected"
	if err := server.documents.Save(recordSubmitted); err != nil {
		t.Fatalf("save submitted document failed: %v", err)
	}
	if err := server.documents.Save(recordRejected); err != nil {
		t.Fatalf("save rejected document failed: %v", err)
	}
	transition, err := server.workflows.Execute("generic_request_flow", "draft", "submit")
	if err != nil {
		t.Fatalf("prepare workflow transition failed: %v", err)
	}
	if err := server.workflows.CreateSideEffects(transition, "document", recordSubmitted.Header.ID, time.Now().UTC()); err != nil {
		t.Fatalf("create workflow side effects failed: %v", err)
	}
	actor := ActorContext{
		ActorID: "user_admin",
		PermissionChecker: func(permissionKey string) bool {
			switch permissionKey {
			case "module.read", "document.list", "configuration.read":
				return true
			default:
				return false
			}
		},
	}

	resp := server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "business.analytics.exception.cluster", "arguments": map[string]any{"status": "pending", "group_by": "kind"}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("business.analytics.exception.cluster with status filter failed: %+v", resp.Error)
	}
	clusters := resp.Result.(map[string]any)["structuredContent"].(map[string]any)["items"].([]map[string]any)
	if len(clusters) != 1 || anyString(clusters[0]["group_key"]) != "workflow_approval" {
		t.Fatalf("expected only pending approval cluster, got %+v", clusters)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "business.analytics.exception.cluster", "arguments": map[string]any{"group_by": "kind"}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("business.analytics.exception.cluster failed: %+v", resp.Error)
	}
	clusters = resp.Result.(map[string]any)["structuredContent"].(map[string]any)["items"].([]map[string]any)
	var documentCluster map[string]any
	for _, item := range clusters {
		if anyString(item["group_key"]) == "document_status" {
			documentCluster = item
			break
		}
	}
	if len(documentCluster) == 0 {
		t.Fatalf("expected document_status cluster, got %+v", clusters)
	}
	statuses := documentCluster["statuses"].([]string)
	if !(contains(statuses, "submitted") && contains(statuses, "rejected")) {
		t.Fatalf("expected mixed statuses in cluster, got %+v", documentCluster)
	}
	drilldown := documentCluster["drilldown"].(map[string]any)
	targetArgs := drilldown["target_arguments"].(map[string]any)
	statusSlice := stringSliceArg(targetArgs, "statuses")
	if !(contains(statusSlice, "submitted") && contains(statusSlice, "rejected")) {
		t.Fatalf("expected drilldown to preserve all statuses, got %+v", drilldown)
	}
}

func TestServerEngagementFlow(t *testing.T) {
	server := newTestServer(t)
	actor := ActorContext{
		ActorID: "user_admin",
		PermissionChecker: func(permissionKey string) bool {
			return permissionKey == "configuration.read" || permissionKey == "configuration.manage"
		},
	}

	resp := server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      200,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "engagement.program.create",
			"arguments": map[string]any{
				"program_key":   "loyalty",
				"name":          "Customer Loyalty",
				"subject_type":  "customer",
				"confirm_apply": true,
			},
		}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("engagement.program.create failed: %+v", resp.Error)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      201,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "engagement.program.version.create",
			"arguments": map[string]any{
				"program_key":   "loyalty",
				"confirm_apply": true,
			},
		}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("engagement.program.version.create failed: %+v", resp.Error)
	}
	version := resp.Result.(map[string]any)["structuredContent"].(engagement.ProgramVersion)

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      202,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "engagement.program.version.save",
			"arguments": map[string]any{
				"program_key":   "loyalty",
				"version":       version.Version,
				"confirm_apply": true,
				"rules": []map[string]any{
					{"key": "earn_purchase", "action": "credit_points", "source_event_types": []string{"order.completed"}, "subject_source": "actor_id", "account_key": "points", "fixed_amount": 10},
					{"key": "bronze_tier", "action": "set_tier", "source_event_types": []string{"order.completed"}, "subject_source": "actor_id", "account_key": "points", "threshold": 10, "tier_key": "bronze"},
				},
			},
		}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("engagement.program.version.save failed: %+v", resp.Error)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      203,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "engagement.program.version.publish",
			"arguments": map[string]any{
				"program_key":   "loyalty",
				"version":       version.Version,
				"confirm_apply": true,
			},
		}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("engagement.program.version.publish failed: %+v", resp.Error)
	}

	if err := server.eventing.Record(eventing.Event{ID: "evt-loyalty", Type: "order.completed", AggregateType: "order", AggregateID: "ord-1", ActorID: "cust-1", OccurredAt: time.Now().UTC()}); err != nil {
		t.Fatalf("record event failed: %v", err)
	}
	if _, err := server.eventing.DispatchPending(10); err != nil {
		t.Fatalf("dispatch failed: %v", err)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      204,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "engagement.balance.get",
			"arguments": map[string]any{
				"program_key": "loyalty",
				"subject_id":  "cust-1",
				"account_key": "points",
			},
		}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("engagement.balance.get failed: %+v", resp.Error)
	}
	balance := resp.Result.(map[string]any)["structuredContent"].(engagement.BalanceSnapshot)
	if balance.Balance != 10 {
		t.Fatalf("expected balance 10, got %+v", balance)
	}
}

func TestServerDataOpsBackupRunRequiresManagePermission(t *testing.T) {
	server := newTestServer(t)
	actor := ActorContext{
		ActorID: "user_viewer",
		PermissionChecker: func(permissionKey string) bool {
			return permissionKey == "configuration.read"
		},
	}
	resp := server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      99,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "dataops.backup.run",
			"arguments": map[string]any{
				"selected_data_classes": []string{"configuration"},
				"confirm_apply":         true,
			},
		}),
	}, actor)
	if resp.Error == nil {
		t.Fatal("expected backup run without configuration.manage to fail")
	}
}

func TestServerAnalyticsScopedEndpointFiltersToolsAndResources(t *testing.T) {
	server := newTestServer(t)
	actor := ActorContext{
		EndpointScope: EndpointScopeAnalytics,
		PermissionChecker: func(permissionKey string) bool {
			return permissionKey == "analytics.read" || permissionKey == "template.read"
		},
	}

	resp := server.Handle(context.Background(), JSONRPCRequest{JSONRPC: "2.0", ID: 1, Method: "tools/list"}, actor)
	if resp.Error != nil {
		t.Fatalf("analytics scoped tools/list failed: %+v", resp.Error)
	}
	tools := resp.Result.(map[string]any)["tools"].([]ToolDescriptor)
	toolNames := make([]string, 0, len(tools))
	for _, tool := range tools {
		toolNames = append(toolNames, tool.Name)
		if tool.Scope != EndpointScopeAnalytics {
			t.Fatalf("expected analytics scope, got %+v", tool)
		}
	}
	if contains(toolNames, "template.definition.list") {
		t.Fatalf("did not expect template tool on analytics endpoint: %+v", toolNames)
	}
	if !contains(toolNames, "analytics.snapshot.get") {
		t.Fatalf("expected analytics tool on analytics endpoint: %+v", toolNames)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{JSONRPC: "2.0", ID: 2, Method: "resources/list"}, actor)
	if resp.Error != nil {
		t.Fatalf("analytics scoped resources/list failed: %+v", resp.Error)
	}
	resources := resp.Result.(map[string]any)["resources"].([]ResourceDescriptor)
	resourceURIs := make([]string, 0, len(resources))
	for _, resource := range resources {
		resourceURIs = append(resourceURIs, resource.URI)
		if resource.Scope != EndpointScopeAnalytics {
			t.Fatalf("expected analytics scope, got %+v", resource)
		}
	}
	if contains(resourceURIs, templateDesignerResourceURI) {
		t.Fatalf("did not expect template resource on analytics endpoint: %+v", resourceURIs)
	}
	if !contains(resourceURIs, analyticsStudioResourceURI) {
		t.Fatalf("expected analytics studio resource on analytics endpoint: %+v", resourceURIs)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "template.definition.list", "arguments": map[string]any{}}),
	}, actor)
	if resp.Error == nil || !strings.Contains(resp.Error.Message, "not available on this endpoint") {
		t.Fatalf("expected scoped endpoint tool rejection, got %+v", resp.Error)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      4,
		Method:  "resources/read",
		Params:  mustJSON(t, map[string]any{"uri": templateDesignerResourceURI}),
	}, actor)
	if resp.Error == nil || !strings.Contains(resp.Error.Message, "not available on this endpoint") {
		t.Fatalf("expected scoped endpoint resource rejection, got %+v", resp.Error)
	}
}

func TestServerTemplateDraftFlowAndPreview(t *testing.T) {
	server := newTestServer(t)
	actor := ActorContext{
		ActorID: "user:agent",
		PermissionChecker: func(permissionKey string) bool {
			switch permissionKey {
			case "template.read", "template.manage", "template.render":
				return true
			default:
				return false
			}
		},
	}

	resp := server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "template.definition.get", "arguments": map[string]any{"template_key": "clinic.registration.print"}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("template.definition.get failed: %+v", resp.Error)
	}
	defPayload := resp.Result.(map[string]any)
	meta := defPayload["_meta"].(map[string]any)["orbyte/app"].(map[string]any)
	if meta["resource_uri"] != "orbyte://apps/template.designer?template_key=clinic.registration.print" {
		t.Fatalf("expected template app resource uri, got %+v", meta)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "template.draft.save",
			"arguments": map[string]any{
				"template_key": "clinic.registration.print",
				"style":        ".template-visual{color:#222}",
				"visual_template": map[string]any{
					"schema_version": "visual-grid/v1",
					"title":          "Registration Slip",
					"sections": []map[string]any{{
						"id":    "body",
						"title": "Body",
						"kind":  "body",
						"rows": []map[string]any{{
							"id": "body-row-1",
							"columns": []map[string]any{{
								"id":   "body-cell-1",
								"span": 12,
								"blocks": []map[string]any{{
									"id":   "block-1",
									"type": "text",
									"text": "Registration Slip",
								}},
							}},
						}},
					}},
				},
			},
		}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("template.draft.save failed: %+v", resp.Error)
	}
	saved := resp.Result.(map[string]any)["structuredContent"].(map[string]any)
	draft := saved["draft"].(templateoutput.Version)
	if draft.Status != "draft" {
		t.Fatalf("expected draft status, got %+v", draft)
	}
	if !strings.Contains(draft.Body, "Registration Slip") {
		t.Fatalf("expected visual template body to be serialized, got %+v", draft)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "template.render.preview", "arguments": map[string]any{"template_key": "clinic.registration.print", "sample": true}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("template.render.preview failed: %+v", resp.Error)
	}
	preview := resp.Result.(map[string]any)["structuredContent"].(map[string]any)["output"].(map[string]any)
	if !strings.Contains(preview["html"].(string), "Registration Slip") {
		t.Fatalf("expected rendered preview html, got %+v", preview)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      4,
		Method:  "resources/read",
		Params:  mustJSON(t, map[string]any{"uri": "orbyte://apps/template.designer?template_key=clinic.registration.print"}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("template resource read failed: %+v", resp.Error)
	}
	contents := resp.Result.(map[string]any)["contents"].([]ResourceContent)
	if len(contents) != 1 || !strings.Contains(contents[0].Text, "Registration Slip") {
		t.Fatalf("expected template designer app html, got %+v", contents)
	}
}

func TestServerWorkflowDraftLifecycleAndPublishConfirmation(t *testing.T) {
	server := newTestServer(t)
	if err := server.config.Save(config.Entry{
		Key:   "platform.mcp",
		Scope: "deployment",
		Value: map[string]any{
			"enabled":                            true,
			"governance_enabled":                 false,
			"default_action_mode":                "draft_only",
			"tool_states_json":                   "{}",
			"blocked_action_classes_json":        "[]",
			"blocked_tool_keys_json":             "[]",
			"blocked_document_types_json":        "[]",
			"allowed_submit_document_types_json": "[]",
			"domain_policy_overrides_json":       "{}",
		},
	}); err != nil {
		t.Fatalf("save platform.mcp config failed: %v", err)
	}
	actor := ActorContext{
		ActorID:         "user_admin",
		EffectiveUserID: "user_admin",
		LocationID:      "loc_hq",
		PermissionChecker: func(permissionKey string) bool {
			return permissionKey == "configuration.read" || permissionKey == "configuration.manage" || permissionKey == "identity.manage_users"
		},
	}

	resp := server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "workflow.definition.get", "arguments": map[string]any{"workflow_key": "generic_request_flow"}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("workflow.definition.get failed: %+v", resp.Error)
	}
	meta := resp.Result.(map[string]any)["_meta"].(map[string]any)["orbyte/app"].(map[string]any)
	if !strings.Contains(meta["resource_uri"].(string), "orbyte://apps/workflow.manager") {
		t.Fatalf("expected workflow manager app meta, got %+v", meta)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "workflow.draft.create", "arguments": map[string]any{"workflow_key": "generic_request_flow"}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("workflow.draft.create failed: %+v", resp.Error)
	}
	draft := resp.Result.(map[string]any)["structuredContent"].(workflow.Definition)
	if draft.Status != "draft" {
		t.Fatalf("expected draft, got %+v", draft)
	}

	draft.Actions[0].AssignmentStrategy = "requester_manager"
	draft.Actions[0].FallbackRoleKey = "platform_admin"
	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "workflow.draft.save", "arguments": map[string]any{"workflow": draft}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("workflow.draft.save failed: %+v", resp.Error)
	}

	subject, err := server.identity.CreateUser("requester", "password123", "loc_hq", "", "", "")
	if err != nil {
		t.Fatalf("create requester failed: %v", err)
	}
	manager, err := server.identity.CreateUser("manager", "password123", "loc_hq", "", "", "")
	if err != nil {
		t.Fatalf("create manager failed: %v", err)
	}
	if _, err := server.identity.UpsertReportingLine(identity.ReportingLine{
		SubjectUserID:    subject.ID,
		ManagerUserID:    manager.ID,
		RelationshipType: "primary_manager",
		Status:           "active",
		LocationID:       "loc_hq",
	}); err != nil {
		t.Fatalf("save reporting line failed: %v", err)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      4,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "workflow.draft.simulate",
			"arguments": map[string]any{
				"workflow_key": "generic_request_flow",
				"version":      draft.Version,
				"input": map[string]any{
					"current_state":   "draft",
					"action":          "submit",
					"organization_id": "org_root",
					"location_id":     "loc_hq",
					"additional_input": map[string]any{
						"requester_user_id": subject.ID,
					},
				},
			},
		}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("workflow.draft.simulate failed: %+v", resp.Error)
	}
	structured := resp.Result.(map[string]any)["structuredContent"].(map[string]any)
	preview := structured["routing_preview"].(map[string]any)
	if preview["resolved_assignee_user_id"] != manager.ID {
		t.Fatalf("expected resolved manager assignee, got %+v", preview)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      5,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "workflow.draft.publish", "arguments": map[string]any{"workflow_key": "generic_request_flow", "version": draft.Version}}),
	}, actor)
	if resp.Error == nil || !strings.Contains(resp.Error.Message, "confirm_publish") {
		t.Fatalf("expected publish confirmation error, got %+v", resp.Error)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      6,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "workflow.draft.publish", "arguments": map[string]any{"workflow_key": "generic_request_flow", "version": draft.Version, "confirm_publish": true}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("workflow.draft.publish failed: %+v", resp.Error)
	}
	published := resp.Result.(map[string]any)["structuredContent"].(workflow.Definition)
	if published.Status != "published" {
		t.Fatalf("expected published status, got %+v", published)
	}
}

func TestServerWorkflowHierarchyAndRuntimeTools(t *testing.T) {
	server := newTestServer(t)
	actor := ActorContext{
		ActorID:         "user_admin",
		EffectiveUserID: "user_admin",
		LocationID:      "loc_hq",
		PermissionChecker: func(permissionKey string) bool {
			return permissionKey == "configuration.read" || permissionKey == "identity.manage_users"
		},
	}

	subject, err := server.identity.CreateUser("chain_user", "password123", "loc_hq", "", "", "")
	if err != nil {
		t.Fatalf("create subject failed: %v", err)
	}
	manager, err := server.identity.CreateUser("chain_manager", "password123", "loc_hq", "", "", "")
	if err != nil {
		t.Fatalf("create manager failed: %v", err)
	}
	resp := server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "workflow.reporting_line.save",
			"arguments": map[string]any{
				"reporting_line": map[string]any{
					"subject_user_id":   subject.ID,
					"manager_user_id":   manager.ID,
					"relationship_type": "primary_manager",
					"status":            "active",
					"location_id":       "loc_hq",
					"organization_id":   "org_root",
					"operating_unit_id": "",
				},
			},
		}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("workflow.reporting_line.save failed: %+v", resp.Error)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "workflow.hierarchy.graph.get", "arguments": map[string]any{"location_id": "loc_hq", "status": "active"}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("workflow.hierarchy.graph.get failed: %+v", resp.Error)
	}
	graph := resp.Result.(map[string]any)["structuredContent"].(map[string]any)
	if len(graph["edges"].([]workflowHierarchyEdge)) == 0 {
		t.Fatalf("expected hierarchy edges, got %+v", graph)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "workflow.hierarchy.chain.get", "arguments": map[string]any{"user_id": subject.ID, "location_id": "loc_hq"}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("workflow.hierarchy.chain.get failed: %+v", resp.Error)
	}
	chain := resp.Result.(map[string]any)["structuredContent"].(map[string]any)["items"].([]map[string]any)
	if len(chain) < 2 {
		t.Fatalf("expected manager chain entries, got %+v", chain)
	}

	now := time.Now().UTC()
	if err := server.workflows.ApplyMutation(workflow.Mutation{
		Tasks: []workflow.Task{{
			ID:          "task:mcp",
			WorkflowKey: "generic_request_flow",
			TargetType:  "document",
			TargetID:    "doc:mcp",
			TaskType:    "review",
			Status:      "open",
			CreatedAt:   now,
		}},
		Approvals: []workflow.Approval{{
			ID:          "approval:mcp",
			WorkflowKey: "generic_request_flow",
			TargetType:  "document",
			TargetID:    "doc:mcp",
			Status:      "pending",
			RequestedAt: now,
		}},
		History: []workflow.HistoryEvent{{
			ID:          "history:mcp",
			WorkflowKey: "generic_request_flow",
			TargetType:  "document",
			TargetID:    "doc:mcp",
			Action:      "submit",
			OccurredAt:  now,
		}},
	}); err != nil {
		t.Fatalf("seed workflow runtime data failed: %v", err)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      4,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "workflow.runtime.tasks.list", "arguments": map[string]any{"target_id": "doc:mcp"}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("workflow.runtime.tasks.list failed: %+v", resp.Error)
	}
	tasks := resp.Result.(map[string]any)["structuredContent"].(map[string]any)["items"].([]workflow.Task)
	if len(tasks) != 1 {
		t.Fatalf("expected one task, got %+v", tasks)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      5,
		Method:  "resources/read",
		Params:  mustJSON(t, map[string]any{"uri": "orbyte://apps/workflow.manager?workflow_key=generic_request_flow&user_id=" + url.QueryEscape(subject.ID)}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("workflow manager resource failed: %+v", resp.Error)
	}
	contents := resp.Result.(map[string]any)["contents"].([]ResourceContent)
	if len(contents) != 1 || !strings.Contains(contents[0].Text, "Workflow Manager") {
		t.Fatalf("expected workflow manager html, got %+v", contents)
	}
}

func TestServerControlPlaneToolsAndResources(t *testing.T) {
	server := newTestServer(t)
	actor := ActorContext{
		ActorID:         "user_admin",
		EffectiveUserID: "user_admin",
		LocationID:      "loc_hq",
		PermissionChecker: func(permissionKey string) bool {
			switch permissionKey {
			case "configuration.read", "configuration.manage", "identity.manage_users", "ops.read", "search.manage", "module.manage":
				return true
			default:
				return false
			}
		},
	}

	resp := server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "config.definition.list", "arguments": map[string]any{}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("config.definition.list failed: %+v", resp.Error)
	}
	if len(resp.Result.(map[string]any)["structuredContent"].(map[string]any)["items"].([]config.Definition)) == 0 {
		t.Fatal("expected config definitions")
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "config.bundle.validate",
			"arguments": map[string]any{
				"bundle": map[string]any{
					"name": "mcp-bundle",
					"config_entries": []map[string]any{{
						"key":   "identity.auth",
						"scope": "deployment",
						"value": map[string]any{
							"providers": map[string]any{
								"password": map[string]any{"enabled": true},
								"google":   map[string]any{"enabled": false, "client_id": "", "client_secret": ""},
							},
							"login_rate_limit_attempts": 3,
						},
					}},
				},
			},
		}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("config.bundle.validate failed: %+v", resp.Error)
	}
	validation := resp.Result.(map[string]any)["structuredContent"].(map[string]any)["validation"].(configBundleValidation)
	if !validation.Valid {
		t.Fatalf("expected valid bundle, got %+v", validation)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "feature_flag.targeting.get",
			"arguments": map[string]any{
				"flag_key":    "platform.admin_console",
				"location_id": "loc_hq",
			},
		}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("feature_flag.targeting.get failed: %+v", resp.Error)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      4,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "identity.role_permission_matrix.get", "arguments": map[string]any{}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("identity.role_permission_matrix.get failed: %+v", resp.Error)
	}
	matrix := resp.Result.(map[string]any)["structuredContent"].(rolePermissionMatrix)
	if len(matrix.Roles) == 0 || len(matrix.Permissions) == 0 {
		t.Fatalf("expected populated role matrix, got %+v", matrix)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      5,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "identity.role_permission.grant",
			"arguments": map[string]any{
				"role_id":        "role_admin",
				"permission_key": "audit.read",
				"confirm_apply":  true,
			},
		}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("identity.role_permission.grant failed: %+v", resp.Error)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      6,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "integration.adapter.list", "arguments": map[string]any{}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("integration.adapter.list failed: %+v", resp.Error)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      7,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "readiness.get", "arguments": map[string]any{}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("readiness.get failed: %+v", resp.Error)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      8,
		Method:  "resources/read",
		Params:  mustJSON(t, map[string]any{"uri": "orbyte://control-plane/config.catalog"}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("config catalog resource failed: %+v", resp.Error)
	}
	if len(resp.Result.(map[string]any)["contents"].([]ResourceContent)) != 1 {
		t.Fatalf("expected control-plane resource content, got %+v", resp.Result)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      9,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "implementation.tenant.inspect",
			"arguments": map[string]any{
				"location_id": "loc_hq",
			},
		}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("implementation.tenant.inspect failed: %+v", resp.Error)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      10,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "search.runtime.list", "arguments": map[string]any{}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("search.runtime.list failed: %+v", resp.Error)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      11,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "offline.sync.list", "arguments": map[string]any{}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("offline.sync.list failed: %+v", resp.Error)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      12,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "implementation.session.create",
			"arguments": map[string]any{
				"name":        "Tenant rollout",
				"location_id": "loc_hq",
			},
		}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("implementation.session.create failed: %+v", resp.Error)
	}
	session := resp.Result.(map[string]any)["structuredContent"].(ImplementationSession)
	if session.ID == "" {
		t.Fatalf("expected session id, got %+v", session)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      13,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "implementation.plan.build",
			"arguments": map[string]any{
				"session_id": session.ID,
				"plan": map[string]any{
					"bundle": map[string]any{
						"config_entries": []map[string]any{{
							"key":   "identity.auth",
							"scope": "deployment",
							"value": map[string]any{
								"providers": map[string]any{
									"password": map[string]any{"enabled": true},
									"google":   map[string]any{"enabled": false, "client_id": "", "client_secret": ""},
								},
								"login_rate_limit_attempts": 5,
							},
						}},
					},
					"module_actions": []map[string]any{{
						"module_key": "analytics",
						"enabled":    false,
					}},
				},
			},
		}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("implementation.plan.build failed: %+v", resp.Error)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      14,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "implementation.plan.validate", "arguments": map[string]any{"session_id": session.ID}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("implementation.plan.validate failed: %+v", resp.Error)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      15,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "implementation.stage.commit", "arguments": map[string]any{"session_id": session.ID, "confirm_apply": true}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("implementation.stage.commit failed: %+v", resp.Error)
	}
	commit := resp.Result.(map[string]any)["structuredContent"].(map[string]any)
	if commit["change_set"] == nil {
		t.Fatalf("expected change-set in commit result, got %+v", commit)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      16,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "implementation.rollback.plan", "arguments": map[string]any{"session_id": session.ID}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("implementation.rollback.plan failed: %+v", resp.Error)
	}
}

func TestServerExpandedControlPlaneCoverage(t *testing.T) {
	server := newTestServer(t)
	if err := server.config.Save(config.Entry{
		Key:   "platform.mcp",
		Scope: "deployment",
		Value: map[string]any{
			"enabled":                            true,
			"governance_enabled":                 false,
			"default_action_mode":                "draft_only",
			"tool_states_json":                   "{}",
			"blocked_action_classes_json":        "[]",
			"blocked_tool_keys_json":             "[]",
			"blocked_document_types_json":        "[]",
			"allowed_submit_document_types_json": "[]",
			"domain_policy_overrides_json":       "{}",
		},
	}); err != nil {
		t.Fatalf("save platform.mcp config failed: %v", err)
	}
	actor := ActorContext{
		ActorID:         "user_admin",
		EffectiveUserID: "user_admin",
		LocationID:      "loc_hq",
		PermissionChecker: func(permissionKey string) bool {
			switch permissionKey {
			case "configuration.read", "configuration.manage", "identity.manage_users", "ops.read", "search.manage", "module.manage":
				return true
			default:
				return false
			}
		},
	}

	if err := server.search.RegisterIndex(search.IndexDefinition{
		Key:           "documents.summary.search",
		Title:         "Document Summary",
		SourceKind:    "projection",
		ProjectionKey: "document_summary",
		Modes:         []string{"keyword"},
		Fields:        []search.IndexFieldDefinition{{Key: "status", Path: "status", Type: "string"}},
	}); err != nil {
		t.Fatalf("register search index failed: %v", err)
	}
	if err := server.reference.RegisterType(reference.TypeDefinition{Key: "country", DisplayName: "Country"}); err != nil {
		t.Fatalf("register reference type failed: %v", err)
	}
	if err := server.reference.UpsertRecord(reference.Record{
		TypeKey:     "country",
		Key:         "id",
		DisplayName: "Indonesia",
		Scope:       "deployment",
	}); err != nil {
		t.Fatalf("upsert reference record failed: %v", err)
	}

	batch := server.offline.StartBatch("corr-mcp-offline", "user_admin", "device-1", 1)
	server.offline.RecordOutcome(&batch, offline.SyncResultItem{
		IdempotencyKey: "idem-mcp-conflict",
		Status:         offline.StatusConflict,
		Kind:           "document",
		Operation:      "update",
		TargetID:       "doc-conflict",
		Conflict: offline.SyncConflict{
			Current:   map[string]any{"status": "approved"},
			Attempted: map[string]any{"status": "submitted"},
		},
	})

	for _, uri := range []string{
		flagCatalogResourceURI,
		roleMatrixResourceURI,
		moduleCompatResourceURI,
		integrationHealthResourceURI,
		readinessResourceURI,
		searchRuntimeResourceURI,
		offlineOpsResourceURI,
		policyRuntimeResourceURI,
		referenceCatalogResourceURI,
		implementationBlueprintsURI,
		runbooksResourceURI,
	} {
		resp := server.Handle(context.Background(), JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      uri,
			Method:  "resources/read",
			Params:  mustJSON(t, map[string]any{"uri": uri}),
		}, actor)
		if resp.Error != nil {
			t.Fatalf("resource %s failed: %+v", uri, resp.Error)
		}
		contents := resp.Result.(map[string]any)["contents"].([]ResourceContent)
		if len(contents) != 1 || contents[0].MIMEType != "application/json" || strings.TrimSpace(contents[0].Text) == "" {
			t.Fatalf("unexpected contents for %s: %+v", uri, contents)
		}
	}

	resp := server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "config.entry.list",
			"arguments": map[string]any{
				"config_keys":   []string{"identity.auth"},
				"config_scopes": []string{"deployment"},
			},
		}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("config.entry.list failed: %+v", resp.Error)
	}
	if len(resp.Result.(map[string]any)["structuredContent"].(map[string]any)["items"].([]config.Entry)) == 0 {
		t.Fatal("expected filtered config entries")
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "config.effective.get",
			"arguments": map[string]any{
				"location_id": "loc_hq",
			},
		}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("config.effective.get failed: %+v", resp.Error)
	}
	if len(resp.Result.(map[string]any)["structuredContent"].(map[string]any)["items"].([]config.EffectiveValue)) == 0 {
		t.Fatal("expected effective config values")
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "config.compare",
			"arguments": map[string]any{
				"left":  map[string]any{"label": "left"},
				"right": map[string]any{"location_id": "loc_hq"},
			},
		}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("config.compare failed: %+v", resp.Error)
	}
	comparePayload := resp.Result.(map[string]any)["structuredContent"].(map[string]any)
	if comparePayload["left"].(config.CompareContext).Label != "left" {
		t.Fatalf("expected compare context label to be preserved, got %+v", comparePayload)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      4,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "config.bundle.export",
			"arguments": map[string]any{
				"name":          "control-plane-export",
				"config_keys":   []string{"identity.auth"},
				"config_scopes": []string{"deployment"},
				"include_flags": true,
				"flag_keys":     []string{"platform.admin_console"},
			},
		}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("config.bundle.export failed: %+v", resp.Error)
	}
	exported := resp.Result.(map[string]any)["structuredContent"].(map[string]any)["bundle"].(configBundle)
	if exported.Name != "control-plane-export" || len(exported.ConfigEntries) == 0 {
		t.Fatalf("expected exported bundle content, got %+v", exported)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      5,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "feature_flag.definition.list", "arguments": map[string]any{}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("feature_flag.definition.list failed: %+v", resp.Error)
	}
	if len(resp.Result.(map[string]any)["structuredContent"].(map[string]any)["items"].([]featureflags.Definition)) == 0 {
		t.Fatal("expected feature flag definitions")
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      6,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "feature_flag.value.upsert",
			"arguments": map[string]any{
				"value": map[string]any{
					"flag_key": "platform.admin_console",
					"scope":    "location",
					"scope_id": "loc_hq",
					"enabled":  true,
					"status":   "active",
				},
				"confirm_apply": true,
			},
		}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("feature_flag.value.upsert failed: %+v", resp.Error)
	}
	upsertedFlag := resp.Result.(map[string]any)["structuredContent"].(map[string]any)["value"].(featureflags.Value)
	if upsertedFlag.ScopeID != "loc_hq" {
		t.Fatalf("expected location-scoped flag value, got %+v", upsertedFlag)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      7,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "feature_flag.value.list", "arguments": map[string]any{}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("feature_flag.value.list failed: %+v", resp.Error)
	}
	if len(resp.Result.(map[string]any)["structuredContent"].(map[string]any)["items"].([]featureflags.Value)) == 0 {
		t.Fatal("expected feature flag values")
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      8,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "config.bundle.apply",
			"arguments": map[string]any{
				"bundle": configBundle{
					Name: "applied",
					ConfigEntries: []config.Entry{{
						Key:   "identity.auth",
						Scope: "deployment",
						Value: map[string]any{
							"providers": map[string]any{
								"password": map[string]any{"enabled": true},
								"google":   map[string]any{"enabled": false, "client_id": "", "client_secret": ""},
							},
							"login_rate_limit_attempts": 7,
						},
					}},
				},
				"confirm_apply": true,
			},
		}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("config.bundle.apply failed: %+v", resp.Error)
	}
	applied := resp.Result.(map[string]any)["structuredContent"].(map[string]any)
	if executed, _ := applied["executed"].(bool); !executed {
		t.Fatalf("expected config bundle apply to execute, got %+v", applied)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      9,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "module.list", "arguments": map[string]any{}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("module.list failed: %+v", resp.Error)
	}
	if len(resp.Result.(map[string]any)["structuredContent"].(map[string]any)["items"].([]module.Detail)) == 0 {
		t.Fatal("expected modules")
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      10,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "module.compatibility.list", "arguments": map[string]any{}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("module.compatibility.list failed: %+v", resp.Error)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      11,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "search.runtime.get", "arguments": map[string]any{"index_key": "documents.summary.search"}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("search.runtime.get failed: %+v", resp.Error)
	}
	if resp.Result.(map[string]any)["structuredContent"].(search.IndexRuntime).IndexKey != "documents.summary.search" {
		t.Fatalf("unexpected search runtime payload: %+v", resp.Result)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      12,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "search.consistency.get", "arguments": map[string]any{"index_key": "documents.summary.search"}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("search.consistency.get failed: %+v", resp.Error)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      13,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "offline.sync.get", "arguments": map[string]any{"batch_id": batch.ID}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("offline.sync.get failed: %+v", resp.Error)
	}
	offlinePayload := resp.Result.(map[string]any)["structuredContent"].(map[string]any)
	if offlinePayload["batch"].(offline.SyncBatch).ID != batch.ID {
		t.Fatalf("expected requested batch, got %+v", offlinePayload)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      14,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "offline.conflict.list", "arguments": map[string]any{}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("offline.conflict.list failed: %+v", resp.Error)
	}
	if len(resp.Result.(map[string]any)["structuredContent"].(map[string]any)["items"].([]any)) == 0 {
		t.Fatal("expected offline conflicts")
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      15,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name":      "identity.role_permission.revoke",
			"arguments": map[string]any{"role_id": "role_admin", "permission_key": "audit.read", "confirm_apply": true},
		}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("identity.role_permission.revoke failed: %+v", resp.Error)
	}
}

func TestServerIntegrationAndOpsControlPlaneCoverage(t *testing.T) {
	server := newTestServer(t)
	actor := ActorContext{
		ActorID:         "user_admin",
		EffectiveUserID: "user_admin",
		LocationID:      "loc_hq",
		PermissionChecker: func(permissionKey string) bool {
			switch permissionKey {
			case "configuration.read", "configuration.manage", "ops.read":
				return true
			default:
				return false
			}
		},
	}

	success, err := server.integration.CreateDelivery(integration.SubmissionRecord{
		ExternalSystemKey: "fake_erp",
		EndpointKey:       "fake_erp.default",
		ContractKey:       "document.submit",
		ContractVersion:   1,
		Intent:            "command",
		Mode:              "sync",
		OperationType:     "submit_document",
		DocumentID:        "doc-ok",
		CorrelationID:     "corr-trace",
		Payload:           map[string]any{"title": "ok"},
	})
	if err != nil {
		t.Fatalf("create successful submission failed: %v", err)
	}
	if _, err := server.integration.ProcessSubmission(success.ID); err != nil {
		t.Fatalf("process successful submission failed: %v", err)
	}

	failing, err := server.integration.CreateDelivery(integration.SubmissionRecord{
		ExternalSystemKey: "fake_erp",
		EndpointKey:       "fake_erp.default",
		ContractKey:       "document.submit",
		ContractVersion:   1,
		Intent:            "command",
		Mode:              "sync",
		OperationType:     "submit_document",
		DocumentID:        "doc-fail",
		CorrelationID:     "corr-trace",
		Payload:           map[string]any{"force_fail": true},
	})
	if err != nil {
		t.Fatalf("create failing submission failed: %v", err)
	}
	for i := 0; i < 3; i++ {
		failing, err = server.integration.ProcessSubmission(failing.ID)
		if err != nil {
			t.Fatalf("process failing submission attempt %d failed: %v", i+1, err)
		}
	}
	if failing.Status != "dead_letter" {
		t.Fatalf("expected dead letter status, got %+v", failing)
	}
	letters := server.integration.ListDeadLetters()
	if len(letters) == 0 {
		t.Fatal("expected integration dead letters")
	}

	now := time.Now().UTC()
	if err := server.audit.Record(audit.Event{
		ID:            "audit-trace",
		Action:        "configuration.apply",
		TargetType:    "config",
		TargetID:      "identity.auth",
		ActorID:       "user_admin",
		OccurredAt:    now,
		CorrelationID: "corr-trace",
	}); err != nil {
		t.Fatalf("record audit event failed: %v", err)
	}
	if err := server.eventing.Record(eventing.Event{
		ID:            "evt-trace",
		Type:          "document.submitted",
		AggregateType: "document",
		AggregateID:   "doc-ok",
		ActorID:       "user_admin",
		OccurredAt:    now.Add(time.Second),
		CorrelationID: "corr-trace",
	}); err != nil {
		t.Fatalf("record domain event failed: %v", err)
	}
	traceBatch := server.offline.StartBatch("corr-trace", "user_admin", "device-trace", 1)
	server.offline.RecordOutcome(&traceBatch, offline.SyncResultItem{
		IdempotencyKey: "trace-item",
		Status:         offline.StatusAccepted,
		Kind:           "document",
		Operation:      "create",
		TargetID:       "doc-ok",
	})

	var resp JSONRPCResponse
	for _, tc := range []struct {
		id   any
		name string
		args map[string]any
	}{
		{id: 1, name: "integration.system.list", args: map[string]any{}},
		{id: 2, name: "integration.system.config.get", args: map[string]any{"system_key": "http_bridge"}},
		{id: 3, name: "integration.system.config.update", args: map[string]any{"system_key": "http_bridge", "settings": map[string]any{"url": "https://bridge.example", "method": "POST"}, "confirm_apply": true}},
		{id: 4, name: "integration.endpoint.list", args: map[string]any{}},
		{id: 5, name: "integration.endpoint.config.get", args: map[string]any{"endpoint_key": "fake_erp.default"}},
		{id: 6, name: "integration.endpoint.config.update", args: map[string]any{"endpoint_key": "fake_erp.default", "settings": map[string]any{"mode": "sync"}, "confirm_apply": true}},
		{id: 7, name: "integration.submission.list", args: map[string]any{}},
		{id: 8, name: "integration.submission.get", args: map[string]any{"submission_id": success.ID}},
		{id: 9, name: "integration.dead_letter.list", args: map[string]any{}},
		{id: 10, name: "ops.health.get", args: map[string]any{}},
		{id: 11, name: "ops.audit.correlation.get", args: map[string]any{"correlation_id": "corr-trace"}},
		{id: 12, name: "ops.trace.get", args: map[string]any{"correlation_id": "corr-trace"}},
	} {
		resp = server.Handle(context.Background(), JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      tc.id,
			Method:  "tools/call",
			Params:  mustJSON(t, map[string]any{"name": tc.name, "arguments": tc.args}),
		}, actor)
		if resp.Error != nil {
			t.Fatalf("%s failed: %+v", tc.name, resp.Error)
		}
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      13,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "integration.dead_letter.replay", "arguments": map[string]any{"dead_letter_id": "missing-dead-letter", "confirm_apply": true}}),
	}, actor)
	if resp.Error == nil || !strings.Contains(resp.Error.Message, "dead letter") {
		t.Fatalf("expected replay of missing dead letter to fail, got %+v", resp.Error)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      14,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "ops.trace.get", "arguments": map[string]any{"correlation_id": "corr-trace"}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("ops.trace.get verification failed: %+v", resp.Error)
	}
	trace := resp.Result.(map[string]any)["structuredContent"].(map[string]any)
	switch steps := trace["steps"].(type) {
	case []any:
		if len(steps) == 0 {
			t.Fatalf("expected operational trace steps, got %+v", trace)
		}
	case []map[string]any:
		if len(steps) == 0 {
			t.Fatalf("expected operational trace steps, got %+v", trace)
		}
	default:
		t.Fatalf("expected operational trace steps, got %+v", trace)
	}
}

func TestServerRuntimeCatalogListsAndGets(t *testing.T) {
	server := newTestServer(t)
	actor := ActorContext{
		ActorID:         "user_admin",
		EffectiveUserID: "user_admin",
		LocationID:      "loc_hq",
		PermissionChecker: func(permissionKey string) bool {
			switch permissionKey {
			case "template.read", "template.manage", "template.render", "analytics.read", "analytics.author", "analytics.manage_reports", "analytics.deliver_reports", "configuration.read", "configuration.manage", "identity.manage_users":
				return true
			default:
				return false
			}
		},
	}

	resp := server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "template.definition.list", "arguments": map[string]any{}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("template.definition.list failed: %+v", resp.Error)
	}
	if len(resp.Result.(map[string]any)["structuredContent"].(map[string]any)["items"].([]templateoutput.Definition)) == 0 {
		t.Fatal("expected template definitions")
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "template.draft.get", "arguments": map[string]any{"template_key": "clinic.registration.print"}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("template.draft.get failed: %+v", resp.Error)
	}
	if resp.Result.(map[string]any)["_meta"] == nil {
		t.Fatal("expected template draft metadata")
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "workflow.definition.list", "arguments": map[string]any{}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("workflow.definition.list failed: %+v", resp.Error)
	}
	items := resp.Result.(map[string]any)["structuredContent"].(map[string]any)["items"].([]workflow.Definition)
	if len(items) == 0 {
		t.Fatal("expected workflow definitions")
	}
	workflowKey := items[0].Key

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      4,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "workflow.version.list", "arguments": map[string]any{"workflow_key": workflowKey}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("workflow.version.list failed: %+v", resp.Error)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      5,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "workflow.draft.create", "arguments": map[string]any{"workflow_key": workflowKey}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("workflow.draft.create failed: %+v", resp.Error)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      6,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "workflow.draft.get", "arguments": map[string]any{"workflow_key": workflowKey}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("workflow.draft.get failed: %+v", resp.Error)
	}
	draftCtx := resp.Result.(map[string]any)["structuredContent"].(map[string]any)
	draft := draftCtx["draft"].(workflow.Definition)

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      7,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "workflow.draft.validate", "arguments": map[string]any{"workflow": draft}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("workflow.draft.validate failed: %+v", resp.Error)
	}

	now := time.Now().UTC()
	if err := server.workflows.ApplyMutation(workflow.Mutation{
		Approvals: []workflow.Approval{{
			ID:          "approval:list",
			WorkflowKey: workflowKey,
			TargetType:  "document",
			TargetID:    "doc:list",
			StageKey:    "approval",
			Status:      "pending",
			RequestedAt: now,
		}},
		History: []workflow.HistoryEvent{{
			ID:          "history:list",
			WorkflowKey: workflowKey,
			TargetType:  "document",
			TargetID:    "doc:list",
			Action:      "submit",
			OccurredAt:  now,
		}},
	}); err != nil {
		t.Fatalf("seed workflow list data failed: %v", err)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      8,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "workflow.runtime.approvals.list", "arguments": map[string]any{"target_id": "doc:list"}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("workflow.runtime.approvals.list failed: %+v", resp.Error)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      9,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "workflow.runtime.history.get", "arguments": map[string]any{"target_type": "document", "target_id": "doc:list"}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("workflow.runtime.history.get failed: %+v", resp.Error)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      10,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "workflow.hierarchy.summary.get", "arguments": map[string]any{"location_id": "loc_hq"}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("workflow.hierarchy.summary.get failed: %+v", resp.Error)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      11,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "workflow.reporting_line.list", "arguments": map[string]any{"status": "active"}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("workflow.reporting_line.list failed: %+v", resp.Error)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      11,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "analytics.metric.save",
			"arguments": map[string]any{"metric": map[string]any{
				"name": "Submitted Count",
				"spec": map[string]any{"source_kind": "snapshot", "measures": []string{"submitted"}},
			}},
		}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("analytics.metric.save failed: %+v", resp.Error)
	}
	metric := resp.Result.(map[string]any)["structuredContent"].(analytics.SavedMetric)

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      12,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "analytics.query.save",
			"arguments": map[string]any{"query": map[string]any{
				"name": "Submitted Query",
				"spec": map[string]any{"source_kind": "snapshot", "measures": []string{"submitted"}},
			}},
		}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("analytics.query.save failed: %+v", resp.Error)
	}
	query := resp.Result.(map[string]any)["structuredContent"].(analytics.SavedQuery)

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      13,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "analytics.dashboard.save",
			"arguments": map[string]any{"dashboard": map[string]any{
				"name":    "Ops Dashboard",
				"widgets": []map[string]any{{"title": "Submitted", "kind": "chart", "query_id": query.ID}},
			}},
		}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("analytics.dashboard.save failed: %+v", resp.Error)
	}
	dashboard := resp.Result.(map[string]any)["structuredContent"].(map[string]any)["dashboard"].(analytics.Dashboard)

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      13.1,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "analytics.dashboard.widget_catalog",
			"arguments": map[string]any{
				"surface": "dashboard",
			},
		}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("analytics.dashboard.widget_catalog failed: %+v", resp.Error)
	}
	catalogResult := resp.Result.(map[string]any)
	catalog := catalogResult["structuredContent"].(map[string]any)
	if len(catalog["items"].([]module.DashboardWidgetDefinition)) == 0 {
		t.Fatalf("expected dashboard widget catalog items, got %+v", catalog)
	}
	catalogContent := catalogResult["content"].([]ContentBlock)[0].Text
	if !strings.Contains(catalogContent, "analytics.demo.submitted_documents") {
		t.Fatalf("expected widget catalog content to list widget keys, got %q", catalogContent)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      13.15,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "analytics.dashboard.widget.preview",
			"arguments": map[string]any{
				"title":      "Submitted Documents",
				"surface":    "dashboard",
				"widget_key": "analytics.demo.submitted_documents",
			},
		}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("analytics.dashboard.widget.preview failed: %+v", resp.Error)
	}
	widgetPreviewResult := resp.Result.(map[string]any)
	widgetPreview := widgetPreviewResult["structuredContent"].(map[string]any)
	if widgetPreview["artifact"] == nil {
		t.Fatalf("expected widget preview artifact payload, got %+v", widgetPreview)
	}
	widgetArtifact := widgetPreview["artifact"].(map[string]any)
	if widgetArtifact["kind"] != "dashboard_widget" {
		t.Fatalf("expected dashboard_widget artifact, got %+v", widgetArtifact)
	}
	widgetPreviewContent := widgetPreviewResult["content"].([]ContentBlock)[0].Text
	if !strings.Contains(widgetPreviewContent, "<orbyte-dashboard-artifact>") {
		t.Fatalf("expected widget preview content to include artifact block, got %q", widgetPreviewContent)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      13.16,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "analytics.dashboard.widgets.preview",
			"arguments": map[string]any{
				"title":   "Submitted Approval Widgets",
				"surface": "dashboard",
				"widget_keys": []string{
					"analytics.demo.submitted_documents",
					"analytics.demo.approval_rate",
				},
			},
		}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("analytics.dashboard.widgets.preview failed: %+v", resp.Error)
	}
	widgetsPreviewResult := resp.Result.(map[string]any)
	widgetsPreview := widgetsPreviewResult["structuredContent"].(map[string]any)
	artifacts, _ := widgetsPreview["artifacts"].([]map[string]any)
	if len(artifacts) != 2 {
		t.Fatalf("expected 2 widget artifacts, got %+v", widgetsPreview)
	}
	widgetsPreviewContent := widgetsPreviewResult["content"].([]ContentBlock)[0].Text
	if strings.Count(widgetsPreviewContent, "<orbyte-dashboard-artifact>") != 2 {
		t.Fatalf("expected widgets preview content to include 2 artifact blocks, got %q", widgetsPreviewContent)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      13.17,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "analytics.dashboard.widgets.preview",
			"arguments": map[string]any{
				"title":       "Underperforming branches versus benchmark",
				"surface":     "dashboard",
				"description": "compare branches against the strongest branch and show trend",
				"intent":      "insight",
			},
		}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("analytics.dashboard.widgets.preview with inferred insight failed: %+v", resp.Error)
	}
	inferredWidgets := resp.Result.(map[string]any)["structuredContent"].(map[string]any)["widgets"].([]module.DashboardWidgetDefinition)
	if len(inferredWidgets) == 0 {
		t.Fatalf("expected inferred widgets, got %+v", inferredWidgets)
	}
	inferredKeys := make([]string, 0, len(inferredWidgets))
	for _, item := range inferredWidgets {
		inferredKeys = append(inferredKeys, item.Key)
	}
	if len(inferredKeys) > 3 {
		t.Fatalf("expected insight inference to stay focused, got %+v", inferredKeys)
	}
	for _, key := range inferredKeys {
		if strings.Contains(key, "branch_map") {
			t.Fatalf("did not expect geography widget for generic insight inference, got %+v", inferredKeys)
		}
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      13.2,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "analytics.dashboard.board.preview",
			"arguments": map[string]any{
				"title":       "Preview board",
				"surface":     "dashboard",
				"widget_keys": []string{"analytics.demo.submitted_documents"},
			},
		}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("analytics.dashboard.board.preview failed: %+v", resp.Error)
	}
	previewResult := resp.Result.(map[string]any)
	preview := previewResult["structuredContent"].(map[string]any)
	if preview["artifact"] == nil {
		t.Fatalf("expected preview artifact payload, got %+v", preview)
	}
	previewContent := previewResult["content"].([]ContentBlock)[0].Text
	if !strings.Contains(previewContent, "<orbyte-dashboard-artifact>") {
		t.Fatalf("expected preview content to include artifact block, got %q", previewContent)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      133,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "analytics.dashboard.board.create",
			"arguments": map[string]any{
				"title":       "Created board",
				"surface":     "dashboard",
				"widget_keys": []string{"analytics.demo.submitted_documents", "analytics.demo.approval_rate"},
			},
		}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("analytics.dashboard.board.create failed: %+v", resp.Error)
	}
	createdResult := resp.Result.(map[string]any)
	createdBoard := createdResult["structuredContent"].(map[string]any)
	if createdBoard["artifact"] == nil {
		t.Fatalf("expected created dashboard artifact payload, got %+v", createdBoard)
	}
	createdContent := createdResult["content"].([]ContentBlock)[0].Text
	if !strings.Contains(createdContent, "<orbyte-dashboard-artifact>") {
		t.Fatalf("expected created dashboard content to include artifact block, got %q", createdContent)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      133.1,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "analytics.dashboard.board.preview",
			"arguments": map[string]any{
				"title":       "Submitted Approval Dashboard",
				"surface":     "dashboard",
				"description": "submitted approval rate",
			},
		}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("analytics.dashboard.board.preview without widget_keys failed: %+v", resp.Error)
	}
	inferredPreview := resp.Result.(map[string]any)["structuredContent"].(map[string]any)["dashboard"].(analytics.Dashboard)
	if len(inferredPreview.Widgets) == 0 {
		t.Fatalf("expected inferred dashboard widgets, got %+v", inferredPreview)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      14,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "analytics.report.definition.save",
			"arguments": map[string]any{"report": map[string]any{
				"name":      "Submitted Report",
				"dimension": "document_type",
				"format":    "csv",
			}},
		}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("analytics.report.definition.save failed: %+v", resp.Error)
	}
	reportDef := resp.Result.(map[string]any)["structuredContent"].(analytics.ReportDefinition)

	for _, tc := range []struct {
		id   any
		name string
		args map[string]any
	}{
		{id: 16, name: "analytics.dashboard.list", args: map[string]any{}},
		{id: 17, name: "analytics.dashboard.get", args: map[string]any{"dashboard_id": dashboard.ID}},
		{id: 18, name: "analytics.metric.list", args: map[string]any{}},
		{id: 19, name: "analytics.metric.get", args: map[string]any{"metric_id": metric.ID}},
		{id: 20, name: "analytics.query.list", args: map[string]any{}},
		{id: 21, name: "analytics.query.get", args: map[string]any{"query_id": query.ID}},
		{id: 22, name: "analytics.report.definition.list", args: map[string]any{}},
		{id: 23, name: "analytics.report.definition.get", args: map[string]any{"report_id": reportDef.ID}},
	} {
		resp = server.Handle(context.Background(), JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      tc.id,
			Method:  "tools/call",
			Params:  mustJSON(t, map[string]any{"name": tc.name, "arguments": tc.args}),
		}, actor)
		if resp.Error != nil {
			t.Fatalf("%s failed: %+v", tc.name, resp.Error)
		}
	}
}

func TestServerExtendedPlatformAndExecutionCoverage(t *testing.T) {
	server := newTestServer(t)
	if err := server.config.Save(config.Entry{
		Key:   "platform.mcp",
		Scope: "deployment",
		Value: map[string]any{
			"enabled":                            true,
			"governance_enabled":                 false,
			"default_action_mode":                "draft_only",
			"tool_states_json":                   "{}",
			"blocked_action_classes_json":        "[]",
			"blocked_tool_keys_json":             "[]",
			"blocked_document_types_json":        "[]",
			"allowed_submit_document_types_json": "[]",
			"domain_policy_overrides_json":       "{}",
		},
	}); err != nil {
		t.Fatalf("save platform.mcp config failed: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server.jobs.Start(ctx)
	defer server.jobs.Stop()

	actor := ActorContext{
		ActorID:         "user_admin",
		EffectiveUserID: "user_admin",
		LocationID:      "loc_hq",
		PermissionChecker: func(permissionKey string) bool {
			switch permissionKey {
			case "configuration.read", "configuration.manage", "identity.manage_users", "module.manage":
				return true
			default:
				return false
			}
		},
	}

	if err := server.policy.Register(policy.HookDefinition{
		Key:           "documents.search.visibility",
		Kind:          "search",
		Target:        "document_search",
		AllowedScopes: []string{"deployment", "location"},
		DefaultRule:   map[string]any{"hidden_statuses": []string{}},
		Engine:        policy.EngineRego,
	}); err != nil {
		t.Fatalf("register policy hook failed: %v", err)
	}
	if err := server.reference.RegisterType(reference.TypeDefinition{Key: "country", DisplayName: "Country"}); err != nil {
		t.Fatalf("register reference type failed: %v", err)
	}

	resp := server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "policy.hook.list", "arguments": map[string]any{}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("policy.hook.list failed: %+v", resp.Error)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "policy.module.upsert",
			"arguments": map[string]any{
				"hook_key": "documents.search.visibility",
				"scope":    "location",
				"scope_id": "loc_hq",
				"source": `package orbyte.policy.documents.search.visibility

import rego.v1

default decision := {"allowed": true}

decision := {"allowed": false, "code": "status_hidden", "reason": "document status hidden by policy"} if {
	input.inputs.status != ""
	input.inputs.status in object.get(input.rule, "hidden_statuses", [])
}`,
				"confirm_apply": true,
			},
		}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("policy.module.upsert failed: %+v", resp.Error)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "policy.hook.get", "arguments": map[string]any{"hook_key": "documents.search.visibility", "location_id": "loc_hq"}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("policy.hook.get failed: %+v", resp.Error)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      4,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "identity.role_binding.list", "arguments": map[string]any{}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("identity.role_binding.list failed: %+v", resp.Error)
	}
	bindings := resp.Result.(map[string]any)["structuredContent"].(map[string]any)["items"].([]identity.RoleBinding)
	if len(bindings) == 0 {
		t.Fatal("expected role bindings")
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      5,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "identity.role_binding.priority.set", "arguments": map[string]any{"binding_id": bindings[0].ID, "priority": 200, "confirm_apply": true}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("identity.role_binding.priority.set failed: %+v", resp.Error)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      6,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "module.disable", "arguments": map[string]any{"module_key": "analytics", "confirm_apply": true}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("module.disable failed: %+v", resp.Error)
	}
	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      7,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "module.enable", "arguments": map[string]any{"module_key": "analytics", "confirm_apply": true}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("module.enable failed: %+v", resp.Error)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      8,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "reference.type.list", "arguments": map[string]any{}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("reference.type.list failed: %+v", resp.Error)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      9,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "reference.record.upsert",
			"arguments": map[string]any{
				"record": map[string]any{
					"type_key":     "country",
					"key":          "sg",
					"display_name": "Singapore",
					"scope":        "location",
					"scope_id":     "loc_hq",
				},
				"confirm_apply": true,
			},
		}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("reference.record.upsert failed: %+v", resp.Error)
	}

	for _, tc := range []struct {
		id   any
		name string
		args map[string]any
	}{
		{id: 10, name: "reference.record.list", args: map[string]any{"type_key": "country"}},
		{id: 11, name: "reference.resolve", args: map[string]any{"type_key": "country", "location_id": "loc_hq"}},
		{id: 12, name: "implementation.session.create", args: map[string]any{"name": "extended", "location_id": "loc_hq"}},
	} {
		resp = server.Handle(context.Background(), JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      tc.id,
			Method:  "tools/call",
			Params:  mustJSON(t, map[string]any{"name": tc.name, "arguments": tc.args}),
		}, actor)
		if resp.Error != nil {
			t.Fatalf("%s failed: %+v", tc.name, resp.Error)
		}
	}
	session := resp.Result.(map[string]any)["structuredContent"].(ImplementationSession)

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      13,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "implementation.plan.build",
			"arguments": map[string]any{
				"session_id": session.ID,
				"plan": map[string]any{
					"bundle": map[string]any{
						"config_entries": []map[string]any{{
							"key":   "identity.auth",
							"scope": "deployment",
							"value": map[string]any{
								"providers": map[string]any{
									"password": map[string]any{"enabled": true},
									"google":   map[string]any{"enabled": false, "client_id": "", "client_secret": ""},
								},
								"login_rate_limit_attempts": 11,
							},
						}},
					},
					"module_actions": []map[string]any{{"module_key": "analytics", "enabled": true}},
				},
			},
		}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("implementation.plan.build failed: %+v", resp.Error)
	}

	for _, tc := range []struct {
		id   any
		name string
		args map[string]any
	}{
		{id: 14, name: "implementation.session.list", args: map[string]any{}},
		{id: 15, name: "implementation.session.get", args: map[string]any{"session_id": session.ID}},
		{id: 16, name: "implementation.stage.diff", args: map[string]any{"session_id": session.ID}},
		{id: 17, name: "implementation.verify.state", args: map[string]any{"session_id": session.ID}},
		{id: 18, name: "implementation.verify.readiness", args: map[string]any{"session_id": session.ID}},
		{id: 19, name: "implementation.verify.diff", args: map[string]any{"session_id": session.ID}},
		{id: 20, name: "implementation.verify.smoke", args: map[string]any{"session_id": session.ID}},
	} {
		resp = server.Handle(context.Background(), JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      tc.id,
			Method:  "tools/call",
			Params:  mustJSON(t, map[string]any{"name": tc.name, "arguments": tc.args}),
		}, actor)
		if resp.Error != nil {
			t.Fatalf("%s failed: %+v", tc.name, resp.Error)
		}
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      21,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "implementation.stage.commit", "arguments": map[string]any{"session_id": session.ID, "confirm_apply": true}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("implementation.stage.commit failed: %+v", resp.Error)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      22,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "implementation.checkpoint.create", "arguments": map[string]any{"session_id": session.ID, "name": "after-commit"}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("implementation.checkpoint.create failed: %+v", resp.Error)
	}
	checkpoint := resp.Result.(map[string]any)["structuredContent"].(ImplementationCheckpoint)

	for _, tc := range []struct {
		id   any
		name string
		args map[string]any
	}{
		{id: 23, name: "implementation.checkpoint.list", args: map[string]any{"session_id": session.ID}},
		{id: 24, name: "implementation.rollback.plan", args: map[string]any{"session_id": session.ID}},
	} {
		resp = server.Handle(context.Background(), JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      tc.id,
			Method:  "tools/call",
			Params:  mustJSON(t, map[string]any{"name": tc.name, "arguments": tc.args}),
		}, actor)
		if resp.Error != nil {
			t.Fatalf("%s failed: %+v", tc.name, resp.Error)
		}
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      25,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "implementation.rollback.apply", "arguments": map[string]any{"session_id": session.ID, "confirm_apply": true}}),
	}, actor)
	if resp.Error != nil && !strings.Contains(resp.Error.Message, "google_client_secret") {
		t.Fatalf("unexpected implementation.rollback.apply result: %+v", resp.Error)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      26,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "implementation.checkpoint.restore", "arguments": map[string]any{"session_id": session.ID, "checkpoint_id": checkpoint.ID, "confirm_apply": true}}),
	}, actor)
	if resp.Error != nil && !strings.Contains(resp.Error.Message, "google_client_secret") {
		t.Fatalf("unexpected implementation.checkpoint.restore result: %+v", resp.Error)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      27,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "implementation.plan.build", "arguments": map[string]any{"session_id": session.ID, "plan": map[string]any{"bundle": map[string]any{"config_entries": []map[string]any{{"key": "identity.auth", "scope": "deployment", "value": map[string]any{"providers": map[string]any{"password": map[string]any{"enabled": true}, "google": map[string]any{"enabled": false, "client_id": "", "client_secret": ""}}, "login_rate_limit_attempts": 13}}}}}}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("second implementation.plan.build failed: %+v", resp.Error)
	}
	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      28,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "implementation.stage.discard", "arguments": map[string]any{"session_id": session.ID, "confirm_apply": true}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("implementation.stage.discard failed: %+v", resp.Error)
	}
	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      29,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "implementation.session.close", "arguments": map[string]any{"session_id": session.ID, "confirm_apply": true}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("implementation.session.close failed: %+v", resp.Error)
	}
}

func TestServerDataOpsAndEngagementExpandedCoverage(t *testing.T) {
	server := newTestServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server.jobs.Start(ctx)
	defer server.jobs.Stop()

	actor := ActorContext{
		ActorID:         "user_admin",
		EffectiveUserID: "user_admin",
		LocationID:      "loc_hq",
		PermissionChecker: func(permissionKey string) bool {
			switch permissionKey {
			case "configuration.read", "configuration.manage":
				return true
			default:
				return false
			}
		},
	}

	for _, uri := range []string{dataopsCatalogResourceURI, dataopsArtifactsResourceURI, dataopsCheckpointsResourceURI} {
		resp := server.Handle(context.Background(), JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      uri,
			Method:  "resources/read",
			Params:  mustJSON(t, map[string]any{"uri": uri}),
		}, actor)
		if resp.Error != nil {
			t.Fatalf("dataops resource %s failed: %+v", uri, resp.Error)
		}
	}

	resp := server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "dataops.backup.run",
			"arguments": map[string]any{
				"selected_data_classes": []string{"configuration"},
				"incremental":           true,
				"confirm_apply":         true,
			},
		}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("dataops.backup.run failed: %+v", resp.Error)
	}
	backupPayload := resp.Result.(map[string]any)["structuredContent"].(map[string]any)
	backupRun := backupPayload["run"].(dataops.OperationRun)
	backupJob := backupPayload["job"].(jobs.Job)
	waitForMCPJobStatus(t, server.jobs, backupJob.ID, jobs.StatusSucceeded)

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "dataops.operation.get", "arguments": map[string]any{"operation_id": backupRun.ID}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("dataops.operation.get failed: %+v", resp.Error)
	}
	backupRun = resp.Result.(map[string]any)["structuredContent"].(dataops.OperationRun)
	if backupRun.ArtifactID == "" {
		t.Fatalf("expected backup artifact id, got %+v", backupRun)
	}

	for _, tc := range []struct {
		id   any
		name string
		args map[string]any
	}{
		{id: 3, name: "dataops.artifact.list", args: map[string]any{}},
		{id: 4, name: "dataops.artifact.get", args: map[string]any{"artifact_id": backupRun.ArtifactID}},
		{id: 5, name: "dataops.checkpoint.list", args: map[string]any{}},
		{id: 6, name: "dataops.restore.plan", args: map[string]any{"artifact_id": backupRun.ArtifactID}},
		{id: 7, name: "dataops.restore.validate", args: map[string]any{"artifact_id": backupRun.ArtifactID}},
	} {
		resp = server.Handle(context.Background(), JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      tc.id,
			Method:  "tools/call",
			Params:  mustJSON(t, map[string]any{"name": tc.name, "arguments": tc.args}),
		}, actor)
		if resp.Error != nil {
			t.Fatalf("%s failed: %+v", tc.name, resp.Error)
		}
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      8,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "dataops.restore.run", "arguments": map[string]any{"artifact_id": backupRun.ArtifactID, "confirm_apply": true}}),
	}, actor)
	if resp.Error == nil || !strings.Contains(resp.Error.Message, "restore validation failed") {
		t.Fatalf("expected restore validation failure, got %+v", resp.Error)
	}

	for _, tc := range []struct {
		id   any
		name string
		args map[string]any
	}{
		{id: 9, name: "dataops.archive.plan", args: map[string]any{"selected_data_classes": []string{"transactional"}, "document_types": []string{"generic_request"}, "statuses": []string{"draft"}, "created_before": time.Now().UTC().Format(time.RFC3339)}},
		{id: 10, name: "dataops.export.plan", args: map[string]any{"selected_data_classes": []string{"configuration"}}},
	} {
		resp = server.Handle(context.Background(), JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      tc.id,
			Method:  "tools/call",
			Params:  mustJSON(t, map[string]any{"name": tc.name, "arguments": tc.args}),
		}, actor)
		if resp.Error != nil {
			t.Fatalf("%s failed: %+v", tc.name, resp.Error)
		}
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      11,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "dataops.export.run", "arguments": map[string]any{"selected_data_classes": []string{"configuration"}, "confirm_apply": true}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("dataops.export.run failed: %+v", resp.Error)
	}
	exportJob := resp.Result.(map[string]any)["structuredContent"].(map[string]any)["job"].(jobs.Job)
	waitForMCPJobStatus(t, server.jobs, exportJob.ID, jobs.StatusSucceeded)

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      12,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "dataops.migration.register",
			"arguments": map[string]any{
				"selected_data_classes": []string{"configuration"},
				"segments": []map[string]any{{
					"data_class":  "configuration",
					"adapter_key": "config.entries",
					"records":     []map[string]any{{"key": "identity.auth"}},
				}},
			},
		}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("dataops.migration.register failed: %+v", resp.Error)
	}
	migrationArtifact := resp.Result.(map[string]any)["structuredContent"].(dataops.Artifact)

	for _, tc := range []struct {
		id   any
		name string
		args map[string]any
	}{
		{id: 13, name: "dataops.migration.plan", args: map[string]any{"artifact_id": migrationArtifact.ID}},
		{id: 14, name: "dataops.migration.validate", args: map[string]any{"artifact_id": migrationArtifact.ID}},
	} {
		resp = server.Handle(context.Background(), JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      tc.id,
			Method:  "tools/call",
			Params:  mustJSON(t, map[string]any{"name": tc.name, "arguments": tc.args}),
		}, actor)
		if resp.Error != nil {
			t.Fatalf("%s failed: %+v", tc.name, resp.Error)
		}
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      15,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "dataops.migration.run", "arguments": map[string]any{"artifact_id": migrationArtifact.ID, "confirm_apply": true}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("dataops.migration.run failed: %+v", resp.Error)
	}
	migrationJob := resp.Result.(map[string]any)["structuredContent"].(map[string]any)["job"].(jobs.Job)
	waitForMCPJobStatus(t, server.jobs, migrationJob.ID, jobs.StatusSucceeded)

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      16,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "engagement.program.create",
			"arguments": map[string]any{
				"program_key":   "rewards",
				"name":          "Rewards",
				"subject_type":  "customer",
				"confirm_apply": true,
			},
		}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("engagement.program.create failed: %+v", resp.Error)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      17,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "engagement.program.update", "arguments": map[string]any{"program_key": "rewards", "name": "Rewards Plus", "status": "active", "confirm_apply": true}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("engagement.program.update failed: %+v", resp.Error)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      18,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "engagement.program.version.create", "arguments": map[string]any{"program_key": "rewards", "confirm_apply": true}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("engagement.program.version.create failed: %+v", resp.Error)
	}
	version := resp.Result.(map[string]any)["structuredContent"].(engagement.ProgramVersion)

	rules := []map[string]any{
		{"key": "earn_purchase", "action": "credit_points", "source_event_types": []string{"order.completed"}, "subject_source": "actor_id", "account_key": "points", "fixed_amount": 10},
		{"key": "bronze_tier", "action": "set_tier", "source_event_types": []string{"order.completed"}, "subject_source": "actor_id", "account_key": "points", "threshold": 10, "tier_key": "bronze"},
		{"key": "starter_badge", "action": "grant_achievement", "source_event_types": []string{"order.completed"}, "subject_source": "actor_id", "account_key": "points", "threshold": 10, "achievement_key": "starter"},
	}
	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      19,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "engagement.program.version.save", "arguments": map[string]any{"program_key": "rewards", "version": version.Version, "rules": rules, "confirm_apply": true}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("engagement.program.version.save failed: %+v", resp.Error)
	}

	for _, tc := range []struct {
		id   any
		name string
		args map[string]any
	}{
		{id: 20, name: "engagement.program.list", args: map[string]any{}},
		{id: 21, name: "engagement.program.get", args: map[string]any{"program_key": "rewards"}},
		{id: 22, name: "engagement.program.version.validate", args: map[string]any{"program_key": "rewards", "version": version.Version}},
	} {
		resp = server.Handle(context.Background(), JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      tc.id,
			Method:  "tools/call",
			Params:  mustJSON(t, map[string]any{"name": tc.name, "arguments": tc.args}),
		}, actor)
		if resp.Error != nil {
			t.Fatalf("%s failed: %+v", tc.name, resp.Error)
		}
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      23,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "engagement.program.version.publish", "arguments": map[string]any{"program_key": "rewards", "version": version.Version, "confirm_apply": true}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("engagement.program.version.publish failed: %+v", resp.Error)
	}

	if err := server.eventing.Record(eventing.Event{ID: "evt-rewards", Type: "order.completed", AggregateType: "order", AggregateID: "ord-2", ActorID: "cust-2", OccurredAt: time.Now().UTC()}); err != nil {
		t.Fatalf("record engagement event failed: %v", err)
	}
	if _, err := server.eventing.DispatchPending(10); err != nil {
		t.Fatalf("dispatch engagement event failed: %v", err)
	}

	for _, tc := range []struct {
		id   any
		name string
		args map[string]any
	}{
		{id: 24, name: "engagement.subject.get", args: map[string]any{"program_key": "rewards", "subject_id": "cust-2"}},
		{id: 25, name: "engagement.account.list", args: map[string]any{"program_key": "rewards", "subject_id": "cust-2"}},
		{id: 26, name: "engagement.balance.get", args: map[string]any{"program_key": "rewards", "subject_id": "cust-2", "account_key": "points"}},
		{id: 27, name: "engagement.journal.list", args: map[string]any{"program_key": "rewards", "subject_id": "cust-2"}},
		{id: 28, name: "engagement.qualification.get", args: map[string]any{"program_key": "rewards", "subject_id": "cust-2"}},
		{id: 29, name: "engagement.achievement.list", args: map[string]any{"program_key": "rewards", "subject_id": "cust-2"}},
		{id: 30, name: "engagement.consumer.list", args: map[string]any{}},
	} {
		resp = server.Handle(context.Background(), JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      tc.id,
			Method:  "tools/call",
			Params:  mustJSON(t, map[string]any{"name": tc.name, "arguments": tc.args}),
		}, actor)
		if resp.Error != nil {
			t.Fatalf("%s failed: %+v", tc.name, resp.Error)
		}
	}
	consumers := resp.Result.(map[string]any)["structuredContent"].(map[string]any)["items"].([]engagement.ConsumerState)
	if len(consumers) == 0 {
		t.Fatal("expected engagement consumers")
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      31,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "engagement.consumer.get", "arguments": map[string]any{"consumer_id": consumers[0].ID}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("engagement.consumer.get failed: %+v", resp.Error)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      32,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "engagement.replay.plan", "arguments": map[string]any{"program_key": "rewards", "version": version.Version}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("engagement.replay.plan failed: %+v", resp.Error)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      33,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "engagement.replay.run", "arguments": map[string]any{"program_key": "rewards", "version": version.Version, "confirm_apply": true}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("engagement.replay.run failed: %+v", resp.Error)
	}
	replayPayload := resp.Result.(map[string]any)["structuredContent"].(map[string]any)
	replayRun := replayPayload["run"].(engagement.ReplayRun)
	replayJob := replayPayload["job"].(jobs.Job)
	waitForMCPJobStatus(t, server.jobs, replayJob.ID, jobs.StatusSucceeded)

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      34,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "engagement.replay.get", "arguments": map[string]any{"replay_run_id": replayRun.ID}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("engagement.replay.get failed: %+v", resp.Error)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      35,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "engagement.simulation.run",
			"arguments": map[string]any{
				"program_key": "rewards",
				"version":     version.Version,
				"event": map[string]any{
					"id":             "evt-sim",
					"type":           "order.completed",
					"aggregate_type": "order",
					"aggregate_id":   "ord-sim",
					"actor_id":       "cust-sim",
					"occurred_at":    time.Now().UTC(),
				},
			},
		}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("engagement.simulation.run failed: %+v", resp.Error)
	}
}

func TestServerRejectsMalformedImplementationBundleArguments(t *testing.T) {
	server := newTestServer(t)
	actor := ActorContext{
		ActorID:         "user_admin",
		EffectiveUserID: "user_admin",
		LocationID:      "loc_hq",
		PermissionChecker: func(permissionKey string) bool {
			switch permissionKey {
			case "configuration.manage":
				return true
			default:
				return false
			}
		},
	}

	resp := server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "implementation.config.plan", "arguments": map[string]any{"bundle": "bad-shape"}}),
	}, actor)
	if resp.Error == nil {
		t.Fatal("expected malformed bundle to return an error")
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "implementation.config.apply", "arguments": map[string]any{"bundle": "bad-shape", "confirm_apply": true}}),
	}, actor)
	if resp.Error == nil {
		t.Fatal("expected malformed apply bundle to return an error")
	}
}

func TestImplementationStageCommitDoesNotPartiallyApplyOnValidationFailure(t *testing.T) {
	server := newTestServer(t)
	actor := ActorContext{
		ActorID:         "user_admin",
		EffectiveUserID: "user_admin",
		LocationID:      "loc_hq",
		PermissionChecker: func(permissionKey string) bool {
			switch permissionKey {
			case "configuration.read", "configuration.manage", "module.manage":
				return true
			default:
				return false
			}
		},
	}

	before, found := findConfigEntry(server.config, "identity.auth", "deployment", "")
	if !found {
		t.Fatal("expected seeded identity.auth config")
	}

	resp := server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "implementation.session.create", "arguments": map[string]any{"name": "partial-apply-test", "location_id": "loc_hq"}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("implementation.session.create failed: %+v", resp.Error)
	}
	sessionID := resp.Result.(map[string]any)["structuredContent"].(ImplementationSession).ID

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "implementation.plan.build",
			"arguments": map[string]any{
				"session_id": sessionID,
				"plan": map[string]any{
					"bundle": map[string]any{
						"config_entries": []map[string]any{{
							"key":   "identity.auth",
							"scope": "deployment",
							"value": map[string]any{
								"providers": map[string]any{
									"password": map[string]any{"enabled": true},
									"google":   map[string]any{"enabled": false, "client_id": "", "client_secret": ""},
								},
								"login_rate_limit_attempts": 9,
							},
						}},
					},
					"system_config_updates": []map[string]any{{
						"key":      "missing_system",
						"settings": map[string]any{"base_url": "https://invalid.example"},
					}},
				},
			},
		}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("implementation.plan.build failed: %+v", resp.Error)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "implementation.stage.commit", "arguments": map[string]any{"session_id": sessionID, "confirm_apply": true}}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("implementation.stage.commit returned transport error: %+v", resp.Error)
	}
	report := resp.Result.(map[string]any)["structuredContent"].(ImplementationVerificationReport)
	if report.Passed {
		t.Fatal("expected failed validation report")
	}

	after, found := findConfigEntry(server.config, "identity.auth", "deployment", "")
	if !found {
		t.Fatal("expected identity.auth config to remain present")
	}
	if after.Value["login_rate_limit_attempts"] != before.Value["login_rate_limit_attempts"] {
		t.Fatalf("expected config to remain unchanged, before=%v after=%v", before.Value["login_rate_limit_attempts"], after.Value["login_rate_limit_attempts"])
	}
}

func TestServerCallsAnalyticsToolAndReadsAppResource(t *testing.T) {
	server := newTestServer(t)
	resp := server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "analytics.snapshot.get", "arguments": map[string]any{}}),
	}, ActorContext{
		PermissionChecker: func(permissionKey string) bool { return permissionKey == "analytics.read" },
	})
	if resp.Error != nil {
		t.Fatalf("tools/call failed: %+v", resp.Error)
	}
	result := resp.Result.(map[string]any)
	meta := result["_meta"].(map[string]any)["orbyte/app"].(map[string]any)
	if meta["resource_uri"] != "orbyte://apps/analytics.cockpit" {
		t.Fatalf("expected app resource uri, got %+v", meta)
	}
	if meta["stream_uri"] != "/mcp/analytics/events/analytics/snapshot" {
		t.Fatalf("expected app stream uri, got %+v", meta)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "resources/read",
		Params:  mustJSON(t, map[string]any{"uri": "orbyte://apps/analytics.cockpit"}),
	}, ActorContext{
		PermissionChecker: func(permissionKey string) bool { return permissionKey == "analytics.read" },
	})
	if resp.Error != nil {
		t.Fatalf("resources/read failed: %+v", resp.Error)
	}
	contents := resp.Result.(map[string]any)["contents"].([]ResourceContent)
	if len(contents) != 1 || contents[0].MIMEType != "text/html" {
		t.Fatalf("expected html resource, got %+v", contents)
	}
	if contents[0].Text == "" {
		t.Fatal("expected app html")
	}
	if !strings.Contains(contents[0].Text, "/mcp/analytics/events/analytics/snapshot") {
		t.Fatalf("expected app html to include stream uri, got %q", contents[0].Text)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "resources/read",
		Params:  mustJSON(t, map[string]any{"uri": "orbyte://analytics/snapshot/current"}),
	}, ActorContext{
		PermissionChecker: func(permissionKey string) bool { return permissionKey == "analytics.read" },
	})
	if resp.Error != nil {
		t.Fatalf("read analytics resource failed: %+v", resp.Error)
	}
	jsonContents := resp.Result.(map[string]any)["contents"].([]ResourceContent)
	if len(jsonContents) != 1 || jsonContents[0].MIMEType != "application/json" {
		t.Fatalf("expected json resource, got %+v", jsonContents)
	}
	if !strings.Contains(jsonContents[0].Text, "\"generated_at\"") {
		t.Fatalf("expected analytics json payload, got %q", jsonContents[0].Text)
	}
}

func TestServerRejectsDisallowedAndUnsupportedContracts(t *testing.T) {
	server := newTestServer(t)

	resp := server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "analytics.snapshot.get", "arguments": map[string]any{}}),
	}, ActorContext{
		PermissionChecker: func(string) bool { return false },
	})
	if resp.Error == nil || !strings.Contains(resp.Error.Message, "not allowed") {
		t.Fatalf("expected disallowed tool error, got %+v", resp.Error)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "resources/read",
		Params:  mustJSON(t, map[string]any{"uri": "orbyte://missing"}),
	}, ActorContext{
		PermissionChecker: func(permissionKey string) bool { return permissionKey == "analytics.read" },
	})
	if resp.Error == nil || !strings.Contains(resp.Error.Message, "resource not found") {
		t.Fatalf("expected missing resource error, got %+v", resp.Error)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "tool.missing", "arguments": map[string]any{}}),
	}, ActorContext{
		PermissionChecker: func(permissionKey string) bool { return permissionKey == "analytics.read" },
	})
	if resp.Error == nil || !strings.Contains(resp.Error.Message, "tool not found") {
		t.Fatalf("expected missing tool error, got %+v", resp.Error)
	}
}

func TestServerHandlesGenericViewAppsAndUnavailableServices(t *testing.T) {
	server := newGenericViewServer(t)
	resp := server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "resources/read",
		Params:  mustJSON(t, map[string]any{"uri": "orbyte://apps/analytics.generic"}),
	}, ActorContext{
		PermissionChecker: func(permissionKey string) bool { return permissionKey == "analytics.read" },
	})
	if resp.Error != nil {
		t.Fatalf("generic app resource failed: %+v", resp.Error)
	}
	contents := resp.Result.(map[string]any)["contents"].([]ResourceContent)
	if len(contents) != 1 || !strings.Contains(contents[0].Text, "Generic MCP app bound to shared view definition.") {
		t.Fatalf("expected generic view app html, got %+v", contents)
	}

	unavailable := NewServer(ServerDeps{})
	resp = unavailable.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "analytics.snapshot.get", "arguments": map[string]any{}}),
	}, ActorContext{})
	if resp.Error == nil || !strings.Contains(resp.Error.Message, "mcp tools are unavailable") {
		t.Fatalf("expected unavailable tools error, got %+v", resp.Error)
	}

	resp = unavailable.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "resources/read",
		Params:  mustJSON(t, map[string]any{"uri": "orbyte://analytics/snapshot/current"}),
	}, ActorContext{})
	if resp.Error == nil || !strings.Contains(resp.Error.Message, "mcp resources are unavailable") {
		t.Fatalf("expected unavailable resources error, got %+v", resp.Error)
	}
}

func TestServerRejectsBrokenProvidersAndApps(t *testing.T) {
	server := newBrokenProviderServer(t)

	resp := server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "resources/read",
		Params:  mustJSON(t, map[string]any{"uri": "orbyte://broken/provider"}),
	}, ActorContext{
		PermissionChecker: func(permissionKey string) bool { return permissionKey == "analytics.read" },
	})
	if resp.Error == nil || !strings.Contains(resp.Error.Message, "unsupported resource provider") {
		t.Fatalf("expected unsupported provider error, got %+v", resp.Error)
	}

	if _, err := server.renderApp(ActorContext{}, module.MCPAppDefinition{Key: "empty"}); err == nil || !strings.Contains(err.Error(), "app renderer is not configured") {
		t.Fatalf("expected app renderer error, got %v", err)
	}
}

func TestServerRejectsUnsupportedOperationsAndUnavailableAnalytics(t *testing.T) {
	server := newUnsupportedToolServer(t)

	resp := server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "analytics.snapshot.unsupported", "arguments": map[string]any{}}),
	}, ActorContext{
		PermissionChecker: func(permissionKey string) bool { return permissionKey == "analytics.read" },
	})
	if resp.Error == nil || !strings.Contains(resp.Error.Message, "unsupported tool operation") {
		t.Fatalf("expected unsupported tool operation error, got %+v", resp.Error)
	}

	server = NewServer(ServerDeps{
		Modules:   newTestModules(t),
		Templates: newTestTemplates(t),
		Workflows: workflow.NewService(),
		Identity:  newTestIdentity(t),
	})
	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "resources/read",
		Params:  mustJSON(t, map[string]any{"uri": "orbyte://analytics/snapshot/current"}),
	}, ActorContext{
		PermissionChecker: func(permissionKey string) bool { return permissionKey == "analytics.read" },
	})
	if resp.Error == nil || !strings.Contains(resp.Error.Message, "analytics is unavailable") {
		t.Fatalf("expected unavailable analytics error, got %+v", resp.Error)
	}
}

func TestHelpers(t *testing.T) {
	errResp := errorResponse("id-1", http.StatusForbidden, errors.New("denied"))
	if errResp.Error == nil || errResp.Error.Code != -32003 {
		t.Fatalf("expected forbidden code, got %+v", errResp.Error)
	}
	if allowsAll(func(permissionKey string) bool { return permissionKey != "deny" }, []string{"ok", "deny"}) {
		t.Fatal("expected permission check to fail")
	}
	if !allowsAll(func(permissionKey string) bool { return permissionKey == "ok" }, []string{"", "ok"}) {
		t.Fatal("expected blank permissions to be ignored")
	}
	cloned := cloneMap(map[string]any{"a": 1})
	cloned["a"] = 2
	if original := cloneMap(nil); original != nil {
		t.Fatalf("expected nil clone for nil map, got %+v", original)
	}
	if firstNonEmpty("", " value ", "fallback") != "value" {
		t.Fatalf("expected trimmed first non-empty value")
	}
	if escaped := escapeHTML(`a&<>"'`); escaped != "a&amp;&lt;&gt;&quot;&#39;" {
		t.Fatalf("unexpected html escape output %q", escaped)
	}
}

func newTestServer(t *testing.T) *Server {
	t.Helper()
	modules := newTestModules(t)
	documents := document.NewService()
	documents.RegisterSpecializedViewer(document.SpecializedViewer{
		Hint:             "promotion.plan",
		RequestKinds:     []string{"promotion_plan"},
		DetailPath:       "/ui/promotion/plans/detail",
		FormPath:         "/ui/promotion/plans/form",
		EditableStatuses: []string{"draft"},
	})
	flows := workflow.NewService()
	ident := newTestIdentity(t)
	cfg := config.NewService()
	flags := featureflags.NewService()
	policySvc := policy.NewServiceWithConfig(cfg)
	health := runtimehealth.NewTracker()
	health.SetBootstrapped(true)
	health.SetBackgroundStarted(true)
	integrationSvc := integration.NewService(observability.NewService(), nil)
	auditSvc := audit.NewService()
	eventingSvc := eventing.NewService()
	searchSvc := search.NewService()
	jobSvc := jobs.NewService()
	referenceSvc := reference.NewService()
	obsSvc := observability.NewService()
	models := model.NewService()
	fieldSecurity := securityfields.NewService(policySvc)
	analyticsSvc := analytics.NewService(documents, flows, eventingSvc, searchSvc, audit.NewService(), obsSvc)
	offlineSvc := offline.NewService(modules, nil, searchSvc)
	dataopsSvc := dataops.NewService(cfg, flags, modules, referenceSvc, ident, documents, integrationSvc)
	dataopsSvc.AttachJobs(jobSvc)
	engagementSvc := engagement.NewService()
	engagementSvc.AttachRuntime(eventingSvc, jobSvc)
	searchSvc.AttachSources(documents, models)
	searchSvc.AttachFieldSecurity(fieldSecurity)
	return NewServer(ServerDeps{
		Modules:                   modules,
		Analytics:                 analyticsSvc,
		Templates:                 newTestTemplates(t),
		Workflows:                 flows,
		Identity:                  ident,
		Config:                    cfg,
		Flags:                     flags,
		Integration:               integrationSvc,
		Documents:                 documents,
		DocumentActions:           application.NewDocumentActions(documents, flows, ident, policySvc, application.NewMemorySubmitStore(documents, flows, auditSvc, eventingSvc)),
		Models:                    models,
		Reference:                 referenceSvc,
		Search:                    searchSvc,
		FieldSecurity:             fieldSecurity,
		Policy:                    policySvc,
		Eventing:                  eventingSvc,
		Jobs:                      jobSvc,
		Health:                    health,
		Audit:                     auditSvc,
		Observability:             obsSvc,
		Offline:                   offlineSvc,
		Dataops:                   dataopsSvc,
		Engagement:                engagementSvc,
		AnalyticsStreamPath:       "/mcp/events/analytics/snapshot",
		AnalyticsScopedStreamPath: "/mcp/analytics/events/analytics/snapshot",
	})
}

func newGenericViewServer(t *testing.T) *Server {
	t.Helper()
	modules := newTestModules(t)
	if err := modules.Register(module.Manifest{
		Key: "analytics.generic",
		Frontend: module.FrontendDefinition{
			Views: []module.ViewDefinition{{
				Key: "analytics.generic.view", Title: "Analytics Generic", Kind: "dashboard",
			}},
		},
		MCP: module.MCPDefinition{
			Resources: []module.MCPResourceDefinition{{
				Key: "analytics.generic.app", Title: "Analytics Generic App", URI: "orbyte://apps/analytics.generic", MIMEType: "text/html", Provider: "mcp.app", RequiredPermissions: []string{"analytics.read"}, AppKey: "analytics.generic",
			}},
			Apps: []module.MCPAppDefinition{{
				Key: "analytics.generic", Title: "Analytics Generic", ResourceKey: "analytics.generic.app", ViewKey: "analytics.generic.view", RequiredPermissions: []string{"analytics.read"},
			}},
		},
	}, "system"); err != nil {
		t.Fatalf("register generic manifest failed: %v", err)
	}
	return NewServer(ServerDeps{
		Modules:                   modules,
		Analytics:                 analytics.NewService(document.NewService(), workflow.NewService(), eventing.NewService(), search.NewService(), audit.NewService(), observability.NewService()),
		Templates:                 newTestTemplates(t),
		Workflows:                 workflow.NewService(),
		Identity:                  newTestIdentity(t),
		AnalyticsStreamPath:       "/mcp/events/analytics/snapshot",
		AnalyticsScopedStreamPath: "/mcp/analytics/events/analytics/snapshot",
	})
}

func newBrokenProviderServer(t *testing.T) *Server {
	t.Helper()
	modules := newTestModules(t)
	if err := modules.Register(module.Manifest{
		Key: "analytics.broken",
		MCP: module.MCPDefinition{
			Resources: []module.MCPResourceDefinition{
				{Key: "broken.provider", Title: "Broken Provider", URI: "orbyte://broken/provider", MIMEType: "application/json", Provider: "unsupported.provider", RequiredPermissions: []string{"analytics.read"}},
			},
		},
	}, "system"); err != nil {
		t.Fatalf("register broken manifest failed: %v", err)
	}
	return NewServer(ServerDeps{
		Modules:   modules,
		Analytics: analytics.NewService(document.NewService(), workflow.NewService(), eventing.NewService(), search.NewService(), audit.NewService(), observability.NewService()),
		Templates: newTestTemplates(t),
		Workflows: workflow.NewService(),
		Identity:  newTestIdentity(t),
	})
}

func newUnsupportedToolServer(t *testing.T) *Server {
	t.Helper()
	modules := newTestModules(t)
	if err := modules.Register(module.Manifest{
		Key: "analytics.unsupported",
		MCP: module.MCPDefinition{
			Tools: []module.MCPToolDefinition{{
				Key: "analytics.snapshot.unsupported", Title: "Unsupported", Operation: "unsupported.operation", RequiredPermissions: []string{"analytics.read"},
			}},
		},
	}, "system"); err != nil {
		t.Fatalf("register unsupported tool manifest failed: %v", err)
	}
	return NewServer(ServerDeps{
		Modules:   modules,
		Analytics: analytics.NewService(document.NewService(), workflow.NewService(), eventing.NewService(), search.NewService(), audit.NewService(), observability.NewService()),
		Templates: newTestTemplates(t),
		Workflows: workflow.NewService(),
		Identity:  newTestIdentity(t),
	})
}

func newTestIdentity(t *testing.T) *identity.Service {
	t.Helper()
	return identity.NewService(organization.NewService())
}

func newTestTemplates(t *testing.T) *templateoutput.Service {
	t.Helper()
	svc := templateoutput.NewService(document.NewService(), reporting.NewService(nil))
	if err := svc.RegisterDefinition(templateoutput.Definition{
		Key:           "clinic.registration.print",
		Title:         "Clinic Registration Print",
		TargetKind:    "document",
		TargetKey:     "clinic_registration",
		RendererKind:  "visual",
		DefaultFormat: "html",
		DefaultBody:   `{"schema_version":"visual-grid/v1","title":"Default Registration Template","sections":[{"id":"body","title":"Body","kind":"body","rows":[{"id":"body-row-1","columns":[{"id":"body-cell-1","span":12,"blocks":[{"id":"block-1","type":"text","text":"Default Registration Template"}]}]}]}]}`,
	}); err != nil {
		t.Fatalf("register template definition failed: %v", err)
	}
	return svc
}

func newTestModules(t *testing.T) *module.Service {
	t.Helper()
	modules := module.NewService()
	if err := modules.Register(module.Manifest{
		Key: "analytics",
		Frontend: module.FrontendDefinition{
			CustomEntries: []module.CustomEntryDefinition{{
				Key: "analytics.cockpit", Title: "Analytics Cockpit", RoutePath: "/analytics/cockpit", BundleKey: "analytics-cockpit", ComponentExport: "render",
			}},
			DashboardWidgets: []module.DashboardWidgetDefinition{
				{
					Key:          "analytics.demo.submitted_documents",
					Title:        "Submitted Documents",
					Surface:      module.UISurfaceDashboard,
					RendererKind: "metric",
					DataPath:     "/ui/data/dashboard/demo",
					Metric:       &module.DashboardMetricSpec{ValuePath: "overview.submitted_documents"},
				},
				{
					Key:          "analytics.demo.approval_rate",
					Title:        "Approval Rate",
					Surface:      module.UISurfaceDashboard,
					RendererKind: "gauge",
					DataPath:     "/ui/data/dashboard/demo",
					Gauge:        &module.DashboardGaugeSpec{ValuePath: "overview.approval_rate"},
				},
			},
		},
		Bundles: []module.BundleDefinition{{Key: "analytics-cockpit", Script: "console.log('analytics')"}},
		MCP: module.MCPDefinition{
			Tools: []module.MCPToolDefinition{{
				Key: "analytics.snapshot.get", Title: "Get Analytics Snapshot", Operation: "analytics.snapshot.get", RequiredPermissions: []string{"analytics.read"}, AppKey: "analytics.cockpit",
			}},
			Resources: []module.MCPResourceDefinition{
				{Key: "analytics.snapshot.current", Title: "Current Analytics Snapshot", URI: "orbyte://analytics/snapshot/current", MIMEType: "application/json", Provider: "analytics.snapshot.current", RequiredPermissions: []string{"analytics.read"}},
				{Key: "analytics.cockpit.app", Title: "Analytics Cockpit App", URI: "orbyte://apps/analytics.cockpit", MIMEType: "text/html", Provider: "mcp.app", RequiredPermissions: []string{"analytics.read"}, AppKey: "analytics.cockpit"},
			},
			Apps: []module.MCPAppDefinition{{
				Key: "analytics.cockpit", Title: "Analytics Cockpit", ResourceKey: "analytics.cockpit.app", CustomEntryKey: "analytics.cockpit", RequiredPermissions: []string{"analytics.read"},
			}},
		},
	}, "system"); err != nil {
		t.Fatalf("register manifest failed: %v", err)
	}
	return modules
}

func mustJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	buf, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal params failed: %v", err)
	}
	return buf
}

func contains(items []string, expected string) bool {
	for _, item := range items {
		if item == expected {
			return true
		}
	}
	return false
}

func containsToolNamed(items []ToolDescriptor, expected string) bool {
	for _, item := range items {
		if item.Name == expected {
			return true
		}
	}
	return false
}

func waitForMCPJobStatus(t *testing.T, svc *jobs.Service, jobID string, want string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		job, ok := svc.Get(jobID)
		if ok && job.Status == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	job, _ := svc.Get(jobID)
	t.Fatalf("expected job %s status %s, got %+v", jobID, want, job)
}

func TestInitializedNotificationIsAccepted(t *testing.T) {
	server := NewServer(ServerDeps{})
	resp := server.Handle(context.Background(), JSONRPCRequest{JSONRPC: "2.0", Method: "notifications/initialized"}, ActorContext{})
	if resp.Error != nil {
		t.Fatalf("expected initialized notification to succeed, got %+v", resp.Error)
	}
	if result, ok := resp.Result.(map[string]any); !ok || len(result) != 0 {
		t.Fatalf("expected empty initialized result, got %#v", resp.Result)
	}
}
