package workflow

import (
	"context"
	"database/sql"
	"encoding/json"
	"sort"

	"clinic/internal/platform/shared"
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
	const query = `
		INSERT INTO workflow_tasks (
			task_id, workflow_key, target_type, target_id, task_type, status, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)`
	_, err := r.db.ExecContext(context.Background(), query, task.ID, task.WorkflowKey, task.TargetType, task.TargetID, task.TaskType, task.Status, task.CreatedAt)
	return err
}

func (r *PostgresRepository) ListTasks() []Task {
	const query = `SELECT task_id, workflow_key, target_type, target_id, task_type, status, created_at FROM workflow_tasks`
	rows, err := r.db.QueryContext(context.Background(), query)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]Task, 0)
	for rows.Next() {
		var item Task
		if err := rows.Scan(&item.ID, &item.WorkflowKey, &item.TargetType, &item.TargetID, &item.TaskType, &item.Status, &item.CreatedAt); err != nil {
			continue
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.Before(items[j].CreatedAt) })
	return items
}

func (r *PostgresRepository) UpdateTaskStatus(taskID, status string) error {
	const query = `UPDATE workflow_tasks SET status = $1 WHERE task_id = $2`
	_, err := r.db.ExecContext(context.Background(), query, status, taskID)
	return err
}

func (r *PostgresRepository) SaveApproval(approval Approval) error {
	const query = `
		INSERT INTO workflow_approvals (
			approval_id, workflow_key, target_type, target_id, status, requested_at
		) VALUES ($1, $2, $3, $4, $5, $6)`
	_, err := r.db.ExecContext(context.Background(), query, approval.ID, approval.WorkflowKey, approval.TargetType, approval.TargetID, approval.Status, approval.RequestedAt)
	return err
}

func (r *PostgresRepository) ListApprovals() []Approval {
	const query = `SELECT approval_id, workflow_key, target_type, target_id, status, requested_at FROM workflow_approvals`
	rows, err := r.db.QueryContext(context.Background(), query)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]Approval, 0)
	for rows.Next() {
		var item Approval
		if err := rows.Scan(&item.ID, &item.WorkflowKey, &item.TargetType, &item.TargetID, &item.Status, &item.RequestedAt); err != nil {
			continue
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].RequestedAt.Before(items[j].RequestedAt) })
	return items
}

func (r *PostgresRepository) UpdateApprovalStatus(approvalID, status string) error {
	const query = `UPDATE workflow_approvals SET status = $1 WHERE approval_id = $2`
	_, err := r.db.ExecContext(context.Background(), query, status, approvalID)
	return err
}
