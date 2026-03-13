package securityfields

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"orbyte/internal/platform/document"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/policy"
	"orbyte/internal/platform/shared"
)

type PermissionChecker func(permissionKey string) bool

type AccessContext struct {
	ActorID           string
	SessionID         string
	OrganizationID    string
	LocationID        string
	ScopeID           string
	Channel           string
	State             string
	PermissionChecker PermissionChecker
}

type FieldAccess struct {
	Visible       bool   `json:"visible"`
	Editable      bool   `json:"editable"`
	Mask          string `json:"mask,omitempty"`
	SearchVisible bool   `json:"search_visible"`
	ExportVisible bool   `json:"export_visible"`
}

type AccessProfile struct {
	ResourceKind string                 `json:"resource_kind"`
	ResourceKey  string                 `json:"resource_key"`
	Fields       map[string]FieldAccess `json:"fields"`
}

type DocumentProfile struct {
	ResourceKind string                 `json:"resource_kind"`
	ResourceKey  string                 `json:"resource_key"`
	Fields       map[string]FieldAccess `json:"fields"`
}

type Service struct {
	policy *policy.Service
	ttl    time.Duration

	mu       sync.RWMutex
	profiles map[string]cachedProfile
}

type cachedProfile struct {
	profile   AccessProfile
	expiresAt time.Time
}

func NewService(policySvc *policy.Service) *Service {
	return &Service{
		policy:   policySvc,
		ttl:      time.Minute,
		profiles: map[string]cachedProfile{},
	}
}

func (s *Service) ModelProfile(ctx AccessContext, def model.Definition) AccessProfile {
	if s == nil {
		return defaultModelProfile(ctx, def)
	}
	key := cacheKey(ctx, def)
	if key != "" {
		if profile, ok := s.cachedProfile(key); ok {
			return profile
		}
	}
	profile := defaultModelProfile(ctx, def)
	if s.policy != nil && hasHook(s.policy, "models.fields.profile") {
		decision := s.policy.Evaluate(policy.Request{
			HookKey:        "models.fields.profile",
			ActorID:        strings.TrimSpace(ctx.ActorID),
			OrganizationID: strings.TrimSpace(ctx.OrganizationID),
			LocationID:     strings.TrimSpace(ctx.LocationID),
			ScopeID:        firstNonEmpty(strings.TrimSpace(ctx.ScopeID), strings.TrimSpace(ctx.LocationID)),
			Inputs: map[string]any{
				"model_key":   def.Key,
				"channel":     firstNonEmpty(strings.TrimSpace(ctx.Channel), "api"),
				"state":       strings.TrimSpace(ctx.State),
				"field_keys":  fieldKeys(def.Fields),
				"field_count": len(def.Fields),
			},
		})
		if fields, ok := decision.Output["fields"].(map[string]any); ok {
			for key, raw := range fields {
				current := profile.Fields[key]
				mergeFieldAccess(&current, raw)
				profile.Fields[key] = current
			}
		}
	}
	if key != "" {
		s.storeProfile(key, profile)
	}
	return profile
}

func (s *Service) DocumentProfile(ctx AccessContext, record document.Record) DocumentProfile {
	if s == nil {
		return DocumentProfile{ResourceKind: "document", ResourceKey: record.Header.Type, Fields: map[string]FieldAccess{}}
	}
	key := documentCacheKey(ctx, record)
	if key != "" {
		if profile, ok := s.cachedDocumentProfile(key); ok {
			return profile
		}
	}
	profile := DocumentProfile{
		ResourceKind: "document",
		ResourceKey:  record.Header.Type,
		Fields:       map[string]FieldAccess{},
	}
	if s.policy != nil && hasHook(s.policy, "documents.fields.profile") {
		decision := s.policy.Evaluate(policy.Request{
			HookKey:        "documents.fields.profile",
			ActorID:        strings.TrimSpace(ctx.ActorID),
			OrganizationID: firstNonEmpty(strings.TrimSpace(ctx.OrganizationID), strings.TrimSpace(record.Header.OrganizationID)),
			LocationID:     firstNonEmpty(strings.TrimSpace(ctx.LocationID), strings.TrimSpace(record.Header.LocationID)),
			ScopeID:        firstNonEmpty(strings.TrimSpace(ctx.ScopeID), strings.TrimSpace(record.Header.LocationID)),
			Inputs: map[string]any{
				"document_id":     record.Header.ID,
				"document_type":   record.Header.Type,
				"document_status": record.Header.Status,
				"channel":         firstNonEmpty(strings.TrimSpace(ctx.Channel), "api"),
			},
		})
		if fields, ok := decision.Output["fields"].(map[string]any); ok {
			for key, raw := range fields {
				current := profile.Fields[key]
				mergeFieldAccess(&current, raw)
				profile.Fields[key] = current
			}
		}
	}
	if key != "" {
		s.storeDocumentProfile(key, profile)
	}
	return profile
}

func (s *Service) SanitizeModelRecord(profile AccessProfile, record model.Record) model.Record {
	record.Values = sanitizeValues(profile, record.Values)
	return record
}

func (s *Service) SanitizeModelRecords(profile AccessProfile, records []model.Record) []model.Record {
	items := make([]model.Record, 0, len(records))
	for _, record := range records {
		items = append(items, s.SanitizeModelRecord(profile, record))
	}
	return items
}

func (s *Service) ValidateModelWrite(profile AccessProfile, values map[string]any, def model.Definition) error {
	for _, field := range def.Fields {
		if _, ok := values[field.Key]; !ok {
			continue
		}
		if field.ReadOnly {
			return shared.Forbidden("field is read only: " + field.Key)
		}
		access, ok := profile.Fields[field.Key]
		if ok && !access.Editable {
			return shared.Forbidden("field update is not allowed: " + field.Key)
		}
	}
	return nil
}

func (s *Service) SanitizeDocumentPayload(profile DocumentProfile, payload map[string]any) map[string]any {
	return sanitizeDocumentPayload(profile, payload, "")
}

func (s *Service) ValidateDocumentWrite(profile DocumentProfile, payload map[string]any, prefix string) error {
	for _, path := range payloadPaths(payload, prefix) {
		access, ok := profile.Fields[path]
		if ok && !access.Editable {
			return shared.Forbidden("field update is not allowed: " + path)
		}
	}
	return nil
}

func sanitizeValues(profile AccessProfile, values map[string]any) map[string]any {
	if len(values) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(values))
	for key, value := range values {
		access, ok := profile.Fields[key]
		if ok && !access.Visible {
			continue
		}
		if ok && access.Mask != "" {
			out[key] = ApplyMask(access.Mask, value)
			continue
		}
		out[key] = value
	}
	return out
}

func defaultModelProfile(ctx AccessContext, def model.Definition) AccessProfile {
	profile := AccessProfile{
		ResourceKind: "model",
		ResourceKey:  def.Key,
		Fields:       map[string]FieldAccess{},
	}
	for _, field := range def.Fields {
		visible := true
		editable := !field.ReadOnly
		if field.ReadPermissionKey != "" && !allows(ctx.PermissionChecker, field.ReadPermissionKey) {
			visible = false
		}
		if field.WritePermissionKey != "" && !allows(ctx.PermissionChecker, field.WritePermissionKey) {
			editable = false
		}
		searchVisible := true
		if field.SearchVisible != nil {
			searchVisible = *field.SearchVisible
		} else if field.Sensitive {
			searchVisible = false
		}
		exportVisible := true
		if field.ExportVisible != nil {
			exportVisible = *field.ExportVisible
		} else if field.Sensitive {
			exportVisible = false
		}
		mask := strings.TrimSpace(field.DefaultMask)
		if !visible {
			mask = ""
		}
		profile.Fields[field.Key] = FieldAccess{
			Visible:       visible,
			Editable:      editable,
			Mask:          mask,
			SearchVisible: searchVisible,
			ExportVisible: exportVisible,
		}
	}
	return profile
}

func mergeFieldAccess(access *FieldAccess, raw any) {
	entry, ok := raw.(map[string]any)
	if !ok {
		return
	}
	if value, ok := entry["visible"]; ok {
		access.Visible = boolValue(value, access.Visible)
	}
	if value, ok := entry["editable"]; ok {
		access.Editable = boolValue(value, access.Editable)
	}
	if value, ok := entry["search_visible"]; ok {
		access.SearchVisible = boolValue(value, access.SearchVisible)
	}
	if value, ok := entry["export_visible"]; ok {
		access.ExportVisible = boolValue(value, access.ExportVisible)
	}
	if value, ok := entry["mask"]; ok {
		text := strings.TrimSpace(fmt.Sprintf("%v", value))
		if text != "" {
			access.Mask = text
		}
	}
}

func allows(check PermissionChecker, permissionKey string) bool {
	if check == nil {
		return true
	}
	return check(strings.TrimSpace(permissionKey))
}

func hasHook(policySvc *policy.Service, hookKey string) bool {
	for _, def := range policySvc.Definitions() {
		if def.Key == hookKey {
			return true
		}
	}
	return false
}

func fieldKeys(fields []model.FieldDefinition) []string {
	keys := make([]string, 0, len(fields))
	for _, field := range fields {
		keys = append(keys, field.Key)
	}
	return keys
}

func boolValue(value any, fallback bool) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	default:
		return fallback
	}
}

func cacheKey(ctx AccessContext, def model.Definition) string {
	channel := strings.TrimSpace(ctx.Channel)
	if channel == "" {
		channel = "api"
	}
	actor := firstNonEmpty(strings.TrimSpace(ctx.SessionID), strings.TrimSpace(ctx.ActorID))
	if actor == "" {
		return ""
	}
	return strings.Join([]string{
		actor,
		strings.TrimSpace(ctx.OrganizationID),
		strings.TrimSpace(ctx.LocationID),
		channel,
		strings.TrimSpace(ctx.State),
		def.Key,
	}, "|")
}

func documentCacheKey(ctx AccessContext, record document.Record) string {
	channel := strings.TrimSpace(ctx.Channel)
	if channel == "" {
		channel = "api"
	}
	actor := firstNonEmpty(strings.TrimSpace(ctx.SessionID), strings.TrimSpace(ctx.ActorID), "system")
	return strings.Join([]string{
		actor,
		firstNonEmpty(strings.TrimSpace(ctx.OrganizationID), strings.TrimSpace(record.Header.OrganizationID)),
		firstNonEmpty(strings.TrimSpace(ctx.LocationID), strings.TrimSpace(record.Header.LocationID)),
		channel,
		strings.TrimSpace(record.Header.Status),
		record.Header.Type,
		record.Header.ID,
	}, "|")
}

func (s *Service) cachedProfile(key string) (AccessProfile, bool) {
	now := time.Now().UTC()
	s.mu.RLock()
	entry, ok := s.profiles[key]
	s.mu.RUnlock()
	if !ok || now.After(entry.expiresAt) {
		if ok {
			s.mu.Lock()
			delete(s.profiles, key)
			s.mu.Unlock()
		}
		return AccessProfile{}, false
	}
	return entry.profile, true
}

func (s *Service) storeProfile(key string, profile AccessProfile) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.profiles[key] = cachedProfile{
		profile:   profile,
		expiresAt: time.Now().UTC().Add(s.ttl),
	}
}

func (s *Service) cachedDocumentProfile(key string) (DocumentProfile, bool) {
	now := time.Now().UTC()
	s.mu.RLock()
	entry, ok := s.profiles[key]
	s.mu.RUnlock()
	if !ok || now.After(entry.expiresAt) {
		if ok {
			s.mu.Lock()
			delete(s.profiles, key)
			s.mu.Unlock()
		}
		return DocumentProfile{}, false
	}
	return DocumentProfile{
		ResourceKind: entry.profile.ResourceKind,
		ResourceKey:  entry.profile.ResourceKey,
		Fields:       entry.profile.Fields,
	}, true
}

func (s *Service) storeDocumentProfile(key string, profile DocumentProfile) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.profiles[key] = cachedProfile{
		profile: AccessProfile{
			ResourceKind: profile.ResourceKind,
			ResourceKey:  profile.ResourceKey,
			Fields:       profile.Fields,
		},
		expiresAt: time.Now().UTC().Add(s.ttl),
	}
}

func sanitizeDocumentPayload(profile DocumentProfile, payload map[string]any, prefix string) map[string]any {
	if len(payload) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(payload))
	for key, value := range payload {
		path := joinPath(prefix, key)
		access, hasAccess := profile.Fields[path]
		if hasAccess && !access.Visible {
			continue
		}
		if typed, ok := value.(map[string]any); ok {
			child := sanitizeDocumentPayload(profile, typed, path)
			if len(child) == 0 && hasAccess && !access.Visible {
				continue
			}
			out[key] = child
			continue
		}
		if hasAccess && access.Mask != "" {
			out[key] = ApplyMask(access.Mask, value)
			continue
		}
		out[key] = value
	}
	return out
}

func payloadPaths(payload map[string]any, prefix string) []string {
	paths := make([]string, 0)
	for key, value := range payload {
		path := joinPath(prefix, key)
		paths = append(paths, path)
		if typed, ok := value.(map[string]any); ok {
			paths = append(paths, payloadPaths(typed, path)...)
		}
	}
	return paths
}

func joinPath(prefix, key string) string {
	if strings.TrimSpace(prefix) == "" {
		return strings.TrimSpace(key)
	}
	if strings.TrimSpace(key) == "" {
		return strings.TrimSpace(prefix)
	}
	return strings.TrimSpace(prefix) + "." + strings.TrimSpace(key)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
