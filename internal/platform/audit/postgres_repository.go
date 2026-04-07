package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"orbyte/internal/platform/store"
)

type PostgresRepository struct {
	db store.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return NewPostgresRepositoryWithDB(store.UninstrumentedDB(db))
}

func NewPostgresRepositoryWithDB(db store.DB) *PostgresRepository {
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
		` + auditPostgresWhereClause + `
		ORDER BY occurred_at ASC`
	rows, err := r.db.QueryContext(context.Background(), query, auditPostgresArgs(filter)...)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := scanAuditEventRows(rows)
	sort.Slice(items, func(i, j int) bool {
		return items[i].OccurredAt.Before(items[j].OccurredAt)
	})
	return items
}

func (r *PostgresRepository) Search(filter Query) SearchResult {
	page := filter.Page
	if page <= 0 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	orderExpr := auditPostgresOrderExpr(filter.Sort)
	direction := "ASC"
	if strings.EqualFold(filter.Direction, "desc") {
		direction = "DESC"
	}
	args := auditPostgresArgs(filter)
	var total int
	countQuery := `SELECT COUNT(*) FROM audit_events ` + auditPostgresWhereClause
	if err := r.db.QueryRowContext(context.Background(), countQuery, args...).Scan(&total); err != nil {
		return SearchResult{Items: []Event{}, Total: 0, Facets: map[string]any{}, Summary: map[string]any{"count": 0}}
	}
	pagedArgs := append(append([]any(nil), args...), pageSize, offset)
	query := fmt.Sprintf(`
		SELECT audit_event_id, action, target_type, target_id, actor_id,
			COALESCE(actor_kind,''), COALESCE(on_behalf_of_user_id,''), COALESCE(delegation_grant_id,''), COALESCE(from_state,''), COALESCE(to_state,''),
			COALESCE(organization_id,''), COALESCE(location_id,''), COALESCE(operating_unit_id,''),
			COALESCE(request_id,''), occurred_at,
			COALESCE(change_summary_json, '{}'::jsonb), COALESCE(metadata_json, '{}'::jsonb), COALESCE(correlation_id,'')
		FROM audit_events
		%s
		ORDER BY %s %s
		LIMIT $20 OFFSET $21`, auditPostgresWhereClause, orderExpr, direction)
	rows, err := r.db.QueryContext(context.Background(), query, pagedArgs...)
	if err != nil {
		return SearchResult{Items: []Event{}, Total: total, Facets: map[string]any{}, Summary: map[string]any{"count": total}}
	}
	defer rows.Close()
	paged := scanAuditEventRows(rows)
	return SearchResult{
		Items:   paged,
		Total:   total,
		Facets:  r.auditPostgresFacets(filter),
		Summary: map[string]any{"count": total},
	}
}

func nullableTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}

const auditPostgresWhereClause = `
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
		  AND ($13 = '' OR request_id = $13)
		  AND ($14 = '' OR delegation_grant_id = $14)
		  AND ($15 = '' OR from_state = $15)
		  AND ($16 = '' OR to_state = $16)
		  AND ($17 = '' OR metadata_json ? $17)
		  AND ($18 = '' OR metadata_json ->> $17 = $18)
		  AND ($19 = '' OR audit_event_id ILIKE '%' || $19 || '%' OR action ILIKE '%' || $19 || '%' OR target_type ILIKE '%' || $19 || '%' OR target_id ILIKE '%' || $19 || '%' OR actor_id ILIKE '%' || $19 || '%' OR actor_kind ILIKE '%' || $19 || '%' OR request_id ILIKE '%' || $19 || '%' OR correlation_id ILIKE '%' || $19 || '%' OR metadata_json::text ILIKE '%' || $19 || '%' OR change_summary_json::text ILIKE '%' || $19 || '%')`

func auditPostgresArgs(filter Query) []any {
	return []any{
		filter.TargetType, filter.TargetID, filter.ActorID, filter.ActorKind, filter.OnBehalfOfUserID, filter.Action,
		filter.CorrelationID, filter.OrganizationID, filter.LocationID, filter.OperatingUnitID,
		nullableTime(filter.OccurredFrom), nullableTime(filter.OccurredTo),
		filter.RequestID, filter.DelegationGrantID, filter.FromState, filter.ToState, filter.MetadataKey, filter.MetadataValue, filter.Text,
	}
}

func auditPostgresOrderExpr(sortKey string) string {
	switch sortKey {
	case "action":
		return "action"
	case "target_type":
		return "target_type"
	case "target_id":
		return "target_id"
	case "actor_id":
		return "actor_id"
	case "actor_kind":
		return "actor_kind"
	case "correlation_id":
		return "correlation_id"
	default:
		return "occurred_at"
	}
}

func scanAuditEventRows(rows interface {
	Next() bool
	Scan(dest ...any) error
}) []Event {
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
	return items
}

func (r *PostgresRepository) auditPostgresFacets(filter Query) map[string]any {
	return map[string]any{
		"actions":      r.auditPostgresFacetCounts(filter, "action"),
		"target_types": r.auditPostgresFacetCounts(filter, "target_type"),
		"actors":       r.auditPostgresFacetCounts(filter, "actor_id"),
	}
}

func (r *PostgresRepository) auditPostgresFacetCounts(filter Query, field string) map[string]int {
	switch field {
	case "action", "target_type", "actor_id":
	default:
		return map[string]int{}
	}
	query := fmt.Sprintf(`
		SELECT COALESCE(%s, '') AS value, COUNT(*)
		FROM audit_events
		%s
		  AND COALESCE(%s, '') <> ''
		GROUP BY %s
		ORDER BY COUNT(*) DESC, %s ASC
		LIMIT 50`, field, auditPostgresWhereClause, field, field, field)
	rows, err := r.db.QueryContext(context.Background(), query, auditPostgresArgs(filter)...)
	if err != nil {
		return map[string]int{}
	}
	defer rows.Close()
	counts := map[string]int{}
	for rows.Next() {
		var key string
		var count int
		if err := rows.Scan(&key, &count); err != nil {
			continue
		}
		counts[key] = count
	}
	return counts
}
