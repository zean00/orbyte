package app

import (
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/module"
)

func productionCostingCoreKernelPackManifests() []module.Manifest {
	return []module.Manifest{productionCostingCoreKernelPackManifest()}
}

func productionCostingCoreKernelPackManifest() module.Manifest {
	return module.Manifest{
		Key:                    "production_costing_core",
		Name:                   "Production Costing Core",
		NameI18n:               localize("Production Costing Core", "Inti Costing Produksi"),
		Version:                "1.0.0",
		DomainFamily:           "business",
		DependencyRequirements: requiredModuleDependencies("production_core", "inventory_core", "finance_reporting_core"),
		AdminConsole: module.AdminConsoleDefinition{
			Title:           "Production Costing Console",
			TitleI18n:       localize("Production Costing Console", "Konsol Costing Produksi"),
			Description:     "Routing standards, production cost capture, allocations, and variance analysis.",
			DescriptionI18n: localize("Routing standards, production cost capture, allocations, and variance analysis.", "Standar routing, capture biaya produksi, alokasi output, dan analisis variance."),
			Sections: []module.AdminConsoleSectionDefinition{
				{
					Key:       "setup",
					Title:     "Setup",
					TitleI18n: localize("Setup", "Pengaturan"),
					Kind:      module.AdminConsoleSectionResourceLinks,
					Links: []module.AdminConsoleLinkDefinition{
						adminConsoleLink("routings", "Production Routings", "Routing Produksi", "/ui/production/routings", "Manage production routings.", "Kelola routing produksi.", "production_routing.list"),
						adminConsoleLink("routing_steps", "Routing Steps", "Langkah Routing", "/ui/production/routing-steps", "Manage routing steps.", "Kelola langkah routing.", "production_routing_step.list"),
						adminConsoleLink("cost_rates", "Production Cost Rates", "Tarif Biaya Produksi", "/ui/production/cost-rates", "Manage production cost rates.", "Kelola tarif biaya produksi.", "production_cost_rate.list"),
						adminConsoleLink("captures", "Production Cost Captures", "Capture Biaya Produksi", "/ui/production/cost-captures", "Review production cost captures.", "Tinjau capture biaya produksi.", "production_cost_capture.list"),
					},
				},
				{
					Key:       "reports",
					Title:     "Reports",
					TitleI18n: localize("Reports", "Laporan"),
					Kind:      module.AdminConsoleSectionResourceLinks,
					Links: []module.AdminConsoleLinkDefinition{
						adminConsoleLink("cost_summary", "Production Cost Summary", "Ringkasan Biaya Produksi", "/ui/finance/production-cost-summary", "Open production cost summary.", "Buka ringkasan biaya produksi.", "finance.read"),
						adminConsoleLink("variance", "Production Variance", "Variance Produksi", "/ui/finance/production-variance", "Open production variance report.", "Buka laporan variance produksi.", "finance.read"),
						adminConsoleLink("allocations", "Output Allocations", "Alokasi Output", "/ui/production/output-allocations", "Review output cost allocations.", "Tinjau alokasi biaya output.", "production_output_allocation.list"),
						adminConsoleLink("variance_cases", "Variance Cases", "Kasus Variance", "/ui/production/variance-cases", "Review production variance cases.", "Tinjau kasus variance produksi.", "production_variance_case.list"),
					},
				},
			},
		},
		Models: []model.Definition{
			productionCostingModelDefinition("production_routing", "Production Routing", []model.FieldDefinition{
				{Key: "organization_id", Label: "Organization", Type: "string", Required: true},
				{Key: "location_id", Label: "Location", Type: "string"},
				{Key: "code", Label: "Code", Type: "string", Required: true},
				{Key: "name", Label: "Name", Type: "string", Required: true},
				{Key: "produced_item_code", Label: "Produced Item", Type: "string", Required: true},
				{Key: "default_output_quantity", Label: "Default Output Quantity", Type: "number"},
				{Key: "effective_start_date", Label: "Effective Start", Type: "string"},
				{Key: "effective_end_date", Label: "Effective End", Type: "string"},
				{Key: "status", Label: "Status", Type: "string", DefaultValue: "active"},
			}),
			productionCostingModelDefinition("production_routing_step", "Production Routing Step", []model.FieldDefinition{
				{Key: "organization_id", Label: "Organization", Type: "string", Required: true},
				{Key: "location_id", Label: "Location", Type: "string"},
				{Key: "routing_id", Label: "Routing", Type: "string", Required: true},
				{Key: "sequence", Label: "Sequence", Type: "int", Required: true},
				{Key: "stage_code", Label: "Stage Code", Type: "string"},
				{Key: "work_center_code", Label: "Work Center", Type: "string"},
				{Key: "cost_driver", Label: "Cost Driver", Type: "string", DefaultValue: "labor"},
				{Key: "standard_quantity", Label: "Standard Quantity", Type: "number"},
				{Key: "standard_rate", Label: "Standard Rate", Type: "number"},
				{Key: "status", Label: "Status", Type: "string", DefaultValue: "active"},
			}),
			productionCostingModelDefinition("production_cost_rate", "Production Cost Rate", []model.FieldDefinition{
				{Key: "organization_id", Label: "Organization", Type: "string", Required: true},
				{Key: "location_id", Label: "Location", Type: "string"},
				{Key: "work_center_code", Label: "Work Center", Type: "string", Required: true},
				{Key: "rate_type", Label: "Rate Type", Type: "string", Required: true},
				{Key: "standard_rate", Label: "Standard Rate", Type: "number", Required: true},
				{Key: "effective_start_date", Label: "Effective Start", Type: "string"},
				{Key: "effective_end_date", Label: "Effective End", Type: "string"},
				{Key: "status", Label: "Status", Type: "string", DefaultValue: "active"},
			}),
			productionCostingModelDefinition("production_cost_capture", "Production Cost Capture", []model.FieldDefinition{
				{Key: "organization_id", Label: "Organization", Type: "string", Required: true},
				{Key: "location_id", Label: "Location", Type: "string"},
				{Key: "production_order_id", Label: "Production Order", Type: "string", Required: true},
				{Key: "work_center_code", Label: "Work Center", Type: "string"},
				{Key: "employee_id", Label: "Employee", Type: "string"},
				{Key: "roster_slot_id", Label: "Roster Slot", Type: "string"},
				{Key: "attendance_day_id", Label: "Attendance Day", Type: "string"},
				{Key: "capture_type", Label: "Capture Type", Type: "string", Required: true},
				{Key: "source", Label: "Source", Type: "string", DefaultValue: "manual_entry"},
				{Key: "capture_date", Label: "Capture Date", Type: "string"},
				{Key: "quantity", Label: "Quantity", Type: "number"},
				{Key: "actual_rate", Label: "Actual Rate", Type: "number"},
				{Key: "actual_cost", Label: "Actual Cost", Type: "number"},
				{Key: "credit_account_code", Label: "Credit Account", Type: "string"},
				{Key: "posting_id", Label: "Posting ID", Type: "string"},
				{Key: "status", Label: "Status", Type: "string", DefaultValue: "approved"},
			}),
			productionCostingModelDefinition("production_variance_case", "Production Variance Case", []model.FieldDefinition{
				{Key: "organization_id", Label: "Organization", Type: "string", Required: true},
				{Key: "location_id", Label: "Location", Type: "string"},
				{Key: "production_order_id", Label: "Production Order", Type: "string", Required: true},
				{Key: "order_number", Label: "Order Number", Type: "string"},
				{Key: "finished_item_code", Label: "Finished Item", Type: "string"},
				{Key: "variance_type", Label: "Variance Type", Type: "string"},
				{Key: "amount", Label: "Amount", Type: "number"},
				{Key: "assignee", Label: "Assignee", Type: "string"},
				{Key: "status", Label: "Status", Type: "string", DefaultValue: "open"},
				{Key: "notes", Label: "Notes", Type: "string"},
			}),
			productionCostingModelDefinition("production_output_allocation", "Production Output Allocation", []model.FieldDefinition{
				{Key: "organization_id", Label: "Organization", Type: "string", Required: true},
				{Key: "location_id", Label: "Location", Type: "string"},
				{Key: "source_production_output_id", Label: "Production Output", Type: "string", Required: true},
				{Key: "production_order_id", Label: "Production Order", Type: "string", Required: true},
				{Key: "output_item_code", Label: "Output Item", Type: "string", Required: true},
				{Key: "output_item_name", Label: "Output Name", Type: "string"},
				{Key: "warehouse_code", Label: "Warehouse", Type: "string"},
				{Key: "output_quantity", Label: "Output Quantity", Type: "number"},
				{Key: "allocation_basis", Label: "Allocation Basis", Type: "string"},
				{Key: "allocation_share_percent", Label: "Share Percent", Type: "number"},
				{Key: "allocated_total_cost", Label: "Allocated Total Cost", Type: "number"},
				{Key: "allocated_unit_cost", Label: "Allocated Unit Cost", Type: "number"},
				{Key: "output_date", Label: "Output Date", Type: "string"},
				{Key: "status", Label: "Status", Type: "string", DefaultValue: "posted"},
			}),
		},
		Security: module.SecurityDefinition{
			Permissions: append(
				append(
					append(
						commercialModelPermissions("production_routing", "Production Routing"),
						commercialModelPermissions("production_routing_step", "Production Routing Step")...,
					),
					append(
						commercialModelPermissions("production_cost_rate", "Production Cost Rate"),
						append(
							commercialModelPermissions("production_cost_capture", "Production Cost Capture"),
							append(
								commercialModelPermissions("production_variance_case", "Production Variance Case"),
								commercialModelPermissions("production_output_allocation", "Production Output Allocation")...,
							)...,
						)...,
					)...,
				),
				module.PermissionDefinition{
					Key:             "production.costing.manage",
					Action:          "manage",
					Resource:        "production_costing",
					DisplayName:     "Manage Production Costing",
					DisplayNameI18n: localize("Manage Production Costing", "Kelola Costing Produksi"),
				},
			),
			RoleTemplates: []module.RoleTemplateDefinition{
				{
					Key:           "production_cost_manager",
					Name:          "Production Cost Manager",
					NameI18n:      localize("Production Cost Manager", "Manajer Cost Produksi"),
					AllowedScopes: []string{"deployment", "organization", "location"},
					PermissionKeys: []string{
						"production_routing.create", "production_routing.list", "production_routing.read", "production_routing.update",
						"production_routing_step.create", "production_routing_step.list", "production_routing_step.read", "production_routing_step.update",
						"production_cost_rate.create", "production_cost_rate.list", "production_cost_rate.read", "production_cost_rate.update",
						"production_cost_capture.create", "production_cost_capture.list", "production_cost_capture.read", "production_cost_capture.update",
						"production_variance_case.create", "production_variance_case.list", "production_variance_case.read", "production_variance_case.update",
						"production_output_allocation.create", "production_output_allocation.list", "production_output_allocation.read", "production_output_allocation.update",
						"production.costing.manage", "finance.read", "document.read",
					},
				},
			},
		},
		Frontend: module.FrontendDefinition{
			Menus: []module.MenuDefinition{
				{Key: "production.routings", Label: "Production Routings", LabelI18n: localize("Production Routings", "Routing Produksi"), ActionKey: "production.routings.list", Order: 65, RequiredPermissions: []string{"production_routing.list"}},
				{Key: "production.cost_rates", Label: "Production Cost Rates", LabelI18n: localize("Production Cost Rates", "Tarif Biaya Produksi"), ActionKey: "production.cost_rates.list", Order: 66, RequiredPermissions: []string{"production_cost_rate.list"}},
				{Key: "production.cost_captures", Label: "Production Cost Captures", LabelI18n: localize("Production Cost Captures", "Capture Biaya Produksi"), ActionKey: "production.cost_captures.list", Order: 67, RequiredPermissions: []string{"production_cost_capture.list"}},
				{Key: "production.output_allocations", Label: "Production Output Allocations", LabelI18n: localize("Production Output Allocations", "Alokasi Output Produksi"), ActionKey: "production.output_allocations.list", Order: 68, RequiredPermissions: []string{"production_output_allocation.list"}},
				{Key: "production.variance_cases", Label: "Production Variance Cases", LabelI18n: localize("Production Variance Cases", "Kasus Variance Produksi"), ActionKey: "production.variance_cases.list", Order: 69, RequiredPermissions: []string{"production_variance_case.list"}},
				{Key: "finance.production_cost_summary", Label: "Production Cost Summary", LabelI18n: localize("Production Cost Summary", "Ringkasan Biaya Produksi"), ActionKey: "finance.production_cost_summary", Order: 145, RequiredPermissions: []string{"finance.read"}},
				{Key: "finance.production_variance", Label: "Production Variance", LabelI18n: localize("Production Variance", "Variance Produksi"), ActionKey: "finance.production_variance", Order: 146, RequiredPermissions: []string{"finance.read"}},
			},
			Actions: []module.ActionDefinition{
				{Key: "production.routings.list", Label: "Production Routings", Kind: "navigate", RoutePath: "/production/routings", ViewKey: "production.routings.list", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"production_routing.list"}},
				{Key: "production.routings.detail", Label: "Production Routing Detail", Kind: "navigate", RoutePath: "/production/routings/detail", ViewKey: "production.routings.detail", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"production_routing.read"}},
				{Key: "production.routings.form", Label: "Production Routing Form", Kind: "navigate", RoutePath: "/production/routings/form", ViewKey: "production.routings.form", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"production_routing.update"}},
				{Key: "production.routing_steps.list", Label: "Production Routing Steps", Kind: "navigate", RoutePath: "/production/routing-steps", ViewKey: "production.routing_steps.list", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"production_routing_step.list"}},
				{Key: "production.cost_rates.list", Label: "Production Cost Rates", Kind: "navigate", RoutePath: "/production/cost-rates", ViewKey: "production.cost_rates.list", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"production_cost_rate.list"}},
				{Key: "production.cost_captures.list", Label: "Production Cost Captures", Kind: "navigate", RoutePath: "/production/cost-captures", ViewKey: "production.cost_captures.list", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"production_cost_capture.list"}},
				{Key: "production.output_allocations.list", Label: "Production Output Allocations", Kind: "navigate", RoutePath: "/production/output-allocations", ViewKey: "production.output_allocations.list", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"production_output_allocation.list"}},
				{Key: "production.variance_cases.list", Label: "Production Variance Cases", Kind: "navigate", RoutePath: "/production/variance-cases", ViewKey: "production.variance_cases.list", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"production_variance_case.list"}},
				{Key: "finance.production_cost_summary", Label: "Production Cost Summary", Kind: "navigate", RoutePath: "/finance/production-cost-summary", CustomEntryKey: "finance.production_cost_summary", RenderMode: module.RenderModeCustom, RequiredPermissions: []string{"finance.read"}},
				{Key: "finance.production_variance", Label: "Production Variance", Kind: "navigate", RoutePath: "/finance/production-variance", CustomEntryKey: "finance.production_variance", RenderMode: module.RenderModeCustom, RequiredPermissions: []string{"finance.read"}},
			},
			Views: []module.ViewDefinition{
				commercialModelListView("production.routings.list", "Production Routings", "production_routing", []module.ColumnDefinition{
					{Key: "code", Label: "Code", Path: "values.code"},
					{Key: "name", Label: "Name", Path: "values.name"},
					{Key: "produced_item_code", Label: "Produced Item", Path: "values.produced_item_code"},
					{Key: "status", Label: "Status", Path: "values.status"},
				}, []string{"active", "inactive"}),
				commercialModelDetailView("production.routings.detail", "Production Routing Detail", "production_routing", []module.FieldDefinition{
					{Key: "code", Label: "Code", Path: "values.code", Type: "string"},
					{Key: "name", Label: "Name", Path: "values.name", Type: "string"},
					{Key: "produced_item_code", Label: "Produced Item", Path: "values.produced_item_code", Type: "string"},
					{Key: "default_output_quantity", Label: "Default Output Qty", Path: "values.default_output_quantity", Type: "number"},
					{Key: "status", Label: "Status", Path: "values.status", Type: "string"},
				}),
				commercialModelFormView("production.routings.form", "Production Routing Form", "production_routing", []module.FieldDefinition{
					{Key: "organization_id", Label: "Organization", Path: "values.organization_id", Type: "string", Widget: "text", Required: true},
					{Key: "location_id", Label: "Location", Path: "values.location_id", Type: "string", Widget: "text"},
					{Key: "code", Label: "Code", Path: "values.code", Type: "string", Widget: "text", Required: true},
					{Key: "name", Label: "Name", Path: "values.name", Type: "string", Widget: "text", Required: true},
					{Key: "produced_item_code", Label: "Produced Item", Path: "values.produced_item_code", Type: "string", Widget: "text", Required: true},
					{Key: "default_output_quantity", Label: "Default Output Qty", Path: "values.default_output_quantity", Type: "number", Widget: "text"},
					{Key: "status", Label: "Status", Path: "values.status", Type: "string", Widget: "select", Options: []string{"active", "inactive"}},
				}),
				commercialModelListView("production.routing_steps.list", "Production Routing Steps", "production_routing_step", []module.ColumnDefinition{
					{Key: "routing_id", Label: "Routing", Path: "values.routing_id"},
					{Key: "sequence", Label: "Sequence", Path: "values.sequence"},
					{Key: "work_center_code", Label: "Work Center", Path: "values.work_center_code"},
					{Key: "cost_driver", Label: "Driver", Path: "values.cost_driver"},
					{Key: "standard_rate", Label: "Std Rate", Path: "values.standard_rate"},
				}, []string{"active", "inactive"}),
				commercialModelListView("production.cost_rates.list", "Production Cost Rates", "production_cost_rate", []module.ColumnDefinition{
					{Key: "work_center_code", Label: "Work Center", Path: "values.work_center_code"},
					{Key: "rate_type", Label: "Rate Type", Path: "values.rate_type"},
					{Key: "standard_rate", Label: "Rate", Path: "values.standard_rate"},
					{Key: "status", Label: "Status", Path: "values.status"},
				}, []string{"active", "inactive"}),
				commercialModelListView("production.cost_captures.list", "Production Cost Captures", "production_cost_capture", []module.ColumnDefinition{
					{Key: "production_order_id", Label: "Order", Path: "values.production_order_id"},
					{Key: "employee_id", Label: "Employee", Path: "values.employee_id"},
					{Key: "roster_slot_id", Label: "Roster Slot", Path: "values.roster_slot_id"},
					{Key: "capture_type", Label: "Type", Path: "values.capture_type"},
					{Key: "quantity", Label: "Qty", Path: "values.quantity"},
					{Key: "actual_cost", Label: "Cost", Path: "values.actual_cost"},
					{Key: "status", Label: "Status", Path: "values.status"},
				}, []string{"draft", "approved", "posted", "cancelled"}),
				commercialModelListView("production.output_allocations.list", "Production Output Allocations", "production_output_allocation", []module.ColumnDefinition{
					{Key: "source_production_output_id", Label: "Output", Path: "values.source_production_output_id"},
					{Key: "output_item_code", Label: "Output Item", Path: "values.output_item_code"},
					{Key: "output_quantity", Label: "Qty", Path: "values.output_quantity"},
					{Key: "allocated_total_cost", Label: "Allocated Cost", Path: "values.allocated_total_cost"},
					{Key: "allocated_unit_cost", Label: "Unit Cost", Path: "values.allocated_unit_cost"},
				}, []string{"posted"}),
				commercialModelListView("production.variance_cases.list", "Production Variance Cases", "production_variance_case", []module.ColumnDefinition{
					{Key: "order_number", Label: "Order", Path: "values.order_number"},
					{Key: "finished_item_code", Label: "Finished Item", Path: "values.finished_item_code"},
					{Key: "variance_type", Label: "Type", Path: "values.variance_type"},
					{Key: "amount", Label: "Amount", Path: "values.amount"},
					{Key: "status", Label: "Status", Path: "values.status"},
				}, []string{"open", "investigating", "corrected", "closed"}),
			},
			CustomEntries: []module.CustomEntryDefinition{
				{Key: "finance.production_cost_summary", Title: "Production Cost Summary", TitleI18n: localize("Production Cost Summary", "Ringkasan Biaya Produksi"), RoutePath: "/finance/production-cost-summary", BundleKey: "finance-reports", ComponentExport: "render", RequiredPermissions: []string{"finance.read"}},
				{Key: "finance.production_variance", Title: "Production Variance", TitleI18n: localize("Production Variance", "Variance Produksi"), RoutePath: "/finance/production-variance", BundleKey: "finance-reports", ComponentExport: "render", RequiredPermissions: []string{"finance.read"}},
			},
		},
	}
}

func productionCostingModelDefinition(key, singular string, fields []model.FieldDefinition) model.Definition {
	return model.Definition{
		Key:                 key,
		DisplayName:         singular,
		DisplayNameI18n:     localize(singular, singular),
		OwnerModuleKey:      "production_costing_core",
		Version:             "v1",
		CreatePermissionKey: key + ".create",
		ListPermissionKey:   key + ".list",
		ReadPermissionKey:   key + ".read",
		UpdatePermissionKey: key + ".update",
		DefaultSort:         fields[0].Key,
		Fields:              fields,
	}
}
