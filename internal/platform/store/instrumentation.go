package store

import (
	"context"
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"fmt"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"orbyte/internal/platform/observability"
)

type QueryHandle interface {
	Exec(query string, args ...any) (sql.Result, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	Query(query string, args ...any) (*sql.Rows, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) RowScanner
	QueryRowContext(ctx context.Context, query string, args ...any) RowScanner
}

type DB interface {
	QueryHandle
	BeginTx(ctx context.Context, opts *sql.TxOptions) (Tx, error)
	RawDB() *sql.DB
}

type Tx interface {
	QueryHandle
	Commit() error
	Rollback() error
}

type RowScanner interface {
	Scan(dest ...any) error
}

type QueryMonitorOptions struct {
	SlowThreshold time.Duration
	TopOperations int
	SlowQueries   int
}

type QuerySummary struct {
	Operation       string    `json:"operation"`
	Subsystem       string    `json:"subsystem"`
	Component       string    `json:"component"`
	StatementKind   string    `json:"statement_kind"`
	Fingerprint     string    `json:"fingerprint"`
	Count           int64     `json:"count"`
	ErrorCount      int64     `json:"error_count"`
	SlowCount       int64     `json:"slow_count"`
	RowsAffected    int64     `json:"rows_affected"`
	TotalMillis     int64     `json:"total_millis"`
	AverageMillis   float64   `json:"average_millis"`
	LastDurationMS  int64     `json:"last_duration_millis"`
	LastOccurredAt  time.Time `json:"last_occurred_at,omitempty"`
	LastError       string    `json:"last_error,omitempty"`
}

type SlowQueryRecord struct {
	OccurredAt      time.Time `json:"occurred_at"`
	Operation       string    `json:"operation"`
	Subsystem       string    `json:"subsystem"`
	Component       string    `json:"component"`
	StatementKind   string    `json:"statement_kind"`
	Fingerprint     string    `json:"fingerprint"`
	DurationMillis  int64     `json:"duration_millis"`
	Outcome         string    `json:"outcome"`
	RowsAffected    int64     `json:"rows_affected,omitempty"`
	Error           string    `json:"error,omitempty"`
}

type QuerySnapshot struct {
	GeneratedAt    time.Time         `json:"generated_at"`
	SlowThreshold  int64             `json:"slow_threshold_millis"`
	TopOperations  []QuerySummary    `json:"top_operations"`
	RecentSlow     []SlowQueryRecord `json:"recent_slow_queries"`
}

type QueryMonitor struct {
	mu            sync.RWMutex
	obs           *observability.Service
	slowThreshold time.Duration
	topLimit      int
	slowLimit     int
	operations    map[string]*queryAggregate
	slowQueries   []SlowQueryRecord
}

type queryAggregate struct {
	QuerySummary
	totalDuration time.Duration
}

type queryRecord struct {
	Operation     string
	Subsystem     string
	Component     string
	StatementKind string
	Fingerprint   string
	Duration      time.Duration
	RowsAffected  int64
	Err           error
}

type instrumentedDB struct {
	raw       *sql.DB
	monitor   *QueryMonitor
	subsystem string
	component string
}

type instrumentedTx struct {
	raw       *sql.Tx
	monitor   *QueryMonitor
	subsystem string
	component string
}

type instrumentedRow struct {
	row         *sql.Row
	monitor     *QueryMonitor
	subsystem   string
	component   string
	operation   string
	kind        string
	query       string
	started     time.Time
	recorded    bool
}

var (
	whitespacePattern = regexp.MustCompile(`\s+`)
	literalPattern    = regexp.MustCompile(`'(?:''|[^'])*'|\b\d+\b|\$\d+`)
)

func NewQueryMonitor(obs *observability.Service, opts QueryMonitorOptions) *QueryMonitor {
	if opts.SlowThreshold <= 0 {
		opts.SlowThreshold = 250 * time.Millisecond
	}
	if opts.TopOperations <= 0 {
		opts.TopOperations = 20
	}
	if opts.SlowQueries <= 0 {
		opts.SlowQueries = 50
	}
	return &QueryMonitor{
		obs:           obs,
		slowThreshold: opts.SlowThreshold,
		topLimit:      opts.TopOperations,
		slowLimit:     opts.SlowQueries,
		operations:    map[string]*queryAggregate{},
		slowQueries:   []SlowQueryRecord{},
	}
}

func (m *QueryMonitor) Record(record queryRecord) {
	if m == nil {
		return
	}
	outcome := "ok"
	if record.Err != nil {
		outcome = "error"
	}
	if m.obs != nil {
		baseKey := "db.query." + sanitizeMetricPart(record.Subsystem) + "." + sanitizeMetricPart(record.Component) + "." + sanitizeMetricPart(record.StatementKind)
		m.obs.Inc(baseKey + ".total")
		m.obs.Inc(baseKey + "." + outcome + ".total")
		m.obs.Observe(baseKey+".duration", record.Duration)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	key := strings.Join([]string{
		record.Subsystem,
		record.Component,
		record.Operation,
		record.StatementKind,
		record.Fingerprint,
	}, "|")
	item := m.operations[key]
	if item == nil {
		item = &queryAggregate{QuerySummary: QuerySummary{
			Operation:     record.Operation,
			Subsystem:     record.Subsystem,
			Component:     record.Component,
			StatementKind: record.StatementKind,
			Fingerprint:   record.Fingerprint,
		}}
		m.operations[key] = item
	}
	item.Count++
	item.LastOccurredAt = time.Now().UTC()
	item.LastDurationMS = record.Duration.Milliseconds()
	item.RowsAffected += record.RowsAffected
	item.totalDuration += record.Duration
	item.TotalMillis = item.totalDuration.Milliseconds()
	item.AverageMillis = float64(item.TotalMillis) / float64(item.Count)
	if record.Err != nil {
		item.ErrorCount++
		item.LastError = record.Err.Error()
	} else {
		item.LastError = ""
	}
	if record.Duration >= m.slowThreshold {
		item.SlowCount++
		slow := SlowQueryRecord{
			OccurredAt:     time.Now().UTC(),
			Operation:      record.Operation,
			Subsystem:      record.Subsystem,
			Component:      record.Component,
			StatementKind:  record.StatementKind,
			Fingerprint:    record.Fingerprint,
			DurationMillis: record.Duration.Milliseconds(),
			Outcome:        outcome,
			RowsAffected:   record.RowsAffected,
		}
		if record.Err != nil {
			slow.Error = record.Err.Error()
		}
		m.slowQueries = append(m.slowQueries, slow)
		if len(m.slowQueries) > m.slowLimit {
			m.slowQueries = append([]SlowQueryRecord(nil), m.slowQueries[len(m.slowQueries)-m.slowLimit:]...)
		}
		if m.obs != nil {
			_ = m.obs.EmitLogEvent("db.query.slow", map[string]any{
				"operation":       slow.Operation,
				"subsystem":       slow.Subsystem,
				"component":       slow.Component,
				"statement_kind":  slow.StatementKind,
				"fingerprint":     slow.Fingerprint,
				"duration_millis": slow.DurationMillis,
				"outcome":         slow.Outcome,
				"rows_affected":   slow.RowsAffected,
				"error":           slow.Error,
			})
		}
	}
}

func (m *QueryMonitor) Snapshot() QuerySnapshot {
	if m == nil {
		return QuerySnapshot{}
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	items := make([]QuerySummary, 0, len(m.operations))
	for _, item := range m.operations {
		items = append(items, item.QuerySummary)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].TotalMillis == items[j].TotalMillis {
			return items[i].Operation < items[j].Operation
		}
		return items[i].TotalMillis > items[j].TotalMillis
	})
	if len(items) > m.topLimit {
		items = append([]QuerySummary(nil), items[:m.topLimit]...)
	}
	slow := append([]SlowQueryRecord(nil), m.slowQueries...)
	return QuerySnapshot{
		GeneratedAt:   time.Now().UTC(),
		SlowThreshold: m.slowThreshold.Milliseconds(),
		TopOperations: items,
		RecentSlow:    slow,
	}
}

func UninstrumentedDB(db *sql.DB) DB {
	if db == nil {
		return nil
	}
	return &instrumentedDB{raw: db}
}

func UninstrumentedTx(tx *sql.Tx) Tx {
	if tx == nil {
		return nil
	}
	return &instrumentedTx{raw: tx}
}

func InstrumentDB(db *sql.DB, monitor *QueryMonitor, subsystem, component string) DB {
	if db == nil {
		return nil
	}
	return &instrumentedDB{raw: db, monitor: monitor, subsystem: subsystem, component: component}
}

func (db *instrumentedDB) RawDB() *sql.DB {
	if db == nil {
		return nil
	}
	return db.raw
}

func (db *instrumentedDB) BeginTx(ctx context.Context, opts *sql.TxOptions) (Tx, error) {
	started := time.Now()
	tx, err := db.raw.BeginTx(ctx, opts)
	db.record("transaction.begin", "other", "", started, 0, err)
	if err != nil {
		return nil, err
	}
	return &instrumentedTx{raw: tx, monitor: db.monitor, subsystem: db.subsystem, component: db.component}, nil
}

func (db *instrumentedDB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	started := time.Now()
	result, err := db.raw.ExecContext(ctx, query, args...)
	rowsAffected := rowsAffectedFromResult(result)
	db.record(operationName(), statementKind(query), query, started, rowsAffected, err)
	return result, err
}

func (db *instrumentedDB) Exec(query string, args ...any) (sql.Result, error) {
	return db.ExecContext(context.Background(), query, args...)
}

func (db *instrumentedDB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	started := time.Now()
	rows, err := db.raw.QueryContext(ctx, query, args...)
	db.record(operationName(), statementKind(query), query, started, 0, err)
	return rows, err
}

func (db *instrumentedDB) Query(query string, args ...any) (*sql.Rows, error) {
	return db.QueryContext(context.Background(), query, args...)
}

func (db *instrumentedDB) QueryRowContext(ctx context.Context, query string, args ...any) RowScanner {
	started := time.Now()
	row := db.raw.QueryRowContext(ctx, query, args...)
	return &instrumentedRow{
		row:       row,
		monitor:   db.monitor,
		subsystem: db.subsystem,
		component: db.component,
		operation: operationName(),
		kind:      statementKind(query),
		query:     query,
		started:   started,
	}
}

func (db *instrumentedDB) QueryRow(query string, args ...any) RowScanner {
	return db.QueryRowContext(context.Background(), query, args...)
}

func (db *instrumentedDB) record(operation, kind, query string, started time.Time, rowsAffected int64, err error) {
	if db == nil || db.monitor == nil {
		return
	}
	db.monitor.Record(queryRecord{
		Operation:     operation,
		Subsystem:     db.subsystem,
		Component:     db.component,
		StatementKind: kind,
		Fingerprint:   fingerprintQuery(query),
		Duration:      time.Since(started),
		RowsAffected:  rowsAffected,
		Err:           err,
	})
}

func (tx *instrumentedTx) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	started := time.Now()
	result, err := tx.raw.ExecContext(ctx, query, args...)
	rowsAffected := rowsAffectedFromResult(result)
	tx.record(operationName(), statementKind(query), query, started, rowsAffected, err)
	return result, err
}

func (tx *instrumentedTx) Exec(query string, args ...any) (sql.Result, error) {
	return tx.ExecContext(context.Background(), query, args...)
}

func (tx *instrumentedTx) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	started := time.Now()
	rows, err := tx.raw.QueryContext(ctx, query, args...)
	tx.record(operationName(), statementKind(query), query, started, 0, err)
	return rows, err
}

func (tx *instrumentedTx) Query(query string, args ...any) (*sql.Rows, error) {
	return tx.QueryContext(context.Background(), query, args...)
}

func (tx *instrumentedTx) QueryRowContext(ctx context.Context, query string, args ...any) RowScanner {
	started := time.Now()
	row := tx.raw.QueryRowContext(ctx, query, args...)
	return &instrumentedRow{
		row:       row,
		monitor:   tx.monitor,
		subsystem: tx.subsystem,
		component: tx.component,
		operation: operationName(),
		kind:      statementKind(query),
		query:     query,
		started:   started,
	}
}

func (tx *instrumentedTx) QueryRow(query string, args ...any) RowScanner {
	return tx.QueryRowContext(context.Background(), query, args...)
}

func (row *instrumentedRow) Scan(dest ...any) error {
	err := row.row.Scan(dest...)
	row.record(0, err)
	return err
}

func (row *instrumentedRow) record(rowsAffected int64, err error) {
	if row == nil || row.recorded || row.monitor == nil {
		return
	}
	row.recorded = true
	row.monitor.Record(queryRecord{
		Operation:     row.operation,
		Subsystem:     row.subsystem,
		Component:     row.component,
		StatementKind: row.kind,
		Fingerprint:   fingerprintQuery(row.query),
		Duration:      time.Since(row.started),
		RowsAffected:  rowsAffected,
		Err:           err,
	})
}

func (tx *instrumentedTx) Commit() error {
	started := time.Now()
	err := tx.raw.Commit()
	tx.record("transaction.commit", "other", "", started, 0, err)
	return err
}

func (tx *instrumentedTx) Rollback() error {
	started := time.Now()
	err := tx.raw.Rollback()
	tx.record("transaction.rollback", "other", "", started, 0, err)
	return err
}

func (tx *instrumentedTx) record(operation, kind, query string, started time.Time, rowsAffected int64, err error) {
	if tx == nil || tx.monitor == nil {
		return
	}
	tx.monitor.Record(queryRecord{
		Operation:     operation,
		Subsystem:     tx.subsystem,
		Component:     tx.component,
		StatementKind: kind,
		Fingerprint:   fingerprintQuery(query),
		Duration:      time.Since(started),
		RowsAffected:  rowsAffected,
		Err:           err,
	})
}

func rowsAffectedFromResult(result sql.Result) int64 {
	if result == nil {
		return 0
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0
	}
	return count
}

func operationName() string {
	pcs := make([]uintptr, 16)
	n := runtime.Callers(3, pcs)
	frames := runtime.CallersFrames(pcs[:n])
	for {
		frame, more := frames.Next()
		if !strings.Contains(frame.Function, "internal/platform/store.") {
			return sanitizeOperationName(frame.Function)
		}
		if !more {
			break
		}
	}
	return "unknown"
}

func sanitizeOperationName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "unknown"
	}
	name = strings.TrimPrefix(name, "orbyte/")
	name = strings.TrimPrefix(name, "orbyte.")
	name = strings.ReplaceAll(name, "(*", "")
	name = strings.ReplaceAll(name, ")", "")
	name = strings.ReplaceAll(name, "/", ".")
	return name
}

func statementKind(query string) string {
	fields := strings.Fields(strings.ToLower(strings.TrimSpace(query)))
	if len(fields) == 0 {
		return "other"
	}
	switch fields[0] {
	case "select":
		return "select"
	case "insert":
		return "insert"
	case "update":
		return "update"
	case "delete":
		return "delete"
	default:
		return "other"
	}
}

func fingerprintQuery(query string) string {
	normalized := strings.ToLower(strings.TrimSpace(query))
	if normalized == "" {
		return ""
	}
	normalized = literalPattern.ReplaceAllString(normalized, "?")
	normalized = whitespacePattern.ReplaceAllString(normalized, " ")
	sum := sha1.Sum([]byte(normalized))
	return hex.EncodeToString(sum[:8])
}

func sanitizeMetricPart(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return "unknown"
	}
	var builder strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		default:
			builder.WriteByte('_')
		}
	}
	return strings.Trim(builder.String(), "_")
}

func (s QuerySnapshot) String() string {
	return fmt.Sprintf("QuerySnapshot{operations=%d slow=%d}", len(s.TopOperations), len(s.RecentSlow))
}
