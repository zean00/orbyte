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

func TestServerExpandedControlPlaneCoverage(t *testing.T) {
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
				"name": "Ops Dashboard",
				"widgets": []map[string]any{{"title": "Submitted", "kind": "chart", "query_id": query.ID}},
			}},
		}),
	}, actor)
	if resp.Error != nil {
		t.Fatalf("analytics.dashboard.save failed: %+v", resp.Error)
	}
	dashboard := resp.Result.(map[string]any)["structuredContent"].(analytics.Dashboard)

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
				"hook_key":      "documents.search.visibility",
				"scope":         "location",
				"scope_id":      "loc_hq",
				"source":        `package orbyte.policy.documents.search.visibility

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

	unavailable := NewServer(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "", "", nil)
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

	server = NewServer(newTestModules(t), nil, newTestTemplates(t), workflow.NewService(), newTestIdentity(t), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "", "", nil)
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
	return NewServer(modules, analyticsSvc, newTestTemplates(t), flows, ident, cfg, flags, integrationSvc, referenceSvc, searchSvc, policySvc, eventingSvc, jobSvc, health, auditSvc, obsSvc, offlineSvc, dataopsSvc, engagementSvc, "/mcp/events/analytics/snapshot", "/mcp/analytics/events/analytics/snapshot", nil)
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
	return NewServer(modules, analytics.NewService(document.NewService(), workflow.NewService(), eventing.NewService(), search.NewService(), audit.NewService(), observability.NewService()), newTestTemplates(t), workflow.NewService(), newTestIdentity(t), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "/mcp/events/analytics/snapshot", "/mcp/analytics/events/analytics/snapshot", nil)
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
	return NewServer(modules, analytics.NewService(document.NewService(), workflow.NewService(), eventing.NewService(), search.NewService(), audit.NewService(), observability.NewService()), newTestTemplates(t), workflow.NewService(), newTestIdentity(t), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "", "", nil)
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
	return NewServer(modules, analytics.NewService(document.NewService(), workflow.NewService(), eventing.NewService(), search.NewService(), audit.NewService(), observability.NewService()), newTestTemplates(t), workflow.NewService(), newTestIdentity(t), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "", "", nil)
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
