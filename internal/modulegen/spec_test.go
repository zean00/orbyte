package modulegen

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOptionalBoolSetAndString(t *testing.T) {
	var flag optionalBool
	if err := flag.Set("yes"); err != nil {
		t.Fatalf("set true failed: %v", err)
	}
	if !flag.set || !flag.value || flag.String() != "true" {
		t.Fatalf("unexpected true flag state: %+v", flag)
	}
	if err := flag.Set("off"); err != nil {
		t.Fatalf("set false failed: %v", err)
	}
	if !flag.set || flag.value || flag.String() != "false" {
		t.Fatalf("unexpected false flag state: %+v", flag)
	}
	if err := flag.Set("maybe"); err == nil {
		t.Fatal("expected invalid boolean to fail")
	}
}

func TestLoadSpec(t *testing.T) {
	if spec, err := LoadSpec(""); err != nil || spec.Module.Key != "" {
		t.Fatalf("expected empty spec for empty path, got spec=%+v err=%v", spec, err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "spec.yaml")
	content := []byte("module:\n  key: sample\n  name: Sample\n  version: 1.0.0\n  domain_family: clinic\n  kind: document\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write spec failed: %v", err)
	}

	spec, err := LoadSpec(path)
	if err != nil {
		t.Fatalf("LoadSpec failed: %v", err)
	}
	if spec.Module.Key != "sample" || spec.Module.Kind != KindDocument {
		t.Fatalf("unexpected loaded spec: %+v", spec)
	}
}
