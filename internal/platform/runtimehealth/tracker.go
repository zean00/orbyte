package runtimehealth

import (
	"context"
	"sync"
	"time"
)

const degradeAfterFailures = 3

type Checker func(context.Context) error
type DBStatsProvider func() *DBStats

type Tracker struct {
	mu                sync.RWMutex
	bootstrapped      bool
	backgroundStarted bool
	shuttingDown      bool
	checker           Checker
	dbStatsProvider   DBStatsProvider
	subsystems        map[string]SubsystemStatus
}

type SubsystemStatus struct {
	Name                string    `json:"name"`
	Status              string    `json:"status"`
	ConsecutiveFailures int       `json:"consecutive_failures"`
	LastSuccessAt       time.Time `json:"last_success_at,omitempty"`
	LastFailureAt       time.Time `json:"last_failure_at,omitempty"`
	LastError           string    `json:"last_error,omitempty"`
}

type Snapshot struct {
	Live              bool              `json:"live"`
	Ready             bool              `json:"ready"`
	Bootstrapped      bool              `json:"bootstrapped"`
	BackgroundStarted bool              `json:"background_started"`
	ShuttingDown      bool              `json:"shutting_down"`
	DependencyOK      bool              `json:"dependency_ok"`
	DependencyError   string            `json:"dependency_error,omitempty"`
	Database          *DBStats          `json:"database,omitempty"`
	Subsystems        []SubsystemStatus `json:"subsystems"`
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
		subsystems: map[string]SubsystemStatus{},
	}
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

func (t *Tracker) Snapshot(ctx context.Context) Snapshot {
	if t == nil {
		return Snapshot{Live: true, Ready: true, DependencyOK: true}
	}
	t.mu.RLock()
	bootstrapped := t.bootstrapped
	backgroundStarted := t.backgroundStarted
	shuttingDown := t.shuttingDown
	checker := t.checker
	dbStatsProvider := t.dbStatsProvider
	subsystems := make([]SubsystemStatus, 0, len(t.subsystems))
	ready := bootstrapped && backgroundStarted && !shuttingDown
	for _, item := range t.subsystems {
		subsystems = append(subsystems, item)
		if item.Status == "degraded" {
			ready = false
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
		}
	}
	var dbStats *DBStats
	if dbStatsProvider != nil {
		dbStats = dbStatsProvider()
	}
	return Snapshot{
		Live:              true,
		Ready:             ready,
		Bootstrapped:      bootstrapped,
		BackgroundStarted: backgroundStarted,
		ShuttingDown:      shuttingDown,
		DependencyOK:      dependencyOK,
		DependencyError:   dependencyError,
		Database:          dbStats,
		Subsystems:        subsystems,
	}
}
