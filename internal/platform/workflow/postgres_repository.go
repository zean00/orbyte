package workflow

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"orbyte/internal/platform/shared"
)

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) SaveDefinition(def Definition) error {
	now := time.Now().UTC()
	def.Version = 1
	def.Status = "published"
	if def.CreatedAt.IsZero() {
		def.CreatedAt = now
	}
	if def.UpdatedAt.IsZero() {
		def.UpdatedAt = now
	}
	if def.PublishedAt.IsZero() {
		def.PublishedAt = now
	}
	states, actions, err := marshalDefinition(def)
	if err != nil {
		return err
	}
	tx, err := r.db.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(context.Background(), `
		INSERT INTO workflow_definition_versions (
			workflow_key, version_no, status, states_json, actions_json, created_at, updated_at, updated_by, published_at, published_by
		) VALUES ($1,$2,$3,$4,$5,$6,$7,NULLIF($8,''),$9,NULLIF($10,''))`,
		def.Key, def.Version, def.Status, states, actions, def.CreatedAt, def.UpdatedAt, def.UpdatedBy, nullableTime(def.PublishedAt), def.PublishedBy,
	); err != nil {
		return shared.Conflict("workflow definition already exists")
	}
	if _, err := tx.ExecContext(context.Background(), `
		INSERT INTO workflow_definitions (workflow_key, version_no, states_json, actions_json, updated_at)
		VALUES ($1,$2,$3,$4,$5)`,
		def.Key, def.Version, states, actions, def.UpdatedAt,
	); err != nil {
		return shared.Conflict("workflow definition already exists")
	}
	return tx.Commit()
}

func (r *PostgresRepository) GetDefinition(key string) (Definition, bool) {
	row := r.db.QueryRowContext(context.Background(), `
		SELECT workflow_key, version_no, states_json, actions_json, updated_at
		FROM workflow_definitions
		WHERE workflow_key = $1`, key)
	def, err := scanPublishedDefinition(row)
	if err != nil {
		return Definition{}, false
	}
	return def, true
}

func (r *PostgresRepository) GetDefinitionVersion(key string, version int) (Definition, bool) {
	row := r.db.QueryRowContext(context.Background(), `
		SELECT workflow_key, version_no, status, states_json, actions_json, created_at, updated_at,
		       COALESCE(updated_by,''), published_at, COALESCE(published_by,'')
		FROM workflow_definition_versions
		WHERE workflow_key = $1 AND version_no = $2`, key, version)
	def, err := scanVersionedDefinition(row)
	if err != nil {
		return Definition{}, false
	}
	return def, true
}

func (r *PostgresRepository) ListDefinitions() []Definition {
	rows, err := r.db.QueryContext(context.Background(), `
		SELECT workflow_key, version_no, states_json, actions_json, updated_at
		FROM workflow_definitions`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]Definition, 0)
	for rows.Next() {
		def, err := scanPublishedDefinition(rows)
		if err == nil {
			items = append(items, def)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Key < items[j].Key })
	return items
}

func (r *PostgresRepository) ListDefinitionVersions(key string) []Definition {
	rows, err := r.db.QueryContext(context.Background(), `
		SELECT workflow_key, version_no, status, states_json, actions_json, created_at, updated_at,
		       COALESCE(updated_by,''), published_at, COALESCE(published_by,'')
		FROM workflow_definition_versions
		WHERE workflow_key = $1
		ORDER BY version_no ASC`, key)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]Definition, 0)
	for rows.Next() {
		def, err := scanVersionedDefinition(rows)
		if err == nil {
			items = append(items, def)
		}
	}
	return items
}

func (r *PostgresRepository) CreateDraft(key, actorID string) (Definition, error) {
	published, ok := r.GetDefinition(key)
	if !ok {
		return Definition{}, shared.NotFound("workflow definition not found")
	}
	if draft, ok := r.currentDraft(key); ok {
		return Definition{}, shared.Conflict("workflow draft already exists: version " + stringFromInt(draft.Version))
	}
	now := time.Now().UTC()
	draft := published
	draft.Version++
	draft.Status = "draft"
	draft.CreatedAt = now
	draft.UpdatedAt = now
	draft.UpdatedBy = actorID
	draft.PublishedAt = time.Time{}
	draft.PublishedBy = ""
	states, actions, err := marshalDefinition(draft)
	if err != nil {
		return Definition{}, err
	}
	_, err = r.db.ExecContext(context.Background(), `
		INSERT INTO workflow_definition_versions (
			workflow_key, version_no, status, states_json, actions_json, created_at, updated_at, updated_by
		) VALUES ($1,$2,$3,$4,$5,$6,$7,NULLIF($8,''))`,
		draft.Key, draft.Version, draft.Status, states, actions, draft.CreatedAt, draft.UpdatedAt, draft.UpdatedBy,
	)
	if err != nil {
		return Definition{}, err
	}
	return draft, nil
}

func (r *PostgresRepository) SaveDraft(def Definition, actorID string) (Definition, error) {
	states, actions, err := marshalDefinition(def)
	if err != nil {
		return Definition{}, err
	}
	def.UpdatedAt = time.Now().UTC()
	def.UpdatedBy = actorID
	result, err := r.db.ExecContext(context.Background(), `
		UPDATE workflow_definition_versions
		SET states_json = $1, actions_json = $2, updated_at = $3, updated_by = NULLIF($4,'')
		WHERE workflow_key = $5 AND version_no = $6 AND status = 'draft'`,
		states, actions, def.UpdatedAt, def.UpdatedBy, def.Key, def.Version,
	)
	if err != nil {
		return Definition{}, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return Definition{}, err
	}
	if rows == 0 {
		return Definition{}, shared.NotFound("workflow draft not found")
	}
	updated, _ := r.GetDefinitionVersion(def.Key, def.Version)
	return updated, nil
}

func (r *PostgresRepository) PublishDefinition(key string, version int, actorID string) (Definition, error) {
	def, ok := r.GetDefinitionVersion(key, version)
	if !ok || def.Status != "draft" {
		return Definition{}, shared.NotFound("workflow draft not found")
	}
	states, actions, err := marshalDefinition(def)
	if err != nil {
		return Definition{}, err
	}
	now := time.Now().UTC()
	tx, err := r.db.BeginTx(context.Background(), nil)
	if err != nil {
		return Definition{}, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(context.Background(), `
		UPDATE workflow_definition_versions
		SET status = 'published', updated_at = $1, updated_by = NULLIF($2,''), published_at = $1, published_by = NULLIF($2,'')
		WHERE workflow_key = $3 AND version_no = $4`,
		now, actorID, key, version,
	); err != nil {
		return Definition{}, err
	}
	if _, err := tx.ExecContext(context.Background(), `
		INSERT INTO workflow_definitions (workflow_key, version_no, states_json, actions_json, updated_at)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (workflow_key) DO UPDATE SET
			version_no = EXCLUDED.version_no,
			states_json = EXCLUDED.states_json,
			actions_json = EXCLUDED.actions_json,
			updated_at = EXCLUDED.updated_at`,
		key, version, states, actions, now,
	); err != nil {
		return Definition{}, err
	}
	if err := tx.Commit(); err != nil {
		return Definition{}, err
	}
	published, _ := r.GetDefinitionVersion(key, version)
	return published, nil
}

func (r *PostgresRepository) SaveTask(task Task) error {
	candidateRoleKeys, err := json.Marshal(task.CandidateRoleKeys)
	if err != nil {
		return shared.Validation("invalid workflow task candidate roles")
	}
	metadata, err := json.Marshal(task.Metadata)
	if err != nil {
		return shared.Validation("invalid workflow task metadata")
	}
	_, err = r.db.ExecContext(context.Background(), `
		INSERT INTO workflow_tasks (
			task_id, workflow_key, workflow_version, target_type, target_id, task_type, status, assignment_mode,
			assignee_user_id, assignee_role_key, candidate_role_keys_json, created_by, created_at,
			due_at, escalate_at, metadata_json
		) VALUES ($1,$2,$3,$4,$5,$6,$7,NULLIF($8,''),NULLIF($9,''),NULLIF($10,''),$11,NULLIF($12,''),$13,$14,$15,$16)`,
		task.ID, task.WorkflowKey, task.WorkflowVersion, task.TargetType, task.TargetID, task.TaskType, task.Status, task.AssignmentMode, task.AssigneeUserID, task.AssigneeRoleKey, candidateRoleKeys, task.CreatedBy, task.CreatedAt, nullableTime(task.DueAt), nullableTime(task.EscalateAt), metadata,
	)
	return err
}

func (r *PostgresRepository) ListTasks() []Task {
	rows, err := r.db.QueryContext(context.Background(), `
		SELECT task_id, workflow_key, COALESCE(workflow_version,0), target_type, target_id, task_type, status,
		       COALESCE(assignment_mode,''), COALESCE(assignee_user_id,''), COALESCE(assignee_role_key,''),
		       COALESCE(candidate_role_keys_json,'[]'::jsonb), COALESCE(created_by,''), created_at, due_at, escalate_at,
		       COALESCE(metadata_json,'{}'::jsonb)
		FROM workflow_tasks`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]Task, 0)
	for rows.Next() {
		var item Task
		var candidateRoleKeys []byte
		var metadata []byte
		var dueAt sql.NullTime
		var escalateAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.WorkflowKey, &item.WorkflowVersion, &item.TargetType, &item.TargetID, &item.TaskType, &item.Status, &item.AssignmentMode, &item.AssigneeUserID, &item.AssigneeRoleKey, &candidateRoleKeys, &item.CreatedBy, &item.CreatedAt, &dueAt, &escalateAt, &metadata); err != nil {
			continue
		}
		_ = json.Unmarshal(candidateRoleKeys, &item.CandidateRoleKeys)
		_ = json.Unmarshal(metadata, &item.Metadata)
		if dueAt.Valid {
			item.DueAt = dueAt.Time
		}
		if escalateAt.Valid {
			item.EscalateAt = escalateAt.Time
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.Before(items[j].CreatedAt) })
	return items
}

func (r *PostgresRepository) UpdateTaskStatus(update TaskStatusUpdate) error {
	_, err := r.db.ExecContext(context.Background(), `
		UPDATE workflow_tasks
		SET status = $1,
		    metadata_json = COALESCE(metadata_json,'{}'::jsonb) || jsonb_build_object('resolved_by', NULLIF($2,''), 'resolved_at', $3)
		WHERE task_id = $4`,
		update.Status, update.ResolvedBy, nullableTime(update.ResolvedAt), update.ID,
	)
	return err
}

func (r *PostgresRepository) SaveApproval(approval Approval) error {
	candidateRoleKeys, err := json.Marshal(approval.CandidateRoleKeys)
	if err != nil {
		return shared.Validation("invalid workflow approval candidate roles")
	}
	metadata, err := json.Marshal(approval.Metadata)
	if err != nil {
		return shared.Validation("invalid workflow approval metadata")
	}
	_, err = r.db.ExecContext(context.Background(), `
		INSERT INTO workflow_approvals (
			approval_id, workflow_key, workflow_version, target_type, target_id, status, stage_key, requested_by, requested_at,
			resolved_by, resolved_at, candidate_role_keys_json, due_at, metadata_json
		) VALUES ($1,$2,$3,$4,$5,$6,NULLIF($7,''),NULLIF($8,''),$9,NULLIF($10,''),$11,$12,$13,$14)`,
		approval.ID, approval.WorkflowKey, approval.WorkflowVersion, approval.TargetType, approval.TargetID, approval.Status, approval.StageKey, approval.RequestedBy, approval.RequestedAt, approval.ResolvedBy, nullableTime(approval.ResolvedAt), candidateRoleKeys, nullableTime(approval.DueAt), metadata,
	)
	return err
}

func (r *PostgresRepository) ListApprovals() []Approval {
	rows, err := r.db.QueryContext(context.Background(), `
		SELECT approval_id, workflow_key, COALESCE(workflow_version,0), target_type, target_id, status,
		       COALESCE(stage_key,''), COALESCE(requested_by,''), requested_at, COALESCE(resolved_by,''), resolved_at,
		       COALESCE(candidate_role_keys_json,'[]'::jsonb), due_at, COALESCE(metadata_json,'{}'::jsonb)
		FROM workflow_approvals`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]Approval, 0)
	for rows.Next() {
		var item Approval
		var candidateRoleKeys []byte
		var metadata []byte
		var resolvedAt sql.NullTime
		var dueAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.WorkflowKey, &item.WorkflowVersion, &item.TargetType, &item.TargetID, &item.Status, &item.StageKey, &item.RequestedBy, &item.RequestedAt, &item.ResolvedBy, &resolvedAt, &candidateRoleKeys, &dueAt, &metadata); err != nil {
			continue
		}
		_ = json.Unmarshal(candidateRoleKeys, &item.CandidateRoleKeys)
		_ = json.Unmarshal(metadata, &item.Metadata)
		if resolvedAt.Valid {
			item.ResolvedAt = resolvedAt.Time
		}
		if dueAt.Valid {
			item.DueAt = dueAt.Time
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].RequestedAt.Before(items[j].RequestedAt) })
	return items
}

func (r *PostgresRepository) UpdateApprovalStatus(update ApprovalStatusUpdate) error {
	_, err := r.db.ExecContext(context.Background(), `
		UPDATE workflow_approvals
		SET status = $1, resolved_by = NULLIF($2,''), resolved_at = $3
		WHERE approval_id = $4`,
		update.Status, update.ResolvedBy, nullableTime(update.ResolvedAt), update.ID,
	)
	return err
}

func (r *PostgresRepository) SaveHistory(event HistoryEvent) error {
	assignment, err := json.Marshal(event.AssignmentSummary)
	if err != nil {
		return shared.Validation("invalid workflow history assignment summary")
	}
	metadata, err := json.Marshal(event.Metadata)
	if err != nil {
		return shared.Validation("invalid workflow history metadata")
	}
	_, err = r.db.ExecContext(context.Background(), `
		INSERT INTO workflow_history (
			history_id, workflow_key, workflow_version, target_type, target_id, action, from_state, to_state,
			actor_id, occurred_at, decision_code, decision_reason, assignment_summary_json, metadata_json
		) VALUES ($1,$2,$3,$4,$5,$6,NULLIF($7,''),NULLIF($8,''),NULLIF($9,''),$10,NULLIF($11,''),NULLIF($12,''),$13,$14)`,
		event.ID, event.WorkflowKey, event.WorkflowVersion, event.TargetType, event.TargetID, event.Action, event.FromState, event.ToState, event.ActorID, event.OccurredAt, event.DecisionCode, event.DecisionReason, assignment, metadata,
	)
	return err
}

func (r *PostgresRepository) ListHistory(targetType, targetID string) []HistoryEvent {
	rows, err := r.db.QueryContext(context.Background(), `
		SELECT history_id, workflow_key, COALESCE(workflow_version,0), target_type, target_id, action,
		       COALESCE(from_state,''), COALESCE(to_state,''), COALESCE(actor_id,''), occurred_at,
		       COALESCE(decision_code,''), COALESCE(decision_reason,''), COALESCE(assignment_summary_json,'{}'::jsonb), COALESCE(metadata_json,'{}'::jsonb)
		FROM workflow_history
		WHERE ($1 = '' OR target_type = $1) AND ($2 = '' OR target_id = $2)
		ORDER BY occurred_at ASC`, targetType, targetID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]HistoryEvent, 0)
	for rows.Next() {
		var item HistoryEvent
		var assignment []byte
		var metadata []byte
		if err := rows.Scan(&item.ID, &item.WorkflowKey, &item.WorkflowVersion, &item.TargetType, &item.TargetID, &item.Action, &item.FromState, &item.ToState, &item.ActorID, &item.OccurredAt, &item.DecisionCode, &item.DecisionReason, &assignment, &metadata); err != nil {
			continue
		}
		_ = json.Unmarshal(assignment, &item.AssignmentSummary)
		_ = json.Unmarshal(metadata, &item.Metadata)
		items = append(items, item)
	}
	return items
}

func (r *PostgresRepository) currentDraft(key string) (Definition, bool) {
	row := r.db.QueryRowContext(context.Background(), `
		SELECT workflow_key, version_no, status, states_json, actions_json, created_at, updated_at,
		       COALESCE(updated_by,''), published_at, COALESCE(published_by,'')
		FROM workflow_definition_versions
		WHERE workflow_key = $1 AND status = 'draft'`, key)
	def, err := scanVersionedDefinition(row)
	if err != nil {
		return Definition{}, false
	}
	return def, true
}

func marshalDefinition(def Definition) ([]byte, []byte, error) {
	states, err := json.Marshal(def.States)
	if err != nil {
		return nil, nil, shared.Validation("invalid workflow states")
	}
	actions, err := json.Marshal(def.Actions)
	if err != nil {
		return nil, nil, shared.Validation("invalid workflow actions")
	}
	return states, actions, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanPublishedDefinition(row scanner) (Definition, error) {
	var def Definition
	var statesJSON []byte
	var actionsJSON []byte
	if err := row.Scan(&def.Key, &def.Version, &statesJSON, &actionsJSON, &def.UpdatedAt); err != nil {
		return Definition{}, err
	}
	def.Status = "published"
	if err := json.Unmarshal(statesJSON, &def.States); err != nil {
		return Definition{}, err
	}
	if err := json.Unmarshal(actionsJSON, &def.Actions); err != nil {
		return Definition{}, err
	}
	return def, nil
}

func scanVersionedDefinition(row scanner) (Definition, error) {
	var def Definition
	var statesJSON []byte
	var actionsJSON []byte
	var publishedAt sql.NullTime
	if err := row.Scan(&def.Key, &def.Version, &def.Status, &statesJSON, &actionsJSON, &def.CreatedAt, &def.UpdatedAt, &def.UpdatedBy, &publishedAt, &def.PublishedBy); err != nil {
		return Definition{}, err
	}
	if publishedAt.Valid {
		def.PublishedAt = publishedAt.Time
	}
	if err := json.Unmarshal(statesJSON, &def.States); err != nil {
		return Definition{}, err
	}
	if err := json.Unmarshal(actionsJSON, &def.Actions); err != nil {
		return Definition{}, err
	}
	return def, nil
}

func stringFromInt(value int) string {
	return fmt.Sprintf("%d", value)
}

func nullableTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}
