package modulegen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func boolPtr(value bool) *bool {
	return &value
}

func writeGeneratorBootstrap(t *testing.T, root string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, "internal", "modules"), 0o755); err != nil {
		t.Fatalf("mkdir modules failed: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "internal", "platform", "app"), 0o755); err != nil {
		t.Fatalf("mkdir app failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "internal", "modules", "registry.go"), []byte(`package modules

import (
	platformmodule "orbyte/internal/platform/module"
	// modulegen:imports
)

func allManifests() []platformmodule.Manifest {
	return []platformmodule.Manifest{
		// modulegen:manifests
	}
}
`), 0o644); err != nil {
		t.Fatalf("write registry failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "internal", "platform", "app", "app.go"), []byte(`package app

func builtInModuleManifests() {}
`), 0o644); err != nil {
		t.Fatalf("write app failed: %v", err)
	}
}

func TestResolveSpecAppliesKindDefaults(t *testing.T) {
	spec, err := ResolveSpec(Spec{}, Options{
		Root:         t.TempDir(),
		Key:          "sales",
		Name:         "Sales",
		Kind:         string(KindHybrid),
		DomainFamily: "business",
	})
	if err != nil {
		t.Fatalf("resolve spec failed: %v", err)
	}
	if spec.Module.Version != "1.0.0" {
		t.Fatalf("expected default version, got %s", spec.Module.Version)
	}
	if spec.Document.Type == "" || spec.Model.Key == "" {
		t.Fatalf("expected derived defaults for hybrid module, got %+v", spec)
	}
	if len(spec.Model.Fields) < 2 || spec.Model.Fields[0].Key != "name" || spec.Model.Fields[1].Key != "status" {
		t.Fatalf("expected default model fields, got %+v", spec.Model.Fields)
	}
	if !spec.Features.Search || !spec.Features.UI || !spec.Features.CustomUI || !spec.Features.Tests {
		t.Fatalf("expected default features enabled, got %+v", spec.Features)
	}
	if !spec.Features.Reporting {
		t.Fatalf("expected reporting enabled by default for hybrid modules, got %+v", spec.Features)
	}
	if spec.Scaffold.ReportingHelper == nil || !*spec.Scaffold.ReportingHelper {
		t.Fatalf("expected reporting helper scaffold enabled, got %+v", spec.Scaffold)
	}
	if spec.Scaffold.ObservabilityHelper == nil || !*spec.Scaffold.ObservabilityHelper {
		t.Fatalf("expected observability helper scaffold enabled, got %+v", spec.Scaffold)
	}
	if spec.Scaffold.ManifestTest == nil || !*spec.Scaffold.ManifestTest || spec.Scaffold.RegistrationTest == nil || !*spec.Scaffold.RegistrationTest {
		t.Fatalf("expected test scaffolds enabled, got %+v", spec.Scaffold)
	}
	if spec.Manifest.ModelStub == nil || !*spec.Manifest.ModelStub ||
		spec.Manifest.DatasetStub == nil || !*spec.Manifest.DatasetStub ||
		spec.Manifest.SearchIndexStub == nil || !*spec.Manifest.SearchIndexStub ||
		spec.Manifest.RoleTemplateStub == nil || !*spec.Manifest.RoleTemplateStub ||
		spec.Manifest.PolicyHookStub == nil || !*spec.Manifest.PolicyHookStub ||
		spec.Manifest.ObservabilityStub == nil || !*spec.Manifest.ObservabilityStub ||
		spec.Manifest.GenericUIStub == nil || !*spec.Manifest.GenericUIStub ||
		spec.Manifest.CustomUIStub == nil || !*spec.Manifest.CustomUIStub {
		t.Fatalf("expected manifest stubs enabled by default, got %+v", spec.Manifest)
	}
}

func TestResolveSpecAppliesMinimalProfile(t *testing.T) {
	spec, err := ResolveSpec(Spec{}, Options{
		Root:         t.TempDir(),
		Profile:      "minimal",
		Key:          "notes",
		Name:         "Notes",
		Kind:         string(KindModel),
		DomainFamily: "business",
	})
	if err != nil {
		t.Fatalf("resolve spec failed: %v", err)
	}
	if spec.Features.Search || spec.Features.UI || spec.Features.CustomUI || spec.Features.Policy || spec.Features.Observability || spec.Features.Reporting {
		t.Fatalf("expected minimal profile to disable optional features, got %+v", spec.Features)
	}
	if !spec.Features.Tests {
		t.Fatalf("expected minimal profile to keep tests enabled, got %+v", spec.Features)
	}
	if spec.Manifest.ModelStub == nil || !*spec.Manifest.ModelStub {
		t.Fatalf("expected minimal profile to keep model stub for model module, got %+v", spec.Manifest)
	}
	if spec.Manifest.GenericUIStub == nil || *spec.Manifest.GenericUIStub {
		t.Fatalf("expected minimal profile to disable generic ui stub, got %+v", spec.Manifest)
	}
	if spec.Scaffold.ManifestTest == nil || !*spec.Scaffold.ManifestTest || spec.Scaffold.RegistrationTest == nil || *spec.Scaffold.RegistrationTest {
		t.Fatalf("expected minimal profile test scaffold mix, got %+v", spec.Scaffold)
	}
}

func TestResolveSpecAppliesIntegrationFirstProfile(t *testing.T) {
	spec, err := ResolveSpec(Spec{}, Options{
		Root:         t.TempDir(),
		Profile:      "integration-first",
		Key:          "syncbridge",
		Name:         "Sync Bridge",
		DomainFamily: "ops",
	})
	if err != nil {
		t.Fatalf("resolve spec failed: %v", err)
	}
	if spec.Module.Kind != KindIntegration {
		t.Fatalf("expected integration-first profile to force integration kind, got %s", spec.Module.Kind)
	}
	if spec.Features.Search || spec.Features.CustomUI {
		t.Fatalf("expected integration-first profile to disable search and custom ui, got %+v", spec.Features)
	}
	if !spec.Features.UI || !spec.Features.Policy || !spec.Features.Observability || !spec.Features.Reporting {
		t.Fatalf("expected integration-first profile to enable operational features, got %+v", spec.Features)
	}
	if spec.Manifest.SearchIndexStub == nil || *spec.Manifest.SearchIndexStub {
		t.Fatalf("expected integration-first profile to disable search stub, got %+v", spec.Manifest)
	}
}

func TestResolveSpecProfileAllowsCLIOverride(t *testing.T) {
	spec, err := ResolveSpec(Spec{}, Options{
		Root:              t.TempDir(),
		Profile:           "minimal",
		Key:               "catalog",
		Name:              "Catalog",
		Kind:              string(KindModel),
		DomainFamily:      "business",
		WithGenericUIStub: optionalBool{set: true, value: true},
		WithUI:            optionalBool{set: true, value: true},
	})
	if err != nil {
		t.Fatalf("resolve spec failed: %v", err)
	}
	if !spec.Features.UI {
		t.Fatalf("expected cli to re-enable ui on top of profile, got %+v", spec.Features)
	}
	if spec.Manifest.GenericUIStub == nil || !*spec.Manifest.GenericUIStub {
		t.Fatalf("expected cli to re-enable generic ui stub on top of profile, got %+v", spec.Manifest)
	}
}

func TestResolveSpecAppliesCLIOverridesToScaffolds(t *testing.T) {
	spec, err := ResolveSpec(Spec{
		Module: ModuleIdentity{
			Key:  "catalog",
			Name: "Catalog",
			Kind: KindModel,
		},
		Features: FeatureOptions{
			Observability: true,
			Reporting:     true,
			Tests:         true,
		},
		Scaffold: ScaffoldOptions{
			ObservabilityHelper: boolPtr(false),
			ReportingHelper:     boolPtr(false),
			ManifestTest:        boolPtr(false),
			RegistrationTest:    boolPtr(false),
		},
	}, Options{
		Root:                    t.TempDir(),
		WithObservabilityHelper: optionalBool{set: true, value: true},
		WithReportingHelper:     optionalBool{set: true, value: true},
		WithManifestTest:        optionalBool{set: true, value: true},
		WithRegistrationTest:    optionalBool{set: true, value: true},
	})
	if err != nil {
		t.Fatalf("resolve spec failed: %v", err)
	}
	if spec.Scaffold.ObservabilityHelper == nil || !*spec.Scaffold.ObservabilityHelper ||
		spec.Scaffold.ReportingHelper == nil || !*spec.Scaffold.ReportingHelper ||
		spec.Scaffold.ManifestTest == nil || !*spec.Scaffold.ManifestTest ||
		spec.Scaffold.RegistrationTest == nil || !*spec.Scaffold.RegistrationTest {
		t.Fatalf("expected cli scaffold overrides to enable all scaffold files, got %+v", spec.Scaffold)
	}
}

func TestResolveSpecAppliesCLIOverridesToManifestStubs(t *testing.T) {
	spec, err := ResolveSpec(Spec{
		Module: ModuleIdentity{
			Key:  "catalog",
			Name: "Catalog",
			Kind: KindHybrid,
		},
		Manifest: ManifestOptions{
			ModelStub:         boolPtr(false),
			DatasetStub:       boolPtr(false),
			SearchIndexStub:   boolPtr(false),
			RoleTemplateStub:  boolPtr(false),
			PolicyHookStub:    boolPtr(false),
			ObservabilityStub: boolPtr(false),
			GenericUIStub:     boolPtr(false),
			CustomUIStub:      boolPtr(false),
		},
	}, Options{
		Root:                  t.TempDir(),
		WithModelStub:         optionalBool{set: true, value: true},
		WithDatasetStub:       optionalBool{set: true, value: true},
		WithSearchIndexStub:   optionalBool{set: true, value: true},
		WithRoleTemplateStub:  optionalBool{set: true, value: true},
		WithPolicyHookStub:    optionalBool{set: true, value: true},
		WithObservabilityStub: optionalBool{set: true, value: true},
		WithGenericUIStub:     optionalBool{set: true, value: true},
		WithCustomUIStub:      optionalBool{set: true, value: true},
	})
	if err != nil {
		t.Fatalf("resolve spec failed: %v", err)
	}
	if spec.Manifest.ModelStub == nil || !*spec.Manifest.ModelStub ||
		spec.Manifest.DatasetStub == nil || !*spec.Manifest.DatasetStub ||
		spec.Manifest.SearchIndexStub == nil || !*spec.Manifest.SearchIndexStub ||
		spec.Manifest.RoleTemplateStub == nil || !*spec.Manifest.RoleTemplateStub ||
		spec.Manifest.PolicyHookStub == nil || !*spec.Manifest.PolicyHookStub ||
		spec.Manifest.ObservabilityStub == nil || !*spec.Manifest.ObservabilityStub ||
		spec.Manifest.GenericUIStub == nil || !*spec.Manifest.GenericUIStub ||
		spec.Manifest.CustomUIStub == nil || !*spec.Manifest.CustomUIStub {
		t.Fatalf("expected cli manifest overrides to enable all stubs, got %+v", spec.Manifest)
	}
}

func TestPlanModuleCreatesFilesAndPatchesRegistry(t *testing.T) {
	root := t.TempDir()
	writeGeneratorBootstrap(t, root)

	spec, err := ResolveSpec(Spec{}, Options{
		Root:         root,
		Key:          "sales",
		Name:         "Sales",
		Kind:         string(KindModel),
		DomainFamily: "business",
	})
	if err != nil {
		t.Fatalf("resolve spec failed: %v", err)
	}
	plan, err := PlanModule(root, spec)
	if err != nil {
		t.Fatalf("plan module failed: %v", err)
	}
	foundManifest := false
	foundRegistry := false
	foundObservability := false
	foundReporting := false
	foundManifestTest := false
	foundRegistrationTest := false
	for _, file := range plan.Files {
		switch {
		case strings.HasSuffix(file.Path, "manifest.go"):
			foundManifest = true
			if !strings.Contains(file.Content, `Key:          "sales"`) {
				t.Fatalf("expected manifest content to include module key, got %s", file.Content)
			}
		case strings.HasSuffix(file.Path, "observability.go"):
			foundObservability = true
		case strings.HasSuffix(file.Path, "reporting.go"):
			foundReporting = true
			if !strings.Contains(file.Content, "DatasetDefinitions") {
				t.Fatalf("expected reporting helper content, got %s", file.Content)
			}
		case strings.HasSuffix(file.Path, "service_test.go"):
			foundManifestTest = true
		case strings.HasSuffix(file.Path, "registration_test.go"):
			foundRegistrationTest = true
		case strings.HasSuffix(file.Path, filepath.Join("internal", "modules", "registry.go")):
			foundRegistry = true
			if !strings.Contains(file.Content, `salesmodule "orbyte/internal/modules/sales"`) {
				t.Fatalf("expected registry import patch, got %s", file.Content)
			}
			if !strings.Contains(file.Content, "salesmodule.Manifest()") {
				t.Fatalf("expected registry manifest patch, got %s", file.Content)
			}
		}
	}
	if !foundManifest || !foundRegistry || !foundObservability || !foundReporting || !foundManifestTest || !foundRegistrationTest {
		t.Fatalf("expected manifest, registry, helper, and test outputs; got manifest=%v registry=%v observability=%v reporting=%v manifest_test=%v registration_test=%v", foundManifest, foundRegistry, foundObservability, foundReporting, foundManifestTest, foundRegistrationTest)
	}
}

func TestPlanModuleRendersSecurityFieldMetadata(t *testing.T) {
	root := t.TempDir()
	writeGeneratorBootstrap(t, root)

	spec, err := ResolveSpec(Spec{
		Module: ModuleIdentity{
			Key:          "patients",
			Name:         "Patients",
			Kind:         KindModel,
			DomainFamily: "healthcare",
		},
		Model: ModelOptions{
			Key:         "patient",
			DisplayName: "Patient",
			Fields: []ModelFieldOptions{
				{Key: "full_name", Label: "Full Name", Type: "string", Required: true},
				{Key: "patient_ssn", Label: "Patient SSN", Type: "string", Sensitive: true, DefaultMask: "last4", ReadPermissionKey: "patients.ssn.read", WritePermissionKey: "patients.ssn.write", SearchVisible: boolPtr(false), ExportVisible: boolPtr(false)},
			},
		},
	}, Options{Root: root})
	if err != nil {
		t.Fatalf("resolve spec failed: %v", err)
	}
	plan, err := PlanModule(root, spec)
	if err != nil {
		t.Fatalf("plan module failed: %v", err)
	}
	var manifest string
	for _, file := range plan.Files {
		if strings.HasSuffix(file.Path, "manifest.go") {
			manifest = file.Content
			break
		}
	}
	if manifest == "" {
		t.Fatal("expected manifest.go in plan output")
	}
	for _, needle := range []string{
		`Key: "patient_ssn"`,
		`Sensitive: true`,
		`DefaultMask: "last4"`,
		`ReadPermissionKey: "patients.ssn.read"`,
		`WritePermissionKey: "patients.ssn.write"`,
		`SearchVisible: boolPtr(false)`,
		`ExportVisible: boolPtr(false)`,
		`DefaultSort:         "full_name"`,
		`Path: "full_name"`,
	} {
		if !strings.Contains(manifest, needle) {
			t.Fatalf("expected manifest to contain %q, got:\n%s", needle, manifest)
		}
	}
}

func TestPlanModuleRespectsManifestOptions(t *testing.T) {
	root := t.TempDir()
	writeGeneratorBootstrap(t, root)

	spec, err := ResolveSpec(Spec{
		Manifest: ManifestOptions{
			ModelStub:         boolPtr(false),
			DatasetStub:       boolPtr(false),
			SearchIndexStub:   boolPtr(false),
			RoleTemplateStub:  boolPtr(false),
			PolicyHookStub:    boolPtr(false),
			ObservabilityStub: boolPtr(false),
			GenericUIStub:     boolPtr(false),
			CustomUIStub:      boolPtr(false),
		},
	}, Options{
		Root:         root,
		Key:          "support",
		Name:         "Support",
		Kind:         string(KindHybrid),
		DomainFamily: "services",
	})
	if err != nil {
		t.Fatalf("resolve spec failed: %v", err)
	}
	plan, err := PlanModule(root, spec)
	if err != nil {
		t.Fatalf("plan module failed: %v", err)
	}
	for _, file := range plan.Files {
		if filepath.Base(file.Path) != "manifest.go" {
			continue
		}
		if strings.Contains(file.Content, "Models: []model.Definition") ||
			strings.Contains(file.Content, "Datasets: []module.DatasetDefinition") ||
			strings.Contains(file.Content, "SearchIndexes: []search.IndexDefinition") ||
			strings.Contains(file.Content, "RoleTemplates: []module.RoleTemplateDefinition") ||
			strings.Contains(file.Content, "PolicyHooks: []module.PolicyHookDefinition") ||
			strings.Contains(file.Content, "Observability: module.ObservabilityDefinition") ||
			strings.Contains(file.Content, "Frontend: module.FrontendDefinition") ||
			strings.Contains(file.Content, "Bundles: []module.BundleDefinition") {
			t.Fatalf("expected manifest sections to be omitted when manifest options disabled, got %s", file.Content)
		}
		return
	}
	t.Fatal("expected manifest.go output")
}

func TestPlanModuleRespectsScaffoldOptions(t *testing.T) {
	root := t.TempDir()
	writeGeneratorBootstrap(t, root)

	spec, err := ResolveSpec(Spec{
		Scaffold: ScaffoldOptions{
			ObservabilityHelper: boolPtr(false),
			ReportingHelper:     boolPtr(false),
			ManifestTest:        boolPtr(false),
			RegistrationTest:    boolPtr(false),
		},
	}, Options{
		Root:         root,
		Key:          "support",
		Name:         "Support",
		Kind:         string(KindModel),
		DomainFamily: "services",
	})
	if err != nil {
		t.Fatalf("resolve spec failed: %v", err)
	}
	plan, err := PlanModule(root, spec)
	if err != nil {
		t.Fatalf("plan module failed: %v", err)
	}
	for _, file := range plan.Files {
		switch filepath.Base(file.Path) {
		case "observability.go", "reporting.go", "service_test.go", "registration_test.go":
			t.Fatalf("did not expect %s to be generated when scaffold options disabled", filepath.Base(file.Path))
		}
	}
}

func TestScaffoldWritesFiles(t *testing.T) {
	root := t.TempDir()
	writeGeneratorBootstrap(t, root)

	spec, err := ResolveSpec(Spec{}, Options{
		Root:         root,
		Key:          "billing",
		Name:         "Billing",
		Kind:         string(KindDocument),
		DomainFamily: "business",
	})
	if err != nil {
		t.Fatalf("resolve spec failed: %v", err)
	}
	if _, err := Scaffold(root, spec); err != nil {
		t.Fatalf("scaffold failed: %v", err)
	}
	for _, path := range []string{
		filepath.Join(root, "internal", "modules", "billing", "manifest.go"),
		filepath.Join(root, "internal", "modules", "billing", "bundle.js"),
		filepath.Join(root, "internal", "modules", "billing", "observability.go"),
		filepath.Join(root, "internal", "modules", "billing", "registration_test.go"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected generated file %s: %v", path, err)
		}
	}
}
