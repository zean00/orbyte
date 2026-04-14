package app

import (
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/search"
)

func workflowApprovalPolicyKernelPackManifest() module.Manifest {
	return module.Manifest{
		Key:                  "workflow_approval_policy",
		Name:                 "Workflow Approval Policy",
		NameI18n:             localize("Workflow Approval Policy", "Kebijakan Persetujuan Workflow"),
		Version:              "1.0.0",
		DomainFamily:         "platform",
		Description:          "Shared approval policies, stages, approver groups, and delegation rules for governed document routing.",
		DescriptionI18n:      localize("Shared approval policies, stages, approver groups, and delegation rules for governed document routing.", "Kebijakan persetujuan, tahap, grup approver, dan aturan delegasi bersama untuk routing dokumen yang terkelola."),
		BusinessCapabilities: []string{"approval policy", "multi-stage approval", "approver groups", "delegation rules", "shared workflow routing"},
		DependencyRequirements: []module.DependencyRequirement{
			{ModuleKey: "platform.core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "identity", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "documents", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "organization_structure", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindOptional},
			{ModuleKey: "employee_workforce", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindOptional},
		},
		AdminConsole: module.AdminConsoleDefinition{
			Title:           "Approval Policy Console",
			TitleI18n:       localize("Approval Policy Console", "Konsol Kebijakan Persetujuan"),
			Description:     "Configure shared approval policies, staged routing, approver groups, and delegation rules.",
			DescriptionI18n: localize("Configure shared approval policies, staged routing, approver groups, and delegation rules.", "Konfigurasikan kebijakan persetujuan bersama, routing bertahap, grup approver, dan aturan delegasi."),
			Sections: []module.AdminConsoleSectionDefinition{{
				Key:       "approval_policy_setup",
				Title:     "Approval Policy Setup",
				TitleI18n: localize("Approval Policy Setup", "Pengaturan Kebijakan Persetujuan"),
				Kind:      module.AdminConsoleSectionResourceLinks,
				Links: []module.AdminConsoleLinkDefinition{
					adminConsoleLink("policies", "Approval Policies", "Kebijakan Persetujuan", "/ui/approval/policies", "Open shared approval policy records.", "Buka data kebijakan persetujuan bersama.", "approval_policy.list"),
					adminConsoleLink("stages", "Policy Stages", "Tahap Kebijakan", "/ui/approval/stages", "Open ordered approval stages.", "Buka tahap persetujuan berurutan.", "approval_policy.read"),
					adminConsoleLink("groups", "Approver Groups", "Grup Approver", "/ui/approval/groups", "Open approver group masters.", "Buka data master grup approver.", "approval_policy.read"),
					adminConsoleLink("members", "Group Members", "Anggota Grup", "/ui/approval/members", "Open approver group memberships.", "Buka keanggotaan grup approver.", "approval_policy.read"),
					adminConsoleLink("delegations", "Delegation Rules", "Aturan Delegasi", "/ui/approval/delegations", "Open approval delegation rules.", "Buka aturan delegasi persetujuan.", "approval_policy.read"),
				},
			}},
		},
		Models: []model.Definition{
			approvalPolicyModel("approval_policy", "Approval Policy", "Kebijakan Persetujuan", []model.FieldDefinition{
				{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Type: "string", Required: true},
				{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Type: "string", Required: true},
				{Key: "document_type", Label: "Document Type", LabelI18n: localize("Document Type", "Tipe Dokumen"), Type: "string"},
				{Key: "workflow_key", Label: "Workflow Key", LabelI18n: localize("Workflow Key", "Kunci Workflow"), Type: "string"},
				{Key: "action", Label: "Action", LabelI18n: localize("Action", "Aksi"), Type: "string", DefaultValue: "submit"},
				{Key: "organization_id", Label: "Organization", LabelI18n: localize("Organization", "Organisasi"), Type: "string"},
				{Key: "location_id", Label: "Location", LabelI18n: localize("Location", "Lokasi"), Type: "string"},
				{Key: "operating_unit_id", Label: "Operating Unit", LabelI18n: localize("Operating Unit", "Unit Operasi"), Type: "string"},
				{Key: "department_id", Label: "Department", LabelI18n: localize("Department", "Departemen"), Type: "string"},
				{Key: "cost_center_id", Label: "Cost Center", LabelI18n: localize("Cost Center", "Pusat Biaya"), Type: "string"},
				{Key: "minimum_amount_minor", Label: "Minimum Amount Minor", LabelI18n: localize("Minimum Amount Minor", "Jumlah Minimum Minor"), Type: "number"},
				{Key: "maximum_amount_minor", Label: "Maximum Amount Minor", LabelI18n: localize("Maximum Amount Minor", "Jumlah Maksimum Minor"), Type: "number"},
				{Key: "routing_mode", Label: "Routing Mode", LabelI18n: localize("Routing Mode", "Mode Routing"), Type: "string"},
				{Key: "assignment_strategy", Label: "Assignment Strategy", LabelI18n: localize("Assignment Strategy", "Strategi Penugasan"), Type: "string"},
				{Key: "assignment_mode", Label: "Assignment Mode", LabelI18n: localize("Assignment Mode", "Mode Penugasan"), Type: "string"},
				{Key: "assignee_role_key", Label: "Assignee Role", LabelI18n: localize("Assignee Role", "Peran Penerima Tugas"), Type: "string"},
				{Key: "fallback_role_key", Label: "Fallback Role", LabelI18n: localize("Fallback Role", "Peran Cadangan"), Type: "string"},
				{Key: "approver_group_id", Label: "Approver Group", LabelI18n: localize("Approver Group", "Grup Approver"), Type: "string"},
				{Key: "explicit_user_id", Label: "Explicit User", LabelI18n: localize("Explicit User", "Pengguna Eksplisit"), Type: "string"},
				{Key: "candidate_role_keys", Label: "Candidate Role Keys", LabelI18n: localize("Candidate Role Keys", "Kunci Peran Kandidat"), Type: "string"},
				{Key: "priority", Label: "Priority", LabelI18n: localize("Priority", "Prioritas"), Type: "number", DefaultValue: 0},
				{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string", DefaultValue: "active"},
			}, []model.RelationDefinition{{Key: "stages", Type: "has_many", TargetModelKey: "approval_policy_stage", ForeignKey: "policy_id"}}),
			approvalPolicyModel("approval_policy_stage", "Approval Policy Stage", "Tahap Kebijakan Persetujuan", []model.FieldDefinition{
				{Key: "policy_id", Label: "Policy", LabelI18n: localize("Policy", "Kebijakan"), Type: "string", Required: true},
				{Key: "stage_key", Label: "Stage Key", LabelI18n: localize("Stage Key", "Kunci Tahap"), Type: "string", Required: true},
				{Key: "sequence", Label: "Sequence", LabelI18n: localize("Sequence", "Urutan"), Type: "number", Required: true},
				{Key: "required_approver_count", Label: "Required Approver Count", LabelI18n: localize("Required Approver Count", "Jumlah Approver Wajib"), Type: "number", DefaultValue: 1},
				{Key: "routing_mode", Label: "Routing Mode", LabelI18n: localize("Routing Mode", "Mode Routing"), Type: "string"},
				{Key: "assignment_strategy", Label: "Assignment Strategy", LabelI18n: localize("Assignment Strategy", "Strategi Penugasan"), Type: "string"},
				{Key: "assignment_mode", Label: "Assignment Mode", LabelI18n: localize("Assignment Mode", "Mode Penugasan"), Type: "string"},
				{Key: "assignee_role_key", Label: "Assignee Role", LabelI18n: localize("Assignee Role", "Peran Penerima Tugas"), Type: "string"},
				{Key: "fallback_role_key", Label: "Fallback Role", LabelI18n: localize("Fallback Role", "Peran Cadangan"), Type: "string"},
				{Key: "approver_group_id", Label: "Approver Group", LabelI18n: localize("Approver Group", "Grup Approver"), Type: "string"},
				{Key: "explicit_user_id", Label: "Explicit User", LabelI18n: localize("Explicit User", "Pengguna Eksplisit"), Type: "string"},
				{Key: "candidate_role_keys", Label: "Candidate Role Keys", LabelI18n: localize("Candidate Role Keys", "Kunci Peran Kandidat"), Type: "string"},
				{Key: "due_after_seconds", Label: "Due After Seconds", LabelI18n: localize("Due After Seconds", "Jatuh Tempo Dalam Detik"), Type: "number"},
				{Key: "escalate_after_seconds", Label: "Escalate After Seconds", LabelI18n: localize("Escalate After Seconds", "Eskalasi Dalam Detik"), Type: "number"},
				{Key: "requires_different_actor", Label: "Requires Different Actor", LabelI18n: localize("Requires Different Actor", "Memerlukan Aktor Berbeda"), Type: "boolean"},
				{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string", DefaultValue: "active"},
			}, nil),
			approvalPolicyModel("approver_group", "Approver Group", "Grup Approver", []model.FieldDefinition{
				{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Type: "string", Required: true},
				{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Type: "string", Required: true},
				{Key: "organization_id", Label: "Organization", LabelI18n: localize("Organization", "Organisasi"), Type: "string"},
				{Key: "location_id", Label: "Location", LabelI18n: localize("Location", "Lokasi"), Type: "string"},
				{Key: "operating_unit_id", Label: "Operating Unit", LabelI18n: localize("Operating Unit", "Unit Operasi"), Type: "string"},
				{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string", DefaultValue: "active"},
			}, []model.RelationDefinition{{Key: "members", Type: "has_many", TargetModelKey: "approver_group_member", ForeignKey: "approver_group_id"}}),
			approvalPolicyModel("approver_group_member", "Approver Group Member", "Anggota Grup Approver", []model.FieldDefinition{
				{Key: "approver_group_id", Label: "Approver Group", LabelI18n: localize("Approver Group", "Grup Approver"), Type: "string", Required: true},
				{Key: "user_id", Label: "User", LabelI18n: localize("User", "Pengguna"), Type: "string", Required: true},
				{Key: "organization_id", Label: "Organization", LabelI18n: localize("Organization", "Organisasi"), Type: "string"},
				{Key: "location_id", Label: "Location", LabelI18n: localize("Location", "Lokasi"), Type: "string"},
				{Key: "operating_unit_id", Label: "Operating Unit", LabelI18n: localize("Operating Unit", "Unit Operasi"), Type: "string"},
				{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string", DefaultValue: "active"},
			}, nil),
			approvalPolicyModel("approval_delegation_rule", "Approval Delegation Rule", "Aturan Delegasi Persetujuan", []model.FieldDefinition{
				{Key: "approver_user_id", Label: "Approver User", LabelI18n: localize("Approver User", "Pengguna Approver"), Type: "string", Required: true},
				{Key: "delegate_user_id", Label: "Delegate User", LabelI18n: localize("Delegate User", "Pengguna Delegasi"), Type: "string", Required: true},
				{Key: "effective_from", Label: "Effective From", LabelI18n: localize("Effective From", "Berlaku Dari"), Type: "string"},
				{Key: "effective_to", Label: "Effective To", LabelI18n: localize("Effective To", "Berlaku Sampai"), Type: "string"},
				{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string", DefaultValue: "active"},
			}, nil),
		},
		Datasets: []module.DatasetDefinition{{
			Key:        "approval.policy.summary",
			Title:      "Approval Policy Summary",
			TitleI18n:  localize("Approval Policy Summary", "Ringkasan Kebijakan Persetujuan"),
			SourceKind: "model",
			ModelKey:   "approval_policy",
			Dimensions: []module.DatasetDimension{{Key: "by_status", Label: "By Status", LabelI18n: localize("By Status", "Berdasarkan Status"), Path: "status"}},
			Measures:   []module.DatasetMeasure{{Key: "total", Label: "Total", LabelI18n: localize("Total", "Total"), Kind: "count"}},
		}},
		SearchIndexes: []search.IndexDefinition{
			approvalPolicySearchIndex("approval.policies.search", "Approval Policy Search", "approval_policy", "approval.policies.list"),
			approvalPolicySearchIndex("approval.stages.search", "Approval Stage Search", "approval_policy_stage", "approval.stages.list"),
			approvalPolicySearchIndex("approval.groups.search", "Approver Group Search", "approver_group", "approval.groups.list"),
			approvalPolicySearchIndex("approval.members.search", "Approver Group Member Search", "approver_group_member", "approval.members.list"),
			approvalPolicySearchIndex("approval.delegations.search", "Approval Delegation Search", "approval_delegation_rule", "approval.delegations.list"),
		},
		Security: module.SecurityDefinition{
			Permissions: []module.PermissionDefinition{
				{Key: "approval_policy.create", Action: "create", Resource: "approval_policy", DisplayName: "Create Approval Policies", DisplayNameI18n: localize("Create Approval Policies", "Buat Kebijakan Persetujuan")},
				{Key: "approval_policy.list", Action: "list", Resource: "approval_policy", DisplayName: "List Approval Policies", DisplayNameI18n: localize("List Approval Policies", "Daftar Kebijakan Persetujuan")},
				{Key: "approval_policy.read", Action: "read", Resource: "approval_policy", DisplayName: "Read Approval Policies", DisplayNameI18n: localize("Read Approval Policies", "Lihat Kebijakan Persetujuan")},
				{Key: "approval_policy.update", Action: "update", Resource: "approval_policy", DisplayName: "Update Approval Policies", DisplayNameI18n: localize("Update Approval Policies", "Perbarui Kebijakan Persetujuan")},
			},
			RoleTemplates: []module.RoleTemplateDefinition{{
				Key: "approval_policy_manager", Name: "Approval Policy Manager", NameI18n: localize("Approval Policy Manager", "Pengelola Kebijakan Persetujuan"), AllowedScopes: []string{"deployment", "organization", "location"}, PermissionKeys: []string{"approval_policy.create", "approval_policy.list", "approval_policy.read", "approval_policy.update"},
			}},
		},
		Frontend: module.FrontendDefinition{
			Menus: []module.MenuDefinition{
				{Key: "approval.policies", Label: "Approval Policies", LabelI18n: localize("Approval Policies", "Kebijakan Persetujuan"), ActionKey: "approval.policies.list", Order: 19, RequiredPermissions: []string{"approval_policy.list"}},
				{Key: "approval.groups", Label: "Approver Groups", LabelI18n: localize("Approver Groups", "Grup Approver"), ActionKey: "approval.groups.list", Order: 20, RequiredPermissions: []string{"approval_policy.read"}},
			},
			Actions: append(approvalPolicyActions("policies", "Approval Policies", "Approval Policy Detail", "Approval Policy Form", "approval_policy.list", "approval_policy.read", "approval_policy.update"),
				append(approvalPolicyActions("stages", "Policy Stages", "Policy Stage Detail", "Policy Stage Form", "approval_policy.read", "approval_policy.read", "approval_policy.update"),
					append(approvalPolicyActions("groups", "Approver Groups", "Approver Group Detail", "Approver Group Form", "approval_policy.read", "approval_policy.read", "approval_policy.update"),
						append(approvalPolicyActions("members", "Group Members", "Group Member Detail", "Group Member Form", "approval_policy.read", "approval_policy.read", "approval_policy.update"),
							approvalPolicyActions("delegations", "Delegation Rules", "Delegation Rule Detail", "Delegation Rule Form", "approval_policy.read", "approval_policy.read", "approval_policy.update")...)...)...)...),
			Views: []module.ViewDefinition{
				commercialModelListView("approval.policies.list", "Approval Policies", "approval_policy", []module.ColumnDefinition{
					{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code"},
					{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name"},
					{Key: "document_type", Label: "Document Type", LabelI18n: localize("Document Type", "Tipe Dokumen"), Path: "values.document_type"},
					{Key: "action", Label: "Action", LabelI18n: localize("Action", "Aksi"), Path: "values.action"},
					{Key: "priority", Label: "Priority", LabelI18n: localize("Priority", "Prioritas"), Path: "values.priority"},
					{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status"},
				}, []string{"active", "inactive"}),
				commercialModelDetailView("approval.policies.detail", "Approval Policy Detail", "approval_policy", approvalPolicyFields("approval_policy", false)),
				commercialModelFormView("approval.policies.form", "Approval Policy Form", "approval_policy", approvalPolicyFields("approval_policy", true)),
				commercialModelListView("approval.stages.list", "Policy Stages", "approval_policy_stage", []module.ColumnDefinition{
					{Key: "policy_id", Label: "Policy", LabelI18n: localize("Policy", "Kebijakan"), Path: "values.policy_id"},
					{Key: "stage_key", Label: "Stage", LabelI18n: localize("Stage", "Tahap"), Path: "values.stage_key"},
					{Key: "sequence", Label: "Sequence", LabelI18n: localize("Sequence", "Urutan"), Path: "values.sequence"},
					{Key: "assignment_strategy", Label: "Strategy", LabelI18n: localize("Strategy", "Strategi"), Path: "values.assignment_strategy"},
					{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status"},
				}, []string{"active", "inactive"}),
				commercialModelDetailView("approval.stages.detail", "Policy Stage Detail", "approval_policy_stage", approvalPolicyFields("approval_policy_stage", false)),
				commercialModelFormView("approval.stages.form", "Policy Stage Form", "approval_policy_stage", approvalPolicyFields("approval_policy_stage", true)),
				commercialModelListView("approval.groups.list", "Approver Groups", "approver_group", []module.ColumnDefinition{
					{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code"},
					{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name"},
					{Key: "location_id", Label: "Location", LabelI18n: localize("Location", "Lokasi"), Path: "values.location_id"},
					{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status"},
				}, []string{"active", "inactive"}),
				commercialModelDetailView("approval.groups.detail", "Approver Group Detail", "approver_group", approvalPolicyFields("approver_group", false)),
				commercialModelFormView("approval.groups.form", "Approver Group Form", "approver_group", approvalPolicyFields("approver_group", true)),
				commercialModelListView("approval.members.list", "Group Members", "approver_group_member", []module.ColumnDefinition{
					{Key: "approver_group_id", Label: "Group", LabelI18n: localize("Group", "Grup"), Path: "values.approver_group_id"},
					{Key: "user_id", Label: "User", LabelI18n: localize("User", "Pengguna"), Path: "values.user_id"},
					{Key: "location_id", Label: "Location", LabelI18n: localize("Location", "Lokasi"), Path: "values.location_id"},
					{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status"},
				}, []string{"active", "inactive"}),
				commercialModelDetailView("approval.members.detail", "Group Member Detail", "approver_group_member", approvalPolicyFields("approver_group_member", false)),
				commercialModelFormView("approval.members.form", "Group Member Form", "approver_group_member", approvalPolicyFields("approver_group_member", true)),
				commercialModelListView("approval.delegations.list", "Delegation Rules", "approval_delegation_rule", []module.ColumnDefinition{
					{Key: "approver_user_id", Label: "Approver", LabelI18n: localize("Approver", "Approver"), Path: "values.approver_user_id"},
					{Key: "delegate_user_id", Label: "Delegate", LabelI18n: localize("Delegate", "Delegasi"), Path: "values.delegate_user_id"},
					{Key: "effective_from", Label: "From", LabelI18n: localize("From", "Dari"), Path: "values.effective_from"},
					{Key: "effective_to", Label: "To", LabelI18n: localize("To", "Sampai"), Path: "values.effective_to"},
					{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status"},
				}, []string{"active", "inactive"}),
				commercialModelDetailView("approval.delegations.detail", "Delegation Rule Detail", "approval_delegation_rule", approvalPolicyFields("approval_delegation_rule", false)),
				commercialModelFormView("approval.delegations.form", "Delegation Rule Form", "approval_delegation_rule", approvalPolicyFields("approval_delegation_rule", true)),
			},
		},
	}
}

func workflowApprovalPolicyKernelPackManifests() []module.Manifest {
	return []module.Manifest{workflowApprovalPolicyKernelPackManifest()}
}

func approvalPolicyModel(key, name, nameID string, fields []model.FieldDefinition, relations []model.RelationDefinition) model.Definition {
	return model.Definition{
		Key:                 key,
		DisplayName:         name,
		DisplayNameI18n:     localize(name, nameID),
		OwnerModuleKey:      "workflow_approval_policy",
		Version:             "v1",
		CreatePermissionKey: "approval_policy.create",
		ListPermissionKey:   "approval_policy.list",
		ReadPermissionKey:   "approval_policy.read",
		UpdatePermissionKey: "approval_policy.update",
		DefaultSort:         "code",
		Fields:              fields,
		Relations:           relations,
	}
}

func approvalPolicyActions(prefix, listLabel, detailLabel, formLabel, listPerm, readPerm, updatePerm string) []module.ActionDefinition {
	base := "/approval/" + prefix
	return []module.ActionDefinition{
		{Key: "approval." + prefix + ".list", Label: listLabel, LabelI18n: localize(listLabel, listLabel), Kind: "navigate", RoutePath: base, ViewKey: "approval." + prefix + ".list", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{listPerm}},
		{Key: "approval." + prefix + ".detail", Label: detailLabel, LabelI18n: localize(detailLabel, detailLabel), Kind: "navigate", RoutePath: base + "/detail", ViewKey: "approval." + prefix + ".detail", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{readPerm}},
		{Key: "approval." + prefix + ".form", Label: formLabel, LabelI18n: localize(formLabel, formLabel), Kind: "navigate", RoutePath: base + "/form", ViewKey: "approval." + prefix + ".form", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{updatePerm}},
		{Key: "approval." + prefix + ".new", Label: "New " + listLabel, LabelI18n: localize("New "+listLabel, "Baru"), Kind: "navigate", RoutePath: base + "/new", ViewKey: "approval." + prefix + ".form", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{updatePerm}},
	}
}

func approvalPolicySearchIndex(key, title, modelKey, viewKey string) search.IndexDefinition {
	return search.IndexDefinition{
		Key:                 key,
		Title:               title,
		SourceKind:          "model",
		ModelKey:            modelKey,
		ViewKey:             viewKey,
		Modes:               []string{"keyword", "hybrid"},
		OrganizationSplit:   true,
		RequiredPermissions: []string{"approval_policy.list"},
		QueryFilterFields:   []string{"organization_id", "location_id", "status"},
		QuerySortFields:     []string{"code", "name", "updated_at"},
		Fields: []search.IndexFieldDefinition{
			{Key: "organization_id", Path: "organization_id", Type: "string", Facet: true},
			{Key: "location_id", Path: "location_id", Type: "string", Facet: true},
			{Key: "code", Path: "code", Type: "string", Searchable: true, Sort: true, Optional: true},
			{Key: "name", Path: "name", Type: "string", Searchable: true, Sort: true, Optional: true},
			{Key: "status", Path: "status", Type: "string", Facet: true, Sort: true},
		},
	}
}

func approvalPolicyFields(modelKey string, editable bool) []module.FieldDefinition {
	var fields []model.FieldDefinition
	switch modelKey {
	case "approval_policy":
		fields = []model.FieldDefinition{
			{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Type: "string", Required: true},
			{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Type: "string", Required: true},
			{Key: "document_type", Label: "Document Type", LabelI18n: localize("Document Type", "Tipe Dokumen"), Type: "string"},
			{Key: "workflow_key", Label: "Workflow Key", LabelI18n: localize("Workflow Key", "Kunci Workflow"), Type: "string"},
			{Key: "action", Label: "Action", LabelI18n: localize("Action", "Aksi"), Type: "string"},
			{Key: "organization_id", Label: "Organization", LabelI18n: localize("Organization", "Organisasi"), Type: "string"},
			{Key: "location_id", Label: "Location", LabelI18n: localize("Location", "Lokasi"), Type: "string"},
			{Key: "operating_unit_id", Label: "Operating Unit", LabelI18n: localize("Operating Unit", "Unit Operasi"), Type: "string"},
			{Key: "department_id", Label: "Department", LabelI18n: localize("Department", "Departemen"), Type: "string"},
			{Key: "cost_center_id", Label: "Cost Center", LabelI18n: localize("Cost Center", "Pusat Biaya"), Type: "string"},
			{Key: "minimum_amount_minor", Label: "Minimum Amount Minor", LabelI18n: localize("Minimum Amount Minor", "Jumlah Minimum Minor"), Type: "number"},
			{Key: "maximum_amount_minor", Label: "Maximum Amount Minor", LabelI18n: localize("Maximum Amount Minor", "Jumlah Maksimum Minor"), Type: "number"},
			{Key: "routing_mode", Label: "Routing Mode", LabelI18n: localize("Routing Mode", "Mode Routing"), Type: "string"},
			{Key: "assignment_strategy", Label: "Assignment Strategy", LabelI18n: localize("Assignment Strategy", "Strategi Penugasan"), Type: "string"},
			{Key: "assignment_mode", Label: "Assignment Mode", LabelI18n: localize("Assignment Mode", "Mode Penugasan"), Type: "string"},
			{Key: "assignee_role_key", Label: "Assignee Role", LabelI18n: localize("Assignee Role", "Peran Penerima Tugas"), Type: "string"},
			{Key: "fallback_role_key", Label: "Fallback Role", LabelI18n: localize("Fallback Role", "Peran Cadangan"), Type: "string"},
			{Key: "approver_group_id", Label: "Approver Group", LabelI18n: localize("Approver Group", "Grup Approver"), Type: "string"},
			{Key: "explicit_user_id", Label: "Explicit User", LabelI18n: localize("Explicit User", "Pengguna Eksplisit"), Type: "string"},
			{Key: "candidate_role_keys", Label: "Candidate Role Keys", LabelI18n: localize("Candidate Role Keys", "Kunci Peran Kandidat"), Type: "string"},
			{Key: "priority", Label: "Priority", LabelI18n: localize("Priority", "Prioritas"), Type: "number"},
			{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string"},
		}
	case "approval_policy_stage":
		fields = []model.FieldDefinition{
			{Key: "policy_id", Label: "Policy", LabelI18n: localize("Policy", "Kebijakan"), Type: "string", Required: true},
			{Key: "stage_key", Label: "Stage Key", LabelI18n: localize("Stage Key", "Kunci Tahap"), Type: "string", Required: true},
			{Key: "sequence", Label: "Sequence", LabelI18n: localize("Sequence", "Urutan"), Type: "number", Required: true},
			{Key: "required_approver_count", Label: "Required Approver Count", LabelI18n: localize("Required Approver Count", "Jumlah Approver Wajib"), Type: "number"},
			{Key: "routing_mode", Label: "Routing Mode", LabelI18n: localize("Routing Mode", "Mode Routing"), Type: "string"},
			{Key: "assignment_strategy", Label: "Assignment Strategy", LabelI18n: localize("Assignment Strategy", "Strategi Penugasan"), Type: "string"},
			{Key: "assignment_mode", Label: "Assignment Mode", LabelI18n: localize("Assignment Mode", "Mode Penugasan"), Type: "string"},
			{Key: "assignee_role_key", Label: "Assignee Role", LabelI18n: localize("Assignee Role", "Peran Penerima Tugas"), Type: "string"},
			{Key: "fallback_role_key", Label: "Fallback Role", LabelI18n: localize("Fallback Role", "Peran Cadangan"), Type: "string"},
			{Key: "approver_group_id", Label: "Approver Group", LabelI18n: localize("Approver Group", "Grup Approver"), Type: "string"},
			{Key: "explicit_user_id", Label: "Explicit User", LabelI18n: localize("Explicit User", "Pengguna Eksplisit"), Type: "string"},
			{Key: "candidate_role_keys", Label: "Candidate Role Keys", LabelI18n: localize("Candidate Role Keys", "Kunci Peran Kandidat"), Type: "string"},
			{Key: "due_after_seconds", Label: "Due After Seconds", LabelI18n: localize("Due After Seconds", "Jatuh Tempo Dalam Detik"), Type: "number"},
			{Key: "escalate_after_seconds", Label: "Escalate After Seconds", LabelI18n: localize("Escalate After Seconds", "Eskalasi Dalam Detik"), Type: "number"},
			{Key: "requires_different_actor", Label: "Requires Different Actor", LabelI18n: localize("Requires Different Actor", "Memerlukan Aktor Berbeda"), Type: "boolean"},
			{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string"},
		}
	case "approver_group":
		fields = []model.FieldDefinition{
			{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Type: "string", Required: true},
			{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Type: "string", Required: true},
			{Key: "organization_id", Label: "Organization", LabelI18n: localize("Organization", "Organisasi"), Type: "string"},
			{Key: "location_id", Label: "Location", LabelI18n: localize("Location", "Lokasi"), Type: "string"},
			{Key: "operating_unit_id", Label: "Operating Unit", LabelI18n: localize("Operating Unit", "Unit Operasi"), Type: "string"},
			{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string"},
		}
	case "approver_group_member":
		fields = []model.FieldDefinition{
			{Key: "approver_group_id", Label: "Approver Group", LabelI18n: localize("Approver Group", "Grup Approver"), Type: "string", Required: true},
			{Key: "user_id", Label: "User", LabelI18n: localize("User", "Pengguna"), Type: "string", Required: true},
			{Key: "organization_id", Label: "Organization", LabelI18n: localize("Organization", "Organisasi"), Type: "string"},
			{Key: "location_id", Label: "Location", LabelI18n: localize("Location", "Lokasi"), Type: "string"},
			{Key: "operating_unit_id", Label: "Operating Unit", LabelI18n: localize("Operating Unit", "Unit Operasi"), Type: "string"},
			{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string"},
		}
	case "approval_delegation_rule":
		fields = []model.FieldDefinition{
			{Key: "approver_user_id", Label: "Approver User", LabelI18n: localize("Approver User", "Pengguna Approver"), Type: "string", Required: true},
			{Key: "delegate_user_id", Label: "Delegate User", LabelI18n: localize("Delegate User", "Pengguna Delegasi"), Type: "string", Required: true},
			{Key: "effective_from", Label: "Effective From", LabelI18n: localize("Effective From", "Berlaku Dari"), Type: "string"},
			{Key: "effective_to", Label: "Effective To", LabelI18n: localize("Effective To", "Berlaku Sampai"), Type: "string"},
			{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string"},
		}
	}
	result := make([]module.FieldDefinition, 0, len(fields))
	for _, field := range fields {
		widget := "text"
		if field.Type == "number" {
			widget = "number"
		}
		if field.Type == "boolean" {
			widget = "checkbox"
		}
		result = append(result, module.FieldDefinition{
			Key:       field.Key,
			Label:     field.Label,
			LabelI18n: field.LabelI18n,
			Path:      "values." + field.Key,
			Type:      field.Type,
			Widget:    widget,
			Required:  editable && field.Required,
		})
	}
	return result
}
