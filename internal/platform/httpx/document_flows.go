package httpx

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	application "orbyte/internal/platform/application"
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/idempotency"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/securityfields"
	"orbyte/internal/platform/shared"
)

type commitDocumentFlowRequest struct {
	OrganizationID    string                    `json:"organization_id"`
	LocationID        string                    `json:"location_id"`
	PrimaryDocumentID string                    `json:"primary_document_id,omitempty"`
	Documents         map[string]map[string]any `json:"documents"`
	IdempotencyKey    string                    `json:"idempotency_key,omitempty"`
}

type flowInstanceItem struct {
	Definition module.DocumentFlowDocumentDefinition `json:"definition"`
	Record     document.Record                       `json:"record"`
}

type flowInstancePayload struct {
	Flow                module.DocumentFlowDefinition `json:"flow"`
	FlowKey             string                        `json:"flow_key"`
	PrimaryDocumentID   string                        `json:"primary_document_id"`
	PrimaryDocumentType string                        `json:"primary_document_type"`
	ActiveDocumentKey   string                        `json:"active_document_key"`
	Items               []flowInstanceItem            `json:"items"`
}

func registerDocumentFlowRoutes(mux *http.ServeMux, ident *identity.Service, modules *module.Service, docs *document.Service, docActions *application.DocumentActions, fieldSecurity *securityfields.Service, idempotencySvc *idempotency.Service) {
	mux.HandleFunc("POST /document-flows/", func(w http.ResponseWriter, r *http.Request) {
		flowKey, ok := documentFlowCommitPath(r.URL.Path)
		if !ok {
			http.NotFound(w, r)
			return
		}
		flow, ok := modules.DocumentFlowForSurface(flowKey, module.UISurfaceUser)
		if !ok {
			respondError(w, shared.NotFound("document flow not found"))
			return
		}
		var req commitDocumentFlowRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, shared.Validation("invalid document flow payload"))
			return
		}
		if strings.TrimSpace(req.OrganizationID) == "" {
			respondError(w, shared.Validation("organization_id is required"))
			return
		}
		p, ok := requireAuthorization(w, r, ident, "document.create", req.LocationID, "")
		if !ok {
			return
		}
		if !principalAllowsAll(ident, p, flow.RequiredPermissions) {
			respondError(w, shared.Forbidden("document flow is not allowed"))
			return
		}
		locationID := strings.TrimSpace(req.LocationID)
		if locationID == "" && p.kind == userPrincipal {
			locationID = p.currentLocationID
		}
		resolvedDocs, err := resolveDocumentFlowDocuments(flow, req.Documents)
		if err != nil {
			respondError(w, err)
			return
		}
		existingByKey := map[string]document.Record{}
		primaryDocumentID := strings.TrimSpace(req.PrimaryDocumentID)
		if primaryDocumentID != "" {
			instance, err := resolveDocumentFlowInstance(modules, docs, flow.Key, primaryDocumentID)
			if err != nil {
				respondError(w, err)
				return
			}
			for _, item := range instance.Items {
				existingByKey[item.Definition.Key] = item.Record
			}
			existingKeys := map[string]bool{}
			for key := range existingByKey {
				existingKeys[key] = true
			}
			resolvedKeys := map[string]bool{}
			for _, item := range resolvedDocs {
				resolvedKeys[item.Definition.Key] = true
			}
			if len(existingKeys) != len(resolvedKeys) {
				respondError(w, shared.Validation("document flow branch changes are not supported during edit"))
				return
			}
			for key := range existingKeys {
				if !resolvedKeys[key] {
					respondError(w, shared.Validation("document flow branch changes are not supported during edit"))
					return
				}
			}
		}
		for _, item := range resolvedDocs {
			if !principalAllowsAll(ident, p, item.Definition.RequiredPermissions) {
				respondError(w, shared.Forbidden("document flow document is not allowed"))
				return
			}
			if !principalAllowsDocumentType(p, item.Definition.DocumentType) {
				respondError(w, shared.Forbidden("delegation grant does not allow this document type"))
				return
			}
			candidate := flowDocumentCandidate(item.Definition, req.OrganizationID, locationID, item.Payload)
			if existing, ok := existingByKey[item.Definition.Key]; ok {
				candidate = existing
				candidate.Body.Payload = item.Payload
			}
			if err := validateDocumentWrite(fieldSecurity, ident, p, candidate, item.Payload, "", "api"); err != nil {
				respondError(w, err)
				return
			}
		}

		outcome, err := idempotencySvc.Execute("document_flow.commit:"+flow.Key, req.IdempotencyKey, principalActorID(p), req, func() (idempotency.Outcome, error) {
			created := make([]document.Record, 0, len(resolvedDocs))
			createdByKey := map[string]document.Record{}
			var primary document.Record
			for _, item := range resolvedDocs {
				record, err := upsertDocumentFlowRecord(docs, docActions, existingByKey[item.Definition.Key], item.Definition, req.OrganizationID, locationID, principalActingContext(p), item.Payload)
				if err != nil {
					return idempotency.Outcome{}, err
				}
				created = append(created, sanitizeDocumentRecord(fieldSecurity, ident, p, record, "api"))
				createdByKey[item.Definition.Key] = record
				if item.Definition.PrimaryOutput {
					primary = record
				}
			}
			if primary.Header.ID == "" {
				return idempotency.Outcome{}, shared.Validation("document flow primary output is missing")
			}
			for _, item := range resolvedDocs {
				record := createdByKey[item.Definition.Key]
				record.Header.Metadata = mergeFlowMetadata(record.Header.Metadata, flow.Key, item.Definition.Key, primary.Header.ID)
				if err := docs.Save(record); err != nil {
					return idempotency.Outcome{}, err
				}
				createdByKey[item.Definition.Key] = record
			}
			for _, item := range resolvedDocs {
				if item.Definition.PrimaryOutput {
					continue
				}
				record := createdByKey[item.Definition.Key]
				if hasFlowLink(primary, record.Header.ID, flow.Key, item.Definition.Key) {
					continue
				}
				linkType := strings.TrimSpace(item.Definition.LinkType)
				if linkType == "" {
					linkType = "related_to"
				}
				if _, err := docs.AddLink(primary.Header.ID, record.Header.ID, linkType, map[string]any{
					"flow_key":          flow.Key,
					"flow_document_key": item.Definition.Key,
				}); err != nil {
					return idempotency.Outcome{}, err
				}
			}
			return idempotency.Outcome{
				StatusCode: http.StatusCreated,
				Response: map[string]any{
					"flow_key":              flow.Key,
					"primary_document_id":   primary.Header.ID,
					"primary_document_type": primary.Header.Type,
					"items":                 created,
				},
			}, nil
		})
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, outcome.StatusCode, outcome.Response)
	})
}

type resolvedFlowDocument struct {
	Definition module.DocumentFlowDocumentDefinition
	Payload    map[string]any
}

func documentFlowCommitPath(path string) (string, bool) {
	trimmed := strings.TrimSpace(strings.TrimPrefix(path, "/document-flows/"))
	if trimmed == "" || !strings.HasSuffix(trimmed, "/commit") {
		return "", false
	}
	key := strings.TrimSuffix(trimmed, "/commit")
	key = strings.TrimSuffix(key, "/")
	if key == "" {
		return "", false
	}
	return key, true
}

func resolveDocumentFlowDocuments(flow module.DocumentFlowDefinition, inputs map[string]map[string]any) ([]resolvedFlowDocument, error) {
	if len(flow.Steps) == 0 {
		return nil, shared.Validation("document flow has no steps")
	}
	stepMap := map[string]module.DocumentFlowStepDefinition{}
	for _, step := range flow.Steps {
		stepMap[step.Key] = step
	}
	context := map[string]any{"documents": map[string]any{}}
	docContext := context["documents"].(map[string]any)
	for key, payload := range inputs {
		docContext[key] = map[string]any{"payload": payload}
	}

	resolved := make([]resolvedFlowDocument, 0)
	seenSteps := map[string]bool{}
	current := flow.Steps[0]
	for {
		if seenSteps[current.Key] {
			return nil, shared.Validation("document flow contains a cycle")
		}
		seenSteps[current.Key] = true
		for _, item := range current.Documents {
			payload := map[string]any{}
			if source, ok := inputs[item.Key]; ok && source != nil {
				payload = source
			}
			resolved = append(resolved, resolvedFlowDocument{Definition: item, Payload: payload})
		}
		next := resolveNextStepKey(current, context)
		if next == "" {
			break
		}
		target, ok := stepMap[next]
		if !ok {
			return nil, shared.Validation("document flow branch target is not registered")
		}
		current = target
	}
	return resolved, nil
}

func resolveDocumentFlowInstance(modules *module.Service, docs *document.Service, flowKey, documentID string) (flowInstancePayload, error) {
	record, err := docs.Get(documentID)
	if err != nil {
		return flowInstancePayload{}, err
	}
	metadata := record.Header.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	if flowKey == "" {
		flowKey = strings.TrimSpace(stringifyValue(metadata["flow_key"]))
	}
	if flowKey == "" {
		for _, link := range record.Links {
			if candidate := strings.TrimSpace(stringifyValue(link.Metadata["flow_key"])); candidate != "" {
				flowKey = candidate
				break
			}
		}
	}
	if flowKey == "" {
		return flowInstancePayload{}, shared.NotFound("document flow not found")
	}
	flow, ok := modules.DocumentFlowForSurface(flowKey, module.UISurfaceUser)
	if !ok {
		return flowInstancePayload{}, shared.NotFound("document flow not found")
	}
	primaryDocumentID := strings.TrimSpace(stringifyValue(metadata["flow_primary_document_id"]))
	if primaryDocumentID == "" {
		primaryDocumentID = record.Header.ID
	}
	primary := record
	if primary.Header.ID != primaryDocumentID {
		primary, err = docs.Get(primaryDocumentID)
		if err != nil {
			return flowInstancePayload{}, err
		}
	}
	recordsByKey := map[string]document.Record{}
	primaryKey := strings.TrimSpace(stringifyValue(primary.Header.Metadata["flow_document_key"]))
	if primaryKey == "" {
		primaryKey = primaryDocumentKey(flow)
	}
	if primaryKey != "" {
		recordsByKey[primaryKey] = primary
	}
	for _, link := range primary.Links {
		if strings.TrimSpace(stringifyValue(link.Metadata["flow_key"])) != flow.Key {
			continue
		}
		docKey := strings.TrimSpace(stringifyValue(link.Metadata["flow_document_key"]))
		if docKey == "" {
			continue
		}
		linked, err := docs.Get(link.LinkedDocumentID)
		if err != nil {
			continue
		}
		recordsByKey[docKey] = linked
	}
	activeDocumentKey := strings.TrimSpace(stringifyValue(record.Header.Metadata["flow_document_key"]))
	if activeDocumentKey == "" {
		activeDocumentKey = primaryKey
	}
	inputs := map[string]map[string]any{}
	for key, item := range recordsByKey {
		inputs[key] = item.Body.Payload
	}
	resolvedDocs, err := resolveDocumentFlowDocuments(flow, inputs)
	if err != nil {
		return flowInstancePayload{}, err
	}
	items := make([]flowInstanceItem, 0, len(resolvedDocs))
	for _, item := range resolvedDocs {
		record, ok := recordsByKey[item.Definition.Key]
		if !ok {
			continue
		}
		items = append(items, flowInstanceItem{Definition: item.Definition, Record: record})
	}
	if activeDocumentKey == "" && len(items) > 0 {
		activeDocumentKey = items[0].Definition.Key
	}
	return flowInstancePayload{
		Flow:                flow,
		FlowKey:             flow.Key,
		PrimaryDocumentID:   primary.Header.ID,
		PrimaryDocumentType: primary.Header.Type,
		ActiveDocumentKey:   activeDocumentKey,
		Items:               items,
	}, nil
}

func primaryDocumentKey(flow module.DocumentFlowDefinition) string {
	for _, step := range flow.Steps {
		for _, item := range step.Documents {
			if item.PrimaryOutput {
				return item.Key
			}
		}
	}
	return ""
}

func flowDocumentCandidate(def module.DocumentFlowDocumentDefinition, organizationID, locationID string, payload map[string]any) document.Record {
	return document.Record{
		Header: document.Header{
			Type:           def.DocumentType,
			Status:         "draft",
			OrganizationID: organizationID,
			LocationID:     locationID,
		},
		Body: document.Body{Payload: payload},
	}
}

func upsertDocumentFlowRecord(docs *document.Service, docActions *application.DocumentActions, existing document.Record, def module.DocumentFlowDocumentDefinition, organizationID, locationID string, acting application.ActingContext, payload map[string]any) (document.Record, error) {
	if existing.Header.ID != "" {
		if docActions == nil {
			return document.Record{}, shared.Conflict("document flow update is not available")
		}
		return docActions.UpdateDraft(existing.Header.ID, acting, payload, existing.Header.Version, existing.Header.ETag)
	}
	return docs.Create(def.DocumentType, organizationID, locationID, acting.EffectiveActorID(), payload)
}

func mergeFlowMetadata(metadata map[string]any, flowKey, flowDocumentKey, primaryDocumentID string) map[string]any {
	next := map[string]any{}
	for key, value := range metadata {
		next[key] = value
	}
	next["flow_key"] = flowKey
	next["flow_document_key"] = flowDocumentKey
	next["flow_primary_document_id"] = primaryDocumentID
	return next
}

func hasFlowLink(primary document.Record, linkedDocumentID, flowKey, flowDocumentKey string) bool {
	for _, link := range primary.Links {
		if link.LinkedDocumentID != linkedDocumentID {
			continue
		}
		if strings.TrimSpace(stringifyValue(link.Metadata["flow_key"])) != flowKey {
			continue
		}
		if strings.TrimSpace(stringifyValue(link.Metadata["flow_document_key"])) != flowDocumentKey {
			continue
		}
		return true
	}
	return false
}

func resolveNextStepKey(step module.DocumentFlowStepDefinition, context map[string]any) string {
	for _, rule := range step.NextRules {
		value := resolveFlowContextPath(context, rule.Path)
		if rule.Truthy && isTruthy(value) {
			return rule.NextStepKey
		}
		if rule.Equals != "" && stringifyValue(value) == rule.Equals {
			return rule.NextStepKey
		}
		if len(rule.In) > 0 {
			current := stringifyValue(value)
			for _, option := range rule.In {
				if current == option {
					return rule.NextStepKey
				}
			}
		}
	}
	return strings.TrimSpace(step.NextStepKey)
}

func resolveFlowContextPath(payload map[string]any, path string) any {
	current := any(payload)
	for _, key := range strings.Split(strings.TrimSpace(path), ".") {
		asMap, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = asMap[key]
	}
	return current
}

func stringifyValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case nil:
		return ""
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

func isTruthy(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.TrimSpace(typed) != "" && !strings.EqualFold(strings.TrimSpace(typed), "false")
	case nil:
		return false
	default:
		return true
	}
}
