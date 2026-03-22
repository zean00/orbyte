package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"orbyte/internal/modulegen"
	"orbyte/internal/modules"
	"orbyte/internal/platform/app"
	platformmodule "orbyte/internal/platform/module"

	"go.yaml.in/yaml/v3"
)

type explainDependencyRequirement struct {
	ModuleKey    string `yaml:"module_key"`
	VersionRange string `yaml:"version_range"`
	Kind         string `yaml:"kind"`
}

type explainSpec struct {
	Module                 modulegen.ModuleIdentity       `yaml:"module"`
	StarterPack            string                         `yaml:"starter_pack,omitempty"`
	Features               modulegen.FeatureOptions       `yaml:"features"`
	Scaffold               modulegen.ScaffoldOptions      `yaml:"scaffold"`
	Manifest               modulegen.ManifestOptions      `yaml:"manifest"`
	Dependencies           []string                       `yaml:"dependencies,omitempty"`
	DependencyRequirements []explainDependencyRequirement `yaml:"dependency_requirements,omitempty"`
	Document               modulegen.DocumentOptions      `yaml:"document,omitempty"`
	Model                  modulegen.ModelOptions         `yaml:"model,omitempty"`
}

func main() {
	if len(os.Args) < 3 || os.Args[1] != "module" {
		fatalf("usage: modulegen module <init|plan|validate|lint|explain> [flags]")
	}
	command := os.Args[2]
	opts, err := parseOptions(os.Args[3:])
	if err != nil {
		fatalf("%v", err)
	}
	if command == "lint" {
		if err := lintModules(opts); err != nil {
			fatalf("%v", err)
		}
		return
	}
	spec, err := modulegen.LoadSpec(opts.SpecPath)
	if err != nil {
		fatalf("load spec: %v", err)
	}
	resolved, err := modulegen.ResolveSpec(spec, opts)
	if err != nil {
		fatalf("resolve spec: %v", err)
	}
	switch command {
	case "explain":
		rendered, err := yaml.Marshal(toExplainSpec(resolved))
		if err != nil {
			fatalf("render resolved spec: %v", err)
		}
		fmt.Print(string(rendered))
	case "validate":
		fmt.Printf("module %s is valid\n", resolved.Module.Key)
	case "plan":
		plan, err := modulegen.PlanModule(opts.Root, resolved)
		if err != nil {
			fatalf("plan module: %v", err)
		}
		if opts.JSON {
			rendered, err := json.MarshalIndent(plan, "", "  ")
			if err != nil {
				fatalf("render plan json: %v", err)
			}
			fmt.Println(string(rendered))
			return
		}
		fmt.Printf("module: %s (%s)\n", plan.Spec.Module.Key, plan.Spec.Module.Kind)
		for _, file := range plan.Files {
			rel, _ := filepath.Rel(modulegenRoot(opts.Root), file.Path)
			fmt.Printf("- %s\n", rel)
		}
	case "init":
		if err := writeSpecFile(opts, resolved); err != nil {
			fatalf("write spec: %v", err)
		}
		plan, err := modulegen.Scaffold(opts.Root, resolved)
		if err != nil {
			fatalf("scaffold module: %v", err)
		}
		fmt.Printf("generated module %s\n", plan.Spec.Module.Key)
		for _, file := range plan.Files {
			rel, _ := filepath.Rel(modulegenRoot(opts.Root), file.Path)
			fmt.Printf("- %s\n", rel)
		}
	default:
		fatalf("unsupported command %q", command)
	}
}

func parseOptions(args []string) (modulegen.Options, error) {
	var opts modulegen.Options
	fs := flag.NewFlagSet("modulegen", flag.ContinueOnError)
	fs.StringVar(&opts.Root, "root", ".", "repository root")
	fs.StringVar(&opts.SpecPath, "spec", "", "path to yaml spec")
	fs.StringVar(&opts.Profile, "profile", "", "generator profile: minimal|backoffice|search-heavy|integration-first")
	fs.StringVar(&opts.StarterPack, "starter-pack", "", "starter pack: minimal|document-workflow|masterdata-search|integration-adapter")
	fs.StringVar(&opts.Key, "key", "", "module key")
	fs.StringVar(&opts.Name, "name", "", "module name")
	fs.StringVar(&opts.Version, "version", "", "module version")
	fs.StringVar(&opts.DomainFamily, "domain-family", "", "module domain family")
	fs.StringVar(&opts.Kind, "kind", "", "module kind")
	fs.Var(&opts.WithSearch, "with-search", "enable search features")
	fs.Var(&opts.WithUI, "with-ui", "enable generic ui features")
	fs.Var(&opts.WithCustomUI, "with-custom-ui", "enable custom ui bundle stub")
	fs.Var(&opts.WithTests, "with-tests", "generate tests")
	fs.Var(&opts.WithPolicy, "with-policy", "generate policy hook stub")
	fs.Var(&opts.WithObservability, "with-observability", "generate observability stubs")
	fs.Var(&opts.WithReporting, "with-reporting", "generate reporting stubs")
	fs.Var(&opts.WithObservabilityHelper, "with-observability-helper", "generate observability helper file")
	fs.Var(&opts.WithReportingHelper, "with-reporting-helper", "generate reporting helper file")
	fs.Var(&opts.WithManifestTest, "with-manifest-test", "generate manifest test file")
	fs.Var(&opts.WithRegistrationTest, "with-registration-test", "generate registration test file")
	fs.Var(&opts.WithModelStub, "with-model-stub", "generate model definition stub in the manifest")
	fs.Var(&opts.WithDatasetStub, "with-dataset-stub", "generate dataset stub in the manifest")
	fs.Var(&opts.WithSearchIndexStub, "with-search-index-stub", "generate search index stub in the manifest")
	fs.Var(&opts.WithRoleTemplateStub, "with-role-template-stub", "generate role template stub in the manifest")
	fs.Var(&opts.WithPolicyHookStub, "with-policy-hook-stub", "generate policy hook stub in the manifest")
	fs.Var(&opts.WithObservabilityStub, "with-observability-stub", "generate observability manifest stub")
	fs.Var(&opts.WithGenericUIStub, "with-generic-ui-stub", "generate generic UI manifest stubs")
	fs.Var(&opts.WithCustomUIStub, "with-custom-ui-stub", "generate custom UI action and bundle stubs in the manifest")
	fs.BoolVar(&opts.JSON, "json", false, "emit machine-readable json output where supported")
	if err := fs.Parse(args); err != nil {
		return modulegen.Options{}, err
	}
	opts.Root = modulegenRoot(opts.Root)
	return opts, nil
}

func modulegenRoot(root string) string {
	if strings.TrimSpace(root) == "" {
		return "."
	}
	return strings.TrimSpace(root)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func toExplainSpec(spec modulegen.Spec) explainSpec {
	out := explainSpec{
		Module:       spec.Module,
		StarterPack:  spec.StarterPack,
		Features:     spec.Features,
		Scaffold:     spec.Scaffold,
		Manifest:     spec.Manifest,
		Dependencies: append([]string(nil), spec.Dependencies...),
		Document:     spec.Document,
		Model:        spec.Model,
	}
	if len(spec.DependencyRequirements) > 0 {
		out.DependencyRequirements = make([]explainDependencyRequirement, 0, len(spec.DependencyRequirements))
		for _, dep := range spec.DependencyRequirements {
			out.DependencyRequirements = append(out.DependencyRequirements, explainDependencyRequirement{
				ModuleKey:    dep.ModuleKey,
				VersionRange: dep.VersionRange,
				Kind:         string(dep.Kind),
			})
		}
	}
	return out
}

func lintModules(opts modulegen.Options) error {
	manifests, err := modules.ForProfile(firstNonEmpty(opts.Profile, modules.ProfileAll))
	if err != nil {
		return err
	}
	manifests = append(app.BuiltInModuleManifests(), manifests...)
	svc := platformmodule.NewService()
	report := svc.Lint(manifests)
	if opts.JSON {
		rendered, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(rendered))
	} else {
		if len(report.Diagnostics) == 0 {
			fmt.Println("lint clean")
		}
		for _, diagnostic := range report.Diagnostics {
			fmt.Printf("%s %s %s: %s\n", diagnostic.Severity, diagnostic.ModuleKey, diagnostic.Code, diagnostic.Message)
		}
	}
	if !report.Valid() {
		return report.Error()
	}
	return nil
}

func writeSpecFile(opts modulegen.Options, spec modulegen.Spec) error {
	path := strings.TrimSpace(opts.SpecPath)
	if path == "" {
		path = filepath.Join(modulegenRoot(opts.Root), spec.Module.Key+".module.yaml")
	}
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	rendered, err := yaml.Marshal(toExplainSpec(spec))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, rendered, 0o644)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
