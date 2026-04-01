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
	for _, field := range []string{"approval_stage_key", "approval_candidate_user_ids_json", "approval_recorded_user_ids_json", "department_id"} {
		if !modelHasField(attendance, "leave_request", field) {
			t.Fatalf("expected leave_request to include %s", field)
		}
	}
	for _, field := range []string{"amendment_count", "last_amended_at", "last_amended_by", "last_amendment_reason", "requested_hours", "count_basis", "counted_dates_json", "counted_work_calendar_id", "counted_roster_slot_ids_json"} {
		if !modelHasField(attendance, "leave_request", field) {
			t.Fatalf("expected leave_request to include %s", field)
		}
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

func TestWorkforceAttendanceIncludesApprovalPermissions(t *testing.T) {
	manifest := workforceAttendanceKernelPackManifest()
	var hasApprove bool
	var hasReject bool
	var hasCancel bool
	var hasAmend bool
	var hasInboxRead bool
	var hasApproverRole bool
	var hasManagerCancel bool
	var hasManagerAmend bool
	for _, permission := range manifest.Security.Permissions {
		switch permission.Key {
		case "attendance.leave_inbox.read":
			hasInboxRead = true
		case "attendance.approve":
			hasApprove = true
		case "attendance.reject":
			hasReject = true
		case "attendance.cancel":
			hasCancel = true
		case "attendance.amend":
			hasAmend = true
		}
	}
	for _, role := range manifest.Security.RoleTemplates {
		if role.Key == "attendance_approver" {
			hasApproverRole = true
		}
		if role.Key == "attendance_manager" {
			for _, permissionKey := range role.PermissionKeys {
				if permissionKey == "attendance.cancel" {
					hasManagerCancel = true
				}
				if permissionKey == "attendance.amend" {
					hasManagerAmend = true
				}
			}
		}
	}
	if !hasApprove || !hasReject || !hasCancel || !hasAmend || !hasInboxRead || !hasApproverRole || !hasManagerCancel || !hasManagerAmend {
		t.Fatalf("expected attendance approval/amend/cancel permissions and roles, got inboxRead=%v approve=%v reject=%v cancel=%v amend=%v approverRole=%v managerCancel=%v managerAmend=%v", hasInboxRead, hasApprove, hasReject, hasCancel, hasAmend, hasApproverRole, hasManagerCancel, hasManagerAmend)
	}
}
