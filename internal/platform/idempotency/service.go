package idempotency

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"orbyte/internal/platform/shared"
)

type Service struct {
	repo Repository
	now  func() time.Time
}

func NewService() *Service {
	return NewServiceWithRepository(NewMemoryRepository())
}

func NewServiceWithRepository(repo Repository) *Service {
	return &Service{
		repo: repo,
		now:  func() time.Time { return time.Now().UTC() },
	}
}

func (s *Service) List() []Record {
	if s == nil || s.repo == nil {
		return nil
	}
	return s.repo.List()
}

func (s *Service) Execute(operation, key, actorID string, request any, fn func() (Outcome, error)) (Outcome, error) {
	if s == nil || s.repo == nil || strings.TrimSpace(key) == "" {
		return fn()
	}
	operation = strings.TrimSpace(operation)
	key = strings.TrimSpace(key)
	requestHash, err := hashRequest(request)
	if err != nil {
		return Outcome{}, err
	}
	if existing, ok := s.repo.Get(operation, key); ok {
		if existing.RequestHash != requestHash {
			return Outcome{}, shared.Conflict("idempotency key was already used with a different request")
		}
		return Outcome{
			StatusCode: existing.ResponseCode,
			Response:   cloneMap(existing.Response),
		}, nil
	}
	outcome, err := fn()
	record := Record{
		Key:          key,
		Operation:    operation,
		ActorID:      strings.TrimSpace(actorID),
		RequestHash:  requestHash,
		Status:       "succeeded",
		ResponseCode: outcome.StatusCode,
		Response:     cloneMap(outcome.Response),
		CreatedAt:    s.now(),
		UpdatedAt:    s.now(),
	}
	if err != nil {
		record.Status = "failed"
		record.Error = err.Error()
		record.ResponseCode = httpStatusForError(err)
		record.Response = map[string]any{"error": err.Error()}
		if saveErr := s.repo.Save(record); saveErr != nil {
			return Outcome{}, saveErr
		}
		return Outcome{}, err
	}
	if outcome.StatusCode == 0 {
		record.ResponseCode = 200
	}
	if saveErr := s.repo.Save(record); saveErr != nil {
		return Outcome{}, saveErr
	}
	return outcome, nil
}

func hashRequest(request any) (string, error) {
	encoded, err := json.Marshal(request)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func httpStatusForError(err error) int {
	var platformErr shared.Error
	if errors.As(err, &platformErr) {
		switch platformErr.Kind {
		case shared.KindValidation:
			return 400
		case shared.KindUnauthorized:
			return 401
		case shared.KindForbidden:
			return 403
		case shared.KindNotFound:
			return 404
		case shared.KindConflict:
			return 409
		default:
			return 500
		}
	}
	return 500
}

func cloneMap(source map[string]any) map[string]any {
	if source == nil {
		return nil
	}
	target := make(map[string]any, len(source))
	for key, value := range source {
		target[key] = value
	}
	return target
}
