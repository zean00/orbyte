package module

type Repository interface {
	Get(key string) (InstalledModule, bool)
	List() []InstalledModule
	Save(item InstalledModule) error
	GetActivation(baseModuleKey, scope, scopeID string) (LocalExtensionActivation, bool)
	ListActivations() []LocalExtensionActivation
	SaveActivation(item LocalExtensionActivation) error
	DeleteActivation(baseModuleKey, scope, scopeID string) error
}
