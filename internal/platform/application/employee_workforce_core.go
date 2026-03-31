package application

import (
	"fmt"
	"strings"
	"time"

	"orbyte/internal/platform/model"
)

type EmployeeWorkforceCoreService struct {
	models *model.Service
}

func NewEmployeeWorkforceCoreService(models *model.Service) *EmployeeWorkforceCoreService {
	return &EmployeeWorkforceCoreService{models: models}
}

func (s *EmployeeWorkforceCoreService) ResolveEmployeeByUser(userID string) (model.Record, bool, error) {
	if s == nil || s.models == nil || strings.TrimSpace(userID) == "" {
		return model.Record{}, false, nil
	}
	items, _, err := s.models.List("employee_profile", model.Query{
		Filters:  map[string]string{"user_id": strings.TrimSpace(userID)},
		SortKey:  "employee_code",
		Desc:     true,
		Page:     1,
		PageSize: model.MaxPageSize,
	})
	if err != nil || len(items) == 0 {
		return model.Record{}, false, err
	}
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(workforceStringValue(item.Values["employment_status"])), "active") && strings.EqualFold(strings.TrimSpace(workforceStringValue(item.Values["status"])), "active") {
			return item, true, nil
		}
	}
	return model.Record{}, false, nil
}

func (s *EmployeeWorkforceCoreService) ResolveCurrentAssignment(employeeID string, at time.Time) (model.Record, bool, error) {
	if s == nil || s.models == nil || strings.TrimSpace(employeeID) == "" {
		return model.Record{}, false, nil
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	items, _, err := s.models.List("employee_assignment", model.Query{
		Filters:  map[string]string{"employee_id": strings.TrimSpace(employeeID)},
		SortKey:  "effective_from",
		Desc:     true,
		Page:     1,
		PageSize: model.MaxPageSize,
	})
	if err != nil || len(items) == 0 {
		return model.Record{}, false, err
	}
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(workforceStringValue(item.Values["status"])), "active") {
			if assignmentEffectiveAt(item, at) {
				return item, true, nil
			}
		}
	}
	return model.Record{}, false, nil
}

func assignmentEffectiveAt(item model.Record, at time.Time) bool {
	from := parseDateOnly(item.Values["effective_from"])
	if !from.IsZero() && at.Before(from) {
		return false
	}
	to := parseDateOnly(item.Values["effective_to"])
	if !to.IsZero() && at.After(to.Add(23*time.Hour+59*time.Minute+59*time.Second)) {
		return false
	}
	return true
}

func parseDateOnly(value any) time.Time {
	text := strings.TrimSpace(workforceStringValue(value))
	if text == "" {
		return time.Time{}
	}
	parsed, err := time.Parse("2006-01-02", text)
	if err != nil {
		return time.Time{}
	}
	return parsed.UTC()
}

func workforceStringValue(value any) string {
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
