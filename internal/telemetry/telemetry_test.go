package telemetry

import (
	"errors"
	"strings"
	"testing"
)

// --- exportErrorMessage ---

func TestExportErrorMessage_MetricsUnimplementedIsExplainedAsBenign(t *testing.T) {
	// What Jaeger returns: it speaks OTLP traces but not OTLP metrics.
	err := errors.New("failed to upload metrics: rpc error: code = Unimplemented " +
		"desc = unknown service opentelemetry.proto.collector.metrics.v1.MetricsService")

	msg := exportErrorMessage(err)

	if !strings.Contains(msg, "traces are unaffected") {
		t.Errorf("expected the message to say traces still work, got: %s", msg)
	}
	if strings.Contains(msg, "export error") {
		t.Errorf("a trace-only backend is a valid setup, not an error: %s", msg)
	}
}

func TestExportErrorMessage_OtherErrorsStaySurfaced(t *testing.T) {
	err := errors.New("connection refused")
	msg := exportErrorMessage(err)

	if !strings.Contains(msg, "export error") {
		t.Errorf("expected a genuine failure to be reported as an error, got: %s", msg)
	}
	if !strings.Contains(msg, "connection refused") {
		t.Errorf("expected the underlying cause to be preserved, got: %s", msg)
	}
}

func TestExportErrorMessage_UnimplementedTracesIsNotMuffled(t *testing.T) {
	// Unimplemented on the *trace* service is a real problem: nothing arrives.
	err := errors.New("rpc error: code = Unimplemented desc = unknown service " +
		"opentelemetry.proto.collector.trace.v1.TraceService")

	if msg := exportErrorMessage(err); !strings.Contains(msg, "export error") {
		t.Errorf("expected a failing trace service to be reported, got: %s", msg)
	}
}

// --- Init ---

func TestInit_DisabledReturnsUsableShutdown(t *testing.T) {
	shutdown, err := Init(t.Context(), Config{Enabled: false})
	if err != nil {
		t.Fatalf("disabled init must not error: %v", err)
	}
	if shutdown == nil {
		t.Fatal("shutdown must be non-nil so callers can always defer it")
	}
	if err := shutdown(t.Context()); err != nil {
		t.Errorf("no-op shutdown must not error: %v", err)
	}
}

// --- ConfigFromEnv ---

func TestConfigFromEnv_AppliesDefaults(t *testing.T) {
	cfg := ConfigFromEnv(Config{})
	if cfg.Endpoint != "localhost:4317" {
		t.Errorf("expected default endpoint, got %s", cfg.Endpoint)
	}
	if cfg.ServiceName != "coding-agent" {
		t.Errorf("expected default service name, got %s", cfg.ServiceName)
	}
	if cfg.CaptureContent {
		t.Error("content capture must be off unless explicitly enabled")
	}
}

func TestConfigFromEnv_ExplicitValuesWinOverEnv(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "from-env:4317")

	cfg := ConfigFromEnv(Config{Endpoint: "explicit:4317"})
	if cfg.Endpoint != "explicit:4317" {
		t.Errorf("expected the explicit flag value to win, got %s", cfg.Endpoint)
	}
}

func TestConfigFromEnv_ReadsEndpointFromEnv(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "collector.internal:4317")

	if cfg := ConfigFromEnv(Config{}); cfg.Endpoint != "collector.internal:4317" {
		t.Errorf("expected the env endpoint, got %s", cfg.Endpoint)
	}
}

func TestConfigFromEnv_CaptureContentOptIn(t *testing.T) {
	t.Setenv("OTEL_GENAI_CAPTURE_MESSAGE_CONTENT", "true")

	if cfg := ConfigFromEnv(Config{}); !cfg.CaptureContent {
		t.Error("expected capture to be enabled when the env var is true")
	}
}
