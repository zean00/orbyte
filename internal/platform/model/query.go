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
	if query.SortKey != "" && !allowedQueryKey(def, query.SortKey) {
		return Query{}, shared.Validation(fmt.Sprintf("unsupported sort key: %s", query.SortKey))
	}
	cleaned := map[string]string{}
	for key, value := range query.Filters {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		if !allowedQueryKey(def, key) {
			return Query{}, shared.Validation(fmt.Sprintf("unsupported filter key: %s", key))
		}
		cleaned[key] = value
	}
	query.Filters = cleaned
	return query, nil
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
