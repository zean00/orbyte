package notification

import "sort"

type MemoryRepository struct {
	items []Item
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{items: []Item{}}
}

func (r *MemoryRepository) Save(item Item) error {
	for i := range r.items {
		if r.items[i].ID == item.ID {
			r.items[i] = cloneItem(item)
			return nil
		}
	}
	r.items = append(r.items, cloneItem(item))
	return nil
}

func (r *MemoryRepository) Find(id string) (Item, bool) {
	for _, item := range r.items {
		if item.ID == id {
			return cloneItem(item), true
		}
	}
	return Item{}, false
}

func (r *MemoryRepository) List(filter Filter) []Item {
	items := make([]Item, 0, len(r.items))
	for _, item := range r.items {
		if filter.UserID != "" && item.UserID != filter.UserID {
			continue
		}
		if filter.Category != "" && item.Category != filter.Category {
			continue
		}
		if filter.Status != "" && item.Status != filter.Status {
			continue
		}
		if filter.TargetType != "" && item.TargetType != filter.TargetType {
			continue
		}
		if filter.TargetID != "" && item.TargetID != filter.TargetID {
			continue
		}
		items = append(items, cloneItem(item))
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Status == items[j].Status {
			return items[i].CreatedAt.After(items[j].CreatedAt)
		}
		if items[i].Status == "unread" {
			return true
		}
		if items[j].Status == "unread" {
			return false
		}
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	return items
}
