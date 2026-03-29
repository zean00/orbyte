package app

import (
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/search"
	"orbyte/internal/platform/workflow"
)

func recallCoreKernelPackManifests() []module.Manifest {
	return []module.Manifest{recallCoreKernelPackManifest()}
}

func recallCoreKernelPackManifest() module.Manifest {
	return module.Manifest{
		Key:          "recall_core",
		Name:         "Recall Core",
		NameI18n:     localize("Recall Core", "Inti Recall"),
		Version:      "1.0.0",
		DomainFamily: "business",
		OwnedDocumentTypes: []string{
			"recall_case",
			"recall_action",
		},
		OwnedWorkflowKeys: []string{
			"recall_case_flow",
			"recall_action_flow",
		},
		OwnedTemplateKeys: []string{
			"recall.recall_case.print.default",
			"recall.recall_action.print.default",
		},
		DependencyRequirements: []module.DependencyRequirement{
			{ModuleKey: "platform.core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "inventory_core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "traceability_core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
		},
		AdminConsole: module.AdminConsoleDefinition{
			Title:           "Recall Console",
			TitleI18n:       localize("Recall Console", "Konsol Recall"),
			Description:     "Create recall cases, contain affected batches, and work generated internal actions.",
			DescriptionI18n: localize("Create recall cases, contain affected batches, and work generated internal actions.", "Buat kasus recall, lakukan containment pada batch terdampak, dan kerjakan aksi internal yang dihasilkan."),
			Sections: []module.AdminConsoleSectionDefinition{
				{
					Key:       "operations",
					Title:     "Recall Operations",
					TitleI18n: localize("Recall Operations", "Operasi Recall"),
					Kind:      module.AdminConsoleSectionResourceLinks,
					Links: []module.AdminConsoleLinkDefinition{
						adminConsoleLink("cases", "Recall Cases", "Kasus Recall", "/ui/recall/cases", "Open recall cases.", "Buka kasus recall.", "document.list"),
						adminConsoleLink("actions", "Recall Actions", "Aksi Recall", "/ui/recall/actions", "Open generated recall actions.", "Buka aksi recall yang dihasilkan.", "document.list"),
						adminConsoleLink("batches", "Batches", "Batch", "/ui/inventory/batches", "Open affected batches.", "Buka batch yang terdampak.", "inventory_batch.list"),
					},
				},
				{
					Key:       "workflows",
					Title:     "Recall Workflows",
					TitleI18n: localize("Recall Workflows", "Workflow Recall"),
					Kind:      module.AdminConsoleSectionWorkflowLinks,
					Links: []module.AdminConsoleLinkDefinition{
						adminConsoleLink("recall_case_flow", "Recall Case Workflow", "Workflow Kasus Recall", "/admin/workflows/designer?key=recall_case_flow", "Open the recall-case workflow.", "Buka workflow kasus recall.", "workflow.read"),
						adminConsoleLink("recall_action_flow", "Recall Action Workflow", "Workflow Aksi Recall", "/admin/workflows/designer?key=recall_action_flow", "Open the recall-action workflow.", "Buka workflow aksi recall.", "workflow.read"),
					},
				},
			},
		},
		Documents: []document.Definition{
			{
				Type:                   "recall_case",
				DisplayName:            "Recall Case",
				SchemaVersion:          "v1",
				WorkflowKey:            "recall_case_flow",
				NumberingKey:           "recall_case_number",
				OwnerModuleKey:         "recall_core",
				AllowedLinkTypes:       []string{"recall_for", "related_to"},
				AllowedAttachmentTypes: []string{"note", "image", "document"},
			},
			{
				Type:                   "recall_action",
				DisplayName:            "Recall Action",
				SchemaVersion:          "v1",
				WorkflowKey:            "recall_action_flow",
				NumberingKey:           "recall_action_number",
				OwnerModuleKey:         "recall_core",
				AllowedLinkTypes:       []string{"recall_for", "related_to"},
				AllowedAttachmentTypes: []string{"note", "image", "document"},
			},
		},
		Workflows: []workflow.Definition{
			{
				Key:    "recall_case_flow",
				States: []string{"draft", "submitted", "active", "rejected", "cancelled"},
				Actions: []workflow.ActionRule{
					{Action: "submit", FromState: "draft", ToState: "submitted", PermissionKey: "document.submit"},
					{Action: "approve", FromState: "submitted", ToState: "active", PermissionKey: "document.approve"},
					{Action: "reject", FromState: "submitted", ToState: "rejected", PermissionKey: "document.reject"},
					{Action: "reopen", FromState: "rejected", ToState: "draft", PermissionKey: "document.reopen"},
					{Action: "cancel", FromState: "draft", ToState: "cancelled", PermissionKey: "document.cancel"},
					{Action: "cancel", FromState: "submitted", ToState: "cancelled", PermissionKey: "document.cancel"},
				},
			},
			{
				Key:    "recall_action_flow",
				States: []string{"draft", "submitted", "completed", "rejected", "cancelled"},
				Actions: []workflow.ActionRule{
					{Action: "submit", FromState: "draft", ToState: "submitted", PermissionKey: "document.submit"},
					{Action: "approve", FromState: "submitted", ToState: "completed", PermissionKey: "document.approve"},
					{Action: "reject", FromState: "submitted", ToState: "rejected", PermissionKey: "document.reject"},
					{Action: "reopen", FromState: "rejected", ToState: "draft", PermissionKey: "document.reopen"},
					{Action: "cancel", FromState: "draft", ToState: "cancelled", PermissionKey: "document.cancel"},
					{Action: "cancel", FromState: "submitted", ToState: "cancelled", PermissionKey: "document.cancel"},
				},
			},
		},
		SearchIndexes: []search.IndexDefinition{
			commercialDocumentSearchIndex("recall.cases.search", "Recall Case Search", "recall_case", "recall.cases.list", []search.IndexFieldDefinition{
				{Key: "number", Path: "header.number", Type: "string", Searchable: true},
				{Key: "status", Path: "header.status", Type: "string", Facet: true, Sort: true},
				{Key: "title", Path: "body.payload.title", Type: "string", Searchable: true},
				{Key: "recall_reference", Path: "body.payload.recall_reference", Type: "string", Searchable: true},
				{Key: "severity", Path: "body.payload.severity", Type: "string", Facet: true},
			}),
			commercialDocumentSearchIndex("recall.actions.search", "Recall Action Search", "recall_action", "recall.actions.list", []search.IndexFieldDefinition{
				{Key: "number", Path: "header.number", Type: "string", Searchable: true},
				{Key: "status", Path: "header.status", Type: "string", Facet: true, Sort: true},
				{Key: "action_type", Path: "body.payload.action_type", Type: "string", Facet: true, Searchable: true},
				{Key: "source_document_number", Path: "body.payload.source_document_number", Type: "string", Searchable: true},
				{Key: "batch_code", Path: "body.payload.batch_code", Type: "string", Searchable: true},
			}),
		},
		Security: module.SecurityDefinition{
			Permissions: []module.PermissionDefinition{
				{Key: "recall.read", Action: "read", Resource: "recall", DisplayName: "Read Recall", DisplayNameI18n: localize("Read Recall", "Lihat Recall")},
			},
			RoleTemplates: []module.RoleTemplateDefinition{
				{
					Key:            "recall_manager",
					Name:           "Recall Manager",
					NameI18n:       localize("Recall Manager", "Manajer Recall"),
					AllowedScopes:  []string{"deployment", "location"},
					PermissionKeys: []string{"recall.read", "inventory_batch.read", "inventory_batch.update", "document.create", "document.list", "document.read", "document.update_draft", "document.submit", "document.approve", "document.reject", "document.reopen", "document.cancel"},
				},
			},
		},
		Frontend: module.FrontendDefinition{
			Menus: []module.MenuDefinition{
				{Key: "recall.cases", Label: "Recall Cases", LabelI18n: localize("Recall Cases", "Kasus Recall"), ActionKey: "recall.cases.list", Order: 65, RequiredPermissions: []string{"document.list"}},
				{Key: "recall.actions", Label: "Recall Actions", LabelI18n: localize("Recall Actions", "Aksi Recall"), ActionKey: "recall.actions.list", Order: 66, RequiredPermissions: []string{"document.list"}},
			},
			Actions: append(
				commercialDocumentActions("recall.cases", "recall_case", "Recall Cases", "Recall Case", "New Recall Case", "/recall/cases"),
				commercialDocumentActions("recall.actions", "recall_action", "Recall Actions", "Recall Action", "New Recall Action", "/recall/actions")...,
			),
			Views: append(
				recallDocumentViews("recall.cases", "recall_case", "Recall Cases", "Recall Case Detail", "Recall Case Draft", recallCaseColumns(), []string{"draft", "submitted", "active", "rejected", "cancelled"}, recallCaseDetailSections(), recallCaseFormSections()),
				recallDocumentViews("recall.actions", "recall_action", "Recall Actions", "Recall Action Detail", "Recall Action Draft", recallActionColumns(), []string{"draft", "submitted", "completed", "rejected", "cancelled"}, recallActionDetailSections(), recallActionFormSections())...,
			),
		},
		Templates: []module.TemplateDefinition{
			commercialTemplateDefinition("recall.recall_case.print.default", "Recall Case Print", "recall_case", "Recall Case", []string{"title", "reason", "severity", "recall_reference", "affected_batches", "impact_summary"}),
			commercialTemplateDefinition("recall.recall_action.print.default", "Recall Action Print", "recall_action", "Recall Action", []string{"action_type", "source_document_number", "item_code", "batch_code", "warehouse_code", "quantity"}),
		},
	}
}

func recallDocumentViews(prefix, documentType, listTitle, detailTitle, formTitle string, columns []module.ColumnDefinition, statusOptions []string, detailSections, formSections []module.SectionDefinition) []module.ViewDefinition {
	views := commercialDocumentViews(prefix, documentType, listTitle, detailTitle, formTitle, columns, statusOptions, detailSections, formSections)
	if len(views) > 1 {
		views[1].AllowedActions = []string{"submit", "approve", "reject", "reopen", "cancel"}
	}
	return views
}

func recallCaseColumns() []module.ColumnDefinition {
	return []module.ColumnDefinition{
		{Key: "number", Label: "Case", LabelI18n: localize("Case", "Kasus"), Path: "header.number"},
		{Key: "title", Label: "Title", LabelI18n: localize("Title", "Judul"), Path: "body.payload.title"},
		{Key: "recall_reference", Label: "Reference", LabelI18n: localize("Reference", "Referensi"), Path: "body.payload.recall_reference"},
		{Key: "severity", Label: "Severity", LabelI18n: localize("Severity", "Keparahan"), Path: "body.payload.severity"},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "header.status"},
	}
}

func recallActionColumns() []module.ColumnDefinition {
	return []module.ColumnDefinition{
		{Key: "number", Label: "Action", LabelI18n: localize("Action", "Aksi"), Path: "header.number"},
		{Key: "action_type", Label: "Type", LabelI18n: localize("Type", "Jenis"), Path: "body.payload.action_type"},
		{Key: "source_document_number", Label: "Source Doc", LabelI18n: localize("Source Doc", "Dokumen Sumber"), Path: "body.payload.source_document_number"},
		{Key: "batch_code", Label: "Batch", LabelI18n: localize("Batch", "Batch"), Path: "body.payload.batch_code"},
		{Key: "quantity", Label: "Quantity", LabelI18n: localize("Quantity", "Jumlah"), Path: "body.payload.quantity"},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "header.status"},
	}
}

func recallCaseDetailSections() []module.SectionDefinition {
	return []module.SectionDefinition{{Key: "summary", Title: "Recall Summary", TitleI18n: localize("Recall Summary", "Ringkasan Recall"), Fields: []module.FieldDefinition{
		{Key: "number", Label: "Number", LabelI18n: localize("Number", "Nomor"), Path: "header.number", Type: "string"},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "header.status", Type: "string"},
		{Key: "title", Label: "Title", LabelI18n: localize("Title", "Judul"), Path: "body.payload.title", Type: "string"},
		{Key: "reason", Label: "Reason", LabelI18n: localize("Reason", "Alasan"), Path: "body.payload.reason", Type: "string"},
		{Key: "severity", Label: "Severity", LabelI18n: localize("Severity", "Keparahan"), Path: "body.payload.severity", Type: "string"},
		{Key: "recall_reference", Label: "Reference", LabelI18n: localize("Reference", "Referensi"), Path: "body.payload.recall_reference", Type: "string"},
		{Key: "containment_mode", Label: "Containment", LabelI18n: localize("Containment", "Containment"), Path: "body.payload.containment_mode", Type: "string"},
		{Key: "generated_action_count", Label: "Generated Actions", LabelI18n: localize("Generated Actions", "Aksi Dihasilkan"), Path: "body.payload.generated_action_count", Type: "number"},
		{Key: "affected_batches", Label: "Affected Batches", LabelI18n: localize("Affected Batches", "Batch Terdampak"), Path: "body.payload.affected_batches", Type: "object", Widget: "trace_batches"},
		{Key: "impact_summary", Label: "Impact Summary", LabelI18n: localize("Impact Summary", "Ringkasan Dampak"), Path: "body.payload.impact_summary", Type: "object", Widget: "json"},
	}}}
}

func recallCaseFormSections() []module.SectionDefinition {
	return []module.SectionDefinition{{Key: "edit", Title: "Recall Draft", TitleI18n: localize("Recall Draft", "Draft Recall"), Fields: []module.FieldDefinition{
		{Key: "title", Label: "Title", LabelI18n: localize("Title", "Judul"), Path: "body.payload.title", Type: "string", Widget: "text", Required: true},
		{Key: "reason", Label: "Reason", LabelI18n: localize("Reason", "Alasan"), Path: "body.payload.reason", Type: "string", Widget: "textarea"},
		{Key: "severity", Label: "Severity", LabelI18n: localize("Severity", "Keparahan"), Path: "body.payload.severity", Type: "string", Widget: "select", Options: []string{"low", "medium", "high", "critical"}},
		{Key: "recall_reference", Label: "Reference", LabelI18n: localize("Reference", "Referensi"), Path: "body.payload.recall_reference", Type: "string", Widget: "text"},
		{Key: "containment_mode", Label: "Containment", LabelI18n: localize("Containment", "Containment"), Path: "body.payload.containment_mode", Type: "string", Widget: "select", Options: []string{"recalled"}},
		{Key: "affected_batches", Label: "Affected Batches", LabelI18n: localize("Affected Batches", "Batch Terdampak"), Path: "body.payload.affected_batches", Type: "object", Widget: "trace_batches"},
	}}}
}

func recallActionDetailSections() []module.SectionDefinition {
	return []module.SectionDefinition{{Key: "summary", Title: "Recall Action Summary", TitleI18n: localize("Recall Action Summary", "Ringkasan Aksi Recall"), Fields: []module.FieldDefinition{
		{Key: "number", Label: "Number", LabelI18n: localize("Number", "Nomor"), Path: "header.number", Type: "string"},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "header.status", Type: "string"},
		{Key: "action_type", Label: "Action Type", LabelI18n: localize("Action Type", "Jenis Aksi"), Path: "body.payload.action_type", Type: "string"},
		{Key: "source_recall_case_number", Label: "Recall Case", LabelI18n: localize("Recall Case", "Kasus Recall"), Path: "body.payload.source_recall_case_number", Type: "string"},
		{Key: "source_document_number", Label: "Source Document", LabelI18n: localize("Source Document", "Dokumen Sumber"), Path: "body.payload.source_document_number", Type: "string"},
		{Key: "item_code", Label: "Item", LabelI18n: localize("Item", "Item"), Path: "body.payload.item_code", Type: "string"},
		{Key: "batch_code", Label: "Batch", LabelI18n: localize("Batch", "Batch"), Path: "body.payload.batch_code", Type: "string"},
		{Key: "warehouse_code", Label: "Warehouse", LabelI18n: localize("Warehouse", "Gudang"), Path: "body.payload.warehouse_code", Type: "string"},
		{Key: "quantity", Label: "Quantity", LabelI18n: localize("Quantity", "Jumlah"), Path: "body.payload.quantity", Type: "number"},
		{Key: "notes", Label: "Notes", LabelI18n: localize("Notes", "Catatan"), Path: "body.payload.notes", Type: "string"},
	}}}
}

func recallActionFormSections() []module.SectionDefinition {
	return []module.SectionDefinition{{Key: "edit", Title: "Recall Action Draft", TitleI18n: localize("Recall Action Draft", "Draft Aksi Recall"), Fields: []module.FieldDefinition{
		{Key: "action_type", Label: "Action Type", LabelI18n: localize("Action Type", "Jenis Aksi"), Path: "body.payload.action_type", Type: "string", Widget: "text", ReadOnly: true},
		{Key: "source_recall_case_number", Label: "Recall Case", LabelI18n: localize("Recall Case", "Kasus Recall"), Path: "body.payload.source_recall_case_number", Type: "string", Widget: "text", ReadOnly: true},
		{Key: "source_document_number", Label: "Source Document", LabelI18n: localize("Source Document", "Dokumen Sumber"), Path: "body.payload.source_document_number", Type: "string", Widget: "text", ReadOnly: true},
		{Key: "item_code", Label: "Item", LabelI18n: localize("Item", "Item"), Path: "body.payload.item_code", Type: "string", Widget: "text", ReadOnly: true},
		{Key: "batch_code", Label: "Batch", LabelI18n: localize("Batch", "Batch"), Path: "body.payload.batch_code", Type: "string", Widget: "text", ReadOnly: true},
		{Key: "warehouse_code", Label: "Warehouse", LabelI18n: localize("Warehouse", "Gudang"), Path: "body.payload.warehouse_code", Type: "string", Widget: "text", ReadOnly: true},
		{Key: "quantity", Label: "Quantity", LabelI18n: localize("Quantity", "Jumlah"), Path: "body.payload.quantity", Type: "number", Widget: "text", ReadOnly: true},
		{Key: "notes", Label: "Notes", LabelI18n: localize("Notes", "Catatan"), Path: "body.payload.notes", Type: "string", Widget: "textarea"},
	}}}
}
