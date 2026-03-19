package secretstore

import (
	"fmt"
	"strings"
	"time"
)

type Service struct {
	repo Repository
}

func NewService() *Service {
	return NewServiceWithRepository(NewMemoryRepository())
}

func NewServiceWithRepository(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) List() []Secret {
	if s == nil || s.repo == nil {
		return nil
	}
	return s.repo.List()
}

func (s *Service) Upsert(name, ref, value string) (Secret, error) {
	now := time.Now().UTC()
	ref = strings.TrimSpace(ref)
	if ref == "" {
		ref = fmt.Sprintf("secret:%d", now.UnixNano())
	}
	secret := Secret{
		Ref:       ref,
		Name:      strings.TrimSpace(name),
		Value:     value,
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if existing, ok := s.repo.Get(ref); ok {
		secret.CreatedAt = existing.CreatedAt
	}
	return secret, s.repo.Save(secret)
}

func (s *Service) Resolve(ref string) (string, bool) {
	if s == nil || s.repo == nil {
		return "", false
	}
	secret, ok := s.repo.Get(strings.TrimSpace(ref))
	if !ok || secret.Status != "active" {
		return "", false
	}
	return secret.Value, true
}
