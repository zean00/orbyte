package model

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"clinic/internal/platform/shared"
)

type Service struct {
	repo        Repository
	defaults    map[string]DefaultEvaluator
	computes    map[string]ComputeEvaluator
	constraints map[string]ConstraintEvaluator
}

func NewService() *Service {
	return NewServiceWithRepository(NewMemoryRepository())
}

func NewServiceWithRepository(repo Repository) *Service {
	return &Service{
		repo:        repo,
		defaults:    map[string]DefaultEvaluator{},
		computes:    map[string]ComputeEvaluator{},
		constraints: map[string]ConstraintEvaluator{},
	}
}

func (s *Service) WithRepository(repo Repository) *Service {
	if repo == nil {
		return s
	}
	return &Service{
		repo:        repo,
		defaults:    s.defaults,
		computes:    s.computes,
		constraints: s.constraints,
	}
}

func (s *Service) Repository() Repository {
	return s.repo
}

func (s *Service) WithRawRecordSave(record Record) error {
	return s.repo.SaveRecord(record)
}

func (s *Service) Register(def Definition) error {
	if strings.TrimSpace(def.Key) == "" || strings.TrimSpace(def.DisplayName) == "" {
		return shared.Validation("model key and display_name are required")
	}
	if _, exists := s.repo.GetDefinition(def.Key); exists {
		return shared.Conflict("model definition already exists")
	}
	for _, field := range def.Fields {
		if strings.TrimSpace(field.Key) == "" || strings.TrimSpace(field.Type) == "" {
			return shared.Validation("model field key and type are required")
		}
	}
	for _, relation := range def.Relations {
		if strings.TrimSpace(relation.Key) == "" || strings.TrimSpace(relation.Type) == "" || strings.TrimSpace(relation.TargetModelKey) == "" || strings.TrimSpace(relation.ForeignKey) == "" {
			return shared.Validation("model relation key, type, target_model_key, and foreign_key are required")
		}
	}
	return s.repo.SaveDefinition(def)
}

func (s *Service) Definitions() []Definition {
	return s.repo.ListDefinitions()
}

func (s *Service) Definition(key string) (Definition, bool) {
	return s.repo.GetDefinition(key)
}

func (s *Service) SetDefaultEvaluator(key string, eval DefaultEvaluator) {
	if strings.TrimSpace(key) == "" || eval == nil {
		return
	}
	s.defaults[key] = eval
}

func (s *Service) SetComputeEvaluator(key string, eval ComputeEvaluator) {
	if strings.TrimSpace(key) == "" || eval == nil {
		return
	}
	s.computes[key] = eval
}

func (s *Service) SetConstraintEvaluator(key string, eval ConstraintEvaluator) {
	if strings.TrimSpace(key) == "" || eval == nil {
		return
	}
	s.constraints[key] = eval
}

func (s *Service) Create(modelKey, actorID string, values map[string]any) (Record, error) {
	def, ok := s.repo.GetDefinition(strings.TrimSpace(modelKey))
	if !ok {
		return Record{}, shared.NotFound("model definition not found")
	}
	now := time.Now().UTC()
	record := Record{
		ModelKey:  def.Key,
		ID:        fmt.Sprintf("%s:%d", def.Key, now.UnixNano()),
		Version:   1,
		Values:    cloneMap(values),
		CreatedBy: fallbackActor(actorID),
		CreatedAt: now,
		UpdatedBy: fallbackActor(actorID),
		UpdatedAt: now,
	}
	if err := s.prepareRecord(def, &record, nil, actorID); err != nil {
		return Record{}, err
	}
	if err := s.repo.SaveRecord(record); err != nil {
		return Record{}, err
	}
	return record, nil
}

func (s *Service) Update(modelKey, id, actorID string, values map[string]any, expectedVersion int) (Record, error) {
	def, ok := s.repo.GetDefinition(strings.TrimSpace(modelKey))
	if !ok {
		return Record{}, shared.NotFound("model definition not found")
	}
	current, ok := s.repo.GetRecord(def.Key, strings.TrimSpace(id))
	if !ok {
		return Record{}, shared.NotFound("record not found")
	}
	if expectedVersion > 0 && current.Version != expectedVersion {
		return Record{}, shared.Conflict("record version mismatch")
	}
	record := current
	record.Values = cloneMap(values)
	record.Version++
	record.UpdatedBy = fallbackActor(actorID)
	record.UpdatedAt = time.Now().UTC()
	if err := s.prepareRecord(def, &record, &current, actorID); err != nil {
		return Record{}, err
	}
	if err := s.repo.SaveRecord(record); err != nil {
		return Record{}, err
	}
	return record, nil
}

func (s *Service) Get(modelKey, id string) (Record, error) {
	record, ok := s.repo.GetRecord(strings.TrimSpace(modelKey), strings.TrimSpace(id))
	if !ok {
		return Record{}, shared.NotFound("record not found")
	}
	return record, nil
}

func (s *Service) CreateRelated(modelKey, id, relationKey, actorID string, values map[string]any) (Record, error) {
	def, ok := s.repo.GetDefinition(strings.TrimSpace(modelKey))
	if !ok {
		return Record{}, shared.NotFound("model definition not found")
	}
	parent, ok := s.repo.GetRecord(def.Key, strings.TrimSpace(id))
	if !ok {
		return Record{}, shared.NotFound("parent record not found")
	}
	relation, ok := relationDefinition(def, relationKey)
	if !ok {
		return Record{}, shared.NotFound("model relation not found")
	}
	nextValues := cloneMap(values)
	nextValues[relation.ForeignKey] = parent.ID
	return s.Create(relation.TargetModelKey, actorID, nextValues)
}

func (s *Service) Related(modelKey, id, relationKey string, query Query) ([]Record, int, error) {
	def, ok := s.repo.GetDefinition(strings.TrimSpace(modelKey))
	if !ok {
		return nil, 0, shared.NotFound("model definition not found")
	}
	relation, ok := relationDefinition(def, relationKey)
	if !ok {
		return nil, 0, shared.NotFound("model relation not found")
	}
	if query.Filters == nil {
		query.Filters = map[string]string{}
	}
	query.Filters[relation.ForeignKey] = strings.TrimSpace(id)
	return s.List(relation.TargetModelKey, query)
}

func (s *Service) List(modelKey string, query Query) ([]Record, int, error) {
	def, ok := s.repo.GetDefinition(strings.TrimSpace(modelKey))
	if !ok {
		return nil, 0, shared.NotFound("model definition not found")
	}
	query, err := NormalizeQuery(def, query)
	if err != nil {
		return nil, 0, err
	}
	if repo, ok := s.repo.(interface {
		QueryRecords(def Definition, query Query) ([]Record, int, error)
	}); ok {
		return repo.QueryRecords(def, query)
	}
	items := s.repo.ListRecords(def.Key)
	filtered := make([]Record, 0, len(items))
	for _, item := range items {
		if !matchesFilters(item, query.Filters) {
			continue
		}
		filtered = append(filtered, item)
	}
	sortKey := strings.TrimSpace(query.SortKey)
	if sortKey == "" {
		sortKey = def.DefaultSort
	}
	sort.Slice(filtered, func(i, j int) bool {
		left := stringValue(resolveField(filtered[i].Values, sortKey))
		right := stringValue(resolveField(filtered[j].Values, sortKey))
		if sortKey == "" {
			left = filtered[i].ID
			right = filtered[j].ID
		}
		if query.Desc {
			return left > right
		}
		return left < right
	})
	total := len(filtered)
	pageSize := query.PageSize
	page := query.Page
	start := (page - 1) * pageSize
	if start >= len(filtered) {
		return []Record{}, total, nil
	}
	end := start + pageSize
	if end > len(filtered) {
		end = len(filtered)
	}
	return filtered[start:end], total, nil
}

func (s *Service) CreateComposite(modelKey, actorID string, mutation CompositeMutation) (Record, map[string][]Record, error) {
	parent, err := s.Create(modelKey, actorID, mutation.Values)
	if err != nil {
		return Record{}, nil, err
	}
	related, err := s.applyRelations(modelKey, parent.ID, actorID, mutation.Relations)
	if err != nil {
		_ = s.repo.DeleteRecord(modelKey, parent.ID)
		return Record{}, nil, err
	}
	return parent, related, nil
}

func (s *Service) UpdateComposite(modelKey, id, actorID string, mutation CompositeMutation) (Record, map[string][]Record, error) {
	parent, err := s.Update(modelKey, id, actorID, mutation.Values, mutation.ExpectedVersion)
	if err != nil {
		return Record{}, nil, err
	}
	related, err := s.applyRelations(modelKey, parent.ID, actorID, mutation.Relations)
	if err != nil {
		return Record{}, nil, err
	}
	return parent, related, nil
}

func (s *Service) applyRelations(modelKey, id, actorID string, relationMutations map[string][]ChildMutation) (map[string][]Record, error) {
	if len(relationMutations) == 0 {
		return map[string][]Record{}, nil
	}
	def, ok := s.repo.GetDefinition(strings.TrimSpace(modelKey))
	if !ok {
		return nil, shared.NotFound("model definition not found")
	}
	result := map[string][]Record{}
	for relationKey, mutations := range relationMutations {
		relation, ok := relationDefinition(def, relationKey)
		if !ok {
			return nil, shared.NotFound("model relation not found")
		}
		existing, _, err := s.Related(modelKey, id, relationKey, Query{Page: 1, PageSize: 1000})
		if err != nil {
			return nil, err
		}
		existingByID := map[string]Record{}
		for _, item := range existing {
			existingByID[item.ID] = item
		}
		applied := make([]Record, 0, len(mutations))
		for _, mutation := range mutations {
			op := strings.TrimSpace(mutation.Operation)
			if op == "" {
				op = "upsert"
			}
			if op == "delete" {
				if strings.TrimSpace(mutation.ID) == "" {
					continue
				}
				if _, ok := existingByID[strings.TrimSpace(mutation.ID)]; !ok {
					return nil, shared.NotFound("related record not found")
				}
				if err := s.repo.DeleteRecord(relation.TargetModelKey, strings.TrimSpace(mutation.ID)); err != nil {
					return nil, err
				}
				continue
			}
			values := cloneMap(mutation.Values)
			values[relation.ForeignKey] = id
			if strings.TrimSpace(mutation.ID) == "" {
				created, err := s.Create(relation.TargetModelKey, actorID, values)
				if err != nil {
					return nil, err
				}
				if _, err := s.applyRelations(relation.TargetModelKey, created.ID, actorID, mutation.Relations); err != nil {
					_ = s.repo.DeleteRecord(relation.TargetModelKey, created.ID)
					return nil, err
				}
				applied = append(applied, created)
				continue
			}
			current, ok := existingByID[strings.TrimSpace(mutation.ID)]
			if !ok {
				return nil, shared.NotFound("related record not found")
			}
			expected := mutation.ExpectedVersion
			if expected == 0 {
				expected = current.Version
			}
			updated, err := s.Update(relation.TargetModelKey, current.ID, actorID, values, expected)
			if err != nil {
				return nil, err
			}
			if _, err := s.applyRelations(relation.TargetModelKey, updated.ID, actorID, mutation.Relations); err != nil {
				return nil, err
			}
			applied = append(applied, updated)
		}
		result[relationKey] = applied
	}
	return result, nil
}

func (s *Service) prepareRecord(def Definition, record *Record, existing *Record, actorID string) error {
	if record.Values == nil {
		record.Values = map[string]any{}
	}
	for _, field := range def.Fields {
		if _, ok := record.Values[field.Key]; !ok && field.DefaultValue != nil {
			record.Values[field.Key] = field.DefaultValue
		}
		if _, ok := record.Values[field.Key]; !ok && strings.TrimSpace(field.DefaultRuleKey) != "" {
			if eval, ok := s.defaults[field.DefaultRuleKey]; ok {
				value, err := eval(RuleInput{ModelKey: def.Key, FieldKey: field.Key, Values: cloneMap(record.Values), ActorID: actorID})
				if err != nil {
					return err
				}
				record.Values[field.Key] = value
			}
		}
	}
	for _, field := range def.Fields {
		if strings.TrimSpace(field.ComputeRuleKey) == "" {
			continue
		}
		if eval, ok := s.computes[field.ComputeRuleKey]; ok {
			value, err := eval(RuleInput{ModelKey: def.Key, FieldKey: field.Key, Values: cloneMap(record.Values), Existing: existingValues(existing), ActorID: actorID})
			if err != nil {
				return err
			}
			record.Values[field.Key] = value
		}
	}
	for _, field := range def.Fields {
		if field.Required && strings.TrimSpace(stringValue(record.Values[field.Key])) == "" {
			return shared.Validation("required model field is missing: " + field.Key)
		}
		for _, key := range field.ConstraintRuleKeys {
			if eval, ok := s.constraints[key]; ok {
				if err := eval(RuleInput{ModelKey: def.Key, FieldKey: field.Key, Values: cloneMap(record.Values), Existing: existingValues(existing), ActorID: actorID}); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func relationDefinition(def Definition, relationKey string) (RelationDefinition, bool) {
	for _, relation := range def.Relations {
		if relation.Key == strings.TrimSpace(relationKey) {
			return relation, true
		}
	}
	return RelationDefinition{}, false
}

func matchesFilters(record Record, filters map[string]string) bool {
	for key, value := range filters {
		if strings.TrimSpace(value) == "" {
			continue
		}
		current := stringValue(resolveField(record.Values, key))
		candidate := strings.TrimSpace(value)
		if strings.EqualFold(strings.TrimSpace(key), "name") {
			if !strings.Contains(strings.ToLower(current), strings.ToLower(candidate)) {
				return false
			}
			continue
		}
		if !strings.EqualFold(current, candidate) {
			return false
		}
	}
	return true
}

func resolveField(values map[string]any, key string) any {
	if values == nil || strings.TrimSpace(key) == "" {
		return ""
	}
	return values[strings.TrimSpace(key)]
}

func existingValues(existing *Record) map[string]any {
	if existing == nil {
		return nil
	}
	return cloneMap(existing.Values)
}

func cloneMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return map[string]any{}
	}
	output := make(map[string]any, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func fallbackActor(actorID string) string {
	if strings.TrimSpace(actorID) == "" {
		return "system"
	}
	return strings.TrimSpace(actorID)
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case fmt.Stringer:
		return strings.TrimSpace(typed.String())
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case bool:
		if typed {
			return "true"
		}
		return "false"
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", value))
	}
}
