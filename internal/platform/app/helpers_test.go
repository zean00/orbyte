package app

import (
	"testing"
	"time"

	"orbyte/internal/platform/module"
)

func TestPolicyAndHelperConversions(t *testing.T) {
	rule := map[string]any{
		"items":   []any{"a", " ", "b"},
		"enabled": true,
		"count":   float64(3),
	}
	items := stringSliceRule(rule, "items")
	if len(items) != 2 || items[0] != "a" || items[1] != "b" {
		t.Fatalf("expected string slice conversion, got %+v", items)
	}
	if !boolRule(rule, "enabled") {
		t.Fatal("expected bool rule conversion")
	}
	if intRule(rule, "count") != 3 {
		t.Fatalf("expected int rule conversion, got %d", intRule(rule, "count"))
	}
	if !containsValue(items, "a") || containsValue(items, "c") {
		t.Fatalf("containsValue mismatch for %+v", items)
	}
	if firstReportFormat(nil) != "csv" || firstReportFormat([]string{"xlsx"}) != "xlsx" {
		t.Fatal("unexpected report format selection")
	}
	if firstValue("", " x ", "y") != " x " {
		t.Fatalf("unexpected firstValue result: %q", firstValue("", " x ", "y"))
	}
	if stringValue("  hello ") != "hello" {
		t.Fatalf("unexpected stringValue result: %q", stringValue("  hello "))
	}
}

func TestDefaultPolicyRuleCoversKnownHooks(t *testing.T) {
	cases := []string{
		"documents.extension.view",
		"documents.extension.write",
		"documents.workflow.transition",
		"documents.search.visibility",
		"documents.numbering.assign",
		"documents.action.render",
		"integration.submission.preflight",
		"unknown",
	}
	for _, hook := range cases {
		rule := defaultPolicyRule(hook)
		if rule == nil {
			t.Fatalf("expected default rule for %s", hook)
		}
	}
	if boolRule(defaultPolicyRule("documents.numbering.assign"), "include_location") != true {
		t.Fatal("expected numbering default include_location")
	}
}

func TestExternalBrokerRoutesIncludesEnabledPublishableEventsOnly(t *testing.T) {
	moduleSvc := module.NewService()
	if err := moduleSvc.Register(module.Manifest{
		Key:     "documents",
		Name:    "Documents",
		Version: "1.0.0",
		Observability: module.ObservabilityDefinition{
			DomainEvents: []module.DomainEventDefinition{
				{Type: "document.submitted", ExternalPublish: true, Topic: "documents.lifecycle.submitted"},
				{Type: "document.reject", ExternalPublish: false, Topic: "documents.lifecycle.rejected"},
			},
		},
	}, "system"); err != nil {
		t.Fatalf("register module failed: %v", err)
	}
	if err := moduleSvc.Register(module.Manifest{
		Key:     "analytics",
		Name:    "Analytics",
		Version: "1.0.0",
		Observability: module.ObservabilityDefinition{
			DomainEvents: []module.DomainEventDefinition{
				{Type: "analytics.snapshot.captured", ExternalPublish: true, Topic: "analytics.snapshot.captured"},
			},
		},
	}, "system"); err != nil {
		t.Fatalf("register module failed: %v", err)
	}
	if _, err := moduleSvc.Disable("analytics", "tester"); err != nil {
		t.Fatalf("disable module failed: %v", err)
	}

	routes := externalBrokerRoutes(moduleSvc, "clinic")
	if len(routes) != 1 {
		t.Fatalf("expected one broker route, got %+v", routes)
	}
	if routes["document.submitted"] != "clinic.documents.lifecycle.submitted" {
		t.Fatalf("unexpected broker topic: %+v", routes)
	}
}

func TestBrokerTopicFormatting(t *testing.T) {
	if brokerTopic("clinic", "documents.lifecycle") != "clinic.documents.lifecycle" {
		t.Fatal("expected prefixed broker topic")
	}
	if brokerTopic(" clinic. ", ".documents.lifecycle.") != "clinic.documents.lifecycle" {
		t.Fatal("expected trimmed broker topic")
	}
	if brokerTopic("", "documents.lifecycle") != "documents.lifecycle" {
		t.Fatal("expected bare topic without prefix")
	}
	if brokerTopic("clinic", "") != "clinic" {
		t.Fatal("expected prefix-only broker topic")
	}
}

func TestFirstValueStillReturnsFirstNonEmptyString(t *testing.T) {
	if got := firstValue("", "", "fallback"); got != "fallback" {
		t.Fatalf("unexpected firstValue result: %q", got)
	}
	if got := firstValue("", time.Now().UTC().Format(time.RFC3339), "fallback"); got == "" {
		t.Fatal("expected non-empty value")
	}
}
