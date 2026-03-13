package workflow

import (
	"context"
	"database/sql"
	"encoding/json"
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
	states, err := json.Marshal(def.States)
	if err != nil {
		return shared.Validation("invalid workflow states")
	}
	actions, err := json.Marshal(def.Actions)
	if err != nil {
		return shared.Validation("invalid workflow actions")
	}
	const query = `
		INSERT INTO workflow_definitions (
			workflow_key, states_json, actions_json
		) VALUES ($1, $2, $3)`
	_, err = r.db.ExecContext(context.Background(), query, def.Key, states, actions)
	if err != nil {
		return shared.Conflict("workflow definition already exists")
	}
	return nil
}

func (r *PostgresRepository) GetDefinition(key string) (Definition, bool) {
	const query = `
		SELECT workflow_key, states_json, actions_json
		FROM workflow_definitions
		WHERE workflow_key = $1`
	var (
		def         Definition
		statesJSON  []byte
		actionsJSON []byte
	)
	err := r.db.QueryRowContext(context.Background(), query, key).Scan(&def.Key, &statesJSON, &actionsJSON)
	if err != nil {
		return Definition{}, false
	}
	if err := json.Unmarshal(statesJSON, &def.States); err != nil {
		return Definition{}, false
	}
	if err := json.Unmarshal(actionsJSON, &def.Actions); err != nil {
		return Definition{}, false
	}
	return def, true
}

func (r *PostgresRepository) ListDefinitions() []Definition {
	const query = `SELECT workflow_key, states_json, actions_json FROM workflow_definitions`
	rows, err := r.db.QueryContext(context.Background(), query)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]Definition, 0)
	for rows.Next() {
		var (
			def         Definition
			statesJSON  []byte
			actionsJSON []byte
		)
		if err := rows.Scan(&def.Key, &statesJSON, &actionsJSON); err != nil {
			continue
		}
		if err := json.Unmarshal(statesJSON, &def.States); err != nil {
			continue
		}
		if err := json.Unmarshal(actionsJSON, &def.Actions); err != nil {
			continue
		}
		items = append(items, def)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Key < items[j].Key })
	return items
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
	const query = `
		INSERT INTO workflow_tasks (
			task_id, workflow_key, target_type, target_id, task_type, status, assignment_mode,
			assignee_user_id, assignee_role_key, candidate_role_keys_json, created_by, created_at,
			due_at, escalate_at, metadata_json
		) VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7,''), NULLIF($8,''), NULLIF($9,''), $10, NULLIF($11,''), $12, $13, $14, $15)`
	_, err = r.db.ExecContext(context.Background(), query, task.ID, task.WorkflowKey, task.TargetType, task.TargetID, task.TaskType, task.Status, task.AssignmentMode, task.AssigneeUserID, task.AssigneeRoleKey, candidateRoleKeys, task.CreatedBy, task.CreatedAt, nullableTime(task.DueAt), nullableTime(task.EscalateAt), metadata)
	return err
}

func (r *PostgresRepository) ListTasks() []Task {
	const query = `SELECT task_id, workflow_key, target_type, target_id, task_type, status, COALESCE(assignment_mode,''), COALESCE(assignee_user_id,''), COALESCE(assignee_role_key,''), COALESCE(candidate_role_keys_json,'[]'::jsonb), COALESCE(created_by,''), created_at, due_at, escalate_at, COALESCE(metadata_json,'{}'::jsonb) FROM workflow_tasks`
	rows, err := r.db.QueryContext(context.Background(), query)
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
		if err := rows.Scan(&item.ID, &item.WorkflowKey, &item.TargetType, &item.TargetID, &item.TaskType, &item.Status, &item.AssignmentMode, &item.AssigneeUserID, &item.AssigneeRoleKey, &candidateRoleKeys, &item.CreatedBy, &item.CreatedAt, &dueAt, &escalateAt, &metadata); err != nil {
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
	const query = `UPDATE workflow_tasks SET status = $1, metadata_json = COALESCE(metadata_json,'{}'::jsonb) || jsonb_build_object('resolved_by', NULLIF($2,''), 'resolved_at', $3) WHERE task_id = $4`
	_, err := r.db.ExecContext(context.Background(), query, update.Status, update.ResolvedBy, nullableTime(update.ResolvedAt), update.ID)
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
	const query = `
		INSERT INTO workflow_approvals (
			approval_id, workflow_key, target_type, target_id, status, stage_key, requested_by, requested_at,
			resolved_by, resolved_at, candidate_role_keys_json, due_at, metadata_json
		) VALUES ($1, $2, $3, $4, $5, NULLIF($6,''), NULLIF($7,''), $8, NULLIF($9,''), $10, $11, $12, $13)`
	_, err = r.db.ExecContext(context.Background(), query, approval.ID, approval.WorkflowKey, approval.TargetType, approval.TargetID, approval.Status, approval.StageKey, approval.RequestedBy, approval.RequestedAt, approval.ResolvedBy, nullableTime(approval.ResolvedAt), candidateRoleKeys, nullableTime(approval.DueAt), metadata)
	return err
}

func (r *PostgresRepository) ListApprovals() []Approval {
	const query = `SELECT approval_id, workflow_key, target_type, target_id, status, COALESCE(stage_key,''), COALESCE(requested_by,''), requested_at, COALESCE(resolved_by,''), resolved_at, COALESCE(candidate_role_keys_json,'[]'::jsonb), due_at, COALESCE(metadata_json,'{}'::jsonb) FROM workflow_approvals`
	rows, err := r.db.QueryContext(context.Background(), query)
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
		if err := rows.Scan(&item.ID, &item.WorkflowKey, &item.TargetType, &item.TargetID, &item.Status, &item.StageKey, &item.RequestedBy, &item.RequestedAt, &item.ResolvedBy, &resolvedAt, &candidateRoleKeys, &dueAt, &metadata); err != nil {
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
	const query = `UPDATE workflow_approvals SET status = $1, resolved_by = NULLIF($2,''), resolved_at = $3 WHERE approval_id = $4`
	_, err := r.db.ExecContext(context.Background(), query, update.Status, update.ResolvedBy, nullableTime(update.ResolvedAt), update.ID)
	return err
}

func nullableTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}
