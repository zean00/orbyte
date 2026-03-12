package search

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestTypesenseBackendLifecycleAndHelpers(t *testing.T) {
	var seen []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.Path+"?"+r.URL.RawQuery)
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/collections":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"ok":true}`))
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/documents"):
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"doc1"}`))
		case r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"doc1"}`))
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/documents/search"):
			payload := map[string]any{
				"found": 1,
				"hits": []map[string]any{{
					"document":        map[string]any{"id": "doc1", "source_id": "doc1", "source_kind": "document", "title": "searchable"},
					"text_match":      1.2,
					"vector_distance": 0.3,
				}},
			}
			_ = json.NewEncoder(w).Encode(payload)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	backend := NewTypesenseBackend(server.URL, "secret", time.Second)
	def := IndexDefinition{
		Key:               "documents.requests.search",
		Title:             "Requests",
		SourceKind:        "document",
		DocumentType:      "generic_request",
		Modes:             []string{"keyword", "vector", "hybrid"},
		OrganizationSplit: true,
		Fields:            []IndexFieldDefinition{{Key: "title", Path: "body.payload.title", Type: "string", Searchable: true, Sort: true}, {Key: "status", Path: "header.status", Type: "string", Facet: true}},
		VectorFields:      []VectorFieldDefinition{{Key: "semantic", SourcePaths: []string{"body.payload.title"}, EmbeddingMode: "typesense_auto", RemoteModel: "ts/small", Dimensions: 8}},
	}
	if err := backend.EnsureIndex(def, "org_default"); err != nil {
		t.Fatalf("ensure index failed: %v", err)
	}
	if err := backend.Upsert(def, "org_default", IndexedRecord{
		ID:             "doc1",
		SourceID:       "doc1",
		SourceKind:     "document",
		OrganizationID: "org_default",
		LocationID:     "loc_hq",
		Version:        2,
		UpdatedAt:      time.Now().UTC(),
		Fields:         map[string]any{"title": "searchable", "status": "submitted"},
		Vectors:        map[string][]float32{"semantic": []float32{0.1, 0.2}},
	}); err != nil {
		t.Fatalf("upsert failed: %v", err)
	}
	result, err := backend.Search(def, "org_default", QueryRequest{
		Mode:        "hybrid",
		Query:       "searchable",
		Vector:      []float32{0.1, 0.2},
		VectorField: "semantic",
		Filters:     map[string]string{"status": "submitted"},
		SortBy:      "title",
		Page:        1,
		PageSize:    10,
	})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if result.Total != 1 || len(result.Hits) != 1 {
		t.Fatalf("unexpected search result: %+v", result)
	}
	if err := backend.Delete(def, "org_default", "doc1"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if len(seen) != 4 {
		t.Fatalf("expected lifecycle requests, got %+v", seen)
	}
	if filter := typesenseFilter(map[string]string{"status": "submitted", "location_id": "loc_hq"}); !strings.Contains(filter, "status:=submitted") {
		t.Fatalf("unexpected filter encoding: %s", filter)
	}
	if !strings.Contains(formatVector([]float32{0.1, 0.2}), "0.1") {
		t.Fatalf("unexpected vector format")
	}
	fields := typesenseFields(def)
	if len(fields) < 3 {
		t.Fatalf("expected schema fields, got %+v", fields)
	}
	if doc := typesenseDocument(IndexedRecord{ID: "doc2", SourceID: "doc2", SourceKind: "document", OrganizationID: "org_default", UpdatedAt: time.Now().UTC(), Fields: map[string]any{"title": "x"}}); doc["title"] != "x" {
		t.Fatalf("unexpected document encoding: %+v", doc)
	}
	if typesenseFieldType("int") != "int32" || stringFrom("text") != "text" {
		t.Fatal("expected helper conversions to work")
	}
}

func TestTypesenseBackendDoRequiresConfig(t *testing.T) {
	backend := NewTypesenseBackend("", "", time.Second)
	if _, _, err := backend.do(http.MethodGet, "/collections", nil, nil); err == nil {
		t.Fatal("expected missing config error")
	}
}
