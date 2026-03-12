package eventing

import (
	"context"
	"encoding/json"
	"time"

	"github.com/nats-io/nats.go"
)

type NATSPublisher struct {
	conn *nats.Conn
}

func NewNATSPublisher(url string, timeout time.Duration) (*NATSPublisher, error) {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	conn, err := nats.Connect(url, nats.Timeout(timeout))
	if err != nil {
		return nil, err
	}
	return &NATSPublisher{conn: conn}, nil
}

func (p *NATSPublisher) Publish(_ context.Context, topic string, key string, event Event) error {
	body, err := json.Marshal(event)
	if err != nil {
		return err
	}
	msg := &nats.Msg{Subject: topic, Data: body, Header: nats.Header{}}
	msg.Header.Set("X-Event-ID", event.ID)
	msg.Header.Set("X-Event-Type", event.Type)
	msg.Header.Set("X-Aggregate-ID", event.AggregateID)
	msg.Header.Set("X-Aggregate-Type", event.AggregateType)
	msg.Header.Set("X-Event-Key", key)
	if event.CorrelationID != "" {
		msg.Header.Set("X-Correlation-ID", event.CorrelationID)
	}
	if event.SchemaVersion != "" {
		msg.Header.Set("X-Schema-Version", event.SchemaVersion)
	}
	return p.conn.PublishMsg(msg)
}

func (p *NATSPublisher) Close() error {
	if p == nil || p.conn == nil {
		return nil
	}
	p.conn.Close()
	return nil
}
