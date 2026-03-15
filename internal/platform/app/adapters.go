package app

import (
	"strings"
	"time"

	"orbyte/internal/platform/eventing"
)

func configureAdapters(graph *serviceGraph) []func() error {
	closers := []func() error{}
	natsCfg := graph.config.NATSPolicy()
	if !natsCfg.Enabled || natsCfg.URL == "" {
		return closers
	}
	routes := externalBrokerRoutes(graph.modules, natsCfg.SubjectPrefix)
	if len(routes) == 0 {
		return closers
	}
	publisher, err := eventing.NewNATSPublisher(natsCfg.URL, time.Duration(natsCfg.TimeoutSeconds)*time.Second)
	if err != nil {
		graph.logger.Error("nats publisher unavailable", map[string]any{"error": err.Error(), "url": natsCfg.URL})
		return closers
	}
	sinkName := firstValue(strings.TrimSpace(natsCfg.SinkName), "nats")
	graph.eventing.RegisterBrokerSink(sinkName, publisher, routes)
	return append(closers, publisher.Close)
}
