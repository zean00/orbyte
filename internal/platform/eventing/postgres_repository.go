package eventing

import (
	"context"
	"database/sql"
	"encoding/json"
	"sort"
	"time"
)

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) SaveEvent(event Event) error {
	payload, err := json.Marshal(event.Payload)
	if err != nil {
		return err
	}
	const query = `
		INSERT INTO domain_events (
			event_id, event_type, event_version, schema_version, aggregate_type, aggregate_id, actor_id, correlation_id, organization_id, location_id, module_key, occurred_at, payload_json
		) VALUES ($1,$2,$3,NULLIF($4,''),$5,$6,NULLIF($7,''),NULLIF($8,''),NULLIF($9,''),NULLIF($10,''),NULLIF($11,''),$12,$13)`
	_, err = r.db.ExecContext(context.Background(), query,
		event.ID, event.Type, event.Version, event.SchemaVersion, event.AggregateType, event.AggregateID, event.ActorID, event.CorrelationID, event.OrganizationID, event.LocationID, event.ModuleKey, event.OccurredAt, payload,
	)
	return err
}

func (r *PostgresRepository) SaveOutbox(record OutboxRecord) error {
	const query = `
		INSERT INTO outbox_records (
			outbox_id, event_id, event_type, status, attempt_count, last_error, created_at, dispatched_at
		) VALUES ($1,$2,$3,$4,$5,NULLIF($6,''),$7,$8)`
	_, err := r.db.ExecContext(context.Background(), query,
		record.ID, record.EventID, record.EventType, record.Status, record.AttemptCount, record.LastError, record.CreatedAt, nullableTime(record.DispatchedAt),
	)
	return err
}

func (r *PostgresRepository) SaveDelivery(record OutboxDeliveryRecord) error {
	const query = `
		INSERT INTO outbox_deliveries (
			delivery_id, outbox_id, event_id, event_type, sink_name, status, attempt_count, last_error, created_at, dispatched_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,NULLIF($8,''),$9,$10)
		ON CONFLICT (outbox_id, sink_name) DO NOTHING`
	_, err := r.db.ExecContext(context.Background(), query,
		record.ID, record.OutboxID, record.EventID, record.EventType, record.SinkName, record.Status, record.AttemptCount, record.LastError, record.CreatedAt, nullableTime(record.DispatchedAt),
	)
	return err
}

func (r *PostgresRepository) GetEvent(eventID string) (Event, bool) {
	const query = `SELECT event_id, event_type, event_version, COALESCE(schema_version,''), aggregate_type, aggregate_id, COALESCE(actor_id,''), COALESCE(correlation_id,''), COALESCE(organization_id,''), COALESCE(location_id,''), COALESCE(module_key,''), occurred_at, COALESCE(payload_json,'{}'::jsonb) FROM domain_events WHERE event_id = $1`
	var item Event
	var payload []byte
	err := r.db.QueryRowContext(context.Background(), query, eventID).Scan(&item.ID, &item.Type, &item.Version, &item.SchemaVersion, &item.AggregateType, &item.AggregateID, &item.ActorID, &item.CorrelationID, &item.OrganizationID, &item.LocationID, &item.ModuleKey, &item.OccurredAt, &payload)
	if err != nil {
		return Event{}, false
	}
	_ = json.Unmarshal(payload, &item.Payload)
	return item, true
}

func (r *PostgresRepository) ListEvents() []Event {
	const query = `SELECT event_id, event_type, event_version, COALESCE(schema_version,''), aggregate_type, aggregate_id, COALESCE(actor_id,''), COALESCE(correlation_id,''), COALESCE(organization_id,''), COALESCE(location_id,''), COALESCE(module_key,''), occurred_at, COALESCE(payload_json,'{}'::jsonb) FROM domain_events`
	rows, err := r.db.QueryContext(context.Background(), query)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]Event, 0)
	for rows.Next() {
		var item Event
		var payload []byte
		if err := rows.Scan(&item.ID, &item.Type, &item.Version, &item.SchemaVersion, &item.AggregateType, &item.AggregateID, &item.ActorID, &item.CorrelationID, &item.OrganizationID, &item.LocationID, &item.ModuleKey, &item.OccurredAt, &payload); err != nil {
			continue
		}
		_ = json.Unmarshal(payload, &item.Payload)
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].OccurredAt.Before(items[j].OccurredAt) })
	return items
}

func (r *PostgresRepository) ListOutbox() []OutboxRecord {
	const query = `SELECT outbox_id, event_id, event_type, status, attempt_count, COALESCE(last_error,''), created_at, dispatched_at FROM outbox_records`
	rows, err := r.db.QueryContext(context.Background(), query)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]OutboxRecord, 0)
	for rows.Next() {
		var item OutboxRecord
		var dispatchedAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.EventID, &item.EventType, &item.Status, &item.AttemptCount, &item.LastError, &item.CreatedAt, &dispatchedAt); err != nil {
			continue
		}
		if dispatchedAt.Valid {
			item.DispatchedAt = dispatchedAt.Time
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.Before(items[j].CreatedAt) })
	return items
}

func (r *PostgresRepository) ListDeliveries() []OutboxDeliveryRecord {
	const query = `SELECT delivery_id, outbox_id, event_id, event_type, sink_name, status, attempt_count, COALESCE(last_error,''), created_at, dispatched_at FROM outbox_deliveries`
	rows, err := r.db.QueryContext(context.Background(), query)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]OutboxDeliveryRecord, 0)
	for rows.Next() {
		var item OutboxDeliveryRecord
		var dispatchedAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.OutboxID, &item.EventID, &item.EventType, &item.SinkName, &item.Status, &item.AttemptCount, &item.LastError, &item.CreatedAt, &dispatchedAt); err != nil {
			continue
		}
		if dispatchedAt.Valid {
			item.DispatchedAt = dispatchedAt.Time
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.Before(items[j].CreatedAt) })
	return items
}

func (r *PostgresRepository) ListDeliveriesByOutbox(outboxID string) []OutboxDeliveryRecord {
	const query = `SELECT delivery_id, outbox_id, event_id, event_type, sink_name, status, attempt_count, COALESCE(last_error,''), created_at, dispatched_at FROM outbox_deliveries WHERE outbox_id = $1`
	rows, err := r.db.QueryContext(context.Background(), query, outboxID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]OutboxDeliveryRecord, 0)
	for rows.Next() {
		var item OutboxDeliveryRecord
		var dispatchedAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.OutboxID, &item.EventID, &item.EventType, &item.SinkName, &item.Status, &item.AttemptCount, &item.LastError, &item.CreatedAt, &dispatchedAt); err != nil {
			continue
		}
		if dispatchedAt.Valid {
			item.DispatchedAt = dispatchedAt.Time
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.Before(items[j].CreatedAt) })
	return items
}

func (r *PostgresRepository) ListDeadLetters() []DeadLetterRecord {
	const query = `SELECT dead_letter_id, outbox_id, event_id, event_type, COALESCE(sink_name,''), reason, attempt_count, created_at FROM dead_letter_records`
	rows, err := r.db.QueryContext(context.Background(), query)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]DeadLetterRecord, 0)
	for rows.Next() {
		var item DeadLetterRecord
		if err := rows.Scan(&item.ID, &item.OutboxID, &item.EventID, &item.EventType, &item.SinkName, &item.Reason, &item.AttemptCount, &item.CreatedAt); err != nil {
			continue
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.Before(items[j].CreatedAt) })
	return items
}

func (r *PostgresRepository) ClaimPendingDeliveries(limit int) []OutboxDeliveryRecord {
	if limit <= 0 {
		limit = 100
	}
	const query = `
		UPDATE outbox_deliveries
		SET status = 'processing', attempt_count = attempt_count + 1
		WHERE delivery_id IN (
			SELECT delivery_id FROM outbox_deliveries
			WHERE status = 'pending'
			ORDER BY created_at ASC
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING delivery_id, outbox_id, event_id, event_type, sink_name, status, attempt_count, COALESCE(last_error,''), created_at, dispatched_at`
	rows, err := r.db.QueryContext(context.Background(), query, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]OutboxDeliveryRecord, 0)
	for rows.Next() {
		var item OutboxDeliveryRecord
		var dispatchedAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.OutboxID, &item.EventID, &item.EventType, &item.SinkName, &item.Status, &item.AttemptCount, &item.LastError, &item.CreatedAt, &dispatchedAt); err != nil {
			continue
		}
		if dispatchedAt.Valid {
			item.DispatchedAt = dispatchedAt.Time
		}
		items = append(items, item)
	}
	return items
}

func (r *PostgresRepository) ClaimPending(limit int) []OutboxRecord {
	deliveries := r.ClaimPendingDeliveries(limit)
	seen := map[string]struct{}{}
	items := make([]OutboxRecord, 0, len(deliveries))
	all := r.ListOutbox()
	for _, delivery := range deliveries {
		if _, ok := seen[delivery.OutboxID]; ok {
			continue
		}
		seen[delivery.OutboxID] = struct{}{}
		for _, outbox := range all {
			if outbox.ID == delivery.OutboxID {
				items = append(items, outbox)
				break
			}
		}
	}
	return items
}

func (r *PostgresRepository) MarkDispatched(outboxID string, update OutboxRecord) error {
	const query = `UPDATE outbox_records SET status = $1, dispatched_at = $2, last_error = NULL WHERE outbox_id = $3`
	_, err := r.db.ExecContext(context.Background(), query, update.Status, nullableTime(update.DispatchedAt), outboxID)
	return err
}

func (r *PostgresRepository) MarkFailed(outboxID string, update OutboxRecord) error {
	const query = `UPDATE outbox_records SET status = $1, last_error = NULLIF($2,''), attempt_count = $3 WHERE outbox_id = $4`
	_, err := r.db.ExecContext(context.Background(), query, update.Status, update.LastError, update.AttemptCount, outboxID)
	return err
}

func (r *PostgresRepository) SaveDeadLetter(record DeadLetterRecord) error {
	const query = `INSERT INTO dead_letter_records (dead_letter_id, outbox_id, event_id, event_type, sink_name, reason, attempt_count, created_at) VALUES ($1,$2,$3,$4,NULLIF($5,''),$6,$7,$8)`
	_, err := r.db.ExecContext(context.Background(), query, record.ID, record.OutboxID, record.EventID, record.EventType, record.SinkName, record.Reason, record.AttemptCount, record.CreatedAt)
	return err
}

func (r *PostgresRepository) MarkDeliveryDispatched(deliveryID string, update OutboxDeliveryRecord) error {
	const query = `UPDATE outbox_deliveries SET status = $1, dispatched_at = $2, last_error = NULL WHERE delivery_id = $3`
	_, err := r.db.ExecContext(context.Background(), query, update.Status, nullableTime(update.DispatchedAt), deliveryID)
	return err
}

func (r *PostgresRepository) MarkDeliveryFailed(deliveryID string, update OutboxDeliveryRecord) error {
	const query = `UPDATE outbox_deliveries SET status = $1, last_error = NULLIF($2,''), attempt_count = $3 WHERE delivery_id = $4`
	_, err := r.db.ExecContext(context.Background(), query, update.Status, update.LastError, update.AttemptCount, deliveryID)
	return err
}

func nullableTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}
