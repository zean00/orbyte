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
