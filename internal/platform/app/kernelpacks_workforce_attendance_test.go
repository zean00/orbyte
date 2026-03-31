package app

import "testing"

func TestWorkforceAttendanceAdminConsoleUsesRegisteredRoutes(t *testing.T) {
	manifest := workforceAttendanceKernelPackManifest()
	expected := map[string]string{
		"calendars":       "/ui/attendance/calendars",
		"shift_templates": "/ui/attendance/shift-templates",
		"rosters":         "/ui/attendance/rosters",
		"roster_slots":    "/ui/attendance/roster-slots",
		"events":          "/ui/attendance/events",
		"days":            "/ui/attendance/days",
		"leave":           "/ui/attendance/leave-requests",
		"overtime":        "/ui/attendance/overtime-requests",
		"adjustments":     "/ui/attendance/adjustments",
	}
	for _, section := range manifest.AdminConsole.Sections {
		for _, link := range section.Links {
			want, ok := expected[link.Key]
			if !ok {
				continue
			}
			if link.RoutePath != want {
				t.Fatalf("expected %s route %q, got %q", link.Key, want, link.RoutePath)
			}
			delete(expected, link.Key)
		}
	}
	if len(expected) != 0 {
		t.Fatalf("missing admin console links: %v", expected)
	}
}

func TestWorkforceAttendanceAndOperationalModelsExposeAttendanceReferences(t *testing.T) {
	attendance := workforceAttendanceKernelPackManifest()
	if !modelHasField(attendance, "attendance_day", "overtime_hours") {
		t.Fatal("expected attendance_day to include overtime_hours")
	}
	if !modelHasField(attendance, "workforce_roster_slot", "register_code") {
		t.Fatal("expected workforce_roster_slot to include register_code")
	}
	if !modelHasField(attendance, "leave_request", "approval_status") {
		t.Fatal("expected leave_request to include approval_status")
	}

	pos := posCoreKernelPackManifest()
	if !modelHasField(pos, "pos_shift", "roster_slot_id") {
		t.Fatal("expected pos_shift to include roster_slot_id")
	}
	if !modelHasField(pos, "pos_shift", "attendance_day_id") {
		t.Fatal("expected pos_shift to include attendance_day_id")
	}

	productionCosting := productionCostingCoreKernelPackManifest()
	if !modelHasField(productionCosting, "production_cost_capture", "attendance_day_id") {
		t.Fatal("expected production_cost_capture to include attendance_day_id")
	}
}
