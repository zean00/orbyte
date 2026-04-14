package model

import "testing"

func TestBuildPostgresRecordQueryRejectsUnsupportedSortKey(t *testing.T) {
	def := Definition{
		Key:         "party",
		DisplayName: "Party",
		Version:     "v1",
		Fields:      []FieldDefinition{{Key: "name", Type: "string"}},
	}
	if _, _, _, err := buildPostgresRecordQuery(def, Query{SortKey: "bad_sort"}, true); err == nil {
		t.Fatal("expected unsupported sort key to be rejected")
	}
}

func TestBuildPostgresRecordQueryRejectsUnsupportedFilterKey(t *testing.T) {
	def := Definition{
		Key:         "party",
		DisplayName: "Party",
		Version:     "v1",
		Fields:      []FieldDefinition{{Key: "name", Type: "string"}},
	}
	if _, _, _, err := buildPostgresRecordQuery(def, Query{Filters: map[string]string{"bad_filter": "x"}}, true); err == nil {
		t.Fatal("expected unsupported filter key to be rejected")
	}
}
