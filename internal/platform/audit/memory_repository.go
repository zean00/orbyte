package audit

import "sort"

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
		if filter.TargetType != "" && item.TargetType != filter.TargetType {
			continue
		}
		if filter.TargetID != "" && item.TargetID != filter.TargetID {
			continue
		}
		if filter.ActorID != "" && item.ActorID != filter.ActorID {
			continue
		}
		if filter.ActorKind != "" && item.ActorKind != filter.ActorKind {
			continue
		}
		if filter.OnBehalfOfUserID != "" && item.OnBehalfOfUserID != filter.OnBehalfOfUserID {
			continue
		}
		if filter.Action != "" && item.Action != filter.Action {
			continue
		}
		if filter.CorrelationID != "" && item.CorrelationID != filter.CorrelationID {
			continue
		}
		if filter.OrganizationID != "" && item.OrganizationID != filter.OrganizationID {
			continue
		}
		if filter.LocationID != "" && item.LocationID != filter.LocationID {
			continue
		}
		if filter.OperatingUnitID != "" && item.OperatingUnitID != filter.OperatingUnitID {
			continue
		}
		if !filter.OccurredFrom.IsZero() && item.OccurredAt.Before(filter.OccurredFrom) {
			continue
		}
		if !filter.OccurredTo.IsZero() && item.OccurredAt.After(filter.OccurredTo) {
			continue
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].OccurredAt.Before(items[j].OccurredAt)
	})
	return items
}
