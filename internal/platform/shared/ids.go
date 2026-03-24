package shared

import (
	"crypto/rand"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
)

type IDService struct {
	mu      sync.Mutex
	now     func() time.Time
	entropy io.Reader
}

var defaultIDService = NewIDService()

func DefaultIDService() *IDService {
	return defaultIDService
}

func NewIDService() *IDService {
	return &IDService{
		now:     func() time.Time { return time.Now().UTC() },
		entropy: ulid.Monotonic(rand.Reader, 0),
	}
}

func NewIDServiceWithEntropy(now func() time.Time, entropy io.Reader) *IDService {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	if entropy == nil {
		entropy = ulid.Monotonic(rand.Reader, 0)
	}
	return &IDService{now: now, entropy: entropy}
}

func NewID(prefix string) string {
	return defaultIDService.New(prefix)
}

func ChildID(prefix string, parts ...string) string {
	return defaultIDService.Child(prefix, parts...)
}

func (s *IDService) New(prefix string) string {
	id := s.ulid()
	prefix = strings.TrimSpace(prefix)
	switch {
	case prefix == "":
		return id
	case strings.HasSuffix(prefix, "_"):
		return prefix + id
	default:
		return strings.TrimRight(prefix, ":") + ":" + id
	}
}

func (s *IDService) Child(prefix string, parts ...string) string {
	items := make([]string, 0, len(parts)+2)
	prefix = strings.Trim(strings.TrimSpace(prefix), ":")
	if prefix != "" {
		items = append(items, prefix)
	}
	for _, part := range parts {
		part = strings.Trim(strings.TrimSpace(part), ":")
		if part != "" {
			items = append(items, part)
		}
	}
	items = append(items, s.ulid())
	return strings.Join(items, ":")
}

func (s *IDService) ulid() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, err := ulid.New(ulid.Timestamp(s.now()), s.entropy)
	if err != nil {
		panic(fmt.Errorf("generate ulid: %w", err))
	}
	return value.String()
}
