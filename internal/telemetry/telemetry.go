// Package telemetry wires OpenTelemetry traces and metrics for the agent.

// Instrumentation call sites never check whether telemetry is enabled: when
// Init is called with Enabled=false no global provider is registered, and the
// OTel API returns no-op tracers and meters. That is what makes the whole
// thing opt-in without conditionals scattered through the agent loop.
package telemetry

import (
	"context"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
)

const scopeName = "github.com/lucasmonteverdi1/coding-agent"

type Config struct {
	Enabled        bool
	Endpoint       string
	ServiceName    string
	CaptureContent bool
}

// ConfigFromEnv fills unset fields from the standard OTel environment
// variables. Explicit flag values win over the environment.
func ConfigFromEnv(cfg Config) Config {
	if cfg.Endpoint == "" {
		cfg.Endpoint = os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	}
	if cfg.Endpoint == "" {
		cfg.Endpoint = "localhost:4317"
	}
	if cfg.ServiceName == "" {
		cfg.ServiceName = os.Getenv("OTEL_SERVICE_NAME")
	}
	if cfg.ServiceName == "" {
		cfg.ServiceName = "coding-agent"
	}
	if !cfg.CaptureContent {
		cfg.CaptureContent, _ = strconv.ParseBool(os.Getenv("OTEL_GENAI_CAPTURE_MESSAGE_CONTENT"))
	}
	return cfg
}

var captureContent bool

// CaptureContent reports whether prompt/response content may be recorded.
// Off unless OTEL_GENAI_CAPTURE_MESSAGE_CONTENT=true.
func CaptureContent() bool { return captureContent }

// exportErrorMessage turns an export failure into something a user can act on.
// A backend that only speaks traces (Jaeger, for one) is a valid setup, not a
// misconfiguration, so it gets a note rather than an error that sends people
// hunting for a bug that isn't there.
func exportErrorMessage(err error) string {
	msg := err.Error()
	if strings.Contains(msg, "Unimplemented") && strings.Contains(msg, "MetricsService") {
		return "otel: this backend accepts traces but not metrics (Jaeger works this way) — " +
			"traces are unaffected. Use an OpenTelemetry Collector to view metrics.\n"
	}
	return "otel: export error (further errors suppressed): " + msg + "\n"
}

// Init registers the global tracer and meter providers and returns a shutdown
// function that flushes pending spans. When cfg.Enabled is false it is a no-op
// and returns a no-op shutdown, so callers can always defer the result.
func Init(ctx context.Context, cfg Config) (func(context.Context) error, error) {
	noop := func(context.Context) error { return nil }
	if !cfg.Enabled {
		return noop, nil
	}

	captureContent = cfg.CaptureContent

	// Export failures must never reach the REPL's stdout, a dead collector is
	// not the user's problem. Log once so a misconfiguration is still findable.
	var once sync.Once
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		once.Do(func() {
			os.Stderr.WriteString(exportErrorMessage(err))
		})
	}))

	res, err := resource.Merge(resource.Default(), resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(cfg.ServiceName),
	))
	if err != nil {
		return noop, err
	}

	// The gRPC exporters connect lazily and retry in the background, so a
	// missing collector costs nothing at startup and never blocks a turn.
	traceExp, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(cfg.Endpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return noop, err
	}

	metricExp, err := otlpmetricgrpc.New(ctx,
		otlpmetricgrpc.WithEndpoint(cfg.Endpoint),
		otlpmetricgrpc.WithInsecure(),
	)
	if err != nil {
		return noop, err
	}

	tp := trace.NewTracerProvider(
		trace.WithBatcher(traceExp),
		trace.WithResource(res),
	)
	mp := metric.NewMeterProvider(
		metric.WithReader(metric.NewPeriodicReader(metricExp)),
		metric.WithResource(res),
	)

	otel.SetTracerProvider(tp)
	otel.SetMeterProvider(mp)

	return func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		// Flush before shutdown so a turn that just ended is not lost.
		tp.ForceFlush(ctx)
		mp.ForceFlush(ctx)

		// Both are shut down even if the first fails; only the trace error is
		// returned, since a backend that rejects metrics is a valid setup and
		// must not make exiting the REPL look like a failure.
		traceErr := tp.Shutdown(ctx)
		mp.Shutdown(ctx)
		return traceErr
	}, nil
}
