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

func (s *Server) filterToolSummaries(actor ActorContext, arguments map[string]any, search bool) []toolSummary {
	descriptors := s.discoverableTools(actor)
	domains := listArg(arguments, "domain", "domains")
	labels := listArg(arguments, "label", "labels")
	sourceType := stringArg(arguments, "source_type")
	query := strings.ToLower(strings.TrimSpace(stringArg(arguments, "query")))
	items := make([]toolSummary, 0, len(descriptors))
	for _, item := range descriptors {
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
			if !strings.Contains(haystack, query) {
				continue
			}
		}
		items = append(items, summary)
	}
	sort.Slice(items, func(i, j int) bool {
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

func (s *Server) toolsListMeta(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	items := s.filterToolSummaries(actor, arguments, false)
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Found %d discoverable tools.", len(items))}},
		"structuredContent": map[string]any{"items": items},
	}, true, nil
}

func (s *Server) toolsSearchMeta(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	items := s.filterToolSummaries(actor, arguments, true)
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
	items := make([]ToolDescriptor, 0, len(toolIDs))
	for _, toolID := range toolIDs {
		descriptor, ok := s.toolDescriptorByName(actor, toolID)
		if !ok {
			return nil, true, shared.Validation("tool_id not found: " + toolID)
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
	out := make([]PlaybookSummary, 0, len(items))
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
			if !strings.Contains(haystack, query) {
				continue
			}
		}
		out = append(out, summary)
	}
	limit := intArg(arguments, "limit")
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
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
