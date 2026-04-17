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
	platformsearch "orbyte/internal/platform/search"
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
	RecommendedTools     []string                       `json:"recommended_tools,omitempty"`
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
		moduleName := detail.Manifest.Name
		items = append(items,
			syntheticToolDefinition{
				Name:                moduleKey + ".business.info.get",
				Title:               moduleName + " Business Info",
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
				Title:               moduleName + " Business Records",
				Description:         businessRecordSearchDescription(moduleKey),
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
					Title:               moduleName + " Business Documents",
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
					Title:               moduleName + " Create Draft Document",
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
					Title:               moduleName + " Update Draft Document",
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
		switch moduleKey {
		case "planning_core":
			mk := moduleKey
			items = append(items,
				syntheticToolDefinition{
					Name:                mk + ".replenishment.insight.summary",
					Title:               moduleName + " Replenishment Insight Summary",
					Description:         "Summarize replenishment risk, healthy items, shortage pressure, and vendor readiness for a warehouse. Use this first when deciding what needs replenishment now.",
					ModuleKey:           mk,
					RequiredPermissions: []string{"module.read"},
					InputSchema: map[string]any{"type": "object", "properties": map[string]any{
						"warehouse_code":            map[string]any{"type": "string"},
						"item_code":                 map[string]any{"type": "string"},
						"category_code":             map[string]any{"type": "string"},
						"coverage_status":           map[string]any{"type": "string"},
						"shortage_only":             map[string]any{"type": "boolean"},
						"has_inbound_only":          map[string]any{"type": "boolean"},
						"has_preferred_vendor_only": map[string]any{"type": "boolean"},
						"limit":                     map[string]any{"type": "integer"},
					}},
					Handler: func(s *Server, actor ActorContext, arguments map[string]any) (map[string]any, error) {
						return s.replenishmentInsightSummary(actor, arguments)
					},
				},
				syntheticToolDefinition{
					Name:                mk + ".replenishment.plan.summary",
					Title:               moduleName + " Replenishment Plan Summary",
					Description:         "Recommend replenishment quantities and vendor grouping based on current shortage rows. Use this after the insight summary to turn risk signals into an executable replenishment plan.",
					ModuleKey:           mk,
					RequiredPermissions: []string{"module.read"},
					InputSchema: map[string]any{"type": "object", "properties": map[string]any{
						"warehouse_code":            map[string]any{"type": "string"},
						"item_code":                 map[string]any{"type": "string"},
						"category_code":             map[string]any{"type": "string"},
						"coverage_status":           map[string]any{"type": "string"},
						"shortage_only":             map[string]any{"type": "boolean"},
						"has_inbound_only":          map[string]any{"type": "boolean"},
						"has_preferred_vendor_only": map[string]any{"type": "boolean"},
						"limit":                     map[string]any{"type": "integer"},
					}},
					Handler: func(s *Server, actor ActorContext, arguments map[string]any) (map[string]any, error) {
						return s.replenishmentPlanSummary(actor, arguments)
					},
				},
				syntheticToolDefinition{
					Name:                mk + ".purchase_requests.draft.create",
					Title:               moduleName + " Create Draft Purchase Requests",
					Description:         "Create draft purchase requests from replenishment selections. Use this only after confirming the recommended items and quantities.",
					ModuleKey:           mk,
					RequiredPermissions: []string{"document.create"},
					InputSchema: map[string]any{"type": "object", "properties": map[string]any{
						"selections": map[string]any{"type": "array", "items": map[string]any{"type": "object", "properties": map[string]any{
							"item_code":      map[string]any{"type": "string"},
							"warehouse_code": map[string]any{"type": "string"},
							"quantity":       map[string]any{"type": "number"},
						}, "required": []string{"item_code", "warehouse_code", "quantity"}}},
					}},
					Handler: func(s *Server, actor ActorContext, arguments map[string]any) (map[string]any, error) {
						return s.replenishmentPurchaseRequestDraftCreate(actor, arguments)
					},
				},
			)
		case "pos_core":
			mk := moduleKey
			items = append(items, syntheticToolDefinition{
				Name:                mk + ".sales.strategy.summary",
				Title:               moduleName + " Sales Strategy Summary",
				Description:         "Summarize POS sales, best-selling items, repeat bundle patterns, and member segment behavior for campaign planning. Use this before recommending a promotion strategy.",
				ModuleKey:           mk,
				RequiredPermissions: []string{"module.read"},
				InputSchema: map[string]any{"type": "object", "properties": map[string]any{
					"date_from":     map[string]any{"type": "string"},
					"date_to":       map[string]any{"type": "string"},
					"store_code":    map[string]any{"type": "string"},
					"register_code": map[string]any{"type": "string"},
					"limit":         map[string]any{"type": "integer"},
				}},
				Handler: func(s *Server, actor ActorContext, arguments map[string]any) (map[string]any, error) {
					return s.posSalesStrategySummary(actor, arguments)
				},
			})
		case "promotion_core":
			mk := moduleKey
			items = append(items,
				syntheticToolDefinition{
					Name:                mk + ".performance.summary",
					Title:               moduleName + " Promotion Performance Summary",
					Description:         "Summarize promotion campaigns, codes, redemptions, and likely underperforming offers. Use this when deciding which current promotion to replace.",
					ModuleKey:           mk,
					RequiredPermissions: []string{"module.read"},
					InputSchema: map[string]any{"type": "object", "properties": map[string]any{
						"store_code": map[string]any{"type": "string"},
						"limit":      map[string]any{"type": "integer"},
					}},
					Handler: func(s *Server, actor ActorContext, arguments map[string]any) (map[string]any, error) {
						return s.promotionPerformanceSummary(actor, arguments)
					},
				},
				syntheticToolDefinition{
					Name:                mk + ".strategy.plan.draft.create",
					Title:               moduleName + " Create Promotion Plan Draft",
					Description:         "Create a draft generic_request promotion strategy plan. Use this when asked to create a draft recommendation or promotion plan artifact.",
					ModuleKey:           mk,
					RequiredPermissions: []string{"document.create"},
					InputSchema: map[string]any{"type": "object", "properties": map[string]any{
						"title":             map[string]any{"type": "string"},
						"summary":           map[string]any{"type": "string"},
						"target_products":   map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
						"target_segment":    map[string]any{"type": "string"},
						"replaced_campaign": map[string]any{"type": "string"},
						"campaign_kind":     map[string]any{"type": "string"},
						"organization_id":   map[string]any{"type": "string"},
						"location_id":       map[string]any{"type": "string"},
						"confirm_apply":     map[string]any{"type": "boolean"},
					}, "required": []string{"title", "confirm_apply"}},
					Handler: func(s *Server, actor ActorContext, arguments map[string]any) (map[string]any, error) {
						return s.promotionStrategyPlanDraftCreate(actor, arguments)
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
			item.RecommendedTools = s.recommendedModuleToolNames(item.Key, actor)
			return map[string]any{
				"content":           []ContentBlock{{Type: "text", Text: businessModuleGetText(moduleKey, item.RecommendedTools)}},
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
		"content":           []ContentBlock{{Type: "text", Text: renderBusinessRecordSearchText("business documents", items)}},
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
	locationID := firstNonEmpty(stringArg(arguments, "location_id"), strings.TrimSpace(actor.LocationID), "loc_hq")
	organizationID := firstNonEmpty(stringArg(arguments, "organization_id"), strings.TrimSpace(actor.OrganizationID), "org_default")
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
	record, err := s.documents.Create(documentType, organizationID, locationID, documentWriteActorID(actor), payload)
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
	case "":
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
			"content":           []ContentBlock{{Type: "text", Text: renderBusinessRecordSearchText("business records", items)}},
			"structuredContent": map[string]any{"items": items},
		}, true, nil
	case "document":
		items, err := s.filteredDocumentSummaries(actor, arguments)
		if err != nil {
			return nil, true, err
		}
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
			"content":           []ContentBlock{{Type: "text", Text: renderBusinessRecordSearchText("business records", items)}},
			"structuredContent": map[string]any{"items": items},
		}, true, nil
	case "model":
		items, err := s.filteredModelSummaries(actor, arguments)
		if err != nil {
			return nil, true, err
		}
		return map[string]any{
			"content":           []ContentBlock{{Type: "text", Text: renderBusinessRecordSearchText("business records", items)}},
			"structuredContent": map[string]any{"items": items},
		}, true, nil
	default:
		return nil, true, shared.Validation("resource_kind must be document or model; for POS sales or promotion strategy use pos_core.sales.strategy.summary or promotion_core.performance.summary")
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

func (s *Server) commercialItemSearch(actor ActorContext, arguments map[string]any) (map[string]any, error) {
	if s == nil || s.models == nil {
		return nil, fmt.Errorf("models are unavailable")
	}
	query := strings.TrimSpace(stringArg(arguments, "query"))
	sku := strings.TrimSpace(stringArg(arguments, "sku"))
	productCode := strings.TrimSpace(stringArg(arguments, "product_code"))
	if query == "" && sku == "" && productCode == "" {
		return nil, shared.Validation("query, sku, or product_code is required")
	}
	def, items, err := s.listCommercialItems(actor)
	if err != nil {
		return nil, err
	}
	indexRanks, indexScores := s.commercialItemSearchIndex(actor, query)
	type match struct {
		record     model.Record
		matchKind  string
		rank       int
		indexScore float64
	}
	matches := make([]match, 0, len(items))
	for _, item := range items {
		if !commercialItemIsActiveSellable(item) {
			continue
		}
		matchKind, rank := commercialItemSearchMatch(item, query, sku, productCode)
		if matchKind == "" {
			if hitRank, ok := indexRanks[item.ID]; ok {
				matchKind = "index"
				rank = 300 - hitRank
			} else if query != "" && commercialItemContainsQuery(item, query) {
				matchKind = "contains"
				rank = 200
			} else {
				continue
			}
		}
		matches = append(matches, match{
			record:     item,
			matchKind:  matchKind,
			rank:       rank,
			indexScore: indexScores[item.ID],
		})
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].rank != matches[j].rank {
			return matches[i].rank > matches[j].rank
		}
		if matches[i].indexScore != matches[j].indexScore {
			return matches[i].indexScore > matches[j].indexScore
		}
		return matches[i].record.UpdatedAt.After(matches[j].record.UpdatedAt)
	})
	page := positiveIntArg(arguments, "page", 1)
	pageSize := positiveIntArg(arguments, "page_size", 10)
	if pageSize > 25 {
		pageSize = 25
	}
	total := len(matches)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	resultItems := make([]map[string]any, 0, end-start)
	for _, item := range matches[start:end] {
		resultItems = append(resultItems, commercialItemSearchPayload(item.record, item.matchKind))
	}
	summary := fmt.Sprintf("Found %d commercial items.", total)
	if len(resultItems) > 0 {
		summary += " Top matches: " + strings.Join(commercialItemNames(resultItems, 3), "; ")
	}
	_ = def
	return map[string]any{
		"content": []ContentBlock{{Type: "text", Text: summary}},
		"structuredContent": map[string]any{
			"items":     resultItems,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		},
	}, nil
}

func (s *Server) commercialItemGet(actor ActorContext, arguments map[string]any) (map[string]any, error) {
	if s == nil || s.models == nil {
		return nil, fmt.Errorf("models are unavailable")
	}
	_, item, err := s.resolveCommercialItem(actor, arguments)
	if err != nil {
		return nil, err
	}
	payload := commercialItemDetailPayload(item)
	return map[string]any{
		"content": []ContentBlock{{
			Type: "text",
			Text: fmt.Sprintf("Loaded commercial item %s.", firstNonEmpty(stringValue(item.Values["name"]), item.ID)),
		}},
		"structuredContent": payload,
	}, nil
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

func businessRecordSearchDescription(moduleKey string) string {
	switch strings.TrimSpace(moduleKey) {
	case "pos_core":
		return "Search business records owned by pos_core. For campaign planning, prefer pos_core.sales.strategy.summary over generic record search."
	case "promotion_core":
		return "Search business records owned by promotion_core. For replacement analysis, prefer promotion_core.performance.summary over generic record search."
	default:
		return "Search business records owned by " + moduleKey + "."
	}
}

func (s *Server) recommendedModuleToolNames(moduleKey string, actor ActorContext) []string {
	defs := s.syntheticToolDefinitions(actor)
	items := make([]string, 0)
	for _, def := range defs {
		if def.ModuleKey == moduleKey {
			items = append(items, def.Name)
		}
	}
	sort.Strings(items)
	return items
}

func businessModuleGetText(moduleKey string, recommendedTools []string) string {
	text := fmt.Sprintf("Loaded business module %s.", moduleKey)
	if len(recommendedTools) == 0 {
		return text
	}
	limit := len(recommendedTools)
	if limit > 3 {
		limit = 3
	}
	return text + " Recommended tools: " + strings.Join(recommendedTools[:limit], ", ") + "."
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
	filters := mapArg(arguments, "filters")
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
		if len(filters) > 0 && !documentMatchesFilters(item, filters) {
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

func (s *Server) listModelRecordsForQueryText(modelKey string, query model.Query) ([]model.Record, error) {
	if s == nil || s.models == nil {
		return nil, nil
	}
	pageSize := query.PageSize
	if pageSize <= 0 || pageSize > model.MaxPageSize {
		pageSize = model.MaxPageSize
	}
	page := 1
	var records []model.Record
	for {
		pagedQuery := query
		pagedQuery.Page = page
		pagedQuery.PageSize = pageSize
		items, total, err := s.models.List(modelKey, pagedQuery)
		if err != nil {
			return nil, err
		}
		records = append(records, items...)
		if len(items) == 0 || len(records) >= total || len(items) < pageSize {
			break
		}
		page++
	}
	return records, nil
}

func (s *Server) listCommercialItems(actor ActorContext) (model.Definition, []model.Record, error) {
	if s == nil || s.models == nil {
		return model.Definition{}, nil, fmt.Errorf("models are unavailable")
	}
	def, ok := s.models.Definition("commercial_item")
	if !ok {
		return model.Definition{}, nil, shared.NotFound("commercial_item definition not found")
	}
	if !allowsAll(actor.PermissionChecker, []string{def.ListPermissionKey}) {
		return model.Definition{}, nil, fmt.Errorf("tool is not allowed")
	}
	items, err := s.listModelRecordsForQueryText(def.Key, model.Query{Page: 1, PageSize: model.MaxPageSize})
	if err != nil {
		return model.Definition{}, nil, err
	}
	return def, s.sanitizeModelRecords(actor, def, items), nil
}

func (s *Server) resolveCommercialItem(actor ActorContext, arguments map[string]any) (model.Definition, model.Record, error) {
	if s == nil || s.models == nil {
		return model.Definition{}, model.Record{}, fmt.Errorf("models are unavailable")
	}
	recordID := strings.TrimSpace(stringArg(arguments, "record_id"))
	sku := strings.TrimSpace(stringArg(arguments, "sku"))
	productCode := strings.TrimSpace(stringArg(arguments, "product_code"))
	name := strings.TrimSpace(stringArg(arguments, "name"))
	selectors := 0
	for _, value := range []string{recordID, sku, productCode, name} {
		if value != "" {
			selectors++
		}
	}
	if selectors != 1 {
		return model.Definition{}, model.Record{}, shared.Validation("exactly one of record_id, sku, product_code, or name is required")
	}
	def, ok := s.models.Definition("commercial_item")
	if !ok {
		return model.Definition{}, model.Record{}, shared.NotFound("commercial_item definition not found")
	}
	if recordID != "" {
		if !allowsAll(actor.PermissionChecker, []string{def.ReadPermissionKey}) {
			return model.Definition{}, model.Record{}, fmt.Errorf("tool is not allowed")
		}
		record, err := s.models.Get(def.Key, recordID)
		if err != nil {
			return model.Definition{}, model.Record{}, err
		}
		return def, s.sanitizeModelRecord(actor, def, record), nil
	}
	_, items, err := s.listCommercialItems(actor)
	if err != nil {
		return model.Definition{}, model.Record{}, err
	}
	matches := make([]model.Record, 0, 1)
	for _, item := range items {
		if !commercialItemIsActiveSellable(item) {
			continue
		}
		switch {
		case sku != "" && strings.EqualFold(stringValue(item.Values["sku"]), sku):
			matches = append(matches, item)
		case productCode != "" && strings.EqualFold(stringValue(item.Values["product_code"]), productCode):
			matches = append(matches, item)
		case name != "" && strings.EqualFold(stringValue(item.Values["name"]), name):
			matches = append(matches, item)
		}
	}
	if len(matches) == 0 {
		return model.Definition{}, model.Record{}, shared.NotFound("commercial item not found")
	}
	if len(matches) > 1 {
		selectorName := "name"
		if sku != "" {
			selectorName = "sku"
		} else if productCode != "" {
			selectorName = "product_code"
		}
		return model.Definition{}, model.Record{}, shared.Validation("commercial item " + selectorName + " is ambiguous")
	}
	return def, matches[0], nil
}

func (s *Server) commercialItemSearchIndex(actor ActorContext, query string) (map[string]int, map[string]float64) {
	ranks := map[string]int{}
	scores := map[string]float64{}
	query = strings.TrimSpace(query)
	if s == nil || s.search == nil || query == "" {
		return ranks, scores
	}
	result, err := s.search.Query("commercial.items.search", strings.TrimSpace(actor.OrganizationID), strings.TrimSpace(actor.LocationID), platformsearch.QueryRequest{
		Query:    query,
		Page:     1,
		PageSize: 50,
	})
	if err != nil {
		return ranks, scores
	}
	for index, hit := range result.Hits {
		ranks[hit.SourceID] = index
		scores[hit.SourceID] = hit.Score
	}
	return ranks, scores
}

func commercialItemSearchMatch(record model.Record, query, sku, productCode string) (string, int) {
	itemSKU := stringValue(record.Values["sku"])
	itemProductCode := stringValue(record.Values["product_code"])
	itemName := stringValue(record.Values["name"])
	switch {
	case sku != "" && strings.EqualFold(itemSKU, sku):
		return "exact_sku", 1000
	case productCode != "" && strings.EqualFold(itemProductCode, productCode):
		return "exact_product_code", 950
	case query != "" && strings.EqualFold(itemName, query):
		return "exact_name", 900
	case sku != "" && strings.HasPrefix(strings.ToLower(itemSKU), strings.ToLower(sku)):
		return "prefix_sku", 850
	case productCode != "" && strings.HasPrefix(strings.ToLower(itemProductCode), strings.ToLower(productCode)):
		return "prefix_product_code", 800
	case query != "" && strings.HasPrefix(strings.ToLower(itemName), strings.ToLower(query)):
		return "prefix_name", 750
	default:
		return "", 0
	}
}

func commercialItemContainsQuery(record model.Record, query string) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return false
	}
	for _, candidate := range []string{
		stringValue(record.Values["sku"]),
		stringValue(record.Values["product_code"]),
		stringValue(record.Values["name"]),
		stringValue(record.Values["description"]),
		stringValue(record.Values["variant_label"]),
	} {
		if strings.Contains(strings.ToLower(candidate), query) {
			return true
		}
	}
	return false
}

func commercialItemIsActiveSellable(record model.Record) bool {
	if !boolValue(record.Values["is_sellable"]) {
		return false
	}
	status := strings.TrimSpace(stringValue(record.Values["status"]))
	return status == "" || strings.EqualFold(status, "active")
}

func commercialItemSearchPayload(record model.Record, matchKind string) map[string]any {
	return map[string]any{
		"record_id":     record.ID,
		"product_code":  stringValue(record.Values["product_code"]),
		"sku":           stringValue(record.Values["sku"]),
		"name":          stringValue(record.Values["name"]),
		"description":   stringValue(record.Values["description"]),
		"item_type":     stringValue(record.Values["item_type"]),
		"category_code": stringValue(record.Values["category_code"]),
		"base_price":    record.Values["base_price"],
		"currency_code": stringValue(record.Values["currency_code"]),
		"status":        stringValue(record.Values["status"]),
		"match_kind":    matchKind,
	}
}

func commercialItemDetailPayload(record model.Record) map[string]any {
	return map[string]any{
		"record_id":               record.ID,
		"product_code":            stringValue(record.Values["product_code"]),
		"sku":                     stringValue(record.Values["sku"]),
		"name":                    stringValue(record.Values["name"]),
		"description":             stringValue(record.Values["description"]),
		"item_type":               stringValue(record.Values["item_type"]),
		"category_code":           stringValue(record.Values["category_code"]),
		"variant_label":           stringValue(record.Values["variant_label"]),
		"is_sellable":             boolValue(record.Values["is_sellable"]),
		"uom_code":                stringValue(record.Values["uom_code"]),
		"base_price":              record.Values["base_price"],
		"currency_code":           stringValue(record.Values["currency_code"]),
		"inventory_enabled":       boolValue(record.Values["inventory_enabled"]),
		"inventory_tracking_mode": stringValue(record.Values["inventory_tracking_mode"]),
		"replenishment_enabled":   boolValue(record.Values["replenishment_enabled"]),
		"replenishment_mode":      stringValue(record.Values["replenishment_mode"]),
		"target_stock_quantity":   record.Values["target_stock_quantity"],
		"status":                  stringValue(record.Values["status"]),
		"record":                  record,
	}
}

func commercialItemNames(items []map[string]any, limit int) []string {
	names := make([]string, 0, limit)
	for _, item := range items {
		if len(names) >= limit {
			break
		}
		name := strings.TrimSpace(anyString(item["name"]))
		if name == "" {
			continue
		}
		matchKind := strings.TrimSpace(anyString(item["match_kind"]))
		if matchKind != "" {
			name += " [" + matchKind + "]"
		}
		names = append(names, name)
	}
	return names
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

func renderBusinessRecordSearchText(label string, items []businessRecordSummary) string {
	if len(items) == 0 {
		return fmt.Sprintf("Found 0 %s.", label)
	}
	limit := len(items)
	if limit > 5 {
		limit = 5
	}
	lines := []string{fmt.Sprintf("Found %d %s.", len(items), label)}
	for _, item := range items[:limit] {
		line := item.RecordID
		if item.ResourceKey != "" {
			line = item.ResourceKey + " " + line
		}
		if item.Title != "" {
			line += " - " + item.Title
		}
		if item.Status != "" {
			line += " [" + item.Status + "]"
		}
		lines = append(lines, "- "+line)
	}
	if len(items) > limit {
		lines = append(lines, fmt.Sprintf("- ... %d more", len(items)-limit))
	}
	return strings.Join(lines, "\n")
}

type replenishmentRecommendation struct {
	ItemCode            string   `json:"item_code"`
	ItemName            string   `json:"item_name"`
	WarehouseCode       string   `json:"warehouse_code"`
	RecommendedQuantity float64  `json:"recommended_quantity"`
	ShortageQuantity    float64  `json:"shortage_quantity"`
	ForecastDemand      float64  `json:"forecast_demand_quantity"`
	AvailableQuantity   float64  `json:"available_quantity"`
	CoverageStatus      string   `json:"coverage_status"`
	PreferredVendorID   string   `json:"preferred_vendor_id,omitempty"`
	PreferredVendorName string   `json:"preferred_vendor_name,omitempty"`
	LeadTimeDays        float64  `json:"lead_time_days,omitempty"`
	RecommendedOrderBy  string   `json:"recommended_order_by_date,omitempty"`
	TimeCritical        bool     `json:"time_critical"`
	PurchaseRequestRefs []string `json:"purchase_request_refs,omitempty"`
	Rationale           string   `json:"rationale"`
}

func replenishmentQueryFromArguments(arguments map[string]any) (string, string, string, string, bool, bool, bool) {
	return strings.TrimSpace(stringArg(arguments, "warehouse_code")),
		strings.TrimSpace(stringArg(arguments, "item_code")),
		strings.TrimSpace(stringArg(arguments, "category_code")),
		strings.TrimSpace(stringArg(arguments, "coverage_status")),
		boolArg(arguments, "shortage_only"),
		boolArg(arguments, "has_inbound_only"),
		boolArg(arguments, "has_preferred_vendor_only")
}

func replenishmentPurchaseRequestRefs(value any) []string {
	refs := recordList(value)
	items := make([]string, 0, len(refs))
	for _, ref := range refs {
		number := strings.TrimSpace(textValue(ref["number"]))
		if number == "" {
			number = strings.TrimSpace(textValue(ref["id"]))
		}
		if number != "" {
			items = append(items, number)
		}
	}
	sort.Strings(items)
	return items
}

func canonicalReplenishmentKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "_", " ")
	value = strings.ReplaceAll(value, "-", " ")
	return strings.Join(strings.Fields(value), " ")
}

func replenishmentRecommendationFromRow(row map[string]any) replenishmentRecommendation {
	itemName := firstNonEmptyString(textValue(row["item_name"]), textValue(row["item_code"]))
	vendorName := strings.TrimSpace(textValue(row["preferred_vendor_name"]))
	coverage := strings.TrimSpace(textValue(row["coverage_status"]))
	forecast := roundBusinessMoney(numberValue(row["forecast_demand_quantity"]))
	available := roundBusinessMoney(numberValue(row["available_quantity"]))
	shortage := roundBusinessMoney(numberValue(row["shortage_quantity"]))
	recommended := roundBusinessMoney(numberValue(row["normalized_request_quantity"]))
	rationaleParts := []string{
		fmt.Sprintf("available %.0f", available),
		fmt.Sprintf("forecast %.0f", forecast),
		fmt.Sprintf("shortage %.0f", shortage),
	}
	if vendorName != "" {
		rationaleParts = append(rationaleParts, "vendor "+vendorName)
	}
	if boolValue(row["time_critical"]) {
		rationaleParts = append(rationaleParts, "time critical")
	}
	return replenishmentRecommendation{
		ItemCode:            strings.TrimSpace(textValue(row["item_code"])),
		ItemName:            itemName,
		WarehouseCode:       strings.TrimSpace(textValue(row["warehouse_code"])),
		RecommendedQuantity: recommended,
		ShortageQuantity:    shortage,
		ForecastDemand:      forecast,
		AvailableQuantity:   available,
		CoverageStatus:      coverage,
		PreferredVendorID:   strings.TrimSpace(textValue(row["preferred_vendor_id"])),
		PreferredVendorName: vendorName,
		LeadTimeDays:        roundBusinessMoney(numberValue(row["lead_time_days"])),
		RecommendedOrderBy:  strings.TrimSpace(textValue(row["recommended_order_by_date"])),
		TimeCritical:        boolValue(row["time_critical"]),
		PurchaseRequestRefs: replenishmentPurchaseRequestRefs(row["purchase_request_refs"]),
		Rationale:           strings.Join(rationaleParts, ", "),
	}
}

func (s *Server) replenishmentInsightSummary(actor ActorContext, arguments map[string]any) (map[string]any, error) {
	if s == nil || s.planning == nil {
		return nil, fmt.Errorf("planning is unavailable")
	}
	warehouseCode, itemCode, categoryCode, coverageStatus, shortageOnly, hasInboundOnly, hasPreferredVendorOnly := replenishmentQueryFromArguments(arguments)
	summary := s.planning.ReplenishmentSummaryScoped(firstNonEmpty(actor.OrganizationID, "org_default"), firstNonEmpty(actor.LocationID, "loc_hq"), warehouseCode, itemCode, categoryCode, coverageStatus, shortageOnly, hasInboundOnly, hasPreferredVendorOnly, time.Now().UTC())
	atRisk := make([]replenishmentRecommendation, 0)
	healthy := make([]map[string]any, 0)
	limit := intArg(arguments, "limit")
	if limit <= 0 {
		limit = 5
	}
	for _, row := range summary.Items {
		if roundBusinessMoney(numberValue(row["normalized_request_quantity"])) > 0 {
			atRisk = append(atRisk, replenishmentRecommendationFromRow(row))
			continue
		}
		healthy = append(healthy, map[string]any{
			"item_code":          strings.TrimSpace(textValue(row["item_code"])),
			"item_name":          firstNonEmptyString(textValue(row["item_name"]), textValue(row["item_code"])),
			"warehouse_code":     strings.TrimSpace(textValue(row["warehouse_code"])),
			"available_quantity": roundBusinessMoney(numberValue(row["available_quantity"])),
			"coverage_status":    strings.TrimSpace(textValue(row["coverage_status"])),
		})
	}
	if len(atRisk) > limit {
		atRisk = atRisk[:limit]
	}
	if len(healthy) > limit {
		healthy = healthy[:limit]
	}
	text := fmt.Sprintf("Reviewed %d replenishment candidates across %d warehouse(s). %d item(s) are at active replenishment risk with %.0f suggested units total.", summary.CandidateCount, summary.WarehouseCount, summary.ShortageItemCount, roundBusinessMoney(summary.TotalSuggestedRequestQuantity))
	if len(atRisk) > 0 {
		names := make([]string, 0, len(atRisk))
		for _, item := range atRisk {
			names = append(names, fmt.Sprintf("%s (%.0f)", item.ItemName, item.RecommendedQuantity))
		}
		text += fmt.Sprintf("\nTop replenishment risks: %s.", strings.Join(names, ", "))
	}
	if len(healthy) > 0 {
		names := make([]string, 0, len(healthy))
		for _, item := range healthy {
			names = append(names, firstNonEmptyString(textValue(item["item_name"]), textValue(item["item_code"])))
		}
		text += fmt.Sprintf("\nHealthy / skip for now: %s.", strings.Join(names, ", "))
	}
	return map[string]any{
		"content": []ContentBlock{{Type: "text", Text: text}},
		"structuredContent": map[string]any{
			"summary":       summary,
			"at_risk_items": atRisk,
			"healthy_items": healthy,
		},
	}, nil
}

func (s *Server) replenishmentPlanSummary(actor ActorContext, arguments map[string]any) (map[string]any, error) {
	if s == nil || s.planning == nil {
		return nil, fmt.Errorf("planning is unavailable")
	}
	warehouseCode, itemCode, categoryCode, coverageStatus, shortageOnly, hasInboundOnly, hasPreferredVendorOnly := replenishmentQueryFromArguments(arguments)
	summary := s.planning.ReplenishmentSummaryScoped(firstNonEmpty(actor.OrganizationID, "org_default"), firstNonEmpty(actor.LocationID, "loc_hq"), warehouseCode, itemCode, categoryCode, coverageStatus, shortageOnly, hasInboundOnly, hasPreferredVendorOnly, time.Now().UTC())
	limit := intArg(arguments, "limit")
	if limit <= 0 {
		limit = 5
	}
	recommended := make([]replenishmentRecommendation, 0)
	groups := map[string]map[string]any{}
	for _, row := range summary.Items {
		if roundBusinessMoney(numberValue(row["normalized_request_quantity"])) <= 0 {
			continue
		}
		item := replenishmentRecommendationFromRow(row)
		recommended = append(recommended, item)
		groupKey := strings.TrimSpace(item.WarehouseCode) + "|" + strings.TrimSpace(item.PreferredVendorID)
		group := groups[groupKey]
		if group == nil {
			group = map[string]any{
				"warehouse_code":         item.WarehouseCode,
				"preferred_vendor_id":    item.PreferredVendorID,
				"preferred_vendor_name":  item.PreferredVendorName,
				"recommended_items":      []map[string]any{},
				"recommended_line_count": 0,
				"recommended_total_qty":  0.0,
			}
			groups[groupKey] = group
		}
		group["recommended_items"] = append(group["recommended_items"].([]map[string]any), map[string]any{
			"item_code": item.ItemCode,
			"item_name": item.ItemName,
			"quantity":  item.RecommendedQuantity,
		})
		group["recommended_line_count"] = anyInt(group["recommended_line_count"]) + 1
		group["recommended_total_qty"] = roundBusinessMoney(numberValue(group["recommended_total_qty"]) + item.RecommendedQuantity)
	}
	if len(recommended) > limit {
		recommended = recommended[:limit]
	}
	groupList := make([]map[string]any, 0, len(groups))
	for _, group := range groups {
		groupList = append(groupList, group)
	}
	sort.Slice(groupList, func(i, j int) bool {
		left := numberValue(groupList[i]["recommended_total_qty"])
		right := numberValue(groupList[j]["recommended_total_qty"])
		if left != right {
			return left > right
		}
		return textValue(groupList[i]["warehouse_code"]) < textValue(groupList[j]["warehouse_code"])
	})
	text := fmt.Sprintf("Built a replenishment plan with %d recommended line(s) totaling %.0f units.", len(recommended), roundBusinessMoney(summary.TotalSuggestedRequestQuantity))
	if len(recommended) > 0 {
		names := make([]string, 0, len(recommended))
		for _, item := range recommended {
			if item.PreferredVendorName != "" {
				names = append(names, fmt.Sprintf("%s %.0f via %s", item.ItemName, item.RecommendedQuantity, item.PreferredVendorName))
			} else {
				names = append(names, fmt.Sprintf("%s %.0f", item.ItemName, item.RecommendedQuantity))
			}
		}
		text += fmt.Sprintf("\nRecommended replenishment lines: %s.", strings.Join(names, ", "))
	}
	return map[string]any{
		"content": []ContentBlock{{Type: "text", Text: text}},
		"structuredContent": map[string]any{
			"recommended_lines": recommended,
			"request_groups":    groupList,
			"summary": map[string]any{
				"recommended_line_count": len(recommended),
				"recommended_total_qty":  roundBusinessMoney(summary.TotalSuggestedRequestQuantity),
				"warehouse_code":         warehouseCode,
			},
		},
	}, nil
}

func (s *Server) replenishmentPurchaseRequestDraftCreate(actor ActorContext, arguments map[string]any) (map[string]any, error) {
	if s == nil || s.planning == nil || s.documents == nil {
		return nil, fmt.Errorf("planning is unavailable")
	}
	if !allowsAll(actor.PermissionChecker, []string{"document.create"}) {
		return nil, fmt.Errorf("tool is not allowed")
	}
	rawSelections, _ := arguments["selections"].([]any)
	selections := make([]application.ReplenishmentSelection, 0, len(rawSelections))
	for _, item := range rawSelections {
		row, ok := item.(map[string]any)
		if !ok {
			continue
		}
		selections = append(selections, application.ReplenishmentSelection{
			ItemCode:      strings.TrimSpace(stringValue(row["item_code"])),
			WarehouseCode: strings.TrimSpace(stringValue(row["warehouse_code"])),
			Quantity:      roundBusinessMoney(numberValue(row["quantity"])),
		})
	}
	if len(selections) == 0 {
		return nil, shared.Validation("at least one replenishment selection is required")
	}
	organizationID := firstNonEmpty(actor.OrganizationID, "org_default")
	locationID := firstNonEmpty(actor.LocationID, "loc_hq")
	type selectedRecommendation struct {
		Recommendation replenishmentRecommendation
		Quantity       float64
	}
	recommendations := map[string]selectedRecommendation{}
	for _, selection := range selections {
		selectionKey := canonicalReplenishmentKey(selection.ItemCode)
		var matched *replenishmentRecommendation
		queryCodes := []string{selection.ItemCode, ""}
		if s.models != nil && selectionKey != "" {
			if items, _, err := s.models.List("commercial_item", model.Query{Page: 1, PageSize: model.MaxPageSize}); err == nil {
				for _, item := range items {
					nameKey := canonicalReplenishmentKey(stringValue(item.Values["name"]))
					sku := strings.TrimSpace(stringValue(item.Values["sku"]))
					if sku == "" {
						continue
					}
					if nameKey == selectionKey || strings.Contains(nameKey, selectionKey) || strings.Contains(selectionKey, nameKey) {
						queryCodes = append([]string{sku}, queryCodes...)
						break
					}
				}
			}
		}
		for _, queryItemCode := range queryCodes {
			summary := s.planning.ReplenishmentSummaryScoped(organizationID, locationID, selection.WarehouseCode, queryItemCode, "", "", false, false, false, time.Now().UTC())
			for _, row := range summary.Items {
				itemCode := strings.TrimSpace(textValue(row["item_code"]))
				itemName := firstNonEmptyString(textValue(row["item_name"]), itemCode)
				itemCodeKey := canonicalReplenishmentKey(itemCode)
				itemNameKey := canonicalReplenishmentKey(itemName)
				codeMatches := itemCode == queryItemCode || itemCode == selection.ItemCode || itemCodeKey == selectionKey || strings.Contains(itemCodeKey, selectionKey) || strings.Contains(selectionKey, itemCodeKey)
				nameMatches := itemNameKey == selectionKey || strings.Contains(itemNameKey, selectionKey) || strings.Contains(selectionKey, itemNameKey)
				if !codeMatches && !nameMatches {
					continue
				}
				item := replenishmentRecommendationFromRow(row)
				matched = &item
				break
			}
			if matched != nil {
				break
			}
		}
		if matched == nil || matched.ItemCode == "" {
			return nil, shared.Validation(fmt.Sprintf("no replenishment recommendation exists for %s in %s", selection.ItemCode, selection.WarehouseCode))
		}
		if matched.RecommendedQuantity <= 0 {
			return nil, shared.Validation(fmt.Sprintf("%s in %s is currently covered and should not be turned into a purchase request", matched.ItemCode, selection.WarehouseCode))
		}
		key := selection.WarehouseCode + "|" + matched.ItemCode
		recommendations[key] = selectedRecommendation{
			Recommendation: *matched,
			Quantity:       roundBusinessMoney(selection.Quantity),
		}
	}
	type requestGroup struct {
		WarehouseCode string
		VendorID      string
		VendorName    string
		Lines         []map[string]any
	}
	groupOrder := make([]string, 0)
	groups := map[string]*requestGroup{}
	for _, selected := range recommendations {
		recommendation := selected.Recommendation
		groupKey := recommendation.WarehouseCode + "|" + recommendation.PreferredVendorID + "|" + recommendation.PreferredVendorName
		group := groups[groupKey]
		if group == nil {
			group = &requestGroup{
				WarehouseCode: recommendation.WarehouseCode,
				VendorID:      recommendation.PreferredVendorID,
				VendorName:    recommendation.PreferredVendorName,
				Lines:         make([]map[string]any, 0),
			}
			groups[groupKey] = group
			groupOrder = append(groupOrder, groupKey)
		}
		group.Lines = append(group.Lines, map[string]any{
			"item_code":      recommendation.ItemCode,
			"description":    recommendation.ItemName,
			"warehouse_code": recommendation.WarehouseCode,
			"quantity":       roundBusinessMoney(selected.Quantity),
		})
	}
	records := make([]document.Record, 0, len(groupOrder))
	for _, key := range groupOrder {
		group := groups[key]
		payload := map[string]any{
			"planning_generated":      true,
			"planning_source":         "replenishment",
			"planning_warehouse_code": group.WarehouseCode,
			"request_date":            time.Now().UTC().Format("2006-01-02"),
			"lines":                   group.Lines,
		}
		if group.VendorID != "" {
			payload["vendor_id"] = group.VendorID
		}
		if group.VendorName != "" {
			payload["vendor_name"] = group.VendorName
		}
		record, err := s.documents.Create("purchase_request", organizationID, locationID, documentWriteActorID(actor), payload)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	items := make([]map[string]any, 0, len(records))
	text := fmt.Sprintf("Prepared %d draft purchase request(s) from replenishment planning.", len(records))
	for _, record := range records {
		openPath := ""
		if s.documents != nil {
			openPath = s.documents.ResolveWorkspaceOpenPath(record)
		}
		lineSummaries := make([]string, 0, len(recordList(record.Body.Payload["lines"])))
		for _, line := range recordList(record.Body.Payload["lines"]) {
			description := firstNonEmptyString(strings.TrimSpace(textValue(line["description"])), strings.TrimSpace(textValue(line["item_code"])))
			lineSummaries = append(lineSummaries, fmt.Sprintf("%s x%.0f", description, roundBusinessMoney(numberValue(line["quantity"]))))
		}
		sort.Strings(lineSummaries)
		if vendor := strings.TrimSpace(textValue(record.Body.Payload["vendor_name"])); vendor != "" {
			text += fmt.Sprintf(" %s for %s.", vendor, strings.Join(lineSummaries, ", "))
		} else {
			text += fmt.Sprintf(" %s.", strings.Join(lineSummaries, ", "))
		}
		if openPath != "" {
			text += " Open draft: " + openPath
		}
		sanitized := s.sanitizeDocumentRecord(actor, record)
		items = append(items, map[string]any{
			"record":          sanitized,
			"summary":         s.documentSummary(sanitized, false),
			"document_id":     record.Header.ID,
			"document_type":   record.Header.Type,
			"document_status": record.Header.Status,
			"title":           strings.TrimSpace(firstNonEmptyString(record.Header.Number, textValue(record.Body.Payload["vendor_name"]), record.Header.ID)),
			"open_path":       openPath,
			"vendor_name":     strings.TrimSpace(textValue(record.Body.Payload["vendor_name"])),
			"warehouse_code":  strings.TrimSpace(textValue(record.Body.Payload["planning_warehouse_code"])),
			"line_count":      len(recordList(record.Body.Payload["lines"])),
		})
	}
	return map[string]any{
		"content": []ContentBlock{{Type: "text", Text: text}},
		"structuredContent": map[string]any{
			"documents": items,
			"count":     len(items),
		},
	}, nil
}

func (s *Server) posSalesStrategySummary(actor ActorContext, arguments map[string]any) (map[string]any, error) {
	if s == nil || s.models == nil {
		return nil, fmt.Errorf("models are unavailable")
	}
	sales, err := s.listModelRecordsForTool(actor, "pos_sale", model.Query{PageSize: model.MaxPageSize})
	if err != nil {
		return nil, err
	}
	items, err := s.listModelRecordsForTool(actor, "commercial_item", model.Query{PageSize: model.MaxPageSize})
	if err != nil {
		return nil, err
	}
	customers, err := s.listModelRecordsForTool(actor, "customer_profile", model.Query{PageSize: model.MaxPageSize})
	if err != nil {
		return nil, err
	}
	storeCode := strings.TrimSpace(stringArg(arguments, "store_code"))
	registerCode := strings.TrimSpace(stringArg(arguments, "register_code"))
	dateFrom, dateTo := strategyDateBounds(arguments)
	itemNames := make(map[string]string, len(items))
	for _, item := range items {
		itemNames[textValue(item.Values["sku"])] = textValue(item.Values["name"])
	}
	type productAggregate struct {
		ItemCode  string  `json:"item_code"`
		ItemName  string  `json:"item_name"`
		SaleCount int     `json:"sale_count"`
		UnitsSold float64 `json:"units_sold"`
		Revenue   float64 `json:"revenue_total"`
	}
	type segmentAggregate struct {
		Segment        string  `json:"segment"`
		SaleCount      int     `json:"sale_count"`
		RevenueTotal   float64 `json:"revenue_total"`
		ComboSaleCount int     `json:"combo_sale_count"`
		CustomerCount  int     `json:"customer_count"`
	}
	type comboAggregate struct {
		Key         string   `json:"key"`
		ItemCodes   []string `json:"item_codes"`
		ItemNames   []string `json:"item_names"`
		SaleCount   int      `json:"sale_count"`
		CustomerIDs map[string]struct{}
		MemberTiers map[string]struct{}
	}
	customerByParty := make(map[string]model.Record, len(customers))
	for _, customer := range customers {
		customerByParty[textValue(customer.Values["party_id"])] = customer
	}
	productMap := map[string]*productAggregate{}
	segmentMap := map[string]*segmentAggregate{}
	comboMap := map[string]*comboAggregate{}
	repeatCustomerSales := map[string]int{}
	repeatCustomerRevenue := map[string]float64{}
	totalSales := 0
	totalRevenue := 0.0
	for _, sale := range sales {
		if textValue(sale.Values["status"]) != "completed" {
			continue
		}
		if storeCode != "" && textValue(sale.Values["store_code"]) != storeCode {
			continue
		}
		if registerCode != "" && textValue(sale.Values["register_code"]) != registerCode {
			continue
		}
		if !withinStrategyTimeWindow(sale.CreatedAt, dateFrom, dateTo) {
			continue
		}
		totalSales++
		totalRevenue += roundBusinessMoney(numberValue(sale.Values["total_amount"]))
		partyID := textValue(sale.Values["party_id"])
		customer := customerByParty[partyID]
		memberTier := firstNonEmptyString(textValue(customer.Values["member_tier"]), "walk-in")
		segment := strings.ToLower(strings.TrimSpace(memberTier))
		if segment == "" {
			segment = "walk-in"
		}
		seg := segmentMap[segment]
		if seg == nil {
			seg = &segmentAggregate{Segment: segment}
			segmentMap[segment] = seg
		}
		seg.SaleCount++
		seg.RevenueTotal = roundBusinessMoney(seg.RevenueTotal + numberValue(sale.Values["total_amount"]))
		if partyID != "" {
			repeatCustomerSales[partyID]++
			repeatCustomerRevenue[partyID] = roundBusinessMoney(repeatCustomerRevenue[partyID] + numberValue(sale.Values["total_amount"]))
		}
		var lines []map[string]any
		_ = json.Unmarshal([]byte(textValue(sale.Values["lines_json"])), &lines)
		seenCodes := make([]string, 0, len(lines))
		seenSet := map[string]struct{}{}
		for _, line := range lines {
			itemCode := textValue(line["item_code"])
			if itemCode == "" {
				continue
			}
			agg := productMap[itemCode]
			if agg == nil {
				agg = &productAggregate{ItemCode: itemCode, ItemName: firstNonEmptyString(itemNames[itemCode], itemCode)}
				productMap[itemCode] = agg
			}
			agg.SaleCount++
			agg.UnitsSold += numberValue(line["quantity"])
			agg.Revenue = roundBusinessMoney(agg.Revenue + numberValue(line["line_total"]))
			if _, exists := seenSet[itemCode]; !exists {
				seenSet[itemCode] = struct{}{}
				seenCodes = append(seenCodes, itemCode)
			}
		}
		sort.Strings(seenCodes)
		if len(seenCodes) >= 2 {
			for i := 0; i < len(seenCodes)-1; i++ {
				for j := i + 1; j < len(seenCodes); j++ {
					key := seenCodes[i] + "|" + seenCodes[j]
					combo := comboMap[key]
					if combo == nil {
						combo = &comboAggregate{
							Key:         key,
							ItemCodes:   []string{seenCodes[i], seenCodes[j]},
							ItemNames:   []string{firstNonEmptyString(itemNames[seenCodes[i]], seenCodes[i]), firstNonEmptyString(itemNames[seenCodes[j]], seenCodes[j])},
							CustomerIDs: map[string]struct{}{},
							MemberTiers: map[string]struct{}{},
						}
						comboMap[key] = combo
					}
					combo.SaleCount++
					if partyID != "" {
						combo.CustomerIDs[partyID] = struct{}{}
					}
					if memberTier != "" {
						combo.MemberTiers[strings.ToLower(memberTier)] = struct{}{}
					}
					seg.ComboSaleCount++
				}
			}
		}
	}
	products := make([]productAggregate, 0, len(productMap))
	for _, item := range productMap {
		products = append(products, *item)
	}
	sort.Slice(products, func(i, j int) bool {
		if products[i].SaleCount == products[j].SaleCount {
			return products[i].Revenue > products[j].Revenue
		}
		return products[i].SaleCount > products[j].SaleCount
	})
	segments := make([]segmentAggregate, 0, len(segmentMap))
	for _, item := range segmentMap {
		customerCount := 0
		for partyID := range repeatCustomerSales {
			customer := customerByParty[partyID]
			if strings.EqualFold(textValue(customer.Values["member_tier"]), item.Segment) {
				customerCount++
			}
		}
		item.CustomerCount = customerCount
		segments = append(segments, *item)
	}
	sort.Slice(segments, func(i, j int) bool {
		if segments[i].ComboSaleCount == segments[j].ComboSaleCount {
			return segments[i].RevenueTotal > segments[j].RevenueTotal
		}
		return segments[i].ComboSaleCount > segments[j].ComboSaleCount
	})
	type comboSummary struct {
		ItemCodes           []string `json:"item_codes"`
		ItemNames           []string `json:"item_names"`
		SaleCount           int      `json:"sale_count"`
		RepeatCustomerCount int      `json:"repeat_customer_count"`
		MemberTiers         []string `json:"member_tiers"`
	}
	combos := make([]comboSummary, 0, len(comboMap))
	for _, combo := range comboMap {
		memberTiers := setToSortedSlice(combo.MemberTiers)
		combos = append(combos, comboSummary{
			ItemCodes:           combo.ItemCodes,
			ItemNames:           combo.ItemNames,
			SaleCount:           combo.SaleCount,
			RepeatCustomerCount: len(combo.CustomerIDs),
			MemberTiers:         memberTiers,
		})
	}
	sort.Slice(combos, func(i, j int) bool {
		if combos[i].SaleCount == combos[j].SaleCount {
			return combos[i].RepeatCustomerCount > combos[j].RepeatCustomerCount
		}
		return combos[i].SaleCount > combos[j].SaleCount
	})
	limit := intArg(arguments, "limit")
	if limit <= 0 {
		limit = 5
	}
	if len(products) > limit {
		products = products[:limit]
	}
	if len(combos) > limit {
		combos = combos[:limit]
	}
	if len(segments) > limit {
		segments = segments[:limit]
	}
	recommendation := map[string]any{}
	if len(combos) > 0 {
		topCombo := combos[0]
		targetSegment := "all customers"
		if len(topCombo.MemberTiers) > 0 {
			targetSegment = topCombo.MemberTiers[0] + " members"
		} else if len(segments) > 0 && segments[0].Segment != "" && segments[0].Segment != "walk-in" {
			targetSegment = segments[0].Segment + " members"
		}
		recommendation = map[string]any{
			"campaign_kind":   "bundle",
			"target_products": topCombo.ItemNames,
			"target_segment":  targetSegment,
			"rationale":       fmt.Sprintf("%s were purchased together %d times, concentrated in %s.", strings.Join(topCombo.ItemNames, " + "), topCombo.SaleCount, targetSegment),
		}
	}
	text := fmt.Sprintf("Reviewed %d completed POS sales totaling %.0f.", totalSales, roundBusinessMoney(totalRevenue))
	if len(products) > 0 {
		top := products[0]
		text += fmt.Sprintf("\nTop product: %s sold %.0f units across %d sales.", top.ItemName, top.UnitsSold, top.SaleCount)
	}
	if len(combos) > 0 {
		topCombo := combos[0]
		text += fmt.Sprintf("\nStrongest bundle signal: %s were purchased together %d times", strings.Join(topCombo.ItemNames, " + "), topCombo.SaleCount)
		if len(topCombo.MemberTiers) > 0 {
			text += fmt.Sprintf(" by %s.", strings.Join(topCombo.MemberTiers, ", "))
		} else {
			text += "."
		}
	}
	if len(segments) > 0 {
		topSegment := segments[0]
		text += fmt.Sprintf("\nMost responsive segment: %s with %d sales and %d combo-sale occurrences.", topSegment.Segment, topSegment.SaleCount, topSegment.ComboSaleCount)
	}
	if len(recommendation) > 0 {
		text += fmt.Sprintf("\nRecommendation signal: run a %s campaign for %s targeting %s.", stringValue(recommendation["campaign_kind"]), strings.Join(anyStringSlice(recommendation["target_products"]), " + "), stringValue(recommendation["target_segment"]))
	}
	return map[string]any{
		"content": []ContentBlock{{Type: "text", Text: text}},
		"structuredContent": map[string]any{
			"summary": map[string]any{
				"sale_count":    totalSales,
				"revenue_total": roundBusinessMoney(totalRevenue),
				"date_from":     dateStringOrEmpty(dateFrom),
				"date_to":       dateStringOrEmpty(dateTo),
			},
			"top_products":          products,
			"bundle_patterns":       combos,
			"segment_performance":   segments,
			"recommendation_signal": recommendation,
		},
	}, nil
}

func (s *Server) promotionPerformanceSummary(actor ActorContext, arguments map[string]any) (map[string]any, error) {
	if s == nil || s.models == nil {
		return nil, fmt.Errorf("models are unavailable")
	}
	campaigns, err := s.listModelRecordsForTool(actor, "promotion_campaign", model.Query{PageSize: model.MaxPageSize})
	if err != nil {
		return nil, err
	}
	codes, err := s.listModelRecordsForTool(actor, "promotion_code", model.Query{PageSize: model.MaxPageSize})
	if err != nil {
		return nil, err
	}
	redemptions, err := s.listModelRecordsForTool(actor, "promotion_redemption", model.Query{PageSize: model.MaxPageSize})
	if err != nil {
		return nil, err
	}
	rules, err := s.listModelRecordsForTool(actor, "discount_rule", model.Query{PageSize: model.MaxPageSize})
	if err != nil {
		return nil, err
	}
	items, err := s.listModelRecordsForTool(actor, "commercial_item", model.Query{PageSize: model.MaxPageSize})
	if err != nil {
		return nil, err
	}
	storeCode := strings.TrimSpace(stringArg(arguments, "store_code"))
	itemNames := make(map[string]string, len(items))
	for _, item := range items {
		itemNames[textValue(item.Values["sku"])] = textValue(item.Values["name"])
	}
	type campaignSummary struct {
		CampaignCode    string   `json:"campaign_code"`
		CampaignName    string   `json:"campaign_name"`
		TriggerMode     string   `json:"trigger_mode,omitempty"`
		PromoCodes      []string `json:"promo_codes,omitempty"`
		LinkedItems     []string `json:"linked_items,omitempty"`
		RedemptionCount int      `json:"redemption_count"`
		DiscountTotal   float64  `json:"discount_total"`
		Status          string   `json:"status,omitempty"`
	}
	campaignMap := map[string]*campaignSummary{}
	for _, campaign := range campaigns {
		code := textValue(campaign.Values["code"])
		if code == "" {
			continue
		}
		if storeCode != "" {
			storeCodes := strings.ToLower(textValue(campaign.Values["store_codes"]))
			if storeCodes != "" && !strings.Contains(storeCodes, strings.ToLower(storeCode)) {
				continue
			}
		}
		campaignMap[code] = &campaignSummary{
			CampaignCode: code,
			CampaignName: textValue(campaign.Values["name"]),
			TriggerMode:  textValue(campaign.Values["trigger_mode"]),
			Status:       textValue(campaign.Values["status"]),
		}
	}
	for _, code := range codes {
		campaignCode := textValue(code.Values["promotion_campaign_code"])
		summary := campaignMap[campaignCode]
		if summary == nil {
			continue
		}
		summary.PromoCodes = append(summary.PromoCodes, textValue(code.Values["code"]))
	}
	for _, rule := range rules {
		campaignCode := textValue(rule.Values["promotion_campaign_code"])
		summary := campaignMap[campaignCode]
		if summary == nil {
			continue
		}
		for _, itemCode := range splitCSVStrings(textValue(rule.Values["item_codes"])) {
			name := firstNonEmptyString(itemNames[itemCode], itemCode)
			if !containsString(summary.LinkedItems, name) {
				summary.LinkedItems = append(summary.LinkedItems, name)
			}
		}
	}
	for _, redemption := range redemptions {
		if textValue(redemption.Values["status"]) != "active" {
			continue
		}
		if storeCode != "" && textValue(redemption.Values["store_code"]) != storeCode {
			continue
		}
		campaignCode := textValue(redemption.Values["promotion_campaign_code"])
		summary := campaignMap[campaignCode]
		if summary == nil {
			continue
		}
		summary.RedemptionCount++
		summary.DiscountTotal = roundBusinessMoney(summary.DiscountTotal + numberValue(redemption.Values["discount_amount_total"]))
	}
	list := make([]campaignSummary, 0, len(campaignMap))
	for _, item := range campaignMap {
		sort.Strings(item.PromoCodes)
		sort.Strings(item.LinkedItems)
		list = append(list, *item)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].RedemptionCount == list[j].RedemptionCount {
			return list[i].CampaignName < list[j].CampaignName
		}
		return list[i].RedemptionCount < list[j].RedemptionCount
	})
	underperforming := map[string]any{}
	if len(list) > 0 {
		item := list[0]
		underperforming = map[string]any{
			"campaign_name":    item.CampaignName,
			"campaign_code":    item.CampaignCode,
			"redemption_count": item.RedemptionCount,
			"reason":           fmt.Sprintf("%s has only %d redemption(s) and weaker demand than the strongest POS bundle signal.", item.CampaignName, item.RedemptionCount),
			"linked_items":     item.LinkedItems,
		}
	}
	text := fmt.Sprintf("Reviewed %d promotion campaigns and %d redemption records.", len(list), len(redemptions))
	if len(list) > 0 {
		item := list[0]
		text += fmt.Sprintf("\nWeakest current promotion: %s with %d redemption(s).", item.CampaignName, item.RedemptionCount)
		if len(item.LinkedItems) > 0 {
			text += fmt.Sprintf(" Linked items: %s.", strings.Join(item.LinkedItems, ", "))
		}
	}
	return map[string]any{
		"content": []ContentBlock{{Type: "text", Text: text}},
		"structuredContent": map[string]any{
			"campaigns":              list,
			"underperforming_signal": underperforming,
		},
	}, nil
}

func (s *Server) promotionStrategyPlanDraftCreate(actor ActorContext, arguments map[string]any) (map[string]any, error) {
	if s == nil || s.documents == nil {
		return nil, fmt.Errorf("documents are unavailable")
	}
	if !allowsAll(actor.PermissionChecker, []string{"document.create"}) {
		return nil, fmt.Errorf("tool is not allowed")
	}
	if !boolArg(arguments, "confirm_apply") {
		return nil, shared.Validation("confirm_apply must be true")
	}
	title := strings.TrimSpace(stringArg(arguments, "title"))
	if title == "" {
		return nil, shared.Validation("title is required")
	}
	payload := map[string]any{
		"title":             title,
		"summary":           strings.TrimSpace(stringArg(arguments, "summary")),
		"target_products":   anyStringSlice(arguments["target_products"]),
		"target_segment":    strings.TrimSpace(stringArg(arguments, "target_segment")),
		"replaced_campaign": strings.TrimSpace(stringArg(arguments, "replaced_campaign")),
		"campaign_kind":     firstNonEmpty(stringArg(arguments, "campaign_kind"), "promotion_strategy_plan"),
		"request_kind":      "promotion_plan",
		"viewer_hint":       "promotion.plan",
		"status":            "draft",
	}
	if payload["summary"] == "" {
		payload["summary"] = strings.TrimSpace(fmt.Sprintf("Promotion strategy for %s targeting %s. Replace %s.", strings.Join(anyStringSlice(payload["target_products"]), ", "), stringValue(payload["target_segment"]), stringValue(payload["replaced_campaign"])))
	}
	locationID := firstNonEmpty(stringArg(arguments, "location_id"), strings.TrimSpace(actor.LocationID))
	organizationID := firstNonEmpty(stringArg(arguments, "organization_id"), strings.TrimSpace(actor.OrganizationID), "org_default")
	locationID = firstNonEmpty(locationID, "loc_hq")
	candidate := document.Record{
		Header: document.Header{
			Type:           "generic_request",
			Status:         "draft",
			OrganizationID: organizationID,
			LocationID:     locationID,
		},
		Body: document.Body{Payload: payload},
	}
	if err := s.validateDocumentWrite(actor, candidate, payload); err != nil {
		return nil, err
	}
	record, err := s.documents.Create("generic_request", organizationID, locationID, documentWriteActorID(actor), payload)
	if err != nil {
		return nil, err
	}
	if s.search != nil {
		s.search.RefreshDocument(record)
	}
	resp, _, err := s.renderBusinessDocumentDraftResult(actor, record, "Created promotion strategy draft "+record.Header.ID+" as generic_request.")
	return resp, err
}

func (s *Server) listModelRecordsForTool(actor ActorContext, modelKey string, query model.Query) ([]model.Record, error) {
	if s == nil || s.models == nil {
		return nil, fmt.Errorf("models are unavailable")
	}
	def, ok := s.models.Definition(modelKey)
	if !ok {
		return nil, shared.NotFound("model definition not found")
	}
	if !allowsAll(actor.PermissionChecker, []string{def.ListPermissionKey}) {
		return nil, fmt.Errorf("tool is not allowed")
	}
	if query.PageSize <= 0 || query.PageSize > model.MaxPageSize {
		query.PageSize = model.MaxPageSize
	}
	if query.Page <= 0 {
		query.Page = 1
	}
	out := make([]model.Record, 0)
	for page := query.Page; ; page++ {
		pageQuery := query
		pageQuery.Page = page
		items, total, err := s.models.List(modelKey, pageQuery)
		if err != nil {
			return nil, err
		}
		items = s.sanitizeModelRecords(actor, def, items)
		out = append(out, items...)
		if len(items) == 0 || page*pageQuery.PageSize >= total {
			break
		}
	}
	return out, nil
}

func strategyDateBounds(arguments map[string]any) (time.Time, time.Time) {
	return parseStrategyDate(stringArg(arguments, "date_from")), parseStrategyDate(stringArg(arguments, "date_to"))
}

func parseStrategyDate(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.UTC()
		}
	}
	return time.Time{}
}

func withinStrategyTimeWindow(at, from, to time.Time) bool {
	if !from.IsZero() && at.Before(from) {
		return false
	}
	if !to.IsZero() {
		end := to
		if to.Hour() == 0 && to.Minute() == 0 && to.Second() == 0 {
			end = to.Add(24*time.Hour - time.Nanosecond)
		}
		if at.After(end) {
			return false
		}
	}
	return true
}

func splitCSVStrings(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ';' || r == '\n' })
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			items = append(items, part)
		}
	}
	return items
}

func setToSortedSlice(items map[string]struct{}) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	for item := range items {
		out = append(out, item)
	}
	sort.Strings(out)
	return out
}

func roundBusinessMoney(value float64) float64 {
	return float64(int(value*100+0.5)) / 100
}

func dateStringOrEmpty(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format("2006-01-02")
}

func textValue(value any) string {
	return stringValue(value)
}

func numberValue(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case int32:
		return float64(v)
	case json.Number:
		f, _ := v.Float64()
		return f
	default:
		return 0
	}
}

func boolValue(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(strings.TrimSpace(v), "true")
	default:
		return false
	}
}

func firstNonEmptyString(values ...string) string {
	return firstNonEmpty(values...)
}

func anyStringSlice(value any) []string {
	switch items := value.(type) {
	case []string:
		return append([]string(nil), items...)
	case []any:
		result := make([]string, 0, len(items))
		for _, item := range items {
			if text := strings.TrimSpace(stringValue(item)); text != "" {
				result = append(result, text)
			}
		}
		return result
	default:
		text := strings.TrimSpace(stringValue(value))
		if text == "" {
			return nil
		}
		return []string{text}
	}
}

func recordList(value any) []map[string]any {
	switch typed := value.(type) {
	case []map[string]any:
		return append([]map[string]any(nil), typed...)
	case []any:
		rows := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			if row, ok := item.(map[string]any); ok {
				rows = append(rows, row)
			}
		}
		return rows
	default:
		return nil
	}
}

func documentMatchesFilters(record document.Record, filters map[string]any) bool {
	for key, expected := range filters {
		if !documentFieldMatchesFilter(record, key, expected) {
			return false
		}
	}
	return true
}

func documentFieldMatchesFilter(record document.Record, key string, expected any) bool {
	expectedText := strings.ToLower(strings.TrimSpace(anyString(expected)))
	if expectedText == "" {
		return true
	}
	candidates := documentFilterCandidateValues(record, key)
	for _, candidate := range candidates {
		candidateText := strings.ToLower(strings.TrimSpace(candidate))
		if candidateText == "" {
			continue
		}
		if candidateText == expectedText || strings.Contains(candidateText, expectedText) {
			return true
		}
	}
	return false
}

func documentFilterCandidateValues(record document.Record, key string) []string {
	normalized := strings.TrimSpace(strings.ToLower(key))
	switch normalized {
	case "document_id", "id", "record_id":
		return []string{record.Header.ID}
	case "document_type", "type", "resource_key":
		return []string{record.Header.Type}
	case "status":
		return []string{record.Header.Status}
	case "number":
		return []string{record.Header.Number}
	case "organization_id":
		return []string{record.Header.OrganizationID}
	case "location_id":
		return []string{record.Header.LocationID}
	}
	if value, ok := lookupPayloadValue(record.Body.Payload, normalized); ok {
		return flattenFilterValues(value)
	}
	return nil
}

func lookupPayloadValue(payload map[string]any, key string) (any, bool) {
	if payload == nil {
		return nil, false
	}
	if value, ok := payload[key]; ok {
		return value, true
	}
	for candidateKey, value := range payload {
		if strings.EqualFold(strings.TrimSpace(candidateKey), key) {
			return value, true
		}
	}
	return nil, false
}

func flattenFilterValues(value any) []string {
	switch typed := value.(type) {
	case string:
		return []string{typed}
	case fmt.Stringer:
		return []string{typed.String()}
	case []string:
		return typed
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			out = append(out, flattenFilterValues(item)...)
		}
		return out
	case map[string]any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			out = append(out, flattenFilterValues(item)...)
		}
		return out
	default:
		if text := anyString(value); text != "" {
			return []string{text}
		}
		return nil
	}
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
	summary := s.documentSummary(sanitized, false)
	text := renderBusinessDocumentText(sanitized, summary)
	if includeFull {
		return map[string]any{
			"content":           []ContentBlock{{Type: "text", Text: text}},
			"structuredContent": sanitized,
		}, true, nil
	}
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: text}},
		"structuredContent": summary,
	}, true, nil
}

func renderBusinessDocumentText(record document.Record, item businessRecordSummary) string {
	lines := []string{fmt.Sprintf("Loaded business document %s.", item.RecordID)}
	if item.ResourceKey != "" {
		lines = append(lines, "Type: "+item.ResourceKey)
	}
	if item.Title != "" {
		lines = append(lines, "Title: "+item.Title)
	}
	if item.Status != "" {
		lines = append(lines, "Status: "+item.Status)
	}
	if item.OrganizationID != "" {
		lines = append(lines, "Organization: "+item.OrganizationID)
	}
	if item.LocationID != "" {
		lines = append(lines, "Location: "+item.LocationID)
	}
	if payload, ok := item.Record["payload"]; ok && payload != nil {
		if encoded, err := json.Marshal(payload); err == nil && len(encoded) > 2 {
			lines = append(lines, "Payload summary: "+string(encoded))
		}
	}
	lines = append(lines, businessDocumentInterpretationLines(record)...)
	return strings.Join(lines, "\n")
}

func businessDocumentInterpretationLines(record document.Record) []string {
	payload := record.Body.Payload
	switch strings.TrimSpace(record.Header.Type) {
	case "travel_request":
		if amount := payloadValueText(payload, "estimated_total_amount", "approved_amount", "total_amount"); amount != "" {
			return []string{fmt.Sprintf("Business summary: travel request amount is %s.", amount)}
		}
	case "cash_advance":
		if amount := payloadValueText(payload, "approved_amount", "outstanding_amount", "total_amount", "requested_amount"); amount != "" {
			return []string{fmt.Sprintf("Business summary: cash advance amount is %s and status is %s.", amount, firstNonEmpty(record.Header.Status, "unknown"))}
		}
	case "expense_claim":
		if amount := payloadValueText(payload, "reimbursable_amount", "approved_amount", "claim_total_amount", "total_amount"); amount != "" {
			return []string{fmt.Sprintf("Business summary: expense claim total is %s and status is %s.", amount, firstNonEmpty(record.Header.Status, "unknown"))}
		}
	case "advance_liquidation":
		lines := make([]string, 0, 3)
		claimTotal := payloadValueText(payload, "claim_total_amount", "reimbursable_amount", "total_amount")
		advanceAmount := payloadValueText(payload, "advance_amount")
		appliedAmount := payloadValueText(payload, "advance_applied_amount")
		netAmount := payloadValueText(payload, "net_settlement_amount")
		if claimTotal != "" || advanceAmount != "" || appliedAmount != "" || netAmount != "" {
			lines = append(lines, fmt.Sprintf(
				"Settlement summary: claim total %s, advance amount %s, advance applied %s, net settlement %s.",
				firstNonEmpty(claimTotal, "0"),
				firstNonEmpty(advanceAmount, "0"),
				firstNonEmpty(appliedAmount, "0"),
				firstNonEmpty(netAmount, "0"),
			))
		}
		switch direction := strings.TrimSpace(anyString(payload["settlement_direction"])); direction {
		case "company_owes_employee":
			lines = append(lines, fmt.Sprintf("Interpretation: the company owes the employee %s.", firstNonEmpty(netAmount, "0")))
		case "employee_owes_company":
			lines = append(lines, fmt.Sprintf("Interpretation: the employee owes the company %s.", firstNonEmpty(netAmount, "0")))
		case "balanced":
			lines = append(lines, "Interpretation: the settlement is balanced and neither side owes the other.")
		}
		return lines
	case "reimbursement_payment":
		lines := make([]string, 0, 2)
		if amount := payloadValueText(payload, "amount_paid", "net_settlement_amount", "total_amount"); amount != "" {
			lines = append(lines, fmt.Sprintf("Payment summary: reimbursement payment amount is %s and status is %s.", amount, firstNonEmpty(record.Header.Status, "unknown")))
		}
		if settlement := payloadValueText(payload, "net_settlement_amount"); settlement != "" {
			lines = append(lines, fmt.Sprintf("Interpretation: this payment settles %s owed to the employee.", settlement))
		}
		return lines
	}
	return nil
}

func payloadValueText(payload map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := payload[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) != "" {
				return strings.TrimSpace(typed)
			}
		case float64:
			return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", typed), "0"), ".")
		case float32:
			return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", typed), "0"), ".")
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			return fmt.Sprint(typed)
		default:
			text := strings.TrimSpace(fmt.Sprint(typed))
			if text != "" && text != "<nil>" {
				return text
			}
		}
	}
	return ""
}

func (s *Server) renderBusinessDocumentDraftResult(actor ActorContext, record document.Record, text string) (map[string]any, bool, error) {
	sanitized := s.sanitizeDocumentRecord(actor, record)
	openPath := ""
	if s.documents != nil {
		openPath = s.documents.ResolveWorkspaceOpenPath(record)
	}
	resultText := strings.TrimSpace(text)
	if openPath != "" {
		resultText = strings.TrimSpace(resultText + " Open draft: " + openPath)
	}
	return map[string]any{
		"content": []ContentBlock{{Type: "text", Text: resultText}},
		"structuredContent": map[string]any{
			"record":          sanitized,
			"summary":         s.documentSummary(sanitized, false),
			"document_id":     record.Header.ID,
			"document_type":   record.Header.Type,
			"document_status": record.Header.Status,
			"title":           strings.TrimSpace(stringValue(record.Body.Payload["title"])),
			"open_path":       openPath,
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
	if strings.TrimSpace(query.SortKey) == "" {
		// MCP generic record search should not inherit module defaults that may
		// point to non-queryable fields such as "code".
		query.SortKey = "updated_at"
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

func documentWriteActorID(actor ActorContext) string {
	if userID := firstNonEmpty(strings.TrimSpace(actor.EffectiveUserID), strings.TrimSpace(actor.OnBehalfOfUserID)); userID != "" {
		return userID
	}
	if strings.EqualFold(strings.TrimSpace(actor.ActorKind), "service") {
		return "user_admin"
	}
	return strings.TrimSpace(actor.ActorID)
}

func firstString(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}
