package modulegen

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"orbyte/internal/platform/module"

	"go.yaml.in/yaml/v3"
)

type Kind string

const (
	KindDocument    Kind = "document"
	KindModel       Kind = "model"
	KindHybrid      Kind = "hybrid"
	KindIntegration Kind = "integration"
)

type optionalBool struct {
	set   bool
	value bool
}

func (b *optionalBool) Set(value string) error {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "y", "on":
		b.set = true
		b.value = true
		return nil
	case "0", "false", "no", "n", "off":
		b.set = true
		b.value = false
		return nil
	default:
		return fmt.Errorf("invalid boolean %q", value)
	}
}

func (b *optionalBool) String() string {
	if !b.set {
		return ""
	}
	if b.value {
		return "true"
	}
	return "false"
}

type ModuleIdentity struct {
	Key          string `yaml:"key"`
	Name         string `yaml:"name"`
	Version      string `yaml:"version"`
	DomainFamily string `yaml:"domain_family"`
	Kind         Kind   `yaml:"kind"`
}

type FeatureOptions struct {
	Search        bool `yaml:"search"`
	UI            bool `yaml:"ui"`
	CustomUI      bool `yaml:"custom_ui"`
	Tests         bool `yaml:"tests"`
	Policy        bool `yaml:"policy"`
	Observability bool `yaml:"observability"`
	Reporting     bool `yaml:"reporting"`
}

type ScaffoldOptions struct {
	ObservabilityHelper *bool `yaml:"observability_helper"`
	ReportingHelper     *bool `yaml:"reporting_helper"`
	ManifestTest        *bool `yaml:"manifest_test"`
	RegistrationTest    *bool `yaml:"registration_test"`
}

type ManifestOptions struct {
	ModelStub         *bool `yaml:"model_stub"`
	DatasetStub       *bool `yaml:"dataset_stub"`
	SearchIndexStub   *bool `yaml:"search_index_stub"`
	RoleTemplateStub  *bool `yaml:"role_template_stub"`
	PolicyHookStub    *bool `yaml:"policy_hook_stub"`
	ObservabilityStub *bool `yaml:"observability_stub"`
	GenericUIStub     *bool `yaml:"generic_ui_stub"`
	CustomUIStub      *bool `yaml:"custom_ui_stub"`
}

type DocumentOptions struct {
	Type        string `yaml:"type"`
	DisplayName string `yaml:"display_name"`
}

type ModelOptions struct {
	Key         string              `yaml:"key"`
	DisplayName string              `yaml:"display_name"`
	Fields      []ModelFieldOptions `yaml:"fields,omitempty"`
}

type ModelFieldOptions struct {
	Key                string   `yaml:"key"`
	Label              string   `yaml:"label,omitempty"`
	Type               string   `yaml:"type"`
	Required           bool     `yaml:"required,omitempty"`
	ReadOnly           bool     `yaml:"read_only,omitempty"`
	Indexed            bool     `yaml:"indexed,omitempty"`
	Sensitive          bool     `yaml:"sensitive,omitempty"`
	SecurityClass      string   `yaml:"security_class,omitempty"`
	DefaultMask        string   `yaml:"default_mask,omitempty"`
	SearchVisible      *bool    `yaml:"search_visible,omitempty"`
	ExportVisible      *bool    `yaml:"export_visible,omitempty"`
	ReadPermissionKey  string   `yaml:"read_permission_key,omitempty"`
	WritePermissionKey string   `yaml:"write_permission_key,omitempty"`
	DefaultValue       any      `yaml:"default_value,omitempty"`
	DefaultRuleKey     string   `yaml:"default_rule_key,omitempty"`
	ComputeRuleKey     string   `yaml:"compute_rule_key,omitempty"`
	ConstraintRuleKeys []string `yaml:"constraint_rule_keys,omitempty"`
}

type Spec struct {
	Module                 ModuleIdentity                 `yaml:"module"`
	StarterPack            string                         `yaml:"starter_pack,omitempty"`
	Features               FeatureOptions                 `yaml:"features"`
	Scaffold               ScaffoldOptions                `yaml:"scaffold"`
	Manifest               ManifestOptions                `yaml:"manifest"`
	Dependencies           []string                       `yaml:"dependencies"`
	DependencyRequirements []module.DependencyRequirement `yaml:"dependency_requirements"`
	Document               DocumentOptions                `yaml:"document"`
	Model                  ModelOptions                   `yaml:"model"`
}

type Options struct {
	Root                    string
	SpecPath                string
	Profile                 string
	StarterPack             string
	JSON                    bool
	Key                     string
	Name                    string
	Version                 string
	DomainFamily            string
	Kind                    string
	WithSearch              optionalBool
	WithUI                  optionalBool
	WithCustomUI            optionalBool
	WithTests               optionalBool
	WithPolicy              optionalBool
	WithObservability       optionalBool
	WithReporting           optionalBool
	WithObservabilityHelper optionalBool
	WithReportingHelper     optionalBool
	WithManifestTest        optionalBool
	WithRegistrationTest    optionalBool
	WithModelStub           optionalBool
	WithDatasetStub         optionalBool
	WithSearchIndexStub     optionalBool
	WithRoleTemplateStub    optionalBool
	WithPolicyHookStub      optionalBool
	WithObservabilityStub   optionalBool
	WithGenericUIStub       optionalBool
	WithCustomUIStub        optionalBool
}

func LoadSpec(path string) (Spec, error) {
	var spec Spec
	if strings.TrimSpace(path) == "" {
		return spec, nil
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return Spec{}, err
	}
	if err := yaml.Unmarshal(content, &spec); err != nil {
		return Spec{}, err
	}
	return spec, nil
}

func ResolveSpec(base Spec, opts Options) (Spec, error) {
	spec := base
	if strings.TrimSpace(opts.Key) != "" {
		spec.Module.Key = strings.TrimSpace(opts.Key)
	}
	if strings.TrimSpace(opts.Name) != "" {
		spec.Module.Name = strings.TrimSpace(opts.Name)
	}
	if strings.TrimSpace(opts.Version) != "" {
		spec.Module.Version = strings.TrimSpace(opts.Version)
	}
	if strings.TrimSpace(opts.DomainFamily) != "" {
		spec.Module.DomainFamily = strings.TrimSpace(opts.DomainFamily)
	}
	if strings.TrimSpace(opts.Kind) != "" {
		spec.Module.Kind = Kind(strings.TrimSpace(opts.Kind))
	}
	if spec.Module.Version == "" {
		spec.Module.Version = "1.0.0"
	}
	if spec.Module.DomainFamily == "" {
		spec.Module.DomainFamily = "business"
	}
	if spec.Module.Kind == "" {
		spec.Module.Kind = KindHybrid
	}
	applyKindDefaults(&spec)
	if strings.TrimSpace(opts.Profile) != "" {
		if err := applyProfile(&spec, strings.TrimSpace(opts.Profile)); err != nil {
			return Spec{}, err
		}
	}
	if strings.TrimSpace(opts.StarterPack) != "" {
		spec.StarterPack = strings.TrimSpace(opts.StarterPack)
		if err := applyProfile(&spec, spec.StarterPack); err != nil {
			return Spec{}, err
		}
	}
	applyBoolOverride(&spec.Features.Search, opts.WithSearch)
	applyBoolOverride(&spec.Features.UI, opts.WithUI)
	applyBoolOverride(&spec.Features.CustomUI, opts.WithCustomUI)
	applyBoolOverride(&spec.Features.Tests, opts.WithTests)
	applyBoolOverride(&spec.Features.Policy, opts.WithPolicy)
	applyBoolOverride(&spec.Features.Observability, opts.WithObservability)
	applyBoolOverride(&spec.Features.Reporting, opts.WithReporting)
	fillDerivedDefaults(&spec)
	fillScaffoldDefaults(&spec)
	fillManifestDefaults(&spec)
	applyBoolPtrOverride(&spec.Scaffold.ObservabilityHelper, opts.WithObservabilityHelper)
	applyBoolPtrOverride(&spec.Scaffold.ReportingHelper, opts.WithReportingHelper)
	applyBoolPtrOverride(&spec.Scaffold.ManifestTest, opts.WithManifestTest)
	applyBoolPtrOverride(&spec.Scaffold.RegistrationTest, opts.WithRegistrationTest)
	applyBoolPtrOverride(&spec.Manifest.ModelStub, opts.WithModelStub)
	applyBoolPtrOverride(&spec.Manifest.DatasetStub, opts.WithDatasetStub)
	applyBoolPtrOverride(&spec.Manifest.SearchIndexStub, opts.WithSearchIndexStub)
	applyBoolPtrOverride(&spec.Manifest.RoleTemplateStub, opts.WithRoleTemplateStub)
	applyBoolPtrOverride(&spec.Manifest.PolicyHookStub, opts.WithPolicyHookStub)
	applyBoolPtrOverride(&spec.Manifest.ObservabilityStub, opts.WithObservabilityStub)
	applyBoolPtrOverride(&spec.Manifest.GenericUIStub, opts.WithGenericUIStub)
	applyBoolPtrOverride(&spec.Manifest.CustomUIStub, opts.WithCustomUIStub)
	return spec, ValidateSpec(rootOrDefault(opts.Root), spec)
}

func ValidateSpec(root string, spec Spec) error {
	if !validModuleKey(spec.Module.Key) {
		return fmt.Errorf("module key %q is invalid", spec.Module.Key)
	}
	if strings.TrimSpace(spec.Module.Name) == "" {
		return fmt.Errorf("module name is required")
	}
	switch spec.Module.Kind {
	case KindDocument, KindModel, KindHybrid, KindIntegration:
	default:
		return fmt.Errorf("unsupported module kind %q", spec.Module.Kind)
	}
	moduleDir := filepath.Join(root, "internal", "modules", spec.Module.Key)
	if _, err := os.Stat(moduleDir); err == nil {
		return fmt.Errorf("module directory already exists: %s", moduleDir)
	}
	appPath := filepath.Join(root, "internal", "platform", "app", "app.go")
	if content, err := os.ReadFile(appPath); err == nil {
		if strings.Contains(string(content), `Key: "`+spec.Module.Key+`"`) {
			return fmt.Errorf("module key %q already exists in bootstrap manifests", spec.Module.Key)
		}
	}
	registryPath := filepath.Join(root, "internal", "modules", "registry.go")
	if content, err := os.ReadFile(registryPath); err == nil {
		if strings.Contains(string(content), `internal/modules/`+spec.Module.Key+`"`) {
			return fmt.Errorf("module key %q already exists in module registry", spec.Module.Key)
		}
		if !strings.Contains(string(content), "// modulegen:manifests") {
			return fmt.Errorf("module registry missing // modulegen:manifests marker")
		}
	}
	if spec.Module.Kind == KindDocument || spec.Module.Kind == KindHybrid {
		if strings.TrimSpace(spec.Document.Type) == "" {
			return fmt.Errorf("document.type is required for %s modules", spec.Module.Kind)
		}
	}
	if spec.Module.Kind == KindModel || spec.Module.Kind == KindHybrid {
		if strings.TrimSpace(spec.Model.Key) == "" {
			return fmt.Errorf("model.key is required for %s modules", spec.Module.Kind)
		}
		if len(spec.Model.Fields) == 0 {
			return fmt.Errorf("model.fields is required for %s modules", spec.Module.Kind)
		}
		seen := map[string]bool{}
		for _, field := range spec.Model.Fields {
			if strings.TrimSpace(field.Key) == "" {
				return fmt.Errorf("model.fields.key is required")
			}
			if strings.TrimSpace(field.Type) == "" {
				return fmt.Errorf("model.fields[%s].type is required", field.Key)
			}
			if seen[field.Key] {
				return fmt.Errorf("model field %q is duplicated", field.Key)
			}
			seen[field.Key] = true
		}
	}
	return nil
}

func applyKindDefaults(spec *Spec) {
	switch spec.Module.Kind {
	case KindDocument:
		if spec.Features.Search == false && spec.Features.UI == false && spec.Features.CustomUI == false && spec.Features.Tests == false && spec.Features.Policy == false && spec.Features.Observability == false && spec.Features.Reporting == false {
			spec.Features = FeatureOptions{Search: true, UI: true, CustomUI: true, Tests: true, Policy: true, Observability: true, Reporting: false}
		}
		spec.Dependencies = defaultDependencies(spec.Dependencies, "platform.core", "identity", "documents")
	case KindModel:
		if spec.Features.Search == false && spec.Features.UI == false && spec.Features.CustomUI == false && spec.Features.Tests == false && spec.Features.Policy == false && spec.Features.Observability == false && spec.Features.Reporting == false {
			spec.Features = FeatureOptions{Search: true, UI: true, CustomUI: true, Tests: true, Policy: true, Observability: true, Reporting: true}
		}
		spec.Dependencies = defaultDependencies(spec.Dependencies, "platform.core", "identity")
	case KindHybrid:
		if spec.Features.Search == false && spec.Features.UI == false && spec.Features.CustomUI == false && spec.Features.Tests == false && spec.Features.Policy == false && spec.Features.Observability == false && spec.Features.Reporting == false {
			spec.Features = FeatureOptions{Search: true, UI: true, CustomUI: true, Tests: true, Policy: true, Observability: true, Reporting: true}
		}
		spec.Dependencies = defaultDependencies(spec.Dependencies, "platform.core", "identity", "documents")
	case KindIntegration:
		if spec.Features.Search == false && spec.Features.UI == false && spec.Features.CustomUI == false && spec.Features.Tests == false && spec.Features.Policy == false && spec.Features.Observability == false && spec.Features.Reporting == false {
			spec.Features = FeatureOptions{Search: false, UI: true, CustomUI: true, Tests: true, Policy: true, Observability: true, Reporting: true}
		}
		spec.Dependencies = defaultDependencies(spec.Dependencies, "platform.core", "identity")
	}
	if len(spec.DependencyRequirements) == 0 {
		spec.DependencyRequirements = make([]module.DependencyRequirement, 0, len(spec.Dependencies))
		for _, dep := range spec.Dependencies {
			spec.DependencyRequirements = append(spec.DependencyRequirements, module.DependencyRequirement{
				ModuleKey:    dep,
				VersionRange: ">=1.0.0,<2.0.0",
				Kind:         module.DependencyKindRequired,
			})
		}
	}
	if strings.TrimSpace(spec.StarterPack) == "" {
		spec.StarterPack = defaultStarterPack(spec.Module.Kind)
	}
}

func fillDerivedDefaults(spec *Spec) {
	if spec.Document.Type == "" && (spec.Module.Kind == KindDocument || spec.Module.Kind == KindHybrid) {
		spec.Document.Type = spec.Module.Key + "_request"
	}
	if spec.Document.DisplayName == "" && spec.Document.Type != "" {
		spec.Document.DisplayName = spec.Module.Name + " Request"
	}
	if spec.Model.Key == "" && (spec.Module.Kind == KindModel || spec.Module.Kind == KindHybrid) {
		spec.Model.Key = spec.Module.Key
	}
	if spec.Model.DisplayName == "" && spec.Model.Key != "" {
		spec.Model.DisplayName = spec.Module.Name
	}
	if len(spec.Model.Fields) == 0 && (spec.Module.Kind == KindModel || spec.Module.Kind == KindHybrid) {
		spec.Model.Fields = []ModelFieldOptions{
			{Key: "name", Label: "Name", Type: "string", Required: true},
			{Key: "status", Label: "Status", Type: "string", DefaultValue: "active"},
		}
	}
}

func fillScaffoldDefaults(spec *Spec) {
	spec.Scaffold.ObservabilityHelper = boolPtrDefault(spec.Scaffold.ObservabilityHelper, spec.Features.Observability)
	spec.Scaffold.ReportingHelper = boolPtrDefault(spec.Scaffold.ReportingHelper, spec.Features.Reporting)
	spec.Scaffold.ManifestTest = boolPtrDefault(spec.Scaffold.ManifestTest, spec.Features.Tests)
	spec.Scaffold.RegistrationTest = boolPtrDefault(spec.Scaffold.RegistrationTest, spec.Features.Tests)
}

func fillManifestDefaults(spec *Spec) {
	spec.Manifest.ModelStub = boolPtrDefault(spec.Manifest.ModelStub, spec.Module.Kind == KindModel || spec.Module.Kind == KindHybrid)
	spec.Manifest.DatasetStub = boolPtrDefault(spec.Manifest.DatasetStub, spec.Features.Reporting && (spec.Module.Kind == KindModel || spec.Module.Kind == KindHybrid))
	spec.Manifest.SearchIndexStub = boolPtrDefault(spec.Manifest.SearchIndexStub, spec.Features.Search)
	spec.Manifest.RoleTemplateStub = boolPtrDefault(spec.Manifest.RoleTemplateStub, true)
	spec.Manifest.PolicyHookStub = boolPtrDefault(spec.Manifest.PolicyHookStub, spec.Features.Policy)
	spec.Manifest.ObservabilityStub = boolPtrDefault(spec.Manifest.ObservabilityStub, spec.Features.Observability)
	spec.Manifest.GenericUIStub = boolPtrDefault(spec.Manifest.GenericUIStub, spec.Features.UI)
	spec.Manifest.CustomUIStub = boolPtrDefault(spec.Manifest.CustomUIStub, spec.Features.CustomUI)
}

func boolPtrDefault(value *bool, fallback bool) *bool {
	if value != nil {
		return value
	}
	return &fallback
}

func validModuleKey(key string) bool {
	ok, _ := regexp.MatchString(`^[a-z][a-z0-9_]*$`, strings.TrimSpace(key))
	return ok
}

func applyBoolOverride(target *bool, override optionalBool) {
	if override.set {
		*target = override.value
	}
}

func applyBoolPtrOverride(target **bool, override optionalBool) {
	if override.set {
		value := override.value
		*target = &value
	}
}

func defaultDependencies(current []string, defaults ...string) []string {
	if len(current) > 0 {
		return append([]string(nil), current...)
	}
	return append([]string(nil), defaults...)
}

func applyProfile(spec *Spec, profile string) error {
	switch strings.TrimSpace(strings.ToLower(profile)) {
	case "", "default":
		return nil
	case "minimal":
		spec.StarterPack = "minimal"
		spec.Features.Search = false
		spec.Features.UI = false
		spec.Features.CustomUI = false
		spec.Features.Tests = true
		spec.Features.Policy = false
		spec.Features.Observability = false
		spec.Features.Reporting = false
		spec.Scaffold = ScaffoldOptions{
			ObservabilityHelper: boolPtrDefault(nil, false),
			ReportingHelper:     boolPtrDefault(nil, false),
			ManifestTest:        boolPtrDefault(nil, true),
			RegistrationTest:    boolPtrDefault(nil, false),
		}
		spec.Manifest = ManifestOptions{
			ModelStub:         boolPtrDefault(nil, spec.Module.Kind == KindModel || spec.Module.Kind == KindHybrid),
			DatasetStub:       boolPtrDefault(nil, false),
			SearchIndexStub:   boolPtrDefault(nil, false),
			RoleTemplateStub:  boolPtrDefault(nil, true),
			PolicyHookStub:    boolPtrDefault(nil, false),
			ObservabilityStub: boolPtrDefault(nil, false),
			GenericUIStub:     boolPtrDefault(nil, false),
			CustomUIStub:      boolPtrDefault(nil, false),
		}
		return nil
	case "backoffice", "document-workflow":
		spec.StarterPack = "document-workflow"
		spec.Features.Search = true
		spec.Features.UI = true
		spec.Features.CustomUI = false
		spec.Features.Tests = true
		spec.Features.Policy = true
		spec.Features.Observability = true
		spec.Features.Reporting = true
		spec.Scaffold = ScaffoldOptions{
			ObservabilityHelper: boolPtrDefault(nil, true),
			ReportingHelper:     boolPtrDefault(nil, true),
			ManifestTest:        boolPtrDefault(nil, true),
			RegistrationTest:    boolPtrDefault(nil, true),
		}
		spec.Manifest = ManifestOptions{
			ModelStub:         boolPtrDefault(nil, spec.Module.Kind == KindModel || spec.Module.Kind == KindHybrid),
			DatasetStub:       boolPtrDefault(nil, true),
			SearchIndexStub:   boolPtrDefault(nil, true),
			RoleTemplateStub:  boolPtrDefault(nil, true),
			PolicyHookStub:    boolPtrDefault(nil, true),
			ObservabilityStub: boolPtrDefault(nil, true),
			GenericUIStub:     boolPtrDefault(nil, true),
			CustomUIStub:      boolPtrDefault(nil, false),
		}
		return nil
	case "search-heavy", "masterdata-search":
		spec.StarterPack = "masterdata-search"
		spec.Features.Search = true
		spec.Features.UI = true
		spec.Features.CustomUI = false
		spec.Features.Tests = true
		spec.Features.Policy = true
		spec.Features.Observability = true
		spec.Features.Reporting = true
		spec.Scaffold = ScaffoldOptions{
			ObservabilityHelper: boolPtrDefault(nil, true),
			ReportingHelper:     boolPtrDefault(nil, true),
			ManifestTest:        boolPtrDefault(nil, true),
			RegistrationTest:    boolPtrDefault(nil, true),
		}
		spec.Manifest = ManifestOptions{
			ModelStub:         boolPtrDefault(nil, spec.Module.Kind == KindModel || spec.Module.Kind == KindHybrid),
			DatasetStub:       boolPtrDefault(nil, true),
			SearchIndexStub:   boolPtrDefault(nil, true),
			RoleTemplateStub:  boolPtrDefault(nil, true),
			PolicyHookStub:    boolPtrDefault(nil, true),
			ObservabilityStub: boolPtrDefault(nil, true),
			GenericUIStub:     boolPtrDefault(nil, true),
			CustomUIStub:      boolPtrDefault(nil, false),
		}
		return nil
	case "integration-first", "integration-adapter":
		spec.StarterPack = "integration-adapter"
		spec.Module.Kind = KindIntegration
		spec.Features.Search = false
		spec.Features.UI = true
		spec.Features.CustomUI = false
		spec.Features.Tests = true
		spec.Features.Policy = true
		spec.Features.Observability = true
		spec.Features.Reporting = true
		spec.Scaffold = ScaffoldOptions{
			ObservabilityHelper: boolPtrDefault(nil, true),
			ReportingHelper:     boolPtrDefault(nil, true),
			ManifestTest:        boolPtrDefault(nil, true),
			RegistrationTest:    boolPtrDefault(nil, true),
		}
		spec.Manifest = ManifestOptions{
			ModelStub:         boolPtrDefault(nil, false),
			DatasetStub:       boolPtrDefault(nil, true),
			SearchIndexStub:   boolPtrDefault(nil, false),
			RoleTemplateStub:  boolPtrDefault(nil, true),
			PolicyHookStub:    boolPtrDefault(nil, true),
			ObservabilityStub: boolPtrDefault(nil, true),
			GenericUIStub:     boolPtrDefault(nil, true),
			CustomUIStub:      boolPtrDefault(nil, false),
		}
		return nil
	default:
		return fmt.Errorf("unsupported profile %q", profile)
	}
}

func defaultStarterPack(kind Kind) string {
	switch kind {
	case KindDocument:
		return "document-workflow"
	case KindModel, KindHybrid:
		return "masterdata-search"
	case KindIntegration:
		return "integration-adapter"
	default:
		return "minimal"
	}
}

func rootOrDefault(root string) string {
	if strings.TrimSpace(root) == "" {
		return "."
	}
	return root
}
