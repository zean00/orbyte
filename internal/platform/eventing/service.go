package eventing

import (
	"context"
	"fmt"
	"time"

	"clinic/internal/platform/logging"
	"clinic/internal/platform/observability"
)

const maxAttempts = 3

type Service struct {
	repo          Repository
	handlers      map[string][]Handler
	sinks         map[string]DispatchSink
	logger        *logging.Service
	observability *observability.Service
}

type DispatchResult struct {
	Attempted    int
	Dispatched   int
	Failed       int
	DeadLettered int
	Retried      int
}

type Handler interface {
	Handle(ctx context.Context, event Event) error
}

type DispatchSink interface {
	Name() string
	Deliver(ctx context.Context, event Event) error
	Accepts(event Event) bool
}

type Publisher interface {
	Publish(ctx context.Context, topic string, key string, event Event) error
}

func NewService() *Service {
	return NewServiceWithRepository(NewMemoryRepository(), observability.NewService(), logging.NewService())
}

func NewServiceWithRepository(repo Repository, obs *observability.Service, logger *logging.Service) *Service {
	if obs == nil {
		obs = observability.NewService()
	}
	if logger == nil {
		logger = logging.NewService()
	}
	svc := &Service{repo: repo, handlers: map[string][]Handler{}, sinks: map[string]DispatchSink{}, observability: obs, logger: logger}
	svc.RegisterSink(newLocalHandlerSink(svc))
	return svc
}

func (s *Service) RegisterHandler(eventType string, handler Handler) {
	s.handlers[eventType] = append(s.handlers[eventType], handler)
}

func (s *Service) RegisterSink(sink DispatchSink) {
	if sink == nil || sink.Name() == "" {
		return
	}
	s.sinks[sink.Name()] = sink
}

func (s *Service) RegisterBrokerSink(name string, publisher Publisher, routes map[string]string) {
	if publisher == nil || name == "" {
		return
	}
	s.RegisterSink(&brokerSink{name: name, publisher: publisher, routes: routes})
}

func (s *Service) Record(event Event) error {
	if event.SchemaVersion == "" {
		event.SchemaVersion = "v1"
	}
	if err := s.repo.SaveEvent(event); err != nil {
		return err
	}
	_ = s.observability.RecordDomainEvent(event.Type, event.CorrelationID != "")
	s.observability.Inc("domain.events.recorded.total")
	s.logger.Info("domain event recorded", map[string]any{"event_id": event.ID, "event_type": event.Type, "aggregate_id": event.AggregateID})
	outbox := OutboxRecord{
		ID:        event.ID + ":outbox",
		EventID:   event.ID,
		EventType: event.Type,
		Status:    "pending",
		CreatedAt: time.Now().UTC(),
	}
	if err := s.repo.SaveOutbox(outbox); err != nil {
		return err
	}
	for _, sink := range s.sinks {
		if !sink.Accepts(event) {
			continue
		}
		if err := s.repo.SaveDelivery(OutboxDeliveryRecord{
			ID:        outbox.ID + ":" + sink.Name(),
			OutboxID:  outbox.ID,
			EventID:   outbox.EventID,
			EventType: outbox.EventType,
			SinkName:  sink.Name(),
			Status:    "pending",
			CreatedAt: outbox.CreatedAt,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ListEvents() []Event {
	return s.repo.ListEvents()
}

func (s *Service) ListOutbox() []OutboxRecord {
	return s.repo.ListOutbox()
}

func (s *Service) ListDeliveries() []OutboxDeliveryRecord {
	return s.repo.ListDeliveries()
}

func (s *Service) ListDeadLetters() []DeadLetterRecord {
	return s.repo.ListDeadLetters()
}

func (s *Service) DispatchPending(limit int) (int, error) {
	result, err := s.DispatchPendingDetailed(limit)
	return result.Dispatched, err
}

func (s *Service) DispatchPendingDetailed(limit int) (DispatchResult, error) {
	result := DispatchResult{}
	if err := s.ensureDeliveries(); err != nil {
		return result, err
	}
	items := s.repo.ClaimPendingDeliveries(limit)
	result.Attempted = len(items)
	for _, item := range items {
		s.logger.Info("outbox dispatch started", map[string]any{"outbox_id": item.OutboxID, "event_id": item.EventID, "sink": item.SinkName, "attempt_count": item.AttemptCount})
		event, ok := s.repo.GetEvent(item.EventID)
		if !ok {
			s.observability.Inc("outbox.dispatch.missing_event.total")
			status, err := s.failOrDeadLetter(item, "missing event")
			if err != nil {
				return result, err
			}
			result.Failed++
			if status == "dead_letter" {
				result.DeadLettered++
			} else {
				result.Retried++
			}
			continue
		}
		sink, ok := s.sinks[item.SinkName]
		if !ok {
			status, err := s.failOrDeadLetter(item, "missing sink")
			if err != nil {
				return result, err
			}
			result.Failed++
			if status == "dead_letter" {
				result.DeadLettered++
			} else {
				result.Retried++
			}
			continue
		}
		started := time.Now()
		if err := sink.Deliver(context.Background(), event); err != nil {
			s.observability.Inc("outbox.dispatch.handler_failed.total")
			s.observability.Observe("outbox.dispatch.handler.duration", time.Since(started))
			s.logger.Error("outbox sink failed", map[string]any{"outbox_id": item.OutboxID, "event_id": event.ID, "event_type": event.Type, "sink": item.SinkName, "error": err.Error(), "attempt_count": item.AttemptCount})
			status, dlErr := s.failOrDeadLetter(item, err.Error())
			if dlErr != nil {
				return result, dlErr
			}
			result.Failed++
			if status == "dead_letter" {
				result.DeadLettered++
			} else {
				result.Retried++
			}
			continue
		}
		s.observability.Observe("outbox.dispatch.handler.duration", time.Since(started))
		if err := s.repo.MarkDeliveryDispatched(item.ID, OutboxDeliveryRecord{Status: "dispatched", DispatchedAt: time.Now().UTC()}); err != nil {
			return result, err
		}
		if err := s.refreshOutboxStatus(item.OutboxID); err != nil {
			return result, err
		}
		s.observability.Inc("outbox.dispatch.success.total")
		s.logger.Info("outbox dispatch completed", map[string]any{"outbox_id": item.OutboxID, "event_id": item.EventID, "event_type": item.EventType, "sink": item.SinkName})
		result.Dispatched++
	}
	return result, nil
}

func (s *Service) ensureDeliveries() error {
	known := map[string]struct{}{}
	for _, delivery := range s.repo.ListDeliveries() {
		known[delivery.OutboxID+":"+delivery.SinkName] = struct{}{}
	}
	for _, outbox := range s.repo.ListOutbox() {
		event, ok := s.repo.GetEvent(outbox.EventID)
		if !ok {
			continue
		}
		for _, sink := range s.sinks {
			if !sink.Accepts(event) {
				continue
			}
			key := outbox.ID + ":" + sink.Name()
			if _, ok := known[key]; ok {
				continue
			}
			if err := s.repo.SaveDelivery(OutboxDeliveryRecord{
				ID:        key,
				OutboxID:  outbox.ID,
				EventID:   outbox.EventID,
				EventType: outbox.EventType,
				SinkName:  sink.Name(),
				Status:    "pending",
				CreatedAt: outbox.CreatedAt,
			}); err != nil {
				return err
			}
			known[key] = struct{}{}
		}
	}
	return nil
}

func (s *Service) failOrDeadLetter(item OutboxDeliveryRecord, reason string) (string, error) {
	if item.AttemptCount >= maxAttempts {
		s.observability.Inc("outbox.dead_letter.total")
		s.logger.Error("outbox moved to dead letter", map[string]any{"outbox_id": item.OutboxID, "event_id": item.EventID, "sink": item.SinkName, "reason": reason, "attempt_count": item.AttemptCount})
		if err := s.repo.MarkDeliveryFailed(item.ID, OutboxDeliveryRecord{Status: "dead_letter", LastError: reason, AttemptCount: item.AttemptCount}); err != nil {
			return "", err
		}
		if err := s.refreshOutboxStatus(item.OutboxID); err != nil {
			return "", err
		}
		if err := s.repo.SaveDeadLetter(DeadLetterRecord{
			ID:           fmt.Sprintf("dead:%s:%s:%d", item.OutboxID, item.SinkName, item.AttemptCount),
			OutboxID:     item.OutboxID,
			EventID:      item.EventID,
			EventType:    item.EventType,
			SinkName:     item.SinkName,
			Reason:       reason,
			AttemptCount: item.AttemptCount,
			CreatedAt:    time.Now().UTC(),
		}); err != nil {
			return "", err
		}
		return "dead_letter", nil
	}
	s.observability.Inc("outbox.dispatch.retry.total")
	s.logger.Error("outbox dispatch scheduled for retry", map[string]any{"outbox_id": item.OutboxID, "event_id": item.EventID, "sink": item.SinkName, "reason": reason, "attempt_count": item.AttemptCount})
	if err := s.repo.MarkDeliveryFailed(item.ID, OutboxDeliveryRecord{Status: "pending", LastError: reason, AttemptCount: item.AttemptCount}); err != nil {
		return "", err
	}
	return "pending", s.refreshOutboxStatus(item.OutboxID)
}

func (s *Service) refreshOutboxStatus(outboxID string) error {
	deliveries := s.repo.ListDeliveriesByOutbox(outboxID)
	status := "pending"
	lastError := ""
	dispatchedAt := time.Time{}
	if len(deliveries) == 0 {
		return nil
	}
	allDispatched := true
	allDead := true
	anyProcessing := false
	for _, delivery := range deliveries {
		switch delivery.Status {
		case "processing":
			anyProcessing = true
			allDispatched = false
			allDead = false
		case "pending":
			allDispatched = false
			allDead = false
		case "dispatched":
			if dispatchedAt.IsZero() || delivery.DispatchedAt.After(dispatchedAt) {
				dispatchedAt = delivery.DispatchedAt
			}
			allDead = false
		case "dead_letter":
			allDispatched = false
			lastError = delivery.LastError
		default:
			allDispatched = false
			allDead = false
		}
	}
	switch {
	case allDispatched:
		status = "dispatched"
	case anyProcessing:
		status = "processing"
	case allDead:
		status = "dead_letter"
	default:
		status = "pending"
	}
	if status == "dead_letter" {
		return s.repo.MarkFailed(outboxID, OutboxRecord{Status: status, LastError: lastError})
	}
	if status == "dispatched" {
		return s.repo.MarkDispatched(outboxID, OutboxRecord{Status: status, DispatchedAt: dispatchedAt})
	}
	return s.repo.MarkFailed(outboxID, OutboxRecord{Status: status, LastError: lastError})
}

type Dispatcher struct {
	service   *Service
	interval  time.Duration
	limit     int
	cancel    context.CancelFunc
	onSuccess func()
	onFailure func(error)
}

func NewDispatcher(service *Service, interval time.Duration, limit int) *Dispatcher {
	if interval <= 0 {
		interval = time.Second
	}
	if limit <= 0 {
		limit = 50
	}
	return &Dispatcher{service: service, interval: interval, limit: limit}
}

func (d *Dispatcher) SetHealthHooks(onSuccess func(), onFailure func(error)) {
	if d == nil {
		return
	}
	d.onSuccess = onSuccess
	d.onFailure = onFailure
}

func (d *Dispatcher) Start(parent context.Context) {
	ctx, cancel := context.WithCancel(parent)
	d.cancel = cancel
	go func() {
		ticker := time.NewTicker(d.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				result, err := d.service.DispatchPendingDetailed(d.limit)
				if err != nil {
					if d.onFailure != nil {
						d.onFailure(err)
					}
					continue
				}
				if result.Failed > 0 {
					if d.onFailure != nil {
						d.onFailure(fmt.Errorf("%d outbox delivery failure(s)", result.Failed))
					}
					continue
				}
				if result.Dispatched > 0 && d.onSuccess != nil {
					d.onSuccess()
				}
			}
		}
	}()
}

func (d *Dispatcher) Stop() {
	if d.cancel != nil {
		d.cancel()
	}
}

type localHandlerSink struct {
	service *Service
}

func newLocalHandlerSink(service *Service) *localHandlerSink {
	return &localHandlerSink{service: service}
}

func (s *localHandlerSink) Name() string       { return "local" }
func (s *localHandlerSink) Accepts(Event) bool { return true }
func (s *localHandlerSink) Deliver(ctx context.Context, event Event) error {
	for _, handler := range s.service.handlers[event.Type] {
		if err := handler.Handle(ctx, event); err != nil {
			return err
		}
	}
	return nil
}

type brokerSink struct {
	name      string
	publisher Publisher
	routes    map[string]string
}

func (s *brokerSink) Name() string { return s.name }
func (s *brokerSink) Accepts(event Event) bool {
	_, ok := s.routes[event.Type]
	return ok
}
func (s *brokerSink) Deliver(ctx context.Context, event Event) error {
	topic, ok := s.routes[event.Type]
	if !ok {
		return nil
	}
	return s.publisher.Publish(ctx, topic, event.AggregateID, event)
}

type NoopPublisher struct{}

func (NoopPublisher) Publish(context.Context, string, string, Event) error { return nil }
