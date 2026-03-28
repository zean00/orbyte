package modules

import (
	platformdocument "orbyte/internal/platform/document"
	platformi18n "orbyte/internal/platform/i18n"
	platformmodel "orbyte/internal/platform/model"
	platformmodule "orbyte/internal/platform/module"
	platformsearch "orbyte/internal/platform/search"
	platformworkflow "orbyte/internal/platform/workflow"
)

func localizeClinic(en, id string) platformi18n.LocalizedText {
	return platformi18n.LocalizedText{"en": en, "id": id}
}

func clinicAdminConsoleLink(key, labelEn, labelID, routePath, descriptionEn, descriptionID, permission string) platformmodule.AdminConsoleLinkDefinition {
	link := platformmodule.AdminConsoleLinkDefinition{
		Key:             key,
		Label:           labelEn,
		LabelI18n:       localizeClinic(labelEn, labelID),
		Description:     descriptionEn,
		DescriptionI18n: localizeClinic(descriptionEn, descriptionID),
		RoutePath:       routePath,
	}
	if permission != "" {
		link.RequiredPermissions = []string{permission}
	}
	return link
}

func clinicRegistrationManifest() platformmodule.Manifest {
	return platformmodule.Manifest{
		Key:          "clinic_registration",
		Name:         "Clinic Registration",
		NameI18n:     localizeClinic("Clinic Registration", "Registrasi Klinik"),
		Version:      "1.0.0",
		DomainFamily: "clinic",
		DependencyRequirements: []platformmodule.DependencyRequirement{
			{ModuleKey: "platform.core", VersionRange: ">=1.0.0,<2.0.0", Kind: platformmodule.DependencyKindRequired},
			{ModuleKey: "identity", VersionRange: ">=1.0.0,<2.0.0", Kind: platformmodule.DependencyKindRequired},
			{ModuleKey: "documents", VersionRange: ">=1.1.0,<2.0.0", Kind: platformmodule.DependencyKindRequired},
			{ModuleKey: "masterdata", VersionRange: ">=1.0.0,<2.0.0", Kind: platformmodule.DependencyKindRequired},
			{ModuleKey: "reference_masterdata", VersionRange: ">=1.0.0,<2.0.0", Kind: platformmodule.DependencyKindRequired},
			{ModuleKey: "integration", VersionRange: ">=1.0.0,<2.0.0", Kind: platformmodule.DependencyKindOptional},
		},
		AdminConsole: platformmodule.AdminConsoleDefinition{
			Title:           "Clinic Registration Console",
			TitleI18n:       localizeClinic("Clinic Registration Console", "Konsol Registrasi Klinik"),
			Description:     "Clinic registration operations, patient records, and workflow shortcuts.",
			DescriptionI18n: localizeClinic("Clinic registration operations, patient records, and workflow shortcuts.", "Operasi registrasi klinik, rekam pasien, dan pintasan workflow."),
			Sections: []platformmodule.AdminConsoleSectionDefinition{
				{
					Key:       "clinic_operations",
					Title:     "Clinic Operations",
					TitleI18n: localizeClinic("Clinic Operations", "Operasi Klinik"),
					Kind:      platformmodule.AdminConsoleSectionResourceLinks,
					Links: []platformmodule.AdminConsoleLinkDefinition{
						clinicAdminConsoleLink("registrations", "Registrations", "Registrasi", "/ui/clinic/registrations", "Open clinic registrations.", "Buka registrasi klinik.", "document.list"),
						clinicAdminConsoleLink("encounters", "Encounters", "Encounter", "/ui/clinic/encounters", "Open clinic encounters.", "Buka encounter klinik.", "document.list"),
						clinicAdminConsoleLink("patients", "Patients", "Pasien", "/ui/clinic/patients", "Open patient records.", "Buka data pasien.", "clinic.patient.list"),
					},
				},
				{
					Key:       "clinic_workflows",
					Title:     "Clinic Workflows",
					TitleI18n: localizeClinic("Clinic Workflows", "Workflow Klinik"),
					Kind:      platformmodule.AdminConsoleSectionWorkflowLinks,
					Links: []platformmodule.AdminConsoleLinkDefinition{
						clinicAdminConsoleLink("registration_flow", "Registration Workflow", "Workflow Registrasi", "/admin/workflows/designer?key=clinic_registration_flow", "Open the clinic registration workflow.", "Buka workflow registrasi klinik.", "configuration.read"),
						clinicAdminConsoleLink("encounter_flow", "Encounter Workflow", "Workflow Encounter", "/admin/workflows/designer?key=clinic_encounter_flow", "Open the clinic encounter workflow.", "Buka workflow encounter klinik.", "configuration.read"),
					},
				},
			},
		},
		OwnedEntityTypes:   []string{"patient_profile", "practitioner_profile", "payer_profile"},
		OwnedDocumentTypes: []string{"clinic_registration", "clinic_encounter"},
		OwnedWorkflowKeys:  []string{"clinic_registration_flow", "clinic_encounter_flow"},
		OwnedProjectionKeys: []string{
			"document_summary",
		},
		Models: []platformmodel.Definition{
			{
				Key:                 "patient_profile",
				DisplayName:         "Patient Profile",
				DisplayNameI18n:     localizeClinic("Patient Profile", "Profil Pasien"),
				OwnerModuleKey:      "clinic_registration",
				Version:             "v1",
				CreatePermissionKey: "clinic.patient.create",
				ListPermissionKey:   "clinic.patient.list",
				ReadPermissionKey:   "clinic.patient.read",
				UpdatePermissionKey: "clinic.patient.update",
				DefaultSort:         "display_name",
				Fields: []platformmodel.FieldDefinition{
					{Key: "party_id", Label: "Party ID", Type: "string", Required: true},
					{Key: "organization_id", Label: "Organization ID", Type: "string", Required: true},
					{Key: "location_id", Label: "Location ID", Type: "string"},
					{Key: "display_name", Label: "Display Name", LabelI18n: localizeClinic("Display Name", "Nama Tampil"), Type: "string", Required: true},
					{Key: "patient_identifier_type", Label: "Identifier Type", LabelI18n: localizeClinic("Identifier Type", "Tipe Identitas"), Type: "string", Required: true},
					{Key: "patient_identifier_value", Label: "Identifier Value", LabelI18n: localizeClinic("Identifier Value", "Nilai Identitas"), Type: "string", Required: true},
					{Key: "date_of_birth", Label: "Date of Birth", LabelI18n: localizeClinic("Date of Birth", "Tanggal Lahir"), Type: "string"},
					{Key: "gender", Label: "Gender", LabelI18n: localizeClinic("Gender", "Jenis Kelamin"), Type: "string"},
					{Key: "status", Label: "Status", Type: "string", DefaultValue: "active"},
				},
			},
			{
				Key:                 "practitioner_profile",
				DisplayName:         "Practitioner Profile",
				DisplayNameI18n:     localizeClinic("Practitioner Profile", "Profil Praktisi"),
				OwnerModuleKey:      "clinic_registration",
				Version:             "v1",
				CreatePermissionKey: "clinic.practitioner.create",
				ListPermissionKey:   "clinic.practitioner.list",
				ReadPermissionKey:   "clinic.practitioner.read",
				UpdatePermissionKey: "clinic.practitioner.update",
				DefaultSort:         "display_name",
				Fields: []platformmodel.FieldDefinition{
					{Key: "party_id", Label: "Party ID", Type: "string", Required: true},
					{Key: "organization_id", Label: "Organization ID", Type: "string", Required: true},
					{Key: "location_id", Label: "Location ID", Type: "string"},
					{Key: "display_name", Label: "Display Name", LabelI18n: localizeClinic("Display Name", "Nama Tampil"), Type: "string", Required: true},
					{Key: "practitioner_type", Label: "Practitioner Type", LabelI18n: localizeClinic("Practitioner Type", "Tipe Praktisi"), Type: "string", Required: true},
					{Key: "license_number", Label: "License Number", LabelI18n: localizeClinic("License Number", "Nomor Izin"), Type: "string"},
					{Key: "status", Label: "Status", Type: "string", DefaultValue: "active"},
				},
			},
			{
				Key:                 "payer_profile",
				DisplayName:         "Payer Profile",
				DisplayNameI18n:     localizeClinic("Payer Profile", "Profil Penjamin"),
				OwnerModuleKey:      "clinic_registration",
				Version:             "v1",
				CreatePermissionKey: "clinic.payer.create",
				ListPermissionKey:   "clinic.payer.list",
				ReadPermissionKey:   "clinic.payer.read",
				UpdatePermissionKey: "clinic.payer.update",
				DefaultSort:         "display_name",
				Fields: []platformmodel.FieldDefinition{
					{Key: "party_id", Label: "Party ID", Type: "string", Required: true},
					{Key: "organization_id", Label: "Organization ID", Type: "string", Required: true},
					{Key: "display_name", Label: "Display Name", LabelI18n: localizeClinic("Display Name", "Nama Tampil"), Type: "string", Required: true},
					{Key: "payer_type", Label: "Payer Type", LabelI18n: localizeClinic("Payer Type", "Tipe Penjamin"), Type: "string", Required: true},
					{Key: "policy_reference", Label: "Policy Reference", LabelI18n: localizeClinic("Policy Reference", "Referensi Polis"), Type: "string"},
					{Key: "status", Label: "Status", Type: "string", DefaultValue: "active"},
				},
			},
		},
		Documents: []platformdocument.Definition{
			{
				Type:                   "clinic_registration",
				DisplayName:            "Clinic Registration",
				SchemaVersion:          "v1",
				WorkflowKey:            "clinic_registration_flow",
				NumberingKey:           "clinic_registration_number",
				OwnerModuleKey:         "clinic_registration",
				AllowedLinkTypes:       []string{"patient_profile", "encounter"},
				AllowedAttachmentTypes: []string{"note", "document"},
			},
			{
				Type:                   "clinic_encounter",
				DisplayName:            "Clinic Encounter",
				SchemaVersion:          "v1",
				WorkflowKey:            "clinic_encounter_flow",
				NumberingKey:           "clinic_encounter_number",
				OwnerModuleKey:         "clinic_registration",
				AllowedLinkTypes:       []string{"registration", "patient_profile"},
				AllowedAttachmentTypes: []string{"note", "document"},
			},
		},
		Workflows: []platformworkflow.Definition{
			{
				Key:    "clinic_registration_flow",
				States: []string{"draft", "submitted", "approved", "cancelled"},
				Actions: []platformworkflow.ActionRule{
					{Action: "submit", FromState: "draft", ToState: "submitted", PermissionKey: "document.submit", TaskType: "registration_review", CreateApproval: true, AssignmentMode: "role_queue", AssigneeRoleKey: "clinic_reviewer", CandidateRoleKeys: []string{"clinic_reviewer"}, ApprovalStageKey: "registration_review", LinkMode: "tokenized", LinkTTLSeconds: 24 * 60 * 60, LinkAllowedActions: []string{"approve", "reject"}},
					{Action: "approve", FromState: "submitted", ToState: "approved", PermissionKey: "document.approve"},
					{Action: "reject", FromState: "submitted", ToState: "draft", PermissionKey: "document.reject"},
					{Action: "reopen", FromState: "approved", ToState: "draft", PermissionKey: "document.reopen"},
					{Action: "cancel", FromState: "draft", ToState: "cancelled", PermissionKey: "document.cancel"},
					{Action: "cancel", FromState: "submitted", ToState: "cancelled", PermissionKey: "document.cancel"},
				},
			},
			{
				Key:    "clinic_encounter_flow",
				States: []string{"draft", "submitted", "approved", "cancelled"},
				Actions: []platformworkflow.ActionRule{
					{Action: "submit", FromState: "draft", ToState: "submitted", PermissionKey: "document.submit"},
					{Action: "approve", FromState: "submitted", ToState: "approved", PermissionKey: "document.approve"},
					{Action: "reopen", FromState: "approved", ToState: "draft", PermissionKey: "document.reopen"},
					{Action: "cancel", FromState: "draft", ToState: "cancelled", PermissionKey: "document.cancel"},
					{Action: "cancel", FromState: "submitted", ToState: "cancelled", PermissionKey: "document.cancel"},
				},
			},
		},
		Datasets: []platformmodule.DatasetDefinition{
			{
				Key:        "clinic.registration.daily",
				Title:      "Clinic Registrations",
				TitleI18n:  localizeClinic("Clinic Registrations", "Registrasi Klinik"),
				SourceKind: "model",
				ModelKey:   "patient_profile",
				Dimensions: []platformmodule.DatasetDimension{{Key: "by_status", Label: "By Status", LabelI18n: localizeClinic("By Status", "Berdasarkan Status"), Path: "status"}},
				Measures:   []platformmodule.DatasetMeasure{{Key: "total", Label: "Total", Kind: "count"}},
			},
			{
				Key:        "clinic.practitioner.coverage",
				Title:      "Practitioner Coverage",
				TitleI18n:  localizeClinic("Practitioner Coverage", "Cakupan Praktisi"),
				SourceKind: "model",
				ModelKey:   "practitioner_profile",
				Dimensions: []platformmodule.DatasetDimension{{Key: "by_type", Label: "By Type", LabelI18n: localizeClinic("By Type", "Berdasarkan Tipe"), Path: "practitioner_type"}},
				Measures:   []platformmodule.DatasetMeasure{{Key: "total", Label: "Total", Kind: "count"}},
			},
		},
		SearchIndexes: []platformsearch.IndexDefinition{
			{
				Key:                 "clinic.patient.lookup",
				Title:               "Patient Lookup",
				SourceKind:          "model",
				ModelKey:            "patient_profile",
				ViewKey:             "clinic.patients.list",
				Modes:               []string{"keyword"},
				OrganizationSplit:   true,
				RequiredPermissions: []string{"clinic.patient.list"},
				QueryFilterFields:   []string{"status", "location_id"},
				QuerySortFields:     []string{"display_name", "updated_at"},
				Fields: []platformsearch.IndexFieldDefinition{
					{Key: "display_name", Path: "display_name", Type: "string", Searchable: true, Sort: true},
					{Key: "patient_identifier_value", Path: "patient_identifier_value", Type: "string", Searchable: true},
					{Key: "status", Path: "status", Type: "string", Facet: true, Sort: true},
					{Key: "location_id", Path: "location_id", Type: "string", Facet: true},
				},
			},
			{
				Key:                 "clinic.registration.worklist",
				Title:               "Clinic Registration Worklist",
				SourceKind:          "document",
				DocumentType:        "clinic_registration",
				ViewKey:             "clinic.registrations.list",
				Modes:               []string{"keyword"},
				OrganizationSplit:   true,
				RequiredPermissions: []string{"document.list"},
				QueryFilterFields:   []string{"status", "location_id", "document_type"},
				QuerySortFields:     []string{"updated_at"},
				Fields: []platformsearch.IndexFieldDefinition{
					{Key: "document_type", Path: "header.type", Type: "string", Facet: true},
					{Key: "status", Path: "header.status", Type: "string", Facet: true, Sort: true},
					{Key: "patient_name", Path: "body.payload.patient_name", Type: "string", Searchable: true},
					{Key: "appointment_type", Path: "body.payload.appointment_type", Type: "string", Facet: true},
				},
			},
			{
				Key:                 "clinic.encounter.worklist",
				Title:               "Clinic Encounter Worklist",
				SourceKind:          "document",
				DocumentType:        "clinic_encounter",
				ViewKey:             "clinic.encounters.list",
				Modes:               []string{"keyword"},
				OrganizationSplit:   true,
				RequiredPermissions: []string{"document.list"},
				QueryFilterFields:   []string{"status", "location_id", "document_type"},
				QuerySortFields:     []string{"updated_at"},
				Fields: []platformsearch.IndexFieldDefinition{
					{Key: "document_type", Path: "header.type", Type: "string", Facet: true},
					{Key: "status", Path: "header.status", Type: "string", Facet: true, Sort: true},
					{Key: "patient_name", Path: "body.payload.patient_name", Type: "string", Searchable: true},
					{Key: "practitioner_name", Path: "body.payload.practitioner_name", Type: "string", Searchable: true},
				},
			},
		},
		Security: platformmodule.SecurityDefinition{
			Permissions: []platformmodule.PermissionDefinition{
				{Key: "clinic.patient.create", Action: "create", Resource: "patient_profile", DisplayName: "Create Patients", DisplayNameI18n: localizeClinic("Create Patients", "Buat Pasien")},
				{Key: "clinic.patient.list", Action: "list", Resource: "patient_profile", DisplayName: "List Patients", DisplayNameI18n: localizeClinic("List Patients", "Daftar Pasien")},
				{Key: "clinic.patient.read", Action: "read", Resource: "patient_profile", DisplayName: "Read Patients", DisplayNameI18n: localizeClinic("Read Patients", "Lihat Pasien")},
				{Key: "clinic.patient.update", Action: "update", Resource: "patient_profile", DisplayName: "Update Patients", DisplayNameI18n: localizeClinic("Update Patients", "Perbarui Pasien")},
				{Key: "clinic.practitioner.create", Action: "create", Resource: "practitioner_profile", DisplayName: "Create Practitioners"},
				{Key: "clinic.practitioner.list", Action: "list", Resource: "practitioner_profile", DisplayName: "List Practitioners"},
				{Key: "clinic.practitioner.read", Action: "read", Resource: "practitioner_profile", DisplayName: "Read Practitioners"},
				{Key: "clinic.practitioner.update", Action: "update", Resource: "practitioner_profile", DisplayName: "Update Practitioners"},
				{Key: "clinic.payer.create", Action: "create", Resource: "payer_profile", DisplayName: "Create Payers"},
				{Key: "clinic.payer.list", Action: "list", Resource: "payer_profile", DisplayName: "List Payers"},
				{Key: "clinic.payer.read", Action: "read", Resource: "payer_profile", DisplayName: "Read Payers"},
				{Key: "clinic.payer.update", Action: "update", Resource: "payer_profile", DisplayName: "Update Payers"},
				{Key: "clinic.report.read", Action: "read", Resource: "clinic_report", DisplayName: "Read Clinic Reports"},
				{Key: "clinic.integration.manage", Action: "manage", Resource: "clinic_integration", DisplayName: "Manage Clinic Integrations"},
			},
			RoleTemplates: []platformmodule.RoleTemplateDefinition{
				{Key: "registration_clerk", Name: "Registration Clerk", NameI18n: localizeClinic("Registration Clerk", "Petugas Registrasi"), AllowedScopes: []string{"location"}, PermissionKeys: []string{"clinic.patient.create", "clinic.patient.list", "clinic.patient.read", "clinic.patient.update", "document.create", "document.list", "document.read", "document.update_draft", "document.submit"}},
				{Key: "clinic_reviewer", Name: "Clinic Reviewer", NameI18n: localizeClinic("Clinic Reviewer", "Peninjau Klinik"), AllowedScopes: []string{"location"}, PermissionKeys: []string{"clinic.patient.list", "clinic.patient.read", "document.list", "document.read", "document.approve", "document.reject", "document.reopen", "document.cancel"}},
				{Key: "practitioner", Name: "Practitioner", NameI18n: localizeClinic("Practitioner", "Praktisi"), AllowedScopes: []string{"location"}, PermissionKeys: []string{"clinic.patient.list", "clinic.patient.read", "clinic.practitioner.read", "document.list", "document.read", "document.submit", "document.approve"}},
				{Key: "clinic_supervisor", Name: "Clinic Supervisor", NameI18n: localizeClinic("Clinic Supervisor", "Supervisor Klinik"), AllowedScopes: []string{"location"}, PermissionKeys: []string{"clinic.patient.list", "clinic.patient.read", "clinic.practitioner.list", "clinic.practitioner.read", "clinic.payer.list", "clinic.payer.read", "clinic.report.read", "clinic.integration.manage", "document.list", "document.read", "document.approve", "document.reject", "document.reopen", "document.cancel"}},
			},
			PolicyHooks: []platformmodule.PolicyHookDefinition{
				{Key: "clinic.registration.approval", Kind: "workflow", Target: "clinic_registration_approval"},
			},
		},
		Frontend: platformmodule.FrontendDefinition{
			Menus: []platformmodule.MenuDefinition{
				{Key: "clinic.registrations", Label: "Clinic Registrations", LabelI18n: localizeClinic("Clinic Registrations", "Registrasi Klinik"), ActionKey: "clinic.registrations.list", Order: 40, RequiredPermissions: []string{"document.list"}},
				{Key: "clinic.patients", Label: "Patients", LabelI18n: localizeClinic("Patients", "Pasien"), ActionKey: "clinic.patients.list", Order: 41, RequiredPermissions: []string{"clinic.patient.list"}},
			},
			Actions: []platformmodule.ActionDefinition{
				{Key: "clinic.registrations.list", Label: "Registrations", LabelI18n: localizeClinic("Registrations", "Registrasi"), Kind: "navigate", RoutePath: "/clinic/registrations", ViewKey: "clinic.registrations.list", RenderMode: platformmodule.RenderModeGeneric, RequiredPermissions: []string{"document.list"}},
				{Key: "clinic.encounters.list", Label: "Encounters", LabelI18n: localizeClinic("Encounters", "Encounter"), Kind: "navigate", RoutePath: "/clinic/encounters", ViewKey: "clinic.encounters.list", RenderMode: platformmodule.RenderModeGeneric, RequiredPermissions: []string{"document.list"}},
				{Key: "clinic.patients.list", Label: "Patients", LabelI18n: localizeClinic("Patients", "Pasien"), Kind: "navigate", RoutePath: "/clinic/patients", ViewKey: "clinic.patients.list", RenderMode: platformmodule.RenderModeGeneric, RequiredPermissions: []string{"clinic.patient.list"}},
			},
			Views: []platformmodule.ViewDefinition{
				{
					Key:                 "clinic.registrations.list",
					Title:               "Clinic Registrations",
					TitleI18n:           localizeClinic("Clinic Registrations", "Registrasi Klinik"),
					Kind:                "list",
					DocumentType:        "clinic_registration",
					RequiredPermissions: []string{"document.list"},
					Columns: []platformmodule.ColumnDefinition{
						{Key: "status", Label: "Status", LabelI18n: localizeClinic("Status", "Status"), Path: "header.status"},
						{Key: "number", Label: "Number", LabelI18n: localizeClinic("Number", "Nomor"), Path: "header.number"},
					},
					Filters:         []platformmodule.FilterDefinition{{Key: "status", Label: "Status", LabelI18n: localizeClinic("Status", "Status"), Type: "enum", Options: []string{"draft", "submitted", "approved", "cancelled"}}},
					DefaultPageSize: 10,
				},
				{
					Key:                 "clinic.encounters.list",
					Title:               "Clinic Encounters",
					TitleI18n:           localizeClinic("Clinic Encounters", "Encounter Klinik"),
					Kind:                "list",
					DocumentType:        "clinic_encounter",
					RequiredPermissions: []string{"document.list"},
					Columns: []platformmodule.ColumnDefinition{
						{Key: "status", Label: "Status", LabelI18n: localizeClinic("Status", "Status"), Path: "header.status"},
						{Key: "number", Label: "Number", LabelI18n: localizeClinic("Number", "Nomor"), Path: "header.number"},
					},
					DefaultPageSize: 10,
				},
				{
					Key:                 "clinic.patients.list",
					Title:               "Patients",
					TitleI18n:           localizeClinic("Patients", "Pasien"),
					Kind:                "list",
					ModelKey:            "patient_profile",
					RequiredPermissions: []string{"clinic.patient.list"},
					Columns: []platformmodule.ColumnDefinition{
						{Key: "display_name", Label: "Patient", LabelI18n: localizeClinic("Patient", "Pasien"), Path: "values.display_name"},
						{Key: "patient_identifier_value", Label: "Identifier", LabelI18n: localizeClinic("Identifier", "Identitas"), Path: "values.patient_identifier_value"},
						{Key: "status", Label: "Status", LabelI18n: localizeClinic("Status", "Status"), Path: "values.status"},
					},
					DefaultPageSize: 10,
				},
			},
		},
	}
}
