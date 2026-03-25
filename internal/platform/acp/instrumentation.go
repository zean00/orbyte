package acp

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"orbyte/internal/platform/observability"
)

type Instrumentation struct {
	obs        *observability.Service
	tracer     trace.Tracer
	metricKeys struct {
		sessionsStarted  string
		sessionsEnded    string
		sessionsActive   string
		sessionDuration  string
	}
}

func NewInstrumentation(obs *observability.Service, tracer trace.Tracer) *Instrumentation {
	i := &Instrumentation{
		obs:    obs,
		tracer: tracer,
	}
	i.metricKeys.sessionsStarted = "acp.sessions.started.total"
	i.metricKeys.sessionsEnded = "acp.sessions.ended.total"
	i.metricKeys.sessionsActive = "acp.sessions.active"
	i.metricKeys.sessionDuration = "acp.session.duration.seconds"
	return i
}

func (i *Instrumentation) RecordSessionStarted() {
	if i.obs == nil {
		return
	}
	i.obs.Inc(i.metricKeys.sessionsStarted)
	i.obs.Inc(i.metricKeys.sessionsActive)
}

func (i *Instrumentation) RecordSessionEnded(reason string) {
	if i.obs == nil {
		return
	}
	labels := map[string]string{"reason": reason}
	_ = i.obs.RecordMetric(i.metricKeys.sessionsEnded, labels, 1)
	i.obs.Add(i.metricKeys.sessionsActive, -1)
}

func (i *Instrumentation) RecordSessionDuration(duration time.Duration) {
	if i.obs == nil {
		return
	}
	i.obs.Observe(i.metricKeys.sessionDuration, duration)
}

func (i *Instrumentation) StartSessionSpan(ctx context.Context, sessionID, userID string) (context.Context, trace.Span) {
	if i.tracer == nil {
		return ctx, nil
	}
	return i.tracer.Start(ctx, "acp.session.start",
		trace.WithAttributes(
			attribute.String("session.id", sessionID),
			attribute.String("user.id", userID),
		),
	)
}

func (i *Instrumentation) EndSessionSpan(span trace.Span, err error) {
	if span == nil {
		return
	}
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
	}
	span.End()
}

func (i *Instrumentation) StartPromptSpan(ctx context.Context, sessionID string) (context.Context, trace.Span) {
	if i.tracer == nil {
		return ctx, nil
	}
	return i.tracer.Start(ctx, "acp.session.prompt",
		trace.WithAttributes(
			attribute.String("session.id", sessionID),
		),
	)
}

func (i *Instrumentation) StartApprovalSpan(ctx context.Context, sessionID, approvalID string) (context.Context, trace.Span) {
	if i.tracer == nil {
		return ctx, nil
	}
	return i.tracer.Start(ctx, "acp.session.approval",
		trace.WithAttributes(
			attribute.String("session.id", sessionID),
			attribute.String("approval.id", approvalID),
		),
	)
}
