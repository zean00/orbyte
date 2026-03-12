package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"sort"
)

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) Save(event Event) error {
	metadata, err := json.Marshal(event.Metadata)
	if err != nil {
		return err
	}
	const query = `
		INSERT INTO audit_events (
			audit_event_id, action, target_type, target_id, actor_id,
			from_state, to_state, occurred_at, metadata_json, correlation_id
		) VALUES ($1,$2,$3,$4,$5,NULLIF($6,''),NULLIF($7,''),$8,$9,NULLIF($10,''))`
	_, err = r.db.ExecContext(context.Background(), query,
		event.ID,
		event.Action,
		event.TargetType,
		event.TargetID,
		event.ActorID,
		event.FromState,
		event.ToState,
		event.OccurredAt,
		metadata,
		event.CorrelationID,
	)
	return err
}

func (r *PostgresRepository) List() []Event {
	const query = `
		SELECT audit_event_id, action, target_type, target_id, actor_id,
			COALESCE(from_state,''), COALESCE(to_state,''), occurred_at,
			COALESCE(metadata_json, '{}'::jsonb), COALESCE(correlation_id,'')
		FROM audit_events`
	rows, err := r.db.QueryContext(context.Background(), query)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]Event, 0)
	for rows.Next() {
		var item Event
		var metadata []byte
		if err := rows.Scan(&item.ID, &item.Action, &item.TargetType, &item.TargetID, &item.ActorID, &item.FromState, &item.ToState, &item.OccurredAt, &metadata, &item.CorrelationID); err != nil {
			continue
		}
		_ = json.Unmarshal(metadata, &item.Metadata)
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].OccurredAt.Before(items[j].OccurredAt)
	})
	return items
}
