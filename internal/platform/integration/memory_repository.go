package integration

import (
	"fmt"
	"sort"
)

type MemoryRepository struct {
	systems     map[string]ExternalSystem
	endpoints   map[string]Endpoint
	contracts   map[string]Contract
	mappings    map[string]Mapping
	submissions map[string]SubmissionRecord
	attempts    map[string][]SubmissionAttempt
	deadLetters map[string]DeadLetterRecord
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		systems:     map[string]ExternalSystem{},
		endpoints:   map[string]Endpoint{},
		contracts:   map[string]Contract{},
		mappings:    map[string]Mapping{},
		submissions: map[string]SubmissionRecord{},
		attempts:    map[string][]SubmissionAttempt{},
		deadLetters: map[string]DeadLetterRecord{},
	}
}

func (r *MemoryRepository) SaveSystem(system ExternalSystem) error {
	if system.Settings == nil {
		system.Settings = map[string]any{}
	}
	r.systems[system.Key] = system
	return nil
}

func (r *MemoryRepository) SaveEndpoint(endpoint Endpoint) error {
	if endpoint.Settings == nil {
		endpoint.Settings = map[string]any{}
	}
	r.endpoints[endpoint.Key] = endpoint
	return nil
}

func (r *MemoryRepository) ListEndpoints() []Endpoint {
	items := make([]Endpoint, 0, len(r.endpoints))
	for _, item := range r.endpoints {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Key < items[j].Key })
	return items
}

func (r *MemoryRepository) GetEndpoint(key string) (Endpoint, bool) {
	item, ok := r.endpoints[key]
	return item, ok
}

func (r *MemoryRepository) SaveContract(contract Contract) error {
	if contract.Schema == nil {
		contract.Schema = map[string]any{}
	}
	r.contracts[contractKey(contract.Key, contract.Version)] = contract
	return nil
}

func (r *MemoryRepository) ListContracts() []Contract {
	items := make([]Contract, 0, len(r.contracts))
	for _, item := range r.contracts {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Key == items[j].Key {
			return items[i].Version < items[j].Version
		}
		return items[i].Key < items[j].Key
	})
	return items
}

func (r *MemoryRepository) GetContract(key string, version int) (Contract, bool) {
	item, ok := r.contracts[contractKey(key, version)]
	return item, ok
}

func (r *MemoryRepository) SaveMapping(mapping Mapping) error {
	if mapping.Rule == nil {
		mapping.Rule = map[string]any{}
	}
	r.mappings[mapping.Key] = mapping
	return nil
}

func (r *MemoryRepository) ListMappings() []Mapping {
	items := make([]Mapping, 0, len(r.mappings))
	for _, item := range r.mappings {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Key < items[j].Key })
	return items
}

func (r *MemoryRepository) ListSystems() []ExternalSystem {
	items := make([]ExternalSystem, 0, len(r.systems))
	for _, item := range r.systems {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Key < items[j].Key })
	return items
}

func (r *MemoryRepository) GetSystem(key string) (ExternalSystem, bool) {
	item, ok := r.systems[key]
	return item, ok
}

func (r *MemoryRepository) SaveSubmission(record SubmissionRecord) error {
	if record.Payload == nil {
		record.Payload = map[string]any{}
	}
	if record.Result == nil {
		record.Result = map[string]any{}
	}
	r.submissions[record.ID] = record
	return nil
}

func (r *MemoryRepository) ListSubmissions() []SubmissionRecord {
	items := make([]SubmissionRecord, 0, len(r.submissions))
	for _, item := range r.submissions {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.Before(items[j].CreatedAt) })
	return items
}

func (r *MemoryRepository) GetSubmission(id string) (SubmissionRecord, bool) {
	item, ok := r.submissions[id]
	return item, ok
}

func (r *MemoryRepository) FindSubmissionByIdempotency(externalSystemKey, endpointKey, contractKey, idempotencyKey string) (SubmissionRecord, bool) {
	for _, item := range r.submissions {
		if item.ExternalSystemKey == externalSystemKey && item.EndpointKey == endpointKey && item.ContractKey == contractKey && item.IdempotencyKey == idempotencyKey {
			return item, true
		}
	}
	return SubmissionRecord{}, false
}

func (r *MemoryRepository) SaveSubmissionAttempt(attempt SubmissionAttempt) error {
	if attempt.Request == nil {
		attempt.Request = map[string]any{}
	}
	if attempt.Response == nil {
		attempt.Response = map[string]any{}
	}
	r.attempts[attempt.SubmissionID] = append(r.attempts[attempt.SubmissionID], attempt)
	sort.Slice(r.attempts[attempt.SubmissionID], func(i, j int) bool {
		if r.attempts[attempt.SubmissionID][i].Attempt == r.attempts[attempt.SubmissionID][j].Attempt {
			return r.attempts[attempt.SubmissionID][i].ID < r.attempts[attempt.SubmissionID][j].ID
		}
		return r.attempts[attempt.SubmissionID][i].Attempt < r.attempts[attempt.SubmissionID][j].Attempt
	})
	return nil
}

func (r *MemoryRepository) ListSubmissionAttempts(submissionID string) []SubmissionAttempt {
	items := append([]SubmissionAttempt(nil), r.attempts[submissionID]...)
	sort.Slice(items, func(i, j int) bool {
		if items[i].Attempt == items[j].Attempt {
			return items[i].ID < items[j].ID
		}
		return items[i].Attempt < items[j].Attempt
	})
	return items
}

func (r *MemoryRepository) SaveDeadLetter(record DeadLetterRecord) error {
	if record.Payload == nil {
		record.Payload = map[string]any{}
	}
	r.deadLetters[record.ID] = record
	return nil
}

func (r *MemoryRepository) ListDeadLetters() []DeadLetterRecord {
	items := make([]DeadLetterRecord, 0, len(r.deadLetters))
	for _, item := range r.deadLetters {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.Before(items[j].CreatedAt) })
	return items
}

func (r *MemoryRepository) GetDeadLetter(id string) (DeadLetterRecord, bool) {
	item, ok := r.deadLetters[id]
	return item, ok
}

func contractKey(key string, version int) string {
	return fmt.Sprintf("%s:%d", key, version)
}
