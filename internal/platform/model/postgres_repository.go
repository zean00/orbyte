package model

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

func (r *PostgresRepository) SaveDefinition(def Definition) error {
	fields, _ := json.Marshal(def.Fields)
	relations, _ := json.Marshal(def.Relations)
	const query = `
		INSERT INTO model_definitions (
			model_key, display_name, owner_module_key, version_key, create_permission_key, list_permission_key, read_permission_key, update_permission_key, default_sort, fields_json, relations_json
		) VALUES ($1,$2,NULLIF($3,''),$4,NULLIF($5,''),NULLIF($6,''),NULLIF($7,''),NULLIF($8,''),NULLIF($9,''),$10,$11)
		ON CONFLICT (model_key) DO UPDATE SET
			display_name = EXCLUDED.display_name,
			owner_module_key = EXCLUDED.owner_module_key,
			version_key = EXCLUDED.version_key,
			create_permission_key = EXCLUDED.create_permission_key,
			list_permission_key = EXCLUDED.list_permission_key,
			read_permission_key = EXCLUDED.read_permission_key,
			update_permission_key = EXCLUDED.update_permission_key,
			default_sort = EXCLUDED.default_sort,
			fields_json = EXCLUDED.fields_json,
			relations_json = EXCLUDED.relations_json`
	_, err := r.db.ExecContext(context.Background(), query, def.Key, def.DisplayName, def.OwnerModuleKey, def.Version, def.CreatePermissionKey, def.ListPermissionKey, def.ReadPermissionKey, def.UpdatePermissionKey, def.DefaultSort, fields, relations)
	return err
}

func (r *PostgresRepository) ListDefinitions() []Definition {
	const query = `SELECT model_key, display_name, COALESCE(owner_module_key,''), version_key, COALESCE(create_permission_key,''), COALESCE(list_permission_key,''), COALESCE(read_permission_key,''), COALESCE(update_permission_key,''), COALESCE(default_sort,''), fields_json, relations_json FROM model_definitions ORDER BY model_key ASC`
	rows, err := r.db.QueryContext(context.Background(), query)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]Definition, 0)
	for rows.Next() {
		item, ok := scanDefinition(rows)
		if ok {
			items = append(items, item)
		}
	}
	return items
}

func (r *PostgresRepository) GetDefinition(key string) (Definition, bool) {
	const query = `SELECT model_key, display_name, COALESCE(owner_module_key,''), version_key, COALESCE(create_permission_key,''), COALESCE(list_permission_key,''), COALESCE(read_permission_key,''), COALESCE(update_permission_key,''), COALESCE(default_sort,''), fields_json, relations_json FROM model_definitions WHERE model_key = $1`
	row := r.db.QueryRowContext(context.Background(), query, key)
	var (
		item      Definition
		fields    []byte
		relations []byte
	)
	err := row.Scan(&item.Key, &item.DisplayName, &item.OwnerModuleKey, &item.Version, &item.CreatePermissionKey, &item.ListPermissionKey, &item.ReadPermissionKey, &item.UpdatePermissionKey, &item.DefaultSort, &fields, &relations)
	if err != nil {
		return Definition{}, false
	}
	_ = json.Unmarshal(fields, &item.Fields)
	_ = json.Unmarshal(relations, &item.Relations)
	return item, true
}

func (r *PostgresRepository) SaveRecord(record Record) error {
	values, _ := json.Marshal(record.Values)
	const query = `
		INSERT INTO model_records (
			model_key, record_id, version, values_json, created_by, created_at, updated_by, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (model_key, record_id) DO UPDATE SET
			version = EXCLUDED.version,
			values_json = EXCLUDED.values_json,
			updated_by = EXCLUDED.updated_by,
			updated_at = EXCLUDED.updated_at`
	_, err := r.db.ExecContext(context.Background(), query, record.ModelKey, record.ID, record.Version, values, record.CreatedBy, record.CreatedAt, record.UpdatedBy, record.UpdatedAt)
	return err
}

func (r *PostgresRepository) DeleteRecord(modelKey, id string) error {
	const query = `DELETE FROM model_records WHERE model_key = $1 AND record_id = $2`
	_, err := r.db.ExecContext(context.Background(), query, modelKey, id)
	return err
}

func (r *PostgresRepository) GetRecord(modelKey, id string) (Record, bool) {
	const query = `SELECT model_key, record_id, version, values_json, created_by, created_at, updated_by, updated_at FROM model_records WHERE model_key = $1 AND record_id = $2`
	var (
		item   Record
		values []byte
	)
	if err := r.db.QueryRowContext(context.Background(), query, modelKey, id).Scan(&item.ModelKey, &item.ID, &item.Version, &values, &item.CreatedBy, &item.CreatedAt, &item.UpdatedBy, &item.UpdatedAt); err != nil {
		return Record{}, false
	}
	_ = json.Unmarshal(values, &item.Values)
	return item, true
}

func (r *PostgresRepository) ListRecords(modelKey string) []Record {
	const query = `SELECT model_key, record_id, version, values_json, created_by, created_at, updated_by, updated_at FROM model_records WHERE model_key = $1`
	rows, err := r.db.QueryContext(context.Background(), query, modelKey)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]Record, 0)
	for rows.Next() {
		var (
			item   Record
			values []byte
		)
		if err := rows.Scan(&item.ModelKey, &item.ID, &item.Version, &values, &item.CreatedBy, &item.CreatedAt, &item.UpdatedBy, &item.UpdatedAt); err != nil {
			continue
		}
		_ = json.Unmarshal(values, &item.Values)
		items = append(items, item)
	}
	return items
}

func (r *PostgresRepository) QueryRecords(def Definition, query Query) ([]Record, int, error) {
	listQuery, countQuery, args := buildPostgresRecordQuery(def, query, true)
	countArgs := args[:len(args)-2]
	var total int
	if err := r.db.QueryRowContext(context.Background(), countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(context.Background(), listQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]Record, 0)
	for rows.Next() {
		var (
			item   Record
			values []byte
		)
		if err := rows.Scan(&item.ModelKey, &item.ID, &item.Version, &values, &item.CreatedBy, &item.CreatedAt, &item.UpdatedBy, &item.UpdatedAt); err != nil {
			return nil, 0, err
		}
		_ = json.Unmarshal(values, &item.Values)
		items = append(items, item)
	}
	return items, total, nil
}

func scanDefinition(scanner interface{ Scan(dest ...any) error }) (Definition, bool) {
	var (
		item      Definition
		fields    []byte
		relations []byte
	)
	if err := scanner.Scan(&item.Key, &item.DisplayName, &item.OwnerModuleKey, &item.Version, &item.CreatePermissionKey, &item.ListPermissionKey, &item.ReadPermissionKey, &item.UpdatePermissionKey, &item.DefaultSort, &fields, &relations); err != nil {
		return Definition{}, false
	}
	_ = json.Unmarshal(fields, &item.Fields)
	_ = json.Unmarshal(relations, &item.Relations)
	return item, true
}
