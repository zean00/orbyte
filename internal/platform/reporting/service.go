package reporting

import (
	"fmt"
	"sort"
	"strings"

	"orbyte/internal/platform/document"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/search"
	"orbyte/internal/platform/securityfields"
	"orbyte/internal/platform/shared"
)

type DatasetDefinition struct {
	Key        string                `json:"key"`
	Title      string                `json:"title"`
	SourceKind string                `json:"source_kind"`
	ModelKey   string                `json:"model_key,omitempty"`
	Dimensions []DimensionDefinition `json:"dimensions,omitempty"`
	Measures   []MeasureDefinition   `json:"measures,omitempty"`
}

type DimensionDefinition struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Path  string `json:"path"`
}

type MeasureDefinition struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Kind  string `json:"kind"`
	Path  string `json:"path,omitempty"`
}

type QueryRequest struct {
	ModelKey   string   `json:"model_key,omitempty"`
	Dimensions []string `json:"dimensions,omitempty"`
	Measures   []string `json:"measures,omitempty"`
	GroupBy    []string `json:"group_by,omitempty"`
	SortBy     string   `json:"sort_by,omitempty"`
	Desc       bool     `json:"desc,omitempty"`
	Limit      int      `json:"limit,omitempty"`
}

const (
	maxSelectedDimensions = 8
	maxSelectedMeasures   = 8
	maxGroupLimit         = 200
)

type Service struct {
	models    *model.Service
	documents *document.Service
	search    *search.Service
	fields    *securityfields.Service
	datasets  map[string]DatasetDefinition
}

func NewService(models *model.Service) *Service {
	return &Service{models: models, datasets: map[string]DatasetDefinition{}}
}

func (s *Service) AttachDocumentSources(documents *document.Service, searchSvc *search.Service) {
	s.documents = documents
	s.search = searchSvc
}

func (s *Service) AttachFieldSecurity(fields *securityfields.Service) {
	s.fields = fields
}

func (s *Service) Register(def DatasetDefinition) error {
	if strings.TrimSpace(def.Key) == "" || strings.TrimSpace(def.Title) == "" || strings.TrimSpace(def.SourceKind) == "" {
		return shared.Validation("dataset key, title, and source_kind are required")
	}
	if _, exists := s.datasets[def.Key]; exists {
		return shared.Conflict("dataset definition already exists")
	}
	s.datasets[def.Key] = def
	return nil
}

func (s *Service) Definitions() []DatasetDefinition {
	items := make([]DatasetDefinition, 0, len(s.datasets))
	for _, item := range s.datasets {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Key < items[j].Key })
	return items
}

func (s *Service) Definition(key string) (DatasetDefinition, bool) {
	item, ok := s.datasets[strings.TrimSpace(key)]
	return item, ok
}

func (s *Service) Execute(key string) (map[string]any, error) {
	return s.ExecuteView(key, model.Query{Page: 1, PageSize: 1000}, QueryRequest{})
}

func (s *Service) ExecuteQuery(key string, query model.Query) (map[string]any, error) {
	return s.ExecuteView(key, query, QueryRequest{})
}

func (s *Service) ExecuteView(key string, query model.Query, view QueryRequest) (map[string]any, error) {
	view, err := boundedQueryRequest(view)
	if err != nil {
		return nil, err
	}
	def, ok := s.datasets[strings.TrimSpace(key)]
	if !ok {
		return nil, shared.NotFound("dataset definition not found")
	}
	switch def.SourceKind {
	case "model":
		if query.PageSize <= 0 {
			query.Page = 1
			query.PageSize = model.MaxPageSize
		}
		if query.PageSize > model.MaxPageSize {
			query.PageSize = model.MaxPageSize
		}
		items, total, err := s.models.List(def.ModelKey, query)
		if err != nil {
			return nil, err
		}
		dimensions := selectDimensions(def.Dimensions, view.Dimensions)
		measures := selectMeasures(def.Measures, view.Measures)
		groupBy := dimensions
		if len(view.GroupBy) > 0 {
			groupBy = selectDimensions(def.Dimensions, view.GroupBy)
		}
		return buildResult(def.Key, def.Title, total, modelRows(items), dimensions, measures, groupBy, view), nil
	default:
		return nil, shared.Validation("unsupported dataset source_kind")
	}
}

func (s *Service) ExecuteAdHocModel(query model.Query, view QueryRequest) (map[string]any, error) {
	var err error
	view, err = boundedQueryRequest(view)
	if err != nil {
		return nil, err
	}
	modelKey := strings.TrimSpace(view.ModelKey)
	if modelKey == "" {
		return nil, shared.Validation("model_key is required")
	}
	def, ok := s.models.Definition(modelKey)
	if !ok {
		return nil, shared.NotFound("model definition not found")
	}
	dimensions := make([]DimensionDefinition, 0, len(view.Dimensions))
	if len(view.Dimensions) == 0 {
		for _, field := range def.Fields {
			dimensions = append(dimensions, DimensionDefinition{Key: field.Key, Label: field.Label, Path: field.Key})
		}
	} else {
		for _, key := range view.Dimensions {
			key = strings.TrimSpace(key)
			if key == "" {
				continue
			}
			dimensions = append(dimensions, DimensionDefinition{Key: key, Label: key, Path: key})
		}
	}
	measures := make([]MeasureDefinition, 0, len(view.Measures))
	for _, key := range view.Measures {
		parts := strings.Split(strings.TrimSpace(key), ":")
		if len(parts) == 1 {
			measures = append(measures, MeasureDefinition{Key: parts[0], Label: parts[0], Kind: "count"})
			continue
		}
		measures = append(measures, MeasureDefinition{Key: parts[0] + "_" + parts[1], Label: parts[0] + " " + parts[1], Kind: parts[0], Path: parts[1]})
	}
	if len(measures) == 0 {
		measures = []MeasureDefinition{{Key: "total", Label: "Total", Kind: "count"}}
	}
	definition := DatasetDefinition{
		Key:        "adhoc:" + modelKey,
		Title:      "Ad Hoc " + def.DisplayName,
		SourceKind: "model",
		ModelKey:   modelKey,
		Dimensions: dimensions,
		Measures:   measures,
	}
	clone := *s
	clone.datasets = map[string]DatasetDefinition{definition.Key: definition}
	return clone.ExecuteView(definition.Key, query, view)
}

func (s *Service) ExecuteAdHocSource(source string, query model.Query, view QueryRequest) (map[string]any, error) {
	var err error
	view, err = boundedQueryRequest(view)
	if err != nil {
		return nil, err
	}
	source = strings.TrimSpace(source)
	switch {
	case strings.HasPrefix(source, "models/"):
		view.ModelKey = strings.TrimSpace(strings.TrimPrefix(source, "models/"))
		return s.ExecuteAdHocModel(query, view)
	case source == "documents":
		if s.documents == nil {
			return nil, shared.Validation("document source is not configured")
		}
		rows := documentRows(s.documents.List(), query, s.fields)
		definition := DatasetDefinition{
			Key:        "adhoc:documents",
			Title:      "Ad Hoc Documents",
			SourceKind: "documents",
			Dimensions: chooseDimensions(view.Dimensions, []string{"header.type", "header.status", "header.organization_id", "header.location_id", "header.number"}),
			Measures:   chooseMeasures(view.Measures),
		}
		groupBy := selectDimensions(definition.Dimensions, view.GroupBy)
		if len(view.GroupBy) == 0 {
			groupBy = definition.Dimensions
		}
		return buildResult(definition.Key, definition.Title, len(rows), rows, definition.Dimensions, definition.Measures, groupBy, view), nil
	case source == "document_projections":
		if s.search == nil {
			return nil, shared.Validation("projection source is not configured")
		}
		rows := projectionRows(s.search.ListDocuments(), query)
		definition := DatasetDefinition{
			Key:        "adhoc:document_projections",
			Title:      "Ad Hoc Document Projections",
			SourceKind: "document_projections",
			Dimensions: chooseDimensions(view.Dimensions, []string{"document_type", "status", "organization_id", "location_id"}),
			Measures:   chooseMeasures(view.Measures),
		}
		groupBy := selectDimensions(definition.Dimensions, view.GroupBy)
		if len(view.GroupBy) == 0 {
			groupBy = definition.Dimensions
		}
		return buildResult(definition.Key, definition.Title, len(rows), rows, definition.Dimensions, definition.Measures, groupBy, view), nil
	default:
		return nil, shared.NotFound("reporting source not found")
	}
}

func boundedQueryRequest(view QueryRequest) (QueryRequest, error) {
	view.Dimensions = uniqueTrimmed(view.Dimensions)
	view.Measures = uniqueTrimmed(view.Measures)
	view.GroupBy = uniqueTrimmed(view.GroupBy)
	if len(view.Dimensions) > maxSelectedDimensions {
		return QueryRequest{}, shared.Validation("too many dimensions requested")
	}
	if len(view.Measures) > maxSelectedMeasures {
		return QueryRequest{}, shared.Validation("too many measures requested")
	}
	if len(view.GroupBy) > maxSelectedDimensions {
		return QueryRequest{}, shared.Validation("too many group_by dimensions requested")
	}
	if view.Limit < 0 {
		return QueryRequest{}, shared.Validation("limit must be positive")
	}
	if view.Limit > maxGroupLimit {
		view.Limit = maxGroupLimit
	}
	return view, nil
}

func uniqueTrimmed(items []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	return out
}

func selectDimensions(all []DimensionDefinition, keys []string) []DimensionDefinition {
	if len(keys) == 0 {
		return append([]DimensionDefinition(nil), all...)
	}
	index := map[string]DimensionDefinition{}
	for _, item := range all {
		index[item.Key] = item
	}
	out := make([]DimensionDefinition, 0, len(keys))
	for _, key := range keys {
		if item, ok := index[strings.TrimSpace(key)]; ok {
			out = append(out, item)
		}
	}
	return out
}

func selectMeasures(all []MeasureDefinition, keys []string) []MeasureDefinition {
	if len(keys) == 0 {
		return append([]MeasureDefinition(nil), all...)
	}
	index := map[string]MeasureDefinition{}
	for _, item := range all {
		index[item.Key] = item
	}
	out := make([]MeasureDefinition, 0, len(keys))
	for _, key := range keys {
		if item, ok := index[strings.TrimSpace(key)]; ok {
			out = append(out, item)
		}
	}
	return out
}

func chooseDimensions(keys, defaults []string) []DimensionDefinition {
	selected := defaults
	if len(keys) > 0 {
		selected = keys
	}
	out := make([]DimensionDefinition, 0, len(selected))
	for _, key := range selected {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		out = append(out, DimensionDefinition{Key: key, Label: key, Path: key})
	}
	return out
}

func chooseMeasures(keys []string) []MeasureDefinition {
	if len(keys) == 0 {
		return []MeasureDefinition{{Key: "total", Label: "Total", Kind: "count"}}
	}
	out := make([]MeasureDefinition, 0, len(keys))
	for _, key := range keys {
		parts := strings.Split(strings.TrimSpace(key), ":")
		if len(parts) == 1 {
			out = append(out, MeasureDefinition{Key: parts[0], Label: parts[0], Kind: "count"})
			continue
		}
		out = append(out, MeasureDefinition{Key: parts[0] + "_" + parts[1], Label: parts[0] + " " + parts[1], Kind: parts[0], Path: parts[1]})
	}
	return out
}

func sortDatasetRows(rows []map[string]any, sortBy string, desc bool) {
	key := strings.TrimSpace(sortBy)
	if key == "" {
		return
	}
	sort.Slice(rows, func(i, j int) bool {
		leftRaw := rows[i][key]
		rightRaw := rows[j][key]
		switch leftRaw.(type) {
		case int, int64, float32, float64:
			left := numericValue(leftRaw)
			right := numericValue(rightRaw)
			if desc {
				return left > right
			}
			return left < right
		}
		left := modelValue(leftRaw)
		right := modelValue(rightRaw)
		if desc {
			return left > right
		}
		return left < right
	})
}

func buildResult(datasetKey, title string, total int, rows []map[string]any, dimensions []DimensionDefinition, measures []MeasureDefinition, groupBy []DimensionDefinition, view QueryRequest) map[string]any {
	out := map[string]any{
		"dataset_key":         datasetKey,
		"title":               title,
		"total":               total,
		"selected_dimensions": dimensions,
		"selected_measures":   measures,
		"rows":                buildRows(rows, dimensions, measures),
		"summary":             aggregateRows(rows, measures),
	}
	groups := groupRows(rows, groupBy, measures)
	sortDatasetRows(groups, view.SortBy, view.Desc)
	if view.Limit > 0 && len(groups) > view.Limit {
		groups = groups[:view.Limit]
	}
	out["groups"] = groups
	return out
}

func modelValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", value))
	}
}

func resolvePath(values map[string]any, path string) any {
	if values == nil || strings.TrimSpace(path) == "" {
		return nil
	}
	current := any(values)
	for _, part := range strings.Split(strings.TrimSpace(path), ".") {
		nextMap, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = nextMap[part]
	}
	return current
}

func aggregateMeasure(items []model.Record, measure MeasureDefinition) any {
	switch measure.Kind {
	case "count":
		return len(items)
	case "sum":
		total := 0.0
		for _, item := range items {
			total += numericValue(resolvePath(item.Values, measure.Path))
		}
		return total
	case "avg":
		if len(items) == 0 {
			return 0.0
		}
		total := 0.0
		for _, item := range items {
			total += numericValue(resolvePath(item.Values, measure.Path))
		}
		return total / float64(len(items))
	case "min":
		if len(items) == 0 {
			return 0.0
		}
		min := numericValue(resolvePath(items[0].Values, measure.Path))
		for _, item := range items[1:] {
			value := numericValue(resolvePath(item.Values, measure.Path))
			if value < min {
				min = value
			}
		}
		return min
	case "max":
		if len(items) == 0 {
			return 0.0
		}
		max := numericValue(resolvePath(items[0].Values, measure.Path))
		for _, item := range items[1:] {
			value := numericValue(resolvePath(item.Values, measure.Path))
			if value > max {
				max = value
			}
		}
		return max
	default:
		return nil
	}
}

func modelRows(items []model.Record) []map[string]any {
	rows := make([]map[string]any, 0, len(items))
	for _, item := range items {
		row := map[string]any{"id": item.ID}
		for key, value := range item.Values {
			row[key] = value
		}
		rows = append(rows, row)
	}
	return rows
}

func documentRows(items []document.Record, query model.Query, fields *securityfields.Service) []map[string]any {
	rows := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if !matchesDocumentQuery(item, query) {
			continue
		}
		payload := item.Body.Payload
		if fields != nil {
			payload = fields.SanitizeDocumentPayload(fields.DocumentProfile(securityfields.AccessContext{
				OrganizationID: item.Header.OrganizationID,
				LocationID:     item.Header.LocationID,
				ScopeID:        item.Header.LocationID,
				Channel:        "report",
				State:          item.Header.Status,
			}, item), payload)
		}
		rows = append(rows, map[string]any{
			"id": item.Header.ID,
			"header": map[string]any{
				"id":              item.Header.ID,
				"type":            item.Header.Type,
				"status":          item.Header.Status,
				"version":         item.Header.Version,
				"etag":            item.Header.ETag,
				"organization_id": item.Header.OrganizationID,
				"location_id":     item.Header.LocationID,
				"number":          item.Header.Number,
				"updated_at":      item.Header.UpdatedAt,
			},
			"body": map[string]any{
				"schema_version": item.Body.SchemaVersion,
				"payload":        payload,
			},
		})
	}
	return applyRowQuery(rows, query)
}

func projectionRows(items []search.DocumentSummary, query model.Query) []map[string]any {
	rows := make([]map[string]any, 0, len(items))
	for _, item := range items {
		row := map[string]any{
			"document_id":     item.DocumentID,
			"document_type":   item.DocumentType,
			"status":          item.Status,
			"version":         item.Version,
			"etag":            item.ETag,
			"organization_id": item.OrganizationID,
			"location_id":     item.LocationID,
			"updated_at":      item.UpdatedAt,
		}
		if !matchesRowFilters(row, query.Filters) {
			continue
		}
		rows = append(rows, row)
	}
	return applyRowQuery(rows, query)
}

func buildRows(items []map[string]any, dimensions []DimensionDefinition, measures []MeasureDefinition) []map[string]any {
	rows := make([]map[string]any, 0, len(items))
	for _, item := range items {
		row := map[string]any{"id": item["id"]}
		for _, dimension := range dimensions {
			row[dimension.Key] = resolvePath(item, dimension.Path)
		}
		for _, measure := range measures {
			if measure.Kind == "count" {
				row[measure.Key] = 1
				continue
			}
			row[measure.Key] = resolvePath(item, measure.Path)
		}
		rows = append(rows, row)
	}
	return rows
}

func aggregateRows(items []map[string]any, measures []MeasureDefinition) map[string]any {
	summary := map[string]any{}
	for _, measure := range measures {
		summary[measure.Key] = aggregateRowMeasure(items, measure)
	}
	return summary
}

func groupRows(items []map[string]any, groupBy []DimensionDefinition, measures []MeasureDefinition) []map[string]any {
	if len(groupBy) == 0 {
		return nil
	}
	buckets := map[string]map[string]any{}
	order := make([]string, 0)
	for _, item := range items {
		parts := make([]string, 0, len(groupBy))
		groupValues := map[string]any{}
		for _, dimension := range groupBy {
			value := strings.TrimSpace(modelValue(resolvePath(item, dimension.Path)))
			if value == "" {
				value = "unknown"
			}
			parts = append(parts, value)
			groupValues[dimension.Key] = value
		}
		bucketKey := strings.Join(parts, "|")
		if _, ok := buckets[bucketKey]; !ok {
			buckets[bucketKey] = groupValues
			order = append(order, bucketKey)
		}
		for _, measure := range measures {
			buckets[bucketKey][measure.Key] = accumulateRowMeasure(buckets[bucketKey][measure.Key], item, measure)
		}
	}
	out := make([]map[string]any, 0, len(order))
	for _, key := range order {
		group := buckets[key]
		for _, measure := range measures {
			if measure.Kind == "avg" {
				group[measure.Key] = finalizeAverage(group[measure.Key])
			}
		}
		out = append(out, group)
	}
	return out
}

func aggregateRowMeasure(items []map[string]any, measure MeasureDefinition) any {
	switch measure.Kind {
	case "count":
		return len(items)
	case "sum":
		total := 0.0
		for _, item := range items {
			total += numericValue(resolvePath(item, measure.Path))
		}
		return total
	case "avg":
		if len(items) == 0 {
			return 0.0
		}
		total := 0.0
		for _, item := range items {
			total += numericValue(resolvePath(item, measure.Path))
		}
		return total / float64(len(items))
	case "min":
		if len(items) == 0 {
			return 0.0
		}
		min := numericValue(resolvePath(items[0], measure.Path))
		for _, item := range items[1:] {
			value := numericValue(resolvePath(item, measure.Path))
			if value < min {
				min = value
			}
		}
		return min
	case "max":
		if len(items) == 0 {
			return 0.0
		}
		max := numericValue(resolvePath(items[0], measure.Path))
		for _, item := range items[1:] {
			value := numericValue(resolvePath(item, measure.Path))
			if value > max {
				max = value
			}
		}
		return max
	default:
		return nil
	}
}

func accumulateRowMeasure(current any, item map[string]any, measure MeasureDefinition) any {
	switch measure.Kind {
	case "count":
		existing, _ := current.(int)
		return existing + 1
	case "sum":
		existing, _ := current.(float64)
		return existing + numericValue(resolvePath(item, measure.Path))
	case "avg":
		state, _ := current.(map[string]any)
		if state == nil {
			state = map[string]any{"sum": 0.0, "count": 0}
		}
		state["sum"] = state["sum"].(float64) + numericValue(resolvePath(item, measure.Path))
		state["count"] = state["count"].(int) + 1
		return state
	case "min":
		value := numericValue(resolvePath(item, measure.Path))
		if current == nil {
			return value
		}
		existing, _ := current.(float64)
		if value < existing {
			return value
		}
		return existing
	case "max":
		value := numericValue(resolvePath(item, measure.Path))
		if current == nil {
			return value
		}
		existing, _ := current.(float64)
		if value > existing {
			return value
		}
		return existing
	default:
		return current
	}
}

func matchesDocumentQuery(item document.Record, query model.Query) bool {
	for key, value := range query.Filters {
		want := strings.TrimSpace(value)
		if want == "" {
			continue
		}
		switch key {
		case "type":
			if !strings.Contains(strings.ToLower(item.Header.Type), strings.ToLower(want)) {
				return false
			}
		case "status":
			if !strings.Contains(strings.ToLower(item.Header.Status), strings.ToLower(want)) {
				return false
			}
		case "organization_id":
			if !strings.Contains(strings.ToLower(item.Header.OrganizationID), strings.ToLower(want)) {
				return false
			}
		case "location_id":
			if !strings.Contains(strings.ToLower(item.Header.LocationID), strings.ToLower(want)) {
				return false
			}
		}
	}
	return true
}

func matchesRowFilters(row map[string]any, filters map[string]string) bool {
	for key, value := range filters {
		want := strings.TrimSpace(value)
		if want == "" {
			continue
		}
		if !strings.Contains(strings.ToLower(modelValue(resolvePath(row, key))), strings.ToLower(want)) {
			return false
		}
	}
	return true
}

func applyRowQuery(rows []map[string]any, query model.Query) []map[string]any {
	filtered := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		if matchesRowFilters(row, query.Filters) {
			filtered = append(filtered, row)
		}
	}
	if query.SortKey != "" {
		sortDatasetRows(filtered, query.SortKey, false)
	}
	page := query.Page
	pageSize := query.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		return filtered
	}
	start := (page - 1) * pageSize
	if start >= len(filtered) {
		return []map[string]any{}
	}
	end := start + pageSize
	if end > len(filtered) {
		end = len(filtered)
	}
	return filtered[start:end]
}

func accumulateMeasure(current any, item model.Record, measure MeasureDefinition) any {
	switch measure.Kind {
	case "count":
		existing, _ := current.(int)
		return existing + 1
	case "sum":
		existing, _ := current.(float64)
		return existing + numericValue(resolvePath(item.Values, measure.Path))
	case "avg":
		state, _ := current.(map[string]any)
		if state == nil {
			state = map[string]any{"sum": 0.0, "count": 0}
		}
		state["sum"] = state["sum"].(float64) + numericValue(resolvePath(item.Values, measure.Path))
		state["count"] = state["count"].(int) + 1
		return state
	case "min":
		value := numericValue(resolvePath(item.Values, measure.Path))
		if current == nil {
			return value
		}
		existing, _ := current.(float64)
		if value < existing {
			return value
		}
		return existing
	case "max":
		value := numericValue(resolvePath(item.Values, measure.Path))
		if current == nil {
			return value
		}
		existing, _ := current.(float64)
		if value > existing {
			return value
		}
		return existing
	default:
		return current
	}
}

func finalizeAverage(value any) any {
	state, _ := value.(map[string]any)
	if state == nil {
		return 0.0
	}
	count, _ := state["count"].(int)
	if count == 0 {
		return 0.0
	}
	sum, _ := state["sum"].(float64)
	return sum / float64(count)
}

func numericValue(value any) float64 {
	switch typed := value.(type) {
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case float64:
		return typed
	case float32:
		return float64(typed)
	case string:
		parsed := strings.TrimSpace(typed)
		if parsed == "" {
			return 0
		}
		var out float64
		fmt.Sscanf(parsed, "%f", &out)
		return out
	default:
		return 0
	}
}
