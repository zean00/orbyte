package notification

import (
	"context"
	"database/sql"
	"encoding/json"

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

func (r *PostgresRepository) Save(item Item) error {
	const query = `
		INSERT INTO notification_items (
			notification_id, user_id, category, channel, status, title, body,
			target_type, target_id, deep_link_path, action_link_path, metadata_json,
			created_at, read_at, dismissed_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11, $12::jsonb,
			$13, $14, $15
		)
		ON CONFLICT (notification_id) DO UPDATE SET
			user_id = EXCLUDED.user_id,
			category = EXCLUDED.category,
			channel = EXCLUDED.channel,
			status = EXCLUDED.status,
			title = EXCLUDED.title,
			body = EXCLUDED.body,
			target_type = EXCLUDED.target_type,
			target_id = EXCLUDED.target_id,
			deep_link_path = EXCLUDED.deep_link_path,
			action_link_path = EXCLUDED.action_link_path,
			metadata_json = EXCLUDED.metadata_json,
			created_at = EXCLUDED.created_at,
			read_at = EXCLUDED.read_at,
			dismissed_at = EXCLUDED.dismissed_at`
	metadataJSON, _ := json.Marshal(cloneMap(item.Metadata))
	var readAt any
	if !item.ReadAt.IsZero() {
		readAt = item.ReadAt
	}
	var dismissedAt any
	if !item.DismissedAt.IsZero() {
		dismissedAt = item.DismissedAt
	}
	_, err := r.db.ExecContext(context.Background(), query,
		item.ID,
		item.UserID,
		item.Category,
		item.Channel,
		item.Status,
		item.Title,
		item.Body,
		item.TargetType,
		item.TargetID,
		item.DeepLinkPath,
		item.ActionLinkPath,
		string(metadataJSON),
		item.CreatedAt,
		readAt,
		dismissedAt,
	)
	return err
}

func (r *PostgresRepository) Find(id string) (Item, bool) {
	const query = `
		SELECT notification_id, user_id, COALESCE(category, ''), COALESCE(channel, ''), status, title, COALESCE(body, ''),
		       COALESCE(target_type, ''), COALESCE(target_id, ''), COALESCE(deep_link_path, ''), COALESCE(action_link_path, ''),
		       COALESCE(metadata_json, '{}'::jsonb), created_at, read_at, dismissed_at
		FROM notification_items
		WHERE notification_id = $1`
	item, err := r.scanRow(r.db.QueryRowContext(context.Background(), query, id))
	if err != nil {
		return Item{}, false
	}
	return item, true
}

func (r *PostgresRepository) List(filter Filter) []Item {
	query := `
		SELECT notification_id, user_id, COALESCE(category, ''), COALESCE(channel, ''), status, title, COALESCE(body, ''),
		       COALESCE(target_type, ''), COALESCE(target_id, ''), COALESCE(deep_link_path, ''), COALESCE(action_link_path, ''),
		       COALESCE(metadata_json, '{}'::jsonb), created_at, read_at, dismissed_at
		FROM notification_items
		WHERE 1=1`
	args := []any{}
	if filter.UserID != "" {
		args = append(args, filter.UserID)
		query += " AND user_id = $" + itoa(len(args))
	}
	if filter.Category != "" {
		args = append(args, filter.Category)
		query += " AND category = $" + itoa(len(args))
	}
	if filter.Status != "" {
		args = append(args, filter.Status)
		query += " AND status = $" + itoa(len(args))
	}
	if filter.TargetType != "" {
		args = append(args, filter.TargetType)
		query += " AND target_type = $" + itoa(len(args))
	}
	if filter.TargetID != "" {
		args = append(args, filter.TargetID)
		query += " AND target_id = $" + itoa(len(args))
	}
	query += " ORDER BY CASE WHEN status = 'unread' THEN 0 ELSE 1 END, created_at DESC"
	rows, err := r.db.QueryContext(context.Background(), query, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]Item, 0)
	for rows.Next() {
		item, err := r.scanRows(rows)
		if err != nil {
			continue
		}
		items = append(items, item)
	}
	return items
}

func (r *PostgresRepository) scanRow(row store.RowScanner) (Item, error) {
	var (
		item         Item
		metadataJSON []byte
		readAt       sql.NullTime
		dismissedAt  sql.NullTime
	)
	if err := row.Scan(
		&item.ID,
		&item.UserID,
		&item.Category,
		&item.Channel,
		&item.Status,
		&item.Title,
		&item.Body,
		&item.TargetType,
		&item.TargetID,
		&item.DeepLinkPath,
		&item.ActionLinkPath,
		&metadataJSON,
		&item.CreatedAt,
		&readAt,
		&dismissedAt,
	); err != nil {
		return Item{}, err
	}
	if readAt.Valid {
		item.ReadAt = readAt.Time
	}
	if dismissedAt.Valid {
		item.DismissedAt = dismissedAt.Time
	}
	_ = json.Unmarshal(metadataJSON, &item.Metadata)
	if item.Metadata == nil {
		item.Metadata = map[string]any{}
	}
	return item, nil
}

func (r *PostgresRepository) scanRows(rows *sql.Rows) (Item, error) {
	return r.scanRow(rows)
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	buf := [20]byte{}
	index := len(buf)
	for value > 0 {
		index--
		buf[index] = byte('0' + value%10)
		value /= 10
	}
	return string(buf[index:])
}
