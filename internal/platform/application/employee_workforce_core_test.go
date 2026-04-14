package application

import (
	"testing"
	"time"

	"orbyte/internal/platform/model"
)

func TestEmployeeWorkforceResolveEmployeeByUserAndCurrentAssignment(t *testing.T) {
	models := model.NewService()
	mustRegisterEmployeeWorkforceModels(t, models)

	if _, err := models.Create("party", "user_admin", map[string]any{
		"name":   "Jane Operator",
		"status": "active",
	}); err != nil {
		t.Fatalf("create party seed: %v", err)
	}

	inactive, err := models.Create("employee_profile", "user_admin", map[string]any{
		"party_id":           "party_old",
		"user_id":            "user_jane",
		"employee_code":      "EMP-OLD",
		"employment_status":  "inactive",
		"status":             "inactive",
		"organization_id":    "org_default",
		"department_id":      "dept_old",
		"cost_center_id":     "cc_old",
		"organization_unit_id": "unit_old",
	})
	if err != nil {
		t.Fatalf("create inactive employee: %v", err)
	}
	active, err := models.Create("employee_profile", "user_admin", map[string]any{
		"party_id":             "party_active",
		"user_id":              "user_jane",
		"employee_code":        "EMP-NEW",
		"employment_status":    "active",
		"status":               "active",
		"organization_id":      "org_default",
		"department_id":        "dept_current",
		"cost_center_id":       "cc_current",
		"organization_unit_id": "unit_current",
	})
	if err != nil {
		t.Fatalf("create active employee: %v", err)
	}

	if _, err := models.Create("employee_assignment", "user_admin", map[string]any{
		"employee_id":          active.ID,
		"organization_id":      "org_default",
		"location_id":          "loc_main",
		"department_id":        "dept_old",
		"assignment_type":      "primary",
		"effective_from":       "2026-01-01",
		"effective_to":         "2026-02-15",
		"status":               "active",
	}); err != nil {
		t.Fatalf("create historical assignment: %v", err)
	}
	current, err := models.Create("employee_assignment", "user_admin", map[string]any{
		"employee_id":          active.ID,
		"organization_id":      "org_default",
		"location_id":          "loc_main",
		"department_id":        "dept_current",
		"cost_center_id":       "cc_current",
		"assignment_type":      "primary",
		"effective_from":       "2026-02-16",
		"status":               "active",
	})
	if err != nil {
		t.Fatalf("create current assignment: %v", err)
	}
	if _, err := models.Create("employee_assignment", "user_admin", map[string]any{
		"employee_id":          inactive.ID,
		"organization_id":      "org_default",
		"location_id":          "loc_other",
		"department_id":        "dept_inactive",
		"assignment_type":      "primary",
		"effective_from":       "2026-03-01",
		"status":               "inactive",
	}); err != nil {
		t.Fatalf("create inactive assignment: %v", err)
	}

	service := NewEmployeeWorkforceCoreService(models)

	employee, ok, err := service.ResolveEmployeeByUser("user_jane")
	if err != nil {
		t.Fatalf("resolve employee by user: %v", err)
	}
	if !ok {
		t.Fatal("expected employee for linked user")
	}
	if employee.ID != active.ID {
		t.Fatalf("expected active employee %s, got %s", active.ID, employee.ID)
	}

	asOf, _ := time.Parse("2006-01-02", "2026-03-31")
	assignment, ok, err := service.ResolveCurrentAssignment(active.ID, asOf)
	if err != nil {
		t.Fatalf("resolve current assignment: %v", err)
	}
	if !ok {
		t.Fatal("expected current assignment")
	}
	if assignment.ID != current.ID {
		t.Fatalf("expected current assignment %s, got %s", current.ID, assignment.ID)
	}
	if got := assignment.Values["department_id"]; got != "dept_current" {
		t.Fatalf("expected current department dept_current, got %v", got)
	}
}

func TestEmployeeWorkforceResolveEmployeeByUserReturnsMissWhenOnlyInactiveProfilesExist(t *testing.T) {
	models := model.NewService()
	mustRegisterEmployeeWorkforceModels(t, models)

	if _, err := models.Create("employee_profile", "user_admin", map[string]any{
		"party_id":          "party_old",
		"user_id":           "user_former",
		"employee_code":     "EMP-OLD",
		"employment_status": "terminated",
		"status":            "inactive",
	}); err != nil {
		t.Fatalf("create inactive employee: %v", err)
	}

	service := NewEmployeeWorkforceCoreService(models)
	employee, ok, err := service.ResolveEmployeeByUser("user_former")
	if err != nil {
		t.Fatalf("resolve employee by user: %v", err)
	}
	if ok {
		t.Fatalf("expected inactive-only employee lookup to miss, got %+v", employee)
	}
}

func TestEmployeeWorkforceResolveCurrentAssignmentReturnsMissWhenNoAssignmentIsCurrent(t *testing.T) {
	models := model.NewService()
	mustRegisterEmployeeWorkforceModels(t, models)

	employee, err := models.Create("employee_profile", "user_admin", map[string]any{
		"party_id":          "party_active",
		"user_id":           "user_jane",
		"employee_code":     "EMP-NEW",
		"employment_status": "active",
		"status":            "active",
	})
	if err != nil {
		t.Fatalf("create employee: %v", err)
	}
	if _, err := models.Create("employee_assignment", "user_admin", map[string]any{
		"employee_id":     employee.ID,
		"organization_id": "org_default",
		"assignment_type": "primary",
		"effective_from":  "2026-04-01",
		"status":          "active",
	}); err != nil {
		t.Fatalf("create future assignment: %v", err)
	}
	if _, err := models.Create("employee_assignment", "user_admin", map[string]any{
		"employee_id":     employee.ID,
		"organization_id": "org_default",
		"assignment_type": "primary",
		"effective_from":  "2026-01-01",
		"effective_to":    "2026-02-01",
		"status":          "inactive",
	}); err != nil {
		t.Fatalf("create inactive assignment: %v", err)
	}

	service := NewEmployeeWorkforceCoreService(models)
	asOf, _ := time.Parse("2006-01-02", "2026-03-31")
	assignment, ok, err := service.ResolveCurrentAssignment(employee.ID, asOf)
	if err != nil {
		t.Fatalf("resolve current assignment: %v", err)
	}
	if ok {
		t.Fatalf("expected no current assignment, got %+v", assignment)
	}
}

func mustRegisterEmployeeWorkforceModels(t *testing.T, models *model.Service) {
	t.Helper()
	for _, def := range []model.Definition{
		{Key: "party", DisplayName: "Party", DefaultSort: "name", Fields: []model.FieldDefinition{{Key: "name", Type: "string"}, {Key: "status", Type: "string"}}},
		{Key: "employee_profile", DisplayName: "Employee Profile", DefaultSort: "employee_code", Fields: []model.FieldDefinition{{Key: "party_id", Type: "string"}, {Key: "user_id", Type: "string"}, {Key: "employee_code", Type: "string"}, {Key: "employment_status", Type: "string"}, {Key: "status", Type: "string"}, {Key: "organization_id", Type: "string"}, {Key: "department_id", Type: "string"}, {Key: "cost_center_id", Type: "string"}, {Key: "organization_unit_id", Type: "string"}}},
		{Key: "employee_assignment", DisplayName: "Employee Assignment", DefaultSort: "effective_from", Fields: []model.FieldDefinition{{Key: "employee_id", Type: "string"}, {Key: "organization_id", Type: "string"}, {Key: "location_id", Type: "string"}, {Key: "department_id", Type: "string"}, {Key: "cost_center_id", Type: "string"}, {Key: "assignment_type", Type: "string"}, {Key: "effective_from", Type: "string"}, {Key: "effective_to", Type: "string"}, {Key: "status", Type: "string"}}},
	} {
		if err := models.Register(def); err != nil {
			t.Fatalf("register %s: %v", def.Key, err)
		}
	}
}
