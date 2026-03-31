package app

import (
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/search"
)

func workforceAttendanceKernelPackManifest() module.Manifest {
	return module.Manifest{
		Key:                  "workforce_attendance",
		Name:                 "Workforce Attendance",
		NameI18n:             localize("Workforce Attendance", "Kehadiran Tenaga Kerja"),
		Version:              "1.0.0",
		DomainFamily:         "platform",
		Description:          "Shared shift, roster, attendance, leave, overtime, and adjustment records for workforce operations.",
		DescriptionI18n:      localize("Shared shift, roster, attendance, leave, overtime, and adjustment records for workforce operations.", "Data shift, roster, kehadiran, cuti, lembur, dan penyesuaian tenaga kerja bersama untuk operasi bisnis."),
		BusinessCapabilities: []string{"shift templates", "roster slotting", "attendance events", "attendance summaries", "leave and overtime records"},
		DependencyRequirements: []module.DependencyRequirement{
			{ModuleKey: "platform.core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "employee_workforce", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "organization_structure", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "workflow_approval_policy", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindOptional},
		},
		AdminConsole: module.AdminConsoleDefinition{
			Title:           "Attendance Console",
			TitleI18n:       localize("Attendance Console", "Konsol Kehadiran"),
			Description:     "Shared scheduling, attendance, leave, overtime, and workforce day summaries.",
			DescriptionI18n: localize("Shared scheduling, attendance, leave, overtime, and workforce day summaries.", "Penjadwalan, kehadiran, cuti, lembur, dan ringkasan harian tenaga kerja bersama."),
			Sections: []module.AdminConsoleSectionDefinition{{
				Key:       "workforce_attendance_operations",
				Title:     "Attendance Operations",
				TitleI18n: localize("Attendance Operations", "Operasi Kehadiran"),
				Kind:      module.AdminConsoleSectionResourceLinks,
				Links: []module.AdminConsoleLinkDefinition{
					adminConsoleLink("calendars", "Work Calendars", "Kalender Kerja", "/ui/attendance/calendars", "Open work calendars.", "Buka kalender kerja.", "attendance.list"),
					adminConsoleLink("shift_templates", "Shift Templates", "Template Shift", "/ui/attendance/shift-templates", "Open shift templates.", "Buka template shift.", "attendance.list"),
					adminConsoleLink("rosters", "Rosters", "Roster", "/ui/attendance/rosters", "Open roster headers.", "Buka roster.", "attendance.list"),
					adminConsoleLink("roster_slots", "Roster Slots", "Slot Roster", "/ui/attendance/roster-slots", "Open employee roster slots.", "Buka slot roster karyawan.", "attendance.list"),
					adminConsoleLink("events", "Attendance Events", "Event Kehadiran", "/ui/attendance/events", "Open attendance events.", "Buka event kehadiran.", "attendance.list"),
					adminConsoleLink("days", "Attendance Days", "Hari Kehadiran", "/ui/attendance/days", "Open attendance day summaries.", "Buka ringkasan hari kehadiran.", "attendance.list"),
					adminConsoleLink("leave", "Leave Requests", "Permintaan Cuti", "/ui/attendance/leave-requests", "Open leave requests.", "Buka permintaan cuti.", "attendance.list"),
					adminConsoleLink("overtime", "Overtime Requests", "Permintaan Lembur", "/ui/attendance/overtime-requests", "Open overtime requests.", "Buka permintaan lembur.", "attendance.list"),
					adminConsoleLink("adjustments", "Attendance Adjustments", "Penyesuaian Kehadiran", "/ui/attendance/adjustments", "Open attendance adjustments.", "Buka penyesuaian kehadiran.", "attendance.list"),
				},
			}},
		},
		Models: []model.Definition{
			attendanceModelDefinition("work_calendar", "Work Calendar", []model.FieldDefinition{
				{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Type: "string", Required: true},
				{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Type: "string", Required: true},
				{Key: "organization_id", Label: "Organization", LabelI18n: localize("Organization", "Organisasi"), Type: "string"},
				{Key: "location_id", Label: "Location", LabelI18n: localize("Location", "Lokasi"), Type: "string"},
				{Key: "working_days_json", Label: "Working Days JSON", LabelI18n: localize("Working Days JSON", "JSON Hari Kerja"), Type: "string"},
				{Key: "holiday_dates_json", Label: "Holiday Dates JSON", LabelI18n: localize("Holiday Dates JSON", "JSON Tanggal Libur"), Type: "string"},
				{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string", DefaultValue: "active"},
			}),
			attendanceModelDefinition("shift_template", "Shift Template", []model.FieldDefinition{
				{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Type: "string", Required: true},
				{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Type: "string", Required: true},
				{Key: "organization_id", Label: "Organization", LabelI18n: localize("Organization", "Organisasi"), Type: "string"},
				{Key: "location_id", Label: "Location", LabelI18n: localize("Location", "Lokasi"), Type: "string"},
				{Key: "start_time", Label: "Start Time", LabelI18n: localize("Start Time", "Jam Mulai"), Type: "string", Required: true},
				{Key: "end_time", Label: "End Time", LabelI18n: localize("End Time", "Jam Selesai"), Type: "string", Required: true},
				{Key: "break_minutes", Label: "Break Minutes", LabelI18n: localize("Break Minutes", "Menit Istirahat"), Type: "number"},
				{Key: "late_grace_minutes", Label: "Late Grace", LabelI18n: localize("Late Grace", "Toleransi Telat"), Type: "number"},
				{Key: "early_out_grace_minutes", Label: "Early Out Grace", LabelI18n: localize("Early Out Grace", "Toleransi Pulang Cepat"), Type: "number"},
				{Key: "overnight", Label: "Overnight", LabelI18n: localize("Overnight", "Lintas Hari"), Type: "bool"},
				{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string", DefaultValue: "active"},
			}),
			attendanceModelDefinition("workforce_roster", "Workforce Roster", []model.FieldDefinition{
				{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Type: "string", Required: true},
				{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Type: "string", Required: true},
				{Key: "organization_id", Label: "Organization", LabelI18n: localize("Organization", "Organisasi"), Type: "string", Required: true},
				{Key: "location_id", Label: "Location", LabelI18n: localize("Location", "Lokasi"), Type: "string"},
				{Key: "start_date", Label: "Start Date", LabelI18n: localize("Start Date", "Tanggal Mulai"), Type: "string", Required: true},
				{Key: "end_date", Label: "End Date", LabelI18n: localize("End Date", "Tanggal Selesai"), Type: "string", Required: true},
				{Key: "publish_at", Label: "Publish At", LabelI18n: localize("Publish At", "Dipublikasikan Pada"), Type: "string"},
				{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string", DefaultValue: "draft"},
			}),
			attendanceModelDefinition("workforce_roster_slot", "Workforce Roster Slot", []model.FieldDefinition{
				{Key: "roster_id", Label: "Roster", LabelI18n: localize("Roster", "Roster"), Type: "string"},
				{Key: "employee_id", Label: "Employee", LabelI18n: localize("Employee", "Karyawan"), Type: "string", Required: true},
				{Key: "organization_id", Label: "Organization", LabelI18n: localize("Organization", "Organisasi"), Type: "string", Required: true},
				{Key: "location_id", Label: "Location", LabelI18n: localize("Location", "Lokasi"), Type: "string"},
				{Key: "organization_unit_id", Label: "Organization Unit", LabelI18n: localize("Organization Unit", "Unit Organisasi"), Type: "string"},
				{Key: "department_id", Label: "Department", LabelI18n: localize("Department", "Departemen"), Type: "string"},
				{Key: "cost_center_id", Label: "Cost Center", LabelI18n: localize("Cost Center", "Pusat Biaya"), Type: "string"},
				{Key: "shift_template_id", Label: "Shift Template", LabelI18n: localize("Shift Template", "Template Shift"), Type: "string"},
				{Key: "shift_date", Label: "Shift Date", LabelI18n: localize("Shift Date", "Tanggal Shift"), Type: "string", Required: true},
				{Key: "planned_start_time", Label: "Planned Start", LabelI18n: localize("Planned Start", "Rencana Mulai"), Type: "string", Required: true},
				{Key: "planned_end_time", Label: "Planned End", LabelI18n: localize("Planned End", "Rencana Selesai"), Type: "string", Required: true},
				{Key: "break_minutes", Label: "Break Minutes", LabelI18n: localize("Break Minutes", "Menit Istirahat"), Type: "number"},
				{Key: "late_grace_minutes", Label: "Late Grace", LabelI18n: localize("Late Grace", "Toleransi Telat"), Type: "number"},
				{Key: "early_out_grace_minutes", Label: "Early Out Grace", LabelI18n: localize("Early Out Grace", "Toleransi Pulang Cepat"), Type: "number"},
				{Key: "overnight", Label: "Overnight", LabelI18n: localize("Overnight", "Lintas Hari"), Type: "bool"},
				{Key: "store_code", Label: "Store", LabelI18n: localize("Store", "Toko"), Type: "string"},
				{Key: "register_code", Label: "Register", LabelI18n: localize("Register", "Register"), Type: "string"},
				{Key: "warehouse_code", Label: "Warehouse", LabelI18n: localize("Warehouse", "Gudang"), Type: "string"},
				{Key: "work_center_code", Label: "Work Center", LabelI18n: localize("Work Center", "Pusat Kerja"), Type: "string"},
				{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string", DefaultValue: "active"},
			}),
			attendanceModelDefinition("attendance_event", "Attendance Event", []model.FieldDefinition{
				{Key: "employee_id", Label: "Employee", LabelI18n: localize("Employee", "Karyawan"), Type: "string", Required: true},
				{Key: "attendance_date", Label: "Attendance Date", LabelI18n: localize("Attendance Date", "Tanggal Kehadiran"), Type: "string", Required: true},
				{Key: "organization_id", Label: "Organization", LabelI18n: localize("Organization", "Organisasi"), Type: "string"},
				{Key: "location_id", Label: "Location", LabelI18n: localize("Location", "Lokasi"), Type: "string"},
				{Key: "roster_slot_id", Label: "Roster Slot", LabelI18n: localize("Roster Slot", "Slot Roster"), Type: "string"},
				{Key: "attendance_day_id", Label: "Attendance Day", LabelI18n: localize("Attendance Day", "Hari Kehadiran"), Type: "string"},
				{Key: "event_type", Label: "Event Type", LabelI18n: localize("Event Type", "Tipe Event"), Type: "string", Required: true},
				{Key: "occurred_at", Label: "Occurred At", LabelI18n: localize("Occurred At", "Terjadi Pada"), Type: "string", Required: true},
				{Key: "source", Label: "Source", LabelI18n: localize("Source", "Sumber"), Type: "string", DefaultValue: "manual"},
				{Key: "store_code", Label: "Store", LabelI18n: localize("Store", "Toko"), Type: "string"},
				{Key: "register_code", Label: "Register", LabelI18n: localize("Register", "Register"), Type: "string"},
				{Key: "warehouse_code", Label: "Warehouse", LabelI18n: localize("Warehouse", "Gudang"), Type: "string"},
				{Key: "work_center_code", Label: "Work Center", LabelI18n: localize("Work Center", "Pusat Kerja"), Type: "string"},
				{Key: "notes", Label: "Notes", LabelI18n: localize("Notes", "Catatan"), Type: "string"},
				{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string", DefaultValue: "active"},
			}),
			attendanceModelDefinition("attendance_day", "Attendance Day", []model.FieldDefinition{
				{Key: "employee_id", Label: "Employee", LabelI18n: localize("Employee", "Karyawan"), Type: "string", Required: true},
				{Key: "attendance_date", Label: "Attendance Date", LabelI18n: localize("Attendance Date", "Tanggal Kehadiran"), Type: "string", Required: true},
				{Key: "organization_id", Label: "Organization", LabelI18n: localize("Organization", "Organisasi"), Type: "string"},
				{Key: "location_id", Label: "Location", LabelI18n: localize("Location", "Lokasi"), Type: "string"},
				{Key: "roster_slot_id", Label: "Roster Slot", LabelI18n: localize("Roster Slot", "Slot Roster"), Type: "string"},
				{Key: "shift_template_id", Label: "Shift Template", LabelI18n: localize("Shift Template", "Template Shift"), Type: "string"},
				{Key: "planned_start_at", Label: "Planned Start", LabelI18n: localize("Planned Start", "Rencana Mulai"), Type: "string"},
				{Key: "planned_end_at", Label: "Planned End", LabelI18n: localize("Planned End", "Rencana Selesai"), Type: "string"},
				{Key: "actual_in_at", Label: "Actual In", LabelI18n: localize("Actual In", "Masuk Aktual"), Type: "string"},
				{Key: "actual_out_at", Label: "Actual Out", LabelI18n: localize("Actual Out", "Keluar Aktual"), Type: "string"},
				{Key: "break_minutes", Label: "Break Minutes", LabelI18n: localize("Break Minutes", "Menit Istirahat"), Type: "number"},
				{Key: "worked_hours", Label: "Worked Hours", LabelI18n: localize("Worked Hours", "Jam Kerja"), Type: "number"},
				{Key: "late_minutes", Label: "Late Minutes", LabelI18n: localize("Late Minutes", "Menit Telat"), Type: "number"},
				{Key: "early_out_minutes", Label: "Early Out Minutes", LabelI18n: localize("Early Out Minutes", "Menit Pulang Cepat"), Type: "number"},
				{Key: "overtime_hours", Label: "Overtime Hours", LabelI18n: localize("Overtime Hours", "Jam Lembur"), Type: "number"},
				{Key: "attendance_status", Label: "Attendance Status", LabelI18n: localize("Attendance Status", "Status Kehadiran"), Type: "string"},
				{Key: "absence_code_id", Label: "Absence Code", LabelI18n: localize("Absence Code", "Kode Absensi"), Type: "string"},
				{Key: "leave_request_id", Label: "Leave Request", LabelI18n: localize("Leave Request", "Permintaan Cuti"), Type: "string"},
				{Key: "overtime_request_id", Label: "Overtime Request", LabelI18n: localize("Overtime Request", "Permintaan Lembur"), Type: "string"},
				{Key: "attendance_adjustment_id", Label: "Adjustment", LabelI18n: localize("Adjustment", "Penyesuaian"), Type: "string"},
				{Key: "overnight_shift", Label: "Overnight Shift", LabelI18n: localize("Overnight Shift", "Shift Lintas Hari"), Type: "bool"},
				{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string", DefaultValue: "active"},
			}),
			attendanceModelDefinition("absence_code", "Absence Code", []model.FieldDefinition{
				{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Type: "string", Required: true},
				{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Type: "string", Required: true},
				{Key: "category", Label: "Category", LabelI18n: localize("Category", "Kategori"), Type: "string", Required: true},
				{Key: "deduct_from_payroll", Label: "Deduct Payroll", LabelI18n: localize("Deduct Payroll", "Potong Payroll"), Type: "bool"},
				{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string", DefaultValue: "active"},
			}),
			attendanceModelDefinition("leave_request", "Leave Request", []model.FieldDefinition{
				{Key: "employee_id", Label: "Employee", LabelI18n: localize("Employee", "Karyawan"), Type: "string", Required: true},
				{Key: "organization_id", Label: "Organization", LabelI18n: localize("Organization", "Organisasi"), Type: "string"},
				{Key: "location_id", Label: "Location", LabelI18n: localize("Location", "Lokasi"), Type: "string"},
				{Key: "absence_code_id", Label: "Absence Code", LabelI18n: localize("Absence Code", "Kode Absensi"), Type: "string", Required: true},
				{Key: "start_date", Label: "Start Date", LabelI18n: localize("Start Date", "Tanggal Mulai"), Type: "string", Required: true},
				{Key: "end_date", Label: "End Date", LabelI18n: localize("End Date", "Tanggal Selesai"), Type: "string", Required: true},
				{Key: "requested_hours", Label: "Requested Hours", LabelI18n: localize("Requested Hours", "Jam Diminta"), Type: "number"},
				{Key: "approval_status", Label: "Approval Status", LabelI18n: localize("Approval Status", "Status Persetujuan"), Type: "string", DefaultValue: "draft"},
				{Key: "approval_policy_id", Label: "Approval Policy", LabelI18n: localize("Approval Policy", "Kebijakan Persetujuan"), Type: "string"},
				{Key: "approver_user_id", Label: "Approver User", LabelI18n: localize("Approver User", "Pengguna Penyetuju"), Type: "string"},
				{Key: "notes", Label: "Notes", LabelI18n: localize("Notes", "Catatan"), Type: "string"},
				{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string", DefaultValue: "active"},
			}),
			attendanceModelDefinition("overtime_request", "Overtime Request", []model.FieldDefinition{
				{Key: "employee_id", Label: "Employee", LabelI18n: localize("Employee", "Karyawan"), Type: "string", Required: true},
				{Key: "attendance_date", Label: "Attendance Date", LabelI18n: localize("Attendance Date", "Tanggal Kehadiran"), Type: "string", Required: true},
				{Key: "organization_id", Label: "Organization", LabelI18n: localize("Organization", "Organisasi"), Type: "string"},
				{Key: "location_id", Label: "Location", LabelI18n: localize("Location", "Lokasi"), Type: "string"},
				{Key: "requested_hours", Label: "Requested Hours", LabelI18n: localize("Requested Hours", "Jam Diminta"), Type: "number", Required: true},
				{Key: "approved_hours", Label: "Approved Hours", LabelI18n: localize("Approved Hours", "Jam Disetujui"), Type: "number"},
				{Key: "approval_status", Label: "Approval Status", LabelI18n: localize("Approval Status", "Status Persetujuan"), Type: "string", DefaultValue: "draft"},
				{Key: "approval_policy_id", Label: "Approval Policy", LabelI18n: localize("Approval Policy", "Kebijakan Persetujuan"), Type: "string"},
				{Key: "approver_user_id", Label: "Approver User", LabelI18n: localize("Approver User", "Pengguna Penyetuju"), Type: "string"},
				{Key: "notes", Label: "Notes", LabelI18n: localize("Notes", "Catatan"), Type: "string"},
				{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string", DefaultValue: "active"},
			}),
			attendanceModelDefinition("attendance_adjustment", "Attendance Adjustment", []model.FieldDefinition{
				{Key: "employee_id", Label: "Employee", LabelI18n: localize("Employee", "Karyawan"), Type: "string", Required: true},
				{Key: "attendance_date", Label: "Attendance Date", LabelI18n: localize("Attendance Date", "Tanggal Kehadiran"), Type: "string", Required: true},
				{Key: "organization_id", Label: "Organization", LabelI18n: localize("Organization", "Organisasi"), Type: "string"},
				{Key: "location_id", Label: "Location", LabelI18n: localize("Location", "Lokasi"), Type: "string"},
				{Key: "reason_code", Label: "Reason Code", LabelI18n: localize("Reason Code", "Kode Alasan"), Type: "string", Required: true},
				{Key: "corrected_in_at", Label: "Corrected In", LabelI18n: localize("Corrected In", "Masuk Koreksi"), Type: "string"},
				{Key: "corrected_out_at", Label: "Corrected Out", LabelI18n: localize("Corrected Out", "Keluar Koreksi"), Type: "string"},
				{Key: "corrected_break_minutes", Label: "Corrected Break", LabelI18n: localize("Corrected Break", "Istirahat Koreksi"), Type: "number"},
				{Key: "approval_status", Label: "Approval Status", LabelI18n: localize("Approval Status", "Status Persetujuan"), Type: "string", DefaultValue: "draft"},
				{Key: "approval_policy_id", Label: "Approval Policy", LabelI18n: localize("Approval Policy", "Kebijakan Persetujuan"), Type: "string"},
				{Key: "approver_user_id", Label: "Approver User", LabelI18n: localize("Approver User", "Pengguna Penyetuju"), Type: "string"},
				{Key: "notes", Label: "Notes", LabelI18n: localize("Notes", "Catatan"), Type: "string"},
				{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string", DefaultValue: "active"},
			}),
		},
		Datasets: []module.DatasetDefinition{
			attendanceDataset("attendance.day.summary", "Attendance Day Summary", "attendance_day", "attendance_status"),
			attendanceDataset("attendance.roster.summary", "Roster Slot Summary", "workforce_roster_slot", "location_id"),
			attendanceDataset("attendance.leave.summary", "Leave Request Summary", "leave_request", "approval_status"),
		},
		SearchIndexes: []search.IndexDefinition{
			attendanceSearchIndex("attendance.calendars.search", "Work Calendar Search", "work_calendar", "attendance.calendars.list", []string{"code", "name", "status"}),
			attendanceSearchIndex("attendance.shift_templates.search", "Shift Template Search", "shift_template", "attendance.shift_templates.list", []string{"code", "name", "status"}),
			attendanceSearchIndex("attendance.rosters.search", "Roster Search", "workforce_roster", "attendance.rosters.list", []string{"code", "name", "status"}),
			attendanceSearchIndex("attendance.roster_slots.search", "Roster Slot Search", "workforce_roster_slot", "attendance.roster_slots.list", []string{"employee_id", "shift_date", "store_code", "register_code", "work_center_code", "status"}),
			attendanceSearchIndex("attendance.events.search", "Attendance Event Search", "attendance_event", "attendance.events.list", []string{"employee_id", "attendance_date", "event_type", "source", "status"}),
			attendanceSearchIndex("attendance.days.search", "Attendance Day Search", "attendance_day", "attendance.days.list", []string{"employee_id", "attendance_date", "attendance_status", "status"}),
			attendanceSearchIndex("attendance.leave.search", "Leave Request Search", "leave_request", "attendance.leave_requests.list", []string{"employee_id", "start_date", "end_date", "approval_status", "status"}),
			attendanceSearchIndex("attendance.overtime.search", "Overtime Request Search", "overtime_request", "attendance.overtime_requests.list", []string{"employee_id", "attendance_date", "approval_status", "status"}),
			attendanceSearchIndex("attendance.adjustments.search", "Attendance Adjustment Search", "attendance_adjustment", "attendance.adjustments.list", []string{"employee_id", "attendance_date", "approval_status", "status"}),
		},
		Security: module.SecurityDefinition{
			Permissions: []module.PermissionDefinition{
				{Key: "attendance.create", Action: "create", Resource: "attendance", DisplayName: "Create Attendance Records", DisplayNameI18n: localize("Create Attendance Records", "Buat Data Kehadiran")},
				{Key: "attendance.list", Action: "list", Resource: "attendance", DisplayName: "List Attendance Records", DisplayNameI18n: localize("List Attendance Records", "Daftar Data Kehadiran")},
				{Key: "attendance.read", Action: "read", Resource: "attendance", DisplayName: "Read Attendance Records", DisplayNameI18n: localize("Read Attendance Records", "Lihat Data Kehadiran")},
				{Key: "attendance.update", Action: "update", Resource: "attendance", DisplayName: "Update Attendance Records", DisplayNameI18n: localize("Update Attendance Records", "Perbarui Data Kehadiran")},
			},
			RoleTemplates: []module.RoleTemplateDefinition{{
				Key: "attendance_manager", Name: "Attendance Manager", NameI18n: localize("Attendance Manager", "Pengelola Kehadiran"), AllowedScopes: []string{"deployment", "organization", "location"}, PermissionKeys: []string{"attendance.create", "attendance.list", "attendance.read", "attendance.update", "employee.read", "organization_structure.read"},
			}},
		},
		Frontend: module.FrontendDefinition{
			Menus: []module.MenuDefinition{
				{Key: "attendance.calendars", Label: "Work Calendars", LabelI18n: localize("Work Calendars", "Kalender Kerja"), ActionKey: "attendance.calendars.list", Order: 20, RequiredPermissions: []string{"attendance.list"}},
				{Key: "attendance.shift_templates", Label: "Shift Templates", LabelI18n: localize("Shift Templates", "Template Shift"), ActionKey: "attendance.shift_templates.list", Order: 21, RequiredPermissions: []string{"attendance.list"}},
				{Key: "attendance.rosters", Label: "Rosters", LabelI18n: localize("Rosters", "Roster"), ActionKey: "attendance.rosters.list", Order: 22, RequiredPermissions: []string{"attendance.list"}},
				{Key: "attendance.roster_slots", Label: "Roster Slots", LabelI18n: localize("Roster Slots", "Slot Roster"), ActionKey: "attendance.roster_slots.list", Order: 23, RequiredPermissions: []string{"attendance.list"}},
				{Key: "attendance.events", Label: "Attendance Events", LabelI18n: localize("Attendance Events", "Event Kehadiran"), ActionKey: "attendance.events.list", Order: 24, RequiredPermissions: []string{"attendance.list"}},
				{Key: "attendance.days", Label: "Attendance Days", LabelI18n: localize("Attendance Days", "Hari Kehadiran"), ActionKey: "attendance.days.list", Order: 25, RequiredPermissions: []string{"attendance.list"}},
				{Key: "attendance.leave_requests", Label: "Leave Requests", LabelI18n: localize("Leave Requests", "Permintaan Cuti"), ActionKey: "attendance.leave_requests.list", Order: 26, RequiredPermissions: []string{"attendance.list"}},
				{Key: "attendance.overtime_requests", Label: "Overtime Requests", LabelI18n: localize("Overtime Requests", "Permintaan Lembur"), ActionKey: "attendance.overtime_requests.list", Order: 27, RequiredPermissions: []string{"attendance.list"}},
				{Key: "attendance.adjustments", Label: "Attendance Adjustments", LabelI18n: localize("Attendance Adjustments", "Penyesuaian Kehadiran"), ActionKey: "attendance.adjustments.list", Order: 28, RequiredPermissions: []string{"attendance.list"}},
			},
			Actions: append(
				append(
					append(
						append(
							attendanceActions("calendars", "Work Calendars", "Work Calendar Detail", "Work Calendar Form"),
							attendanceActions("shift_templates", "Shift Templates", "Shift Template Detail", "Shift Template Form")...,
						),
						attendanceActions("rosters", "Rosters", "Roster Detail", "Roster Form")...,
					),
					attendanceActions("roster_slots", "Roster Slots", "Roster Slot Detail", "Roster Slot Form")...,
				),
				append(
					append(
						append(
							attendanceActions("events", "Attendance Events", "Attendance Event Detail", "Attendance Event Form"),
							attendanceActions("days", "Attendance Days", "Attendance Day Detail", "Attendance Day Form")...,
						),
						attendanceActions("leave_requests", "Leave Requests", "Leave Request Detail", "Leave Request Form")...,
					),
					append(
						attendanceActions("overtime_requests", "Overtime Requests", "Overtime Request Detail", "Overtime Request Form"),
						attendanceActions("adjustments", "Attendance Adjustments", "Attendance Adjustment Detail", "Attendance Adjustment Form")...,
					)...,
				)...,
			),
			Views: []module.ViewDefinition{
				commercialModelListView("attendance.calendars.list", "Work Calendars", "work_calendar", []module.ColumnDefinition{{Key: "code", Label: "Code", Path: "values.code"}, {Key: "name", Label: "Name", Path: "values.name"}, {Key: "status", Label: "Status", Path: "values.status"}}, []string{"active", "inactive"}),
				commercialModelDetailView("attendance.calendars.detail", "Work Calendar Detail", "work_calendar", attendanceCalendarFields(false)),
				commercialModelFormView("attendance.calendars.form", "Work Calendar Form", "work_calendar", attendanceCalendarFields(true)),
				commercialModelListView("attendance.shift_templates.list", "Shift Templates", "shift_template", []module.ColumnDefinition{{Key: "code", Label: "Code", Path: "values.code"}, {Key: "name", Label: "Name", Path: "values.name"}, {Key: "start_time", Label: "Start", Path: "values.start_time"}, {Key: "end_time", Label: "End", Path: "values.end_time"}, {Key: "status", Label: "Status", Path: "values.status"}}, []string{"active", "inactive"}),
				commercialModelDetailView("attendance.shift_templates.detail", "Shift Template Detail", "shift_template", attendanceShiftTemplateFields(false)),
				commercialModelFormView("attendance.shift_templates.form", "Shift Template Form", "shift_template", attendanceShiftTemplateFields(true)),
				commercialModelListView("attendance.rosters.list", "Rosters", "workforce_roster", []module.ColumnDefinition{{Key: "code", Label: "Code", Path: "values.code"}, {Key: "name", Label: "Name", Path: "values.name"}, {Key: "start_date", Label: "Start Date", Path: "values.start_date"}, {Key: "end_date", Label: "End Date", Path: "values.end_date"}, {Key: "status", Label: "Status", Path: "values.status"}}, []string{"draft", "published", "archived"}),
				commercialModelDetailView("attendance.rosters.detail", "Roster Detail", "workforce_roster", attendanceRosterFields(false)),
				commercialModelFormView("attendance.rosters.form", "Roster Form", "workforce_roster", attendanceRosterFields(true)),
				commercialModelListView("attendance.roster_slots.list", "Roster Slots", "workforce_roster_slot", []module.ColumnDefinition{{Key: "employee_id", Label: "Employee", Path: "values.employee_id"}, {Key: "shift_date", Label: "Date", Path: "values.shift_date"}, {Key: "store_code", Label: "Store", Path: "values.store_code"}, {Key: "register_code", Label: "Register", Path: "values.register_code"}, {Key: "work_center_code", Label: "Work Center", Path: "values.work_center_code"}, {Key: "status", Label: "Status", Path: "values.status"}}, []string{"active", "inactive"}),
				commercialModelDetailView("attendance.roster_slots.detail", "Roster Slot Detail", "workforce_roster_slot", attendanceRosterSlotFields(false)),
				commercialModelFormView("attendance.roster_slots.form", "Roster Slot Form", "workforce_roster_slot", attendanceRosterSlotFields(true)),
				commercialModelListView("attendance.events.list", "Attendance Events", "attendance_event", []module.ColumnDefinition{{Key: "employee_id", Label: "Employee", Path: "values.employee_id"}, {Key: "attendance_date", Label: "Date", Path: "values.attendance_date"}, {Key: "event_type", Label: "Event", Path: "values.event_type"}, {Key: "occurred_at", Label: "Occurred At", Path: "values.occurred_at"}, {Key: "status", Label: "Status", Path: "values.status"}}, []string{"active", "inactive"}),
				commercialModelDetailView("attendance.events.detail", "Attendance Event Detail", "attendance_event", attendanceEventFields(false)),
				commercialModelFormView("attendance.events.form", "Attendance Event Form", "attendance_event", attendanceEventFields(true)),
				commercialModelListView("attendance.days.list", "Attendance Days", "attendance_day", []module.ColumnDefinition{{Key: "employee_id", Label: "Employee", Path: "values.employee_id"}, {Key: "attendance_date", Label: "Date", Path: "values.attendance_date"}, {Key: "attendance_status", Label: "Attendance", Path: "values.attendance_status"}, {Key: "worked_hours", Label: "Worked Hours", Path: "values.worked_hours"}, {Key: "overtime_hours", Label: "Overtime", Path: "values.overtime_hours"}}, []string{"active", "inactive"}),
				commercialModelDetailView("attendance.days.detail", "Attendance Day Detail", "attendance_day", attendanceDayFields(false)),
				commercialModelFormView("attendance.days.form", "Attendance Day Form", "attendance_day", attendanceDayFields(true)),
				commercialModelListView("attendance.leave_requests.list", "Leave Requests", "leave_request", []module.ColumnDefinition{{Key: "employee_id", Label: "Employee", Path: "values.employee_id"}, {Key: "start_date", Label: "Start Date", Path: "values.start_date"}, {Key: "end_date", Label: "End Date", Path: "values.end_date"}, {Key: "approval_status", Label: "Approval", Path: "values.approval_status"}, {Key: "status", Label: "Status", Path: "values.status"}}, []string{"draft", "submitted", "approved", "rejected"}),
				commercialModelDetailView("attendance.leave_requests.detail", "Leave Request Detail", "leave_request", attendanceLeaveFields(false)),
				commercialModelFormView("attendance.leave_requests.form", "Leave Request Form", "leave_request", attendanceLeaveFields(true)),
				commercialModelListView("attendance.overtime_requests.list", "Overtime Requests", "overtime_request", []module.ColumnDefinition{{Key: "employee_id", Label: "Employee", Path: "values.employee_id"}, {Key: "attendance_date", Label: "Date", Path: "values.attendance_date"}, {Key: "requested_hours", Label: "Requested", Path: "values.requested_hours"}, {Key: "approval_status", Label: "Approval", Path: "values.approval_status"}, {Key: "status", Label: "Status", Path: "values.status"}}, []string{"draft", "submitted", "approved", "rejected"}),
				commercialModelDetailView("attendance.overtime_requests.detail", "Overtime Request Detail", "overtime_request", attendanceOvertimeFields(false)),
				commercialModelFormView("attendance.overtime_requests.form", "Overtime Request Form", "overtime_request", attendanceOvertimeFields(true)),
				commercialModelListView("attendance.adjustments.list", "Attendance Adjustments", "attendance_adjustment", []module.ColumnDefinition{{Key: "employee_id", Label: "Employee", Path: "values.employee_id"}, {Key: "attendance_date", Label: "Date", Path: "values.attendance_date"}, {Key: "reason_code", Label: "Reason", Path: "values.reason_code"}, {Key: "approval_status", Label: "Approval", Path: "values.approval_status"}, {Key: "status", Label: "Status", Path: "values.status"}}, []string{"draft", "submitted", "approved", "rejected"}),
				commercialModelDetailView("attendance.adjustments.detail", "Attendance Adjustment Detail", "attendance_adjustment", attendanceAdjustmentFields(false)),
				commercialModelFormView("attendance.adjustments.form", "Attendance Adjustment Form", "attendance_adjustment", attendanceAdjustmentFields(true)),
			},
		},
		Offline: module.OfflineDefinition{
			Models: []module.OfflineModelDefinition{
				{ModelKey: "work_calendar", Title: "Work Calendar", TitleI18n: localize("Work Calendar", "Kalender Kerja"), CreatePermissionKey: "attendance.create", UpdatePermissionKey: "attendance.update", RequiredPermissions: []string{"attendance.read"}},
				{ModelKey: "shift_template", Title: "Shift Template", TitleI18n: localize("Shift Template", "Template Shift"), CreatePermissionKey: "attendance.create", UpdatePermissionKey: "attendance.update", RequiredPermissions: []string{"attendance.read"}},
				{ModelKey: "workforce_roster", Title: "Workforce Roster", TitleI18n: localize("Workforce Roster", "Roster Tenaga Kerja"), CreatePermissionKey: "attendance.create", UpdatePermissionKey: "attendance.update", RequiredPermissions: []string{"attendance.read"}},
				{ModelKey: "workforce_roster_slot", Title: "Workforce Roster Slot", TitleI18n: localize("Workforce Roster Slot", "Slot Roster"), CreatePermissionKey: "attendance.create", UpdatePermissionKey: "attendance.update", RequiredPermissions: []string{"attendance.read"}},
				{ModelKey: "attendance_event", Title: "Attendance Event", TitleI18n: localize("Attendance Event", "Event Kehadiran"), CreatePermissionKey: "attendance.create", UpdatePermissionKey: "attendance.update", RequiredPermissions: []string{"attendance.read"}},
				{ModelKey: "attendance_day", Title: "Attendance Day", TitleI18n: localize("Attendance Day", "Hari Kehadiran"), CreatePermissionKey: "attendance.create", UpdatePermissionKey: "attendance.update", RequiredPermissions: []string{"attendance.read"}},
				{ModelKey: "leave_request", Title: "Leave Request", TitleI18n: localize("Leave Request", "Permintaan Cuti"), CreatePermissionKey: "attendance.create", UpdatePermissionKey: "attendance.update", RequiredPermissions: []string{"attendance.read"}},
				{ModelKey: "overtime_request", Title: "Overtime Request", TitleI18n: localize("Overtime Request", "Permintaan Lembur"), CreatePermissionKey: "attendance.create", UpdatePermissionKey: "attendance.update", RequiredPermissions: []string{"attendance.read"}},
				{ModelKey: "attendance_adjustment", Title: "Attendance Adjustment", TitleI18n: localize("Attendance Adjustment", "Penyesuaian Kehadiran"), CreatePermissionKey: "attendance.create", UpdatePermissionKey: "attendance.update", RequiredPermissions: []string{"attendance.read"}},
			},
		},
	}
}

func attendanceModelDefinition(key, displayName string, fields []model.FieldDefinition) model.Definition {
	return model.Definition{
		Key:                 key,
		DisplayName:         displayName,
		DisplayNameI18n:     localize(displayName, displayName),
		OwnerModuleKey:      "workforce_attendance",
		Version:             "v1",
		CreatePermissionKey: "attendance.create",
		ListPermissionKey:   "attendance.list",
		ReadPermissionKey:   "attendance.read",
		UpdatePermissionKey: "attendance.update",
		DefaultSort:         firstAttendanceSortKey(key),
		Fields:              fields,
	}
}

func firstAttendanceSortKey(modelKey string) string {
	switch modelKey {
	case "attendance_event":
		return "occurred_at"
	case "attendance_day", "overtime_request", "attendance_adjustment":
		return "attendance_date"
	case "workforce_roster_slot":
		return "shift_date"
	case "workforce_roster":
		return "start_date"
	case "leave_request":
		return "start_date"
	default:
		return "code"
	}
}

func attendanceDataset(key, title, modelKey, dimensionPath string) module.DatasetDefinition {
	return module.DatasetDefinition{
		Key:        key,
		Title:      title,
		TitleI18n:  localize(title, title),
		SourceKind: "model",
		ModelKey:   modelKey,
		Dimensions: []module.DatasetDimension{{Key: "group", Label: "Group", LabelI18n: localize("Group", "Grup"), Path: dimensionPath}},
		Measures:   []module.DatasetMeasure{{Key: "total", Label: "Total", LabelI18n: localize("Total", "Total"), Kind: "count"}},
	}
}

func attendanceSearchIndex(key, title, modelKey, viewKey string, fieldKeys []string) search.IndexDefinition {
	fields := make([]search.IndexFieldDefinition, 0, len(fieldKeys))
	filterFields := []string{}
	for _, fieldKey := range fieldKeys {
		fields = append(fields, search.IndexFieldDefinition{Key: fieldKey, Path: fieldKey, Type: "string", Searchable: true, Facet: fieldKey == "status" || fieldKey == "approval_status"})
		if fieldKey == "status" || fieldKey == "approval_status" || fieldKey == "location_id" {
			filterFields = append(filterFields, fieldKey)
		}
	}
	return search.IndexDefinition{
		Key:                 key,
		Title:               title,
		SourceKind:          "model",
		ModelKey:            modelKey,
		ViewKey:             viewKey,
		Modes:               []string{"keyword", "hybrid"},
		OrganizationSplit:   true,
		RequiredPermissions: []string{"attendance.list"},
		QueryFilterFields:   uniqueAttendanceStrings(filterFields),
		QuerySortFields:     fieldKeys,
		Fields:              fields,
	}
}

func uniqueAttendanceStrings(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func attendanceActions(prefix, listLabel, detailLabel, formLabel string) []module.ActionDefinition {
	base := "/attendance/" + prefix
	return []module.ActionDefinition{
		{Key: "attendance." + prefix + ".list", Label: listLabel, LabelI18n: localize(listLabel, listLabel), Kind: "navigate", RoutePath: base, ViewKey: "attendance." + prefix + ".list", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"attendance.list"}},
		{Key: "attendance." + prefix + ".detail", Label: detailLabel, LabelI18n: localize(detailLabel, detailLabel), Kind: "navigate", RoutePath: base + "/detail", ViewKey: "attendance." + prefix + ".detail", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"attendance.read"}},
		{Key: "attendance." + prefix + ".form", Label: formLabel, LabelI18n: localize(formLabel, formLabel), Kind: "navigate", RoutePath: base + "/form", ViewKey: "attendance." + prefix + ".form", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"attendance.update"}},
		{Key: "attendance." + prefix + ".new", Label: "New " + listLabel, LabelI18n: localize("New "+listLabel, "Baru"), Kind: "navigate", RoutePath: base + "/new", ViewKey: "attendance." + prefix + ".form", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"attendance.update"}},
	}
}

func attendanceCalendarFields(form bool) []module.FieldDefinition {
	return []module.FieldDefinition{
		{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code", Type: "string", Widget: widgetForForm(form, "text"), Required: true},
		{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name", Type: "string", Widget: widgetForForm(form, "text"), Required: true},
		{Key: "organization_id", Label: "Organization", LabelI18n: localize("Organization", "Organisasi"), Path: "values.organization_id", Type: "string", Widget: widgetForForm(form, "text")},
		{Key: "location_id", Label: "Location", LabelI18n: localize("Location", "Lokasi"), Path: "values.location_id", Type: "string", Widget: widgetForForm(form, "text")},
		{Key: "working_days_json", Label: "Working Days JSON", LabelI18n: localize("Working Days JSON", "JSON Hari Kerja"), Path: "values.working_days_json", Type: "string", Widget: widgetForForm(form, "textarea")},
		{Key: "holiday_dates_json", Label: "Holiday Dates JSON", LabelI18n: localize("Holiday Dates JSON", "JSON Tanggal Libur"), Path: "values.holiday_dates_json", Type: "string", Widget: widgetForForm(form, "textarea")},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string", Widget: widgetForForm(form, "select"), Options: []string{"active", "inactive"}},
	}
}

func attendanceShiftTemplateFields(form bool) []module.FieldDefinition {
	return []module.FieldDefinition{
		{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code", Type: "string", Widget: widgetForForm(form, "text"), Required: true},
		{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name", Type: "string", Widget: widgetForForm(form, "text"), Required: true},
		{Key: "organization_id", Label: "Organization", LabelI18n: localize("Organization", "Organisasi"), Path: "values.organization_id", Type: "string", Widget: widgetForForm(form, "text")},
		{Key: "location_id", Label: "Location", LabelI18n: localize("Location", "Lokasi"), Path: "values.location_id", Type: "string", Widget: widgetForForm(form, "text")},
		{Key: "start_time", Label: "Start Time", LabelI18n: localize("Start Time", "Jam Mulai"), Path: "values.start_time", Type: "string", Widget: widgetForForm(form, "text"), Required: true},
		{Key: "end_time", Label: "End Time", LabelI18n: localize("End Time", "Jam Selesai"), Path: "values.end_time", Type: "string", Widget: widgetForForm(form, "text"), Required: true},
		{Key: "break_minutes", Label: "Break Minutes", LabelI18n: localize("Break Minutes", "Menit Istirahat"), Path: "values.break_minutes", Type: "number", Widget: widgetForForm(form, "text")},
		{Key: "late_grace_minutes", Label: "Late Grace", LabelI18n: localize("Late Grace", "Toleransi Telat"), Path: "values.late_grace_minutes", Type: "number", Widget: widgetForForm(form, "text")},
		{Key: "early_out_grace_minutes", Label: "Early Out Grace", LabelI18n: localize("Early Out Grace", "Toleransi Pulang Cepat"), Path: "values.early_out_grace_minutes", Type: "number", Widget: widgetForForm(form, "text")},
		{Key: "overnight", Label: "Overnight", LabelI18n: localize("Overnight", "Lintas Hari"), Path: "values.overnight", Type: "bool", Widget: widgetForForm(form, "checkbox")},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string", Widget: widgetForForm(form, "select"), Options: []string{"active", "inactive"}},
	}
}

func attendanceRosterFields(form bool) []module.FieldDefinition {
	return []module.FieldDefinition{
		{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code", Type: "string", Widget: widgetForForm(form, "text"), Required: true},
		{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name", Type: "string", Widget: widgetForForm(form, "text"), Required: true},
		{Key: "organization_id", Label: "Organization", LabelI18n: localize("Organization", "Organisasi"), Path: "values.organization_id", Type: "string", Widget: widgetForForm(form, "text"), Required: true},
		{Key: "location_id", Label: "Location", LabelI18n: localize("Location", "Lokasi"), Path: "values.location_id", Type: "string", Widget: widgetForForm(form, "text")},
		{Key: "start_date", Label: "Start Date", LabelI18n: localize("Start Date", "Tanggal Mulai"), Path: "values.start_date", Type: "string", Widget: widgetForForm(form, "text"), Required: true},
		{Key: "end_date", Label: "End Date", LabelI18n: localize("End Date", "Tanggal Selesai"), Path: "values.end_date", Type: "string", Widget: widgetForForm(form, "text"), Required: true},
		{Key: "publish_at", Label: "Publish At", LabelI18n: localize("Publish At", "Dipublikasikan Pada"), Path: "values.publish_at", Type: "string", Widget: widgetForForm(form, "text")},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string", Widget: widgetForForm(form, "select"), Options: []string{"draft", "published", "archived"}},
	}
}

func attendanceRosterSlotFields(form bool) []module.FieldDefinition {
	return []module.FieldDefinition{
		{Key: "roster_id", Label: "Roster", LabelI18n: localize("Roster", "Roster"), Path: "values.roster_id", Type: "string", Widget: widgetForForm(form, "select")},
		{Key: "employee_id", Label: "Employee", LabelI18n: localize("Employee", "Karyawan"), Path: "values.employee_id", Type: "string", Widget: widgetForForm(form, "select"), Required: true},
		{Key: "organization_id", Label: "Organization", LabelI18n: localize("Organization", "Organisasi"), Path: "values.organization_id", Type: "string", Widget: widgetForForm(form, "text"), Required: true},
		{Key: "location_id", Label: "Location", LabelI18n: localize("Location", "Lokasi"), Path: "values.location_id", Type: "string", Widget: widgetForForm(form, "text")},
		{Key: "organization_unit_id", Label: "Organization Unit", LabelI18n: localize("Organization Unit", "Unit Organisasi"), Path: "values.organization_unit_id", Type: "string", Widget: widgetForForm(form, "select")},
		{Key: "department_id", Label: "Department", LabelI18n: localize("Department", "Departemen"), Path: "values.department_id", Type: "string", Widget: widgetForForm(form, "select")},
		{Key: "cost_center_id", Label: "Cost Center", LabelI18n: localize("Cost Center", "Pusat Biaya"), Path: "values.cost_center_id", Type: "string", Widget: widgetForForm(form, "select")},
		{Key: "shift_template_id", Label: "Shift Template", LabelI18n: localize("Shift Template", "Template Shift"), Path: "values.shift_template_id", Type: "string", Widget: widgetForForm(form, "select")},
		{Key: "shift_date", Label: "Shift Date", LabelI18n: localize("Shift Date", "Tanggal Shift"), Path: "values.shift_date", Type: "string", Widget: widgetForForm(form, "text"), Required: true},
		{Key: "planned_start_time", Label: "Planned Start", LabelI18n: localize("Planned Start", "Rencana Mulai"), Path: "values.planned_start_time", Type: "string", Widget: widgetForForm(form, "text"), Required: true},
		{Key: "planned_end_time", Label: "Planned End", LabelI18n: localize("Planned End", "Rencana Selesai"), Path: "values.planned_end_time", Type: "string", Widget: widgetForForm(form, "text"), Required: true},
		{Key: "break_minutes", Label: "Break Minutes", LabelI18n: localize("Break Minutes", "Menit Istirahat"), Path: "values.break_minutes", Type: "number", Widget: widgetForForm(form, "text")},
		{Key: "late_grace_minutes", Label: "Late Grace", LabelI18n: localize("Late Grace", "Toleransi Telat"), Path: "values.late_grace_minutes", Type: "number", Widget: widgetForForm(form, "text")},
		{Key: "early_out_grace_minutes", Label: "Early Out Grace", LabelI18n: localize("Early Out Grace", "Toleransi Pulang Cepat"), Path: "values.early_out_grace_minutes", Type: "number", Widget: widgetForForm(form, "text")},
		{Key: "overnight", Label: "Overnight", LabelI18n: localize("Overnight", "Lintas Hari"), Path: "values.overnight", Type: "bool", Widget: widgetForForm(form, "checkbox")},
		{Key: "store_code", Label: "Store", LabelI18n: localize("Store", "Toko"), Path: "values.store_code", Type: "string", Widget: widgetForForm(form, "text")},
		{Key: "register_code", Label: "Register", LabelI18n: localize("Register", "Register"), Path: "values.register_code", Type: "string", Widget: widgetForForm(form, "text")},
		{Key: "warehouse_code", Label: "Warehouse", LabelI18n: localize("Warehouse", "Gudang"), Path: "values.warehouse_code", Type: "string", Widget: widgetForForm(form, "text")},
		{Key: "work_center_code", Label: "Work Center", LabelI18n: localize("Work Center", "Pusat Kerja"), Path: "values.work_center_code", Type: "string", Widget: widgetForForm(form, "text")},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string", Widget: widgetForForm(form, "select"), Options: []string{"active", "inactive"}},
	}
}

func attendanceEventFields(form bool) []module.FieldDefinition {
	return []module.FieldDefinition{
		{Key: "employee_id", Label: "Employee", LabelI18n: localize("Employee", "Karyawan"), Path: "values.employee_id", Type: "string", Widget: widgetForForm(form, "select"), Required: true},
		{Key: "attendance_date", Label: "Attendance Date", LabelI18n: localize("Attendance Date", "Tanggal Kehadiran"), Path: "values.attendance_date", Type: "string", Widget: widgetForForm(form, "text"), Required: true},
		{Key: "organization_id", Label: "Organization", LabelI18n: localize("Organization", "Organisasi"), Path: "values.organization_id", Type: "string", Widget: widgetForForm(form, "text")},
		{Key: "location_id", Label: "Location", LabelI18n: localize("Location", "Lokasi"), Path: "values.location_id", Type: "string", Widget: widgetForForm(form, "text")},
		{Key: "roster_slot_id", Label: "Roster Slot", LabelI18n: localize("Roster Slot", "Slot Roster"), Path: "values.roster_slot_id", Type: "string", Widget: widgetForForm(form, "select")},
		{Key: "attendance_day_id", Label: "Attendance Day", LabelI18n: localize("Attendance Day", "Hari Kehadiran"), Path: "values.attendance_day_id", Type: "string", Widget: widgetForForm(form, "select")},
		{Key: "event_type", Label: "Event Type", LabelI18n: localize("Event Type", "Tipe Event"), Path: "values.event_type", Type: "string", Widget: widgetForForm(form, "select"), Options: []string{"clock_in", "clock_out", "break_start", "break_end", "manual_adjustment"}, Required: true},
		{Key: "occurred_at", Label: "Occurred At", LabelI18n: localize("Occurred At", "Terjadi Pada"), Path: "values.occurred_at", Type: "string", Widget: widgetForForm(form, "text"), Required: true},
		{Key: "source", Label: "Source", LabelI18n: localize("Source", "Sumber"), Path: "values.source", Type: "string", Widget: widgetForForm(form, "select"), Options: []string{"manual", "pos_shift", "device", "import"}},
		{Key: "store_code", Label: "Store", LabelI18n: localize("Store", "Toko"), Path: "values.store_code", Type: "string", Widget: widgetForForm(form, "text")},
		{Key: "register_code", Label: "Register", LabelI18n: localize("Register", "Register"), Path: "values.register_code", Type: "string", Widget: widgetForForm(form, "text")},
		{Key: "warehouse_code", Label: "Warehouse", LabelI18n: localize("Warehouse", "Gudang"), Path: "values.warehouse_code", Type: "string", Widget: widgetForForm(form, "text")},
		{Key: "work_center_code", Label: "Work Center", LabelI18n: localize("Work Center", "Pusat Kerja"), Path: "values.work_center_code", Type: "string", Widget: widgetForForm(form, "text")},
		{Key: "notes", Label: "Notes", LabelI18n: localize("Notes", "Catatan"), Path: "values.notes", Type: "string", Widget: widgetForForm(form, "textarea")},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string", Widget: widgetForForm(form, "select"), Options: []string{"active", "inactive"}},
	}
}

func attendanceDayFields(form bool) []module.FieldDefinition {
	return []module.FieldDefinition{
		{Key: "employee_id", Label: "Employee", LabelI18n: localize("Employee", "Karyawan"), Path: "values.employee_id", Type: "string", Widget: widgetForForm(form, "select"), Required: true},
		{Key: "attendance_date", Label: "Attendance Date", LabelI18n: localize("Attendance Date", "Tanggal Kehadiran"), Path: "values.attendance_date", Type: "string", Widget: widgetForForm(form, "text"), Required: true},
		{Key: "roster_slot_id", Label: "Roster Slot", LabelI18n: localize("Roster Slot", "Slot Roster"), Path: "values.roster_slot_id", Type: "string", Widget: widgetForForm(form, "select")},
		{Key: "shift_template_id", Label: "Shift Template", LabelI18n: localize("Shift Template", "Template Shift"), Path: "values.shift_template_id", Type: "string", Widget: widgetForForm(form, "select")},
		{Key: "planned_start_at", Label: "Planned Start", LabelI18n: localize("Planned Start", "Rencana Mulai"), Path: "values.planned_start_at", Type: "string", Widget: widgetForForm(form, "text")},
		{Key: "planned_end_at", Label: "Planned End", LabelI18n: localize("Planned End", "Rencana Selesai"), Path: "values.planned_end_at", Type: "string", Widget: widgetForForm(form, "text")},
		{Key: "actual_in_at", Label: "Actual In", LabelI18n: localize("Actual In", "Masuk Aktual"), Path: "values.actual_in_at", Type: "string", Widget: widgetForForm(form, "text")},
		{Key: "actual_out_at", Label: "Actual Out", LabelI18n: localize("Actual Out", "Keluar Aktual"), Path: "values.actual_out_at", Type: "string", Widget: widgetForForm(form, "text")},
		{Key: "break_minutes", Label: "Break Minutes", LabelI18n: localize("Break Minutes", "Menit Istirahat"), Path: "values.break_minutes", Type: "number", Widget: widgetForForm(form, "text")},
		{Key: "worked_hours", Label: "Worked Hours", LabelI18n: localize("Worked Hours", "Jam Kerja"), Path: "values.worked_hours", Type: "number", Widget: widgetForForm(form, "text")},
		{Key: "late_minutes", Label: "Late Minutes", LabelI18n: localize("Late Minutes", "Menit Telat"), Path: "values.late_minutes", Type: "number", Widget: widgetForForm(form, "text")},
		{Key: "early_out_minutes", Label: "Early Out Minutes", LabelI18n: localize("Early Out Minutes", "Menit Pulang Cepat"), Path: "values.early_out_minutes", Type: "number", Widget: widgetForForm(form, "text")},
		{Key: "overtime_hours", Label: "Overtime Hours", LabelI18n: localize("Overtime Hours", "Jam Lembur"), Path: "values.overtime_hours", Type: "number", Widget: widgetForForm(form, "text")},
		{Key: "attendance_status", Label: "Attendance Status", LabelI18n: localize("Attendance Status", "Status Kehadiran"), Path: "values.attendance_status", Type: "string", Widget: widgetForForm(form, "select"), Options: []string{"unscheduled", "present", "late", "partial", "absent", "on_leave"}},
		{Key: "absence_code_id", Label: "Absence Code", LabelI18n: localize("Absence Code", "Kode Absensi"), Path: "values.absence_code_id", Type: "string", Widget: widgetForForm(form, "select")},
		{Key: "leave_request_id", Label: "Leave Request", LabelI18n: localize("Leave Request", "Permintaan Cuti"), Path: "values.leave_request_id", Type: "string", Widget: widgetForForm(form, "select")},
		{Key: "overtime_request_id", Label: "Overtime Request", LabelI18n: localize("Overtime Request", "Permintaan Lembur"), Path: "values.overtime_request_id", Type: "string", Widget: widgetForForm(form, "select")},
		{Key: "attendance_adjustment_id", Label: "Attendance Adjustment", LabelI18n: localize("Attendance Adjustment", "Penyesuaian Kehadiran"), Path: "values.attendance_adjustment_id", Type: "string", Widget: widgetForForm(form, "select")},
		{Key: "overnight_shift", Label: "Overnight Shift", LabelI18n: localize("Overnight Shift", "Shift Lintas Hari"), Path: "values.overnight_shift", Type: "bool", Widget: widgetForForm(form, "checkbox")},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string", Widget: widgetForForm(form, "select"), Options: []string{"active", "inactive"}},
	}
}

func attendanceLeaveFields(form bool) []module.FieldDefinition {
	return []module.FieldDefinition{
		{Key: "employee_id", Label: "Employee", LabelI18n: localize("Employee", "Karyawan"), Path: "values.employee_id", Type: "string", Widget: widgetForForm(form, "select"), Required: true},
		{Key: "organization_id", Label: "Organization", LabelI18n: localize("Organization", "Organisasi"), Path: "values.organization_id", Type: "string", Widget: widgetForForm(form, "text")},
		{Key: "location_id", Label: "Location", LabelI18n: localize("Location", "Lokasi"), Path: "values.location_id", Type: "string", Widget: widgetForForm(form, "text")},
		{Key: "absence_code_id", Label: "Absence Code", LabelI18n: localize("Absence Code", "Kode Absensi"), Path: "values.absence_code_id", Type: "string", Widget: widgetForForm(form, "select"), Required: true},
		{Key: "start_date", Label: "Start Date", LabelI18n: localize("Start Date", "Tanggal Mulai"), Path: "values.start_date", Type: "string", Widget: widgetForForm(form, "text"), Required: true},
		{Key: "end_date", Label: "End Date", LabelI18n: localize("End Date", "Tanggal Selesai"), Path: "values.end_date", Type: "string", Widget: widgetForForm(form, "text"), Required: true},
		{Key: "requested_hours", Label: "Requested Hours", LabelI18n: localize("Requested Hours", "Jam Diminta"), Path: "values.requested_hours", Type: "number", Widget: widgetForForm(form, "text")},
		{Key: "approval_status", Label: "Approval Status", LabelI18n: localize("Approval Status", "Status Persetujuan"), Path: "values.approval_status", Type: "string", Widget: widgetForForm(form, "select"), Options: []string{"draft", "submitted", "approved", "rejected"}},
		{Key: "approval_policy_id", Label: "Approval Policy", LabelI18n: localize("Approval Policy", "Kebijakan Persetujuan"), Path: "values.approval_policy_id", Type: "string", Widget: widgetForForm(form, "select")},
		{Key: "approver_user_id", Label: "Approver User", LabelI18n: localize("Approver User", "Pengguna Penyetuju"), Path: "values.approver_user_id", Type: "string", Widget: widgetForForm(form, "select")},
		{Key: "notes", Label: "Notes", LabelI18n: localize("Notes", "Catatan"), Path: "values.notes", Type: "string", Widget: widgetForForm(form, "textarea")},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string", Widget: widgetForForm(form, "select"), Options: []string{"active", "inactive"}},
	}
}

func attendanceOvertimeFields(form bool) []module.FieldDefinition {
	return []module.FieldDefinition{
		{Key: "employee_id", Label: "Employee", LabelI18n: localize("Employee", "Karyawan"), Path: "values.employee_id", Type: "string", Widget: widgetForForm(form, "select"), Required: true},
		{Key: "attendance_date", Label: "Attendance Date", LabelI18n: localize("Attendance Date", "Tanggal Kehadiran"), Path: "values.attendance_date", Type: "string", Widget: widgetForForm(form, "text"), Required: true},
		{Key: "organization_id", Label: "Organization", LabelI18n: localize("Organization", "Organisasi"), Path: "values.organization_id", Type: "string", Widget: widgetForForm(form, "text")},
		{Key: "location_id", Label: "Location", LabelI18n: localize("Location", "Lokasi"), Path: "values.location_id", Type: "string", Widget: widgetForForm(form, "text")},
		{Key: "requested_hours", Label: "Requested Hours", LabelI18n: localize("Requested Hours", "Jam Diminta"), Path: "values.requested_hours", Type: "number", Widget: widgetForForm(form, "text"), Required: true},
		{Key: "approved_hours", Label: "Approved Hours", LabelI18n: localize("Approved Hours", "Jam Disetujui"), Path: "values.approved_hours", Type: "number", Widget: widgetForForm(form, "text")},
		{Key: "approval_status", Label: "Approval Status", LabelI18n: localize("Approval Status", "Status Persetujuan"), Path: "values.approval_status", Type: "string", Widget: widgetForForm(form, "select"), Options: []string{"draft", "submitted", "approved", "rejected"}},
		{Key: "approval_policy_id", Label: "Approval Policy", LabelI18n: localize("Approval Policy", "Kebijakan Persetujuan"), Path: "values.approval_policy_id", Type: "string", Widget: widgetForForm(form, "select")},
		{Key: "approver_user_id", Label: "Approver User", LabelI18n: localize("Approver User", "Pengguna Penyetuju"), Path: "values.approver_user_id", Type: "string", Widget: widgetForForm(form, "select")},
		{Key: "notes", Label: "Notes", LabelI18n: localize("Notes", "Catatan"), Path: "values.notes", Type: "string", Widget: widgetForForm(form, "textarea")},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string", Widget: widgetForForm(form, "select"), Options: []string{"active", "inactive"}},
	}
}

func attendanceAdjustmentFields(form bool) []module.FieldDefinition {
	return []module.FieldDefinition{
		{Key: "employee_id", Label: "Employee", LabelI18n: localize("Employee", "Karyawan"), Path: "values.employee_id", Type: "string", Widget: widgetForForm(form, "select"), Required: true},
		{Key: "attendance_date", Label: "Attendance Date", LabelI18n: localize("Attendance Date", "Tanggal Kehadiran"), Path: "values.attendance_date", Type: "string", Widget: widgetForForm(form, "text"), Required: true},
		{Key: "organization_id", Label: "Organization", LabelI18n: localize("Organization", "Organisasi"), Path: "values.organization_id", Type: "string", Widget: widgetForForm(form, "text")},
		{Key: "location_id", Label: "Location", LabelI18n: localize("Location", "Lokasi"), Path: "values.location_id", Type: "string", Widget: widgetForForm(form, "text")},
		{Key: "reason_code", Label: "Reason Code", LabelI18n: localize("Reason Code", "Kode Alasan"), Path: "values.reason_code", Type: "string", Widget: widgetForForm(form, "text"), Required: true},
		{Key: "corrected_in_at", Label: "Corrected In", LabelI18n: localize("Corrected In", "Masuk Koreksi"), Path: "values.corrected_in_at", Type: "string", Widget: widgetForForm(form, "text")},
		{Key: "corrected_out_at", Label: "Corrected Out", LabelI18n: localize("Corrected Out", "Keluar Koreksi"), Path: "values.corrected_out_at", Type: "string", Widget: widgetForForm(form, "text")},
		{Key: "corrected_break_minutes", Label: "Corrected Break", LabelI18n: localize("Corrected Break", "Istirahat Koreksi"), Path: "values.corrected_break_minutes", Type: "number", Widget: widgetForForm(form, "text")},
		{Key: "approval_status", Label: "Approval Status", LabelI18n: localize("Approval Status", "Status Persetujuan"), Path: "values.approval_status", Type: "string", Widget: widgetForForm(form, "select"), Options: []string{"draft", "submitted", "approved", "rejected"}},
		{Key: "approval_policy_id", Label: "Approval Policy", LabelI18n: localize("Approval Policy", "Kebijakan Persetujuan"), Path: "values.approval_policy_id", Type: "string", Widget: widgetForForm(form, "select")},
		{Key: "approver_user_id", Label: "Approver User", LabelI18n: localize("Approver User", "Pengguna Penyetuju"), Path: "values.approver_user_id", Type: "string", Widget: widgetForForm(form, "select")},
		{Key: "notes", Label: "Notes", LabelI18n: localize("Notes", "Catatan"), Path: "values.notes", Type: "string", Widget: widgetForForm(form, "textarea")},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string", Widget: widgetForForm(form, "select"), Options: []string{"active", "inactive"}},
	}
}
