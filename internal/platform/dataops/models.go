package dataops

import "time"

type DataClass string

const (
	DataClassConfiguration DataClass = "configuration"
	DataClassMaster        DataClass = "master"
	DataClassTransactional DataClass = "transactional"
)

type OperationType string

const (
	OperationBackup    OperationType = "backup"
	OperationRestore   OperationType = "restore"
	OperationArchive   OperationType = "archive"
	OperationExport    OperationType = "export"
	OperationMigration OperationType = "migration"
)

type ArtifactType string

const (
	ArtifactTypeBackup         ArtifactType = "backup"
	ArtifactTypeArchive        ArtifactType = "archive"
	ArtifactTypeExport         ArtifactType = "export"
	ArtifactTypeMigrationInput ArtifactType = "migration_input"
)

type Artifact struct {
	ID        string            `json:"id"`
	Type      ArtifactType      `json:"type"`
	Name      string            `json:"name,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
	CreatedBy string            `json:"created_by,omitempty"`
	Manifest  ArtifactManifest  `json:"manifest"`
	Segments  []ArtifactSegment `json:"segments,omitempty"`
}

type ArtifactManifest struct {
	SchemaVersion    string           `json:"schema_version"`
	OperationType    OperationType    `json:"operation_type"`
	SourceProfile    string           `json:"source_profile,omitempty"`
	DataClasses      []DataClass      `json:"data_classes"`
	BaseArtifactID   string           `json:"base_artifact_id,omitempty"`
	SegmentSummaries []SegmentSummary `json:"segment_summaries,omitempty"`
	Compatibility    map[string]any   `json:"compatibility,omitempty"`
}

type ArtifactSegment struct {
	ID               string    `json:"id"`
	DataClass        DataClass `json:"data_class"`
	AdapterKey       string    `json:"adapter_key"`
	Mode             string    `json:"mode,omitempty"`
	RecordCount      int       `json:"record_count"`
	Checksum         string    `json:"checksum"`
	BaseArtifactID   string    `json:"base_artifact_id,omitempty"`
	BaseCheckpointAt time.Time `json:"base_checkpoint_at,omitempty"`
	CheckpointAt     time.Time `json:"checkpoint_at,omitempty"`
	Payload          any       `json:"payload,omitempty"`
}

type SegmentSummary struct {
	ID               string    `json:"id"`
	DataClass        DataClass `json:"data_class"`
	AdapterKey       string    `json:"adapter_key"`
	Mode             string    `json:"mode,omitempty"`
	RecordCount      int       `json:"record_count"`
	Checksum         string    `json:"checksum"`
	BaseArtifactID   string    `json:"base_artifact_id,omitempty"`
	BaseCheckpointAt time.Time `json:"base_checkpoint_at,omitempty"`
	CheckpointAt     time.Time `json:"checkpoint_at,omitempty"`
}

type IncrementalCheckpoint struct {
	ID           string    `json:"id"`
	DataClass    DataClass `json:"data_class"`
	AdapterKey   string    `json:"adapter_key"`
	ArtifactID   string    `json:"artifact_id,omitempty"`
	CheckpointAt time.Time `json:"checkpoint_at"`
	CreatedAt    time.Time `json:"created_at"`
}

type ValidationIssue struct {
	Code       string    `json:"code"`
	Severity   string    `json:"severity"`
	Message    string    `json:"message"`
	DataClass  DataClass `json:"data_class,omitempty"`
	AdapterKey string    `json:"adapter_key,omitempty"`
	Path       string    `json:"path,omitempty"`
}

type ValidationReport struct {
	Valid  bool              `json:"valid"`
	Issues []ValidationIssue `json:"issues,omitempty"`
}

type OperationRun struct {
	ID                  string           `json:"id"`
	Type                OperationType    `json:"type"`
	Status              string           `json:"status"`
	JobID               string           `json:"job_id,omitempty"`
	ArtifactID          string           `json:"artifact_id,omitempty"`
	SelectedDataClasses []DataClass      `json:"selected_data_classes,omitempty"`
	Validation          ValidationReport `json:"validation"`
	Request             map[string]any   `json:"request,omitempty"`
	Summary             map[string]any   `json:"summary,omitempty"`
	StartedAt           time.Time        `json:"started_at"`
	CompletedAt         time.Time        `json:"completed_at,omitempty"`
	CreatedBy           string           `json:"created_by,omitempty"`
}

type AdapterCapability struct {
	AdapterKey          string    `json:"adapter_key"`
	DataClass           DataClass `json:"data_class"`
	SupportsIncremental bool      `json:"supports_incremental"`
	SupportsArchive     bool      `json:"supports_archive"`
}

type BackupRequest struct {
	Name                string      `json:"name,omitempty"`
	SelectedDataClasses []DataClass `json:"selected_data_classes"`
	Incremental         bool        `json:"incremental,omitempty"`
	ActorID             string      `json:"actor_id,omitempty"`
}

type BackupPlan struct {
	ArtifactType        ArtifactType     `json:"artifact_type"`
	SelectedDataClasses []DataClass      `json:"selected_data_classes"`
	Incremental         bool             `json:"incremental,omitempty"`
	SegmentSummaries    []SegmentSummary `json:"segment_summaries,omitempty"`
	Validation          ValidationReport `json:"validation"`
}

type RestoreRequest struct {
	ArtifactID          string      `json:"artifact_id"`
	SelectedDataClasses []DataClass `json:"selected_data_classes"`
	ActorID             string      `json:"actor_id,omitempty"`
}

type RestorePlan struct {
	ArtifactID          string           `json:"artifact_id"`
	SelectedDataClasses []DataClass      `json:"selected_data_classes"`
	SegmentSummaries    []SegmentSummary `json:"segment_summaries,omitempty"`
	Validation          ValidationReport `json:"validation"`
}

type ArchiveRequest struct {
	Name                string      `json:"name,omitempty"`
	SelectedDataClasses []DataClass `json:"selected_data_classes"`
	DocumentTypes       []string    `json:"document_types,omitempty"`
	Statuses            []string    `json:"statuses,omitempty"`
	OrganizationID      string      `json:"organization_id,omitempty"`
	LocationID          string      `json:"location_id,omitempty"`
	CreatedBefore       time.Time   `json:"created_before,omitempty"`
	ActorID             string      `json:"actor_id,omitempty"`
}

type ExportRequest struct {
	Name                string      `json:"name,omitempty"`
	SelectedDataClasses []DataClass `json:"selected_data_classes"`
	ActorID             string      `json:"actor_id,omitempty"`
}

type MigrationRegisterRequest struct {
	Name                string             `json:"name,omitempty"`
	SelectedDataClasses []DataClass        `json:"selected_data_classes"`
	Segments            []MigrationSegment `json:"segments"`
	ActorID             string             `json:"actor_id,omitempty"`
}

type MigrationSegment struct {
	DataClass  DataClass `json:"data_class"`
	AdapterKey string    `json:"adapter_key"`
	Records    []any     `json:"records,omitempty"`
}
