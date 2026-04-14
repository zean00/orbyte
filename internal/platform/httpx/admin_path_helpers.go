package httpx

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/integration"
	"orbyte/internal/platform/shared"
)

func adminModulePath(path string) (string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 4 || parts[0] != "admin" || parts[1] != "api" || parts[2] != "modules" {
		return "", false
	}
	return strings.TrimSpace(parts[3]), parts[3] != ""
}

func adminModuleConsolePath(path string) (string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 5 || parts[0] != "admin" || parts[1] != "api" || parts[2] != "modules" || parts[4] != "console" {
		return "", false
	}
	return strings.TrimSpace(parts[3]), parts[3] != ""
}

func adminModuleActionPath(path string) (string, string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 6 || parts[0] != "admin" || parts[1] != "api" || parts[2] != "modules" || parts[4] != "actions" {
		return "", "", false
	}
	return strings.TrimSpace(parts[3]), strings.TrimSpace(parts[5]), parts[3] != "" && parts[5] != ""
}

func adminConfigKeyPath(path string) (string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 6 || parts[0] != "admin" || parts[1] != "api" || parts[2] != "config" || parts[3] != "entries" || parts[5] != "value" {
		return "", false
	}
	return strings.TrimSpace(parts[4]), parts[4] != ""
}

func adminRolePermissionPath(path string) (string, string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 8 || parts[0] != "admin" || parts[1] != "api" || parts[2] != "security" || parts[3] != "roles" || parts[5] != "permissions" {
		return "", "", false
	}
	return strings.TrimSpace(parts[4]), strings.TrimSpace(parts[6]), parts[4] != "" && parts[6] != "" && parts[7] == "value"
}

func adminPolicyHookPath(path string) (string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 5 || parts[0] != "admin" || parts[1] != "api" || parts[2] != "security" || parts[3] != "policy-hooks" {
		return "", false
	}
	return strings.TrimSpace(parts[4]), parts[4] != ""
}

func adminPolicyHookRegoPath(path string) (string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 6 || parts[0] != "admin" || parts[1] != "api" || parts[2] != "security" || parts[3] != "policy-hooks" || parts[5] != "rego" {
		return "", false
	}
	return strings.TrimSpace(parts[4]), parts[4] != ""
}

func adminRoleTemplateActionPath(path string) (string, string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 8 || parts[0] != "admin" || parts[1] != "api" || parts[2] != "security" || parts[3] != "role-templates" || parts[6] != "actions" || parts[7] != "apply" {
		return "", "", false
	}
	return strings.TrimSpace(parts[4]), strings.TrimSpace(parts[5]), parts[4] != "" && parts[5] != ""
}

func adminIntegrationSubmissionActionPath(path string) (string, string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 7 || parts[0] != "admin" || parts[1] != "api" || parts[2] != "integrations" || parts[3] != "submissions" || parts[5] != "actions" {
		return "", "", false
	}
	return strings.TrimSpace(parts[4]), strings.TrimSpace(parts[6]), parts[4] != "" && parts[6] != ""
}

func adminIntegrationSubmissionDetailPath(path string) (string, string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 5 || len(parts) > 6 || parts[0] != "admin" || parts[1] != "api" || parts[2] != "integrations" || parts[3] != "submissions" {
		return "", "", false
	}
	detail := ""
	if len(parts) == 6 {
		detail = strings.TrimSpace(parts[5])
	}
	return strings.TrimSpace(parts[4]), detail, parts[4] != ""
}

func adminIntegrationSystemDetailPath(path string) (string, string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 6 || parts[0] != "admin" || parts[1] != "api" || parts[2] != "integrations" || parts[3] != "systems" {
		return "", "", false
	}
	return strings.TrimSpace(parts[4]), strings.TrimSpace(parts[5]), parts[4] != "" && parts[5] != ""
}

func adminIntegrationEndpointDetailPath(path string) (string, string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 6 || parts[0] != "admin" || parts[1] != "api" || parts[2] != "integrations" || parts[3] != "endpoints" {
		return "", "", false
	}
	return strings.TrimSpace(parts[4]), strings.TrimSpace(parts[5]), parts[4] != "" && parts[5] != ""
}

func respondIntegrationError(w http.ResponseWriter, err error, payload any) {
	var validationErr integration.ValidationError
	if errors.As(err, &validationErr) {
		response := map[string]any{
			"error": map[string]any{
				"kind":    shared.KindValidation,
				"message": validationErr.Error(),
			},
			"issues": validationErr.Issues,
		}
		if payload != nil {
			response["payload"] = payload
		}
		respondJSON(w, http.StatusBadRequest, response)
		return
	}
	respondError(w, err)
}

func adminFeatureFlagPath(path string) (string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 5 || parts[0] != "admin" || parts[1] != "api" || parts[2] != "feature-flags" || parts[4] != "value" {
		return "", false
	}
	return strings.TrimSpace(parts[3]), parts[3] != ""
}

func adminOperatingUnitPath(path string) (string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 5 || parts[0] != "admin" || parts[1] != "api" || parts[2] != "operating-units" {
		return "", false
	}
	return strings.TrimSpace(parts[3]), parts[3] != "" && parts[4] == "value"
}

func adminWorkflowPath(path string) (string, int, string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 4 || parts[0] != "admin" || parts[1] != "api" || parts[2] != "workflows" {
		return "", 0, "", false
	}
	key := strings.TrimSpace(parts[3])
	if key == "" {
		return "", 0, "", false
	}
	if len(parts) == 4 {
		return key, 0, "", true
	}
	if len(parts) == 5 && parts[4] == "drafts" {
		return key, 0, "drafts", true
	}
	if len(parts) == 5 && parts[4] == "versions" {
		return key, 0, "versions", true
	}
	if len(parts) == 6 && parts[4] == "versions" {
		version, err := strconv.Atoi(strings.TrimSpace(parts[5]))
		if err != nil || version <= 0 {
			return "", 0, "", false
		}
		return key, version, "", true
	}
	if len(parts) == 7 && parts[4] == "versions" {
		version, err := strconv.Atoi(strings.TrimSpace(parts[5]))
		if err != nil || version <= 0 {
			return "", 0, "", false
		}
		return key, version, strings.TrimSpace(parts[6]), true
	}
	return "", 0, "", false
}

func adminReportingLinePath(path string) (string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 4 || parts[0] != "admin" || parts[1] != "api" || parts[2] != "reporting-lines" {
		return "", false
	}
	return strings.TrimSpace(parts[3]), parts[3] != ""
}

func reportingLineFromRequest(id string, req reportingLineRequest) identity.ReportingLine {
	line := identity.ReportingLine{
		ID:               strings.TrimSpace(id),
		SubjectUserID:    strings.TrimSpace(req.SubjectUserID),
		ManagerUserID:    strings.TrimSpace(req.ManagerUserID),
		RelationshipType: strings.TrimSpace(req.RelationshipType),
		OrganizationID:   strings.TrimSpace(req.OrganizationID),
		LocationID:       strings.TrimSpace(req.LocationID),
		OperatingUnitID:  strings.TrimSpace(req.OperatingUnitID),
		Status:           strings.TrimSpace(req.Status),
		Priority:         req.Priority,
	}
	if parsed, err := parseOptionalTime(req.EffectiveFrom); err == nil {
		line.EffectiveFrom = parsed
	}
	if parsed, err := parseOptionalTime(req.EffectiveTo); err == nil {
		line.EffectiveTo = parsed
	}
	return line
}

func parseOptionalTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, value)
}
