package model

type Repository interface {
	SaveDefinition(def Definition) error
	ListDefinitions() []Definition
	GetDefinition(key string) (Definition, bool)
	SaveRecord(record Record) error
	DeleteRecord(modelKey, id string) error
	GetRecord(modelKey, id string) (Record, bool)
	ListRecords(modelKey string) []Record
}
