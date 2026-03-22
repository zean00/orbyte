package main

import (
	"testing"

	"orbyte/internal/modulegen"
	"orbyte/internal/platform/module"
)

func TestParseOptionsAndModulegenRoot(t *testing.T) {
	opts, err := parseOptions([]string{
		"-root", " /tmp/orbyte ",
		"-spec", "specs/test.yaml",
		"-profile", "minimal",
		"-starter-pack", "document-workflow",
		"-json",
		"-key", "sample",
		"-name", "Sample",
		"-version", "1.0.0",
		"-domain-family", "clinic",
		"-kind", "document",
	})
	if err != nil {
		t.Fatalf("parseOptions failed: %v", err)
	}
	if opts.Root != "/tmp/orbyte" || opts.SpecPath != "specs/test.yaml" || opts.Profile != "minimal" || opts.StarterPack != "document-workflow" || !opts.JSON || opts.Key != "sample" || opts.Kind != "document" {
		t.Fatalf("unexpected parsed options: %+v", opts)
	}

	if got := modulegenRoot("  "); got != "." {
		t.Fatalf("expected default root, got %q", got)
	}
}

func TestToExplainSpec(t *testing.T) {
	spec := modulegen.Spec{
		Module: modulegen.ModuleIdentity{
			Key:     "sample",
			Name:    "Sample",
			Version: "1.0.0",
			Kind:    "document",
		},
		Dependencies: []string{"base.module"},
		DependencyRequirements: []module.DependencyRequirement{
			{ModuleKey: "base.module", VersionRange: ">=1.0.0", Kind: module.DependencyKindRequired},
		},
	}
	explained := toExplainSpec(spec)
	if explained.Module.Key != "sample" || len(explained.Dependencies) != 1 || len(explained.DependencyRequirements) != 1 {
		t.Fatalf("unexpected explain spec: %+v", explained)
	}
	if explained.StarterPack != "" {
		t.Fatalf("expected empty starter pack when spec does not define one, got %+v", explained)
	}
	if explained.DependencyRequirements[0].Kind != string(module.DependencyKindRequired) {
		t.Fatalf("unexpected dependency kind: %+v", explained.DependencyRequirements[0])
	}
}
