package eventing

type Repository interface {
	SaveEvent(event Event) error
	SaveOutbox(record OutboxRecord) error
	SaveDelivery(record OutboxDeliveryRecord) error
	GetEvent(eventID string) (Event, bool)
	ListEvents() []Event
	ListOutbox() []OutboxRecord
	ListDeliveries() []OutboxDeliveryRecord
	ListDeliveriesByOutbox(outboxID string) []OutboxDeliveryRecord
	ListDeadLetters() []DeadLetterRecord
	ClaimPendingDeliveries(limit int) []OutboxDeliveryRecord
	MarkDispatched(outboxID string, update OutboxRecord) error
	MarkFailed(outboxID string, update OutboxRecord) error
	MarkDeliveryDispatched(deliveryID string, update OutboxDeliveryRecord) error
	MarkDeliveryFailed(deliveryID string, update OutboxDeliveryRecord) error
	SaveDeadLetter(record DeadLetterRecord) error
}
