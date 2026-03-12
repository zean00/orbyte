package search

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type TypesenseBackend struct {
	endpoint string
	apiKey   string
	client   *http.Client
}

func NewTypesenseBackend(endpoint, apiKey string, timeout time.Duration) *TypesenseBackend {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &TypesenseBackend{
		endpoint: strings.TrimRight(strings.TrimSpace(endpoint), "/"),
		apiKey:   strings.TrimSpace(apiKey),
		client:   &http.Client{Timeout: timeout},
	}
}

func (b *TypesenseBackend) EnsureIndex(def IndexDefinition, organizationID string) error {
	body := map[string]any{
		"name":   collectionKey(def.Key, organizationID),
		"fields": typesenseFields(def),
	}
	status, _, err := b.do(http.MethodPost, "/collections", body, nil)
	if err != nil {
		return err
	}
	if status == http.StatusConflict {
		return nil
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("typesense ensure index failed with status %d", status)
	}
	return nil
}

func (b *TypesenseBackend) Upsert(def IndexDefinition, organizationID string, record IndexedRecord) error {
	status, _, err := b.do(http.MethodPost, "/collections/"+url.PathEscape(collectionKey(def.Key, organizationID))+"/documents?action=upsert", typesenseDocument(record), nil)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("typesense upsert failed with status %d", status)
	}
	return nil
}

func (b *TypesenseBackend) Delete(def IndexDefinition, organizationID, sourceID string) error {
	status, _, err := b.do(http.MethodDelete, "/collections/"+url.PathEscape(collectionKey(def.Key, organizationID))+"/documents/"+url.PathEscape(sourceID), nil, nil)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("typesense delete failed with status %d", status)
	}
	return nil
}

func (b *TypesenseBackend) Search(def IndexDefinition, organizationID string, req QueryRequest) (QueryResult, error) {
	params := url.Values{}
	params.Set("q", strings.TrimSpace(req.Query))
	params.Set("page", strconv.Itoa(req.Page))
	params.Set("per_page", strconv.Itoa(req.PageSize))
	queryBy := make([]string, 0, len(def.Fields))
	for _, field := range def.Fields {
		if field.Searchable {
			queryBy = append(queryBy, field.Key)
		}
	}
	if len(queryBy) > 0 {
		params.Set("query_by", strings.Join(queryBy, ","))
	}
	if filter := typesenseFilter(req.Filters); filter != "" {
		params.Set("filter_by", filter)
	}
	if req.SortBy != "" {
		order := "asc"
		if req.Desc {
			order = "desc"
		}
		params.Set("sort_by", req.SortBy+":"+order)
	}
	mode := normalizeQueryMode(req.Mode, def)
	if len(req.Vector) > 0 && (mode == "vector" || mode == "hybrid") {
		vectorKey := req.VectorField
		if vectorKey == "" && len(def.VectorFields) > 0 {
			vectorKey = def.VectorFields[0].Key
		}
		params.Set("vector_query", fmt.Sprintf("%s:(%s)", vectorKey, formatVector(req.Vector)))
	}
	status, payload, err := b.do(http.MethodGet, "/collections/"+url.PathEscape(collectionKey(def.Key, organizationID))+"/documents/search", nil, params)
	if err != nil {
		return QueryResult{}, err
	}
	if status < 200 || status >= 300 {
		return QueryResult{}, fmt.Errorf("typesense search failed with status %d", status)
	}
	var raw struct {
		Found int `json:"found"`
		Hits  []struct {
			Document       map[string]any `json:"document"`
			TextMatch      float64        `json:"text_match"`
			VectorDistance float64        `json:"vector_distance"`
		} `json:"hits"`
	}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return QueryResult{}, err
	}
	hits := make([]QueryHit, 0, len(raw.Hits))
	for _, hit := range raw.Hits {
		hits = append(hits, QueryHit{
			ID:         stringFrom(hit.Document["id"]),
			SourceID:   stringFrom(hit.Document["source_id"]),
			SourceKind: stringFrom(hit.Document["source_kind"]),
			Score:      hit.TextMatch + hit.VectorDistance,
			Fields:     hit.Document,
		})
	}
	return QueryResult{IndexKey: def.Key, Mode: mode, Total: raw.Found, Page: req.Page, PageSize: req.PageSize, Hits: hits}, nil
}

func (b *TypesenseBackend) do(method, path string, body any, params url.Values) (int, []byte, error) {
	if b.endpoint == "" || b.apiKey == "" {
		return 0, nil, fmt.Errorf("typesense backend is not configured")
	}
	target := b.endpoint + path
	if len(params) > 0 {
		target += "?" + params.Encode()
	}
	var payload io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
		payload = bytes.NewReader(encoded)
	}
	req, err := http.NewRequest(method, target, payload)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("X-TYPESENSE-API-KEY", b.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := b.client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, data, nil
}

func typesenseFields(def IndexDefinition) []map[string]any {
	fields := []map[string]any{
		{"name": "id", "type": "string"},
		{"name": "source_id", "type": "string"},
		{"name": "source_kind", "type": "string", "facet": true},
		{"name": "organization_id", "type": "string", "facet": true},
		{"name": "location_id", "type": "string", "facet": true, "optional": true},
		{"name": "version", "type": "int32"},
		{"name": "updated_at", "type": "string"},
	}
	for _, field := range def.Fields {
		item := map[string]any{
			"name":     field.Key,
			"type":     typesenseFieldType(field.Type),
			"facet":    field.Facet,
			"optional": field.Optional,
			"sort":     field.Sort,
		}
		fields = append(fields, item)
	}
	for _, field := range def.VectorFields {
		item := map[string]any{
			"name": field.Key,
			"type": "float[]",
		}
		if field.Dimensions > 0 {
			item["num_dim"] = field.Dimensions
		}
		if field.EmbeddingMode == "typesense_auto" {
			item["embed"] = map[string]any{
				"from": field.SourcePaths,
				"model_config": map[string]any{
					"model_name": field.RemoteModel,
				},
			}
		}
		fields = append(fields, item)
	}
	return fields
}

func typesenseDocument(record IndexedRecord) map[string]any {
	payload := map[string]any{
		"id":              record.ID,
		"source_id":       record.SourceID,
		"source_kind":     record.SourceKind,
		"organization_id": record.OrganizationID,
		"location_id":     record.LocationID,
		"version":         record.Version,
		"updated_at":      record.UpdatedAt.Format(time.RFC3339Nano),
	}
	for key, value := range record.Fields {
		payload[key] = value
	}
	for key, vector := range record.Vectors {
		payload[key] = vector
	}
	return payload
}

func typesenseFieldType(fieldType string) string {
	switch strings.TrimSpace(fieldType) {
	case "string[]":
		return "string[]"
	case "int":
		return "int32"
	case "float":
		return "float"
	case "bool":
		return "bool"
	default:
		return "string"
	}
}

func typesenseFilter(filters map[string]string) string {
	parts := make([]string, 0, len(filters))
	for key, value := range filters {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s:=%s", key, value))
	}
	return strings.Join(parts, " && ")
}

func formatVector(vector []float32) string {
	parts := make([]string, 0, len(vector))
	for _, value := range vector {
		parts = append(parts, strconv.FormatFloat(float64(value), 'f', -1, 32))
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func stringFrom(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}
