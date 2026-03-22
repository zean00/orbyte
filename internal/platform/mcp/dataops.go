package mcp

import (
	"fmt"
	"strings"
	"time"

	"orbyte/internal/platform/dataops"
	"orbyte/internal/platform/shared"
)

func (s *Server) dataopsCatalogResource(actor ActorContext) (map[string]any, error) {
	_ = actor
	if s == nil || s.dataops == nil {
		return nil, fmt.Errorf("dataops is unavailable")
	}
	return map[string]any{
		"data_classes": []dataops.DataClass{dataops.DataClassConfiguration, dataops.DataClassMaster, dataops.DataClassTransactional},
		"capabilities": s.dataops.Capabilities(),
	}, nil
}

func (s *Server) dataopsArtifactsResource(actor ActorContext) (map[string]any, error) {
	_ = actor
	if s == nil || s.dataops == nil {
		return nil, fmt.Errorf("dataops is unavailable")
	}
	return map[string]any{"items": s.dataops.Artifacts()}, nil
}

func (s *Server) dataopsCheckpointsResource(actor ActorContext) (map[string]any, error) {
	_ = actor
	if s == nil || s.dataops == nil {
		return nil, fmt.Errorf("dataops is unavailable")
	}
	return map[string]any{"items": s.dataops.Checkpoints()}, nil
}

func (s *Server) dataopsArtifactList(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	_ = arguments
	if s == nil || s.dataops == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	items := s.dataops.Artifacts()
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded %d data operation artifacts.", len(items))}}, "structuredContent": map[string]any{"items": items}}, true, nil
}

func (s *Server) dataopsArtifactGet(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.dataops == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	artifactID := strings.TrimSpace(stringArg(arguments, "artifact_id"))
	if artifactID == "" {
		return nil, true, shared.Validation("artifact_id is required")
	}
	item, ok := s.dataops.Artifact(artifactID)
	if !ok {
		return nil, true, shared.NotFound("artifact not found")
	}
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded artifact %s.", artifactID)}}, "structuredContent": item}, true, nil
}

func (s *Server) dataopsOperationGet(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.dataops == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	operationID := strings.TrimSpace(stringArg(arguments, "operation_id"))
	if operationID == "" {
		return nil, true, shared.Validation("operation_id is required")
	}
	item, ok := s.dataops.Operation(operationID)
	if !ok {
		return nil, true, shared.NotFound("operation not found")
	}
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded data operation run %s.", operationID)}}, "structuredContent": item}, true, nil
}

func (s *Server) dataopsCheckpointList(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	_ = arguments
	if s == nil || s.dataops == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	items := s.dataops.Checkpoints()
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded %d data operation checkpoints.", len(items))}}, "structuredContent": map[string]any{"items": items}}, true, nil
}

func (s *Server) dataopsBackupPlan(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.dataops == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	plan, err := s.dataops.BackupPlan(dataops.BackupRequest{
		Name:                strings.TrimSpace(stringArg(arguments, "name")),
		SelectedDataClasses: dataClassesArg(arguments, "selected_data_classes"),
		Incremental:         boolArg(arguments, "incremental"),
		ActorID:             workflowActorID(actor),
	})
	if err != nil {
		return nil, true, err
	}
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Planned %s backup for %d data classes.", plan.ArtifactType, len(plan.SelectedDataClasses))}}, "structuredContent": plan}, true, nil
}

func (s *Server) dataopsBackupRun(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.dataops == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"configuration.manage"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	if !boolArg(arguments, "confirm_apply") {
		return nil, true, shared.Validation("confirm_apply must be true")
	}
	run, job, err := s.dataops.BackupRun(dataops.BackupRequest{
		Name:                strings.TrimSpace(stringArg(arguments, "name")),
		SelectedDataClasses: dataClassesArg(arguments, "selected_data_classes"),
		Incremental:         boolArg(arguments, "incremental"),
		ActorID:             workflowActorID(actor),
	})
	if err != nil {
		return nil, true, err
	}
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Queued backup run %s.", run.ID)}}, "structuredContent": map[string]any{"run": run, "job": job}}, true, nil
}

func (s *Server) dataopsRestorePlan(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	return s.dataopsRestorePlanLike(actor, arguments, false)
}

func (s *Server) dataopsRestoreValidate(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	return s.dataopsRestorePlanLike(actor, arguments, true)
}

func (s *Server) dataopsRestorePlanLike(actor ActorContext, arguments map[string]any, validate bool) (map[string]any, bool, error) {
	if s == nil || s.dataops == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	plan, err := s.dataops.RestorePlan(dataops.RestoreRequest{
		ArtifactID:          strings.TrimSpace(stringArg(arguments, "artifact_id")),
		SelectedDataClasses: dataClassesArg(arguments, "selected_data_classes"),
		ActorID:             workflowActorID(actor),
	})
	if err != nil {
		return nil, true, err
	}
	text := "Built restore plan."
	if validate {
		text = "Validated restore plan."
	}
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: text}}, "structuredContent": plan}, true, nil
}

func (s *Server) dataopsRestoreRun(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.dataops == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"configuration.manage"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	if !boolArg(arguments, "confirm_apply") {
		return nil, true, shared.Validation("confirm_apply must be true")
	}
	run, job, err := s.dataops.RestoreRun(dataops.RestoreRequest{
		ArtifactID:          strings.TrimSpace(stringArg(arguments, "artifact_id")),
		SelectedDataClasses: dataClassesArg(arguments, "selected_data_classes"),
		ActorID:             workflowActorID(actor),
	})
	if err != nil {
		return nil, true, err
	}
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Queued restore run %s.", run.ID)}}, "structuredContent": map[string]any{"run": run, "job": job}}, true, nil
}

func (s *Server) dataopsArchivePlan(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.dataops == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	plan, err := s.dataops.ArchivePlan(dataops.ArchiveRequest{
		Name:                strings.TrimSpace(stringArg(arguments, "name")),
		SelectedDataClasses: dataClassesArg(arguments, "selected_data_classes"),
		DocumentTypes:       stringSliceArg(arguments, "document_types"),
		Statuses:            stringSliceArg(arguments, "statuses"),
		OrganizationID:      strings.TrimSpace(stringArg(arguments, "organization_id")),
		LocationID:          strings.TrimSpace(stringArg(arguments, "location_id")),
		CreatedBefore:       timeArg(arguments, "created_before"),
		ActorID:             workflowActorID(actor),
	})
	if err != nil {
		return nil, true, err
	}
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: "Planned transactional archive."}}, "structuredContent": plan}, true, nil
}

func (s *Server) dataopsArchiveRun(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.dataops == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"configuration.manage"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	if !boolArg(arguments, "confirm_apply") {
		return nil, true, shared.Validation("confirm_apply must be true")
	}
	run, job, err := s.dataops.ArchiveRun(dataops.ArchiveRequest{
		Name:                strings.TrimSpace(stringArg(arguments, "name")),
		SelectedDataClasses: dataClassesArg(arguments, "selected_data_classes"),
		DocumentTypes:       stringSliceArg(arguments, "document_types"),
		Statuses:            stringSliceArg(arguments, "statuses"),
		OrganizationID:      strings.TrimSpace(stringArg(arguments, "organization_id")),
		LocationID:          strings.TrimSpace(stringArg(arguments, "location_id")),
		CreatedBefore:       timeArg(arguments, "created_before"),
		ActorID:             workflowActorID(actor),
	})
	if err != nil {
		return nil, true, err
	}
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Queued archive run %s.", run.ID)}}, "structuredContent": map[string]any{"run": run, "job": job}}, true, nil
}

func (s *Server) dataopsExportPlan(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.dataops == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	plan, err := s.dataops.ExportPlan(dataops.ExportRequest{
		Name:                strings.TrimSpace(stringArg(arguments, "name")),
		SelectedDataClasses: dataClassesArg(arguments, "selected_data_classes"),
		ActorID:             workflowActorID(actor),
	})
	if err != nil {
		return nil, true, err
	}
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: "Planned external export."}}, "structuredContent": plan}, true, nil
}

func (s *Server) dataopsExportRun(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.dataops == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"configuration.manage"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	if !boolArg(arguments, "confirm_apply") {
		return nil, true, shared.Validation("confirm_apply must be true")
	}
	run, job, err := s.dataops.ExportRun(dataops.ExportRequest{
		Name:                strings.TrimSpace(stringArg(arguments, "name")),
		SelectedDataClasses: dataClassesArg(arguments, "selected_data_classes"),
		ActorID:             workflowActorID(actor),
	})
	if err != nil {
		return nil, true, err
	}
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Queued export run %s.", run.ID)}}, "structuredContent": map[string]any{"run": run, "job": job}}, true, nil
}

func (s *Server) dataopsMigrationRegister(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.dataops == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"configuration.manage"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	segments := make([]dataops.MigrationSegment, 0)
	rawSegments, _ := arguments["segments"].([]any)
	for _, raw := range rawSegments {
		record, ok := raw.(map[string]any)
		if !ok {
			return nil, true, shared.Validation("segments must be objects")
		}
		segment := dataops.MigrationSegment{
			DataClass:  dataops.DataClass(strings.TrimSpace(stringValue(record["data_class"]))),
			AdapterKey: strings.TrimSpace(stringValue(record["adapter_key"])),
		}
		if records, ok := record["records"].([]any); ok {
			segment.Records = append([]any(nil), records...)
		}
		segments = append(segments, segment)
	}
	artifact, err := s.dataops.MigrationRegister(dataops.MigrationRegisterRequest{
		Name:                strings.TrimSpace(stringArg(arguments, "name")),
		SelectedDataClasses: dataClassesArg(arguments, "selected_data_classes"),
		Segments:            segments,
		ActorID:             workflowActorID(actor),
	})
	if err != nil {
		return nil, true, err
	}
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Registered migration input artifact %s.", artifact.ID)}}, "structuredContent": artifact}, true, nil
}

func (s *Server) dataopsMigrationPlan(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	return s.dataopsMigrationPlanLike(actor, arguments, false)
}

func (s *Server) dataopsMigrationValidate(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	return s.dataopsMigrationPlanLike(actor, arguments, true)
}

func (s *Server) dataopsMigrationPlanLike(actor ActorContext, arguments map[string]any, validate bool) (map[string]any, bool, error) {
	if s == nil || s.dataops == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	plan, err := s.dataops.MigrationPlan(dataops.RestoreRequest{
		ArtifactID:          strings.TrimSpace(stringArg(arguments, "artifact_id")),
		SelectedDataClasses: dataClassesArg(arguments, "selected_data_classes"),
		ActorID:             workflowActorID(actor),
	})
	if err != nil {
		return nil, true, err
	}
	text := "Built migration plan."
	if validate {
		text = "Validated migration plan."
	}
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: text}}, "structuredContent": plan}, true, nil
}

func (s *Server) dataopsMigrationRun(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.dataops == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"configuration.manage"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	if !boolArg(arguments, "confirm_apply") {
		return nil, true, shared.Validation("confirm_apply must be true")
	}
	run, job, err := s.dataops.MigrationRun(dataops.RestoreRequest{
		ArtifactID:          strings.TrimSpace(stringArg(arguments, "artifact_id")),
		SelectedDataClasses: dataClassesArg(arguments, "selected_data_classes"),
		ActorID:             workflowActorID(actor),
	})
	if err != nil {
		return nil, true, err
	}
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Queued migration run %s.", run.ID)}}, "structuredContent": map[string]any{"run": run, "job": job}}, true, nil
}

func dataClassesArg(arguments map[string]any, key string) []dataops.DataClass {
	values := stringSliceArg(arguments, key)
	items := make([]dataops.DataClass, 0, len(values))
	for _, value := range values {
		items = append(items, dataops.DataClass(strings.TrimSpace(value)))
	}
	return items
}

func timeArg(arguments map[string]any, key string) time.Time {
	raw := strings.TrimSpace(stringArg(arguments, key))
	if raw == "" {
		return time.Time{}
	}
	value, _ := time.Parse(time.RFC3339, raw)
	return value
}

func stringValue(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}
