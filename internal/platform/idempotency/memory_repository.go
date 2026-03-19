package idempotency

import "sort"

type MemoryRepository struct {
	records map[string]Record
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{records: map[string]Record{}}
}

func (r *MemoryRepository) Get(operation, key string) (Record, bool) {
	record, ok := r.records[storeKey(operation, key)]
	return record, ok
}

func (r *MemoryRepository) List() []Record {
	items := make([]Record, 0, len(r.records))
	for _, item := range r.records {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Operation == items[j].Operation {
			return items[i].Key < items[j].Key
		}
		return items[i].Operation < items[j].Operation
	})
	return items
}

func (r *MemoryRepository) Save(record Record) error {
	r.records[storeKey(record.Operation, record.Key)] = record
	return nil
}

func storeKey(operation, key string) string {
	return operation + "|" + key
}
