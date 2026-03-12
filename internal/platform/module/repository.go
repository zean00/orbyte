package module

type Repository interface {
	Get(key string) (InstalledModule, bool)
	List() []InstalledModule
	Save(item InstalledModule) error
}
