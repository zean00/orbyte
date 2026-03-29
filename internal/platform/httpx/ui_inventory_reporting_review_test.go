package httpx

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"orbyte/internal/platform/activity"
	application "orbyte/internal/platform/application"
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/organization"
	"orbyte/internal/platform/policy"
	"orbyte/internal/platform/reporting"
	"orbyte/internal/platform/securityfields"
)

func TestUIModelReportingInventoryBatchFiltersBeforePaginationAndResanitizes(t *testing.T) {
	org := organization.NewService()
	ident := identity.NewService(org)
	for _, permission := range []identity.Permission{
		{Key: "inventory_batch.list", Action: "list", Resource: "inventory_batch"},
		{Key: "inventory_batch.read", Action: "read", Resource: "inventory_batch"},
	} {
		if err := ident.UpsertPermission(permission); err != nil {
			t.Fatalf("upsert permission %s failed: %v", permission.Key, err)
		}
		if err := ident.GrantRolePermission(identity.RolePermission{RoleID: "role_admin", PermissionKey: permission.Key}); err != nil {
			t.Fatalf("grant role_admin permission %s failed: %v", permission.Key, err)
		}
	}
	models := model.NewService()
	if err := models.Register(model.Definition{
		Key:               "inventory_batch",
		DisplayName:       "Inventory Batch",
		Version:           "v1",
		ListPermissionKey: "inventory_batch.list",
		ReadPermissionKey: "inventory_batch.read",
		DefaultSort:       "batch_code",
		Fields: []model.FieldDefinition{
			{Key: "item_code", Type: "string"},
			{Key: "warehouse_code", Type: "string"},
			{Key: "batch_code", Type: "string"},
			{Key: "status", Type: "string"},
			{Key: "hold_notes", Type: "string"},
			{Key: "recall_reference", Type: "string"},
		},
	}); err != nil {
		t.Fatalf("register model failed: %v", err)
	}
	for i := 1; i <= 25; i++ {
		status := "active"
		if i > 20 {
			status = "blocked"
		}
		if _, err := models.Create("inventory_batch", "user_admin", map[string]any{
			"item_code":        "ITEM-1",
			"warehouse_code":   "MAIN",
			"batch_code":       "BATCH-" + strconv.Itoa(i),
			"status":           status,
			"hold_notes":       "internal secret",
			"recall_reference": "RECALL-001",
		}); err != nil {
			t.Fatalf("create batch %d failed: %v", i, err)
		}
	}

	policies := policy.NewService()
	if err := policies.Register(policy.HookDefinition{
		Key:           "models.fields.profile",
		Kind:          "security",
		Target:        "model_fields",
		AllowedScopes: []string{"deployment", "location"},
		DefaultRule:   map[string]any{"fields": map[string]any{}},
	}); err != nil {
		t.Fatalf("register policy hook failed: %v", err)
	}
	if err := policies.SetEvaluator("models.fields.profile", func(req policy.Request) policy.Decision {
		return policy.Decision{Allowed: true, Output: map[string]any{
			"fields": map[string]any{
				"hold_notes":       map[string]any{"visible": false},
				"recall_reference": map[string]any{"visible": false},
			},
		}}
	}); err != nil {
		t.Fatalf("set policy evaluator failed: %v", err)
	}
	fieldSecurity := securityfields.NewService(policies)
	inventorySvc := application.NewInventoryCoreService(document.NewService(), nil, models, nil)

	mux := http.NewServeMux()
	registerUIModelReportingRoutes(
		mux,
		ident,
		models,
		activity.NewService(),
		reporting.NewService(models),
		document.NewService(),
		inventorySvc,
		fieldSecurity,
	)

	adminSession := ident.Sessions()[0]
	req := httptest.NewRequest(http.MethodGet, "/ui/data/models?model=inventory_batch&status=blocked&page=1&page_size=20", nil)
	req = req.WithContext(context.WithValue(req.Context(), principalContextKey, principal{
		kind:              userPrincipal,
		userID:            adminSession.UserID,
		effectiveUserID:   adminSession.UserID,
		sessionID:         adminSession.ID,
		currentLocationID: adminSession.CurrentLocationID,
		authMethod:        "test",
	}))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Items []model.Record `json:"items"`
		Total int            `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if payload.Total != 5 {
		t.Fatalf("expected total 5 after filtering, got %d", payload.Total)
	}
	if len(payload.Items) != 5 {
		t.Fatalf("expected 5 filtered items on first page, got %d", len(payload.Items))
	}
	for _, item := range payload.Items {
		if item.Values["status"] != "blocked" {
			t.Fatalf("expected blocked batch, got %+v", item.Values)
		}
		if _, ok := item.Values["hold_notes"]; ok {
			t.Fatalf("expected hold_notes to remain sanitized, got %+v", item.Values)
		}
		if _, ok := item.Values["recall_reference"]; ok {
			t.Fatalf("expected recall_reference to remain sanitized, got %+v", item.Values)
		}
	}
}

func TestUIInventoryBatchTraceRequiresTraceabilityPermission(t *testing.T) {
	org := organization.NewService()
	ident := identity.NewService(org)
	if err := ident.UpsertPermission(identity.Permission{Key: "inventory_batch.read", Action: "read", Resource: "inventory_batch"}); err != nil {
		t.Fatalf("upsert inventory_batch.read failed: %v", err)
	}
	if err := ident.UpsertPermission(identity.Permission{Key: "document.list", Action: "list", Resource: "document"}); err != nil {
		t.Fatalf("upsert document.list failed: %v", err)
	}
	if err := ident.UpsertPermission(identity.Permission{Key: "traceability.read", Action: "read", Resource: "traceability"}); err != nil {
		t.Fatalf("upsert traceability.read failed: %v", err)
	}
	if err := ident.UpsertRole(identity.Role{ID: "role_trace_gap", Key: "trace-gap", Name: "Trace Gap"}); err != nil {
		t.Fatalf("upsert role failed: %v", err)
	}
	for _, permissionKey := range []string{"inventory_batch.read", "document.list"} {
		if err := ident.GrantRolePermission(identity.RolePermission{RoleID: "role_trace_gap", PermissionKey: permissionKey}); err != nil {
			t.Fatalf("grant role permission %s failed: %v", permissionKey, err)
		}
	}
	user, err := ident.CreateUser("trace-gap", "Password123!", "loc_hq", "role_trace_gap", "deployment", "")
	if err != nil {
		t.Fatalf("create user failed: %v", err)
	}

	models := model.NewService()
	if err := models.Register(model.Definition{
		Key:         "inventory_batch",
		DisplayName: "Inventory Batch",
		Version:     "v1",
		Fields: []model.FieldDefinition{
			{Key: "item_code", Type: "string"},
			{Key: "warehouse_code", Type: "string"},
			{Key: "batch_code", Type: "string"},
		},
	}); err != nil {
		t.Fatalf("register model failed: %v", err)
	}
	record, err := models.Create("inventory_batch", "user_admin", map[string]any{
		"item_code":      "ITEM-1",
		"warehouse_code": "MAIN",
		"batch_code":     "B-001",
	})
	if err != nil {
		t.Fatalf("create batch failed: %v", err)
	}
	inventorySvc := application.NewInventoryCoreService(document.NewService(), nil, models, nil)
	traceabilitySvc := application.NewTraceabilityCoreService(document.NewService(), models, inventorySvc)

	mux := http.NewServeMux()
	registerUIInventoryRoutes(mux, ident, inventorySvc, traceabilitySvc)

	req := httptest.NewRequest(http.MethodGet, "/ui/data/inventory/batches/"+record.ID+"/trace", nil)
	req = req.WithContext(context.WithValue(req.Context(), principalContextKey, principal{
		kind:              userPrincipal,
		userID:            user.ID,
		effectiveUserID:   user.ID,
		sessionID:         ident.Sessions()[len(ident.Sessions())-1].ID,
		currentLocationID: "loc_hq",
		authMethod:        "test",
	}))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 without traceability.read, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestUIModelReportingInventoryBatchDetailResanitizesDecoratedRecord(t *testing.T) {
	org := organization.NewService()
	ident := identity.NewService(org)
	for _, permission := range []identity.Permission{
		{Key: "inventory_batch.list", Action: "list", Resource: "inventory_batch"},
		{Key: "inventory_batch.read", Action: "read", Resource: "inventory_batch"},
	} {
		if err := ident.UpsertPermission(permission); err != nil {
			t.Fatalf("upsert permission %s failed: %v", permission.Key, err)
		}
		if err := ident.GrantRolePermission(identity.RolePermission{RoleID: "role_admin", PermissionKey: permission.Key}); err != nil {
			t.Fatalf("grant role_admin permission %s failed: %v", permission.Key, err)
		}
	}
	models := model.NewService()
	if err := models.Register(model.Definition{
		Key:               "inventory_batch",
		DisplayName:       "Inventory Batch",
		Version:           "v1",
		ListPermissionKey: "inventory_batch.list",
		ReadPermissionKey: "inventory_batch.read",
		Fields: []model.FieldDefinition{
			{Key: "item_code", Type: "string"},
			{Key: "warehouse_code", Type: "string"},
			{Key: "batch_code", Type: "string"},
			{Key: "status", Type: "string"},
			{Key: "hold_notes", Type: "string"},
			{Key: "recall_reference", Type: "string"},
			{Key: "expiration_date", Type: "string"},
		},
	}); err != nil {
		t.Fatalf("register model failed: %v", err)
	}
	record, err := models.Create("inventory_batch", "user_admin", map[string]any{
		"item_code":        "ITEM-1",
		"warehouse_code":   "MAIN",
		"batch_code":       "B-001",
		"status":           "recalled",
		"hold_notes":       "internal secret",
		"recall_reference": "RECALL-001",
		"expiration_date":  time.Now().UTC().AddDate(0, 0, 10).Format(time.DateOnly),
	})
	if err != nil {
		t.Fatalf("create batch failed: %v", err)
	}

	policies := policy.NewService()
	if err := policies.Register(policy.HookDefinition{
		Key:           "models.fields.profile",
		Kind:          "security",
		Target:        "model_fields",
		AllowedScopes: []string{"deployment", "location"},
		DefaultRule:   map[string]any{"fields": map[string]any{}},
	}); err != nil {
		t.Fatalf("register policy hook failed: %v", err)
	}
	if err := policies.SetEvaluator("models.fields.profile", func(req policy.Request) policy.Decision {
		return policy.Decision{Allowed: true, Output: map[string]any{
			"fields": map[string]any{
				"hold_notes":       map[string]any{"visible": false},
				"recall_reference": map[string]any{"visible": false},
			},
		}}
	}); err != nil {
		t.Fatalf("set policy evaluator failed: %v", err)
	}
	fieldSecurity := securityfields.NewService(policies)
	inventorySvc := application.NewInventoryCoreService(document.NewService(), nil, models, nil)

	mux := http.NewServeMux()
	registerUIModelReportingRoutes(
		mux,
		ident,
		models,
		activity.NewService(),
		reporting.NewService(models),
		document.NewService(),
		inventorySvc,
		fieldSecurity,
	)

	adminSession := ident.Sessions()[0]
	req := httptest.NewRequest(http.MethodGet, "/ui/data/models/inventory_batch/"+record.ID, nil)
	req = req.WithContext(context.WithValue(req.Context(), principalContextKey, principal{
		kind:              userPrincipal,
		userID:            adminSession.UserID,
		effectiveUserID:   adminSession.UserID,
		sessionID:         adminSession.ID,
		currentLocationID: adminSession.CurrentLocationID,
		authMethod:        "test",
	}))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Record model.Record `json:"record"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if _, ok := payload.Record.Values["hold_notes"]; ok {
		t.Fatalf("expected hold_notes to remain sanitized, got %+v", payload.Record.Values)
	}
	if _, ok := payload.Record.Values["recall_reference"]; ok {
		t.Fatalf("expected recall_reference to remain sanitized, got %+v", payload.Record.Values)
	}
}
