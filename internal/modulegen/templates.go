package modulegen

import (
	"bytes"
	"fmt"
	"go/format"
	"strconv"
	"strings"
	"text/template"
)

type templateData struct {
	Spec                   Spec
	PackageName            string
	ImportAlias            string
	ServiceName            string
	HasDocument            bool
	HasModel               bool
	HasSearch              bool
	HasUI                  bool
	HasCustomUI            bool
	HasPolicy              bool
	HasObservability       bool
	HasReporting           bool
	HasTests               bool
	HasModelStub           bool
	HasDatasetStub         bool
	HasSearchIndexStub     bool
	HasRoleTemplateStub    bool
	HasPolicyHookStub      bool
	HasObservabilityStub   bool
	HasGenericUIStub       bool
	HasCustomUIStub        bool
	HasObservabilityHelper bool
	HasReportingHelper     bool
	HasManifestTest        bool
	HasRegistrationTest    bool
	DocumentPermissionBase string
	ModelPermissionBase    string
	RouteBase              string
	ModelFieldLiterals     []string
	ModelPrimaryFieldKey   string
	ModelStatusFieldKey    string
	HasModelStatusField    bool
}

func buildTemplateData(spec Spec) templateData {
	pkg := spec.Module.Key
	routeBase := "/" + strings.ReplaceAll(spec.Module.Key, "_", "-")
	modelFields := renderModelFieldLiterals(spec.Model.Fields)
	primaryField := firstModelFieldKey(spec.Model.Fields, "name")
	statusField := firstModelFieldKey(spec.Model.Fields, "status")
	return templateData{
		Spec:                   spec,
		PackageName:            pkg,
		ImportAlias:            pkg + "module",
		ServiceName:            exportedName(spec.Module.Key) + "Service",
		HasDocument:            spec.Module.Kind == KindDocument || spec.Module.Kind == KindHybrid,
		HasModel:               spec.Module.Kind == KindModel || spec.Module.Kind == KindHybrid,
		HasSearch:              spec.Features.Search,
		HasUI:                  spec.Features.UI,
		HasCustomUI:            spec.Features.CustomUI,
		HasPolicy:              spec.Features.Policy,
		HasObservability:       spec.Features.Observability,
		HasReporting:           spec.Features.Reporting,
		HasTests:               spec.Features.Tests,
		HasModelStub:           spec.Manifest.ModelStub != nil && *spec.Manifest.ModelStub,
		HasDatasetStub:         spec.Manifest.DatasetStub != nil && *spec.Manifest.DatasetStub,
		HasSearchIndexStub:     spec.Manifest.SearchIndexStub != nil && *spec.Manifest.SearchIndexStub,
		HasRoleTemplateStub:    spec.Manifest.RoleTemplateStub != nil && *spec.Manifest.RoleTemplateStub,
		HasPolicyHookStub:      spec.Manifest.PolicyHookStub != nil && *spec.Manifest.PolicyHookStub,
		HasObservabilityStub:   spec.Manifest.ObservabilityStub != nil && *spec.Manifest.ObservabilityStub,
		HasGenericUIStub:       spec.Manifest.GenericUIStub != nil && *spec.Manifest.GenericUIStub,
		HasCustomUIStub:        spec.Manifest.CustomUIStub != nil && *spec.Manifest.CustomUIStub,
		HasObservabilityHelper: spec.Scaffold.ObservabilityHelper != nil && *spec.Scaffold.ObservabilityHelper,
		HasReportingHelper:     spec.Scaffold.ReportingHelper != nil && *spec.Scaffold.ReportingHelper,
		HasManifestTest:        spec.Scaffold.ManifestTest != nil && *spec.Scaffold.ManifestTest,
		HasRegistrationTest:    spec.Scaffold.RegistrationTest != nil && *spec.Scaffold.RegistrationTest,
		DocumentPermissionBase: spec.Module.Key + ".document",
		ModelPermissionBase:    spec.Module.Key,
		RouteBase:              routeBase,
		ModelFieldLiterals:     modelFields,
		ModelPrimaryFieldKey:   primaryField,
		ModelStatusFieldKey:    statusField,
		HasModelStatusField:    statusField != "",
	}
}

func renderModelFieldLiterals(fields []ModelFieldOptions) []string {
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		parts := []string{
			`Key: ` + strconv.Quote(field.Key),
			`Label: ` + strconv.Quote(firstNonEmpty(field.Label, exportedName(field.Key))),
			`Type: ` + strconv.Quote(field.Type),
		}
		if field.Required {
			parts = append(parts, "Required: true")
		}
		if field.ReadOnly {
			parts = append(parts, "ReadOnly: true")
		}
		if field.Indexed {
			parts = append(parts, "Indexed: true")
		}
		if field.Sensitive {
			parts = append(parts, "Sensitive: true")
		}
		if strings.TrimSpace(field.SecurityClass) != "" {
			parts = append(parts, `SecurityClass: `+strconv.Quote(strings.TrimSpace(field.SecurityClass)))
		}
		if strings.TrimSpace(field.DefaultMask) != "" {
			parts = append(parts, `DefaultMask: `+strconv.Quote(strings.TrimSpace(field.DefaultMask)))
		}
		if field.SearchVisible != nil {
			parts = append(parts, fmt.Sprintf("SearchVisible: boolPtr(%t)", *field.SearchVisible))
		}
		if field.ExportVisible != nil {
			parts = append(parts, fmt.Sprintf("ExportVisible: boolPtr(%t)", *field.ExportVisible))
		}
		if strings.TrimSpace(field.ReadPermissionKey) != "" {
			parts = append(parts, `ReadPermissionKey: `+strconv.Quote(strings.TrimSpace(field.ReadPermissionKey)))
		}
		if strings.TrimSpace(field.WritePermissionKey) != "" {
			parts = append(parts, `WritePermissionKey: `+strconv.Quote(strings.TrimSpace(field.WritePermissionKey)))
		}
		if field.DefaultValue != nil {
			parts = append(parts, "DefaultValue: "+renderLiteral(field.DefaultValue))
		}
		if strings.TrimSpace(field.DefaultRuleKey) != "" {
			parts = append(parts, `DefaultRuleKey: `+strconv.Quote(strings.TrimSpace(field.DefaultRuleKey)))
		}
		if strings.TrimSpace(field.ComputeRuleKey) != "" {
			parts = append(parts, `ComputeRuleKey: `+strconv.Quote(strings.TrimSpace(field.ComputeRuleKey)))
		}
		if len(field.ConstraintRuleKeys) > 0 {
			keys := make([]string, 0, len(field.ConstraintRuleKeys))
			for _, key := range field.ConstraintRuleKeys {
				keys = append(keys, strconv.Quote(strings.TrimSpace(key)))
			}
			parts = append(parts, "ConstraintRuleKeys: []string{"+strings.Join(keys, ", ")+"}")
		}
		out = append(out, "{"+strings.Join(parts, ", ")+"}")
	}
	return out
}

func firstModelFieldKey(fields []ModelFieldOptions, preferred string) string {
	for _, field := range fields {
		if field.Key == preferred {
			return field.Key
		}
	}
	if len(fields) == 0 {
		return ""
	}
	return fields[0].Key
}

func renderLiteral(value any) string {
	switch typed := value.(type) {
	case string:
		return strconv.Quote(typed)
	case bool:
		if typed {
			return "true"
		}
		return "false"
	case int:
		return strconv.Itoa(typed)
	case int64:
		return fmt.Sprintf("%d", typed)
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(typed), 'f', -1, 32)
	default:
		return strconv.Quote(fmt.Sprintf("%v", typed))
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func exportedName(value string) string {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == '_' || r == '-' || r == ' '
	})
	if len(parts) == 0 {
		return "Module"
	}
	var b strings.Builder
	for _, part := range parts {
		if part == "" {
			continue
		}
		b.WriteString(strings.ToUpper(part[:1]))
		if len(part) > 1 {
			b.WriteString(part[1:])
		}
	}
	if b.Len() == 0 {
		return "Module"
	}
	return b.String()
}

func renderGoTemplate(source string, data templateData) (string, error) {
	tpl, err := parseTextTemplate(source)
	if err != nil {
		return "", err
	}
	var out bytes.Buffer
	if err := tpl.Execute(&out, data); err != nil {
		return "", err
	}
	formatted, err := format.Source(out.Bytes())
	if err != nil {
		return out.String(), nil
	}
	return string(formatted), nil
}

func parseTextTemplate(source string) (*template.Template, error) {
	return template.New("text").Funcs(template.FuncMap{
		"exportedName": exportedName,
	}).Parse(source)
}

const manifestTemplate = `package {{.PackageName}}

import (
	{{- if .HasCustomUIStub }}
	_ "embed"
	{{- end }}
	"clinic/internal/platform/module"
	{{- if .HasModelStub }}
	"clinic/internal/platform/model"
	{{- end }}
	{{- if .HasSearchIndexStub }}
	"clinic/internal/platform/search"
	{{- end }}
)

{{- if .HasCustomUIStub }}
//go:embed bundle.js
var customBundleScript string
{{ end }}

func Manifest() module.Manifest {
	return module.Manifest{
		Key:          "{{.Spec.Module.Key}}",
		Name:         "{{.Spec.Module.Name}}",
		Version:      "{{.Spec.Module.Version}}",
		DomainFamily: "{{.Spec.Module.DomainFamily}}",
		DependencyRequirements: []module.DependencyRequirement{
			{{- range .Spec.DependencyRequirements }}
			{ModuleKey: "{{.ModuleKey}}", VersionRange: "{{.VersionRange}}", Kind: module.DependencyKind{{ if eq .Kind "optional" }}Optional{{ else if eq .Kind "ui_extension" }}UIExtension{{ else if eq .Kind "integration" }}Integration{{ else }}Required{{ end }}},
			{{- end }}
		},
		{{- if .HasDocument }}
		OwnedDocumentTypes: []string{"{{.Spec.Document.Type}}"},
		{{- end }}
		{{- if .HasModelStub }}
		Models: []model.Definition{
			{
				Key:                 "{{.Spec.Model.Key}}",
				DisplayName:         "{{.Spec.Model.DisplayName}}",
				OwnerModuleKey:      "{{.Spec.Module.Key}}",
				Version:             "v1",
				CreatePermissionKey: "{{.ModelPermissionBase}}.create",
				ListPermissionKey:   "{{.ModelPermissionBase}}.list",
				ReadPermissionKey:   "{{.ModelPermissionBase}}.read",
				UpdatePermissionKey: "{{.ModelPermissionBase}}.update",
				DefaultSort:         "{{.ModelPrimaryFieldKey}}",
				Fields: []model.FieldDefinition{
					{{- range .ModelFieldLiterals }}
					{{.}},
					{{- end }}
				},
			},
		},
			{{- if .HasDatasetStub }}
			Datasets: []module.DatasetDefinition{
				{
				Key:        "{{.Spec.Module.Key}}.summary",
				Title:      "{{.Spec.Module.Name}} Summary",
				SourceKind: "model",
				ModelKey:   "{{.Spec.Model.Key}}",
				Dimensions: []module.DatasetDimension{
					{Key: "{{if .HasModelStatusField}}by_{{.ModelStatusFieldKey}}{{else}}by_{{.ModelPrimaryFieldKey}}{{end}}", Label: "{{if .HasModelStatusField}}By Status{{else}}By {{exportedName .ModelPrimaryFieldKey}}{{end}}", Path: "{{if .HasModelStatusField}}{{.ModelStatusFieldKey}}{{else}}{{.ModelPrimaryFieldKey}}{{end}}"},
				},
				Measures: []module.DatasetMeasure{
					{Key: "total", Label: "Total", Kind: "count"},
				},
				},
			},
			{{- end }}
			{{- end }}
			{{- if .HasSearchIndexStub }}
			SearchIndexes: []search.IndexDefinition{
			{
				Key:                 "{{.Spec.Module.Key}}.search",
				Title:               "{{.Spec.Module.Name}} Search",
				SourceKind:          "{{if .HasModel}}model{{else}}document{{end}}",
				{{- if .HasModel }}
				ModelKey:            "{{.Spec.Model.Key}}",
				{{- else if .HasDocument }}
				DocumentType:        "{{.Spec.Document.Type}}",
				{{- end }}
				ViewKey:             "{{.Spec.Module.Key}}.list",
				Modes:               []string{"keyword", "vector", "hybrid"},
				OrganizationSplit:   true,
				RequiredPermissions: []string{"{{if .HasModel}}{{.ModelPermissionBase}}.list{{else}}{{.DocumentPermissionBase}}.list{{end}}"},
				QueryFilterFields:   []string{"{{if .HasModelStatusField}}{{.ModelStatusFieldKey}}{{else}}{{.ModelPrimaryFieldKey}}{{end}}"},
				QuerySortFields:     []string{"{{if .HasModel}}{{.ModelPrimaryFieldKey}}{{else}}name{{end}}", "updated_at"},
				Fields: []search.IndexFieldDefinition{
					{Key: "name", Path: "{{if .HasModel}}{{.ModelPrimaryFieldKey}}{{else}}body.payload.title{{end}}", Type: "string", Searchable: true, Sort: true},
					{Key: "{{if .HasModelStatusField}}{{.ModelStatusFieldKey}}{{else}}status{{end}}", Path: "{{if .HasModel}}{{if .HasModelStatusField}}{{.ModelStatusFieldKey}}{{else}}{{.ModelPrimaryFieldKey}}{{end}}{{else}}header.status{{end}}", Type: "string", Facet: true, Sort: true},
				},
				VectorFields: []search.VectorFieldDefinition{
					{
						Key: "semantic", SourcePaths: []string{"{{if .HasModel}}{{.ModelPrimaryFieldKey}}{{else}}body.payload.title{{end}}"}, EmbeddingMode: "external", Dimensions: 8, DistanceMetric: "cosine",
					},
				},
			},
		},
		{{- end }}
		Security: module.SecurityDefinition{
			Permissions: []module.PermissionDefinition{
				{{- if .HasModel }}
				{Key: "{{.ModelPermissionBase}}.create", Action: "create", Resource: "{{.Spec.Model.Key}}", DisplayName: "Create {{.Spec.Module.Name}}"},
				{Key: "{{.ModelPermissionBase}}.list", Action: "list", Resource: "{{.Spec.Model.Key}}", DisplayName: "List {{.Spec.Module.Name}}"},
				{Key: "{{.ModelPermissionBase}}.read", Action: "read", Resource: "{{.Spec.Model.Key}}", DisplayName: "Read {{.Spec.Module.Name}}"},
				{Key: "{{.ModelPermissionBase}}.update", Action: "update", Resource: "{{.Spec.Model.Key}}", DisplayName: "Update {{.Spec.Module.Name}}"},
				{{- end }}
				{{- if .HasDocument }}
				{Key: "{{.DocumentPermissionBase}}.create", Action: "create", Resource: "{{.Spec.Document.Type}}", DisplayName: "Create {{.Spec.Module.Name}} Documents"},
				{Key: "{{.DocumentPermissionBase}}.list", Action: "list", Resource: "{{.Spec.Document.Type}}", DisplayName: "List {{.Spec.Module.Name}} Documents"},
				{Key: "{{.DocumentPermissionBase}}.read", Action: "read", Resource: "{{.Spec.Document.Type}}", DisplayName: "Read {{.Spec.Module.Name}} Documents"},
				{Key: "{{.DocumentPermissionBase}}.update", Action: "update", Resource: "{{.Spec.Document.Type}}", DisplayName: "Update {{.Spec.Module.Name}} Documents"},
				{{- end }}
			},
			{{- if .HasRoleTemplateStub }}
			RoleTemplates: []module.RoleTemplateDefinition{
				{
					Key:           "{{.Spec.Module.Key}}_manager",
					Name:          "{{.Spec.Module.Name}} Manager",
					AllowedScopes: []string{"deployment", "location"},
					PermissionKeys: []string{
						{{- if .HasModel }}
						"{{.ModelPermissionBase}}.create", "{{.ModelPermissionBase}}.list", "{{.ModelPermissionBase}}.read", "{{.ModelPermissionBase}}.update",
						{{- end }}
						{{- if .HasDocument }}
						"{{.DocumentPermissionBase}}.create", "{{.DocumentPermissionBase}}.list", "{{.DocumentPermissionBase}}.read", "{{.DocumentPermissionBase}}.update",
						{{- end }}
					},
				},
			},
			{{- end }}
			{{- if .HasPolicyHookStub }}
			PolicyHooks: []module.PolicyHookDefinition{
				{Key: "{{.Spec.Module.Key}}.access", Kind: "access", Target: "{{.Spec.Module.Key}}", InputContractKey: "{{.Spec.Module.Key}}.access.v1", OutputContractKey: "decision.v1", Description: "Module access policy hook."},
			},
			{{- end }}
		},
		{{- if .HasObservabilityStub }}
		Observability: module.ObservabilityDefinition{
			Metrics: []module.MetricDefinition{
				{Key: "{{.Spec.Module.Key}}.requests.total", Type: "counter", Labels: []string{"action", "outcome"}, Description: "Module request totals"},
			},
			LogEvents: []module.LogEventDefinition{
				{Key: "{{.Spec.Module.Key}}.event", Category: "{{.Spec.Module.Key}}", Severity: "info", RequiredFields: []string{"actor_id", "module_key"}},
			},
			DomainEvents: []module.DomainEventDefinition{
				{Type: "{{.Spec.Module.Key}}.changed", Role: "producer", CorrelationRequired: true},
			},
		},
		{{- end }}
		{{- if .HasGenericUIStub }}
		Frontend: module.FrontendDefinition{
			Menus: []module.MenuDefinition{
				{
				Key:                 "{{.Spec.Module.Key}}",
				Label:               "{{.Spec.Module.Name}}",
				ActionKey:           "{{.Spec.Module.Key}}.list",
				Order:               20,
				RequiredPermissions: []string{"{{if .HasModel}}{{.ModelPermissionBase}}.list{{else}}{{.DocumentPermissionBase}}.list{{end}}"},
				},
			},
			Actions: []module.ActionDefinition{
				{Key: "{{.Spec.Module.Key}}.list", Label: "{{.Spec.Module.Name}}", Kind: "navigate", RoutePath: "{{.RouteBase}}", ViewKey: "{{.Spec.Module.Key}}.list", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"{{if .HasModel}}{{.ModelPermissionBase}}.list{{else}}{{.DocumentPermissionBase}}.list{{end}}"}},
				{Key: "{{.Spec.Module.Key}}.detail", Label: "{{.Spec.Module.Name}} Detail", Kind: "navigate", RoutePath: "{{.RouteBase}}/detail", ViewKey: "{{.Spec.Module.Key}}.detail", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"{{if .HasModel}}{{.ModelPermissionBase}}.read{{else}}{{.DocumentPermissionBase}}.read{{end}}"}},
				{Key: "{{.Spec.Module.Key}}.form", Label: "{{.Spec.Module.Name}} Form", Kind: "navigate", RoutePath: "{{.RouteBase}}/form", ViewKey: "{{.Spec.Module.Key}}.form", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"{{if .HasModel}}{{.ModelPermissionBase}}.update{{else}}{{.DocumentPermissionBase}}.update{{end}}"}},
				{{- if .HasCustomUIStub }}
				{Key: "{{.Spec.Module.Key}}.workspace", Label: "{{.Spec.Module.Name}} Workspace", Kind: "navigate", RoutePath: "{{.RouteBase}}/workspace", CustomEntryKey: "{{.Spec.Module.Key}}.workspace", RenderMode: module.RenderModeCustom, RequiredPermissions: []string{"{{if .HasModel}}{{.ModelPermissionBase}}.read{{else}}{{.DocumentPermissionBase}}.read{{end}}"}},
				{{- end }}
			},
			Views: []module.ViewDefinition{
				{
					Key:   "{{.Spec.Module.Key}}.list",
					Title: "{{.Spec.Module.Name}}",
					Kind:  "list",
					{{if .HasModel}}ModelKey: "{{.Spec.Model.Key}}",{{else}}DocumentType: "{{.Spec.Document.Type}}",{{end}}
					RequiredPermissions: []string{"{{if .HasModel}}{{.ModelPermissionBase}}.list{{else}}{{.DocumentPermissionBase}}.list{{end}}"},
					Columns: []module.ColumnDefinition{
						{Key: "name", Label: "Name", Path: "{{if .HasModel}}values.name{{else}}header.type{{end}}"},
						{Key: "status", Label: "Status", Path: "{{if .HasModel}}values.status{{else}}header.status{{end}}"},
					},
					Filters: []module.FilterDefinition{
						{Key: "status", Label: "Status", Type: "enum", Options: []string{"active", "inactive", "blocked"}},
					},
					DefaultPageSize: 20,
					EmptyState:      "No records yet.",
				},
				{
					Key:   "{{.Spec.Module.Key}}.detail",
					Title: "{{.Spec.Module.Name}} Detail",
					Kind:  "detail",
					{{if .HasModel}}ModelKey: "{{.Spec.Model.Key}}",{{else}}DocumentType: "{{.Spec.Document.Type}}",{{end}}
					RequiredPermissions: []string{"{{if .HasModel}}{{.ModelPermissionBase}}.read{{else}}{{.DocumentPermissionBase}}.read{{end}}"},
					Tabs: []module.TabDefinition{
						{
							Key:   "summary",
							Title: "Summary",
							Sections: []module.SectionDefinition{
								{
									Key:   "core",
									Title: "Core",
									Fields: []module.FieldDefinition{
										{Key: "name", Label: "Name", Path: "{{if .HasModel}}values.name{{else}}header.type{{end}}", Type: "string"},
										{Key: "status", Label: "Status", Path: "{{if .HasModel}}values.status{{else}}header.status{{end}}", Type: "string"},
									},
								},
							},
						},
					},
				},
				{
					Key:   "{{.Spec.Module.Key}}.form",
					Title: "{{.Spec.Module.Name}} Form",
					Kind:  "form",
					{{if .HasModel}}ModelKey: "{{.Spec.Model.Key}}",{{else}}DocumentType: "{{.Spec.Document.Type}}",{{end}}
					RequiredPermissions: []string{"{{if .HasModel}}{{.ModelPermissionBase}}.update{{else}}{{.DocumentPermissionBase}}.update{{end}}"},
					Sections: []module.SectionDefinition{
						{
							Key:   "edit",
							Title: "Edit",
							Fields: []module.FieldDefinition{
								{Key: "name", Label: "Name", Path: "{{if .HasModel}}values.name{{else}}payload.title{{end}}", Type: "string", Widget: "text", Placeholder: "Name"},
								{Key: "status", Label: "Status", Path: "{{if .HasModel}}values.status{{else}}payload.status{{end}}", Type: "string", Widget: "select", Options: []string{"active", "inactive", "blocked"}},
							},
						},
					},
				},
			},
			{{- if .HasCustomUIStub }}
			CustomEntries: []module.CustomEntryDefinition{
				{
					Key:                 "{{.Spec.Module.Key}}.workspace",
					Title:               "{{.Spec.Module.Name}} Workspace",
					RoutePath:           "{{.RouteBase}}/workspace",
					BundleKey:           "{{.Spec.Module.Key}}-workspace",
					ComponentExport:     "render{{.ServiceName}}Workspace",
					RequiredPermissions: []string{"{{if .HasModel}}{{.ModelPermissionBase}}.read{{else}}{{.DocumentPermissionBase}}.read{{end}}"},
				},
			},
			{{- end }}
		},
		{{- end }}
		{{- if .HasCustomUIStub }}
		Bundles: []module.BundleDefinition{
			{
				Key:    "{{.Spec.Module.Key}}-workspace",
				Script: customBundleScript,
			},
		},
		{{- end }}
	}
}

func boolPtr(value bool) *bool {
	return &value
}
`

const serviceTemplate = `package {{.PackageName}}

type {{.ServiceName}} struct{}

func NewService() *{{.ServiceName}} {
	return &{{.ServiceName}}{}
}
`

const observabilityTemplate = `package {{.PackageName}}

import "clinic/internal/platform/observability"

func RegisterObservability(obs *observability.Service) {
	if obs == nil {
		return
	}
		{{- if .HasObservabilityStub }}
		for _, def := range Manifest().Observability.Metrics {
		obs.RegisterMetricDefinition(observability.MetricDefinition{
			Key:         def.Key,
			Type:        def.Type,
			Labels:      append([]string(nil), def.Labels...),
			Description: def.Description,
			ModuleKey:   Manifest().Key,
		})
	}
	for _, def := range Manifest().Observability.LogEvents {
		obs.RegisterLogEventDefinition(observability.LogEventDefinition{
			Key:            def.Key,
			Category:       def.Category,
			Severity:       def.Severity,
			RequiredFields: append([]string(nil), def.RequiredFields...),
			ModuleKey:      Manifest().Key,
		})
	}
	for _, def := range Manifest().Observability.DomainEvents {
		obs.RegisterDomainEventDefinition(observability.DomainEventDefinition{
			Type:                def.Type,
			Role:                def.Role,
			CorrelationRequired: def.CorrelationRequired,
			ModuleKey:           Manifest().Key,
		})
	}
	{{- end }}
}
`

const reportingTemplate = `package {{.PackageName}}

import (
	"clinic/internal/platform/module"
	"clinic/internal/platform/reporting"
)

func RegisterReporting(svc *reporting.Service) error {
	if svc == nil {
		return nil
	}
	for _, dataset := range Manifest().Datasets {
		def := reporting.DatasetDefinition{
			Key:        dataset.Key,
			Title:      dataset.Title,
			SourceKind: dataset.SourceKind,
			ModelKey:   dataset.ModelKey,
			Dimensions: make([]reporting.DimensionDefinition, 0, len(dataset.Dimensions)),
			Measures:   make([]reporting.MeasureDefinition, 0, len(dataset.Measures)),
		}
		for _, dimension := range dataset.Dimensions {
			def.Dimensions = append(def.Dimensions, reporting.DimensionDefinition{
				Key:   dimension.Key,
				Label: dimension.Label,
				Path:  dimension.Path,
			})
		}
		for _, measure := range dataset.Measures {
			def.Measures = append(def.Measures, reporting.MeasureDefinition{
				Key:   measure.Key,
				Label: measure.Label,
				Kind:  measure.Kind,
				Path:  measure.Path,
			})
		}
		if err := svc.Register(def); err != nil {
			return err
		}
	}
	return nil
}

func DatasetDefinitions() []module.DatasetDefinition {
	return append([]module.DatasetDefinition(nil), Manifest().Datasets...)
}
`

const bundleGoTemplate = `package {{.PackageName}}

import _ "embed"

//go:embed bundle.js
var customBundleScript string
`

const bundleJSTemplate = `(function() {
  window.ClinicModuleBundles = window.ClinicModuleBundles || {};
  window.ClinicModuleBundles.render{{.ServiceName}}Workspace = function(root, context) {
    root.innerHTML = '<section class="panel"><h2>{{.Spec.Module.Name}} Workspace</h2><p>Replace this custom bundle stub with module-specific UI.</p><pre>' + JSON.stringify(context || {}, null, 2) + '</pre></section>';
  };
}());
`

const testTemplate = `package {{.PackageName}}

import "testing"

func TestManifest(t *testing.T) {
	manifest := Manifest()
	if manifest.Key != "{{.Spec.Module.Key}}" {
		t.Fatalf("expected manifest key {{.Spec.Module.Key}}, got %s", manifest.Key)
	}
	if manifest.Name == "" {
		t.Fatal("expected manifest name")
	}
}
`

const registrationTestTemplate = `package {{.PackageName}}

import "testing"

func TestGeneratedScaffolds(t *testing.T) {
	manifest := Manifest()
	{{- if .HasObservabilityHelper }}
	RegisterObservability(nil)
	{{- end }}
	{{- if .HasReportingHelper }}
	if got := DatasetDefinitions(); len(got) != len(manifest.Datasets) {
		t.Fatalf("expected dataset helper to mirror manifest datasets, got %d want %d", len(got), len(manifest.Datasets))
	}
	{{- end }}
		{{- if .HasCustomUIStub }}
		if len(manifest.Bundles) == 0 {
		t.Fatal("expected custom bundle scaffold")
	}
	{{- end }}
}
`
