package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"orbyte/internal/platform/shared"
)

type toolSummary struct {
	ToolID      string   `json:"tool_id"`
	Name        string   `json:"name"`
	Title       string   `json:"title,omitempty"`
	Description string   `json:"description,omitempty"`
	ModuleKey   string   `json:"module_key,omitempty"`
	SourceType  string   `json:"source_type,omitempty"`
	Domains     []string `json:"domains,omitempty"`
	Labels      []string `json:"labels,omitempty"`
}

func minimalToolDescriptors(actor ActorContext) []ToolDescriptor {
	return []ToolDescriptor{
		{Name: "tools.list", Title: "List Available Tools", Description: "List lightweight summaries of discoverable MCP tools with optional domain and label filters.", ModuleKey: "platform.core", SourceType: "built_in", InputSchema: map[string]any{"type": "object", "properties": map[string]any{"domain": map[string]any{"type": "string"}, "domains": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, "label": map[string]any{"type": "string"}, "labels": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, "source_type": map[string]any{"type": "string"}, "limit": map[string]any{"type": "integer"}}}},
		{Name: "tools.search", Title: "Search Tools", Description: "Search discoverable MCP tools by title, description, business domain, and labels.", ModuleKey: "platform.core", SourceType: "built_in", InputSchema: map[string]any{"type": "object", "properties": map[string]any{"query": map[string]any{"type": "string"}, "domain": map[string]any{"type": "string"}, "domains": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, "label": map[string]any{"type": "string"}, "labels": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, "source_type": map[string]any{"type": "string"}, "limit": map[string]any{"type": "integer"}}}},
		{Name: "tools.describe", Title: "Describe Tools", Description: "Get detailed descriptions, schemas, and governance metadata for one or more tools.", ModuleKey: "platform.core", SourceType: "built_in", InputSchema: map[string]any{"type": "object", "properties": map[string]any{"tool_ids": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}}, "required": []string{"tool_ids"}}},
		{Name: "tools.call", Title: "Call Tool", Description: "Call one discoverable MCP tool by id with a validated payload.", ModuleKey: "platform.core", SourceType: "built_in", InputSchema: map[string]any{"type": "object", "properties": map[string]any{"tool_id": map[string]any{"type": "string"}, "payload": map[string]any{"type": "object"}, "catalog_context": map[string]any{"type": "object"}}, "required": []string{"tool_id"}}},
		{Name: "playbooks.list", Title: "List Playbooks", Description: "List workflow playbooks that bundle relevant MCP tools for common use cases.", ModuleKey: "platform.core", SourceType: "built_in", InputSchema: map[string]any{"type": "object", "properties": map[string]any{"domain": map[string]any{"type": "string"}, "label": map[string]any{"type": "string"}, "limit": map[string]any{"type": "integer"}}}},
		{Name: "playbooks.search", Title: "Search Playbooks", Description: "Search workflow playbooks by name, description, domain, labels, and keywords.", ModuleKey: "platform.core", SourceType: "built_in", InputSchema: map[string]any{"type": "object", "properties": map[string]any{"query": map[string]any{"type": "string"}, "domain": map[string]any{"type": "string"}, "label": map[string]any{"type": "string"}, "limit": map[string]any{"type": "integer"}}}},
		{Name: "playbooks.describe", Title: "Describe Playbook", Description: "Get the full workflow contract for one playbook, including trigger conditions, ordered tool sequence, required final facts, required artifacts or draft outputs, guardrails, success checks, pitfalls, and recommended tools.", ModuleKey: "platform.core", SourceType: "built_in", InputSchema: map[string]any{"type": "object", "properties": map[string]any{"playbook_id": map[string]any{"type": "string"}}, "required": []string{"playbook_id"}}},
	}
}

func (s *Server) minimalExposedTools(actor ActorContext) []ToolDescriptor {
	items := minimalToolDescriptors(actor)
	out := make([]ToolDescriptor, 0, len(items))
	for _, item := range items {
		out = append(out, s.decorateToolDescriptorWithGovernance(item, nil))
	}
	return out
}

func toToolSummary(item ToolDescriptor) toolSummary {
	return toolSummary{
		ToolID:      item.Name,
		Name:        item.Name,
		Title:       item.Title,
		Description: item.Description,
		ModuleKey:   item.ModuleKey,
		SourceType:  item.SourceType,
		Domains:     append([]string(nil), item.Contract.BusinessDomains...),
		Labels:      append([]string(nil), item.CapabilityKeys...),
	}
}

func listArg(arguments map[string]any, singular, plural string) []string {
	items := stringArrayArg(arguments, plural)
	if len(items) > 0 {
		return mergeUniqueStrings(nil, items)
	}
	value := stringArg(arguments, singular)
	if value == "" {
		return nil
	}
	return []string{value}
}

func (s *Server) filterToolSummaries(actor ActorContext, arguments map[string]any, search bool, catalogOpts ToolCatalogOptions) []toolSummary {
	descriptors := s.discoverableTools(actor)
	activeCapabilities := s.normalizeCompactCapabilities(catalogOpts)
	domains := listArg(arguments, "domain", "domains")
	labels := listArg(arguments, "label", "labels")
	sourceType := stringArg(arguments, "source_type")
	query := strings.ToLower(strings.TrimSpace(stringArg(arguments, "query")))
	terms := searchTerms(query)
	items := make([]toolSummary, 0, len(descriptors))
	scores := map[string]int{}
	for _, item := range descriptors {
		if len(activeCapabilities) > 0 && !intersectsStrings(item.CapabilityKeys, activeCapabilities) {
			continue
		}
		summary := toToolSummary(item)
		if len(domains) > 0 && !intersectsStrings(summary.Domains, domains) {
			continue
		}
		if len(labels) > 0 && !intersectsStrings(summary.Labels, labels) {
			continue
		}
		if sourceType != "" && strings.TrimSpace(summary.SourceType) != sourceType {
			continue
		}
		if search && query != "" {
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
			scores[summary.Name] = score
		}
		items = append(items, summary)
	}
	sort.Slice(items, func(i, j int) bool {
		if search && query != "" {
			left := scores[items[i].Name]
			right := scores[items[j].Name]
			if left != right {
				return left > right
			}
		}
		if items[i].Name == items[j].Name {
			return items[i].Title < items[j].Title
		}
		return items[i].Name < items[j].Name
	})
	limit := intArg(arguments, "limit")
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items
}

func searchTerms(query string) []string {
	if strings.TrimSpace(query) == "" {
		return nil
	}
	raw := strings.FieldsFunc(strings.ToLower(query), func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9')
	})
	terms := make([]string, 0, len(raw))
	for _, term := range raw {
		if len(term) < 3 {
			continue
		}
		switch term {
		case "the", "and", "for", "with", "that", "this", "from", "into", "show", "using":
			continue
		}
		terms = append(terms, term)
	}
	return mergeUniqueStrings(nil, terms)
}

func searchTermsMatch(haystack string, terms []string) bool {
	if len(terms) == 0 {
		return true
	}
	for _, term := range terms {
		if strings.Contains(haystack, term) {
			return true
		}
	}
	return false
}

func toolSearchScore(item toolSummary, query string, terms []string) int {
	score := 0
	lowerName := strings.ToLower(strings.TrimSpace(item.Name))
	lowerTitle := strings.ToLower(strings.TrimSpace(item.Title))
	lowerDescription := strings.ToLower(strings.TrimSpace(item.Description))
	if lowerName == query {
		score += 100
	}
	if lowerTitle == query {
		score += 90
	}
	if strings.Contains(lowerName, query) {
		score += 40
	}
	if strings.Contains(lowerTitle, query) {
		score += 30
	}
	if strings.Contains(lowerDescription, query) {
		score += 20
	}
	for _, domain := range item.Domains {
		if strings.EqualFold(strings.TrimSpace(domain), strings.TrimSpace(query)) {
			score += 25
		}
	}
	for _, label := range item.Labels {
		if strings.EqualFold(strings.TrimSpace(label), strings.TrimSpace(query)) {
			score += 15
		}
	}
	for _, term := range terms {
		if strings.Contains(lowerName, term) {
			score += 25
		}
		if strings.Contains(lowerTitle, term) {
			score += 18
		}
		if strings.Contains(lowerDescription, term) {
			score += 12
		}
		for _, domain := range item.Domains {
			if strings.EqualFold(strings.TrimSpace(domain), term) {
				score += 15
			}
		}
		for _, label := range item.Labels {
			if strings.EqualFold(strings.TrimSpace(label), term) {
				score += 10
			}
		}
	}
	switch strings.TrimSpace(item.SourceType) {
	case "module":
		score += 5
	case "built_in":
		score += 3
	}
	return score
}

func (s *Server) toolsListMeta(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	catalogOpts := s.catalogOptionsFromExposureMode()
	items := s.filterToolSummaries(actor, arguments, false, catalogOpts)
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Found %d discoverable tools.", len(items))}},
		"structuredContent": map[string]any{"items": items},
	}, true, nil
}

func (s *Server) toolsSearchMeta(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	catalogOpts := s.catalogOptionsFromExposureMode()
	items := s.filterToolSummaries(actor, arguments, true, catalogOpts)
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Matched %d tools.", len(items))}},
		"structuredContent": map[string]any{"items": items},
	}, true, nil
}

func (s *Server) toolsDescribeMeta(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	toolIDs := stringArrayArg(arguments, "tool_ids")
	if len(toolIDs) == 0 {
		return nil, true, shared.Validation("tool_ids is required")
	}
	catalogOpts := s.catalogOptionsFromExposureMode()
	activeCapabilities := s.normalizeCompactCapabilities(catalogOpts)
	items := make([]ToolDescriptor, 0, len(toolIDs))
	for _, toolID := range toolIDs {
		descriptor, ok := s.toolDescriptorByName(actor, toolID)
		if !ok {
			return nil, true, shared.Validation("tool_id not found: " + toolID)
		}
		if len(activeCapabilities) > 0 && !intersectsStrings(descriptor.CapabilityKeys, activeCapabilities) {
			continue
		}
		items = append(items, descriptor)
	}
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded %d tool descriptions.", len(items))}},
		"structuredContent": map[string]any{"items": items},
	}, true, nil
}

func (s *Server) toolsCallMeta(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	toolID := stringArg(arguments, "tool_id")
	if toolID == "" {
		return nil, true, shared.Validation("tool_id is required")
	}
	if isMetaToolName(toolID) {
		return nil, true, shared.Validation("tools.call cannot invoke meta tools")
	}
	payload := mapArg(arguments, "payload")
	if payload == nil {
		payload = map[string]any{}
	}
	catalogContext := ToolCatalogOptions{}
	if raw := mapArg(arguments, "catalog_context"); raw != nil {
		data, _ := json.Marshal(raw)
		catalogContext = parseToolCatalogOptions(data)
	}
	result, err := s.callTool(context.Background(), actor, toolID, payload, catalogContext)
	if err != nil {
		return nil, true, err
	}
	return result, true, nil
}

func (s *Server) filterPlaybookSummaries(arguments map[string]any, search bool) []PlaybookSummary {
	items := s.playbooks()
	domain := strings.ToLower(strings.TrimSpace(stringArg(arguments, "domain")))
	label := strings.ToLower(strings.TrimSpace(stringArg(arguments, "label")))
	query := strings.ToLower(strings.TrimSpace(stringArg(arguments, "query")))
	terms := searchTerms(query)
	out := make([]PlaybookSummary, 0, len(items))
	scores := map[string]int{}
	for _, item := range items {
		summary := playbookSummary(item)
		if domain != "" && !containsCaseFold(summary.Domains, domain) {
			continue
		}
		if label != "" && !containsCaseFold(summary.Labels, label) {
			continue
		}
		if search && query != "" {
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
			scores[summary.ID] = score
		}
		out = append(out, summary)
	}
	sort.Slice(out, func(i, j int) bool {
		if search && query != "" {
			left := scores[out[i].ID]
			right := scores[out[j].ID]
			if left != right {
				return left > right
			}
		}
		if out[i].Name == out[j].Name {
			return out[i].ID < out[j].ID
		}
		return out[i].Name < out[j].Name
	})
	limit := intArg(arguments, "limit")
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func playbookSearchScore(item PlaybookDefinition, query string, terms []string) int {
	score := 0
	lowerID := strings.ToLower(strings.TrimSpace(item.ID))
	lowerName := strings.ToLower(strings.TrimSpace(item.Name))
	lowerDescription := strings.ToLower(strings.TrimSpace(item.Description))
	lowerUseWhen := strings.ToLower(strings.TrimSpace(item.UseWhen))
	if lowerID == query {
		score += 100
	}
	if lowerName == query {
		score += 90
	}
	if strings.Contains(lowerID, query) {
		score += 40
	}
	if strings.Contains(lowerName, query) {
		score += 35
	}
	if strings.Contains(lowerDescription, query) {
		score += 25
	}
	if strings.Contains(lowerUseWhen, query) {
		score += 20
	}
	for _, term := range terms {
		if strings.Contains(lowerID, term) {
			score += 20
		}
		if strings.Contains(lowerName, term) {
			score += 18
		}
		if strings.Contains(lowerDescription, term) {
			score += 14
		}
		if strings.Contains(lowerUseWhen, term) {
			score += 12
		}
		for _, domain := range item.Domains {
			if strings.EqualFold(strings.TrimSpace(domain), term) {
				score += 15
			}
		}
		for _, label := range item.Labels {
			if strings.EqualFold(strings.TrimSpace(label), term) {
				score += 10
			}
		}
		for _, keyword := range item.Keywords {
			if strings.EqualFold(strings.TrimSpace(keyword), term) {
				score += 12
			}
		}
	}
	return score
}

func containsCaseFold(items []string, needle string) bool {
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item), strings.TrimSpace(needle)) {
			return true
		}
	}
	return false
}

func (s *Server) playbooksListMeta(_ ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	items := s.filterPlaybookSummaries(arguments, false)
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Found %d playbooks.", len(items))}},
		"structuredContent": map[string]any{"items": items},
	}, true, nil
}

func (s *Server) playbooksSearchMeta(_ ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	items := s.filterPlaybookSummaries(arguments, true)
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Matched %d playbooks.", len(items))}},
		"structuredContent": map[string]any{"items": items},
	}, true, nil
}

func (s *Server) playbooksDescribeMeta(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	playbookID := stringArg(arguments, "playbook_id")
	if playbookID == "" {
		return nil, true, shared.Validation("playbook_id is required")
	}
	for _, item := range s.playbooks() {
		if item.ID != playbookID {
			continue
		}
		describedTools := make([]ToolDescriptor, 0, len(item.ToolIDs))
		for _, toolID := range item.ToolIDs {
			if descriptor, ok := s.toolDescriptorByName(actor, toolID); ok {
				describedTools = append(describedTools, descriptor)
			}
		}
		return map[string]any{
			"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded playbook %s.", item.Name)}},
			"structuredContent": map[string]any{
				"playbook": item,
				"tools":    describedTools,
			},
		}, true, nil
	}
	return nil, true, shared.Validation("playbook_id not found")
}
