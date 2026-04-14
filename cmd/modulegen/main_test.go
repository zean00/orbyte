package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"orbyte/internal/modulegen"
	platformmodule "orbyte/internal/platform/module"
)

func TestParseOptionsAndHelpers(t *testing.T) {
	opts, err := parseOptions([]string{
		"-root", " ./workspace ",
		"-spec", "spec.yaml",
		"-profile", "minimal",
		"-json",
	})
	if err != nil {
		t.Fatalf("parseOptions failed: %v", err)
	}
	if opts.Root != "./workspace" {
		t.Fatalf("expected trimmed root, got %q", opts.Root)
	}
	if opts.SpecPath != "spec.yaml" || opts.Profile != "minimal" || !opts.JSON {
		t.Fatalf("unexpected options: %+v", opts)
	}
	if got := modulegenRoot("  "); got != "." {
		t.Fatalf("expected default root '.', got %q", got)
	}
	if got := firstNonEmpty(" ", " value "); got != "value" {
		t.Fatalf("expected first populated helper result, got %q", got)
	}
}

func TestToExplainSpecAndWriteSpecFile(t *testing.T) {
	spec := modulegen.Spec{
		Module: modulegen.ModuleIdentity{Key: "test_module", Kind: "domain"},
		StarterPack: "minimal",
		Dependencies: []string{"platform.core"},
		DependencyRequirements: []platformmodule.DependencyRequirement{{
			ModuleKey:    "platform.core",
			VersionRange: ">=1.0.0",
			Kind:         platformmodule.DependencyKindRequired,
		}},
	}
	rendered := toExplainSpec(spec)
	if rendered.Module.Key != "test_module" || len(rendered.DependencyRequirements) != 1 {
		t.Fatalf("unexpected explain spec: %+v", rendered)
	}

	root := t.TempDir()
	opts := modulegen.Options{Root: root}
	if err := writeSpecFile(opts, spec); err != nil {
		t.Fatalf("writeSpecFile failed: %v", err)
	}
	path := filepath.Join(root, "test_module.module.yaml")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read generated spec: %v", err)
	}
	if !strings.Contains(string(content), "module:") || !strings.Contains(string(content), "test_module") {
		t.Fatalf("unexpected written spec: %s", string(content))
	}
	if err := os.WriteFile(path, []byte("existing"), 0o644); err != nil {
		t.Fatalf("seed spec file: %v", err)
	}
	if err := writeSpecFile(opts, spec); err != nil {
		t.Fatalf("writeSpecFile on existing file failed: %v", err)
	}
	content, err = os.ReadFile(path)
	if err != nil {
		t.Fatalf("read existing spec: %v", err)
	}
	if string(content) != "existing" {
		t.Fatalf("expected existing file to remain untouched, got %s", string(content))
	}
}

func TestLintModulesJSONOutput(t *testing.T) {
	root := t.TempDir()
	opts := modulegen.Options{Root: root, JSON: true}
	originalStdout := os.Stdout
	outputFile, err := os.CreateTemp(t.TempDir(), "modulegen-lint-*.json")
	if err != nil {
		t.Fatalf("CreateTemp failed: %v", err)
	}
	defer outputFile.Close()
	os.Stdout = outputFile
	defer func() { os.Stdout = originalStdout }()

	if err := lintModules(opts); err != nil {
		t.Fatalf("lintModules failed: %v", err)
	}
	if err := outputFile.Close(); err != nil {
		t.Fatalf("close lint output file: %v", err)
	}
	output, err := os.ReadFile(outputFile.Name())
	if err != nil {
		t.Fatalf("read lint output: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(bytesTrimSpace(output), &payload); err != nil {
		t.Fatalf("expected json lint output, got %s err=%v", string(output), err)
	}
}

func bytesTrimSpace(input []byte) []byte {
	return []byte(strings.TrimSpace(string(input)))
}
