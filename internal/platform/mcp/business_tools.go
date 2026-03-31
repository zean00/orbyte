package mcp

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	application "orbyte/internal/platform/application"
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/securityfields"
	"orbyte/internal/platform/shared"
)

type businessModuleInfo struct {
	Key                  string                         `json:"key"`
	Name                 string                         `json:"name"`
	Description          string                         `json:"description,omitempty"`
	Version              string                         `json:"version,omitempty"`
	DomainFamily         string                         `json:"domain_family,omitempty"`
	Category             string                         `json:"category,omitempty"`
	Role                 module.ModuleRole              `json:"role,omitempty"`
	Enabled              bool                           `json:"enabled"`
	LifecycleState       string                         `json:"lifecycle_state,omitempty"`
	BusinessCapabilities []string                       `json:"business_capabilities,omitempty"`
	OwnedDocumentTypes   []string                       `json:"owned_document_types,omitempty"`
	OwnedModelKeys       []string                       `json:"owned_model_keys,omitempty"`
	ReferenceTypes       []string                       `json:"reference_types,omitempty"`
	Datasets             []string                       `json:"datasets,omitempty"`
	Dependencies         []module.DependencyRequirement `json:"dependencies,omitempty"`
	Dependents           []string                       `json:"dependents,omitempty"`
}

type businessRecordSummary struct {
	ResourceKind   string         `json:"resource_kind"`
	ModuleKey      string         `json:"module_key,omitempty"`
	ResourceKey    string         `json:"resource_key,omitempty"`
	RecordID       string         `json:"record_id"`
	Title          string         `json:"title,omitempty"`
	Status         string         `json:"status,omitempty"`
	OrganizationID string         `json:"organization_id,omitempty"`
	LocationID     string         `json:"location_id,omitempty"`
	UpdatedAt      time.Time      `json:"updated_at,omitempty"`
	Record         map[string]any `json:"record,omitempty"`
}

type syntheticToolDefinition struct {
	Name                string
	Title               string
	Description         string
	ModuleKey           string
	RequiredPermissions []string
	InputSchema         map[string]any
	Handler             func(*Server, ActorContext, map[string]any) (map[string]any, error)
}

func (s *Server) listSyntheticTools(actor ActorContext) []ToolDescriptor {
	defs := s.syntheticToolDefinitions(actor)
	items := make([]ToolDescriptor, 0, len(defs))
	for _, def := range defs {
		if !s.ToolEnabled(def.Name) || !allowsAll(actor.PermissionChecker, def.RequiredPermissions) {
			continue
		}
		items = append(items, s.decorateToolDescriptorWithGovernance(ToolDescriptor{
			Name:        def.Name,
			Title:       def.Title,
			Description: def.Description,
			ModuleKey:   def.ModuleKey,
			SourceType:  "synthetic",
			Scope:       scopeForModule(def.ModuleKey),
			InputSchema: cloneMap(def.InputSchema),
			Contract: ContractDescriptor{
				Version:              ContractVersion,
				Stability:            "stable",
				SideEffectClass:      defaultToolSideEffectClass(def.Name, ""),
				Idempotency:          defaultToolIdempotency(def.Name, ""),
				AuditAction:          "mcp.tool." + strings.TrimSpace(def.Name),
				ActionClass:          defaultToolActionClass(def.Name, ""),
				RiskClass:            defaultToolRiskClass(defaultToolActionClass(def.Name, ""), def.Name, ""),
				DraftOnly:            defaultToolActionClass(def.Name, "") == "draft",
				RequiresConfirmation: defaultToolRequiresConfirmation(defaultToolActionClass(def.Name, ""), def.Name, ""),
				RequiresApproval:     defaultToolRequiresApproval(defaultToolActionClass(def.Name, ""), def.Name, ""),
				GovernanceTags:       defaultToolGovernanceTags(defaultToolActionClass(def.Name, ""), def.Name, ""),
				BusinessDomains:      defaultToolBusinessDomains(def.Name),
				RequiredPermissions:  append([]string(nil), def.RequiredPermissions...),
			},
		}, nil))
	}
	return items
}

func (s *Server) syntheticToolDefinition(name string, actor ActorContext) (syntheticToolDefinition, bool) {
	for _, def := range s.syntheticToolDefinitions(actor) {
		if strings.TrimSpace(def.Name) == strings.TrimSpace(name) {
			return def, true
		}
	}
	return syntheticToolDefinition{}, false
}

func (s *Server) syntheticToolDefinitions(actor ActorContext) []syntheticToolDefinition {
	if s == nil || s.modules == nil {
		return nil
	}
	items := make([]syntheticToolDefinition, 0)
	for _, detail := range s.modules.List() {
		if !detail.Installed.Enabled {
			continue
		}
		scope := scopeForModule(detail.Manifest.Key)
		if !scopeMatches(actor.EndpointScope, scope) {
			continue
		}
		moduleKey := detail.Manifest.Key
		items = append(items,
			syntheticToolDefinition{
				Name:                moduleKey + ".business.info.get",
				Title:               detail.Manifest.Name + " Business Info",
				Description:         "Get business metadata and capabilities for " + moduleKey + ".",
				ModuleKey:           moduleKey,
				RequiredPermissions: []string{"module.read"},
				InputSchema:         map[string]any{"type": "object", "properties": map[string]any{"organization_id": map[string]any{"type": "string"}, "location_id": map[string]any{"type": "string"}}},
				Handler: func(s *Server, actor ActorContext, arguments map[string]any) (map[string]any, error) {
					arguments["module_key"] = moduleKey
					resp, _, err := s.businessModuleGet(actor, arguments)
					return resp, err
				},
			},
		)
		if len(moduleOwnedModelKeys(detail.Manifest)) > 0 || len(moduleOwnedDocumentTypes(detail.Manifest)) > 0 {
			items = append(items, syntheticToolDefinition{
				Name:                moduleKey + ".business.records.search",
				Title:               detail.Manifest.Name + " Business Records",
				Description:         "Search business records owned by " + moduleKey + ".",
				ModuleKey:           moduleKey,
				RequiredPermissions: []string{"module.read"},
				InputSchema:         map[string]any{"type": "object", "properties": map[string]any{"resource_kind": map[string]any{"type": "string"}, "document_type": map[string]any{"type": "string"}, "model_key": map[string]any{"type": "string"}, "query": map[string]any{"type": "string"}, "status": map[string]any{"type": "string"}, "filters": map[string]any{"type": "object"}, "page": map[string]any{"type": "integer"}, "page_size": map[string]any{"type": "integer"}, "include_full_payload": map[string]any{"type": "boolean"}}},
				Handler: func(s *Server, actor ActorContext, arguments map[string]any) (map[string]any, error) {
					arguments["module_key"] = moduleKey
					resp, _, err := s.businessRecordSearch(actor, arguments)
					return resp, err
				},
			})
		}
		if len(moduleOwnedDocumentTypes(detail.Manifest)) > 0 {
			items = append(items,
				syntheticToolDefinition{
					Name:                moduleKey + ".business.documents.search",
					Title:               detail.Manifest.Name + " Business Documents",
					Description:         "Search business documents owned by " + moduleKey + ".",
					ModuleKey:           moduleKey,
					RequiredPermissions: []string{"document.list"},
					InputSchema:         map[string]any{"type": "object", "properties": map[string]any{"document_type": map[string]any{"type": "string"}, "status": map[string]any{"type": "string"}, "query": map[string]any{"type": "string"}, "page": map[string]any{"type": "integer"}, "page_size": map[string]any{"type": "integer"}}},
					Handler: func(s *Server, actor ActorContext, arguments map[string]any) (map[string]any, error) {
						arguments["module_key"] = moduleKey
						resp, _, err := s.businessDocumentSearch(actor, arguments)
						return resp, err
					},
				},
				syntheticToolDefinition{
					Name:                moduleKey + ".business.document.draft.create",
					Title:               detail.Manifest.Name + " Create Draft Document",
					Description:         "Create a draft document owned by " + moduleKey + " after confirmation.",
					ModuleKey:           moduleKey,
					RequiredPermissions: []string{"document.create"},
					InputSchema:         map[string]any{"type": "object", "properties": map[string]any{"document_type": map[string]any{"type": "string"}, "organization_id": map[string]any{"type": "string"}, "location_id": map[string]any{"type": "string"}, "payload": map[string]any{"type": "object"}, "confirm_apply": map[string]any{"type": "boolean"}}, "required": []string{"document_type", "payload", "confirm_apply"}},
					Handler: func(s *Server, actor ActorContext, arguments map[string]any) (map[string]any, error) {
						arguments["module_key"] = moduleKey
						resp, _, err := s.businessDocumentDraftCreate(actor, arguments)
						return resp, err
					},
				},
				syntheticToolDefinition{
					Name:                moduleKey + ".business.document.draft.update",
					Title:               detail.Manifest.Name + " Update Draft Document",
					Description:         "Update a draft document owned by " + moduleKey + " after confirmation.",
					ModuleKey:           moduleKey,
					RequiredPermissions: []string{"document.update_draft"},
					InputSchema:         map[string]any{"type": "object", "properties": map[string]any{"document_id": map[string]any{"type": "string"}, "payload": map[string]any{"type": "object"}, "expected_version": map[string]any{"type": "integer"}, "expected_etag": map[string]any{"type": "string"}, "confirm_apply": map[string]any{"type": "boolean"}}, "required": []string{"document_id", "payload", "confirm_apply"}},
					Handler: func(s *Server, actor ActorContext, arguments map[string]any) (map[string]any, error) {
						arguments["module_key"] = moduleKey
						resp, _, err := s.businessDocumentDraftUpdate(actor, arguments)
						return resp, err
					},
				},
			)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items
}

func (s *Server) businessModuleList(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.modules == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"module.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	query := strings.ToLower(strings.TrimSpace(stringArg(arguments, "query")))
	items := s.businessModuleInfos(actor, stringArg(arguments, "organization_id"), stringArg(arguments, "location_id"))
	if query != "" {
		filtered := make([]businessModuleInfo, 0, len(items))
		for _, item := range items {
			if businessModuleMatches(item, query) {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded %d business modules.", len(items))}},
		"structuredContent": map[string]any{"items": items},
	}, true, nil
}

func (s *Server) businessModuleGet(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.modules == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"module.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	moduleKey := strings.TrimSpace(stringArg(arguments, "module_key"))
	if moduleKey == "" {
		return nil, true, shared.Validation("module_key is required")
	}
	items := s.businessModuleInfos(actor, stringArg(arguments, "organization_id"), stringArg(arguments, "location_id"))
	for _, item := range items {
		if item.Key == moduleKey {
			return map[string]any{
				"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded business module %s.", moduleKey)}},
				"structuredContent": item,
			}, true, nil
		}
	}
	return nil, true, shared.NotFound("module not found")
}

func (s *Server) businessCapabilitySearch(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.modules == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"module.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	query := strings.ToLower(strings.TrimSpace(stringArg(arguments, "query")))
	if query == "" {
		return nil, true, shared.Validation("query is required")
	}
	items := s.businessModuleInfos(actor, stringArg(arguments, "organization_id"), stringArg(arguments, "location_id"))
	matches := make([]map[string]any, 0)
	for _, item := range items {
		for _, capability := range item.BusinessCapabilities {
			if strings.Contains(strings.ToLower(capability), query) {
				matches = append(matches, map[string]any{
					"module_key":           item.Key,
					"module_name":          item.Name,
					"capability":           capability,
					"description":          item.Description,
					"owned_document_types": append([]string(nil), item.OwnedDocumentTypes...),
					"owned_model_keys":     append([]string(nil), item.OwnedModelKeys...),
				})
			}
		}
	}
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Found %d matching business capabilities.", len(matches))}},
		"structuredContent": map[string]any{"items": matches},
	}, true, nil
}

func (s *Server) businessDocumentTypeList(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.documents == nil || s.modules == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"document.list"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	moduleKey := strings.TrimSpace(stringArg(arguments, "module_key"))
	items := make([]map[string]any, 0)
	defs := s.documents.Definitions()
	for _, def := range defs {
		info := map[string]any{
			"document_type":  def.Type,
			"display_name":   def.DisplayName,
			"workflow_key":   def.WorkflowKey,
			"schema_version": def.SchemaVersion,
			"module_keys":    s.documentOwnerModuleKeys(def.Type),
		}
		if moduleKey != "" && !containsString(info["module_keys"].([]string), moduleKey) {
			continue
		}
		items = append(items, info)
	}
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded %d business document types.", len(items))}},
		"structuredContent": map[string]any{"items": items},
	}, true, nil
}

func (s *Server) businessDocumentSearch(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.documents == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"document.list"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	items, err := s.filteredDocumentSummaries(actor, arguments)
	if err != nil {
		return nil, true, err
	}
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Found %d business documents.", len(items))}},
		"structuredContent": map[string]any{"items": items},
	}, true, nil
}

func (s *Server) businessDocumentGet(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.documents == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"document.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	documentID := strings.TrimSpace(stringArg(arguments, "document_id"))
	if documentID == "" {
		return nil, true, shared.Validation("document_id is required")
	}
	record, err := s.documents.Get(documentID)
	if err != nil {
		return nil, true, err
	}
	if strings.TrimSpace(stringArg(arguments, "module_key")) != "" && !containsString(s.documentOwnerModuleKeys(record.Header.Type), stringArg(arguments, "module_key")) {
		return nil, true, shared.NotFound("document not found")
	}
	return s.renderBusinessDocumentResult(actor, record, boolArg(arguments, "include_full_payload"))
}

func (s *Server) businessDocumentDraftCreate(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.documents == nil || s.modules == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"document.create"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	if !boolArg(arguments, "confirm_apply") {
		return nil, true, shared.Validation("confirm_apply must be true")
	}
	documentType := strings.TrimSpace(stringArg(arguments, "document_type"))
	if documentType == "" {
		return nil, true, shared.Validation("document_type is required")
	}
	if moduleKey := strings.TrimSpace(stringArg(arguments, "module_key")); moduleKey != "" && !containsString(s.documentOwnerModuleKeys(documentType), moduleKey) {
		return nil, true, shared.Validation("document_type is not owned by module")
	}
	payload := mapArg(arguments, "payload")
	locationID := firstNonEmpty(stringArg(arguments, "location_id"), strings.TrimSpace(actor.LocationID))
	organizationID := strings.TrimSpace(stringArg(arguments, "organization_id"))
	candidate := document.Record{
		Header: document.Header{
			Type:           documentType,
			Status:         "draft",
			OrganizationID: organizationID,
			LocationID:     locationID,
		},
		Body: document.Body{Payload: payload},
	}
	if err := s.validateDocumentWrite(actor, candidate, payload); err != nil {
		return nil, true, err
	}
	if err := validateMCPDocumentPayloadForType(s.modules, documentType, payload); err != nil {
		return nil, true, err
	}
	record, err := s.documents.Create(documentType, organizationID, locationID, effectiveActorID(actor), payload)
	if err != nil {
		return nil, true, err
	}
	if s.search != nil {
		s.search.RefreshDocument(record)
	}
	return s.renderBusinessDocumentDraftResult(actor, record, "Created draft document "+record.Header.ID+".")
}

func (s *Server) businessDocumentDraftUpdate(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.documents == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"document.update_draft"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	if !boolArg(arguments, "confirm_apply") {
		return nil, true, shared.Validation("confirm_apply must be true")
	}
	documentID := strings.TrimSpace(stringArg(arguments, "document_id"))
	if documentID == "" {
		return nil, true, shared.Validation("document_id is required")
	}
	current, err := s.documents.Get(documentID)
	if err != nil {
		return nil, true, err
	}
	if strings.TrimSpace(stringArg(arguments, "module_key")) != "" && !containsString(s.documentOwnerModuleKeys(current.Header.Type), stringArg(arguments, "module_key")) {
		return nil, true, shared.NotFound("document not found")
	}
	payload := mapArg(arguments, "payload")
	mergedPayload := mergeMap(current.Body.Payload, payload)
	if err := s.validateDocumentWrite(actor, current, payload); err != nil {
		return nil, true, err
	}
	if err := validateMCPDocumentPayloadForType(s.modules, current.Header.Type, mergedPayload); err != nil {
		return nil, true, err
	}
	var updated document.Record
	if s.documentActions != nil {
		updated, err = s.documentActions.UpdateDraft(
			documentID,
			application.ActingContext{
				ActorID:           actor.ActorID,
				OnBehalfOfUserID:  actor.OnBehalfOfUserID,
				DelegationGrantID: actor.DelegationGrantID,
			},
			payload,
			intArg(arguments, "expected_version"),
			stringArg(arguments, "expected_etag"),
		)
	} else {
		updated, err = s.updateDraftDocumentFallback(current, actor, payload, intArg(arguments, "expected_version"), stringArg(arguments, "expected_etag"))
	}
	if err != nil {
		return nil, true, err
	}
	if s.search != nil {
		s.search.RefreshDocument(updated)
	}
	return s.renderBusinessDocumentDraftResult(actor, updated, "Updated draft document "+updated.Header.ID+".")
}

func (s *Server) businessRecordSearch(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	moduleKey := strings.TrimSpace(stringArg(arguments, "module_key"))
	kind := strings.TrimSpace(stringArg(arguments, "resource_kind"))
	switch kind {
	case "", "document":
		documents := []businessRecordSummary(nil)
		if allowsAll(actor.PermissionChecker, []string{"document.list"}) {
			var err error
			documents, err = s.filteredDocumentSummaries(actor, arguments)
			if err != nil {
				return nil, true, err
			}
		}
		models, err := s.filteredModelSummaries(actor, arguments)
		if err != nil {
			return nil, true, err
		}
		if kind == "document" {
			models = nil
		}
		items := make([]businessRecordSummary, 0, len(documents)+len(models))
		items = append(items, documents...)
		items = append(items, models...)
		if moduleKey != "" {
			filtered := make([]businessRecordSummary, 0, len(items))
			for _, item := range items {
				if item.ModuleKey == moduleKey {
					filtered = append(filtered, item)
				}
			}
			items = filtered
		}
		sort.Slice(items, func(i, j int) bool { return items[i].UpdatedAt.After(items[j].UpdatedAt) })
		return map[string]any{
			"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Found %d business records.", len(items))}},
			"structuredContent": map[string]any{"items": items},
		}, true, nil
	case "model":
		items, err := s.filteredModelSummaries(actor, arguments)
		if err != nil {
			return nil, true, err
		}
		return map[string]any{
			"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Found %d business records.", len(items))}},
			"structuredContent": map[string]any{"items": items},
		}, true, nil
	default:
		return nil, true, shared.Validation("resource_kind must be document or model")
	}
}

func (s *Server) businessRecordGet(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	switch strings.TrimSpace(stringArg(arguments, "resource_kind")) {
	case "document":
		return s.businessDocumentGet(actor, arguments)
	case "model":
		return s.businessModelGet(actor, arguments)
	default:
		return nil, true, shared.Validation("resource_kind must be document or model")
	}
}

func (s *Server) businessRecordRelated(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	switch strings.TrimSpace(stringArg(arguments, "resource_kind")) {
	case "document":
		return s.businessDocumentRelated(actor, arguments)
	case "model":
		return s.businessModelRelated(actor, arguments)
	default:
		return nil, true, shared.Validation("resource_kind must be document or model")
	}
}

func (s *Server) businessReferenceTypeList(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	_ = arguments
	if s == nil || s.reference == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	items := s.reference.Types()
	sort.Slice(items, func(i, j int) bool { return items[i].Key < items[j].Key })
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded %d reference types.", len(items))}},
		"structuredContent": map[string]any{"items": items},
	}, true, nil
}

func (s *Server) businessReferenceResolve(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.reference == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	typeKey := strings.TrimSpace(stringArg(arguments, "type_key"))
	if typeKey == "" {
		return nil, true, shared.Validation("type_key is required")
	}
	set, err := s.reference.Resolve(typeKey, stringArg(arguments, "organization_id"), firstNonEmpty(stringArg(arguments, "location_id"), actor.LocationID), time.Time{})
	if err != nil {
		return nil, true, err
	}
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Resolved %d reference records for %s.", len(set.Items), typeKey)}},
		"structuredContent": set,
	}, true, nil
}

func (s *Server) businessDatasetList(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	_ = actor
	if s == nil || s.modules == nil {
		return nil, false, nil
	}
	moduleKey := strings.TrimSpace(stringArg(arguments, "module_key"))
	items := make([]map[string]any, 0)
	for _, detail := range s.modules.List() {
		if !detail.Installed.Enabled {
			continue
		}
		if moduleKey != "" && detail.Manifest.Key != moduleKey {
			continue
		}
		for _, dataset := range detail.Manifest.Datasets {
			items = append(items, map[string]any{
				"module_key":  detail.Manifest.Key,
				"dataset_key": dataset.Key,
				"title":       dataset.Title,
				"source_kind": dataset.SourceKind,
				"model_key":   dataset.ModelKey,
			})
		}
	}
	sort.Slice(items, func(i, j int) bool { return anyString(items[i]["dataset_key"]) < anyString(items[j]["dataset_key"]) })
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded %d datasets.", len(items))}},
		"structuredContent": map[string]any{"items": items},
	}, true, nil
}

func (s *Server) businessModelGet(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.models == nil {
		return nil, false, nil
	}
	modelKey := strings.TrimSpace(stringArg(arguments, "model_key"))
	recordID := strings.TrimSpace(stringArg(arguments, "record_id"))
	if modelKey == "" || recordID == "" {
		return nil, true, shared.Validation("model_key and record_id are required")
	}
	def, ok := s.models.Definition(modelKey)
	if !ok {
		return nil, true, shared.NotFound("model definition not found")
	}
	if !allowsAll(actor.PermissionChecker, []string{def.ReadPermissionKey}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	record, err := s.models.Get(modelKey, recordID)
	if err != nil {
		return nil, true, err
	}
	sanitized := s.sanitizeModelRecord(actor, def, record)
	if boolArg(arguments, "include_full_payload") {
		return map[string]any{
			"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded business model record %s.", recordID)}},
			"structuredContent": map[string]any{"definition": def, "record": sanitized},
		}, true, nil
	}
	summary := s.modelSummary(def, sanitized)
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded business model record %s.", recordID)}},
		"structuredContent": summary,
	}, true, nil
}

func (s *Server) businessModelRelated(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.models == nil {
		return nil, false, nil
	}
	modelKey := strings.TrimSpace(stringArg(arguments, "model_key"))
	recordID := strings.TrimSpace(stringArg(arguments, "record_id"))
	relationKey := strings.TrimSpace(stringArg(arguments, "relation_key"))
	if modelKey == "" || recordID == "" || relationKey == "" {
		return nil, true, shared.Validation("model_key, record_id, and relation_key are required")
	}
	def, ok := s.models.Definition(modelKey)
	if !ok {
		return nil, true, shared.NotFound("model definition not found")
	}
	if !allowsAll(actor.PermissionChecker, []string{def.ReadPermissionKey}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	targetDef, ok := relatedModelDefinition(s.models, def, relationKey)
	if !ok {
		return nil, true, shared.NotFound("model relation not found")
	}
	query := modelQueryFromArguments(arguments)
	if err := s.validateModelQuery(actor, targetDef, query); err != nil {
		return nil, true, err
	}
	items, total, err := s.models.Related(modelKey, recordID, relationKey, query)
	if err != nil {
		return nil, true, err
	}
	items = s.sanitizeModelRecords(actor, targetDef, items)
	summaries := make([]businessRecordSummary, 0, len(items))
	for _, item := range items {
		summaries = append(summaries, s.modelSummary(targetDef, item))
	}
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded %d related model records.", len(summaries))}},
		"structuredContent": map[string]any{"items": summaries, "total": total, "relation_key": relationKey},
	}, true, nil
}

func (s *Server) businessDocumentRelated(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.documents == nil {
		return nil, false, nil
	}
	documentID := strings.TrimSpace(stringArg(arguments, "document_id"))
	if documentID == "" {
		return nil, true, shared.Validation("document_id is required")
	}
	if !allowsAll(actor.PermissionChecker, []string{"document.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	record, err := s.documents.Get(documentID)
	if err != nil {
		return nil, true, err
	}
	record = s.sanitizeDocumentRecord(actor, record)
	return map[string]any{
		"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded document links for %s.", documentID)}},
		"structuredContent": map[string]any{
			"links":       record.Links,
			"attachments": record.Attachments,
		},
	}, true, nil
}

func (s *Server) businessModuleInfos(actor ActorContext, organizationID, locationID string) []businessModuleInfo {
	if s == nil || s.modules == nil {
		return nil
	}
	locationID = firstNonEmpty(strings.TrimSpace(locationID), strings.TrimSpace(actor.LocationID))
	items := s.modules.ListForScope(strings.TrimSpace(organizationID), locationID, "")
	dependents := reverseModuleDependencies(s.modules.List())
	result := make([]businessModuleInfo, 0, len(items))
	for _, item := range items {
		info := businessModuleInfo{
			Key:                  item.Manifest.Key,
			Name:                 item.Manifest.Name,
			Description:          firstNonEmpty(item.Manifest.Description, item.Manifest.Category),
			Version:              item.Manifest.Version,
			DomainFamily:         item.Manifest.DomainFamily,
			Category:             item.Manifest.Category,
			Role:                 item.Manifest.Role,
			Enabled:              item.Installed.Enabled,
			LifecycleState:       item.LifecycleState,
			BusinessCapabilities: append([]string(nil), item.Manifest.BusinessCapabilities...),
			OwnedDocumentTypes:   moduleOwnedDocumentTypes(item.Manifest),
			OwnedModelKeys:       moduleOwnedModelKeys(item.Manifest),
			ReferenceTypes:       moduleReferenceTypeKeys(item.Manifest),
			Datasets:             moduleDatasetKeys(item.Manifest),
			Dependencies:         append([]module.DependencyRequirement(nil), item.Manifest.DependencyRequirements...),
			Dependents:           append([]string(nil), dependents[item.Manifest.Key]...),
		}
		result = append(result, info)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Key < result[j].Key })
	return result
}

func businessModuleMatches(item businessModuleInfo, query string) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return true
	}
	candidates := []string{item.Key, item.Name, item.Description, item.DomainFamily, item.Category}
	candidates = append(candidates, item.BusinessCapabilities...)
	candidates = append(candidates, item.OwnedDocumentTypes...)
	candidates = append(candidates, item.OwnedModelKeys...)
	for _, candidate := range candidates {
		if strings.Contains(strings.ToLower(candidate), query) {
			return true
		}
	}
	return false
}

func (s *Server) filteredDocumentSummaries(actor ActorContext, arguments map[string]any) ([]businessRecordSummary, error) {
	if s == nil || s.documents == nil {
		return nil, fmt.Errorf("documents are unavailable")
	}
	moduleKey := strings.TrimSpace(stringArg(arguments, "module_key"))
	documentType := strings.TrimSpace(stringArg(arguments, "document_type"))
	status := strings.TrimSpace(stringArg(arguments, "status"))
	organizationID := strings.TrimSpace(stringArg(arguments, "organization_id"))
	locationID := strings.TrimSpace(stringArg(arguments, "location_id"))
	includeFull := boolArg(arguments, "include_full_payload")
	query := strings.ToLower(strings.TrimSpace(stringArg(arguments, "query")))
	items := make([]businessRecordSummary, 0)
	for _, item := range s.documents.List() {
		if documentType != "" && item.Header.Type != documentType {
			continue
		}
		if status != "" && item.Header.Status != status {
			continue
		}
		if organizationID != "" && item.Header.OrganizationID != organizationID {
			continue
		}
		if locationID != "" && item.Header.LocationID != locationID {
			continue
		}
		moduleKeys := s.documentOwnerModuleKeys(item.Header.Type)
		if moduleKey != "" && !containsString(moduleKeys, moduleKey) {
			continue
		}
		if query != "" && !documentMatchesQuery(item, query) {
			continue
		}
		sanitized := s.sanitizeDocumentRecord(actor, item)
		summary := s.documentSummary(sanitized, includeFull)
		summary.ModuleKey = firstString(moduleKeys)
		summary.ResourceKey = sanitized.Header.Type
		items = append(items, summary)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].UpdatedAt.After(items[j].UpdatedAt) })
	return paginateBusinessRecordSummaries(items, intArg(arguments, "page"), intArg(arguments, "page_size")), nil
}

func (s *Server) filteredModelSummaries(actor ActorContext, arguments map[string]any) ([]businessRecordSummary, error) {
	if s == nil || s.models == nil {
		return nil, nil
	}
	moduleKey := strings.TrimSpace(stringArg(arguments, "module_key"))
	modelKey := strings.TrimSpace(stringArg(arguments, "model_key"))
	includeFull := boolArg(arguments, "include_full_payload")
	queryText := strings.ToLower(strings.TrimSpace(stringArg(arguments, "query")))
	items := make([]businessRecordSummary, 0)
	for _, def := range s.models.Definitions() {
		if modelKey != "" && def.Key != modelKey {
			continue
		}
		if moduleKey != "" && def.OwnerModuleKey != moduleKey {
			continue
		}
		if !allowsAll(actor.PermissionChecker, []string{def.ListPermissionKey}) {
			continue
		}
		query := modelQueryFromArguments(arguments)
		if err := s.validateModelQuery(actor, def, query); err != nil {
			return nil, err
		}
		records, _, err := s.models.List(def.Key, query)
		if err != nil {
			return nil, err
		}
		records = s.sanitizeModelRecords(actor, def, records)
		for _, record := range records {
			if queryText != "" && !modelMatchesQuery(record, queryText) {
				continue
			}
			summary := s.modelSummary(def, record)
			if includeFull {
				summary.Record = map[string]any{
					"record":     record,
					"definition": def,
				}
			}
			items = append(items, summary)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].UpdatedAt.After(items[j].UpdatedAt) })
	return paginateBusinessRecordSummaries(items, intArg(arguments, "page"), intArg(arguments, "page_size")), nil
}

func (s *Server) documentSummary(record document.Record, includeFull bool) businessRecordSummary {
	summary := businessRecordSummary{
		ResourceKind:   "document",
		ResourceKey:    record.Header.Type,
		RecordID:       record.Header.ID,
		Title:          firstNonEmpty(stringValue(record.Body.Payload["title"]), stringValue(record.Body.Payload["name"]), record.Header.Number, record.Header.ID),
		Status:         record.Header.Status,
		OrganizationID: record.Header.OrganizationID,
		LocationID:     record.Header.LocationID,
		UpdatedAt:      record.Header.UpdatedAt,
	}
	if includeFull {
		summary.Record = map[string]any{
			"header": record.Header,
			"body":   record.Body,
			"lines":  record.Lines,
			"links":  record.Links,
		}
	} else {
		summary.Record = map[string]any{
			"header": map[string]any{
				"id":         record.Header.ID,
				"type":       record.Header.Type,
				"status":     record.Header.Status,
				"number":     record.Header.Number,
				"updated_at": record.Header.UpdatedAt,
			},
			"payload": summarizePayload(record.Body.Payload),
		}
	}
	return summary
}

func (s *Server) modelSummary(def model.Definition, record model.Record) businessRecordSummary {
	return businessRecordSummary{
		ResourceKind:   "model",
		ModuleKey:      def.OwnerModuleKey,
		ResourceKey:    def.Key,
		RecordID:       record.ID,
		Title:          firstNonEmpty(stringValue(record.Values["name"]), stringValue(record.Values["title"]), stringValue(record.Values["code"]), record.ID),
		Status:         stringValue(record.Values["status"]),
		OrganizationID: stringValue(record.Values["organization_id"]),
		LocationID:     stringValue(record.Values["location_id"]),
		UpdatedAt:      record.UpdatedAt,
		Record: map[string]any{
			"id":     record.ID,
			"values": summarizePayload(record.Values),
		},
	}
}

func (s *Server) renderBusinessDocumentResult(actor ActorContext, record document.Record, includeFull bool) (map[string]any, bool, error) {
	sanitized := s.sanitizeDocumentRecord(actor, record)
	if includeFull {
		return map[string]any{
			"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded business document %s.", record.Header.ID)}},
			"structuredContent": sanitized,
		}, true, nil
	}
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded business document %s.", record.Header.ID)}},
		"structuredContent": s.documentSummary(sanitized, false),
	}, true, nil
}

func (s *Server) renderBusinessDocumentDraftResult(actor ActorContext, record document.Record, text string) (map[string]any, bool, error) {
	sanitized := s.sanitizeDocumentRecord(actor, record)
	return map[string]any{
		"content": []ContentBlock{{Type: "text", Text: text}},
		"structuredContent": map[string]any{
			"record":  sanitized,
			"summary": s.documentSummary(sanitized, false),
		},
	}, true, nil
}

func (s *Server) sanitizeDocumentRecord(actor ActorContext, record document.Record) document.Record {
	if s.fieldSecurity == nil {
		return record
	}
	profile := s.fieldSecurity.DocumentProfile(securityfields.AccessContext{
		ActorID:        effectiveActorID(actor),
		SessionID:      actor.SessionID,
		OrganizationID: record.Header.OrganizationID,
		LocationID:     firstNonEmpty(actor.LocationID, record.Header.LocationID),
		ScopeID:        firstNonEmpty(actor.LocationID, record.Header.LocationID),
		Channel:        "mcp",
		State:          record.Header.Status,
		PermissionChecker: func(permissionKey string) bool {
			return strings.TrimSpace(permissionKey) == "" || (actor.PermissionChecker != nil && actor.PermissionChecker(permissionKey))
		},
	}, record)
	record.Body.Payload = s.fieldSecurity.SanitizeDocumentPayload(profile, record.Body.Payload)
	return record
}

func (s *Server) validateDocumentWrite(actor ActorContext, record document.Record, payload map[string]any) error {
	if s.fieldSecurity == nil {
		return nil
	}
	profile := s.fieldSecurity.DocumentProfile(securityfields.AccessContext{
		ActorID:        effectiveActorID(actor),
		SessionID:      actor.SessionID,
		OrganizationID: record.Header.OrganizationID,
		LocationID:     firstNonEmpty(actor.LocationID, record.Header.LocationID),
		ScopeID:        firstNonEmpty(actor.LocationID, record.Header.LocationID),
		Channel:        "mcp",
		State:          record.Header.Status,
		PermissionChecker: func(permissionKey string) bool {
			return strings.TrimSpace(permissionKey) == "" || (actor.PermissionChecker != nil && actor.PermissionChecker(permissionKey))
		},
	}, record)
	return s.fieldSecurity.ValidateDocumentWrite(profile, payload, "")
}

func (s *Server) sanitizeModelRecord(actor ActorContext, def model.Definition, record model.Record) model.Record {
	if s.fieldSecurity == nil {
		return record
	}
	return s.fieldSecurity.SanitizeModelRecord(s.modelAccessProfile(actor, def), record)
}

func (s *Server) sanitizeModelRecords(actor ActorContext, def model.Definition, records []model.Record) []model.Record {
	if s.fieldSecurity == nil {
		return records
	}
	return s.fieldSecurity.SanitizeModelRecords(s.modelAccessProfile(actor, def), records)
}

func (s *Server) validateModelQuery(actor ActorContext, def model.Definition, query model.Query) error {
	if s.fieldSecurity == nil {
		return nil
	}
	profile := s.modelAccessProfile(actor, def)
	for key := range query.Filters {
		if access, ok := profile.Fields[key]; ok && !access.Visible {
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

func (s *Server) modelAccessProfile(actor ActorContext, def model.Definition) securityfields.AccessProfile {
	if s.fieldSecurity == nil {
		return securityfields.AccessProfile{ResourceKind: "model", ResourceKey: def.Key, Fields: map[string]securityfields.FieldAccess{}}
	}
	return s.fieldSecurity.ModelProfile(securityfields.AccessContext{
		ActorID:    effectiveActorID(actor),
		SessionID:  actor.SessionID,
		LocationID: actor.LocationID,
		ScopeID:    actor.LocationID,
		Channel:    "mcp",
		PermissionChecker: func(permissionKey string) bool {
			return strings.TrimSpace(permissionKey) == "" || (actor.PermissionChecker != nil && actor.PermissionChecker(permissionKey))
		},
	}, def)
}

func (s *Server) updateDraftDocumentFallback(current document.Record, actor ActorContext, payload map[string]any, expectedVersion int, expectedETag string) (document.Record, error) {
	if current.Header.Status != "draft" {
		return document.Record{}, shared.Conflict("only draft documents may be updated")
	}
	if expectedVersion > 0 && current.Header.Version != expectedVersion {
		return document.Record{}, shared.Conflict("document version mismatch")
	}
	if expectedETag != "" && current.Header.ETag != expectedETag {
		return document.Record{}, shared.Conflict("document etag mismatch")
	}
	current.Header.Version++
	current.Header.ETag = fmt.Sprintf("%s:%d", current.Header.ID, current.Header.Version)
	current.Header.UpdatedBy = effectiveActorID(actor)
	current.Header.UpdatedAt = time.Now().UTC()
	current.Body.Payload = mergeMap(current.Body.Payload, payload)
	current.Body.ContentHash = document.ContentHash(current.Body.Payload)
	return current, s.documents.Save(current)
}

func validateMCPDocumentPayloadForType(modules *module.Service, documentType string, payload map[string]any) error {
	fields := mcpDocumentValidationFieldsForType(modules, documentType)
	for _, field := range fields {
		path := mcpValidationPayloadPath(field.Path)
		if path == "" || field.ReadOnly {
			continue
		}
		if field.Required && isEmptyValidationValue(resolveValidationPath(payload, path), field.Type) {
			label := strings.TrimSpace(field.Label)
			if label == "" {
				label = path
			}
			return shared.Validation(label + " is required")
		}
	}
	return nil
}

func mcpDocumentValidationFieldsForType(modules *module.Service, documentType string) []module.FieldDefinition {
	if modules == nil || strings.TrimSpace(documentType) == "" {
		return nil
	}
	merged := map[string]module.FieldDefinition{}
	order := make([]string, 0)
	for _, view := range modules.Views() {
		if view.Kind != "form" || view.DocumentType != documentType {
			continue
		}
		for _, field := range collectModuleValidationFields(view.Fields, view.Sections, view.Tabs) {
			path := mcpValidationPayloadPath(field.Path)
			if path == "" || field.ReadOnly {
				continue
			}
			if _, ok := merged[path]; !ok {
				order = append(order, path)
				merged[path] = field
				continue
			}
			current := merged[path]
			current.Required = current.Required || field.Required
			merged[path] = current
		}
	}
	items := make([]module.FieldDefinition, 0, len(order))
	for _, path := range order {
		items = append(items, merged[path])
	}
	return items
}

func collectModuleValidationFields(fields []module.FieldDefinition, sections []module.SectionDefinition, tabs []module.TabDefinition) []module.FieldDefinition {
	items := append([]module.FieldDefinition{}, fields...)
	for _, section := range sections {
		items = append(items, section.Fields...)
	}
	for _, tab := range tabs {
		for _, section := range tab.Sections {
			items = append(items, section.Fields...)
		}
	}
	return items
}

func mcpValidationPayloadPath(path string) string {
	switch {
	case strings.HasPrefix(strings.TrimSpace(path), "body.payload."):
		return strings.TrimPrefix(strings.TrimSpace(path), "body.payload.")
	case strings.HasPrefix(strings.TrimSpace(path), "payload."):
		return strings.TrimPrefix(strings.TrimSpace(path), "payload.")
	default:
		return ""
	}
}

func resolveValidationPath(payload map[string]any, path string) any {
	current := any(payload)
	for _, part := range strings.Split(strings.TrimSpace(path), ".") {
		currentMap, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = currentMap[part]
	}
	return current
}

func isEmptyValidationValue(value any, valueType string) bool {
	switch typed := value.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(typed) == ""
	case []any:
		return len(typed) == 0
	case []string:
		return len(typed) == 0
	case map[string]any:
		return len(typed) == 0
	default:
		return false
	}
}

func moduleOwnedDocumentTypes(manifest module.Manifest) []string {
	items := append([]string{}, manifest.OwnedDocumentTypes...)
	for _, def := range manifest.Documents {
		if !containsString(items, def.Type) {
			items = append(items, def.Type)
		}
	}
	sort.Strings(items)
	return items
}

func (s *Server) documentOwnerModuleKeys(documentType string) []string {
	if s == nil || s.modules == nil {
		return nil
	}
	items := make([]string, 0)
	for _, detail := range s.modules.List() {
		if !detail.Installed.Enabled {
			continue
		}
		if containsString(moduleOwnedDocumentTypes(detail.Manifest), documentType) {
			items = append(items, detail.Manifest.Key)
		}
	}
	sort.Strings(items)
	return items
}

func moduleOwnedModelKeys(manifest module.Manifest) []string {
	items := make([]string, 0, len(manifest.Models))
	for _, def := range manifest.Models {
		if !containsString(items, def.Key) {
			items = append(items, def.Key)
		}
	}
	sort.Strings(items)
	return items
}

func moduleReferenceTypeKeys(manifest module.Manifest) []string {
	items := make([]string, 0, len(manifest.ReferenceTypes))
	for _, def := range manifest.ReferenceTypes {
		items = append(items, def.Key)
	}
	sort.Strings(items)
	return items
}

func moduleDatasetKeys(manifest module.Manifest) []string {
	items := make([]string, 0, len(manifest.Datasets))
	for _, def := range manifest.Datasets {
		items = append(items, def.Key)
	}
	sort.Strings(items)
	return items
}

func reverseModuleDependencies(items []module.Detail) map[string][]string {
	result := map[string][]string{}
	for _, item := range items {
		for _, dep := range item.Manifest.DependencyRequirements {
			result[dep.ModuleKey] = append(result[dep.ModuleKey], item.Manifest.Key)
		}
	}
	for key := range result {
		sort.Strings(result[key])
	}
	return result
}

func summarizePayload(payload map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range payload {
		switch typed := value.(type) {
		case map[string]any:
			out[key] = fmt.Sprintf("%d fields", len(typed))
		case []any:
			out[key] = fmt.Sprintf("%d items", len(typed))
		default:
			out[key] = value
		}
	}
	return out
}

func documentMatchesQuery(record document.Record, query string) bool {
	values := []string{record.Header.ID, record.Header.Type, record.Header.Status, record.Header.Number, stringValue(record.Body.Payload["title"]), stringValue(record.Body.Payload["name"])}
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), query) {
			return true
		}
	}
	raw, _ := json.Marshal(record.Body.Payload)
	return strings.Contains(strings.ToLower(string(raw)), query)
}

func modelMatchesQuery(record model.Record, query string) bool {
	if strings.Contains(strings.ToLower(record.ID), query) {
		return true
	}
	raw, _ := json.Marshal(record.Values)
	return strings.Contains(strings.ToLower(string(raw)), query)
}

func paginateBusinessRecordSummaries(items []businessRecordSummary, page, pageSize int) []businessRecordSummary {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	start := (page - 1) * pageSize
	if start >= len(items) {
		return []businessRecordSummary{}
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}
	return items[start:end]
}

func relatedModelDefinition(models *model.Service, def model.Definition, relationKey string) (model.Definition, bool) {
	for _, relation := range def.Relations {
		if relation.Key == relationKey {
			return models.Definition(relation.TargetModelKey)
		}
	}
	return model.Definition{}, false
}

func modelQueryFromArguments(arguments map[string]any) model.Query {
	query := model.Query{
		Filters:  stringMapString(arguments["filters"]),
		SortKey:  stringArg(arguments, "sort_key"),
		Desc:     boolArg(arguments, "desc"),
		Page:     intArg(arguments, "page"),
		PageSize: intArg(arguments, "page_size"),
	}
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 {
		query.PageSize = model.DefaultPageSize
	}
	return query
}

func mapArg(arguments map[string]any, key string) map[string]any {
	value, _ := arguments[key]
	return cloneMap(stringMapAny(value))
}

func stringMapAny(value any) map[string]any {
	switch typed := value.(type) {
	case map[string]any:
		return typed
	default:
		return map[string]any{}
	}
}

func stringMapString(value any) map[string]string {
	switch typed := value.(type) {
	case map[string]string:
		out := make(map[string]string, len(typed))
		for key, item := range typed {
			out[strings.TrimSpace(key)] = strings.TrimSpace(item)
		}
		return out
	case map[string]any:
		out := make(map[string]string, len(typed))
		for key, item := range typed {
			if text, ok := item.(string); ok {
				out[strings.TrimSpace(key)] = strings.TrimSpace(text)
			}
		}
		return out
	default:
		return map[string]string{}
	}
}

func mergeMap(base, patch map[string]any) map[string]any {
	merged := cloneMap(base)
	for key, value := range patch {
		if current, ok := merged[key].(map[string]any); ok {
			if next, ok := value.(map[string]any); ok {
				merged[key] = mergeMap(current, next)
				continue
			}
		}
		merged[key] = value
	}
	return merged
}

func effectiveActorID(actor ActorContext) string {
	return firstNonEmpty(strings.TrimSpace(actor.EffectiveUserID), strings.TrimSpace(actor.ActorID))
}

func firstString(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}
