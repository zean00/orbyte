package document

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"orbyte/internal/platform/shared"
)

type Service struct {
	repo                  Repository
	specializedViewers    map[string]SpecializedViewer
	specializedFallback   map[string]string
	nativeDocumentViewers map[string]NativeDocumentViewer
}

func NewService() *Service {
	svc := NewServiceWithRepository(NewMemoryRepository())
	_ = svc.Register(Definition{
		Type:                   "generic_request",
		DisplayName:            "Generic Request",
		SchemaVersion:          "v1",
		WorkflowKey:            "generic_request_flow",
		NumberingKey:           "generic_request_number",
		AllowedLinkTypes:       []string{"related_to", "amends"},
		AllowedAttachmentTypes: []string{"note", "image", "document"},
	})
	return svc
}

func NewServiceWithRepository(repo Repository) *Service {
	return &Service{
		repo:                  repo,
		specializedViewers:    map[string]SpecializedViewer{},
		specializedFallback:   map[string]string{},
		nativeDocumentViewers: map[string]NativeDocumentViewer{},
	}
}

func (s *Service) Register(def Definition) error {
	if def.Type == "" {
		return shared.Validation("document type is required")
	}
	return s.repo.SaveDefinition(def)
}

func (s *Service) RegisterExtension(def ExtensionDefinition) error {
	if def.DocumentType == "" {
		return shared.Validation("document type is required")
	}
	if def.ModuleKey == "" {
		return shared.Validation("module key is required")
	}
	if _, ok := s.repo.GetDefinition(def.DocumentType); !ok {
		return shared.NotFound("document definition not found")
	}
	return s.repo.SaveExtensionDefinition(def)
}

func (s *Service) DocumentTypes() []string {
	defs := s.repo.ListDefinitions()
	keys := make([]string, 0, len(defs))
	for _, def := range defs {
		keys = append(keys, def.Type)
	}
	return keys
}

func (s *Service) Definitions() []Definition {
	defs := s.repo.ListDefinitions()
	sort.Slice(defs, func(i, j int) bool { return defs[i].Type < defs[j].Type })
	return defs
}

func (s *Service) List() []Record {
	return s.repo.ListRecords()
}

func (s *Service) Get(documentID string) (Record, error) {
	record, ok := s.repo.GetRecord(documentID)
	if !ok {
		return Record{}, shared.NotFound("document not found")
	}
	return record, nil
}

func (s *Service) Definition(documentType string) (Definition, error) {
	def, ok := s.repo.GetDefinition(documentType)
	if !ok {
		return Definition{}, shared.NotFound("document definition not found")
	}
	return def, nil
}

func (s *Service) ExtensionDefinitions(documentType string) []ExtensionDefinition {
	return s.repo.ListExtensionDefinitions(documentType)
}

func (s *Service) Save(record Record) error {
	return s.repo.SaveRecord(record)
}

func (s *Service) Delete(documentID string) error {
	if strings.TrimSpace(documentID) == "" {
		return shared.Validation("document id is required")
	}
	return s.repo.DeleteRecord(documentID)
}

func (s *Service) Create(documentType, organizationID, locationID, actorID string, payload map[string]any) (Record, error) {
	def, ok := s.repo.GetDefinition(documentType)
	if !ok {
		return Record{}, shared.NotFound("document definition not found")
	}
	now := time.Now().UTC()
	id := shared.NewID("doc_")
	normalizedPayload := NormalizePayload(payload)
	body := Body{
		DocumentID:    id,
		SchemaVersion: def.SchemaVersion,
		Payload:       normalizedPayload,
		ContentHash:   ContentHash(normalizedPayload),
	}
	record := Record{
		Header: Header{
			ID:             id,
			Type:           documentType,
			Status:         "draft",
			Version:        1,
			ETag:           fmt.Sprintf("%s:%d", id, 1),
			OrganizationID: organizationID,
			LocationID:     locationID,
			CreatedBy:      actorID,
			CreatedAt:      now,
			UpdatedBy:      actorID,
			UpdatedAt:      now,
			TotalAmount:    shared.Money{Currency: "IDR"},
		},
		Body: body,
	}
	if err := s.repo.SaveRecord(record); err != nil {
		return Record{}, err
	}
	return record, nil
}

func (s *Service) ReplaceExtension(documentID, moduleKey string, extensionPayload map[string]any) (Record, error) {
	record, err := s.Get(documentID)
	if err != nil {
		return Record{}, err
	}
	if !hasExtensionDefinition(s.repo.ListExtensionDefinitions(record.Header.Type), moduleKey) {
		return Record{}, shared.Validation("document extension is not registered")
	}
	record.Body.Payload = SetExtensionPayload(record.Body.Payload, moduleKey, extensionPayload)
	record.Body.ContentHash = ContentHash(record.Body.Payload)
	if err := s.repo.SaveRecord(record); err != nil {
		return Record{}, err
	}
	return record, nil
}

func (s *Service) Render(record Record, mode ViewMode, enabledModules map[string]bool) Record {
	record.Body.Payload = PayloadForView(record.Body.Payload, mode, enabledModules)
	return record
}

func (s *Service) ReplaceLines(documentID string, lines []Line) error {
	record, err := s.Get(documentID)
	if err != nil {
		return err
	}
	for i := range lines {
		if lines[i].ID == "" {
			lines[i].ID = fmt.Sprintf("line:%s:%d", documentID, i+1)
		}
		lines[i].DocumentID = documentID
		if lines[i].LineNo == 0 {
			lines[i].LineNo = i + 1
		}
		if err := lines[i].Amount.Validate(); err != nil {
			return err
		}
	}
	if err := s.repo.SaveLines(documentID, lines); err != nil {
		return err
	}
	record.Lines = append([]Line(nil), lines...)
	return s.repo.SaveRecord(record)
}

func (s *Service) AddLink(documentID, linkedDocumentID, linkType string, metadata map[string]any) (Link, error) {
	record, err := s.Get(documentID)
	if err != nil {
		return Link{}, err
	}
	def, err := s.Definition(record.Header.Type)
	if err != nil {
		return Link{}, err
	}
	if !contains(def.AllowedLinkTypes, linkType) {
		return Link{}, shared.Validation("link type is not allowed for document definition")
	}
	link := Link{
		ID:               shared.ChildID("link", documentID),
		DocumentID:       documentID,
		LinkedDocumentID: linkedDocumentID,
		LinkType:         linkType,
		Metadata:         metadata,
		CreatedAt:        time.Now().UTC(),
	}
	links := s.repo.ListLinks(documentID)
	links = append(links, link)
	if err := s.repo.SaveLinks(documentID, links); err != nil {
		return Link{}, err
	}
	record.Links = append(record.Links, link)
	return link, s.repo.SaveRecord(record)
}

func (s *Service) AddAttachment(documentID, attachmentType, fileName, contentType, storageKey string, sizeBytes int64) (Attachment, error) {
	record, err := s.Get(documentID)
	if err != nil {
		return Attachment{}, err
	}
	def, err := s.Definition(record.Header.Type)
	if err != nil {
		return Attachment{}, err
	}
	if !contains(def.AllowedAttachmentTypes, attachmentType) {
		return Attachment{}, shared.Validation("attachment type is not allowed for document definition")
	}
	attachment := Attachment{
		ID:             shared.ChildID("attachment", documentID),
		DocumentID:     documentID,
		AttachmentType: attachmentType,
		FileName:       fileName,
		ContentType:    contentType,
		StorageKey:     storageKey,
		SizeBytes:      sizeBytes,
		CreatedAt:      time.Now().UTC(),
	}
	attachments := s.repo.ListAttachments(documentID)
	attachments = append(attachments, attachment)
	if err := s.repo.SaveAttachments(documentID, attachments); err != nil {
		return Attachment{}, err
	}
	record.Attachments = append(record.Attachments, attachment)
	return attachment, s.repo.SaveRecord(record)
}

func (s *Service) RemoveLink(documentID, linkID string) error {
	record, err := s.Get(documentID)
	if err != nil {
		return err
	}
	links := s.repo.ListLinks(documentID)
	filtered := make([]Link, 0, len(links))
	found := false
	for _, link := range links {
		if link.ID == linkID {
			found = true
			continue
		}
		filtered = append(filtered, link)
	}
	if !found {
		return shared.NotFound("document link not found")
	}
	if err := s.repo.SaveLinks(documentID, filtered); err != nil {
		return err
	}
	record.Links = filtered
	return s.repo.SaveRecord(record)
}

func (s *Service) RemoveAttachment(documentID, attachmentID string) error {
	record, err := s.Get(documentID)
	if err != nil {
		return err
	}
	attachments := s.repo.ListAttachments(documentID)
	filtered := make([]Attachment, 0, len(attachments))
	found := false
	for _, attachment := range attachments {
		if attachment.ID == attachmentID {
			found = true
			continue
		}
		filtered = append(filtered, attachment)
	}
	if !found {
		return shared.NotFound("document attachment not found")
	}
	if err := s.repo.SaveAttachments(documentID, filtered); err != nil {
		return err
	}
	record.Attachments = filtered
	return s.repo.SaveRecord(record)
}

func contains(values []string, candidate string) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
}

func ContentHash(payload map[string]any) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%v", payload)))
	return hex.EncodeToString(h[:])
}

func NormalizePayload(payload map[string]any) map[string]any {
	normalized := cloneMap(payload)
	if normalized == nil {
		normalized = map[string]any{}
	}
	normalized = normalizePayloadMap(normalized)
	extensions := extensionMap(normalized)
	normalized["extensions"] = extensions
	return normalized
}

func SetExtensionPayload(payload map[string]any, moduleKey string, extensionPayload map[string]any) map[string]any {
	normalized := NormalizePayload(payload)
	extensions := extensionMap(normalized)
	extensions[moduleKey] = cloneMap(extensionPayload)
	normalized["extensions"] = extensions
	return normalized
}

func PayloadForView(payload map[string]any, mode ViewMode, enabledModules map[string]bool) map[string]any {
	normalized := NormalizePayload(payload)
	switch mode {
	case ViewRaw:
		return normalized
	case ViewExpanded:
		base := cloneMap(normalized)
		base["extensions"] = filterExtensions(extensionMap(normalized), enabledModules)
		return base
	default:
		base := cloneMap(normalized)
		delete(base, "extensions")
		return base
	}
}

func ExtensionPayload(payload map[string]any, moduleKey string) map[string]any {
	extensions := extensionMap(payload)
	if current, ok := extensions[moduleKey].(map[string]any); ok {
		return cloneMap(current)
	}
	return nil
}

func cloneMap(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	output := make(map[string]any, len(input))
	for key, value := range input {
		output[key] = deepClone(value)
	}
	return output
}

func deepClone(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneMap(typed)
	case []any:
		items := make([]any, len(typed))
		for i, item := range typed {
			items[i] = deepClone(item)
		}
		return items
	case []map[string]any:
		items := make([]map[string]any, len(typed))
		for i, item := range typed {
			items[i] = cloneMap(item)
		}
		return items
	default:
		return typed
	}
}

func normalizePayloadMap(input map[string]any) map[string]any {
	output := make(map[string]any, len(input))
	for key, value := range input {
		normalizedValue := normalizePayloadValue(value)
		if isLineCollectionKey(key) {
			normalizedValue = normalizeLineCollectionValue(normalizedValue)
		}
		output[key] = normalizedValue
	}
	return output
}

func normalizePayloadValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return normalizePayloadMap(cloneMap(typed))
	case []any:
		items := make([]any, len(typed))
		for i, item := range typed {
			items[i] = normalizePayloadValue(item)
		}
		return items
	case []map[string]any:
		items := make([]map[string]any, len(typed))
		for i, item := range typed {
			items[i] = normalizePayloadMap(cloneMap(item))
		}
		return items
	default:
		return typed
	}
}

func isLineCollectionKey(key string) bool {
	key = strings.TrimSpace(strings.ToLower(key))
	return key == "lines" || strings.HasSuffix(key, "_lines")
}

func normalizeLineCollectionValue(value any) any {
	switch typed := value.(type) {
	case []any:
		items := make([]any, len(typed))
		for i, item := range typed {
			row, ok := item.(map[string]any)
			if !ok {
				items[i] = item
				continue
			}
			items[i] = ensurePayloadLineID(row, i)
		}
		return items
	case []map[string]any:
		items := make([]map[string]any, len(typed))
		for i, item := range typed {
			items[i] = ensurePayloadLineID(item, i)
		}
		return items
	default:
		return value
	}
}

func ensurePayloadLineID(line map[string]any, index int) map[string]any {
	next := cloneMap(line)
	if next == nil {
		next = map[string]any{}
	}
	if lineID := strings.TrimSpace(textValue(next["line_id"])); lineID != "" {
		next["line_id"] = lineID
		return next
	}
	if genericID := strings.TrimSpace(textValue(next["id"])); genericID != "" {
		next["line_id"] = genericID
		return next
	}
	next["line_id"] = derivedPayloadLineID(next, index)
	return next
}

func derivedPayloadLineID(line map[string]any, index int) string {
	scrubbed := cloneMap(line)
	delete(scrubbed, "line_id")
	payload, err := json.Marshal(scrubbed)
	if err != nil {
		payload = []byte(fmt.Sprintf("%v", scrubbed))
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("%d:%s", index, payload)))
	return "line_" + hex.EncodeToString(sum[:8])
}

func textValue(value any) string {
	if value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return fmt.Sprintf("%v", value)
	}
}

func extensionMap(payload map[string]any) map[string]any {
	if payload == nil {
		return map[string]any{}
	}
	current, ok := payload["extensions"].(map[string]any)
	if ok {
		return cloneMap(current)
	}
	return map[string]any{}
}

func filterExtensions(extensions map[string]any, enabledModules map[string]bool) map[string]any {
	if len(extensions) == 0 {
		return map[string]any{}
	}
	filtered := make(map[string]any)
	for moduleKey, value := range extensions {
		if enabledModules == nil || enabledModules[moduleKey] {
			filtered[moduleKey] = deepClone(value)
		}
	}
	return filtered
}

func hasExtensionDefinition(defs []ExtensionDefinition, moduleKey string) bool {
	for _, def := range defs {
		if def.ModuleKey == moduleKey {
			return true
		}
	}
	return false
}

func SortedExtensionModuleKeys(defs []ExtensionDefinition) []string {
	keys := make([]string, 0, len(defs))
	for _, def := range defs {
		keys = append(keys, def.ModuleKey)
	}
	sort.Strings(keys)
	return keys
}
