package workflow

func (s *Service) ListKeys() []string {
	defs := s.repo.ListDefinitions()
	keys := make([]string, 0, len(defs))
	for _, def := range defs {
		keys = append(keys, def.Key)
	}
	return keys
}
