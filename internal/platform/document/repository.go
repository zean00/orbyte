package document

type Repository interface {
	SaveDefinition(def Definition) error
	GetDefinition(documentType string) (Definition, bool)
	ListDefinitions() []Definition
	SaveExtensionDefinition(def ExtensionDefinition) error
	ListExtensionDefinitions(documentType string) []ExtensionDefinition
	SaveRecord(record Record) error
	GetRecord(documentID string) (Record, bool)
	ListRecords() []Record
	DeleteRecord(documentID string) error
	SaveLines(documentID string, lines []Line) error
	ListLines(documentID string) []Line
	SaveLinks(documentID string, links []Link) error
	ListLinks(documentID string) []Link
	SaveAttachments(documentID string, attachments []Attachment) error
	ListAttachments(documentID string) []Attachment
}
