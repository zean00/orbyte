package engagement

import (
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

func (r *PostgresRepository) SaveProgram(program Program) error {
	_, err := r.db.Exec(`INSERT INTO engagement_programs (program_key, name, subject_type, status, published_version, created_at, updated_at, updated_by)
VALUES ($1,$2,NULLIF($3,''),$4,NULLIF($5,0),$6,$7,NULLIF($8,''))
ON CONFLICT (program_key) DO UPDATE SET name=EXCLUDED.name, subject_type=EXCLUDED.subject_type, status=EXCLUDED.status, published_version=EXCLUDED.published_version, created_at=EXCLUDED.created_at, updated_at=EXCLUDED.updated_at, updated_by=EXCLUDED.updated_by`,
		program.Key, program.Name, program.SubjectType, program.Status, program.PublishedVersion, program.CreatedAt, program.UpdatedAt, program.UpdatedBy)
	return err
}

func (r *PostgresRepository) GetProgram(key string) (Program, bool) {
	row := r.db.QueryRow(`SELECT program_key, name, COALESCE(subject_type,''), status, COALESCE(published_version,0), created_at, updated_at, COALESCE(updated_by,'')
FROM engagement_programs WHERE program_key = $1`, key)
	var item Program
	if err := row.Scan(&item.Key, &item.Name, &item.SubjectType, &item.Status, &item.PublishedVersion, &item.CreatedAt, &item.UpdatedAt, &item.UpdatedBy); err != nil {
		return Program{}, false
	}
	return item, true
}

func (r *PostgresRepository) ListPrograms() []Program {
	rows, err := r.db.Query(`SELECT program_key, name, COALESCE(subject_type,''), status, COALESCE(published_version,0), created_at, updated_at, COALESCE(updated_by,'')
FROM engagement_programs ORDER BY program_key ASC`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]Program, 0)
	for rows.Next() {
		var item Program
		_ = rows.Scan(&item.Key, &item.Name, &item.SubjectType, &item.Status, &item.PublishedVersion, &item.CreatedAt, &item.UpdatedAt, &item.UpdatedBy)
		items = append(items, item)
	}
	return items
}

func (r *PostgresRepository) SaveVersion(version ProgramVersion) error {
	rules, _ := json.Marshal(version.Rules)
	_, err := r.db.Exec(`INSERT INTO engagement_program_versions (program_key, version_no, status, change_note, rules_json, created_at, updated_at, published_at, published_by, last_error, last_replay_id)
VALUES ($1,$2,$3,NULLIF($4,''),$5,$6,$7,$8,NULLIF($9,''),NULLIF($10,''),NULLIF($11,''))
ON CONFLICT (program_key, version_no) DO UPDATE SET status=EXCLUDED.status, change_note=EXCLUDED.change_note, rules_json=EXCLUDED.rules_json, created_at=EXCLUDED.created_at, updated_at=EXCLUDED.updated_at, published_at=EXCLUDED.published_at, published_by=EXCLUDED.published_by, last_error=EXCLUDED.last_error, last_replay_id=EXCLUDED.last_replay_id`,
		version.ProgramKey, version.Version, version.Status, version.ChangeNote, rules, version.CreatedAt, version.UpdatedAt, nullTime(version.PublishedAt), version.PublishedBy, version.LastError, version.LastReplayID)
	return err
}

func (r *PostgresRepository) GetVersion(programKey string, version int) (ProgramVersion, bool) {
	row := r.db.QueryRow(`SELECT program_key, version_no, status, COALESCE(change_note,''), COALESCE(rules_json,'[]'::jsonb), created_at, updated_at, published_at, COALESCE(published_by,''), COALESCE(last_error,''), COALESCE(last_replay_id,'')
FROM engagement_program_versions WHERE program_key = $1 AND version_no = $2`, programKey, version)
	item, err := scanVersion(row)
	if err != nil {
		return ProgramVersion{}, false
	}
	return item, true
}

func (r *PostgresRepository) ListVersions(programKey string) []ProgramVersion {
	rows, err := r.db.Query(`SELECT program_key, version_no, status, COALESCE(change_note,''), COALESCE(rules_json,'[]'::jsonb), created_at, updated_at, published_at, COALESCE(published_by,''), COALESCE(last_error,''), COALESCE(last_replay_id,'')
FROM engagement_program_versions WHERE program_key = $1 ORDER BY version_no ASC`, programKey)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]ProgramVersion, 0)
	for rows.Next() {
		item, scanErr := scanVersion(rows)
		if scanErr == nil {
			items = append(items, item)
		}
	}
	return items
}

func (r *PostgresRepository) SaveJournalEntry(entry JournalEntry) error {
	_, err := r.db.Exec(`INSERT INTO engagement_journal_entries (entry_id, program_key, version_no, subject_id, account_key, entry_type, amount, rule_key, event_id, event_type, correlation_id, occurred_at, created_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,NULLIF($8,''),NULLIF($9,''),NULLIF($10,''),NULLIF($11,''),$12,$13)
ON CONFLICT (entry_id) DO UPDATE SET program_key=EXCLUDED.program_key, version_no=EXCLUDED.version_no, subject_id=EXCLUDED.subject_id, account_key=EXCLUDED.account_key, entry_type=EXCLUDED.entry_type, amount=EXCLUDED.amount, rule_key=EXCLUDED.rule_key, event_id=EXCLUDED.event_id, event_type=EXCLUDED.event_type, correlation_id=EXCLUDED.correlation_id, occurred_at=EXCLUDED.occurred_at, created_at=EXCLUDED.created_at`,
		entry.ID, entry.ProgramKey, entry.Version, entry.SubjectID, entry.AccountKey, entry.EntryType, entry.Amount, entry.RuleKey, entry.EventID, entry.EventType, entry.CorrelationID, entry.OccurredAt, entry.CreatedAt)
	return err
}

func (r *PostgresRepository) ListJournal(programKey, subjectID, accountKey string) []JournalEntry {
	rows, err := r.db.Query(`SELECT entry_id, program_key, version_no, subject_id, account_key, entry_type, amount, COALESCE(rule_key,''), COALESCE(event_id,''), COALESCE(event_type,''), COALESCE(correlation_id,''), occurred_at, created_at
FROM engagement_journal_entries
WHERE ($1 = '' OR program_key = $1) AND ($2 = '' OR subject_id = $2) AND ($3 = '' OR account_key = $3)
ORDER BY occurred_at ASC, created_at ASC`, programKey, subjectID, accountKey)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]JournalEntry, 0)
	for rows.Next() {
		var item JournalEntry
		_ = rows.Scan(&item.ID, &item.ProgramKey, &item.Version, &item.SubjectID, &item.AccountKey, &item.EntryType, &item.Amount, &item.RuleKey, &item.EventID, &item.EventType, &item.CorrelationID, &item.OccurredAt, &item.CreatedAt)
		items = append(items, item)
	}
	return items
}

func (r *PostgresRepository) SaveBalance(balance BalanceSnapshot) error {
	_, err := r.db.Exec(`INSERT INTO engagement_balances (program_key, subject_id, account_key, balance, updated_at)
VALUES ($1,$2,$3,$4,$5)
ON CONFLICT (program_key, subject_id, account_key) DO UPDATE SET balance=EXCLUDED.balance, updated_at=EXCLUDED.updated_at`,
		balance.ProgramKey, balance.SubjectID, balance.AccountKey, balance.Balance, balance.UpdatedAt)
	return err
}

func (r *PostgresRepository) GetBalance(programKey, subjectID, accountKey string) (BalanceSnapshot, bool) {
	row := r.db.QueryRow(`SELECT program_key, subject_id, account_key, balance, updated_at
FROM engagement_balances WHERE program_key = $1 AND subject_id = $2 AND account_key = $3`, programKey, subjectID, accountKey)
	var item BalanceSnapshot
	if err := row.Scan(&item.ProgramKey, &item.SubjectID, &item.AccountKey, &item.Balance, &item.UpdatedAt); err != nil {
		return BalanceSnapshot{}, false
	}
	return item, true
}

func (r *PostgresRepository) ListBalances(programKey, subjectID string) []BalanceSnapshot {
	rows, err := r.db.Query(`SELECT program_key, subject_id, account_key, balance, updated_at
FROM engagement_balances WHERE ($1 = '' OR program_key = $1) AND ($2 = '' OR subject_id = $2)
ORDER BY account_key ASC`, programKey, subjectID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]BalanceSnapshot, 0)
	for rows.Next() {
		var item BalanceSnapshot
		_ = rows.Scan(&item.ProgramKey, &item.SubjectID, &item.AccountKey, &item.Balance, &item.UpdatedAt)
		items = append(items, item)
	}
	return items
}

func (r *PostgresRepository) SaveQualification(state QualificationState) error {
	_, err := r.db.Exec(`INSERT INTO engagement_qualifications (program_key, subject_id, tier_key, score, updated_at)
VALUES ($1,$2,NULLIF($3,''),$4,$5)
ON CONFLICT (program_key, subject_id) DO UPDATE SET tier_key=EXCLUDED.tier_key, score=EXCLUDED.score, updated_at=EXCLUDED.updated_at`,
		state.ProgramKey, state.SubjectID, state.TierKey, state.Score, state.UpdatedAt)
	return err
}

func (r *PostgresRepository) GetQualification(programKey, subjectID string) (QualificationState, bool) {
	row := r.db.QueryRow(`SELECT program_key, subject_id, COALESCE(tier_key,''), score, updated_at
FROM engagement_qualifications WHERE program_key = $1 AND subject_id = $2`, programKey, subjectID)
	var item QualificationState
	if err := row.Scan(&item.ProgramKey, &item.SubjectID, &item.TierKey, &item.Score, &item.UpdatedAt); err != nil {
		return QualificationState{}, false
	}
	return item, true
}

func (r *PostgresRepository) SaveAchievement(grant AchievementGrant) error {
	_, err := r.db.Exec(`INSERT INTO engagement_achievements (grant_id, program_key, subject_id, achievement_key, rule_key, event_id, granted_at)
VALUES ($1,$2,$3,$4,NULLIF($5,''),NULLIF($6,''),$7)
ON CONFLICT (grant_id) DO UPDATE SET program_key=EXCLUDED.program_key, subject_id=EXCLUDED.subject_id, achievement_key=EXCLUDED.achievement_key, rule_key=EXCLUDED.rule_key, event_id=EXCLUDED.event_id, granted_at=EXCLUDED.granted_at`,
		grant.ID, grant.ProgramKey, grant.SubjectID, grant.AchievementKey, grant.RuleKey, grant.EventID, grant.GrantedAt)
	return err
}

func (r *PostgresRepository) ListAchievements(programKey, subjectID string) []AchievementGrant {
	rows, err := r.db.Query(`SELECT grant_id, program_key, subject_id, achievement_key, COALESCE(rule_key,''), COALESCE(event_id,''), granted_at
FROM engagement_achievements WHERE ($1 = '' OR program_key = $1) AND ($2 = '' OR subject_id = $2)
ORDER BY granted_at ASC`, programKey, subjectID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]AchievementGrant, 0)
	for rows.Next() {
		var item AchievementGrant
		_ = rows.Scan(&item.ID, &item.ProgramKey, &item.SubjectID, &item.AchievementKey, &item.RuleKey, &item.EventID, &item.GrantedAt)
		items = append(items, item)
	}
	return items
}

func (r *PostgresRepository) HasAchievement(programKey, subjectID, achievementKey string) bool {
	row := r.db.QueryRow(`SELECT COUNT(1) FROM engagement_achievements WHERE program_key = $1 AND subject_id = $2 AND achievement_key = $3`, programKey, subjectID, achievementKey)
	var count int
	if err := row.Scan(&count); err != nil {
		return false
	}
	return count > 0
}

func (r *PostgresRepository) MarkProcessed(idempotencyKey string) bool {
	result, err := r.db.Exec(`INSERT INTO engagement_processed_events (idempotency_key, processed_at) VALUES ($1,$2)
ON CONFLICT (idempotency_key) DO NOTHING`, idempotencyKey, time.Now().UTC())
	if err != nil {
		return false
	}
	affected, err := result.RowsAffected()
	return err == nil && affected > 0
}

func (r *PostgresRepository) ClearProgramState(programKey string) error {
	if _, err := r.db.Exec(`DELETE FROM engagement_journal_entries WHERE program_key = $1`, programKey); err != nil {
		return err
	}
	if _, err := r.db.Exec(`DELETE FROM engagement_balances WHERE program_key = $1`, programKey); err != nil {
		return err
	}
	if _, err := r.db.Exec(`DELETE FROM engagement_qualifications WHERE program_key = $1`, programKey); err != nil {
		return err
	}
	if _, err := r.db.Exec(`DELETE FROM engagement_achievements WHERE program_key = $1`, programKey); err != nil {
		return err
	}
	if _, err := r.db.Exec(`DELETE FROM engagement_processed_events WHERE idempotency_key LIKE $1`, programKey+"|%"); err != nil {
		return err
	}
	if _, err := r.db.Exec(`UPDATE engagement_consumers SET processed = 0, last_event_id = NULL, last_event_at = NULL, last_error = NULL, status = 'idle', updated_at = $2 WHERE program_key = $1`, programKey, time.Now().UTC()); err != nil {
		return err
	}
	return nil
}

func (r *PostgresRepository) SaveConsumerState(state ConsumerState) error {
	eventTypes, _ := json.Marshal(state.EventTypes)
	_, err := r.db.Exec(`INSERT INTO engagement_consumers (consumer_id, program_key, version_no, event_types_json, processed, last_event_id, last_event_at, last_error, status, updated_at)
VALUES ($1,$2,$3,$4,$5,NULLIF($6,''),$7,NULLIF($8,''),$9,$10)
ON CONFLICT (consumer_id) DO UPDATE SET program_key=EXCLUDED.program_key, version_no=EXCLUDED.version_no, event_types_json=EXCLUDED.event_types_json, processed=EXCLUDED.processed, last_event_id=EXCLUDED.last_event_id, last_event_at=EXCLUDED.last_event_at, last_error=EXCLUDED.last_error, status=EXCLUDED.status, updated_at=EXCLUDED.updated_at`,
		state.ID, state.ProgramKey, state.Version, eventTypes, state.Processed, state.LastEventID, nullTime(state.LastEventAt), state.LastError, state.Status, state.UpdatedAt)
	return err
}

func (r *PostgresRepository) GetConsumerState(id string) (ConsumerState, bool) {
	row := r.db.QueryRow(`SELECT consumer_id, program_key, version_no, COALESCE(event_types_json,'[]'::jsonb), processed, COALESCE(last_event_id,''), last_event_at, COALESCE(last_error,''), status, updated_at
FROM engagement_consumers WHERE consumer_id = $1`, id)
	item, err := scanConsumerState(row)
	if err != nil {
		return ConsumerState{}, false
	}
	return item, true
}

func (r *PostgresRepository) ListConsumerStates() []ConsumerState {
	rows, err := r.db.Query(`SELECT consumer_id, program_key, version_no, COALESCE(event_types_json,'[]'::jsonb), processed, COALESCE(last_event_id,''), last_event_at, COALESCE(last_error,''), status, updated_at
FROM engagement_consumers ORDER BY consumer_id ASC`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]ConsumerState, 0)
	for rows.Next() {
		item, scanErr := scanConsumerState(rows)
		if scanErr == nil {
			items = append(items, item)
		}
	}
	return items
}

func (r *PostgresRepository) SaveReplayRun(run ReplayRun) error {
	validation, _ := json.Marshal(run.Validation)
	_, err := r.db.Exec(`INSERT INTO engagement_replay_runs (replay_run_id, program_key, version_no, status, matching_events, processed, error_message, started_at, completed_at, created_by, validation_json, job_id)
VALUES ($1,$2,$3,$4,$5,$6,NULLIF($7,''),$8,$9,NULLIF($10,''),$11,NULLIF($12,''))
ON CONFLICT (replay_run_id) DO UPDATE SET program_key=EXCLUDED.program_key, version_no=EXCLUDED.version_no, status=EXCLUDED.status, matching_events=EXCLUDED.matching_events, processed=EXCLUDED.processed, error_message=EXCLUDED.error_message, started_at=EXCLUDED.started_at, completed_at=EXCLUDED.completed_at, created_by=EXCLUDED.created_by, validation_json=EXCLUDED.validation_json, job_id=EXCLUDED.job_id`,
		run.ID, run.ProgramKey, run.Version, run.Status, run.MatchingEvents, run.Processed, run.Error, run.StartedAt, nullTime(run.CompletedAt), run.CreatedBy, validation, run.JobID)
	return err
}

func (r *PostgresRepository) GetReplayRun(id string) (ReplayRun, bool) {
	row := r.db.QueryRow(`SELECT replay_run_id, program_key, version_no, status, matching_events, processed, COALESCE(error_message,''), started_at, completed_at, COALESCE(created_by,''), COALESCE(validation_json,'{}'::jsonb), COALESCE(job_id,'')
FROM engagement_replay_runs WHERE replay_run_id = $1`, id)
	item, err := scanReplayRun(row)
	if err != nil {
		return ReplayRun{}, false
	}
	return item, true
}

func (r *PostgresRepository) ListReplayRuns() []ReplayRun {
	rows, err := r.db.Query(`SELECT replay_run_id, program_key, version_no, status, matching_events, processed, COALESCE(error_message,''), started_at, completed_at, COALESCE(created_by,''), COALESCE(validation_json,'{}'::jsonb), COALESCE(job_id,'')
FROM engagement_replay_runs ORDER BY started_at DESC`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]ReplayRun, 0)
	for rows.Next() {
		item, scanErr := scanReplayRun(rows)
		if scanErr == nil {
			items = append(items, item)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].StartedAt.After(items[j].StartedAt) })
	return items
}

type scanner interface {
	Scan(dest ...any) error
}

func scanVersion(row scanner) (ProgramVersion, error) {
	var item ProgramVersion
	var rules []byte
	var publishedAt sql.NullTime
	err := row.Scan(&item.ProgramKey, &item.Version, &item.Status, &item.ChangeNote, &rules, &item.CreatedAt, &item.UpdatedAt, &publishedAt, &item.PublishedBy, &item.LastError, &item.LastReplayID)
	if err != nil {
		return ProgramVersion{}, err
	}
	if publishedAt.Valid {
		item.PublishedAt = publishedAt.Time
	}
	_ = json.Unmarshal(defaultJSONArray(rules), &item.Rules)
	return item, nil
}

func scanConsumerState(row scanner) (ConsumerState, error) {
	var item ConsumerState
	var eventTypes []byte
	var lastEventAt sql.NullTime
	err := row.Scan(&item.ID, &item.ProgramKey, &item.Version, &eventTypes, &item.Processed, &item.LastEventID, &lastEventAt, &item.LastError, &item.Status, &item.UpdatedAt)
	if err != nil {
		return ConsumerState{}, err
	}
	if lastEventAt.Valid {
		item.LastEventAt = lastEventAt.Time
	}
	_ = json.Unmarshal(defaultJSONArray(eventTypes), &item.EventTypes)
	return item, nil
}

func scanReplayRun(row scanner) (ReplayRun, error) {
	var item ReplayRun
	var validation []byte
	var completedAt sql.NullTime
	err := row.Scan(&item.ID, &item.ProgramKey, &item.Version, &item.Status, &item.MatchingEvents, &item.Processed, &item.Error, &item.StartedAt, &completedAt, &item.CreatedBy, &validation, &item.JobID)
	if err != nil {
		return ReplayRun{}, err
	}
	if completedAt.Valid {
		item.CompletedAt = completedAt.Time
	}
	_ = json.Unmarshal(defaultJSONObject(validation), &item.Validation)
	return item, nil
}

func defaultJSONArray(raw []byte) []byte {
	if len(raw) == 0 {
		return []byte(`[]`)
	}
	return raw
}

func defaultJSONObject(raw []byte) []byte {
	if len(raw) == 0 {
		return []byte(`{}`)
	}
	return raw
}

func nullTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}
