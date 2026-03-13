package policy

import (
	"reflect"
	"strings"
	"testing"

	"orbyte/internal/platform/config"

	"github.com/open-policy-agent/opa/v1/rego"
)

func TestNewServiceAndDefinitions(t *testing.T) {
	svc := NewService()
	if svc == nil || svc.cfg == nil {
		t.Fatal("expected config-backed default service")
	}
	if len(svc.Definitions()) != 0 {
		t.Fatalf("expected empty definitions on new service, got %d", len(svc.Definitions()))
	}
}

func TestRegisterValidationAndHelpers(t *testing.T) {
	svc := NewServiceWithConfig(config.NewService())
	for _, tc := range []struct {
		name string
		def  HookDefinition
	}{
		{name: "missing key", def: HookDefinition{Kind: "search", Target: "document"}},
		{name: "missing kind", def: HookDefinition{Key: "k", Target: "document"}},
		{name: "missing target", def: HookDefinition{Key: "k", Kind: "search"}},
	} {
		if err := svc.Register(tc.def); err == nil {
			t.Fatalf("expected validation error for %s", tc.name)
		}
	}
	if got := normalizedEngine("unknown"); got != EngineGo {
		t.Fatalf("expected unknown engine to normalize to go, got %s", got)
	}
	if got := sanitizeRegoIdent("Documents.Search-Visibility"); got != "documents_search_visibility" {
		t.Fatalf("unexpected sanitized ident %q", got)
	}
	if got := RegoPackageForHook("documents.search.visibility"); got != "clinic.policy.documents.search.visibility" {
		t.Fatalf("unexpected rego package %q", got)
	}
	if got := firstNonEmpty("", " test ", "later"); got != "test" {
		t.Fatalf("unexpected first non-empty %q", got)
	}
	if got := stringValue(10); got != "" {
		t.Fatalf("expected non-string value to render empty, got %q", got)
	}
}

func TestServiceResolvesScopedRuleFromConfig(t *testing.T) {
	cfg := config.NewService()
	svc := NewServiceWithConfig(cfg)
	if err := svc.Register(HookDefinition{
		Key:           "documents.workflow.transition",
		Kind:          "workflow",
		Target:        "document_transition",
		AllowedScopes: []string{"deployment", "location"},
		DefaultRule:   map[string]any{"blocked_actions": []string{}},
	}); err != nil {
		t.Fatalf("register hook failed: %v", err)
	}
	if err := svc.UpsertRule("documents.workflow.transition", "location", "loc_hq", "tester", map[string]any{"blocked_actions": []string{"cancel"}}); err != nil {
		t.Fatalf("upsert rule failed: %v", err)
	}
	rule, ok := svc.ResolveRule("documents.workflow.transition", "", "loc_hq")
	if !ok {
		t.Fatal("expected resolved rule")
	}
	blocked, _ := rule.Value["blocked_actions"].([]string)
	if len(blocked) != 1 || blocked[0] != "cancel" {
		t.Fatalf("expected location-scoped blocked action, got %#v", rule.Value)
	}
}

func TestEvaluateInjectsResolvedRule(t *testing.T) {
	cfg := config.NewService()
	svc := NewServiceWithConfig(cfg)
	if err := svc.Register(HookDefinition{
		Key:           "documents.search.visibility",
		Kind:          "search",
		Target:        "document_search",
		AllowedScopes: []string{"deployment"},
		DefaultRule:   map[string]any{"hidden_statuses": []string{"cancelled"}},
	}); err != nil {
		t.Fatalf("register hook failed: %v", err)
	}
	if err := svc.SetEvaluator("documents.search.visibility", func(req Request) Decision {
		status, _ := req.Inputs["status"].(string)
		hidden, _ := req.Rule["hidden_statuses"].([]string)
		for _, candidate := range hidden {
			if candidate == status {
				return Decision{Allowed: false, Reason: "hidden", Rule: req.Rule}
			}
		}
		return Decision{Allowed: true, Rule: req.Rule}
	}); err != nil {
		t.Fatalf("set evaluator failed: %v", err)
	}
	decision := svc.Evaluate(Request{
		HookKey: "documents.search.visibility",
		Inputs:  map[string]any{"status": "cancelled"},
	})
	if decision.Allowed {
		t.Fatal("expected cancelled document to be denied")
	}
	if decision.Rule == nil {
		t.Fatal("expected decision to include resolved rule")
	}
}

func TestEvaluateRegoHookUsesDefaultModule(t *testing.T) {
	cfg := config.NewService()
	svc := NewServiceWithConfig(cfg)
	if err := svc.Register(HookDefinition{
		Key:               "documents.search.visibility",
		Kind:              "search",
		Target:            "document_search",
		AllowedScopes:     []string{"deployment"},
		DefaultRule:       map[string]any{"hidden_statuses": []string{"cancelled"}},
		Engine:            EngineRego,
		RegoPackage:       "clinic.policy.documents.search.visibility",
		RegoQuery:         "data.orbyte.policy.documents.search.visibility.decision",
		DefaultRegoSource: validSearchVisibilityModule(),
	}); err != nil {
		t.Fatalf("register hook failed: %v", err)
	}
	decision := svc.Evaluate(Request{
		HookKey: "documents.search.visibility",
		Inputs:  map[string]any{"status": "cancelled"},
	})
	if decision.Allowed {
		t.Fatalf("expected rego policy to deny hidden status, got %+v", decision)
	}
	if decision.Code != "status_hidden" {
		t.Fatalf("expected status_hidden code, got %+v", decision)
	}
}

func TestUpsertModuleRejectsInvalidRego(t *testing.T) {
	cfg := config.NewService()
	svc := NewServiceWithConfig(cfg)
	if err := svc.Register(HookDefinition{
		Key:               "integration.submission.preflight",
		Kind:              "integration",
		Target:            "integration_submission",
		AllowedScopes:     []string{"deployment"},
		DefaultRule:       map[string]any{"blocked_operation_types": []string{}},
		Engine:            EngineRego,
		RegoPackage:       "clinic.policy.integration.submission.preflight",
		RegoQuery:         "data.orbyte.policy.integration.submission.preflight.decision",
		DefaultRegoSource: validPreflightModule(),
	}); err != nil {
		t.Fatalf("register hook failed: %v", err)
	}
	if err := svc.UpsertModule("integration.submission.preflight", "deployment", "", "tester", "package bad\n default decision = "); err == nil {
		t.Fatal("expected invalid rego source to be rejected")
	}
}

func TestSetEvaluatorAndUpsertRuleErrors(t *testing.T) {
	svc := NewServiceWithConfig(config.NewService())
	if err := svc.SetEvaluator("missing", func(req Request) Decision { return Decision{Allowed: true} }); err == nil {
		t.Fatal("expected missing hook to fail")
	}
	if err := svc.Register(HookDefinition{Key: "documents.action.render", Kind: "ui", Target: "document_action_render"}); err != nil {
		t.Fatalf("register hook failed: %v", err)
	}
	if err := svc.SetEvaluator("documents.action.render", nil); err == nil {
		t.Fatal("expected nil evaluator to fail")
	}

	plain := NewServiceWithConfig(nil)
	plain.defs["documents.action.render"] = HookDefinition{Key: "documents.action.render", Kind: "ui", Target: "document_action_render"}
	if err := plain.UpsertRule("documents.action.render", "deployment", "", "tester", map[string]any{}); err == nil {
		t.Fatal("expected non-config-backed upsert rule to fail")
	}
}

func TestResolveRuleAndModuleFallbacks(t *testing.T) {
	svc := NewService()
	svc.defs["documents.action.render"] = HookDefinition{
		Key:         "documents.action.render",
		Kind:        "ui",
		Target:      "document_action_render",
		DefaultRule: map[string]any{"primary_actions": []string{"submit"}},
	}
	value, ok := svc.ResolveRule("documents.action.render", "", "")
	if !ok || value.SourceScope != "default" {
		t.Fatalf("expected default rule fallback, got %+v ok=%v", value, ok)
	}
	if _, ok := svc.ResolveRule("missing", "", ""); ok {
		t.Fatal("expected missing rule lookup to fail")
	}
	svc.defs["documents.search.visibility"] = HookDefinition{
		Key:               "documents.search.visibility",
		Kind:              "search",
		Target:            "document_search",
		Engine:            EngineRego,
		DefaultRegoSource: validSearchVisibilityModule(),
	}
	module, ok := svc.ResolveModule("documents.search.visibility", "", "")
	if !ok || strings.TrimSpace(stringValue(module.Value["source"])) == "" {
		t.Fatalf("expected default rego module fallback, got %+v ok=%v", module, ok)
	}
	if _, ok := svc.ResolveModule("documents.action.render", "", ""); ok {
		t.Fatal("expected non-rego hook module lookup to fail")
	}
}

func TestEvaluateRegoFailsClosedOnInvalidResult(t *testing.T) {
	cfg := config.NewService()
	svc := NewServiceWithConfig(cfg)
	if err := svc.Register(HookDefinition{
		Key:           "integration.submission.preflight",
		Kind:          "integration",
		Target:        "integration_submission",
		AllowedScopes: []string{"deployment"},
		DefaultRule:   map[string]any{"blocked_operation_types": []string{}},
		Engine:        EngineRego,
		RegoPackage:   "clinic.policy.integration.submission.preflight",
		RegoQuery:     "data.orbyte.policy.integration.submission.preflight.decision",
	}); err != nil {
		t.Fatalf("register hook failed: %v", err)
	}
	if err := svc.UpsertModule("integration.submission.preflight", "deployment", "", "tester", `package orbyte.policy.integration.submission.preflight

import rego.v1

decision := true`); err != nil {
		t.Fatalf("upsert module failed: %v", err)
	}
	decision := svc.Evaluate(Request{
		HookKey: "integration.submission.preflight",
		Inputs:  map[string]any{"operation_type": "push_invoice"},
	})
	if decision.Allowed || decision.Code != "policy_eval_invalid_result" {
		t.Fatalf("expected invalid result to fail closed, got %+v", decision)
	}
}

func TestRuntimeIncludesCompileStatus(t *testing.T) {
	cfg := config.NewService()
	svc := NewServiceWithConfig(cfg)
	if err := svc.Register(HookDefinition{
		Key:               "integration.submission.preflight",
		Kind:              "integration",
		Target:            "integration_submission",
		AllowedScopes:     []string{"deployment"},
		DefaultRule:       map[string]any{"blocked_operation_types": []string{}},
		Engine:            EngineRego,
		RegoPackage:       "clinic.policy.integration.submission.preflight",
		RegoQuery:         "data.orbyte.policy.integration.submission.preflight.decision",
		DefaultRegoSource: validPreflightModule(),
	}); err != nil {
		t.Fatalf("register hook failed: %v", err)
	}
	runtime, ok := svc.Runtime("integration.submission.preflight", "", "")
	if !ok {
		t.Fatal("expected runtime")
	}
	if !runtime.CompileValid || !runtime.RegoConfigured {
		t.Fatalf("expected runtime compile status, got %+v", runtime)
	}
}

func TestRuntimeVariantsAndRuntimesList(t *testing.T) {
	cfg := config.NewService()
	svc := NewServiceWithConfig(cfg)
	if err := svc.Register(HookDefinition{
		Key:         "documents.action.render",
		Kind:        "ui",
		Target:      "document_action_render",
		DefaultRule: map[string]any{"hidden_actions": []string{}},
	}); err != nil {
		t.Fatalf("register go hook failed: %v", err)
	}
	if err := svc.Register(HookDefinition{
		Key:               "documents.search.visibility",
		Kind:              "search",
		Target:            "document_search",
		Engine:            EngineRego,
		DefaultRule:       map[string]any{"hidden_statuses": []string{}},
		DefaultRegoSource: validSearchVisibilityModule(),
	}); err != nil {
		t.Fatalf("register rego hook failed: %v", err)
	}
	runtime, ok := svc.Runtime("documents.action.render", "", "")
	if !ok || runtime.Engine != EngineGo {
		t.Fatalf("expected go runtime, got %+v ok=%v", runtime, ok)
	}
	svc.defs["integration.submission.preflight"] = HookDefinition{
		Key:         "integration.submission.preflight",
		Kind:        "integration",
		Target:      "integration_submission",
		Engine:      EngineRego,
		DefaultRule: map[string]any{},
	}
	runtime, ok = svc.Runtime("integration.submission.preflight", "", "")
	if !ok || runtime.CompileValid || runtime.CompileError == "" {
		t.Fatalf("expected missing rego source diagnostics, got %+v ok=%v", runtime, ok)
	}
	if _, ok := svc.Runtime("missing", "", ""); ok {
		t.Fatal("expected missing runtime lookup to fail")
	}
	if items := svc.Runtimes("", ""); len(items) != 3 {
		t.Fatalf("expected 3 runtimes, got %d", len(items))
	}
}

func TestValidateConfiguredModulesRejectsBrokenStoredSource(t *testing.T) {
	cfg := config.NewService()
	svc := NewServiceWithConfig(cfg)
	if err := svc.Register(HookDefinition{
		Key:               "integration.submission.preflight",
		Kind:              "integration",
		Target:            "integration_submission",
		AllowedScopes:     []string{"deployment"},
		DefaultRule:       map[string]any{"blocked_operation_types": []string{}},
		Engine:            EngineRego,
		RegoPackage:       "clinic.policy.integration.submission.preflight",
		RegoQuery:         "data.orbyte.policy.integration.submission.preflight.decision",
		DefaultRegoSource: validPreflightModule(),
	}); err != nil {
		t.Fatalf("register hook failed: %v", err)
	}
	if err := cfg.Save(config.Entry{
		Key:       moduleConfigKey("integration.submission.preflight"),
		ModuleKey: "platform.core",
		Category:  "policy",
		Scope:     "deployment",
		Value:     map[string]any{"source": "package broken\n default decision = "},
		UpdatedBy: "tester",
	}); err != nil {
		t.Fatalf("save broken module source failed: %v", err)
	}
	err := svc.ValidateConfiguredModules()
	if err == nil || !strings.Contains(err.Error(), "integration.submission.preflight") {
		t.Fatalf("expected broken configured module to be rejected, got %v", err)
	}
}

func TestValidateConfiguredModulesCoversScopedAndNonConfigPaths(t *testing.T) {
	svc := NewService()
	svc.defs["documents.action.render"] = HookDefinition{Key: "documents.action.render", Kind: "ui", Target: "document_action_render"}
	svc.defs["documents.search.visibility"] = HookDefinition{
		Key:               "documents.search.visibility",
		Kind:              "search",
		Target:            "document_search",
		Engine:            EngineRego,
		RegoPackage:       "clinic.policy.documents.search.visibility",
		RegoQuery:         "data.orbyte.policy.documents.search.visibility.decision",
		DefaultRegoSource: validSearchVisibilityModule(),
	}
	if err := svc.ValidateConfiguredModules(); err != nil {
		t.Fatalf("expected non-config-backed validation to pass, got %v", err)
	}
}

func TestEvaluateErrorBranches(t *testing.T) {
	svc := NewService()
	decision := svc.Evaluate(Request{HookKey: "missing"})
	if decision.Allowed || decision.Code != "policy_hook_not_found" {
		t.Fatalf("expected missing hook denial, got %+v", decision)
	}
	svc.defs["documents.action.render"] = HookDefinition{
		Key:         "documents.action.render",
		Kind:        "ui",
		Target:      "document_action_render",
		DefaultRule: map[string]any{},
	}
	decision = svc.Evaluate(Request{HookKey: "documents.action.render"})
	if decision.Allowed || decision.Code != "policy_evaluator_not_found" {
		t.Fatalf("expected missing evaluator denial, got %+v", decision)
	}
}

func TestDecisionFromRegoResultsRejectsInvalidShapes(t *testing.T) {
	if _, ok := decisionFromRegoResults(nil); ok {
		t.Fatal("expected nil results to fail")
	}
}

func TestRuleFieldsCoverage(t *testing.T) {
	keys := []string{
		"documents.extension.view",
		"documents.extension.write",
		"documents.workflow.transition",
		"documents.search.visibility",
		"documents.numbering.assign",
		"documents.action.render",
		"integration.submission.preflight",
		"unknown",
	}
	for _, key := range keys {
		_ = ruleFields(key)
	}
}

func TestUpsertModuleAndResolveModuleScopes(t *testing.T) {
	cfg := config.NewService()
	svc := NewServiceWithConfig(cfg)
	def := HookDefinition{
		Key:           "documents.search.visibility",
		Kind:          "search",
		Target:        "document_search",
		AllowedScopes: []string{"deployment", "location"},
		DefaultRule:   map[string]any{"hidden_statuses": []string{}},
		Engine:        EngineRego,
	}
	if err := svc.Register(def); err != nil {
		t.Fatalf("register hook failed: %v", err)
	}
	if err := svc.UpsertModule("documents.search.visibility", "location", "loc_hq", "tester", validSearchVisibilityModule()); err != nil {
		t.Fatalf("upsert module failed: %v", err)
	}
	value, ok := svc.ResolveModule("documents.search.visibility", "", "loc_hq")
	if !ok || value.SourceScope != "location" {
		t.Fatalf("expected scoped module resolution, got %+v ok=%v", value, ok)
	}
}

func TestUpsertModuleNonRegoAndNonConfigErrors(t *testing.T) {
	cfg := config.NewService()
	svc := NewServiceWithConfig(cfg)
	if err := svc.Register(HookDefinition{Key: "documents.action.render", Kind: "ui", Target: "document_action_render"}); err != nil {
		t.Fatalf("register hook failed: %v", err)
	}
	if err := svc.UpsertModule("documents.action.render", "deployment", "", "tester", validSearchVisibilityModule()); err == nil {
		t.Fatal("expected non-rego hook upsert module to fail")
	}
	plain := NewService()
	plain.cfg = nil
	plain.defs["documents.search.visibility"] = HookDefinition{
		Key:    "documents.search.visibility",
		Kind:   "search",
		Target: "document_search",
		Engine: EngineRego,
	}
	if err := plain.UpsertModule("documents.search.visibility", "deployment", "", "tester", validSearchVisibilityModule()); err == nil {
		t.Fatal("expected non-config-backed upsert module to fail")
	}
	if err := plain.UpsertModule("missing", "deployment", "", "tester", validSearchVisibilityModule()); err == nil {
		t.Fatal("expected missing hook upsert module to fail")
	}
}

func TestDecisionFromRegoResultsValidOutputAndCloneMap(t *testing.T) {
	decision, ok := decisionFromRegoResults([]rego.Result{{
		Expressions: []*rego.ExpressionValue{{
			Value: map[string]any{
				"allowed": true,
				"code":    "ok",
				"reason":  "fine",
				"output":  map[string]any{"placement": "primary"},
			},
		}},
	}})
	if !ok || !decision.Allowed || decision.Output["placement"] != "primary" {
		t.Fatalf("unexpected decision: %+v", decision)
	}
	source := map[string]any{"a": 1}
	cloned := cloneMap(source)
	cloned["a"] = 2
	if source["a"] != 1 {
		t.Fatal("expected cloneMap to avoid mutating source")
	}
	if cloneMap(nil) == nil {
		t.Fatal("expected nil clone to return empty map")
	}
}

func TestCompiledKeyStable(t *testing.T) {
	if compiledKey("hook", "query", "source") != compiledKey("hook", "query", "source") {
		t.Fatal("expected compiled key to be stable")
	}
	if compiledKey("hook", "query", "source") == compiledKey("hook", "query", "other") {
		t.Fatal("expected different sources to change key")
	}
}

func TestPrepareCachesCompiledModule(t *testing.T) {
	svc := NewService()
	def := HookDefinition{
		Key:       "documents.search.visibility",
		Kind:      "search",
		Target:    "document_search",
		Engine:    EngineRego,
		RegoQuery: "data.orbyte.policy.documents.search.visibility.decision",
	}
	first, err := svc.prepare(def, validSearchVisibilityModule())
	if err != nil {
		t.Fatalf("prepare failed: %v", err)
	}
	second, err := svc.prepare(def, validSearchVisibilityModule())
	if err != nil {
		t.Fatalf("prepare second failed: %v", err)
	}
	if len(svc.prepared) != 1 {
		t.Fatalf("expected cached prepared query, got %d", len(svc.prepared))
	}
	if reflect.TypeOf(first) != reflect.TypeOf(second) {
		t.Fatal("expected same prepared query type")
	}
}

func validSearchVisibilityModule() string {
	return `package orbyte.policy.documents.search.visibility

import rego.v1

default decision := {"allowed": true}

decision := {"allowed": false, "code": "status_hidden", "reason": "document status hidden by policy"} if {
	input.inputs.status != ""
	input.inputs.status in object.get(input.rule, "hidden_statuses", [])
}`
}

func validPreflightModule() string {
	return `package orbyte.policy.integration.submission.preflight

import rego.v1

default decision := {"allowed": true}

decision := {"allowed": false, "code": "operation_blocked", "reason": "integration operation blocked by policy"} if {
	input.inputs.operation_type in object.get(input.rule, "blocked_operation_types", [])
}`
}
