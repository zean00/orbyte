package httpx

import (
	"strings"

	"orbyte/internal/platform/document"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/reporting"
	"orbyte/internal/platform/securityfields"
	"orbyte/internal/platform/shared"
)

func documentAccessProfile(fieldSecurity *securityfields.Service, ident *identity.Service, p principal, record document.Record, channel string) securityfields.DocumentProfile {
	if fieldSecurity == nil {
		return securityfields.DocumentProfile{ResourceKind: "document", ResourceKey: record.Header.Type, Fields: map[string]securityfields.FieldAccess{}}
	}
	return fieldSecurity.DocumentProfile(securityfields.AccessContext{
		ActorID:        principalActorID(p),
		SessionID:      p.sessionID,
		OrganizationID: record.Header.OrganizationID,
		LocationID:     firstNonEmpty(p.currentLocationID, record.Header.LocationID),
		ScopeID:        firstNonEmpty(p.currentLocationID, record.Header.LocationID),
		Channel:        channel,
		State:          record.Header.Status,
		PermissionChecker: func(permissionKey string) bool {
			if strings.TrimSpace(permissionKey) == "" {
				return true
			}
			if ident == nil {
				return false
			}
			return principalAllowsPermission(ident, p, permissionKey, p.currentLocationID)
		},
	}, record)
}

func sanitizeDocumentRecord(fieldSecurity *securityfields.Service, ident *identity.Service, p principal, record document.Record, channel string) document.Record {
	if fieldSecurity == nil {
		return record
	}
	record.Body.Payload = fieldSecurity.SanitizeDocumentPayload(documentAccessProfile(fieldSecurity, ident, p, record, channel), record.Body.Payload)
	return record
}

func validateDocumentWrite(fieldSecurity *securityfields.Service, ident *identity.Service, p principal, record document.Record, payload map[string]any, prefix, channel string) error {
	if fieldSecurity == nil {
		return nil
	}
	return fieldSecurity.ValidateDocumentWrite(documentAccessProfile(fieldSecurity, ident, p, record, channel), payload, prefix)
}

func validateDocumentReportingSelections(fieldSecurity *securityfields.Service, ident *identity.Service, p principal, sample document.Record, requestedDims, requestedMeasures, requestedGroupBy []string) ([]string, []string, []string, error) {
	if fieldSecurity == nil {
		return requestedDims, requestedMeasures, requestedGroupBy, nil
	}
	profile := documentAccessProfile(fieldSecurity, ident, p, sample, "report")
	for _, key := range requestedDims {
		path := strings.TrimSpace(key)
		if path == "" {
			continue
		}
		if access, ok := profile.Fields[pathToDocumentFieldPath(path)]; ok && !access.ExportVisible {
			return nil, nil, nil, shared.Forbidden("reporting dimension is not allowed: " + path)
		}
	}
	for _, key := range requestedGroupBy {
		path := strings.TrimSpace(key)
		if path == "" {
			continue
		}
		if access, ok := profile.Fields[pathToDocumentFieldPath(path)]; ok && !access.ExportVisible {
			return nil, nil, nil, shared.Forbidden("reporting group_by is not allowed: " + path)
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
		if path == "" {
			continue
		}
		if access, ok := profile.Fields[pathToDocumentFieldPath(path)]; ok && !access.ExportVisible {
			return nil, nil, nil, shared.Forbidden("reporting measure is not allowed: " + path)
		}
	}
	return requestedDims, measures, requestedGroupBy, nil
}

func filterDocumentReportingDataset(def reporting.DatasetDefinition, profile securityfields.DocumentProfile) reporting.DatasetDefinition {
	filtered := def
	filtered.Dimensions = make([]reporting.DimensionDefinition, 0, len(def.Dimensions))
	for _, item := range def.Dimensions {
		if access, ok := profile.Fields[pathToDocumentFieldPath(item.Path)]; ok && !access.ExportVisible {
			continue
		}
		filtered.Dimensions = append(filtered.Dimensions, item)
	}
	filtered.Measures = make([]reporting.MeasureDefinition, 0, len(def.Measures))
	for _, item := range def.Measures {
		if item.Kind != "count" && strings.TrimSpace(item.Path) != "" {
			if access, ok := profile.Fields[pathToDocumentFieldPath(item.Path)]; ok && !access.ExportVisible {
				continue
			}
		}
		filtered.Measures = append(filtered.Measures, item)
	}
	return filtered
}

func pathToDocumentFieldPath(path string) string {
	path = strings.TrimSpace(path)
	switch {
	case strings.HasPrefix(path, "body.payload."):
		return strings.TrimPrefix(path, "body.payload.")
	case strings.HasPrefix(path, "payload."):
		return strings.TrimPrefix(path, "payload.")
	default:
		return path
	}
}
