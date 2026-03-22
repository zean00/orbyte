package runtimehealth

import (
	"context"
	"sync"
	"time"
)

const degradeAfterFailures = 3

type Checker func(context.Context) error
type DBStatsProvider func() *DBStats

type SubsystemConfig struct {
	FailureCategory  string
	RunbookID        string
	OperatorHint     string
	ImpactsReadiness bool
}

type Tracker struct {
	mu                sync.RWMutex
	bootstrapped      bool
	backgroundStarted bool
	shuttingDown      bool
	checker           Checker
	dbStatsProvider   DBStatsProvider
	subsystems        map[string]SubsystemStatus
	subsystemConfig   map[string]SubsystemConfig
}

type SubsystemStatus struct {
	Name                string    `json:"name"`
	Status              string    `json:"status"`
	FailureCategory     string    `json:"failure_category,omitempty"`
	RunbookID           string    `json:"runbook_id,omitempty"`
	OperatorHint        string    `json:"operator_hint,omitempty"`
	ImpactsReadiness    bool      `json:"impacts_readiness"`
	ConsecutiveFailures int       `json:"consecutive_failures"`
	LastSuccessAt       time.Time `json:"last_success_at,omitempty"`
	LastFailureAt       time.Time `json:"last_failure_at,omitempty"`
	LastError           string    `json:"last_error,omitempty"`
}

type Snapshot struct {
	Status              string            `json:"status"`
	Live                bool              `json:"live"`
	Ready               bool              `json:"ready"`
	Bootstrapped        bool              `json:"bootstrapped"`
	BackgroundStarted   bool              `json:"background_started"`
	ShuttingDown        bool              `json:"shutting_down"`
	DependencyOK        bool              `json:"dependency_ok"`
	DependencyCategory  string            `json:"dependency_category,omitempty"`
	DependencyHint      string            `json:"dependency_hint,omitempty"`
	DependencyRunbookID string            `json:"dependency_runbook_id,omitempty"`
	OperatorHint        string            `json:"operator_hint,omitempty"`
	RunbookIDs          []string          `json:"runbook_ids,omitempty"`
	FailureCategories   []string          `json:"failure_categories,omitempty"`
	DependencyError     string            `json:"dependency_error,omitempty"`
	Database            *DBStats          `json:"database,omitempty"`
	Subsystems          []SubsystemStatus `json:"subsystems"`
}

type DBStats struct {
	MaxOpenConnections int   `json:"max_open_connections"`
	OpenConnections    int   `json:"open_connections"`
	InUse              int   `json:"in_use"`
	Idle               int   `json:"idle"`
	WaitCount          int64 `json:"wait_count"`
	WaitDurationMillis int64 `json:"wait_duration_millis"`
	MaxIdleClosed      int64 `json:"max_idle_closed"`
	MaxIdleTimeClosed  int64 `json:"max_idle_time_closed"`
	MaxLifetimeClosed  int64 `json:"max_lifetime_closed"`
}

func NewTracker() *Tracker {
	return &Tracker{
		subsystems:      map[string]SubsystemStatus{},
		subsystemConfig: map[string]SubsystemConfig{},
	}
}

func (t *Tracker) ConfigureSubsystem(name string, cfg SubsystemConfig) {
	if t == nil || name == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.subsystemConfig[name] = cfg
	item := t.subsystems[name]
	item.Name = name
	item.FailureCategory = cfg.FailureCategory
	item.RunbookID = cfg.RunbookID
	item.OperatorHint = cfg.OperatorHint
	item.ImpactsReadiness = cfg.ImpactsReadiness
	t.subsystems[name] = item
}

func (t *Tracker) SetChecker(checker Checker) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.checker = checker
}

func (t *Tracker) SetDBStatsProvider(provider DBStatsProvider) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.dbStatsProvider = provider
}

func (t *Tracker) SetBootstrapped(ready bool) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.bootstrapped = ready
}

func (t *Tracker) SetBackgroundStarted(started bool) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.backgroundStarted = started
}

func (t *Tracker) SetShuttingDown(shuttingDown bool) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.shuttingDown = shuttingDown
}

func (t *Tracker) MarkSuccess(name string) {
	if t == nil || name == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	item := t.subsystems[name]
	item.Name = name
	item.Status = "healthy"
	t.applyConfig(name, &item)
	item.ConsecutiveFailures = 0
	item.LastError = ""
	item.LastSuccessAt = time.Now().UTC()
	t.subsystems[name] = item
}

func (t *Tracker) MarkFailure(name string, err error) {
	if t == nil || name == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	item := t.subsystems[name]
	item.Name = name
	t.applyConfig(name, &item)
	item.ConsecutiveFailures++
	item.LastFailureAt = time.Now().UTC()
	if err != nil {
		item.LastError = err.Error()
	}
	if item.ConsecutiveFailures >= degradeAfterFailures {
		item.Status = "degraded"
	} else {
		item.Status = "starting"
	}
	t.subsystems[name] = item
}

func (t *Tracker) applyConfig(name string, item *SubsystemStatus) {
	cfg, ok := t.subsystemConfig[name]
	if !ok {
		item.ImpactsReadiness = true
		if item.FailureCategory == "" {
			item.FailureCategory = name
		}
		return
	}
	item.FailureCategory = cfg.FailureCategory
	item.RunbookID = cfg.RunbookID
	item.OperatorHint = cfg.OperatorHint
	item.ImpactsReadiness = cfg.ImpactsReadiness
}

func (t *Tracker) Snapshot(ctx context.Context) Snapshot {
	if t == nil {
		return Snapshot{Status: "healthy", Live: true, Ready: true, DependencyOK: true}
	}
	t.mu.RLock()
	bootstrapped := t.bootstrapped
	backgroundStarted := t.backgroundStarted
	shuttingDown := t.shuttingDown
	checker := t.checker
	dbStatsProvider := t.dbStatsProvider
	subsystems := make([]SubsystemStatus, 0, len(t.subsystems))
	ready := bootstrapped && backgroundStarted && !shuttingDown
	categories := map[string]struct{}{}
	runbooks := map[string]struct{}{}
	operatorHint := ""
	for _, item := range t.subsystems {
		if item.Status == "" {
			item.Status = "idle"
		}
		subsystems = append(subsystems, item)
		if item.Status == "degraded" && item.ImpactsReadiness {
			ready = false
		}
		if item.Status == "degraded" || item.Status == "starting" {
			if item.FailureCategory != "" {
				categories[item.FailureCategory] = struct{}{}
			}
			if item.RunbookID != "" {
				runbooks[item.RunbookID] = struct{}{}
			}
			if operatorHint == "" && item.OperatorHint != "" {
				operatorHint = item.OperatorHint
			}
		}
	}
	t.mu.RUnlock()

	dependencyOK := true
	dependencyError := ""
	if checker != nil {
		if err := checker(ctx); err != nil {
			dependencyOK = false
			dependencyError = err.Error()
			ready = false
			categories["dependency_unavailable"] = struct{}{}
			runbooks["runtime.dependencies"] = struct{}{}
			if operatorHint == "" {
				operatorHint = "Check database and runtime dependencies before re-enabling readiness."
			}
		}
	}
	var dbStats *DBStats
	if dbStatsProvider != nil {
		dbStats = dbStatsProvider()
	}
	status := "healthy"
	switch {
	case shuttingDown:
		status = "failed"
	case !bootstrapped || !backgroundStarted:
		status = "starting"
	case !ready:
		status = "degraded"
	}
	categoryItems := make([]string, 0, len(categories))
	for key := range categories {
		categoryItems = append(categoryItems, key)
	}
	runbookItems := make([]string, 0, len(runbooks))
	for key := range runbooks {
		runbookItems = append(runbookItems, key)
	}
	return Snapshot{
		Status:              status,
		Live:                true,
		Ready:               ready,
		Bootstrapped:        bootstrapped,
		BackgroundStarted:   backgroundStarted,
		ShuttingDown:        shuttingDown,
		DependencyOK:        dependencyOK,
		DependencyCategory:  firstNonEmptyCategory(dependencyOK),
		DependencyHint:      firstNonEmptyHint(dependencyOK),
		DependencyRunbookID: firstNonEmptyRunbook(dependencyOK),
		OperatorHint:        operatorHint,
		RunbookIDs:          runbookItems,
		FailureCategories:   categoryItems,
		DependencyError:     dependencyError,
		Database:            dbStats,
		Subsystems:          subsystems,
	}
}

func firstNonEmptyCategory(ok bool) string {
	if ok {
		return ""
	}
	return "dependency_unavailable"
}

func firstNonEmptyHint(ok bool) string {
	if ok {
		return ""
	}
	return "Check the primary datastore and runtime dependencies."
}

func firstNonEmptyRunbook(ok bool) string {
	if ok {
		return ""
	}
	return "runtime.dependencies"
}
