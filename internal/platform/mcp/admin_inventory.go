package mcp

import (
	"encoding/json"
	"sort"
	"strings"
)

type ToolInventoryItem struct {
	Key                  string             `json:"key"`
	Title                string             `json:"title"`
	Description          string             `json:"description,omitempty"`
	ModuleKey            string             `json:"module_key,omitempty"`
	SourceType           string             `json:"source_type,omitempty"`
	BuiltIn              bool               `json:"built_in,omitempty"`
	Generated            bool               `json:"generated,omitempty"`
	EndpointScope        string             `json:"endpoint_scope,omitempty"`
	RequiredPermissions  []string           `json:"required_permissions,omitempty"`
	Operation            string             `json:"operation,omitempty"`
	Enabled              bool               `json:"enabled"`
	ActionClass          string             `json:"action_class,omitempty"`
	RiskClass            string             `json:"risk_class,omitempty"`
	DraftOnly            bool               `json:"draft_only,omitempty"`
	RequiresConfirmation bool               `json:"requires_confirmation,omitempty"`
	RequiresApproval     bool               `json:"requires_approval,omitempty"`
	GovernanceTags       []string           `json:"governance_tags,omitempty"`
	BusinessDomains      []string           `json:"business_domains,omitempty"`
	Contract             ContractDescriptor `json:"contract,omitempty"`
}

type ResourceInventoryItem struct {
	Key                 string             `json:"key"`
	Title               string             `json:"title"`
	Description         string             `json:"description,omitempty"`
	URI                 string             `json:"uri"`
	ModuleKey           string             `json:"module_key,omitempty"`
	EndpointScope       string             `json:"endpoint_scope,omitempty"`
	RequiredPermissions []string           `json:"required_permissions,omitempty"`
	Contract            ContractDescriptor `json:"contract,omitempty"`
}

type AppInventoryItem struct {
	Key                 string             `json:"key"`
	Title               string             `json:"title"`
	Description         string             `json:"description,omitempty"`
	ResourceKey         string             `json:"resource_key,omitempty"`
	ViewKey             string             `json:"view_key,omitempty"`
	CustomEntryKey      string             `json:"custom_entry_key,omitempty"`
	ModuleKey           string             `json:"module_key,omitempty"`
	EndpointScope       string             `json:"endpoint_scope,omitempty"`
	RequiredPermissions []string           `json:"required_permissions,omitempty"`
	Contract            ContractDescriptor `json:"contract,omitempty"`
}

type runtimeConfig struct {
	Enabled    bool
	ToolStates map[string]bool
}

func (s *Server) MCPEnabled() bool {
	return s.mcpRuntimeConfig().Enabled
}

func (s *Server) ToolEnabled(key string) bool {
	cfg := s.mcpRuntimeConfig()
	if !cfg.Enabled {
		return false
	}
	enabled, ok := cfg.ToolStates[strings.TrimSpace(key)]
	if !ok {
		return true
	}
	return enabled
}

func (s *Server) ToolInventory() []ToolInventoryItem {
	items := make([]ToolInventoryItem, 0)
	for _, reg := range s.mustBuiltInToolRegistrations() {
		def := reg.definition
		contract := builtInToolContract(def.name, def.permission, def.contract)
		items = append(items, ToolInventoryItem{
			Key:                  def.name,
			Title:                def.title,
			Description:          def.description,
			ModuleKey:            "platform.core",
			SourceType:           "built_in",
			BuiltIn:              true,
			EndpointScope:        builtInToolScope(def.name),
			RequiredPermissions:  []string{def.permission},
			Enabled:              s.ToolEnabled(def.name),
			ActionClass:          contract.ActionClass,
			RiskClass:            contract.RiskClass,
			DraftOnly:            contract.DraftOnly,
			RequiresConfirmation: contract.RequiresConfirmation,
			RequiresApproval:     contract.RequiresApproval,
			GovernanceTags:       append([]string(nil), contract.GovernanceTags...),
			BusinessDomains:      append([]string(nil), contract.BusinessDomains...),
			Contract:             contract,
		})
	}
	for _, def := range s.syntheticToolDefinitions(ActorContext{}) {
		contract := builtInToolContract(def.Name, firstString(def.RequiredPermissions), ContractDescriptor{
			RequiredPermissions: append([]string(nil), def.RequiredPermissions...),
		})
		items = append(items, ToolInventoryItem{
			Key:                  def.Name,
			Title:                def.Title,
			Description:          def.Description,
			ModuleKey:            def.ModuleKey,
			SourceType:           "synthetic",
			Generated:            true,
			EndpointScope:        scopeForModule(def.ModuleKey),
			RequiredPermissions:  append([]string(nil), def.RequiredPermissions...),
			Enabled:              s.ToolEnabled(def.Name),
			ActionClass:          contract.ActionClass,
			RiskClass:            contract.RiskClass,
			DraftOnly:            contract.DraftOnly,
			RequiresConfirmation: contract.RequiresConfirmation,
			RequiresApproval:     contract.RequiresApproval,
			GovernanceTags:       append([]string(nil), contract.GovernanceTags...),
			BusinessDomains:      append([]string(nil), contract.BusinessDomains...),
			Contract:             contract,
		})
	}
	if s != nil && s.modules != nil {
		for _, detail := range s.modules.List() {
			for _, def := range detail.Manifest.MCP.Tools {
				contract := contractDescriptorFromModule(
					def.Contract,
					def.RequiredPermissions,
					defaultToolSideEffectClass(def.Key, def.Operation),
					defaultToolIdempotency(def.Key, def.Operation),
					"mcp.tool."+strings.TrimSpace(def.Key),
				)
				items = append(items, ToolInventoryItem{
					Key:                  def.Key,
					Title:                def.Title,
					Description:          def.Description,
					ModuleKey:            detail.Manifest.Key,
					SourceType:           "module",
					EndpointScope:        scopeForModule(detail.Manifest.Key),
					RequiredPermissions:  append([]string(nil), def.RequiredPermissions...),
					Operation:            def.Operation,
					Enabled:              s.ToolEnabled(def.Key),
					ActionClass:          contract.ActionClass,
					RiskClass:            contract.RiskClass,
					DraftOnly:            contract.DraftOnly,
					RequiresConfirmation: contract.RequiresConfirmation,
					RequiresApproval:     contract.RequiresApproval,
					GovernanceTags:       append([]string(nil), contract.GovernanceTags...),
					BusinessDomains:      append([]string(nil), contract.BusinessDomains...),
					Contract:             contract,
				})
			}
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Key < items[j].Key })
	return items
}

func (s *Server) ResourceInventory() []ResourceInventoryItem {
	items := make([]ResourceInventoryItem, 0)
	if s == nil || s.modules == nil {
		return items
	}
	for _, detail := range s.modules.List() {
		for _, def := range detail.Manifest.MCP.Resources {
			items = append(items, ResourceInventoryItem{
				Key:                 def.Key,
				Title:               def.Title,
				Description:         def.Description,
				URI:                 def.URI,
				ModuleKey:           detail.Manifest.Key,
				EndpointScope:       scopeForModule(detail.Manifest.Key),
				RequiredPermissions: append([]string(nil), def.RequiredPermissions...),
				Contract: contractDescriptorFromModule(
					def.Contract,
					def.RequiredPermissions,
					"read",
					"read-only",
					"mcp.resource."+strings.TrimSpace(def.Key),
				),
			})
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Key < items[j].Key })
	return items
}

func (s *Server) AppInventory() []AppInventoryItem {
	items := make([]AppInventoryItem, 0)
	if s == nil || s.modules == nil {
		return items
	}
	for _, detail := range s.modules.List() {
		for _, def := range detail.Manifest.MCP.Apps {
			items = append(items, AppInventoryItem{
				Key:                 def.Key,
				Title:               def.Title,
				Description:         def.Description,
				ResourceKey:         def.ResourceKey,
				ViewKey:             def.ViewKey,
				CustomEntryKey:      def.CustomEntryKey,
				ModuleKey:           detail.Manifest.Key,
				EndpointScope:       scopeForModule(detail.Manifest.Key),
				RequiredPermissions: append([]string(nil), def.RequiredPermissions...),
				Contract: contractDescriptorFromModule(
					def.Contract,
					def.RequiredPermissions,
					"read",
					"read-only",
					"mcp.app."+strings.TrimSpace(def.Key),
				),
			})
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Key < items[j].Key })
	return items
}

func (s *Server) toolDefined(name string) bool {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return false
	}
	if _, ok := s.mustBuiltInToolRegistrationIndex()[trimmed]; ok {
		return true
	}
	if _, ok := s.syntheticToolDefinition(trimmed, ActorContext{}); ok {
		return true
	}
	if s == nil || s.modules == nil {
		return false
	}
	for _, detail := range s.modules.List() {
		for _, def := range detail.Manifest.MCP.Tools {
			if strings.TrimSpace(def.Key) == trimmed {
				return true
			}
		}
	}
	return false
}

func (s *Server) mcpRuntimeConfig() runtimeConfig {
	cfg := runtimeConfig{Enabled: true, ToolStates: map[string]bool{}}
	if s == nil || s.config == nil {
		return cfg
	}
	value, ok := s.config.Resolve("platform.mcp", "", "")
	if !ok {
		return cfg
	}
	rawEnabled, ok := value.Value["enabled"].(bool)
	if ok {
		cfg.Enabled = rawEnabled
	}
	switch raw := value.Value["tool_states_json"].(type) {
	case string:
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			return cfg
		}
		_ = json.Unmarshal([]byte(trimmed), &cfg.ToolStates)
	case map[string]any:
		for key, item := range raw {
			flag, ok := item.(bool)
			if !ok {
				continue
			}
			cfg.ToolStates[strings.TrimSpace(key)] = flag
		}
	}
	return cfg
}
