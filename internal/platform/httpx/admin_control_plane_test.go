package httpx

import (
	"testing"

	"orbyte/internal/platform/config"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/runtimehealth"
)

func TestBuildAdminReadinessReportIgnoresDisabledModules(t *testing.T) {
	cfg := config.NewService()
	modules := module.NewService()
	for _, manifest := range builtInTestModuleManifests() {
		if err := modules.Register(manifest, "system"); err != nil {
			t.Fatalf("register module failed: %v", err)
		}
	}
	if _, err := modules.Disable("analytics", "tester"); err != nil {
		t.Fatalf("disable module failed: %v", err)
	}

	health := runtimehealth.NewTracker()
	health.SetBootstrapped(true)
	health.SetBackgroundStarted(true)

	report := buildAdminReadinessReport(cfg, modules, health)
	if report.BlockedForApply {
		t.Fatalf("expected disabled modules to remain non-blocking, got %+v", report)
	}
	if report.Status != "ready" {
		t.Fatalf("expected readiness status ready, got %+v", report)
	}
	if len(report.ModuleCompatibility) != 0 {
		t.Fatalf("expected no blocking module compatibility entries, got %+v", report.ModuleCompatibility)
	}
}
