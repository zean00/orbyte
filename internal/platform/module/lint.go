package module

import (
	"fmt"
	"sort"
	"strings"

	"orbyte/internal/platform/shared"
)

const (
	KernelVersion = "1.0.0"

	CapabilityGenericUI       = "generic_ui"
	CapabilityWorkflowRuntime = "workflow_runtime"
	CapabilitySearchRuntime   = "search_runtime"
	CapabilityOfflineSync     = "offline_sync"
	CapabilityObservability   = "observability_ops"
	CapabilityMCP             = "mcp"
	CapabilityReporting       = "reporting"
)

func DefaultKernelCapabilities() map[string]bool {
	return map[string]bool{
		CapabilityGenericUI:       true,
		CapabilityWorkflowRuntime: true,
		CapabilitySearchRuntime:   true,
		CapabilityOfflineSync:     true,
		CapabilityObservability:   true,
		CapabilityMCP:             true,
		CapabilityReporting:       true,
	}
}

func (r LintReport) Valid() bool {
	for _, diagnostic := range r.Diagnostics {
		if diagnostic.Severity == SeverityError {
			return false
		}
	}
	return true
}

func (r LintReport) Error() error {
	for _, diagnostic := range r.Diagnostics {
		if diagnostic.Severity == SeverityError {
			switch diagnostic.Code {
			case "duplicate_contract", "duplicate_route", "missing_dependency", "kernel_capability_missing", "kernel_version_incompatible":
				return shared.Conflict(diagnostic.Message)
			default:
				return shared.Validation(diagnostic.Message)
			}
		}
	}
	return nil
}

func (s *Service) SetKernelVersion(version string) {
	version = strings.TrimSpace(version)
	if version == "" {
		version = KernelVersion
	}
	s.kernelVersion = version
}

func (s *Service) SetKernelCapabilities(capabilities map[string]bool) {
	s.kernelCapabilities = map[string]bool{}
	for key, enabled := range capabilities {
		s.kernelCapabilities[strings.TrimSpace(key)] = enabled
	}
}

func (s *Service) Lint(manifests []Manifest) LintReport {
	report := LintReport{}
	existing := map[string]Manifest{}
	for _, manifest := range manifests {
		if err := validateManifest(existing, manifest); err != nil {
			report.Diagnostics = append(report.Diagnostics, diagnosticFromError(manifest.Key, err))
		}
		report.Diagnostics = append(report.Diagnostics, lintOwnedContracts(manifest)...)
		report.Diagnostics = append(report.Diagnostics, lintReferences(manifest)...)
		report.Diagnostics = append(report.Diagnostics, s.KernelCompatibility(manifest)...)
		existing[manifest.Key] = manifest
	}
	sort.Slice(report.Diagnostics, func(i, j int) bool {
		if report.Diagnostics[i].ModuleKey == report.Diagnostics[j].ModuleKey {
			if report.Diagnostics[i].Severity == report.Diagnostics[j].Severity {
				if report.Diagnostics[i].Code == report.Diagnostics[j].Code {
					return report.Diagnostics[i].Path < report.Diagnostics[j].Path
				}
				return report.Diagnostics[i].Code < report.Diagnostics[j].Code
			}
			return report.Diagnostics[i].Severity < report.Diagnostics[j].Severity
		}
		return report.Diagnostics[i].ModuleKey < report.Diagnostics[j].ModuleKey
	})
	return report
}

func (s *Service) KernelCompatibility(manifest Manifest) []Diagnostic {
	diagnostics := make([]Diagnostic, 0)
	if strings.TrimSpace(manifest.KernelVersionRange) != "" {
		compatible, err := versionSatisfies(s.kernelVersion, manifest.KernelVersionRange)
		if err != nil {
			diagnostics = append(diagnostics, Diagnostic{
				Severity:  SeverityError,
				Code:      "kernel_version_range_invalid",
				Message:   fmt.Sprintf("module %q has invalid kernel_version_range %q", manifest.Key, manifest.KernelVersionRange),
				ModuleKey: manifest.Key,
				Path:      "kernel_version_range",
			})
		} else if !compatible {
			diagnostics = append(diagnostics, Diagnostic{
				Severity:  SeverityError,
				Code:      "kernel_version_incompatible",
				Message:   fmt.Sprintf("module %q requires kernel %s but current kernel is %s", manifest.Key, manifest.KernelVersionRange, s.kernelVersion),
				ModuleKey: manifest.Key,
				Path:      "kernel_version_range",
			})
		}
	}
	for _, capability := range manifest.RequiredCapabilities {
		capability = strings.TrimSpace(capability)
		if capability == "" {
			continue
		}
		if !s.kernelCapabilities[capability] {
			diagnostics = append(diagnostics, Diagnostic{
				Severity:  SeverityError,
				Code:      "kernel_capability_missing",
				Message:   fmt.Sprintf("module %q requires missing kernel capability %q", manifest.Key, capability),
				ModuleKey: manifest.Key,
				Path:      "required_capabilities",
			})
		}
	}
	return diagnostics
}

func lintOwnedContracts(manifest Manifest) []Diagnostic {
	diagnostics := make([]Diagnostic, 0)
	for _, def := range manifest.Documents {
		if !containsString(manifest.OwnedDocumentTypes, def.Type) {
			diagnostics = append(diagnostics, Diagnostic{
				Severity:  SeverityWarning,
				Code:      "owned_document_missing",
				Message:   fmt.Sprintf("document %q is defined but not listed in owned_document_types", def.Type),
				ModuleKey: manifest.Key,
				Path:      "owned_document_types",
			})
		}
	}
	for _, def := range manifest.Workflows {
		if !containsString(manifest.OwnedWorkflowKeys, def.Key) {
			diagnostics = append(diagnostics, Diagnostic{
				Severity:  SeverityWarning,
				Code:      "owned_workflow_missing",
				Message:   fmt.Sprintf("workflow %q is defined but not listed in owned_workflow_keys", def.Key),
				ModuleKey: manifest.Key,
				Path:      "owned_workflow_keys",
			})
		}
	}
	for _, permission := range manifest.Security.Permissions {
		if !containsString(manifest.OwnedPermissionKeys, permission.Key) {
			diagnostics = append(diagnostics, Diagnostic{
				Severity:  SeverityWarning,
				Code:      "owned_permission_missing",
				Message:   fmt.Sprintf("permission %q is defined but not listed in owned_permission_keys", permission.Key),
				ModuleKey: manifest.Key,
				Path:      "owned_permission_keys",
			})
		}
	}
	for _, projection := range manifest.Observability.Projections {
		if !containsString(manifest.OwnedProjectionKeys, projection.Key) {
			diagnostics = append(diagnostics, Diagnostic{
				Severity:  SeverityWarning,
				Code:      "owned_projection_missing",
				Message:   fmt.Sprintf("projection %q is defined but not listed in owned_projection_keys", projection.Key),
				ModuleKey: manifest.Key,
				Path:      "owned_projection_keys",
			})
		}
	}
	for _, item := range manifest.Templates {
		if !containsString(manifest.OwnedTemplateKeys, item.Key) {
			diagnostics = append(diagnostics, Diagnostic{
				Severity:  SeverityWarning,
				Code:      "owned_template_missing",
				Message:   fmt.Sprintf("template %q is defined but not listed in owned_template_keys", item.Key),
				ModuleKey: manifest.Key,
				Path:      "owned_template_keys",
			})
		}
	}
	return diagnostics
}

func lintReferences(manifest Manifest) []Diagnostic {
	diagnostics := make([]Diagnostic, 0)
	views := map[string]bool{}
	bundles := map[string]bool{}
	models := map[string]bool{}
	documents := map[string]bool{}
	indexes := map[string]bool{}
	projections := map[string]bool{}
	workflows := map[string]bool{}
	for _, item := range manifest.Frontend.Views {
		views[item.Key] = true
	}
	for _, item := range manifest.Bundles {
		bundles[item.Key] = true
	}
	for _, item := range manifest.Models {
		models[item.Key] = true
	}
	for _, item := range manifest.Documents {
		documents[item.Type] = true
	}
	for _, item := range manifest.SearchIndexes {
		indexes[item.Key] = true
	}
	for _, item := range manifest.Observability.Projections {
		projections[item.Key] = true
	}
	for _, item := range manifest.Workflows {
		workflows[item.Key] = true
	}

	for _, dashboard := range manifest.Observability.Dashboards {
		if dashboard.ViewKey != "" && !views[dashboard.ViewKey] {
			diagnostics = append(diagnostics, Diagnostic{Severity: SeverityWarning, Code: "missing_view_reference", Message: fmt.Sprintf("dashboard %q references missing view %q", dashboard.Key, dashboard.ViewKey), ModuleKey: manifest.Key, Path: "observability.dashboards"})
		}
	}
	for _, report := range manifest.Observability.Reports {
		if report.Dataset != "" && !containsDataset(manifest.Datasets, report.Dataset) {
			diagnostics = append(diagnostics, Diagnostic{Severity: SeverityWarning, Code: "missing_dataset_reference", Message: fmt.Sprintf("report %q references missing dataset %q", report.Key, report.Dataset), ModuleKey: manifest.Key, Path: "observability.reports"})
		}
	}
	for _, api := range manifest.SelfService.APIs {
		if api.DocumentType != "" && !documents[api.DocumentType] {
			diagnostics = append(diagnostics, Diagnostic{Severity: SeverityWarning, Code: "missing_document_reference", Message: fmt.Sprintf("self-service api %q references missing document type %q", api.Key, api.DocumentType), ModuleKey: manifest.Key, Path: "self_service.apis"})
		}
		if api.ModelKey != "" && !models[api.ModelKey] {
			diagnostics = append(diagnostics, Diagnostic{Severity: SeverityWarning, Code: "missing_model_reference", Message: fmt.Sprintf("self-service api %q references missing model %q", api.Key, api.ModelKey), ModuleKey: manifest.Key, Path: "self_service.apis"})
		}
		if api.FlowKey != "" && !workflows[api.FlowKey] {
			diagnostics = append(diagnostics, Diagnostic{Severity: SeverityWarning, Code: "missing_flow_reference", Message: fmt.Sprintf("self-service api %q references flow/workflow %q that is not owned by the module", api.Key, api.FlowKey), ModuleKey: manifest.Key, Path: "self_service.apis"})
		}
	}
	for _, item := range manifest.Offline.Documents {
		if !documents[item.Type] {
			diagnostics = append(diagnostics, Diagnostic{Severity: SeverityWarning, Code: "missing_document_reference", Message: fmt.Sprintf("offline document %q references missing document type %q", item.Title, item.Type), ModuleKey: manifest.Key, Path: "offline.documents"})
		}
	}
	for _, item := range manifest.Offline.Models {
		if !models[item.ModelKey] {
			diagnostics = append(diagnostics, Diagnostic{Severity: SeverityWarning, Code: "missing_model_reference", Message: fmt.Sprintf("offline model %q references missing model %q", item.Title, item.ModelKey), ModuleKey: manifest.Key, Path: "offline.models"})
		}
	}
	for _, item := range manifest.Offline.Projections {
		if !indexes[item.IndexKey] && !projections[item.IndexKey] {
			diagnostics = append(diagnostics, Diagnostic{Severity: SeverityWarning, Code: "missing_projection_reference", Message: fmt.Sprintf("offline projection %q references missing index/projection %q", item.Title, item.IndexKey), ModuleKey: manifest.Key, Path: "offline.projections"})
		}
	}
	return diagnostics
}

func diagnosticFromError(moduleKey string, err error) Diagnostic {
	message := err.Error()
	code := "manifest_invalid"
	switch {
	case strings.Contains(message, "already registered"):
		code = "duplicate_contract"
	case strings.Contains(message, "route path"):
		code = "duplicate_route"
	case strings.Contains(message, "required"):
		code = "missing_required"
	}
	return Diagnostic{Severity: SeverityError, Code: code, Message: message, ModuleKey: moduleKey}
}

func containsDataset(items []DatasetDefinition, key string) bool {
	for _, item := range items {
		if item.Key == key {
			return true
		}
	}
	return false
}
