package otel

import (
	"context"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.39.0"
	"go.opentelemetry.io/otel/trace"
)

type Service struct {
	tracerProvider *sdktrace.TracerProvider
	tracer         trace.Tracer
	meter          interface{}
	propagator     propagation.TextMapPropagator
	serviceName    string
}

func NewService(serviceName string) *Service {
	return &Service{
		serviceName: serviceName,
		propagator:  otel.GetTextMapPropagator(),
	}
}

func (s *Service) Tracer() trace.Tracer {
	if s.tracer == nil {
		s.tracer = otel.Tracer(s.serviceName)
	}
	return s.tracer
}

func (s *Service) Propagator() propagation.TextMapPropagator {
	return s.propagator
}

func (s *Service) ServiceName() string {
	return s.serviceName
}

func (s *Service) StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return s.Tracer().Start(ctx, name, opts...)
}

func (s *Service) EndSpan(span trace.Span, err error) {
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
	}
	span.End()
}

func (s *Service) AddSpanAttributes(span trace.Span, attrs ...attribute.KeyValue) {
	span.SetAttributes(attrs...)
}

func (s *Service) InjectTraceContext(ctx context.Context, carrier propagation.TextMapCarrier) context.Context {
	s.propagator.Inject(ctx, carrier)
	return ctx
}

func (s *Service) ExtractTraceContext(ctx context.Context, carrier propagation.TextMapCarrier) context.Context {
	return s.propagator.Extract(ctx, carrier)
}

func (s *Service) SpanFromContext(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}

func createResource(serviceName, version string) (*resource.Resource, error) {
	return resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(version),
		),
	)
}

func InitTracerProvider(ctx context.Context, serviceName, version string, exporter sdktrace.SpanExporter) (*sdktrace.TracerProvider, error) {
	res, err := createResource(serviceName, version)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	return tp, nil
}

func InitStdoutTracerProvider(ctx context.Context, serviceName, version string) (*sdktrace.TracerProvider, error) {
	exporter, err := stdouttrace.New(
		stdouttrace.WithWriter(os.Stdout),
	)
	if err != nil {
		return nil, err
	}
	return InitTracerProvider(ctx, serviceName, version, exporter)
}

func Shutdown(ctx context.Context, tp *sdktrace.TracerProvider) error {
	if tp == nil {
		return nil
	}
	return tp.Shutdown(ctx)
}
