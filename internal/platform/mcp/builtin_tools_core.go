package mcp

func (s *Server) appendBuiltInCoreToolRegistrations(registry []builtInToolRegistration) []builtInToolRegistration {
	if s != nil && s.templates != nil {
		registry = append(registry,
			mustBuiltInToolRegistration(func(s *Server, actor ActorContext, _ map[string]any) (map[string]any, bool, error) {
				return s.templateDefinitionList(actor), true, nil
			}, builtInTool{
				name:        "template.definition.list",
				title:       "List Template Definitions",
				description: "List available print template definitions.",
				permission:  "template.read",
			}),
			mustBuiltInToolRegistration((*Server).templateDefinitionGet, builtInTool{
				name:        "template.definition.get",
				title:       "Get Template Definition",
				description: "Get one template definition and version metadata.",
				permission:  "template.read",
				inputSchema: map[string]any{"type": "object", "properties": map[string]any{"template_key": map[string]any{"type": "string"}}, "required": []string{"template_key"}},
			}),
			mustBuiltInToolRegistration((*Server).templateDraftGet, builtInTool{
				name:        "template.draft.get",
				title:       "Get Template Draft",
				description: "Load the latest draft or defaults for a template.",
				permission:  "template.read",
				inputSchema: map[string]any{"type": "object", "properties": map[string]any{"template_key": map[string]any{"type": "string"}}, "required": []string{"template_key"}},
			}),
			mustBuiltInToolRegistration((*Server).templateDraftSave, builtInTool{
				name:        "template.draft.save",
				title:       "Save Template Draft",
				description: "Create or update a template draft.",
				permission:  "template.manage",
				inputSchema: map[string]any{"type": "object", "properties": map[string]any{"template_key": map[string]any{"type": "string"}}, "required": []string{"template_key"}},
			}),
			mustBuiltInToolRegistration((*Server).templateRenderPreview, builtInTool{
				name:        "template.render.preview",
				title:       "Preview Template Render",
				description: "Render a template preview in HTML or the requested output format.",
				permission:  "template.render",
				inputSchema: map[string]any{"type": "object", "properties": map[string]any{"template_key": map[string]any{"type": "string"}}},
			}),
		)
	}
	if s != nil && s.analytics != nil {
		registry = append(registry,
			mustBuiltInToolRegistration((*Server).analyticsDashboardList, builtInTool{name: "analytics.dashboard.list", title: "List Dashboards", description: "List runtime analytics dashboards.", permission: "analytics.read"}),
			mustBuiltInToolRegistration((*Server).analyticsDashboardGet, builtInTool{name: "analytics.dashboard.get", title: "Get Dashboard", description: "Get one runtime analytics dashboard.", permission: "analytics.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"dashboard_id": map[string]any{"type": "string"}}, "required": []string{"dashboard_id"}}}),
			mustBuiltInToolRegistration((*Server).analyticsDashboardSave, builtInTool{name: "analytics.dashboard.save", title: "Save Dashboard", description: "Create or update a runtime analytics dashboard.", permission: "analytics.author", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"dashboard": map[string]any{"type": "object"}}, "required": []string{"dashboard"}}}),
			mustBuiltInToolRegistration((*Server).analyticsDashboardDelete, builtInTool{name: "analytics.dashboard.delete", title: "Delete Dashboard", description: "Delete a runtime analytics dashboard.", permission: "analytics.author", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"dashboard_id": map[string]any{"type": "string"}}, "required": []string{"dashboard_id"}}}),
			mustBuiltInToolRegistration((*Server).analyticsMetricList, builtInTool{name: "analytics.metric.list", title: "List Saved Metrics", description: "List runtime analytics saved metrics.", permission: "analytics.read"}),
			mustBuiltInToolRegistration((*Server).analyticsMetricGet, builtInTool{name: "analytics.metric.get", title: "Get Saved Metric", description: "Get one runtime analytics saved metric.", permission: "analytics.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"metric_id": map[string]any{"type": "string"}}, "required": []string{"metric_id"}}}),
			mustBuiltInToolRegistration((*Server).analyticsMetricSave, builtInTool{name: "analytics.metric.save", title: "Save Saved Metric", description: "Create or update a runtime analytics saved metric.", permission: "analytics.author", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"metric": map[string]any{"type": "object"}}, "required": []string{"metric"}}}),
			mustBuiltInToolRegistration((*Server).analyticsMetricDelete, builtInTool{name: "analytics.metric.delete", title: "Delete Saved Metric", description: "Delete a runtime analytics saved metric.", permission: "analytics.author", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"metric_id": map[string]any{"type": "string"}}, "required": []string{"metric_id"}}}),
			mustBuiltInToolRegistration((*Server).analyticsQueryList, builtInTool{name: "analytics.query.list", title: "List Saved Queries", description: "List runtime analytics saved queries.", permission: "analytics.read"}),
			mustBuiltInToolRegistration((*Server).analyticsQueryGet, builtInTool{name: "analytics.query.get", title: "Get Saved Query", description: "Get one runtime analytics saved query.", permission: "analytics.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"query_id": map[string]any{"type": "string"}}, "required": []string{"query_id"}}}),
			mustBuiltInToolRegistration((*Server).analyticsQuerySave, builtInTool{name: "analytics.query.save", title: "Save Saved Query", description: "Create or update a runtime analytics saved query.", permission: "analytics.author", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"query": map[string]any{"type": "object"}}, "required": []string{"query"}}}),
			mustBuiltInToolRegistration((*Server).analyticsQueryDelete, builtInTool{name: "analytics.query.delete", title: "Delete Saved Query", description: "Delete a runtime analytics saved query.", permission: "analytics.author", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"query_id": map[string]any{"type": "string"}}, "required": []string{"query_id"}}}),
			mustBuiltInToolRegistration((*Server).analyticsQueryExecute, builtInTool{name: "analytics.query.execute", title: "Execute Analytics Query", description: "Run an ad hoc analytics query and return table plus chart data.", permission: "analytics.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"query": map[string]any{"type": "object"}}, "required": []string{"query"}}}),
			mustBuiltInToolRegistration((*Server).analyticsChartGenerate, builtInTool{name: "analytics.chart.generate", title: "Generate Analytics Chart", description: "Generate a normalized chart spec from a query or execution result.", permission: "analytics.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"query": map[string]any{"type": "object"}, "result": map[string]any{"type": "object"}}}}),
			mustBuiltInToolRegistration((*Server).analyticsReportDefinitionList, builtInTool{name: "analytics.report.definition.list", title: "List Report Definitions", description: "List analytics report definitions.", permission: "analytics.read"}),
			mustBuiltInToolRegistration((*Server).analyticsReportDefinitionGet, builtInTool{name: "analytics.report.definition.get", title: "Get Report Definition", description: "Get one analytics report definition.", permission: "analytics.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"report_id": map[string]any{"type": "string"}}, "required": []string{"report_id"}}}),
			mustBuiltInToolRegistration((*Server).analyticsReportDefinitionSave, builtInTool{name: "analytics.report.definition.save", title: "Save Report Definition", description: "Create or update an analytics report definition.", permission: "analytics.manage_reports", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"report": map[string]any{"type": "object"}}, "required": []string{"report"}}}),
			mustBuiltInToolRegistration((*Server).analyticsReportDefinitionDelete, builtInTool{name: "analytics.report.definition.delete", title: "Delete Report Definition", description: "Delete an analytics report definition.", permission: "analytics.manage_reports", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"report_id": map[string]any{"type": "string"}}, "required": []string{"report_id"}}}),
			mustBuiltInToolRegistration((*Server).analyticsReportRun, builtInTool{name: "analytics.report.run", title: "Run Analytics Report", description: "Run a stored analytics report definition.", permission: "analytics.manage_reports", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"report_id": map[string]any{"type": "string"}}, "required": []string{"report_id"}}}),
			mustBuiltInToolRegistration((*Server).analyticsReportDeliver, builtInTool{name: "analytics.report.deliver", title: "Deliver Analytics Report", description: "Deliver a report artifact or run a report and deliver it.", permission: "analytics.deliver_reports", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"artifact_id": map[string]any{"type": "string"}, "report_id": map[string]any{"type": "string"}, "channel": map[string]any{"type": "string"}, "recipient": map[string]any{"type": "string"}}}}),
		)
	}
	if s != nil && s.workflows != nil {
		registry = append(registry,
			mustBuiltInToolRegistration((*Server).workflowDefinitionList, builtInTool{name: "workflow.definition.list", title: "List Workflow Definitions", description: "List workflow definitions and published versions.", permission: "configuration.read"}),
			mustBuiltInToolRegistration((*Server).workflowDefinitionGet, builtInTool{name: "workflow.definition.get", title: "Get Workflow Definition", description: "Get one workflow definition plus versions and current draft.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"workflow_key": map[string]any{"type": "string"}}, "required": []string{"workflow_key"}}}),
			mustBuiltInToolRegistration((*Server).workflowVersionList, builtInTool{name: "workflow.version.list", title: "List Workflow Versions", description: "List all workflow versions for a definition.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"workflow_key": map[string]any{"type": "string"}}, "required": []string{"workflow_key"}}}),
			mustBuiltInToolRegistration((*Server).workflowDraftCreate, builtInTool{name: "workflow.draft.create", title: "Create Workflow Draft", description: "Create a new workflow draft from the current published version.", permission: "configuration.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"workflow_key": map[string]any{"type": "string"}}, "required": []string{"workflow_key"}}}),
			mustBuiltInToolRegistration((*Server).workflowDraftGet, builtInTool{name: "workflow.draft.get", title: "Get Workflow Draft", description: "Load the current workflow draft or a draft version.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"workflow_key": map[string]any{"type": "string"}, "version": map[string]any{"type": "integer"}}, "required": []string{"workflow_key"}}}),
			mustBuiltInToolRegistration((*Server).workflowDraftSave, builtInTool{name: "workflow.draft.save", title: "Save Workflow Draft", description: "Create or update a workflow draft definition.", permission: "configuration.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"workflow": map[string]any{"type": "object"}}, "required": []string{"workflow"}}}),
			mustBuiltInToolRegistration((*Server).workflowDraftValidate, builtInTool{name: "workflow.draft.validate", title: "Validate Workflow Draft", description: "Validate a workflow draft or draft version.", permission: "configuration.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"workflow": map[string]any{"type": "object"}, "workflow_key": map[string]any{"type": "string"}, "version": map[string]any{"type": "integer"}}}}),
			mustBuiltInToolRegistration((*Server).workflowDraftSimulate, builtInTool{name: "workflow.draft.simulate", title: "Simulate Workflow Draft", description: "Simulate a workflow transition and preview routing.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"workflow": map[string]any{"type": "object"}, "workflow_key": map[string]any{"type": "string"}, "version": map[string]any{"type": "integer"}, "input": map[string]any{"type": "object"}}}}),
			mustBuiltInToolRegistration((*Server).workflowDraftPublish, builtInTool{name: "workflow.draft.publish", title: "Publish Workflow Draft", description: "Publish a workflow draft version. Requires explicit confirmation.", permission: "configuration.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"workflow_key": map[string]any{"type": "string"}, "version": map[string]any{"type": "integer"}, "confirm_publish": map[string]any{"type": "boolean"}}, "required": []string{"workflow_key", "version", "confirm_publish"}}}),
			mustBuiltInToolRegistration((*Server).workflowRuntimeTasksList, builtInTool{name: "workflow.runtime.tasks.list", title: "List Workflow Tasks", description: "List read-only workflow tasks.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"workflow_key": map[string]any{"type": "string"}, "status": map[string]any{"type": "string"}, "assignee_user_id": map[string]any{"type": "string"}, "target_id": map[string]any{"type": "string"}}}}),
			mustBuiltInToolRegistration((*Server).workflowRuntimeApprovalsList, builtInTool{name: "workflow.runtime.approvals.list", title: "List Workflow Approvals", description: "List read-only workflow approvals.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"workflow_key": map[string]any{"type": "string"}, "status": map[string]any{"type": "string"}, "target_id": map[string]any{"type": "string"}, "stage_key": map[string]any{"type": "string"}}}}),
			mustBuiltInToolRegistration((*Server).workflowRuntimeHistoryGet, builtInTool{name: "workflow.runtime.history.get", title: "Get Workflow History", description: "Get workflow history for one target.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"target_type": map[string]any{"type": "string"}, "target_id": map[string]any{"type": "string"}}, "required": []string{"target_type", "target_id"}}}),
			mustBuiltInToolRegistration((*Server).workflowHierarchyGraphGet, builtInTool{name: "workflow.hierarchy.graph.get", title: "Get Workflow Hierarchy Graph", description: "Get the reporting-line graph for workflow routing.", permission: "identity.manage_users", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"organization_id": map[string]any{"type": "string"}, "location_id": map[string]any{"type": "string"}, "operating_unit_id": map[string]any{"type": "string"}, "status": map[string]any{"type": "string"}}}}),
			mustBuiltInToolRegistration((*Server).workflowHierarchyChainGet, builtInTool{name: "workflow.hierarchy.chain.get", title: "Get Workflow Hierarchy Chain", description: "Get the manager chain for a user.", permission: "identity.manage_users", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"user_id": map[string]any{"type": "string"}, "organization_id": map[string]any{"type": "string"}, "location_id": map[string]any{"type": "string"}, "operating_unit_id": map[string]any{"type": "string"}}, "required": []string{"user_id"}}}),
			mustBuiltInToolRegistration((*Server).workflowHierarchySummaryGet, builtInTool{name: "workflow.hierarchy.summary.get", title: "Get Workflow Hierarchy Summary", description: "Get hierarchy coverage and exception summary.", permission: "identity.manage_users", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"organization_id": map[string]any{"type": "string"}, "location_id": map[string]any{"type": "string"}, "operating_unit_id": map[string]any{"type": "string"}, "status": map[string]any{"type": "string"}}}}),
			mustBuiltInToolRegistration((*Server).workflowReportingLineList, builtInTool{name: "workflow.reporting_line.list", title: "List Reporting Lines", description: "List reporting lines used for workflow routing.", permission: "identity.manage_users", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"subject_user_id": map[string]any{"type": "string"}, "manager_user_id": map[string]any{"type": "string"}, "status": map[string]any{"type": "string"}}}}),
			mustBuiltInToolRegistration((*Server).workflowReportingLineSave, builtInTool{name: "workflow.reporting_line.save", title: "Save Reporting Line", description: "Create or update a reporting line for workflow routing.", permission: "identity.manage_users", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"reporting_line": map[string]any{"type": "object"}}, "required": []string{"reporting_line"}}}),
		)
	}
	if s != nil && s.config != nil {
		registry = append(registry,
			mustBuiltInToolRegistration((*Server).configDefinitionList, builtInTool{name: "config.definition.list", title: "List Config Definitions", description: "List configuration definitions and allowed scopes.", permission: "configuration.read"}),
			mustBuiltInToolRegistration((*Server).configEntryList, builtInTool{name: "config.entry.list", title: "List Config Entries", description: "List stored configuration entries.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"config_keys": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, "config_scopes": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}}}}),
			mustBuiltInToolRegistration((*Server).configEffectiveGet, builtInTool{name: "config.effective.get", title: "Get Effective Config", description: "Get effective configuration for a context.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"organization_id": map[string]any{"type": "string"}, "location_id": map[string]any{"type": "string"}}}}),
			mustBuiltInToolRegistration((*Server).configCompare, builtInTool{name: "config.compare", title: "Compare Config Contexts", description: "Compare effective configuration across two contexts.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"left": map[string]any{"type": "object"}, "right": map[string]any{"type": "object"}}}}),
			mustBuiltInToolRegistration((*Server).configBundleExport, builtInTool{name: "config.bundle.export", title: "Export Config Bundle", description: "Export config and feature flag values into a promotion bundle.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"name": map[string]any{"type": "string"}, "config_keys": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, "config_scopes": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, "include_flags": map[string]any{"type": "boolean"}, "flag_keys": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, "flag_scopes": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}}}}),
			mustBuiltInToolRegistration((*Server).configBundleValidate, builtInTool{name: "config.bundle.validate", title: "Validate Config Bundle", description: "Validate a configuration bundle without applying it.", permission: "configuration.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"bundle": map[string]any{"type": "object"}}, "required": []string{"bundle"}}}),
			mustBuiltInToolRegistration((*Server).configBundleApply, builtInTool{name: "config.bundle.apply", title: "Apply Config Bundle", description: "Apply a validated configuration bundle. Requires explicit confirmation.", permission: "configuration.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"bundle": map[string]any{"type": "object"}, "confirm_apply": map[string]any{"type": "boolean"}}, "required": []string{"bundle", "confirm_apply"}}}),
		)
	}
	if s != nil && s.flags != nil {
		registry = append(registry,
			mustBuiltInToolRegistration((*Server).featureFlagDefinitionList, builtInTool{name: "feature_flag.definition.list", title: "List Feature Flag Definitions", description: "List feature flag definitions.", permission: "configuration.read"}),
			mustBuiltInToolRegistration((*Server).featureFlagValueList, builtInTool{name: "feature_flag.value.list", title: "List Feature Flag Values", description: "List stored feature flag values.", permission: "configuration.read"}),
			mustBuiltInToolRegistration((*Server).featureFlagTargetingGet, builtInTool{name: "feature_flag.targeting.get", title: "Get Feature Flag Targeting", description: "Inspect raw overrides and effective resolution for one feature flag.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"flag_key": map[string]any{"type": "string"}, "organization_id": map[string]any{"type": "string"}, "location_id": map[string]any{"type": "string"}, "operating_unit_id": map[string]any{"type": "string"}}, "required": []string{"flag_key"}}}),
			mustBuiltInToolRegistration((*Server).featureFlagValueUpsert, builtInTool{name: "feature_flag.value.upsert", title: "Upsert Feature Flag Value", description: "Create or update a feature flag override. Requires explicit confirmation when activating changes.", permission: "configuration.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"value": map[string]any{"type": "object"}, "confirm_apply": map[string]any{"type": "boolean"}}, "required": []string{"value", "confirm_apply"}}}),
		)
	}
	if s != nil && s.identity != nil {
		registry = append(registry,
			mustBuiltInToolRegistration((*Server).identityRolePermissionMatrixGet, builtInTool{name: "identity.role_permission_matrix.get", title: "Get Role Permission Matrix", description: "Get roles, permissions, grants, and bindings in matrix form.", permission: "identity.manage_users"}),
			mustBuiltInToolRegistration((*Server).identityRolePermissionGrant, builtInTool{name: "identity.role_permission.grant", title: "Grant Role Permission", description: "Grant a permission to an existing role. Requires confirmation.", permission: "identity.manage_users", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"role_id": map[string]any{"type": "string"}, "permission_key": map[string]any{"type": "string"}, "confirm_apply": map[string]any{"type": "boolean"}}, "required": []string{"role_id", "permission_key", "confirm_apply"}}}),
			mustBuiltInToolRegistration((*Server).identityRolePermissionRevoke, builtInTool{name: "identity.role_permission.revoke", title: "Revoke Role Permission", description: "Revoke a permission from an existing role. Requires confirmation.", permission: "identity.manage_users", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"role_id": map[string]any{"type": "string"}, "permission_key": map[string]any{"type": "string"}, "confirm_apply": map[string]any{"type": "boolean"}}, "required": []string{"role_id", "permission_key", "confirm_apply"}}}),
			mustBuiltInToolRegistration((*Server).identityRoleBindingList, builtInTool{name: "identity.role_binding.list", title: "List Role Bindings", description: "List current role bindings.", permission: "identity.manage_users"}),
			mustBuiltInToolRegistration((*Server).identityRoleBindingPrioritySet, builtInTool{name: "identity.role_binding.priority.set", title: "Set Role Binding Priority", description: "Set role binding priority. Requires confirmation.", permission: "identity.manage_users", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"binding_id": map[string]any{"type": "string"}, "priority": map[string]any{"type": "integer"}, "confirm_apply": map[string]any{"type": "boolean"}}, "required": []string{"binding_id", "priority", "confirm_apply"}}}),
		)
	}
	if s != nil && s.modules != nil {
		registry = append(registry,
			mustBuiltInToolRegistration((*Server).moduleList, builtInTool{name: "module.list", title: "List Modules", description: "List installed modules and lifecycle state.", permission: "configuration.read"}),
			mustBuiltInToolRegistration((*Server).moduleCompatibilityList, builtInTool{name: "module.compatibility.list", title: "List Module Compatibility", description: "List module compatibility diagnostics.", permission: "configuration.read"}),
			mustBuiltInToolRegistration((*Server).moduleEnable, builtInTool{name: "module.enable", title: "Enable Module", description: "Enable one module. Requires confirmation.", permission: "module.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"module_key": map[string]any{"type": "string"}, "confirm_apply": map[string]any{"type": "boolean"}}, "required": []string{"module_key", "confirm_apply"}}}),
			mustBuiltInToolRegistration((*Server).moduleDisable, builtInTool{name: "module.disable", title: "Disable Module", description: "Disable one module. Requires confirmation.", permission: "module.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"module_key": map[string]any{"type": "string"}, "confirm_apply": map[string]any{"type": "boolean"}}, "required": []string{"module_key", "confirm_apply"}}}),
			mustBuiltInToolRegistration((*Server).businessModuleList, builtInTool{name: "business.module.list", title: "List Business Modules", description: "List enabled business modules with descriptions, capabilities, documents, models, and dependencies.", permission: "module.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"organization_id": map[string]any{"type": "string"}, "location_id": map[string]any{"type": "string"}, "query": map[string]any{"type": "string"}}}}),
			mustBuiltInToolRegistration((*Server).businessModuleGet, builtInTool{name: "business.module.get", title: "Get Business Module", description: "Get one module's business metadata and owned artifacts.", permission: "module.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"module_key": map[string]any{"type": "string"}, "organization_id": map[string]any{"type": "string"}, "location_id": map[string]any{"type": "string"}}, "required": []string{"module_key"}}}),
			mustBuiltInToolRegistration((*Server).businessCapabilitySearch, builtInTool{name: "business.capability.search", title: "Search Business Capabilities", description: "Search business capabilities across enabled modules.", permission: "module.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"query": map[string]any{"type": "string"}, "organization_id": map[string]any{"type": "string"}, "location_id": map[string]any{"type": "string"}}, "required": []string{"query"}}}),
			mustBuiltInToolRegistration((*Server).businessTopologyMap, builtInTool{name: "business.topology.map", title: "Map Business Topology", description: "Map enabled business modules, capabilities, and dependencies in one topology view.", permission: "module.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"organization_id": map[string]any{"type": "string"}, "location_id": map[string]any{"type": "string"}}}}),
			mustBuiltInToolRegistration((*Server).businessHealthSummary, builtInTool{name: "business.health.summary", title: "Get Business Health Summary", description: "Summarize module, workflow, document, audit, and analytics health across the business.", permission: "module.read", contract: ContractDescriptor{BusinessDomains: []string{"cross-domain", "operations", "finance"}}, inputSchema: map[string]any{"type": "object", "properties": map[string]any{"organization_id": map[string]any{"type": "string"}, "location_id": map[string]any{"type": "string"}}}}),
			mustBuiltInToolRegistration((*Server).businessExceptionSearch, builtInTool{name: "business.exception.search", title: "Search Business Exceptions", description: "List pending approvals, open workflow tasks, and unresolved business exceptions.", permission: "module.read", contract: ContractDescriptor{BusinessDomains: []string{"cross-domain", "operations"}}, inputSchema: map[string]any{"type": "object", "properties": map[string]any{"status": map[string]any{"type": "string"}, "page": map[string]any{"type": "integer"}, "page_size": map[string]any{"type": "integer"}}}}),
		)
	}
	if s != nil && s.search != nil {
		registry = append(registry,
			mustBuiltInToolRegistration((*Server).searchIndexList, builtInTool{name: "search.runtime.list", title: "List Search Runtime", description: "List search indexes and runtime status.", permission: "search.manage"}),
			mustBuiltInToolRegistration((*Server).searchRuntimeGet, builtInTool{name: "search.runtime.get", title: "Get Search Runtime", description: "Get runtime state for one search index.", permission: "search.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"index_key": map[string]any{"type": "string"}}, "required": []string{"index_key"}}}),
			mustBuiltInToolRegistration((*Server).searchConsistencyGet, builtInTool{name: "search.consistency.get", title: "Get Search Consistency", description: "Get consistency report for one search index.", permission: "search.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"index_key": map[string]any{"type": "string"}}, "required": []string{"index_key"}}}),
			mustBuiltInToolRegistration((*Server).searchRebuild, builtInTool{name: "search.rebuild", title: "Rebuild Search Index", description: "Rebuild one search index. Requires confirmation.", permission: "search.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"index_key": map[string]any{"type": "string"}, "confirm_apply": map[string]any{"type": "boolean"}}, "required": []string{"index_key", "confirm_apply"}}}),
			mustBuiltInToolRegistration((*Server).searchRepair, builtInTool{name: "search.repair", title: "Repair Search Index", description: "Repair one search index. Requires confirmation.", permission: "search.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"index_key": map[string]any{"type": "string"}, "mode": map[string]any{"type": "string"}, "target_id": map[string]any{"type": "string"}, "confirm_apply": map[string]any{"type": "boolean"}}, "required": []string{"index_key", "confirm_apply"}}}),
			mustBuiltInToolRegistration((*Server).searchReconcile, builtInTool{name: "search.reconcile", title: "Reconcile Search Index", description: "Run a consistency scan for one search index. Requires confirmation.", permission: "search.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"index_key": map[string]any{"type": "string"}, "confirm_apply": map[string]any{"type": "boolean"}}, "required": []string{"index_key", "confirm_apply"}}}),
			mustBuiltInToolRegistration((*Server).searchSchemaPlan, builtInTool{name: "search.schema.plan", title: "Plan Search Schema", description: "Plan a candidate schema version for one index.", permission: "search.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"index_key": map[string]any{"type": "string"}, "version": map[string]any{"type": "string"}}, "required": []string{"index_key", "version"}}}),
			mustBuiltInToolRegistration((*Server).searchSchemaBuild, builtInTool{name: "search.schema.build", title: "Build Search Schema", description: "Build the candidate search schema. Requires confirmation.", permission: "search.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"index_key": map[string]any{"type": "string"}, "confirm_apply": map[string]any{"type": "boolean"}}, "required": []string{"index_key", "confirm_apply"}}}),
			mustBuiltInToolRegistration((*Server).searchSchemaActivate, builtInTool{name: "search.schema.activate", title: "Activate Search Schema", description: "Activate the candidate search schema. Requires confirmation.", permission: "search.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"index_key": map[string]any{"type": "string"}, "confirm_apply": map[string]any{"type": "boolean"}}, "required": []string{"index_key", "confirm_apply"}}}),
			mustBuiltInToolRegistration((*Server).searchQuery, builtInTool{name: "search.query", title: "Query Search Index", description: "Query documents using keyword, vector, or hybrid search modes.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"index_key": map[string]any{"type": "string"}, "mode": map[string]any{"type": "string"}, "query": map[string]any{"type": "string"}, "vector_text": map[string]any{"type": "string"}, "filters": map[string]any{"type": "object"}, "page": map[string]any{"type": "integer"}, "page_size": map[string]any{"type": "integer"}}, "required": []string{"index_key"}}}),
		)
	}
	if s != nil && s.offline != nil {
		registry = append(registry,
			mustBuiltInToolRegistration((*Server).offlineSyncList, builtInTool{name: "offline.sync.list", title: "List Offline Sync Batches", description: "List offline sync batches and recent outcomes.", permission: "ops.read"}),
			mustBuiltInToolRegistration((*Server).offlineSyncGet, builtInTool{name: "offline.sync.get", title: "Get Offline Sync Batch", description: "Get one offline sync batch and its outcomes.", permission: "ops.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"batch_id": map[string]any{"type": "string"}}, "required": []string{"batch_id"}}}),
			mustBuiltInToolRegistration((*Server).offlineConflictList, builtInTool{name: "offline.conflict.list", title: "List Offline Conflicts", description: "List offline sync conflicts.", permission: "ops.read"}),
		)
	}
	if s != nil && s.policy != nil {
		registry = append(registry,
			mustBuiltInToolRegistration((*Server).policyHookList, builtInTool{name: "policy.hook.list", title: "List Policy Hook Runtime", description: "List policy hook runtimes.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"organization_id": map[string]any{"type": "string"}, "location_id": map[string]any{"type": "string"}}}}),
			mustBuiltInToolRegistration((*Server).policyHookGet, builtInTool{name: "policy.hook.get", title: "Get Policy Hook Runtime", description: "Get one policy hook runtime.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"hook_key": map[string]any{"type": "string"}, "organization_id": map[string]any{"type": "string"}, "location_id": map[string]any{"type": "string"}}, "required": []string{"hook_key"}}}),
			mustBuiltInToolRegistration((*Server).policyModuleUpsert, builtInTool{name: "policy.module.upsert", title: "Update Policy Module", description: "Update Rego source for a policy hook. Requires confirmation.", permission: "configuration.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"hook_key": map[string]any{"type": "string"}, "scope": map[string]any{"type": "string"}, "scope_id": map[string]any{"type": "string"}, "source": map[string]any{"type": "string"}, "confirm_apply": map[string]any{"type": "boolean"}}, "required": []string{"hook_key", "source", "confirm_apply"}}}),
		)
	}
	if s != nil && s.reference != nil {
		registry = append(registry,
			mustBuiltInToolRegistration((*Server).referenceTypeList, builtInTool{name: "reference.type.list", title: "List Reference Types", description: "List reference data types.", permission: "configuration.read"}),
			mustBuiltInToolRegistration((*Server).referenceRecordList, builtInTool{name: "reference.record.list", title: "List Reference Records", description: "List records for one reference type.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"type_key": map[string]any{"type": "string"}}, "required": []string{"type_key"}}}),
			mustBuiltInToolRegistration((*Server).referenceResolve, builtInTool{name: "reference.resolve", title: "Resolve Reference Records", description: "Resolve effective records for one reference type.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"type_key": map[string]any{"type": "string"}, "organization_id": map[string]any{"type": "string"}, "location_id": map[string]any{"type": "string"}}, "required": []string{"type_key"}}}),
			mustBuiltInToolRegistration((*Server).referenceRecordUpsert, builtInTool{name: "reference.record.upsert", title: "Upsert Reference Record", description: "Create or update a reference record. Requires confirmation.", permission: "configuration.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"record": map[string]any{"type": "object"}, "confirm_apply": map[string]any{"type": "boolean"}}, "required": []string{"record", "confirm_apply"}}}),
			mustBuiltInToolRegistration((*Server).businessReferenceTypeList, builtInTool{name: "business.reference.type.list", title: "List Business Reference Types", description: "List business reference types across modules.", permission: "configuration.read"}),
			mustBuiltInToolRegistration((*Server).businessReferenceResolve, builtInTool{name: "business.reference.resolve", title: "Resolve Business Reference Records", description: "Resolve effective business reference records for one type.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"type_key": map[string]any{"type": "string"}, "organization_id": map[string]any{"type": "string"}, "location_id": map[string]any{"type": "string"}}, "required": []string{"type_key"}}}),
		)
	}
	if s != nil && s.integration != nil {
		registry = append(registry,
			mustBuiltInToolRegistration((*Server).integrationAdapterList, builtInTool{name: "integration.adapter.list", title: "List Integration Adapters", description: "List registered integration adapters and config schema.", permission: "configuration.read"}),
			mustBuiltInToolRegistration((*Server).integrationSystemList, builtInTool{name: "integration.system.list", title: "List Integration Systems", description: "List integration systems.", permission: "configuration.read"}),
			mustBuiltInToolRegistration((*Server).integrationSystemConfigGet, builtInTool{name: "integration.system.config.get", title: "Get Integration System Config", description: "Inspect one integration system config.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"system_key": map[string]any{"type": "string"}}, "required": []string{"system_key"}}}),
			mustBuiltInToolRegistration((*Server).integrationSystemConfigUpdate, builtInTool{name: "integration.system.config.update", title: "Update Integration System Config", description: "Update integration system config. Requires confirmation.", permission: "configuration.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"system_key": map[string]any{"type": "string"}, "settings": map[string]any{"type": "object"}, "confirm_apply": map[string]any{"type": "boolean"}}, "required": []string{"system_key", "settings", "confirm_apply"}}}),
			mustBuiltInToolRegistration((*Server).integrationEndpointList, builtInTool{name: "integration.endpoint.list", title: "List Integration Endpoints", description: "List integration endpoints.", permission: "configuration.read"}),
			mustBuiltInToolRegistration((*Server).integrationEndpointConfigGet, builtInTool{name: "integration.endpoint.config.get", title: "Get Integration Endpoint Config", description: "Inspect one integration endpoint config.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"endpoint_key": map[string]any{"type": "string"}}, "required": []string{"endpoint_key"}}}),
			mustBuiltInToolRegistration((*Server).integrationEndpointConfigUpdate, builtInTool{name: "integration.endpoint.config.update", title: "Update Integration Endpoint Config", description: "Update integration endpoint config. Requires confirmation.", permission: "configuration.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"endpoint_key": map[string]any{"type": "string"}, "settings": map[string]any{"type": "object"}, "confirm_apply": map[string]any{"type": "boolean"}}, "required": []string{"endpoint_key", "settings", "confirm_apply"}}}),
			mustBuiltInToolRegistration((*Server).integrationSubmissionList, builtInTool{name: "integration.submission.list", title: "List Integration Submissions", description: "List integration submissions.", permission: "configuration.read"}),
			mustBuiltInToolRegistration((*Server).integrationSubmissionGet, builtInTool{name: "integration.submission.get", title: "Get Integration Submission", description: "Inspect one integration submission.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"submission_id": map[string]any{"type": "string"}}, "required": []string{"submission_id"}}}),
			mustBuiltInToolRegistration((*Server).integrationSubmissionCreate, builtInTool{name: "integration.submission.create", title: "Create Integration Submission", description: "Create and queue a new integration submission. Requires confirmation.", permission: "configuration.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"system_key": map[string]any{"type": "string"}, "operation_type": map[string]any{"type": "string"}, "document_id": map[string]any{"type": "string"}, "correlation_id": map[string]any{"type": "string"}, "payload": map[string]any{"type": "object"}, "confirm_apply": map[string]any{"type": "boolean"}}, "required": []string{"system_key", "operation_type", "confirm_apply"}}}),
			mustBuiltInToolRegistration((*Server).integrationDeadLetterList, builtInTool{name: "integration.dead_letter.list", title: "List Integration Dead Letters", description: "List integration dead letters.", permission: "configuration.read"}),
			mustBuiltInToolRegistration((*Server).integrationDeadLetterReplay, builtInTool{name: "integration.dead_letter.replay", title: "Replay Integration Dead Letter", description: "Replay one integration dead letter. Requires confirmation.", permission: "configuration.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"dead_letter_id": map[string]any{"type": "string"}, "confirm_apply": map[string]any{"type": "boolean"}}, "required": []string{"dead_letter_id", "confirm_apply"}}}),
		)
	}
	if s != nil && s.documents != nil {
		registry = append(registry,
			mustBuiltInToolRegistration((*Server).documentTypeList, builtInTool{name: "document.type.list", title: "List Document Types", description: "List registered document types.", permission: "document.list"}),
			mustBuiltInToolRegistration((*Server).documentTypeGet, builtInTool{name: "document.type.get", title: "Get Document Type", description: "Get one document type definition.", permission: "document.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"document_type": map[string]any{"type": "string"}}, "required": []string{"document_type"}}}),
			mustBuiltInToolRegistration((*Server).documentList, builtInTool{name: "document.list", title: "List Documents", description: "List documents.", permission: "document.list", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"document_type": map[string]any{"type": "string"}}}}),
			mustBuiltInToolRegistration((*Server).documentGet, builtInTool{name: "document.get", title: "Get Document", description: "Get one document by ID.", permission: "document.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"document_id": map[string]any{"type": "string"}}, "required": []string{"document_id"}}}),
			mustBuiltInToolRegistration((*Server).documentCreate, builtInTool{name: "document.create", title: "Create Document", description: "Create a new document.", permission: "document.create", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"document_type": map[string]any{"type": "string"}, "payload": map[string]any{"type": "object"}, "organization_id": map[string]any{"type": "string"}, "location_id": map[string]any{"type": "string"}}, "required": []string{"document_type"}}}),
			mustBuiltInToolRegistration((*Server).documentUpdate, builtInTool{name: "document.update", title: "Update Document", description: "Update an existing document.", permission: "document.update_draft", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"document_id": map[string]any{"type": "string"}, "document": map[string]any{"type": "object"}}, "required": []string{"document_id", "document"}}}),
			mustBuiltInToolRegistration((*Server).documentDelete, builtInTool{name: "document.delete", title: "Delete Document", description: "Delete a document by ID.", permission: "configuration.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"document_id": map[string]any{"type": "string"}}, "required": []string{"document_id"}}}),
			mustBuiltInToolRegistration((*Server).businessDocumentTypeList, builtInTool{name: "business.document.type.list", title: "List Business Document Types", description: "List business document types and owning modules.", permission: "document.list", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"module_key": map[string]any{"type": "string"}}}}),
			mustBuiltInToolRegistration((*Server).businessDocumentSearch, builtInTool{name: "business.document.search", title: "Search Business Documents", description: "Search business documents by module, type, status, scope, and keyword.", permission: "document.list", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"module_key": map[string]any{"type": "string"}, "document_type": map[string]any{"type": "string"}, "status": map[string]any{"type": "string"}, "organization_id": map[string]any{"type": "string"}, "location_id": map[string]any{"type": "string"}, "query": map[string]any{"type": "string"}, "page": map[string]any{"type": "integer"}, "page_size": map[string]any{"type": "integer"}}}}),
			mustBuiltInToolRegistration((*Server).businessDocumentGet, builtInTool{name: "business.document.get", title: "Get Business Document", description: "Get one business document with filtered or full sanitized payload.", permission: "document.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"document_id": map[string]any{"type": "string"}, "module_key": map[string]any{"type": "string"}, "include_full_payload": map[string]any{"type": "boolean"}}, "required": []string{"document_id"}}}),
			mustBuiltInToolRegistration((*Server).businessDocumentDraftCreate, builtInTool{name: "business.document.draft.create", title: "Create Business Draft Document", description: "Create a business draft document after explicit confirmation.", permission: "document.create", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"module_key": map[string]any{"type": "string"}, "document_type": map[string]any{"type": "string"}, "organization_id": map[string]any{"type": "string"}, "location_id": map[string]any{"type": "string"}, "payload": map[string]any{"type": "object"}, "confirm_apply": map[string]any{"type": "boolean"}}, "required": []string{"document_type", "payload", "confirm_apply"}}}),
			mustBuiltInToolRegistration((*Server).businessDocumentDraftUpdate, builtInTool{name: "business.document.draft.update", title: "Update Business Draft Document", description: "Update an existing business draft document after explicit confirmation.", permission: "document.update_draft", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"module_key": map[string]any{"type": "string"}, "document_id": map[string]any{"type": "string"}, "payload": map[string]any{"type": "object"}, "expected_version": map[string]any{"type": "integer"}, "expected_etag": map[string]any{"type": "string"}, "confirm_apply": map[string]any{"type": "boolean"}}, "required": []string{"document_id", "payload", "confirm_apply"}}}),
			mustBuiltInToolRegistration((*Server).businessRecordSearch, builtInTool{name: "business.record.search", title: "Search Business Records", description: "Search business documents and models in one generic MCP view.", permission: "module.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"resource_kind": map[string]any{"type": "string"}, "module_key": map[string]any{"type": "string"}, "document_type": map[string]any{"type": "string"}, "model_key": map[string]any{"type": "string"}, "status": map[string]any{"type": "string"}, "filters": map[string]any{"type": "object"}, "organization_id": map[string]any{"type": "string"}, "location_id": map[string]any{"type": "string"}, "query": map[string]any{"type": "string"}, "page": map[string]any{"type": "integer"}, "page_size": map[string]any{"type": "integer"}, "include_full_payload": map[string]any{"type": "boolean"}}}}),
			mustBuiltInToolRegistration((*Server).businessRecordGet, builtInTool{name: "business.record.get", title: "Get Business Record", description: "Get one business document or model record.", permission: "module.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"resource_kind": map[string]any{"type": "string"}, "document_id": map[string]any{"type": "string"}, "model_key": map[string]any{"type": "string"}, "record_id": map[string]any{"type": "string"}, "include_full_payload": map[string]any{"type": "boolean"}}, "required": []string{"resource_kind"}}}),
			mustBuiltInToolRegistration((*Server).businessRecordRelated, builtInTool{name: "business.record.related", title: "Get Related Business Records", description: "Follow business document links or model relations.", permission: "module.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"resource_kind": map[string]any{"type": "string"}, "document_id": map[string]any{"type": "string"}, "model_key": map[string]any{"type": "string"}, "record_id": map[string]any{"type": "string"}, "relation_key": map[string]any{"type": "string"}, "filters": map[string]any{"type": "object"}, "sort_key": map[string]any{"type": "string"}, "desc": map[string]any{"type": "boolean"}, "page": map[string]any{"type": "integer"}, "page_size": map[string]any{"type": "integer"}}, "required": []string{"resource_kind"}}}),
			mustBuiltInToolRegistration((*Server).businessTimelineGet, builtInTool{name: "business.timeline.get", title: "Get Business Timeline", description: "Load audit and workflow history for a business document or model record.", permission: "module.read", contract: ContractDescriptor{BusinessDomains: []string{"cross-domain", "workflow"}}, inputSchema: map[string]any{"type": "object", "properties": map[string]any{"resource_kind": map[string]any{"type": "string"}, "document_id": map[string]any{"type": "string"}, "model_key": map[string]any{"type": "string"}, "record_id": map[string]any{"type": "string"}}, "required": []string{"resource_kind"}}}),
			mustBuiltInToolRegistration((*Server).businessRelationshipsGet, builtInTool{name: "business.relationships.get", title: "Get Business Relationships", description: "Inspect a business record together with its direct linked relationships.", permission: "module.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"resource_kind": map[string]any{"type": "string"}, "document_id": map[string]any{"type": "string"}, "model_key": map[string]any{"type": "string"}, "record_id": map[string]any{"type": "string"}, "relation_key": map[string]any{"type": "string"}, "filters": map[string]any{"type": "object"}, "sort_key": map[string]any{"type": "string"}, "desc": map[string]any{"type": "boolean"}, "page": map[string]any{"type": "integer"}, "page_size": map[string]any{"type": "integer"}}, "required": []string{"resource_kind"}}}),
			mustBuiltInToolRegistration((*Server).businessDatasetList, builtInTool{name: "business.dataset.list", title: "List Business Datasets", description: "List business datasets exposed by enabled modules.", permission: "module.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"module_key": map[string]any{"type": "string"}}}}),
			mustBuiltInToolRegistration((*Server).pricingPromotionAdvisorReview, builtInTool{name: "pricing.promotion.advisor.review", title: "Review Pricing and Promotion Setup", description: "Analyze pricing and promotion coverage, owned artifacts, and draft paths for recommendations.", permission: "module.read", contract: ContractDescriptor{ActionClass: "analyze", RiskClass: "low", GovernanceTags: []string{"advisor-pack", "business-comprehension"}, BusinessDomains: []string{"pricing", "commercial"}}, inputSchema: map[string]any{"type": "object", "properties": map[string]any{"organization_id": map[string]any{"type": "string"}, "location_id": map[string]any{"type": "string"}}}}),
			mustBuiltInToolRegistration((*Server).taxStructureAdvisorReview, builtInTool{name: "tax.structure.advisor.review", title: "Review Tax Structure", description: "Analyze tax-related module coverage, records, and recommended draft paths.", permission: "module.read", contract: ContractDescriptor{ActionClass: "analyze", RiskClass: "low", GovernanceTags: []string{"advisor-pack", "business-comprehension"}, BusinessDomains: []string{"tax", "finance"}}, inputSchema: map[string]any{"type": "object", "properties": map[string]any{"organization_id": map[string]any{"type": "string"}, "location_id": map[string]any{"type": "string"}}}}),
			mustBuiltInToolRegistration((*Server).treasuryReconciliationAdvisorReview, builtInTool{name: "treasury.reconciliation.advisor.review", title: "Review Treasury and Reconciliation", description: "Analyze treasury and reconciliation coverage, exceptions, and follow-up investigation tools.", permission: "module.read", contract: ContractDescriptor{ActionClass: "analyze", RiskClass: "low", GovernanceTags: []string{"advisor-pack", "business-comprehension"}, BusinessDomains: []string{"treasury", "finance"}}, inputSchema: map[string]any{"type": "object", "properties": map[string]any{"organization_id": map[string]any{"type": "string"}, "location_id": map[string]any{"type": "string"}}}}),
			mustBuiltInToolRegistration((*Server).inventoryHealthAdvisorReview, builtInTool{name: "inventory.health.advisor.review", title: "Review Inventory Health", description: "Analyze inventory, warehouse, and production-oriented business coverage and health signals.", permission: "module.read", contract: ContractDescriptor{ActionClass: "analyze", RiskClass: "low", GovernanceTags: []string{"advisor-pack", "business-comprehension"}, BusinessDomains: []string{"inventory", "operations"}}, inputSchema: map[string]any{"type": "object", "properties": map[string]any{"organization_id": map[string]any{"type": "string"}, "location_id": map[string]any{"type": "string"}}}}),
			mustBuiltInToolRegistration((*Server).partyMasterAdvisorReview, builtInTool{name: "party.master.advisor.review", title: "Review Party Master", description: "Analyze customer, vendor, contact, and address master coverage and data quality follow-up paths.", permission: "module.read", contract: ContractDescriptor{ActionClass: "analyze", RiskClass: "low", GovernanceTags: []string{"advisor-pack", "business-comprehension"}, BusinessDomains: []string{"party", "masterdata"}}, inputSchema: map[string]any{"type": "object", "properties": map[string]any{"organization_id": map[string]any{"type": "string"}, "location_id": map[string]any{"type": "string"}}}}),
			mustBuiltInToolRegistration((*Server).businessAnalyticsOverview, builtInTool{
				name:        "business.analytics.overview",
				title:       "Get Cross-Domain Analytics Overview",
				description: "Return a scoped cross-domain analytical summary with KPI cards, anomalies, exceptions, and drilldowns.",
				permission:  "module.read",
				contract: ContractDescriptor{
					ActionClass:    "analyze",
					RiskClass:      "low",
					GovernanceTags: []string{"business-comprehension", "analytics"},
					BusinessDomains: []string{
						"cross-domain",
						"analytics",
					},
				},
				inputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"organization_id":   map[string]any{"type": "string"},
						"location_id":       map[string]any{"type": "string"},
						"operating_unit_id": map[string]any{"type": "string"},
						"domain":            map[string]any{"type": "string"},
					},
				},
			}),
			mustBuiltInToolRegistration((*Server).businessAnalyticsAnomalySearch, builtInTool{
				name:        "business.analytics.anomaly.search",
				title:       "Search Analytical Anomalies",
				description: "Identify cross-domain anomaly signals such as backlog spikes, rejection pressure, audit gaps, or master-data issues.",
				permission:  "module.read",
				contract: ContractDescriptor{
					ActionClass:    "analyze",
					RiskClass:      "low",
					GovernanceTags: []string{"business-comprehension", "analytics"},
					BusinessDomains: []string{
						"cross-domain",
						"analytics",
					},
				},
				inputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"organization_id":   map[string]any{"type": "string"},
						"location_id":       map[string]any{"type": "string"},
						"operating_unit_id": map[string]any{"type": "string"},
						"domain":            map[string]any{"type": "string"},
						"page":              map[string]any{"type": "integer"},
						"page_size":         map[string]any{"type": "integer"},
					},
				},
			}),
			mustBuiltInToolRegistration((*Server).businessAnalyticsExceptionCluster, builtInTool{
				name:        "business.analytics.exception.cluster",
				title:       "Cluster Business Exceptions",
				description: "Group open business exceptions by area, status, severity, and aging for cross-domain investigation.",
				permission:  "module.read",
				contract: ContractDescriptor{
					ActionClass:    "analyze",
					RiskClass:      "low",
					GovernanceTags: []string{"business-comprehension", "analytics"},
					BusinessDomains: []string{
						"cross-domain",
						"analytics",
					},
				},
				inputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"status":   map[string]any{"type": "string"},
						"group_by": map[string]any{"type": "string"},
					},
				},
			}),
			mustBuiltInToolRegistration((*Server).businessAnalyticsDrilldown, builtInTool{
				name:        "business.analytics.drilldown",
				title:       "Resolve Analytical Drilldown",
				description: "Resolve a stable analytical drilldown handle into the next MCP tool and arguments.",
				permission:  "module.read",
				contract: ContractDescriptor{
					ActionClass:    "analyze",
					RiskClass:      "low",
					GovernanceTags: []string{"business-comprehension", "analytics"},
					BusinessDomains: []string{
						"cross-domain",
						"analytics",
					},
				},
				inputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"handle": map[string]any{"type": "string"},
					},
					"required": []string{"handle"},
				},
			}),
			mustBuiltInToolRegistration((*Server).businessAnalyticsDomainSummary, builtInTool{
				name:        "business.analytics.domain.summary",
				title:       "Get Domain Analytics Summary",
				description: "Return a scoped analytical summary for one business domain with KPI cards and investigation drilldowns.",
				permission:  "module.read",
				contract: ContractDescriptor{
					ActionClass:    "analyze",
					RiskClass:      "low",
					GovernanceTags: []string{"business-comprehension", "analytics"},
					BusinessDomains: []string{
						"cross-domain",
						"analytics",
					},
				},
				inputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"domain":            map[string]any{"type": "string"},
						"organization_id":   map[string]any{"type": "string"},
						"location_id":       map[string]any{"type": "string"},
						"operating_unit_id": map[string]any{"type": "string"},
					},
					"required": []string{"domain"},
				},
			}),
		)
	}
	if s != nil && s.analytics != nil {
		registry = append(registry,
			mustBuiltInToolRegistration((*Server).businessAnalyticsKPISummary, builtInTool{name: "business.analytics.kpi.summary", title: "Get Business KPI Summary", description: "Return current and recent analytics KPIs for business investigation.", permission: "analytics.read", contract: ContractDescriptor{ActionClass: "analyze", RiskClass: "low", GovernanceTags: []string{"business-comprehension"}, BusinessDomains: []string{"cross-domain", "analytics"}}}),
			mustBuiltInToolRegistration((*Server).businessAnalyticsTrend, builtInTool{
				name:        "business.analytics.trend",
				title:       "Get Business Analytics Trend",
				description: "Return scoped time-series analytics with grouped trend points and drilldowns.",
				permission:  "analytics.read",
				contract: ContractDescriptor{
					ActionClass:    "analyze",
					RiskClass:      "low",
					GovernanceTags: []string{"business-comprehension", "analytics"},
					BusinessDomains: []string{
						"cross-domain",
						"analytics",
					},
				},
				inputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"date_from": map[string]any{"type": "string"},
						"date_to":   map[string]any{"type": "string"},
						"bucket":    map[string]any{"type": "string"},
						"limit":     map[string]any{"type": "integer"},
					},
				},
			}),
		)
	}
	return registry
}
