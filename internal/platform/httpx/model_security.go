package httpx

import (
	"strings"

	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/reporting"
	"orbyte/internal/platform/securityfields"
	"orbyte/internal/platform/shared"
)

func modelAccessProfile(fieldSecurity *securityfields.Service, ident *identity.Service, p principal, def model.Definition, channel string) securityfields.AccessProfile {
	if fieldSecurity == nil {
		return securityfields.AccessProfile{ResourceKind: "model", ResourceKey: def.Key, Fields: map[string]securityfields.FieldAccess{}}
	}
	return fieldSecurity.ModelProfile(securityfields.AccessContext{
		ActorID:    principalActorID(p),
		SessionID:  p.sessionID,
		LocationID: p.currentLocationID,
		ScopeID:    p.currentLocationID,
		Channel:    channel,
		PermissionChecker: func(permissionKey string) bool {
			if strings.TrimSpace(permissionKey) == "" {
				return true
			}
			if ident == nil {
				return false
			}
			return principalAllowsPermission(ident, p, permissionKey, p.currentLocationID)
		},
	}, def)
}

func sanitizeModelRecord(fieldSecurity *securityfields.Service, ident *identity.Service, p principal, def model.Definition, record model.Record, channel string) model.Record {
	if fieldSecurity == nil {
		return record
	}
	return fieldSecurity.SanitizeModelRecord(modelAccessProfile(fieldSecurity, ident, p, def, channel), record)
}

func sanitizeModelRecords(fieldSecurity *securityfields.Service, ident *identity.Service, p principal, def model.Definition, records []model.Record, channel string) []model.Record {
	if fieldSecurity == nil {
		return records
	}
	return fieldSecurity.SanitizeModelRecords(modelAccessProfile(fieldSecurity, ident, p, def, channel), records)
}

func validateModelWriteAccess(fieldSecurity *securityfields.Service, ident *identity.Service, p principal, def model.Definition, values map[string]any, channel string) error {
	if fieldSecurity == nil {
		return nil
	}
	return fieldSecurity.ValidateModelWrite(modelAccessProfile(fieldSecurity, ident, p, def, channel), values, def)
}

func validateModelQueryAccess(fieldSecurity *securityfields.Service, ident *identity.Service, p principal, def model.Definition, query model.Query, channel string) error {
	if fieldSecurity == nil {
		return nil
	}
	profile := modelAccessProfile(fieldSecurity, ident, p, def, channel)
	for key := range query.Filters {
		if key == "" {
			continue
		}
		access, ok := profile.Fields[key]
		if !ok {
			continue
		}
		if !access.Visible {
			return shared.Forbidden("field filter is not allowed: " + key)
		}
	}
	if query.SortKey != "" {
		if access, ok := profile.Fields[query.SortKey]; ok && !access.Visible {
			return shared.Forbidden("field sort is not allowed: " + query.SortKey)
		}
	}
	return nil
}

func reportingSelectionsForModel(fieldSecurity *securityfields.Service, ident *identity.Service, p principal, def model.Definition, requestedDims, requestedMeasures, requestedGroupBy []string) ([]string, []string, []string, error) {
	profile := modelAccessProfile(fieldSecurity, ident, p, def, "report")
	dimensions := requestedDims
	if len(dimensions) == 0 {
		for _, field := range def.Fields {
			access, ok := profile.Fields[field.Key]
			if !ok || !access.ExportVisible {
				continue
			}
			dimensions = append(dimensions, field.Key)
		}
	}
	for _, key := range dimensions {
		access, ok := profile.Fields[strings.TrimSpace(key)]
		if ok && !access.ExportVisible {
			return nil, nil, nil, shared.Forbidden("reporting dimension is not allowed: " + key)
		}
	}
	for _, key := range requestedGroupBy {
		access, ok := profile.Fields[strings.TrimSpace(key)]
		if ok && !access.ExportVisible {
			return nil, nil, nil, shared.Forbidden("reporting group_by is not allowed: " + key)
		}
	}
	measures := requestedMeasures
	if len(measures) == 0 {
		measures = []string{"count"}
	}
	for _, key := range measures {
		parts := strings.Split(strings.TrimSpace(key), ":")
		if len(parts) != 2 {
			continue
		}
		path := strings.TrimSpace(parts[1])
		access, ok := profile.Fields[path]
		if ok && !access.ExportVisible {
			return nil, nil, nil, shared.Forbidden("reporting measure is not allowed: " + path)
		}
	}
	return dimensions, measures, requestedGroupBy, nil
}

func reportingSelectionsForDataset(fieldSecurity *securityfields.Service, ident *identity.Service, p principal, def reporting.DatasetDefinition, modelDef model.Definition, requestedDims, requestedMeasures, requestedGroupBy []string) ([]string, []string, []string, error) {
	profile := modelAccessProfile(fieldSecurity, ident, p, modelDef, "report")
	dimensions := requestedDims
	if len(dimensions) == 0 {
		for _, item := range def.Dimensions {
			access, ok := profile.Fields[strings.TrimSpace(item.Path)]
			if ok && !access.ExportVisible {
				continue
			}
			dimensions = append(dimensions, item.Key)
		}
	}
	for _, key := range dimensions {
		dimension, ok := datasetDimension(def, key)
		if !ok {
			continue
		}
		access, ok := profile.Fields[strings.TrimSpace(dimension.Path)]
		if ok && !access.ExportVisible {
			return nil, nil, nil, shared.Forbidden("reporting dimension is not allowed: " + key)
		}
	}
	measures := requestedMeasures
	if len(measures) == 0 {
		for _, item := range def.Measures {
			if item.Kind == "count" || strings.TrimSpace(item.Path) == "" {
				measures = append(measures, item.Key)
				continue
			}
			access, ok := profile.Fields[strings.TrimSpace(item.Path)]
			if ok && !access.ExportVisible {
				continue
			}
			measures = append(measures, item.Key)
		}
		if len(measures) == 0 {
			measures = []string{"count"}
		}
	}
	for _, key := range measures {
		measure, ok := datasetMeasure(def, key)
		if !ok || measure.Kind == "count" || strings.TrimSpace(measure.Path) == "" {
			continue
		}
		access, ok := profile.Fields[strings.TrimSpace(measure.Path)]
		if ok && !access.ExportVisible {
			return nil, nil, nil, shared.Forbidden("reporting measure is not allowed: " + key)
		}
	}
	for _, key := range requestedGroupBy {
		dimension, ok := datasetDimension(def, key)
		if !ok {
			continue
		}
		access, ok := profile.Fields[strings.TrimSpace(dimension.Path)]
		if ok && !access.ExportVisible {
			return nil, nil, nil, shared.Forbidden("reporting group_by is not allowed: " + key)
		}
	}
	return dimensions, measures, requestedGroupBy, nil
}

func datasetDimension(def reporting.DatasetDefinition, key string) (reporting.DimensionDefinition, bool) {
	for _, item := range def.Dimensions {
		if item.Key == strings.TrimSpace(key) {
			return item, true
		}
	}
	return reporting.DimensionDefinition{}, false
}

func datasetMeasure(def reporting.DatasetDefinition, key string) (reporting.MeasureDefinition, bool) {
	for _, item := range def.Measures {
		if item.Key == strings.TrimSpace(key) {
			return item, true
		}
	}
	return reporting.MeasureDefinition{}, false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
