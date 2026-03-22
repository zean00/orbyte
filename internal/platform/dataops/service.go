package dataops

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"orbyte/internal/platform/config"
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/featureflags"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/integration"
	"orbyte/internal/platform/jobs"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/reference"
	"orbyte/internal/platform/shared"
)

const JobExecuteOperation = "dataops.execute_operation"

type Service struct {
	repo        Repository
	config      *config.Service
	flags       *featureflags.Service
	modules     *module.Service
	reference   *reference.Service
	identity    *identity.Service
	documents   *document.Service
	integration *integration.Service
	jobs        *jobs.Service
}

func NewService(cfg *config.Service, flags *featureflags.Service, modules *module.Service, referenceSvc *reference.Service, identitySvc *identity.Service, documentSvc *document.Service, integrationSvc *integration.Service) *Service {
	return NewServiceWithRepository(NewMemoryRepository(), cfg, flags, modules, referenceSvc, identitySvc, documentSvc, integrationSvc)
}

func NewServiceWithRepository(repo Repository, cfg *config.Service, flags *featureflags.Service, modules *module.Service, referenceSvc *reference.Service, identitySvc *identity.Service, documentSvc *document.Service, integrationSvc *integration.Service) *Service {
	if repo == nil {
		repo = NewMemoryRepository()
	}
	return &Service{
		repo:        repo,
		config:      cfg,
		flags:       flags,
		modules:     modules,
		reference:   referenceSvc,
		identity:    identitySvc,
		documents:   documentSvc,
		integration: integrationSvc,
	}
}

func (s *Service) Capabilities() []AdapterCapability {
	items := []AdapterCapability{
		{AdapterKey: "config.entries", DataClass: DataClassConfiguration, SupportsIncremental: true},
		{AdapterKey: "feature_flags.values", DataClass: DataClassConfiguration, SupportsIncremental: true},
		{AdapterKey: "module.installed", DataClass: DataClassConfiguration, SupportsIncremental: true},
		{AdapterKey: "integration.systems", DataClass: DataClassConfiguration, SupportsIncremental: true},
		{AdapterKey: "integration.endpoints", DataClass: DataClassConfiguration, SupportsIncremental: true},
		{AdapterKey: "integration.contracts", DataClass: DataClassConfiguration, SupportsIncremental: true},
		{AdapterKey: "integration.mappings", DataClass: DataClassConfiguration, SupportsIncremental: true},
		{AdapterKey: "reference.types", DataClass: DataClassMaster, SupportsIncremental: false},
		{AdapterKey: "reference.records", DataClass: DataClassMaster, SupportsIncremental: true},
		{AdapterKey: "identity.roles", DataClass: DataClassMaster, SupportsIncremental: true},
		{AdapterKey: "identity.permissions", DataClass: DataClassMaster, SupportsIncremental: false},
		{AdapterKey: "identity.role_permissions", DataClass: DataClassMaster, SupportsIncremental: false},
		{AdapterKey: "identity.users", DataClass: DataClassMaster, SupportsIncremental: true},
		{AdapterKey: "identity.role_bindings", DataClass: DataClassMaster, SupportsIncremental: false},
		{AdapterKey: "identity.service_principals", DataClass: DataClassMaster, SupportsIncremental: true},
		{AdapterKey: "identity.reporting_lines", DataClass: DataClassMaster, SupportsIncremental: true},
		{AdapterKey: "documents.records", DataClass: DataClassTransactional, SupportsIncremental: true, SupportsArchive: true},
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].DataClass == items[j].DataClass {
			return items[i].AdapterKey < items[j].AdapterKey
		}
		return items[i].DataClass < items[j].DataClass
	})
	return items
}

func (s *Service) Artifacts() []Artifact {
	return s.repo.ListArtifacts()
}

func (s *Service) Artifact(id string) (Artifact, bool) {
	return s.repo.GetArtifact(strings.TrimSpace(id))
}

func (s *Service) Operation(id string) (OperationRun, bool) {
	return s.repo.GetOperation(strings.TrimSpace(id))
}

func (s *Service) Checkpoints() []IncrementalCheckpoint {
	return s.repo.ListCheckpoints()
}

func (s *Service) AttachJobs(jobSvc *jobs.Service) {
	s.jobs = jobSvc
	if jobSvc == nil {
		return
	}
	jobSvc.RegisterHandler(JobExecuteOperation, func(_ context.Context, payload map[string]any) (map[string]any, error) {
		operationID, _ := payload["operation_id"].(string)
		return s.executeOperation(operationID)
	})
}

func (s *Service) BackupPlan(req BackupRequest) (BackupPlan, error) {
	classes, err := normalizeDataClasses(req.SelectedDataClasses)
	if err != nil {
		return BackupPlan{}, err
	}
	segments, issues, err := s.collectBackupSegments(classes, req.Incremental)
	if err != nil {
		return BackupPlan{}, err
	}
	return BackupPlan{
		ArtifactType:        ArtifactTypeBackup,
		SelectedDataClasses: classes,
		Incremental:         req.Incremental,
		SegmentSummaries:    summarizeSegments(segments),
		Validation:          reportFromIssues(issues),
	}, nil
}

func (s *Service) BackupRun(req BackupRequest) (OperationRun, jobs.Job, error) {
	plan, err := s.BackupPlan(req)
	if err != nil {
		return OperationRun{}, jobs.Job{}, err
	}
	return s.enqueueOperation(OperationBackup, plan.Validation, plan.SelectedDataClasses, backupRequestMap(req), req.ActorID)
}

func (s *Service) ExportPlan(req ExportRequest) (BackupPlan, error) {
	classes, err := normalizeDataClasses(req.SelectedDataClasses)
	if err != nil {
		return BackupPlan{}, err
	}
	segments, issues, err := s.collectExportSegments(classes)
	if err != nil {
		return BackupPlan{}, err
	}
	return BackupPlan{
		ArtifactType:        ArtifactTypeExport,
		SelectedDataClasses: classes,
		SegmentSummaries:    summarizeSegments(segments),
		Validation:          reportFromIssues(issues),
	}, nil
}

func (s *Service) ExportRun(req ExportRequest) (OperationRun, jobs.Job, error) {
	plan, err := s.ExportPlan(req)
	if err != nil {
		return OperationRun{}, jobs.Job{}, err
	}
	return s.enqueueOperation(OperationExport, plan.Validation, plan.SelectedDataClasses, exportRequestMap(req), req.ActorID)
}

func (s *Service) RestorePlan(req RestoreRequest) (RestorePlan, error) {
	artifact, ok := s.repo.GetArtifact(strings.TrimSpace(req.ArtifactID))
	if !ok {
		return RestorePlan{}, shared.NotFound("artifact not found")
	}
	if artifact.Type != ArtifactTypeBackup && artifact.Type != ArtifactTypeArchive {
		return RestorePlan{}, shared.Validation("artifact is not restorable")
	}
	classes, err := normalizeSelectedOrDefault(req.SelectedDataClasses, artifact.Manifest.DataClasses)
	if err != nil {
		return RestorePlan{}, err
	}
	segments, lineageIssues, err := s.resolveRestoreSegments(artifact, classes)
	if err != nil {
		return RestorePlan{}, err
	}
	issues := append([]ValidationIssue(nil), lineageIssues...)
	if len(segments) == 0 {
		issues = append(issues, ValidationIssue{Code: "segment_missing", Severity: "error", Message: "artifact does not contain the selected data classes"})
	}
	issues = append(issues, s.validateRestoreSegments(segments)...)
	return RestorePlan{
		ArtifactID:          artifact.ID,
		SelectedDataClasses: classes,
		SegmentSummaries:    summarizeSegments(segments),
		Validation:          reportFromIssues(issues),
	}, nil
}

func (s *Service) RestoreRun(req RestoreRequest) (OperationRun, jobs.Job, error) {
	plan, err := s.RestorePlan(req)
	if err != nil {
		return OperationRun{}, jobs.Job{}, err
	}
	if !plan.Validation.Valid {
		return OperationRun{}, jobs.Job{}, shared.Validation("restore validation failed")
	}
	return s.enqueueOperation(OperationRestore, plan.Validation, plan.SelectedDataClasses, restoreRequestMap(req), req.ActorID)
}

func (s *Service) ArchivePlan(req ArchiveRequest) (BackupPlan, error) {
	classes, err := normalizeDataClasses(req.SelectedDataClasses)
	if err != nil {
		return BackupPlan{}, err
	}
	if len(classes) != 1 || classes[0] != DataClassTransactional {
		return BackupPlan{}, shared.Validation("archive only supports transactional data")
	}
	segments, issues, err := s.collectArchiveSegments(req)
	if err != nil {
		return BackupPlan{}, err
	}
	return BackupPlan{
		ArtifactType:        ArtifactTypeArchive,
		SelectedDataClasses: classes,
		SegmentSummaries:    summarizeSegments(segments),
		Validation:          reportFromIssues(issues),
	}, nil
}

func (s *Service) ArchiveRun(req ArchiveRequest) (OperationRun, jobs.Job, error) {
	plan, err := s.ArchivePlan(req)
	if err != nil {
		return OperationRun{}, jobs.Job{}, err
	}
	return s.enqueueOperation(OperationArchive, plan.Validation, plan.SelectedDataClasses, archiveRequestMap(req), req.ActorID)
}

func (s *Service) MigrationRegister(req MigrationRegisterRequest) (Artifact, error) {
	classes, err := normalizeDataClasses(req.SelectedDataClasses)
	if err != nil {
		return Artifact{}, err
	}
	now := time.Now().UTC()
	segments := make([]ArtifactSegment, 0, len(req.Segments))
	for _, item := range req.Segments {
		if !containsDataClass(classes, item.DataClass) {
			return Artifact{}, shared.Validation("migration segment data_class is not selected")
		}
		payload := map[string]any{"records": item.Records}
		segments = append(segments, ArtifactSegment{
			ID:          fmt.Sprintf("segment:%d", time.Now().UTC().UnixNano()),
			DataClass:   item.DataClass,
			AdapterKey:  strings.TrimSpace(item.AdapterKey),
			Mode:        "external",
			RecordCount: len(item.Records),
			Checksum:    checksum(payload),
			Payload:     payload,
		})
	}
	artifact := Artifact{
		ID:        fmt.Sprintf("artifact:%d", now.UnixNano()),
		Type:      ArtifactTypeMigrationInput,
		Name:      firstNonEmpty(strings.TrimSpace(req.Name), "migration_input"),
		CreatedAt: now,
		CreatedBy: req.ActorID,
		Manifest: ArtifactManifest{
			SchemaVersion:    "v1",
			OperationType:    OperationMigration,
			SourceProfile:    "external",
			DataClasses:      classes,
			SegmentSummaries: summarizeSegments(segments),
			Compatibility:    map[string]any{"format": "structured_json"},
		},
		Segments: segments,
	}
	if err := s.repo.SaveArtifact(artifact); err != nil {
		return Artifact{}, err
	}
	return artifact, nil
}

func (s *Service) MigrationPlan(req RestoreRequest) (RestorePlan, error) {
	artifact, ok := s.repo.GetArtifact(strings.TrimSpace(req.ArtifactID))
	if !ok {
		return RestorePlan{}, shared.NotFound("artifact not found")
	}
	if artifact.Type != ArtifactTypeMigrationInput {
		return RestorePlan{}, shared.Validation("artifact is not a migration input")
	}
	classes, err := normalizeSelectedOrDefault(req.SelectedDataClasses, artifact.Manifest.DataClasses)
	if err != nil {
		return RestorePlan{}, err
	}
	segments := filterSegmentsByClasses(artifact.Segments, classes)
	issues := s.validateMigrationSegments(segments)
	return RestorePlan{
		ArtifactID:          artifact.ID,
		SelectedDataClasses: classes,
		SegmentSummaries:    summarizeSegments(segments),
		Validation:          reportFromIssues(issues),
	}, nil
}

func (s *Service) MigrationRun(req RestoreRequest) (OperationRun, jobs.Job, error) {
	plan, err := s.MigrationPlan(req)
	if err != nil {
		return OperationRun{}, jobs.Job{}, err
	}
	if !plan.Validation.Valid {
		return OperationRun{}, jobs.Job{}, shared.Validation("migration validation failed")
	}
	return s.enqueueOperation(OperationMigration, plan.Validation, plan.SelectedDataClasses, restoreRequestMap(req), req.ActorID)
}

func (s *Service) enqueueOperation(opType OperationType, validation ValidationReport, classes []DataClass, request map[string]any, actorID string) (OperationRun, jobs.Job, error) {
	if s == nil || s.jobs == nil {
		return OperationRun{}, jobs.Job{}, shared.Conflict("data operations jobs are not configured")
	}
	now := time.Now().UTC()
	run := OperationRun{
		ID:                  fmt.Sprintf("run:%d", now.UnixNano()),
		Type:                opType,
		Status:              jobs.StatusQueued,
		SelectedDataClasses: append([]DataClass(nil), classes...),
		Validation:          validation,
		Request:             cloneMap(request),
		StartedAt:           now,
		CreatedBy:           actorID,
	}
	if err := s.repo.SaveOperation(run); err != nil {
		return OperationRun{}, jobs.Job{}, err
	}
	job, err := s.jobs.EnqueueUnique(JobExecuteOperation, map[string]any{"operation_id": run.ID, "correlation_id": run.ID}, JobExecuteOperation+":"+run.ID)
	if err != nil {
		return OperationRun{}, jobs.Job{}, err
	}
	run.JobID = job.ID
	if err := s.repo.SaveOperation(run); err != nil {
		return OperationRun{}, jobs.Job{}, err
	}
	return run, job, nil
}

func (s *Service) executeOperation(operationID string) (map[string]any, error) {
	run, ok := s.repo.GetOperation(strings.TrimSpace(operationID))
	if !ok {
		return nil, shared.NotFound("data operation not found")
	}
	run.Status = jobs.StatusRunning
	_ = s.repo.SaveOperation(run)
	var (
		artifactID string
		summary    map[string]any
		err        error
	)
	switch run.Type {
	case OperationBackup:
		var req BackupRequest
		req, err = backupRequestFromMap(run.Request)
		if err == nil {
			artifactID, summary, err = s.executeBackup(req)
		}
	case OperationExport:
		var req ExportRequest
		req, err = exportRequestFromMap(run.Request)
		if err == nil {
			artifactID, summary, err = s.executeExport(req)
		}
	case OperationRestore:
		var req RestoreRequest
		req, err = restoreRequestFromMap(run.Request)
		if err == nil {
			summary, err = s.executeRestore(req)
			artifactID = req.ArtifactID
		}
	case OperationArchive:
		var req ArchiveRequest
		req, err = archiveRequestFromMap(run.Request)
		if err == nil {
			artifactID, summary, err = s.executeArchive(req)
		}
	case OperationMigration:
		var req RestoreRequest
		req, err = restoreRequestFromMap(run.Request)
		if err == nil {
			summary, err = s.executeMigration(req)
			artifactID = req.ArtifactID
		}
	default:
		err = shared.Validation("data operation type is invalid")
	}
	if err != nil {
		run.Status = jobs.StatusFailed
		run.CompletedAt = time.Now().UTC()
		run.Summary = mergeSummary(run.Summary, map[string]any{"error": err.Error()})
		_ = s.repo.SaveOperation(run)
		return nil, err
	}
	run.Status = jobs.StatusSucceeded
	run.ArtifactID = artifactID
	run.CompletedAt = time.Now().UTC()
	run.Summary = mergeSummary(run.Summary, summary)
	if err := s.repo.SaveOperation(run); err != nil {
		return nil, err
	}
	return map[string]any{"operation_id": run.ID, "artifact_id": artifactID, "summary": run.Summary}, nil
}

func (s *Service) executeBackup(req BackupRequest) (string, map[string]any, error) {
	plan, err := s.BackupPlan(req)
	if err != nil {
		return "", nil, err
	}
	now := time.Now().UTC()
	segments, _, err := s.collectBackupSegments(plan.SelectedDataClasses, req.Incremental)
	if err != nil {
		return "", nil, err
	}
	baseArtifactID := commonBaseArtifactID(segments)
	artifact := Artifact{
		ID:        fmt.Sprintf("artifact:%d", now.UnixNano()),
		Type:      ArtifactTypeBackup,
		Name:      firstNonEmpty(strings.TrimSpace(req.Name), "backup"),
		CreatedAt: now,
		CreatedBy: req.ActorID,
		Manifest: ArtifactManifest{
			SchemaVersion:    "v1",
			OperationType:    OperationBackup,
			SourceProfile:    "orbyte",
			DataClasses:      plan.SelectedDataClasses,
			BaseArtifactID:   baseArtifactID,
			SegmentSummaries: summarizeSegments(segments),
		},
		Segments: segments,
	}
	if err := s.repo.SaveArtifact(artifact); err != nil {
		return "", nil, err
	}
	for _, segment := range artifact.Segments {
		if segment.CheckpointAt.IsZero() {
			continue
		}
		_ = s.repo.SaveCheckpoint(IncrementalCheckpoint{
			ID:           fmt.Sprintf("checkpoint:%d", time.Now().UTC().UnixNano()),
			DataClass:    segment.DataClass,
			AdapterKey:   segment.AdapterKey,
			ArtifactID:   artifact.ID,
			CheckpointAt: segment.CheckpointAt,
			CreatedAt:    now,
		})
	}
	return artifact.ID, map[string]any{"segment_count": len(artifact.Segments)}, nil
}

func (s *Service) executeExport(req ExportRequest) (string, map[string]any, error) {
	plan, err := s.ExportPlan(req)
	if err != nil {
		return "", nil, err
	}
	now := time.Now().UTC()
	segments, _, err := s.collectExportSegments(plan.SelectedDataClasses)
	if err != nil {
		return "", nil, err
	}
	artifact := Artifact{
		ID:        fmt.Sprintf("artifact:%d", now.UnixNano()),
		Type:      ArtifactTypeExport,
		Name:      firstNonEmpty(strings.TrimSpace(req.Name), "export"),
		CreatedAt: now,
		CreatedBy: req.ActorID,
		Manifest: ArtifactManifest{
			SchemaVersion:    "v1",
			OperationType:    OperationExport,
			SourceProfile:    "orbyte",
			DataClasses:      plan.SelectedDataClasses,
			SegmentSummaries: summarizeSegments(segments),
			Compatibility:    map[string]any{"format": "structured_json"},
		},
		Segments: segments,
	}
	if err := s.repo.SaveArtifact(artifact); err != nil {
		return "", nil, err
	}
	return artifact.ID, map[string]any{"segment_count": len(artifact.Segments)}, nil
}

func (s *Service) executeRestore(req RestoreRequest) (map[string]any, error) {
	plan, err := s.RestorePlan(req)
	if err != nil {
		return nil, err
	}
	if !plan.Validation.Valid {
		return nil, shared.Validation("restore validation failed")
	}
	artifact, _ := s.repo.GetArtifact(strings.TrimSpace(req.ArtifactID))
	segments, _, err := s.resolveRestoreSegments(artifact, plan.SelectedDataClasses)
	if err != nil {
		return nil, err
	}
	segments = orderSegmentsForApply(segments)
	for _, item := range restoreOrderForClasses(plan.SelectedDataClasses) {
		for _, segment := range segments {
			if segment.DataClass != item {
				continue
			}
			if err := s.applyInternalSegment(segment, req.ActorID); err != nil {
				return nil, err
			}
		}
	}
	return map[string]any{"segment_count": len(segments)}, nil
}

func (s *Service) executeArchive(req ArchiveRequest) (string, map[string]any, error) {
	plan, err := s.ArchivePlan(req)
	if err != nil {
		return "", nil, err
	}
	now := time.Now().UTC()
	segments, _, err := s.collectArchiveSegments(req)
	if err != nil {
		return "", nil, err
	}
	artifact := Artifact{
		ID:        fmt.Sprintf("artifact:%d", now.UnixNano()),
		Type:      ArtifactTypeArchive,
		Name:      firstNonEmpty(strings.TrimSpace(req.Name), "archive"),
		CreatedAt: now,
		CreatedBy: req.ActorID,
		Manifest: ArtifactManifest{
			SchemaVersion:    "v1",
			OperationType:    OperationArchive,
			SourceProfile:    "orbyte",
			DataClasses:      plan.SelectedDataClasses,
			SegmentSummaries: summarizeSegments(segments),
		},
		Segments: segments,
	}
	if err := s.repo.SaveArtifact(artifact); err != nil {
		return "", nil, err
	}
	for _, segment := range segments {
		if segment.AdapterKey != "documents.records" {
			continue
		}
		records, err := decodeRecords[document.Record](segment.Payload)
		if err != nil {
			return "", nil, err
		}
		for _, record := range records {
			if err := s.documents.Delete(record.Header.ID); err != nil {
				return "", nil, err
			}
		}
	}
	return artifact.ID, map[string]any{"segment_count": len(segments)}, nil
}

func (s *Service) executeMigration(req RestoreRequest) (map[string]any, error) {
	plan, err := s.MigrationPlan(req)
	if err != nil {
		return nil, err
	}
	if !plan.Validation.Valid {
		return nil, shared.Validation("migration validation failed")
	}
	artifact, _ := s.repo.GetArtifact(strings.TrimSpace(req.ArtifactID))
	segments := orderSegmentsForApply(filterSegmentsByClasses(artifact.Segments, plan.SelectedDataClasses))
	for _, item := range restoreOrderForClasses(plan.SelectedDataClasses) {
		for _, segment := range segments {
			if segment.DataClass != item {
				continue
			}
			if err := s.applyMigrationSegment(segment, req.ActorID); err != nil {
				return nil, err
			}
		}
	}
	return map[string]any{"segment_count": len(segments)}, nil
}

func (s *Service) collectBackupSegments(classes []DataClass, incremental bool) ([]ArtifactSegment, []ValidationIssue, error) {
	segments := make([]ArtifactSegment, 0)
	issues := make([]ValidationIssue, 0)
	for _, capability := range s.Capabilities() {
		if !containsDataClass(classes, capability.DataClass) {
			continue
		}
		baseCheckpoint, _ := s.latestCheckpoint(capability.AdapterKey)
		baseArtifactID := s.latestArtifactForAdapter(capability.AdapterKey)
		mode := "full"
		if incremental && capability.SupportsIncremental {
			mode = "incremental"
		}
		if incremental && !capability.SupportsIncremental {
			issues = append(issues, ValidationIssue{Code: "incremental_fallback", Severity: "warning", Message: fmt.Sprintf("%s falls back to full snapshot because it has no incremental watermark.", capability.AdapterKey), DataClass: capability.DataClass, AdapterKey: capability.AdapterKey})
		}
		payload, updatedAt, count, err := s.extractInternalSegment(capability.AdapterKey, mode == "incremental", baseCheckpoint)
		if err != nil {
			return nil, nil, err
		}
		segments = append(segments, ArtifactSegment{
			ID:               fmt.Sprintf("segment:%d", time.Now().UTC().UnixNano()),
			DataClass:        capability.DataClass,
			AdapterKey:       capability.AdapterKey,
			Mode:             mode,
			RecordCount:      count,
			Checksum:         checksum(payload),
			BaseArtifactID:   baseArtifactID,
			BaseCheckpointAt: baseCheckpoint,
			CheckpointAt:     updatedAt,
			Payload:          payload,
		})
	}
	return segments, issues, nil
}

func (s *Service) collectExportSegments(classes []DataClass) ([]ArtifactSegment, []ValidationIssue, error) {
	segments := make([]ArtifactSegment, 0)
	for _, capability := range s.Capabilities() {
		if !containsDataClass(classes, capability.DataClass) {
			continue
		}
		payload, count, err := s.extractExportSegment(capability.AdapterKey)
		if err != nil {
			return nil, nil, err
		}
		segments = append(segments, ArtifactSegment{
			ID:          fmt.Sprintf("segment:%d", time.Now().UTC().UnixNano()),
			DataClass:   capability.DataClass,
			AdapterKey:  capability.AdapterKey,
			Mode:        "export",
			RecordCount: count,
			Checksum:    checksum(payload),
			Payload:     payload,
		})
	}
	return segments, nil, nil
}

func (s *Service) collectArchiveSegments(req ArchiveRequest) ([]ArtifactSegment, []ValidationIssue, error) {
	records := make([]document.Record, 0)
	for _, record := range s.documents.List() {
		if len(req.DocumentTypes) > 0 && !containsString(req.DocumentTypes, record.Header.Type) {
			continue
		}
		if len(req.Statuses) > 0 && !containsString(req.Statuses, record.Header.Status) {
			continue
		}
		if req.OrganizationID != "" && record.Header.OrganizationID != req.OrganizationID {
			continue
		}
		if req.LocationID != "" && record.Header.LocationID != req.LocationID {
			continue
		}
		if !req.CreatedBefore.IsZero() && !record.Header.CreatedAt.Before(req.CreatedBefore) {
			continue
		}
		records = append(records, record)
	}
	payload := map[string]any{"records": records}
	updatedAt := maxDocumentUpdatedAt(records)
	return []ArtifactSegment{{
		ID:           fmt.Sprintf("segment:%d", time.Now().UTC().UnixNano()),
		DataClass:    DataClassTransactional,
		AdapterKey:   "documents.records",
		Mode:         "archive",
		RecordCount:  len(records),
		Checksum:     checksum(payload),
		CheckpointAt: updatedAt,
		Payload:      payload,
	}}, nil, nil
}

func (s *Service) validateRestoreSegments(segments []ArtifactSegment) []ValidationIssue {
	issues := make([]ValidationIssue, 0)
	for _, segment := range segments {
		switch segment.AdapterKey {
		case "config.entries":
			records, err := decodeRecords[config.Entry](segment.Payload)
			if err != nil {
				issues = append(issues, ValidationIssue{Code: "decode_error", Severity: "error", Message: "invalid configuration segment payload", DataClass: segment.DataClass, AdapterKey: segment.AdapterKey})
				continue
			}
			for _, item := range records {
				if _, ok := s.config.Definition(item.Key); !ok {
					issues = append(issues, ValidationIssue{Code: "config_definition_missing", Severity: "error", Message: fmt.Sprintf("configuration definition %s is not registered", item.Key), DataClass: segment.DataClass, AdapterKey: segment.AdapterKey, Path: item.Key})
				}
			}
		case "feature_flags.values":
			records, err := decodeRecords[featureflags.Value](segment.Payload)
			if err != nil {
				issues = append(issues, ValidationIssue{Code: "decode_error", Severity: "error", Message: "invalid feature flag segment payload", DataClass: segment.DataClass, AdapterKey: segment.AdapterKey})
				continue
			}
			defs := map[string]bool{}
			for _, def := range s.flags.Definitions() {
				defs[def.Key] = true
			}
			for _, item := range records {
				if !defs[item.FlagKey] {
					issues = append(issues, ValidationIssue{Code: "feature_flag_definition_missing", Severity: "error", Message: fmt.Sprintf("feature flag %s is not registered", item.FlagKey), DataClass: segment.DataClass, AdapterKey: segment.AdapterKey, Path: item.FlagKey})
				}
			}
		case "module.installed":
			items, err := decodeRecords[module.InstalledModule](segment.Payload)
			if err != nil {
				issues = append(issues, ValidationIssue{Code: "decode_error", Severity: "error", Message: "invalid module segment payload", DataClass: segment.DataClass, AdapterKey: segment.AdapterKey})
				continue
			}
			known := map[string]bool{}
			for _, detail := range s.modules.List() {
				known[detail.Manifest.Key] = true
			}
			for _, item := range items {
				if !known[item.Key] {
					issues = append(issues, ValidationIssue{Code: "module_missing", Severity: "error", Message: fmt.Sprintf("module %s is not registered", item.Key), DataClass: segment.DataClass, AdapterKey: segment.AdapterKey, Path: item.Key})
				}
			}
		case "documents.records":
			records, err := decodeRecords[document.Record](segment.Payload)
			if err != nil {
				issues = append(issues, ValidationIssue{Code: "decode_error", Severity: "error", Message: "invalid document segment payload", DataClass: segment.DataClass, AdapterKey: segment.AdapterKey})
				continue
			}
			for _, item := range records {
				if _, err := s.documents.Definition(item.Header.Type); err != nil {
					issues = append(issues, ValidationIssue{Code: "document_definition_missing", Severity: "error", Message: fmt.Sprintf("document type %s is not registered", item.Header.Type), DataClass: segment.DataClass, AdapterKey: segment.AdapterKey, Path: item.Header.Type})
				}
			}
		}
	}
	return issues
}

func (s *Service) validateMigrationSegments(segments []ArtifactSegment) []ValidationIssue {
	issues := make([]ValidationIssue, 0)
	for _, segment := range segments {
		if !supportsMigrationAdapter(segment.AdapterKey) {
			issues = append(issues, ValidationIssue{
				Code:       "migration_adapter_unsupported",
				Severity:   "error",
				Message:    fmt.Sprintf("migration adapter %s is not supported", segment.AdapterKey),
				DataClass:  segment.DataClass,
				AdapterKey: segment.AdapterKey,
			})
			continue
		}
		records, err := decodeUntypedRecords(segment.Payload)
		if err != nil {
			issues = append(issues, ValidationIssue{Code: "decode_error", Severity: "error", Message: "invalid migration payload", DataClass: segment.DataClass, AdapterKey: segment.AdapterKey})
			continue
		}
		for i, item := range records {
			switch segment.AdapterKey {
			case "config.entries":
				key, _ := item["key"].(string)
				if strings.TrimSpace(key) == "" {
					issues = append(issues, ValidationIssue{Code: "config_key_required", Severity: "error", Message: "configuration key is required", DataClass: segment.DataClass, AdapterKey: segment.AdapterKey, Path: fmt.Sprintf("%d.key", i)})
				}
			case "reference.records":
				typeKey, _ := item["type_key"].(string)
				key, _ := item["key"].(string)
				if strings.TrimSpace(typeKey) == "" || strings.TrimSpace(key) == "" {
					issues = append(issues, ValidationIssue{Code: "reference_identity_required", Severity: "error", Message: "reference type_key and key are required", DataClass: segment.DataClass, AdapterKey: segment.AdapterKey, Path: fmt.Sprintf("%d", i)})
				}
			case "documents.records":
				docType, _ := item["document_type"].(string)
				if strings.TrimSpace(docType) == "" {
					issues = append(issues, ValidationIssue{Code: "document_type_required", Severity: "error", Message: "document_type is required", DataClass: segment.DataClass, AdapterKey: segment.AdapterKey, Path: fmt.Sprintf("%d.document_type", i)})
				}
			}
		}
	}
	return issues
}

func (s *Service) applyInternalSegment(segment ArtifactSegment, actorID string) error {
	switch segment.AdapterKey {
	case "config.entries":
		items, err := decodeRecords[config.Entry](segment.Payload)
		if err != nil {
			return err
		}
		for _, item := range items {
			if actorID != "" {
				item.UpdatedBy = actorID
			}
			if err := s.config.SaveStored(item); err != nil {
				return err
			}
		}
	case "feature_flags.values":
		items, err := decodeRecords[featureflags.Value](segment.Payload)
		if err != nil {
			return err
		}
		for _, item := range items {
			if actorID != "" {
				item.UpdatedBy = actorID
			}
			if err := s.flags.UpsertValue(item); err != nil {
				return err
			}
		}
	case "module.installed":
		items, err := decodeRecords[module.InstalledModule](segment.Payload)
		if err != nil {
			return err
		}
		for _, item := range items {
			if item.Enabled {
				if _, err := s.modules.Enable(item.Key, actorID); err != nil {
					return err
				}
			} else {
				if _, err := s.modules.Disable(item.Key, actorID); err != nil {
					return err
				}
			}
		}
	case "integration.systems":
		items, err := decodeRecords[integration.ExternalSystem](segment.Payload)
		if err != nil {
			return err
		}
		for _, item := range items {
			if err := s.integration.RegisterSystem(item); err != nil && !isAlreadyExists(err) {
				return err
			}
			if len(item.Settings) == 0 {
				continue
			}
			if _, _, err := s.integration.UpdateSystemSettings(item.Key, item.Settings); err != nil {
				if strings.EqualFold(item.Status, "inactive") {
					continue
				}
				return err
			}
		}
	case "integration.endpoints":
		items, err := decodeRecords[integration.Endpoint](segment.Payload)
		if err != nil {
			return err
		}
		for _, item := range items {
			if err := s.integration.RegisterEndpoint(item); err != nil && !isAlreadyExists(err) {
				return err
			}
			if len(item.Settings) == 0 {
				continue
			}
			if _, _, err := s.integration.UpdateEndpointSettings(item.Key, item.Settings); err != nil {
				if strings.EqualFold(item.Status, "inactive") {
					continue
				}
				return err
			}
		}
	case "integration.contracts":
		items, err := decodeRecords[integration.Contract](segment.Payload)
		if err != nil {
			return err
		}
		for _, item := range items {
			if err := s.integration.RegisterContract(item); err != nil && !isAlreadyExists(err) {
				return err
			}
		}
	case "integration.mappings":
		items, err := decodeRecords[integration.Mapping](segment.Payload)
		if err != nil {
			return err
		}
		for _, item := range items {
			if err := s.integration.RegisterMapping(item); err != nil && !isAlreadyExists(err) {
				return err
			}
		}
	case "reference.types":
		items, err := decodeRecords[reference.TypeDefinition](segment.Payload)
		if err != nil {
			return err
		}
		for _, item := range items {
			if err := s.reference.RegisterType(item); err != nil && !isAlreadyExists(err) {
				return err
			}
		}
	case "reference.records":
		items, err := decodeRecords[reference.Record](segment.Payload)
		if err != nil {
			return err
		}
		for _, item := range items {
			if actorID != "" {
				item.UpdatedBy = actorID
			}
			if err := s.reference.UpsertRecord(item); err != nil {
				return err
			}
		}
	case "identity.roles":
		items, err := decodeRecords[identity.Role](segment.Payload)
		if err != nil {
			return err
		}
		for _, item := range items {
			if err := s.identity.UpsertRole(item); err != nil {
				return err
			}
		}
	case "identity.permissions":
		items, err := decodeRecords[identity.Permission](segment.Payload)
		if err != nil {
			return err
		}
		for _, item := range items {
			if err := s.identity.UpsertPermission(item); err != nil {
				return err
			}
		}
	case "identity.role_permissions":
		items, err := decodeRecords[identity.RolePermission](segment.Payload)
		if err != nil {
			return err
		}
		for _, item := range items {
			if err := s.identity.GrantRolePermission(item); err != nil {
				return err
			}
		}
	case "identity.users":
		items, err := decodeRecords[identity.User](segment.Payload)
		if err != nil {
			return err
		}
		for _, item := range items {
			if err := s.identity.UpsertUser(item); err != nil {
				return err
			}
		}
	case "identity.role_bindings":
		items, err := decodeRecords[identity.RoleBinding](segment.Payload)
		if err != nil {
			return err
		}
		for _, item := range items {
			if err := s.identity.UpsertRoleBinding(item); err != nil {
				return err
			}
		}
	case "identity.service_principals":
		items, err := decodeRecords[identity.ServicePrincipal](segment.Payload)
		if err != nil {
			return err
		}
		for _, item := range items {
			if _, err := s.identity.UpsertServicePrincipal(item); err != nil {
				return err
			}
		}
	case "identity.reporting_lines":
		items, err := decodeRecords[identity.ReportingLine](segment.Payload)
		if err != nil {
			return err
		}
		for _, item := range items {
			if _, err := s.identity.UpsertReportingLine(item); err != nil {
				return err
			}
		}
	case "documents.records":
		items, err := decodeRecords[document.Record](segment.Payload)
		if err != nil {
			return err
		}
		for _, item := range items {
			if err := s.documents.Save(item); err != nil {
				return err
			}
		}
	default:
		return shared.Validation("unsupported restore adapter")
	}
	return nil
}

func (s *Service) applyMigrationSegment(segment ArtifactSegment, actorID string) error {
	records, err := decodeUntypedRecords(segment.Payload)
	if err != nil {
		return err
	}
	switch segment.AdapterKey {
	case "config.entries":
		for _, item := range records {
			entry, err := mapToStruct[config.Entry](item)
			if err != nil {
				return err
			}
			if actorID != "" {
				entry.UpdatedBy = actorID
			}
			if err := s.config.Save(entry); err != nil {
				return err
			}
		}
	case "feature_flags.values":
		for _, item := range records {
			value, err := mapToStruct[featureflags.Value](item)
			if err != nil {
				return err
			}
			if actorID != "" {
				value.UpdatedBy = actorID
			}
			if err := s.flags.UpsertValue(value); err != nil {
				return err
			}
		}
	case "reference.types":
		for _, item := range records {
			def, err := mapToStruct[reference.TypeDefinition](item)
			if err != nil {
				return err
			}
			if err := s.reference.RegisterType(def); err != nil && !isAlreadyExists(err) {
				return err
			}
		}
	case "reference.records":
		for _, item := range records {
			record, err := mapToStruct[reference.Record](item)
			if err != nil {
				return err
			}
			if actorID != "" {
				record.UpdatedBy = actorID
			}
			if err := s.reference.UpsertRecord(record); err != nil {
				return err
			}
		}
	case "identity.roles":
		for _, item := range records {
			role, err := mapToStruct[identity.Role](item)
			if err != nil {
				return err
			}
			if err := s.identity.UpsertRole(role); err != nil {
				return err
			}
		}
	case "identity.permissions":
		for _, item := range records {
			permission, err := mapToStruct[identity.Permission](item)
			if err != nil {
				return err
			}
			if err := s.identity.UpsertPermission(permission); err != nil {
				return err
			}
		}
	case "identity.role_permissions":
		for _, item := range records {
			grant, err := mapToStruct[identity.RolePermission](item)
			if err != nil {
				return err
			}
			if err := s.identity.GrantRolePermission(grant); err != nil {
				return err
			}
		}
	case "identity.users":
		for _, item := range records {
			user, err := mapToStruct[identity.User](item)
			if err != nil {
				return err
			}
			if err := s.identity.UpsertUser(user); err != nil {
				return err
			}
		}
	case "identity.role_bindings":
		for _, item := range records {
			binding, err := mapToStruct[identity.RoleBinding](item)
			if err != nil {
				return err
			}
			if err := s.identity.UpsertRoleBinding(binding); err != nil {
				return err
			}
		}
	case "identity.service_principals":
		for _, item := range records {
			principal, err := mapToStruct[identity.ServicePrincipal](item)
			if err != nil {
				return err
			}
			if _, err := s.identity.UpsertServicePrincipal(principal); err != nil {
				return err
			}
		}
	case "identity.reporting_lines":
		for _, item := range records {
			line, err := mapToStruct[identity.ReportingLine](item)
			if err != nil {
				return err
			}
			if _, err := s.identity.UpsertReportingLine(line); err != nil {
				return err
			}
		}
	case "documents.records":
		for _, item := range records {
			documentType, _ := item["document_type"].(string)
			payload, _ := item["payload"].(map[string]any)
			organizationID, _ := item["organization_id"].(string)
			locationID, _ := item["location_id"].(string)
			createActorID, _ := item["actor_id"].(string)
			if strings.TrimSpace(createActorID) == "" {
				createActorID = actorID
			}
			if _, err := s.documents.Create(documentType, organizationID, locationID, createActorID, payload); err != nil {
				return err
			}
		}
	case "integration.systems":
		for _, item := range records {
			system, err := mapToStruct[integration.ExternalSystem](item)
			if err != nil {
				return err
			}
			if err := s.integration.RegisterSystem(system); err != nil && !isAlreadyExists(err) {
				return err
			}
			if len(system.Settings) == 0 {
				continue
			}
			if _, _, err := s.integration.UpdateSystemSettings(system.Key, system.Settings); err != nil {
				if strings.EqualFold(system.Status, "inactive") {
					continue
				}
				return err
			}
		}
	case "integration.endpoints":
		for _, item := range records {
			endpoint, err := mapToStruct[integration.Endpoint](item)
			if err != nil {
				return err
			}
			if err := s.integration.RegisterEndpoint(endpoint); err != nil && !isAlreadyExists(err) {
				return err
			}
			if len(endpoint.Settings) == 0 {
				continue
			}
			if _, _, err := s.integration.UpdateEndpointSettings(endpoint.Key, endpoint.Settings); err != nil {
				if strings.EqualFold(endpoint.Status, "inactive") {
					continue
				}
				return err
			}
		}
	case "integration.contracts":
		for _, item := range records {
			contract, err := mapToStruct[integration.Contract](item)
			if err != nil {
				return err
			}
			if err := s.integration.RegisterContract(contract); err != nil && !isAlreadyExists(err) {
				return err
			}
		}
	case "integration.mappings":
		for _, item := range records {
			mapping, err := mapToStruct[integration.Mapping](item)
			if err != nil {
				return err
			}
			if err := s.integration.RegisterMapping(mapping); err != nil && !isAlreadyExists(err) {
				return err
			}
		}
	default:
		return shared.Validation("unsupported migration adapter")
	}
	return nil
}

func (s *Service) extractInternalSegment(adapterKey string, incremental bool, baseCheckpoint time.Time) (any, time.Time, int, error) {
	switch adapterKey {
	case "config.entries":
		items := s.config.Entries()
		filtered := filterUpdated(items, baseCheckpoint, incremental, func(item config.Entry) time.Time { return item.UpdatedAt })
		return map[string]any{"records": filtered}, maxUpdated(filtered, func(item config.Entry) time.Time { return item.UpdatedAt }), len(filtered), nil
	case "feature_flags.values":
		items := s.flags.Values()
		filtered := filterUpdated(items, baseCheckpoint, incremental, func(item featureflags.Value) time.Time { return item.UpdatedAt })
		return map[string]any{"records": filtered}, maxUpdated(filtered, func(item featureflags.Value) time.Time { return item.UpdatedAt }), len(filtered), nil
	case "module.installed":
		items := make([]module.InstalledModule, 0)
		for _, detail := range s.modules.List() {
			items = append(items, detail.Installed)
		}
		filtered := filterUpdated(items, baseCheckpoint, incremental, func(item module.InstalledModule) time.Time { return item.UpdatedAt })
		return map[string]any{"records": filtered}, maxUpdated(filtered, func(item module.InstalledModule) time.Time { return item.UpdatedAt }), len(filtered), nil
	case "integration.systems":
		items := s.integration.ListSystems()
		filtered := filterUpdated(items, baseCheckpoint, incremental, func(item integration.ExternalSystem) time.Time { return item.UpdatedAt })
		return map[string]any{"records": filtered}, maxUpdated(filtered, func(item integration.ExternalSystem) time.Time { return item.UpdatedAt }), len(filtered), nil
	case "integration.endpoints":
		items := s.integration.ListEndpoints()
		filtered := filterUpdated(items, baseCheckpoint, incremental, func(item integration.Endpoint) time.Time { return item.UpdatedAt })
		return map[string]any{"records": filtered}, maxUpdated(filtered, func(item integration.Endpoint) time.Time { return item.UpdatedAt }), len(filtered), nil
	case "integration.contracts":
		items := s.integration.ListContracts()
		filtered := filterUpdated(items, baseCheckpoint, incremental, func(item integration.Contract) time.Time { return item.UpdatedAt })
		return map[string]any{"records": filtered}, maxUpdated(filtered, func(item integration.Contract) time.Time { return item.UpdatedAt }), len(filtered), nil
	case "integration.mappings":
		items := s.integration.ListMappings()
		filtered := filterUpdated(items, baseCheckpoint, incremental, func(item integration.Mapping) time.Time { return item.UpdatedAt })
		return map[string]any{"records": filtered}, maxUpdated(filtered, func(item integration.Mapping) time.Time { return item.UpdatedAt }), len(filtered), nil
	case "reference.types":
		items := s.reference.Types()
		return map[string]any{"records": items}, time.Time{}, len(items), nil
	case "reference.records":
		items := make([]reference.Record, 0)
		for _, def := range s.reference.Types() {
			items = append(items, s.reference.Records(def.Key)...)
		}
		filtered := filterUpdated(items, baseCheckpoint, incremental, func(item reference.Record) time.Time { return item.UpdatedAt })
		return map[string]any{"records": filtered}, maxUpdated(filtered, func(item reference.Record) time.Time { return item.UpdatedAt }), len(filtered), nil
	case "identity.roles":
		items := s.identity.Roles()
		filtered := filterUpdated(items, baseCheckpoint, incremental, func(item identity.Role) time.Time { return item.UpdatedAt })
		return map[string]any{"records": filtered}, maxUpdated(filtered, func(item identity.Role) time.Time { return item.UpdatedAt }), len(filtered), nil
	case "identity.permissions":
		items := s.identity.Permissions()
		return map[string]any{"records": items}, time.Time{}, len(items), nil
	case "identity.role_permissions":
		items := s.identity.RolePermissions()
		return map[string]any{"records": items}, time.Time{}, len(items), nil
	case "identity.users":
		items := s.identity.Users()
		filtered := filterUpdated(items, baseCheckpoint, incremental, func(item identity.User) time.Time { return item.UpdatedAt })
		return map[string]any{"records": filtered}, maxUpdated(filtered, func(item identity.User) time.Time { return item.UpdatedAt }), len(filtered), nil
	case "identity.role_bindings":
		items := s.identity.Bindings()
		return map[string]any{"records": items}, time.Time{}, len(items), nil
	case "identity.service_principals":
		items := s.identity.ServicePrincipals()
		filtered := filterUpdated(items, baseCheckpoint, incremental, func(item identity.ServicePrincipal) time.Time { return item.UpdatedAt })
		return map[string]any{"records": filtered}, maxUpdated(filtered, func(item identity.ServicePrincipal) time.Time { return item.UpdatedAt }), len(filtered), nil
	case "identity.reporting_lines":
		items := s.identity.ReportingLines()
		filtered := filterUpdated(items, baseCheckpoint, incremental, func(item identity.ReportingLine) time.Time { return item.UpdatedAt })
		return map[string]any{"records": filtered}, maxUpdated(filtered, func(item identity.ReportingLine) time.Time { return item.UpdatedAt }), len(filtered), nil
	case "documents.records":
		items := s.documents.List()
		filtered := filterUpdated(items, baseCheckpoint, incremental, func(item document.Record) time.Time { return item.Header.UpdatedAt })
		return map[string]any{"records": filtered}, maxUpdated(filtered, func(item document.Record) time.Time { return item.Header.UpdatedAt }), len(filtered), nil
	default:
		return nil, time.Time{}, 0, shared.Validation("unsupported backup adapter")
	}
}

func (s *Service) extractExportSegment(adapterKey string) (any, int, error) {
	switch adapterKey {
	case "config.entries":
		items := s.config.Entries()
		return map[string]any{"records": items}, len(items), nil
	default:
		payload, _, count, err := s.extractInternalSegment(adapterKey, false, time.Time{})
		return payload, count, err
	}
}

func restoreOrderForClasses(classes []DataClass) []DataClass {
	order := []DataClass{DataClassConfiguration, DataClassMaster, DataClassTransactional}
	items := make([]DataClass, 0, len(classes))
	for _, candidate := range order {
		if containsDataClass(classes, candidate) {
			items = append(items, candidate)
		}
	}
	return items
}

func summarizeSegments(segments []ArtifactSegment) []SegmentSummary {
	items := make([]SegmentSummary, 0, len(segments))
	for _, item := range segments {
		items = append(items, SegmentSummary{
			ID:               item.ID,
			DataClass:        item.DataClass,
			AdapterKey:       item.AdapterKey,
			Mode:             item.Mode,
			RecordCount:      item.RecordCount,
			Checksum:         item.Checksum,
			BaseArtifactID:   item.BaseArtifactID,
			BaseCheckpointAt: item.BaseCheckpointAt,
			CheckpointAt:     item.CheckpointAt,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].DataClass == items[j].DataClass {
			return items[i].AdapterKey < items[j].AdapterKey
		}
		return items[i].DataClass < items[j].DataClass
	})
	return items
}

func backupRequestMap(req BackupRequest) map[string]any {
	return map[string]any{
		"name":                  req.Name,
		"selected_data_classes": dataClassesToStrings(req.SelectedDataClasses),
		"incremental":           req.Incremental,
		"actor_id":              req.ActorID,
	}
}

func exportRequestMap(req ExportRequest) map[string]any {
	return map[string]any{
		"name":                  req.Name,
		"selected_data_classes": dataClassesToStrings(req.SelectedDataClasses),
		"actor_id":              req.ActorID,
	}
}

func restoreRequestMap(req RestoreRequest) map[string]any {
	return map[string]any{
		"artifact_id":           req.ArtifactID,
		"selected_data_classes": dataClassesToStrings(req.SelectedDataClasses),
		"actor_id":              req.ActorID,
	}
}

func archiveRequestMap(req ArchiveRequest) map[string]any {
	return map[string]any{
		"name":                  req.Name,
		"selected_data_classes": dataClassesToStrings(req.SelectedDataClasses),
		"document_types":        append([]string(nil), req.DocumentTypes...),
		"statuses":              append([]string(nil), req.Statuses...),
		"organization_id":       req.OrganizationID,
		"location_id":           req.LocationID,
		"created_before":        req.CreatedBefore.Format(time.RFC3339),
		"actor_id":              req.ActorID,
	}
}

func backupRequestFromMap(input map[string]any) (BackupRequest, error) {
	return BackupRequest{
		Name:                stringAny(input["name"]),
		SelectedDataClasses: stringsToDataClasses(stringSliceAny(input["selected_data_classes"])),
		Incremental:         boolAny(input["incremental"]),
		ActorID:             stringAny(input["actor_id"]),
	}, nil
}

func exportRequestFromMap(input map[string]any) (ExportRequest, error) {
	return ExportRequest{
		Name:                stringAny(input["name"]),
		SelectedDataClasses: stringsToDataClasses(stringSliceAny(input["selected_data_classes"])),
		ActorID:             stringAny(input["actor_id"]),
	}, nil
}

func restoreRequestFromMap(input map[string]any) (RestoreRequest, error) {
	return RestoreRequest{
		ArtifactID:          stringAny(input["artifact_id"]),
		SelectedDataClasses: stringsToDataClasses(stringSliceAny(input["selected_data_classes"])),
		ActorID:             stringAny(input["actor_id"]),
	}, nil
}

func archiveRequestFromMap(input map[string]any) (ArchiveRequest, error) {
	req := ArchiveRequest{
		Name:                stringAny(input["name"]),
		SelectedDataClasses: stringsToDataClasses(stringSliceAny(input["selected_data_classes"])),
		DocumentTypes:       stringSliceAny(input["document_types"]),
		Statuses:            stringSliceAny(input["statuses"]),
		OrganizationID:      stringAny(input["organization_id"]),
		LocationID:          stringAny(input["location_id"]),
		ActorID:             stringAny(input["actor_id"]),
	}
	if value := stringAny(input["created_before"]); strings.TrimSpace(value) != "" {
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			return ArchiveRequest{}, err
		}
		req.CreatedBefore = parsed
	}
	return req, nil
}

func dataClassesToStrings(items []DataClass) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, string(item))
	}
	return out
}

func stringsToDataClasses(items []string) []DataClass {
	out := make([]DataClass, 0, len(items))
	for _, item := range items {
		out = append(out, DataClass(strings.TrimSpace(item)))
	}
	return out
}

func mergeSummary(left, right map[string]any) map[string]any {
	if len(left) == 0 && len(right) == 0 {
		return nil
	}
	out := cloneMap(left)
	if out == nil {
		out = map[string]any{}
	}
	for key, value := range right {
		out[key] = value
	}
	return out
}

func (s *Service) latestCheckpoint(adapterKey string) (time.Time, string) {
	for _, item := range s.repo.ListCheckpoints() {
		if item.AdapterKey == adapterKey {
			return item.CheckpointAt, item.ArtifactID
		}
	}
	return time.Time{}, ""
}

func (s *Service) latestArtifactForAdapter(adapterKey string) string {
	for _, artifact := range s.repo.ListArtifacts() {
		if artifact.Type != ArtifactTypeBackup {
			continue
		}
		for _, segment := range artifact.Segments {
			if segment.AdapterKey == adapterKey {
				return artifact.ID
			}
		}
	}
	return ""
}

func commonBaseArtifactID(segments []ArtifactSegment) string {
	base := ""
	for _, segment := range segments {
		if strings.TrimSpace(segment.BaseArtifactID) == "" {
			continue
		}
		if base == "" {
			base = segment.BaseArtifactID
			continue
		}
		if base != segment.BaseArtifactID {
			return ""
		}
	}
	return base
}

func (s *Service) resolveRestoreSegments(artifact Artifact, classes []DataClass) ([]ArtifactSegment, []ValidationIssue, error) {
	seenArtifacts := map[string]bool{}
	resolved := make([]ArtifactSegment, 0)
	positionsByAdapter := map[string][]int{}
	issues := make([]ValidationIssue, 0)
	var visit func(Artifact) error
	visit = func(current Artifact) error {
		if seenArtifacts[current.ID] {
			return nil
		}
		seenArtifacts[current.ID] = true
		filtered := filterSegmentsByClasses(current.Segments, classes)
		for _, segment := range filtered {
			if segment.Mode != "incremental" {
				continue
			}
			baseArtifactID := strings.TrimSpace(segment.BaseArtifactID)
			if baseArtifactID == "" {
				issues = append(issues, ValidationIssue{
					Code:       "incremental_base_missing",
					Severity:   "error",
					Message:    fmt.Sprintf("incremental segment %s is missing a base artifact reference", segment.AdapterKey),
					DataClass:  segment.DataClass,
					AdapterKey: segment.AdapterKey,
				})
				continue
			}
			baseArtifact, ok := s.repo.GetArtifact(baseArtifactID)
			if !ok {
				issues = append(issues, ValidationIssue{
					Code:       "incremental_base_not_found",
					Severity:   "error",
					Message:    fmt.Sprintf("base artifact %s for incremental segment %s was not found", baseArtifactID, segment.AdapterKey),
					DataClass:  segment.DataClass,
					AdapterKey: segment.AdapterKey,
				})
				continue
			}
			if err := visit(baseArtifact); err != nil {
				return err
			}
		}
		for _, segment := range filtered {
			if segment.Mode == "full" {
				for _, idx := range positionsByAdapter[segment.AdapterKey] {
					resolved[idx] = ArtifactSegment{}
				}
				positionsByAdapter[segment.AdapterKey] = nil
			}
			resolved = append(resolved, segment)
			positionsByAdapter[segment.AdapterKey] = append(positionsByAdapter[segment.AdapterKey], len(resolved)-1)
		}
		return nil
	}
	if err := visit(artifact); err != nil {
		return nil, nil, err
	}
	items := make([]ArtifactSegment, 0, len(resolved))
	for _, segment := range resolved {
		if strings.TrimSpace(segment.AdapterKey) == "" {
			continue
		}
		items = append(items, segment)
	}
	return orderSegmentsForApply(items), issues, nil
}

func supportsMigrationAdapter(adapterKey string) bool {
	switch adapterKey {
	case "config.entries",
		"feature_flags.values",
		"reference.types",
		"reference.records",
		"identity.roles",
		"identity.permissions",
		"identity.role_permissions",
		"identity.users",
		"identity.role_bindings",
		"identity.service_principals",
		"identity.reporting_lines",
		"documents.records",
		"integration.systems",
		"integration.endpoints",
		"integration.contracts",
		"integration.mappings":
		return true
	default:
		return false
	}
}

func checksum(value any) string {
	payload, _ := json.Marshal(value)
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func reportFromIssues(issues []ValidationIssue) ValidationReport {
	valid := true
	for _, item := range issues {
		if strings.EqualFold(item.Severity, "error") {
			valid = false
			break
		}
	}
	return ValidationReport{Valid: valid, Issues: issues}
}

func normalizeDataClasses(classes []DataClass) ([]DataClass, error) {
	if len(classes) == 0 {
		return nil, shared.Validation("selected_data_classes is required")
	}
	seen := map[DataClass]bool{}
	items := make([]DataClass, 0, len(classes))
	for _, item := range classes {
		switch item {
		case DataClassConfiguration, DataClassMaster, DataClassTransactional:
		default:
			return nil, shared.Validation("selected_data_classes contains an unknown class")
		}
		if !seen[item] {
			seen[item] = true
			items = append(items, item)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i] < items[j] })
	return items, nil
}

func normalizeSelectedOrDefault(selected, defaults []DataClass) ([]DataClass, error) {
	if len(selected) == 0 {
		return normalizeDataClasses(defaults)
	}
	return normalizeDataClasses(selected)
}

func containsDataClass(items []DataClass, candidate DataClass) bool {
	for _, item := range items {
		if item == candidate {
			return true
		}
	}
	return false
}

func containsString(items []string, candidate string) bool {
	for _, item := range items {
		if item == candidate {
			return true
		}
	}
	return false
}

func filterSegmentsByClasses(items []ArtifactSegment, classes []DataClass) []ArtifactSegment {
	filtered := make([]ArtifactSegment, 0)
	for _, item := range items {
		if containsDataClass(classes, item.DataClass) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func orderSegmentsForApply(items []ArtifactSegment) []ArtifactSegment {
	ordered := append([]ArtifactSegment(nil), items...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].DataClass == ordered[j].DataClass {
			return adapterApplyOrder(ordered[i].AdapterKey) < adapterApplyOrder(ordered[j].AdapterKey)
		}
		return ordered[i].DataClass < ordered[j].DataClass
	})
	return ordered
}

func adapterApplyOrder(adapterKey string) int {
	switch adapterKey {
	case "config.entries":
		return 10
	case "feature_flags.values":
		return 20
	case "module.installed":
		return 30
	case "integration.systems":
		return 40
	case "integration.endpoints":
		return 50
	case "integration.contracts":
		return 60
	case "integration.mappings":
		return 70
	case "reference.types":
		return 110
	case "reference.records":
		return 120
	case "identity.roles":
		return 130
	case "identity.permissions":
		return 140
	case "identity.role_permissions":
		return 150
	case "identity.users":
		return 160
	case "identity.role_bindings":
		return 170
	case "identity.service_principals":
		return 180
	case "identity.reporting_lines":
		return 190
	case "documents.records":
		return 210
	default:
		return 1000
	}
}

func maxDocumentUpdatedAt(items []document.Record) time.Time {
	var latest time.Time
	for _, item := range items {
		if item.Header.UpdatedAt.After(latest) {
			latest = item.Header.UpdatedAt
		}
	}
	return latest
}

func firstNonEmpty(values ...string) string {
	for _, item := range values {
		if strings.TrimSpace(item) != "" {
			return strings.TrimSpace(item)
		}
	}
	return ""
}

func decodeRecords[T any](payload any) ([]T, error) {
	switch value := payload.(type) {
	case map[string]any:
		return mapRecordsToStructs[T](value["records"])
	default:
		return nil, shared.Validation("segment payload is invalid")
	}
}

func decodeUntypedRecords(payload any) ([]map[string]any, error) {
	switch value := payload.(type) {
	case map[string]any:
		raw, ok := value["records"]
		if !ok {
			return nil, shared.Validation("segment payload records are required")
		}
		data, err := json.Marshal(raw)
		if err != nil {
			return nil, err
		}
		var items []map[string]any
		if err := json.Unmarshal(data, &items); err != nil {
			return nil, err
		}
		return items, nil
	default:
		return nil, shared.Validation("segment payload is invalid")
	}
}

func mapRecordsToStructs[T any](raw any) ([]T, error) {
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var items []T
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func mapToStruct[T any](raw map[string]any) (T, error) {
	var item T
	data, err := json.Marshal(raw)
	if err != nil {
		return item, err
	}
	if err := json.Unmarshal(data, &item); err != nil {
		return item, err
	}
	return item, nil
}

func maxUpdated[T any](items []T, field func(T) time.Time) time.Time {
	var latest time.Time
	for _, item := range items {
		if field(item).After(latest) {
			latest = field(item)
		}
	}
	return latest
}

func filterUpdated[T any](items []T, since time.Time, incremental bool, field func(T) time.Time) []T {
	if !incremental || since.IsZero() {
		return append([]T(nil), items...)
	}
	filtered := make([]T, 0)
	for _, item := range items {
		if field(item).After(since) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func isAlreadyExists(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "already exists")
}

func cloneMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func stringAny(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func boolAny(value any) bool {
	item, _ := value.(bool)
	return item
}

func stringSliceAny(value any) []string {
	switch items := value.(type) {
	case []string:
		return append([]string(nil), items...)
	case []any:
		out := make([]string, 0, len(items))
		for _, item := range items {
			if text, ok := item.(string); ok {
				out = append(out, strings.TrimSpace(text))
			}
		}
		return out
	default:
		return nil
	}
}
