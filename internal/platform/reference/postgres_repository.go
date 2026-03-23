package reference

import (
	"database/sql"
	"encoding/json"
	"sort"
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

func (r *PostgresRepository) SaveType(def TypeDefinition) error {
	allowedScopes, _ := json.Marshal(def.AllowedScopes)
	_, err := r.db.Exec(`
		INSERT INTO reference_type_definitions (reference_type_key, display_name, owner_module_key, value_type, allowed_scopes_json)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (reference_type_key) DO UPDATE SET
			display_name = EXCLUDED.display_name,
			owner_module_key = EXCLUDED.owner_module_key,
			value_type = EXCLUDED.value_type,
			allowed_scopes_json = EXCLUDED.allowed_scopes_json
	`, def.Key, def.DisplayName, def.OwnerModuleKey, def.ValueType, allowedScopes)
	return err
}

func (r *PostgresRepository) GetType(key string) (TypeDefinition, bool) {
	row := r.db.QueryRow(`SELECT display_name, COALESCE(owner_module_key,''), COALESCE(value_type,''), allowed_scopes_json FROM reference_type_definitions WHERE reference_type_key = $1`, key)
	var def TypeDefinition
	var allowedScopes []byte
	if err := row.Scan(&def.DisplayName, &def.OwnerModuleKey, &def.ValueType, &allowedScopes); err != nil {
		return TypeDefinition{}, false
	}
	def.Key = key
	_ = json.Unmarshal(allowedScopes, &def.AllowedScopes)
	return def, true
}

func (r *PostgresRepository) ListTypes() []TypeDefinition {
	rows, err := r.db.Query(`SELECT reference_type_key, display_name, COALESCE(owner_module_key,''), COALESCE(value_type,''), allowed_scopes_json FROM reference_type_definitions`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var items []TypeDefinition
	for rows.Next() {
		var def TypeDefinition
		var allowedScopes []byte
		if err := rows.Scan(&def.Key, &def.DisplayName, &def.OwnerModuleKey, &def.ValueType, &allowedScopes); err != nil {
			continue
		}
		_ = json.Unmarshal(allowedScopes, &def.AllowedScopes)
		items = append(items, def)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Key < items[j].Key })
	return items
}

func (r *PostgresRepository) SaveRecord(record Record) error {
	valueJSON, _ := json.Marshal(record.Value)
	externalJSON, _ := json.Marshal(record.ExternalCodes)
	_, err := r.db.Exec(`
		INSERT INTO reference_records (reference_type_key, reference_key, display_name, scope_type, scope_id, status, value_json, external_codes_json, effective_from, effective_to, updated_at, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NULLIF($9, TIMESTAMPTZ '0001-01-01 00:00:00Z'), NULLIF($10, TIMESTAMPTZ '0001-01-01 00:00:00Z'), $11, $12)
		ON CONFLICT (reference_type_key, reference_key, scope_type, scope_id) DO UPDATE SET
			display_name = EXCLUDED.display_name,
			status = EXCLUDED.status,
			value_json = EXCLUDED.value_json,
			external_codes_json = EXCLUDED.external_codes_json,
			effective_from = EXCLUDED.effective_from,
			effective_to = EXCLUDED.effective_to,
			updated_at = EXCLUDED.updated_at,
			updated_by = EXCLUDED.updated_by
	`, record.TypeKey, record.Key, record.DisplayName, normalizeScope(record.Scope), record.ScopeID, record.Status, valueJSON, externalJSON, zeroTime(record.EffectiveFrom), zeroTime(record.EffectiveTo), record.UpdatedAt, record.UpdatedBy)
	return err
}

func (r *PostgresRepository) ListRecords(typeKey string) []Record {
	rows, err := r.db.Query(`SELECT reference_key, display_name, scope_type, COALESCE(scope_id,''), status, value_json, external_codes_json, effective_from, effective_to, updated_at, COALESCE(updated_by,'') FROM reference_records WHERE reference_type_key = $1`, typeKey)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var items []Record
	for rows.Next() {
		var item Record
		var valueJSON, externalJSON []byte
		if err := rows.Scan(&item.Key, &item.DisplayName, &item.Scope, &item.ScopeID, &item.Status, &valueJSON, &externalJSON, &item.EffectiveFrom, &item.EffectiveTo, &item.UpdatedAt, &item.UpdatedBy); err != nil {
			continue
		}
		item.TypeKey = typeKey
		_ = json.Unmarshal(valueJSON, &item.Value)
		_ = json.Unmarshal(externalJSON, &item.ExternalCodes)
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Key == items[j].Key {
			return items[i].Scope < items[j].Scope
		}
		return items[i].Key < items[j].Key
	})
	return items
}

func zeroTime(value time.Time) time.Time {
	if value.IsZero() {
		return time.Time{}
	}
	return value
}
