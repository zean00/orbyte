package model

import (
	"fmt"
	"strings"

	"orbyte/internal/platform/shared"
)

func NormalizeQuery(def Definition, query Query) (Query, error) {
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 {
		query.PageSize = DefaultPageSize
	}
	if query.PageSize > MaxPageSize {
		query.PageSize = MaxPageSize
	}
	query.SortKey = strings.TrimSpace(query.SortKey)
	if query.SortKey == "" {
		query.SortKey = strings.TrimSpace(def.DefaultSort)
	}
	if err := assertSafeQuery(def, query); err != nil {
		return Query{}, err
	}
	cleaned := map[string]string{}
	for key, value := range query.Filters {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		cleaned[key] = value
	}
	query.Filters = cleaned
	return query, nil
}

func assertSafeQuery(def Definition, query Query) error {
	sortKey := strings.TrimSpace(query.SortKey)
	if sortKey != "" && !allowedQueryKey(def, sortKey) {
		return shared.Validation(fmt.Sprintf("unsupported sort key: %s", sortKey))
	}
	for key := range query.Filters {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if !allowedQueryKey(def, key) {
			return shared.Validation(fmt.Sprintf("unsupported filter key: %s", key))
		}
	}
	return nil
}

func allowedQueryKey(def Definition, key string) bool {
	switch strings.TrimSpace(key) {
	case "", "id", "created_at", "updated_at":
		return true
	}
	for _, field := range def.Fields {
		if field.Key == strings.TrimSpace(key) {
			return true
		}
	}
	return false
}
