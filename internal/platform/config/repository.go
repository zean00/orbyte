package config

type Repository interface {
	Get(key, scope, scopeID string) (Entry, bool)
	List() []Entry
	Save(entry Entry) error
}
