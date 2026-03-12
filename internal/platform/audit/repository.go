package audit

type Repository interface {
	Save(event Event) error
	List() []Event
}
