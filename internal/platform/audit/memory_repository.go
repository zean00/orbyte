package audit

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type MemoryRepository struct {
	events []Event
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{events: []Event{}}
}

func (r *MemoryRepository) Save(event Event) error {
	r.events = append(r.events, event)
	return nil
}

func (r *MemoryRepository) List() []Event {
	items := append([]Event(nil), r.events...)
	sort.Slice(items, func(i, j int) bool {
		return items[i].OccurredAt.Before(items[j].OccurredAt)
	})
	return items
}

func (r *MemoryRepository) Query(filter Query) []Event {
	items := make([]Event, 0, len(r.events))
	for _, item := range r.events {
		if !matchesQuery(item, filter) {
			continue
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].OccurredAt.Before(items[j].OccurredAt)
	})
	return items
}

func (r *MemoryRepository) Search(filter Query) SearchResult {
	items := r.Query(filter)
	sortAuditEvents(items, filter)
	total := len(items)
	paged := paginateAuditEvents(items, filter.Page, filter.PageSize)
	return SearchResult{
		Items:   paged,
		Total:   total,
		Facets:  auditFacets(items),
		Summary: map[string]any{"count": total},
	}
}

func matchesQuery(item Event, filter Query) bool {
	if filter.TargetType != "" && item.TargetType != filter.TargetType {
		return false
	}
	if filter.TargetID != "" && item.TargetID != filter.TargetID {
		return false
	}
	if filter.ActorID != "" && item.ActorID != filter.ActorID {
		return false
	}
	if filter.ActorKind != "" && item.ActorKind != filter.ActorKind {
		return false
	}
	if filter.OnBehalfOfUserID != "" && item.OnBehalfOfUserID != filter.OnBehalfOfUserID {
		return false
	}
	if filter.Action != "" && item.Action != filter.Action {
		return false
	}
	if filter.CorrelationID != "" && item.CorrelationID != filter.CorrelationID {
		return false
	}
	if filter.RequestID != "" && item.RequestID != filter.RequestID {
		return false
	}
	if filter.DelegationGrantID != "" && item.DelegationGrantID != filter.DelegationGrantID {
		return false
	}
	if filter.FromState != "" && item.FromState != filter.FromState {
		return false
	}
	if filter.ToState != "" && item.ToState != filter.ToState {
		return false
	}
	if filter.OrganizationID != "" && item.OrganizationID != filter.OrganizationID {
		return false
	}
	if filter.LocationID != "" && item.LocationID != filter.LocationID {
		return false
	}
	if filter.OperatingUnitID != "" && item.OperatingUnitID != filter.OperatingUnitID {
		return false
	}
	if !filter.OccurredFrom.IsZero() && item.OccurredAt.Before(filter.OccurredFrom) {
		return false
	}
	if !filter.OccurredTo.IsZero() && item.OccurredAt.After(filter.OccurredTo) {
		return false
	}
	if filter.MetadataKey != "" {
		value, ok := item.Metadata[filter.MetadataKey]
		if !ok {
			return false
		}
		if filter.MetadataValue != "" && !strings.EqualFold(strings.TrimSpace(stringifyAuditValue(value)), strings.TrimSpace(filter.MetadataValue)) {
			return false
		}
	}
	if filter.Text != "" && !auditEventContains(item, filter.Text) {
		return false
	}
	return true
}

func sortAuditEvents(items []Event, filter Query) {
	sortKey := strings.TrimSpace(filter.Sort)
	if sortKey == "" {
		sortKey = "occurred_at"
	}
	desc := strings.EqualFold(filter.Direction, "desc")
	sort.SliceStable(items, func(i, j int) bool {
		left := auditSortValue(items[i], sortKey)
		right := auditSortValue(items[j], sortKey)
		if desc {
			return left > right
		}
		return left < right
	})
}

func auditSortValue(item Event, key string) string {
	switch key {
	case "action":
		return item.Action
	case "target_type":
		return item.TargetType
	case "target_id":
		return item.TargetID
	case "actor_id":
		return item.ActorID
	case "actor_kind":
		return item.ActorKind
	case "correlation_id":
		return item.CorrelationID
	default:
		return item.OccurredAt.UTC().Format("2006-01-02T15:04:05.000000000Z07:00")
	}
}

func paginateAuditEvents(items []Event, page, pageSize int) []Event {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	start := (page - 1) * pageSize
	if start >= len(items) {
		return []Event{}
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}
	return append([]Event(nil), items[start:end]...)
}

func auditFacets(items []Event) map[string]any {
	actions := map[string]int{}
	targetTypes := map[string]int{}
	actors := map[string]int{}
	for _, item := range items {
		actions[item.Action]++
		targetTypes[item.TargetType]++
		if item.ActorID != "" {
			actors[item.ActorID]++
		}
	}
	return map[string]any{"actions": actions, "target_types": targetTypes, "actors": actors}
}

func auditEventContains(item Event, text string) bool {
	needle := strings.ToLower(strings.TrimSpace(text))
	if needle == "" {
		return true
	}
	values := []string{item.ID, item.Action, item.TargetType, item.TargetID, item.ActorID, item.ActorKind, item.OnBehalfOfUserID, item.DelegationGrantID, item.FromState, item.ToState, item.OrganizationID, item.LocationID, item.OperatingUnitID, item.RequestID, item.CorrelationID}
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), needle) {
			return true
		}
	}
	for key, value := range item.Metadata {
		if strings.Contains(strings.ToLower(key), needle) || strings.Contains(strings.ToLower(stringifyAuditValue(value)), needle) {
			return true
		}
	}
	for key, value := range item.ChangeSummary {
		if strings.Contains(strings.ToLower(key), needle) || strings.Contains(strings.ToLower(stringifyAuditValue(value)), needle) {
			return true
		}
	}
	return false
}

func stringifyAuditValue(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	if raw, err := json.Marshal(value); err == nil {
		return strings.TrimSpace(string(raw))
	}
	return fmt.Sprint(value)
}
