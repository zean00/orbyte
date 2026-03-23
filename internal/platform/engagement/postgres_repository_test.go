package engagement

import (
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestPostgresRepositoryProgramStateAndReplayPersistence(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New failed: %v", err)
	}
	defer db.Close()

	repo := NewPostgresRepository(db)
	now := time.Now().UTC()
	publishedAt := now.Add(time.Minute)
	completedAt := now.Add(2 * time.Minute)

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO engagement_programs (")).
		WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.SaveProgram(Program{
		Key:              "loyalty",
		Name:             "Customer Loyalty",
		SubjectType:      "customer",
		Status:           "active",
		PublishedVersion: 1,
		CreatedAt:        now,
		UpdatedAt:        now,
		UpdatedBy:        "tester",
	}); err != nil {
		t.Fatalf("save program failed: %v", err)
	}

	programRows := sqlmock.NewRows([]string{"program_key", "name", "subject_type", "status", "published_version", "created_at", "updated_at", "updated_by"}).
		AddRow("loyalty", "Customer Loyalty", "customer", "active", 1, now, now, "tester")
	mock.ExpectQuery(regexp.QuoteMeta("SELECT program_key, name, COALESCE(subject_type,''), status, COALESCE(published_version,0), created_at, updated_at, COALESCE(updated_by,'')")).
		WithArgs("loyalty").
		WillReturnRows(programRows)
	program, ok := repo.GetProgram("loyalty")
	if !ok || program.Key != "loyalty" || program.PublishedVersion != 1 {
		t.Fatalf("unexpected program result ok=%v program=%+v", ok, program)
	}

	listProgramRows := sqlmock.NewRows([]string{"program_key", "name", "subject_type", "status", "published_version", "created_at", "updated_at", "updated_by"}).
		AddRow("loyalty", "Customer Loyalty", "customer", "active", 1, now, now, "tester")
	mock.ExpectQuery(regexp.QuoteMeta("SELECT program_key, name, COALESCE(subject_type,''), status, COALESCE(published_version,0), created_at, updated_at, COALESCE(updated_by,'')")).
		WillReturnRows(listProgramRows)
	programs := repo.ListPrograms()
	if len(programs) != 1 || programs[0].Key != "loyalty" {
		t.Fatalf("unexpected programs: %+v", programs)
	}

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO engagement_program_versions (")).
		WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.SaveVersion(ProgramVersion{
		ProgramKey:   "loyalty",
		Version:      1,
		Status:       "published",
		ChangeNote:   "initial",
		Rules:        []Rule{{Key: "earn", Action: "credit_points", SourceEventTypes: []string{"order.completed"}, SubjectSource: "actor_id", FixedAmount: 10}},
		CreatedAt:    now,
		UpdatedAt:    now,
		PublishedAt:  publishedAt,
		PublishedBy:  "tester",
		LastError:    "",
		LastReplayID: "replay-1",
	}); err != nil {
		t.Fatalf("save version failed: %v", err)
	}

	versionRows := sqlmock.NewRows([]string{"program_key", "version_no", "status", "change_note", "rules_json", "created_at", "updated_at", "published_at", "published_by", "last_error", "last_replay_id"}).
		AddRow("loyalty", 1, "published", "initial", `[{"key":"earn","action":"credit_points","source_event_types":["order.completed"],"subject_source":"actor_id","fixed_amount":10}]`, now, now, publishedAt, "tester", "", "replay-1")
	mock.ExpectQuery(regexp.QuoteMeta("SELECT program_key, version_no, status, COALESCE(change_note,''), COALESCE(rules_json,'[]'::jsonb), created_at, updated_at, published_at, COALESCE(published_by,''), COALESCE(last_error,''), COALESCE(last_replay_id,'')")).
		WithArgs("loyalty", 1).
		WillReturnRows(versionRows)
	version, ok := repo.GetVersion("loyalty", 1)
	if !ok || version.Version != 1 || len(version.Rules) != 1 || version.LastReplayID != "replay-1" {
		t.Fatalf("unexpected version result ok=%v version=%+v", ok, version)
	}

	listVersionRows := sqlmock.NewRows([]string{"program_key", "version_no", "status", "change_note", "rules_json", "created_at", "updated_at", "published_at", "published_by", "last_error", "last_replay_id"}).
		AddRow("loyalty", 1, "published", "initial", `[{"key":"earn","action":"credit_points","source_event_types":["order.completed"],"subject_source":"actor_id","fixed_amount":10}]`, now, now, publishedAt, "tester", "", "replay-1")
	mock.ExpectQuery(regexp.QuoteMeta("SELECT program_key, version_no, status, COALESCE(change_note,''), COALESCE(rules_json,'[]'::jsonb), created_at, updated_at, published_at, COALESCE(published_by,''), COALESCE(last_error,''), COALESCE(last_replay_id,'')")).
		WithArgs("loyalty").
		WillReturnRows(listVersionRows)
	versions := repo.ListVersions("loyalty")
	if len(versions) != 1 || versions[0].Version != 1 {
		t.Fatalf("unexpected versions: %+v", versions)
	}

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO engagement_journal_entries (")).
		WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.SaveJournalEntry(JournalEntry{
		ID:         "entry-1",
		ProgramKey: "loyalty",
		Version:    1,
		SubjectID:  "cust-1",
		AccountKey: "points",
		EntryType:  "credit",
		Amount:     10,
		RuleKey:    "earn",
		EventID:    "evt-1",
		EventType:  "order.completed",
		OccurredAt: now,
		CreatedAt:  now,
	}); err != nil {
		t.Fatalf("save journal entry failed: %v", err)
	}

	journalRows := sqlmock.NewRows([]string{"entry_id", "program_key", "version_no", "subject_id", "account_key", "entry_type", "amount", "rule_key", "event_id", "event_type", "correlation_id", "occurred_at", "created_at"}).
		AddRow("entry-1", "loyalty", 1, "cust-1", "points", "credit", 10, "earn", "evt-1", "order.completed", "", now, now)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT entry_id, program_key, version_no, subject_id, account_key, entry_type, amount, COALESCE(rule_key,''), COALESCE(event_id,''), COALESCE(event_type,''), COALESCE(correlation_id,''), occurred_at, created_at")).
		WithArgs("loyalty", "cust-1", "points").
		WillReturnRows(journalRows)
	journal := repo.ListJournal("loyalty", "cust-1", "points")
	if len(journal) != 1 || journal[0].ID != "entry-1" {
		t.Fatalf("unexpected journal: %+v", journal)
	}

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO engagement_balances (")).
		WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.SaveBalance(BalanceSnapshot{ProgramKey: "loyalty", SubjectID: "cust-1", AccountKey: "points", Balance: 10, UpdatedAt: now}); err != nil {
		t.Fatalf("save balance failed: %v", err)
	}
	balanceRows := sqlmock.NewRows([]string{"program_key", "subject_id", "account_key", "balance", "updated_at"}).
		AddRow("loyalty", "cust-1", "points", 10, now)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT program_key, subject_id, account_key, balance, updated_at")).
		WithArgs("loyalty", "cust-1", "points").
		WillReturnRows(balanceRows)
	balance, ok := repo.GetBalance("loyalty", "cust-1", "points")
	if !ok || balance.Balance != 10 {
		t.Fatalf("unexpected balance result ok=%v balance=%+v", ok, balance)
	}
	listBalanceRows := sqlmock.NewRows([]string{"program_key", "subject_id", "account_key", "balance", "updated_at"}).
		AddRow("loyalty", "cust-1", "points", 10, now)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT program_key, subject_id, account_key, balance, updated_at")).
		WithArgs("loyalty", "cust-1").
		WillReturnRows(listBalanceRows)
	balances := repo.ListBalances("loyalty", "cust-1")
	if len(balances) != 1 || balances[0].AccountKey != "points" {
		t.Fatalf("unexpected balances: %+v", balances)
	}

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO engagement_qualifications (")).
		WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.SaveQualification(QualificationState{ProgramKey: "loyalty", SubjectID: "cust-1", TierKey: "silver", Score: 10, UpdatedAt: now}); err != nil {
		t.Fatalf("save qualification failed: %v", err)
	}
	qualificationRows := sqlmock.NewRows([]string{"program_key", "subject_id", "tier_key", "score", "updated_at"}).
		AddRow("loyalty", "cust-1", "silver", 10, now)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT program_key, subject_id, COALESCE(tier_key,''), score, updated_at")).
		WithArgs("loyalty", "cust-1").
		WillReturnRows(qualificationRows)
	qualification, ok := repo.GetQualification("loyalty", "cust-1")
	if !ok || qualification.TierKey != "silver" {
		t.Fatalf("unexpected qualification result ok=%v qualification=%+v", ok, qualification)
	}

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO engagement_achievements (")).
		WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.SaveAchievement(AchievementGrant{ID: "grant-1", ProgramKey: "loyalty", SubjectID: "cust-1", AchievementKey: "spender", RuleKey: "earn", EventID: "evt-1", GrantedAt: now}); err != nil {
		t.Fatalf("save achievement failed: %v", err)
	}
	achievementRows := sqlmock.NewRows([]string{"grant_id", "program_key", "subject_id", "achievement_key", "rule_key", "event_id", "granted_at"}).
		AddRow("grant-1", "loyalty", "cust-1", "spender", "earn", "evt-1", now)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT grant_id, program_key, subject_id, achievement_key, COALESCE(rule_key,''), COALESCE(event_id,''), granted_at")).
		WithArgs("loyalty", "cust-1").
		WillReturnRows(achievementRows)
	achievements := repo.ListAchievements("loyalty", "cust-1")
	if len(achievements) != 1 || achievements[0].AchievementKey != "spender" {
		t.Fatalf("unexpected achievements: %+v", achievements)
	}

	hasAchievementRows := sqlmock.NewRows([]string{"count"}).AddRow(1)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(1) FROM engagement_achievements WHERE program_key = $1 AND subject_id = $2 AND achievement_key = $3")).
		WithArgs("loyalty", "cust-1", "spender").
		WillReturnRows(hasAchievementRows)
	if !repo.HasAchievement("loyalty", "cust-1", "spender") {
		t.Fatal("expected achievement existence check to return true")
	}

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO engagement_processed_events (idempotency_key, processed_at) VALUES ($1,$2) ON CONFLICT (idempotency_key) DO NOTHING")).
		WillReturnResult(sqlmock.NewResult(1, 1))
	if !repo.MarkProcessed("loyalty|v1|earn|evt-1|cust-1|points") {
		t.Fatal("expected first processed marker to succeed")
	}

	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM engagement_journal_entries WHERE program_key = $1")).
		WithArgs("loyalty").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM engagement_balances WHERE program_key = $1")).
		WithArgs("loyalty").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM engagement_qualifications WHERE program_key = $1")).
		WithArgs("loyalty").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM engagement_achievements WHERE program_key = $1")).
		WithArgs("loyalty").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM engagement_processed_events WHERE idempotency_key LIKE $1")).
		WithArgs("loyalty|%").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE engagement_consumers SET processed = 0, last_event_id = NULL, last_event_at = NULL, last_error = NULL, status = 'idle', updated_at = $2 WHERE program_key = $1")).
		WithArgs("loyalty", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.ClearProgramState("loyalty"); err != nil {
		t.Fatalf("clear program state failed: %v", err)
	}

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO engagement_consumers (")).
		WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.SaveConsumerState(ConsumerState{
		ID:          "loyalty:v1",
		ProgramKey:  "loyalty",
		Version:     1,
		EventTypes:  []string{"order.completed"},
		Processed:   3,
		LastEventID: "evt-1",
		LastEventAt: now,
		Status:      "active",
		UpdatedAt:   now,
	}); err != nil {
		t.Fatalf("save consumer failed: %v", err)
	}
	consumerRows := sqlmock.NewRows([]string{"consumer_id", "program_key", "version_no", "event_types_json", "processed", "last_event_id", "last_event_at", "last_error", "status", "updated_at"}).
		AddRow("loyalty:v1", "loyalty", 1, `["order.completed"]`, 3, "evt-1", now, "", "active", now)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT consumer_id, program_key, version_no, COALESCE(event_types_json,'[]'::jsonb), processed, COALESCE(last_event_id,''), last_event_at, COALESCE(last_error,''), status, updated_at")).
		WithArgs("loyalty:v1").
		WillReturnRows(consumerRows)
	consumer, ok := repo.GetConsumerState("loyalty:v1")
	if !ok || consumer.Processed != 3 || len(consumer.EventTypes) != 1 {
		t.Fatalf("unexpected consumer result ok=%v consumer=%+v", ok, consumer)
	}

	listConsumerRows := sqlmock.NewRows([]string{"consumer_id", "program_key", "version_no", "event_types_json", "processed", "last_event_id", "last_event_at", "last_error", "status", "updated_at"}).
		AddRow("loyalty:v1", "loyalty", 1, `["order.completed"]`, 3, "evt-1", now, "", "active", now)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT consumer_id, program_key, version_no, COALESCE(event_types_json,'[]'::jsonb), processed, COALESCE(last_event_id,''), last_event_at, COALESCE(last_error,''), status, updated_at")).
		WillReturnRows(listConsumerRows)
	consumers := repo.ListConsumerStates()
	if len(consumers) != 1 || consumers[0].ID != "loyalty:v1" {
		t.Fatalf("unexpected consumers: %+v", consumers)
	}

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO engagement_replay_runs (")).
		WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.SaveReplayRun(ReplayRun{
		ID:             "replay-1",
		ProgramKey:     "loyalty",
		Version:        1,
		Status:         "succeeded",
		MatchingEvents: 1,
		Processed:      1,
		StartedAt:      now,
		CompletedAt:    completedAt,
		CreatedBy:      "tester",
		Validation:     ValidationReport{Valid: true},
		JobID:          "job-1",
	}); err != nil {
		t.Fatalf("save replay run failed: %v", err)
	}
	replayRows := sqlmock.NewRows([]string{"replay_run_id", "program_key", "version_no", "status", "matching_events", "processed", "error_message", "started_at", "completed_at", "created_by", "validation_json", "job_id"}).
		AddRow("replay-1", "loyalty", 1, "succeeded", 1, 1, "", now, completedAt, "tester", `{"valid":true}`, "job-1")
	mock.ExpectQuery(regexp.QuoteMeta("SELECT replay_run_id, program_key, version_no, status, matching_events, processed, COALESCE(error_message,''), started_at, completed_at, COALESCE(created_by,''), COALESCE(validation_json,'{}'::jsonb), COALESCE(job_id,'')")).
		WithArgs("replay-1").
		WillReturnRows(replayRows)
	replay, ok := repo.GetReplayRun("replay-1")
	if !ok || replay.JobID != "job-1" || !replay.Validation.Valid {
		t.Fatalf("unexpected replay result ok=%v replay=%+v", ok, replay)
	}

	listReplayRows := sqlmock.NewRows([]string{"replay_run_id", "program_key", "version_no", "status", "matching_events", "processed", "error_message", "started_at", "completed_at", "created_by", "validation_json", "job_id"}).
		AddRow("older", "loyalty", 1, "succeeded", 1, 1, "", now.Add(-time.Minute), completedAt, "tester", `{"valid":true}`, "job-older").
		AddRow("replay-1", "loyalty", 1, "succeeded", 1, 1, "", now, completedAt, "tester", `{"valid":true}`, "job-1")
	mock.ExpectQuery(regexp.QuoteMeta("SELECT replay_run_id, program_key, version_no, status, matching_events, processed, COALESCE(error_message,''), started_at, completed_at, COALESCE(created_by,''), COALESCE(validation_json,'{}'::jsonb), COALESCE(job_id,'')")).
		WillReturnRows(listReplayRows)
	replays := repo.ListReplayRuns()
	if len(replays) != 2 || replays[0].ID != "replay-1" || replays[1].ID != "older" {
		t.Fatalf("unexpected replay list: %+v", replays)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestEngagementPostgresScanHelpers(t *testing.T) {
	now := time.Now().UTC()

	version, err := scanVersion(stubScanner{values: []any{"loyalty", 1, "draft", "", []byte(nil), now, now, nil, "", "", ""}})
	if err != nil {
		t.Fatalf("scanVersion failed: %v", err)
	}
	if len(version.Rules) != 0 {
		t.Fatalf("expected empty rules fallback, got %+v", version.Rules)
	}

	consumer, err := scanConsumerState(stubScanner{values: []any{"loyalty:v1", "loyalty", 1, []byte(nil), 0, "", nil, "", "idle", now}})
	if err != nil {
		t.Fatalf("scanConsumerState failed: %v", err)
	}
	if len(consumer.EventTypes) != 0 {
		t.Fatalf("expected empty consumer event types fallback, got %+v", consumer.EventTypes)
	}

	replay, err := scanReplayRun(stubScanner{values: []any{"replay-1", "loyalty", 1, "queued", 1, 0, "", now, nil, "", []byte(nil), ""}})
	if err != nil {
		t.Fatalf("scanReplayRun failed: %v", err)
	}
	if replay.Validation.Valid {
		t.Fatalf("expected empty validation fallback, got %+v", replay.Validation)
	}

	if string(defaultJSONArray(nil)) != "[]" {
		t.Fatalf("unexpected defaultJSONArray fallback: %s", defaultJSONArray(nil))
	}
	if string(defaultJSONObject(nil)) != "{}" {
		t.Fatalf("unexpected defaultJSONObject fallback: %s", defaultJSONObject(nil))
	}
	if nullTime(time.Time{}) != nil {
		t.Fatal("expected zero time to map to nil")
	}
	if nullTime(now) == nil {
		t.Fatal("expected non-zero time to survive nullTime")
	}
}

type stubScanner struct {
	values []any
	err    error
}

func (s stubScanner) Scan(dest ...any) error {
	if s.err != nil {
		return s.err
	}
	if len(dest) != len(s.values) {
		return fmt.Errorf("dest count %d does not match values count %d", len(dest), len(s.values))
	}
	for i := range dest {
		switch target := dest[i].(type) {
		case *string:
			switch value := s.values[i].(type) {
			case nil:
				*target = ""
			case string:
				*target = value
			default:
				return fmt.Errorf("value %d is not a string", i)
			}
		case *int:
			value, ok := s.values[i].(int)
			if !ok {
				return fmt.Errorf("value %d is not an int", i)
			}
			*target = value
		case *[]byte:
			switch value := s.values[i].(type) {
			case nil:
				*target = nil
			case []byte:
				*target = value
			case string:
				*target = []byte(value)
			default:
				return fmt.Errorf("value %d is not bytes", i)
			}
		case *time.Time:
			value, ok := s.values[i].(time.Time)
			if !ok {
				return fmt.Errorf("value %d is not time.Time", i)
			}
			*target = value
		case *sql.NullTime:
			switch value := s.values[i].(type) {
			case nil:
				*target = sql.NullTime{}
			case time.Time:
				*target = sql.NullTime{Time: value, Valid: true}
			default:
				return fmt.Errorf("value %d is not nullable time", i)
			}
		default:
			return fmt.Errorf("unsupported scan target type %T", dest[i])
		}
	}
	return nil
}

type replayFailureRepo struct {
	*MemoryRepository
	hideVersion bool
	hideProgram bool
	clearErr    error
}

func (r *replayFailureRepo) GetVersion(programKey string, version int) (ProgramVersion, bool) {
	if r.hideVersion {
		return ProgramVersion{}, false
	}
	return r.MemoryRepository.GetVersion(programKey, version)
}

func (r *replayFailureRepo) GetProgram(key string) (Program, bool) {
	if r.hideProgram {
		return Program{}, false
	}
	return r.MemoryRepository.GetProgram(key)
}

func (r *replayFailureRepo) ClearProgramState(programKey string) error {
	if r.clearErr != nil {
		return r.clearErr
	}
	return r.MemoryRepository.ClearProgramState(programKey)
}

func TestExecuteReplayFailurePaths(t *testing.T) {
	t.Run("missing run", func(t *testing.T) {
		svc := NewService()
		if _, err := svc.executeReplay("missing"); err == nil {
			t.Fatal("expected missing replay run error")
		}
	})

	t.Run("missing version", func(t *testing.T) {
		repo := &replayFailureRepo{MemoryRepository: NewMemoryRepository(), hideVersion: true}
		svc := NewServiceWithRepository(repo)
		run := ReplayRun{ID: "replay-1", ProgramKey: "loyalty", Version: 1, Status: "queued", StartedAt: time.Now().UTC()}
		if err := repo.SaveReplayRun(run); err != nil {
			t.Fatalf("save replay run failed: %v", err)
		}
		if _, err := svc.executeReplay(run.ID); err == nil {
			t.Fatal("expected missing version error")
		}
		updated, ok := svc.GetReplayRun(run.ID)
		if !ok || updated.Status != "failed" || updated.Error == "" {
			t.Fatalf("unexpected replay state after missing version: %+v", updated)
		}
	})

	t.Run("missing program", func(t *testing.T) {
		repo := &replayFailureRepo{MemoryRepository: NewMemoryRepository(), hideProgram: true}
		svc := NewServiceWithRepository(repo)
		if err := repo.SaveVersion(ProgramVersion{ProgramKey: "loyalty", Version: 1, Status: "published"}); err != nil {
			t.Fatalf("save version failed: %v", err)
		}
		run := ReplayRun{ID: "replay-2", ProgramKey: "loyalty", Version: 1, Status: "queued", StartedAt: time.Now().UTC()}
		if err := repo.SaveReplayRun(run); err != nil {
			t.Fatalf("save replay run failed: %v", err)
		}
		if _, err := svc.executeReplay(run.ID); err == nil {
			t.Fatal("expected missing program error")
		}
		updated, _ := svc.GetReplayRun(run.ID)
		if updated.Status != "failed" || updated.Error == "" {
			t.Fatalf("unexpected replay state after missing program: %+v", updated)
		}
	})

	t.Run("program no longer published", func(t *testing.T) {
		svc := NewService()
		if _, err := svc.CreateProgram("loyalty", "Customer Loyalty", "customer", "tester"); err != nil {
			t.Fatalf("create program failed: %v", err)
		}
		if err := svc.repo.SaveVersion(ProgramVersion{ProgramKey: "loyalty", Version: 1, Status: "published"}); err != nil {
			t.Fatalf("save version failed: %v", err)
		}
		if err := svc.repo.SaveReplayRun(ReplayRun{ID: "replay-3", ProgramKey: "loyalty", Version: 1, Status: "queued", StartedAt: time.Now().UTC()}); err != nil {
			t.Fatalf("save replay run failed: %v", err)
		}
		if _, err := svc.executeReplay("replay-3"); err == nil {
			t.Fatal("expected publication guard failure")
		}
		run, _ := svc.GetReplayRun("replay-3")
		if run.Status != "failed" || run.Error == "" {
			t.Fatalf("unexpected replay state after publication guard failure: %+v", run)
		}
	})

	t.Run("clear state failure", func(t *testing.T) {
		repo := &replayFailureRepo{MemoryRepository: NewMemoryRepository(), clearErr: errors.New("clear failed")}
		svc := NewServiceWithRepository(repo)
		if err := repo.SaveProgram(Program{Key: "loyalty", Status: "active", PublishedVersion: 1}); err != nil {
			t.Fatalf("save program failed: %v", err)
		}
		if err := repo.SaveVersion(ProgramVersion{ProgramKey: "loyalty", Version: 1, Status: "published"}); err != nil {
			t.Fatalf("save version failed: %v", err)
		}
		if err := repo.SaveReplayRun(ReplayRun{ID: "replay-4", ProgramKey: "loyalty", Version: 1, Status: "queued", StartedAt: time.Now().UTC()}); err != nil {
			t.Fatalf("save replay run failed: %v", err)
		}
		if _, err := svc.executeReplay("replay-4"); err == nil {
			t.Fatal("expected clear state failure")
		}
		run, _ := svc.GetReplayRun("replay-4")
		if run.Status != "failed" || run.Error != "clear failed" {
			t.Fatalf("unexpected replay state after clear failure: %+v", run)
		}
	})
}
