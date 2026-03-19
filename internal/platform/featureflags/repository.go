package featureflags

type Repository interface {
	SaveDefinition(def Definition) error
	GetDefinition(key string) (Definition, bool)
	ListDefinitions() []Definition
	SaveValue(value Value) error
	GetValue(flagKey, scope, scopeID string) (Value, bool)
	ListValues() []Value
}
