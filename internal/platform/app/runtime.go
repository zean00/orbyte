package app

import (
	"time"

	"orbyte/internal/platform/analytics"
	"orbyte/internal/platform/eventing"
	"orbyte/internal/platform/searchruntime"
)

type runtimeGraph struct {
	dispatcher         *eventing.Dispatcher
	analyticsScheduler *analytics.Scheduler
}

func configureRuntime(graph *serviceGraph) *runtimeGraph {
	searchruntime.Attach(graph.search, graph.documents, graph.models, graph.jobs, graph.fieldSecurity, graph.eventing)
	graph.analytics.AttachRuntime(graph.jobs)
	graph.integration.AttachRuntime(graph.policy, graph.jobs)
	bootstrapRuntimeModuleContracts(graph.modules, graph.observability, graph.analytics)

	dispatcher := eventing.NewDispatcher(graph.eventing, time.Second, 50)
	scheduler := analytics.NewScheduler(graph.analytics, time.Minute, 30*24*time.Hour)
	graph.jobs.SetHealthHooks(func() { graph.runtimeHealth.MarkSuccess("jobs") }, func(err error) { graph.runtimeHealth.MarkFailure("jobs", err) })
	dispatcher.SetHealthHooks(func() { graph.runtimeHealth.MarkSuccess("dispatcher") }, func(err error) { graph.runtimeHealth.MarkFailure("dispatcher", err) })
	scheduler.SetHealthHooks(func() { graph.runtimeHealth.MarkSuccess("scheduler") }, func(err error) { graph.runtimeHealth.MarkFailure("scheduler", err) })

	return &runtimeGraph{
		dispatcher:         dispatcher,
		analyticsScheduler: scheduler,
	}
}
