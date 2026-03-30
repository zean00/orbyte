package httpx

import (
	"sort"

	"orbyte/internal/platform/module"
)

func buildAdminModuleDependencyGraph(details []module.ScopedDetail) adminModuleDependencyGraph {
	detailByKey := make(map[string]module.ScopedDetail, len(details))
	for _, detail := range details {
		detailByKey[detail.Manifest.Key] = detail
	}
	nodes := make([]adminModuleDependencyNode, 0, len(details))
	for _, detail := range details {
		nodes = append(nodes, adminModuleDependencyNode{
			ModuleKey:      detail.Manifest.Key,
			Name:           detail.Manifest.Name,
			Version:        detail.Manifest.Version,
			Enabled:        detail.Installed.Enabled,
			LifecycleState: detail.LifecycleState,
			Role:           string(detail.Manifest.Role),
			DomainFamily:   detail.Manifest.DomainFamily,
			Category:       detail.Manifest.Category,
			Status:         moduleNodeStatus(detail),
			ConsolePath:    "/admin/modules/" + detail.Manifest.Key,
		})
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ModuleKey < nodes[j].ModuleKey })

	edges := make([]adminModuleDependencyEdge, 0)
	for _, detail := range details {
		diagnostics := diagnosticsByModuleKey(detail.DependencyDiagnostics)
		requirements := manifestDependencies(detail.Manifest)
		for _, requirement := range requirements {
			diagnostic, ok := diagnostics[requirement.ModuleKey]
			enabled := false
			compatible := requirement.Kind == module.DependencyKindOptional
			reason := ""
			if ok {
				enabled = diagnostic.Enabled
				compatible = diagnostic.Compatible
				reason = diagnostic.Reason
			} else if dependency, found := detailByKey[requirement.ModuleKey]; found {
				enabled = dependency.Installed.Enabled
				compatible = true
			}
			edges = append(edges, adminModuleDependencyEdge{
				SourceModuleKey: detail.Manifest.Key,
				TargetModuleKey: requirement.ModuleKey,
				Kind:            string(requirement.Kind),
				VersionRange:    requirement.VersionRange,
				Status:          moduleEdgeStatus(requirement, ok, enabled, compatible),
				Enabled:         enabled,
				Compatible:      compatible,
				Reason:          reason,
			})
		}
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].SourceModuleKey == edges[j].SourceModuleKey {
			if edges[i].TargetModuleKey == edges[j].TargetModuleKey {
				return edges[i].Kind < edges[j].Kind
			}
			return edges[i].TargetModuleKey < edges[j].TargetModuleKey
		}
		return edges[i].SourceModuleKey < edges[j].SourceModuleKey
	})

	summary := adminModuleDependencySummary{
		TotalModules: len(nodes),
		TotalEdges:   len(edges),
	}
	for _, node := range nodes {
		if node.Enabled {
			summary.EnabledModules++
		}
		if node.Status != "healthy" {
			summary.UnhealthyModules++
		}
	}
	return adminModuleDependencyGraph{Nodes: nodes, Edges: edges, Summary: summary}
}

func buildFocusedAdminModuleDependencyGraph(details []module.ScopedDetail, moduleKey string) adminModuleDependencyGraph {
	if moduleKey == "" {
		return adminModuleDependencyGraph{}
	}
	include := map[string]bool{moduleKey: true}
	for _, detail := range details {
		if detail.Manifest.Key == moduleKey {
			for _, requirement := range manifestDependencies(detail.Manifest) {
				include[requirement.ModuleKey] = true
			}
			break
		}
	}
	for _, detail := range details {
		for _, requirement := range manifestDependencies(detail.Manifest) {
			if requirement.ModuleKey == moduleKey {
				include[detail.Manifest.Key] = true
				break
			}
		}
	}
	filtered := make([]module.ScopedDetail, 0, len(include))
	for _, detail := range details {
		if include[detail.Manifest.Key] {
			filtered = append(filtered, detail)
		}
	}
	return buildAdminModuleDependencyGraph(filtered)
}

func moduleNodeStatus(detail module.ScopedDetail) string {
	if !detail.Installed.Enabled || detail.LifecycleState == "disabled" {
		return "disabled"
	}
	if detail.LifecycleState == "healthy" {
		return "healthy"
	}
	return "warning"
}

func moduleEdgeStatus(requirement module.DependencyRequirement, found, enabled, compatible bool) string {
	if !found {
		return "missing"
	}
	if !enabled {
		if requirement.Kind == module.DependencyKindOptional {
			return "optional"
		}
		return "disabled"
	}
	if !compatible {
		return "incompatible"
	}
	return "ok"
}

func diagnosticsByModuleKey(diagnostics []module.DependencyDiagnostic) map[string]module.DependencyDiagnostic {
	items := make(map[string]module.DependencyDiagnostic, len(diagnostics))
	for _, diagnostic := range diagnostics {
		items[diagnostic.ModuleKey] = diagnostic
	}
	return items
}

func manifestDependencies(manifest module.Manifest) []module.DependencyRequirement {
	if len(manifest.DependencyRequirements) > 0 {
		return append([]module.DependencyRequirement(nil), manifest.DependencyRequirements...)
	}
	if len(manifest.Dependencies) == 0 {
		return nil
	}
	items := make([]module.DependencyRequirement, 0, len(manifest.Dependencies))
	for _, dependency := range manifest.Dependencies {
		items = append(items, module.DependencyRequirement{
			ModuleKey: dependency,
			Kind:      module.DependencyKindRequired,
		})
	}
	return items
}
