package audit

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

func (r *PostgresRepository) Save(event Event) error {
	metadata, err := json.Marshal(event.Metadata)
	if err != nil {
		return err
	}
	changeSummary, err := json.Marshal(event.ChangeSummary)
	if err != nil {
		return err
	}
	const query = `
		INSERT INTO audit_events (
			audit_event_id, action, target_type, target_id, actor_id,
			actor_kind, on_behalf_of_user_id, delegation_grant_id, from_state, to_state, organization_id, location_id, operating_unit_id,
			request_id, occurred_at, change_summary_json, metadata_json, correlation_id
		) VALUES ($1,$2,$3,$4,$5,NULLIF($6,''),NULLIF($7,''),NULLIF($8,''),NULLIF($9,''),NULLIF($10,''),NULLIF($11,''),NULLIF($12,''),NULLIF($13,''),$14,$15,$16,NULLIF($17,''))`
	_, err = r.db.ExecContext(context.Background(), query,
		event.ID,
		event.Action,
		event.TargetType,
		event.TargetID,
		event.ActorID,
		event.ActorKind,
		event.OnBehalfOfUserID,
		event.DelegationGrantID,
		event.FromState,
		event.ToState,
		event.OrganizationID,
		event.LocationID,
		event.OperatingUnitID,
		event.RequestID,
		event.OccurredAt,
		changeSummary,
		metadata,
		event.CorrelationID,
	)
	return err
}

func (r *PostgresRepository) List() []Event {
	return r.Query(Query{})
}

func (r *PostgresRepository) Query(filter Query) []Event {
	const query = `
		SELECT audit_event_id, action, target_type, target_id, actor_id,
			COALESCE(actor_kind,''), COALESCE(on_behalf_of_user_id,''), COALESCE(delegation_grant_id,''), COALESCE(from_state,''), COALESCE(to_state,''),
			COALESCE(organization_id,''), COALESCE(location_id,''), COALESCE(operating_unit_id,''),
			COALESCE(request_id,''), occurred_at,
			COALESCE(change_summary_json, '{}'::jsonb), COALESCE(metadata_json, '{}'::jsonb), COALESCE(correlation_id,'')
		FROM audit_events
		WHERE ($1 = '' OR target_type = $1)
		  AND ($2 = '' OR target_id = $2)
		  AND ($3 = '' OR actor_id = $3)
		  AND ($4 = '' OR actor_kind = $4)
		  AND ($5 = '' OR on_behalf_of_user_id = $5)
		  AND ($6 = '' OR action = $6)
		  AND ($7 = '' OR correlation_id = $7)
		  AND ($8 = '' OR organization_id = $8)
		  AND ($9 = '' OR location_id = $9)
		  AND ($10 = '' OR operating_unit_id = $10)
		  AND ($11::timestamptz IS NULL OR occurred_at >= $11)
		  AND ($12::timestamptz IS NULL OR occurred_at <= $12)
		ORDER BY occurred_at ASC`
	rows, err := r.db.QueryContext(context.Background(), query,
		filter.TargetType, filter.TargetID, filter.ActorID, filter.ActorKind, filter.OnBehalfOfUserID, filter.Action,
		filter.CorrelationID, filter.OrganizationID, filter.LocationID, filter.OperatingUnitID,
		nullableTime(filter.OccurredFrom), nullableTime(filter.OccurredTo),
	)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]Event, 0)
	for rows.Next() {
		var item Event
		var metadata []byte
		var changeSummary []byte
		if err := rows.Scan(&item.ID, &item.Action, &item.TargetType, &item.TargetID, &item.ActorID, &item.ActorKind, &item.OnBehalfOfUserID, &item.DelegationGrantID, &item.FromState, &item.ToState, &item.OrganizationID, &item.LocationID, &item.OperatingUnitID, &item.RequestID, &item.OccurredAt, &changeSummary, &metadata, &item.CorrelationID); err != nil {
			continue
		}
		_ = json.Unmarshal(changeSummary, &item.ChangeSummary)
		_ = json.Unmarshal(metadata, &item.Metadata)
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].OccurredAt.Before(items[j].OccurredAt)
	})
	return items
}

func nullableTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}
