package model

import (
	"fmt"
	"strings"
)

func buildPostgresRecordQuery(def Definition, query Query, withPaging bool) (string, string, []any) {
	args := []any{def.Key}
	where := []string{"model_key = $1"}
	argPos := 2
	for key, value := range query.Filters {
		columnExpr, contains := filterColumnExpression(def, key)
		if contains {
			where = append(where, fmt.Sprintf("%s ILIKE $%d", columnExpr, argPos))
			args = append(args, "%"+value+"%")
		} else {
			where = append(where, fmt.Sprintf("%s = $%d", columnExpr, argPos))
			args = append(args, value)
		}
		argPos++
	}
	baseWhere := " WHERE " + strings.Join(where, " AND ")
	orderBy := "record_id ASC"
	if query.SortKey != "" {
		orderBy = sortColumnExpression(query.SortKey)
		if query.Desc {
			orderBy += " DESC"
		} else {
			orderBy += " ASC"
		}
	}
	listQuery := `SELECT model_key, record_id, version, values_json, created_by, created_at, updated_by, updated_at FROM model_records` + baseWhere + ` ORDER BY ` + orderBy
	if withPaging {
		listQuery += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argPos, argPos+1)
		args = append(args, query.PageSize, (query.Page-1)*query.PageSize)
	}
	countQuery := `SELECT COUNT(*) FROM model_records` + baseWhere
	return listQuery, countQuery, args
}

func sortColumnExpression(key string) string {
	switch key {
	case "id":
		return "record_id"
	case "created_at":
		return "created_at"
	case "updated_at":
		return "updated_at"
	default:
		return fmt.Sprintf("LOWER(COALESCE(values_json->>'%s',''))", key)
	}
}

func filterColumnExpression(def Definition, key string) (string, bool) {
	switch key {
	case "id":
		return "record_id", false
	case "created_at":
		return "CAST(created_at AS TEXT)", false
	case "updated_at":
		return "CAST(updated_at AS TEXT)", false
	default:
		fieldType := ""
		for _, field := range def.Fields {
			if field.Key == key {
				fieldType = field.Type
				break
			}
		}
		contains := fieldType == "" || fieldType == "string" || fieldType == "text"
		return fmt.Sprintf("COALESCE(values_json->>'%s','')", key), contains
	}
}
