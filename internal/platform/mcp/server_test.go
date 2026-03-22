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
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/observability"
	"orbyte/internal/platform/offline"
	"orbyte/internal/platform/organization"
	"orbyte/internal/platform/policy"
	"orbyte/internal/platform/reference"
	"orbyte/internal/platform/reporting"
	"orbyte/internal/platform/runtimehealth"
	"orbyte/internal/platform/search"
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
	savedDashboard := resp.Result.(map[string]any)["structuredContent"].(analytics.Dashboard)
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

	unavailable := NewServer(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "", "")
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

	server = NewServer(newTestModules(t), nil, newTestTemplates(t), workflow.NewService(), newTestIdentity(t), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "", "")
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
	analyticsSvc := analytics.NewService(documents, flows, eventingSvc, searchSvc, audit.NewService(), obsSvc)
	offlineSvc := offline.NewService(modules, nil, searchSvc)
	dataopsSvc := dataops.NewService(cfg, flags, modules, referenceSvc, ident, documents, integrationSvc)
	dataopsSvc.AttachJobs(jobSvc)
	engagementSvc := engagement.NewService()
	engagementSvc.AttachRuntime(eventingSvc, jobSvc)
	return NewServer(modules, analyticsSvc, newTestTemplates(t), flows, ident, cfg, flags, integrationSvc, referenceSvc, searchSvc, policySvc, eventingSvc, jobSvc, health, auditSvc, obsSvc, offlineSvc, dataopsSvc, engagementSvc, "/mcp/events/analytics/snapshot", "/mcp/analytics/events/analytics/snapshot")
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
	return NewServer(modules, analytics.NewService(document.NewService(), workflow.NewService(), eventing.NewService(), search.NewService(), audit.NewService(), observability.NewService()), newTestTemplates(t), workflow.NewService(), newTestIdentity(t), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "/mcp/events/analytics/snapshot", "/mcp/analytics/events/analytics/snapshot")
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
	return NewServer(modules, analytics.NewService(document.NewService(), workflow.NewService(), eventing.NewService(), search.NewService(), audit.NewService(), observability.NewService()), newTestTemplates(t), workflow.NewService(), newTestIdentity(t), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "", "")
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
	return NewServer(modules, analytics.NewService(document.NewService(), workflow.NewService(), eventing.NewService(), search.NewService(), audit.NewService(), observability.NewService()), newTestTemplates(t), workflow.NewService(), newTestIdentity(t), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "", "")
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
