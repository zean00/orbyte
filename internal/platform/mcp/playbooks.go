package mcp

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"
)

const (
	MCPExposureModeFull    = "full"
	MCPExposureModeCompact = "compact"
	MCPExposureModeMinimal = "minimal"
)

type PlaybookDefinition struct {
	ID                   string             `json:"id"`
	Name                 string             `json:"name"`
	Description          string             `json:"description,omitempty"`
	Domains              []string           `json:"domains,omitempty"`
	Labels               []string           `json:"labels,omitempty"`
	Keywords             []string           `json:"keywords,omitempty"`
	UseWhen              string             `json:"use_when,omitempty"`
	WorkflowSteps        []PlaybookToolStep `json:"workflow_steps,omitempty"`
	ToolInventory        []string           `json:"tool_inventory,omitempty"`
	RequiredFinalFacts   []string           `json:"required_final_facts,omitempty"`
	RequiredArtifacts    []string           `json:"required_artifacts,omitempty"`
	RequiredDraftOutputs []string           `json:"required_draft_outputs,omitempty"`
	Guardrails           []string           `json:"guardrails,omitempty"`
	SuccessChecks        []string           `json:"success_checks,omitempty"`
	Examples             []string           `json:"examples,omitempty"`
	Constraints          []string           `json:"constraints,omitempty"`
	Pitfalls             []string           `json:"pitfalls,omitempty"`
}

type PlaybookToolStep struct {
	Step        string         `json:"step"`
	Title       string         `json:"title,omitempty"`
	ToolID      string         `json:"tool_id"`
	Description string         `json:"description,omitempty"`
	Required    bool           `json:"required,omitempty"`
	When        string         `json:"when,omitempty"`
	Output      string         `json:"output,omitempty"`
	Arguments   map[string]any `json:"arguments,omitempty"`
}

type playbookDefinitionAlias struct {
	ID                   string              `json:"id"`
	Name                 string              `json:"name"`
	Description          string              `json:"description,omitempty"`
	Domains              []string            `json:"domains,omitempty"`
	Labels               []string            `json:"labels,omitempty"`
	Keywords             []string            `json:"keywords,omitempty"`
	UseWhen              string              `json:"use_when,omitempty"`
	WorkflowSteps        json.RawMessage     `json:"workflow_steps,omitempty"`
	ToolSequence         []PlaybookToolStep  `json:"tool_sequence,omitempty"`
	ToolIDs              []string            `json:"tool_ids,omitempty"`
	ToolInventory        []string            `json:"tool_inventory,omitempty"`
	RequiredFinalFacts   []string            `json:"required_final_facts,omitempty"`
	RequiredArtifacts    []string            `json:"required_artifacts,omitempty"`
	RequiredDraftOutputs []string            `json:"required_draft_outputs,omitempty"`
	Guardrails           []string            `json:"guardrails,omitempty"`
	SuccessChecks        []string            `json:"success_checks,omitempty"`
	Examples             []string            `json:"examples,omitempty"`
	Constraints          []string            `json:"constraints,omitempty"`
	Pitfalls             []string            `json:"pitfalls,omitempty"`
}

func (p *PlaybookDefinition) UnmarshalJSON(data []byte) error {
	var raw playbookDefinitionAlias
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*p = PlaybookDefinition{
		ID:                   raw.ID,
		Name:                 raw.Name,
		Description:          raw.Description,
		Domains:              raw.Domains,
		Labels:               raw.Labels,
		Keywords:             raw.Keywords,
		UseWhen:              raw.UseWhen,
		RequiredFinalFacts:   raw.RequiredFinalFacts,
		RequiredArtifacts:    raw.RequiredArtifacts,
		RequiredDraftOutputs: raw.RequiredDraftOutputs,
		Guardrails:           raw.Guardrails,
		SuccessChecks:        raw.SuccessChecks,
		Examples:             raw.Examples,
		Constraints:          raw.Constraints,
		Pitfalls:             raw.Pitfalls,
	}
	if len(raw.WorkflowSteps) > 0 {
		var typed []PlaybookToolStep
		if err := json.Unmarshal(raw.WorkflowSteps, &typed); err == nil {
			p.WorkflowSteps = typed
		} else {
			var legacy []string
			if err := json.Unmarshal(raw.WorkflowSteps, &legacy); err == nil {
				p.WorkflowSteps = legacyStringsToWorkflowSteps(legacy)
			}
		}
	}
	if len(raw.ToolSequence) > 0 {
		p.WorkflowSteps = append([]PlaybookToolStep(nil), raw.ToolSequence...)
	}
	if len(raw.ToolInventory) > 0 {
		p.ToolInventory = append([]string(nil), raw.ToolInventory...)
	} else {
		p.ToolInventory = append([]string(nil), raw.ToolIDs...)
	}
	return nil
}

func legacyStringsToWorkflowSteps(items []string) []PlaybookToolStep {
	out := make([]PlaybookToolStep, 0, len(items))
	for index, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		out = append(out, PlaybookToolStep{
			Step:        "step_" + strings.TrimSpace(strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(item, " ", "_"), "-", "_"), ".", ""))),
			Title:       "Step " + strconv.Itoa(index+1),
			Description: item,
		})
	}
	return out
}

type PlaybookSummary struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Description      string   `json:"description,omitempty"`
	Domains          []string `json:"domains,omitempty"`
	Labels           []string `json:"labels,omitempty"`
	RecommendedTools []string `json:"recommended_tools,omitempty"`
}

func normalizeExposureMode(mode string) string {
	trimmed := strings.TrimSpace(strings.ToLower(mode))
	if trimmed == "" {
		return ""
	}
	switch trimmed {
	case MCPExposureModeCompact:
		return MCPExposureModeCompact
	case MCPExposureModeMinimal:
		return MCPExposureModeMinimal
	default:
		return MCPExposureModeFull
	}
}

func normalizePlaybookDefinition(item PlaybookDefinition) PlaybookDefinition {
	item.ID = strings.TrimSpace(item.ID)
	item.Name = strings.TrimSpace(item.Name)
	item.Description = strings.TrimSpace(item.Description)
	item.UseWhen = strings.TrimSpace(item.UseWhen)
	item.Domains = mergeUniqueStrings(nil, item.Domains)
	item.Labels = mergeUniqueStrings(nil, item.Labels)
	item.Keywords = mergeUniqueStrings(nil, item.Keywords)
	item.WorkflowSteps = normalizePlaybookToolSteps(item.WorkflowSteps)
	item.ToolInventory = mergeUniqueStrings(nil, item.ToolInventory)
	if len(item.ToolInventory) == 0 {
		item.ToolInventory = workflowToolInventory(item.WorkflowSteps)
	}
	item.RequiredFinalFacts = trimNonEmptyStrings(item.RequiredFinalFacts)
	item.RequiredArtifacts = trimNonEmptyStrings(item.RequiredArtifacts)
	item.RequiredDraftOutputs = trimNonEmptyStrings(item.RequiredDraftOutputs)
	item.Guardrails = trimNonEmptyStrings(item.Guardrails)
	item.SuccessChecks = trimNonEmptyStrings(item.SuccessChecks)
	item.Examples = trimNonEmptyStrings(item.Examples)
	item.Constraints = trimNonEmptyStrings(item.Constraints)
	item.Pitfalls = trimNonEmptyStrings(item.Pitfalls)
	if item.ID == "" {
		item.ID = strings.ToLower(strings.ReplaceAll(item.Name, " ", "_"))
	}
	return item
}

func normalizePlaybookToolSteps(items []PlaybookToolStep) []PlaybookToolStep {
	out := make([]PlaybookToolStep, 0, len(items))
	for _, item := range items {
		item.Step = strings.TrimSpace(item.Step)
		item.Title = strings.TrimSpace(item.Title)
		item.ToolID = strings.TrimSpace(item.ToolID)
		item.Description = strings.TrimSpace(item.Description)
		item.When = strings.TrimSpace(item.When)
		item.Output = strings.TrimSpace(item.Output)
		if item.Step == "" && item.Title == "" && item.ToolID == "" && item.Description == "" {
			continue
		}
		if item.Step == "" {
			item.Step = strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(item.Title), " ", "_"), "-", "_"))
		}
		out = append(out, item)
	}
	return out
}

func workflowToolInventory(items []PlaybookToolStep) []string {
	toolIDs := make([]string, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.ToolID) == "" {
			continue
		}
		toolIDs = append(toolIDs, item.ToolID)
	}
	return mergeUniqueStrings(nil, toolIDs)
}

func trimNonEmptyStrings(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		out = append(out, item)
	}
	return out
}

func (s *Server) playbooks() []PlaybookDefinition {
	cfg := s.mcpRuntimeConfig()
	items := make([]PlaybookDefinition, 0, len(cfg.Playbooks))
	for _, item := range cfg.Playbooks {
		item = normalizePlaybookDefinition(item)
		if item.ID == "" || item.Name == "" {
			continue
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Name == items[j].Name {
			return items[i].ID < items[j].ID
		}
		return items[i].Name < items[j].Name
	})
	return items
}

func playbookSummary(item PlaybookDefinition) PlaybookSummary {
	recommended := playbookRecommendedTools(item)
	return PlaybookSummary{
		ID:               item.ID,
		Name:             item.Name,
		Description:      item.Description,
		Domains:          append([]string(nil), item.Domains...),
		Labels:           append([]string(nil), item.Labels...),
		RecommendedTools: recommended,
	}
}

func playbookRecommendedTools(item PlaybookDefinition) []string {
	seen := make(map[string]struct{})
	recommended := make([]string, 0, 5)
	for _, step := range item.WorkflowSteps {
		toolID := strings.TrimSpace(step.ToolID)
		if toolID == "" {
			continue
		}
		if _, ok := seen[toolID]; ok {
			continue
		}
		seen[toolID] = struct{}{}
		recommended = append(recommended, toolID)
		if len(recommended) >= 5 {
			break
		}
	}
	if len(recommended) == 0 {
	for _, toolID := range item.ToolInventory {
			toolID = strings.TrimSpace(toolID)
			if toolID == "" {
				continue
			}
			if _, ok := seen[toolID]; ok {
				continue
			}
			seen[toolID] = struct{}{}
			recommended = append(recommended, toolID)
			if len(recommended) >= 5 {
				break
			}
		}
	}
	return recommended
}

func (s *Server) discoverableTools(actor ActorContext) []ToolDescriptor {
	all := s.listTools(actor)
	items := make([]ToolDescriptor, 0, len(all))
	for _, item := range all {
		if isMetaToolName(item.Name) {
			continue
		}
		items = append(items, item)
	}
	return items
}

func isMetaToolName(name string) bool {
	name = strings.TrimSpace(name)
	return strings.HasPrefix(name, "tools.") || strings.HasPrefix(name, "playbooks.") || strings.HasPrefix(name, "skills.")
}

func parsePlaybooks(value any) []PlaybookDefinition {
	switch raw := value.(type) {
	case string:
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			return nil
		}
		var items []PlaybookDefinition
		_ = json.Unmarshal([]byte(trimmed), &items)
		return items
	case []PlaybookDefinition:
		return append([]PlaybookDefinition(nil), raw...)
	case []any:
		data, _ := json.Marshal(raw)
		var items []PlaybookDefinition
		_ = json.Unmarshal(data, &items)
		return items
	default:
		return nil
	}
}
