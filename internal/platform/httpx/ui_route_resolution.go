package httpx

import (
	"net/http"
	"strings"
	"time"

	"orbyte/internal/platform/document"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/workflow"
)

type uiRouteResolutionResponse struct {
	Status           string                         `json:"status"`
	RequestedPath    string                         `json:"requested_path"`
	ResolvedPath     string                         `json:"resolved_path,omitempty"`
	Surface          module.UISurface               `json:"surface"`
	SuggestedSurface module.UISurface               `json:"suggested_surface,omitempty"`
	FallbackPath     string                         `json:"fallback_path,omitempty"`
	Message          string                         `json:"message,omitempty"`
	Path             string                         `json:"path,omitempty"`
	ModuleKey        string                         `json:"module_key,omitempty"`
	RenderMode       module.ActionRenderMode        `json:"render_mode,omitempty"`
	Action           module.ActionDefinition        `json:"action,omitempty"`
	View             *module.ViewDefinition         `json:"view,omitempty"`
	CustomEntry      *module.CustomEntryDefinition  `json:"custom_entry,omitempty"`
	Flow             *module.DocumentFlowDefinition `json:"flow,omitempty"`
}

func resolveUIRoute(ident *identity.Service, modules *module.Service, p principal, surface module.UISurface, path string) uiRouteResolutionResponse {
	response := uiRouteResolutionResponse{
		Status:        "not_found",
		RequestedPath: path,
		Surface:       surface,
		FallbackPath:  fallbackPathForSurface(ident, modules, p, surface),
		Message:       "route not found",
	}
	resolution, ok := modules.ResolveRouteForSurface(path, surface)
	if !ok {
		for _, candidate := range availableUISurfaces(ident, modules, p) {
			candidateSurface := module.UISurface(candidate)
			if candidateSurface == surface {
				continue
			}
			if _, found := modules.ResolveRouteForSurface(path, candidateSurface); found {
				response.Status = "surface_mismatch"
				response.SuggestedSurface = candidateSurface
				response.FallbackPath = fallbackPathForSurface(ident, modules, p, candidateSurface)
				response.Message = "route belongs to a different surface"
				return response
			}
		}
		return response
	}
	if !principalAllowsAll(ident, p, resolution.Action.RequiredPermissions) {
		response.Status = "forbidden"
		response.Message = "route is not allowed"
		return response
	}
	if resolution.View != nil && !principalAllowsAll(ident, p, resolution.View.RequiredPermissions) {
		response.Status = "forbidden"
		response.Message = "view is not allowed"
		return response
	}
	if resolution.CustomEntry != nil && !principalAllowsAll(ident, p, resolution.CustomEntry.RequiredPermissions) {
		response.Status = "forbidden"
		response.Message = "route is not allowed"
		return response
	}
	if resolution.Flow != nil && !principalAllowsAll(ident, p, resolution.Flow.RequiredPermissions) {
		response.Status = "forbidden"
		response.Message = "route is not allowed"
		return response
	}
	response.Status = "ok"
	response.ResolvedPath = resolution.Path
	response.Path = resolution.Path
	response.ModuleKey = resolution.ModuleKey
	response.RenderMode = resolution.RenderMode
	response.Action = resolution.Action
	response.View = resolution.View
	response.CustomEntry = resolution.CustomEntry
	response.Flow = resolution.Flow
	response.Message = ""
	return response
}

func fallbackPathsForSurfaces(ident *identity.Service, modules *module.Service, p principal) map[string]string {
	items := map[string]string{}
	for _, surface := range availableUISurfaces(ident, modules, p) {
		if path := fallbackPathForSurface(ident, modules, p, module.UISurface(surface)); path != "" {
			items[surface] = path
		}
	}
	return items
}

func fallbackPathForSurface(ident *identity.Service, modules *module.Service, p principal, surface module.UISurface) string {
	menus, actions, _, _, _ := visibleUIContracts(ident, modules, p, surface)
	return defaultRouteForSurface(ident, p.userID, uiSurfacePreference(surface), menus, actions)
}

func visibleSelfServiceAPIs(ident *identity.Service, modules *module.Service, p principal) []module.SelfServiceAPIDefinition {
	items := make([]module.SelfServiceAPIDefinition, 0)
	for _, item := range modules.SelfServiceAPIs() {
		if !principalAllowsAll(ident, p, item.RequiredPermissions) {
			continue
		}
		items = append(items, item)
	}
	return items
}

func requestedUISurface(r *http.Request) module.UISurface {
	raw := strings.TrimSpace(r.URL.Query().Get("surface"))
	switch module.UISurface(raw) {
	case module.UISurfaceAdmin:
		return module.UISurfaceAdmin
	case module.UISurfaceBackoffice:
		return module.UISurfaceBackoffice
	case module.UISurfaceWorklist:
		return module.UISurfaceWorklist
	case module.UISurfaceSelfService:
		return module.UISurfaceSelfService
	case module.UISurfacePOS:
		return module.UISurfacePOS
	case module.UISurfaceMobile:
		return module.UISurfaceMobile
	default:
		return module.UISurfaceBackoffice
	}
}

func uiSurfacePreference(surface module.UISurface) string {
	switch surface {
	case module.UISurfaceAdmin:
		return "admin"
	case module.UISurfaceSelfService:
		return "self_service"
	default:
		return "user"
	}
}

func availableUISurfaces(ident *identity.Service, modules *module.Service, p principal) []string {
	items := make([]string, 0, 4)
	if menus, actions, _, _, _ := visibleUIContracts(ident, modules, p, module.UISurfaceBackoffice); len(menus) > 0 || len(actions) > 0 {
		items = append(items, string(module.UISurfaceBackoffice))
	}
	if menus, actions, _, _, _ := visibleUIContracts(ident, modules, p, module.UISurfaceWorklist); len(menus) > 0 || len(actions) > 0 {
		items = append(items, string(module.UISurfaceWorklist))
	}
	if menus, actions, _, _, _ := visibleUIContracts(ident, modules, p, module.UISurfaceSelfService); len(menus) > 0 || len(actions) > 0 {
		items = append(items, string(module.UISurfaceSelfService))
	}
	if menus, actions, _, _, _ := visibleUIContracts(ident, modules, p, module.UISurfacePOS); len(menus) > 0 || len(actions) > 0 {
		items = append(items, string(module.UISurfacePOS))
	}
	if len(items) == 0 {
		items = append(items, string(module.UISurfaceBackoffice))
	}
	return items
}

func filterWorkflowTasksForUI(docs *document.Service, items []workflow.Task, currentUserID string, r *http.Request) []uiWorklistTask {
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	targetIDFilter := strings.TrimSpace(r.URL.Query().Get("target_id"))
	workflowKeyFilter := strings.TrimSpace(r.URL.Query().Get("workflow_key"))
	dueFilter := strings.TrimSpace(r.URL.Query().Get("due"))
	mineFilter := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("mine")), "1") || strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("mine")), "true")
	now := time.Now().UTC()
	output := make([]uiWorklistTask, 0, len(items))
	for _, item := range items {
		if statusFilter != "" && item.Status != statusFilter {
			continue
		}
		if targetIDFilter != "" && item.TargetID != targetIDFilter {
			continue
		}
		if workflowKeyFilter != "" && item.WorkflowKey != workflowKeyFilter {
			continue
		}
		if mineFilter && item.AssigneeUserID != currentUserID {
			continue
		}
		if dueFilter == "overdue" && (item.DueAt.IsZero() || !item.DueAt.Before(now)) {
			continue
		}
		documentType, targetTitle, targetNumber, targetStatus, targetUpdatedAt := workflowTargetDocumentSummary(docs, item.TargetType, item.TargetID)
		output = append(output, uiWorklistTask{
			ID:              item.ID,
			WorkflowKey:     item.WorkflowKey,
			WorkflowVersion: item.WorkflowVersion,
			TargetType:      item.TargetType,
			TargetID:        item.TargetID,
			DocumentType:    documentType,
			TargetTitle:     targetTitle,
			TargetNumber:    targetNumber,
			TargetStatus:    targetStatus,
			TargetUpdatedAt: targetUpdatedAt,
			TaskType:        item.TaskType,
			Status:          item.Status,
			AssignmentMode:  item.AssignmentMode,
			AssigneeUserID:  item.AssigneeUserID,
			AssigneeRoleKey: item.AssigneeRoleKey,
			IsMine:          item.AssigneeUserID != "" && item.AssigneeUserID == currentUserID,
			DueAt:           formatOptionalTime(item.DueAt),
			EscalateAt:      formatOptionalTime(item.EscalateAt),
			Metadata:        item.Metadata,
		})
	}
	return output
}

func filterWorkflowApprovalsForUI(docs *document.Service, items []workflow.Approval, currentUserID string, r *http.Request) []uiWorklistApproval {
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	targetIDFilter := strings.TrimSpace(r.URL.Query().Get("target_id"))
	workflowKeyFilter := strings.TrimSpace(r.URL.Query().Get("workflow_key"))
	dueFilter := strings.TrimSpace(r.URL.Query().Get("due"))
	requestedByMe := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("requested_by_me")), "1") || strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("requested_by_me")), "true")
	now := time.Now().UTC()
	output := make([]uiWorklistApproval, 0, len(items))
	for _, item := range items {
		if statusFilter != "" && item.Status != statusFilter {
			continue
		}
		if targetIDFilter != "" && item.TargetID != targetIDFilter {
			continue
		}
		if workflowKeyFilter != "" && item.WorkflowKey != workflowKeyFilter {
			continue
		}
		if requestedByMe && item.RequestedBy != currentUserID {
			continue
		}
		if dueFilter == "overdue" && (item.DueAt.IsZero() || !item.DueAt.Before(now)) {
			continue
		}
		documentType, targetTitle, targetNumber, targetStatus, targetUpdatedAt := workflowTargetDocumentSummary(docs, item.TargetType, item.TargetID)
		output = append(output, uiWorklistApproval{
			ID:              item.ID,
			WorkflowKey:     item.WorkflowKey,
			WorkflowVersion: item.WorkflowVersion,
			TargetType:      item.TargetType,
			TargetID:        item.TargetID,
			DocumentType:    documentType,
			TargetTitle:     targetTitle,
			TargetNumber:    targetNumber,
			TargetStatus:    targetStatus,
			TargetUpdatedAt: targetUpdatedAt,
			Status:          item.Status,
			StageKey:        item.StageKey,
			RequestedBy:     item.RequestedBy,
			DueAt:           formatOptionalTime(item.DueAt),
			Metadata:        item.Metadata,
		})
	}
	return output
}

func formatOptionalTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func workflowTargetDocumentSummary(docs *document.Service, targetType, targetID string) (documentType, title, number, status, updatedAt string) {
	if docs == nil || targetType != "document" || strings.TrimSpace(targetID) == "" {
		return "", "", "", "", ""
	}
	record, err := docs.Get(targetID)
	if err != nil {
		return "", "", "", "", ""
	}
	title, _ = record.Body.Payload["title"].(string)
	return record.Header.Type, title, record.Header.Number, record.Header.Status, formatOptionalTime(record.Header.UpdatedAt)
}

func countWorkflowItems[T interface{ GetStatus() string }](items []T, status string) int {
	total := 0
	for _, item := range items {
		if item.GetStatus() == status {
			total++
		}
	}
	return total
}

func countWorkflowStatuses[T interface{ GetStatus() string }](items []T) map[string]int {
	counts := map[string]int{}
	for _, item := range items {
		counts[item.GetStatus()]++
	}
	return counts
}

func countWorklistMine(items []uiWorklistTask) int {
	total := 0
	for _, item := range items {
		if item.IsMine {
			total++
		}
	}
	return total
}

func countApprovalsRequestedByMe(items []uiWorklistApproval, currentUserID string) int {
	total := 0
	for _, item := range items {
		if currentUserID != "" && item.RequestedBy == currentUserID {
			total++
		}
	}
	return total
}

func countWorklistTaskWorkflowKeys(items []uiWorklistTask) int {
	keys := map[string]struct{}{}
	for _, item := range items {
		if item.WorkflowKey != "" {
			keys[item.WorkflowKey] = struct{}{}
		}
	}
	return len(keys)
}

func countWorklistApprovalWorkflowKeys(items []uiWorklistApproval) int {
	keys := map[string]struct{}{}
	for _, item := range items {
		if item.WorkflowKey != "" {
			keys[item.WorkflowKey] = struct{}{}
		}
	}
	return len(keys)
}

func (i uiWorklistTask) GetStatus() string     { return i.Status }
func (i uiWorklistApproval) GetStatus() string { return i.Status }
