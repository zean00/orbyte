package policy

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"orbyte/internal/platform/config"
	"orbyte/internal/platform/shared"

	"github.com/open-policy-agent/opa/v1/rego"
)

const (
	EngineGo   = "go"
	EngineRego = "rego"
)

type Request struct {
	HookKey        string         `json:"hook_key"`
	ActorID        string         `json:"actor_id,omitempty"`
	OrganizationID string         `json:"organization_id,omitempty"`
	LocationID     string         `json:"location_id,omitempty"`
	ScopeID        string         `json:"scope_id,omitempty"`
	Inputs         map[string]any `json:"inputs,omitempty"`
	Rule           map[string]any `json:"rule,omitempty"`
}

type Decision struct {
	Allowed bool           `json:"allowed"`
	Code    string         `json:"code,omitempty"`
	Reason  string         `json:"reason,omitempty"`
	Output  map[string]any `json:"output,omitempty"`
	Rule    map[string]any `json:"rule,omitempty"`
	Trace   map[string]any `json:"trace,omitempty"`
}

type Evaluator func(Request) Decision

type HookDefinition struct {
	Key               string         `json:"key"`
	Kind              string         `json:"kind"`
	Target            string         `json:"target"`
	InputContractKey  string         `json:"input_contract_key,omitempty"`
	OutputContractKey string         `json:"output_contract_key,omitempty"`
	Description       string         `json:"description,omitempty"`
	RuleSchemaKey     string         `json:"rule_schema_key,omitempty"`
	AllowedScopes     []string       `json:"allowed_scopes,omitempty"`
	DefaultRule       map[string]any `json:"default_rule,omitempty"`
	Engine            string         `json:"engine,omitempty"`
	RegoPackage       string         `json:"rego_package,omitempty"`
	RegoQuery         string         `json:"rego_query,omitempty"`
	DefaultRegoSource string         `json:"default_rego_source,omitempty"`
}

type HookRuntime struct {
	Definition     HookDefinition           `json:"definition"`
	Rule           config.EffectiveValue    `json:"rule"`
	RuleFields     []config.FieldDefinition `json:"rule_fields,omitempty"`
	Engine         string                   `json:"engine"`
	RegoPackage    string                   `json:"rego_package,omitempty"`
	RegoQuery      string                   `json:"rego_query,omitempty"`
	RegoConfigured bool                     `json:"rego_configured"`
	RegoSource     string                   `json:"rego_source,omitempty"`
	CompileValid   bool                     `json:"compile_valid"`
	CompileError   string                   `json:"compile_error,omitempty"`
	EvalValid      bool                     `json:"eval_valid"`
	EvalError      string                   `json:"eval_error,omitempty"`
}

type Service struct {
	cfg        *config.Service
	defs       map[string]HookDefinition
	evaluators map[string]Evaluator

	mu       sync.RWMutex
	prepared map[string]rego.PreparedEvalQuery
}

func NewService() *Service {
	return NewServiceWithConfig(config.NewService())
}

func NewServiceWithConfig(cfg *config.Service) *Service {
	return &Service{
		cfg:        cfg,
		defs:       map[string]HookDefinition{},
		evaluators: map[string]Evaluator{},
		prepared:   map[string]rego.PreparedEvalQuery{},
	}
}

func (s *Service) Register(def HookDefinition) error {
	if strings.TrimSpace(def.Key) == "" {
		return shared.Validation("policy hook key is required")
	}
	if strings.TrimSpace(def.Kind) == "" {
		return shared.Validation("policy hook kind is required")
	}
	if strings.TrimSpace(def.Target) == "" {
		return shared.Validation("policy hook target is required")
	}
	if def.RuleSchemaKey == "" {
		def.RuleSchemaKey = def.Key
	}
	if len(def.AllowedScopes) == 0 {
		def.AllowedScopes = []string{"deployment"}
	}
	if def.DefaultRule == nil {
		def.DefaultRule = map[string]any{}
	}
	def.Engine = normalizedEngine(def.Engine)
	if def.Engine == EngineRego {
		if strings.TrimSpace(def.RegoPackage) == "" {
			def.RegoPackage = regoPackageForHook(def.Key)
		}
		if strings.TrimSpace(def.RegoQuery) == "" {
			def.RegoQuery = "data." + def.RegoPackage + ".decision"
		}
	}
	s.defs[def.Key] = def
	s.registerRuleDefinition(def)
	if def.Engine == EngineRego {
		s.registerModuleDefinition(def)
	}
	return nil
}

func (s *Service) Definitions() []HookDefinition {
	items := make([]HookDefinition, 0, len(s.defs))
	for _, def := range s.defs {
		items = append(items, def)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Key < items[j].Key })
	return items
}

func (s *Service) SetEvaluator(hookKey string, eval Evaluator) error {
	if _, ok := s.defs[hookKey]; !ok {
		return shared.NotFound("policy hook not found")
	}
	if eval == nil {
		return shared.Validation("policy evaluator is required")
	}
	s.evaluators[hookKey] = eval
	return nil
}

func (s *Service) UpsertRule(hookKey, scope, scopeID, actorID string, value map[string]any) error {
	def, ok := s.defs[hookKey]
	if !ok {
		return shared.NotFound("policy hook not found")
	}
	if s.cfg == nil {
		return shared.Conflict("policy service is not config-backed")
	}
	if scope == "" {
		scope = "deployment"
	}
	return s.cfg.Save(config.Entry{
		Key:         ruleConfigKey(hookKey),
		ModuleKey:   "platform.core",
		Category:    "policy",
		Scope:       scope,
		ScopeID:     scopeID,
		Value:       cloneMap(value),
		UpdatedAt:   time.Now().UTC(),
		UpdatedBy:   firstNonEmpty(actorID, "system"),
		Description: def.Description,
	})
}

func (s *Service) ResolveRule(hookKey, organizationID, locationID string) (config.EffectiveValue, bool) {
	def, ok := s.defs[hookKey]
	if !ok {
		return config.EffectiveValue{}, false
	}
	if s.cfg == nil {
		return config.EffectiveValue{
			Key:         ruleConfigKey(hookKey),
			ModuleKey:   "platform.core",
			Scope:       "effective",
			Value:       cloneMap(def.DefaultRule),
			SourceScope: "default",
			ResolvedAt:  time.Now().UTC(),
		}, true
	}
	value, ok := s.cfg.Resolve(ruleConfigKey(hookKey), organizationID, locationID)
	if !ok {
		return config.EffectiveValue{
			Key:         ruleConfigKey(hookKey),
			ModuleKey:   "platform.core",
			Scope:       "effective",
			Value:       cloneMap(def.DefaultRule),
			SourceScope: "default",
			ResolvedAt:  time.Now().UTC(),
		}, true
	}
	return value, true
}

func (s *Service) UpsertModule(hookKey, scope, scopeID, actorID, source string) error {
	def, ok := s.defs[hookKey]
	if !ok {
		return shared.NotFound("policy hook not found")
	}
	if def.Engine != EngineRego {
		return shared.Validation("policy hook is not Rego-backed")
	}
	if s.cfg == nil {
		return shared.Conflict("policy service is not config-backed")
	}
	if strings.TrimSpace(source) == "" {
		return shared.Validation("rego source is required")
	}
	if _, err := s.prepare(def, source); err != nil {
		return shared.Validation(fmt.Sprintf("invalid rego source: %v", err))
	}
	if scope == "" {
		scope = "deployment"
	}
	return s.cfg.Save(config.Entry{
		Key:         moduleConfigKey(hookKey),
		ModuleKey:   "platform.core",
		Category:    "policy",
		Scope:       scope,
		ScopeID:     scopeID,
		Value:       map[string]any{"source": source},
		UpdatedAt:   time.Now().UTC(),
		UpdatedBy:   firstNonEmpty(actorID, "system"),
		Description: "Rego policy source for " + hookKey,
	})
}

func (s *Service) ResolveModule(hookKey, organizationID, locationID string) (config.EffectiveValue, bool) {
	def, ok := s.defs[hookKey]
	if !ok || def.Engine != EngineRego {
		return config.EffectiveValue{}, false
	}
	if s.cfg != nil {
		if value, ok := s.cfg.Resolve(moduleConfigKey(hookKey), organizationID, locationID); ok {
			source := strings.TrimSpace(stringValue(value.Value["source"]))
			if source != "" {
				return value, true
			}
		}
	}
	if strings.TrimSpace(def.DefaultRegoSource) == "" {
		return config.EffectiveValue{}, false
	}
	return config.EffectiveValue{
		Key:         moduleConfigKey(hookKey),
		ModuleKey:   "platform.core",
		Scope:       "effective",
		Value:       map[string]any{"source": def.DefaultRegoSource},
		SourceScope: "default",
		ResolvedAt:  time.Now().UTC(),
	}, true
}

func (s *Service) Runtime(hookKey, organizationID, locationID string) (HookRuntime, bool) {
	def, ok := s.defs[hookKey]
	if !ok {
		return HookRuntime{}, false
	}
	rule, _ := s.ResolveRule(hookKey, organizationID, locationID)
	runtime := HookRuntime{
		Definition: def,
		Rule:       rule,
		RuleFields: ruleFields(def.RuleSchemaKey),
		Engine:     def.Engine,
	}
	if def.Engine != EngineRego {
		if _, ok := s.evaluators[def.Key]; ok {
			runtime.EvalValid = true
		} else {
			runtime.EvalError = "policy evaluator is not configured"
		}
		return runtime, true
	}
	runtime.RegoPackage = def.RegoPackage
	runtime.RegoQuery = def.RegoQuery
	moduleValue, ok := s.ResolveModule(hookKey, organizationID, locationID)
	if !ok {
		runtime.CompileValid = false
		runtime.CompileError = "rego source is not configured"
		return runtime, true
	}
	runtime.RegoConfigured = true
	runtime.RegoSource = strings.TrimSpace(stringValue(moduleValue.Value["source"]))
	prepared, err := s.prepare(def, runtime.RegoSource)
	if err != nil {
		runtime.CompileValid = false
		runtime.CompileError = err.Error()
		return runtime, true
	}
	runtime.CompileValid = true
	if err := validatePreparedDecision(prepared, Request{
		HookKey:        def.Key,
		OrganizationID: organizationID,
		LocationID:     locationID,
	}); err != nil {
		runtime.EvalError = err.Error()
		return runtime, true
	}
	runtime.EvalValid = true
	return runtime, true
}

func (s *Service) Runtimes(organizationID, locationID string) []HookRuntime {
	defs := s.Definitions()
	items := make([]HookRuntime, 0, len(defs))
	for _, def := range defs {
		runtime, _ := s.Runtime(def.Key, organizationID, locationID)
		items = append(items, runtime)
	}
	return items
}

func (s *Service) ValidateConfiguredModules() error {
	for _, def := range s.Definitions() {
		if def.Engine != EngineRego {
			continue
		}
		if value, ok := s.ResolveModule(def.Key, "", ""); ok {
			if err := s.validateRegoSource(def, strings.TrimSpace(stringValue(value.Value["source"]))); err != nil {
				return fmt.Errorf("policy hook %s: %w", def.Key, err)
			}
		}
		if s.cfg == nil {
			continue
		}
		for _, entry := range s.cfg.Entries() {
			if entry.Key != moduleConfigKey(def.Key) {
				continue
			}
			source := strings.TrimSpace(stringValue(entry.Value["source"]))
			if source == "" {
				continue
			}
			if err := s.validateRegoSource(def, source); err != nil {
				return fmt.Errorf("policy hook %s scope %s/%s: %w", def.Key, entry.Scope, entry.ScopeID, err)
			}
		}
	}
	return nil
}

func (s *Service) Evaluate(req Request) Decision {
	def, ok := s.defs[req.HookKey]
	if !ok {
		return Decision{Allowed: false, Code: "policy_hook_not_found", Reason: "policy hook not found"}
	}
	rule, ok := s.ResolveRule(req.HookKey, req.OrganizationID, req.LocationID)
	if ok {
		req.Rule = cloneMap(rule.Value)
	} else if req.Rule == nil {
		req.Rule = cloneMap(def.DefaultRule)
	}
	if req.Inputs == nil {
		req.Inputs = map[string]any{}
	}
	if def.Engine == EngineRego {
		decision := s.evaluateRego(def, req)
		if decision.Trace == nil {
			decision.Trace = map[string]any{}
		}
		decision.Trace["hook_key"] = req.HookKey
		decision.Trace["engine"] = def.Engine
		return decision
	}
	eval, ok := s.evaluators[req.HookKey]
	if !ok {
		return Decision{Allowed: false, Code: "policy_evaluator_not_found", Reason: "policy evaluator not configured", Rule: req.Rule, Trace: map[string]any{"hook_key": req.HookKey, "engine": def.Engine}}
	}
	decision := eval(req)
	if decision.Rule == nil {
		decision.Rule = req.Rule
	}
	if decision.Trace == nil {
		decision.Trace = map[string]any{}
	}
	decision.Trace["hook_key"] = req.HookKey
	decision.Trace["engine"] = def.Engine
	return decision
}

func (s *Service) evaluateRego(def HookDefinition, req Request) Decision {
	moduleValue, ok := s.ResolveModule(def.Key, req.OrganizationID, req.LocationID)
	if !ok {
		return Decision{Allowed: false, Code: "policy_eval_missing_module", Reason: "rego source is not configured", Rule: req.Rule}
	}
	source := strings.TrimSpace(stringValue(moduleValue.Value["source"]))
	prepared, err := s.prepare(def, source)
	if err != nil {
		return Decision{Allowed: false, Code: "policy_eval_compile_error", Reason: err.Error(), Rule: req.Rule}
	}
	results, err := prepared.Eval(context.Background(), rego.EvalInput(map[string]any{
		"hook_key":        req.HookKey,
		"actor_id":        req.ActorID,
		"organization_id": req.OrganizationID,
		"location_id":     req.LocationID,
		"scope_id":        req.ScopeID,
		"inputs":          cloneMap(req.Inputs),
		"rule":            cloneMap(req.Rule),
	}))
	if err != nil {
		return Decision{Allowed: false, Code: "policy_eval_error", Reason: err.Error(), Rule: req.Rule}
	}
	decision, ok := decisionFromRegoResults(results)
	if !ok {
		return Decision{Allowed: false, Code: "policy_eval_invalid_result", Reason: "rego policy must return a decision object", Rule: req.Rule}
	}
	if decision.Rule == nil {
		decision.Rule = req.Rule
	}
	if decision.Trace == nil {
		decision.Trace = map[string]any{}
	}
	decision.Trace["hook_key"] = def.Key
	decision.Trace["engine"] = def.Engine
	return decision
}

func (s *Service) validateRegoSource(def HookDefinition, source string) error {
	prepared, err := s.prepare(def, source)
	if err != nil {
		return err
	}
	return validatePreparedDecision(prepared, Request{HookKey: def.Key})
}

func (s *Service) prepare(def HookDefinition, source string) (rego.PreparedEvalQuery, error) {
	cacheKey := compiledKey(def.Key, def.RegoQuery, source)
	s.mu.RLock()
	if prepared, ok := s.prepared[cacheKey]; ok {
		s.mu.RUnlock()
		return prepared, nil
	}
	s.mu.RUnlock()
	query, err := rego.New(
		rego.Query(def.RegoQuery),
		rego.Module(def.Key+".rego", source),
	).PrepareForEval(context.Background())
	if err != nil {
		return rego.PreparedEvalQuery{}, err
	}
	s.mu.Lock()
	s.prepared[cacheKey] = query
	s.mu.Unlock()
	return query, nil
}

func (s *Service) registerRuleDefinition(def HookDefinition) {
	if s.cfg == nil {
		return
	}
	_ = s.cfg.RegisterDefinition(config.Definition{
		Key:           ruleConfigKey(def.Key),
		ModuleKey:     "platform.core",
		Category:      "policy",
		DisplayName:   def.Key,
		Description:   def.Description,
		AllowedScopes: append([]string(nil), def.AllowedScopes...),
		DefaultValue:  cloneMap(def.DefaultRule),
		Fields:        ruleFields(def.RuleSchemaKey),
	})
}

func (s *Service) registerModuleDefinition(def HookDefinition) {
	if s.cfg == nil {
		return
	}
	_ = s.cfg.RegisterDefinition(config.Definition{
		Key:           moduleConfigKey(def.Key),
		ModuleKey:     "platform.core",
		Category:      "policy",
		DisplayName:   def.Key + " Rego",
		Description:   "Rego policy source for " + def.Key,
		AllowedScopes: append([]string(nil), def.AllowedScopes...),
		DefaultValue:  map[string]any{"source": def.DefaultRegoSource},
		Fields: []config.FieldDefinition{{
			Key: "source", Label: "Source", Type: "string", Required: true, Description: "Embedded Rego module source.",
		}},
	})
}

func ruleConfigKey(hookKey string) string {
	return "policy." + hookKey
}

func moduleConfigKey(hookKey string) string {
	return "policy.rego." + hookKey
}

func normalizedEngine(engine string) string {
	switch strings.ToLower(strings.TrimSpace(engine)) {
	case "", EngineGo:
		return EngineGo
	case EngineRego:
		return EngineRego
	default:
		return EngineGo
	}
}

func regoPackageForHook(hookKey string) string {
	parts := strings.Split(hookKey, ".")
	items := make([]string, 0, len(parts)+2)
	items = append(items, "orbyte", "policy")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		items = append(items, sanitizeRegoIdent(part))
	}
	return strings.Join(items, ".")
}

func RegoPackageForHook(hookKey string) string {
	return regoPackageForHook(hookKey)
}

func sanitizeRegoIdent(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "policy"
	}
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r + ('a' - 'A'))
		case r >= '0' && r <= '9':
			if b.Len() == 0 {
				b.WriteByte('_')
			}
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return "policy"
	}
	return out
}

func compiledKey(hookKey, query, source string) string {
	sum := sha256.Sum256([]byte(source))
	return hookKey + "::" + query + "::" + hex.EncodeToString(sum[:])
}

func validatePreparedDecision(prepared rego.PreparedEvalQuery, req Request) error {
	if req.Inputs == nil {
		req.Inputs = map[string]any{}
	}
	if req.Rule == nil {
		req.Rule = map[string]any{}
	}
	results, err := prepared.Eval(context.Background(), rego.EvalInput(map[string]any{
		"hook_key":        req.HookKey,
		"actor_id":        req.ActorID,
		"organization_id": req.OrganizationID,
		"location_id":     req.LocationID,
		"scope_id":        req.ScopeID,
		"inputs":          cloneMap(req.Inputs),
		"rule":            cloneMap(req.Rule),
	}))
	if err != nil {
		return err
	}
	if _, ok := decisionFromRegoResults(results); !ok {
		return fmt.Errorf("rego policy must return a decision object")
	}
	return nil
}

func decisionFromRegoResults(results []rego.Result) (Decision, bool) {
	if len(results) == 0 || len(results[0].Expressions) == 0 {
		return Decision{}, false
	}
	value, ok := results[0].Expressions[0].Value.(map[string]any)
	if !ok {
		return Decision{}, false
	}
	allowed, ok := value["allowed"].(bool)
	if !ok {
		return Decision{}, false
	}
	decision := Decision{
		Allowed: allowed,
		Code:    strings.TrimSpace(stringValue(value["code"])),
		Reason:  strings.TrimSpace(stringValue(value["reason"])),
	}
	if output, ok := value["output"].(map[string]any); ok {
		decision.Output = cloneMap(output)
	}
	if trace, ok := value["trace"].(map[string]any); ok {
		decision.Trace = cloneMap(trace)
	}
	return decision, true
}

func ruleFields(schemaKey string) []config.FieldDefinition {
	switch schemaKey {
	case "documents.extension.view", "documents.extension.write":
		return []config.FieldDefinition{
			{Key: "allowed_modules", Label: "Allowed Modules", Type: "string_list"},
			{Key: "denied_statuses", Label: "Denied Statuses", Type: "string_list"},
			{Key: "required_permissions", Label: "Required Permissions", Type: "string_list"},
		}
	case "documents.workflow.transition":
		return []config.FieldDefinition{
			{Key: "blocked_actions", Label: "Blocked Actions", Type: "string_list"},
			{Key: "allowed_actions", Label: "Allowed Actions", Type: "string_list"},
			{Key: "allowed_statuses", Label: "Allowed Statuses", Type: "string_list"},
			{Key: "minimum_amount_minor", Label: "Minimum Amount Minor", Type: "int"},
			{Key: "require_number", Label: "Require Number", Type: "bool"},
		}
	case "documents.workflow.assignment":
		return []config.FieldDefinition{
			{Key: "assignee_role_key", Label: "Assignee Role Key", Type: "string"},
			{Key: "candidate_role_keys", Label: "Candidate Role Keys", Type: "string_list"},
			{Key: "assignment_mode", Label: "Assignment Mode", Type: "string"},
			{Key: "assignee_user_id", Label: "Assignee User ID", Type: "string"},
		}
	case "documents.workflow.sla":
		return []config.FieldDefinition{
			{Key: "due_after_seconds", Label: "Due After Seconds", Type: "int"},
			{Key: "escalate_after_seconds", Label: "Escalate After Seconds", Type: "int"},
		}
	case "documents.search.visibility":
		return []config.FieldDefinition{
			{Key: "hidden_statuses", Label: "Hidden Statuses", Type: "string_list"},
			{Key: "allowed_types", Label: "Allowed Types", Type: "string_list"},
			{Key: "location_allowlist", Label: "Location Allowlist", Type: "string_list"},
		}
	case "documents.numbering.assign":
		return []config.FieldDefinition{
			{Key: "prefix", Label: "Prefix", Type: "string"},
			{Key: "include_location", Label: "Include Location", Type: "bool"},
			{Key: "include_date", Label: "Include Date", Type: "bool"},
			{Key: "sequence_padding", Label: "Sequence Padding", Type: "int"},
		}
	case "documents.action.render":
		return []config.FieldDefinition{
			{Key: "hidden_actions", Label: "Hidden Actions", Type: "string_list"},
			{Key: "primary_actions", Label: "Primary Actions", Type: "string_list"},
		}
	case "integration.submission.preflight":
		return []config.FieldDefinition{
			{Key: "blocked_operation_types", Label: "Blocked Operation Types", Type: "string_list"},
			{Key: "required_system_status", Label: "Required System Status", Type: "string"},
		}
	default:
		return nil
	}
}

func cloneMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return map[string]any{}
	}
	output := make(map[string]any, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func stringValue(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
