package application

import (
	"fmt"
	"strings"
	"time"

	"orbyte/internal/platform/model"
)

type WorkforceAttendanceCoreService struct {
	models    *model.Service
	workforce *EmployeeWorkforceCoreService
}

type AttendanceScope struct {
	OrganizationID string
	LocationID     string
	StoreCode      string
	RegisterCode   string
	WarehouseCode  string
	WorkCenterCode string
}

type AttendanceEventInput struct {
	EmployeeID      string
	EventType       string
	OccurredAt      time.Time
	Source          string
	Notes           string
	RosterSlotID    string
	AttendanceDayID string
	Scope           AttendanceScope
}

func NewWorkforceAttendanceCoreService(models *model.Service, workforce *EmployeeWorkforceCoreService) *WorkforceAttendanceCoreService {
	return &WorkforceAttendanceCoreService{models: models, workforce: workforce}
}

func (s *WorkforceAttendanceCoreService) ResolveRosterSlot(employeeID string, at time.Time, scope AttendanceScope) (model.Record, bool, error) {
	if s == nil || s.models == nil || strings.TrimSpace(employeeID) == "" {
		return model.Record{}, false, nil
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	items, _, err := s.models.List("workforce_roster_slot", model.Query{
		Filters:  map[string]string{"employee_id": strings.TrimSpace(employeeID)},
		SortKey:  "shift_date",
		Desc:     true,
		Page:     1,
		PageSize: model.MaxPageSize,
	})
	if err != nil {
		return model.Record{}, false, err
	}
	for _, item := range items {
		if !strings.EqualFold(strings.TrimSpace(attendanceStringValue(item.Values["status"])), "active") {
			continue
		}
		if !s.rosterPublished(item) {
			continue
		}
		if !rosterScopeMatches(item, scope) {
			continue
		}
		if !rosterSlotAppliesAt(item, at) {
			continue
		}
		return item, true, nil
	}
	return model.Record{}, false, nil
}

func (s *WorkforceAttendanceCoreService) RecordAttendanceEvent(input AttendanceEventInput, actorID string) (model.Record, model.Record, error) {
	event, day, _, err := s.recordAttendanceEventWithState(input, actorID)
	return event, day, err
}

func (s *WorkforceAttendanceCoreService) recordAttendanceEventWithState(input AttendanceEventInput, actorID string) (model.Record, model.Record, model.Record, error) {
	if s == nil || s.models == nil || strings.TrimSpace(input.EmployeeID) == "" {
		return model.Record{}, model.Record{}, model.Record{}, nil
	}
	if input.OccurredAt.IsZero() {
		input.OccurredAt = time.Now().UTC()
	}
	if strings.TrimSpace(input.EventType) == "" {
		input.EventType = "clock_in"
	}
	if strings.TrimSpace(input.Source) == "" {
		input.Source = "manual"
	}
	slot, attendanceDay, attendanceDate, err := s.resolveAttendanceEventContext(&input)
	if err != nil {
		return model.Record{}, model.Record{}, model.Record{}, err
	}
	previousDay, _ := s.findAttendanceDay(input.EmployeeID, attendanceDate)
	event, err := s.models.Create("attendance_event", actorID, map[string]any{
		"employee_id":       strings.TrimSpace(input.EmployeeID),
		"attendance_date":   attendanceDate,
		"event_type":        strings.TrimSpace(input.EventType),
		"occurred_at":       input.OccurredAt.UTC().Format(time.RFC3339),
		"source":            strings.TrimSpace(input.Source),
		"organization_id":   strings.TrimSpace(input.Scope.OrganizationID),
		"location_id":       strings.TrimSpace(input.Scope.LocationID),
		"store_code":        strings.TrimSpace(input.Scope.StoreCode),
		"register_code":     strings.TrimSpace(input.Scope.RegisterCode),
		"warehouse_code":    strings.TrimSpace(input.Scope.WarehouseCode),
		"work_center_code":  strings.TrimSpace(input.Scope.WorkCenterCode),
		"roster_slot_id":    strings.TrimSpace(input.RosterSlotID),
		"attendance_day_id": strings.TrimSpace(input.AttendanceDayID),
		"notes":             strings.TrimSpace(input.Notes),
		"status":            "active",
	})
	if err != nil {
		return model.Record{}, model.Record{}, model.Record{}, err
	}
	day, err := s.syncAttendanceDayForDate(input.EmployeeID, attendanceDate, input.OccurredAt)
	if err != nil {
		rollbackErr := s.rollbackAttendanceEventState(event.ID, previousDay, attendanceDay)
		if rollbackErr != nil {
			return model.Record{}, model.Record{}, model.Record{}, fmt.Errorf("sync attendance day: %w (rollback failed: %v)", err, rollbackErr)
		}
		return model.Record{}, model.Record{}, model.Record{}, err
	}
	if day.ID != "" && event.ID != "" && textValue(event.Values["attendance_day_id"]) != day.ID {
		updatedValues := cloneMap(event.Values)
		updatedValues["attendance_day_id"] = day.ID
		updatedEvent, updateErr := s.models.Update("attendance_event", event.ID, actorID, updatedValues, event.Version)
		if updateErr != nil {
			rollbackErr := s.rollbackAttendanceEventState(event.ID, previousDay, day)
			if rollbackErr != nil {
				return model.Record{}, model.Record{}, model.Record{}, fmt.Errorf("link attendance event to day: %w (rollback failed: %v)", updateErr, rollbackErr)
			}
			return model.Record{}, model.Record{}, model.Record{}, updateErr
		}
		event = updatedEvent
	}
	_ = slot
	return event, day, previousDay, nil
}

func (s *WorkforceAttendanceCoreService) SyncAttendanceDay(employeeID string, at time.Time) (model.Record, error) {
	if s == nil || s.models == nil || strings.TrimSpace(employeeID) == "" {
		return model.Record{}, nil
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	return s.syncAttendanceDayForDate(employeeID, at.UTC().Format("2006-01-02"), at)
}

func (s *WorkforceAttendanceCoreService) syncAttendanceDayForDate(employeeID, day string, at time.Time) (model.Record, error) {
	if s == nil || s.models == nil || strings.TrimSpace(employeeID) == "" {
		return model.Record{}, nil
	}
	day = strings.TrimSpace(day)
	if day == "" {
		if at.IsZero() {
			at = time.Now().UTC()
		}
		day = at.UTC().Format("2006-01-02")
	}
	if at.IsZero() {
		at = attendanceDate(day)
	}
	syncAt := attendanceDate(day)
	if syncAt.IsZero() {
		syncAt = at
	}
	slot, _ := s.findRosterSlotForDay(employeeID, day, at)
	events, err := s.listAttendanceEvents(employeeID, day)
	if err != nil {
		return model.Record{}, err
	}
	adjustment, _ := s.findApprovedAdjustment(employeeID, day)
	leave, hasLeave := s.findApprovedLeave(employeeID, syncAt)
	overtime, hasOvertime := s.findApprovedOvertime(employeeID, day)

	actualIn, actualOut, breakMinutes := summarizeAttendanceEvents(events)
	if adjustment.ID != "" {
		if corrected := attendanceTimestamp(adjustment.Values["corrected_in_at"]); !corrected.IsZero() {
			actualIn = corrected
		}
		if corrected := attendanceTimestamp(adjustment.Values["corrected_out_at"]); !corrected.IsZero() {
			actualOut = corrected
		}
		if minutes := attendanceInt(adjustment.Values["corrected_break_minutes"]); minutes > 0 {
			breakMinutes = minutes
		}
	}

	plannedStart, plannedEnd, shiftTemplateID, overnight, lateGrace, earlyGrace := rosterTiming(slot)
	workedHours := attendanceWorkedHours(actualIn, actualOut, breakMinutes)
	lateMinutes := 0
	earlyOutMinutes := 0
	overtimeHours := 0.0
	status := "unscheduled"
	if hasLeave {
		status = "on_leave"
	} else if !actualIn.IsZero() && actualOut.IsZero() {
		status = "partial"
	} else if !actualIn.IsZero() {
		status = "present"
	} else if slot.ID != "" {
		status = "absent"
	}
	if !plannedStart.IsZero() && !actualIn.IsZero() {
		graceAt := plannedStart.Add(time.Duration(lateGrace) * time.Minute)
		if actualIn.After(graceAt) {
			lateMinutes = int(actualIn.Sub(graceAt).Minutes())
			status = "late"
		}
	}
	if !plannedEnd.IsZero() && !actualOut.IsZero() {
		earlyThreshold := plannedEnd.Add(-time.Duration(earlyGrace) * time.Minute)
		if actualOut.Before(earlyThreshold) {
			earlyOutMinutes = int(earlyThreshold.Sub(actualOut).Minutes())
		}
		if actualOut.After(plannedEnd) {
			overtimeHours = roundAttendanceHours(actualOut.Sub(plannedEnd).Hours())
		}
	}
	if hasOvertime {
		if approvedHours := attendanceFloat(overtime.Values["approved_hours"]); approvedHours > 0 {
			overtimeHours = approvedHours
		}
	}

	values := map[string]any{
		"employee_id":              strings.TrimSpace(employeeID),
		"attendance_date":          day,
		"organization_id":          firstAttendanceValue(slot.Values["organization_id"], leave.Values["organization_id"]),
		"location_id":              firstAttendanceValue(slot.Values["location_id"], leave.Values["location_id"]),
		"roster_slot_id":           slot.ID,
		"shift_template_id":        shiftTemplateID,
		"planned_start_at":         attendanceRFC3339(plannedStart),
		"planned_end_at":           attendanceRFC3339(plannedEnd),
		"actual_in_at":             attendanceRFC3339(actualIn),
		"actual_out_at":            attendanceRFC3339(actualOut),
		"break_minutes":            breakMinutes,
		"worked_hours":             workedHours,
		"late_minutes":             lateMinutes,
		"early_out_minutes":        earlyOutMinutes,
		"overtime_hours":           overtimeHours,
		"attendance_status":        status,
		"absence_code_id":          attendanceStringValue(leave.Values["absence_code_id"]),
		"leave_request_id":         leave.ID,
		"overtime_request_id":      overtime.ID,
		"attendance_adjustment_id": adjustment.ID,
		"overnight_shift":          overnight,
		"status":                   "active",
	}
	return s.upsertAttendanceDay(employeeID, day, values)
}

func (s *WorkforceAttendanceCoreService) resolveAttendanceEventContext(input *AttendanceEventInput) (model.Record, model.Record, string, error) {
	var slot model.Record
	var attendanceDay model.Record
	var err error
	if strings.TrimSpace(input.AttendanceDayID) != "" {
		attendanceDay, err = s.models.Get("attendance_day", input.AttendanceDayID)
		if err != nil {
			return model.Record{}, model.Record{}, "", err
		}
		if strings.TrimSpace(input.RosterSlotID) == "" {
			input.RosterSlotID = attendanceStringValue(attendanceDay.Values["roster_slot_id"])
		}
	}
	if strings.TrimSpace(input.RosterSlotID) != "" {
		slot, err = s.models.Get("workforce_roster_slot", input.RosterSlotID)
		if err != nil {
			return model.Record{}, model.Record{}, "", err
		}
	} else if resolved, ok, resolveErr := s.ResolveRosterSlot(input.EmployeeID, input.OccurredAt, input.Scope); resolveErr != nil {
		return model.Record{}, model.Record{}, "", resolveErr
	} else if ok {
		slot = resolved
		input.RosterSlotID = slot.ID
	}
	s.applyScopeFromRosterSlot(input, slot)
	attendanceDate := strings.TrimSpace(attendanceStringValue(attendanceDay.Values["attendance_date"]))
	if attendanceDate == "" {
		attendanceDate = strings.TrimSpace(attendanceStringValue(slot.Values["shift_date"]))
	}
	if attendanceDate == "" {
		attendanceDate = input.OccurredAt.UTC().Format("2006-01-02")
	}
	return slot, attendanceDay, attendanceDate, nil
}

func (s *WorkforceAttendanceCoreService) applyScopeFromRosterSlot(input *AttendanceEventInput, slot model.Record) {
	if slot.ID == "" {
		return
	}
	if input.Scope.OrganizationID == "" {
		input.Scope.OrganizationID = attendanceStringValue(slot.Values["organization_id"])
	}
	if input.Scope.LocationID == "" {
		input.Scope.LocationID = attendanceStringValue(slot.Values["location_id"])
	}
	if input.Scope.StoreCode == "" {
		input.Scope.StoreCode = attendanceStringValue(slot.Values["store_code"])
	}
	if input.Scope.RegisterCode == "" {
		input.Scope.RegisterCode = attendanceStringValue(slot.Values["register_code"])
	}
	if input.Scope.WarehouseCode == "" {
		input.Scope.WarehouseCode = attendanceStringValue(slot.Values["warehouse_code"])
	}
	if input.Scope.WorkCenterCode == "" {
		input.Scope.WorkCenterCode = attendanceStringValue(slot.Values["work_center_code"])
	}
}

func (s *WorkforceAttendanceCoreService) findAttendanceDay(employeeID, day string) (model.Record, bool) {
	items, _, err := s.models.List("attendance_day", model.Query{
		Filters:  map[string]string{"employee_id": strings.TrimSpace(employeeID), "attendance_date": strings.TrimSpace(day)},
		Page:     1,
		PageSize: 1,
	})
	if err != nil || len(items) == 0 {
		return model.Record{}, false
	}
	return items[0], true
}

func (s *WorkforceAttendanceCoreService) rollbackAttendanceEventState(eventID string, previousDay, currentDay model.Record) error {
	if strings.TrimSpace(eventID) != "" {
		if err := s.deleteModelRecord("attendance_event", eventID); err != nil {
			return err
		}
	}
	if previousDay.ID != "" {
		return s.models.WithRawRecordSave(previousDay)
	}
	if currentDay.ID != "" {
		return s.deleteModelRecord("attendance_day", currentDay.ID)
	}
	return nil
}

func (s *WorkforceAttendanceCoreService) deleteModelRecord(modelKey, id string) error {
	repo := s.models.Repository()
	if repo == nil {
		return nil
	}
	return repo.DeleteRecord(modelKey, strings.TrimSpace(id))
}

func (s *WorkforceAttendanceCoreService) rosterPublished(slot model.Record) bool {
	rosterID := strings.TrimSpace(attendanceStringValue(slot.Values["roster_id"]))
	if rosterID == "" {
		return true
	}
	roster, err := s.models.Get("workforce_roster", rosterID)
	if err != nil {
		return false
	}
	status := strings.TrimSpace(attendanceStringValue(roster.Values["status"]))
	return status == "" || strings.EqualFold(status, "published") || strings.EqualFold(status, "active")
}

func rosterScopeMatches(slot model.Record, scope AttendanceScope) bool {
	if scope.LocationID != "" {
		value := strings.TrimSpace(attendanceStringValue(slot.Values["location_id"]))
		if value != "" && value != strings.TrimSpace(scope.LocationID) {
			return false
		}
	}
	if scope.StoreCode != "" && !matchesScopeCode(slot.Values["store_code"], scope.StoreCode) {
		return false
	}
	if scope.RegisterCode != "" && !matchesScopeCode(slot.Values["register_code"], scope.RegisterCode) {
		return false
	}
	if scope.WarehouseCode != "" && !matchesScopeCode(slot.Values["warehouse_code"], scope.WarehouseCode) {
		return false
	}
	if scope.WorkCenterCode != "" && !matchesScopeCode(slot.Values["work_center_code"], scope.WorkCenterCode) {
		return false
	}
	return true
}

func matchesScopeCode(value any, expected string) bool {
	text := strings.TrimSpace(attendanceStringValue(value))
	if text == "" {
		return true
	}
	return strings.EqualFold(text, strings.TrimSpace(expected))
}

func rosterSlotAppliesAt(slot model.Record, at time.Time) bool {
	start, end := rosterSlotWindow(slot)
	if start.IsZero() || end.IsZero() {
		return false
	}
	return !at.Before(start) && !at.After(end)
}

func rosterSlotWindow(slot model.Record) (time.Time, time.Time) {
	shiftDate := attendanceDate(slot.Values["shift_date"])
	if shiftDate.IsZero() {
		return time.Time{}, time.Time{}
	}
	startClock := attendanceClock(slot.Values["planned_start_time"])
	endClock := attendanceClock(slot.Values["planned_end_time"])
	start := combineDateAndClock(shiftDate, startClock)
	end := combineDateAndClock(shiftDate, endClock)
	overnight := attendanceBool(slot.Values["overnight"])
	if !start.IsZero() && !end.IsZero() && (overnight || !end.After(start)) {
		end = end.Add(24 * time.Hour)
	}
	return start, end
}

func (s *WorkforceAttendanceCoreService) findRosterSlotForDay(employeeID, day string, at time.Time) (model.Record, bool) {
	if slot, ok, err := s.ResolveRosterSlot(employeeID, at, AttendanceScope{}); err == nil && ok {
		return slot, true
	}
	items, _, err := s.models.List("workforce_roster_slot", model.Query{
		Filters:  map[string]string{"employee_id": strings.TrimSpace(employeeID), "shift_date": strings.TrimSpace(day)},
		SortKey:  "planned_start_time",
		Page:     1,
		PageSize: model.MaxPageSize,
	})
	if err != nil {
		return model.Record{}, false
	}
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(attendanceStringValue(item.Values["status"])), "active") && s.rosterPublished(item) {
			return item, true
		}
	}
	return model.Record{}, false
}

func (s *WorkforceAttendanceCoreService) listAttendanceEvents(employeeID, day string) ([]model.Record, error) {
	items, _, err := s.models.List("attendance_event", model.Query{
		Filters:  map[string]string{"employee_id": strings.TrimSpace(employeeID), "attendance_date": strings.TrimSpace(day)},
		SortKey:  "occurred_at",
		Page:     1,
		PageSize: model.MaxPageSize,
	})
	if err != nil {
		return nil, err
	}
	active := make([]model.Record, 0, len(items))
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(attendanceStringValue(item.Values["status"])), "active") {
			active = append(active, item)
		}
	}
	return active, nil
}

func (s *WorkforceAttendanceCoreService) findApprovedAdjustment(employeeID, day string) (model.Record, bool) {
	items, _, err := s.models.List("attendance_adjustment", model.Query{
		Filters:  map[string]string{"employee_id": strings.TrimSpace(employeeID), "attendance_date": strings.TrimSpace(day)},
		SortKey:  "updated_at",
		Desc:     true,
		Page:     1,
		PageSize: model.MaxPageSize,
	})
	if err != nil {
		return model.Record{}, false
	}
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(attendanceStringValue(item.Values["approval_status"])), "approved") && strings.EqualFold(strings.TrimSpace(attendanceStringValue(item.Values["status"])), "active") {
			return item, true
		}
	}
	return model.Record{}, false
}

func (s *WorkforceAttendanceCoreService) findApprovedLeave(employeeID string, at time.Time) (model.Record, bool) {
	items, _, err := s.models.List("leave_request", model.Query{
		Filters:  map[string]string{"employee_id": strings.TrimSpace(employeeID)},
		SortKey:  "start_date",
		Desc:     true,
		Page:     1,
		PageSize: model.MaxPageSize,
	})
	if err != nil {
		return model.Record{}, false
	}
	for _, item := range items {
		if !strings.EqualFold(strings.TrimSpace(attendanceStringValue(item.Values["approval_status"])), "approved") {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(attendanceStringValue(item.Values["status"])), "active") {
			continue
		}
		start := attendanceDate(item.Values["start_date"])
		end := attendanceDate(item.Values["end_date"])
		if start.IsZero() {
			continue
		}
		if end.IsZero() {
			end = start
		}
		if !at.Before(start) && !at.After(end.Add(23*time.Hour+59*time.Minute+59*time.Second)) {
			return item, true
		}
	}
	return model.Record{}, false
}

func (s *WorkforceAttendanceCoreService) findApprovedOvertime(employeeID, day string) (model.Record, bool) {
	items, _, err := s.models.List("overtime_request", model.Query{
		Filters:  map[string]string{"employee_id": strings.TrimSpace(employeeID), "attendance_date": strings.TrimSpace(day)},
		SortKey:  "updated_at",
		Desc:     true,
		Page:     1,
		PageSize: model.MaxPageSize,
	})
	if err != nil {
		return model.Record{}, false
	}
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(attendanceStringValue(item.Values["approval_status"])), "approved") &&
			strings.EqualFold(strings.TrimSpace(attendanceStringValue(item.Values["status"])), "active") {
			return item, true
		}
	}
	return model.Record{}, false
}

func summarizeAttendanceEvents(events []model.Record) (time.Time, time.Time, int) {
	var actualIn time.Time
	var actualOut time.Time
	breakOpen := time.Time{}
	breakMinutes := 0
	for _, item := range events {
		occurredAt := attendanceTimestamp(item.Values["occurred_at"])
		if occurredAt.IsZero() {
			continue
		}
		switch strings.TrimSpace(attendanceStringValue(item.Values["event_type"])) {
		case "clock_in":
			if actualIn.IsZero() || occurredAt.Before(actualIn) {
				actualIn = occurredAt
			}
		case "clock_out":
			if actualOut.IsZero() || occurredAt.After(actualOut) {
				actualOut = occurredAt
			}
		case "break_start":
			if breakOpen.IsZero() || occurredAt.Before(breakOpen) {
				breakOpen = occurredAt
			}
		case "break_end":
			if !breakOpen.IsZero() && occurredAt.After(breakOpen) {
				breakMinutes += int(occurredAt.Sub(breakOpen).Minutes())
				breakOpen = time.Time{}
			}
		}
	}
	return actualIn, actualOut, breakMinutes
}

func rosterTiming(slot model.Record) (time.Time, time.Time, string, bool, int, int) {
	if slot.ID == "" {
		return time.Time{}, time.Time{}, "", false, 0, 0
	}
	start, end := rosterSlotWindow(slot)
	return start, end, strings.TrimSpace(attendanceStringValue(slot.Values["shift_template_id"])), attendanceBool(slot.Values["overnight"]), attendanceInt(slot.Values["late_grace_minutes"]), attendanceInt(slot.Values["early_out_grace_minutes"])
}

func attendanceWorkedHours(actualIn, actualOut time.Time, breakMinutes int) float64 {
	if actualIn.IsZero() || actualOut.IsZero() || !actualOut.After(actualIn) {
		return 0
	}
	hours := actualOut.Sub(actualIn).Hours() - (float64(breakMinutes) / 60)
	if hours < 0 {
		return 0
	}
	return roundAttendanceHours(hours)
}

func roundAttendanceHours(value float64) float64 {
	return float64(int(value*100+0.5)) / 100
}

func (s *WorkforceAttendanceCoreService) upsertAttendanceDay(employeeID, day string, values map[string]any) (model.Record, error) {
	items, _, err := s.models.List("attendance_day", model.Query{
		Filters:  map[string]string{"employee_id": strings.TrimSpace(employeeID), "attendance_date": strings.TrimSpace(day)},
		Page:     1,
		PageSize: 1,
	})
	if err != nil {
		return model.Record{}, err
	}
	if len(items) == 0 {
		return s.models.Create("attendance_day", "system", values)
	}
	current := items[0]
	merged := cloneMap(current.Values)
	for key, value := range values {
		merged[key] = value
	}
	return s.models.Update("attendance_day", current.ID, "system", merged, current.Version)
}

func attendanceDate(value any) time.Time {
	text := strings.TrimSpace(attendanceStringValue(value))
	if text == "" {
		return time.Time{}
	}
	parsed, err := time.Parse("2006-01-02", text)
	if err != nil {
		return time.Time{}
	}
	return parsed.UTC()
}

func attendanceClock(value any) time.Duration {
	text := strings.TrimSpace(attendanceStringValue(value))
	if text == "" {
		return 0
	}
	parsed, err := time.Parse("15:04", text)
	if err != nil {
		return 0
	}
	return time.Duration(parsed.Hour())*time.Hour + time.Duration(parsed.Minute())*time.Minute
}

func combineDateAndClock(day time.Time, clock time.Duration) time.Time {
	if day.IsZero() {
		return time.Time{}
	}
	return day.UTC().Add(clock)
}

func attendanceTimestamp(value any) time.Time {
	text := strings.TrimSpace(attendanceStringValue(value))
	if text == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339, text)
	if err != nil {
		return time.Time{}
	}
	return parsed.UTC()
}

func attendanceRFC3339(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func attendanceStringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []byte:
		return string(typed)
	default:
		if value == nil {
			return ""
		}
		return fmt.Sprint(value)
	}
}

func attendanceInt(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return 0
	}
}

func attendanceFloat(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	default:
		return 0
	}
}

func attendanceBool(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(strings.TrimSpace(typed), "true")
	default:
		return false
	}
}

func firstAttendanceValue(values ...any) string {
	for _, value := range values {
		if text := strings.TrimSpace(attendanceStringValue(value)); text != "" {
			return text
		}
	}
	return ""
}
