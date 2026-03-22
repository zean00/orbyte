package mcp

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"orbyte/internal/platform/config"
	"orbyte/internal/platform/featureflags"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/integration"
	"orbyte/internal/platform/reference"
	"orbyte/internal/platform/shared"
)

func (s *Server) searchIndexList(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	_ = arguments
	if s == nil || s.search == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"search.manage"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded %d search indexes.", len(s.search.IndexDefinitions()))}}, "structuredContent": map[string]any{"items": s.search.IndexDefinitions(), "runtime_items": s.search.IndexRuntimes()}}, true, nil
}

func (s *Server) searchRuntimeGet(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.search == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"search.manage"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	indexKey := strings.TrimSpace(stringArg(arguments, "index_key"))
	if indexKey == "" {
		return nil, true, shared.Validation("index_key is required")
	}
	runtime, ok := s.search.IndexRuntime(indexKey)
	if !ok {
		return nil, true, shared.NotFound("search index not found")
	}
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded runtime state for %s.", indexKey)}}, "structuredContent": runtime}, true, nil
}

func (s *Server) searchConsistencyGet(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.search == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"search.manage"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	indexKey := strings.TrimSpace(stringArg(arguments, "index_key"))
	if indexKey == "" {
		return nil, true, shared.Validation("index_key is required")
	}
	report, err := s.search.ConsistencyReport(indexKey)
	if err != nil {
		return nil, true, err
	}
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded consistency report for %s.", indexKey)}}, "structuredContent": report}, true, nil
}

func (s *Server) searchRebuild(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	return s.executeSearchMutation(actor, arguments, "rebuild")
}

func (s *Server) searchRepair(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	return s.executeSearchMutation(actor, arguments, "repair")
}

func (s *Server) searchReconcile(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	return s.executeSearchMutation(actor, arguments, "reconcile")
}

func (s *Server) executeSearchMutation(actor ActorContext, arguments map[string]any, action string) (map[string]any, bool, error) {
	if s == nil || s.search == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"search.manage"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	if !boolArg(arguments, "confirm_apply") {
		return nil, true, shared.Validation("confirm_apply must be true")
	}
	indexKey := strings.TrimSpace(stringArg(arguments, "index_key"))
	if indexKey == "" {
		return nil, true, shared.Validation("index_key is required")
	}
	var payload any
	var err error
	switch action {
	case "rebuild":
		payload, err = s.search.RebuildIndex(indexKey)
	case "repair":
		payload, err = s.search.RepairIndex(indexKey, strings.TrimSpace(stringArg(arguments, "mode")), strings.TrimSpace(stringArg(arguments, "target_id")))
	case "reconcile":
		payload, err = s.search.ConsistencyReport(indexKey)
	default:
		err = shared.Validation("search action is invalid")
	}
	if err != nil {
		return nil, true, err
	}
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Executed %s for search index %s.", action, indexKey)}}, "structuredContent": map[string]any{"executed": true, "index_key": indexKey, "action": action, "result": payload}}, true, nil
}

func (s *Server) searchSchemaPlan(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.search == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"search.manage"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	indexKey := strings.TrimSpace(stringArg(arguments, "index_key"))
	version := strings.TrimSpace(stringArg(arguments, "version"))
	if indexKey == "" || version == "" {
		return nil, true, shared.Validation("index_key and version are required")
	}
	runtime, err := s.search.PlanIndexSchemaVersion(indexKey, version)
	if err != nil {
		return nil, true, err
	}
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Planned schema version %s for %s.", version, indexKey)}}, "structuredContent": runtime}, true, nil
}

func (s *Server) searchSchemaBuild(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	return s.executeSearchSchemaMutation(actor, arguments, "build")
}

func (s *Server) searchSchemaActivate(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	return s.executeSearchSchemaMutation(actor, arguments, "activate")
}

func (s *Server) executeSearchSchemaMutation(actor ActorContext, arguments map[string]any, action string) (map[string]any, bool, error) {
	if s == nil || s.search == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"search.manage"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	if !boolArg(arguments, "confirm_apply") {
		return nil, true, shared.Validation("confirm_apply must be true")
	}
	indexKey := strings.TrimSpace(stringArg(arguments, "index_key"))
	if indexKey == "" {
		return nil, true, shared.Validation("index_key is required")
	}
	var (
		runtime any
		err     error
	)
	switch action {
	case "build":
		runtime, err = s.search.BuildCandidateIndex(indexKey)
	case "activate":
		runtime, err = s.search.ActivateCandidateIndex(indexKey)
	default:
		err = shared.Validation("search schema action is invalid")
	}
	if err != nil {
		return nil, true, err
	}
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Executed schema %s for %s.", action, indexKey)}}, "structuredContent": runtime}, true, nil
}

func (s *Server) offlineSyncList(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	_ = arguments
	if s == nil || s.offline == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"ops.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: "Loaded offline sync batches and recent outcomes."}}, "structuredContent": map[string]any{"summary": s.offline.SyncSummary(), "batches": s.offline.RecentBatches(50), "recent_items": s.offline.RecentOutcomes(200)}}, true, nil
}

func (s *Server) offlineSyncGet(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.offline == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"ops.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	batchID := strings.TrimSpace(stringArg(arguments, "batch_id"))
	if batchID == "" {
		return nil, true, shared.Validation("batch_id is required")
	}
	for _, batch := range s.offline.RecentBatches(200) {
		if batch.ID == batchID {
			return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded offline sync batch %s.", batchID)}}, "structuredContent": map[string]any{"batch": batch, "items": s.offline.BatchOutcomes(batchID)}}, true, nil
		}
	}
	return nil, true, shared.NotFound("offline sync batch not found")
}

func (s *Server) offlineConflictList(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	_ = arguments
	if s == nil || s.offline == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"ops.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	items := make([]any, 0)
	for _, item := range s.offline.RecentOutcomes(200) {
		if item.Status == "conflict" {
			items = append(items, item)
		}
	}
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded %d offline conflicts.", len(items))}}, "structuredContent": map[string]any{"items": items}}, true, nil
}

func (s *Server) searchRuntimeResource(actor ActorContext) (map[string]any, error) {
	_ = actor
	if s == nil || s.search == nil {
		return nil, fmt.Errorf("search is unavailable")
	}
	return map[string]any{"items": s.search.IndexDefinitions(), "runtime_items": s.search.IndexRuntimes()}, nil
}

func (s *Server) offlineSyncResource(actor ActorContext) (map[string]any, error) {
	_ = actor
	if s == nil || s.offline == nil {
		return nil, fmt.Errorf("offline sync is unavailable")
	}
	return map[string]any{"summary": s.offline.SyncSummary(), "batches": s.offline.RecentBatches(50), "recent_items": s.offline.RecentOutcomes(200)}, nil
}

func (s *Server) policyRuntimeResource(actor ActorContext) (map[string]any, error) {
	_ = actor
	if s == nil || s.policy == nil {
		return nil, fmt.Errorf("policy runtime is unavailable")
	}
	return map[string]any{"items": s.policy.Runtimes("", "")}, nil
}

func (s *Server) referenceCatalogResource(actor ActorContext) (map[string]any, error) {
	_ = actor
	if s == nil || s.reference == nil {
		return nil, fmt.Errorf("reference catalog is unavailable")
	}
	return map[string]any{"types": s.reference.Types()}, nil
}

func (s *Server) implementationBlueprintResource(actor ActorContext) (map[string]any, error) {
	_ = actor
	return map[string]any{
		"desired_operating_model": map[string]any{
			"organization_model":  []string{"organization", "location", "operating_unit"},
			"identity_model":      []string{"roles", "role_bindings", "reporting_lines"},
			"control_plane":       []string{"config", "feature_flags", "modules", "integrations", "policy"},
			"verification_targets": []string{"readiness", "policy_runtime", "integration_health", "search_runtime", "offline_sync"},
		},
		"implementation_steps": []string{"inspect", "stage", "validate", "commit", "verify", "checkpoint", "rollback"},
	}, nil
}

func (s *Server) policyHookList(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	_ = arguments
	if s == nil || s.policy == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	items := s.policy.Runtimes(stringArg(arguments, "organization_id"), stringArg(arguments, "location_id"))
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded %d policy hook runtimes.", len(items))}}, "structuredContent": map[string]any{"items": items}}, true, nil
}

func (s *Server) policyHookGet(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.policy == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	hookKey := strings.TrimSpace(stringArg(arguments, "hook_key"))
	if hookKey == "" {
		return nil, true, shared.Validation("hook_key is required")
	}
	runtime, ok := s.policy.Runtime(hookKey, stringArg(arguments, "organization_id"), stringArg(arguments, "location_id"))
	if !ok {
		return nil, true, shared.NotFound("policy hook not found")
	}
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded policy runtime for %s.", hookKey)}}, "structuredContent": runtime}, true, nil
}

func (s *Server) policyModuleUpsert(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.policy == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"configuration.manage"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	if !boolArg(arguments, "confirm_apply") {
		return nil, true, shared.Validation("confirm_apply must be true")
	}
	hookKey := strings.TrimSpace(stringArg(arguments, "hook_key"))
	source := strings.TrimSpace(stringArg(arguments, "source"))
	if hookKey == "" || source == "" {
		return nil, true, shared.Validation("hook_key and source are required")
	}
	if err := s.policy.UpsertModule(hookKey, strings.TrimSpace(stringArg(arguments, "scope")), strings.TrimSpace(stringArg(arguments, "scope_id")), workflowActorID(actor), source); err != nil {
		return nil, true, err
	}
	runtime, _ := s.policy.Runtime(hookKey, stringArg(arguments, "organization_id"), stringArg(arguments, "location_id"))
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Updated policy module for %s.", hookKey)}}, "structuredContent": map[string]any{"executed": true, "runtime": runtime}}, true, nil
}

func (s *Server) moduleEnable(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	return s.setModuleEnabled(actor, arguments, true)
}

func (s *Server) moduleDisable(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	return s.setModuleEnabled(actor, arguments, false)
}

func (s *Server) setModuleEnabled(actor ActorContext, arguments map[string]any, enabled bool) (map[string]any, bool, error) {
	if s == nil || s.modules == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"module.manage"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	if !boolArg(arguments, "confirm_apply") {
		return nil, true, shared.Validation("confirm_apply must be true")
	}
	moduleKey := strings.TrimSpace(stringArg(arguments, "module_key"))
	if moduleKey == "" {
		return nil, true, shared.Validation("module_key is required")
	}
	var (
		item any
		err  error
	)
	if enabled {
		item, err = s.modules.Enable(moduleKey, workflowActorID(actor))
	} else {
		item, err = s.modules.Disable(moduleKey, workflowActorID(actor))
	}
	if err != nil {
		return nil, true, err
	}
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Set module %s enabled=%t.", moduleKey, enabled)}}, "structuredContent": map[string]any{"executed": true, "module": item}}, true, nil
}

func (s *Server) identityRoleBindingList(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	_ = arguments
	if s == nil || s.identity == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"identity.manage_users"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	items := s.identity.Bindings()
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded %d role bindings.", len(items))}}, "structuredContent": map[string]any{"items": items}}, true, nil
}

func (s *Server) identityRoleBindingPrioritySet(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.identity == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"identity.manage_users"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	if !boolArg(arguments, "confirm_apply") {
		return nil, true, shared.Validation("confirm_apply must be true")
	}
	bindingID := strings.TrimSpace(stringArg(arguments, "binding_id"))
	priority := implementationIntArg(arguments, "priority")
	if bindingID == "" {
		return nil, true, shared.Validation("binding_id is required")
	}
	item, err := s.identity.SetRoleBindingPriority(bindingID, priority)
	if err != nil {
		return nil, true, err
	}
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Updated role binding priority for %s.", bindingID)}}, "structuredContent": map[string]any{"executed": true, "binding": item}}, true, nil
}

func (s *Server) referenceTypeList(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	_ = arguments
	if s == nil || s.reference == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	items := s.reference.Types()
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded %d reference types.", len(items))}}, "structuredContent": map[string]any{"items": items}}, true, nil
}

func (s *Server) referenceRecordList(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.reference == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	typeKey := strings.TrimSpace(stringArg(arguments, "type_key"))
	if typeKey == "" {
		return nil, true, shared.Validation("type_key is required")
	}
	items := s.reference.Records(typeKey)
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded %d reference records for %s.", len(items), typeKey)}}, "structuredContent": map[string]any{"items": items}}, true, nil
}

func (s *Server) referenceResolve(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.reference == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	typeKey := strings.TrimSpace(stringArg(arguments, "type_key"))
	if typeKey == "" {
		return nil, true, shared.Validation("type_key is required")
	}
	result, err := s.reference.Resolve(typeKey, stringArg(arguments, "organization_id"), stringArg(arguments, "location_id"), time.Time{})
	if err != nil {
		return nil, true, err
	}
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Resolved reference set for %s.", typeKey)}}, "structuredContent": result}, true, nil
}

func (s *Server) referenceRecordUpsert(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.reference == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"configuration.manage"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	if !boolArg(arguments, "confirm_apply") {
		return nil, true, shared.Validation("confirm_apply must be true")
	}
	var record reference.Record
	if err := decodeObjectArg(arguments, "record", &record); err != nil {
		return nil, true, err
	}
	record.UpdatedBy = workflowActorID(actor)
	if err := s.reference.UpsertRecord(record); err != nil {
		return nil, true, err
	}
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Upserted reference record %s/%s.", record.TypeKey, record.Key)}}, "structuredContent": map[string]any{"executed": true, "record": record}}, true, nil
}

func (s *Server) implementationSessionCreate(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.implementation == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"configuration.manage"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	session := s.implementation.Create(workflowActorID(actor), strings.TrimSpace(stringArg(arguments, "name")), ImplementationContext{
		OrganizationID:  strings.TrimSpace(stringArg(arguments, "organization_id")),
		LocationID:      strings.TrimSpace(stringArg(arguments, "location_id")),
		OperatingUnitID: strings.TrimSpace(stringArg(arguments, "operating_unit_id")),
	})
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Created implementation session %s.", session.ID)}}, "structuredContent": session}, true, nil
}

func (s *Server) implementationSessionList(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	_ = arguments
	if s == nil || s.implementation == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	items := s.implementation.List()
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded %d implementation sessions.", len(items))}}, "structuredContent": map[string]any{"items": items}}, true, nil
}

func (s *Server) implementationSessionGet(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.implementation == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	session, ok := s.implementation.Get(strings.TrimSpace(stringArg(arguments, "session_id")))
	if !ok {
		return nil, true, shared.NotFound("implementation session not found")
	}
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded implementation session %s.", session.ID)}}, "structuredContent": session}, true, nil
}

func (s *Server) implementationSessionClose(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.implementation == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"configuration.manage"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	if !boolArg(arguments, "confirm_apply") {
		return nil, true, shared.Validation("confirm_apply must be true")
	}
	session, ok := s.implementation.Close(strings.TrimSpace(stringArg(arguments, "session_id")))
	if !ok {
		return nil, true, shared.NotFound("implementation session not found")
	}
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Closed implementation session %s.", session.ID)}}, "structuredContent": session}, true, nil
}

func (s *Server) implementationPlanBuild(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.implementation == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"configuration.manage"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	session, ok := s.implementation.Get(strings.TrimSpace(stringArg(arguments, "session_id")))
	if !ok {
		return nil, true, shared.NotFound("implementation session not found")
	}
	plan, err := decodeImplementationPlan(arguments)
	if err != nil {
		return nil, true, err
	}
	session.StagedPlan = plan
	s.implementation.Save(session)
	diff := s.buildImplementationDiff(session)
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Staged implementation plan for session %s.", session.ID)}}, "structuredContent": map[string]any{"session": session, "diff": diff}}, true, nil
}

func (s *Server) implementationPlanValidate(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.implementation == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"configuration.manage"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	session, ok := s.implementation.Get(strings.TrimSpace(stringArg(arguments, "session_id")))
	if !ok {
		return nil, true, shared.NotFound("implementation session not found")
	}
	report := s.validateImplementationPlan(session)
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Validated implementation session %s: passed=%t.", session.ID, report.Passed)}}, "structuredContent": report}, true, nil
}

func (s *Server) implementationStageDiff(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.implementation == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	session, ok := s.implementation.Get(strings.TrimSpace(stringArg(arguments, "session_id")))
	if !ok {
		return nil, true, shared.NotFound("implementation session not found")
	}
	diff := s.buildImplementationDiff(session)
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Computed staged diff for session %s.", session.ID)}}, "structuredContent": diff}, true, nil
}

func (s *Server) implementationStageDiscard(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.implementation == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"configuration.manage"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	if !boolArg(arguments, "confirm_apply") {
		return nil, true, shared.Validation("confirm_apply must be true")
	}
	session, ok := s.implementation.Get(strings.TrimSpace(stringArg(arguments, "session_id")))
	if !ok {
		return nil, true, shared.NotFound("implementation session not found")
	}
	session.StagedPlan = ImplementationPlanEnvelope{}
	s.implementation.Save(session)
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Discarded staged plan for session %s.", session.ID)}}, "structuredContent": session}, true, nil
}

func (s *Server) implementationStageCommit(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.implementation == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"configuration.manage"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	if !boolArg(arguments, "confirm_apply") {
		return nil, true, shared.Validation("confirm_apply must be true")
	}
	session, ok := s.implementation.Get(strings.TrimSpace(stringArg(arguments, "session_id")))
	if !ok {
		return nil, true, shared.NotFound("implementation session not found")
	}
	report := s.validateImplementationPlan(session)
	if !report.Passed {
		return map[string]any{"content": []ContentBlock{{Type: "text", Text: "Implementation commit blocked by validation issues."}}, "structuredContent": report}, true, nil
	}
	changeSet, err := s.applyImplementationPlan(actor, &session)
	if err != nil {
		return nil, true, err
	}
	s.implementation.Save(session)
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Committed change-set %s for session %s.", changeSet.ID, session.ID)}}, "structuredContent": map[string]any{"session": session, "change_set": changeSet}}, true, nil
}

func (s *Server) implementationCheckpointCreate(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.implementation == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"configuration.manage"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	session, ok := s.implementation.Get(strings.TrimSpace(stringArg(arguments, "session_id")))
	if !ok {
		return nil, true, shared.NotFound("implementation session not found")
	}
	item := ImplementationCheckpoint{
		ID:        fmt.Sprintf("checkpoint:%d", time.Now().UTC().UnixNano()),
		Name:      firstNonEmpty(strings.TrimSpace(stringArg(arguments, "name")), "checkpoint"),
		CreatedAt: time.Now().UTC(),
		CreatedBy: workflowActorID(actor),
	}
	if len(session.ChangeSets) > 0 {
		item.ChangeSetID = session.ChangeSets[len(session.ChangeSets)-1].ID
	}
	session.Checkpoints = append(session.Checkpoints, item)
	s.implementation.Save(session)
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Created checkpoint %s.", item.ID)}}, "structuredContent": item}, true, nil
}

func (s *Server) implementationCheckpointList(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.implementation == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	session, ok := s.implementation.Get(strings.TrimSpace(stringArg(arguments, "session_id")))
	if !ok {
		return nil, true, shared.NotFound("implementation session not found")
	}
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded %d checkpoints.", len(session.Checkpoints))}}, "structuredContent": map[string]any{"items": session.Checkpoints}}, true, nil
}

func (s *Server) implementationCheckpointRestore(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.implementation == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"configuration.manage"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	if !boolArg(arguments, "confirm_apply") {
		return nil, true, shared.Validation("confirm_apply must be true")
	}
	session, ok := s.implementation.Get(strings.TrimSpace(stringArg(arguments, "session_id")))
	if !ok {
		return nil, true, shared.NotFound("implementation session not found")
	}
	checkpointID := strings.TrimSpace(stringArg(arguments, "checkpoint_id"))
	for _, checkpoint := range session.Checkpoints {
		if checkpoint.ID != checkpointID {
			continue
		}
		if checkpoint.ChangeSetID == "" {
			return map[string]any{"content": []ContentBlock{{Type: "text", Text: "Checkpoint has no committed change-set to restore."}}, "structuredContent": map[string]any{"executed": false, "checkpoint": checkpoint}}, true, nil
		}
		plan := s.rollbackPlanForChangeSet(session, checkpoint.ChangeSetID)
		if !plan.Reversible {
			return map[string]any{"content": []ContentBlock{{Type: "text", Text: "Checkpoint restore requires manual remediation."}}, "structuredContent": plan}, true, nil
		}
		if err := s.applyRollbackPlan(actor, &session, plan); err != nil {
			return nil, true, err
		}
		s.implementation.Save(session)
		return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Restored checkpoint %s.", checkpoint.ID)}}, "structuredContent": plan}, true, nil
	}
	return nil, true, shared.NotFound("implementation checkpoint not found")
}

func (s *Server) implementationVerifyState(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	report, handled, err := s.verifyImplementation(actor, arguments, false)
	if !handled || err != nil {
		return report, handled, err
	}
	return report, true, nil
}

func (s *Server) implementationVerifyReadiness(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	report, handled, err := s.verifyImplementation(actor, arguments, true)
	if !handled || err != nil {
		return report, handled, err
	}
	return report, true, nil
}

func (s *Server) implementationVerifyDiff(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	return s.implementationStageDiff(actor, arguments)
}

func (s *Server) implementationVerifySmoke(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	report, handled, err := s.verifyImplementation(actor, arguments, true)
	if !handled || err != nil {
		return report, handled, err
	}
	structured := report["structuredContent"].(ImplementationVerificationReport)
	structured.Warnings = append(structured.Warnings, "smoke verification checks readiness, policy runtime, integration health, search runtime, and offline sync summaries")
	return map[string]any{"content": report["content"], "structuredContent": structured}, true, nil
}

func (s *Server) verifyImplementation(actor ActorContext, arguments map[string]any, includeReadiness bool) (map[string]any, bool, error) {
	if s == nil || s.implementation == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	var ctx ImplementationContext
	if sessionID := strings.TrimSpace(stringArg(arguments, "session_id")); sessionID != "" {
		session, ok := s.implementation.Get(sessionID)
		if !ok {
			return nil, true, shared.NotFound("implementation session not found")
		}
		ctx = session.Context
	} else {
		ctx = ImplementationContext{OrganizationID: stringArg(arguments, "organization_id"), LocationID: stringArg(arguments, "location_id"), OperatingUnitID: stringArg(arguments, "operating_unit_id")}
	}
	report := s.buildVerificationReport(ctx, includeReadiness)
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Verification passed=%t.", report.Passed)}}, "structuredContent": report}, true, nil
}

func (s *Server) implementationRollbackPlan(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.implementation == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	session, ok := s.implementation.Get(strings.TrimSpace(stringArg(arguments, "session_id")))
	if !ok {
		return nil, true, shared.NotFound("implementation session not found")
	}
	changeSetID := strings.TrimSpace(stringArg(arguments, "change_set_id"))
	if changeSetID == "" && len(session.ChangeSets) > 0 {
		changeSetID = session.ChangeSets[len(session.ChangeSets)-1].ID
	}
	plan := s.rollbackPlanForChangeSet(session, changeSetID)
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Built rollback plan for %s.", changeSetID)}}, "structuredContent": plan}, true, nil
}

func (s *Server) implementationRollbackApply(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.implementation == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"configuration.manage"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	if !boolArg(arguments, "confirm_apply") {
		return nil, true, shared.Validation("confirm_apply must be true")
	}
	session, ok := s.implementation.Get(strings.TrimSpace(stringArg(arguments, "session_id")))
	if !ok {
		return nil, true, shared.NotFound("implementation session not found")
	}
	changeSetID := strings.TrimSpace(stringArg(arguments, "change_set_id"))
	if changeSetID == "" && len(session.ChangeSets) > 0 {
		changeSetID = session.ChangeSets[len(session.ChangeSets)-1].ID
	}
	plan := s.rollbackPlanForChangeSet(session, changeSetID)
	if !plan.Reversible {
		return map[string]any{"content": []ContentBlock{{Type: "text", Text: "Rollback requires manual remediation."}}, "structuredContent": plan}, true, nil
	}
	if err := s.applyRollbackPlan(actor, &session, plan); err != nil {
		return nil, true, err
	}
	s.implementation.Save(session)
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Applied rollback plan for %s.", changeSetID)}}, "structuredContent": plan}, true, nil
}

func decodeImplementationPlan(arguments map[string]any) (ImplementationPlanEnvelope, error) {
	var plan ImplementationPlanEnvelope
	if err := decodeOptionalObjectArg(arguments, "plan", &plan); err != nil {
		return ImplementationPlanEnvelope{}, err
	}
	if len(plan.Bundle.ConfigEntries) == 0 && len(plan.Bundle.FeatureFlags) == 0 {
		var bundle configBundle
		if err := decodeOptionalObjectArg(arguments, "bundle", &bundle); err != nil {
			return ImplementationPlanEnvelope{}, err
		}
		plan.Bundle = bundle
	}
	if len(plan.RoleGrants) == 0 {
		plan.RoleGrants = decodeRoleGrants(arguments)
	}
	return plan, nil
}

func (s *Server) validateImplementationPlan(session ImplementationSession) ImplementationVerificationReport {
	report := ImplementationVerificationReport{Passed: true, Context: session.Context, GeneratedAt: time.Now().UTC()}
	if s.config != nil && s.flags != nil && s.modules != nil {
		validation := validateConfigBundle(s.config, s.flags, s.modules, session.StagedPlan.Bundle)
		report.Checks = append(report.Checks, map[string]any{"name": "config_bundle", "passed": validation.Valid, "issues": validation.Issues, "warnings": validation.Warnings})
		if !validation.Valid {
			report.Passed = false
		}
	}
	for _, item := range session.StagedPlan.ModuleActions {
		detail, ok := s.modules.Get(item.ModuleKey)
		passed := ok
		message := "module found"
		if !ok {
			message = "module not found"
		} else if err := s.modules.ValidateSetEnabled(item.ModuleKey, item.Enabled); err != nil {
			passed = false
			message = err.Error()
		} else if item.Enabled && detail.LifecycleState == "blocked" {
			passed = false
			message = "module has blocking compatibility diagnostics"
		}
		report.Checks = append(report.Checks, map[string]any{"name": "module_action", "module_key": item.ModuleKey, "passed": passed, "message": message})
		if !passed {
			report.Passed = false
		}
	}
	for _, grant := range session.StagedPlan.RoleGrants {
		err := validateRoleGrant(s.identity, grant)
		passed := err == nil
		report.Checks = append(report.Checks, map[string]any{"name": "role_permission", "role_id": grant.RoleID, "permission_key": grant.PermissionKey, "passed": passed, "message": errorText(err)})
		if !passed {
			report.Passed = false
		}
	}
	for _, update := range session.StagedPlan.SystemConfigUpdates {
		if s.integration == nil {
			report.Checks = append(report.Checks, map[string]any{"name": "integration_system", "system_key": update.Key, "passed": false, "message": "integration service is not configured"})
			report.Passed = false
			continue
		}
		_, err := s.integration.ValidateSystemSettings(update.Key, update.Settings)
		passed := err == nil
		report.Checks = append(report.Checks, map[string]any{"name": "integration_system", "system_key": update.Key, "passed": passed, "message": errorText(err)})
		if !passed {
			report.Passed = false
		}
	}
	for _, update := range session.StagedPlan.EndpointConfigUpdates {
		if s.integration == nil {
			report.Checks = append(report.Checks, map[string]any{"name": "integration_endpoint", "endpoint_key": update.Key, "passed": false, "message": "integration service is not configured"})
			report.Passed = false
			continue
		}
		_, err := s.integration.ValidateEndpointSettings(update.Key, update.Settings)
		passed := err == nil
		report.Checks = append(report.Checks, map[string]any{"name": "integration_endpoint", "endpoint_key": update.Key, "passed": passed, "message": errorText(err)})
		if !passed {
			report.Passed = false
		}
	}
	for _, item := range session.StagedPlan.ReferenceRecordUpserts {
		err := validateReferenceUpsert(s.reference, item)
		passed := err == nil
		report.Checks = append(report.Checks, map[string]any{"name": "reference_record", "type_key": item.TypeKey, "key": item.Key, "passed": passed, "message": errorText(err)})
		if !passed {
			report.Passed = false
		}
	}
	for _, item := range session.StagedPlan.PolicyModuleUpdates {
		if s.policy == nil {
			report.Checks = append(report.Checks, map[string]any{"name": "policy_module", "hook_key": item.HookKey, "passed": false, "message": "policy service is not configured"})
			report.Passed = false
			continue
		}
		runtime, ok := s.policy.Runtime(item.HookKey, session.Context.OrganizationID, session.Context.LocationID)
		if !ok {
			report.Checks = append(report.Checks, map[string]any{"name": "policy_module", "hook_key": item.HookKey, "passed": false, "message": "policy hook not found"})
			report.Passed = false
			continue
		}
		err := s.policy.ValidateModuleRuntimeSource(item.HookKey, item.Source)
		report.Checks = append(report.Checks, map[string]any{"name": "policy_module", "hook_key": item.HookKey, "passed": err == nil, "current_runtime": runtime, "message": errorText(err)})
		if err != nil {
			report.Passed = false
		}
	}
	return report
}

func (s *Server) buildImplementationDiff(session ImplementationSession) map[string]any {
	items := make([]map[string]any, 0)
	for _, entry := range session.StagedPlan.Bundle.ConfigEntries {
		before, found := findConfigEntry(s.config, entry.Key, entry.Scope, entry.ScopeID)
		items = append(items, map[string]any{"kind": "config_entry", "key": entry.Key, "scope": entry.Scope, "scope_id": entry.ScopeID, "found": found, "before": before, "after": entry})
	}
	for _, value := range session.StagedPlan.Bundle.FeatureFlags {
		before, found := findFeatureFlagValue(s.flags, value.FlagKey, value.Scope, value.ScopeID)
		items = append(items, map[string]any{"kind": "feature_flag", "key": value.FlagKey, "scope": value.Scope, "scope_id": value.ScopeID, "found": found, "before": before, "after": value})
	}
	for _, grant := range session.StagedPlan.RoleGrants {
		found := roleGrantExists(s.identity, grant.RoleID, grant.PermissionKey)
		items = append(items, map[string]any{"kind": "role_permission", "role_id": grant.RoleID, "permission_key": grant.PermissionKey, "already_granted": found})
	}
	for _, action := range session.StagedPlan.ModuleActions {
		before, _ := s.modules.Get(action.ModuleKey)
		items = append(items, map[string]any{"kind": "module", "module_key": action.ModuleKey, "before_enabled": before.Installed.Enabled, "after_enabled": action.Enabled})
	}
	for _, update := range session.StagedPlan.SystemConfigUpdates {
		items = append(items, map[string]any{"kind": "integration_system", "key": update.Key, "before": findSystemSettings(s.integration, update.Key), "after": update.Settings})
	}
	for _, update := range session.StagedPlan.EndpointConfigUpdates {
		items = append(items, map[string]any{"kind": "integration_endpoint", "key": update.Key, "before": findEndpointSettings(s.integration, update.Key), "after": update.Settings})
	}
	for _, item := range session.StagedPlan.ReferenceRecordUpserts {
		before, found := findReferenceRecord(s.reference, item.TypeKey, item.Key, item.Scope, item.ScopeID)
		items = append(items, map[string]any{"kind": "reference_record", "type_key": item.TypeKey, "key": item.Key, "found": found, "before": before, "after": item})
	}
	return map[string]any{"session_id": session.ID, "items": items}
}

func (s *Server) applyImplementationPlan(actor ActorContext, session *ImplementationSession) (ImplementationChangeSet, error) {
	report := s.validateImplementationPlan(*session)
	if !report.Passed {
		return ImplementationChangeSet{}, shared.Validation("implementation plan failed validation")
	}
	changeSet := ImplementationChangeSet{
		ID:        fmt.Sprintf("changeset:%d", time.Now().UTC().UnixNano()),
		SessionID: session.ID,
		Status:    "applied",
		CreatedAt: time.Now().UTC(),
		AppliedAt: time.Now().UTC(),
		AppliedBy: workflowActorID(actor),
	}
	for _, entry := range session.StagedPlan.Bundle.ConfigEntries {
		before, found := findConfigEntry(s.config, entry.Key, entry.Scope, entry.ScopeID)
		entry.UpdatedBy = workflowActorID(actor)
		if err := s.config.Save(entry); err != nil {
			return ImplementationChangeSet{}, err
		}
		changeSet.Operations = append(changeSet.Operations, ImplementationOperation{Kind: "config_entry", Action: "save", TargetKey: entry.Key, Scope: entry.Scope, ScopeID: entry.ScopeID, Before: entryMap(before, found), After: configEntryMap(entry), Reversible: found})
	}
	for _, value := range session.StagedPlan.Bundle.FeatureFlags {
		before, found := findFeatureFlagValue(s.flags, value.FlagKey, value.Scope, value.ScopeID)
		value.UpdatedBy = workflowActorID(actor)
		if err := s.flags.UpsertValue(value); err != nil {
			return ImplementationChangeSet{}, err
		}
		changeSet.Operations = append(changeSet.Operations, ImplementationOperation{Kind: "feature_flag", Action: "upsert", TargetKey: value.FlagKey, Scope: value.Scope, ScopeID: value.ScopeID, Before: featureFlagMap(before, found), After: featureFlagValueMap(value), Reversible: found})
	}
	for _, grant := range session.StagedPlan.RoleGrants {
		found := roleGrantExists(s.identity, grant.RoleID, grant.PermissionKey)
		if err := s.identity.GrantRolePermission(identity.RolePermission{RoleID: grant.RoleID, PermissionKey: grant.PermissionKey}); err != nil {
			return ImplementationChangeSet{}, err
		}
		changeSet.Operations = append(changeSet.Operations, ImplementationOperation{Kind: "role_permission", Action: "grant", TargetKey: grant.RoleID + ":" + grant.PermissionKey, Before: map[string]any{"granted": found}, After: map[string]any{"granted": true}, Reversible: true})
	}
	for _, action := range session.StagedPlan.ModuleActions {
		before, _ := s.modules.Get(action.ModuleKey)
		if action.Enabled {
			if _, err := s.modules.Enable(action.ModuleKey, workflowActorID(actor)); err != nil {
				return ImplementationChangeSet{}, err
			}
		} else {
			if _, err := s.modules.Disable(action.ModuleKey, workflowActorID(actor)); err != nil {
				return ImplementationChangeSet{}, err
			}
		}
		changeSet.Operations = append(changeSet.Operations, ImplementationOperation{Kind: "module", Action: "toggle", TargetKey: action.ModuleKey, Before: map[string]any{"enabled": before.Installed.Enabled}, After: map[string]any{"enabled": action.Enabled}, Reversible: true})
	}
	for _, update := range session.StagedPlan.SystemConfigUpdates {
		before := findSystemSettings(s.integration, update.Key)
		if _, _, err := s.integration.UpdateSystemSettings(update.Key, update.Settings); err != nil {
			return ImplementationChangeSet{}, err
		}
		changeSet.Operations = append(changeSet.Operations, ImplementationOperation{Kind: "integration_system", Action: "update", TargetKey: update.Key, Before: before, After: cloneMap(update.Settings), Reversible: len(before) > 0})
	}
	for _, update := range session.StagedPlan.EndpointConfigUpdates {
		before := findEndpointSettings(s.integration, update.Key)
		if _, _, err := s.integration.UpdateEndpointSettings(update.Key, update.Settings); err != nil {
			return ImplementationChangeSet{}, err
		}
		changeSet.Operations = append(changeSet.Operations, ImplementationOperation{Kind: "integration_endpoint", Action: "update", TargetKey: update.Key, Before: before, After: cloneMap(update.Settings), Reversible: len(before) > 0})
	}
	for _, item := range session.StagedPlan.ReferenceRecordUpserts {
		before, found := findReferenceRecord(s.reference, item.TypeKey, item.Key, item.Scope, item.ScopeID)
		record := reference.Record{TypeKey: item.TypeKey, Key: item.Key, DisplayName: item.DisplayName, Scope: item.Scope, ScopeID: item.ScopeID, Status: firstNonEmpty(item.Status, "active"), Value: cloneMap(item.Value), UpdatedBy: workflowActorID(actor)}
		if err := s.reference.UpsertRecord(record); err != nil {
			return ImplementationChangeSet{}, err
		}
		changeSet.Operations = append(changeSet.Operations, ImplementationOperation{Kind: "reference_record", Action: "upsert", TargetKey: item.TypeKey + ":" + item.Key, Scope: item.Scope, ScopeID: item.ScopeID, Before: referenceRecordMap(before, found), After: referenceRecordMap(record, true), Reversible: found})
	}
	for _, item := range session.StagedPlan.PolicyModuleUpdates {
		before, found := findPolicyModuleEntry(s.config, item.HookKey, item.Scope, item.ScopeID)
		if err := s.policy.UpsertModule(item.HookKey, item.Scope, item.ScopeID, workflowActorID(actor), item.Source); err != nil {
			return ImplementationChangeSet{}, err
		}
		changeSet.Operations = append(changeSet.Operations, ImplementationOperation{Kind: "policy_module", Action: "upsert", TargetKey: item.HookKey, Scope: item.Scope, ScopeID: item.ScopeID, Before: before, After: map[string]any{"source": item.Source}, Reversible: found})
	}
	session.ChangeSets = append(session.ChangeSets, changeSet)
	session.LastCommitAt = changeSet.AppliedAt
	session.StagedPlan = ImplementationPlanEnvelope{}
	return changeSet, nil
}

func (s *Server) buildVerificationReport(ctx ImplementationContext, includeReadiness bool) ImplementationVerificationReport {
	report := ImplementationVerificationReport{Passed: true, Context: ctx, GeneratedAt: time.Now().UTC()}
	if includeReadiness && s.config != nil && s.modules != nil {
		readiness := buildAdminReadinessReport(s.config, s.modules, s.health)
		report.Checks = append(report.Checks, map[string]any{"name": "readiness", "passed": !readiness.BlockedForApply, "status": readiness.Status, "details": readiness})
		if readiness.BlockedForApply {
			report.Passed = false
		}
	}
	if s.policy != nil {
		for _, runtime := range s.policy.Runtimes(ctx.OrganizationID, ctx.LocationID) {
			passed := runtime.CompileValid && runtime.EvalValid
			report.Checks = append(report.Checks, map[string]any{"name": "policy_runtime", "hook_key": runtime.Definition.Key, "passed": passed, "runtime": runtime})
			if !passed {
				report.Passed = false
			}
		}
	}
	if s.integration != nil {
		health := s.integration.HealthSummary()
		passed := true
		for _, item := range health {
			if item.Status == "failed" {
				passed = false
				break
			}
		}
		report.Checks = append(report.Checks, map[string]any{"name": "integration_health", "passed": passed, "summary": health})
		if !passed {
			report.Passed = false
		}
	}
	if s.search != nil {
		unhealthy := 0
		for _, runtime := range s.search.IndexRuntimes() {
			passed := runtime.RuntimeStatus != "failed" && runtime.ConsistencyStatus != "rebuild_required"
			report.Checks = append(report.Checks, map[string]any{"name": "search_runtime", "index_key": runtime.IndexKey, "passed": passed, "runtime": runtime})
			if !passed {
				unhealthy++
			}
		}
		if unhealthy > 0 {
			report.Passed = false
		}
	}
	if s.offline != nil {
		summary := s.offline.SyncSummary()
		report.Checks = append(report.Checks, map[string]any{"name": "offline_sync", "passed": true, "summary": summary})
	}
	return report
}

func (s *Server) rollbackPlanForChangeSet(session ImplementationSession, changeSetID string) ImplementationRollbackPlan {
	plan := ImplementationRollbackPlan{SessionID: session.ID, ChangeSetID: changeSetID, Reversible: true}
	for _, changeSet := range session.ChangeSets {
		if changeSet.ID != changeSetID {
			continue
		}
		for i := len(changeSet.Operations) - 1; i >= 0; i-- {
			item := changeSet.Operations[i]
			if !item.Reversible {
				plan.Reversible = false
				plan.ManualRemediation = append(plan.ManualRemediation, "manual remediation required for "+item.Kind+" "+item.TargetKey)
				continue
			}
			plan.Operations = append(plan.Operations, item)
		}
	}
	if len(plan.Operations) == 0 && len(plan.ManualRemediation) == 0 {
		plan.Reversible = false
		plan.ManualRemediation = append(plan.ManualRemediation, "change-set not found or has no reversible operations")
	}
	return plan
}

func (s *Server) applyRollbackPlan(actor ActorContext, session *ImplementationSession, plan ImplementationRollbackPlan) error {
	for _, item := range plan.Operations {
		switch item.Kind {
		case "config_entry":
			if len(item.Before) == 0 {
				continue
			}
			entry := config.Entry{}
			body, _ := json.Marshal(item.Before)
			_ = json.Unmarshal(body, &entry)
			entry.UpdatedBy = workflowActorID(actor)
			if err := s.config.Save(entry); err != nil {
				return err
			}
		case "feature_flag":
			if len(item.Before) == 0 {
				continue
			}
			value := featureflags.Value{}
			body, _ := json.Marshal(item.Before)
			_ = json.Unmarshal(body, &value)
			value.UpdatedBy = workflowActorID(actor)
			if err := s.flags.UpsertValue(value); err != nil {
				return err
			}
		case "role_permission":
			parts := strings.SplitN(item.TargetKey, ":", 2)
			if len(parts) != 2 {
				continue
			}
			if granted, _ := item.Before["granted"].(bool); granted {
				if err := s.identity.GrantRolePermission(identity.RolePermission{RoleID: parts[0], PermissionKey: parts[1]}); err != nil {
					return err
				}
			} else if err := s.identity.RevokeRolePermission(parts[0], parts[1]); err != nil {
				return err
			}
		case "module":
			if enabled, _ := item.Before["enabled"].(bool); enabled {
				if _, err := s.modules.Enable(item.TargetKey, workflowActorID(actor)); err != nil {
					return err
				}
			} else if _, err := s.modules.Disable(item.TargetKey, workflowActorID(actor)); err != nil {
				return err
			}
		case "integration_system":
			if _, _, err := s.integration.UpdateSystemSettings(item.TargetKey, cloneMap(item.Before)); err != nil {
				return err
			}
		case "integration_endpoint":
			if _, _, err := s.integration.UpdateEndpointSettings(item.TargetKey, cloneMap(item.Before)); err != nil {
				return err
			}
		case "reference_record":
			if len(item.Before) == 0 {
				continue
			}
			record := reference.Record{}
			body, _ := json.Marshal(item.Before)
			_ = json.Unmarshal(body, &record)
			record.UpdatedBy = workflowActorID(actor)
			if err := s.reference.UpsertRecord(record); err != nil {
				return err
			}
		case "policy_module":
			if source, _ := item.Before["source"].(string); source != "" {
				if err := s.policy.UpsertModule(item.TargetKey, item.Scope, item.ScopeID, workflowActorID(actor), source); err != nil {
					return err
				}
			}
		}
	}
	session.Checkpoints = append(session.Checkpoints, ImplementationCheckpoint{
		ID:          fmt.Sprintf("checkpoint:%d", time.Now().UTC().UnixNano()),
		ChangeSetID: plan.ChangeSetID,
		Name:        "rollback",
		CreatedAt:   time.Now().UTC(),
		CreatedBy:   workflowActorID(actor),
	})
	return nil
}

func findConfigEntry(cfg *config.Service, key, scope, scopeID string) (config.Entry, bool) {
	if cfg == nil {
		return config.Entry{}, false
	}
	for _, item := range cfg.Entries() {
		if item.Key == key && item.Scope == scope && item.ScopeID == scopeID {
			return item, true
		}
	}
	return config.Entry{}, false
}

func findFeatureFlagValue(flags *featureflags.Service, key, scope, scopeID string) (featureflags.Value, bool) {
	if flags == nil {
		return featureflags.Value{}, false
	}
	for _, item := range flags.Values() {
		if item.FlagKey == key && item.Scope == scope && item.ScopeID == scopeID {
			return item, true
		}
	}
	return featureflags.Value{}, false
}

func roleGrantExists(ident *identity.Service, roleID, permissionKey string) bool {
	if ident == nil {
		return false
	}
	for _, item := range ident.RolePermissions() {
		if item.RoleID == roleID && item.PermissionKey == permissionKey {
			return true
		}
	}
	return false
}

func findSystemSettings(svc *integration.Service, key string) map[string]any {
	if svc == nil {
		return nil
	}
	for _, item := range svc.ListSystems() {
		if item.Key == key {
			return cloneMap(item.Settings)
		}
	}
	return nil
}

func findEndpointSettings(svc *integration.Service, key string) map[string]any {
	if svc == nil {
		return nil
	}
	for _, item := range svc.ListEndpoints() {
		if item.Key == key {
			return cloneMap(item.Settings)
		}
	}
	return nil
}

func findReferenceRecord(svc *reference.Service, typeKey, key, scope, scopeID string) (reference.Record, bool) {
	if svc == nil {
		return reference.Record{}, false
	}
	for _, item := range svc.Records(typeKey) {
		if item.Key == key && item.Scope == scope && item.ScopeID == scopeID {
			return item, true
		}
	}
	return reference.Record{}, false
}

func findPolicyModuleEntry(cfg *config.Service, hookKey, scope, scopeID string) (map[string]any, bool) {
	entry, ok := findConfigEntry(cfg, "policy.rego."+hookKey, scope, scopeID)
	if !ok {
		return nil, false
	}
	return cloneMap(entry.Value), true
}

func validateReferenceUpsert(svc *reference.Service, item implementationReferenceUpsert) error {
	if svc == nil {
		return shared.Validation("reference service is not configured")
	}
	record := reference.Record{
		TypeKey:     item.TypeKey,
		Key:         item.Key,
		DisplayName: item.DisplayName,
		Scope:       item.Scope,
		ScopeID:     item.ScopeID,
		Status:      firstNonEmpty(item.Status, "active"),
		Value:       cloneMap(item.Value),
	}
	if strings.TrimSpace(record.TypeKey) == "" || strings.TrimSpace(record.Key) == "" || strings.TrimSpace(record.DisplayName) == "" {
		return shared.Validation("reference record type_key, key, and display_name are required")
	}
	def, ok := svc.Type(record.TypeKey)
	if !ok {
		return shared.NotFound("reference type not found")
	}
	scope := strings.TrimSpace(record.Scope)
	if scope == "" {
		scope = "deployment"
	}
	allowed := false
	for _, item := range def.AllowedScopes {
		if strings.TrimSpace(item) == scope {
			allowed = true
			break
		}
	}
	if !allowed {
		return shared.Validation("reference record scope is not allowed")
	}
	return nil
}

func validateRoleGrant(ident *identity.Service, grant implementationRoleGrant) error {
	if ident == nil {
		return shared.Validation("identity service is not configured")
	}
	if strings.TrimSpace(grant.RoleID) == "" || strings.TrimSpace(grant.PermissionKey) == "" {
		return shared.Validation("role_id and permission_key are required")
	}
	roleFound := false
	for _, item := range ident.Roles() {
		if item.ID == grant.RoleID {
			roleFound = true
			break
		}
	}
	if !roleFound {
		return shared.NotFound("role not found")
	}
	permissionFound := false
	for _, item := range ident.Permissions() {
		if item.Key == grant.PermissionKey {
			permissionFound = true
			break
		}
	}
	if !permissionFound {
		return shared.NotFound("permission not found")
	}
	return nil
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func entryMap(entry config.Entry, found bool) map[string]any {
	if !found {
		return nil
	}
	return configEntryMap(entry)
}

func configEntryMap(entry config.Entry) map[string]any {
	return map[string]any{"key": entry.Key, "module_key": entry.ModuleKey, "scope": entry.Scope, "scope_id": entry.ScopeID, "value": cloneMap(entry.Value), "description": entry.Description}
}

func featureFlagMap(value featureflags.Value, found bool) map[string]any {
	if !found {
		return nil
	}
	return featureFlagValueMap(value)
}

func featureFlagValueMap(value featureflags.Value) map[string]any {
	return map[string]any{"flag_key": value.FlagKey, "scope": value.Scope, "scope_id": value.ScopeID, "enabled": value.Enabled}
}

func referenceRecordMap(record reference.Record, found bool) map[string]any {
	if !found {
		return nil
	}
	return map[string]any{"type_key": record.TypeKey, "key": record.Key, "display_name": record.DisplayName, "scope": record.Scope, "scope_id": record.ScopeID, "status": record.Status, "value": cloneMap(record.Value)}
}

func implementationIntArg(arguments map[string]any, key string) int {
	switch value := arguments[key].(type) {
	case int:
		return value
	case float64:
		return int(value)
	default:
		return 0
	}
}
