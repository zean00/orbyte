package idempotency

type Repository interface {
	Get(operation, key string) (Record, bool)
	List() []Record
	Save(record Record) error
}
