package document

import (
	"sort"

	"orbyte/internal/platform/shared"
)

type MemoryRepository struct {
	definitions map[string]Definition
	extensions  map[string]map[string]ExtensionDefinition
	records     map[string]Record
	lines       map[string][]Line
	links       map[string][]Link
	attachments map[string][]Attachment
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		definitions: map[string]Definition{},
		extensions:  map[string]map[string]ExtensionDefinition{},
		records:     map[string]Record{},
		lines:       map[string][]Line{},
		links:       map[string][]Link{},
		attachments: map[string][]Attachment{},
	}
}

func (r *MemoryRepository) SaveDefinition(def Definition) error {
	if _, exists := r.definitions[def.Type]; exists {
		return shared.Conflict("document definition already exists")
	}
	r.definitions[def.Type] = def
	return nil
}

func (r *MemoryRepository) GetDefinition(documentType string) (Definition, bool) {
	def, ok := r.definitions[documentType]
	return def, ok
}

func (r *MemoryRepository) ListDefinitions() []Definition {
	defs := make([]Definition, 0, len(r.definitions))
	for _, def := range r.definitions {
		defs = append(defs, def)
	}
	sort.Slice(defs, func(i, j int) bool {
		return defs[i].Type < defs[j].Type
	})
	return defs
}

func (r *MemoryRepository) SaveExtensionDefinition(def ExtensionDefinition) error {
	if r.extensions[def.DocumentType] == nil {
		r.extensions[def.DocumentType] = map[string]ExtensionDefinition{}
	}
	if _, exists := r.extensions[def.DocumentType][def.ModuleKey]; exists {
		return shared.Conflict("document extension definition already exists")
	}
	r.extensions[def.DocumentType][def.ModuleKey] = def
	return nil
}

func (r *MemoryRepository) ListExtensionDefinitions(documentType string) []ExtensionDefinition {
	items := make([]ExtensionDefinition, 0)
	for _, def := range r.extensions[documentType] {
		items = append(items, def)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ModuleKey < items[j].ModuleKey })
	return items
}

func (r *MemoryRepository) SaveRecord(record Record) error {
	record.Lines = append([]Line(nil), r.lines[record.Header.ID]...)
	record.Links = append([]Link(nil), r.links[record.Header.ID]...)
	record.Attachments = append([]Attachment(nil), r.attachments[record.Header.ID]...)
	r.records[record.Header.ID] = record
	return nil
}

func (r *MemoryRepository) GetRecord(documentID string) (Record, bool) {
	record, ok := r.records[documentID]
	if ok {
		record.Lines = append([]Line(nil), r.lines[documentID]...)
		record.Links = append([]Link(nil), r.links[documentID]...)
		record.Attachments = append([]Attachment(nil), r.attachments[documentID]...)
	}
	return record, ok
}

func (r *MemoryRepository) ListRecords() []Record {
	records := make([]Record, 0, len(r.records))
	for _, record := range r.records {
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].Header.CreatedAt.Before(records[j].Header.CreatedAt)
	})
	return records
}

func (r *MemoryRepository) DeleteRecord(documentID string) error {
	delete(r.records, documentID)
	delete(r.lines, documentID)
	delete(r.links, documentID)
	delete(r.attachments, documentID)
	return nil
}

func (r *MemoryRepository) SaveLines(documentID string, lines []Line) error {
	items := append([]Line(nil), lines...)
	sort.Slice(items, func(i, j int) bool { return items[i].LineNo < items[j].LineNo })
	r.lines[documentID] = items
	if record, ok := r.records[documentID]; ok {
		record.Lines = append([]Line(nil), items...)
		r.records[documentID] = record
	}
	return nil
}

func (r *MemoryRepository) ListLines(documentID string) []Line {
	return append([]Line(nil), r.lines[documentID]...)
}

func (r *MemoryRepository) SaveLinks(documentID string, links []Link) error {
	items := append([]Link(nil), links...)
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.Before(items[j].CreatedAt) })
	r.links[documentID] = items
	if record, ok := r.records[documentID]; ok {
		record.Links = append([]Link(nil), items...)
		r.records[documentID] = record
	}
	return nil
}

func (r *MemoryRepository) ListLinks(documentID string) []Link {
	return append([]Link(nil), r.links[documentID]...)
}

func (r *MemoryRepository) SaveAttachments(documentID string, attachments []Attachment) error {
	items := append([]Attachment(nil), attachments...)
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.Before(items[j].CreatedAt) })
	r.attachments[documentID] = items
	if record, ok := r.records[documentID]; ok {
		record.Attachments = append([]Attachment(nil), items...)
		r.records[documentID] = record
	}
	return nil
}

func (r *MemoryRepository) ListAttachments(documentID string) []Attachment {
	return append([]Attachment(nil), r.attachments[documentID]...)
}
