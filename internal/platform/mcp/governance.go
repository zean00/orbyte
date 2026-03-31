package mcp

import (
	"encoding/json"
	"fmt"
	"strings"
)

type domainGovernanceOverride struct {
	BlockedActionClasses       []string `json:"blocked_action_classes,omitempty"`
	BlockedToolKeys            []string `json:"blocked_tool_keys,omitempty"`
	BlockedDocumentTypes       []string `json:"blocked_document_types,omitempty"`
	AllowedSubmitDocumentTypes []string `json:"allowed_submit_document_types,omitempty"`
}

type governanceEvaluation struct {
	Allowed             bool
	PolicyState         string
	PolicyReason        string
	EffectiveVisibility string
}

func (s *Server) decorateToolDescriptorWithGovernance(descriptor ToolDescriptor, arguments map[string]any) ToolDescriptor {
	evaluation := s.evaluateToolGovernance(descriptor, arguments)
	descriptor.PolicyState = evaluation.PolicyState
	descriptor.PolicyReason = evaluation.PolicyReason
	descriptor.EffectiveVisibility = evaluation.EffectiveVisibility
	return descriptor
}

func (s *Server) evaluateToolGovernance(descriptor ToolDescriptor, arguments map[string]any) governanceEvaluation {
	evaluation := defaultGovernanceEvaluation(descriptor)
	cfg := s.mcpRuntimeConfig()
	if !cfg.GovernanceEnabled || !toolUsesGovernancePolicy(descriptor) {
		return evaluation
	}

	toolKey := strings.TrimSpace(descriptor.Name)
	actionClass := strings.TrimSpace(descriptor.Contract.ActionClass)
	documentType := s.governanceDocumentType(arguments)
	override := cfg.overrideForDomains(descriptor.Contract.BusinessDomains)

	if containsString(cfg.BlockedToolKeys, toolKey) || containsString(override.BlockedToolKeys, toolKey) {
		return blockedGovernanceEvaluation("blocked by MCP governance policy for this tool")
	}
	if containsString(cfg.BlockedActionClasses, actionClass) || containsString(override.BlockedActionClasses, actionClass) {
		return blockedGovernanceEvaluation(fmt.Sprintf("%s actions are blocked by MCP governance policy", firstNonEmpty(actionClass, "this")))
	}
	if documentType != "" && (containsString(cfg.BlockedDocumentTypes, documentType) || containsString(override.BlockedDocumentTypes, documentType)) {
		return blockedGovernanceEvaluation(fmt.Sprintf("document type %s is blocked by MCP governance policy", documentType))
	}

	if strings.TrimSpace(cfg.DefaultActionMode) == "draft_only" {
		switch actionClass {
		case "submit":
			allowlist := cfg.allowedSubmitDocumentTypesForOverride(override)
			if documentType != "" {
				if len(allowlist) > 0 {
					if !containsString(allowlist, documentType) {
						return blockedGovernanceEvaluation(fmt.Sprintf("submit for document type %s is blocked by MCP governance policy", documentType))
					}
					return evaluation
				}
			} else if len(allowlist) > 0 {
				evaluation.PolicyReason = "submit allowlist applies at execution time"
				return evaluation
			}
			return blockedGovernanceEvaluation("submit actions are blocked by default in draft-only MCP governance mode")
		case "controlled_mutation":
			return blockedGovernanceEvaluation("controlled mutation actions are blocked by default in draft-only MCP governance mode")
		}
	}

	if actionClass == "submit" && documentType != "" {
		allowlist := cfg.allowedSubmitDocumentTypesForOverride(override)
		if len(allowlist) > 0 && !containsString(allowlist, documentType) {
			return blockedGovernanceEvaluation(fmt.Sprintf("submit for document type %s is blocked by MCP governance policy", documentType))
		}
	}

	if documentType == "" && (len(cfg.BlockedDocumentTypes) > 0 || len(override.BlockedDocumentTypes) > 0 || len(cfg.AllowedSubmitDocumentTypes) > 0 || len(override.AllowedSubmitDocumentTypes) > 0) {
		if actionClass == "draft" || actionClass == "submit" {
			evaluation.PolicyReason = "document type restrictions apply at execution time"
		}
	}

	return evaluation
}

func (cfg runtimeConfig) overrideForDomains(domains []string) domainGovernanceOverride {
	override := domainGovernanceOverride{}
	for _, current := range domains {
		domain := strings.TrimSpace(current)
		if domain == "" {
			continue
		}
		item, ok := cfg.DomainOverrides[domain]
		if !ok {
			continue
		}
		override.BlockedActionClasses = mergeUniqueStrings(override.BlockedActionClasses, item.BlockedActionClasses)
		override.BlockedToolKeys = mergeUniqueStrings(override.BlockedToolKeys, item.BlockedToolKeys)
		override.BlockedDocumentTypes = mergeUniqueStrings(override.BlockedDocumentTypes, item.BlockedDocumentTypes)
		override.AllowedSubmitDocumentTypes = mergeUniqueStrings(override.AllowedSubmitDocumentTypes, item.AllowedSubmitDocumentTypes)
	}
	return override
}

func (cfg runtimeConfig) allowedSubmitDocumentTypesForOverride(override domainGovernanceOverride) []string {
	return mergeUniqueStrings(cfg.AllowedSubmitDocumentTypes, override.AllowedSubmitDocumentTypes)
}

func (s *Server) governanceDocumentType(arguments map[string]any) string {
	if strings.TrimSpace(stringArg(arguments, "document_type")) != "" {
		return strings.TrimSpace(stringArg(arguments, "document_type"))
	}
	documentID := strings.TrimSpace(stringArg(arguments, "document_id"))
	if documentID == "" || s == nil || s.documents == nil {
		return ""
	}
	record, err := s.documents.Get(documentID)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(record.Header.Type)
}

func defaultGovernanceEvaluation(descriptor ToolDescriptor) governanceEvaluation {
	state := "allowed"
	switch {
	case descriptor.Contract.RequiresApproval:
		state = "approval_required"
	case descriptor.Contract.RequiresConfirmation:
		state = "confirmation_required"
	}
	return governanceEvaluation{
		Allowed:             true,
		PolicyState:         state,
		EffectiveVisibility: "listed",
	}
}

func blockedGovernanceEvaluation(reason string) governanceEvaluation {
	return governanceEvaluation{
		Allowed:             false,
		PolicyState:         "blocked",
		PolicyReason:        strings.TrimSpace(reason),
		EffectiveVisibility: "listed",
	}
}

func parseStringList(value any) []string {
	items := []string{}
	switch current := value.(type) {
	case string:
		trimmed := strings.TrimSpace(current)
		if trimmed == "" {
			return items
		}
		_ = json.Unmarshal([]byte(trimmed), &items)
	case []string:
		items = append(items, current...)
	case []any:
		for _, item := range current {
			text, ok := item.(string)
			if ok {
				items = append(items, text)
			}
		}
	}
	return mergeUniqueStrings(nil, items)
}

func parseDomainOverrides(value any) map[string]domainGovernanceOverride {
	items := map[string]domainGovernanceOverride{}
	switch current := value.(type) {
	case string:
		trimmed := strings.TrimSpace(current)
		if trimmed == "" {
			return items
		}
		_ = json.Unmarshal([]byte(trimmed), &items)
	case map[string]any:
		for key, raw := range current {
			override := domainGovernanceOverride{}
			data, err := json.Marshal(raw)
			if err != nil {
				continue
			}
			if err := json.Unmarshal(data, &override); err != nil {
				continue
			}
			items[strings.TrimSpace(key)] = normalizeDomainOverride(override)
		}
	}
	for key, item := range items {
		items[key] = normalizeDomainOverride(item)
	}
	return items
}

func normalizeDomainOverride(item domainGovernanceOverride) domainGovernanceOverride {
	item.BlockedActionClasses = mergeUniqueStrings(nil, item.BlockedActionClasses)
	item.BlockedToolKeys = mergeUniqueStrings(nil, item.BlockedToolKeys)
	item.BlockedDocumentTypes = mergeUniqueStrings(nil, item.BlockedDocumentTypes)
	item.AllowedSubmitDocumentTypes = mergeUniqueStrings(nil, item.AllowedSubmitDocumentTypes)
	return item
}

func toolUsesGovernancePolicy(descriptor ToolDescriptor) bool {
	name := strings.TrimSpace(descriptor.Name)
	if descriptor.SourceType == "synthetic" {
		return true
	}
	if strings.HasPrefix(name, "business.") {
		return true
	}
	switch {
	case strings.HasPrefix(name, "module."),
		strings.HasPrefix(name, "identity.role_permission."),
		strings.HasPrefix(name, "identity.role_binding."),
		strings.HasPrefix(name, "config.bundle."),
		strings.HasPrefix(name, "feature_flag.value."),
		name == "workflow.draft.publish":
		return true
	default:
		return false
	}
}
