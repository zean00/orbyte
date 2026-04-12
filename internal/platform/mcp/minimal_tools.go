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
		{Name: "skills.list", Title: "List Skills", Description: "Required first step for workflow-like business tasks in minimal mode. List workflow skills to find one matching the current use case before searching individual tools. Only fall back to tools.search or tools.list when no skill fits.", ModuleKey: "platform.core", SourceType: "built_in", InputSchema: map[string]any{"type": "object", "properties": map[string]any{"domain": map[string]any{"type": "string"}, "label": map[string]any{"type": "string"}, "limit": map[string]any{"type": "integer"}}}},
		{Name: "skills.search", Title: "Search Skills", Description: "Required first step for workflow-like business tasks in minimal mode. Search skills by use case or intent, try to find a matching skill first before searching individual tools, then call skills.describe and follow the matched workflow.", ModuleKey: "platform.core", SourceType: "built_in", InputSchema: map[string]any{"type": "object", "properties": map[string]any{"query": map[string]any{"type": "string"}, "domain": map[string]any{"type": "string"}, "label": map[string]any{"type": "string"}, "limit": map[string]any{"type": "integer"}}}},
		{Name: "skills.describe", Title: "Describe Skill", Description: "Get one or more full skill workflow contracts with ordered tool sequences, guardrails, and success checks. Prefer passing all matched skill ids in one bulk call.", ModuleKey: "platform.core", SourceType: "built_in", InputSchema: map[string]any{"type": "object", "properties": map[string]any{"skill_id": map[string]any{"type": "string"}, "skill_ids": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, "playbook_id": map[string]any{"type": "string"}, "playbook_ids": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}}, "anyOf": []map[string]any{{"required": []string{"skill_id"}}, {"required": []string{"skill_ids"}}, {"required": []string{"playbook_id"}}, {"required": []string{"playbook_ids"}}}}},
		{Name: "tools.search", Title: "Search Tools", Description: "Fallback step only when no playbook matches. Search discoverable MCP tools by title, description, business domain, and labels after playbook discovery fails. Pass ALL candidate IDs to tools.describe in a single call.", ModuleKey: "platform.core", SourceType: "built_in", InputSchema: map[string]any{"type": "object", "properties": map[string]any{"query": map[string]any{"type": "string"}, "domain": map[string]any{"type": "string"}, "domains": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, "module_key": map[string]any{"type": "string"}, "module_keys": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, "label": map[string]any{"type": "string"}, "labels": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, "source_type": map[string]any{"type": "string"}, "limit": map[string]any{"type": "integer"}}}},
		{Name: "tools.list", Title: "List Available Tools", Description: "Fallback step only when no playbook matches. List lightweight summaries of discoverable MCP tools after playbook discovery fails. For details, pass ALL candidate IDs to tools.describe in a single call.", ModuleKey: "platform.core", SourceType: "built_in", InputSchema: map[string]any{"type": "object", "properties": map[string]any{"domain": map[string]any{"type": "string"}, "domains": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, "module_key": map[string]any{"type": "string"}, "module_keys": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, "label": map[string]any{"type": "string"}, "labels": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, "source_type": map[string]any{"type": "string"}, "limit": map[string]any{"type": "integer"}}}},
		{Name: "tools.describe", Title: "Describe Tools", Description: "Get detailed descriptions, schemas, and governance metadata for one or more tools. Always pass ALL candidate tool IDs from tools/search in a single call — do not describe tools one at a time.", ModuleKey: "platform.core", SourceType: "built_in", InputSchema: map[string]any{"type": "object", "properties": map[string]any{"tool_ids": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}}, "required": []string{"tool_ids"}}},
		{Name: "tools.call", Title: "Call Tool", Description: "Call one discoverable MCP tool by id with a validated payload.", ModuleKey: "platform.core", SourceType: "built_in", InputSchema: map[string]any{"type": "object", "properties": map[string]any{"tool_id": map[string]any{"type": "string"}, "payload": map[string]any{"type": "object"}, "catalog_context": map[string]any{"type": "object"}}, "required": []string{"tool_id"}}},
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

func summarizeToolMatches(items []toolSummary, noun string) string {
	if len(items) == 0 {
		return fmt.Sprintf("Matched 0 %s.", noun)
	}
	lines := []string{fmt.Sprintf("Matched %d %s. Use the exact tool_id values below.", len(items), noun)}
	limit := len(items)
	if limit > 12 {
		limit = 12
	}
	for index := 0; index < limit; index++ {
		item := items[index]
		label := strings.TrimSpace(item.Title)
		if label == "" {
			label = item.Name
		}
		lines = append(lines, fmt.Sprintf("- tool_id: %s | title: %s", item.ToolID, label))
	}
	if len(items) > limit {
		lines = append(lines, fmt.Sprintf("- ... %d more tools in structuredContent.items", len(items)-limit))
	}
	lines = append(lines, "Next step: pass one or more exact tool_id values to tools.describe.")
	return strings.Join(lines, "\n")
}

func summarizePlaybookMatches(items []PlaybookSummary, noun string) string {
	if len(items) == 0 {
		return fmt.Sprintf("Matched 0 %s.", noun)
	}
	lines := []string{fmt.Sprintf("Matched %d %s. Use the exact skill_id values below.", len(items), noun)}
	limit := len(items)
	if limit > 12 {
		limit = 12
	}
	for index := 0; index < limit; index++ {
		item := items[index]
		label := strings.TrimSpace(item.Name)
		if label == "" {
			label = item.ID
		}
		lines = append(lines, fmt.Sprintf("- skill_id: %s | name: %s", item.ID, label))
	}
	if len(items) > limit {
		lines = append(lines, fmt.Sprintf("- ... %d more skills in structuredContent.items", len(items)-limit))
	}
	lines = append(lines, "Next step: pass one or more exact skill_id values to skills.describe.")
	return strings.Join(lines, "\n")
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
	if search {
		if items, ok := s.searchToolSummaries(actor, arguments, catalogOpts); ok {
			return items
		}
	}
	descriptors := s.discoverableTools(actor)
	activeCapabilities := s.normalizeCompactCapabilities(catalogOpts)
	domains := listArg(arguments, "domain", "domains")
	moduleKeys := listArg(arguments, "module_key", "module_keys")
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
		if len(moduleKeys) > 0 && !containsString(moduleKeys, summary.ModuleKey) {
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
		"content":           []ContentBlock{{Type: "text", Text: summarizeToolMatches(items, "discoverable tools")}},
		"structuredContent": map[string]any{"items": items},
	}, true, nil
}

func (s *Server) toolsSearchMeta(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	catalogOpts := s.catalogOptionsFromExposureMode()
	items := s.filterToolSummaries(actor, arguments, true, catalogOpts)
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: summarizeToolMatches(items, "tools")}},
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
	if search {
		if items, ok := s.searchPlaybookSummaries(arguments); ok {
			return items
		}
	}
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
		"content":           []ContentBlock{{Type: "text", Text: summarizePlaybookMatches(items, "skills")}},
		"structuredContent": map[string]any{"items": items},
	}, true, nil
}

func (s *Server) playbooksSearchMeta(_ ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	items := s.filterPlaybookSummaries(arguments, true)
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: summarizePlaybookMatches(items, "skills")}},
		"structuredContent": map[string]any{"items": items},
	}, true, nil
}

func (s *Server) playbooksDescribeMeta(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	playbookIDs := stringArrayArg(arguments, "playbook_ids")
	if len(playbookIDs) == 0 {
		playbookIDs = stringArrayArg(arguments, "skill_ids")
	}
	if len(playbookIDs) == 0 {
		if playbookID := stringArg(arguments, "playbook_id"); playbookID != "" {
			playbookIDs = []string{playbookID}
		} else if skillID := stringArg(arguments, "skill_id"); skillID != "" {
			playbookIDs = []string{skillID}
		}
	}
	if len(playbookIDs) == 0 {
		return nil, true, shared.Validation("skill_id, skill_ids, playbook_id, or playbook_ids is required")
	}
	index := make(map[string]PlaybookDefinition)
	for _, item := range s.playbooks() {
		index[item.ID] = item
	}
	items := make([]PlaybookDefinition, 0, len(playbookIDs))
	toolsByPlaybook := make(map[string][]ToolDescriptor, len(playbookIDs))
	for _, playbookID := range mergeUniqueStrings(nil, playbookIDs) {
		item, ok := index[playbookID]
		if !ok {
			return nil, true, shared.Validation("playbook_id not found: " + playbookID)
		}
		items = append(items, item)
		describedTools := make([]ToolDescriptor, 0, len(item.ToolInventory))
		for _, toolID := range item.ToolInventory {
			if descriptor, ok := s.toolDescriptorByName(actor, toolID); ok {
				describedTools = append(describedTools, descriptor)
			}
		}
		toolsByPlaybook[item.ID] = describedTools
	}
	content := fmt.Sprintf("Loaded %d playbook workflow contract(s).", len(items))
	structured := map[string]any{
		"items":             items,
		"tools_by_playbook": toolsByPlaybook,
	}
	if len(items) == 1 {
		content = fmt.Sprintf("Loaded playbook %s.", items[0].Name)
		structured["playbook"] = items[0]
		structured["tools"] = toolsByPlaybook[items[0].ID]
	}
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: content}},
		"structuredContent": structured,
	}, true, nil
}
