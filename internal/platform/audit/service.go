package audit

type Service struct {
	repo Repository
}

func NewService() *Service {
	return NewServiceWithRepository(NewMemoryRepository())
}

func NewServiceWithRepository(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Record(event Event) error {
	return s.repo.Save(event)
}

func (s *Service) List() []Event {
	return s.repo.List()
}

func (s *Service) Query(filter Query) []Event {
	if s == nil || s.repo == nil {
		return nil
	}
	return s.repo.Query(filter)
}
