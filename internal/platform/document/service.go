package document

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"time"

	"orbyte/internal/platform/shared"
)

type Service struct {
	repo Repository
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
	return &Service{repo: repo}
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

func (s *Service) Create(documentType, organizationID, locationID, actorID string, payload map[string]any) (Record, error) {
	def, ok := s.repo.GetDefinition(documentType)
	if !ok {
		return Record{}, shared.NotFound("document definition not found")
	}
	now := time.Now().UTC()
	id := fmt.Sprintf("doc_%d", now.UnixNano())
	body := Body{
		DocumentID:    id,
		SchemaVersion: def.SchemaVersion,
		Payload:       NormalizePayload(payload),
		ContentHash:   ContentHash(NormalizePayload(payload)),
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
		ID:               fmt.Sprintf("link:%s:%d", documentID, time.Now().UTC().UnixNano()),
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
		ID:             fmt.Sprintf("attachment:%s:%d", documentID, time.Now().UTC().UnixNano()),
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
	default:
		return typed
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
