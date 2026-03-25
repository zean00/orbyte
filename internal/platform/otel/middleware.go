package otel

import (
	"net/http"
	"strings"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

type HTTPInstrumentationOptions struct {
	Tracer     trace.Tracer
	Propagator propagation.TextMapPropagator
	Operation  string
}

func NewHTTPHandler(handler http.Handler, opts HTTPInstrumentationOptions) http.Handler {
	operation := strings.TrimSpace(opts.Operation)
	if operation == "" {
		operation = "http.server"
	}
	return otelhttp.NewHandler(
		handler,
		operation,
		otelhttp.WithTracerProvider(otel.GetTracerProvider()),
		otelhttp.WithPropagators(opts.Propagator),
	)
}

func getStatusCode(w http.ResponseWriter) int {
	if sc, ok := w.(statusCoder); ok {
		return sc.StatusCode()
	}
	return http.StatusOK
}

type statusCoder interface {
	StatusCode() int
}
