package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	application "orbyte/internal/platform/application"
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/organization"
)

func TestUIPosOpenShiftIgnoresClientSuppliedCashierUserID(t *testing.T) {
	org := organization.NewService()
	ident := identity.NewService(org)
	for _, permission := range []identity.Permission{
		{Key: "pos_shift.create", Action: "create", Resource: "pos_shift"},
	} {
		if err := ident.UpsertPermission(permission); err != nil {
			t.Fatalf("upsert permission %s failed: %v", permission.Key, err)
		}
		if err := ident.GrantRolePermission(identity.RolePermission{RoleID: "role_admin", PermissionKey: permission.Key}); err != nil {
			t.Fatalf("grant role_admin permission %s failed: %v", permission.Key, err)
		}
	}

	models := model.NewService()
	for _, def := range []model.Definition{
		{
			Key:         "pos_store",
			DisplayName: "POS Store",
			DefaultSort: "code",
			Fields: []model.FieldDefinition{
				{Key: "code", Type: "string", Required: true},
				{Key: "name", Type: "string"},
				{Key: "warehouse_code", Type: "string"},
				{Key: "currency_code", Type: "string"},
				{Key: "status", Type: "string"},
			},
		},
		{
			Key:         "pos_register",
			DisplayName: "POS Register",
			DefaultSort: "code",
			Fields: []model.FieldDefinition{
				{Key: "code", Type: "string", Required: true},
				{Key: "name", Type: "string"},
				{Key: "store_code", Type: "string", Required: true},
				{Key: "cash_account_code", Type: "string"},
				{Key: "status", Type: "string"},
			},
		},
		{
			Key:         "pos_shift",
			DisplayName: "POS Shift",
			DefaultSort: "shift_number",
			Fields: []model.FieldDefinition{
				{Key: "shift_number", Type: "string"},
				{Key: "store_code", Type: "string"},
				{Key: "register_code", Type: "string"},
				{Key: "cashier_user_id", Type: "string"},
				{Key: "opened_at", Type: "string"},
				{Key: "opening_cash_amount", Type: "number"},
				{Key: "expected_cash_amount", Type: "number"},
				{Key: "actual_cash_amount", Type: "number"},
				{Key: "over_short_amount", Type: "number"},
				{Key: "status", Type: "string"},
				{Key: "notes", Type: "string"},
			},
		},
	} {
		if err := models.Register(def); err != nil {
			t.Fatalf("register model %s failed: %v", def.Key, err)
		}
	}
	if _, err := models.Create("pos_store", "user_admin", map[string]any{
		"code":           "STORE1",
		"name":           "Store 1",
		"warehouse_code": "MAIN",
		"currency_code":  "IDR",
		"status":         "active",
	}); err != nil {
		t.Fatalf("create store failed: %v", err)
	}
	if _, err := models.Create("pos_register", "user_admin", map[string]any{
		"code":              "REG1",
		"name":              "Register 1",
		"store_code":        "STORE1",
		"cash_account_code": "1000-CASH",
		"status":            "active",
	}); err != nil {
		t.Fatalf("create register failed: %v", err)
	}

	posSvc := application.NewPOSCoreService(document.NewService(), models, nil, nil, nil, nil, nil, nil)
	mux := http.NewServeMux()
	registerUIPosRoutes(mux, ident, posSvc)

	adminSession := ident.Sessions()[0]
	body, _ := json.Marshal(map[string]any{
		"store_code":          "STORE1",
		"register_code":       "REG1",
		"opening_cash_amount": 100.0,
		"cashier_user_id":     "user_other",
	})
	req := httptest.NewRequest(http.MethodPost, "/ui/data/pos/shifts/open", bytes.NewReader(body))
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
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Record model.Record `json:"record"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if got := payload.Record.Values["cashier_user_id"]; got != adminSession.UserID {
		t.Fatalf("expected cashier_user_id %s, got %v", adminSession.UserID, got)
	}
}

func TestUIPosTerminalEnterRateLimitsCashierPINFailures(t *testing.T) {
	org := organization.NewService()
	ident := identity.NewService(org)
	for _, permission := range []identity.Permission{
		{Key: "pos_sale.create", Action: "create", Resource: "pos_sale"},
		{Key: "pos_shift.create", Action: "create", Resource: "pos_shift"},
	} {
		if err := ident.UpsertPermission(permission); err != nil {
			t.Fatalf("upsert permission %s failed: %v", permission.Key, err)
		}
		if err := ident.GrantRolePermission(identity.RolePermission{RoleID: "role_admin", PermissionKey: permission.Key}); err != nil {
			t.Fatalf("grant role_admin permission %s failed: %v", permission.Key, err)
		}
	}
	if err := ident.SetCashierPIN("user_admin", "123456"); err != nil {
		t.Fatalf("set cashier PIN failed: %v", err)
	}

	models := model.NewService()
	for _, def := range []model.Definition{
		{
			Key:         "pos_store",
			DisplayName: "POS Store",
			DefaultSort: "code",
			Fields: []model.FieldDefinition{
				{Key: "code", Type: "string", Required: true},
				{Key: "name", Type: "string"},
				{Key: "warehouse_code", Type: "string"},
				{Key: "currency_code", Type: "string"},
				{Key: "status", Type: "string"},
			},
		},
		{
			Key:         "pos_register",
			DisplayName: "POS Register",
			DefaultSort: "code",
			Fields: []model.FieldDefinition{
				{Key: "code", Type: "string", Required: true},
				{Key: "name", Type: "string"},
				{Key: "store_code", Type: "string", Required: true},
				{Key: "cash_account_code", Type: "string"},
				{Key: "status", Type: "string"},
			},
		},
		{
			Key:         "pos_shift",
			DisplayName: "POS Shift",
			DefaultSort: "shift_number",
			Fields: []model.FieldDefinition{
				{Key: "shift_number", Type: "string"},
				{Key: "store_code", Type: "string"},
				{Key: "register_code", Type: "string"},
				{Key: "cashier_user_id", Type: "string"},
				{Key: "opened_at", Type: "string"},
				{Key: "opening_cash_amount", Type: "number"},
				{Key: "expected_cash_amount", Type: "number"},
				{Key: "actual_cash_amount", Type: "number"},
				{Key: "over_short_amount", Type: "number"},
				{Key: "status", Type: "string"},
				{Key: "notes", Type: "string"},
			},
		},
	} {
		if err := models.Register(def); err != nil {
			t.Fatalf("register model %s failed: %v", def.Key, err)
		}
	}
	if _, err := models.Create("pos_store", "user_admin", map[string]any{
		"code":           "STORE1",
		"name":           "Store 1",
		"warehouse_code": "MAIN",
		"currency_code":  "IDR",
		"status":         "active",
	}); err != nil {
		t.Fatalf("create store failed: %v", err)
	}
	if _, err := models.Create("pos_register", "user_admin", map[string]any{
		"code":              "REG1",
		"name":              "Register 1",
		"store_code":        "STORE1",
		"cash_account_code": "1000-CASH",
		"status":            "active",
	}); err != nil {
		t.Fatalf("create register failed: %v", err)
	}

	posSvc := application.NewPOSCoreService(document.NewService(), models, nil, nil, nil, nil, nil, nil)
	shift, err := posSvc.OpenShift("org_default", "loc_hq", "STORE1", "REG1", "user_admin", "user_admin", 100, "")
	if err != nil {
		t.Fatalf("open shift failed: %v", err)
	}
	mux := http.NewServeMux()
	registerUIPosRoutes(mux, ident, posSvc)

	adminSession := ident.Sessions()[0]
	body, _ := json.Marshal(map[string]any{
		"store_code":    "STORE1",
		"register_code": "REG1",
		"shift_id":      shift.ID,
		"pin":           "000000",
	})
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodPost, "/ui/data/pos/terminal/enter", bytes.NewReader(body))
		req.RemoteAddr = "127.0.0.1:12345"
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
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d expected 401, got %d body=%s", i+1, rec.Code, rec.Body.String())
		}
	}
	req := httptest.NewRequest(http.MethodPost, "/ui/data/pos/terminal/enter", bytes.NewReader(body))
	req.RemoteAddr = "127.0.0.1:12345"
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
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected rate-limited attempt to return 403, got %d body=%s", rec.Code, rec.Body.String())
	}
}
