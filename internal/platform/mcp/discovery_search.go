package mcp

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"orbyte/internal/platform/search"
	"orbyte/internal/platform/shared"
)

const (
	DiscoveryModeKeyword = "keyword"
	DiscoveryModeVector  = "vector"
	DiscoveryModeHybrid  = "hybrid"

	mcpToolsDiscoveryIndexKey     = "mcp.tools.discovery"
	mcpPlaybooksDiscoveryIndexKey = "mcp.playbooks.discovery"
)

type discoveryToolDocument struct {
	ID              string   `json:"id"`
	ToolID          string   `json:"tool_id"`
	Name            string   `json:"name"`
	Title           string   `json:"title"`
	Description     string   `json:"description"`
	ModuleKey       string   `json:"module_key"`
	SourceType      string   `json:"source_type"`
	Scope           string   `json:"scope"`
	ActionClass     string   `json:"action_class"`
	RiskClass       string   `json:"risk_class"`
	BusinessDomains []string `json:"business_domains"`
	Labels          []string `json:"labels"`
	GovernanceTags  []string `json:"governance_tags"`
	SearchText      string   `json:"search_text"`
}

type discoveryPlaybookDocument struct {
	ID               string   `json:"id"`
	PlaybookID       string   `json:"playbook_id"`
	Name             string   `json:"name"`
	Description      string   `json:"description"`
	UseWhen          string   `json:"use_when"`
	Domains          []string `json:"domains"`
	Labels           []string `json:"labels"`
	Keywords         []string `json:"keywords"`
	RecommendedTools []string `json:"recommended_tools"`
	SearchText       string   `json:"search_text"`
}

func normalizeDiscoveryMode(mode string) string {
	switch strings.TrimSpace(strings.ToLower(mode)) {
	case DiscoveryModeKeyword:
		return DiscoveryModeKeyword
	case DiscoveryModeHybrid:
		return DiscoveryModeHybrid
	default:
		return DiscoveryModeVector
	}
}

func (s *Server) toolDiscoveryMode() string {
	cfg := s.mcpRuntimeConfig()
	if cfg.ToolDiscoveryMode != "" {
		return s.effectiveDiscoveryMode(cfg.ToolDiscoveryMode)
	}
	return s.effectiveDiscoveryMode(cfg.DiscoveryMode)
}

func (s *Server) playbookDiscoveryMode() string {
	cfg := s.mcpRuntimeConfig()
	if cfg.PlaybookDiscoveryMode != "" {
		return s.effectiveDiscoveryMode(cfg.PlaybookDiscoveryMode)
	}
	return s.effectiveDiscoveryMode(cfg.DiscoveryMode)
}

func (s *Server) effectiveDiscoveryMode(mode string) string {
	mode = normalizeDiscoveryMode(mode)
	if s == nil || s.search == nil {
		return mode
	}
	runtime := s.search.EmbeddingRuntime()
	if runtime.Semantic {
		return mode
	}
	switch mode {
	case DiscoveryModeVector, DiscoveryModeHybrid:
		return DiscoveryModeKeyword
	default:
		return mode
	}
}

type discoverySyncState struct {
	mu                  sync.Mutex
	registered          bool
	toolFingerprint     string
	playbookFingerprint string
}

func (s *Server) ensureDiscoveryIndexes() error {
	if s == nil || s.search == nil {
		return nil
	}
	cfg := s.mcpRuntimeConfig()
	if !cfg.DiscoveryIndexingEnabled {
		return nil
	}
	tools := s.discoveryToolDocuments()
	playbooks := s.discoveryPlaybookDocuments()
	toolFingerprint := fingerprintAny(tools)
	playbookFingerprint := fingerprintAny(playbooks)
	state := s.discoveryState()
	state.mu.Lock()
	defer state.mu.Unlock()

	if !state.registered {
		if err := ignoreConflictError(s.search.RegisterIndex(search.IndexDefinition{
			Key:               mcpToolsDiscoveryIndexKey,
			Title:             "MCP Tool Discovery",
			SourceKind:        "runtime",
			Modes:             []string{"keyword", "vector", "hybrid"},
			OrganizationSplit: true,
			QueryFilterFields: []string{"module_key", "source_type"},
			QuerySortFields:   []string{"name", "title"},
			Fields: []search.IndexFieldDefinition{
				{Key: "tool_id", Path: "tool_id", Type: "string", Searchable: true},
				{Key: "name", Path: "name", Type: "string", Searchable: true, Sort: true},
				{Key: "title", Path: "title", Type: "string", Searchable: true, Sort: true},
				{Key: "description", Path: "description", Type: "string", Searchable: true},
				{Key: "module_key", Path: "module_key", Type: "string", Facet: true},
				{Key: "source_type", Path: "source_type", Type: "string", Facet: true},
				{Key: "scope", Path: "scope", Type: "string"},
				{Key: "action_class", Path: "action_class", Type: "string"},
				{Key: "risk_class", Path: "risk_class", Type: "string"},
				{Key: "business_domains", Path: "business_domains", Type: "string"},
				{Key: "labels", Path: "labels", Type: "string"},
				{Key: "governance_tags", Path: "governance_tags", Type: "string"},
				{Key: "search_text", Path: "search_text", Type: "string", Searchable: true},
			},
			VectorFields: []search.VectorFieldDefinition{{
				Key:           "semantic",
				SourcePaths:   []string{"search_text"},
				EmbeddingMode: "external",
				Dimensions:    8,
			}},
		})); err != nil {
			return err
		}
		if err := ignoreConflictError(s.search.RegisterIndex(search.IndexDefinition{
			Key:               mcpPlaybooksDiscoveryIndexKey,
			Title:             "MCP Playbook Discovery",
			SourceKind:        "runtime",
			Modes:             []string{"keyword", "vector", "hybrid"},
			OrganizationSplit: true,
			QuerySortFields:   []string{"name"},
			Fields: []search.IndexFieldDefinition{
				{Key: "playbook_id", Path: "playbook_id", Type: "string", Searchable: true},
				{Key: "name", Path: "name", Type: "string", Searchable: true, Sort: true},
				{Key: "description", Path: "description", Type: "string", Searchable: true},
				{Key: "use_when", Path: "use_when", Type: "string", Searchable: true},
				{Key: "domains", Path: "domains", Type: "string"},
				{Key: "labels", Path: "labels", Type: "string"},
				{Key: "keywords", Path: "keywords", Type: "string"},
				{Key: "recommended_tools", Path: "recommended_tools", Type: "string"},
				{Key: "search_text", Path: "search_text", Type: "string", Searchable: true},
			},
			VectorFields: []search.VectorFieldDefinition{{
				Key:           "semantic",
				SourcePaths:   []string{"search_text"},
				EmbeddingMode: "external",
				Dimensions:    8,
			}},
		})); err != nil {
			return err
		}
		state.registered = true
	}
	if state.toolFingerprint != toolFingerprint {
		records := make([]search.RuntimeSourceRecord, 0, len(tools))
		for _, item := range tools {
			records = append(records, search.RuntimeSourceRecord{
				ID:             item.ID,
				SourceID:       item.ToolID,
				OrganizationID: "global",
				Version:        1,
				UpdatedAt:      time.Now().UTC(),
				Payload:        mustJSONMap(item),
			})
		}
		if _, err := s.search.ReplaceRuntimeRecords(mcpToolsDiscoveryIndexKey, records); err != nil {
			return err
		}
		state.toolFingerprint = toolFingerprint
	}
	if state.playbookFingerprint != playbookFingerprint {
		records := make([]search.RuntimeSourceRecord, 0, len(playbooks))
		for _, item := range playbooks {
			records = append(records, search.RuntimeSourceRecord{
				ID:             item.ID,
				SourceID:       item.PlaybookID,
				OrganizationID: "global",
				Version:        1,
				UpdatedAt:      time.Now().UTC(),
				Payload:        mustJSONMap(item),
			})
		}
		if _, err := s.search.ReplaceRuntimeRecords(mcpPlaybooksDiscoveryIndexKey, records); err != nil {
			return err
		}
		state.playbookFingerprint = playbookFingerprint
	}
	return nil
}

func ignoreConflictError(err error) error {
	if err == nil {
		return nil
	}
	if typed, ok := err.(shared.Error); ok && typed.Kind == shared.KindConflict {
		return nil
	}
	return err
}

func (s *Server) discoveryState() *discoverySyncState {
	if s.discoverySync == nil {
		s.discoverySync = &discoverySyncState{}
	}
	return s.discoverySync
}

func (s *Server) discoveryToolDocuments() []discoveryToolDocument {
	items := s.ToolInventory()
	out := make([]discoveryToolDocument, 0, len(items))
	for _, item := range items {
		if !item.Enabled || isMetaToolName(item.Key) {
			continue
		}
		out = append(out, discoveryToolDocument{
			ID:              item.Key,
			ToolID:          item.Key,
			Name:            item.Key,
			Title:           item.Title,
			Description:     item.Description,
			ModuleKey:       item.ModuleKey,
			SourceType:      item.SourceType,
			Scope:           item.EndpointScope,
			ActionClass:     item.ActionClass,
			RiskClass:       item.RiskClass,
			BusinessDomains: append([]string(nil), item.BusinessDomains...),
			Labels:          append([]string(nil), item.CapabilityKeys...),
			GovernanceTags:  append([]string(nil), item.GovernanceTags...),
			SearchText: strings.Join([]string{
				item.Key,
				item.Title,
				item.Description,
				item.ModuleKey,
				item.SourceType,
				item.ActionClass,
				strings.Join(item.BusinessDomains, " "),
				strings.Join(item.CapabilityKeys, " "),
				strings.Join(item.GovernanceTags, " "),
			}, "\n"),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ToolID < out[j].ToolID })
	return out
}

func (s *Server) discoveryPlaybookDocuments() []discoveryPlaybookDocument {
	items := s.playbooks()
	out := make([]discoveryPlaybookDocument, 0, len(items))
	for _, item := range items {
		recommended := playbookRecommendedTools(item)
		out = append(out, discoveryPlaybookDocument{
			ID:               item.ID,
			PlaybookID:       item.ID,
			Name:             item.Name,
			Description:      item.Description,
			UseWhen:          item.UseWhen,
			Domains:          append([]string(nil), item.Domains...),
			Labels:           append([]string(nil), item.Labels...),
			Keywords:         append([]string(nil), item.Keywords...),
			RecommendedTools: recommended,
			SearchText: strings.Join([]string{
				item.ID,
				item.Name,
				item.Description,
				item.UseWhen,
				playbookToolSequenceText(item.WorkflowSteps),
				strings.Join(item.Keywords, " "),
				strings.Join(item.Domains, " "),
				strings.Join(item.Labels, " "),
				strings.Join(item.Guardrails, " "),
				strings.Join(item.SuccessChecks, " "),
				strings.Join(item.Pitfalls, " "),
			}, "\n"),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PlaybookID < out[j].PlaybookID })
	return out
}

func playbookToolSequenceText(items []PlaybookToolStep) string {
	parts := make([]string, 0, len(items))
	for _, item := range items {
		parts = append(parts, strings.TrimSpace(strings.Join([]string{
			item.Step,
			item.ToolID,
			item.Description,
			item.When,
			item.Output,
		}, " ")))
	}
	return strings.Join(parts, "\n")
}

func fingerprintAny(value any) string {
	raw, _ := json.Marshal(value)
	return fmt.Sprintf("%x", sha1.Sum(raw))
}

func mustJSONMap(value any) map[string]any {
	raw, _ := json.Marshal(value)
	out := map[string]any{}
	_ = json.Unmarshal(raw, &out)
	return out
}

func (s *Server) searchToolSummaries(actor ActorContext, arguments map[string]any, catalogOpts ToolCatalogOptions) ([]toolSummary, bool) {
	if s == nil || s.search == nil {
		return nil, false
	}
	if !s.mcpRuntimeConfig().DiscoveryIndexingEnabled {
		return nil, false
	}
	if s.ensureDiscoveryIndexes() != nil {
		return nil, false
	}
	query := strings.TrimSpace(stringArg(arguments, "query"))
	if query == "" {
		return nil, false
	}
	limit := intArg(arguments, "limit")
	if limit <= 0 {
		limit = 12
	}
	domains := listArg(arguments, "domain", "domains")
	moduleKeys := listArg(arguments, "module_key", "module_keys")
	labels := listArg(arguments, "label", "labels")
	sourceType := stringArg(arguments, "source_type")
	req := search.QueryRequest{
		Mode:       s.toolDiscoveryMode(),
		Query:      query,
		VectorText: query,
		Page:       1,
		PageSize:   candidateWindow(limit),
		Filters:    map[string]string{},
	}
	if len(moduleKeys) == 1 {
		req.Filters["module_key"] = moduleKeys[0]
	}
	if sourceType != "" {
		req.Filters["source_type"] = sourceType
	}
	result, err := s.search.Query(mcpToolsDiscoveryIndexKey, "global", "", req)
	if err != nil {
		return nil, false
	}
	activeCapabilities := s.normalizeCompactCapabilities(catalogOpts)
	items := make([]toolSummary, 0, len(result.Hits))
	seen := make(map[string]struct{}, len(result.Hits))
	for _, hit := range result.Hits {
		summary := toolSummary{
			ToolID:      stringValue(hit.Fields["tool_id"]),
			Name:        stringValue(hit.Fields["name"]),
			Title:       stringValue(hit.Fields["title"]),
			Description: stringValue(hit.Fields["description"]),
			ModuleKey:   stringValue(hit.Fields["module_key"]),
			SourceType:  stringValue(hit.Fields["source_type"]),
			Domains:     stringSliceValue(hit.Fields["business_domains"]),
			Labels:      stringSliceValue(hit.Fields["labels"]),
		}
		descriptor, ok := s.toolDescriptorByName(actor, summary.ToolID)
		if !ok {
			continue
		}
		if len(activeCapabilities) > 0 && !intersectsStrings(descriptor.CapabilityKeys, activeCapabilities) {
			continue
		}
		if len(domains) > 0 && !intersectsStrings(summary.Domains, domains) {
			continue
		}
		if len(moduleKeys) > 0 && !containsString(moduleKeys, summary.ModuleKey) {
			continue
		}
		if len(labels) > 0 && !intersectsStrings(summary.Labels, labels) {
			continue
		}
		if sourceType != "" && summary.SourceType != sourceType {
			continue
		}
		items = append(items, summary)
		seen[summary.ToolID] = struct{}{}
	}
	for _, summary := range s.lexicalToolSearchCandidates(actor, arguments, activeCapabilities, candidateWindow(limit)) {
		if _, ok := seen[summary.ToolID]; ok {
			continue
		}
		items = append(items, summary)
		seen[summary.ToolID] = struct{}{}
	}
	sort.SliceStable(items, func(i, j int) bool {
		left := toolSearchScore(items[i], strings.ToLower(query), searchTerms(query))
		right := toolSearchScore(items[j], strings.ToLower(query), searchTerms(query))
		if left == right {
			return items[i].Name < items[j].Name
		}
		return left > right
	})
	if len(items) > limit {
		items = items[:limit]
	}
	return items, true
}

func (s *Server) searchPlaybookSummaries(arguments map[string]any) ([]PlaybookSummary, bool) {
	if s == nil || s.search == nil {
		return nil, false
	}
	if !s.mcpRuntimeConfig().DiscoveryIndexingEnabled {
		return nil, false
	}
	if s.ensureDiscoveryIndexes() != nil {
		return nil, false
	}
	query := strings.TrimSpace(stringArg(arguments, "query"))
	if query == "" {
		return nil, false
	}
	limit := intArg(arguments, "limit")
	if limit <= 0 {
		limit = 8
	}
	domain := strings.ToLower(strings.TrimSpace(stringArg(arguments, "domain")))
	label := strings.ToLower(strings.TrimSpace(stringArg(arguments, "label")))
	result, err := s.search.Query(mcpPlaybooksDiscoveryIndexKey, "global", "", search.QueryRequest{
		Mode:       s.playbookDiscoveryMode(),
		Query:      query,
		VectorText: query,
		Page:       1,
		PageSize:   candidateWindow(limit),
	})
	if err != nil {
		return nil, false
	}
	index := make(map[string]PlaybookDefinition)
	for _, item := range s.playbooks() {
		index[item.ID] = item
	}
	items := make([]PlaybookSummary, 0, len(result.Hits))
	seen := make(map[string]struct{}, len(result.Hits))
	for _, hit := range result.Hits {
		playbookID := stringValue(hit.Fields["playbook_id"])
		item, ok := index[playbookID]
		if !ok {
			continue
		}
		summary := playbookSummary(item)
		if domain != "" && !containsCaseFold(summary.Domains, domain) {
			continue
		}
		if label != "" && !containsCaseFold(summary.Labels, label) {
			continue
		}
		items = append(items, summary)
		seen[summary.ID] = struct{}{}
	}
	for _, summary := range s.lexicalPlaybookSearchCandidates(arguments, candidateWindow(limit)) {
		if _, ok := seen[summary.ID]; ok {
			continue
		}
		items = append(items, summary)
		seen[summary.ID] = struct{}{}
	}
	sort.SliceStable(items, func(i, j int) bool {
		left := playbookSearchScore(index[items[i].ID], strings.ToLower(query), searchTerms(query))
		right := playbookSearchScore(index[items[j].ID], strings.ToLower(query), searchTerms(query))
		if left == right {
			return items[i].Name < items[j].Name
		}
		return left > right
	})
	if len(items) > limit {
		items = items[:limit]
	}
	return items, true
}

func candidateWindow(limit int) int {
	if limit <= 0 {
		return 40
	}
	window := limit * 8
	if window < 40 {
		window = 40
	}
	if window > 200 {
		window = 200
	}
	return window
}

func stringSliceValue(value any) []string {
	switch current := value.(type) {
	case []string:
		return append([]string(nil), current...)
	case []any:
		out := make([]string, 0, len(current))
		for _, item := range current {
			if text := strings.TrimSpace(stringValue(item)); text != "" {
				out = append(out, text)
			}
		}
		return out
	default:
		text := strings.TrimSpace(stringValue(value))
		if text == "" {
			return nil
		}
		return []string{text}
	}
}

func (s *Server) lexicalToolSearchCandidates(actor ActorContext, arguments map[string]any, activeCapabilities []string, limit int) []toolSummary {
	domains := listArg(arguments, "domain", "domains")
	moduleKeys := listArg(arguments, "module_key", "module_keys")
	labels := listArg(arguments, "label", "labels")
	sourceType := stringArg(arguments, "source_type")
	query := strings.ToLower(strings.TrimSpace(stringArg(arguments, "query")))
	terms := searchTerms(query)
	items := make([]toolSummary, 0)
	for _, item := range s.discoverableTools(actor) {
		if len(activeCapabilities) > 0 && !intersectsStrings(item.CapabilityKeys, activeCapabilities) {
			continue
		}
		summary := toToolSummary(item)
		if len(domains) > 0 && !intersectsStrings(summary.Domains, domains) {
			continue
		}
		if len(moduleKeys) > 0 && !containsString(moduleKeys, summary.ModuleKey) {
			continue
		}
		if len(labels) > 0 && !intersectsStrings(summary.Labels, labels) {
			continue
		}
		if sourceType != "" && strings.TrimSpace(summary.SourceType) != sourceType {
			continue
		}
		if query != "" {
			haystack := strings.ToLower(strings.Join([]string{
				summary.Name,
				summary.Title,
				summary.Description,
				strings.Join(summary.Domains, " "),
				strings.Join(summary.Labels, " "),
			}, " "))
			score := toolSearchScore(summary, query, terms)
			if score <= 0 || !searchTermsMatch(haystack, terms) {
				continue
			}
		}
		items = append(items, summary)
	}
	sort.SliceStable(items, func(i, j int) bool {
		left := toolSearchScore(items[i], query, terms)
		right := toolSearchScore(items[j], query, terms)
		if left == right {
			return items[i].Name < items[j].Name
		}
		return left > right
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items
}

func (s *Server) lexicalPlaybookSearchCandidates(arguments map[string]any, limit int) []PlaybookSummary {
	domain := strings.ToLower(strings.TrimSpace(stringArg(arguments, "domain")))
	label := strings.ToLower(strings.TrimSpace(stringArg(arguments, "label")))
	query := strings.ToLower(strings.TrimSpace(stringArg(arguments, "query")))
	terms := searchTerms(query)
	items := s.playbooks()
	out := make([]PlaybookSummary, 0, len(items))
	for _, item := range items {
		summary := playbookSummary(item)
		if domain != "" && !containsCaseFold(summary.Domains, domain) {
			continue
		}
		if label != "" && !containsCaseFold(summary.Labels, label) {
			continue
		}
		if query != "" {
			haystack := strings.ToLower(strings.Join([]string{
				item.ID,
				item.Name,
				item.Description,
				item.UseWhen,
				strings.Join(item.Domains, " "),
				strings.Join(item.Labels, " "),
				strings.Join(item.Keywords, " "),
			}, " "))
			score := playbookSearchScore(item, query, terms)
			if score <= 0 || !searchTermsMatch(haystack, terms) {
				continue
			}
		}
		out = append(out, summary)
	}
	index := make(map[string]PlaybookDefinition, len(items))
	for _, item := range items {
		index[item.ID] = item
	}
	sort.SliceStable(out, func(i, j int) bool {
		left := playbookSearchScore(index[out[i].ID], query, terms)
		right := playbookSearchScore(index[out[j].ID], query, terms)
		if left == right {
			return out[i].Name < out[j].Name
		}
		return left > right
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}
