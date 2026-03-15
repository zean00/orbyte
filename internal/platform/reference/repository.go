package reference

type Repository interface {
	SaveType(TypeDefinition) error
	GetType(string) (TypeDefinition, bool)
	ListTypes() []TypeDefinition
	SaveRecord(Record) error
	ListRecords(typeKey string) []Record
}
