package secretstore

type Repository interface {
	Get(ref string) (Secret, bool)
	List() []Secret
	Save(secret Secret) error
}
