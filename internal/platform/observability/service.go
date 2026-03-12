package observability

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type Timing struct {
	Count         int64   `json:"count"`
	TotalMillis   int64   `json:"total_millis"`
	AverageMillis float64 `json:"average_millis"`
}

type Snapshot struct {
	Counters map[string]int64  `json:"counters"`
	Timings  map[string]Timing `json:"timings"`
	At       time.Time         `json:"at"`
}

type MetricDefinition struct {
	Key         string   `json:"key"`
	Type        string   `json:"type"`
	Labels      []string `json:"labels,omitempty"`
	Description string   `json:"description,omitempty"`
	ModuleKey   string   `json:"module_key,omitempty"`
}

type LogEventDefinition struct {
	Key            string   `json:"key"`
	Category       string   `json:"category,omitempty"`
	Severity       string   `json:"severity,omitempty"`
	RequiredFields []string `json:"required_fields,omitempty"`
	ModuleKey      string   `json:"module_key,omitempty"`
}

type DomainEventDefinition struct {
	Type                string `json:"type"`
	Role                string `json:"role,omitempty"`
	CorrelationRequired bool   `json:"correlation_required,omitempty"`
	ModuleKey           string `json:"module_key,omitempty"`
}

type ContractStatus struct {
	Key        string    `json:"key"`
	ModuleKey  string    `json:"module_key,omitempty"`
	Registered bool      `json:"registered"`
	Observed   int64     `json:"observed"`
	LastSeenAt time.Time `json:"last_seen_at,omitempty"`
	ValidationFailures int64 `json:"validation_failures,omitempty"`
	LastError          string `json:"last_error,omitempty"`
}

type Service struct {
	mu       sync.RWMutex
	counters map[string]int64
	timings  map[string]timingState
	metrics  map[string]MetricDefinition
	logs     map[string]LogEventDefinition
	events   map[string]DomainEventDefinition
	statuses map[string]ContractStatus
}

type timingState struct {
	count int64
	total time.Duration
}

func NewService() *Service {
	return &Service{
		counters: map[string]int64{},
		timings:  map[string]timingState{},
		metrics:  map[string]MetricDefinition{},
		logs:     map[string]LogEventDefinition{},
		events:   map[string]DomainEventDefinition{},
		statuses: map[string]ContractStatus{},
	}
}

func (s *Service) Inc(name string) {
	s.Add(name, 1)
}

func (s *Service) Add(name string, delta int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.counters[name] += delta
}

func (s *Service) Observe(name string, d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.timings[name]
	state.count++
	state.total += d
	s.timings[name] = state
}

func (s *Service) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	counters := make(map[string]int64, len(s.counters))
	for k, v := range s.counters {
		counters[k] = v
	}
	timings := make(map[string]Timing, len(s.timings))
	for k, v := range s.timings {
		avg := 0.0
		if v.count > 0 {
			avg = float64(v.total.Milliseconds()) / float64(v.count)
		}
		timings[k] = Timing{Count: v.count, TotalMillis: v.total.Milliseconds(), AverageMillis: avg}
	}
	return Snapshot{Counters: counters, Timings: timings, At: time.Now().UTC()}
}

func (s *Service) RegisterMetricDefinition(def MetricDefinition) {
	if def.Key == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.metrics[def.Key] = def
	s.statuses["metric:"+def.Key] = ContractStatus{Key: def.Key, ModuleKey: def.ModuleKey, Registered: true}
}

func (s *Service) RegisterLogEventDefinition(def LogEventDefinition) {
	if def.Key == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logs[def.Key] = def
	s.statuses["log:"+def.Key] = ContractStatus{Key: def.Key, ModuleKey: def.ModuleKey, Registered: true}
}

func (s *Service) RegisterDomainEventDefinition(def DomainEventDefinition) {
	if def.Type == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events[def.Type] = def
	s.statuses["event:"+def.Type] = ContractStatus{Key: def.Type, ModuleKey: def.ModuleKey, Registered: true}
}

func (s *Service) MetricDefinitions() []MetricDefinition {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]MetricDefinition, 0, len(s.metrics))
	for _, def := range s.metrics {
		items = append(items, def)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Key < items[j].Key })
	return items
}

func (s *Service) LogEventDefinitions() []LogEventDefinition {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]LogEventDefinition, 0, len(s.logs))
	for _, def := range s.logs {
		items = append(items, def)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Key < items[j].Key })
	return items
}

func (s *Service) DomainEventDefinitions() []DomainEventDefinition {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]DomainEventDefinition, 0, len(s.events))
	for _, def := range s.events {
		items = append(items, def)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Type < items[j].Type })
	return items
}

func (s *Service) ContractStatuses() []ContractStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]ContractStatus, 0, len(s.statuses))
	for _, status := range s.statuses {
		items = append(items, status)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Key < items[j].Key })
	return items
}

func (s *Service) RecordMetric(key string, labels map[string]string, delta int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	def, ok := s.metrics[key]
	if !ok {
		return s.metricFailure(key, "", fmt.Errorf("metric definition not registered: %s", key))
	}
	for _, required := range def.Labels {
		if _, ok := labels[required]; !ok {
			return s.metricFailure(key, def.ModuleKey, fmt.Errorf("metric label missing: %s", required))
		}
	}
	for provided := range labels {
		if !contains(def.Labels, provided) {
			return s.metricFailure(key, def.ModuleKey, fmt.Errorf("metric label unexpected: %s", provided))
		}
	}
	s.counters[key] += delta
	status := s.statuses["metric:"+key]
	status.Key = key
	status.ModuleKey = def.ModuleKey
	status.Registered = true
	status.Observed += delta
	status.LastSeenAt = time.Now().UTC()
	s.statuses["metric:"+key] = status
	return nil
}

func (s *Service) ObserveMetric(key string, labels map[string]string, d time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	def, ok := s.metrics[key]
	if !ok {
		return s.metricFailure(key, "", fmt.Errorf("metric definition not registered: %s", key))
	}
	if def.Type != "timing" {
		return s.metricFailure(key, def.ModuleKey, fmt.Errorf("metric is not a timing: %s", key))
	}
	for _, required := range def.Labels {
		if _, ok := labels[required]; !ok {
			return s.metricFailure(key, def.ModuleKey, fmt.Errorf("metric label missing: %s", required))
		}
	}
	for provided := range labels {
		if !contains(def.Labels, provided) {
			return s.metricFailure(key, def.ModuleKey, fmt.Errorf("metric label unexpected: %s", provided))
		}
	}
	state := s.timings[key]
	state.count++
	state.total += d
	s.timings[key] = state
	status := s.statuses["metric:"+key]
	status.Key = key
	status.ModuleKey = def.ModuleKey
	status.Registered = true
	status.Observed++
	status.LastSeenAt = time.Now().UTC()
	status.LastError = ""
	s.statuses["metric:"+key] = status
	return nil
}

func (s *Service) EmitLogEvent(key string, fields map[string]any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	def, ok := s.logs[key]
	if !ok {
		return s.logFailure(key, "", fmt.Errorf("log definition not registered: %s", key))
	}
	for _, required := range def.RequiredFields {
		if _, ok := fields[required]; !ok {
			return s.logFailure(key, def.ModuleKey, fmt.Errorf("log field missing: %s", required))
		}
	}
	status := s.statuses["log:"+key]
	status.Key = key
	status.ModuleKey = def.ModuleKey
	status.Registered = true
	status.Observed++
	status.LastSeenAt = time.Now().UTC()
	status.LastError = ""
	s.statuses["log:"+key] = status
	return nil
}

func (s *Service) RecordDomainEvent(eventType string, hasCorrelation bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	def, ok := s.events[eventType]
	if !ok {
		return s.eventFailure(eventType, "", fmt.Errorf("domain event definition not registered: %s", eventType))
	}
	if def.CorrelationRequired && !hasCorrelation {
		return s.eventFailure(eventType, def.ModuleKey, fmt.Errorf("domain event correlation is required: %s", eventType))
	}
	status := s.statuses["event:"+eventType]
	status.Key = eventType
	status.ModuleKey = def.ModuleKey
	status.Registered = true
	status.Observed++
	status.LastSeenAt = time.Now().UTC()
	status.LastError = ""
	s.statuses["event:"+eventType] = status
	return nil
}

func (s *Service) metricFailure(key, moduleKey string, err error) error {
	status := s.statuses["metric:"+key]
	status.Key = key
	status.ModuleKey = moduleKey
	status.ValidationFailures++
	status.LastError = err.Error()
	s.statuses["metric:"+key] = status
	return err
}

func (s *Service) logFailure(key, moduleKey string, err error) error {
	status := s.statuses["log:"+key]
	status.Key = key
	status.ModuleKey = moduleKey
	status.ValidationFailures++
	status.LastError = err.Error()
	s.statuses["log:"+key] = status
	return err
}

func (s *Service) eventFailure(key, moduleKey string, err error) error {
	status := s.statuses["event:"+key]
	status.Key = key
	status.ModuleKey = moduleKey
	status.ValidationFailures++
	status.LastError = err.Error()
	s.statuses["event:"+key] = status
	return err
}

func contains(items []string, candidate string) bool {
	for _, item := range items {
		if item == candidate {
			return true
		}
	}
	return false
}

func (s *Service) RenderPrometheus() string {
	snap := s.Snapshot()
	lines := make([]string, 0, len(snap.Counters)+len(snap.Timings)*3)
	keys := make([]string, 0, len(snap.Counters))
	for k := range snap.Counters {
		keys = append(keys, sanitizeKey(k))
	}
	sort.Strings(keys)
	for _, key := range keys {
		original := unsanitizeLookup(snap.Counters, key)
		lines = append(lines, fmt.Sprintf("%s %d", key, snap.Counters[original]))
	}
	timingKeys := make([]string, 0, len(snap.Timings))
	for k := range snap.Timings {
		timingKeys = append(timingKeys, sanitizeKey(k))
	}
	sort.Strings(timingKeys)
	for _, key := range timingKeys {
		original := unsanitizeTimingLookup(snap.Timings, key)
		value := snap.Timings[original]
		lines = append(lines, fmt.Sprintf("%s_count %d", key, value.Count))
		lines = append(lines, fmt.Sprintf("%s_total_millis %d", key, value.TotalMillis))
		lines = append(lines, fmt.Sprintf("%s_average_millis %.2f", key, value.AverageMillis))
	}
	return strings.Join(lines, "\n") + "\n"
}

func sanitizeKey(name string) string {
	replacer := strings.NewReplacer(".", "_", "-", "_", "/", "_")
	return replacer.Replace(name)
}

func unsanitizeLookup(values map[string]int64, sanitized string) string {
	for k := range values {
		if sanitizeKey(k) == sanitized {
			return k
		}
	}
	return sanitized
}

func unsanitizeTimingLookup(values map[string]Timing, sanitized string) string {
	for k := range values {
		if sanitizeKey(k) == sanitized {
			return k
		}
	}
	return sanitized
}
