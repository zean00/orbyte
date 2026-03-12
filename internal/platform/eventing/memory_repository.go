package eventing

import "sort"

type MemoryRepository struct {
	events      []Event
	outbox      []OutboxRecord
	deliveries  []OutboxDeliveryRecord
	deadLetters []DeadLetterRecord
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{events: []Event{}, outbox: []OutboxRecord{}, deliveries: []OutboxDeliveryRecord{}, deadLetters: []DeadLetterRecord{}}
}

func (r *MemoryRepository) SaveEvent(event Event) error {
	r.events = append(r.events, event)
	return nil
}

func (r *MemoryRepository) SaveOutbox(record OutboxRecord) error {
	r.outbox = append(r.outbox, record)
	return nil
}

func (r *MemoryRepository) SaveDelivery(record OutboxDeliveryRecord) error {
	for _, existing := range r.deliveries {
		if existing.OutboxID == record.OutboxID && existing.SinkName == record.SinkName {
			return nil
		}
	}
	r.deliveries = append(r.deliveries, record)
	return nil
}

func (r *MemoryRepository) GetEvent(eventID string) (Event, bool) {
	for _, event := range r.events {
		if event.ID == eventID {
			return event, true
		}
	}
	return Event{}, false
}

func (r *MemoryRepository) ListEvents() []Event {
	items := append([]Event(nil), r.events...)
	sort.Slice(items, func(i, j int) bool { return items[i].OccurredAt.Before(items[j].OccurredAt) })
	return items
}

func (r *MemoryRepository) ListOutbox() []OutboxRecord {
	items := append([]OutboxRecord(nil), r.outbox...)
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.Before(items[j].CreatedAt) })
	return items
}

func (r *MemoryRepository) ListDeliveries() []OutboxDeliveryRecord {
	items := append([]OutboxDeliveryRecord(nil), r.deliveries...)
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.Before(items[j].CreatedAt) })
	return items
}

func (r *MemoryRepository) ListDeliveriesByOutbox(outboxID string) []OutboxDeliveryRecord {
	items := make([]OutboxDeliveryRecord, 0)
	for _, item := range r.deliveries {
		if item.OutboxID == outboxID {
			items = append(items, item)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.Before(items[j].CreatedAt) })
	return items
}

func (r *MemoryRepository) ListDeadLetters() []DeadLetterRecord {
	items := append([]DeadLetterRecord(nil), r.deadLetters...)
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.Before(items[j].CreatedAt) })
	return items
}

func (r *MemoryRepository) ClaimPendingDeliveries(limit int) []OutboxDeliveryRecord {
	claimed := make([]OutboxDeliveryRecord, 0, limit)
	for i := range r.deliveries {
		if r.deliveries[i].Status != "pending" {
			continue
		}
		r.deliveries[i].Status = "processing"
		r.deliveries[i].AttemptCount++
		claimed = append(claimed, r.deliveries[i])
		if limit > 0 && len(claimed) >= limit {
			break
		}
	}
	return claimed
}

func (r *MemoryRepository) ClaimPending(limit int) []OutboxRecord {
	deliveryItems := r.ClaimPendingDeliveries(limit)
	seen := map[string]struct{}{}
	items := make([]OutboxRecord, 0, len(deliveryItems))
	for _, delivery := range deliveryItems {
		if _, ok := seen[delivery.OutboxID]; ok {
			continue
		}
		seen[delivery.OutboxID] = struct{}{}
		for _, outbox := range r.outbox {
			if outbox.ID == delivery.OutboxID {
				items = append(items, outbox)
				break
			}
		}
	}
	return items
}

func (r *MemoryRepository) MarkDispatched(outboxID string, update OutboxRecord) error {
	for i := range r.outbox {
		if r.outbox[i].ID == outboxID {
			r.outbox[i].Status = update.Status
			r.outbox[i].DispatchedAt = update.DispatchedAt
			break
		}
	}
	return nil
}

func (r *MemoryRepository) MarkFailed(outboxID string, update OutboxRecord) error {
	for i := range r.outbox {
		if r.outbox[i].ID == outboxID {
			r.outbox[i].Status = update.Status
			r.outbox[i].AttemptCount = update.AttemptCount
			r.outbox[i].LastError = update.LastError
			return nil
		}
	}
	return nil
}

func (r *MemoryRepository) MarkDeliveryDispatched(deliveryID string, update OutboxDeliveryRecord) error {
	for i := range r.deliveries {
		if r.deliveries[i].ID == deliveryID {
			r.deliveries[i].Status = update.Status
			r.deliveries[i].DispatchedAt = update.DispatchedAt
			r.deliveries[i].LastError = ""
			return nil
		}
	}
	return nil
}

func (r *MemoryRepository) MarkDeliveryFailed(deliveryID string, update OutboxDeliveryRecord) error {
	for i := range r.deliveries {
		if r.deliveries[i].ID == deliveryID {
			r.deliveries[i].Status = update.Status
			r.deliveries[i].AttemptCount = update.AttemptCount
			r.deliveries[i].LastError = update.LastError
			return nil
		}
	}
	return nil
}

func (r *MemoryRepository) SaveDeadLetter(record DeadLetterRecord) error {
	r.deadLetters = append(r.deadLetters, record)
	return nil
}
