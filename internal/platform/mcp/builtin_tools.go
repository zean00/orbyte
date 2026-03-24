package mcp

import "fmt"

type builtInTool struct {
	name        string
	title       string
	description string
	permission  string
	inputSchema map[string]any
	contract    ContractDescriptor
}

type builtInToolHandler func(*Server, ActorContext, map[string]any) (map[string]any, bool, error)

type builtInToolRegistration struct {
	definition builtInTool
	handler    builtInToolHandler
}

func (s *Server) listBuiltInTools(actor ActorContext) []ToolDescriptor {
	registry := s.mustBuiltInToolRegistrations()
	items := make([]ToolDescriptor, 0, len(registry))
	for _, reg := range registry {
		def := reg.definition
		if !scopeMatches(actor.EndpointScope, builtInToolScope(def.name)) {
			continue
		}
		if !allowsAll(actor.PermissionChecker, []string{def.permission}) {
			continue
		}
		items = append(items, ToolDescriptor{
			Name:        def.name,
			Title:       def.title,
			Description: def.description,
			Scope:       builtInToolScope(def.name),
			InputSchema: cloneMap(def.inputSchema),
			Contract:    builtInToolContract(def.name, def.permission, def.contract),
		})
	}
	return items
}

func (s *Server) callBuiltInTool(actor ActorContext, name string, arguments map[string]any) (map[string]any, bool, error) {
	registry := s.mustBuiltInToolRegistrationIndex()
	reg, ok := registry[name]
	if !ok {
		return nil, false, nil
	}
	if !scopeMatches(actor.EndpointScope, builtInToolScope(name)) && builtInToolScope(name) != "" {
		return nil, true, fmt.Errorf("tool is not available on this endpoint")
	}
	if !allowsAll(actor.PermissionChecker, []string{reg.definition.permission}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	return reg.handler(s, actor, arguments)
}

func (s *Server) mustBuiltInToolRegistrations() []builtInToolRegistration {
	if s == nil {
		return nil
	}
	if len(s.builtInToolRegistrations) == 0 {
		s.mustInitBuiltInTools()
	}
	return append([]builtInToolRegistration(nil), s.builtInToolRegistrations...)
}

func (s *Server) mustBuiltInToolRegistrationIndex() map[string]builtInToolRegistration {
	if s == nil {
		return nil
	}
	if len(s.builtInToolIndex) == 0 {
		s.mustInitBuiltInTools()
	}
	index := make(map[string]builtInToolRegistration, len(s.builtInToolIndex))
	for key, reg := range s.builtInToolIndex {
		index[key] = reg
	}
	return index
}

func (s *Server) mustInitBuiltInTools() {
	if s == nil || len(s.builtInToolRegistrations) != 0 {
		return
	}
	registry := make([]builtInToolRegistration, 0)
	registry = s.appendBuiltInCoreToolRegistrations(registry)
	registry = s.appendBuiltInOpsToolRegistrations(registry)
	index := make(map[string]builtInToolRegistration, len(registry))
	for _, reg := range registry {
		index[reg.definition.name] = reg
	}
	s.builtInToolRegistrations = registry
	s.builtInToolIndex = index
}

func buildBuiltInToolRegistrations(defs []builtInTool, handlers map[string]builtInToolHandler) ([]builtInToolRegistration, error) {
	registry := make([]builtInToolRegistration, 0, len(defs))
	seen := make(map[string]struct{}, len(defs))
	for _, def := range defs {
		if def.name == "" {
			return nil, fmt.Errorf("built-in tool definition is missing a name")
		}
		if _, ok := seen[def.name]; ok {
			return nil, fmt.Errorf("duplicate built-in tool definition: %s", def.name)
		}
		seen[def.name] = struct{}{}
		if def.permission == "" {
			return nil, fmt.Errorf("built-in tool %s is missing a permission", def.name)
		}
		handler, ok := handlers[def.name]
		if !ok || handler == nil {
			return nil, fmt.Errorf("built-in tool %s is missing a handler", def.name)
		}
		registry = append(registry, builtInToolRegistration{definition: def, handler: handler})
	}
	return registry, nil
}

func mustBuiltInToolRegistration(handler builtInToolHandler, def builtInTool) builtInToolRegistration {
	items, err := buildBuiltInToolRegistrations([]builtInTool{def}, map[string]builtInToolHandler{def.name: handler})
	if err != nil {
		panic(err)
	}
	return items[0]
}
