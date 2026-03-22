package app

import (
	"time"

	"orbyte/internal/platform/analytics"
	"orbyte/internal/platform/eventing"
	"orbyte/internal/platform/runtimehealth"
	"orbyte/internal/platform/searchruntime"
)

type runtimeGraph struct {
	dispatcher         *eventing.Dispatcher
	analyticsScheduler *analytics.Scheduler
}

func configureRuntime(graph *serviceGraph) *runtimeGraph {
	searchruntime.Attach(graph.search, graph.documents, graph.models, graph.jobs, graph.fieldSecurity, graph.eventing)
	graph.analytics.AttachRuntime(graph.jobs)
	graph.integration.AttachRuntime(graph.policy, graph.jobs, graph.secrets)
	graph.dataops.AttachJobs(graph.jobs)
	bootstrapRuntimeModuleContracts(graph.modules, graph.observability, graph.analytics)

	graph.runtimeHealth.ConfigureSubsystem("jobs", runtimehealth.SubsystemConfig{FailureCategory: "handler_failure", RunbookID: "runtime.jobs", OperatorHint: "Inspect failed or dead-letter jobs and requeue only after correcting the underlying handler/runtime issue.", ImpactsReadiness: true})
	graph.runtimeHealth.ConfigureSubsystem("dispatcher", runtimehealth.SubsystemConfig{FailureCategory: "dispatch_failure", RunbookID: "runtime.outbox", OperatorHint: "Inspect outbox deliveries and dead letters, then retry affected deliveries after fixing the sink/runtime issue.", ImpactsReadiness: true})
	graph.runtimeHealth.ConfigureSubsystem("scheduler", runtimehealth.SubsystemConfig{FailureCategory: "handler_failure", RunbookID: "runtime.scheduler", OperatorHint: "Inspect scheduled analytics/reporting jobs and retry the failed scheduler workload once dependencies are healthy.", ImpactsReadiness: false})

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
