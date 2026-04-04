package document

import (
	"net/url"
	"strings"
)

type SpecializedViewer struct {
	Hint             string
	RequestKinds     []string
	DetailPath       string
	FormPath         string
	EditableStatuses []string
}

type NativeDocumentViewer struct {
	DocumentType     string
	DetailPath       string
	FormPath         string
	EditableStatuses []string
}

func (s *Service) RegisterSpecializedViewer(viewer SpecializedViewer) {
	if s == nil {
		return
	}
	hint := strings.TrimSpace(viewer.Hint)
	if hint == "" {
		return
	}
	normalized := viewer
	normalized.Hint = hint
	if len(normalized.EditableStatuses) == 0 {
		normalized.EditableStatuses = []string{"draft"}
	}
	if s.specializedViewers == nil {
		s.specializedViewers = map[string]SpecializedViewer{}
	}
	if s.specializedFallback == nil {
		s.specializedFallback = map[string]string{}
	}
	s.specializedViewers[hint] = normalized
	for _, kind := range normalized.RequestKinds {
		if trimmed := strings.TrimSpace(kind); trimmed != "" {
			s.specializedFallback[trimmed] = hint
		}
	}
}

func (s *Service) RegisterNativeDocumentViewer(viewer NativeDocumentViewer) {
	if s == nil {
		return
	}
	documentType := strings.TrimSpace(viewer.DocumentType)
	if documentType == "" {
		return
	}
	normalized := viewer
	normalized.DocumentType = documentType
	if len(normalized.EditableStatuses) == 0 {
		normalized.EditableStatuses = []string{"draft"}
	}
	if s.specializedViewers == nil {
		s.specializedViewers = map[string]SpecializedViewer{}
	}
	if s.specializedFallback == nil {
		s.specializedFallback = map[string]string{}
	}
	if s.nativeDocumentViewers == nil {
		s.nativeDocumentViewers = map[string]NativeDocumentViewer{}
	}
	s.nativeDocumentViewers[documentType] = normalized
}

func (s *Service) ResolveWorkspaceOpenPath(record Record) string {
	if viewer, ok := s.specializedViewerForRecord(record); ok {
		documentID := url.QueryEscape(strings.TrimSpace(record.Header.ID))
		if viewer.FormPath != "" && viewerAllowsEdit(viewer, record.Header.Status) {
			return viewer.FormPath + "?id=" + documentID
		}
		if viewer.DetailPath != "" {
			return viewer.DetailPath + "?id=" + documentID
		}
	}
	if viewer, ok := s.nativeDocumentViewerForRecord(record); ok {
		documentID := url.QueryEscape(strings.TrimSpace(record.Header.ID))
		if viewer.FormPath != "" && nativeViewerAllowsEdit(viewer, record.Header.Status) {
			return viewer.FormPath + "?id=" + documentID
		}
		if viewer.DetailPath != "" {
			return viewer.DetailPath + "?id=" + documentID
		}
	}
	return "/ui/documents/detail?id=" + url.QueryEscape(strings.TrimSpace(record.Header.ID))
}

func (s *Service) ResolveWorkspaceEditPath(record Record) string {
	if viewer, ok := s.specializedViewerForRecord(record); ok && viewer.FormPath != "" {
		documentID := url.QueryEscape(strings.TrimSpace(record.Header.ID))
		return viewer.FormPath + "?id=" + documentID
	}
	if viewer, ok := s.nativeDocumentViewerForRecord(record); ok && viewer.FormPath != "" {
		documentID := url.QueryEscape(strings.TrimSpace(record.Header.ID))
		return viewer.FormPath + "?id=" + documentID
	}
	return "/ui/documents/form?id=" + url.QueryEscape(strings.TrimSpace(record.Header.ID))
}

func (s *Service) specializedViewerForRecord(record Record) (SpecializedViewer, bool) {
	if s == nil {
		return SpecializedViewer{}, false
	}
	payload := record.Body.Payload
	hint := strings.TrimSpace(stringValue(payload["viewer_hint"]))
	if hint == "" {
		if requestKind := strings.TrimSpace(stringValue(payload["request_kind"])); requestKind != "" {
			hint = s.specializedFallback[requestKind]
		}
	}
	if hint == "" && looksLikePromotionPlanPayload(payload) {
		hint = "promotion.plan"
	}
	if hint == "" {
		return SpecializedViewer{}, false
	}
	viewer, ok := s.specializedViewers[hint]
	return viewer, ok
}

func (s *Service) nativeDocumentViewerForRecord(record Record) (NativeDocumentViewer, bool) {
	if s == nil || s.nativeDocumentViewers == nil {
		return NativeDocumentViewer{}, false
	}
	if strings.TrimSpace(record.Header.Type) == "" || strings.TrimSpace(record.Header.Type) == "generic_request" {
		return NativeDocumentViewer{}, false
	}
	viewer, ok := s.nativeDocumentViewers[strings.TrimSpace(record.Header.Type)]
	return viewer, ok
}

func viewerAllowsEdit(viewer SpecializedViewer, status string) bool {
	normalizedStatus := strings.ToLower(strings.TrimSpace(status))
	if normalizedStatus == "" {
		return false
	}
	for _, candidate := range viewer.EditableStatuses {
		if normalizedStatus == strings.ToLower(strings.TrimSpace(candidate)) {
			return true
		}
	}
	return false
}

func nativeViewerAllowsEdit(viewer NativeDocumentViewer, status string) bool {
	normalizedStatus := strings.ToLower(strings.TrimSpace(status))
	if normalizedStatus == "" {
		return false
	}
	for _, candidate := range viewer.EditableStatuses {
		if normalizedStatus == strings.ToLower(strings.TrimSpace(candidate)) {
			return true
		}
	}
	return false
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}

func looksLikePromotionPlanPayload(payload map[string]any) bool {
	if len(payload) == 0 {
		return false
	}
	if strings.TrimSpace(stringValue(payload["campaign_kind"])) == "" {
		return false
	}
	if strings.TrimSpace(stringValue(payload["target_segment"])) == "" {
		return false
	}
	if strings.TrimSpace(stringValue(payload["replaced_campaign"])) == "" {
		return false
	}
	items, _ := payload["target_products"].([]any)
	if len(items) > 0 {
		return true
	}
	if items, _ := payload["target_products"].([]string); len(items) > 0 {
		return true
	}
	return false
}
