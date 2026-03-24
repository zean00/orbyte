package app

import (
	"time"

	"orbyte/internal/platform/module"
	"orbyte/internal/platform/reference"
)

func referenceMasterdataKernelPackManifest(seededAt time.Time) module.Manifest {
	return module.Manifest{
		Key:          "reference_masterdata",
		Name:         "Reference Master Data",
		NameI18n:     localize("Reference Master Data", "Data Referensi"),
		Version:      "1.0.0",
		DomainFamily: "platform",
		DependencyRequirements: []module.DependencyRequirement{
			{ModuleKey: "platform.core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
		},
		ReferenceTypes: []reference.TypeDefinition{
			{Key: "currency", DisplayName: "Currency", DisplayNameI18n: localize("Currency", "Mata Uang"), OwnerModuleKey: "reference_masterdata"},
			{Key: "uom", DisplayName: "Unit of Measure", DisplayNameI18n: localize("Unit of Measure", "Satuan Ukur"), OwnerModuleKey: "reference_masterdata"},
			{Key: "party_type", DisplayName: "Party Type", DisplayNameI18n: localize("Party Type", "Tipe Pihak"), OwnerModuleKey: "reference_masterdata"},
			{Key: "location_type", DisplayName: "Location Type", DisplayNameI18n: localize("Location Type", "Tipe Lokasi"), OwnerModuleKey: "reference_masterdata"},
			{Key: "document_reason", DisplayName: "Document Reason", DisplayNameI18n: localize("Document Reason", "Alasan Dokumen"), OwnerModuleKey: "reference_masterdata"},
			{Key: "appointment_type", DisplayName: "Appointment Type", DisplayNameI18n: localize("Appointment Type", "Tipe Janji Temu"), OwnerModuleKey: "reference_masterdata"},
			{Key: "patient_identifier_type", DisplayName: "Patient Identifier Type", DisplayNameI18n: localize("Patient Identifier Type", "Tipe Identitas Pasien"), OwnerModuleKey: "reference_masterdata"},
			{Key: "practitioner_type", DisplayName: "Practitioner Type", DisplayNameI18n: localize("Practitioner Type", "Tipe Praktisi"), OwnerModuleKey: "reference_masterdata"},
			{Key: "payer_type", DisplayName: "Payer Type", DisplayNameI18n: localize("Payer Type", "Tipe Penjamin"), OwnerModuleKey: "reference_masterdata"},
			{Key: "visit_priority", DisplayName: "Visit Priority", DisplayNameI18n: localize("Visit Priority", "Prioritas Kunjungan"), OwnerModuleKey: "reference_masterdata"},
			{Key: "shipment_method", DisplayName: "Shipment Method", DisplayNameI18n: localize("Shipment Method", "Metode Pengiriman"), OwnerModuleKey: "reference_masterdata"},
			{Key: "item_category", DisplayName: "Item Category", DisplayNameI18n: localize("Item Category", "Kategori Item"), OwnerModuleKey: "reference_masterdata"},
		},
		ReferenceRecords: []reference.Record{
			{TypeKey: "currency", Key: "IDR", DisplayName: "Indonesian Rupiah", DisplayNameI18n: localize("Indonesian Rupiah", "Rupiah Indonesia"), Scope: "deployment", UpdatedAt: seededAt, UpdatedBy: "system", Value: map[string]any{"currency_code": "IDR", "minor_unit_scale": 2, "display_symbol": "Rp"}},
			{TypeKey: "uom", Key: "ea", DisplayName: "Each", DisplayNameI18n: localize("Each", "Per Unit"), Scope: "deployment", UpdatedAt: seededAt, UpdatedBy: "system", Value: map[string]any{"uom_code": "ea", "dimension": "count", "precision_scale": 0}},
			{TypeKey: "party_type", Key: "patient", DisplayName: "Patient", DisplayNameI18n: localize("Patient", "Pasien"), Scope: "deployment", UpdatedAt: seededAt, UpdatedBy: "system", Value: map[string]any{"reference_type": "party_type"}},
			{TypeKey: "party_type", Key: "payer", DisplayName: "Payer", DisplayNameI18n: localize("Payer", "Penjamin"), Scope: "deployment", UpdatedAt: seededAt, UpdatedBy: "system", Value: map[string]any{"reference_type": "party_type"}},
			{TypeKey: "party_type", Key: "practitioner", DisplayName: "Practitioner", DisplayNameI18n: localize("Practitioner", "Praktisi"), Scope: "deployment", UpdatedAt: seededAt, UpdatedBy: "system", Value: map[string]any{"reference_type": "party_type"}},
			{TypeKey: "location_type", Key: "clinic", DisplayName: "Clinic", DisplayNameI18n: localize("Clinic", "Klinik"), Scope: "deployment", UpdatedAt: seededAt, UpdatedBy: "system", Value: map[string]any{"reference_type": "location_type"}},
			{TypeKey: "document_reason", Key: "walk_in", DisplayName: "Walk-In Visit", DisplayNameI18n: localize("Walk-In Visit", "Kunjungan Langsung"), Scope: "deployment", UpdatedAt: seededAt, UpdatedBy: "system", Value: map[string]any{"reference_type": "document_reason"}},
			{TypeKey: "document_reason", Key: "follow_up", DisplayName: "Follow Up", DisplayNameI18n: localize("Follow Up", "Kontrol Lanjutan"), Scope: "deployment", UpdatedAt: seededAt, UpdatedBy: "system", Value: map[string]any{"reference_type": "document_reason"}},
			{TypeKey: "appointment_type", Key: "consultation", DisplayName: "Consultation", DisplayNameI18n: localize("Consultation", "Konsultasi"), Scope: "deployment", UpdatedAt: seededAt, UpdatedBy: "system", Value: map[string]any{"reference_type": "appointment_type"}},
			{TypeKey: "patient_identifier_type", Key: "mrn", DisplayName: "Medical Record Number", DisplayNameI18n: localize("Medical Record Number", "Nomor Rekam Medis"), Scope: "deployment", UpdatedAt: seededAt, UpdatedBy: "system", Value: map[string]any{"reference_type": "patient_identifier_type"}},
			{TypeKey: "practitioner_type", Key: "doctor", DisplayName: "Doctor", DisplayNameI18n: localize("Doctor", "Dokter"), Scope: "deployment", UpdatedAt: seededAt, UpdatedBy: "system", Value: map[string]any{"reference_type": "practitioner_type"}},
			{TypeKey: "practitioner_type", Key: "nurse", DisplayName: "Nurse", DisplayNameI18n: localize("Nurse", "Perawat"), Scope: "deployment", UpdatedAt: seededAt, UpdatedBy: "system", Value: map[string]any{"reference_type": "practitioner_type"}},
			{TypeKey: "payer_type", Key: "self_pay", DisplayName: "Self Pay", DisplayNameI18n: localize("Self Pay", "Bayar Sendiri"), Scope: "deployment", UpdatedAt: seededAt, UpdatedBy: "system", Value: map[string]any{"reference_type": "payer_type"}},
			{TypeKey: "payer_type", Key: "insurance", DisplayName: "Insurance", DisplayNameI18n: localize("Insurance", "Asuransi"), Scope: "deployment", UpdatedAt: seededAt, UpdatedBy: "system", Value: map[string]any{"reference_type": "payer_type"}},
			{TypeKey: "visit_priority", Key: "routine", DisplayName: "Routine", DisplayNameI18n: localize("Routine", "Rutin"), Scope: "deployment", UpdatedAt: seededAt, UpdatedBy: "system", Value: map[string]any{"reference_type": "visit_priority"}},
			{TypeKey: "visit_priority", Key: "urgent", DisplayName: "Urgent", DisplayNameI18n: localize("Urgent", "Mendesak"), Scope: "deployment", UpdatedAt: seededAt, UpdatedBy: "system", Value: map[string]any{"reference_type": "visit_priority"}},
		},
		Offline: module.OfflineDefinition{
			References: []module.OfflineReferenceDefinition{
				{TypeKey: "appointment_type", Title: "Appointment Types", TitleI18n: localize("Appointment Types", "Jenis Janji Temu")},
				{TypeKey: "party_type", Title: "Party Types", TitleI18n: localize("Party Types", "Jenis Pihak")},
			},
		},
	}
}
