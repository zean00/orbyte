package application

import (
	"errors"
	"testing"
	"time"

	"orbyte/internal/platform/model"
)

func TestWorkforceAttendanceRecordEventAndSyncDay(t *testing.T) {
	models := model.NewService()
	registerWorkforceAttendanceTestModels(t, models)
	workforce := NewEmployeeWorkforceCoreService(models)
	service := NewWorkforceAttendanceCoreService(models, workforce)

	employee, err := models.Create("employee_profile", "user_admin", map[string]any{
		"party_id":          "party_emp",
		"user_id":           "user_cashier",
		"employee_code":     "EMP-001",
		"employment_status": "active",
		"status":            "active",
	})
	if err != nil {
		t.Fatalf("create employee: %v", err)
	}
	roster, err := models.Create("workforce_roster", "user_admin", map[string]any{
		"code":            "RST-1",
		"name":            "Roster 1",
		"organization_id": "org_default",
		"location_id":     "loc_hq",
		"start_date":      "2026-04-01",
		"end_date":        "2026-04-01",
		"status":          "published",
	})
	if err != nil {
		t.Fatalf("create roster: %v", err)
	}
	if _, err := models.Create("workforce_roster_slot", "user_admin", map[string]any{
		"roster_id":               roster.ID,
		"employee_id":             employee.ID,
		"organization_id":         "org_default",
		"location_id":             "loc_hq",
		"shift_date":              "2026-04-01",
		"planned_start_time":      "08:00",
		"planned_end_time":        "16:00",
		"late_grace_minutes":      5,
		"early_out_grace_minutes": 5,
		"store_code":              "STORE1",
		"register_code":           "REG1",
		"status":                  "active",
	}); err != nil {
		t.Fatalf("create slot: %v", err)
	}

	clockInAt := time.Date(2026, 4, 1, 8, 10, 0, 0, time.UTC)
	event, day, err := service.RecordAttendanceEvent(AttendanceEventInput{
		EmployeeID: employee.ID,
		EventType:  "clock_in",
		OccurredAt: clockInAt,
		Source:     "manual",
		Scope: AttendanceScope{
			OrganizationID: "org_default",
			LocationID:     "loc_hq",
			StoreCode:      "STORE1",
			RegisterCode:   "REG1",
		},
	}, "user_admin")
	if err != nil {
		t.Fatalf("record clock in: %v", err)
	}
	if event.ID == "" || textValue(event.Values["roster_slot_id"]) == "" {
		t.Fatalf("expected event to resolve roster slot, got %+v", event.Values)
	}
	if got := textValue(day.Values["attendance_status"]); got != "late" {
		t.Fatalf("expected late after first clock in, got %s", got)
	}

	_, day, err = service.RecordAttendanceEvent(AttendanceEventInput{
		EmployeeID: employee.ID,
		EventType:  "clock_out",
		OccurredAt: time.Date(2026, 4, 1, 17, 0, 0, 0, time.UTC),
		Source:     "manual",
		Scope: AttendanceScope{
			OrganizationID: "org_default",
			LocationID:     "loc_hq",
			StoreCode:      "STORE1",
			RegisterCode:   "REG1",
		},
	}, "user_admin")
	if err != nil {
		t.Fatalf("record clock out: %v", err)
	}
	if got := textValue(day.Values["attendance_status"]); got != "late" {
		t.Fatalf("expected attendance day to remain late, got %s", got)
	}
	if worked := numberValue(day.Values["worked_hours"]); worked <= 0 {
		t.Fatalf("expected worked hours, got %v", day.Values["worked_hours"])
	}
	if overtime := numberValue(day.Values["overtime_hours"]); overtime <= 0 {
		t.Fatalf("expected overtime hours, got %v", day.Values["overtime_hours"])
	}
}

func TestWorkforceAttendanceSyncDayPrefersApprovedLeave(t *testing.T) {
	models := model.NewService()
	registerWorkforceAttendanceTestModels(t, models)
	workforce := NewEmployeeWorkforceCoreService(models)
	service := NewWorkforceAttendanceCoreService(models, workforce)

	employee, err := models.Create("employee_profile", "user_admin", map[string]any{
		"party_id":          "party_emp",
		"user_id":           "user_leave",
		"employee_code":     "EMP-002",
		"employment_status": "active",
		"status":            "active",
	})
	if err != nil {
		t.Fatalf("create employee: %v", err)
	}
	absenceCode, err := models.Create("absence_code", "user_admin", map[string]any{
		"code":   "ANNUAL",
		"name":   "Annual Leave",
		"status": "active",
	})
	if err != nil {
		t.Fatalf("create absence code: %v", err)
	}
	if _, err := models.Create("leave_request", "user_admin", map[string]any{
		"employee_id":     employee.ID,
		"absence_code_id": absenceCode.ID,
		"start_date":      "2026-04-02",
		"end_date":        "2026-04-02",
		"approval_status": "approved",
		"organization_id": "org_default",
		"location_id":     "loc_hq",
		"status":          "active",
	}); err != nil {
		t.Fatalf("create leave request: %v", err)
	}

	day, err := service.SyncAttendanceDay(employee.ID, time.Date(2026, 4, 2, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("sync attendance day: %v", err)
	}
	if got := textValue(day.Values["attendance_status"]); got != "on_leave" {
		t.Fatalf("expected on_leave, got %s", got)
	}
}

func TestPOSOpenShiftLinksAttendanceContext(t *testing.T) {
	models := model.NewService()
	registerWorkforceAttendanceTestModels(t, models)
	registerPOSTestModels(t, models)
	workforce := NewEmployeeWorkforceCoreService(models)
	attendance := NewWorkforceAttendanceCoreService(models, workforce)

	employee, err := models.Create("employee_profile", "user_admin", map[string]any{
		"party_id":          "party_emp",
		"user_id":           "cashier_1",
		"employee_code":     "EMP-003",
		"employment_status": "active",
		"status":            "active",
	})
	if err != nil {
		t.Fatalf("create employee: %v", err)
	}
	roster, _ := models.Create("workforce_roster", "user_admin", map[string]any{
		"code":            "RST-2",
		"name":            "Roster 2",
		"organization_id": "org_default",
		"location_id":     "loc_main",
		"start_date":      time.Now().UTC().Format("2006-01-02"),
		"end_date":        time.Now().UTC().Format("2006-01-02"),
		"status":          "published",
	})
	if _, err := models.Create("workforce_roster_slot", "user_admin", map[string]any{
		"roster_id":          roster.ID,
		"employee_id":        employee.ID,
		"organization_id":    "org_default",
		"location_id":        "loc_main",
		"shift_date":         time.Now().UTC().Format("2006-01-02"),
		"planned_start_time": time.Now().UTC().Add(-30 * time.Minute).Format("15:04"),
		"planned_end_time":   time.Now().UTC().Add(7 * time.Hour).Format("15:04"),
		"store_code":         "STORE1",
		"register_code":      "REG1",
		"status":             "active",
	}); err != nil {
		t.Fatalf("create roster slot: %v", err)
	}
	if _, err := models.Create("pos_store", "user_admin", map[string]any{"code": "STORE1", "name": "Store 1", "warehouse_code": "WH1", "status": "active"}); err != nil {
		t.Fatalf("create store: %v", err)
	}
	if _, err := models.Create("pos_register", "user_admin", map[string]any{"code": "REG1", "name": "Register 1", "store_code": "STORE1", "status": "active"}); err != nil {
		t.Fatalf("create register: %v", err)
	}

	pos := NewPOSCoreService(nil, models, nil, nil, nil, nil, nil, nil)
	pos.AttachWorkforceAttendance(attendance)

	shift, err := pos.OpenShift("org_default", "loc_main", "STORE1", "REG1", "cashier_1", "cashier_1", 100, "opening")
	if err != nil {
		t.Fatalf("open shift: %v", err)
	}
	if textValue(shift.Values["cashier_employee_id"]) != employee.ID {
		t.Fatalf("expected cashier employee id %s, got %s", employee.ID, textValue(shift.Values["cashier_employee_id"]))
	}
	if textValue(shift.Values["roster_slot_id"]) == "" || textValue(shift.Values["attendance_day_id"]) == "" {
		t.Fatalf("expected shift to link roster slot and attendance day, got %+v", shift.Values)
	}
}

func TestWorkforceAttendanceUsesRosterShiftDateForOvernightEvents(t *testing.T) {
	models := model.NewService()
	registerWorkforceAttendanceTestModels(t, models)
	workforce := NewEmployeeWorkforceCoreService(models)
	service := NewWorkforceAttendanceCoreService(models, workforce)

	employee, err := models.Create("employee_profile", "user_admin", map[string]any{
		"party_id":          "party_emp",
		"user_id":           "user_night",
		"employee_code":     "EMP-004",
		"employment_status": "active",
		"status":            "active",
	})
	if err != nil {
		t.Fatalf("create employee: %v", err)
	}
	roster, err := models.Create("workforce_roster", "user_admin", map[string]any{
		"code":            "RST-OVN",
		"name":            "Overnight Roster",
		"organization_id": "org_default",
		"location_id":     "loc_hq",
		"start_date":      "2026-04-01",
		"end_date":        "2026-04-01",
		"status":          "published",
	})
	if err != nil {
		t.Fatalf("create roster: %v", err)
	}
	slot, err := models.Create("workforce_roster_slot", "user_admin", map[string]any{
		"roster_id":               roster.ID,
		"employee_id":             employee.ID,
		"organization_id":         "org_default",
		"location_id":             "loc_hq",
		"shift_date":              "2026-04-01",
		"planned_start_time":      "22:00",
		"planned_end_time":        "06:00",
		"overnight":               true,
		"late_grace_minutes":      5,
		"early_out_grace_minutes": 5,
		"status":                  "active",
	})
	if err != nil {
		t.Fatalf("create slot: %v", err)
	}

	if _, _, err := service.RecordAttendanceEvent(AttendanceEventInput{
		EmployeeID:   employee.ID,
		EventType:    "clock_in",
		OccurredAt:   time.Date(2026, 4, 1, 22, 5, 0, 0, time.UTC),
		Source:       "manual",
		RosterSlotID: slot.ID,
	}, "user_admin"); err != nil {
		t.Fatalf("record clock in: %v", err)
	}
	_, day, err := service.RecordAttendanceEvent(AttendanceEventInput{
		EmployeeID:   employee.ID,
		EventType:    "clock_out",
		OccurredAt:   time.Date(2026, 4, 2, 6, 10, 0, 0, time.UTC),
		Source:       "manual",
		RosterSlotID: slot.ID,
	}, "user_admin")
	if err != nil {
		t.Fatalf("record clock out: %v", err)
	}
	if got := textValue(day.Values["attendance_date"]); got != "2026-04-01" {
		t.Fatalf("expected overnight attendance day 2026-04-01, got %s", got)
	}
	if got := textValue(day.Values["actual_out_at"]); got != "2026-04-02T06:10:00Z" {
		t.Fatalf("expected overnight actual_out_at on next day, got %s", got)
	}
	events, _, err := models.List("attendance_event", model.Query{
		Filters:  map[string]string{"employee_id": employee.ID},
		Page:     1,
		PageSize: model.MaxPageSize,
	})
	if err != nil {
		t.Fatalf("list attendance events: %v", err)
	}
	for _, item := range events {
		if got := textValue(item.Values["attendance_date"]); got != "2026-04-01" {
			t.Fatalf("expected all overnight events on 2026-04-01, got %s for %s", got, item.ID)
		}
	}
}

func TestWorkforceAttendanceIgnoresInactiveApprovedLeaveAndOvertime(t *testing.T) {
	models := model.NewService()
	registerWorkforceAttendanceTestModels(t, models)
	workforce := NewEmployeeWorkforceCoreService(models)
	service := NewWorkforceAttendanceCoreService(models, workforce)

	employee, err := models.Create("employee_profile", "user_admin", map[string]any{
		"party_id":          "party_emp",
		"user_id":           "user_inactive",
		"employee_code":     "EMP-005",
		"employment_status": "active",
		"status":            "active",
	})
	if err != nil {
		t.Fatalf("create employee: %v", err)
	}
	roster, err := models.Create("workforce_roster", "user_admin", map[string]any{
		"code":            "RST-3",
		"name":            "Roster 3",
		"organization_id": "org_default",
		"location_id":     "loc_hq",
		"start_date":      "2026-04-03",
		"end_date":        "2026-04-03",
		"status":          "published",
	})
	if err != nil {
		t.Fatalf("create roster: %v", err)
	}
	if _, err := models.Create("workforce_roster_slot", "user_admin", map[string]any{
		"roster_id":               roster.ID,
		"employee_id":             employee.ID,
		"organization_id":         "org_default",
		"location_id":             "loc_hq",
		"shift_date":              "2026-04-03",
		"planned_start_time":      "08:00",
		"planned_end_time":        "16:00",
		"late_grace_minutes":      5,
		"early_out_grace_minutes": 5,
		"status":                  "active",
	}); err != nil {
		t.Fatalf("create roster slot: %v", err)
	}
	absenceCode, err := models.Create("absence_code", "user_admin", map[string]any{
		"code":   "SICK",
		"name":   "Sick Leave",
		"status": "active",
	})
	if err != nil {
		t.Fatalf("create absence code: %v", err)
	}
	if _, err := models.Create("leave_request", "user_admin", map[string]any{
		"employee_id":     employee.ID,
		"absence_code_id": absenceCode.ID,
		"start_date":      "2026-04-03",
		"end_date":        "2026-04-03",
		"approval_status": "approved",
		"organization_id": "org_default",
		"location_id":     "loc_hq",
		"status":          "inactive",
	}); err != nil {
		t.Fatalf("create inactive leave request: %v", err)
	}
	if _, err := models.Create("overtime_request", "user_admin", map[string]any{
		"employee_id":     employee.ID,
		"attendance_date": "2026-04-03",
		"approved_hours":  5.0,
		"approval_status": "approved",
		"status":          "inactive",
	}); err != nil {
		t.Fatalf("create inactive overtime request: %v", err)
	}
	if _, _, err := service.RecordAttendanceEvent(AttendanceEventInput{
		EmployeeID: employee.ID,
		EventType:  "clock_in",
		OccurredAt: time.Date(2026, 4, 3, 8, 0, 0, 0, time.UTC),
		Source:     "manual",
	}, "user_admin"); err != nil {
		t.Fatalf("record clock in: %v", err)
	}
	_, day, err := service.RecordAttendanceEvent(AttendanceEventInput{
		EmployeeID: employee.ID,
		EventType:  "clock_out",
		OccurredAt: time.Date(2026, 4, 3, 16, 30, 0, 0, time.UTC),
		Source:     "manual",
	}, "user_admin")
	if err != nil {
		t.Fatalf("record clock out: %v", err)
	}
	if got := textValue(day.Values["attendance_status"]); got == "on_leave" {
		t.Fatalf("expected inactive leave to be ignored, got %s", got)
	}
	if got := numberValue(day.Values["overtime_hours"]); got >= 5.0 {
		t.Fatalf("expected inactive overtime to be ignored, got %v", got)
	}
}

func TestPOSOpenShiftRollsBackAttendanceWhenShiftLinkSaveFails(t *testing.T) {
	repo := &failingAttendanceLinkRepo{Repository: model.NewMemoryRepository()}
	models := model.NewServiceWithRepository(repo)
	registerWorkforceAttendanceTestModels(t, models)
	registerPOSTestModels(t, models)
	workforce := NewEmployeeWorkforceCoreService(models)
	attendance := NewWorkforceAttendanceCoreService(models, workforce)

	employee, err := models.Create("employee_profile", "user_admin", map[string]any{
		"party_id":          "party_emp",
		"user_id":           "cashier_fail",
		"employee_code":     "EMP-006",
		"employment_status": "active",
		"status":            "active",
	})
	if err != nil {
		t.Fatalf("create employee: %v", err)
	}
	roster, _ := models.Create("workforce_roster", "user_admin", map[string]any{
		"code":            "RST-FAIL",
		"name":            "Rollback Roster",
		"organization_id": "org_default",
		"location_id":     "loc_main",
		"start_date":      time.Now().UTC().Format("2006-01-02"),
		"end_date":        time.Now().UTC().Format("2006-01-02"),
		"status":          "published",
	})
	if _, err := models.Create("workforce_roster_slot", "user_admin", map[string]any{
		"roster_id":          roster.ID,
		"employee_id":        employee.ID,
		"organization_id":    "org_default",
		"location_id":        "loc_main",
		"shift_date":         time.Now().UTC().Format("2006-01-02"),
		"planned_start_time": time.Now().UTC().Add(-30 * time.Minute).Format("15:04"),
		"planned_end_time":   time.Now().UTC().Add(7 * time.Hour).Format("15:04"),
		"store_code":         "STORE1",
		"register_code":      "REG1",
		"status":             "active",
	}); err != nil {
		t.Fatalf("create roster slot: %v", err)
	}
	if _, err := models.Create("pos_store", "user_admin", map[string]any{"code": "STORE1", "name": "Store 1", "warehouse_code": "WH1", "status": "active"}); err != nil {
		t.Fatalf("create store: %v", err)
	}
	if _, err := models.Create("pos_register", "user_admin", map[string]any{"code": "REG1", "name": "Register 1", "store_code": "STORE1", "status": "active"}); err != nil {
		t.Fatalf("create register: %v", err)
	}

	pos := NewPOSCoreService(nil, models, nil, nil, nil, nil, nil, nil)
	pos.AttachWorkforceAttendance(attendance)

	if _, err := pos.OpenShift("org_default", "loc_main", "STORE1", "REG1", "cashier_fail", "cashier_fail", 100, "opening"); err == nil {
		t.Fatalf("expected open shift to fail when linking attendance context")
	}
	if items, _, err := models.List("attendance_event", model.Query{Page: 1, PageSize: model.MaxPageSize}); err != nil {
		t.Fatalf("list attendance events: %v", err)
	} else if len(items) != 0 {
		t.Fatalf("expected no attendance events after rollback, got %d", len(items))
	}
	if items, _, err := models.List("pos_shift", model.Query{Page: 1, PageSize: model.MaxPageSize}); err != nil {
		t.Fatalf("list pos shifts: %v", err)
	} else if len(items) != 0 {
		t.Fatalf("expected no pos shifts after rollback, got %d", len(items))
	}
}

func registerWorkforceAttendanceTestModels(t *testing.T, models *model.Service) {
	t.Helper()
	defs := []model.Definition{
		{Key: "employee_profile", DisplayName: "Employee Profile", DefaultSort: "employee_code", Fields: []model.FieldDefinition{{Key: "party_id", Type: "string"}, {Key: "user_id", Type: "string"}, {Key: "employee_code", Type: "string"}, {Key: "employment_status", Type: "string"}, {Key: "status", Type: "string"}}},
		{Key: "workforce_roster", DisplayName: "Workforce Roster", DefaultSort: "start_date", Fields: []model.FieldDefinition{{Key: "code", Type: "string"}, {Key: "name", Type: "string"}, {Key: "organization_id", Type: "string"}, {Key: "location_id", Type: "string"}, {Key: "start_date", Type: "string"}, {Key: "end_date", Type: "string"}, {Key: "status", Type: "string"}}},
		{Key: "workforce_roster_slot", DisplayName: "Roster Slot", DefaultSort: "shift_date", Fields: []model.FieldDefinition{{Key: "roster_id", Type: "string"}, {Key: "employee_id", Type: "string"}, {Key: "organization_id", Type: "string"}, {Key: "location_id", Type: "string"}, {Key: "shift_date", Type: "string"}, {Key: "planned_start_time", Type: "string"}, {Key: "planned_end_time", Type: "string"}, {Key: "late_grace_minutes", Type: "number"}, {Key: "early_out_grace_minutes", Type: "number"}, {Key: "store_code", Type: "string"}, {Key: "register_code", Type: "string"}, {Key: "status", Type: "string"}, {Key: "overnight", Type: "bool"}, {Key: "shift_template_id", Type: "string"}}},
		{Key: "attendance_event", DisplayName: "Attendance Event", DefaultSort: "occurred_at", Fields: []model.FieldDefinition{{Key: "employee_id", Type: "string"}, {Key: "attendance_date", Type: "string"}, {Key: "event_type", Type: "string"}, {Key: "occurred_at", Type: "string"}, {Key: "source", Type: "string"}, {Key: "organization_id", Type: "string"}, {Key: "location_id", Type: "string"}, {Key: "store_code", Type: "string"}, {Key: "register_code", Type: "string"}, {Key: "warehouse_code", Type: "string"}, {Key: "work_center_code", Type: "string"}, {Key: "roster_slot_id", Type: "string"}, {Key: "attendance_day_id", Type: "string"}, {Key: "notes", Type: "string"}, {Key: "status", Type: "string"}}},
		{Key: "attendance_day", DisplayName: "Attendance Day", DefaultSort: "attendance_date", Fields: []model.FieldDefinition{{Key: "employee_id", Type: "string"}, {Key: "attendance_date", Type: "string"}, {Key: "roster_slot_id", Type: "string"}, {Key: "shift_template_id", Type: "string"}, {Key: "planned_start_at", Type: "string"}, {Key: "planned_end_at", Type: "string"}, {Key: "actual_in_at", Type: "string"}, {Key: "actual_out_at", Type: "string"}, {Key: "break_minutes", Type: "number"}, {Key: "worked_hours", Type: "number"}, {Key: "late_minutes", Type: "number"}, {Key: "early_out_minutes", Type: "number"}, {Key: "overtime_hours", Type: "number"}, {Key: "attendance_status", Type: "string"}, {Key: "absence_code_id", Type: "string"}, {Key: "leave_request_id", Type: "string"}, {Key: "overtime_request_id", Type: "string"}, {Key: "attendance_adjustment_id", Type: "string"}, {Key: "overnight_shift", Type: "bool"}, {Key: "organization_id", Type: "string"}, {Key: "location_id", Type: "string"}, {Key: "status", Type: "string"}}},
		{Key: "absence_code", DisplayName: "Absence Code", DefaultSort: "code", Fields: []model.FieldDefinition{{Key: "code", Type: "string"}, {Key: "name", Type: "string"}, {Key: "status", Type: "string"}}},
		{Key: "leave_request", DisplayName: "Leave Request", DefaultSort: "start_date", Fields: []model.FieldDefinition{{Key: "employee_id", Type: "string"}, {Key: "absence_code_id", Type: "string"}, {Key: "start_date", Type: "string"}, {Key: "end_date", Type: "string"}, {Key: "approval_status", Type: "string"}, {Key: "organization_id", Type: "string"}, {Key: "location_id", Type: "string"}, {Key: "status", Type: "string"}}},
		{Key: "overtime_request", DisplayName: "Overtime Request", DefaultSort: "attendance_date", Fields: []model.FieldDefinition{{Key: "employee_id", Type: "string"}, {Key: "attendance_date", Type: "string"}, {Key: "approved_hours", Type: "number"}, {Key: "approval_status", Type: "string"}, {Key: "status", Type: "string"}}},
		{Key: "attendance_adjustment", DisplayName: "Attendance Adjustment", DefaultSort: "attendance_date", Fields: []model.FieldDefinition{{Key: "employee_id", Type: "string"}, {Key: "attendance_date", Type: "string"}, {Key: "corrected_in_at", Type: "string"}, {Key: "corrected_out_at", Type: "string"}, {Key: "corrected_break_minutes", Type: "number"}, {Key: "approval_status", Type: "string"}, {Key: "status", Type: "string"}}},
	}
	for _, def := range defs {
		if err := models.Register(def); err != nil {
			t.Fatalf("register model %s failed: %v", def.Key, err)
		}
	}
}

func registerPOSTestModels(t *testing.T, models *model.Service) {
	t.Helper()
	defs := []model.Definition{
		{Key: "pos_store", DisplayName: "POS Store", DefaultSort: "code", Fields: []model.FieldDefinition{{Key: "code", Type: "string"}, {Key: "name", Type: "string"}, {Key: "warehouse_code", Type: "string"}, {Key: "status", Type: "string"}}},
		{Key: "pos_register", DisplayName: "POS Register", DefaultSort: "code", Fields: []model.FieldDefinition{{Key: "code", Type: "string"}, {Key: "name", Type: "string"}, {Key: "store_code", Type: "string"}, {Key: "status", Type: "string"}}},
		{Key: "pos_shift", DisplayName: "POS Shift", DefaultSort: "shift_number", Fields: []model.FieldDefinition{{Key: "organization_id", Type: "string"}, {Key: "location_id", Type: "string"}, {Key: "shift_number", Type: "string"}, {Key: "store_code", Type: "string"}, {Key: "register_code", Type: "string"}, {Key: "cashier_user_id", Type: "string"}, {Key: "cashier_employee_id", Type: "string"}, {Key: "roster_slot_id", Type: "string"}, {Key: "attendance_day_id", Type: "string"}, {Key: "opened_at", Type: "string"}, {Key: "closed_at", Type: "string"}, {Key: "opening_cash_amount", Type: "number"}, {Key: "expected_cash_amount", Type: "number"}, {Key: "actual_cash_amount", Type: "number"}, {Key: "over_short_amount", Type: "number"}, {Key: "status", Type: "string"}, {Key: "notes", Type: "string"}}},
	}
	for _, def := range defs {
		if _, ok := models.Definition(def.Key); ok {
			continue
		}
		if err := models.Register(def); err != nil {
			t.Fatalf("register model %s failed: %v", def.Key, err)
		}
	}
}

type failingAttendanceLinkRepo struct {
	model.Repository
}

func (r *failingAttendanceLinkRepo) SaveRecord(record model.Record) error {
	if record.ModelKey == "pos_shift" && textValue(record.Values["attendance_day_id"]) != "" {
		return errors.New("forced pos_shift attendance link failure")
	}
	return r.Repository.SaveRecord(record)
}
