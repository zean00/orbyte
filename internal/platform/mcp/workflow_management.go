package mcp

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/shared"
	"orbyte/internal/platform/workflow"
)

type workflowHierarchyNode struct {
	ID                string `json:"id"`
	Username          string `json:"username"`
	Status            string `json:"status"`
	DefaultLocationID string `json:"default_location_id,omitempty"`
}

type workflowHierarchyEdge struct {
	ID               string    `json:"id"`
	SubjectUserID    string    `json:"subject_user_id"`
	ManagerUserID    string    `json:"manager_user_id"`
	RelationshipType string    `json:"relationship_type"`
	Status           string    `json:"status"`
	OrganizationID   string    `json:"organization_id,omitempty"`
	LocationID       string    `json:"location_id,omitempty"`
	OperatingUnitID  string    `json:"operating_unit_id,omitempty"`
	Priority         int       `json:"priority,omitempty"`
	EffectiveFrom    time.Time `json:"effective_from"`
	EffectiveTo      time.Time `json:"effective_to,omitempty"`
}

type workflowHierarchySummary struct {
	TotalUsers      int `json:"total_users"`
	ActiveLines     int `json:"active_lines"`
	OrphanUsers     int `json:"orphan_users"`
	ActingOverrides int `json:"acting_overrides"`
}

func (s *Server) workflowDefinitionList(actor ActorContext, _ map[string]any) (map[string]any, bool, error) {
	if !allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	items := s.workflows.ListDefinitions()
	sort.Slice(items, func(i, j int) bool { return items[i].Key < items[j].Key })
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Found %d workflow definitions.", len(items))}},
		"structuredContent": map[string]any{"items": items},
		"_meta":             s.workflowAppMeta("", 0, "", ""),
	}, true, nil
}

func (s *Server) workflowDefinitionGet(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if !allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	key := strings.TrimSpace(stringArg(arguments, "workflow_key"))
	if key == "" {
		return nil, true, shared.Validation("workflow_key is required")
	}
	def, versions, draft, err := s.workflowState(key)
	if err != nil {
		return nil, true, err
	}
	return map[string]any{
		"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded workflow definition %s.", key)}},
		"structuredContent": map[string]any{
			"definition":    def,
			"versions":      versions,
			"current_draft": draft,
		},
		"_meta": s.workflowAppMeta(key, draft.Version, "", ""),
	}, true, nil
}

func (s *Server) workflowVersionList(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if !allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	key := strings.TrimSpace(stringArg(arguments, "workflow_key"))
	if key == "" {
		return nil, true, shared.Validation("workflow_key is required")
	}
	items := s.workflows.ListVersions(key)
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Found %d versions for workflow %s.", len(items), key)}},
		"structuredContent": map[string]any{"items": items},
		"_meta":             s.workflowAppMeta(key, 0, "", ""),
	}, true, nil
}

func (s *Server) workflowDraftCreate(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if !allowsAll(actor.PermissionChecker, []string{"configuration.manage"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	key := strings.TrimSpace(stringArg(arguments, "workflow_key"))
	if key == "" {
		return nil, true, shared.Validation("workflow_key is required")
	}
	created, err := s.workflows.CreateDraft(key, workflowActorID(actor))
	if err != nil {
		return nil, true, err
	}
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Created draft v%d for workflow %s.", created.Version, key)}},
		"structuredContent": created,
		"_meta":             s.workflowAppMeta(key, created.Version, "", ""),
	}, true, nil
}

func (s *Server) workflowDraftGet(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if !allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	key := strings.TrimSpace(stringArg(arguments, "workflow_key"))
	if key == "" {
		return nil, true, shared.Validation("workflow_key is required")
	}
	draft, err := s.workflowDraftFromArgs(arguments)
	if err != nil {
		return nil, true, err
	}
	def, versions, _, err := s.workflowState(key)
	if err != nil {
		return nil, true, err
	}
	return map[string]any{
		"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded draft context for workflow %s.", key)}},
		"structuredContent": map[string]any{
			"definition": def,
			"versions":   versions,
			"draft":      draft,
		},
		"_meta": s.workflowAppMeta(key, draft.Version, "", ""),
	}, true, nil
}

func (s *Server) workflowDraftSave(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if !allowsAll(actor.PermissionChecker, []string{"configuration.manage"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	var def workflow.Definition
	if err := decodeObjectArg(arguments, "workflow", &def); err != nil {
		return nil, true, err
	}
	saved, err := s.workflows.SaveDraft(def, workflowActorID(actor))
	if err != nil {
		return nil, true, err
	}
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Saved workflow draft v%d for %s.", saved.Version, saved.Key)}},
		"structuredContent": saved,
		"_meta":             s.workflowAppMeta(saved.Key, saved.Version, "", ""),
	}, true, nil
}

func (s *Server) workflowDraftValidate(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if !allowsAll(actor.PermissionChecker, []string{"configuration.manage"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	def, err := s.workflowDefinitionFromArgs(arguments)
	if err != nil {
		return nil, true, err
	}
	result := s.workflows.Validate(def)
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Validated workflow %s.", def.Key)}},
		"structuredContent": result,
		"_meta":             s.workflowAppMeta(def.Key, def.Version, "", ""),
	}, true, nil
}

func (s *Server) workflowDraftSimulate(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if !allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	def, err := s.workflowDefinitionFromArgs(arguments)
	if err != nil {
		return nil, true, err
	}
	var input workflow.SimulationInput
	if err := decodeObjectArg(arguments, "input", &input); err != nil {
		return nil, true, err
	}
	if strings.TrimSpace(input.ActorID) == "" {
		input.ActorID = workflowActorID(actor)
	}
	if strings.TrimSpace(input.OrganizationID) == "" {
		input.OrganizationID = actor.OrganizationID
	}
	if strings.TrimSpace(input.LocationID) == "" {
		input.LocationID = actor.LocationID
	}
	result := s.workflows.Simulate(def, input)
	return map[string]any{
		"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Simulated workflow %s action %s from %s.", def.Key, input.Action, input.CurrentState)}},
		"structuredContent": map[string]any{
			"simulation":      result,
			"routing_preview": s.workflowRoutingPreview(def, input),
		},
		"_meta": s.workflowAppMeta(def.Key, def.Version, stringMapValue(input.AdditionalInput, "requester_user_id"), input.LocationID),
	}, true, nil
}

func (s *Server) workflowDraftPublish(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if !allowsAll(actor.PermissionChecker, []string{"configuration.manage"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	if !boolArg(arguments, "confirm_publish") {
		return nil, true, shared.Validation("confirm_publish must be true")
	}
	key := strings.TrimSpace(stringArg(arguments, "workflow_key"))
	version := intArg(arguments, "version")
	if key == "" || version <= 0 {
		return nil, true, shared.Validation("workflow_key and version are required")
	}
	published, err := s.workflows.Publish(key, version, workflowActorID(actor))
	if err != nil {
		return nil, true, err
	}
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Published workflow %s v%d.", key, version)}},
		"structuredContent": published,
		"_meta":             s.workflowAppMeta(key, version, "", ""),
	}, true, nil
}

func (s *Server) workflowRuntimeTasksList(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if !allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	items := s.workflows.ListTasks()
	filtered := make([]workflow.Task, 0, len(items))
	for _, item := range items {
		if value := stringArg(arguments, "workflow_key"); value != "" && item.WorkflowKey != value {
			continue
		}
		if value := stringArg(arguments, "status"); value != "" && item.Status != value {
			continue
		}
		if value := stringArg(arguments, "assignee_user_id"); value != "" && item.AssigneeUserID != value {
			continue
		}
		if value := stringArg(arguments, "target_id"); value != "" && item.TargetID != value {
			continue
		}
		filtered = append(filtered, item)
	}
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Found %d workflow tasks.", len(filtered))}},
		"structuredContent": map[string]any{"items": filtered},
	}, true, nil
}

func (s *Server) workflowRuntimeApprovalsList(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if !allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	items := s.workflows.ListApprovals()
	filtered := make([]workflow.Approval, 0, len(items))
	for _, item := range items {
		if value := stringArg(arguments, "workflow_key"); value != "" && item.WorkflowKey != value {
			continue
		}
		if value := stringArg(arguments, "status"); value != "" && item.Status != value {
			continue
		}
		if value := stringArg(arguments, "target_id"); value != "" && item.TargetID != value {
			continue
		}
		if value := stringArg(arguments, "stage_key"); value != "" && item.StageKey != value {
			continue
		}
		filtered = append(filtered, item)
	}
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Found %d workflow approvals.", len(filtered))}},
		"structuredContent": map[string]any{"items": filtered},
	}, true, nil
}

func (s *Server) workflowRuntimeHistoryGet(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if !allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	targetType := strings.TrimSpace(stringArg(arguments, "target_type"))
	targetID := strings.TrimSpace(stringArg(arguments, "target_id"))
	if targetType == "" || targetID == "" {
		return nil, true, shared.Validation("target_type and target_id are required")
	}
	items := s.workflows.ListHistory(targetType, targetID)
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Found %d workflow history events for %s %s.", len(items), targetType, targetID)}},
		"structuredContent": map[string]any{"items": items},
	}, true, nil
}

func (s *Server) workflowHierarchyGraphGet(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if !allowsAll(actor.PermissionChecker, []string{"identity.manage_users"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	nodes, edges := workflowHierarchyGraphData(
		s.identity,
		stringArg(arguments, "organization_id"),
		stringArg(arguments, "location_id"),
		stringArg(arguments, "operating_unit_id"),
		stringArg(arguments, "status"),
	)
	return map[string]any{
		"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded hierarchy graph with %d users and %d reporting lines.", len(nodes), len(edges))}},
		"structuredContent": map[string]any{
			"nodes":   nodes,
			"edges":   edges,
			"summary": workflowHierarchySummaryFromData(nodes, edges),
		},
	}, true, nil
}

func (s *Server) workflowHierarchyChainGet(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if !allowsAll(actor.PermissionChecker, []string{"identity.manage_users"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	userID := strings.TrimSpace(stringArg(arguments, "user_id"))
	if userID == "" {
		return nil, true, shared.Validation("user_id is required")
	}
	items := workflowHierarchyChain(s.identity, userID, stringArg(arguments, "organization_id"), stringArg(arguments, "location_id"), stringArg(arguments, "operating_unit_id"))
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded manager chain with %d entries for user %s.", len(items), userID)}},
		"structuredContent": map[string]any{"items": items},
		"_meta":             s.workflowAppMeta("", 0, userID, stringArg(arguments, "location_id")),
	}, true, nil
}

func (s *Server) workflowHierarchySummaryGet(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if !allowsAll(actor.PermissionChecker, []string{"identity.manage_users"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	nodes, edges := workflowHierarchyGraphData(
		s.identity,
		stringArg(arguments, "organization_id"),
		stringArg(arguments, "location_id"),
		stringArg(arguments, "operating_unit_id"),
		stringArg(arguments, "status"),
	)
	summary := workflowHierarchySummaryFromData(nodes, edges)
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: "Loaded workflow hierarchy summary."}},
		"structuredContent": summary,
	}, true, nil
}

func (s *Server) workflowReportingLineList(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if !allowsAll(actor.PermissionChecker, []string{"identity.manage_users"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	items := s.identity.ReportingLines()
	filtered := make([]identity.ReportingLine, 0, len(items))
	for _, item := range items {
		if value := stringArg(arguments, "subject_user_id"); value != "" && item.SubjectUserID != value {
			continue
		}
		if value := stringArg(arguments, "manager_user_id"); value != "" && item.ManagerUserID != value {
			continue
		}
		if value := stringArg(arguments, "status"); value != "" && item.Status != value {
			continue
		}
		filtered = append(filtered, item)
	}
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Found %d reporting lines.", len(filtered))}},
		"structuredContent": map[string]any{"items": filtered},
	}, true, nil
}

func (s *Server) workflowReportingLineSave(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if !allowsAll(actor.PermissionChecker, []string{"identity.manage_users"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	var line identity.ReportingLine
	if err := decodeObjectArg(arguments, "reporting_line", &line); err != nil {
		return nil, true, err
	}
	saved, err := s.identity.UpsertReportingLine(line)
	if err != nil {
		return nil, true, err
	}
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Saved reporting line %s.", saved.ID)}},
		"structuredContent": saved,
		"_meta":             s.workflowAppMeta("", 0, saved.SubjectUserID, saved.LocationID),
	}, true, nil
}

func (s *Server) workflowState(key string) (workflow.Definition, []workflow.Definition, workflow.Definition, error) {
	def, err := s.workflows.Get(strings.TrimSpace(key))
	if err != nil {
		return workflow.Definition{}, nil, workflow.Definition{}, err
	}
	versions := s.workflows.ListVersions(key)
	draft := workflow.Definition{}
	for _, item := range versions {
		if item.Status == "draft" {
			draft = item
			break
		}
	}
	if draft.Key == "" {
		draft = def
	}
	return def, versions, draft, nil
}

func (s *Server) workflowDraftFromArgs(arguments map[string]any) (workflow.Definition, error) {
	key := strings.TrimSpace(stringArg(arguments, "workflow_key"))
	if key == "" {
		return workflow.Definition{}, shared.Validation("workflow_key is required")
	}
	version := intArg(arguments, "version")
	if version > 0 {
		item, err := s.workflows.GetVersion(key, version)
		if err != nil {
			return workflow.Definition{}, err
		}
		if item.Status != "draft" {
			return workflow.Definition{}, shared.Validation("requested version is not a draft")
		}
		return item, nil
	}
	for _, item := range s.workflows.ListVersions(key) {
		if item.Status == "draft" {
			return item, nil
		}
	}
	return workflow.Definition{}, shared.NotFound("workflow draft not found")
}

func (s *Server) workflowDefinitionFromArgs(arguments map[string]any) (workflow.Definition, error) {
	var def workflow.Definition
	if err := decodeOptionalObjectArg(arguments, "workflow", &def); err == nil && strings.TrimSpace(def.Key) != "" {
		return def, nil
	}
	if key := strings.TrimSpace(stringArg(arguments, "workflow_key")); key != "" {
		version := intArg(arguments, "version")
		if version > 0 {
			return s.workflows.GetVersion(key, version)
		}
		return s.workflowDraftFromArgs(arguments)
	}
	return workflow.Definition{}, shared.Validation("workflow or workflow_key is required")
}

func (s *Server) workflowRoutingPreview(def workflow.Definition, input workflow.SimulationInput) map[string]any {
	preview := map[string]any{"valid": false}
	if s.identity == nil {
		preview["error"] = "identity service unavailable"
		return preview
	}
	transition, err := workflowTransitionForMCP(def, input.CurrentState, input.Action)
	if err != nil {
		preview["error"] = err.Error()
		return preview
	}
	requesterUserID := stringMapValue(input.AdditionalInput, "requester_user_id")
	if requesterUserID == "" {
		requesterUserID = strings.TrimSpace(input.ActorID)
	}
	previousApproverID := stringMapValue(input.AdditionalInput, "previous_approver_id")
	sourceUserID := ""
	switch transition.AssignmentStrategy {
	case "requester_manager":
		sourceUserID = requesterUserID
	case "previous_approver_manager":
		sourceUserID = firstNonEmpty(previousApproverID, input.ActorID)
	}
	preview["transition"] = transition
	preview["source_user_id"] = sourceUserID
	preview["valid"] = true
	switch transition.AssignmentStrategy {
	case "", "static_user", "static_role":
		preview["mode"] = firstNonEmpty(transition.AssignmentMode, "static")
		preview["fallback_role_key"] = transition.FallbackRoleKey
		preview["candidate_role_keys"] = append([]string(nil), transition.CandidateRoleKeys...)
	case "requester_manager", "previous_approver_manager":
		if sourceUserID == "" {
			preview["valid"] = false
			preview["error"] = "source user is required for manager routing"
			return preview
		}
		if resolution, ok := s.identity.ResolveManager(sourceUserID, input.OrganizationID, input.LocationID, "", time.Now().UTC()); ok {
			preview["resolved_via"] = resolution.Via
			preview["resolved_assignee_user_id"] = resolution.Manager.ID
			preview["resolved_assignee_username"] = resolution.Manager.Username
			preview["line"] = resolution.Line
			return preview
		}
		preview["fallback_used"] = true
		fallthrough
	case "role_fallback":
		roleKey := firstNonEmpty(transition.FallbackRoleKey, transition.AssigneeRoleKey)
		preview["fallback_role_key"] = roleKey
		candidates := s.identity.ResolveRoleCandidates(roleKey, input.OrganizationID, input.LocationID, "", time.Now().UTC())
		if len(candidates) == 0 {
			preview["valid"] = false
			preview["error"] = "no matching assignee candidates"
			return preview
		}
		if len(candidates) == 1 {
			preview["resolved_via"] = "fallback_role"
			preview["resolved_assignee_user_id"] = candidates[0].ID
			preview["resolved_assignee_username"] = candidates[0].Username
			return preview
		}
		preview["resolved_via"] = "fallback_role"
		preview["resolved_candidate_user_ids"] = mapIdentityUsers(candidates, func(item identity.User) string { return item.ID })
		preview["resolved_candidate_usernames"] = mapIdentityUsers(candidates, func(item identity.User) string { return item.Username })
	default:
		preview["valid"] = false
		preview["error"] = "unsupported assignment strategy"
	}
	return preview
}

func (s *Server) workflowAppMeta(workflowKey string, version int, userID, locationID string) map[string]any {
	values := url.Values{}
	if strings.TrimSpace(workflowKey) != "" {
		values.Set("workflow_key", strings.TrimSpace(workflowKey))
	}
	if version > 0 {
		values.Set("version", strconv.Itoa(version))
	}
	if strings.TrimSpace(userID) != "" {
		values.Set("user_id", strings.TrimSpace(userID))
	}
	if strings.TrimSpace(locationID) != "" {
		values.Set("location_id", strings.TrimSpace(locationID))
	}
	resourceURI := workflowManagerResourceURI
	if encoded := values.Encode(); encoded != "" {
		resourceURI += "?" + encoded
	}
	return map[string]any{
		"orbyte/app": map[string]any{
			"key":          workflowManagerAppKey,
			"title":        "Workflow Manager",
			"resource_uri": resourceURI,
		},
	}
}

func (s *Server) renderWorkflowManagerApp(actor ActorContext, parsed *url.URL) (string, error) {
	if !allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		return "", fmt.Errorf("resource is not allowed")
	}
	definitions := s.workflows.ListDefinitions()
	sort.Slice(definitions, func(i, j int) bool { return definitions[i].Key < definitions[j].Key })
	title := "Workflow Manager"
	workflowKey := strings.TrimSpace(parsed.Query().Get("workflow_key"))
	userID := strings.TrimSpace(parsed.Query().Get("user_id"))
	locationID := strings.TrimSpace(parsed.Query().Get("location_id"))
	version := 0
	if raw := strings.TrimSpace(parsed.Query().Get("version")); raw != "" {
		version, _ = strconv.Atoi(raw)
	}
	summary := `<section><h3>Capabilities</h3><ul><li>Draft create/save/validate/simulate/publish through MCP tools</li><li>Read-only runtime task, approval, and history inspection</li><li>Hierarchy graph, manager chain, and reporting-line management</li></ul></section>`
	body := summary
	if workflowKey != "" {
		def, versions, draft, err := s.workflowState(workflowKey)
		if err != nil {
			return "", err
		}
		selected := draft
		if version > 0 {
			selected, err = s.workflows.GetVersion(workflowKey, version)
			if err != nil {
				return "", err
			}
		}
		simInput := workflow.SimulationInput{CurrentState: "draft", Action: "submit", ActorID: actor.EffectiveUserID, LocationID: firstNonEmpty(locationID, actor.LocationID)}
		if userID != "" {
			simInput.AdditionalInput = map[string]any{"requester_user_id": userID}
		}
		routingPreview := s.workflowRoutingPreview(selected, simInput)
		rawDefinition, _ := json.MarshalIndent(def, "", "  ")
		rawVersions, _ := json.MarshalIndent(versions, "", "  ")
		rawSelected, _ := json.MarshalIndent(selected, "", "  ")
		rawPreview, _ := json.MarshalIndent(routingPreview, "", "  ")
		body = `<section><h3>Definition</h3><pre>` + escapeHTML(string(rawDefinition)) + `</pre></section>` +
			`<section><h3>Versions</h3><pre>` + escapeHTML(string(rawVersions)) + `</pre></section>` +
			`<section><h3>Selected Draft/Version</h3><pre>` + escapeHTML(string(rawSelected)) + `</pre></section>` +
			`<section><h3>Routing Preview</h3><pre>` + escapeHTML(string(rawPreview)) + `</pre></section>`
		title = "Workflow Manager: " + workflowKey
	}
	hierarchyPanel := ""
	if s.identity != nil && allowsAll(actor.PermissionChecker, []string{"identity.manage_users"}) {
		nodes, edges := workflowHierarchyGraphData(s.identity, "", locationID, "", "active")
		summaryData := workflowHierarchySummaryFromData(nodes, edges)
		rawSummary, _ := json.MarshalIndent(summaryData, "", "  ")
		hierarchyPanel = `<section><h3>Hierarchy Summary</h3><pre>` + escapeHTML(string(rawSummary)) + `</pre></section>`
		if userID != "" {
			rawChain, _ := json.MarshalIndent(workflowHierarchyChain(s.identity, userID, "", locationID, ""), "", "  ")
			hierarchyPanel += `<section><h3>Manager Chain</h3><pre>` + escapeHTML(string(rawChain)) + `</pre></section>`
		}
	}
	return `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>` +
		escapeHTML(title) +
		`</title><style>body{font-family:Georgia,serif;background:#f4efe6;color:#17221d;margin:0;padding:24px}main{display:grid;gap:16px}.panel{background:#fffdf9;border:1px solid #d7d0c4;border-radius:14px;padding:16px}.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(180px,1fr));gap:12px}.card{background:#f7f2e9;border:1px solid #d7d0c4;border-radius:12px;padding:12px}.meta{font-size:12px;color:#617066}pre{white-space:pre-wrap;background:#f8f3ea;border:1px solid #d7d0c4;border-radius:12px;padding:12px;overflow:auto}ul{margin:0;padding-left:18px}h1,h3{margin:0 0 12px}p{margin:0 0 12px}</style></head><body><main><section class="panel"><h1>` +
		escapeHTML(title) +
		`</h1><div class="grid"><article class="card"><div class="meta">Definitions</div><strong>` + escapeHTML(strconv.Itoa(len(definitions))) + `</strong></article><article class="card"><div class="meta">Hierarchy Access</div><strong>` + escapeHTML(strconv.FormatBool(allowsAll(actor.PermissionChecker, []string{"identity.manage_users"}))) + `</strong></article></div></section><section class="panel">` +
		body + hierarchyPanel + `</section></main></body></html>`, nil
}

func workflowActorID(actor ActorContext) string {
	return firstNonEmpty(actor.EffectiveUserID, actor.ActorID)
}

func workflowHierarchyGraphData(ident *identity.Service, organizationID, locationID, operatingUnitID, status string) ([]workflowHierarchyNode, []workflowHierarchyEdge) {
	lines := ident.ReportingLines()
	filteredEdges := make([]workflowHierarchyEdge, 0)
	includedUserIDs := map[string]struct{}{}
	status = strings.TrimSpace(status)
	for _, line := range lines {
		if !workflowReportingLineMatchesFilters(line, organizationID, locationID, operatingUnitID, status) {
			continue
		}
		filteredEdges = append(filteredEdges, workflowHierarchyEdge{
			ID:               line.ID,
			SubjectUserID:    line.SubjectUserID,
			ManagerUserID:    line.ManagerUserID,
			RelationshipType: line.RelationshipType,
			Status:           line.Status,
			OrganizationID:   line.OrganizationID,
			LocationID:       line.LocationID,
			OperatingUnitID:  line.OperatingUnitID,
			Priority:         line.Priority,
			EffectiveFrom:    line.EffectiveFrom,
			EffectiveTo:      line.EffectiveTo,
		})
		includedUserIDs[line.SubjectUserID] = struct{}{}
		includedUserIDs[line.ManagerUserID] = struct{}{}
	}
	users := ident.Users()
	nodes := make([]workflowHierarchyNode, 0)
	for _, user := range users {
		if locationID != "" && user.DefaultLocationID != "" && user.DefaultLocationID != locationID {
			if _, ok := includedUserIDs[user.ID]; !ok {
				continue
			}
		}
		if len(includedUserIDs) > 0 {
			if _, ok := includedUserIDs[user.ID]; !ok && locationID != "" {
				continue
			}
		}
		nodes = append(nodes, workflowHierarchyNode{
			ID:                user.ID,
			Username:          user.Username,
			Status:            user.Status,
			DefaultLocationID: user.DefaultLocationID,
		})
	}
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Username == nodes[j].Username {
			return nodes[i].ID < nodes[j].ID
		}
		return nodes[i].Username < nodes[j].Username
	})
	sort.Slice(filteredEdges, func(i, j int) bool {
		if filteredEdges[i].SubjectUserID == filteredEdges[j].SubjectUserID {
			if filteredEdges[i].Priority == filteredEdges[j].Priority {
				return filteredEdges[i].ID < filteredEdges[j].ID
			}
			return filteredEdges[i].Priority > filteredEdges[j].Priority
		}
		return filteredEdges[i].SubjectUserID < filteredEdges[j].SubjectUserID
	})
	return nodes, filteredEdges
}

func workflowHierarchySummaryFromData(nodes []workflowHierarchyNode, edges []workflowHierarchyEdge) workflowHierarchySummary {
	summary := workflowHierarchySummary{TotalUsers: len(nodes)}
	resolvedManagers := map[string]bool{}
	now := time.Now().UTC()
	for _, edge := range edges {
		if edge.Status == "active" && !edge.EffectiveFrom.After(now) && (edge.EffectiveTo.IsZero() || !edge.EffectiveTo.Before(now)) {
			summary.ActiveLines++
			resolvedManagers[edge.SubjectUserID] = true
			if edge.RelationshipType == "acting_manager" {
				summary.ActingOverrides++
			}
		}
	}
	for _, node := range nodes {
		if !resolvedManagers[node.ID] {
			summary.OrphanUsers++
		}
	}
	return summary
}

func workflowHierarchyChain(ident *identity.Service, userID, organizationID, locationID, operatingUnitID string) []map[string]any {
	items := make([]map[string]any, 0)
	visited := map[string]bool{}
	currentUserID := strings.TrimSpace(userID)
	for currentUserID != "" && !visited[currentUserID] {
		visited[currentUserID] = true
		user, ok := ident.FindUser(currentUserID)
		if !ok {
			break
		}
		entry := map[string]any{
			"user_id":   user.ID,
			"username":  user.Username,
			"user":      user,
			"is_origin": len(items) == 0,
		}
		resolution, ok := ident.ResolveManager(currentUserID, organizationID, locationID, operatingUnitID, time.Now().UTC())
		if ok {
			entry["manager_user_id"] = resolution.Manager.ID
			entry["manager_username"] = resolution.Manager.Username
			entry["resolved_via"] = resolution.Via
			entry["line"] = resolution.Line
			currentUserID = resolution.Manager.ID
		} else {
			currentUserID = ""
		}
		items = append(items, entry)
	}
	return items
}

func workflowReportingLineMatchesFilters(line identity.ReportingLine, organizationID, locationID, operatingUnitID, status string) bool {
	if organizationID != "" && line.OrganizationID != "" && line.OrganizationID != organizationID {
		return false
	}
	if locationID != "" && line.LocationID != "" && line.LocationID != locationID {
		return false
	}
	if operatingUnitID != "" && line.OperatingUnitID != "" && line.OperatingUnitID != operatingUnitID {
		return false
	}
	if status != "" && line.Status != status {
		return false
	}
	return true
}

func workflowTransitionForMCP(def workflow.Definition, currentState, action string) (workflow.Transition, error) {
	for _, rule := range def.Actions {
		if rule.Action == action && rule.FromState == currentState {
			return workflow.Transition{
				WorkflowKey:          def.Key,
				WorkflowVersion:      def.Version,
				Action:               rule.Action,
				FromState:            rule.FromState,
				ToState:              rule.ToState,
				PermissionKey:        rule.PermissionKey,
				TaskType:             rule.TaskType,
				CreateApproval:       rule.CreateApproval,
				AssignmentStrategy:   rule.AssignmentStrategy,
				AssignmentMode:       rule.AssignmentMode,
				AssigneeRoleKey:      rule.AssigneeRoleKey,
				CandidateRoleKeys:    append([]string(nil), rule.CandidateRoleKeys...),
				FallbackRoleKey:      rule.FallbackRoleKey,
				ApprovalStageKey:     rule.ApprovalStageKey,
				DueAfterSeconds:      rule.DueAfterSeconds,
				EscalateAfterSeconds: rule.EscalateAfterSeconds,
			}, nil
		}
	}
	return workflow.Transition{}, shared.Conflict("workflow action not allowed from current state")
}

func intArg(arguments map[string]any, key string) int {
	value, ok := arguments[key]
	if !ok || value == nil {
		return 0
	}
	switch typed := value.(type) {
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		number, _ := typed.Int64()
		return int(number)
	case string:
		number, _ := strconv.Atoi(strings.TrimSpace(typed))
		return number
	default:
		return 0
	}
}

func stringMapValue(values map[string]any, key string) string {
	if len(values) == 0 {
		return ""
	}
	value, _ := values[key].(string)
	return strings.TrimSpace(value)
}

func mapIdentityUsers[T any](items []identity.User, mapper func(identity.User) T) []T {
	output := make([]T, 0, len(items))
	for _, item := range items {
		output = append(output, mapper(item))
	}
	return output
}
