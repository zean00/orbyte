package modulegen

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type PlannedFile struct {
	Path    string
	Content string
}

type Plan struct {
	Spec  Spec
	Files []PlannedFile
}

func PlanModule(root string, spec Spec) (Plan, error) {
	root = rootOrDefault(root)
	if err := ValidateSpec(root, spec); err != nil {
		return Plan{}, err
	}
	data := buildTemplateData(spec)
	moduleDir := filepath.Join(root, "internal", "modules", spec.Module.Key)
	files := []PlannedFile{}

	manifestContent, err := renderGoTemplate(manifestTemplate, data)
	if err != nil {
		return Plan{}, err
	}
	files = append(files, PlannedFile{Path: filepath.Join(moduleDir, "manifest.go"), Content: manifestContent})

	serviceContent, err := renderGoTemplate(serviceTemplate, data)
	if err != nil {
		return Plan{}, err
	}
	files = append(files, PlannedFile{Path: filepath.Join(moduleDir, "service.go"), Content: serviceContent})

	if data.HasObservabilityHelper {
		observabilityContent, err := renderGoTemplate(observabilityTemplate, data)
		if err != nil {
			return Plan{}, err
		}
		files = append(files, PlannedFile{Path: filepath.Join(moduleDir, "observability.go"), Content: observabilityContent})
	}

	if data.HasReportingHelper {
		reportingContent, err := renderGoTemplate(reportingTemplate, data)
		if err != nil {
			return Plan{}, err
		}
		files = append(files, PlannedFile{Path: filepath.Join(moduleDir, "reporting.go"), Content: reportingContent})
	}

	if data.HasCustomUIStub || data.HasCustomUI {
		var out bytes.Buffer
		if err := mustRenderText(bundleJSTemplate, data, &out); err != nil {
			return Plan{}, err
		}
		files = append(files, PlannedFile{Path: filepath.Join(moduleDir, "bundle.js"), Content: out.String()})
	}

	if data.HasManifestTest {
		testContent, err := renderGoTemplate(testTemplate, data)
		if err != nil {
			return Plan{}, err
		}
		files = append(files, PlannedFile{Path: filepath.Join(moduleDir, "service_test.go"), Content: testContent})
	}

	if data.HasRegistrationTest {
		registrationTestContent, err := renderGoTemplate(registrationTestTemplate, data)
		if err != nil {
			return Plan{}, err
		}
		files = append(files, PlannedFile{Path: filepath.Join(moduleDir, "registration_test.go"), Content: registrationTestContent})
	}

	registryContent, err := patchRegistry(root, data)
	if err != nil {
		return Plan{}, err
	}
	files = append(files, PlannedFile{Path: filepath.Join(root, "internal", "modules", "registry.go"), Content: registryContent})
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return Plan{Spec: spec, Files: files}, nil
}

func Scaffold(root string, spec Spec) (Plan, error) {
	plan, err := PlanModule(root, spec)
	if err != nil {
		return Plan{}, err
	}
	for _, file := range plan.Files {
		if err := os.MkdirAll(filepath.Dir(file.Path), 0o755); err != nil {
			return Plan{}, err
		}
		if err := os.WriteFile(file.Path, []byte(file.Content), 0o644); err != nil {
			return Plan{}, err
		}
	}
	return plan, nil
}

func patchRegistry(root string, data templateData) (string, error) {
	path := filepath.Join(root, "internal", "modules", "registry.go")
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	text := string(content)
	importLine := data.ImportAlias + ` "clinic/internal/modules/` + data.Spec.Module.Key + `"`
	if strings.Contains(text, importLine) {
		return "", fmt.Errorf("module %s already registered", data.Spec.Module.Key)
	}
	if !strings.Contains(text, "// modulegen:manifests") {
		return "", fmt.Errorf("registry marker not found in %s", path)
	}
	if strings.Contains(text, `import platformmodule "clinic/internal/platform/module"`) && !strings.Contains(text, "import (\n") {
		text = strings.Replace(text, `import platformmodule "clinic/internal/platform/module"`, "import (\n\tplatformmodule \"clinic/internal/platform/module\"\n\t// modulegen:imports\n)", 1)
	}
	if !strings.Contains(text, "// modulegen:imports") {
		return "", fmt.Errorf("registry import marker not found in %s", path)
	}
	text = strings.Replace(text, "// modulegen:imports", "\t"+importLine+"\n\t// modulegen:imports", 1)
	text = strings.Replace(text, "// modulegen:manifests", "\t\t"+data.ImportAlias+".Manifest(),\n\t\t// modulegen:manifests", 1)
	return text, nil
}

func mustRenderText(source string, data templateData, out *bytes.Buffer) error {
	tpl, err := parseTextTemplate(source)
	if err != nil {
		return err
	}
	return tpl.Execute(out, data)
}
