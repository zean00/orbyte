package mcp

import (
	"sync"

	"orbyte/internal/platform/analytics"
)

type AnalyticsStream struct {
	mu          sync.RWMutex
	nextID      int
	latest      analytics.Snapshot
	hasLatest   bool
	subscribers map[int]chan analytics.Snapshot
}

func NewAnalyticsStream() *AnalyticsStream {
	return &AnalyticsStream{subscribers: map[int]chan analytics.Snapshot{}}
}

func (s *AnalyticsStream) Publish(snapshot analytics.Snapshot) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.latest = snapshot
	s.hasLatest = true
	subscribers := make([]chan analytics.Snapshot, 0, len(s.subscribers))
	for _, ch := range s.subscribers {
		subscribers = append(subscribers, ch)
	}
	s.mu.Unlock()

	for _, ch := range subscribers {
		select {
		case ch <- snapshot:
		default:
			select {
			case <-ch:
			default:
			}
			select {
			case ch <- snapshot:
			default:
			}
		}
	}
}

func (s *AnalyticsStream) Latest() (analytics.Snapshot, bool) {
	if s == nil {
		return analytics.Snapshot{}, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.hasLatest {
		return analytics.Snapshot{}, false
	}
	return s.latest, true
}

func (s *AnalyticsStream) Subscribe() (<-chan analytics.Snapshot, func()) {
	if s == nil {
		ch := make(chan analytics.Snapshot)
		close(ch)
		return ch, func() {}
	}
	ch := make(chan analytics.Snapshot, 1)
	s.mu.Lock()
	id := s.nextID
	s.nextID++
	s.subscribers[id] = ch
	s.mu.Unlock()
	return ch, func() {
		s.mu.Lock()
		if existing, ok := s.subscribers[id]; ok {
			delete(s.subscribers, id)
			close(existing)
		}
		s.mu.Unlock()
	}
}
